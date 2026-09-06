// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Zero Root AI

package serve

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"math"
	"sync"
	"time"

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
	"github.com/zeroroot-ai/sdk/graphrag"
	harnessconst "github.com/zeroroot-ai/sdk/harness"
	"github.com/zeroroot-ai/sdk/llm"
	"github.com/zeroroot-ai/sdk/mission"
	"github.com/zeroroot-ai/sdk/planning"
	"github.com/zeroroot-ai/sdk/plugin"
	"github.com/zeroroot-ai/sdk/schema"
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

	// Context
	logger         *slog.Logger
	tracer         trace.Tracer
	mission        types.MissionContext
	target         types.TargetInfo
	planContext    planning.PlanningContext
	missionExecCtx types.MissionExecutionContext

	// Taxonomy support
	taxonomy         *TaxonomyAdapter
	taxonomyInitOnce sync.Once

	// Caching for list operations
	cacheMu      sync.RWMutex
	toolsCache   []tool.Descriptor
	pluginsCache []plugin.Descriptor
	agentsCache  []agent.Descriptor

	// Authorization context — populated from the work envelope's AuthzContext.
	// authzRunID is the mission run ID used to resolve the caller in Authorize calls.
	// failOpenAuthz controls behavior when the daemon is unreachable:
	//   false (default) — fail-closed, treat as deny.
	//   true            — fail-open (dev mode), log WARN and proceed.
	authzRunID    string
	failOpenAuthz bool
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

		// Set global taxonomy in SDK
		graphrag.SetTaxonomy(h.taxonomy)

		h.logger.Info("taxonomy initialized successfully",
			"version", h.taxonomy.Version(),
			"node_types", len(h.taxonomy.NodeTypes()),
			"relationship_types", len(h.taxonomy.RelationshipTypes()),
			"techniques", len(h.taxonomy.TechniqueIDs("")))
	})
}

// SetPlanContext sets the planning context for this harness.
// This should be called by the orchestrator when executing a planned mission.
func (h *CallbackHarness) SetPlanContext(ctx planning.PlanningContext) {
	h.planContext = ctx
}

// SetMissionExecutionContext sets the mission execution context for this harness.
// This should be called by the orchestrator when executing a mission with run history.
func (h *CallbackHarness) SetMissionExecutionContext(ctx types.MissionExecutionContext) {
	h.missionExecCtx = ctx
}

// ============================================================================
// Core Harness Methods
// ============================================================================

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

// ============================================================================
// LLM Operations
// ============================================================================

// Complete performs a single LLM completion request via the orchestrator.
func (h *CallbackHarness) Complete(ctx context.Context, slot string, messages []llm.Message, opts ...llm.CompletionOption) (*llm.CompletionResponse, error) {
	// Start span for LLM completion
	ctx, span := h.tracer.Start(ctx, "gen_ai.chat",
		trace.WithSpanKind(trace.SpanKindClient),
		trace.WithAttributes(
			attribute.String("gen_ai.system", "anthropic"),
			attribute.String("gen_ai.request.model", slot),
			attribute.Int("gen_ai.request.message_count", len(messages)),
		),
	)
	defer span.End()

	// Add prompt attribute for observability
	span.SetAttributes(attribute.String("gen_ai.prompt", formatMessagesForPrompt(messages)))

	// Build completion request with options
	req := llm.NewCompletionRequest(messages, opts...)

	// Convert to proto request
	protoReq := &harnesspb.LLMCompleteRequest{
		Slot:     slot,
		Messages: h.messagesToProto(messages),
	}

	// Apply options
	if req.Temperature != nil {
		temp := *req.Temperature
		protoReq.Temperature = &temp
		span.SetAttributes(attribute.Float64("gen_ai.request.temperature", float64(temp)))
	}
	if req.MaxTokens != nil {
		maxTokens := int32(*req.MaxTokens)
		protoReq.MaxTokens = &maxTokens
		span.SetAttributes(attribute.Int("gen_ai.request.max_tokens", int(*req.MaxTokens)))
	}
	if req.TopP != nil {
		topP := *req.TopP
		protoReq.TopP = &topP
		span.SetAttributes(attribute.Float64("gen_ai.request.top_p", float64(topP)))
	}
	protoReq.Stop = req.Stop

	// Call orchestrator
	resp, err := h.client.LLMComplete(ctx, protoReq)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return nil, fmt.Errorf("LLM complete callback failed: %w", err)
	}

	if resp.Error != nil {
		err := fmt.Errorf("LLM complete error: %s", resp.Error.Message)
		span.RecordError(err)
		span.SetStatus(codes.Error, resp.Error.Message)
		return nil, err
	}

	// Convert response
	result := &llm.CompletionResponse{
		Content:      resp.Content,
		ToolCalls:    h.toolCallsFromProto(resp.ToolCalls),
		FinishReason: resp.FinishReason,
		Usage: llm.TokenUsage{
			InputTokens:  int(resp.Usage.InputTokens),
			OutputTokens: int(resp.Usage.OutputTokens),
			TotalTokens:  int(resp.Usage.TotalTokens),
		},
	}

	// Record token usage and response in span
	span.SetAttributes(
		attribute.Int("gen_ai.usage.input_tokens", result.Usage.InputTokens),
		attribute.Int("gen_ai.usage.output_tokens", result.Usage.OutputTokens),
		attribute.String("gen_ai.response.finish_reason", result.FinishReason),
		attribute.String("gen_ai.completion", result.Content),
		attribute.String("gen_ai.response.model", slot),
	)

	// Track token usage
	h.tokenTracker.Add(slot, result.Usage)

	return result, nil
}

