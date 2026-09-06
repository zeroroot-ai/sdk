// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Zero Root AI

package eval_test

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/zeroroot-ai/sdk/agent"
	graphragpb "github.com/zeroroot-ai/sdk/api/gen/gibson/graphrag/v1"
	testpb "github.com/zeroroot-ai/sdk/api/gen/gibson/test/v1"
	"github.com/zeroroot-ai/sdk/codegen/workspace"
	"github.com/zeroroot-ai/sdk/eval"
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

// This example demonstrates how to use RecordingHarness to capture
// agent execution trajectories for evaluation.
func ExampleRecordingHarness() {
	// Create a mock harness for demonstration
	// In real usage, this would be the actual agent harness
	mockHarness := &minimalMockHarness{}

	// Wrap it with a recording harness
	recorder := eval.NewRecordingHarness(mockHarness)

	// Execute agent operations through the recording harness
	ctx := context.Background()

	// LLM completion
	_, _ = recorder.Complete(ctx, "primary", []llm.Message{
		{Role: "user", Content: "What is 2+2?"},
	})

	// Tool invocation
	_ = recorder.CallToolProto(ctx, "generic-tool", &testpb.GenericRequest{Targets: []string{"example.com"}}, &testpb.GenericResponse{})

	// Finding emit (the worker's write path; agents emit, never recall)
	_ = recorder.SubmitFinding(ctx, &finding.Finding{ID: "f1"})

	// Get the recorded trajectory
	trajectory := recorder.Trajectory()

	// Print trajectory summary
	fmt.Printf("Recorded %d operations\n", len(trajectory.Steps))
	for i, step := range trajectory.Steps {
		// Round duration to milliseconds for consistent output
		durationMs := step.Duration.Round(time.Millisecond)
		fmt.Printf("%d. %s: %s (took %v)\n", i+1, step.Type, step.Name, durationMs)
	}

	// Output:
	// Recorded 3 operations
	// 1. llm: primary (took 0s)
	// 2. tool: generic-tool (took 0s)
	// 3. finding: submit (took 0s)
}

// minimalMockHarness is a minimal harness implementation for the example.
type minimalMockHarness struct {
	agent.BaseHarness // absorbs interface growth; see its doc. Explicit methods below still win.
}

func (m *minimalMockHarness) Complete(ctx context.Context, slot string, messages []llm.Message, opts ...llm.CompletionOption) (*llm.CompletionResponse, error) {
	return &llm.CompletionResponse{Content: "4"}, nil
}

func (m *minimalMockHarness) CallToolProto(ctx context.Context, name string, request protolib.Message, response protolib.Message) error {
	return nil
}

func (m *minimalMockHarness) CallToolProtoStream(ctx context.Context, toolName string, input protolib.Message, output protolib.Message, callback agent.ToolStreamCallback) error {
	return nil
}

func (m *minimalMockHarness) QueueToolWork(ctx context.Context, toolName string, inputs []protolib.Message) (string, error) {
	return "", nil
}

func (m *minimalMockHarness) ToolResults(ctx context.Context, jobID string) <-chan agent.QueuedToolResult {
	ch := make(chan agent.QueuedToolResult)
	close(ch)
	return ch
}

// Stub implementations for other required methods (not shown for brevity)
func (m *minimalMockHarness) CompleteWithTools(ctx context.Context, slot string, messages []llm.Message, tools []llm.ToolDef) (*llm.CompletionResponse, error) {
	return nil, nil
}
func (m *minimalMockHarness) Stream(ctx context.Context, slot string, messages []llm.Message) (<-chan llm.StreamChunk, error) {
	return nil, nil
}
func (m *minimalMockHarness) ListTools(ctx context.Context) ([]tool.Descriptor, error) {
	return nil, nil
}
func (m *minimalMockHarness) QueryPlugin(ctx context.Context, name string, method string, params map[string]any) (any, error) {
	return nil, nil
}
func (m *minimalMockHarness) ListPlugins(ctx context.Context) ([]plugin.Descriptor, error) {
	return nil, nil
}
func (m *minimalMockHarness) DelegateToAgent(ctx context.Context, name string, task agent.Task) (agent.Result, error) {
	return agent.Result{}, nil
}
func (m *minimalMockHarness) ListAgents(ctx context.Context) ([]agent.Descriptor, error) {
	return nil, nil
}
func (m *minimalMockHarness) SubmitFinding(ctx context.Context, f *finding.Finding) error { return nil }
func (m *minimalMockHarness) GetFindings(ctx context.Context, filter finding.Filter) ([]*finding.Finding, error) {
	return nil, nil
}
func (m *minimalMockHarness) PlanContext() planning.PlanningContext { return nil }
func (m *minimalMockHarness) ReportStepHints(ctx context.Context, hints *planning.StepHints) error {
	return nil
}
func (m *minimalMockHarness) Mission() types.MissionContext { return types.MissionContext{} }
func (m *minimalMockHarness) Target() types.TargetInfo      { return types.TargetInfo{} }
func (m *minimalMockHarness) Tracer() trace.Tracer          { return nil }
func (m *minimalMockHarness) Logger() *slog.Logger          { return nil }
func (m *minimalMockHarness) TokenUsage() llm.TokenTracker  { return nil }
func (m *minimalMockHarness) QueryNodes(ctx context.Context, query *graphragpb.GraphQuery) ([]*graphragpb.QueryResult, error) {
	return nil, nil
}
func (m *minimalMockHarness) Observe(ctx context.Context, obs agent.Observation) error { return nil }

