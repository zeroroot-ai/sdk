// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Zero Root AI

package serve

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"time"

	graphragpb "github.com/zeroroot-ai/sdk/api/gen/gibson/graphrag/v1"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
	"google.golang.org/protobuf/encoding/protojson"
	protolib "google.golang.org/protobuf/proto"

	"github.com/zeroroot-ai/sdk/agent"
	componentpb "github.com/zeroroot-ai/sdk/api/gen/gibson/component/v1"
	"github.com/zeroroot-ai/sdk/codegen/workspace"
	"github.com/zeroroot-ai/sdk/finding"
	"github.com/zeroroot-ai/sdk/llm"
	"github.com/zeroroot-ai/sdk/mission"
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
	workID  string // identifies the active work item for platform routing/billing
	mission types.MissionContext
	target  types.TargetInfo
	logger  *slog.Logger
	tracer  trace.Tracer

	// tokenTracker accumulates token usage from Complete/CompleteWithTools calls.
	tokenTracker *CallbackTokenTracker

	// missionExecCtx holds run-history metadata populated from the work item context.
	missionExecCtx types.MissionExecutionContext
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
	return h
}

// ============================================================================
// Observability
// ============================================================================

// Logger returns the structured logger for the agent.
func (h *PlatformHarness) Logger() *slog.Logger { return h.logger }

// Tracer returns the OpenTelemetry tracer for distributed tracing.
func (h *PlatformHarness) Tracer() trace.Tracer { return h.tracer }

// TokenUsage returns the accumulated token usage across all LLM slots.
func (h *PlatformHarness) TokenUsage() llm.TokenTracker { return h.tokenTracker }

// ============================================================================
// Context Access
// ============================================================================

// Mission returns the current mission context.
func (h *PlatformHarness) Mission() types.MissionContext { return h.mission }

// Target returns information about the target being tested.
func (h *PlatformHarness) Target() types.TargetInfo { return h.target }

// ============================================================================
// LLM Access
// ============================================================================

// Complete performs a non-streaming LLM completion through the platform.
// The slot selects which named LLM slot to use (e.g. "primary").
func (h *PlatformHarness) Complete(ctx context.Context, slot string, messages []llm.Message, opts ...llm.CompletionOption) (*llm.CompletionResponse, error) {
	ctx, span := h.tracer.Start(ctx, "gen_ai.chat",
		trace.WithSpanKind(trace.SpanKindClient),
		trace.WithAttributes(
			attribute.String("gen_ai.request.model", slot),
			attribute.Int("gen_ai.request.message_count", len(messages)),
		),
	)
	defer span.End()

	// Convert messages to the platform wire format.
	platMsgs := make([]LLMMessage, len(messages))
	for i, m := range messages {
		platMsgs[i] = LLMMessage{
			Role:    string(m.Role),
			Content: m.Content,
		}
	}

	resp, err := h.client.Complete(ctx, h.workID, slot, platMsgs)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return nil, fmt.Errorf("PlatformHarness.Complete: %w", err)
	}

	result := &llm.CompletionResponse{}
	if resp.Response != nil {
		result.Content = resp.Response.Content
	}
	if resp.Usage != nil {
		result.Usage = llm.TokenUsage{
			InputTokens:  int(resp.Usage.InputTokens),
			OutputTokens: int(resp.Usage.OutputTokens),
			TotalTokens:  int(resp.Usage.InputTokens) + int(resp.Usage.OutputTokens),
		}
		h.tokenTracker.Add(slot, result.Usage)
	}

	span.SetAttributes(
		attribute.Int("gen_ai.usage.input_tokens", result.Usage.InputTokens),
		attribute.Int("gen_ai.usage.output_tokens", result.Usage.OutputTokens),
	)

	return result, nil
}

