// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Zero Root AI

package serve

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/a2aproject/a2a-go/v2/a2a"
	"log/slog"
	"time"

	graphragpb "github.com/zeroroot-ai/sdk/api/gen/gibson/graphrag/v1"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
	"google.golang.org/protobuf/encoding/protojson"
	protolib "google.golang.org/protobuf/proto"

	"github.com/zeroroot-ai/sdk/agent"
	"github.com/zeroroot-ai/sdk/codegen/workspace"
	"github.com/zeroroot-ai/sdk/finding"
	harnessconst "github.com/zeroroot-ai/sdk/harness"
	"github.com/zeroroot-ai/sdk/llm"
	"github.com/zeroroot-ai/sdk/mission"
	"github.com/zeroroot-ai/sdk/openresponses"
	"github.com/zeroroot-ai/sdk/planning"
	"github.com/zeroroot-ai/sdk/plugin"
	"github.com/zeroroot-ai/sdk/tool"
	"github.com/zeroroot-ai/sdk/types"
)

// Compile-time assertion: PlatformHarness must fully satisfy agent.Harness.
var _ agent.Harness = (*PlatformHarness)(nil)

// PlatformHarness implements agent.Harness by proxying all operations through
// PlatformClient's RPC methods to the Gibson platform. It is used when an
// agent runs in pull mode: the agent polls the platform for work items, and
// all harness calls are routed back through that same RPC connection.
//
// PlatformHarness is thread-safe. The workID field is set at construction time
// and never mutated, so no locking is required for reads of the context fields.
type PlatformHarness struct {
	client  *PlatformClient
	workID  string // identifies the active work item for platform routing
	mission types.MissionContext
	target  types.TargetInfo
	logger  *slog.Logger
	tracer  trace.Tracer

	// grant is the task grant of the work item. Gateway calls send it.
	grant *taskGrant

	// tokenTracker accumulates token usage from the model client.
	tokenTracker *CallbackTokenTracker
}

// NewPlatformHarness constructs a PlatformHarness for a polling work item.
//
// The client must already be connected (Connect has been called and succeeded).
// Use WithPlatformWorkID to set the work item ID returned by PollWork.
// Defaults: slog.Default() logger, no-op OTel tracer, empty work ID / mission / target.
func NewPlatformHarness(client *PlatformClient, opts ...PlatformHarnessOption) *PlatformHarness {
	h := &PlatformHarness{
		client:       client,
		logger:       slog.Default(),
		tracer:       defaultNoopTracer(),
		tokenTracker: NewCallbackTokenTracker(),
	}
	for _, opt := range opts {
		opt(h)
	}
	// The harness renews the grant through its own platform client.
	if h.grant != nil && h.grant.renew == nil {
		h.grant.renew = client.RenewTaskGrant
	}
	return h
}

// Logger returns the structured logger for the agent.
func (h *PlatformHarness) Logger() *slog.Logger { return h.logger }

// Tracer returns the OpenTelemetry tracer for distributed tracing.
func (h *PlatformHarness) Tracer() trace.Tracer { return h.tracer }

// TokenUsage returns the accumulated token usage across all LLM slots.
func (h *PlatformHarness) TokenUsage() llm.TokenTracker { return h.tokenTracker }

// Mission returns the current mission context.
func (h *PlatformHarness) Mission() types.MissionContext { return h.mission }

// Target returns information about the target being tested.
func (h *PlatformHarness) Target() types.TargetInfo { return h.target }

// Model resolves a slot to one exact model through the platform and returns the
// client that calls the model gateway. The client sends the grant of the agent.
// It adds token counts to TokenUsage.
func (h *PlatformHarness) Model(ctx context.Context, slot string) (*openresponses.Client, error) {
	resp, err := h.client.ResolveModel(ctx, h.workID, slot)
	if err != nil {
		return nil, fmt.Errorf("PlatformHarness.Model: %w", err)
	}
	return newModelClient(slot, &resolvedModel{
		model: resp.GetModel(), provider: resp.GetProvider(), gatewayURL: resp.GetGatewayUrl(),
		routeHeaders: resp.GetRouteHeaders(), contextWindow: int(resp.GetContextWindow()),
		maxOutputTokens: int(resp.GetMaxOutputTokens()), gatewayCAPEM: resp.GetGatewayCaPem(),
	}, grantHeaders(h.grant.get), h.tokenTracker)
}

