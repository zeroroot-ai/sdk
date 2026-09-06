// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Zero Root AI

package daemonclient

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	daemonpb "github.com/zeroroot-ai/sdk/api/gen/gibson/daemon/v1"
	manifestpb "github.com/zeroroot-ai/sdk/api/gen/gibson/manifest/v1"
	missionpb "github.com/zeroroot-ai/sdk/api/gen/gibson/mission/v1"
)

// TestConvertProtoStatus tests the convertProtoStatus function.
func TestConvertProtoStatus(t *testing.T) {
	tests := []struct {
		name     string
		input    *daemonpb.StatusResponse
		expected *DaemonStatus
	}{
		{
			name:     "nil input",
			input:    nil,
			expected: nil,
		},
		{
			name: "complete status",
			input: &daemonpb.StatusResponse{
				Running:            true,
				Pid:                12345,
				StartTime:          1640000000,
				Uptime:             "2h30m15s",
				GrpcAddress:        "localhost:50002",
				RegistryType:       "embedded",
				RegistryAddr:       "embedded://localhost:2379",
				CallbackAddr:       "localhost:50001",
				AgentCount:         5,
				MissionCount:       10,
				ActiveMissionCount: 2,
			},
			expected: &DaemonStatus{
				Running:      true,
				PID:          12345,
				StartTime:    time.Unix(1640000000, 0),
				Uptime:       "2h30m15s",
				GRPCAddress:  "localhost:50002",
				RegistryType: "embedded",
				RegistryAddr: "embedded://localhost:2379",
				CallbackAddr: "localhost:50001",
				AgentCount:   5,
			},
		},
		{
			name: "zero values",
			input: &daemonpb.StatusResponse{
				Running:      false,
				Pid:          0,
				StartTime:    0,
				Uptime:       "",
				GrpcAddress:  "",
				RegistryType: "",
				RegistryAddr: "",
				CallbackAddr: "",
				AgentCount:   0,
			},
			expected: &DaemonStatus{
				Running:      false,
				PID:          0,
				StartTime:    time.Unix(0, 0),
				Uptime:       "",
				GRPCAddress:  "",
				RegistryType: "",
				RegistryAddr: "",
				CallbackAddr: "",
				AgentCount:   0,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := convertProtoStatus(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

// TestConvertProtoAgents tests the convertProtoAgents function.
func TestConvertProtoAgents(t *testing.T) {
	tests := []struct {
		name     string
		input    []*daemonpb.AgentInfo
		expected []AgentInfo
	}{
		{
			name:     "nil input",
			input:    nil,
			expected: []AgentInfo{},
		},
		{
			name:     "empty slice",
			input:    []*daemonpb.AgentInfo{},
			expected: []AgentInfo{},
		},
		{
			name: "single agent",
			input: []*daemonpb.AgentInfo{
				{
					Id:           "agent-1",
					Name:         "prompt-injection",
					Kind:         "agent",
					Version:      "1.0.0",
					Endpoint:     "localhost:50100",
					Capabilities: []string{"llm", "web"},
					Health:       "healthy",
					LastSeen:     1640000000,
				},
			},
			expected: []AgentInfo{
				{
					Name:        "prompt-injection",
					Version:     "1.0.0",
					Description: "",
					Address:     "localhost:50100",
					Status:      "healthy",
				},
			},
		},
		{
			name: "multiple agents",
			input: []*daemonpb.AgentInfo{
				{
					Name:     "agent-1",
					Version:  "1.0.0",
					Endpoint: "localhost:50100",
					Health:   "healthy",
				},
				{
					Name:     "agent-2",
					Version:  "2.0.0",
					Endpoint: "localhost:50101",
					Health:   "degraded",
				},
			},
			expected: []AgentInfo{
				{
					Name:        "agent-1",
					Version:     "1.0.0",
					Description: "",
					Address:     "localhost:50100",
					Status:      "healthy",
				},
				{
					Name:        "agent-2",
					Version:     "2.0.0",
					Description: "",
					Address:     "localhost:50101",
					Status:      "degraded",
				},
			},
		},
		{
			name: "nil elements are skipped",
			input: []*daemonpb.AgentInfo{
				{
					Name:     "agent-1",
					Version:  "1.0.0",
					Endpoint: "localhost:50100",
					Health:   "healthy",
				},
				nil,
				{
					Name:     "agent-2",
					Version:  "2.0.0",
					Endpoint: "localhost:50101",
					Health:   "healthy",
				},
			},
			expected: []AgentInfo{
				{
					Name:        "agent-1",
					Version:     "1.0.0",
					Description: "",
					Address:     "localhost:50100",
					Status:      "healthy",
				},
				{
					Name:        "agent-2",
					Version:     "2.0.0",
					Description: "",
					Address:     "localhost:50101",
					Status:      "healthy",
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := convertProtoAgents(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

// TestConvertProtoTools tests the convertProtoTools function.
func TestConvertProtoTools(t *testing.T) {
	tests := []struct {
		name     string
		input    []*daemonpb.ToolInfo
		expected []ToolInfo
	}{
		{
			name:     "nil input",
			input:    nil,
			expected: []ToolInfo{},
		},
		{
			name:     "empty slice",
			input:    []*daemonpb.ToolInfo{},
			expected: []ToolInfo{},
		},
		{
			name: "single tool",
			input: []*daemonpb.ToolInfo{
				{
					Id:          "tool-1",
					Name:        "mytool-a",
					Version:     "7.92",
					Endpoint:    "localhost:50200",
					Description: "Network scanner",
					Health:      "healthy",
					LastSeen:    1640000000,
				},
			},
			expected: []ToolInfo{
				{
					Name:        "mytool-a",
					Version:     "7.92",
					Description: "Network scanner",
					Address:     "localhost:50200",
					Status:      "healthy",
				},
			},
		},
		{
			name: "multiple tools with nil element",
			input: []*daemonpb.ToolInfo{
				{
					Name:        "mytool-a",
					Version:     "7.92",
					Description: "Network scanner",
					Endpoint:    "localhost:50200",
					Health:      "healthy",
				},
				nil,
				{
					Name:        "sqlmap",
					Version:     "1.5",
					Description: "SQL injection tool",
					Endpoint:    "localhost:50201",
					Health:      "healthy",
				},
			},
			expected: []ToolInfo{
				{
					Name:        "mytool-a",
					Version:     "7.92",
					Description: "Network scanner",
					Address:     "localhost:50200",
					Status:      "healthy",
				},
				{
					Name:        "sqlmap",
					Version:     "1.5",
					Description: "SQL injection tool",
					Address:     "localhost:50201",
					Status:      "healthy",
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := convertProtoTools(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

// TestConvertProtoPlugins tests the convertProtoPlugins function.
func TestConvertProtoPlugins(t *testing.T) {
	tests := []struct {
		name     string
		input    []*daemonpb.PluginInfo
		expected []PluginInfo
	}{
		{
			name:     "nil input",
			input:    nil,
			expected: []PluginInfo{},
		},
		{
			name:     "empty slice",
			input:    []*daemonpb.PluginInfo{},
			expected: []PluginInfo{},
		},
		{
			name: "single plugin",
			input: []*daemonpb.PluginInfo{
				{
					Id:          "plugin-1",
					Name:        "mitre-lookup",
					Version:     "1.0.0",
					Endpoint:    "localhost:50300",
					Description: "MITRE ATT&CK lookup plugin",
					Health:      "healthy",
					LastSeen:    1640000000,
				},
			},
			expected: []PluginInfo{
				{
					Name:        "mitre-lookup",
					Version:     "1.0.0",
					Description: "MITRE ATT&CK lookup plugin",
					Address:     "localhost:50300",
					Status:      "healthy",
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := convertProtoPlugins(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

// TestConvertProtoMissionEvent tests the convertProtoMissionEvent function.
func TestConvertProtoMissionEvent(t *testing.T) {
	tests := []struct {
		name     string
		input    *daemonpb.RunMissionResponse
		expected MissionEvent
	}{
		{
			name:     "nil input",
			input:    nil,
			expected: MissionEvent{},
		},
		{
			name: "complete mission event",
			input: &daemonpb.RunMissionResponse{
				EventType: "mission_started",
				Timestamp: 1640000000,
				MissionId: "mission-1",
				NodeId:    "node-1",
				Message:   "Mission started successfully",
				Data: mapToTypedMap(map[string]any{
					"mission": "attack.yaml",
				}),
				Error: "",
			},
			expected: MissionEvent{
				Type:      "mission_started",
				Timestamp: time.Unix(1640000000, 0),
				Message:   "Mission started successfully",
				Data: map[string]interface{}{
					"mission": "attack.yaml",
				},
			},
		},
		{
			name: "mission event with no data",
			input: &daemonpb.RunMissionResponse{
				EventType: "mission_completed",
				Timestamp: 1640000100,
				Message:   "Mission completed",
				Data:      nil,
			},
			expected: MissionEvent{
				Type:      "mission_completed",
				Timestamp: time.Unix(1640000100, 0),
				Message:   "Mission completed",
				Data:      nil,
			},
		},
		{
			name: "mission event with empty data",
			input: &daemonpb.RunMissionResponse{
				EventType: "mission.finding",
				Timestamp: 1640000200,
				Message:   "Found vulnerability",
				Data:      mapToTypedMap(map[string]any{}),
			},
			expected: MissionEvent{
				Type:      "mission.finding",
				Timestamp: time.Unix(1640000200, 0),
				Message:   "Found vulnerability",
				Data:      map[string]interface{}{}, // Empty map
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := convertProtoMissionEvent(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

// TestConvertProtoEvent tests the convertProtoEvent function.
func TestConvertProtoEvent(t *testing.T) {
	tests := []struct {
		name     string
		input    *daemonpb.SubscribeResponse
		expected Event
	}{
		{
			name:     "nil input",
			input:    nil,
			expected: Event{},
		},
		{
			name: "complete event",
			input: &daemonpb.SubscribeResponse{
				EventType: "agent_registered",
				Timestamp: 1640000000,
				Source:    "registry",
				Data: mapToTypedMap(map[string]any{
					"agent": "test-agent",
				}),
			},
			expected: Event{
				Type:      "agent_registered",
				Source:    "registry",
				Timestamp: time.Unix(1640000000, 0),
				Data: map[string]interface{}{
					"agent": "test-agent",
				},
			},
		},
		{
			name: "event with no data",
			input: &daemonpb.SubscribeResponse{
				EventType: "system_ready",
				Timestamp: 1640000100,
				Source:    "daemon",
				Data:      nil,
			},
			expected: Event{
				Type:      "system_ready",
				Source:    "daemon",
				Timestamp: time.Unix(1640000100, 0),
				Data:      nil,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := convertProtoEvent(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

// Mock DaemonServiceClient for testing client methods
type mockDaemonServiceClient struct {
	pingResponse        *daemonpb.PingResponse
	pingError           error
	statusResponse      *daemonpb.StatusResponse
	statusError         error
	listAgentsResponse  *daemonpb.ListAgentsResponse
	listAgentsError     error
	listToolsResponse   *daemonpb.ListToolsResponse
	listToolsError      error
	listPluginsResponse *daemonpb.ListPluginsResponse
	listPluginsError    error
}

func (m *mockDaemonServiceClient) Ping(ctx context.Context, req *daemonpb.PingRequest, opts ...grpc.CallOption) (*daemonpb.PingResponse, error) {
	return m.pingResponse, m.pingError
}

func (m *mockDaemonServiceClient) ListMyMemberships(ctx context.Context, req *daemonpb.ListMyMembershipsRequest, opts ...grpc.CallOption) (*daemonpb.ListMyMembershipsResponse, error) {
	return &daemonpb.ListMyMembershipsResponse{}, nil
}

func (m *mockDaemonServiceClient) RenewCapabilityGrant(ctx context.Context, req *daemonpb.RenewCapabilityGrantRequest, opts ...grpc.CallOption) (*daemonpb.RenewCapabilityGrantResponse, error) {
	return &daemonpb.RenewCapabilityGrantResponse{}, nil
}

func (m *mockDaemonServiceClient) ValidateMissionCUE(ctx context.Context, req *daemonpb.ValidateMissionCUERequest, opts ...grpc.CallOption) (*daemonpb.ValidateMissionCUEResponse, error) {
	return &daemonpb.ValidateMissionCUEResponse{}, nil
}

func (m *mockDaemonServiceClient) CompleteMissionCUE(ctx context.Context, req *daemonpb.CompleteMissionCUERequest, opts ...grpc.CallOption) (*daemonpb.CompleteMissionCUEResponse, error) {
	return &daemonpb.CompleteMissionCUEResponse{}, nil
}

func (m *mockDaemonServiceClient) HoverMissionCUE(ctx context.Context, req *daemonpb.HoverMissionCUERequest, opts ...grpc.CallOption) (*daemonpb.HoverMissionCUEResponse, error) {
	return &daemonpb.HoverMissionCUEResponse{}, nil
}

func (m *mockDaemonServiceClient) Status(ctx context.Context, req *daemonpb.StatusRequest, opts ...grpc.CallOption) (*daemonpb.StatusResponse, error) {
	return m.statusResponse, m.statusError
}

func (m *mockDaemonServiceClient) ListAgents(ctx context.Context, req *daemonpb.ListAgentsRequest, opts ...grpc.CallOption) (*daemonpb.ListAgentsResponse, error) {
	return m.listAgentsResponse, m.listAgentsError
}

func (m *mockDaemonServiceClient) ListTools(ctx context.Context, req *daemonpb.ListToolsRequest, opts ...grpc.CallOption) (*daemonpb.ListToolsResponse, error) {
	return m.listToolsResponse, m.listToolsError
}

func (m *mockDaemonServiceClient) ListPlugins(ctx context.Context, req *daemonpb.ListPluginsRequest, opts ...grpc.CallOption) (*daemonpb.ListPluginsResponse, error) {
	return m.listPluginsResponse, m.listPluginsError
}

// Stub implementations for other methods (not tested in this file)
func (m *mockDaemonServiceClient) Connect(ctx context.Context, req *daemonpb.ConnectRequest, opts ...grpc.CallOption) (*daemonpb.ConnectResponse, error) {
	return nil, nil
}
func (m *mockDaemonServiceClient) RunMission(ctx context.Context, req *daemonpb.RunMissionRequest, opts ...grpc.CallOption) (grpc.ServerStreamingClient[daemonpb.RunMissionResponse], error) {
	return nil, nil
}
func (m *mockDaemonServiceClient) StopMission(ctx context.Context, req *daemonpb.StopMissionRequest, opts ...grpc.CallOption) (*daemonpb.StopMissionResponse, error) {
	return nil, nil
}
func (m *mockDaemonServiceClient) ListMissions(ctx context.Context, req *daemonpb.ListMissionsRequest, opts ...grpc.CallOption) (*daemonpb.ListMissionsResponse, error) {
	return nil, nil
}
func (m *mockDaemonServiceClient) GetAgentStatus(ctx context.Context, req *daemonpb.GetAgentStatusRequest, opts ...grpc.CallOption) (*daemonpb.GetAgentStatusResponse, error) {
	return nil, nil
}
func (m *mockDaemonServiceClient) Subscribe(ctx context.Context, req *daemonpb.SubscribeRequest, opts ...grpc.CallOption) (grpc.ServerStreamingClient[daemonpb.SubscribeResponse], error) {
	return nil, nil
}
func (m *mockDaemonServiceClient) StartComponent(ctx context.Context, req *daemonpb.StartComponentRequest, opts ...grpc.CallOption) (*daemonpb.StartComponentResponse, error) {
	return nil, nil
}
func (m *mockDaemonServiceClient) StopComponent(ctx context.Context, req *daemonpb.StopComponentRequest, opts ...grpc.CallOption) (*daemonpb.StopComponentResponse, error) {
	return nil, nil
}
func (m *mockDaemonServiceClient) PauseMission(ctx context.Context, req *daemonpb.PauseMissionRequest, opts ...grpc.CallOption) (*daemonpb.PauseMissionResponse, error) {
	return nil, nil
}
func (m *mockDaemonServiceClient) ResumeMission(ctx context.Context, req *daemonpb.ResumeMissionRequest, opts ...grpc.CallOption) (grpc.ServerStreamingClient[daemonpb.ResumeMissionResponse], error) {
	return nil, nil
}
func (m *mockDaemonServiceClient) GetMissionHistory(ctx context.Context, req *daemonpb.GetMissionHistoryRequest, opts ...grpc.CallOption) (*daemonpb.GetMissionHistoryResponse, error) {
	return nil, nil
}
func (m *mockDaemonServiceClient) QueryPlugin(ctx context.Context, req *daemonpb.QueryPluginRequest, opts ...grpc.CallOption) (*daemonpb.QueryPluginResponse, error) {
	return nil, nil
}
func (m *mockDaemonServiceClient) BuildComponent(ctx context.Context, req *daemonpb.BuildComponentRequest, opts ...grpc.CallOption) (*daemonpb.BuildComponentResponse, error) {
	return nil, nil
}
func (m *mockDaemonServiceClient) ShowComponent(ctx context.Context, req *daemonpb.ShowComponentRequest, opts ...grpc.CallOption) (*daemonpb.ShowComponentResponse, error) {
	return nil, nil
}
func (m *mockDaemonServiceClient) GetComponentLogs(ctx context.Context, req *daemonpb.GetComponentLogsRequest, opts ...grpc.CallOption) (grpc.ServerStreamingClient[daemonpb.GetComponentLogsResponse], error) {
	return nil, nil
}
func (m *mockDaemonServiceClient) ListMissionDefinitions(ctx context.Context, req *daemonpb.ListMissionDefinitionsRequest, opts ...grpc.CallOption) (*daemonpb.ListMissionDefinitionsResponse, error) {
	return nil, nil
}
func (m *mockDaemonServiceClient) GetMissionDefinition(ctx context.Context, req *daemonpb.GetMissionDefinitionRequest, opts ...grpc.CallOption) (*daemonpb.GetMissionDefinitionResponse, error) {
	return nil, nil
}
func (m *mockDaemonServiceClient) CreateMission(ctx context.Context, req *daemonpb.CreateMissionRequest, opts ...grpc.CallOption) (*daemonpb.CreateMissionResponse, error) {
	return nil, nil
}

func (m *mockDaemonServiceClient) CreateTarget(ctx context.Context, req *daemonpb.CreateTargetRequest, opts ...grpc.CallOption) (*daemonpb.CreateTargetResponse, error) {
	return nil, nil
}

func (m *mockDaemonServiceClient) GetTarget(ctx context.Context, req *daemonpb.GetTargetRequest, opts ...grpc.CallOption) (*daemonpb.GetTargetResponse, error) {
	return nil, nil
}

func (m *mockDaemonServiceClient) ListTargets(ctx context.Context, req *daemonpb.ListTargetsRequest, opts ...grpc.CallOption) (*daemonpb.ListTargetsResponse, error) {
	return nil, nil
}

func (m *mockDaemonServiceClient) UpdateTarget(ctx context.Context, req *daemonpb.UpdateTargetRequest, opts ...grpc.CallOption) (*daemonpb.UpdateTargetResponse, error) {
	return nil, nil
}

func (m *mockDaemonServiceClient) DeleteTarget(ctx context.Context, req *daemonpb.DeleteTargetRequest, opts ...grpc.CallOption) (*daemonpb.DeleteTargetResponse, error) {
	return nil, nil
}

func (m *mockDaemonServiceClient) CreateMissionDefinition(ctx context.Context, req *daemonpb.CreateMissionDefinitionRequest, opts ...grpc.CallOption) (*daemonpb.CreateMissionDefinitionResponse, error) {
	return &daemonpb.CreateMissionDefinitionResponse{
		MissionDefinitionId: "test-def-id",
		Info:                &daemonpb.MissionDefinitionInfo{Name: req.GetDefinition().GetName()},
	}, nil
}

func (m *mockDaemonServiceClient) UpdateMissionDefinition(ctx context.Context, req *daemonpb.UpdateMissionDefinitionRequest, opts ...grpc.CallOption) (*daemonpb.UpdateMissionDefinitionResponse, error) {
	return nil, nil
}
func (m *mockDaemonServiceClient) GetMissionGraph(ctx context.Context, req *daemonpb.GetMissionGraphRequest, opts ...grpc.CallOption) (*daemonpb.GetMissionGraphResponse, error) {
	return nil, nil
}
func (m *mockDaemonServiceClient) GetMissionLayout(ctx context.Context, req *daemonpb.GetMissionLayoutRequest, opts ...grpc.CallOption) (*daemonpb.GetMissionLayoutResponse, error) {
	return nil, nil
}
func (m *mockDaemonServiceClient) SaveMissionLayout(ctx context.Context, req *daemonpb.SaveMissionLayoutRequest, opts ...grpc.CallOption) (*daemonpb.SaveMissionLayoutResponse, error) {
	return nil, nil
}

func (m *mockDaemonServiceClient) GetMyPermissions(ctx context.Context, req *daemonpb.GetMyPermissionsRequest, opts ...grpc.CallOption) (*daemonpb.GetMyPermissionsResponse, error) {
	return nil, nil
}
func (m *mockDaemonServiceClient) GetCapabilityManifest(ctx context.Context, req *manifestpb.GetCapabilityManifestRequest, opts ...grpc.CallOption) (*manifestpb.GetCapabilityManifestResponse, error) {
	return nil, nil
}
func (m *mockDaemonServiceClient) WatchManifestInvalidations(ctx context.Context, req *manifestpb.WatchManifestInvalidationsRequest, opts ...grpc.CallOption) (grpc.ServerStreamingClient[manifestpb.WatchManifestInvalidationsResponse], error) {
	return nil, nil
}

// TestClient_Ping tests the Ping method with mock client.
func TestClient_Ping(t *testing.T) {
	tests := []struct {
		name          string
		mockResponse  *daemonpb.PingResponse
		mockError     error
		expectedError string
	}{
		{
			name: "successful ping",
			mockResponse: &daemonpb.PingResponse{
				Timestamp: time.Now().Unix(),
			},
			mockError:     nil,
			expectedError: "",
		},
		{
			name:          "unavailable error",
			mockResponse:  nil,
			mockError:     status.Error(codes.Unavailable, "connection refused"),
			expectedError: "daemon not responding (connection unavailable)",
		},
		{
			name:          "deadline exceeded error",
			mockResponse:  nil,
			mockError:     status.Error(codes.DeadlineExceeded, "timeout"),
			expectedError: "daemon ping timeout",
		},
		{
			name:          "internal error",
			mockResponse:  nil,
			mockError:     status.Error(codes.Internal, "server panic"),
			expectedError: "daemon ping failed: server panic",
		},
		{
			name:          "generic error",
			mockResponse:  nil,
			mockError:     assert.AnError,
			expectedError: "daemon ping failed:",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mock := &mockDaemonServiceClient{
				pingResponse: tt.mockResponse,
				pingError:    tt.mockError,
			}

			client := &Client{
				daemon: mock,
			}

			ctx := context.Background()
			err := client.Ping(ctx)

			if tt.expectedError == "" {
				assert.NoError(t, err)
			} else {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tt.expectedError)
			}
		})
	}
}

// TestClient_Status tests the Status method with mock client.
func TestClient_Status(t *testing.T) {
	tests := []struct {
		name          string
		mockResponse  *daemonpb.StatusResponse
		mockError     error
		expectedError string
	}{
		{
			name: "successful status",
			mockResponse: &daemonpb.StatusResponse{
				Running:      true,
				Pid:          12345,
				StartTime:    time.Now().Unix(),
				Uptime:       "1h30m",
				GrpcAddress:  "localhost:50002",
				RegistryType: "embedded",
				AgentCount:   5,
			},
			mockError:     nil,
			expectedError: "",
		},
		{
			name:          "unavailable error",
			mockResponse:  nil,
			mockError:     status.Error(codes.Unavailable, "connection refused"),
			expectedError: "daemon not responding (is it running?)",
		},
		{
			name:          "deadline exceeded error",
			mockResponse:  nil,
			mockError:     status.Error(codes.DeadlineExceeded, "timeout"),
			expectedError: "daemon status request timeout",
		},
		{
			name:          "internal error",
			mockResponse:  nil,
			mockError:     status.Error(codes.Internal, "database error"),
			expectedError: "failed to get daemon status: database error",
		},
		{
			name:          "generic error",
			mockResponse:  nil,
			mockError:     assert.AnError,
			expectedError: "failed to get daemon status:",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mock := &mockDaemonServiceClient{
				statusResponse: tt.mockResponse,
				statusError:    tt.mockError,
			}

			client := &Client{
				daemon: mock,
			}

			ctx := context.Background()
			result, err := client.Status(ctx)

			if tt.expectedError == "" {
				assert.NoError(t, err)
				assert.NotNil(t, result)
				assert.Equal(t, tt.mockResponse.Running, result.Running)
				assert.Equal(t, int(tt.mockResponse.Pid), result.PID)
			} else {
				assert.Error(t, err)
				assert.Nil(t, result)
				assert.Contains(t, err.Error(), tt.expectedError)
			}
		})
	}
}

// TestClient_ListAgents tests the ListAgents method with mock client.
func TestClient_ListAgents(t *testing.T) {
	tests := []struct {
		name          string
		mockResponse  *daemonpb.ListAgentsResponse
		mockError     error
		expectedCount int
		expectedError string
	}{
		{
			name: "successful list with agents",
			mockResponse: &daemonpb.ListAgentsResponse{
				Agents: []*daemonpb.AgentInfo{
					{
						Name:     "agent-1",
						Version:  "1.0.0",
						Endpoint: "localhost:50100",
						Health:   "healthy",
					},
					{
						Name:     "agent-2",
						Version:  "2.0.0",
						Endpoint: "localhost:50101",
						Health:   "healthy",
					},
				},
			},
			mockError:     nil,
			expectedCount: 2,
			expectedError: "",
		},
		{
			name: "successful list with empty results",
			mockResponse: &daemonpb.ListAgentsResponse{
				Agents: []*daemonpb.AgentInfo{},
			},
			mockError:     nil,
			expectedCount: 0,
			expectedError: "",
		},
		{
			name:          "unavailable error",
			mockResponse:  nil,
			mockError:     status.Error(codes.Unavailable, "connection refused"),
			expectedCount: 0,
			expectedError: "daemon not responding (is it running?)",
		},
		{
			name:          "deadline exceeded error",
			mockResponse:  nil,
			mockError:     status.Error(codes.DeadlineExceeded, "timeout"),
			expectedCount: 0,
			expectedError: "daemon request timeout while listing agents",
		},
		{
			name:          "internal error",
			mockResponse:  nil,
			mockError:     status.Error(codes.Internal, "registry error"),
			expectedCount: 0,
			expectedError: "failed to list agents: registry error",
		},
		{
			name:          "not found error",
			mockResponse:  nil,
			mockError:     status.Error(codes.NotFound, "no agents found"),
			expectedCount: 0,
			expectedError: "failed to list agents: no agents found",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mock := &mockDaemonServiceClient{
				listAgentsResponse: tt.mockResponse,
				listAgentsError:    tt.mockError,
			}

			client := &Client{
				daemon: mock,
			}

			ctx := context.Background()
			result, err := client.ListAgents(ctx)

			if tt.expectedError == "" {
				assert.NoError(t, err)
				assert.NotNil(t, result)
				assert.Len(t, result, tt.expectedCount)
			} else {
				assert.Error(t, err)
				assert.Nil(t, result)
				assert.Contains(t, err.Error(), tt.expectedError)
			}
		})
	}
}

// TestClient_ListTools tests the ListTools method with mock client.
func TestClient_ListTools(t *testing.T) {
	tests := []struct {
		name          string
		mockResponse  *daemonpb.ListToolsResponse
		mockError     error
		expectedCount int
		expectedError string
	}{
		{
			name: "successful list with tools",
			mockResponse: &daemonpb.ListToolsResponse{
				Tools: []*daemonpb.ToolInfo{
					{
						Name:        "mytool-a",
						Version:     "7.92",
						Description: "Network scanner",
						Endpoint:    "localhost:50200",
						Health:      "healthy",
					},
				},
			},
			mockError:     nil,
			expectedCount: 1,
			expectedError: "",
		},
		{
			name: "empty tools list",
			mockResponse: &daemonpb.ListToolsResponse{
				Tools: []*daemonpb.ToolInfo{},
			},
			mockError:     nil,
			expectedCount: 0,
			expectedError: "",
		},
		{
			name:          "unavailable error",
			mockResponse:  nil,
			mockError:     status.Error(codes.Unavailable, "connection refused"),
			expectedCount: 0,
			expectedError: "daemon not responding (is it running?)",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mock := &mockDaemonServiceClient{
				listToolsResponse: tt.mockResponse,
				listToolsError:    tt.mockError,
			}

			client := &Client{
				daemon: mock,
			}

			ctx := context.Background()
			result, err := client.ListTools(ctx)

			if tt.expectedError == "" {
				assert.NoError(t, err)
				assert.NotNil(t, result)
				assert.Len(t, result, tt.expectedCount)
			} else {
				assert.Error(t, err)
				assert.Nil(t, result)
				assert.Contains(t, err.Error(), tt.expectedError)
			}
		})
	}
}

// TestClient_ListPlugins tests the ListPlugins method with mock client.
func TestClient_ListPlugins(t *testing.T) {
	tests := []struct {
		name          string
		mockResponse  *daemonpb.ListPluginsResponse
		mockError     error
		expectedCount int
		expectedError string
	}{
		{
			name: "successful list with plugins",
			mockResponse: &daemonpb.ListPluginsResponse{
				Plugins: []*daemonpb.PluginInfo{
					{
						Name:        "mitre-lookup",
						Version:     "1.0.0",
						Description: "MITRE ATT&CK lookup",
						Endpoint:    "localhost:50300",
						Health:      "healthy",
					},
				},
			},
			mockError:     nil,
			expectedCount: 1,
			expectedError: "",
		},
		{
			name: "empty plugins list",
			mockResponse: &daemonpb.ListPluginsResponse{
				Plugins: []*daemonpb.PluginInfo{},
			},
			mockError:     nil,
			expectedCount: 0,
			expectedError: "",
		},
		{
			name:          "unavailable error",
			mockResponse:  nil,
			mockError:     status.Error(codes.Unavailable, "connection refused"),
			expectedCount: 0,
			expectedError: "daemon not responding (is it running?)",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mock := &mockDaemonServiceClient{
				listPluginsResponse: tt.mockResponse,
				listPluginsError:    tt.mockError,
			}

			client := &Client{
				daemon: mock,
			}

			ctx := context.Background()
			result, err := client.ListPlugins(ctx)

			if tt.expectedError == "" {
				assert.NoError(t, err)
				assert.NotNil(t, result)
				assert.Len(t, result, tt.expectedCount)
			} else {
				assert.Error(t, err)
				assert.Nil(t, result)
				assert.Contains(t, err.Error(), tt.expectedError)
			}
		})
	}
}

func TestConnect_InvalidAddress(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel()

	// Test empty address
	_, err := Connect(ctx, "")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "cannot be empty")
}

func TestConnect_UnixSocketFormat(t *testing.T) {
	// Provide fake OIDC env so credential detection succeeds and the dial
	// attempt reaches the network/socket layer, which then fails as expected.

	tests := []struct {
		name    string
		address string
		wantErr bool
	}{
		{
			name:    "unix scheme with absolute path",
			address: "unix:///nonexistent/socket",
			wantErr: true, // Connection will fail since socket doesn't exist
		},
		{
			name:    "absolute path without scheme",
			address: "/nonexistent/socket",
			wantErr: true, // Connection will fail since socket doesn't exist
		},
		{
			name:    "tcp localhost",
			address: "localhost:59900",
			wantErr: true, // Connection will fail since nothing is listening
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
			defer cancel()

			client, err := Connect(ctx, tt.address)
			if tt.wantErr {
				assert.Error(t, err)
				assert.Nil(t, client)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, client)
				client.Close()
			}
		})
	}
}

// TestClient_CreateMissionDefinition verifies the client forwards the proto
// MissionDefinition to the daemon and returns the server-assigned ID.
func TestClient_CreateMissionDefinition(t *testing.T) {
	mock := &mockDaemonServiceClient{}
	client := &Client{daemon: mock}

	def := &missionpb.MissionDefinition{Name: "hello"}
	result, err := client.CreateMissionDefinition(context.Background(), def)
	assert.NoError(t, err)
	assert.Equal(t, "test-def-id", result.MissionDefinitionID)
	assert.Equal(t, "hello", result.Info.Name)
}

// TestClient_CreateMissionDefinition_NilDefinition verifies the client rejects
// a nil MissionDefinition before hitting the wire.
func TestClient_CreateMissionDefinition_NilDefinition(t *testing.T) {
	mock := &mockDaemonServiceClient{}
	client := &Client{daemon: mock}

	_, err := client.CreateMissionDefinition(context.Background(), nil)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "definition is required")
}