// CompleteWithTools performs a completion with tool definitions supplied to the
// LLM. Tools are converted to componentpb.ToolDefinition wire format and sent
// to the platform for forwarding to the LLM. ToolCalls in the response are
// converted back to llm.ToolCall.
func (h *PlatformHarness) CompleteWithTools(ctx context.Context, slot string, messages []llm.Message, tools []llm.ToolDef) (*llm.CompletionResponse, error) {
	ctx, span := h.tracer.Start(ctx, "gen_ai.chat.tools",
		trace.WithSpanKind(trace.SpanKindClient),
		trace.WithAttributes(
			attribute.String("gen_ai.request.model", slot),
			attribute.Int("gen_ai.request.message_count", len(messages)),
			attribute.Int("gen_ai.request.tool_count", len(tools)),
		),
	)
	defer span.End()

	// Convert messages to the wire format.
	platMsgs := make([]LLMMessage, len(messages))
	for i, m := range messages {
		platMsgs[i] = LLMMessage{Role: string(m.Role), Content: m.Content}
	}

	// Convert llm.ToolDef slice to componentpb.ToolDefinition slice.
	protoDefs := make([]*componentpb.ToolDefinition, 0, len(tools))
	for _, td := range tools {
		schemaJSON, err := json.Marshal(td.Parameters)
		if err != nil {
			span.RecordError(err)
			span.SetStatus(codes.Error, "failed to marshal tool parameters")
			return nil, fmt.Errorf("PlatformHarness.CompleteWithTools: marshal tool %q parameters: %w", td.Name, err)
		}
		protoDefs = append(protoDefs, &componentpb.ToolDefinition{
			Name:            td.Name,
			Description:     td.Description,
			InputSchemaJson: string(schemaJSON),
		})
	}

	resp, err := h.client.CompleteWithTools(ctx, h.workID, slot, platMsgs, protoDefs)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return nil, fmt.Errorf("PlatformHarness.CompleteWithTools: %w", err)
	}

	result := &llm.CompletionResponse{
		FinishReason: resp.FinishReason,
	}
	if resp.Response != nil {
		result.Content = resp.Response.Content
	}
	if resp.Usage != nil {
		result.Usage = llm.TokenUsage{
			InputTokens:  int(resp.Usage.InputTokens),
			OutputTokens: int(resp.Usage.OutputTokens),
			TotalTokens:  int(resp.Usage.InputTokens) + int(resp.Usage.OutputTokens),
		}
		h.tokenTracker.Add(slot, result.Usage)
	}

	// Convert proto ToolCallResult slice to llm.ToolCall slice.
	if len(resp.ToolCalls) > 0 {
		result.ToolCalls = make([]llm.ToolCall, len(resp.ToolCalls))
		for i, tc := range resp.ToolCalls {
			result.ToolCalls[i] = llm.ToolCall{
				ID:        tc.Id,
				Name:      tc.Name,
				Arguments: tc.ArgumentsJson,
			}
		}
	}

	span.SetAttributes(
		attribute.Int("gen_ai.usage.input_tokens", result.Usage.InputTokens),
		attribute.Int("gen_ai.usage.output_tokens", result.Usage.OutputTokens),
		attribute.Int("gen_ai.response.tool_call_count", len(result.ToolCalls)),
	)

	return result, nil
}

// Stream performs a streaming completion request, proxying through the platform's
// CompleteStream RPC. Returns a channel of llm.StreamChunk that is closed when
// the stream completes or encounters an error.
func (h *PlatformHarness) Stream(ctx context.Context, slot string, messages []llm.Message) (<-chan llm.StreamChunk, error) {
	ctx, span := h.tracer.Start(ctx, "gen_ai.chat.stream",
		trace.WithSpanKind(trace.SpanKindClient),
		trace.WithAttributes(
			attribute.String("gen_ai.request.model", slot),
			attribute.Int("gen_ai.request.message_count", len(messages)),
		),
	)

	platMsgs := make([]LLMMessage, len(messages))
	for i, m := range messages {
		platMsgs[i] = LLMMessage{Role: string(m.Role), Content: m.Content}
	}

	rawCh, err := h.client.StreamCompletion(ctx, h.workID, slot, platMsgs)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		span.End()
		return nil, fmt.Errorf("PlatformHarness.Stream: %w", err)
	}

	out := make(chan llm.StreamChunk, 32)
	go func() {
		defer span.End()
		defer close(out)
		for raw := range rawCh {
			if raw.Err != nil {
				span.RecordError(raw.Err)
				span.SetStatus(codes.Error, raw.Err.Error())
				out <- llm.StreamChunk{FinishReason: "error"}
				return
			}
			chunk := llm.StreamChunk{
				Delta: raw.Content,
			}
			if raw.Done {
				chunk.FinishReason = "stop"
				if raw.Usage != nil {
					chunk.Usage = &llm.TokenUsage{
						InputTokens:  int(raw.Usage.InputTokens),
						OutputTokens: int(raw.Usage.OutputTokens),
						TotalTokens:  int(raw.Usage.InputTokens) + int(raw.Usage.OutputTokens),
					}
					h.tokenTracker.Add(slot, *chunk.Usage)
				}
			}
			out <- chunk
		}
	}()

	return out, nil
}