// CallToolProto invokes a tool by name, marshaling the request proto to JSON
// for transport and unmarshaling the JSON response back into resp.
// Uses protojson with UseProtoNames=true for snake_case consistency with the
// existing tool serve code.
func (h *PlatformHarness) CallToolProto(
	ctx context.Context, name string, request protolib.Message, response protolib.Message, opts ...agent.ToolCallOption,
) error {
	ctx, span := h.tracer.Start(ctx, "gen_ai.tool.proto",
		trace.WithSpanKind(trace.SpanKindClient),
		trace.WithAttributes(
			attribute.String("gibson.tool.name", name),
			attribute.String("gibson.tool.request_type", string(request.ProtoReflect().Descriptor().FullName())),
		),
	)
	defer span.End()

	marshaler := protojson.MarshalOptions{UseProtoNames: true}

	inputJSON, err := marshaler.Marshal(request)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "failed to marshal proto request to JSON")
		return fmt.Errorf("PlatformHarness.CallToolProto: marshal request: %w", err)
	}

	outputJSON, toolCallID, err := h.client.CallTool(ctx, h.workID, name, string(inputJSON))
	// The daemon records a failed call too, so the id is set before the error.
	agent.ApplyToolCallOptions(opts).SetToolCallID(toolCallID)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return fmt.Errorf("PlatformHarness.CallToolProto: %w", err)
	}

	unmarshaler := protojson.UnmarshalOptions{DiscardUnknown: true}
	if err := unmarshaler.Unmarshal([]byte(outputJSON), response); err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "failed to unmarshal proto response from JSON")
		return fmt.Errorf("PlatformHarness.CallToolProto: unmarshal response: %w", err)
	}

	return nil
}

// CallToolProtoStream invokes a tool with streaming event callbacks. The request
// proto is marshaled to JSON and sent via the platform's CallToolStream RPC.
// Events are dispatched to the callback as they arrive; the output proto is
// populated from the final "result" event.
func (h *PlatformHarness) CallToolProtoStream(
	ctx context.Context, toolName string, input protolib.Message, output protolib.Message,
	callback agent.ToolStreamCallback, opts ...agent.ToolCallOption,
) error {
	toolCallOpts := agent.ApplyToolCallOptions(opts)
	var toolCallID string
	defer func() { toolCallOpts.SetToolCallID(toolCallID) }()

	ctx, span := h.tracer.Start(ctx, "gen_ai.tool.proto.stream",
		trace.WithSpanKind(trace.SpanKindClient),
		trace.WithAttributes(
			attribute.String("gibson.tool.name", toolName),
		),
	)
	defer span.End()

	marshaler := protojson.MarshalOptions{UseProtoNames: true}
	inputJSON, err := marshaler.Marshal(input)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "failed to marshal proto input to JSON")
		return fmt.Errorf("PlatformHarness.CallToolProtoStream: marshal input: %w", err)
	}

	evtCh, err := h.client.CallToolStream(ctx, h.workID, toolName, string(inputJSON))
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return fmt.Errorf("PlatformHarness.CallToolProtoStream: %w", err)
	}

	unmarshaler := protojson.UnmarshalOptions{DiscardUnknown: true}

	for evt := range evtCh {
		if evt.ToolCallID != "" {
			toolCallID = evt.ToolCallID
		}
		if evt.Err != nil {
			span.RecordError(evt.Err)
			span.SetStatus(codes.Error, evt.Err.Error())
			if callback != nil {
				callback.OnError(evt.Err, true)
			}
			return fmt.Errorf("PlatformHarness.CallToolProtoStream: %w", evt.Err)
		}

		if callback == nil {
			// Still drain the channel; just collect the result.
			if evt.EventType == "result" && evt.PayloadJSON != "" {
				if err := unmarshaler.Unmarshal([]byte(evt.PayloadJSON), output); err != nil {
					return fmt.Errorf("PlatformHarness.CallToolProtoStream: unmarshal result: %w", err)
				}
			}
			continue
		}

		if err := dispatchToolEvent(evt, toolName, output, callback, unmarshaler); err != nil {
			span.RecordError(err)
			span.SetStatus(codes.Error, err.Error())
			return err
		}
	}

	return nil
}

