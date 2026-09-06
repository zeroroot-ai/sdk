// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Zero Root AI

// Package agent provides the core agent abstractions for the Gibson SDK.
//
// BaseHarness provides default implementations for all Harness methods.
// Embed this in custom harness implementations to automatically gain new
// methods without breaking when the interface grows.
//
// Usage:
//
//	type MyHarness struct {
//	    agent.BaseHarness
//	    // custom fields
//	}
//
//	func NewMyHarness(logger *slog.Logger) *MyHarness {
//	    return &MyHarness{
//	        BaseHarness: agent.NewBaseHarness(logger),
//	    }
//	}
//
//	// Override only the methods you need:
//	func (h *MyHarness) Complete(ctx context.Context, slot string, messages []llm.Message, opts ...llm.CompletionOption) (*llm.CompletionResponse, error) {
//	    // custom implementation
//	}
package agent

import (
	"context"
	"errors"
	"log/slog"
	"time"

	"go.opentelemetry.io/otel/trace"
	"google.golang.org/protobuf/proto"

	graphragpb "github.com/zeroroot-ai/sdk/api/gen/gibson/graphrag/v1"

	"github.com/zeroroot-ai/sdk/codegen/workspace"
	"github.com/zeroroot-ai/sdk/finding"
	"github.com/zeroroot-ai/sdk/llm"
	"github.com/zeroroot-ai/sdk/mission"
	"github.com/zeroroot-ai/sdk/planning"
	"github.com/zeroroot-ai/sdk/plugin"
	"github.com/zeroroot-ai/sdk/tool"
	"github.com/zeroroot-ai/sdk/types"
)

// Compile-time assertion: *BaseHarness must satisfy the Harness interface.
var _ Harness = (*BaseHarness)(nil)

// BaseHarness provides default no-op or error-returning implementations for
// every method in the Harness interface. Embed it in a custom harness struct
// so that only the methods relevant to your implementation need to be defined.
// Any method not overridden will return a clear "not implemented" error,
// making incomplete implementations discoverable at runtime rather than at
// compile time.
type BaseHarness struct {
	logger *slog.Logger
}

// NewBaseHarness constructs a BaseHarness with the given logger.
// Pass a nil logger to fall back to the default slog logger.
func NewBaseHarness(logger *slog.Logger) BaseHarness {
	if logger == nil {
		logger = slog.Default()
	}
	return BaseHarness{logger: logger}
}

// ---------------------------------------------------------------------------
// LLM Access Methods
// ---------------------------------------------------------------------------

// Complete returns an error indicating this method is not implemented.
func (b *BaseHarness) Complete(_ context.Context, _ string, _ []llm.Message, _ ...llm.CompletionOption) (*llm.CompletionResponse, error) {
	return nil, errors.New("Complete not implemented in BaseHarness")
}

// CompleteWithTools returns an error indicating this method is not implemented.
func (b *BaseHarness) CompleteWithTools(_ context.Context, _ string, _ []llm.Message, _ []llm.ToolDef) (*llm.CompletionResponse, error) {
	return nil, errors.New("CompleteWithTools not implemented in BaseHarness")
}

// Stream returns an error indicating this method is not implemented.
func (b *BaseHarness) Stream(_ context.Context, _ string, _ []llm.Message) (<-chan llm.StreamChunk, error) {
	return nil, errors.New("Stream not implemented in BaseHarness")
}

// CompleteStructured returns an error indicating this method is not implemented.
func (b *BaseHarness) CompleteStructured(_ context.Context, _ string, _ []llm.Message, _ any) (any, error) {
	return nil, errors.New("CompleteStructured not implemented in BaseHarness")
}

// CompleteStructuredAny returns an error indicating this method is not implemented.
func (b *BaseHarness) CompleteStructuredAny(_ context.Context, _ string, _ []llm.Message, _ any) (any, error) {
	return nil, errors.New("CompleteStructuredAny not implemented in BaseHarness")
}

// ---------------------------------------------------------------------------
// Tool Access Methods
// ---------------------------------------------------------------------------

// CallToolProto returns an error indicating this method is not implemented.
func (b *BaseHarness) CallToolProto(_ context.Context, _ string, _ proto.Message, _ proto.Message) error {
	return errors.New("CallToolProto not implemented in BaseHarness")
}

// CallToolProtoStream returns an error indicating this method is not implemented.
func (b *BaseHarness) CallToolProtoStream(_ context.Context, _ string, _ proto.Message, _ proto.Message, _ ToolStreamCallback) error {
	return errors.New("CallToolProtoStream not implemented in BaseHarness")
}

// ListTools returns an error indicating this method is not implemented.
func (b *BaseHarness) ListTools(_ context.Context) ([]tool.Descriptor, error) {
	return nil, errors.New("ListTools not implemented in BaseHarness")
}

// QueueToolWork returns an error indicating this method is not implemented.
func (b *BaseHarness) QueueToolWork(_ context.Context, _ string, _ []proto.Message) (string, error) {
	return "", errors.New("QueueToolWork not implemented in BaseHarness")
}

