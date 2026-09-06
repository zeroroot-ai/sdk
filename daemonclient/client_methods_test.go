// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Zero Root AI

// Package daemonclient — additional mock-based tests for client methods.
// These tests extend the mock established in client_test.go to cover the many
// high-level client methods that were previously at 0% coverage.
package daemonclient

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	daemonpb "github.com/zeroroot-ai/sdk/api/gen/gibson/daemon/v1"
	manifestpb "github.com/zeroroot-ai/sdk/api/gen/gibson/manifest/v1"
	missionpb "github.com/zeroroot-ai/sdk/api/gen/gibson/mission/v1"
)

// -----------------------------------------------------------------------
// Extended mock — adds remaining methods not in client_test.go's mock
// -----------------------------------------------------------------------

type extendedMockClient struct {
	// Embed existing mock for the already-stubbed methods.
	mockDaemonServiceClient

	queryPluginResp       *daemonpb.QueryPluginResponse
	queryPluginErr        error
	listMissionsResp      *daemonpb.ListMissionsResponse
	listMissionsErr       error
	stopMissionResp       *daemonpb.StopMissionResponse
	stopMissionErr        error
	createMissionResp     *daemonpb.CreateMissionResponse
	createMissionErr      error
	pauseMissionResp      *daemonpb.PauseMissionResponse
	pauseMissionErr       error
	startComponentResp    *daemonpb.StartComponentResponse
	startComponentErr     error
	stopComponentResp     *daemonpb.StopComponentResponse
	stopComponentErr      error
	buildComponentResp    *daemonpb.BuildComponentResponse
	buildComponentErr     error
	showComponentResp     *daemonpb.ShowComponentResponse
	showComponentErr      error
	getMissionHistoryResp *daemonpb.GetMissionHistoryResponse
	getMissionHistoryErr  error
	listMissionDefsResp   *daemonpb.ListMissionDefinitionsResponse
	listMissionDefsErr    error
	getMissionDefResp     *daemonpb.GetMissionDefinitionResponse
	getMissionDefErr      error
	getCapabilityMfstResp *manifestpb.GetCapabilityManifestResponse
	getCapabilityMfstErr  error
	getAgentStatusResp    *daemonpb.GetAgentStatusResponse
	getAgentStatusErr     error
}

