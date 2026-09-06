// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Zero Root AI

package serve

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"go.opentelemetry.io/otel/sdk/trace"
	otelTrace "go.opentelemetry.io/otel/trace"
	"go.opentelemetry.io/otel/trace/noop"
	"google.golang.org/grpc/codes"
	grpcCreds "google.golang.org/grpc/credentials"
	"google.golang.org/grpc/status"

	"github.com/zeroroot-ai/sdk/agent"
	agentpb "github.com/zeroroot-ai/sdk/api/gen/gibson/agent/v1"
	commonpb "github.com/zeroroot-ai/sdk/api/gen/gibson/common/v1"
)

// Agent connects an agent to the Gibson platform in pull mode.
// It performs the Capability Grant bootstrap (discover → register), dials the platform,
// and polls for work until a shutdown signal is received or an error occurs.
//
// WithCapabilityGrant or WithCapabilityGrantFromEnv must be provided; Agent returns an error
// if PlatformURL is not configured.
//
// Example:
//
//	err := serve.Agent(myAgent, serve.WithCapabilityGrantFromEnv())
//	if err != nil {
//	    log.Fatal(err)
//	}
func Agent(a agent.Agent, opts ...Option) error {
	cfg := DefaultConfig()
	for _, opt := range opts {
		opt(cfg)
	}

	if err := validateConfig(cfg); err != nil {
		return err
	}
	slog.Info("starting agent", "name", a.Name(), "platform_url", cfg.PlatformURL)
	return servePlatformAgent(a, cfg)
}

// agentServiceServer implements the gRPC AgentService for an SDK agent.
// It bridges the gRPC protocol to the agent.Agent interface.
type agentServiceServer struct {
	agentpb.UnimplementedAgentServiceServer
	agent agent.Agent

	// callbackTransportCreds, when set, are used as the gRPC transport credentials
	// for the CallbackClient that the Execute handler dials at req.CallbackEndpoint.
	// In SPIFFE mode this carries MTLSClientCredentials built from the agent's
	// X509 source (see platform_serve.go's servePlatformAgent) so the callback dial
	// against the daemon's now-mTLS-only harness callback listener succeeds at
	// handshake. When nil the CallbackClient falls back to its default transport
	// (insecure) — only safe for non-SPIFFE / loopback dev builds.
	// Spec: critical-tls-no-fallbacks Requirement 6.4.
	callbackTransportCreds grpcCreds.TransportCredentials
}

// GetDescriptor returns the agent's descriptor including name, version,
// capabilities, target types, and technique types.
func (s *agentServiceServer) GetDescriptor(ctx context.Context, req *agentpb.GetDescriptorRequest) (*agentpb.GetDescriptorResponse, error) {
	// Agent methods now return []string directly
	capabilities := s.agent.Capabilities()
	targetTypes := s.agent.TargetTypes()
	techniqueTypes := s.agent.TechniqueTypes()

	return &agentpb.GetDescriptorResponse{
		Name:           s.agent.Name(),
		Version:        s.agent.Version(),
		Description:    s.agent.Description(),
		Capabilities:   capabilities,
		TargetTypes:    targetTypes,
		TechniqueTypes: techniqueTypes,
	}, nil
}

// GetSlotSchema returns the LLM slot definitions required by the agent.
// Each slot defines requirements and constraints for LLM provisioning.
func (s *agentServiceServer) GetSlotSchema(ctx context.Context, req *agentpb.GetSlotSchemaRequest) (*agentpb.GetSlotSchemaResponse, error) {
	slots := s.agent.LLMSlots()
	protoSlots := make([]*agentpb.AgentSlotDefinition, len(slots))

	for i, slot := range slots {
		// Note: The current SlotDefinition doesn't have DefaultConfig or separate Constraints fields.
		// We map the available fields to the proto structure.
		var constraints *agentpb.AgentSlotConstraints
		if len(slot.RequiredFeatures) > 0 || slot.MinContextWindow > 0 {
			constraints = &agentpb.AgentSlotConstraints{
				MinContextWindow: int32(slot.MinContextWindow),
				RequiredFeatures: slot.RequiredFeatures,
			}
		}

		// Use PreferredModels to suggest a default config if available
		var defaultConfig *agentpb.AgentSlotConfig
		if len(slot.PreferredModels) > 0 {
			defaultConfig = &agentpb.AgentSlotConfig{
				Model: slot.PreferredModels[0], // Use first preferred model as default
			}
		}

		protoSlots[i] = &agentpb.AgentSlotDefinition{
			Name:          slot.Name,
			Description:   slot.Description,
			Required:      slot.Required,
			DefaultConfig: defaultConfig,
			Constraints:   constraints,
		}
	}

	return &agentpb.GetSlotSchemaResponse{
		Slots: protoSlots,
	}, nil
}