// ToolResults returns a closed channel since this method is not implemented.
// Callers that range over the returned channel will exit immediately.
func (b *BaseHarness) ToolResults(_ context.Context, _ string) <-chan QueuedToolResult {
	ch := make(chan QueuedToolResult)
	close(ch)
	return ch
}

// ---------------------------------------------------------------------------
// Plugin Access Methods
// ---------------------------------------------------------------------------

// QueryPlugin returns an error indicating this method is not implemented.
func (b *BaseHarness) QueryPlugin(_ context.Context, _ string, _ string, _ map[string]any) (any, error) {
	return nil, errors.New("QueryPlugin not implemented in BaseHarness")
}

// ListPlugins returns an error indicating this method is not implemented.
func (b *BaseHarness) ListPlugins(_ context.Context) ([]plugin.Descriptor, error) {
	return nil, errors.New("ListPlugins not implemented in BaseHarness")
}

// ---------------------------------------------------------------------------
// Agent Delegation Methods
// ---------------------------------------------------------------------------

// DelegateToAgent returns an error indicating this method is not implemented.
func (b *BaseHarness) DelegateToAgent(_ context.Context, _ string, _ Task) (Result, error) {
	return Result{}, errors.New("DelegateToAgent not implemented in BaseHarness")
}

// ListAgents returns an error indicating this method is not implemented.
func (b *BaseHarness) ListAgents(_ context.Context) ([]Descriptor, error) {
	return nil, errors.New("ListAgents not implemented in BaseHarness")
}

// ---------------------------------------------------------------------------
// Finding Management Methods
// ---------------------------------------------------------------------------

// SubmitFinding returns an error indicating this method is not implemented.
func (b *BaseHarness) SubmitFinding(_ context.Context, _ *finding.Finding) error {
	return errors.New("SubmitFinding not implemented in BaseHarness")
}

// ---------------------------------------------------------------------------
// Context Access
// ---------------------------------------------------------------------------

// Mission returns a zero-value MissionContext. Override to provide real context.
func (b *BaseHarness) Mission() types.MissionContext {
	return types.MissionContext{}
}

// Target returns a zero-value TargetInfo. Override to provide real target info.
func (b *BaseHarness) Target() types.TargetInfo {
	return types.TargetInfo{}
}

// ---------------------------------------------------------------------------
// Observability
// ---------------------------------------------------------------------------

// Tracer returns a no-op tracer. Override to provide a real OTel tracer.
func (b *BaseHarness) Tracer() trace.Tracer {
	return trace.NewNoopTracerProvider().Tracer("")
}

// Logger returns the logger configured on this BaseHarness.
func (b *BaseHarness) Logger() *slog.Logger {
	return b.logger
}

// TokenUsage returns nil. Override to provide a real token tracker.
func (b *BaseHarness) TokenUsage() llm.TokenTracker {
	return nil
}

// ---------------------------------------------------------------------------
// Observation Emit
// ---------------------------------------------------------------------------

// Observe returns an error indicating this method is not implemented.
func (b *BaseHarness) Observe(_ context.Context, _ Observation) error {
	return errors.New("Observe not implemented in BaseHarness")
}

// ---------------------------------------------------------------------------
// World Read
// ---------------------------------------------------------------------------

// WorldView returns an error indicating this method is not implemented. It fails
// rather than returning an empty slice: an empty World and an unwired harness are
// different facts, and an agent that cannot tell them apart would silently plan
// against nothing.
func (b *BaseHarness) WorldView(_ context.Context, _ ...Handle) (WorldView, error) {
	return WorldView{}, errors.New("WorldView not implemented in BaseHarness")
}

// ---------------------------------------------------------------------------
// Planning Context Methods
// ---------------------------------------------------------------------------

// PlanContext returns nil. Override to provide a real planning context.
func (b *BaseHarness) PlanContext() planning.PlanningContext {
	return nil
}

// ReportStepHints is a no-op. Override to report hints to the planning system.
func (b *BaseHarness) ReportStepHints(_ context.Context, _ *planning.StepHints) error {
	return errors.New("ReportStepHints not implemented in BaseHarness")
}

// ---------------------------------------------------------------------------
// MissionManager Methods
// ---------------------------------------------------------------------------

// CreateMission returns an error indicating this method is not implemented.
func (b *BaseHarness) CreateMission(_ context.Context, _ any, _ string, _ *mission.CreateMissionOpts) (*mission.MissionInfo, error) {
	return nil, errors.New("CreateMission not implemented in BaseHarness")
}

// RunMission returns an error indicating this method is not implemented.
func (b *BaseHarness) RunMission(_ context.Context, _ string, _ *mission.RunMissionOpts) error {
	return errors.New("RunMission not implemented in BaseHarness")
}

// GetMissionStatus returns an error indicating this method is not implemented.
func (b *BaseHarness) GetMissionStatus(_ context.Context, _ string) (*mission.MissionStatusInfo, error) {
	return nil, errors.New("GetMissionStatus not implemented in BaseHarness")
}

// WaitForMission returns an error indicating this method is not implemented.
func (b *BaseHarness) WaitForMission(_ context.Context, _ string, _ time.Duration) (*mission.MissionResult, error) {
	return nil, errors.New("WaitForMission not implemented in BaseHarness")
}

