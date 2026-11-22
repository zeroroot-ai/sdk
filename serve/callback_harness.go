// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Zero Root AI

package serve

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/a2aproject/a2a-go/v2/a2a"
	"io"
	"log/slog"
	"math"
	"sync"
	"time"

	"buf.build/go/protovalidate"
	"github.com/google/uuid"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
	grpccodes "google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/encoding/protojson"
	protolib "google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/reflect/protoregistry"

	"github.com/zeroroot-ai/sdk/agent"
	graphragpb "github.com/zeroroot-ai/sdk/api/gen/gibson/graphrag/v1"
	harnesspb "github.com/zeroroot-ai/sdk/api/gen/gibson/harness/v1"
	typespb "github.com/zeroroot-ai/sdk/api/gen/gibson/types/v1"
	"github.com/zeroroot-ai/sdk/codegen/workspace"
	"github.com/zeroroot-ai/sdk/finding"
	"github.com/zeroroot-ai/sdk/fork"
	harnessconst "github.com/zeroroot-ai/sdk/harness"
	"github.com/zeroroot-ai/sdk/llm"
	"github.com/zeroroot-ai/sdk/mission"
	"github.com/zeroroot-ai/sdk/openresponses"
	"github.com/zeroroot-ai/sdk/planning"
	"github.com/zeroroot-ai/sdk/plugin"
	"github.com/zeroroot-ai/sdk/tool"
	"github.com/zeroroot-ai/sdk/types"
)

// CallbackHarness implements agent.Harness by forwarding all operations
// to the orchestrator via gRPC callbacks. This allows agents running in
// standalone mode to access the full harness functionality.
type CallbackHarness struct {
	// Core dependencies
	client       *CallbackClient
	tokenTracker llm.TokenTracker

	// forkWatcher holds the sandbox id at harness start, before a fork can
	// happen. A fork copies it, so it tells a fork from its parent (D74). It is
	// nil when the hostname cannot be read.
	forkWatcher *fork.Watcher

	// Context
	logger      *slog.Logger
	tracer      trace.Tracer
	mission     types.MissionContext
	target      types.TargetInfo
	planContext planning.PlanningContext

	// Taxonomy support
	taxonomy         *TaxonomyAdapter
	taxonomyInitOnce sync.Once

	// Caching for list operations
	cacheMu      sync.RWMutex
	toolsCache   []tool.Descriptor
	pluginsCache []plugin.Descriptor
}

// NewCallbackHarness creates a new callback-based harness.
//
// It accepts functional options (WithCallbackLogger, WithCallbackTracer,
// WithCallbackMission, WithCallbackTarget) for configuration. When no options
// are supplied the harness uses slog.Default() as the logger and a no-op OTel
// tracer.
//
// The harness automatically fetches the taxonomy from the orchestrator at
// startup. If taxonomy fetch fails, the harness will still function but without
// taxonomy support.
func NewCallbackHarness(client *CallbackClient, opts ...CallbackHarnessOption) *CallbackHarness {
	h := &CallbackHarness{
		client:       client,
		logger:       slog.Default(),
		tracer:       defaultNoopTracer(),
		tokenTracker: NewCallbackTokenTracker(),
	}

	for _, opt := range opts {
		opt(h)
	}

	w, err := fork.NewWatcher()
	if err != nil {
		h.logger.Warn("cannot read the sandbox id; a mission that forks this process is refused", "error", err)
	}
	h.forkWatcher = w

	// Fetch taxonomy at startup (non-blocking, with graceful degradation)
	h.initTaxonomy(context.Background())

	return h
}

// initTaxonomy fetches the taxonomy from the orchestrator and sets it globally.
// This is called automatically at startup. If fetch fails, the harness will
// continue to work but without full taxonomy support.
func (h *CallbackHarness) initTaxonomy(ctx context.Context) {
	h.taxonomyInitOnce.Do(func() {
		// Create timeout context for taxonomy fetch
		fetchCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
		defer cancel()

		req := &harnesspb.GetTaxonomySchemaRequest{}
		resp, err := h.client.GetTaxonomySchema(fetchCtx, req)
		if err != nil {
			h.logger.Warn("failed to fetch taxonomy from orchestrator - continuing without taxonomy support",
				"error", err)
			return
		}

		if resp.Error != nil {
			h.logger.Warn("orchestrator returned error fetching taxonomy - continuing without taxonomy support",
				"error", resp.Error.Message)
			return
		}

		// Create adapter from proto response
		h.taxonomy = NewTaxonomyAdapter(resp)

		h.logger.Info("taxonomy initialized successfully",
			"version", h.taxonomy.Version(),
			"node_types", len(h.taxonomy.NodeTypes()),
			"relationship_types", len(h.taxonomy.RelationshipTypes()),
			"techniques", len(h.taxonomy.TechniqueIDs("")))
	})
}

// Logger returns the structured logger for the agent.
func (h *CallbackHarness) Logger() *slog.Logger {
	return h.logger
}

// Tracer returns the OpenTelemetry tracer for distributed tracing.
func (h *CallbackHarness) Tracer() trace.Tracer {
	return h.tracer
}

// TokenUsage returns the token usage tracker for this execution.
func (h *CallbackHarness) TokenUsage() llm.TokenTracker {
	return h.tokenTracker
}

// Mission returns the current mission context.
func (h *CallbackHarness) Mission() types.MissionContext {
	return h.mission
}

// Target returns information about the target being tested.
func (h *CallbackHarness) Target() types.TargetInfo {
	return h.target
}

// Model resolves a slot to one exact model through the orchestrator and returns
// the client that calls the model gateway. The client sends the task grant of
// the agent. It adds token counts to TokenUsage.
func (h *CallbackHarness) Model(ctx context.Context, slot string) (*openresponses.Client, error) {
	resp, err := h.client.ResolveModel(ctx, slot)
	if err != nil {
		return nil, fmt.Errorf("CallbackHarness.Model: %w", err)
	}
	return newModelClient(slot, &resolvedModel{
		model: resp.GetModel(), provider: resp.GetProvider(), gatewayURL: resp.GetGatewayUrl(),
		routeHeaders: resp.GetRouteHeaders(), contextWindow: int(resp.GetContextWindow()),
		maxOutputTokens: int(resp.GetMaxOutputTokens()), gatewayCAPEM: resp.GetGatewayCaPem(),
	}, h.client.grantHeaders(), h.tokenTracker)
}

// CallToolProto invokes a tool using proto messages via the CallToolProto RPC.
// This is the canonical way to execute tools - all tool calls should use this method.
func (h *CallbackHarness) CallToolProto(
	ctx context.Context, name string, request protolib.Message, response protolib.Message, opts ...agent.ToolCallOption,
) error {
	// Start span for proto tool call
	ctx, span := h.tracer.Start(ctx, "gen_ai.tool.proto",
		trace.WithSpanKind(trace.SpanKindClient),
		trace.WithAttributes(
			attribute.String("gibson.tool.name", name),
			attribute.String("gibson.tool.request_type", string(request.ProtoReflect().Descriptor().FullName())),
		),
	)
	defer span.End()

	// Use protojson marshaler with snake_case field names to match tool schemas
	marshaler := protojson.MarshalOptions{
		UseProtoNames: true, // Use snake_case (proto field names) instead of camelCase
	}

	// Serialize proto request to JSON
	requestJSON, err := marshaler.Marshal(request)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "failed to marshal proto request to JSON")
		return fmt.Errorf("failed to marshal proto request to JSON: %w", err)
	}

	// Create the proto request
	protoReq := &harnesspb.CallToolProtoRequest{
		Name:       name,
		InputJson:  requestJSON,
		InputType:  string(request.ProtoReflect().Descriptor().FullName()),
		OutputType: string(response.ProtoReflect().Descriptor().FullName()),
	}

	// Call via the callback client
	resp, err := h.client.CallToolProto(ctx, protoReq)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return fmt.Errorf("CallToolProto callback failed: %w", err)
	}
	// The daemon records a failed call too, so the id is set before the error.
	agent.ApplyToolCallOptions(opts).SetToolCallID(resp.GetToolCallId())

	if resp.Error != nil {
		err := &agent.ToolCallError{Code: resp.Error.Code.String(), Message: resp.Error.Message}
		span.RecordError(err)
		span.SetStatus(codes.Error, resp.Error.Message)
		return err
	}

	// Use protojson unmarshaler for proper proto field mapping
	unmarshaler := protojson.UnmarshalOptions{
		DiscardUnknown: true, // Ignore fields not in proto (tools may return extra data)
	}

	// Unmarshal JSON into proto response
	if err := unmarshaler.Unmarshal(resp.OutputJson, response); err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "failed to unmarshal into proto response")
		return fmt.Errorf("failed to unmarshal tool output into proto response: %w", err)
	}

	return nil
}