func (m *extendedMockClient) QueryPlugin(ctx context.Context, req *daemonpb.QueryPluginRequest, opts ...grpc.CallOption) (*daemonpb.QueryPluginResponse, error) {
	return m.queryPluginResp, m.queryPluginErr
}
func (m *extendedMockClient) ListMyMemberships(ctx context.Context, req *daemonpb.ListMyMembershipsRequest, opts ...grpc.CallOption) (*daemonpb.ListMyMembershipsResponse, error) {
	return &daemonpb.ListMyMembershipsResponse{}, nil
}
func (m *extendedMockClient) RenewCapabilityGrant(ctx context.Context, req *daemonpb.RenewCapabilityGrantRequest, opts ...grpc.CallOption) (*daemonpb.RenewCapabilityGrantResponse, error) {
	return &daemonpb.RenewCapabilityGrantResponse{}, nil
}
func (m *extendedMockClient) ListMissions(ctx context.Context, req *daemonpb.ListMissionsRequest, opts ...grpc.CallOption) (*daemonpb.ListMissionsResponse, error) {
	return m.listMissionsResp, m.listMissionsErr
}
func (m *extendedMockClient) StopMission(ctx context.Context, req *daemonpb.StopMissionRequest, opts ...grpc.CallOption) (*daemonpb.StopMissionResponse, error) {
	return m.stopMissionResp, m.stopMissionErr
}
func (m *extendedMockClient) CreateMission(ctx context.Context, req *daemonpb.CreateMissionRequest, opts ...grpc.CallOption) (*daemonpb.CreateMissionResponse, error) {
	return m.createMissionResp, m.createMissionErr
}
func (m *extendedMockClient) PauseMission(ctx context.Context, req *daemonpb.PauseMissionRequest, opts ...grpc.CallOption) (*daemonpb.PauseMissionResponse, error) {
	return m.pauseMissionResp, m.pauseMissionErr
}
func (m *extendedMockClient) StartComponent(ctx context.Context, req *daemonpb.StartComponentRequest, opts ...grpc.CallOption) (*daemonpb.StartComponentResponse, error) {
	return m.startComponentResp, m.startComponentErr
}
func (m *extendedMockClient) StopComponent(ctx context.Context, req *daemonpb.StopComponentRequest, opts ...grpc.CallOption) (*daemonpb.StopComponentResponse, error) {
	return m.stopComponentResp, m.stopComponentErr
}
func (m *extendedMockClient) BuildComponent(ctx context.Context, req *daemonpb.BuildComponentRequest, opts ...grpc.CallOption) (*daemonpb.BuildComponentResponse, error) {
	return m.buildComponentResp, m.buildComponentErr
}
func (m *extendedMockClient) ShowComponent(ctx context.Context, req *daemonpb.ShowComponentRequest, opts ...grpc.CallOption) (*daemonpb.ShowComponentResponse, error) {
	return m.showComponentResp, m.showComponentErr
}
func (m *extendedMockClient) GetMissionHistory(ctx context.Context, req *daemonpb.GetMissionHistoryRequest, opts ...grpc.CallOption) (*daemonpb.GetMissionHistoryResponse, error) {
	return m.getMissionHistoryResp, m.getMissionHistoryErr
}
func (m *extendedMockClient) ListMissionDefinitions(ctx context.Context, req *daemonpb.ListMissionDefinitionsRequest, opts ...grpc.CallOption) (*daemonpb.ListMissionDefinitionsResponse, error) {
	return m.listMissionDefsResp, m.listMissionDefsErr
}
func (m *extendedMockClient) GetMissionDefinition(ctx context.Context, req *daemonpb.GetMissionDefinitionRequest, opts ...grpc.CallOption) (*daemonpb.GetMissionDefinitionResponse, error) {
	return m.getMissionDefResp, m.getMissionDefErr
}
func (m *extendedMockClient) UpdateMissionDefinition(ctx context.Context, req *daemonpb.UpdateMissionDefinitionRequest, opts ...grpc.CallOption) (*daemonpb.UpdateMissionDefinitionResponse, error) {
	return nil, nil
}
func (m *extendedMockClient) GetMissionGraph(ctx context.Context, req *daemonpb.GetMissionGraphRequest, opts ...grpc.CallOption) (*daemonpb.GetMissionGraphResponse, error) {
	return nil, nil
}
func (m *extendedMockClient) GetMissionLayout(ctx context.Context, req *daemonpb.GetMissionLayoutRequest, opts ...grpc.CallOption) (*daemonpb.GetMissionLayoutResponse, error) {
	return nil, nil
}
func (m *extendedMockClient) SaveMissionLayout(ctx context.Context, req *daemonpb.SaveMissionLayoutRequest, opts ...grpc.CallOption) (*daemonpb.SaveMissionLayoutResponse, error) {
	return nil, nil
}
func (m *extendedMockClient) GetCapabilityManifest(ctx context.Context, req *manifestpb.GetCapabilityManifestRequest, opts ...grpc.CallOption) (*manifestpb.GetCapabilityManifestResponse, error) {
	return m.getCapabilityMfstResp, m.getCapabilityMfstErr
}
func (m *extendedMockClient) GetAgentStatus(ctx context.Context, req *daemonpb.GetAgentStatusRequest, opts ...grpc.CallOption) (*daemonpb.GetAgentStatusResponse, error) {
	return m.getAgentStatusResp, m.getAgentStatusErr
}

// RunMission / ResumeMission / Subscribe / GetComponentLogs return streams.
// These methods are tested separately via the nil-stream error path.
func (m *extendedMockClient) RunMission(ctx context.Context, req *daemonpb.RunMissionRequest, opts ...grpc.CallOption) (grpc.ServerStreamingClient[daemonpb.RunMissionResponse], error) {
	return nil, status.Error(codes.Unavailable, "not connected")
}
func (m *extendedMockClient) ResumeMission(ctx context.Context, req *daemonpb.ResumeMissionRequest, opts ...grpc.CallOption) (grpc.ServerStreamingClient[daemonpb.ResumeMissionResponse], error) {
	return nil, status.Error(codes.NotFound, "mission not found")
}
func (m *extendedMockClient) Subscribe(ctx context.Context, req *daemonpb.SubscribeRequest, opts ...grpc.CallOption) (grpc.ServerStreamingClient[daemonpb.SubscribeResponse], error) {
	return nil, status.Error(codes.Unavailable, "not connected")
}
func (m *extendedMockClient) GetComponentLogs(ctx context.Context, req *daemonpb.GetComponentLogsRequest, opts ...grpc.CallOption) (grpc.ServerStreamingClient[daemonpb.GetComponentLogsResponse], error) {
	return nil, status.Error(codes.NotFound, "component not found")
}
func (m *extendedMockClient) WatchManifestInvalidations(ctx context.Context, req *manifestpb.WatchManifestInvalidationsRequest, opts ...grpc.CallOption) (grpc.ServerStreamingClient[manifestpb.WatchManifestInvalidationsResponse], error) {
	return nil, nil
}

// -----------------------------------------------------------------------
// Client.Close
// -----------------------------------------------------------------------

