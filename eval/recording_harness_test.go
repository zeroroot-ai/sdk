// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Zero Root AI

package eval

import (
	"context"
	"errors"
	"log/slog"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/otel/trace"
	"go.opentelemetry.io/otel/trace/noop"
	protolib "google.golang.org/protobuf/proto"

	"github.com/zeroroot-ai/sdk/agent"
	graphragpb "github.com/zeroroot-ai/sdk/api/gen/gibson/graphrag/v1"
	testpb "github.com/zeroroot-ai/sdk/api/gen/gibson/test/v1"
	"github.com/zeroroot-ai/sdk/codegen/workspace"
	"github.com/zeroroot-ai/sdk/finding"
	"github.com/zeroroot-ai/sdk/graphrag"
	"github.com/zeroroot-ai/sdk/llm"
	"github.com/zeroroot-ai/sdk/mission"
	"github.com/zeroroot-ai/sdk/planning"
	"github.com/zeroroot-ai/sdk/plugin"
	"github.com/zeroroot-ai/sdk/tool"
	"github.com/zeroroot-ai/sdk/types"
)

// TODO: consolidate mock harness implementations into a shared testutil package.
// Currently duplicated in: agent/harness_test.go, eval/recording_harness_test.go,
// integration/agent_test.go. Each new Harness interface method requires updating all copies.

// compile-time interface check
var _ agent.Harness = (*mockHarness)(nil)

// mockHarness is a minimal mock implementation of agent.Harness for testing.
type mockHarness struct {
	agent.BaseHarness // absorbs interface growth; see its doc. The explicit methods below still win.

	completeFunc      func(ctx context.Context, slot string, messages []llm.Message, opts ...llm.CompletionOption) (*llm.CompletionResponse, error)
	callToolProtoFunc func(ctx context.Context, name string, request protolib.Message, response protolib.Message) error
	submitFindingFunc func(ctx context.Context, f *finding.Finding) error
}

func (m *mockHarness) Complete(ctx context.Context, slot string, messages []llm.Message, opts ...llm.CompletionOption) (*llm.CompletionResponse, error) {
	if m.completeFunc != nil {
		return m.completeFunc(ctx, slot, messages, opts...)
	}
	return &llm.CompletionResponse{Content: "mock response"}, nil
}

func (m *mockHarness) CompleteWithTools(ctx context.Context, slot string, messages []llm.Message, tools []llm.ToolDef) (*llm.CompletionResponse, error) {
	return &llm.CompletionResponse{Content: "mock tool response"}, nil
}

func (m *mockHarness) CompleteStructured(ctx context.Context, slot string, messages []llm.Message, schema any) (any, error) {
	return map[string]any{"result": "structured"}, nil
}

func (m *mockHarness) CompleteStructuredAny(ctx context.Context, slot string, messages []llm.Message, schema any) (any, error) {
	return m.CompleteStructured(ctx, slot, messages, schema)
}

func (m *mockHarness) Stream(ctx context.Context, slot string, messages []llm.Message) (<-chan llm.StreamChunk, error) {
	ch := make(chan llm.StreamChunk)
	close(ch)
	return ch, nil
}

func (m *mockHarness) CallToolProto(ctx context.Context, name string, request protolib.Message, response protolib.Message) error {
	if m.callToolProtoFunc != nil {
		return m.callToolProtoFunc(ctx, name, request, response)
	}
	return nil
}

func (m *mockHarness) CallToolProtoStream(ctx context.Context, toolName string, input protolib.Message, output protolib.Message, callback agent.ToolStreamCallback) error {
	return nil
}

func (m *mockHarness) QueueToolWork(ctx context.Context, toolName string, inputs []protolib.Message) (string, error) {
	return "", nil
}

func (m *mockHarness) ToolResults(ctx context.Context, jobID string) <-chan agent.QueuedToolResult {
	ch := make(chan agent.QueuedToolResult)
	close(ch)
	return ch
}

func (m *mockHarness) ListTools(ctx context.Context) ([]tool.Descriptor, error) {
	return []tool.Descriptor{}, nil
}

func (m *mockHarness) QueryPlugin(ctx context.Context, name string, method string, params map[string]any) (any, error) {
	return "mock plugin result", nil
}

func (m *mockHarness) ListPlugins(ctx context.Context) ([]plugin.Descriptor, error) {
	return []plugin.Descriptor{}, nil
}

func (m *mockHarness) DelegateToAgent(ctx context.Context, name string, task agent.Task) (agent.Result, error) {
	return agent.NewSuccessResult("mock delegation"), nil
}

func (m *mockHarness) ListAgents(ctx context.Context) ([]agent.Descriptor, error) {
	return []agent.Descriptor{}, nil
}

func (m *mockHarness) SubmitFinding(ctx context.Context, f *finding.Finding) error {
	if m.submitFindingFunc != nil {
		return m.submitFindingFunc(ctx, f)
	}
	return nil
}