// CallToolProtoStream invokes a tool with streaming event callbacks.
// This enables real-time progress updates, partial results, and warnings during tool execution.
//
// The method marshals the input proto to JSON, calls the daemon's CallToolProtoStream RPC,
// receives streaming ToolMessage events, dispatches them to the callback methods, and
// returns the final output.
func (h *CallbackHarness) CallToolProtoStream(
	ctx context.Context, toolName string, input protolib.Message, output protolib.Message,
	callback agent.ToolStreamCallback, opts ...agent.ToolCallOption,
) error {
	toolCallOpts := agent.ApplyToolCallOptions(opts)
	var toolCallID string
	defer func() { toolCallOpts.SetToolCallID(toolCallID) }()

	// Start span for streaming proto tool call
	ctx, span := h.tracer.Start(ctx, "gen_ai.tool.proto.stream",
		trace.WithSpanKind(trace.SpanKindClient),
		trace.WithAttributes(
			attribute.String("gibson.tool.name", toolName),
			attribute.String("gibson.tool.request_type", string(input.ProtoReflect().Descriptor().FullName())),
		),
	)
	defer span.End()

	// Use protojson marshaler with snake_case field names to match tool schemas
	marshaler := protojson.MarshalOptions{
		UseProtoNames: true, // Use snake_case (proto field names) instead of camelCase
	}

	// Serialize proto request to JSON
	inputJSON, err := marshaler.Marshal(input)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "failed to marshal proto input to JSON")
		return fmt.Errorf("failed to marshal proto input to JSON: %w", err)
	}
	// Build the streaming RPC request.
	protoReq := &harnesspb.CallToolProtoStreamRequest{
		Name:       toolName,
		InputJson:  inputJSON,
		InputType:  string(input.ProtoReflect().Descriptor().FullName()),
		OutputType: string(output.ProtoReflect().Descriptor().FullName()),
	}

	stream, err := h.client.CallToolProtoStream(ctx, protoReq)
	if err != nil {
		// codes.Unimplemented means the daemon doesn't support streaming yet — return typed error.
		if st, ok := status.FromError(err); ok && st.Code() == grpccodes.Unimplemented {
			span.SetStatus(codes.Error, "tool streaming not supported by daemon")
			return fmt.Errorf("tool streaming not supported by daemon: %w", err)
		}
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return fmt.Errorf("CallToolProtoStream RPC failed: %w", err)
	}

	unmarshaler := protojson.UnmarshalOptions{DiscardUnknown: true}

	for {
		msg, err := stream.Recv()
		if err != nil {
			if errors.Is(err, io.EOF) {
				break
			}
			if st, ok := status.FromError(err); ok && st.Code() == grpccodes.Canceled {
				break
			}
			span.RecordError(err)
			callback.OnError(err, true)
			return fmt.Errorf("CallToolProtoStream receive error: %w", err)
		}
		if id := msg.GetToolCallId(); id != "" {
			toolCallID = id
		}

		if err := dispatchToolStreamMessage(msg, output, callback, unmarshaler); err != nil {
			span.RecordError(err)
			span.SetStatus(codes.Error, err.Error())
			return err
		}
	}

	span.SetStatus(codes.Ok, "")
	return nil
}

// dispatchToolStreamMessage sends one message of a tool stream to the callback.
// It returns an error when the message ends the call: a fatal error event, or a
// complete event whose output cannot be read.
func dispatchToolStreamMessage(
	msg *harnesspb.CallToolProtoStreamResponse, output protolib.Message,
	callback agent.ToolStreamCallback, unmarshaler protojson.UnmarshalOptions,
) error {
	switch payload := msg.GetPayload().(type) {
	case *harnesspb.CallToolProtoStreamResponse_Progress:
		ev := payload.Progress
		callback.OnProgress(int(ev.GetPercent()), ev.GetStage(), ev.GetMessage())

	case *harnesspb.CallToolProtoStreamResponse_Partial:
		ev := payload.Partial
		if len(ev.GetOutputJson()) > 0 {
			if err := unmarshaler.Unmarshal(ev.GetOutputJson(), output); err == nil {
				callback.OnPartial(output, false)
			}
		}

	case *harnesspb.CallToolProtoStreamResponse_Warning:
		ev := payload.Warning
		callback.OnWarning(ev.GetMessage(), ev.GetCode())

	case *harnesspb.CallToolProtoStreamResponse_Error:
		ev := payload.Error
		callbackErr := &agent.ToolCallError{
			Code: ev.GetError().GetCode().String(), Message: ev.GetError().GetMessage(),
		}
		callback.OnError(callbackErr, ev.GetFatal())
		if ev.GetFatal() {
			return callbackErr
		}

	case *harnesspb.CallToolProtoStreamResponse_Complete:
		ev := payload.Complete
		if len(ev.GetOutputJson()) > 0 {
			if err := unmarshaler.Unmarshal(ev.GetOutputJson(), output); err != nil {
				return fmt.Errorf("failed to unmarshal streaming tool output: %w", err)
			}
		}
	}
	return nil
}

// ListTools returns descriptors for all available tools.
// Results are cached per task execution.
func (h *CallbackHarness) ListTools(ctx context.Context) ([]tool.Descriptor, error) {
	// Check cache first
	h.cacheMu.RLock()
	if h.toolsCache != nil {
		defer h.cacheMu.RUnlock()
		return h.toolsCache, nil
	}
	h.cacheMu.RUnlock()

	// Fetch from orchestrator
	protoReq := &harnesspb.ListToolsRequest{}
	resp, err := h.client.ListTools(ctx, protoReq)
	if err != nil {
		return nil, fmt.Errorf("list tools callback failed: %w", err)
	}

	if resp.Error != nil {
		return nil, fmt.Errorf("list tools error: %s", resp.Error.Message)
	}

	// Convert to tool.Descriptor
	tools := make([]tool.Descriptor, len(resp.Tools))
	for i, protoTool := range resp.Tools {
		tools[i] = tool.Descriptor{
			Name:        protoTool.Name,
			Description: protoTool.Description,
			Version:     "unknown", // Proto doesn't include version yet
			// TODO(v1.0): Update proto to include InputMessageType and OutputMessageType
			// InputMessageType:  protoTool.InputMessageType,
			// OutputMessageType: protoTool.OutputMessageType,
		}
	}

	// Cache results
	h.cacheMu.Lock()
	h.toolsCache = tools
	h.cacheMu.Unlock()

	return tools, nil
}

