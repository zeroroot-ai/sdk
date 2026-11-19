// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Zero Root AI

package serve

import (
	"context"
	"encoding/json"
	"net"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/status"
	"google.golang.org/grpc/test/bufconn"

	componentpb "github.com/zeroroot-ai/sdk/api/gen/gibson/component/v1"
	"github.com/zeroroot-ai/sdk/mission"
)

type mockComponentServer struct {
	componentpb.UnimplementedComponentServiceServer

	queryNodesFunc    func(*componentpb.QueryNodesRequest) (*componentpb.QueryNodesResponse, error)
	getFindingsFunc   func(*componentpb.GetFindingsRequest) (*componentpb.GetFindingsResponse, error)
	createMissionFunc func(*componentpb.CreateMissionRequest) (*componentpb.CreateMissionResponse, error)
	getCredentialFunc func(*componentpb.GetCredentialRequest) (*componentpb.GetCredentialResponse, error)
}

func (s *mockComponentServer) QueryNodes(_ context.Context, req *componentpb.QueryNodesRequest) (*componentpb.QueryNodesResponse, error) {
	if s.queryNodesFunc != nil {
		return s.queryNodesFunc(req)
	}
	return nil, status.Error(codes.Unimplemented, "not set")
}

func (s *mockComponentServer) GetFindings(_ context.Context, req *componentpb.GetFindingsRequest) (*componentpb.GetFindingsResponse, error) {
	if s.getFindingsFunc != nil {
		return s.getFindingsFunc(req)
	}
	return nil, status.Error(codes.Unimplemented, "not set")
}

func (s *mockComponentServer) CreateMission(_ context.Context, req *componentpb.CreateMissionRequest) (*componentpb.CreateMissionResponse, error) {
	if s.createMissionFunc != nil {
		return s.createMissionFunc(req)
	}
	return nil, status.Error(codes.Unimplemented, "not set")
}

func (s *mockComponentServer) GetCredential(_ context.Context, req *componentpb.GetCredentialRequest) (*componentpb.GetCredentialResponse, error) {
	if s.getCredentialFunc != nil {
		return s.getCredentialFunc(req)
	}
	return nil, status.Error(codes.Unimplemented, "not set")
}

const parityBufSize = 1024 * 1024

// setupParityTest spins up an in-process gRPC server and returns a
// PlatformHarness connected to it, plus a cleanup function.
func setupParityTest(t *testing.T, mock *mockComponentServer) (*PlatformHarness, func()) {
	t.Helper()
	return setupParityTestWith(t, func(s *grpc.Server) { componentpb.RegisterComponentServiceServer(s, mock) })
}

// setupParityTestWith is setupParityTest for a test that registers its own
// ComponentService implementation.
func setupParityTestWith(t *testing.T, register func(*grpc.Server)) (harness *PlatformHarness, cleanup func()) {
	t.Helper()

	lis := bufconn.Listen(parityBufSize)

	srv := grpc.NewServer()
	register(srv)

	go func() {
		if err := srv.Serve(lis); err != nil {
			t.Logf("mock component server exited: %v", err)
		}
	}()

	ctx := context.Background()
	//nolint:staticcheck
	conn, err := grpc.DialContext(ctx, "bufnet",
		grpc.WithContextDialer(func(context.Context, string) (net.Conn, error) {
			return lis.Dial()
		}),
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	require.NoError(t, err)

	// Build a PlatformClient that reuses the already-dialled connection so we
	// do not need to go through the TLS/URL dial logic.
	pc := &PlatformClient{
		conn:       conn,
		service:    componentpb.NewComponentServiceClient(conn),
		instanceID: "test-instance",
	}

	harness = NewPlatformHarness(pc,
		WithPlatformWorkID("work-001"),
	)

	cleanup = func() {
		conn.Close()
		srv.Stop()
		lis.Close()
	}
	return harness, cleanup
}

func TestPlatformParity_CreateMission_Happy(t *testing.T) {
	wantInfo := mission.MissionInfo{ID: "mission-abc", Name: "web-recon"}
	infoJSON, err := json.Marshal(wantInfo)
	require.NoError(t, err)

	mock := &mockComponentServer{}
	mock.createMissionFunc = func(req *componentpb.CreateMissionRequest) (*componentpb.CreateMissionResponse, error) {
		assert.Equal(t, "work-001", req.WorkId)
		assert.Equal(t, "target-007", req.TargetId)
		assert.NotEmpty(t, req.MissionDefinitionJson)
		return &componentpb.CreateMissionResponse{MissionJson: infoJSON}, nil
	}

	harness, cleanup := setupParityTest(t, mock)
	defer cleanup()

	mission := map[string]any{"steps": []string{"enumerate", "scan"}}
	got, err := harness.CreateMission(context.Background(), mission, "target-007", nil)
	require.NoError(t, err)
	assert.Equal(t, "mission-abc", got.ID)
	assert.Equal(t, "web-recon", got.Name)
}

func TestPlatformParity_CreateMission_RPCError(t *testing.T) {
	mock := &mockComponentServer{}
	mock.createMissionFunc = func(_ *componentpb.CreateMissionRequest) (*componentpb.CreateMissionResponse, error) {
		return nil, status.Error(codes.PermissionDenied, "quota exceeded")
	}

	harness, cleanup := setupParityTest(t, mock)
	defer cleanup()

	_, err := harness.CreateMission(context.Background(), map[string]any{}, "t1", nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "PlatformHarness.CreateMission")
}
