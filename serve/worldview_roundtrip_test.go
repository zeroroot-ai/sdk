// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Zero Root AI

package serve

import (
	"context"
	"errors"
	"net"
	"testing"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/test/bufconn"

	"github.com/zeroroot-ai/sdk/agent"
	harnesspb "github.com/zeroroot-ai/sdk/api/gen/gibson/harness/v1"
)

// worldViewServer is a HarnessCallbackService that answers only WorldView.
type worldViewServer struct {
	harnesspb.UnimplementedHarnessCallbackServiceServer

	gotRequest *harnesspb.WorldViewRequest
	resp       *harnesspb.WorldViewResponse
	err        error
}

func (s *worldViewServer) WorldView(_ context.Context, req *harnesspb.WorldViewRequest) (*harnesspb.WorldViewResponse, error) {
	s.gotRequest = req
	return s.resp, s.err
}

// setupWorldViewHarness dials an in-process daemon serving only WorldView and
// returns a CallbackHarness wired to it.
func setupWorldViewHarness(t *testing.T, srvImpl *worldViewServer) *CallbackHarness {
	t.Helper()

	lis := bufconn.Listen(1024 * 1024)
	srv := grpc.NewServer()
	harnesspb.RegisterHarnessCallbackServiceServer(srv, srvImpl)
	go func() {
		if err := srv.Serve(lis); err != nil {
			t.Logf("worldview test server exited: %v", err)
		}
	}()

	//nolint:staticcheck // bufconn dialling mirrors the other serve tests
	conn, err := grpc.DialContext(context.Background(), "bufnet",
		grpc.WithContextDialer(func(context.Context, string) (net.Conn, error) { return lis.Dial() }),
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	t.Cleanup(func() {
		conn.Close()
		srv.Stop()
		lis.Close()
	})

	client := &CallbackClient{
		conn:      conn,
		client:    harnesspb.NewHarnessCallbackServiceClient(conn),
		connected: true,
		missionID: "mission-1",
		agentName: "recon",
		taskID:    "task-1",
	}
	return &CallbackHarness{client: client, tracer: defaultNoopTracer()}
}

func TestCallbackHarnessWorldView_RoundTrip(t *testing.T) {
	srv := &worldViewServer{resp: &harnesspb.WorldViewResponse{
		Entities: []*harnesspb.WorldEntity{{
			Handle:     "opaque-1",
			Kind:       harnesspb.WorldEntityKind_WORLD_ENTITY_KIND_HOST,
			Label:      "10.0.0.1",
			Attributes: map[string]string{"open_ports": "22"},
		}},
	}}
	h := setupWorldViewHarness(t, srv)

	view, err := h.WorldView(context.Background(), agent.Handle("opaque-1"))
	if err != nil {
		t.Fatalf("WorldView: %v", err)
	}
	if len(view.Entities) != 1 || view.Entities[0].Label != "10.0.0.1" {
		t.Fatalf("entities = %+v", view.Entities)
	}
	if got := srv.gotRequest.GetFocus(); len(got) != 1 || got[0] != "opaque-1" {
		t.Errorf("focus = %v, want [opaque-1]", got)
	}
	// The client stamps the addressing context; it never sends a tenant or a
	// scope, because the request has no field for either.
	if srv.gotRequest.GetContext().GetMissionId() != "mission-1" {
		t.Errorf("mission_id = %q, want mission-1", srv.gotRequest.GetContext().GetMissionId())
	}
}

// TestCallbackHarnessWorldView_RejectionIsAnError pins that a daemon refusal —
// the shape a slice-boundary violation takes on the wire — reaches the agent as
// an error rather than as an empty slice it would read as "the World is empty".
func TestCallbackHarnessWorldView_RejectionIsAnError(t *testing.T) {
	srv := &worldViewServer{resp: &harnesspb.WorldViewResponse{
		Error: &harnesspb.HarnessError{Message: "handle was not issued to this task"},
	}}
	h := setupWorldViewHarness(t, srv)

	view, err := h.WorldView(context.Background(), agent.Handle("forged"))
	if err == nil {
		t.Fatal("a rejected WorldView returned nil error")
	}
	if len(view.Entities) != 0 {
		t.Errorf("rejected WorldView returned %d entities", len(view.Entities))
	}
}

func TestCallbackHarnessWorldView_TransportError(t *testing.T) {
	srv := &worldViewServer{err: errors.New("boom")}
	h := setupWorldViewHarness(t, srv)

	if _, err := h.WorldView(context.Background()); err == nil {
		t.Fatal("transport failure returned nil error")
	}
}

func TestPlatformHarnessWorldView_Unsupported(t *testing.T) {
	var h PlatformHarness
	if _, err := h.WorldView(context.Background()); err == nil {
		t.Fatal("platform pull-mode WorldView must fail loudly, not return an empty slice")
	}
}

func TestBaseHarnessWorldViewIsNotImplemented(t *testing.T) {
	var b agent.BaseHarness
	if _, err := b.WorldView(context.Background()); err == nil {
		t.Fatal("BaseHarness.WorldView must fail rather than return an empty World")
	}
}