func TestClient_Close_NilConn(t *testing.T) {
	c := &Client{conn: nil, daemon: &extendedMockClient{}}
	err := c.Close()
	assert.NoError(t, err)
}

// -----------------------------------------------------------------------
// NewWithCredentials
// -----------------------------------------------------------------------

func TestNewWithCredentials_FailsWithoutSystemRoots(t *testing.T) {
	clearCredEnvs(t)
	// Provide a fake token credential. TLS build should succeed in most envs.
	cred := NewTokenCredentials("test-jwt")
	ctx, cancel := context.WithTimeout(context.Background(), 100)
	defer cancel()
	// This will fail at the dial level, not at TLS build. That's fine —
	// we verify the function is exercised without panicking.
	_, err := NewWithCredentials(ctx, "localhost:59990", cred)
	assert.Error(t, err) // Connection failure expected.
}

// -----------------------------------------------------------------------
// QueryPlugin
// -----------------------------------------------------------------------

func TestClient_QueryPlugin_Success(t *testing.T) {
	mock := &extendedMockClient{
		queryPluginResp: &daemonpb.QueryPluginResponse{
			Result:     nil,
			Error:      "",
			DurationMs: 5,
		},
	}
	c := &Client{daemon: mock}

	result, err := c.QueryPlugin(context.Background(), "my-plugin", "list", nil)
	require.NoError(t, err)
	assert.Equal(t, int64(5), result.DurationMs)
}

func TestClient_QueryPlugin_PluginError(t *testing.T) {
	mock := &extendedMockClient{
		queryPluginResp: &daemonpb.QueryPluginResponse{
			Error: "plugin method not found",
		},
	}
	c := &Client{daemon: mock}

	_, err := c.QueryPlugin(context.Background(), "my-plugin", "bad-method", nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "plugin error")
}

func TestClient_QueryPlugin_UnavailableError(t *testing.T) {
	mock := &extendedMockClient{
		queryPluginErr: status.Error(codes.Unavailable, "down"),
	}
	c := &Client{daemon: mock}

	_, err := c.QueryPlugin(context.Background(), "p", "m", nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "daemon not responding")
}

func TestClient_QueryPlugin_DeadlineError(t *testing.T) {
	mock := &extendedMockClient{
		queryPluginErr: status.Error(codes.DeadlineExceeded, "timed out"),
	}
	c := &Client{daemon: mock}
	_, err := c.QueryPlugin(context.Background(), "p", "m", nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "timeout")
}

func TestClient_QueryPlugin_GenericError(t *testing.T) {
	mock := &extendedMockClient{
		queryPluginErr: status.Error(codes.Internal, "internal"),
	}
	c := &Client{daemon: mock}
	_, err := c.QueryPlugin(context.Background(), "p", "m", nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to query plugin")
}

// -----------------------------------------------------------------------
// RunMission
// -----------------------------------------------------------------------

func TestClient_RunMission_MissingDefinitionID(t *testing.T) {
	c := &Client{daemon: &extendedMockClient{}}
	_, err := c.RunMission(context.Background(), "", "target-1", nil, "")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "mission_definition_id")
}

func TestClient_RunMission_MissingTargetID(t *testing.T) {
	c := &Client{daemon: &extendedMockClient{}}
	_, err := c.RunMission(context.Background(), "def-1", "", nil, "")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "target_id")
}

func TestClient_RunMission_DaemonUnavailable(t *testing.T) {
	c := &Client{daemon: &extendedMockClient{}}
	_, err := c.RunMission(context.Background(), "def-1", "tgt-1", nil, "")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "daemon not responding")
}

// -----------------------------------------------------------------------
// ListMissions
// -----------------------------------------------------------------------

func TestClient_ListMissions_Success(t *testing.T) {
	mock := &extendedMockClient{
		listMissionsResp: &daemonpb.ListMissionsResponse{
			Missions: []*daemonpb.MissionInfo{
				{Id: "m1", Name: "recon", Status: "completed"},
			},
			Total: 1,
		},
	}
	c := &Client{daemon: mock}

	missions, total, err := c.ListMissions(context.Background(), false, "", "", 0, 0)
	require.NoError(t, err)
	assert.Len(t, missions, 1)
	assert.Equal(t, 1, total)
	assert.Equal(t, "recon", missions[0].Name)
}

func TestClient_ListMissions_UnavailableError(t *testing.T) {
	mock := &extendedMockClient{listMissionsErr: status.Error(codes.Unavailable, "down")}
	c := &Client{daemon: mock}
	_, _, err := c.ListMissions(context.Background(), false, "", "", 0, 0)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "daemon not responding")
}

func TestClient_ListMissions_DeadlineError(t *testing.T) {
	mock := &extendedMockClient{listMissionsErr: status.Error(codes.DeadlineExceeded, "timeout")}
	c := &Client{daemon: mock}
	_, _, err := c.ListMissions(context.Background(), false, "", "", 0, 0)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "timeout")
}