// QueryPlugin sends a query to a plugin and returns the result.
func (h *CallbackHarness) QueryPlugin(ctx context.Context, name string, method string, params map[string]any) (any, error) {
	protoReq := &harnesspb.QueryPluginRequest{
		Name:   name,
		Method: method,
		Params: ToTypedMap(params),
	}

	resp, err := h.client.QueryPlugin(ctx, protoReq)
	if err != nil {
		return nil, fmt.Errorf("query plugin callback failed: %w", err)
	}

	if resp.Error != nil {
		return nil, fmt.Errorf("query plugin error: %s", resp.Error.Message)
	}

	// Convert result TypedValue to any
	return FromTypedValue(resp.Result), nil
}

// ListPlugins returns descriptors for all available plugins.
// Results are cached per task execution.
func (h *CallbackHarness) ListPlugins(ctx context.Context) ([]plugin.Descriptor, error) {
	// Check cache first
	h.cacheMu.RLock()
	if h.pluginsCache != nil {
		defer h.cacheMu.RUnlock()
		return h.pluginsCache, nil
	}
	h.cacheMu.RUnlock()

	// Fetch from orchestrator
	protoReq := &harnesspb.ListPluginsRequest{}
	resp, err := h.client.ListPlugins(ctx, protoReq)
	if err != nil {
		return nil, fmt.Errorf("list plugins callback failed: %w", err)
	}

	if resp.Error != nil {
		return nil, fmt.Errorf("list plugins error: %s", resp.Error.Message)
	}

	// Convert to plugin.Descriptor
	// Note: This conversion is simplified - full MethodDescriptors require proto update
	plugins := make([]plugin.Descriptor, len(resp.Plugins))
	for i, protoPlugin := range resp.Plugins {
		// Convert method names to MethodDescriptors (minimal for now)
		methods := make([]plugin.MethodDescriptor, len(protoPlugin.Methods))
		for j, methodName := range protoPlugin.Methods {
			methods[j] = plugin.MethodDescriptor{
				Name: methodName,
				// TODO(v1.0): Add Description, InputSchema, OutputSchema once proto is updated
			}
		}

		plugins[i] = plugin.Descriptor{
			Name:        protoPlugin.Name,
			Version:     protoPlugin.Version,
			Description: protoPlugin.Description,
			Methods:     methods,
		}
	}

	// Cache results
	h.cacheMu.Lock()
	h.pluginsCache = plugins
	h.cacheMu.Unlock()

	return plugins, nil
}

// SubmitFinding records a new security finding.
func (h *CallbackHarness) SubmitFinding(ctx context.Context, f *finding.Finding) error {
	// Start span for finding submission
	ctx, span := h.tracer.Start(ctx, "gibson.finding.submit",
		trace.WithSpanKind(trace.SpanKindClient),
		trace.WithAttributes(
			attribute.String("gibson.finding.title", f.Title),
			attribute.String("gibson.finding.severity", string(f.Severity)),
		),
	)
	defer span.End()

	// Convert finding to proto
	protoReq := &harnesspb.SubmitFindingRequest{
		IdempotencyKey: uuid.NewString(),
		Finding:        FindingToProto(f),
		ToolCallIds:    f.ToolCallIDs,
	}

	resp, err := h.client.SubmitFinding(ctx, protoReq)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return fmt.Errorf("submit finding callback failed: %w", err)
	}

	if resp.Error != nil {
		err := fmt.Errorf("submit finding error: %s", resp.Error.Message)
		span.RecordError(err)
		span.SetStatus(codes.Error, resp.Error.Message)
		return err
	}

	return nil
}

// PlanContext returns the planning context for the current execution.
// Returns nil if no planning context is available (non-planned execution).
func (h *CallbackHarness) PlanContext() planning.PlanningContext {
	return h.planContext
}

// ReportStepHints allows agents to provide feedback to the planning system.
// This forwards the hints to the orchestrator via gRPC callback.
func (h *CallbackHarness) ReportStepHints(ctx context.Context, hints *planning.StepHints) error {
	if hints == nil {
		return nil // Nothing to report
	}

	// Convert to proto message
	protoReq := &harnesspb.ReportStepHintsRequest{
		Hints: &harnesspb.StepHints{
			Confidence: hints.Confidence(),
		},
	}

	resp, err := h.client.ReportStepHints(ctx, protoReq)
	if err != nil {
		return fmt.Errorf("report step hints callback failed: %w", err)
	}

	if resp.Error != nil {
		return fmt.Errorf("report step hints error: %s", resp.Error.Message)
	}

	return nil
}

// CreateMission creates a new mission from a mission definition.
// The missionDef parameter should be JSON-serializable.
//
// To originate a mission checked into the platform's mission catalog instead,
// set opts.CatalogMission (and opts.CatalogParams) and pass nil for missionDef.
// Exactly one of the two reaches the daemon: naming a catalog mission *and*
// supplying a graph is refused here rather than round-tripping to an
// InvalidArgument, because the caller's intent is already ambiguous locally.
func (h *CallbackHarness) CreateMission(ctx context.Context, missionDef any, targetID string, opts *mission.CreateMissionOpts) (*mission.MissionInfo, error) {
	ctx, span := h.tracer.Start(ctx, "CallbackHarness.CreateMission")
	defer span.End()

	req, err := buildCreateMissionRequest(h.client.contextInfo(), missionDef, targetID, opts)
	if err != nil {
		span.RecordError(err)
		return nil, err
	}

	forks := opts != nil && opts.StartsFromCallerState
	if forks && h.forkWatcher == nil {
		err := errors.New("CreateMission: StartsFromCallerState needs the sandbox id of this process, and the hostname cannot be read")
		span.RecordError(err)
		return nil, err
	}

	resp, err := h.client.CreateMission(ctx, req)

	// The daemon forks this process while the call is open. In the parent the
	// call returns as before. In a fork the open call fails, because the fork
	// has a new network identity, and the process claims its own dispatch
	// (D74, sdk#248).
	if forks {
		claim, perr := fork.Point(ctx, h.forkWatcher, h.client)
		if perr != nil {
			span.RecordError(perr)
			return nil, fmt.Errorf("CreateMission: %w", perr)
		}
		if claim != nil {
			if aerr := h.client.ApplyClaim(claim); aerr != nil {
				span.RecordError(aerr)
				return nil, aerr
			}
			return nil, &fork.ErrForked{Claim: claim}
		}
	}

	if err != nil {
		span.RecordError(err)
		return nil, fmt.Errorf("create mission callback failed: %w", err)
	}

	if resp.Error != nil {
		err := fmt.Errorf("create mission error: %s", resp.Error.Message)
		span.RecordError(err)
		return nil, err
	}

	return protoToMissionInfo(resp.Mission), nil
}

// buildCreateMissionRequest assembles the wire request for CreateMission.
//
// It is a pure function so the one rule that cannot be checked against a live
// daemon is checkable here: exactly one of mission_definition_json or
// catalog_mission may be populated. The daemon enforces that too and answers
// InvalidArgument, but an error raised there names the wire rather than the
// argument the caller should drop.
func buildCreateMissionRequest(
	contextInfo *harnesspb.ContextInfo,
	missionDef any,
	targetID string,
	opts *mission.CreateMissionOpts,
) (*harnesspb.CreateMissionRequest, error) {
	catalogMission := ""
	if opts != nil {
		catalogMission = opts.CatalogMission
	}

	req := &harnesspb.CreateMissionRequest{
		IdempotencyKey: uuid.NewString(),
		Context:        contextInfo,
		TargetId:       targetID,
	}

	switch {
	case catalogMission == "":
		// Serialize mission definition to JSON
		missionDefinitionJSON, err := json.Marshal(missionDef)
		if err != nil {
			return nil, fmt.Errorf("failed to serialize mission definition: %w", err)
		}
		req.MissionDefinitionJson = missionDefinitionJSON
	case missionDef != nil:
		// Both inputs supplied. json.Marshal(nil) yields "null", a non-empty
		// body, so letting this through would populate both fields and the
		// daemon would refuse it. Refuse here, where the caller can see which
		// argument to drop.
		return nil, fmt.Errorf(
			"CreateMission: both a mission definition and CatalogMission %q were supplied; pass nil for missionDef when originating a catalog mission",
			catalogMission,
		)
	}

	if opts != nil {
		req.Name = opts.Name
		req.Tags = opts.Tags
		req.CatalogMission = opts.CatalogMission
		req.CatalogParams = opts.CatalogParams

		// The canonical constraint type carries every constraint field.
		// Field 5 (constraints) is deprecated and is not written (ADR-0004).
		req.CanonicalConstraints = opts.Constraints

		if opts.Metadata != nil {
			req.Metadata = ToTypedMap(opts.Metadata)
		}

		if opts.StartsFromCallerState {
			req.StartsFrom = harnesspb.OriginationStart_ORIGINATION_START_CALLER_STATE
		}
	}

	return req, nil
}