// Execute runs the agent with the provided task.
// The task is provided as a typed proto message and the result is
// returned as a typed proto message.
func (s *agentServiceServer) Execute(ctx context.Context, req *agentpb.ExecuteRequest) (*agentpb.ExecuteResponse, error) {
	// Convert proto task to SDK task
	task := ProtoToTask(req.Task)

	// Apply timeout if specified
	if req.TimeoutMs > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, time.Duration(req.TimeoutMs)*time.Millisecond)
		defer cancel()
	}

	// Create harness if callback endpoint is provided
	var harness agent.Harness
	var tracerProvider *trace.TracerProvider
	if req.CallbackEndpoint != "" {
		callbackHarness, tp, err := s.createCallbackHarness(ctx, req, task)
		if err != nil {
			return nil, status.Errorf(codes.Internal, "failed to create callback harness: %v", err)
		}
		harness = callbackHarness
		tracerProvider = tp
		// Clean up callback client and tracer provider when done
		defer func() {
			if ch, ok := harness.(*CallbackHarness); ok && ch.client != nil {
				ch.client.Close()
			}
			if tracerProvider != nil {
				if err := tracerProvider.Shutdown(context.Background()); err != nil {
					slog.Warn("failed to shutdown tracer provider", "error", err)
				}
			}
		}()
	}

	// Execute the agent with the harness (may be nil if no callback endpoint)
	result, err := s.agent.Execute(ctx, harness, task)

	// Build response
	resp := &agentpb.ExecuteResponse{}

	// Convert result to proto
	if err == nil {
		resp.Result = ResultToProto(result)
	} else {
		// Map error to proto error (convert error code to commonpb.ErrorCode)
		errorCode := StringToProtoErrorCode(agent.ErrCodeInternalError)
		resp.Error = &commonpb.Error{
			Code:      errorCode.String(),
			Message:   err.Error(),
			Retryable: false,
		}
	}

	return resp, nil
}

// createCallbackHarness creates a CallbackHarness connected to the orchestrator.
func (s *agentServiceServer) createCallbackHarness(ctx context.Context, req *agentpb.ExecuteRequest, task agent.Task) (*CallbackHarness, *trace.TracerProvider, error) {
	// Create callback client options
	var clientOpts []CallbackClientOption
	// SPIFFE transport credentials take precedence — when the agentServiceServer
	// was constructed in SPIFFE mode (servePlatformAgent wires
	// MTLSClientCredentials from the same X509Source the agent uses to dial the
	// daemon), the callback dial reuses them. Without this, an agent in SPIFFE
	// mode would fall back to insecure.NewCredentials() at
	// callback_client.go:138 and fail handshake against the daemon's
	// mTLS-required harness callback listener.
	// Spec: critical-tls-no-fallbacks Requirement 6.4.
	if s.callbackTransportCreds != nil {
		clientOpts = append(clientOpts, WithCallbackTransportCredentials(s.callbackTransportCreds))
	}
	if req.CallbackToken != "" {
		clientOpts = append(clientOpts, WithCallbackToken(req.CallbackToken))
	}

	// Create and connect callback client
	client, err := NewCallbackClient(req.CallbackEndpoint, clientOpts...)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to create callback client: %w", err)
	}

	if err := client.Connect(ctx); err != nil {
		return nil, nil, fmt.Errorf("failed to connect to orchestrator: %w", err)
	}

	// Convert mission context from proto TypedMap
	mission := ProtoToMissionContext(req.Mission)

	// Set full task context for callback requests including mission-scoped storage fields
	// Pass all context explicitly so the callback service can use them directly
	// for mission-based harness lookup and GraphRAG storage
	client.SetFullContext(TaskContextParams{
		TaskID:       task.ID,
		AgentName:    s.agent.Name(),
		MissionID:    mission.ID,
		TraceID:      req.TraceId,
		SpanID:       req.ParentSpanId,
		MissionRunID: req.MissionRunId,
		AgentRunID:   req.AgentRunId,
		RunNumber:    req.RunNumber,
	})

	// Convert target info from proto TypedMap
	target := ProtoToTargetInfo(req.Target)

	// Create logger for this agent execution
	logger := slog.Default().With(
		"agent", s.agent.Name(),
		"task_id", task.ID,
	)

	// Create tracer based on whether trace context is present
	var tracer otelTrace.Tracer
	var tracerProvider *trace.TracerProvider
	if req.TraceId != "" {
		// Create real tracer with proxy exporter
		tracerProvider = NewProxyTracerProvider(client, req.TraceId, req.ParentSpanId, logger)
		tracer = tracerProvider.Tracer("gibson-agent")
		logger.Debug("created real tracer for distributed tracing",
			"trace_id", req.TraceId,
			"parent_span_id", req.ParentSpanId,
		)
	} else {
		// Use no-op tracer when no trace context is provided
		tracer = noop.NewTracerProvider().Tracer("gibson-agent")
		logger.Debug("created no-op tracer (no trace context)")
	}

	// Create the callback harness
	harness := NewCallbackHarness(client,
		WithCallbackLogger(logger),
		WithCallbackTracer(tracer),
		WithCallbackMission(mission),
		WithCallbackTarget(target),
	)

	return harness, tracerProvider, nil
}

// Health returns the current health status of the agent.
func (s *agentServiceServer) Health(ctx context.Context, req *agentpb.HealthRequest) (*agentpb.HealthResponse, error) {
	health := s.agent.Health(ctx)

	return &agentpb.HealthResponse{
		Status: &commonpb.HealthStatus{
			Status:    health.Status,
			Message:   health.Message,
			CheckedAt: time.Now().UnixMilli(),
		},
	}, nil
}