// CompleteWithTools performs a completion with tool calling enabled.
func (h *CallbackHarness) CompleteWithTools(ctx context.Context, slot string, messages []llm.Message, tools []llm.ToolDef) (*llm.CompletionResponse, error) {
	// Start span for LLM completion with tools
	ctx, span := h.tracer.Start(ctx, "gen_ai.chat",
		trace.WithSpanKind(trace.SpanKindClient),
		trace.WithAttributes(
			attribute.String("gen_ai.system", "anthropic"),
			attribute.String("gen_ai.request.model", slot),
			attribute.Int("gen_ai.request.message_count", len(messages)),
			attribute.Int("gen_ai.request.tool_count", len(tools)),
		),
	)
	defer span.End()

	// Add prompt attribute for observability
	span.SetAttributes(attribute.String("gen_ai.prompt", formatMessagesForPrompt(messages)))

	protoReq := &harnesspb.LLMCompleteWithToolsRequest{
		Slot:     slot,
		Messages: h.messagesToProto(messages),
		Tools:    h.toolDefsToProto(tools),
	}

	resp, err := h.client.LLMCompleteWithTools(ctx, protoReq)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return nil, fmt.Errorf("LLM complete with tools callback failed: %w", err)
	}

	if resp.Error != nil {
		err := fmt.Errorf("LLM complete with tools error: %s", resp.Error.Message)
		span.RecordError(err)
		span.SetStatus(codes.Error, resp.Error.Message)
		return nil, err
	}

	result := &llm.CompletionResponse{
		Content:      resp.Content,
		ToolCalls:    h.toolCallsFromProto(resp.ToolCalls),
		FinishReason: resp.FinishReason,
		Usage: llm.TokenUsage{
			InputTokens:  int(resp.Usage.InputTokens),
			OutputTokens: int(resp.Usage.OutputTokens),
			TotalTokens:  int(resp.Usage.TotalTokens),
		},
	}

	// Record token usage and response in span
	span.SetAttributes(
		attribute.Int("gen_ai.usage.input_tokens", result.Usage.InputTokens),
		attribute.Int("gen_ai.usage.output_tokens", result.Usage.OutputTokens),
		attribute.String("gen_ai.response.finish_reason", result.FinishReason),
		attribute.Int("gen_ai.response.tool_call_count", len(result.ToolCalls)),
		attribute.String("gen_ai.completion", result.Content),
		attribute.String("gen_ai.response.model", slot),
	)

	// Track token usage
	h.tokenTracker.Add(slot, result.Usage)

	return result, nil
}

