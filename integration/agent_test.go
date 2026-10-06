// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Zero Root AI

package integration

import (
	"context"
	"errors"
	"log/slog"
	"time"

	"github.com/zeroroot-ai/sdk/agent"
	graphragpb "github.com/zeroroot-ai/sdk/api/gen/gibson/graphrag/v1"
	"github.com/zeroroot-ai/sdk/codegen/workspace"
	"github.com/zeroroot-ai/sdk/finding"
	"github.com/zeroroot-ai/sdk/graphrag"
	"github.com/zeroroot-ai/sdk/llm"
	"github.com/zeroroot-ai/sdk/mission"
	"github.com/zeroroot-ai/sdk/planning"
	"github.com/zeroroot-ai/sdk/plugin"
	"github.com/zeroroot-ai/sdk/tool"
	"github.com/zeroroot-ai/sdk/types"
	"go.opentelemetry.io/otel/trace"
	protolib "google.golang.org/protobuf/proto"
)

// TODO: consolidate mock harness implementations into a shared testutil package.
// Currently duplicated in: agent/harness_test.go,
// integration/agent_test.go. Each new Harness interface method requires updating all copies.

// compile-time interface check
var _ agent.Harness = (*mockHarness)(nil)

// mockHarness is a minimal mock implementation of agent.Harness for testing.
type mockHarness struct {
	agent.BaseHarness // absorbs interface growth; see its doc. Explicit methods below still win.
}

func (m *mockHarness) Complete(ctx context.Context, slot string, messages []llm.Message, opts ...llm.CompletionOption) (*llm.CompletionResponse, error) {
	return nil, errors.New("not implemented")
}

func (m *mockHarness) CompleteWithTools(ctx context.Context, slot string, messages []llm.Message, tools []llm.ToolDef) (*llm.CompletionResponse, error) {
	return nil, errors.New("not implemented")
}

func (m *mockHarness) CompleteStructured(ctx context.Context, slot string, messages []llm.Message, schema any) (any, error) {
	return nil, errors.New("not implemented")
}

func (m *mockHarness) CompleteStructuredAny(ctx context.Context, slot string, messages []llm.Message, schema any) (any, error) {
	return m.CompleteStructured(ctx, slot, messages, schema)
}

func (m *mockHarness) Stream(ctx context.Context, slot string, messages []llm.Message) (<-chan llm.StreamChunk, error) {
	return nil, errors.New("not implemented")
}

func (m *mockHarness) CallToolProto(ctx context.Context, name string, request protolib.Message, response protolib.Message) error {
	return errors.New("not implemented")
}

func (m *mockHarness) CallToolProtoStream(ctx context.Context, toolName string, input protolib.Message, output protolib.Message, callback agent.ToolStreamCallback) error {
	return errors.New("not implemented")
}

func (m *mockHarness) QueueToolWork(ctx context.Context, toolName string, inputs []protolib.Message) (string, error) {
	return "", errors.New("not implemented")
}

func (m *mockHarness) ToolResults(ctx context.Context, jobID string) <-chan agent.QueuedToolResult {
	return nil
}

func (m *mockHarness) ListTools(ctx context.Context) ([]tool.Descriptor, error) {
	return nil, errors.New("not implemented")
}

func (m *mockHarness) QueryPlugin(ctx context.Context, name string, method string, params map[string]any) (any, error) {
	return nil, errors.New("not implemented")
}

func (m *mockHarness) ListPlugins(ctx context.Context) ([]plugin.Descriptor, error) {
	return nil, errors.New("not implemented")
}

func (m *mockHarness) DelegateToAgent(ctx context.Context, name string, task agent.Task) (agent.Result, error) {
	return agent.Result{}, errors.New("not implemented")
}

func (m *mockHarness) ListAgents(ctx context.Context) ([]agent.Descriptor, error) {
	return nil, errors.New("not implemented")
}

func (m *mockHarness) SubmitFinding(ctx context.Context, f *finding.Finding) error {
	return errors.New("not implemented")
}

func (m *mockHarness) GetFindings(ctx context.Context, filter finding.Filter) ([]*finding.Finding, error) {
	return nil, errors.New("not implemented")
}

func (m *mockHarness) Mission() types.MissionContext {
	return types.MissionContext{}
}

func (m *mockHarness) Target() types.TargetInfo {
	return types.TargetInfo{}
}

func (m *mockHarness) Tracer() trace.Tracer {
	return nil
}