// ListMissions returns an error indicating this method is not implemented.
func (b *BaseHarness) ListMissions(_ context.Context, _ *mission.MissionFilter) ([]*mission.MissionInfo, error) {
	return nil, errors.New("ListMissions not implemented in BaseHarness")
}

// CancelMission returns an error indicating this method is not implemented.
func (b *BaseHarness) CancelMission(_ context.Context, _ string) error {
	return errors.New("CancelMission not implemented in BaseHarness")
}

// GetMissionResults returns an error indicating this method is not implemented.
func (b *BaseHarness) GetMissionResults(_ context.Context, _ string) (*mission.MissionResult, error) {
	return nil, errors.New("GetMissionResults not implemented in BaseHarness")
}

// ---------------------------------------------------------------------------
// Workspace Access Methods
// ---------------------------------------------------------------------------

// Workspace returns nil. Override to provide a real workspace.
func (b *BaseHarness) Workspace() workspace.Workspace {
	return nil
}

// Workspaces returns an empty map. Override to provide real workspaces.
func (b *BaseHarness) Workspaces() map[string]workspace.Workspace {
	return map[string]workspace.Workspace{}
}

// ---------------------------------------------------------------------------
// Authorization Methods
// ---------------------------------------------------------------------------

// Authorize is a no-op implementation that always returns nil (allow).
//
// This default is intentional: it allows existing components that have not
// yet been updated to call Authorize to continue functioning without change
// after the SDK upgrade. The daemon's pre-dispatch FGA check remains the
// primary enforcement layer for such components.
//
// Override this method in real harness implementations (e.g., PlatformHarness,
// CallbackHarness) to forward the call to the daemon's Authorize RPC. Use it
// in test harnesses to inject controlled allow/deny behavior.
//
// Do NOT embed BaseHarness in production harness implementations without
// overriding Authorize — doing so silently skips the defense-in-depth check.
func (b *BaseHarness) Authorize(_ context.Context, _, _ string) error {
	return nil
}

// ── KnowledgeReader ─────────────────────────────────────────────────────────
//
// BaseHarness has no platform behind it, so every knowledge read reports
// ErrKnowledgeUnavailable. Note these return the SENTINEL rather than the
// untyped errors.New strings the rest of this file uses: a caller must be able
// to tell "this harness cannot read" from "the graph knows nothing", and only a
// matchable error carries that. Returning nil, nil here would be the silent
// false negative ErrKnowledgeUnavailable exists to prevent.

// QueryNodes reports ErrKnowledgeUnavailable: BaseHarness has no platform behind it.
func (b *BaseHarness) QueryNodes(context.Context, *graphragpb.GraphQuery) ([]*graphragpb.QueryResult, error) {
	return nil, ErrKnowledgeUnavailable
}

// FindSimilarAttacks reports ErrKnowledgeUnavailable: BaseHarness has no platform behind it.
func (b *BaseHarness) FindSimilarAttacks(context.Context, string, int) ([]*graphragpb.AttackPattern, error) {
	return nil, ErrKnowledgeUnavailable
}

// GetAttackChains reports ErrKnowledgeUnavailable: BaseHarness has no platform behind it.
func (b *BaseHarness) GetAttackChains(context.Context, string, int) ([]*graphragpb.AttackChain, error) {
	return nil, ErrKnowledgeUnavailable
}

// FindSimilarFindings reports ErrKnowledgeUnavailable: BaseHarness has no platform behind it.
func (b *BaseHarness) FindSimilarFindings(context.Context, string, int) ([]*graphragpb.FindingNode, error) {
	return nil, ErrKnowledgeUnavailable
}

// GetRelatedFindings reports ErrKnowledgeUnavailable: BaseHarness has no platform behind it.
func (b *BaseHarness) GetRelatedFindings(context.Context, string) ([]*graphragpb.FindingNode, error) {
	return nil, ErrKnowledgeUnavailable
}

// GetFindings reports ErrKnowledgeUnavailable: BaseHarness has no platform behind it.
func (b *BaseHarness) GetFindings(context.Context, finding.Filter) ([]*finding.Finding, error) {
	return nil, ErrKnowledgeUnavailable
}

// GetRunFindings reports ErrKnowledgeUnavailable: BaseHarness has no platform behind it.
func (b *BaseHarness) GetRunFindings(context.Context, RunScope, finding.Filter) ([]*finding.Finding, error) {
	return nil, ErrKnowledgeUnavailable
}

// GetMissionRunHistory reports ErrKnowledgeUnavailable: BaseHarness has no platform behind it.
func (b *BaseHarness) GetMissionRunHistory(context.Context) ([]types.MissionRunSummary, error) {
	return nil, ErrKnowledgeUnavailable
}

// ApplicationFindings reports ErrKnowledgeUnavailable: BaseHarness has no platform behind it.
func (b *BaseHarness) ApplicationFindings(context.Context, string, []string, int) ([]ApplicationFinding, error) {
	return nil, ErrKnowledgeUnavailable
}
