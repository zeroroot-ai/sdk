// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Zero Root AI

package serve

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"

	harnesspb "github.com/zeroroot-ai/sdk/api/gen/gibson/harness/v1"
	typespb "github.com/zeroroot-ai/sdk/api/gen/gibson/types/v1"
	"github.com/zeroroot-ai/sdk/fork"
	"github.com/zeroroot-ai/sdk/mission"
)

// forkServer is a callback service for the fork contract (D74, sdk#248). It
// records the metadata and the request of each call, and it can act as the
// daemon that forks the caller during CreateMission.
type forkServer struct {
	harnesspb.UnimplementedHarnessCallbackServiceServer

	mu          sync.Mutex
	claims      []*harnesspb.ClaimForkRequest
	claimMD     []metadata.MD
	create      []*harnesspb.CreateMissionRequest
	createMD    []metadata.MD
	duringFork  func() // runs inside CreateMission, as a fork of the caller
	failCreate  error
	claimAnswer *harnesspb.ClaimForkResponse
}

func (s *forkServer) ClaimFork(ctx context.Context, req *harnesspb.ClaimForkRequest) (*harnesspb.ClaimForkResponse, error) {
	md, _ := metadata.FromIncomingContext(ctx)
	s.mu.Lock()
	defer s.mu.Unlock()
	s.claims = append(s.claims, req)
	s.claimMD = append(s.claimMD, md)
	return s.claimAnswer, nil
}

func (s *forkServer) CreateMission(ctx context.Context, req *harnesspb.CreateMissionRequest) (*harnesspb.CreateMissionResponse, error) {
	md, _ := metadata.FromIncomingContext(ctx)
	s.mu.Lock()
	s.create = append(s.create, req)
	s.createMD = append(s.createMD, md)
	during, fail := s.duringFork, s.failCreate
	s.mu.Unlock()
	if during != nil {
		during()
	}
	if fail != nil {
		return nil, fail
	}
	return &harnesspb.CreateMissionResponse{Mission: &harnesspb.MissionInfo{Id: "child-1", Name: "child"}}, nil
}

func forkDispatch() *harnesspb.ClaimForkResponse {
	return &harnesspb.ClaimForkResponse{
		Grant:        "fork-grant",
		MissionId:    "child-1",
		MissionRunId: "child-run-1",
		AgentRunId:   "fork-run-1",
		NodeId:       "node-b",
		Model:        "model-x",
		Task:         &typespb.Task{Id: "task-b", Goal: "scan the next host"},
	}
}

// fakeIdentity is a fake setec identity socket (setec#235). Each request gets
// a new token of the current generation. A snapshot raises the generation.
type fakeIdentity struct {
	mu         sync.Mutex
	generation int
	requests   int
}

func (f *fakeIdentity) snapshot() { f.mu.Lock(); f.generation++; f.mu.Unlock() }

func (f *fakeIdentity) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.requests++
	if r.URL.Query().Get("audience") != fork.SandboxIdentityAudience {
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte(`{"error":"wrong audience"}`))
		return
	}
	_ = json.NewEncoder(w).Encode(map[string]any{"token": fmt.Sprintf("gen%d-req%d", f.generation, f.requests)})
}

// serveIdentity serves a fake identity socket and names it in
// SETEC_IDENTITY_SOCKET for one test.
func serveIdentity(t *testing.T) *fakeIdentity {
	t.Helper()
	dir, err := os.MkdirTemp("", "id") // a Unix socket path has a short limit
	require.NoError(t, err)
	t.Cleanup(func() { _ = os.RemoveAll(dir) })
	path := filepath.Join(dir, "identity.sock")
	lis, err := (&net.ListenConfig{}).Listen(context.Background(), "unix", path)
	require.NoError(t, err)
	f := &fakeIdentity{}
	srv := &http.Server{Handler: f} //nolint:gosec // a test socket
	go func() { _ = srv.Serve(lis) }()
	t.Cleanup(func() { _ = srv.Close() })
	t.Setenv(fork.EnvIdentitySocket, path)
	return f
}