// CompleteStructured performs a completion with provider-native structured output.
// This forwards the request to the orchestrator which handles schema conversion
// and provider-specific structured output mechanisms.
//
// The schemaType parameter should be a Go struct (or pointer to struct) that
// defines the expected response structure. The method generates a JSON schema
// from the type and sends it to the daemon for LLM completion.
func (h *CallbackHarness) CompleteStructured(ctx context.Context, slot string, messages []llm.Message, schemaType any) (any, error) {
	// Generate JSON schema from the Go type
	// This converts the struct definition to a proper JSON schema that the LLM can use
	jsonSchema := schema.FromType(schemaType)

	// Serialize the schema to JSON for transmission
	schemaJSON, err := json.Marshal(jsonSchema)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal schema: %w", err)
	}

	protoReq := &harnesspb.LLMCompleteStructuredRequest{
		Slot:       slot,
		Messages:   h.messagesToProto(messages),
		SchemaJson: string(schemaJSON),
	}

	resp, err := h.client.LLMCompleteStructured(ctx, protoReq)
	if err != nil {
		return nil, fmt.Errorf("LLM complete structured callback failed: %w", err)
	}

	if resp.Error != nil {
		return nil, fmt.Errorf("LLM complete structured error: %s", resp.Error.Message)
	}

	// Convert TypedValue result to Go value
	result := FromTypedValue(resp.Result)

	// Track token usage if available
	if resp.Usage != nil {
		usage := llm.TokenUsage{
			InputTokens:  int(resp.Usage.InputTokens),
			OutputTokens: int(resp.Usage.OutputTokens),
			TotalTokens:  int(resp.Usage.TotalTokens),
		}
		h.tokenTracker.Add(slot, usage)
	}

	return result, nil
}

// CompleteStructuredAny is an alias for CompleteStructured for compatibility.
func (h *CallbackHarness) CompleteStructuredAny(ctx context.Context, slot string, messages []llm.Message, schema any) (any, error) {
	return h.CompleteStructured(ctx, slot, messages, schema)
}

// Stream performs a streaming completion request.
func (h *CallbackHarness) Stream(ctx context.Context, slot string, messages []llm.Message) (<-chan llm.StreamChunk, error) {
	// Start span for streaming LLM completion
	ctx, span := h.tracer.Start(ctx, "gen_ai.chat.stream",
		trace.WithSpanKind(trace.SpanKindClient),
		trace.WithAttributes(
			attribute.String("gen_ai.system", "anthropic"),
			attribute.String("gen_ai.request.model", slot),
			attribute.Int("gen_ai.request.message_count", len(messages)),
		),
	)

	protoReq := &harnesspb.LLMStreamRequest{
		Slot:     slot,
		Messages: h.messagesToProto(messages),
	}

	stream, err := h.client.LLMStream(ctx, protoReq)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		span.End()
		return nil, fmt.Errorf("LLM stream callback failed: %w", err)
	}

	// Create output channel
	chunkChan := make(chan llm.StreamChunk, 10)

	// Spawn goroutine to receive stream chunks
	go func() {
		defer close(chunkChan)
		defer span.End()

		for {
			protoChunk, err := stream.Recv()
			if err != nil {
				// Stream ended (could be EOF or error)
				if err.Error() != "EOF" {
					span.RecordError(err)
					span.SetStatus(codes.Error, err.Error())
				}
				return
			}

			if protoChunk.Error != nil {
				h.logger.Error("stream chunk error", "error", protoChunk.Error.Message)
				err := fmt.Errorf("stream chunk error: %s", protoChunk.Error.Message)
				span.RecordError(err)
				span.SetStatus(codes.Error, protoChunk.Error.Message)
				return
			}

			chunk := llm.StreamChunk{
				Delta:        protoChunk.Delta,
				ToolCalls:    h.toolCallsFromProto(protoChunk.ToolCalls),
				FinishReason: protoChunk.FinishReason,
			}

			if protoChunk.Usage != nil {
				usage := llm.TokenUsage{
					InputTokens:  int(protoChunk.Usage.InputTokens),
					OutputTokens: int(protoChunk.Usage.OutputTokens),
					TotalTokens:  int(protoChunk.Usage.TotalTokens),
				}
				chunk.Usage = &usage

				// Track token usage on final chunk
				if chunk.FinishReason != "" {
					h.tokenTracker.Add(slot, usage)
					// Record final token usage in span
					span.SetAttributes(
						attribute.Int("gen_ai.usage.input_tokens", usage.InputTokens),
						attribute.Int("gen_ai.usage.output_tokens", usage.OutputTokens),
						attribute.String("gen_ai.response.finish_reason", chunk.FinishReason),
					)
				}
			}

			select {
			case chunkChan <- chunk:
			case <-ctx.Done():
				return
			}
		}
	}()

	return chunkChan, nil
}

// ============================================================================
// Tool Operations
// ============================================================================