// dispatchToolEvent sends one event of a pull-mode tool stream to the callback.
// It returns an error when the event is a result that cannot be read.
func dispatchToolEvent(
	evt ToolStreamEvent, toolName string, output protolib.Message,
	callback agent.ToolStreamCallback, unmarshaler protojson.UnmarshalOptions,
) error {
	switch evt.EventType {
	case "progress":
		// Payload: {"percent": N, "phase": "...", "message": "..."}
		var p struct {
			Percent int    `json:"percent"`
			Phase   string `json:"phase"`
			Message string `json:"message"`
		}
		if evt.PayloadJSON != "" {
			_ = json.Unmarshal([]byte(evt.PayloadJSON), &p)
		}
		callback.OnProgress(p.Percent, p.Phase, p.Message)

	case "partial":
		// Payload is a partial proto response JSON.
		if evt.PayloadJSON != "" {
			if err := unmarshaler.Unmarshal([]byte(evt.PayloadJSON), output); err == nil {
				callback.OnPartial(output, true)
			}
		}

	case "warning":
		var w struct {
			Message string `json:"message"`
			Context string `json:"context"`
		}
		if evt.PayloadJSON != "" {
			_ = json.Unmarshal([]byte(evt.PayloadJSON), &w)
		}
		callback.OnWarning(w.Message, w.Context)

	case "error":
		toolErr := fmt.Errorf("tool %s error: %s", toolName, evt.PayloadJSON)
		callback.OnError(toolErr, false)

	case "result":
		if evt.PayloadJSON != "" {
			if err := unmarshaler.Unmarshal([]byte(evt.PayloadJSON), output); err != nil {
				return fmt.Errorf("PlatformHarness.CallToolProtoStream: unmarshal result: %w", err)
			}
			callback.OnPartial(output, false)
		}
	}
	return nil
}

// ListTools returns descriptors for all available tools by querying the platform.
func (h *PlatformHarness) ListTools(ctx context.Context) ([]tool.Descriptor, error) {
	ctx, span := h.tracer.Start(ctx, "gibson.tool.list",
		trace.WithSpanKind(trace.SpanKindClient),
	)
	defer span.End()

	protos, err := h.client.ListTools(ctx, h.workID)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return nil, fmt.Errorf("PlatformHarness.ListTools: %w", err)
	}

	descs := make([]tool.Descriptor, len(protos))
	for i, p := range protos {
		descs[i] = tool.Descriptor{
			Name:              p.Name,
			Version:           p.Version,
			Description:       p.Description,
			Tags:              p.Tags,
			InputMessageType:  p.InputMessageType,
			OutputMessageType: p.OutputMessageType,
		}
	}
	return descs, nil
}

// QueueToolWork submits multiple proto inputs for parallel tool execution.
// Each input is marshaled to JSON individually; the platform assigns a job ID
// for retrieving results via ToolResults.
func (h *PlatformHarness) QueueToolWork(ctx context.Context, toolName string, inputs []protolib.Message) (string, error) {
	ctx, span := h.tracer.Start(ctx, "gibson.tool.queue",
		trace.WithSpanKind(trace.SpanKindClient),
		trace.WithAttributes(
			attribute.String("gibson.tool.name", toolName),
			attribute.Int("gibson.tool.input_count", len(inputs)),
		),
	)
	defer span.End()

	marshaler := protojson.MarshalOptions{UseProtoNames: true}
	inputsJSON := make([]string, len(inputs))
	for i, msg := range inputs {
		b, err := marshaler.Marshal(msg)
		if err != nil {
			span.RecordError(err)
			span.SetStatus(codes.Error, "failed to marshal proto input")
			return "", fmt.Errorf("PlatformHarness.QueueToolWork: marshal input[%d]: %w", i, err)
		}
		inputsJSON[i] = string(b)
	}

	jobID, err := h.client.QueueToolWork(ctx, h.workID, toolName, inputsJSON)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return "", fmt.Errorf("PlatformHarness.QueueToolWork: %w", err)
	}

	span.SetAttributes(attribute.String("gibson.tool.job_id", jobID))
	return jobID, nil
}