// serveForkTCP serves s on a loopback TCP port and returns a client connected
// through Connect, so the dial options of the client are the real ones.
func serveForkTCP(t *testing.T, s *forkServer) *CallbackClient {
	t.Helper()
	lis, err := (&net.ListenConfig{}).Listen(context.Background(), "tcp", "127.0.0.1:0")
	require.NoError(t, err)
	srv := grpc.NewServer()
	harnesspb.RegisterHarnessCallbackServiceServer(srv, s)
	go func() { _ = srv.Serve(lis) }()
	t.Cleanup(srv.Stop)

	client, err := NewCallbackClient(lis.Addr().String(), WithCallbackToken("parent-grant"))
	require.NoError(t, err)
	require.NoError(t, client.Connect(context.Background()))
	t.Cleanup(func() { _ = client.Close() })
	return client
}

// TestCallbackClient_SendsTheSandboxIDOnEachCall proves that Connect installs
// the interceptors of the contract: each call carries a new identity token
// and the hostname of the process.
func TestCallbackClient_SendsTheSandboxIDOnEachCall(t *testing.T) {
	want, err := os.Hostname()
	require.NoError(t, err)
	want = strings.TrimSpace(want)

	serveIdentity(t)
	s := &forkServer{claimAnswer: forkDispatch()}
	client := serveForkTCP(t, s)

	_, err = client.ClaimFork(context.Background(), "sbx-fork-1")
	require.NoError(t, err)
	require.Len(t, s.claimMD, 1)
	require.Equal(t, []string{want}, s.claimMD[0].Get(fork.MetadataSandboxID))
	require.Equal(t, []string{"gen0-req1"}, s.claimMD[0].Get(fork.MetadataSandboxIdentity))
	require.Empty(t, s.claimMD[0].Get("authorization"),
		"the identity token is the only proof of a claim (D80)")
	require.Equal(t, "sbx-fork-1", s.claims[0].GetSandboxId())
}

// TestCallbackClient_ApplyClaimSwitchesTheGrant proves that after a claim each
// call carries the grant and the ids of the fork, and never the grant of the
// parent.
func TestCallbackClient_ApplyClaimSwitchesTheGrant(t *testing.T) {
	serveIdentity(t)
	s := &forkServer{claimAnswer: forkDispatch()}
	client := serveForkTCP(t, s)
	client.SetFullContext(TaskContextParams{TaskID: "task-a", MissionID: "parent-1", AgentRunID: "parent-run-1"})

	claim, err := client.ClaimFork(context.Background(), "sbx-fork-1")
	require.NoError(t, err)
	require.Equal(t, &fork.Claim{
		SandboxID: "sbx-fork-1", Grant: "fork-grant", MissionID: "child-1", MissionRunID: "child-run-1",
		AgentRunID: "fork-run-1", NodeID: "node-b", Model: "model-x", Task: claim.Task,
	}, claim)
	require.Equal(t, "task-b", claim.Task.GetId())

	require.NoError(t, client.ApplyClaim(claim))
	_, err = client.CreateMission(context.Background(), &harnesspb.CreateMissionRequest{TargetId: "t"})
	require.NoError(t, err)

	require.Equal(t, []string{"Bearer fork-grant"}, s.createMD[0].Get("authorization"))
	ctxInfo := s.create[0].GetContext()
	require.Equal(t, "child-1", ctxInfo.GetMissionId())
	require.Equal(t, "child-run-1", ctxInfo.GetMissionRunId())
	require.Equal(t, "fork-run-1", ctxInfo.GetAgentRunId())
	require.Equal(t, "task-b", ctxInfo.GetTaskId())
}

func TestCallbackClient_ApplyClaimRefusals(t *testing.T) {
	c := &CallbackClient{}
	require.Error(t, c.ApplyClaim(nil))
	require.Error(t, c.ApplyClaim(&fork.Claim{}), "a claim with no grant")

	c = &CallbackClient{perRPCCreds: staticPerRPC{}}
	require.Error(t, c.ApplyClaim(&fork.Claim{Grant: "fork-grant"}), "per-RPC credentials cannot change their grant")
}