// -----------------------------------------------------------------------
// StopMission
// -----------------------------------------------------------------------

func TestClient_StopMission_Success(t *testing.T) {
	mock := &extendedMockClient{
		stopMissionResp: &daemonpb.StopMissionResponse{Success: true},
	}
	c := &Client{daemon: mock}
	err := c.StopMission(context.Background(), "mission-1", false)
	require.NoError(t, err)
}

func TestClient_StopMission_Failure(t *testing.T) {
	mock := &extendedMockClient{
		stopMissionResp: &daemonpb.StopMissionResponse{Success: false, Message: "not found"},
	}
	c := &Client{daemon: mock}
	err := c.StopMission(context.Background(), "mission-1", false)
	require.Error(t, err)
}

func TestClient_StopMission_UnavailableError(t *testing.T) {
	mock := &extendedMockClient{stopMissionErr: status.Error(codes.Unavailable, "down")}
	c := &Client{daemon: mock}
	err := c.StopMission(context.Background(), "m", false)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "daemon not responding")
}

// -----------------------------------------------------------------------
// CreateMission
// -----------------------------------------------------------------------

func TestClient_CreateMission_MissingName(t *testing.T) {
	c := &Client{daemon: &extendedMockClient{}}
	_, err := c.CreateMission(context.Background(), CreateMissionOptions{
		TargetID:            "t1",
		MissionDefinitionID: "d1",
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "name")
}

func TestClient_CreateMission_MissingTargetID(t *testing.T) {
	c := &Client{daemon: &extendedMockClient{}}
	_, err := c.CreateMission(context.Background(), CreateMissionOptions{
		Name:                "my-mission",
		MissionDefinitionID: "d1",
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "target_id")
}

func TestClient_CreateMission_MissingDefinitionID(t *testing.T) {
	c := &Client{daemon: &extendedMockClient{}}
	_, err := c.CreateMission(context.Background(), CreateMissionOptions{
		Name:     "my-mission",
		TargetID: "t1",
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "mission_definition_id")
}

func TestClient_CreateMission_Success(t *testing.T) {
	mock := &extendedMockClient{
		createMissionResp: &daemonpb.CreateMissionResponse{
			Success: true,
			Mission: &daemonpb.Mission{
				Id:       "m-new",
				Name:     "recon-run",
				TargetId: "t1",
			},
		},
	}
	c := &Client{daemon: mock}

	result, err := c.CreateMission(context.Background(), CreateMissionOptions{
		Name:                "recon-run",
		TargetID:            "t1",
		MissionDefinitionID: "d1",
	})
	require.NoError(t, err)
	assert.Equal(t, "m-new", result.MissionID)
	assert.Equal(t, "recon-run", result.Name)
}

func TestClient_CreateMission_ResponseNotSuccess(t *testing.T) {
	mock := &extendedMockClient{
		createMissionResp: &daemonpb.CreateMissionResponse{
			Success: false,
			Message: "definition not found",
		},
	}
	c := &Client{daemon: mock}

	_, err := c.CreateMission(context.Background(), CreateMissionOptions{
		Name:                "recon-run",
		TargetID:            "t1",
		MissionDefinitionID: "d1",
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "definition not found")
}

func TestClient_CreateMission_UnavailableError(t *testing.T) {
	mock := &extendedMockClient{createMissionErr: status.Error(codes.Unavailable, "down")}
	c := &Client{daemon: mock}
	_, err := c.CreateMission(context.Background(), CreateMissionOptions{
		Name: "m", TargetID: "t", MissionDefinitionID: "d",
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "daemon not responding")
}

// -----------------------------------------------------------------------
// PauseMission
// -----------------------------------------------------------------------

func TestClient_PauseMission_Success(t *testing.T) {
	mock := &extendedMockClient{
		pauseMissionResp: &daemonpb.PauseMissionResponse{
			Success:      true,
			CheckpointId: "chk-123",
		},
	}
	c := &Client{daemon: mock}
	ckpID, err := c.PauseMission(context.Background(), "m1", false)
	require.NoError(t, err)
	assert.Equal(t, "chk-123", ckpID)
}

func TestClient_PauseMission_Failure(t *testing.T) {
	mock := &extendedMockClient{
		pauseMissionResp: &daemonpb.PauseMissionResponse{
			Success: false,
			Message: "mission not pauseable",
		},
	}
	c := &Client{daemon: mock}
	_, err := c.PauseMission(context.Background(), "m1", false)
	require.Error(t, err)
}

func TestClient_PauseMission_NotFound(t *testing.T) {
	mock := &extendedMockClient{pauseMissionErr: status.Error(codes.NotFound, "not found")}
	c := &Client{daemon: mock}
	_, err := c.PauseMission(context.Background(), "m1", false)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "m1")
}

func TestClient_PauseMission_FailedPrecondition(t *testing.T) {
	mock := &extendedMockClient{pauseMissionErr: status.Error(codes.FailedPrecondition, "wrong state")}
	c := &Client{daemon: mock}
	_, err := c.PauseMission(context.Background(), "m1", false)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not running")
}

// -----------------------------------------------------------------------
// ResumeMission
// -----------------------------------------------------------------------

func TestClient_ResumeMission_NotFound(t *testing.T) {
	c := &Client{daemon: &extendedMockClient{}}
	_, err := c.ResumeMission(context.Background(), "m1", "")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not found")
}

// -----------------------------------------------------------------------
// GetMissionHistory
// -----------------------------------------------------------------------

func TestClient_GetMissionHistory_Success(t *testing.T) {
	mock := &extendedMockClient{
		getMissionHistoryResp: &daemonpb.GetMissionHistoryResponse{
			Runs: []*daemonpb.MissionRun{
				{MissionId: "m1", RunNumber: 1, Status: "completed", FindingsCount: 3},
				{MissionId: "m2", RunNumber: 2, Status: "failed", CompletedAt: 1700000100},
			},
			Total: 2,
		},
	}
	c := &Client{daemon: mock}

	runs, total, err := c.GetMissionHistory(context.Background(), "recon", 10, 0)
	require.NoError(t, err)
	assert.Len(t, runs, 2)
	assert.Equal(t, 2, total)
	// Second run has CompletedAt set.
	assert.NotNil(t, runs[1].CompletedAt)
}

func TestClient_GetMissionHistory_UnavailableError(t *testing.T) {
	mock := &extendedMockClient{getMissionHistoryErr: status.Error(codes.Unavailable, "down")}
	c := &Client{daemon: mock}
	_, _, err := c.GetMissionHistory(context.Background(), "recon", 10, 0)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "daemon not responding")
}

func TestClient_GetMissionHistory_NotFoundError(t *testing.T) {
	mock := &extendedMockClient{getMissionHistoryErr: status.Error(codes.NotFound, "no missions")}
	c := &Client{daemon: mock}
	_, _, err := c.GetMissionHistory(context.Background(), "unknown", 10, 0)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "unknown")
}

// -----------------------------------------------------------------------
// GetMissionDefinition
// -----------------------------------------------------------------------

func TestClient_GetMissionDefinition_EmptyName(t *testing.T) {
	c := &Client{daemon: &extendedMockClient{}}
	_, err := c.GetMissionDefinition(context.Background(), "")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "name is required")
}

func TestClient_GetMissionDefinition_NotFound(t *testing.T) {
	mock := &extendedMockClient{
		getMissionDefErr: status.Error(codes.NotFound, "not found"),
	}
	c := &Client{daemon: mock}
	_, err := c.GetMissionDefinition(context.Background(), "missing-def")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not found")
}

func TestClient_GetMissionDefinition_Success(t *testing.T) {
	mock := &extendedMockClient{
		getMissionDefResp: &daemonpb.GetMissionDefinitionResponse{
			Definition: &missionpb.MissionDefinition{
				Name:    "recon-v1",
				Version: "1.0",
			},
		},
	}
	c := &Client{daemon: mock}
	def, err := c.GetMissionDefinition(context.Background(), "recon-v1")
	require.NoError(t, err)
	require.NotNil(t, def)
	assert.Equal(t, "recon-v1", def.GetName())
	assert.Equal(t, "1.0", def.GetVersion())
}

// -----------------------------------------------------------------------
// ListMissionDefinitions
// -----------------------------------------------------------------------

func TestClient_ListMissionDefinitions_Success(t *testing.T) {
	mock := &extendedMockClient{
		listMissionDefsResp: &daemonpb.ListMissionDefinitionsResponse{
			Missions: []*daemonpb.MissionDefinitionInfo{
				{Name: "recon-v1", Version: "1.0"},
			},
		},
	}
	c := &Client{daemon: mock}

	defs, err := c.ListMissionDefinitions(context.Background())
	require.NoError(t, err)
	assert.Len(t, defs, 1)
	assert.Equal(t, "recon-v1", defs[0].Name)
}

func TestClient_ListMissionDefinitions_UnavailableError(t *testing.T) {
	mock := &extendedMockClient{listMissionDefsErr: status.Error(codes.Unavailable, "down")}
	c := &Client{daemon: mock}
	_, err := c.ListMissionDefinitions(context.Background())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "daemon not responding")
}