// ToolResults returns a channel that receives results as queued tool executions
// complete. Results may arrive out of order; use QueuedToolResult.Index to
// correlate with the original input positions. The channel is closed after all
// results have been received or on error.
func (h *PlatformHarness) ToolResults(ctx context.Context, jobID string) <-chan agent.QueuedToolResult {
	out := make(chan agent.QueuedToolResult, 32)

	rawCh, err := h.client.ToolResults(ctx, h.workID, jobID)
	if err != nil {
		go func() {
			out <- agent.QueuedToolResult{
				Index: 0,
				Error: fmt.Errorf("PlatformHarness.ToolResults: %w", err),
			}
			close(out)
		}()
		return out
	}

	go func() {
		defer close(out)
		for evt := range rawCh {
			if evt.Err != nil {
				out <- agent.QueuedToolResult{
					Index:      int(evt.Index),
					Error:      fmt.Errorf("PlatformHarness.ToolResults: %w", evt.Err),
					ToolCallID: evt.ToolCallID,
				}
				return
			}
			// Output is JSON-encoded; leave as nil — callers that need typed
			// output should use CallToolProto or CallToolProtoStream instead.
			// For QueuedToolResult the OutputJSON is stored in a no-proto path.
			// We deliver a nil Output with no error for a successful result so
			// callers can observe completion by index.
			_ = evt.OutputJSON // available if callers need raw JSON output
			out <- agent.QueuedToolResult{
				Index:      int(evt.Index),
				Output:     nil,
				Error:      nil,
				ToolCallID: evt.ToolCallID,
			}
		}
	}()

	return out
}

// QueryPlugin sends a query to a named plugin method. params is marshaled to
// JSON for transport; the result JSON is unmarshaled back to any.
func (h *PlatformHarness) QueryPlugin(ctx context.Context, name string, method string, params map[string]any) (any, error) {
	ctx, span := h.tracer.Start(ctx, "gibson.plugin.query",
		trace.WithSpanKind(trace.SpanKindClient),
		trace.WithAttributes(
			attribute.String("gibson.plugin.name", name),
			attribute.String("gibson.plugin.method", method),
		),
	)
	defer span.End()

	paramsJSON, err := json.Marshal(params)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "failed to marshal params to JSON")
		return nil, fmt.Errorf("PlatformHarness.QueryPlugin: marshal params: %w", err)
	}

	resultJSON, err := h.client.QueryPlugin(ctx, h.workID, name, method, string(paramsJSON))
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return nil, fmt.Errorf("PlatformHarness.QueryPlugin: %w", err)
	}

	var result any
	if err := json.Unmarshal([]byte(resultJSON), &result); err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "failed to unmarshal result JSON")
		return nil, fmt.Errorf("PlatformHarness.QueryPlugin: unmarshal result: %w", err)
	}

	return result, nil
}

// ListPlugins returns an empty slice — there is no ListPlugins RPC on
// ComponentService. Plugin discovery is not available in pull mode.
func (h *PlatformHarness) ListPlugins(_ context.Context) ([]plugin.Descriptor, error) {
	h.logger.Debug("PlatformHarness.ListPlugins: no ListPlugins RPC available — returning empty list")
	return []plugin.Descriptor{}, nil
}