// RunMission queues a mission for execution.
func (h *CallbackHarness) RunMission(ctx context.Context, missionID string, opts *mission.RunMissionOpts) error {
	ctx, span := h.tracer.Start(ctx, "CallbackHarness.RunMission")
	defer span.End()

	req := &harnesspb.RunMissionRequest{
		IdempotencyKey: uuid.NewString(),
		Context:        h.client.contextInfo(),
		MissionId:      missionID,
	}

	if opts != nil {
		req.Wait = opts.Wait
		req.TimeoutMs = opts.Timeout.Milliseconds()
	}

	resp, err := h.client.RunMission(ctx, req)
	if err != nil {
		span.RecordError(err)
		return fmt.Errorf("run mission callback failed: %w", err)
	}

	if resp.Error != nil {
		err := fmt.Errorf("run mission error: %s", resp.Error.Message)
		span.RecordError(err)
		return err
	}

	return nil
}

// GetMissionStatus returns the current state of a mission.
func (h *CallbackHarness) GetMissionStatus(ctx context.Context, missionID string) (*mission.MissionStatusInfo, error) {
	ctx, span := h.tracer.Start(ctx, "CallbackHarness.GetMissionStatus")
	defer span.End()

	req := &harnesspb.GetMissionStatusRequest{
		Context:   h.client.contextInfo(),
		MissionId: missionID,
	}

	resp, err := h.client.GetMissionStatus(ctx, req)
	if err != nil {
		span.RecordError(err)
		return nil, fmt.Errorf("get mission status callback failed: %w", err)
	}

	if resp.Error != nil {
		err := fmt.Errorf("get mission status error: %s", resp.Error.Message)
		span.RecordError(err)
		return nil, err
	}

	return protoToMissionStatusInfo(resp.Status), nil
}

// WaitForMission blocks until a mission completes or the timeout expires.
func (h *CallbackHarness) WaitForMission(ctx context.Context, missionID string, timeout time.Duration) (*mission.MissionResult, error) {
	ctx, span := h.tracer.Start(ctx, "CallbackHarness.WaitForMission")
	defer span.End()

	req := &harnesspb.WaitForMissionRequest{
		Context:   h.client.contextInfo(),
		MissionId: missionID,
		TimeoutMs: timeout.Milliseconds(),
	}

	resp, err := h.client.WaitForMission(ctx, req)
	if err != nil {
		span.RecordError(err)
		return nil, fmt.Errorf("wait for mission callback failed: %w", err)
	}

	if resp.Error != nil {
		err := fmt.Errorf("wait for mission error: %s", resp.Error.Message)
		span.RecordError(err)
		return nil, err
	}

	return protoToMissionResult(resp.Result), nil
}

// ListMissions returns missions matching the provided filter criteria.
func (h *CallbackHarness) ListMissions(ctx context.Context, filter *mission.MissionFilter) ([]*mission.MissionInfo, error) {
	ctx, span := h.tracer.Start(ctx, "CallbackHarness.ListMissions")
	defer span.End()

	req := &harnesspb.ListMissionsRequest{
		Context: h.client.contextInfo(),
	}

	limit := 0
	if filter != nil {
		limit = filter.Limit
		req.Filter = &harnesspb.MissionFilter{
			Tags: filter.Tags,
		}

		if filter.Status != nil {
			req.Filter.Status = missionStatusToProto(*filter.Status)
		}
		if filter.TargetID != nil {
			req.Filter.TargetId = *filter.TargetID
		}
		if filter.ParentMissionID != nil {
			req.Filter.ParentMissionId = *filter.ParentMissionID
		}
		if filter.CreatedAfter != nil {
			req.Filter.CreatedAfter = filter.CreatedAfter.UnixMilli()
		}
		if filter.CreatedBefore != nil {
			req.Filter.CreatedBefore = filter.CreatedBefore.UnixMilli()
		}
	}

	// Read the pages until the limit is reached or no page is left.
	var missions []*mission.MissionInfo
	for {
		if limit > 0 {
			req.PageSize = int32(min(limit-len(missions), maxMissionPageSize)) //nolint:gosec // bounded by maxMissionPageSize
		}

		resp, err := h.client.ListMissions(ctx, req)
		if err != nil {
			span.RecordError(err)
			return nil, fmt.Errorf("list missions callback failed: %w", err)
		}

		if resp.Error != nil {
			err := fmt.Errorf("list missions error: %s", resp.Error.Message)
			span.RecordError(err)
			return nil, err
		}

		for _, m := range resp.Missions {
			missions = append(missions, protoToMissionInfo(m))
		}

		if resp.GetNextPageToken() == "" || (limit > 0 && len(missions) >= limit) {
			break
		}
		req.PageToken = resp.GetNextPageToken()
	}

	if limit > 0 && len(missions) > limit {
		missions = missions[:limit]
	}
	return missions, nil
}

// maxMissionPageSize is the page_size bound of the wire (ADR-0028, rule 3).
const maxMissionPageSize = 1000

// CancelMission requests cancellation of a running mission.
func (h *CallbackHarness) CancelMission(ctx context.Context, missionID string) error {
	ctx, span := h.tracer.Start(ctx, "CallbackHarness.CancelMission")
	defer span.End()

	req := &harnesspb.CancelMissionRequest{
		Context:   h.client.contextInfo(),
		MissionId: missionID,
	}

	resp, err := h.client.CancelMission(ctx, req)
	if err != nil {
		span.RecordError(err)
		return fmt.Errorf("cancel mission callback failed: %w", err)
	}

	if resp.Error != nil {
		err := fmt.Errorf("cancel mission error: %s", resp.Error.Message)
		span.RecordError(err)
		return err
	}

	return nil
}

// GetMissionResults returns the final results of a completed mission.
func (h *CallbackHarness) GetMissionResults(ctx context.Context, missionID string) (*mission.MissionResult, error) {
	ctx, span := h.tracer.Start(ctx, "CallbackHarness.GetMissionResults")
	defer span.End()

	req := &harnesspb.GetMissionResultsRequest{
		Context:   h.client.contextInfo(),
		MissionId: missionID,
	}

	resp, err := h.client.GetMissionResults(ctx, req)
	if err != nil {
		span.RecordError(err)
		return nil, fmt.Errorf("get mission results callback failed: %w", err)
	}

	if resp.Error != nil {
		err := fmt.Errorf("get mission results error: %s", resp.Error.Message)
		span.RecordError(err)
		return nil, err
	}

	return protoToMissionResult(resp.Result), nil
}