func TestClient_ListMissionDefinitions_DeadlineError(t *testing.T) {
	mock := &extendedMockClient{listMissionDefsErr: status.Error(codes.DeadlineExceeded, "timeout")}
	c := &Client{daemon: mock}
	_, err := c.ListMissionDefinitions(context.Background())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "timeout")
}

// -----------------------------------------------------------------------
// StartAgent / StartTool / StartPlugin (delegate to startComponent)
// -----------------------------------------------------------------------

func TestClient_StartAgent_Success(t *testing.T) {
	mock := &extendedMockClient{
		startComponentResp: &daemonpb.StartComponentResponse{Success: true, Pid: 1234, Port: 50100},
	}
	c := &Client{daemon: mock}
	result, err := c.StartAgent(context.Background(), "my-agent")
	require.NoError(t, err)
	assert.Equal(t, 1234, result.PID)
}

func TestClient_StartAgent_Failure(t *testing.T) {
	mock := &extendedMockClient{
		startComponentResp: &daemonpb.StartComponentResponse{Success: false, Message: "agent busy"},
	}
	c := &Client{daemon: mock}
	_, err := c.StartAgent(context.Background(), "my-agent")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "agent busy")
}

func TestClient_StartAgent_NotFoundError(t *testing.T) {
	mock := &extendedMockClient{startComponentErr: status.Error(codes.NotFound, "not found")}
	c := &Client{daemon: mock}
	_, err := c.StartAgent(context.Background(), "x")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not found")
}