// CompleteStructured performs a completion requesting structured JSON output.
// schema is marshaled to a JSON Schema string which the platform forwards to
// the LLM. The returned JSON string is unmarshaled back to an any value.
func (h *PlatformHarness) CompleteStructured(ctx context.Context, slot string, messages []llm.Message, schema any) (any, error) {
	ctx, span := h.tracer.Start(ctx, "gen_ai.chat.structured",
		trace.WithSpanKind(trace.SpanKindClient),
		trace.WithAttributes(
			attribute.String("gen_ai.request.model", slot),
			attribute.Int("gen_ai.request.message_count", len(messages)),
		),
	)
	defer span.End()

	schemaJSON, err := json.Marshal(schema)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "failed to marshal schema")
		return nil, fmt.Errorf("PlatformHarness.CompleteStructured: marshal schema: %w", err)
	}

	platMsgs := make([]LLMMessage, len(messages))
	for i, m := range messages {
		platMsgs[i] = LLMMessage{Role: string(m.Role), Content: m.Content}
	}

	resultJSON, err := h.client.CompleteStructured(ctx, h.workID, slot, platMsgs, string(schemaJSON))
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return nil, fmt.Errorf("PlatformHarness.CompleteStructured: %w", err)
	}

	var result any
	if err := json.Unmarshal([]byte(resultJSON), &result); err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "failed to unmarshal structured result")
		return nil, fmt.Errorf("PlatformHarness.CompleteStructured: unmarshal result: %w", err)
	}

	return result, nil
}

// CompleteStructuredAny is an alias for CompleteStructured.
func (h *PlatformHarness) CompleteStructuredAny(ctx context.Context, slot string, messages []llm.Message, schema any) (any, error) {
	return h.CompleteStructured(ctx, slot, messages, schema)
}

// ============================================================================
// Tool Access
// ============================================================================

// CallToolProto invokes a tool by name, marshaling the request proto to JSON
// for transport and unmarshaling the JSON response back into resp.
// Uses protojson with UseProtoNames=true for snake_case consistency with the
// existing tool serve code.
func (h *PlatformHarness) CallToolProto(ctx context.Context, name string, request protolib.Message, response protolib.Message) error {
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

	outputJSON, err := h.client.CallTool(ctx, h.workID, name, string(inputJSON))
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
func (h *PlatformHarness) CallToolProtoStream(ctx context.Context, toolName string, input protolib.Message, output protolib.Message, callback agent.ToolStreamCallback) error {
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
					span.RecordError(err)
					span.SetStatus(codes.Error, "failed to unmarshal result")
					return fmt.Errorf("PlatformHarness.CallToolProtoStream: unmarshal result: %w", err)
				}
				callback.OnPartial(output, false)
			}
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
					Index: int(evt.Index),
					Error: fmt.Errorf("PlatformHarness.ToolResults: %w", evt.Err),
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
				Index:  int(evt.Index),
				Output: nil,
				Error:  nil,
			}
		}
	}()

	return out
}

// ============================================================================
// Plugin Access
// ============================================================================

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

// ============================================================================
// Agent Delegation
// ============================================================================

