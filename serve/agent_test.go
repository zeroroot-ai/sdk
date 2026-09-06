// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Zero Root AI

package serve

import (
	"context"
	"net"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/test/bufconn"

	"github.com/zeroroot-ai/sdk/agent"
	agentpb "github.com/zeroroot-ai/sdk/api/gen/gibson/agent/v1"
	commonpb "github.com/zeroroot-ai/sdk/api/gen/gibson/common/v1"
	"github.com/zeroroot-ai/sdk/llm"
	"github.com/zeroroot-ai/sdk/types"
)

// mockAgent is a mock implementation of agent.Agent for testing.
type mockAgent struct {
	name        string
	version     string
	description string
	health      types.HealthStatus
	executeFunc func(ctx context.Context, harness agent.Harness, task agent.Task) (agent.Result, error)
}

func (m *mockAgent) Name() string        { return m.name }
func (m *mockAgent) Version() string     { return m.version }
func (m *mockAgent) Description() string { return m.description }

func (m *mockAgent) Capabilities() []string {
	return []string{"prompt_injection"}
}

func (m *mockAgent) TargetSchemas() []types.TargetSchema {
	return []types.TargetSchema{}
}

func (m *mockAgent) TargetTypes() []string {
	return []string{"llm_chat"}
}

func (m *mockAgent) TechniqueTypes() []string {
	return []string{"prompt_injection"}
}

func (m *mockAgent) LLMSlots() []llm.SlotDefinition {
	return []llm.SlotDefinition{
		{
			Name:             "primary",
			Description:      "Primary LLM for agent reasoning",
			Required:         true,
			MinContextWindow: 8000,
			RequiredFeatures: []string{"function_calling"},
			PreferredModels:  []string{"gpt-4", "claude-3-opus"},
		},
	}
}

func (m *mockAgent) Execute(ctx context.Context, harness agent.Harness, task agent.Task) (agent.Result, error) {
	if m.executeFunc != nil {
		return m.executeFunc(ctx, harness, task)
	}
	return agent.NewSuccessResult(map[string]any{"status": "completed"}), nil
}

func (m *mockAgent) Initialize(ctx context.Context, config map[string]any) error {
	return nil
}

func (m *mockAgent) Shutdown(ctx context.Context) error {
	return nil
}

func (m *mockAgent) Health(ctx context.Context) types.HealthStatus {
	if m.health.Status == "" {
		return types.NewHealthyStatus("Agent is healthy")
	}
	return m.health
}

