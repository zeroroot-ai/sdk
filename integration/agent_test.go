// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Zero Root AI

package integration

import (
	"context"
	"errors"
	"log/slog"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	sdk "github.com/zeroroot-ai/sdk"
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

// TestAgentCreation tests creating an agent using SDK entry points.
func TestAgentCreation(t *testing.T) {
	t.Run("with all required fields", func(t *testing.T) {
		a, err := sdk.NewAgent(
			sdk.WithName("test-agent"),
			sdk.WithVersion("1.0.0"),
			sdk.WithDescription("A test agent for integration testing"),
			sdk.WithCapabilities("prompt_injection"),
			sdk.WithTargetTypes("llm_chat"),
			sdk.WithTechniqueTypes("prompt_injection"),
			sdk.WithExecuteFunc(func(ctx context.Context, h agent.Harness, task agent.Task) (agent.Result, error) {
				return agent.NewSuccessResult("test completed"), nil
			}),
		)

		require.NoError(t, err)
		require.NotNil(t, a)

		assert.Equal(t, "test-agent", a.Name())
		assert.Equal(t, "1.0.0", a.Version())
		assert.Equal(t, "A test agent for integration testing", a.Description())
		assert.Len(t, a.Capabilities(), 1)
		assert.Equal(t, "prompt_injection", a.Capabilities()[0])
		assert.Len(t, a.TargetTypes(), 1)
		assert.Equal(t, "llm_chat", a.TargetTypes()[0])
		assert.Len(t, a.TechniqueTypes(), 1)
		assert.Equal(t, "prompt_injection", a.TechniqueTypes()[0])
	})

	t.Run("with multiple capabilities", func(t *testing.T) {
		a, err := sdk.NewAgent(
			sdk.WithName("multi-capability-agent"),
			sdk.WithVersion("2.0.0"),
			sdk.WithDescription("Agent with multiple capabilities"),
			sdk.WithCapabilities(
				"prompt_injection",
				"jailbreak",
				"data_extraction",
			),
			sdk.WithExecuteFunc(func(ctx context.Context, h agent.Harness, task agent.Task) (agent.Result, error) {
				return agent.NewSuccessResult("done"), nil
			}),
		)

		require.NoError(t, err)
		assert.Len(t, a.Capabilities(), 3)
	})

	t.Run("with LLM slots", func(t *testing.T) {
		a, err := sdk.NewAgent(
			sdk.WithName("llm-agent"),
			sdk.WithVersion("1.0.0"),
			sdk.WithDescription("Agent with LLM requirements"),
			sdk.WithLLMSlot("primary", llm.SlotRequirements{
				MinContextWindow: 8000,
				RequiredFeatures: []string{"chat"},
			}),
			sdk.WithLLMSlot("vision", llm.SlotRequirements{
				MinContextWindow: 4000,
				RequiredFeatures: []string{"vision"},
			}),
			sdk.WithExecuteFunc(func(ctx context.Context, h agent.Harness, task agent.Task) (agent.Result, error) {
				return agent.NewSuccessResult("done"), nil
			}),
		)

		require.NoError(t, err)
		slots := a.LLMSlots()
		assert.Len(t, slots, 2)

		// Find primary slot
		var primarySlot *llm.SlotDefinition
		for i := range slots {
			if slots[i].Name == "primary" {
				primarySlot = &slots[i]
				break
			}
		}
		require.NotNil(t, primarySlot)
		assert.Equal(t, 8000, primarySlot.MinContextWindow)
		assert.Contains(t, primarySlot.RequiredFeatures, "chat")
	})

	t.Run("missing required name", func(t *testing.T) {
		a, err := sdk.NewAgent(
			sdk.WithVersion("1.0.0"),
			sdk.WithDescription("Missing name"),
			sdk.WithExecuteFunc(func(ctx context.Context, h agent.Harness, task agent.Task) (agent.Result, error) {
				return agent.NewSuccessResult("done"), nil
			}),
		)

		assert.Error(t, err)
		assert.Nil(t, a)
		assert.Contains(t, err.Error(), "name")
	})

	t.Run("missing execute function", func(t *testing.T) {
		a, err := sdk.NewAgent(
			sdk.WithName("no-execute"),
			sdk.WithVersion("1.0.0"),
			sdk.WithDescription("Missing execute function"),
		)

		assert.Error(t, err)
		assert.Nil(t, a)
		assert.Contains(t, err.Error(), "execute")
	})
}

// TestAgentLifecycle tests the full agent lifecycle.
func TestAgentLifecycle(t *testing.T) {
	var initialized bool
	var executed bool
	var shutdown bool

	a, err := sdk.NewAgent(
		sdk.WithName("lifecycle-agent"),
		sdk.WithVersion("1.0.0"),
		sdk.WithDescription("Agent to test lifecycle"),
		sdk.WithInitFunc(func(ctx context.Context, config map[string]any) error {
			initialized = true
			// Verify config is passed through
			if val, ok := config["test_key"]; ok && val == "test_value" {
				return nil
			}
			return errors.New("config not passed correctly")
		}),
		sdk.WithExecuteFunc(func(ctx context.Context, h agent.Harness, task agent.Task) (agent.Result, error) {
			executed = true
			return agent.NewSuccessResult("lifecycle test completed"), nil
		}),
		sdk.WithShutdownFunc(func(ctx context.Context) error {
			shutdown = true
			return nil
		}),
	)

	require.NoError(t, err)
	require.NotNil(t, a)

	ctx := context.Background()

	// Test initialization
	t.Run("initialize", func(t *testing.T) {
		config := map[string]any{
			"test_key": "test_value",
		}
		err := a.Initialize(ctx, config)
		require.NoError(t, err)
		assert.True(t, initialized, "initialize function should have been called")
	})

	// Test execution
	t.Run("execute", func(t *testing.T) {
		task := agent.NewTask("test-task-1")
		result, err := a.Execute(ctx, &mockHarness{}, *task)
		require.NoError(t, err)
		assert.True(t, executed, "execute function should have been called")
		assert.Equal(t, agent.StatusSuccess, result.Status)
		assert.Equal(t, "lifecycle test completed", result.Output)
	})

	// Test shutdown
	t.Run("shutdown", func(t *testing.T) {
		err := a.Shutdown(ctx)
		require.NoError(t, err)
		assert.True(t, shutdown, "shutdown function should have been called")
	})
}

// TestAgentExecution tests agent execution with different scenarios.
func TestAgentExecution(t *testing.T) {
	t.Run("successful execution", func(t *testing.T) {
		a, err := sdk.NewAgent(
			sdk.WithName("success-agent"),
			sdk.WithVersion("1.0.0"),
			sdk.WithDescription("Always succeeds"),
			sdk.WithExecuteFunc(func(ctx context.Context, h agent.Harness, task agent.Task) (agent.Result, error) {
				// Verify task data is accessible
				assert.Equal(t, "test-task", task.ID)

				result := agent.NewSuccessResult(map[string]any{
					"status": "completed",
					"data":   "test data",
				})
				return result, nil
			}),
		)

		require.NoError(t, err)

		task := agent.NewTask("test-task")
		ctx := context.Background()
		result, err := a.Execute(ctx, &mockHarness{}, *task)

		require.NoError(t, err)
		assert.Equal(t, agent.StatusSuccess, result.Status)

		output, ok := result.Output.(map[string]any)
		require.True(t, ok)
		assert.Equal(t, "completed", output["status"])
	})

	t.Run("failed execution", func(t *testing.T) {
		expectedErr := errors.New("execution failed")

		a, err := sdk.NewAgent(
			sdk.WithName("fail-agent"),
			sdk.WithVersion("1.0.0"),
			sdk.WithDescription("Always fails"),
			sdk.WithExecuteFunc(func(ctx context.Context, h agent.Harness, task agent.Task) (agent.Result, error) {
				return agent.NewFailedResult(expectedErr), nil
			}),
		)

		require.NoError(t, err)

		task := agent.NewTask("fail-task")
		ctx := context.Background()
		result, err := a.Execute(ctx, &mockHarness{}, *task)

		require.NoError(t, err)
		assert.Equal(t, agent.StatusFailed, result.Status)
		assert.Equal(t, expectedErr, result.Error)
	})

	t.Run("execution with findings", func(t *testing.T) {
		a, err := sdk.NewAgent(
			sdk.WithName("finding-agent"),
			sdk.WithVersion("1.0.0"),
			sdk.WithDescription("Discovers findings"),
			sdk.WithCapabilities("prompt_injection"),
			sdk.WithExecuteFunc(func(ctx context.Context, h agent.Harness, task agent.Task) (agent.Result, error) {
				result := agent.NewSuccessResult("Found vulnerabilities")
				result.AddFinding("finding-1")
				result.AddFinding("finding-2")
				result.SetMetadata("vulnerability_count", 2)
				return result, nil
			}),
		)

		require.NoError(t, err)

		task := agent.NewTask("scan-task")
		ctx := context.Background()
		result, err := a.Execute(ctx, &mockHarness{}, *task)

		require.NoError(t, err)
		assert.Equal(t, agent.StatusSuccess, result.Status)
		assert.Len(t, result.Findings, 2)
		assert.Contains(t, result.Findings, "finding-1")
		assert.Contains(t, result.Findings, "finding-2")

		count, ok := result.GetMetadata("vulnerability_count")
		assert.True(t, ok)
		assert.Equal(t, 2, count)
	})
}

// TestAgentHealth tests agent health status reporting.
func TestAgentHealth(t *testing.T) {
	t.Run("default healthy status", func(t *testing.T) {
		a, err := sdk.NewAgent(
			sdk.WithName("healthy-agent"),
			sdk.WithVersion("1.0.0"),
			sdk.WithDescription("Always healthy"),
			sdk.WithExecuteFunc(func(ctx context.Context, h agent.Harness, task agent.Task) (agent.Result, error) {
				return agent.NewSuccessResult("done"), nil
			}),
		)

		require.NoError(t, err)

		ctx := context.Background()
		health := a.Health(ctx)

		assert.True(t, health.IsHealthy())
		assert.NotEmpty(t, health.Message)
	})

	t.Run("custom health check", func(t *testing.T) {
		healthCheckCalled := false

		a, err := sdk.NewAgent(
			sdk.WithName("custom-health-agent"),
			sdk.WithVersion("1.0.0"),
			sdk.WithDescription("Custom health check"),
			sdk.WithExecuteFunc(func(ctx context.Context, h agent.Harness, task agent.Task) (agent.Result, error) {
				return agent.NewSuccessResult("done"), nil
			}),
			sdk.WithHealthFunc(func(ctx context.Context) types.HealthStatus {
				healthCheckCalled = true
				return types.NewHealthyStatus("custom health check passed")
			}),
		)

		require.NoError(t, err)

		ctx := context.Background()
		health := a.Health(ctx)

		assert.True(t, healthCheckCalled)
		assert.True(t, health.IsHealthy())
		assert.Equal(t, "custom health check passed", health.Message)
	})

	t.Run("unhealthy status", func(t *testing.T) {
		a, err := sdk.NewAgent(
			sdk.WithName("unhealthy-agent"),
			sdk.WithVersion("1.0.0"),
			sdk.WithDescription("Reports unhealthy"),
			sdk.WithExecuteFunc(func(ctx context.Context, h agent.Harness, task agent.Task) (agent.Result, error) {
				return agent.NewSuccessResult("done"), nil
			}),
			sdk.WithHealthFunc(func(ctx context.Context) types.HealthStatus {
				return types.NewUnhealthyStatus("service degraded", map[string]any{
					"error": "dependency unavailable",
				})
			}),
		)

		require.NoError(t, err)

		ctx := context.Background()
		health := a.Health(ctx)

		assert.False(t, health.IsHealthy())
		assert.Equal(t, "service degraded", health.Message)
		assert.NotNil(t, health.Details)
	})
}

// TestAgentCapabilities tests all agent capabilities are properly set.
func TestAgentCapabilities(t *testing.T) {
	capabilities := []string{
		"prompt_injection",
		"jailbreak",
		"data_extraction",
		"model_manipulation",
		"dos",
	}

	for _, cap := range capabilities {
		t.Run(string(cap), func(t *testing.T) {
			a, err := sdk.NewAgent(
				sdk.WithName("cap-test-agent"),
				sdk.WithVersion("1.0.0"),
				sdk.WithDescription("Testing "+string(cap)),
				sdk.WithCapabilities(cap),
				sdk.WithExecuteFunc(func(ctx context.Context, h agent.Harness, task agent.Task) (agent.Result, error) {
					return agent.NewSuccessResult("done"), nil
				}),
			)

			require.NoError(t, err)
			assert.Contains(t, a.Capabilities(), cap)
			// Note: Capabilities are now strings, no validation methods
			assert.NotEmpty(t, cap)
		})
	}
}

// TODO: consolidate mock harness implementations into a shared testutil package.
// Currently duplicated in: agent/harness_test.go, eval/recording_harness_test.go,
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

func (m *mockHarness) CreateGraphRelationship(ctx context.Context, rel graphrag.Relationship) error {
	return errors.New("not implemented")
}

func (m *mockHarness) StoreGraphBatch(ctx context.Context, batch graphrag.Batch) ([]string, error) {
	return nil, errors.New("not implemented")
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