// protoToMissionInfo converts proto MissionInfo to SDK mission.MissionInfo.
func protoToMissionInfo(p *harnesspb.MissionInfo) *mission.MissionInfo {
	if p == nil {
		return nil
	}
	return &mission.MissionInfo{
		ID:              p.Id,
		Name:            p.Name,
		Status:          protoToMissionStatus(p.Status),
		TargetID:        p.TargetId,
		ParentMissionID: p.ParentMissionId,
		CreatedAt:       time.UnixMilli(p.CreatedAt),
		Tags:            p.Tags,
	}
}

// protoToMissionStatus converts proto MissionStatus to SDK mission.MissionStatus.
func protoToMissionStatus(p harnesspb.MissionStatus) mission.MissionStatus {
	switch p {
	case harnesspb.MissionStatus_MISSION_STATUS_SUBMITTED:
		return mission.MissionStatusSubmitted
	case harnesspb.MissionStatus_MISSION_STATUS_WORKING:
		return mission.MissionStatusWorking
	case harnesspb.MissionStatus_MISSION_STATUS_PAUSED:
		return mission.MissionStatusPaused
	case harnesspb.MissionStatus_MISSION_STATUS_COMPLETED:
		return mission.MissionStatusCompleted
	case harnesspb.MissionStatus_MISSION_STATUS_FAILED:
		return mission.MissionStatusFailed
	case harnesspb.MissionStatus_MISSION_STATUS_CANCELED:
		return mission.MissionStatusCanceled
	case harnesspb.MissionStatus_MISSION_STATUS_INPUT_REQUIRED:
		return mission.MissionStatusInputRequired
	default:
		return mission.MissionStatusSubmitted
	}
}

// missionStatusToProto converts SDK mission.MissionStatus to proto MissionStatus.
func missionStatusToProto(s mission.MissionStatus) harnesspb.MissionStatus {
	switch s {
	case mission.MissionStatusSubmitted:
		return harnesspb.MissionStatus_MISSION_STATUS_SUBMITTED
	case mission.MissionStatusWorking:
		return harnesspb.MissionStatus_MISSION_STATUS_WORKING
	case mission.MissionStatusPaused:
		return harnesspb.MissionStatus_MISSION_STATUS_PAUSED
	case mission.MissionStatusCompleted:
		return harnesspb.MissionStatus_MISSION_STATUS_COMPLETED
	case mission.MissionStatusFailed:
		return harnesspb.MissionStatus_MISSION_STATUS_FAILED
	case mission.MissionStatusCanceled:
		return harnesspb.MissionStatus_MISSION_STATUS_CANCELED
	case mission.MissionStatusInputRequired:
		return harnesspb.MissionStatus_MISSION_STATUS_INPUT_REQUIRED
	default:
		return harnesspb.MissionStatus_MISSION_STATUS_UNSPECIFIED
	}
}

// protoToMissionStatusInfo converts proto MissionStatusInfo to SDK mission.MissionStatusInfo.
func protoToMissionStatusInfo(p *harnesspb.MissionStatusInfo) *mission.MissionStatusInfo {
	if p == nil {
		return nil
	}

	findingCounts := make(map[string]int)
	for k, v := range p.FindingCounts {
		findingCounts[k] = int(v)
	}

	return &mission.MissionStatusInfo{
		Status:        protoToMissionStatus(p.Status),
		Progress:      p.Progress,
		Phase:         p.Phase,
		FindingCounts: findingCounts,
		TokenUsage:    p.TokenUsage,
		Duration:      time.Duration(p.DurationMs) * time.Millisecond,
		Error:         p.Error,
	}
}

// protoToMissionResult converts proto MissionResult to SDK mission.MissionResult.
func protoToMissionResult(p *harnesspb.MissionResult) *mission.MissionResult {
	if p == nil {
		return nil
	}

	// Convert findings
	findings := make([]finding.Finding, 0, len(p.Findings))
	for _, f := range p.Findings {
		if f != nil {
			findings = append(findings, *FindingFromProto(f))
		}
	}

	// Convert output
	output := FromTypedMap(p.Output)

	result := &mission.MissionResult{
		MissionID:   p.MissionId,
		Status:      protoToMissionStatus(p.Status),
		Findings:    findings,
		Output:      output,
		Error:       p.Error,
		CompletedAt: time.UnixMilli(p.CompletedAt),
	}

	if p.Metrics != nil {
		result.Metrics = mission.MissionMetrics{
			Duration:      time.Duration(p.Metrics.DurationMs) * time.Millisecond,
			TokensUsed:    p.Metrics.TokensUsed,
			ToolCalls:     int(p.Metrics.ToolCalls),
			AgentCalls:    int(p.Metrics.AgentCalls),
			FindingsCount: int(p.Metrics.FindingsCount),
		}
	}

	return result
}

// QueueToolWork queues multiple tool invocations for parallel execution.
// Returns a job ID that can be used to retrieve results via ToolResults.
// This enables agents to submit batches of work for parallel processing by workers.
func (h *CallbackHarness) QueueToolWork(ctx context.Context, toolName string, inputs []protolib.Message) (string, error) {
	// Start span for queue submission
	ctx, span := h.tracer.Start(ctx, "gibson.tool.queue_work",
		trace.WithSpanKind(trace.SpanKindClient),
		trace.WithAttributes(
			attribute.String("gibson.tool.name", toolName),
			attribute.Int("gibson.tool.batch_size", len(inputs)),
		),
	)
	defer span.End()

	if len(inputs) == 0 {
		err := errors.New("no inputs provided for queue work")
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return "", err
	}

	// Use protojson marshaler with snake_case field names
	marshaler := protojson.MarshalOptions{
		UseProtoNames: true, // Use snake_case (proto field names) instead of camelCase
	}

	// Marshal each input proto to JSON
	inputJSONs := make([]string, len(inputs))
	var inputType, outputType string

	for i, input := range inputs {
		if input == nil {
			err := fmt.Errorf("input at index %d is nil", i)
			span.RecordError(err)
			span.SetStatus(codes.Error, err.Error())
			return "", err
		}

		// Marshal input to JSON
		inputJSON, err := marshaler.Marshal(input)
		if err != nil {
			span.RecordError(err)
			span.SetStatus(codes.Error, "failed to marshal input to JSON")
			return "", fmt.Errorf("failed to marshal input at index %d to JSON: %w", i, err)
		}

		inputJSONs[i] = string(inputJSON)

		// Extract input type from first message (all must be same type)
		if i == 0 {
			inputType = string(input.ProtoReflect().Descriptor().FullName())

			// Output type will be resolved by the daemon from tool metadata.
			outputType = ""
		}
	}

	// Create the proto request
	protoReq := &harnesspb.QueueToolWorkRequest{
		Context:    h.client.contextInfo(),
		ToolName:   toolName,
		InputJsons: inputJSONs,
		InputType:  inputType,
		OutputType: outputType,
	}

	// Call daemon RPC to queue the batch work.
	resp, err := h.client.QueueToolWork(ctx, protoReq)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return "", fmt.Errorf("QueueToolWork callback failed: %w", err)
	}

	if resp.Error != nil {
		err := fmt.Errorf("QueueToolWork error: %s", resp.Error.Message)
		span.RecordError(err)
		span.SetStatus(codes.Error, resp.Error.Message)
		return "", err
	}

	// Record job ID in span
	span.SetAttributes(attribute.String("gibson.tool.job_id", resp.JobId))

	return resp.JobId, nil
}

