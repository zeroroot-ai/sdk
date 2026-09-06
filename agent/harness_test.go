// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Zero Root AI

package agent

import (
	"context"
	"log/slog"
	"os"
	"testing"
	"time"

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
	"go.opentelemetry.io/otel/trace/noop"
	"google.golang.org/protobuf/proto"
)

// TODO: consolidate mock harness implementations into a shared testutil package.
// Currently duplicated in: agent/harness_test.go, eval/recording_harness_test.go,
// integration/agent_test.go. Each new Harness interface method requires updating all copies.

// compile-time interface check
var _ Harness = (*mockHarness)(nil)

// mockHarness is a test implementation of the Harness interface.
type mockHarness struct {
	BaseHarness // absorbs interface growth; see its doc. The explicit methods below still win.

	completeFunc          func(ctx context.Context, slot string, messages []llm.Message, opts ...llm.CompletionOption) (*llm.CompletionResponse, error)
	completeWithToolsFunc func(ctx context.Context, slot string, messages []llm.Message, tools []llm.ToolDef) (*llm.CompletionResponse, error)
	streamFunc            func(ctx context.Context, slot string, messages []llm.Message) (<-chan llm.StreamChunk, error)
	callToolProtoFunc     func(ctx context.Context, name string, request proto.Message, response proto.Message) error
	listToolsFunc         func(ctx context.Context) ([]tool.Descriptor, error)
	queryPluginFunc       func(ctx context.Context, name string, method string, params map[string]any) (any, error)
	listPluginsFunc       func(ctx context.Context) ([]plugin.Descriptor, error)
	delegateToAgentFunc   func(ctx context.Context, name string, task Task) (Result, error)
	listAgentsFunc        func(ctx context.Context) ([]Descriptor, error)
	submitFindingFunc     func(ctx context.Context, f *finding.Finding) error
	getFindingsFunc       func(ctx context.Context, filter finding.Filter) ([]*finding.Finding, error)
	mission               types.MissionContext
	target                types.TargetInfo
	tracer                trace.Tracer
	logger                *slog.Logger
	tokenUsage            llm.TokenTracker
}

func (m *mockHarness) Complete(ctx context.Context, slot string, messages []llm.Message, opts ...llm.CompletionOption) (*llm.CompletionResponse, error) {
	if m.completeFunc != nil {
		return m.completeFunc(ctx, slot, messages, opts...)
	}
	return &llm.CompletionResponse{
		Content:      "mock response",
		FinishReason: "stop",
	}, nil
}

func (m *mockHarness) CompleteWithTools(ctx context.Context, slot string, messages []llm.Message, tools []llm.ToolDef) (*llm.CompletionResponse, error) {
	if m.completeWithToolsFunc != nil {
		return m.completeWithToolsFunc(ctx, slot, messages, tools)
	}
	return &llm.CompletionResponse{
		Content:      "mock tool response",
		FinishReason: "stop",
	}, nil
}

func (m *mockHarness) Stream(ctx context.Context, slot string, messages []llm.Message) (<-chan llm.StreamChunk, error) {
	if m.streamFunc != nil {
		return m.streamFunc(ctx, slot, messages)
	}
	ch := make(chan llm.StreamChunk, 1)
	ch <- llm.StreamChunk{Delta: "mock stream", FinishReason: "stop"}
	close(ch)
	return ch, nil
}

func (m *mockHarness) CompleteStructured(ctx context.Context, slot string, messages []llm.Message, schema any) (any, error) {
	return map[string]any{"result": "structured"}, nil
}

func (m *mockHarness) CompleteStructuredAny(ctx context.Context, slot string, messages []llm.Message, schema any) (any, error) {
	return m.CompleteStructured(ctx, slot, messages, schema)
}

func (m *mockHarness) CallToolProto(ctx context.Context, name string, request proto.Message, response proto.Message) error {
	if m.callToolProtoFunc != nil {
		return m.callToolProtoFunc(ctx, name, request, response)
	}
	return nil
}