// SubmitFinding serializes the finding and submits it to the platform via the
// SubmitFinding RPC tied to the current work item.
func (h *PlatformHarness) SubmitFinding(ctx context.Context, f *finding.Finding) error {
	ctx, span := h.tracer.Start(ctx, "gibson.finding.submit",
		trace.WithSpanKind(trace.SpanKindClient),
		trace.WithAttributes(
			attribute.String("gibson.finding.title", f.Title),
			attribute.String("gibson.finding.severity", string(f.Severity)),
		),
	)
	defer span.End()

	// Encode the finding as JSON for the platform wire format.
	payload, err := json.Marshal(f)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "failed to marshal finding to JSON")
		return fmt.Errorf("PlatformHarness.SubmitFinding: marshal: %w", err)
	}

	_, err = h.client.SubmitFinding(ctx, h.workID, payload)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return fmt.Errorf("PlatformHarness.SubmitFinding: %w", err)
	}

	return nil
}

// PlanContext returns nil. Planning context is not yet propagated through the
// work item's context fields in the component proto. When the daemon adds a
// dedicated "planning_context_json" key to PollWorkResponse.context, this
// method should parse and return it following the same pattern as
// missionExecutionContextFromWorkItem.
func (h *PlatformHarness) PlanContext() planning.PlanningContext {
	return nil
}

// ReportStepHints marshals the hints to JSON and reports them to the platform
// via the ReportStepHints RPC.
func (h *PlatformHarness) ReportStepHints(ctx context.Context, hints *planning.StepHints) error {
	if hints == nil {
		return nil
	}

	ctx, span := h.tracer.Start(ctx, "gibson.planning.report_hints",
		trace.WithSpanKind(trace.SpanKindClient),
	)
	defer span.End()

	// Serialize the exported hint fields to JSON.
	hintsPayload := struct {
		Confidence    float64  `json:"confidence"`
		SuggestedNext []string `json:"suggested_next,omitempty"`
		ReplanReason  string   `json:"replan_reason,omitempty"`
		KeyFindings   []string `json:"key_findings,omitempty"`
	}{
		Confidence:    hints.Confidence(),
		SuggestedNext: hints.SuggestedNext(),
		ReplanReason:  hints.ReplanReason(),
		KeyFindings:   hints.KeyFindings(),
	}

	hintsJSON, err := json.Marshal(hintsPayload)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "failed to marshal hints")
		return fmt.Errorf("PlatformHarness.ReportStepHints: marshal: %w", err)
	}

	if err := h.client.ReportStepHints(ctx, h.workID, hintsJSON); err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return fmt.Errorf("PlatformHarness.ReportStepHints: %w", err)
	}

	return nil
}

// SetMissionExecutionContext stores the mission execution context populated
// from the work item's "mission_execution_context_json" context key. This is
// called by the platform serve loop after constructing the harness.
func (h *PlatformHarness) SetMissionExecutionContext(ctx types.MissionExecutionContext) {
}

// CreateMission marshals the mission definition and opts to JSON and calls the platform's
// CreateMission RPC. The JSON result is unmarshaled to *mission.MissionInfo.
func (h *PlatformHarness) CreateMission(ctx context.Context, missionDef any, targetID string, opts *mission.CreateMissionOpts) (*mission.MissionInfo, error) {
	ctx, span := h.tracer.Start(ctx, "gibson.mission.create",
		trace.WithSpanKind(trace.SpanKindClient),
		trace.WithAttributes(
			attribute.String("gibson.mission.target_id", targetID),
		),
	)
	defer span.End()

	missionDefinitionJSON, err := json.Marshal(missionDef)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "failed to marshal mission definition")
		return nil, fmt.Errorf("PlatformHarness.CreateMission: marshal mission definition: %w", err)
	}

	var optsJSON []byte
	if opts != nil {
		optsJSON, err = json.Marshal(opts)
		if err != nil {
			span.RecordError(err)
			span.SetStatus(codes.Error, "failed to marshal opts")
			return nil, fmt.Errorf("PlatformHarness.CreateMission: marshal opts: %w", err)
		}
	}

	respJSON, err := h.client.CreateMission(ctx, h.workID, missionDefinitionJSON, targetID, optsJSON)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return nil, fmt.Errorf("PlatformHarness.CreateMission: %w", err)
	}

	var info mission.MissionInfo
	if err := json.Unmarshal(respJSON, &info); err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "failed to unmarshal mission info")
		return nil, fmt.Errorf("PlatformHarness.CreateMission: unmarshal: %w", err)
	}

	span.SetAttributes(attribute.String("gibson.mission.id", info.ID))
	return &info, nil
}