func (m *minimalMockHarness) WorldView(ctx context.Context, focus ...agent.Handle) (agent.WorldView, error) {
	return agent.WorldView{}, nil
}

func (m *minimalMockHarness) GraphRAGHealth(ctx context.Context) types.HealthStatus {
	return types.HealthStatus{}
}

func (m *minimalMockHarness) MissionExecutionContext() types.MissionExecutionContext {
	return types.MissionExecutionContext{}
}

func (m *minimalMockHarness) GetMissionRunHistory(ctx context.Context) ([]types.MissionRunSummary, error) {
	return []types.MissionRunSummary{}, nil
}

func (m *minimalMockHarness) GetPreviousRunFindings(ctx context.Context, filter finding.Filter) ([]*finding.Finding, error) {
	return []*finding.Finding{}, nil
}

func (m *minimalMockHarness) GetAllRunFindings(ctx context.Context, filter finding.Filter) ([]*finding.Finding, error) {
	return []*finding.Finding{}, nil
}

// MissionManager methods - stubs for testing
func (m *minimalMockHarness) CreateMission(ctx context.Context, missionDef any, targetID string, opts *mission.CreateMissionOpts) (*mission.MissionInfo, error) {
	return nil, errors.New("not implemented")
}

func (m *minimalMockHarness) RunMission(ctx context.Context, missionID string, opts *mission.RunMissionOpts) error {
	return errors.New("not implemented")
}

func (m *minimalMockHarness) GetMissionStatus(ctx context.Context, missionID string) (*mission.MissionStatusInfo, error) {
	return nil, errors.New("not implemented")
}

func (m *minimalMockHarness) WaitForMission(ctx context.Context, missionID string, timeout time.Duration) (*mission.MissionResult, error) {
	return nil, errors.New("not implemented")
}

func (m *minimalMockHarness) ListMissions(ctx context.Context, filter *mission.MissionFilter) ([]*mission.MissionInfo, error) {
	return nil, errors.New("not implemented")
}

func (m *minimalMockHarness) CancelMission(ctx context.Context, missionID string) error {
	return errors.New("not implemented")
}

func (m *minimalMockHarness) GetMissionResults(ctx context.Context, missionID string) (*mission.MissionResult, error) {
	return nil, errors.New("not implemented")
}

func (m *minimalMockHarness) GetCredential(ctx context.Context, name string) (*types.Credential, error) {
	return &types.Credential{
		Name:   name,
		Type:   "api-key",
		Secret: "mock-secret-value",
	}, nil
}

// CompleteStructured methods
func (m *minimalMockHarness) CompleteStructured(ctx context.Context, slot string, messages []llm.Message, schema any) (any, error) {
	return nil, errors.New("not implemented")
}

func (m *minimalMockHarness) CompleteStructuredAny(ctx context.Context, slot string, messages []llm.Message, schema any) (any, error) {
	return m.CompleteStructured(ctx, slot, messages, schema)
}

func (m *minimalMockHarness) Workspace() workspace.Workspace {
	return nil
}

func (m *minimalMockHarness) Workspaces() map[string]workspace.Workspace {
	return make(map[string]workspace.Workspace)
}

func (m *minimalMockHarness) TaxonomyRegistry() graphrag.TaxonomyIntrospector {
	return nil
}

func (m *minimalMockHarness) Authorize(ctx context.Context, action, resource string) error {
	return nil
}

func (m *minimalMockHarness) StoreGraphNode(ctx context.Context, node graphrag.GraphNode) (string, error) {
	return "", nil
}

func (m *minimalMockHarness) StoreSemantic(ctx context.Context, node graphrag.GraphNode) (string, error) {
	return "", nil
}

func (m *minimalMockHarness) StoreStructured(ctx context.Context, node graphrag.GraphNode) (string, error) {
	return "", nil
}

func (m *minimalMockHarness) CreateGraphRelationship(ctx context.Context, rel graphrag.Relationship) error {
	return nil
}

func (m *minimalMockHarness) StoreGraphBatch(ctx context.Context, batch graphrag.Batch) ([]string, error) {
	return nil, nil
}