func (m *mockHarness) CallToolProtoStream(ctx context.Context, toolName string, input proto.Message, output proto.Message, callback ToolStreamCallback) error {
	return nil
}

func (m *mockHarness) QueueToolWork(ctx context.Context, toolName string, inputs []proto.Message) (string, error) {
	return "", nil
}

func (m *mockHarness) ToolResults(ctx context.Context, jobID string) <-chan QueuedToolResult {
	ch := make(chan QueuedToolResult)
	close(ch)
	return ch
}

func (m *mockHarness) ListTools(ctx context.Context) ([]tool.Descriptor, error) {
	if m.listToolsFunc != nil {
		return m.listToolsFunc(ctx)
	}
	return []tool.Descriptor{
		{Name: "tool1", Description: "Test tool 1"},
		{Name: "tool2", Description: "Test tool 2"},
	}, nil
}

func (m *mockHarness) QueryPlugin(ctx context.Context, name string, method string, params map[string]any) (any, error) {
	if m.queryPluginFunc != nil {
		return m.queryPluginFunc(ctx, name, method, params)
	}
	return map[string]any{"result": "plugin response"}, nil
}

func (m *mockHarness) ListPlugins(ctx context.Context) ([]plugin.Descriptor, error) {
	if m.listPluginsFunc != nil {
		return m.listPluginsFunc(ctx)
	}
	return []plugin.Descriptor{
		{Name: "plugin1", Description: "Test plugin", Version: "1.0.0"},
	}, nil
}

func (m *mockHarness) DelegateToAgent(ctx context.Context, name string, task Task) (Result, error) {
	if m.delegateToAgentFunc != nil {
		return m.delegateToAgentFunc(ctx, name, task)
	}
	return NewSuccessResult("delegated result"), nil
}

func (m *mockHarness) ListAgents(ctx context.Context) ([]Descriptor, error) {
	if m.listAgentsFunc != nil {
		return m.listAgentsFunc(ctx)
	}
	return []Descriptor{
		{Name: "agent1", Version: "1.0.0", Description: "Test agent"},
	}, nil
}

func (m *mockHarness) SubmitFinding(ctx context.Context, f *finding.Finding) error {
	if m.submitFindingFunc != nil {
		return m.submitFindingFunc(ctx, f)
	}
	return nil
}

func (m *mockHarness) GetFindings(ctx context.Context, filter finding.Filter) ([]*finding.Finding, error) {
	if m.getFindingsFunc != nil {
		return m.getFindingsFunc(ctx, filter)
	}
	return []*finding.Finding{}, nil
}

func (m *mockHarness) Mission() types.MissionContext {
	return m.mission
}

func (m *mockHarness) Target() types.TargetInfo {
	return m.target
}

func (m *mockHarness) Tracer() trace.Tracer {
	if m.tracer != nil {
		return m.tracer
	}
	return noop.NewTracerProvider().Tracer("test")
}

func (m *mockHarness) Logger() *slog.Logger {
	if m.logger != nil {
		return m.logger
	}
	return slog.New(slog.NewTextHandler(os.Stdout, nil))
}

func (m *mockHarness) TokenUsage() llm.TokenTracker {
	if m.tokenUsage != nil {
		return m.tokenUsage
	}
	return llm.NewTokenTracker()
}

// GraphRAG proto methods - stubs for testing
func (m *mockHarness) QueryNodes(ctx context.Context, query *graphragpb.GraphQuery) ([]*graphragpb.QueryResult, error) {
	return nil, nil
}

func (m *mockHarness) Observe(ctx context.Context, obs Observation) error { return nil }

func (m *mockHarness) WorldView(ctx context.Context, focus ...Handle) (WorldView, error) {
	return WorldView{}, nil
}

func (m *mockHarness) StoreGraphNode(ctx context.Context, node graphrag.GraphNode) (string, error) {
	return "", nil
}

func (m *mockHarness) StoreSemantic(ctx context.Context, node graphrag.GraphNode) (string, error) {
	return "", nil
}