func (m *mockHarness) GetFindings(ctx context.Context, filter finding.Filter) ([]*finding.Finding, error) {
	return []*finding.Finding{}, nil
}

func (m *mockHarness) Mission() types.MissionContext {
	return types.MissionContext{}
}

func (m *mockHarness) Target() types.TargetInfo {
	return types.TargetInfo{}
}

func (m *mockHarness) Tracer() trace.Tracer {
	return noop.NewTracerProvider().Tracer("test")
}

func (m *mockHarness) Logger() *slog.Logger {
	return slog.Default()
}

func (m *mockHarness) TokenUsage() llm.TokenTracker {
	return nil
}

func (m *mockHarness) QueryNodes(ctx context.Context, query *graphragpb.GraphQuery) ([]*graphragpb.QueryResult, error) {
	return nil, nil
}

func (m *mockHarness) Observe(ctx context.Context, obs agent.Observation) error { return nil }

func (m *mockHarness) WorldView(ctx context.Context, focus ...agent.Handle) (agent.WorldView, error) {
	return agent.WorldView{}, nil
}

func (m *mockHarness) StoreGraphNode(ctx context.Context, node graphrag.GraphNode) (string, error) {
	return "node-123", nil
}

func (m *mockHarness) StoreSemantic(ctx context.Context, node graphrag.GraphNode) (string, error) {
	return "node-123", nil
}

func (m *mockHarness) StoreStructured(ctx context.Context, node graphrag.GraphNode) (string, error) {
	return "node-123", nil
}

func (m *mockHarness) CreateGraphRelationship(ctx context.Context, rel graphrag.Relationship) error {
	return nil
}

func (m *mockHarness) StoreGraphBatch(ctx context.Context, batch graphrag.Batch) ([]string, error) {
	return []string{"node-1", "node-2"}, nil
}

func (m *mockHarness) GraphRAGHealth(ctx context.Context) types.HealthStatus {
	return types.HealthStatus{Status: "healthy"}
}

// PlanContext returns the planning context for the current execution.
func (m *mockHarness) PlanContext() planning.PlanningContext {
	return nil
}

// ReportStepHints allows agents to provide feedback to the planning system.
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

