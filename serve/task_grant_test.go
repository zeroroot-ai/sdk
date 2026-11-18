// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Zero Root AI

package serve

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"

	componentpb "github.com/zeroroot-ai/sdk/api/gen/gibson/component/v1"
	daemonpb "github.com/zeroroot-ai/sdk/api/gen/gibson/daemon/v1"
	"github.com/zeroroot-ai/sdk/openresponses"
)

// grantJWT builds a task grant with the claims of a mission task. The
// signature does not matter: the SDK reads the claims and the daemon verifies.
func grantJWT(t *testing.T, exp time.Time) string {
	t.Helper()
	tok := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"iss": "https://gibson", "aud": "zeroroot.ai/gibson-daemon", "sub": "agent-7", "jti": "j",
		"tenant": "t1", "mission_id": "m1", "task_id": "task-1",
		"iat": time.Now().Add(-time.Minute).Unix(), "exp": exp.Unix(),
	})
	s, err := tok.SignedString([]byte("k"))
	require.NoError(t, err)
	return s
}

func TestTaskGrantNeedsAGrant(t *testing.T) {
	_, err := (*taskGrant)(nil).get(context.Background())
	require.ErrorContains(t, err, "no task grant")
	_, err = newTaskGrant("", nil).get(context.Background())
	require.ErrorContains(t, err, "no task grant")
}

func TestTaskGrantIsKeptWhileItLasts(t *testing.T) {
	g := newTaskGrant(grantJWT(t, time.Now().Add(20*time.Minute)), func(context.Context, string) (string, error) {
		t.Fatal("a fresh grant must not be renewed")
		return "", nil
	})
	got, err := g.get(context.Background())
	require.NoError(t, err)
	assert.NotEmpty(t, got)
}

func TestTaskGrantIsRenewedNearItsEnd(t *testing.T) {
	old := grantJWT(t, time.Now().Add(30*time.Second))
	fresh := grantJWT(t, time.Now().Add(25*time.Minute))
	var presented string
	g := newTaskGrant(old, func(_ context.Context, cur string) (string, error) {
		presented = cur
		return fresh, nil
	})
	got, err := g.get(context.Background())
	require.NoError(t, err)
	assert.Equal(t, fresh, got)
	assert.Equal(t, old, presented, "the renewal presents the current grant")

	// The next call uses the new grant and renews nothing.
	g.renew = func(context.Context, string) (string, error) { t.Fatal("no second renewal"); return "", nil }
	got, err = g.get(context.Background())
	require.NoError(t, err)
	assert.Equal(t, fresh, got)
}

func TestTaskGrantRenewalFailure(t *testing.T) {
	fail := func(context.Context, string) (string, error) { return "", errors.New("daemon down") }

	// The grant still lasts: use it.
	g := newTaskGrant(grantJWT(t, time.Now().Add(time.Minute)), fail)
	got, err := g.get(context.Background())
	require.NoError(t, err)
	assert.NotEmpty(t, got)

	// The grant has ended: the call fails with a clear error.
	g = newTaskGrant(grantJWT(t, time.Now().Add(-time.Second)), fail)
	_, err = g.get(context.Background())
	require.ErrorContains(t, err, "has ended")
	require.ErrorContains(t, err, "daemon down")
}

func TestTaskGrantUnreadable(t *testing.T) {
	_, err := newTaskGrant("not-a-jwt", nil).get(context.Background())
	require.ErrorContains(t, err, "not readable")
}

type grantDaemon struct {
	daemonpb.UnimplementedDaemonServiceServer
	req  *daemonpb.RenewCapabilityGrantRequest
	meta metadata.MD
	next string
}

func (d *grantDaemon) RenewCapabilityGrant(
	ctx context.Context, req *daemonpb.RenewCapabilityGrantRequest,
) (*daemonpb.RenewCapabilityGrantResponse, error) {
	d.req = req
	d.meta, _ = metadata.FromIncomingContext(ctx)
	return &daemonpb.RenewCapabilityGrantResponse{CapabilityGrant: d.next}, nil
}

type pollServer struct {
	componentpb.UnimplementedComponentServiceServer
	grant string
}

func (p *pollServer) PollWork(context.Context, *componentpb.PollWorkRequest) (*componentpb.PollWorkResponse, error) {
	return &componentpb.PollWorkResponse{WorkId: "w1", WorkType: "execute", TaskGrant: p.grant}, nil
}

func TestRenewTaskGrantCallsTheDaemon(t *testing.T) {
	old := grantJWT(t, time.Now().Add(time.Minute))
	d := &grantDaemon{next: "fresh-grant"}
	h, cleanup := setupParityTestWith(t, func(s *grpc.Server) { daemonpb.RegisterDaemonServiceServer(s, d) })
	defer cleanup()

	got, err := h.client.RenewTaskGrant(context.Background(), old)
	require.NoError(t, err)
	assert.Equal(t, "fresh-grant", got)
	assert.Equal(t, "agent-7", d.req.GetAgentId())
	assert.Equal(t, "m1", d.req.GetMissionId())
	assert.Equal(t, "task-1", d.req.GetTaskId())
	assert.Equal(t, []string{old}, d.meta.Get("x-capability-grant"), "the call presents the current task grant")

	d.next = ""
	_, err = h.client.RenewTaskGrant(context.Background(), old)
	require.ErrorContains(t, err, "no grant")
	_, err = h.client.RenewTaskGrant(context.Background(), "junk")
	require.Error(t, err)
}

func TestPollWorkCarriesTheTaskGrant(t *testing.T) {
	h, cleanup := setupParityTestWith(t, func(s *grpc.Server) {
		componentpb.RegisterComponentServiceServer(s, &pollServer{grant: "task-grant-1"})
	})
	defer cleanup()
	w, err := h.client.PollWork(context.Background())
	require.NoError(t, err)
	require.NotNil(t, w)
	assert.Equal(t, "task-grant-1", w.TaskGrant)
}

func TestPlatformHarnessSendsTheTaskGrantToTheGateway(t *testing.T) {
	var got string
	gw := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		got = r.Header.Get("x-capability-grant")
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `{"status":"completed","output":[]}`)
	}))
	defer gw.Close()
	grant := grantJWT(t, time.Now().Add(20*time.Minute))
	h, cleanup := setupParityTestWith(t, func(s *grpc.Server) {
		componentpb.RegisterComponentServiceServer(s, &gatewayComponent{url: gw.URL})
	})
	defer cleanup()
	h.grant = newTaskGrant(grant, nil)

	model, err := h.Model(context.Background(), "primary")
	require.NoError(t, err)
	_, err = model.Create(context.Background(), &openresponses.Request{})
	require.NoError(t, err)
	assert.Equal(t, grant, got)

	// A work item with no grant: the call fails with a clear error and sends nothing.
	got = ""
	h.grant = newTaskGrant("", nil)
	model, err = h.Model(context.Background(), "primary")
	require.NoError(t, err)
	_, err = model.Create(context.Background(), &openresponses.Request{})
	require.ErrorContains(t, err, "no task grant")
	assert.Empty(t, got)
}

type gatewayComponent struct {
	componentpb.UnimplementedComponentServiceServer
	url string
}

func (g *gatewayComponent) ResolveModel(context.Context, *componentpb.ResolveModelRequest) (*componentpb.ResolveModelResponse, error) {
	return &componentpb.ResolveModelResponse{Model: "m", GatewayUrl: g.url}, nil
}