// ToolResults returns a channel that streams results for a queued job.
// The channel will receive one QueuedToolResult per input in the original batch.
// Results may arrive in any order (check the Index field).
// The channel will be closed when all results have been received or an error occurs.
func (h *CallbackHarness) ToolResults(ctx context.Context, jobID string) <-chan agent.QueuedToolResult {
	// Start span for results streaming
	ctx, span := h.tracer.Start(ctx, "gibson.tool.results",
		trace.WithSpanKind(trace.SpanKindClient),
		trace.WithAttributes(
			attribute.String("gibson.tool.job_id", jobID),
		),
	)

	// Create result channel (buffered for better performance)
	resultChan := make(chan agent.QueuedToolResult, 10)

	// Spawn goroutine to receive streaming results
	go func() {
		defer close(resultChan)
		defer span.End()

		// Create streaming request
		protoReq := &harnesspb.ToolResultsRequest{
			Context: h.client.contextInfo(),
			JobId:   jobID,
		}

		// Call daemon RPC to stream results for the queued batch.
		stream, err := h.client.ToolResults(ctx, protoReq)
		if err != nil {
			// Send error result
			h.logger.Error("ToolResults stream failed", "error", err, "job_id", jobID)
			span.RecordError(err)
			span.SetStatus(codes.Error, err.Error())
			resultChan <- agent.QueuedToolResult{
				Index:  0,
				Output: nil,
				Error:  fmt.Errorf("failed to establish results stream: %w", err),
			}
			return
		}

		resultCount := 0
		for {
			// Receive next result from stream
			protoResult, err := stream.Recv()
			if err != nil {
				// Check if stream ended normally
				if errors.Is(err, io.EOF) {
					span.SetAttributes(attribute.Int("gibson.tool.results_count", resultCount))
					return
				}

				// Stream error
				h.logger.Error("error receiving result from stream", "error", err, "job_id", jobID)
				span.RecordError(err)
				span.SetStatus(codes.Error, err.Error())
				resultChan <- agent.QueuedToolResult{
					Index:  0,
					Output: nil,
					Error:  fmt.Errorf("stream recv error: %w", err),
				}
				return
			}

			resultCount++

			// Convert proto result to agent.QueuedToolResult
			var toolResult agent.QueuedToolResult

			// Handle error results
			if protoResult.Error != nil {
				toolResult = agent.QueuedToolResult{
					Index:  int(protoResult.Index),
					Output: nil,
					Error:  fmt.Errorf("%s", protoResult.Error.Message),
				}
			} else {
				// Parse output JSON into proto message using protoregistry
				var outputProto protolib.Message

				if protoResult.OutputType != "" {
					// Look up message type in registry
					msgType, err := protoregistry.GlobalTypes.FindMessageByName(protoreflect.FullName(protoResult.OutputType))
					if err != nil {
						toolResult = agent.QueuedToolResult{
							Index:  int(protoResult.Index),
							Output: nil,
							Error:  fmt.Errorf("unknown output type %s: %w", protoResult.OutputType, err),
						}
					} else {
						// Create new message instance
						outputProto = msgType.New().Interface()

						// Unmarshal JSON into proto message
						unmarshaler := protojson.UnmarshalOptions{
							DiscardUnknown: true, // Ignore fields not in proto
						}

						if err := unmarshaler.Unmarshal([]byte(protoResult.OutputJson), outputProto); err != nil {
							toolResult = agent.QueuedToolResult{
								Index:  int(protoResult.Index),
								Output: nil,
								Error:  fmt.Errorf("failed to unmarshal output JSON: %w", err),
							}
						} else {
							// Successfully unmarshaled output
							toolResult = agent.QueuedToolResult{
								Index:  int(protoResult.Index),
								Output: outputProto,
								Error:  nil,
							}
						}
					}
				} else {
					// No output type specified - this is an error
					toolResult = agent.QueuedToolResult{
						Index:  int(protoResult.Index),
						Output: nil,
						Error:  errors.New("output type not specified in result"),
					}
				}
			}

			// Send result on channel
			toolResult.ToolCallID = protoResult.GetToolCallId()
			select {
			case resultChan <- toolResult:
			case <-ctx.Done():
				h.logger.Warn("context cancelled while sending result", "job_id", jobID)
				span.RecordError(ctx.Err())
				span.SetStatus(codes.Error, ctx.Err().Error())
				return
			}

			// Check if this was the final result
			if protoResult.IsFinal {
				span.SetAttributes(attribute.Int("gibson.tool.results_count", resultCount))
				return
			}
		}
	}()

	return resultChan
}

// Workspace returns the primary workspace for single-repository
// missions, or nil if the mission has no workspace configured (the
// daemon answers FailedPrecondition for that case).
//
// Spec: callback-harness-workspace-rpcs.
//
// v1 surface limitation: the returned Workspace exposes ReadFile,
// WriteFile, ListFiles, Commit, and Push. Calls into Editor() return
// ErrWorkspaceNotImplemented, and Git() exposes only Commit + Push;
// every other GitOps method also returns ErrWorkspaceNotImplemented.
func (h *CallbackHarness) Workspace() workspace.Workspace {
	resp, err := h.client.WorkspaceGetInfo(context.Background(),
		&harnesspb.WorkspaceGetInfoRequest{}) // empty name = primary
	if err != nil {
		if errors.Is(err, ErrWorkspaceNotConfigured) {
			return nil
		}
		h.logger.Warn("Workspace get-info failed", "err", err)
		return nil
	}
	if resp.GetWorkspace() == nil {
		return nil
	}
	return newCallbackWorkspace(h.client, resp.GetWorkspace())
}

// Workspaces returns all workspaces keyed by repository name. Returns
// an empty map if no workspaces are configured.
//
// Spec: callback-harness-workspace-rpcs. Same v1 surface limitations
// as Workspace().
func (h *CallbackHarness) Workspaces() map[string]workspace.Workspace {
	resp, err := h.client.WorkspaceList(context.Background(),
		&harnesspb.WorkspaceListRequest{})
	if err != nil {
		if !errors.Is(err, ErrWorkspaceNotConfigured) {
			h.logger.Warn("Workspaces list failed", "err", err)
		}
		return make(map[string]workspace.Workspace)
	}
	out := make(map[string]workspace.Workspace, len(resp.GetWorkspaces()))
	for _, info := range resp.GetWorkspaces() {
		out[info.GetName()] = newCallbackWorkspace(h.client, info)
	}
	return out
}

// Authorize implements agent.Harness by forwarding the authorization check to
// the daemon's HarnessCallbackService.Authorize RPC. It fails closed: any
// answer other than an explicit allow is an error.
//
// The run id of the RPC is the mission run of the task that the client serves.
// RunTask-style callers set it with CallbackClient.SetFullContext, and a fork
// gets it from its claim.
//
// Error behavior:
//   - nil                        → the daemon allowed the action
//   - ErrUnauthorized            → FGA denied, or the daemon answered
//     PermissionDenied or Unauthenticated; caller must refuse the operation
//   - ErrAuthzServiceUnavailable → daemon/FGA unreachable or the daemon has no
//     Authorize RPC; treated as deny
//   - ErrNoRunContext            → the client serves no mission run, so there
//     is nobody to authorize; treated as deny
//   - ErrInvalidAction           → action or resource is empty/malformed, or the
//     daemon answered InvalidArgument
func (h *CallbackHarness) Authorize(ctx context.Context, action, resource string) error {
	if action == "" || resource == "" {
		return harnessconst.ErrInvalidAction
	}

	runID := h.client.missionRun()
	if runID == "" {
		h.logger.WarnContext(ctx, "Authorize: the client serves no mission run; denying",
			"action", action, "resource", resource)
		return harnessconst.ErrNoRunContext
	}

	req := &harnesspb.AuthorizeRequest{
		RunId:    runID,
		Action:   action,
		Resource: resource,
	}

	resp, err := h.client.Authorize(ctx, req)
	if err != nil {
		return authorizeError(ctx, h.logger, runID, action, resource, err)
	}

	if !resp.Allowed {
		h.logger.Info("authz_denied_local",
			"run_id", runID,
			"action", action,
			"resource", resource,
			"reason", resp.Reason,
		)
		return harnessconst.ErrUnauthorized
	}

	return nil
}