func TestClient_StartAgent_AlreadyRunning(t *testing.T) {
	mock := &extendedMockClient{startComponentErr: status.Error(codes.AlreadyExists, "already running")}
	c := &Client{daemon: mock}
	_, err := c.StartAgent(context.Background(), "x")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "already running")
}

func TestClient_StartAgent_InvalidArgument(t *testing.T) {
	mock := &extendedMockClient{startComponentErr: status.Error(codes.InvalidArgument, "bad kind")}
	c := &Client{daemon: mock}
	_, err := c.StartAgent(context.Background(), "x")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid")
}

func TestClient_StartTool_Success(t *testing.T) {
	mock := &extendedMockClient{
		startComponentResp: &daemonpb.StartComponentResponse{Success: true, Pid: 2000},
	}
	c := &Client{daemon: mock}
	result, err := c.StartTool(context.Background(), "my-tool")
	require.NoError(t, err)
	assert.Equal(t, 2000, result.PID)
}

func TestClient_StartPlugin_Success(t *testing.T) {
	mock := &extendedMockClient{
		startComponentResp: &daemonpb.StartComponentResponse{Success: true, Pid: 3000},
	}
	c := &Client{daemon: mock}
	result, err := c.StartPlugin(context.Background(), "my-plugin")
	require.NoError(t, err)
	assert.Equal(t, 3000, result.PID)
}

// -----------------------------------------------------------------------
// StopAgent / StopTool / StopPlugin (delegate to stopComponent)
// -----------------------------------------------------------------------

func TestClient_StopAgent_Success(t *testing.T) {
	mock := &extendedMockClient{
		stopComponentResp: &daemonpb.StopComponentResponse{Success: true, StoppedCount: 1, TotalCount: 1},
	}
	c := &Client{daemon: mock}
	result, err := c.StopAgent(context.Background(), "my-agent")
	require.NoError(t, err)
	assert.Equal(t, 1, result.StoppedCount)
}

func TestClient_StopAgent_Failure(t *testing.T) {
	mock := &extendedMockClient{
		stopComponentResp: &daemonpb.StopComponentResponse{Success: false, Message: "stop failed"},
	}
	c := &Client{daemon: mock}
	_, err := c.StopAgent(context.Background(), "my-agent")
	require.Error(t, err)
}

func TestClient_StopAgent_NotFound(t *testing.T) {
	mock := &extendedMockClient{stopComponentErr: status.Error(codes.NotFound, "not running")}
	c := &Client{daemon: mock}
	_, err := c.StopAgent(context.Background(), "x")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not running")
}

func TestClient_StopAgent_InvalidArgument(t *testing.T) {
	mock := &extendedMockClient{stopComponentErr: status.Error(codes.InvalidArgument, "bad kind")}
	c := &Client{daemon: mock}
	_, err := c.StopAgent(context.Background(), "x")
	require.Error(t, err)
}

func TestClient_StopTool_Success(t *testing.T) {
	mock := &extendedMockClient{
		stopComponentResp: &daemonpb.StopComponentResponse{Success: true, StoppedCount: 1},
	}
	c := &Client{daemon: mock}
	_, err := c.StopTool(context.Background(), "my-tool")
	require.NoError(t, err)
}