func (m *mockHarness) StoreStructured(ctx context.Context, node graphrag.GraphNode) (string, error) {
	return "", nil
}

func (m *mockHarness) CreateGraphRelationship(ctx context.Context, rel graphrag.Relationship) error {
	return nil
}

func (m *mockHarness) StoreGraphBatch(ctx context.Context, batch graphrag.Batch) ([]string, error) {
	return nil, nil
}

func (m *mockHarness) GraphRAGHealth(ctx context.Context) types.HealthStatus {
	return types.NewHealthyStatus("mock healthy")
}

// Planning methods - stubs for testing
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

// Workspace returns the primary workspace (nil for mock)
func (m *mockHarness) Workspace() workspace.Workspace {
	return nil
}

// Workspaces returns all workspaces (empty map for mock)
func (m *mockHarness) Workspaces() map[string]workspace.Workspace {
	return make(map[string]workspace.Workspace)
}

// TaxonomyRegistry returns nil (no taxonomy introspection in mock)
func (m *mockHarness) Authorize(_ context.Context, _, _ string) error {
	return nil
}

func (m *mockHarness) TaxonomyRegistry() graphrag.TaxonomyIntrospector {
	return nil
}

func TestMockHarness_Complete(t *testing.T) {
	harness := &mockHarness{}
	ctx := context.Background()

	messages := []llm.Message{
		{Role: llm.RoleUser, Content: "test prompt"},
	}

	resp, err := harness.Complete(ctx, "primary", messages)
	if err != nil {
		t.Errorf("Complete() error = %v, want nil", err)
	}
	if resp == nil {
		t.Fatal("Complete() returned nil response")
	}
	if resp.Content != "mock response" {
		t.Errorf("Complete() content = %s, want 'mock response'", resp.Content)
	}
}

func TestMockHarness_CompleteWithCustomFunc(t *testing.T) {
	callCount := 0
	harness := &mockHarness{
		completeFunc: func(ctx context.Context, slot string, messages []llm.Message, opts ...llm.CompletionOption) (*llm.CompletionResponse, error) {
			callCount++
			return &llm.CompletionResponse{Content: "custom response"}, nil
		},
	}

	ctx := context.Background()
	messages := []llm.Message{{Role: llm.RoleUser, Content: "test"}}

	resp, err := harness.Complete(ctx, "primary", messages)
	if err != nil {
		t.Errorf("Complete() error = %v", err)
	}
	if resp.Content != "custom response" {
		t.Errorf("Complete() content = %s, want 'custom response'", resp.Content)
	}
	if callCount != 1 {
		t.Errorf("custom function called %d times, want 1", callCount)
	}
}

func TestMockHarness_CompleteWithTools(t *testing.T) {
	harness := &mockHarness{}
	ctx := context.Background()

	messages := []llm.Message{{Role: llm.RoleUser, Content: "test"}}
	tools := []llm.ToolDef{
		{Name: "test-tool", Description: "A test tool"},
	}

	resp, err := harness.CompleteWithTools(ctx, "primary", messages, tools)
	if err != nil {
		t.Errorf("CompleteWithTools() error = %v", err)
	}
	if resp == nil {
		t.Fatal("CompleteWithTools() returned nil")
	}
}

func TestMockHarness_Stream(t *testing.T) {
	harness := &mockHarness{}
	ctx := context.Background()

	messages := []llm.Message{{Role: llm.RoleUser, Content: "test"}}

	ch, err := harness.Stream(ctx, "primary", messages)
	if err != nil {
		t.Errorf("Stream() error = %v", err)
	}

	chunk := <-ch
	if chunk.Delta != "mock stream" {
		t.Errorf("Stream() chunk = %s, want 'mock stream'", chunk.Delta)
	}
}

func TestMockHarness_CallToolProto(t *testing.T) {
	harness := &mockHarness{}
	ctx := context.Background()

	err := harness.CallToolProto(ctx, "test-tool", nil, nil)
	if err != nil {
		t.Errorf("CallToolProto() error = %v", err)
	}
}