// QueryNodes searches the tenant knowledge graph for a dispatched agent.
func (h *CallbackHarness) QueryNodes(ctx context.Context, query *graphragpb.GraphQuery) ([]*graphragpb.QueryResult, error) {
	resp, err := h.client.QueryNodes(ctx, &harnesspb.QueryNodesRequest{Query: query})
	if err != nil {
		return nil, fmt.Errorf("query nodes callback failed: %w", err)
	}
	if e := resp.GetError(); e != nil && e.GetMessage() != "" {
		return nil, fmt.Errorf("query nodes rejected: %s", e.GetMessage())
	}
	return resp.GetResults(), nil
}

// FindSimilarFindings returns findings semantically close to the given one for a dispatched agent.
func (h *CallbackHarness) FindSimilarFindings(ctx context.Context, findingID string, topK int) ([]*graphragpb.FindingNode, error) {
	resp, err := h.client.FindSimilarFindings(ctx, &harnesspb.FindSimilarFindingsRequest{
		FindingId: findingID, TopK: boundedInt32(topK),
	})
	if err != nil {
		return nil, fmt.Errorf("find similar findings callback failed: %w", err)
	}
	if e := resp.GetError(); e != nil && e.GetMessage() != "" {
		return nil, fmt.Errorf("find similar findings rejected: %s", e.GetMessage())
	}
	return resp.GetResults(), nil
}

// GetRelatedFindings returns findings reachable from the given one by graph relationship for a dispatched agent.
func (h *CallbackHarness) GetRelatedFindings(ctx context.Context, findingID string) ([]*graphragpb.FindingNode, error) {
	resp, err := h.client.GetRelatedFindings(ctx, &harnesspb.GetRelatedFindingsRequest{FindingId: findingID})
	if err != nil {
		return nil, fmt.Errorf("get related findings callback failed: %w", err)
	}
	if e := resp.GetError(); e != nil && e.GetMessage() != "" {
		return nil, fmt.Errorf("get related findings rejected: %s", e.GetMessage())
	}
	return resp.GetResults(), nil
}

// GetFindings returns previously submitted findings matching a filter for a dispatched agent.
func (h *CallbackHarness) GetFindings(ctx context.Context, filter finding.Filter) ([]*finding.Finding, error) {
	resp, err := h.client.GetFindings(ctx, &harnesspb.GetFindingsRequest{Filter: findingFilterToProto(filter)})
	if err != nil {
		return nil, fmt.Errorf("get findings callback failed: %w", err)
	}
	if e := resp.GetError(); e != nil && e.GetMessage() != "" {
		return nil, fmt.Errorf("get findings rejected: %s", e.GetMessage())
	}
	return findingsFromProto(resp.GetFindings()), nil
}

// GetRunFindings returns findings from earlier runs of this mission for a dispatched agent.
func (h *CallbackHarness) GetRunFindings(ctx context.Context, scope agent.RunScope, filter finding.Filter) ([]*finding.Finding, error) {
	resp, err := h.client.GetRunFindings(ctx, &harnesspb.GetRunFindingsRequest{
		Scope: runScopeToProto(scope), Filter: findingFilterToProto(filter),
	})
	if err != nil {
		return nil, fmt.Errorf("get run findings callback failed: %w", err)
	}
	if e := resp.GetError(); e != nil && e.GetMessage() != "" {
		return nil, fmt.Errorf("get run findings rejected: %s", e.GetMessage())
	}
	return findingsFromProto(resp.GetFindings()), nil
}

// GetMissionRunHistory returns every run of the caller’s mission for a dispatched agent.
func (h *CallbackHarness) GetMissionRunHistory(ctx context.Context) ([]types.MissionRunSummary, error) {
	resp, err := h.client.GetMissionRunHistory(ctx, &harnesspb.GetMissionRunHistoryRequest{})
	if err != nil {
		return nil, fmt.Errorf("get mission run history callback failed: %w", err)
	}
	if e := resp.GetError(); e != nil && e.GetMessage() != "" {
		return nil, fmt.Errorf("get mission run history rejected: %s", e.GetMessage())
	}
	return runSummariesFromProto(resp.GetRuns()), nil
}

// ApplicationFindings returns one Application's Findings with their reachability
// and exposure, for a dispatched agent.
//
// Every failure path here returns an error rather than an empty slice. That is
// the standing KnowledgeReader rule (ErrKnowledgeUnavailable), and it matters
// more on this read than on the others: a caller reads "not reachable" as
// "nothing runs this" and ranks the finding last, so a silently empty result
// would bury a live backlog instead of surfacing as a failure.
func (h *CallbackHarness) ApplicationFindings(ctx context.Context, application string, statuses []string, limit int) ([]agent.ApplicationFinding, error) {
	resp, err := h.client.ApplicationFindings(ctx, &harnesspb.ApplicationFindingsRequest{
		Application: application,
		Statuses:    statuses,
		Limit:       int32(limit), //nolint:gosec // a caller-supplied page size; the server caps it.
	})
	if err != nil {
		return nil, fmt.Errorf("application findings callback failed: %w", err)
	}
	if e := resp.GetError(); e != nil && e.GetMessage() != "" {
		return nil, fmt.Errorf("application findings rejected: %s", e.GetMessage())
	}
	v, err := requestValidator()
	if err != nil {
		return nil, fmt.Errorf("application findings: build the validator: %w", err)
	}
	return applicationFindingsFromProto(dropInvalidFixFields(v, resp.GetFindings(), h.logger)), nil
}

// dropInvalidFixFields validates each finding of a response with the rules of
// its proto. A finding that breaks a rule loses fix_head_sha and fix_files and
// keeps the rest, so that an agent never merges on values that break the
// contract. The input is not changed.
func dropInvalidFixFields(v protovalidate.Validator, pfs []*harnesspb.ApplicationFinding, log *slog.Logger) []*harnesspb.ApplicationFinding {
	out := make([]*harnesspb.ApplicationFinding, len(pfs))
	for i, pf := range pfs {
		out[i] = pf
		if pf == nil {
			continue
		}
		if verr := v.Validate(pf); verr != nil {
			c := protolib.Clone(pf).(*harnesspb.ApplicationFinding) //nolint:errcheck,forcetypeassert // Clone returns the same type
			c.FixHeadSha, c.FixFiles = "", nil
			out[i] = c
			if log != nil {
				log.Warn("application finding breaks the contract: its fix head and files are dropped",
					"finding", pf.GetFindingId(), "error", verr.Error())
			}
		}
	}
	return out
}

// requestValidator validates a request with the rules of its proto. The form of
// a technique id is one rule, in the proto, and this is the one place that
// reads it.
var requestValidator = sync.OnceValues(func() (protovalidate.Validator, error) {
	return protovalidate.New() //nolint:wrapcheck // wrapped by the caller
})

// Countermeasures returns the D3FEND countermeasures of one ATT&CK technique,
// for a dispatched agent. A failed read returns an error and never an empty
// slice: an empty slice means that no countermeasure counters the technique.
func (h *CallbackHarness) Countermeasures(ctx context.Context, technique string) ([]agent.Countermeasure, error) {
	if technique == "" {
		return []agent.Countermeasure{}, nil // a Finding with no technique has no countermeasure to read
	}
	req := &harnesspb.GetCountermeasuresRequest{Technique: technique}
	v, err := requestValidator()
	if err != nil {
		return nil, fmt.Errorf("countermeasures: build the validator: %w", err)
	}
	if verr := v.Validate(req); verr != nil {
		return nil, fmt.Errorf("countermeasures: %q is not an ATT&CK technique id: %w", technique, verr)
	}
	resp, err := h.client.GetCountermeasures(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("countermeasures callback failed: %w", err)
	}
	if e := resp.GetError(); e != nil && e.GetMessage() != "" {
		return nil, fmt.Errorf("countermeasures rejected: %s", e.GetMessage())
	}
	return countermeasuresFromProto(resp.GetCountermeasures()), nil
}