// GetCredential returns a mock credential for testing
func (m *mockHarness) GetCredential(ctx context.Context, name string) (*types.Credential, error) {
	return &types.Credential{
		Name:   name,
		Type:   "api-key",
		Secret: "mock-secret-value",
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

// TaxonomyRegistry returns nil (no taxonomy introspection in mock)
func (m *mockHarness) TaxonomyRegistry() graphrag.TaxonomyIntrospector {
	return nil
}

func (m *mockHarness) Authorize(ctx context.Context, action, resource string) error {
	return nil
}

// TestRecordingHarnessBasics tests basic recording harness functionality.
func TestRecordingHarnessBasics(t *testing.T) {
	mock := &mockHarness{}
	recorder := NewRecordingHarness(mock)

	// Initially, trajectory should have no steps
	traj := recorder.Trajectory()
	assert.Empty(t, traj.Steps)
	assert.False(t, traj.StartTime.IsZero())

	// After reset, trajectory should be cleared
	recorder.Reset()
	traj = recorder.Trajectory()
	assert.Empty(t, traj.Steps)
}

// TestRecordingHarnessLLMCalls tests recording of LLM completion calls.
func TestRecordingHarnessLLMCalls(t *testing.T) {
	ctx := context.Background()
	mock := &mockHarness{}
	recorder := NewRecordingHarness(mock)

	// Call Complete
	messages := []llm.Message{{Role: "user", Content: "test"}}
	resp, err := recorder.Complete(ctx, "primary", messages)
	require.NoError(t, err)
	assert.NotNil(t, resp)

	// Verify trajectory recorded the call
	traj := recorder.Trajectory()
	require.Len(t, traj.Steps, 1)

	step := traj.Steps[0]
	assert.Equal(t, "llm", step.Type)
	assert.Equal(t, "primary", step.Name)
	assert.NotNil(t, step.Input)
	assert.NotNil(t, step.Output)
	assert.Empty(t, step.Error)
	assert.False(t, step.StartTime.IsZero())
	assert.Greater(t, step.Duration, time.Duration(0))
}

// TestRecordingHarnessToolCalls tests recording of tool invocations.
func TestRecordingHarnessToolCalls(t *testing.T) {
	ctx := context.Background()
	mock := &mockHarness{}
	recorder := NewRecordingHarness(mock)

	// Call a tool with proto messages
	req := &testpb.GenericRequest{Targets: []string{"https://example.com"}}
	resp := &testpb.GenericResponse{}
	err := recorder.CallToolProto(ctx, "generic-tool", req, resp)
	require.NoError(t, err)

	// Verify trajectory recorded the call
	traj := recorder.Trajectory()
	require.Len(t, traj.Steps, 1)

	step := traj.Steps[0]
	assert.Equal(t, "tool", step.Type)
	assert.Equal(t, "generic-tool", step.Name)
	assert.NotNil(t, step.Input)
	assert.NotNil(t, step.Output)
	assert.Empty(t, step.Error)
	assert.Greater(t, step.Duration, time.Duration(0))
}

// TestRecordingHarnessErrorRecording tests recording of errors.
func TestRecordingHarnessErrorRecording(t *testing.T) {
	ctx := context.Background()

	expectedErr := errors.New("mock error")
	mock := &mockHarness{
		callToolProtoFunc: func(ctx context.Context, name string, request protolib.Message, response protolib.Message) error {
			return expectedErr
		},
	}
	recorder := NewRecordingHarness(mock)

	// Call a tool that returns an error
	req := &testpb.GenericRequest{Targets: []string{"example.com"}}
	resp := &testpb.GenericResponse{}
	err := recorder.CallToolProto(ctx, "failing-tool", req, resp)
	require.Error(t, err)

	// Verify trajectory recorded the error
	traj := recorder.Trajectory()
	require.Len(t, traj.Steps, 1)

	step := traj.Steps[0]
	assert.Equal(t, "tool", step.Type)
	assert.Equal(t, "failing-tool", step.Name)
	assert.Equal(t, "mock error", step.Error)
}

// TestRecordingHarnessFindingSubmission tests recording of finding submissions.
func TestRecordingHarnessFindingSubmission(t *testing.T) {
	ctx := context.Background()
	mock := &mockHarness{}
	recorder := NewRecordingHarness(mock)

	// Submit a finding
	f := &finding.Finding{
		ID:       "finding-1",
		Severity: finding.SeverityHigh,
		Category: "injection",
	}
	err := recorder.SubmitFinding(ctx, f)
	require.NoError(t, err)

	// Verify trajectory recorded the submission
	traj := recorder.Trajectory()
	require.Len(t, traj.Steps, 1)

	step := traj.Steps[0]
	assert.Equal(t, "finding", step.Type)
	assert.Equal(t, "submit", step.Name)
	assert.NotNil(t, step.Input)
	assert.Empty(t, step.Error)
}

// TestRecordingHarnessMultipleOperations tests recording of multiple operations.
func TestRecordingHarnessMultipleOperations(t *testing.T) {
	ctx := context.Background()
	mock := &mockHarness{}
	recorder := NewRecordingHarness(mock)

	// Perform multiple operations
	_, _ = recorder.Complete(ctx, "primary", []llm.Message{{Role: "user", Content: "test"}})
	_ = recorder.CallToolProto(ctx, "generic-tool", &testpb.GenericRequest{Targets: []string{"test"}}, &testpb.GenericResponse{})
	_ = recorder.SubmitFinding(ctx, &finding.Finding{ID: "f1"})

	// Verify all operations were recorded
	traj := recorder.Trajectory()
	require.Len(t, traj.Steps, 3)

	assert.Equal(t, "llm", traj.Steps[0].Type)
	assert.Equal(t, "tool", traj.Steps[1].Type)
	assert.Equal(t, "finding", traj.Steps[2].Type)

	// Verify end time is set
	assert.False(t, traj.EndTime.IsZero())
	assert.True(t, traj.EndTime.After(traj.StartTime) || traj.EndTime.Equal(traj.StartTime))
}

// TestRecordingHarnessReset tests resetting the trajectory.
func TestRecordingHarnessReset(t *testing.T) {
	ctx := context.Background()
	mock := &mockHarness{}
	recorder := NewRecordingHarness(mock)

	// Perform some operations
	_, _ = recorder.Complete(ctx, "primary", []llm.Message{{Role: "user", Content: "test"}})
	traj := recorder.Trajectory()
	require.Len(t, traj.Steps, 1)

	// Reset the trajectory
	recorder.Reset()
	traj = recorder.Trajectory()
	assert.Empty(t, traj.Steps)
	assert.False(t, traj.StartTime.IsZero())
}

// TestRecordingHarnessThreadSafety tests thread-safe trajectory recording.
func TestRecordingHarnessThreadSafety(t *testing.T) {
	ctx := context.Background()
	mock := &mockHarness{}
	recorder := NewRecordingHarness(mock)

	// Perform concurrent operations
	done := make(chan bool)
	for range 10 {
		go func() {
			_, _ = recorder.Complete(ctx, "primary", []llm.Message{{Role: "user", Content: "test"}})
			done <- true
		}()
	}

	// Wait for all goroutines to complete
	for range 10 {
		<-done
	}

	// Verify all operations were recorded
	traj := recorder.Trajectory()
	assert.Len(t, traj.Steps, 10)
}
