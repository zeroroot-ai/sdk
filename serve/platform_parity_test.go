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

	"github.com/zeroroot-ai/sdk/agent"
	componentpb "github.com/zeroroot-ai/sdk/api/gen/gibson/component/v1"
	"github.com/zeroroot-ai/sdk/llm"
	"github.com/zeroroot-ai/sdk/mission"
)

// mockComponentServer backs all parity tests. Each test overrides only the
// specific RPC it needs; unset methods return Unimplemented.

type mockComponentServer struct {
	componentpb.UnimplementedComponentServiceServer

	queryNodesFunc        func(*componentpb.QueryNodesRequest) (*componentpb.QueryNodesResponse, error)
	completeWithToolsFunc func(*componentpb.CompleteWithToolsRequest) (*componentpb.CompleteWithToolsResponse, error)
	getFindingsFunc       func(*componentpb.GetFindingsRequest) (*componentpb.GetFindingsResponse, error)
	delegateToAgentFunc   func(*componentpb.DelegateToAgentRequest) (*componentpb.DelegateToAgentResponse, error)
	createMissionFunc     func(*componentpb.CreateMissionRequest) (*componentpb.CreateMissionResponse, error)
	getCredentialFunc     func(*componentpb.GetCredentialRequest) (*componentpb.GetCredentialResponse, error)
	completeStreamFunc    func(*componentpb.CompleteStreamRequest, grpc.ServerStreamingServer[componentpb.CompleteStreamResponse]) error
}

func (s *mockComponentServer) QueryNodes(_ context.Context, req *componentpb.QueryNodesRequest) (*componentpb.QueryNodesResponse, error) {
	if s.queryNodesFunc != nil {
		return s.queryNodesFunc(req)
	}
	return nil, status.Error(codes.Unimplemented, "not set")
}

func (s *mockComponentServer) CompleteWithTools(_ context.Context, req *componentpb.CompleteWithToolsRequest) (*componentpb.CompleteWithToolsResponse, error) {
	if s.completeWithToolsFunc != nil {
		return s.completeWithToolsFunc(req)
	}
	return nil, status.Error(codes.Unimplemented, "not set")
}

func (s *mockComponentServer) GetFindings(_ context.Context, req *componentpb.GetFindingsRequest) (*componentpb.GetFindingsResponse, error) {
	if s.getFindingsFunc != nil {
		return s.getFindingsFunc(req)
	}
	return nil, status.Error(codes.Unimplemented, "not set")
}