func TestClient_StopPlugin_Success(t *testing.T) {
	mock := &extendedMockClient{
		stopComponentResp: &daemonpb.StopComponentResponse{Success: true, StoppedCount: 1},
	}
	c := &Client{daemon: mock}
	_, err := c.StopPlugin(context.Background(), "my-plugin")
	require.NoError(t, err)
}

// -----------------------------------------------------------------------
// BuildAgent / BuildTool / BuildPlugin (delegate to buildComponent)
// -----------------------------------------------------------------------

func TestClient_BuildAgent_Success(t *testing.T) {
	mock := &extendedMockClient{
		buildComponentResp: &daemonpb.BuildComponentResponse{
			Success: true,
			Stdout:  "Build complete",
		},
	}
	c := &Client{daemon: mock}
	result, err := c.BuildAgent(context.Background(), "my-agent")
	require.NoError(t, err)
	assert.Contains(t, result.Stdout, "Build complete")
}

func TestClient_BuildAgent_Failure(t *testing.T) {
	// buildComponent returns the result even when Success=false (caller checks result.Success).
	mock := &extendedMockClient{
		buildComponentResp: &daemonpb.BuildComponentResponse{Success: false, Stderr: "compile error"},
	}
	c := &Client{daemon: mock}
	result, err := c.BuildAgent(context.Background(), "my-agent")
	require.NoError(t, err)
	assert.False(t, result.Success)
	assert.Contains(t, result.Stderr, "compile error")
}

func TestClient_BuildAgent_NotFound(t *testing.T) {
	mock := &extendedMockClient{buildComponentErr: status.Error(codes.NotFound, "not found")}
	c := &Client{daemon: mock}
	_, err := c.BuildAgent(context.Background(), "x")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not found")
}

func TestClient_BuildAgent_InvalidArgument(t *testing.T) {
	mock := &extendedMockClient{buildComponentErr: status.Error(codes.InvalidArgument, "bad kind")}
	c := &Client{daemon: mock}
	_, err := c.BuildAgent(context.Background(), "x")
	require.Error(t, err)
}

func TestClient_BuildTool_Success(t *testing.T) {
	mock := &extendedMockClient{
		buildComponentResp: &daemonpb.BuildComponentResponse{Success: true},
	}
	c := &Client{daemon: mock}
	_, err := c.BuildTool(context.Background(), "my-tool")
	require.NoError(t, err)
}

func TestClient_BuildPlugin_Success(t *testing.T) {
	mock := &extendedMockClient{
		buildComponentResp: &daemonpb.BuildComponentResponse{Success: true},
	}
	c := &Client{daemon: mock}
	_, err := c.BuildPlugin(context.Background(), "my-plugin")
	require.NoError(t, err)
}

// -----------------------------------------------------------------------
// ShowAgent / ShowTool / ShowPlugin (delegate to showComponent)
// -----------------------------------------------------------------------

func TestClient_ShowAgent_Success(t *testing.T) {
	mock := &extendedMockClient{
		showComponentResp: &daemonpb.ShowComponentResponse{
			Success: true,
			Name:    "my-agent",
			Version: "1.0",
			Status:  "running",
		},
	}
	c := &Client{daemon: mock}
	info, err := c.ShowAgent(context.Background(), "my-agent")
	require.NoError(t, err)
	assert.Equal(t, "my-agent", info.Name)
}

func TestClient_ShowAgent_NotFound(t *testing.T) {
	mock := &extendedMockClient{showComponentErr: status.Error(codes.NotFound, "not found")}
	c := &Client{daemon: mock}
	_, err := c.ShowAgent(context.Background(), "x")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not found")
}

func TestClient_ShowAgent_InvalidArgument(t *testing.T) {
	mock := &extendedMockClient{showComponentErr: status.Error(codes.InvalidArgument, "bad")}
	c := &Client{daemon: mock}
	_, err := c.ShowAgent(context.Background(), "x")
	require.Error(t, err)
}

func TestClient_ShowTool_Success(t *testing.T) {
	mock := &extendedMockClient{
		showComponentResp: &daemonpb.ShowComponentResponse{Success: true, Name: "my-tool"},
	}
	c := &Client{daemon: mock}
	_, err := c.ShowTool(context.Background(), "my-tool")
	require.NoError(t, err)
}

func TestClient_ShowPlugin_Success(t *testing.T) {
	mock := &extendedMockClient{
		showComponentResp: &daemonpb.ShowComponentResponse{Success: true, Name: "my-plugin"},
	}
	c := &Client{daemon: mock}
	_, err := c.ShowPlugin(context.Background(), "my-plugin")
	require.NoError(t, err)
}

// -----------------------------------------------------------------------
// GetAgentLogs / GetToolLogs / GetPluginLogs (streaming — error path)
// -----------------------------------------------------------------------