type staticPerRPC struct{}

func (staticPerRPC) GetRequestMetadata(context.Context, ...string) (map[string]string, error) {
	return nil, nil
}
func (staticPerRPC) RequireTransportSecurity() bool { return false }

// host is a hostname that a test changes, as a fork changes it.
type forkHost struct {
	mu   sync.Mutex
	name string
}

func (h *forkHost) set(n string) { h.mu.Lock(); h.name = n; h.mu.Unlock() }
func (h *forkHost) read() (string, error) {
	h.mu.Lock()
	defer h.mu.Unlock()
	return h.name, nil
}

func forkHarness(t *testing.T, s *forkServer, h *forkHost) *CallbackHarness {
	t.Helper()
	serveIdentity(t)
	srv := grpc.NewServer()
	harnesspb.RegisterHarnessCallbackServiceServer(srv, s)
	conn := dialBufconn(t, srv)
	w, err := fork.NewWatcherWith(h.read)
	require.NoError(t, err)
	harness := &CallbackHarness{
		client: &CallbackClient{
			conn: conn, client: harnesspb.NewHarnessCallbackServiceClient(conn), connected: true,
			token: "parent-grant", missionID: "parent-1", taskID: "task-a",
		},
		tracer:      defaultNoopTracer(),
		forkWatcher: w,
	}
	return harness
}

// TestCreateMission_TheForkGetsErrForked is the #803 path: the daemon forks the
// caller while CreateMission is open. In the fork the call fails, and the
// harness claims the dispatch of the fork and returns it in ErrForked.
func TestCreateMission_TheForkGetsErrForked(t *testing.T) {
	h := &forkHost{name: "sbx-parent"}
	s := &forkServer{claimAnswer: forkDispatch()}
	s.duringFork = func() { h.set("sbx-fork-1") }
	s.failCreate = status.Error(codes.Unavailable, "the connection of the fork is gone")
	harness := forkHarness(t, s, h)

	_, err := harness.CreateMission(context.Background(), map[string]any{"name": "m"}, "target-1",
		&mission.CreateMissionOpts{StartsFromCallerState: true})

	var forked *fork.ErrForked
	require.ErrorAs(t, err, &forked, "the fork gets ErrForked")
	require.Equal(t, "node-b", forked.Claim.NodeID)
	require.Equal(t, "task-b", forked.Claim.Task.GetId())
	require.Equal(t, harnesspb.OriginationStart_ORIGINATION_START_CALLER_STATE, s.create[0].GetStartsFrom())
	require.Len(t, s.claims, 1)
	require.Equal(t, "sbx-fork-1", s.claims[0].GetSandboxId())
	require.Equal(t, "fork-grant", harness.client.token, "the fork uses its own grant after the claim")
}

// TestCreateMission_TheParentGetsTheMission proves that the caller that stays
// in its sandbox gets the result of the call and claims nothing.
func TestCreateMission_TheParentGetsTheMission(t *testing.T) {
	h := &forkHost{name: "sbx-parent"}
	s := &forkServer{claimAnswer: forkDispatch()}
	harness := forkHarness(t, s, h)

	info, err := harness.CreateMission(context.Background(), map[string]any{"name": "m"}, "target-1",
		&mission.CreateMissionOpts{StartsFromCallerState: true})
	require.NoError(t, err)
	require.Equal(t, "child-1", info.ID)
	require.Empty(t, s.claims, "the parent claims nothing")
	require.Equal(t, "parent-grant", harness.client.token)
}

// TestCreateMission_NoForkWithoutTheOption proves that a plain CreateMission
// neither asks for a fork nor checks for one.
func TestCreateMission_NoForkWithoutTheOption(t *testing.T) {
	h := &forkHost{name: "sbx-parent"}
	s := &forkServer{claimAnswer: forkDispatch()}
	s.duringFork = func() { h.set("sbx-other") }
	harness := forkHarness(t, s, h)

	_, err := harness.CreateMission(context.Background(), map[string]any{"name": "m"}, "target-1", &mission.CreateMissionOpts{})
	require.NoError(t, err)
	require.Equal(t, harnesspb.OriginationStart_ORIGINATION_START_UNSPECIFIED, s.create[0].GetStartsFrom())
	require.Empty(t, s.claims)
}