// CallToolProto invokes a tool using proto messages via the CallToolProto RPC.
// This is the canonical way to execute tools - all tool calls should use this method.
func (h *CallbackHarness) CallToolProto(ctx context.Context, name string, request protolib.Message, response protolib.Message) error {
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

	if resp.Error != nil {
		err := fmt.Errorf("CallToolProto error: %s", resp.Error.Message)
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
func (h *CallbackHarness) CallToolProtoStream(ctx context.Context, toolName string, input protolib.Message, output protolib.Message, callback agent.ToolStreamCallback) error {
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
			callbackErr := fmt.Errorf("%s", ev.GetError().GetMessage())
			callback.OnError(callbackErr, ev.GetFatal())
			if ev.GetFatal() {
				span.RecordError(callbackErr)
				span.SetStatus(codes.Error, ev.GetError().GetMessage())
				return callbackErr
			}

		case *harnesspb.CallToolProtoStreamResponse_Complete:
			ev := payload.Complete
			if len(ev.GetOutputJson()) > 0 {
				if err := unmarshaler.Unmarshal(ev.GetOutputJson(), output); err != nil {
					span.RecordError(err)
					span.SetStatus(codes.Error, "failed to unmarshal complete output")
					return fmt.Errorf("failed to unmarshal streaming tool output: %w", err)
				}
			}
		}
	}

	span.SetStatus(codes.Ok, "")
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

// ============================================================================
// Plugin Operations
// ============================================================================

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

// ============================================================================
// Agent Delegation Operations
// ============================================================================

// DelegateToAgent assigns a task to another agent for execution.
func (h *CallbackHarness) DelegateToAgent(ctx context.Context, name string, task agent.Task) (agent.Result, error) {
	protoReq := &harnesspb.DelegateToAgentRequest{
		Name: name,
		Task: TaskToProto(task),
	}

	resp, err := h.client.DelegateToAgent(ctx, protoReq)
	if err != nil {
		return agent.Result{}, fmt.Errorf("delegate to agent callback failed: %w", err)
	}

	if resp.Error != nil {
		return agent.Result{}, fmt.Errorf("delegate to agent error: %s", resp.Error.Message)
	}

	// Convert proto result to SDK result using the helper function
	result := ProtoToResult(resp.Result)

	// Convert error if present
	if resp.Result.Error != nil {
		// Convert map[string]string to map[string]any
		details := make(map[string]any)
		for k, v := range resp.Result.Error.Details {
			details[k] = v
		}

		result.Error = &agent.ResultError{
			Code:      resp.Result.Error.Code.String(),
			Message:   resp.Result.Error.Message,
			Details:   details,
			Retryable: resp.Result.Error.Retryable,
		}
	}

	return result, nil
}

// ListAgents returns descriptors for all available agents.
// Results are cached per task execution.
func (h *CallbackHarness) ListAgents(ctx context.Context) ([]agent.Descriptor, error) {
	// Check cache first
	h.cacheMu.RLock()
	if h.agentsCache != nil {
		defer h.cacheMu.RUnlock()
		return h.agentsCache, nil
	}
	h.cacheMu.RUnlock()

	// Fetch from orchestrator
	protoReq := &harnesspb.ListAgentsRequest{}
	resp, err := h.client.ListAgents(ctx, protoReq)
	if err != nil {
		return nil, fmt.Errorf("list agents callback failed: %w", err)
	}

	if resp.Error != nil {
		return nil, fmt.Errorf("list agents error: %s", resp.Error.Message)
	}

	// Convert to agent.Descriptor
	agents := make([]agent.Descriptor, len(resp.Agents))
	for i, protoAgent := range resp.Agents {
		// Proto returns strings, which match the new agent interface
		agents[i] = agent.Descriptor{
			Name:           protoAgent.Name,
			Version:        protoAgent.Version,
			Description:    protoAgent.Description,
			Capabilities:   protoAgent.Capabilities,
			TargetTypes:    protoAgent.TargetTypes,
			TechniqueTypes: protoAgent.TechniqueTypes,
		}
	}

	// Cache results
	h.cacheMu.Lock()
	h.agentsCache = agents
	h.cacheMu.Unlock()

	return agents, nil
}

// ============================================================================
// Finding Operations
// ============================================================================

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
		Finding: FindingToProto(f),
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

// ============================================================================
// GraphRAG Query Operations
// ============================================================================

// QueryGraphRAG performs a semantic or hybrid query against the knowledge graph.
// Uses auto-routing to select semantic or structured query based on the Query fields.

// ============================================================================
// GraphRAG Storage Operations
// ============================================================================

// ============================================================================
// Planning Operations
// ============================================================================

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
			Confidence:    hints.Confidence(),
			SuggestedNext: hints.SuggestedNext(),
			ReplanReason:  hints.ReplanReason(),
			KeyFindings:   hints.KeyFindings(),
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

// ============================================================================
// Helper Methods for Proto Conversions
// ============================================================================

func (h *CallbackHarness) messagesToProto(messages []llm.Message) []*harnesspb.LLMMessage {
	protoMessages := make([]*harnesspb.LLMMessage, len(messages))
	for i, msg := range messages {
		protoMessages[i] = &harnesspb.LLMMessage{
			Role:        string(msg.Role),
			Content:     msg.Content,
			ToolCalls:   h.toolCallsToProto(msg.ToolCalls),
			ToolResults: h.toolResultsToProto(msg.ToolResults),
			Name:        msg.Name,
		}
	}
	return protoMessages
}

func (h *CallbackHarness) toolCallsToProto(calls []llm.ToolCall) []*harnesspb.ToolCall {
	protoCalls := make([]*harnesspb.ToolCall, len(calls))
	for i, call := range calls {
		protoCalls[i] = &harnesspb.ToolCall{
			Id:        call.ID,
			Name:      call.Name,
			Arguments: call.Arguments,
		}
	}
	return protoCalls
}

func (h *CallbackHarness) toolCallsFromProto(calls []*harnesspb.ToolCall) []llm.ToolCall {
	toolCalls := make([]llm.ToolCall, len(calls))
	for i, call := range calls {
		toolCalls[i] = llm.ToolCall{
			ID:        call.Id,
			Name:      call.Name,
			Arguments: call.Arguments,
		}
	}
	return toolCalls
}

func (h *CallbackHarness) toolResultsToProto(results []llm.ToolResult) []*harnesspb.ToolResult {
	protoResults := make([]*harnesspb.ToolResult, len(results))
	for i, result := range results {
		protoResults[i] = &harnesspb.ToolResult{
			ToolCallId: result.ToolCallID,
			Content:    result.Content,
			IsError:    result.IsError,
		}
	}
	return protoResults
}

func (h *CallbackHarness) toolDefsToProto(tools []llm.ToolDef) []*harnesspb.ToolDef {
	protoTools := make([]*harnesspb.ToolDef, len(tools))
	for i, tool := range tools {
		// Convert parameters to JSONSchemaNode
		// Parameters is map[string]any representing a JSON schema
		protoTools[i] = &harnesspb.ToolDef{
			Name:        tool.Name,
			Description: tool.Description,
			Parameters:  JSONSchemaToProtoNode(tool.Parameters),
		}
	}
	return protoTools
}

// formatMessagesForPrompt formats LLM messages into a readable prompt string
// for observability in traces.
func formatMessagesForPrompt(messages []llm.Message) string {
	if len(messages) == 0 {
		return ""
	}

	var result string
	for i, msg := range messages {
		if i > 0 {
			result += "\n---\n"
		}
		result += fmt.Sprintf("[%s]: %s", msg.Role, msg.Content)
	}
	return result
}

// ============================================================================
// MissionManager Methods
// ============================================================================

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

	resp, err := h.client.CreateMission(ctx, req)
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
		Context:  contextInfo,
		TargetId: targetID,
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

		if opts.Constraints != nil {
			req.Constraints = &harnesspb.MissionConstraints{
				MaxDurationMs: opts.Constraints.MaxDuration.Milliseconds(),
				MaxTokens:     opts.Constraints.MaxTokens,
				MaxCost:       opts.Constraints.MaxCost,
				MaxFindings:   int32(opts.Constraints.MaxFindings),
			}
		}

		if opts.Metadata != nil {
			req.Metadata = ToTypedMap(opts.Metadata)
		}
	}

	return req, nil
}