// RunMission queues a mission for execution via the platform's RunMission RPC.
func (h *PlatformHarness) RunMission(ctx context.Context, missionID string, opts *mission.RunMissionOpts) error {
	ctx, span := h.tracer.Start(ctx, "gibson.mission.run",
		trace.WithSpanKind(trace.SpanKindClient),
		trace.WithAttributes(
			attribute.String("gibson.mission.id", missionID),
		),
	)
	defer span.End()

	var (
		optsJSON []byte
		err      error
	)
	if opts != nil {
		optsJSON, err = json.Marshal(opts)
		if err != nil {
			span.RecordError(err)
			span.SetStatus(codes.Error, "failed to marshal opts")
			return fmt.Errorf("PlatformHarness.RunMission: marshal opts: %w", err)
		}
	}

	if err := h.client.RunMission(ctx, h.workID, missionID, optsJSON); err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return fmt.Errorf("PlatformHarness.RunMission: %w", err)
	}
	return nil
}

// GetMissionStatus retrieves the current status of a mission. The JSON response
// is unmarshaled to *mission.MissionStatusInfo.
func (h *PlatformHarness) GetMissionStatus(ctx context.Context, missionID string) (*mission.MissionStatusInfo, error) {
	ctx, span := h.tracer.Start(ctx, "gibson.mission.status",
		trace.WithSpanKind(trace.SpanKindClient),
		trace.WithAttributes(
			attribute.String("gibson.mission.id", missionID),
		),
	)
	defer span.End()

	respJSON, err := h.client.GetMissionStatus(ctx, h.workID, missionID)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return nil, fmt.Errorf("PlatformHarness.GetMissionStatus: %w", err)
	}

	var info mission.MissionStatusInfo
	if err := json.Unmarshal(respJSON, &info); err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "failed to unmarshal mission status")
		return nil, fmt.Errorf("PlatformHarness.GetMissionStatus: unmarshal: %w", err)
	}
	return &info, nil
}

// WaitForMission blocks until a mission completes or the timeout expires.
// The timeout duration is converted to milliseconds for the platform RPC.
func (h *PlatformHarness) WaitForMission(ctx context.Context, missionID string, timeout time.Duration) (*mission.MissionResult, error) {
	ctx, span := h.tracer.Start(ctx, "gibson.mission.wait",
		trace.WithSpanKind(trace.SpanKindClient),
		trace.WithAttributes(
			attribute.String("gibson.mission.id", missionID),
			attribute.Int64("gibson.mission.timeout_ms", timeout.Milliseconds()),
		),
	)
	defer span.End()

	respJSON, err := h.client.WaitMission(ctx, h.workID, missionID, timeout.Milliseconds())
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return nil, fmt.Errorf("PlatformHarness.WaitForMission: %w", err)
	}

	var result mission.MissionResult
	if err := json.Unmarshal(respJSON, &result); err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "failed to unmarshal mission result")
		return nil, fmt.Errorf("PlatformHarness.WaitForMission: unmarshal: %w", err)
	}
	return &result, nil
}

// ListMissions returns missions matching the provided filter. The filter is
// marshaled to JSON; the JSON response is unmarshaled to []*mission.MissionInfo.
func (h *PlatformHarness) ListMissions(ctx context.Context, filter *mission.MissionFilter) ([]*mission.MissionInfo, error) {
	ctx, span := h.tracer.Start(ctx, "gibson.mission.list",
		trace.WithSpanKind(trace.SpanKindClient),
	)
	defer span.End()

	var (
		filterJSON []byte
		err        error
	)
	if filter != nil {
		filterJSON, err = json.Marshal(filter)
		if err != nil {
			span.RecordError(err)
			span.SetStatus(codes.Error, "failed to marshal filter")
			return nil, fmt.Errorf("PlatformHarness.ListMissions: marshal filter: %w", err)
		}
	}

	respJSON, err := h.client.ListMissions(ctx, h.workID, filterJSON)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return nil, fmt.Errorf("PlatformHarness.ListMissions: %w", err)
	}

	var missions []*mission.MissionInfo
	if err := json.Unmarshal(respJSON, &missions); err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "failed to unmarshal missions")
		return nil, fmt.Errorf("PlatformHarness.ListMissions: unmarshal: %w", err)
	}
	return missions, nil
}