// countermeasuresFromProto maps the wire slice to the SDK-native type. A nil
// entry is skipped, so it never becomes a countermeasure with no id.
func countermeasuresFromProto(pcs []*harnesspb.Countermeasure) []agent.Countermeasure {
	out := make([]agent.Countermeasure, 0, len(pcs))
	for _, pc := range pcs {
		if pc == nil {
			continue
		}
		out = append(out, agent.Countermeasure{
			ID: pc.GetId(), Name: pc.GetName(), Tactic: pc.GetTactic(), NISTControls: pc.GetNistControls(),
		})
	}
	return out
}

// fixChange returns the change of a fix file as a lower case word: added,
// modified, deleted or renamed. An unspecified or unknown change returns "".
func fixChange(c harnesspb.FixFileChange) string {
	switch c {
	case harnesspb.FixFileChange_FIX_FILE_CHANGE_ADDED:
		return "added"
	case harnesspb.FixFileChange_FIX_FILE_CHANGE_MODIFIED:
		return "modified"
	case harnesspb.FixFileChange_FIX_FILE_CHANGE_DELETED:
		return "deleted"
	case harnesspb.FixFileChange_FIX_FILE_CHANGE_RENAMED:
		return "renamed"
	case harnesspb.FixFileChange_FIX_FILE_CHANGE_UNSPECIFIED:
		return ""
	}
	return ""
}

// fixFilesFromProto maps the files of a fix merge request. A nil entry is skipped.
func fixFilesFromProto(pfs []*harnesspb.FixFile) []agent.FixFile {
	if len(pfs) == 0 {
		return nil
	}
	out := make([]agent.FixFile, 0, len(pfs))
	for _, pf := range pfs {
		if pf == nil {
			continue
		}
		out = append(out, agent.FixFile{
			Path: pf.GetPath(), Change: fixChange(pf.GetChange()), PreviousPath: pf.GetPreviousPath(), BlobSHA: pf.GetBlobSha(),
		})
	}
	return out
}

// applicationFindingsFromProto maps the wire slice to the SDK-native type, once,
// rather than at each caller.
func applicationFindingsFromProto(pfs []*harnesspb.ApplicationFinding) []agent.ApplicationFinding {
	out := make([]agent.ApplicationFinding, 0, len(pfs))
	for _, pf := range pfs {
		if pf == nil {
			continue
		}
		out = append(out, agent.ApplicationFinding{
			FindingID:        pf.GetFindingId(),
			Status:           pf.GetStatus(),
			Severity:         pf.GetSeverity(),
			VulnerabilityID:  pf.GetVulnerabilityId(),
			PlaceLabel:       pf.GetPlaceLabel(),
			PlaceKey:         pf.GetPlaceKey(),
			Reachable:        pf.GetReachable(),
			Exposed:          pf.GetExposed(),
			DeploymentKey:    pf.GetDeploymentKey(),
			ImageKey:         pf.GetImageKey(),
			Priority:         pf.GetPriority(),
			PriorityRule:     pf.GetPriorityRule(),
			PriorityReason:   pf.GetPriorityReason(),
			FixBlockedReason: pf.GetFixBlockedReason(),
			MergeRequest:     pf.GetMergeRequest(),
			MergeRequestURL:  pf.GetMergeRequestUrl(),
			Technique:        pf.GetTechnique(),
			FixHeadSHA:       pf.GetFixHeadSha(),
			FixFiles:         fixFilesFromProto(pf.GetFixFiles()),
		})
	}
	return out
}

// findingsFromProto maps a wire slice to the SDK-native type. The conversion
// lives here, once, rather than at each caller — that is the whole reason
// KnowledgeReader returns finding.Finding instead of the proto message.
func findingsFromProto(pfs []*typespb.Finding) []*finding.Finding {
	out := make([]*finding.Finding, 0, len(pfs))
	for _, pf := range pfs {
		if f := FindingFromProto(pf); f != nil {
			out = append(out, f)
		}
	}
	return out
}

func runScopeToProto(s agent.RunScope) harnesspb.RunScope {
	switch s {
	case agent.RunScopePrevious:
		return harnesspb.RunScope_RUN_SCOPE_PREVIOUS
	case agent.RunScopeAll:
		return harnesspb.RunScope_RUN_SCOPE_ALL
	case agent.RunScopeUnspecified:
		return harnesspb.RunScope_RUN_SCOPE_UNSPECIFIED
	default:
		return harnesspb.RunScope_RUN_SCOPE_UNSPECIFIED
	}
}

func findingFilterToProto(f finding.Filter) *harnesspb.FindingFilter {
	pf := &harnesspb.FindingFilter{
		MissionId: f.MissionID,
		AgentName: f.AgentName,
		Status:    statusToProto(f.Status),
	}
	if len(f.Severities) > 0 {
		// The wire filter carries a single severity; the SDK filter carries a
		// set. Send the first and let the caller narrow further client-side
		// rather than silently dropping the whole filter.
		pf.Severity = severityToProto(f.Severities[0])
	}
	pf.Tags = f.Tags
	return pf
}

// runSummariesFromProto maps wire summaries to the SDK-native type. types.MissionRunSummary
// already existed — the conversion belongs here, once, not at every caller.
func runSummariesFromProto(rs []*typespb.MissionRunSummary) []types.MissionRunSummary {
	out := make([]types.MissionRunSummary, 0, len(rs))
	for _, r := range rs {
		if r == nil {
			continue
		}
		s := types.MissionRunSummary{
			MissionID:     r.GetMissionId(),
			RunNumber:     int(r.GetRunNumber()),
			Status:        r.GetStatus(),
			FindingsCount: int(r.GetFindingsCount()),
			CreatedAt:     time.UnixMilli(r.GetCreatedAt()),
		}
		// completed_at is 0 while the run is still in flight; a zero time would
		// read as "finished at the epoch", so leave the pointer nil instead.
		if r.GetCompletedAt() != 0 {
			t := time.UnixMilli(r.GetCompletedAt())
			s.CompletedAt = &t
		}
		out = append(out, s)
	}
	return out
}

// boundedInt32 narrows a caller-supplied count to the wire's int32 without
// wrapping. A negative or absurd top-k is a caller mistake, and silently
// wrapping it into a negative int32 would turn that mistake into a query the
// daemon cannot make sense of.
func boundedInt32(n int) int32 {
	if n < 0 {
		return 0
	}
	if n > math.MaxInt32 {
		return math.MaxInt32
	}
	return int32(n)
}

// Delegate sends an A2A message to another agent through the Agent gateway and
// waits for the A2A Task to end. The platform resolves the route of the agent.
func (h *CallbackHarness) Delegate(
	ctx context.Context, name string, msg *a2a.Message, opts ...agent.DelegateOption,
) (*a2a.Task, error) {
	bankID := agent.ApplyDelegateOptions(opts).BankID
	return delegateVia(ctx, h.tracer, "CallbackHarness", name, func(ctx context.Context) (*resolvedAgent, error) {
		resp, err := h.client.ResolveAgent(ctx, name, bankID)
		if err != nil {
			return nil, err
		}
		return &resolvedAgent{
			gatewayURL: resp.GetGatewayUrl(), routeHeaders: resp.GetRouteHeaders(), gatewayCAPEM: resp.GetGatewayCaPem(),
		}, nil
	}, h.client.grantHeaders(), msg)
}