// TestCreateMission_ForkNeedsASandboxID proves that a process with no readable
// sandbox id cannot ask for a fork of itself.
func TestCreateMission_ForkNeedsASandboxID(t *testing.T) {
	s := &forkServer{claimAnswer: forkDispatch()}
	harness := forkHarness(t, s, &forkHost{name: "sbx-parent"})
	harness.forkWatcher = nil

	_, err := harness.CreateMission(context.Background(), map[string]any{"name": "m"}, "target-1",
		&mission.CreateMissionOpts{StartsFromCallerState: true})
	require.Error(t, err)
	require.Empty(t, s.create, "no call is sent")
}

func TestCallbackClient_ClaimForkErrors(t *testing.T) {
	serveIdentity(t)
	_, err := (&CallbackClient{}).ClaimFork(context.Background(), "sbx-fork-1")
	require.Error(t, err, "a client that is not connected")

	srv := grpc.NewServer()
	harnesspb.RegisterHarnessCallbackServiceServer(srv, &harnesspb.UnimplementedHarnessCallbackServiceServer{})
	conn := dialBufconn(t, srv)
	c := &CallbackClient{conn: conn, client: harnesspb.NewHarnessCallbackServiceClient(conn), connected: true}
	_, err = c.ClaimFork(context.Background(), "sbx-fork-1")
	require.Equal(t, codes.Unimplemented, status.Code(err))
}

// TestCreateMission_AFailedClaimIsReported proves that a fork that cannot
// claim its dispatch gets the error of the claim, not the result of the call.
func TestCreateMission_AFailedClaimIsReported(t *testing.T) {
	h := &forkHost{name: "sbx-parent"}
	s := &forkServer{claimAnswer: forkDispatch()}
	s.duringFork = func() { h.set("sbx-fork-1") }
	harness := forkHarness(t, s, h)
	harness.client.client = failingClaims{HarnessCallbackServiceClient: harness.client.client}

	_, err := harness.CreateMission(context.Background(), map[string]any{"name": "m"}, "target-1",
		&mission.CreateMissionOpts{StartsFromCallerState: true})
	require.Equal(t, codes.AlreadyExists, status.Code(err), "got %v", err)
	var forked *fork.ErrForked
	require.NotErrorAs(t, err, &forked)
}

// failingClaims refuses each claim, as the daemon refuses a second claim.
type failingClaims struct {
	harnesspb.HarnessCallbackServiceClient
}

func (failingClaims) ClaimFork(context.Context, *harnesspb.ClaimForkRequest, ...grpc.CallOption) (*harnesspb.ClaimForkResponse, error) {
	return nil, status.Error(codes.AlreadyExists, "claimed before") //nolint:wrapcheck // the fake answers as the daemon does
}

// TestCreateMission_AClaimWithNoGrantIsRefused proves that the fork does not
// go on with a dispatch that has no grant.
func TestCreateMission_AClaimWithNoGrantIsRefused(t *testing.T) {
	h := &forkHost{name: "sbx-parent"}
	s := &forkServer{claimAnswer: &harnesspb.ClaimForkResponse{NodeId: "node-b"}}
	s.duringFork = func() { h.set("sbx-fork-1") }
	harness := forkHarness(t, s, h)

	_, err := harness.CreateMission(context.Background(), map[string]any{"name": "m"}, "target-1",
		&mission.CreateMissionOpts{StartsFromCallerState: true})
	require.Error(t, err)
	var forked *fork.ErrForked
	require.NotErrorAs(t, err, &forked)
	require.Equal(t, "parent-grant", harness.client.token)
}