// CancelMission requests cancellation of a running mission via the platform.
func (h *PlatformHarness) CancelMission(ctx context.Context, missionID string) error {
	ctx, span := h.tracer.Start(ctx, "gibson.mission.cancel",
		trace.WithSpanKind(trace.SpanKindClient),
		trace.WithAttributes(
			attribute.String("gibson.mission.id", missionID),
		),
	)
	defer span.End()

	if err := h.client.CancelMission(ctx, h.workID, missionID); err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return fmt.Errorf("PlatformHarness.CancelMission: %w", err)
	}
	return nil
}

// GetMissionResults returns the final results of a completed mission. The JSON
// response is unmarshaled to *mission.MissionResult.
func (h *PlatformHarness) GetMissionResults(ctx context.Context, missionID string) (*mission.MissionResult, error) {
	ctx, span := h.tracer.Start(ctx, "gibson.mission.results",
		trace.WithSpanKind(trace.SpanKindClient),
		trace.WithAttributes(
			attribute.String("gibson.mission.id", missionID),
		),
	)
	defer span.End()

	respJSON, err := h.client.GetMissionResults(ctx, h.workID, missionID)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return nil, fmt.Errorf("PlatformHarness.GetMissionResults: %w", err)
	}

	var result mission.MissionResult
	if err := json.Unmarshal(respJSON, &result); err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "failed to unmarshal mission result")
		return nil, fmt.Errorf("PlatformHarness.GetMissionResults: unmarshal: %w", err)
	}
	return &result, nil
}

// Workspace returns nil — workspace support is not available in pull mode.
func (h *PlatformHarness) Workspace() workspace.Workspace {
	h.logger.Debug("PlatformHarness.Workspace: workspace not available in pull mode")
	return nil
}

// Workspaces returns an empty map — workspace support is not available in pull mode.
func (h *PlatformHarness) Workspaces() map[string]workspace.Workspace {
	h.logger.Debug("PlatformHarness.Workspaces: workspace not available in pull mode")
	return map[string]workspace.Workspace{}
}

// Authorize asks the daemon whether the work item of this harness may do action
// on resource, through ComponentService.Authorize. It means the same as
// CallbackHarness.Authorize and fails closed in the same way (D86): it returns
// nil only on an explicit allow.
//
//   - ErrInvalidAction: action or resource is empty, or the daemon refused it
//   - ErrUnauthorized: the daemon denied it, or the work item has no active run
//   - ErrAuthzServiceUnavailable: the daemon cannot answer, or has no RPC
//   - ErrNoRunContext: this harness has no work item
func (h *PlatformHarness) Authorize(ctx context.Context, action, resource string) error {
	if action == "" || resource == "" {
		return harnessconst.ErrInvalidAction
	}
	if h.workID == "" {
		h.logger.WarnContext(ctx, "Authorize: the harness has no work item; denying",
			"action", action, "resource", resource)
		return harnessconst.ErrNoRunContext
	}
	allowed, reason, err := h.client.Authorize(ctx, h.workID, action, resource)
	if err != nil {
		return authorizeError(ctx, h.logger, h.workID, action, resource, err)
	}
	if !allowed {
		h.logger.InfoContext(ctx, "authz_denied_local",
			"work_id", h.workID, "action", action, "resource", resource, "reason", reason)
		return harnessconst.ErrUnauthorized
	}
	return nil
}