// setupAgentTestServer creates an in-memory gRPC server for testing using bufconn.
func setupAgentTestServer(t *testing.T, a agent.Agent) (*grpc.ClientConn, func()) {
	const bufSize = 1024 * 1024
	lis := bufconn.Listen(bufSize)

	srv := grpc.NewServer()
	agentSvc := &agentServiceServer{agent: a}
	agentpb.RegisterAgentServiceServer(srv, agentSvc)

	// Start server
	go func() {
		if err := srv.Serve(lis); err != nil {
			t.Logf("Server exited with error: %v", err)
		}
	}()

	// Create client connection
	ctx := context.Background()
	conn, err := grpc.DialContext(ctx, "bufnet",
		grpc.WithContextDialer(func(context.Context, string) (net.Conn, error) {
			return lis.Dial()
		}),
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	require.NoError(t, err)

	cleanup := func() {
		conn.Close()
		srv.Stop()
		lis.Close()
	}

	return conn, cleanup
}

func TestAgentServiceServer_GetDescriptor(t *testing.T) {
	mockA := &mockAgent{
		name:        "test-agent",
		version:     "1.0.0",
		description: "Test agent for unit testing",
	}

	conn, cleanup := setupAgentTestServer(t, mockA)
	defer cleanup()

	client := agentpb.NewAgentServiceClient(conn)
	resp, err := client.GetDescriptor(context.Background(), &agentpb.GetDescriptorRequest{})

	require.NoError(t, err)
	assert.Equal(t, "test-agent", resp.Name)
	assert.Equal(t, "1.0.0", resp.Version)
	assert.Equal(t, "Test agent for unit testing", resp.Description)
	assert.Contains(t, resp.Capabilities, "prompt_injection")
	assert.Contains(t, resp.TargetTypes, "llm_chat")
	assert.Contains(t, resp.TechniqueTypes, "prompt_injection")
}

func TestAgentServiceServer_GetSlotSchema(t *testing.T) {
	mockA := &mockAgent{
		name:    "test-agent",
		version: "1.0.0",
	}

	conn, cleanup := setupAgentTestServer(t, mockA)
	defer cleanup()

	client := agentpb.NewAgentServiceClient(conn)
	resp, err := client.GetSlotSchema(context.Background(), &agentpb.GetSlotSchemaRequest{})

	require.NoError(t, err)
	require.Len(t, resp.Slots, 1)

	slot := resp.Slots[0]
	assert.Equal(t, "primary", slot.Name)
	assert.Equal(t, "Primary LLM for agent reasoning", slot.Description)
	assert.True(t, slot.Required)
	assert.NotNil(t, slot.DefaultConfig)
	assert.Equal(t, "gpt-4", slot.DefaultConfig.Model) // First preferred model
	assert.NotNil(t, slot.Constraints)
	assert.Equal(t, int32(8000), slot.Constraints.MinContextWindow)
	assert.Contains(t, slot.Constraints.RequiredFeatures, "function_calling")
}

func TestAgentServiceServer_Execute(t *testing.T) {
	tests := []struct {
		name        string
		task        agent.Task
		executeFunc func(ctx context.Context, harness agent.Harness, task agent.Task) (agent.Result, error)
		wantErr     bool
		checkResult func(t *testing.T, resp *agentpb.ExecuteResponse)
	}{
		{
			name: "successful execution",
			task: agent.Task{
				ID:      "test-task-1",
				Context: map[string]any{"objective": "Test goal"},
			},
			executeFunc: func(ctx context.Context, harness agent.Harness, task agent.Task) (agent.Result, error) {
				return agent.NewSuccessResult(map[string]any{
					"message": "Task completed",
					"task_id": task.ID,
				}), nil
			},
			wantErr: false,
			checkResult: func(t *testing.T, resp *agentpb.ExecuteResponse) {
				assert.Nil(t, resp.Error)
				assert.NotNil(t, resp.Result)

				result := ProtoToResult(resp.Result)
				assert.Equal(t, agent.StatusSuccess, result.Status)
			},
		},
		{
			name: "execution with error",
			task: agent.Task{
				ID:      "test-task-2",
				Context: map[string]any{"objective": "Test goal"},
			},
			executeFunc: func(ctx context.Context, harness agent.Harness, task agent.Task) (agent.Result, error) {
				return agent.Result{}, assert.AnError
			},
			wantErr: false,
			checkResult: func(t *testing.T, resp *agentpb.ExecuteResponse) {
				assert.NotNil(t, resp.Error)
				// Error.Code is a string containing the ErrorCode enum name
				assert.Equal(t, commonpb.ErrorCode_ERROR_CODE_INTERNAL.String(), resp.Error.Code)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockA := &mockAgent{
				name:        "test-agent",
				version:     "1.0.0",
				executeFunc: tt.executeFunc,
			}

			conn, cleanup := setupAgentTestServer(t, mockA)
			defer cleanup()

			client := agentpb.NewAgentServiceClient(conn)

			resp, err := client.Execute(context.Background(), &agentpb.ExecuteRequest{
				Task: TaskToProto(tt.task),
			})

			if tt.wantErr {
				assert.Error(t, err)
				return
			}

			require.NoError(t, err)
			if tt.checkResult != nil {
				tt.checkResult(t, resp)
			}
		})
	}
}

func TestAgentServiceServer_Execute_NilTask(t *testing.T) {
	mockA := &mockAgent{
		name:    "test-agent",
		version: "1.0.0",
		executeFunc: func(ctx context.Context, harness agent.Harness, task agent.Task) (agent.Result, error) {
			// With nil typespb.Task, ProtoToTask returns an empty agent.Task
			return agent.Result{
				Status: agent.StatusSuccess,
				Output: map[string]any{"handled": "empty task"},
			}, nil
		},
	}

	conn, cleanup := setupAgentTestServer(t, mockA)
	defer cleanup()

	client := agentpb.NewAgentServiceClient(conn)

	// Test with a nil task - should be handled gracefully (converted to empty task)
	resp, err := client.Execute(context.Background(), &agentpb.ExecuteRequest{
		Task: nil,
	})

	// No gRPC error - nil task is valid and converted to empty task
	require.NoError(t, err)
	assert.NotNil(t, resp)
	assert.NotNil(t, resp.Result)
}

func TestAgentServiceServer_Health(t *testing.T) {
	tests := []struct {
		name         string
		health       types.HealthStatus
		expectStatus string
	}{
		{
			name:         "healthy agent",
			health:       types.NewHealthyStatus("All systems operational"),
			expectStatus: types.StatusHealthy,
		},
		{
			name:         "degraded agent",
			health:       types.NewDegradedStatus("Some issues detected", nil),
			expectStatus: types.StatusDegraded,
		},
		{
			name:         "unhealthy agent",
			health:       types.NewUnhealthyStatus("Critical failure", nil),
			expectStatus: types.StatusUnhealthy,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockA := &mockAgent{
				name:    "test-agent",
				version: "1.0.0",
				health:  tt.health,
			}

			conn, cleanup := setupAgentTestServer(t, mockA)
			defer cleanup()

			client := agentpb.NewAgentServiceClient(conn)
			resp, err := client.Health(context.Background(), &agentpb.HealthRequest{})

			require.NoError(t, err)
			require.NotNil(t, resp.Status)
			assert.Equal(t, tt.expectStatus, resp.Status.Status)
			assert.NotEmpty(t, resp.Status.Message)
			assert.Positive(t, resp.Status.CheckedAt)
		})
	}
}
