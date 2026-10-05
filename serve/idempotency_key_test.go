// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Zero Root AI

package serve

import (
	"context"
	"net"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/test/bufconn"

	componentpb "github.com/zeroroot-ai/sdk/api/gen/gibson/component/v1"
	harnesspb "github.com/zeroroot-ai/sdk/api/gen/gibson/harness/v1"
	"github.com/zeroroot-ai/sdk/finding"
)

// keyRecorder collects the idempotency keys that a test server receives.
type keyRecorder struct {
	mu   sync.Mutex
	keys []string
}

func (r *keyRecorder) add(key string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.keys = append(r.keys, key)
}

// assertOneKeyForEachCall proves that each call sent a key, that the key obeys
// the length rule of the wire, and that no two calls shared a key.
func (r *keyRecorder) assertOneKeyForEachCall(t *testing.T, calls int) {
	t.Helper()
	r.mu.Lock()
	defer r.mu.Unlock()

	require.Len(t, r.keys, calls)
	seen := make(map[string]bool, len(r.keys))
	for _, key := range r.keys {
		assert.NotEmpty(t, key)
		assert.LessOrEqual(t, len(key), 128)
		assert.False(t, seen[key], "two calls sent the key %q", key)
		seen[key] = true
	}
}

// dialBufconn serves srv on an in-memory listener and returns a connection.
func dialBufconn(t *testing.T, srv *grpc.Server) *grpc.ClientConn {
	t.Helper()
	lis := bufconn.Listen(1024 * 1024)
	go func() {
		if err := srv.Serve(lis); err != nil {
			t.Logf("idempotency test server exited: %v", err)
		}
	}()

	//nolint:staticcheck // bufconn dialling mirrors the other serve tests
	conn, err := grpc.DialContext(context.Background(), "bufnet",
		grpc.WithContextDialer(func(context.Context, string) (net.Conn, error) { return lis.Dial() }),
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	require.NoError(t, err)
	t.Cleanup(func() {
		_ = conn.Close()
		srv.Stop()
		_ = lis.Close()
	})
	return conn
}

// keyHarnessServer records the key of each harness request that starts work.
type keyHarnessServer struct {
	harnesspb.UnimplementedHarnessCallbackServiceServer
	rec keyRecorder
}

func (s *keyHarnessServer) SubmitFinding(_ context.Context, req *harnesspb.SubmitFindingRequest) (*harnesspb.SubmitFindingResponse, error) {
	s.rec.add(req.GetIdempotencyKey())
	return &harnesspb.SubmitFindingResponse{}, nil
}

func (s *keyHarnessServer) RunMission(_ context.Context, req *harnesspb.RunMissionRequest) (*harnesspb.RunMissionResponse, error) {
	s.rec.add(req.GetIdempotencyKey())
	return &harnesspb.RunMissionResponse{}, nil
}

// TestCallbackHarness_SendsAnIdempotencyKey proves that the harness sends a new
// key on each submit and run call (ADR-0028, sdk#207).
func TestCallbackHarness_SendsAnIdempotencyKey(t *testing.T) {
	srvImpl := &keyHarnessServer{}
	srv := grpc.NewServer()
	harnesspb.RegisterHarnessCallbackServiceServer(srv, srvImpl)
	conn := dialBufconn(t, srv)

	h := &CallbackHarness{
		client: &CallbackClient{
			conn:      conn,
			client:    harnesspb.NewHarnessCallbackServiceClient(conn),
			connected: true,
			missionID: "mission-1",
			agentName: "recon",
			taskID:    "task-1",
		},
		tracer: defaultNoopTracer(),
	}
	ctx := context.Background()

	for range 2 {
		require.NoError(t, h.SubmitFinding(ctx, &finding.Finding{Title: "open port"}))
		require.NoError(t, h.RunMission(ctx, "mission-2", nil))
	}

	srvImpl.rec.assertOneKeyForEachCall(t, 4)
}

// keyComponentServer records the key of each component request that creates
// something or starts work.
type keyComponentServer struct {
	componentpb.UnimplementedComponentServiceServer
	rec keyRecorder
}

func (s *keyComponentServer) SubmitResult(_ context.Context, req *componentpb.SubmitResultRequest) (*componentpb.SubmitResultResponse, error) {
	s.rec.add(req.GetIdempotencyKey())
	return &componentpb.SubmitResultResponse{}, nil
}

func (s *keyComponentServer) SubmitFinding(_ context.Context, req *componentpb.SubmitFindingRequest) (*componentpb.SubmitFindingResponse, error) {
	s.rec.add(req.GetIdempotencyKey())
	return &componentpb.SubmitFindingResponse{FindingId: "f-1"}, nil
}

func (s *keyComponentServer) CreateMission(_ context.Context, req *componentpb.CreateMissionRequest) (*componentpb.CreateMissionResponse, error) {
	s.rec.add(req.GetIdempotencyKey())
	return &componentpb.CreateMissionResponse{}, nil
}

func (s *keyComponentServer) RunMission(_ context.Context, req *componentpb.RunMissionRequest) (*componentpb.RunMissionResponse, error) {
	s.rec.add(req.GetIdempotencyKey())
	return &componentpb.RunMissionResponse{}, nil
}

// TestPlatformClient_SendsAnIdempotencyKey proves that the platform client
// sends a new key on each create, run and submit call (ADR-0028, sdk#207).
func TestPlatformClient_SendsAnIdempotencyKey(t *testing.T) {
	srvImpl := &keyComponentServer{}
	srv := grpc.NewServer()
	componentpb.RegisterComponentServiceServer(srv, srvImpl)
	conn := dialBufconn(t, srv)

	pc := &PlatformClient{
		conn:       conn,
		service:    componentpb.NewComponentServiceClient(conn),
		instanceID: "test-instance",
	}
	ctx := context.Background()

	for range 2 {
		require.NoError(t, pc.SubmitResult(ctx, "work-1", []byte("{}"), nil))

		findingID, err := pc.SubmitFinding(ctx, "work-1", []byte(`{"title":"open port"}`))
		require.NoError(t, err)
		assert.Equal(t, "f-1", findingID)

		_, err = pc.CreateMission(ctx, "work-1", []byte("{}"), "target-1", nil)
		require.NoError(t, err)

		require.NoError(t, pc.RunMission(ctx, "work-1", "mission-2", nil))
	}

	srvImpl.rec.assertOneKeyForEachCall(t, 8)
}