// TestCreateMission_TheForkClaimsWithItsOwnToken is the #803 path over the
// real interceptors. The daemon snapshots the caller while CreateMission is
// open. The call of the parent carries the token of the parent. The claim of
// the fork carries a new token of the new generation, never the parent token.
func TestCreateMission_TheForkClaimsWithItsOwnToken(t *testing.T) {
	id := serveIdentity(t)
	h := &forkHost{name: "sbx-parent"}
	s := &forkServer{claimAnswer: forkDispatch()}
	s.duringFork = func() { id.snapshot(); h.set("sbx-fork-1") }
	s.failCreate = status.Error(codes.Unavailable, "the connection of the fork is gone")
	client := serveForkTCP(t, s)
	w, err := fork.NewWatcherWith(h.read)
	require.NoError(t, err)
	harness := &CallbackHarness{client: client, tracer: defaultNoopTracer(), forkWatcher: w}

	_, err = harness.CreateMission(context.Background(), map[string]any{"name": "m"}, "target-1",
		&mission.CreateMissionOpts{StartsFromCallerState: true})

	var forked *fork.ErrForked
	require.ErrorAs(t, err, &forked)
	require.Equal(t, []string{"gen0-req1"}, s.createMD[0].Get(fork.MetadataSandboxIdentity))
	require.Len(t, s.claimMD, 1)
	require.Equal(t, []string{"gen1-req2"}, s.claimMD[0].Get(fork.MetadataSandboxIdentity),
		"the fork claims with its own token")
}

// TestCallbackClient_ClaimForkNeedsTheIdentity proves that a process with no
// identity socket cannot claim a fork, and that the error says why.
func TestCallbackClient_ClaimForkNeedsTheIdentity(t *testing.T) {
	t.Setenv(fork.EnvIdentitySocket, "")
	s := &forkServer{claimAnswer: forkDispatch()}
	client := serveForkTCP(t, s)

	_, err := client.ClaimFork(context.Background(), "sbx-fork-1")
	require.ErrorIs(t, err, fork.ErrNoSandboxIdentity)
	require.ErrorContains(t, err, fork.EnvIdentitySocket)
	require.Empty(t, s.claims, "no call is sent")
}

// TestCallbackClient_ClaimForkSendsNoGrant proves that a claim carries the
// identity token and no grant, even when the caller put one on the context,
// and that the grant of the claim replaces the old grant for each later call
// (D80).
func TestCallbackClient_ClaimForkSendsNoGrant(t *testing.T) {
	serveIdentity(t)
	s := &forkServer{claimAnswer: forkDispatch()}
	client := serveForkTCP(t, s)

	ctx := metadata.AppendToOutgoingContext(context.Background(), "authorization", "Bearer expired-source-grant", "x-other", "kept")
	claim, err := client.ClaimFork(ctx, "sbx-fork-1")
	require.NoError(t, err)
	require.Empty(t, s.claimMD[0].Get("authorization"))
	require.Equal(t, []string{"kept"}, s.claimMD[0].Get("x-other"), "only the grant is removed")
	require.NotEmpty(t, s.claimMD[0].Get(fork.MetadataSandboxIdentity))

	require.NoError(t, client.ApplyClaim(claim))
	_, err = client.CreateMission(context.Background(), &harnesspb.CreateMissionRequest{TargetId: "t"})
	require.NoError(t, err)
	require.Equal(t, []string{"Bearer fork-grant"}, s.createMD[0].Get("authorization"), "the grant of the claim replaces the old one")
}

// TestCallbackClient_ClaimForkRefusesPerRPCCredentials proves that a client
// whose credentials put a grant on each call cannot claim.
func TestCallbackClient_ClaimForkRefusesPerRPCCredentials(t *testing.T) {
	serveIdentity(t)
	s := &forkServer{claimAnswer: forkDispatch()}
	srv := grpc.NewServer()
	harnesspb.RegisterHarnessCallbackServiceServer(srv, s)
	conn := dialBufconn(t, srv)
	c := &CallbackClient{conn: conn, client: harnesspb.NewHarnessCallbackServiceClient(conn), connected: true, perRPCCreds: staticPerRPC{}}
	_, err := c.ClaimFork(context.Background(), "sbx-fork-1")
	require.ErrorContains(t, err, "per-RPC credentials")
	require.Empty(t, s.claims, "no call is sent")
}