// QueryNodes is not wired in platform pull-mode; it reports ErrKnowledgeUnavailable.
func (h *PlatformHarness) QueryNodes(context.Context, *graphragpb.GraphQuery) ([]*graphragpb.QueryResult, error) {
	return nil, fmt.Errorf("QueryNodes in platform pull-mode: %w", agent.ErrKnowledgeUnavailable)
}

// FindSimilarAttacks is not wired in platform pull-mode; it reports ErrKnowledgeUnavailable.
func (h *PlatformHarness) FindSimilarAttacks(context.Context, string, int) ([]*graphragpb.AttackPattern, error) {
	return nil, fmt.Errorf("FindSimilarAttacks in platform pull-mode: %w", agent.ErrKnowledgeUnavailable)
}

// GetAttackChains is not wired in platform pull-mode; it reports ErrKnowledgeUnavailable.
func (h *PlatformHarness) GetAttackChains(context.Context, string, int) ([]*graphragpb.AttackChain, error) {
	return nil, fmt.Errorf("GetAttackChains in platform pull-mode: %w", agent.ErrKnowledgeUnavailable)
}

// FindSimilarFindings is not wired in platform pull-mode; it reports ErrKnowledgeUnavailable.
func (h *PlatformHarness) FindSimilarFindings(context.Context, string, int) ([]*graphragpb.FindingNode, error) {
	return nil, fmt.Errorf("FindSimilarFindings in platform pull-mode: %w", agent.ErrKnowledgeUnavailable)
}

// GetRelatedFindings is not wired in platform pull-mode; it reports ErrKnowledgeUnavailable.
func (h *PlatformHarness) GetRelatedFindings(context.Context, string) ([]*graphragpb.FindingNode, error) {
	return nil, fmt.Errorf("GetRelatedFindings in platform pull-mode: %w", agent.ErrKnowledgeUnavailable)
}

// GetFindings is not wired in platform pull-mode; it reports ErrKnowledgeUnavailable.
func (h *PlatformHarness) GetFindings(context.Context, finding.Filter) ([]*finding.Finding, error) {
	return nil, fmt.Errorf("GetFindings in platform pull-mode: %w", agent.ErrKnowledgeUnavailable)
}

// GetRunFindings is not wired in platform pull-mode; it reports ErrKnowledgeUnavailable.
func (h *PlatformHarness) GetRunFindings(context.Context, agent.RunScope, finding.Filter) ([]*finding.Finding, error) {
	return nil, fmt.Errorf("GetRunFindings in platform pull-mode: %w", agent.ErrKnowledgeUnavailable)
}

// ApplicationFindings is not wired in platform pull-mode; it reports ErrKnowledgeUnavailable.
func (h *PlatformHarness) ApplicationFindings(context.Context, string, []string, int) ([]agent.ApplicationFinding, error) {
	return nil, fmt.Errorf("ApplicationFindings in platform pull-mode: %w", agent.ErrKnowledgeUnavailable)
}

// GetMissionRunHistory is not wired in platform pull-mode; it reports ErrKnowledgeUnavailable.
func (h *PlatformHarness) GetMissionRunHistory(context.Context) ([]types.MissionRunSummary, error) {
	return nil, fmt.Errorf("GetMissionRunHistory in platform pull-mode: %w", agent.ErrKnowledgeUnavailable)
}

// Delegate sends an A2A message to another agent through the Agent gateway and
// waits for the A2A Task to end. The platform resolves the route of the agent.
func (h *PlatformHarness) Delegate(
	ctx context.Context, name string, msg *a2a.Message, opts ...agent.DelegateOption,
) (*a2a.Task, error) {
	bankID := agent.ApplyDelegateOptions(opts).BankID
	return delegateVia(ctx, h.tracer, "PlatformHarness", name, func(ctx context.Context) (*resolvedAgent, error) {
		resp, err := h.client.ResolveAgent(ctx, h.workID, name, bankID)
		if err != nil {
			return nil, err
		}
		return &resolvedAgent{
			gatewayURL: resp.GetGatewayUrl(), routeHeaders: resp.GetRouteHeaders(), gatewayCAPEM: resp.GetGatewayCaPem(),
		}, nil
	}, grantHeaders(h.grant.get), msg)
}