// RunMission queues a mission for execution.
func (h *CallbackHarness) RunMission(ctx context.Context, missionID string, opts *mission.RunMissionOpts) error {
	ctx, span := h.tracer.Start(ctx, "CallbackHarness.RunMission")
	defer span.End()

	req := &harnesspb.RunMissionRequest{
		Context:   h.client.contextInfo(),
		MissionId: missionID,
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

	if filter != nil {
		req.Filter = &harnesspb.MissionFilter{
			Tags:   filter.Tags,
			Limit:  int32(filter.Limit),
			Offset: int32(filter.Offset),
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

	missions := make([]*mission.MissionInfo, len(resp.Missions))
	for i, m := range resp.Missions {
		missions[i] = protoToMissionInfo(m)
	}

	return missions, nil
}

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

// ============================================================================
// Mission Proto Conversion Helpers
// ============================================================================

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
	case harnesspb.MissionStatus_MISSION_STATUS_PENDING:
		return mission.MissionStatusPending
	case harnesspb.MissionStatus_MISSION_STATUS_RUNNING:
		return mission.MissionStatusRunning
	case harnesspb.MissionStatus_MISSION_STATUS_PAUSED:
		return mission.MissionStatusPaused
	case harnesspb.MissionStatus_MISSION_STATUS_COMPLETED:
		return mission.MissionStatusCompleted
	case harnesspb.MissionStatus_MISSION_STATUS_FAILED:
		return mission.MissionStatusFailed
	case harnesspb.MissionStatus_MISSION_STATUS_CANCELLED:
		return mission.MissionStatusCancelled
	default:
		return mission.MissionStatusPending
	}
}

// missionStatusToProto converts SDK mission.MissionStatus to proto MissionStatus.
func missionStatusToProto(s mission.MissionStatus) harnesspb.MissionStatus {
	switch s {
	case mission.MissionStatusPending:
		return harnesspb.MissionStatus_MISSION_STATUS_PENDING
	case mission.MissionStatusRunning:
		return harnesspb.MissionStatus_MISSION_STATUS_RUNNING
	case mission.MissionStatusPaused:
		return harnesspb.MissionStatus_MISSION_STATUS_PAUSED
	case mission.MissionStatusCompleted:
		return harnesspb.MissionStatus_MISSION_STATUS_COMPLETED
	case mission.MissionStatusFailed:
		return harnesspb.MissionStatus_MISSION_STATUS_FAILED
	case mission.MissionStatusCancelled:
		return harnesspb.MissionStatus_MISSION_STATUS_CANCELLED
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

// ============================================================================
// Taxonomy Operations
// ============================================================================

// Taxonomy returns the taxonomy adapter for this harness.
// Returns nil if taxonomy was not successfully initialized.
func (h *CallbackHarness) Taxonomy() *TaxonomyAdapter {
	return h.taxonomy
}

// HasTaxonomy returns true if taxonomy is available for this harness.
func (h *CallbackHarness) HasTaxonomy() bool {
	return h.taxonomy != nil
}

// GenerateNodeID generates a deterministic node ID using taxonomy templates.
// Calls the orchestrator's GenerateNodeID RPC method.
func (h *CallbackHarness) GenerateNodeID(ctx context.Context, nodeType string, properties map[string]any) (string, error) {
	req := &harnesspb.GenerateNodeIDRequest{
		NodeType:   nodeType,
		Properties: ToTypedMap(properties),
	}

	resp, err := h.client.GenerateNodeID(ctx, req)
	if err != nil {
		return "", fmt.Errorf("GenerateNodeID callback failed: %w", err)
	}

	if resp.Error != nil {
		return "", fmt.Errorf("GenerateNodeID error: %s", resp.Error.Message)
	}

	return resp.NodeId, nil
}

// ValidationResult represents the result of a taxonomy validation.
type ValidationResult struct {
	Valid    bool
	Errors   []ValidationError
	Warnings []string
}

// ValidationError represents a single validation error.
type ValidationError struct {
	Field   string
	Message string
	Code    string
}

// ValidateFinding validates a finding against the taxonomy schema.
func (h *CallbackHarness) ValidateFinding(ctx context.Context, f *finding.Finding) (*ValidationResult, error) {
	req := &harnesspb.ValidateFindingRequest{
		Finding: FindingToProto(f),
	}

	resp, err := h.client.ValidateFinding(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("ValidateFinding callback failed: %w", err)
	}

	if resp.Error != nil {
		return nil, fmt.Errorf("ValidateFinding error: %s", resp.Error.Message)
	}

	return convertValidationFields(resp.Valid, resp.Errors, resp.Warnings), nil
}

// ValidateGraphNode validates a graph node against the taxonomy schema.
func (h *CallbackHarness) ValidateGraphNode(ctx context.Context, nodeType string, properties map[string]any) (*ValidationResult, error) {
	req := &harnesspb.ValidateGraphNodeRequest{
		NodeType:   nodeType,
		Properties: ToTypedMap(properties),
	}

	resp, err := h.client.ValidateGraphNode(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("ValidateGraphNode callback failed: %w", err)
	}

	if resp.Error != nil {
		return nil, fmt.Errorf("ValidateGraphNode error: %s", resp.Error.Message)
	}

	return convertValidationFields(resp.Valid, resp.Errors, resp.Warnings), nil
}

// ValidateRelationship validates a relationship against the taxonomy schema.
func (h *CallbackHarness) ValidateRelationship(ctx context.Context, relType string, fromNodeType string, toNodeType string, properties map[string]any) (*ValidationResult, error) {
	req := &harnesspb.ValidateRelationshipRequest{
		RelationshipType: relType,
		FromNodeType:     fromNodeType,
		ToNodeType:       toNodeType,
		Properties:       ToTypedMap(properties),
	}

	resp, err := h.client.ValidateRelationship(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("ValidateRelationship callback failed: %w", err)
	}

	if resp.Error != nil {
		return nil, fmt.Errorf("ValidateRelationship error: %s", resp.Error.Message)
	}

	return convertValidationFields(resp.Valid, resp.Errors, resp.Warnings), nil
}

// convertValidationFields converts proto validation fields to ValidationResult.
func convertValidationFields(valid bool, errors []*harnesspb.ValidationError, warnings []string) *ValidationResult {
	result := &ValidationResult{
		Valid:    valid,
		Warnings: warnings,
	}

	for _, e := range errors {
		result.Errors = append(result.Errors, ValidationError{
			Field:   e.Field,
			Message: e.Message,
			Code:    e.Code,
		})
	}

	return result
}

// ============================================================================
// Tool Work Queue Operations
// ============================================================================

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

// ============================================================================
// Workspace Operations
// ============================================================================

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

// ============================================================================
// Authorization Methods
// ============================================================================

// SetAuthzContext stores the mission run ID from the work envelope's AuthzContext.
// This is called by the SDK serve loop after verifying the HMAC signature on the
// envelope. The run ID is forwarded in every subsequent Authorize call so the
// daemon can resolve the executing user and tenant.
//
// failOpen controls behavior when the daemon's Authorize RPC is unreachable:
//   - false (default): fail-closed — treat Unavailable as deny.
//   - true (dev mode): fail-open — log WARN and allow.
func (h *CallbackHarness) SetAuthzContext(runID string, failOpen bool) {
	h.authzRunID = runID
	h.failOpenAuthz = failOpen
}

// Authorize implements agent.Harness by forwarding the authorization check to
// the daemon's HarnessCallbackService.Authorize RPC.
//
// Error behavior:
//   - nil                       → allowed; proceed
//   - ErrUnauthorized           → FGA denied; caller must refuse the operation
//   - ErrAuthzServiceUnavailable → daemon/FGA unreachable; fail-closed by default
//   - ErrInvalidAction          → action or resource is empty/malformed
//
// Graceful degradation: if the daemon does not implement the Authorize RPC
// (codes.Unimplemented), Authorize logs DEBUG and returns nil to preserve
// rolling-upgrade compatibility with older daemon versions.
func (h *CallbackHarness) Authorize(ctx context.Context, action, resource string) error {
	if action == "" || resource == "" {
		return harnessconst.ErrInvalidAction
	}

	runID := h.authzRunID
	if runID == "" {
		// No authz context set — work was dequeued without an AuthzContext (dev mode).
		// Log and allow to avoid breaking components in development environments.
		h.logger.Debug("Authorize called without AuthzContext run_id; allowing (dev mode)",
			"action", action, "resource", resource)
		return nil
	}

	req := &harnesspb.AuthorizeRequest{
		RunId:    runID,
		Action:   action,
		Resource: resource,
	}

	resp, err := h.client.Authorize(ctx, req)
	if err != nil {
		st, ok := status.FromError(err)
		if !ok {
			// Non-gRPC error (connection issue).
			return h.handleAuthzUnavailable(ctx, action, resource, err)
		}

		switch st.Code() {
		case grpccodes.Unimplemented:
			// Old daemon without this RPC — graceful degrade to allow.
			h.logger.Debug("daemon does not support Authorize RPC; defaulting to allow",
				"action", action, "resource", resource)
			return nil

		case grpccodes.Unavailable, grpccodes.DeadlineExceeded:
			return h.handleAuthzUnavailable(ctx, action, resource, err)

		case grpccodes.NotFound:
			// run_id not found in mission authz store — treat as deny.
			h.logger.Warn("Authorize: run_id not found in daemon mission store",
				"run_id", runID, "action", action, "resource", resource)
			return harnessconst.ErrUnauthorized

		case grpccodes.FailedPrecondition:
			// Mission is no longer active.
			h.logger.Warn("Authorize: mission is no longer active",
				"run_id", runID, "action", action, "resource", resource)
			return harnessconst.ErrUnauthorized

		default:
			return fmt.Errorf("Authorize RPC error: %w", errors.New(st.Message()))
		}
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

// handleAuthzUnavailable applies the fail-open / fail-closed policy when the
// daemon's Authorize RPC is unreachable. Returns ErrAuthzServiceUnavailable on
// fail-closed (default), or nil on fail-open (dev mode, with a WARN log).
func (h *CallbackHarness) handleAuthzUnavailable(ctx context.Context, action, resource string, cause error) error {
	if h.failOpenAuthz {
		h.logger.WarnContext(ctx, "authorization service unavailable — proceeding (fail-open mode)",
			"action", action, "resource", resource, "error", cause)
		return nil
	}
	h.logger.ErrorContext(ctx, "authorization service unavailable — denying (fail-closed)",
		"action", action, "resource", resource, "error", cause)
	return harnessconst.ErrAuthzServiceUnavailable
}

// ── KnowledgeReader ─────────────────────────────────────────────────────────
//
// The dispatched agent's read of what earlier work established. These reach the
// daemon over the callback service, which resolves tenant from the task context
// — no method takes a tenant, so another tenant's graph is unrepresentable
// rather than merely refused.
//
// Read-only by construction: the projector is the sole graph writer (ADR-0012),
// and an agent contributes by emitting (SubmitFinding, Observe), never by
// writing here.

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

// FindSimilarAttacks returns attack patterns semantically close to content for a dispatched agent.
func (h *CallbackHarness) FindSimilarAttacks(ctx context.Context, content string, topK int) ([]*graphragpb.AttackPattern, error) {
	resp, err := h.client.FindSimilarAttacks(ctx, &harnesspb.FindSimilarAttacksRequest{
		Content: content, TopK: boundedInt32(topK),
	})
	if err != nil {
		return nil, fmt.Errorf("find similar attacks callback failed: %w", err)
	}
	if e := resp.GetError(); e != nil && e.GetMessage() != "" {
		return nil, fmt.Errorf("find similar attacks rejected: %s", e.GetMessage())
	}
	return resp.GetResults(), nil
}

// GetAttackChains returns multi-hop technique paths from a starting technique for a dispatched agent.
func (h *CallbackHarness) GetAttackChains(ctx context.Context, techniqueID string, maxDepth int) ([]*graphragpb.AttackChain, error) {
	resp, err := h.client.GetAttackChains(ctx, &harnesspb.GetAttackChainsRequest{
		TechniqueId: techniqueID, MaxDepth: boundedInt32(maxDepth),
	})
	if err != nil {
		return nil, fmt.Errorf("get attack chains callback failed: %w", err)
	}
	if e := resp.GetError(); e != nil && e.GetMessage() != "" {
		return nil, fmt.Errorf("get attack chains rejected: %s", e.GetMessage())
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
	return applicationFindingsFromProto(resp.GetFindings()), nil
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
			FindingID:       pf.GetFindingId(),
			Status:          pf.GetStatus(),
			Severity:        pf.GetSeverity(),
			VulnerabilityID: pf.GetVulnerabilityId(),
			PlaceLabel:      pf.GetPlaceLabel(),
			PlaceKey:        pf.GetPlaceKey(),
			Reachable:       pf.GetReachable(),
			Exposed:         pf.GetExposed(),
			DeploymentKey:   pf.GetDeploymentKey(),
			ImageKey:        pf.GetImageKey(),
			Priority:        pf.GetPriority(),
			PriorityRule:    pf.GetPriorityRule(),
			PriorityReason:  pf.GetPriorityReason(),
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