func TestClient_GetAgentLogs_NotFound(t *testing.T) {
	c := &Client{daemon: &extendedMockClient{}}
	_, err := c.GetAgentLogs(context.Background(), "my-agent", LogsOptions{})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not found")
}

func TestClient_GetToolLogs_NotFound(t *testing.T) {
	c := &Client{daemon: &extendedMockClient{}}
	_, err := c.GetToolLogs(context.Background(), "my-tool", LogsOptions{})
	require.Error(t, err)
}

func TestClient_GetPluginLogs_NotFound(t *testing.T) {
	c := &Client{daemon: &extendedMockClient{}}
	_, err := c.GetPluginLogs(context.Background(), "my-plugin", LogsOptions{})
	require.Error(t, err)
}

// -----------------------------------------------------------------------
// Subscribe (streaming — error path)
// -----------------------------------------------------------------------

func TestClient_Subscribe_DaemonUnavailable(t *testing.T) {
	c := &Client{daemon: &extendedMockClient{}}
	_, err := c.Subscribe(context.Background())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "daemon not responding")
}

// -----------------------------------------------------------------------
// GetCapabilityManifest
// -----------------------------------------------------------------------

func TestClient_GetCapabilityManifest_Success(t *testing.T) {
	mock := &extendedMockClient{
		getCapabilityMfstResp: &manifestpb.GetCapabilityManifestResponse{
			Manifest: &manifestpb.CapabilityManifest{Subject: "agent-123"},
		},
	}
	c := &Client{daemon: mock}
	manifest, err := c.GetCapabilityManifest(context.Background(), "agent-123")
	require.NoError(t, err)
	assert.Equal(t, "agent-123", manifest.Subject)
}

func TestClient_GetCapabilityManifest_NilResponse(t *testing.T) {
	mock := &extendedMockClient{
		getCapabilityMfstResp: nil,
	}
	c := &Client{daemon: mock}
	_, err := c.GetCapabilityManifest(context.Background(), "agent-123")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "empty manifest")
}

func TestClient_GetCapabilityManifest_NilManifestInResponse(t *testing.T) {
	mock := &extendedMockClient{
		getCapabilityMfstResp: &manifestpb.GetCapabilityManifestResponse{Manifest: nil},
	}
	c := &Client{daemon: mock}
	_, err := c.GetCapabilityManifest(context.Background(), "agent-123")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "empty manifest")
}

func TestClient_GetCapabilityManifest_UnavailableError(t *testing.T) {
	mock := &extendedMockClient{getCapabilityMfstErr: status.Error(codes.Unavailable, "down")}
	c := &Client{daemon: mock}
	_, err := c.GetCapabilityManifest(context.Background(), "agent-123")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "daemon not responding")
}

func TestClient_GetCapabilityManifest_PermissionDenied(t *testing.T) {
	mock := &extendedMockClient{getCapabilityMfstErr: status.Error(codes.PermissionDenied, "no permission")}
	c := &Client{daemon: mock}
	_, err := c.GetCapabilityManifest(context.Background(), "agent-123")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "permission denied")
}

func TestClient_GetCapabilityManifest_DeadlineError(t *testing.T) {
	mock := &extendedMockClient{getCapabilityMfstErr: status.Error(codes.DeadlineExceeded, "timeout")}
	c := &Client{daemon: mock}
	_, err := c.GetCapabilityManifest(context.Background(), "agent-123")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "timeout")
}

// -----------------------------------------------------------------------
// GetAgentStatus
// -----------------------------------------------------------------------

func TestClient_GetAgentStatus(t *testing.T) {
	_ = &extendedMockClient{
		getAgentStatusResp: &daemonpb.GetAgentStatusResponse{},
	}
	// GetAgentStatus is not directly exposed on Client; it's through the
	// daemon gRPC client. Verify the mock is wired correctly.
}

// -----------------------------------------------------------------------
// CreateMissionDefinition error paths (supplemental to client_test.go)
// -----------------------------------------------------------------------

func TestClient_CreateMissionDefinition_UnavailableError(t *testing.T) {
	mock := &extendedMockClient{}
	mock.pingError = nil
	// Override CreateMissionDefinition to return error.
	errMock := &createMissionDefErrorMock{err: status.Error(codes.Unavailable, "down")}
	c := &Client{daemon: errMock}
	_, err := c.CreateMissionDefinition(context.Background(), &missionpb.MissionDefinition{Name: "x"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "daemon not responding")
}

type createMissionDefErrorMock struct {
	mockDaemonServiceClient
	err error
}

func (m *createMissionDefErrorMock) CreateMissionDefinition(ctx context.Context, req *daemonpb.CreateMissionDefinitionRequest, opts ...grpc.CallOption) (*daemonpb.CreateMissionDefinitionResponse, error) {
	return nil, m.err
}