func TestMockHarness_ListTools(t *testing.T) {
	harness := &mockHarness{}
	ctx := context.Background()

	tools, err := harness.ListTools(ctx)
	if err != nil {
		t.Errorf("ListTools() error = %v", err)
	}
	if len(tools) != 2 {
		t.Errorf("ListTools() returned %d tools, want 2", len(tools))
	}
}

func TestMockHarness_QueryPlugin(t *testing.T) {
	harness := &mockHarness{}
	ctx := context.Background()

	result, err := harness.QueryPlugin(ctx, "plugin1", "method1", map[string]any{})
	if err != nil {
		t.Errorf("QueryPlugin() error = %v", err)
	}
	if result == nil {
		t.Fatal("QueryPlugin() returned nil")
	}
}

func TestMockHarness_ListPlugins(t *testing.T) {
	harness := &mockHarness{}
	ctx := context.Background()

	plugins, err := harness.ListPlugins(ctx)
	if err != nil {
		t.Errorf("ListPlugins() error = %v", err)
	}
	if len(plugins) != 1 {
		t.Errorf("ListPlugins() returned %d plugins, want 1", len(plugins))
	}
}

func TestMockHarness_DelegateToAgent(t *testing.T) {
	harness := &mockHarness{}
	ctx := context.Background()

	task := NewTask("task-1")
	result, err := harness.DelegateToAgent(ctx, "agent1", *task)
	if err != nil {
		t.Errorf("DelegateToAgent() error = %v", err)
	}
	if result.Status != StatusSuccess {
		t.Errorf("DelegateToAgent() status = %v, want %v", result.Status, StatusSuccess)
	}
}

func TestMockHarness_ListAgents(t *testing.T) {
	harness := &mockHarness{}
	ctx := context.Background()

	agents, err := harness.ListAgents(ctx)
	if err != nil {
		t.Errorf("ListAgents() error = %v", err)
	}
	if len(agents) != 1 {
		t.Errorf("ListAgents() returned %d agents, want 1", len(agents))
	}
}

func TestMockHarness_SubmitFinding(t *testing.T) {
	harness := &mockHarness{}
	ctx := context.Background()

	f := &finding.Finding{
		ID:       "finding-1",
		Severity: finding.SeverityHigh,
		Category: finding.CategoryJailbreak,
	}

	err := harness.SubmitFinding(ctx, f)
	if err != nil {
		t.Errorf("SubmitFinding() error = %v", err)
	}
}

func TestMockHarness_GetFindings(t *testing.T) {
	harness := &mockHarness{}
	ctx := context.Background()

	filter := finding.Filter{
		MissionID: "mission-1",
	}

	findings, err := harness.GetFindings(ctx, filter)
	if err != nil {
		t.Errorf("GetFindings() error = %v", err)
	}
	if findings == nil {
		t.Fatal("GetFindings() returned nil")
	}
}

func TestMockHarness_MissionAndTarget(t *testing.T) {
	mission := types.MissionContext{
		ID:   "mission-1",
		Name: "Test Mission",
	}
	target := types.TargetInfo{
		ID:   "target-1",
		Name: "Test Target",
		Type: string("llm_chat"),
	}

	harness := &mockHarness{
		mission: mission,
		target:  target,
	}

	if harness.Mission().ID != "mission-1" {
		t.Errorf("Mission().ID = %s, want mission-1", harness.Mission().ID)
	}
	if harness.Target().ID != "target-1" {
		t.Errorf("Target().ID = %s, want target-1", harness.Target().ID)
	}
}

func TestMockHarness_Observability(t *testing.T) {
	harness := &mockHarness{}

	tracer := harness.Tracer()
	if tracer == nil {
		t.Error("Tracer() returned nil")
	}

	logger := harness.Logger()
	if logger == nil {
		t.Error("Logger() returned nil")
	}

	tracker := harness.TokenUsage()
	if tracker == nil {
		t.Error("TokenUsage() returned nil")
	}
}