func (s *mockComponentServer) DelegateToAgent(_ context.Context, req *componentpb.DelegateToAgentRequest) (*componentpb.DelegateToAgentResponse, error) {
	if s.delegateToAgentFunc != nil {
		return s.delegateToAgentFunc(req)
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

func (s *mockComponentServer) CompleteStream(
	req *componentpb.CompleteStreamRequest,
	stream grpc.ServerStreamingServer[componentpb.CompleteStreamResponse],
) error {
	if s.completeStreamFunc != nil {
		return s.completeStreamFunc(req, stream)
	}
	return status.Error(codes.Unimplemented, "not set")
}

const parityBufSize = 1024 * 1024

// setupParityTest spins up an in-process gRPC server and returns a
// PlatformHarness connected to it, plus a cleanup function.
func setupParityTest(t *testing.T, mock *mockComponentServer) (*PlatformHarness, func()) {
	t.Helper()

	lis := bufconn.Listen(parityBufSize)

	srv := grpc.NewServer()
	componentpb.RegisterComponentServiceServer(srv, mock)

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

	harness := NewPlatformHarness(pc,
		WithPlatformWorkID("work-001"),
	)

	cleanup := func() {
		conn.Close()
		srv.Stop()
		lis.Close()
	}
	return harness, cleanup
}

// --- 1. QueryNodes ---

// --- 3. CompleteWithTools ---

func TestPlatformParity_CompleteWithTools_Happy(t *testing.T) {
	mock := &mockComponentServer{}
	mock.completeWithToolsFunc = func(req *componentpb.CompleteWithToolsRequest) (*componentpb.CompleteWithToolsResponse, error) {
		assert.Equal(t, "work-001", req.WorkId)
		assert.Equal(t, "primary", req.Slot)
		require.Len(t, req.Messages, 1)
		assert.Equal(t, "user", req.Messages[0].Role)
		require.Len(t, req.Tools, 1)
		assert.Equal(t, "mytool_scan", req.Tools[0].Name)
		return &componentpb.CompleteWithToolsResponse{
			FinishReason: "tool_use",
			ToolCalls: []*componentpb.ToolCallResult{
				{Id: "call-1", Name: "mytool_scan", ArgumentsJson: `{"target":"10.0.0.1"}`},
			},
			Usage: &componentpb.TokenUsage{InputTokens: 100, OutputTokens: 20},
		}, nil
	}

	harness, cleanup := setupParityTest(t, mock)
	defer cleanup()

	resp, err := harness.CompleteWithTools(
		context.Background(),
		"primary",
		[]llm.Message{{Role: llm.RoleUser, Content: "scan the network"}},
		[]llm.ToolDef{{
			Name:        "mytool_scan",
			Description: "run mytool",
			Parameters:  map[string]any{"type": "object"},
		}},
	)
	require.NoError(t, err)
	assert.Equal(t, "tool_use", resp.FinishReason)
	require.Len(t, resp.ToolCalls, 1)
	assert.Equal(t, "call-1", resp.ToolCalls[0].ID)
	assert.JSONEq(t, `{"target":"10.0.0.1"}`, resp.ToolCalls[0].Arguments)
	assert.Equal(t, 100, resp.Usage.InputTokens)
}

func TestPlatformParity_CompleteWithTools_RPCError(t *testing.T) {
	mock := &mockComponentServer{}
	mock.completeWithToolsFunc = func(_ *componentpb.CompleteWithToolsRequest) (*componentpb.CompleteWithToolsResponse, error) {
		return nil, status.Error(codes.ResourceExhausted, "rate limited")
	}

	harness, cleanup := setupParityTest(t, mock)
	defer cleanup()

	_, err := harness.CompleteWithTools(context.Background(), "primary", nil, nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "PlatformHarness.CompleteWithTools")
}

// --- 4. GetFindings ---

// --- 5. DelegateToAgent ---

func TestPlatformParity_DelegateToAgent_Happy(t *testing.T) {
	wantResult := agent.Result{Status: agent.StatusSuccess, Output: "recon complete"}
	resultJSON, err := json.Marshal(wantResult)
	require.NoError(t, err)

	mock := &mockComponentServer{}
	mock.delegateToAgentFunc = func(req *componentpb.DelegateToAgentRequest) (*componentpb.DelegateToAgentResponse, error) {
		assert.Equal(t, "work-001", req.WorkId)
		assert.Equal(t, "recon-agent", req.AgentName)
		var task agent.Task
		require.NoError(t, json.Unmarshal(req.TaskJson, &task))
		assert.Equal(t, "task-99", task.ID)
		return &componentpb.DelegateToAgentResponse{ResultJson: resultJSON}, nil
	}

	harness, cleanup := setupParityTest(t, mock)
	defer cleanup()

	result, err := harness.DelegateToAgent(context.Background(), "recon-agent", agent.Task{
		ID:   "task-99",
		Goal: "enumerate subdomains",
	})
	require.NoError(t, err)
	assert.Equal(t, agent.StatusSuccess, result.Status)
}

func TestPlatformParity_DelegateToAgent_RPCError(t *testing.T) {
	mock := &mockComponentServer{}
	mock.delegateToAgentFunc = func(_ *componentpb.DelegateToAgentRequest) (*componentpb.DelegateToAgentResponse, error) {
		return nil, status.Error(codes.Unavailable, "agent offline")
	}

	harness, cleanup := setupParityTest(t, mock)
	defer cleanup()

	_, err := harness.DelegateToAgent(context.Background(), "recon-agent", agent.Task{ID: "t1"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "PlatformHarness.DelegateToAgent")
}

// --- 7. CreateMission ---

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

// --- 9. Stream ---

func TestPlatformParity_Stream_Happy(t *testing.T) {
	mock := &mockComponentServer{}
	mock.completeStreamFunc = func(
		req *componentpb.CompleteStreamRequest,
		stream grpc.ServerStreamingServer[componentpb.CompleteStreamResponse],
	) error {
		assert.Equal(t, "work-001", req.WorkId)
		assert.Equal(t, "primary", req.Slot)

		for _, msg := range []*componentpb.CompleteStreamResponse{
			{Content: "Hello", Done: false},
			{Content: " World", Done: false},
			{Content: "", Done: true, Usage: &componentpb.TokenUsage{InputTokens: 5, OutputTokens: 2}},
		} {
			if err := stream.Send(msg); err != nil {
				return err
			}
		}
		return nil
	}

	harness, cleanup := setupParityTest(t, mock)
	defer cleanup()

	ch, err := harness.Stream(context.Background(), "primary", []llm.Message{
		{Role: llm.RoleUser, Content: "say hello"},
	})
	require.NoError(t, err)
	require.NotNil(t, ch)

	var deltas []string
	var lastChunk llm.StreamChunk
	for chunk := range ch {
		lastChunk = chunk
		if chunk.Delta != "" {
			deltas = append(deltas, chunk.Delta)
		}
	}
	assert.Equal(t, []string{"Hello", " World"}, deltas)
	assert.Equal(t, "stop", lastChunk.FinishReason)
	require.NotNil(t, lastChunk.Usage)
	assert.Equal(t, 5, lastChunk.Usage.InputTokens)
}

func TestPlatformParity_Stream_RPCError(t *testing.T) {
	mock := &mockComponentServer{}
	mock.completeStreamFunc = func(
		_ *componentpb.CompleteStreamRequest,
		_ grpc.ServerStreamingServer[componentpb.CompleteStreamResponse],
	) error {
		return status.Error(codes.Internal, "LLM provider down")
	}

	harness, cleanup := setupParityTest(t, mock)
	defer cleanup()

	// The gRPC client-side error manifests either at Stream() call time or as an
	// error chunk in the channel, depending on when the transport detects it.
	ch, callErr := harness.Stream(context.Background(), "primary", nil)
	if callErr != nil {
		assert.Contains(t, callErr.Error(), "PlatformHarness.Stream")
		return
	}
	require.NotNil(t, ch)
	var sawErrorFinish bool
	for chunk := range ch {
		if chunk.FinishReason == "error" {
			sawErrorFinish = true
		}
	}
	assert.True(t, sawErrorFinish, "expected error finish reason from stream channel")
}