// DelegateToAgent dispatches a task to another agent identified by name. The
// task is marshaled to JSON and sent via the platform's DelegateToAgent RPC;
// the JSON result is unmarshaled back to agent.Result.
func (h *PlatformHarness) DelegateToAgent(ctx context.Context, name string, task agent.Task) (agent.Result, error) {
	ctx, span := h.tracer.Start(ctx, "gibson.agent.delegate",
		trace.WithSpanKind(trace.SpanKindClient),
		trace.WithAttributes(
			attribute.String("gibson.agent.name", name),
			attribute.String("gibson.task.id", task.ID),
		),
	)
	defer span.End()

	taskJSON, err := json.Marshal(task)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "failed to marshal task to JSON")
		return agent.Result{}, fmt.Errorf("PlatformHarness.DelegateToAgent: marshal task: %w", err)
	}

	resultBytes, err := h.client.DelegateToAgent(ctx, h.workID, name, taskJSON)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return agent.Result{}, fmt.Errorf("PlatformHarness.DelegateToAgent: %w", err)
	}

	var result agent.Result
	if err := json.Unmarshal(resultBytes, &result); err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "failed to unmarshal agent result")
		return agent.Result{}, fmt.Errorf("PlatformHarness.DelegateToAgent: unmarshal result: %w", err)
	}

	return result, nil
}

// ListAgents returns descriptors for all available agents by querying the platform.
func (h *PlatformHarness) ListAgents(ctx context.Context) ([]agent.Descriptor, error) {
	ctx, span := h.tracer.Start(ctx, "gibson.agent.list",
		trace.WithSpanKind(trace.SpanKindClient),
	)
	defer span.End()

	protos, err := h.client.ListAgents(ctx, h.workID)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return nil, fmt.Errorf("PlatformHarness.ListAgents: %w", err)
	}

	descs := make([]agent.Descriptor, len(protos))
	for i, p := range protos {
		descs[i] = agent.Descriptor{
			Name:         p.Name,
			Version:      p.Version,
			Description:  p.Description,
			Capabilities: p.Capabilities,
			TargetTypes:  p.TargetTypes,
		}
	}
	return descs, nil
}

// ============================================================================
// Finding Management
// ============================================================================

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

// ============================================================================
// Memory Access
// ============================================================================

// ============================================================================
// Planning
// ============================================================================

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

// ============================================================================
// Mission Execution Context
// ============================================================================

// SetMissionExecutionContext stores the mission execution context populated
// from the work item's "mission_execution_context_json" context key. This is
// called by the platform serve loop after constructing the harness.
func (h *PlatformHarness) SetMissionExecutionContext(ctx types.MissionExecutionContext) {
	h.missionExecCtx = ctx
}

// ============================================================================
// MissionManager — proxied through PlatformClient
// ============================================================================

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

// ============================================================================
// Workspace Access
// ============================================================================

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

// ============================================================================
// Authorization Methods
// ============================================================================

// Authorize is a no-op in PlatformHarness. Platform-mode components execute
// via the daemon's platform pull-mode path which has its own pre-dispatch FGA
// check. A direct harness callback channel for per-operation checks is not
// yet wired in platform mode; this returns nil (allow) unconditionally.
//
// When the platform pull-mode path gains a callback channel, this method
// will be updated to forward to the daemon's Authorize RPC.
func (h *PlatformHarness) Authorize(_ context.Context, _, _ string) error {
	return nil
}

// ── KnowledgeReader ─────────────────────────────────────────────────────────
//
// Not wired in platform pull-mode, and each read says so rather than answering
// empty. Same reasoning as WorldView above: an empty result reads as "the tenant
// knows nothing", which a caller cannot distinguish from "I could not look" —
// and for a security agent that is a silent false negative.
//
// ErrKnowledgeUnavailable is matchable, so a caller can degrade deliberately:
//
//	if errors.Is(err, agent.ErrKnowledgeUnavailable) { /* say so, conclude nothing */ }
//
// These reach the daemon over ComponentService, which does carry all eight RPCs
// — what is missing is the client half plus a decode, since that service still
// returns the JSON-bytes payloads the callback service has now moved off. The
// dispatched path (CallbackHarness) is the one this epic needed, and it is wired.

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