func (m *mockHarness) Logger() *slog.Logger {
	return slog.Default()
}

func (m *mockHarness) TokenUsage() llm.TokenTracker {
	return nil
}

// GraphRAG methods (required by Harness interface)
func (m *mockHarness) QueryNodes(ctx context.Context, query *graphragpb.GraphQuery) ([]*graphragpb.QueryResult, error) {
	return nil, errors.New("not implemented")
}

func (m *mockHarness) Observe(ctx context.Context, obs agent.Observation) error { return nil }

func (m *mockHarness) WorldView(ctx context.Context, focus ...agent.Handle) (agent.WorldView, error) {
	return agent.WorldView{}, nil
}

func (m *mockHarness) StoreGraphNode(ctx context.Context, node graphrag.GraphNode) (string, error) {
	return "", errors.New("not implemented")
}

func (m *mockHarness) StoreSemantic(ctx context.Context, node graphrag.GraphNode) (string, error) {
	return "", errors.New("not implemented")
}

func (m *mockHarness) StoreStructured(ctx context.Context, node graphrag.GraphNode) (string, error) {
	return "", errors.New("not implemented")
}

func (m *mockHarness) GraphRAGHealth(ctx context.Context) types.HealthStatus {
	return types.NewHealthyStatus("ok")
}

func (m *mockHarness) PlanContext() planning.PlanningContext {
	return nil
}

func (m *mockHarness) ReportStepHints(ctx context.Context, hints *planning.StepHints) error {
	return nil
}

// Mission Execution Context methods - stubs for testing
func (m *mockHarness) MissionExecutionContext() types.MissionExecutionContext {
	return types.MissionExecutionContext{}
}

func (m *mockHarness) GetMissionRunHistory(ctx context.Context) ([]types.MissionRunSummary, error) {
	return []types.MissionRunSummary{}, nil
}

func (m *mockHarness) GetPreviousRunFindings(ctx context.Context, filter finding.Filter) ([]*finding.Finding, error) {
	return []*finding.Finding{}, nil
}

func (m *mockHarness) GetAllRunFindings(ctx context.Context, filter finding.Filter) ([]*finding.Finding, error) {
	return []*finding.Finding{}, nil
}

// MissionManager methods - stubs for testing
func (m *mockHarness) CreateMission(ctx context.Context, missionDef any, targetID string, opts *mission.CreateMissionOpts) (*mission.MissionInfo, error) {
	return &mission.MissionInfo{
		ID:       "mock-mission-id",
		Name:     "mock-mission",
		Status:   mission.MissionStatusPending,
		TargetID: targetID,
	}, nil
}

func (m *mockHarness) RunMission(ctx context.Context, missionID string, opts *mission.RunMissionOpts) error {
	return nil
}

func (m *mockHarness) GetMissionStatus(ctx context.Context, missionID string) (*mission.MissionStatusInfo, error) {
	return &mission.MissionStatusInfo{
		Status:   mission.MissionStatusRunning,
		Progress: 0.5,
	}, nil
}

func (m *mockHarness) WaitForMission(ctx context.Context, missionID string, timeout time.Duration) (*mission.MissionResult, error) {
	return &mission.MissionResult{
		MissionID: missionID,
		Status:    mission.MissionStatusCompleted,
	}, nil
}

func (m *mockHarness) ListMissions(ctx context.Context, filter *mission.MissionFilter) ([]*mission.MissionInfo, error) {
	return []*mission.MissionInfo{}, nil
}

func (m *mockHarness) CancelMission(ctx context.Context, missionID string) error {
	return nil
}

func (m *mockHarness) GetMissionResults(ctx context.Context, missionID string) (*mission.MissionResult, error) {
	return &mission.MissionResult{
		MissionID: missionID,
		Status:    mission.MissionStatusCompleted,
	}, nil
}

// Workspace returns nil (no workspace in mock)
func (m *mockHarness) Workspace() workspace.Workspace {
	return nil
}

// Workspaces returns empty map (no workspaces in mock)
func (m *mockHarness) Workspaces() map[string]workspace.Workspace {
	return make(map[string]workspace.Workspace)
}

func (m *mockHarness) Authorize(ctx context.Context, action, resource string) error {
	return nil
}

// TaxonomyRegistry returns nil (no taxonomy introspection in mock)
func (m *mockHarness) TaxonomyRegistry() graphrag.TaxonomyIntrospector {
	return nil
}
