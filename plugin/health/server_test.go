// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Zero Root AI

package health_test

import (
	"context"
	"encoding/json"
	"net"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/zeroroot-ai/sdk/plugin/health"
	"github.com/zeroroot-ai/sdk/plugin/lifecycle"
)

// advance is a test helper that walks a fresh StateMachine to the given state.
func advance(t *testing.T, target lifecycle.State) *lifecycle.StateMachine {
	t.Helper()
	sm := lifecycle.New(lifecycle.LifecycleHooks{})
	path := []lifecycle.State{
		lifecycle.Registering,
		lifecycle.ResolvingSecrets,
		lifecycle.Starting,
		lifecycle.Ready,
	}
	for _, s := range path {
		if err := sm.Transition(s); err != nil {
			t.Fatalf("advance: transition to %s: %v", s, err)
		}
		if s == target {
			return sm
		}
	}
	// Handle off-path states.
	switch target {
	case lifecycle.Bootstrapping:
		// already there on a fresh machine — shouldn't reach here
	case lifecycle.Degraded:
		if err := sm.Transition(lifecycle.Degraded); err != nil {
			t.Fatalf("advance to Degraded: %v", err)
		}
	case lifecycle.Draining:
		if err := sm.Transition(lifecycle.Draining); err != nil {
			t.Fatalf("advance to Draining: %v", err)
		}
	case lifecycle.Stopped:
		if err := sm.Transition(lifecycle.Stopped); err != nil {
			t.Fatalf("advance to Stopped: %v", err)
		}
	}
	return sm
}

// probeBody decodes the JSON probe response body from a ResponseRecorder.
type probeBody struct {
	State     string `json:"state"`
	Timestamp string `json:"timestamp"`
	Reason    string `json:"reason,omitempty"`
}

func decodeBody(t *testing.T, rr *httptest.ResponseRecorder) probeBody {
	t.Helper()
	var pb probeBody
	if err := json.NewDecoder(rr.Body).Decode(&pb); err != nil {
		t.Fatalf("decode probe body: %v", err)
	}
	return pb
}

// ---- /healthz tests ----

// TestHealthzReturns503WhenBootstrapping verifies that the startup probe is 503
// before the plugin has reached Ready.
func TestHealthzReturns503WhenBootstrapping(t *testing.T) {
	sm := lifecycle.New(lifecycle.LifecycleHooks{})
	srv := health.New(sm, ":0", 30*time.Second)

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/healthz", http.NoBody)
	srv.ServeHealthz(rr, req)

	if rr.Code != http.StatusServiceUnavailable {
		t.Errorf("/healthz: got %d, want 503 (state=Bootstrapping)", rr.Code)
	}
	body := decodeBody(t, rr)
	if body.State != "Bootstrapping" {
		t.Errorf("body.state = %q, want Bootstrapping", body.State)
	}
}

// TestHealthzReturns200WhenReady verifies 200 once the machine is in Ready.
func TestHealthzReturns200WhenReady(t *testing.T) {
	sm := advance(t, lifecycle.Ready)
	srv := health.New(sm, ":0", 30*time.Second)

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/healthz", http.NoBody)
	srv.ServeHealthz(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("/healthz: got %d, want 200 (state=Ready)", rr.Code)
	}
}

// TestHealthzReturns200WhenDegraded verifies that /healthz is still 200 in
// Degraded (the plugin is partially serving; the startup probe should remain
// healthy so the orchestrator does not restart unnecessarily).
func TestHealthzReturns200WhenDegraded(t *testing.T) {
	sm := advance(t, lifecycle.Degraded)
	srv := health.New(sm, ":0", 30*time.Second)

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/healthz", http.NoBody)
	srv.ServeHealthz(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("/healthz: got %d, want 200 (state=Degraded)", rr.Code)
	}
}

// TestHealthzReturns200WhenDraining verifies that /healthz is 200 during
// drain (allows the orchestrator to direct in-flight requests elsewhere while
// the plugin finishes work, rather than killing it immediately).
func TestHealthzReturns200WhenDraining(t *testing.T) {
	sm := advance(t, lifecycle.Draining)
	srv := health.New(sm, ":0", 30*time.Second)

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/healthz", http.NoBody)
	srv.ServeHealthz(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("/healthz: got %d, want 200 (state=Draining)", rr.Code)
	}
}

// TestHealthzReturns503WhenStarting verifies that /healthz is 503 while in
// Starting (OnStart hook has not completed yet).
func TestHealthzReturns503WhenStarting(t *testing.T) {
	sm := advance(t, lifecycle.Starting)
	srv := health.New(sm, ":0", 30*time.Second)

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/healthz", http.NoBody)
	srv.ServeHealthz(rr, req)

	if rr.Code != http.StatusServiceUnavailable {
		t.Errorf("/healthz: got %d, want 503 (state=Starting)", rr.Code)
	}
}

// TestHealthzReturns503WhenStopped verifies that /healthz is 503 once the
// plugin has stopped (it should no longer pass the startup probe).
func TestHealthzReturns503WhenStopped(t *testing.T) {
	sm := advance(t, lifecycle.Stopped)
	srv := health.New(sm, ":0", 30*time.Second)

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/healthz", http.NoBody)
	srv.ServeHealthz(rr, req)

	if rr.Code != http.StatusServiceUnavailable {
		t.Errorf("/healthz: got %d, want 503 (state=Stopped)", rr.Code)
	}
}

// TestHealthzBodyContainsStateAndTimestamp checks the response body structure.
func TestHealthzBodyContainsStateAndTimestamp(t *testing.T) {
	sm := advance(t, lifecycle.Ready)
	srv := health.New(sm, ":0", 30*time.Second)

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/healthz", http.NoBody)
	srv.ServeHealthz(rr, req)

	body := decodeBody(t, rr)
	if body.State == "" {
		t.Error("body.state is empty")
	}
	if body.Timestamp == "" {
		t.Error("body.timestamp is empty")
	}
	if _, err := time.Parse(time.RFC3339, body.Timestamp); err != nil {
		t.Errorf("body.timestamp %q is not RFC3339: %v", body.Timestamp, err)
	}
}

// ---- /livez tests ----

// TestLivezReturns503WhenNoHeartbeat verifies that /livez is 503 when the
// plugin is Ready but has never received a daemon heartbeat.
func TestLivezReturns503WhenNoHeartbeat(t *testing.T) {
	sm := advance(t, lifecycle.Ready)
	srv := health.New(sm, ":0", 30*time.Second)
	// No RecordHeartbeat call.

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/livez", http.NoBody)
	srv.ServeLivez(rr, req)

	if rr.Code != http.StatusServiceUnavailable {
		t.Errorf("/livez: got %d, want 503 (no heartbeat)", rr.Code)
	}
}

// TestLivezReturns200WhenRecentHeartbeat verifies that /livez is 200 when the
// plugin is Ready and a heartbeat was just recorded.
func TestLivezReturns200WhenRecentHeartbeat(t *testing.T) {
	sm := advance(t, lifecycle.Ready)
	// Use a large window so the just-recorded heartbeat is never stale.
	srv := health.New(sm, ":0", 24*time.Hour)
	srv.RecordHeartbeat()

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/livez", http.NoBody)
	srv.ServeLivez(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("/livez: got %d, want 200 (recent heartbeat)", rr.Code)
	}
}

// TestLivezReturns503WhenHeartbeatTooOld verifies that /livez is 503 when the
// last heartbeat is older than the liveness window.
func TestLivezReturns503WhenHeartbeatTooOld(t *testing.T) {
	sm := advance(t, lifecycle.Ready)
	// Use a 1 ns window — any recorded heartbeat will immediately be stale.
	srv := health.New(sm, ":0", 1*time.Nanosecond)
	srv.RecordHeartbeat()

	// Ensure at least 1 ns passes before probing.
	time.Sleep(2 * time.Nanosecond)

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/livez", http.NoBody)
	srv.ServeLivez(rr, req)

	if rr.Code != http.StatusServiceUnavailable {
		t.Errorf("/livez: got %d, want 503 (stale heartbeat)", rr.Code)
	}
}

// TestLivezReturns200WhenDegradedAndRecentHeartbeat verifies that /livez
// returns 200 in Degraded state when heartbeat is fresh (Degraded is a
// valid serving state — the plugin is still handling some requests).
func TestLivezReturns200WhenDegradedAndRecentHeartbeat(t *testing.T) {
	sm := advance(t, lifecycle.Degraded)
	srv := health.New(sm, ":0", 24*time.Hour)
	srv.RecordHeartbeat()

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/livez", http.NoBody)
	srv.ServeLivez(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("/livez: got %d, want 200 (Degraded + recent heartbeat)", rr.Code)
	}
}

// TestLivezReturns503WhenDraining verifies that /livez is 503 in Draining
// (the plugin is shutting down and should not receive new liveness-gated work).
func TestLivezReturns503WhenDraining(t *testing.T) {
	sm := advance(t, lifecycle.Draining)
	srv := health.New(sm, ":0", 24*time.Hour)
	srv.RecordHeartbeat()

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/livez", http.NoBody)
	srv.ServeLivez(rr, req)

	if rr.Code != http.StatusServiceUnavailable {
		t.Errorf("/livez: got %d, want 503 (Draining)", rr.Code)
	}
}

// TestLivezReturns503WhenBootstrapping verifies that /livez is 503 while the
// plugin is still bootstrapping (not yet ready to serve).
func TestLivezReturns503WhenBootstrapping(t *testing.T) {
	sm := lifecycle.New(lifecycle.LifecycleHooks{})
	srv := health.New(sm, ":0", 24*time.Hour)
	srv.RecordHeartbeat()

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/livez", http.NoBody)
	srv.ServeLivez(rr, req)

	if rr.Code != http.StatusServiceUnavailable {
		t.Errorf("/livez: got %d, want 503 (Bootstrapping)", rr.Code)
	}
}

// TestLivezBodyContainsReason verifies that 503 responses include a reason.
func TestLivezBodyContainsReason(t *testing.T) {
	sm := lifecycle.New(lifecycle.LifecycleHooks{})
	srv := health.New(sm, ":0", 30*time.Second)

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/livez", http.NoBody)
	srv.ServeLivez(rr, req)

	body := decodeBody(t, rr)
	if body.Reason == "" {
		t.Error("503 livez response should include a reason")
	}
}

// ---- Start / shutdown tests ----

// TestStartListensAndShutdownOnCtxCancel verifies that Start binds a real TCP
// port and that cancelling the context causes the server to stop accepting.
func TestStartListensAndShutdownOnCtxCancel(t *testing.T) {
	sm := advance(t, lifecycle.Ready)
	srv := health.New(sm, "127.0.0.1:0", 30*time.Second)
	srv.RecordHeartbeat()

	ctx, cancel := context.WithCancel(context.Background())
	addr, err := srv.Start(ctx)
	if err != nil {
		t.Fatalf("Start returned error: %v", err)
	}

	// Confirm the server is reachable before cancellation.
	resp, err := http.Get("http://" + addr + "/healthz")
	if err != nil {
		t.Fatalf("GET /healthz before cancel: %v", err)
	}
	resp.Body.Close()

	// Cancel the context and wait briefly for graceful shutdown.
	cancel()
	time.Sleep(100 * time.Millisecond)

	// The server should no longer be reachable.
	_, err = http.Get("http://" + addr + "/healthz")
	if err == nil {
		t.Error("expected error after server shutdown, got nil")
	}
}

// TestStartReturnsBindError verifies that Start returns an error if the listen
// address is unavailable (e.g. already in use).
func TestStartReturnsBindError(t *testing.T) {
	// Bind to port 0 first to claim a random port.
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Skipf("could not bind test listener: %v", err)
	}
	defer ln.Close()
	addr := ln.Addr().String()

	sm := lifecycle.New(lifecycle.LifecycleHooks{})
	srv := health.New(sm, addr, 30*time.Second)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	_, err = srv.Start(ctx)
	if err == nil {
		t.Error("Start on already-bound address should return an error")
	}
}

// ---- State-transition probe integration test ----

// TestProbeResponseFollowsStateTransitions drives the lifecycle state machine
// through all major transitions and verifies that probe status codes change
// accordingly.
func TestProbeResponseFollowsStateTransitions(t *testing.T) {
	sm := lifecycle.New(lifecycle.LifecycleHooks{})
	srv := health.New(sm, ":0", 24*time.Hour)
	srv.RecordHeartbeat()

	probe := func(path string, handler func(http.ResponseWriter, *http.Request)) int {
		rr := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, path, http.NoBody)
		handler(rr, req)
		return rr.Code
	}

	// Bootstrapping: both probes 503.
	if got := probe("/healthz", srv.ServeHealthz); got != 503 {
		t.Errorf("Bootstrapping /healthz = %d, want 503", got)
	}
	if got := probe("/livez", srv.ServeLivez); got != 503 {
		t.Errorf("Bootstrapping /livez = %d, want 503", got)
	}

	mustTransition := func(target lifecycle.State) {
		t.Helper()
		if err := sm.Transition(target); err != nil {
			t.Fatalf("transition to %s: %v", target, err)
		}
	}

	// Advance to Ready.
	mustTransition(lifecycle.Registering)
	mustTransition(lifecycle.ResolvingSecrets)
	mustTransition(lifecycle.Starting)
	mustTransition(lifecycle.Ready)

	if got := probe("/healthz", srv.ServeHealthz); got != 200 {
		t.Errorf("Ready /healthz = %d, want 200", got)
	}
	if got := probe("/livez", srv.ServeLivez); got != 200 {
		t.Errorf("Ready /livez = %d, want 200", got)
	}

	// Degrade.
	mustTransition(lifecycle.Degraded)
	if got := probe("/healthz", srv.ServeHealthz); got != 200 {
		t.Errorf("Degraded /healthz = %d, want 200", got)
	}
	if got := probe("/livez", srv.ServeLivez); got != 200 {
		t.Errorf("Degraded /livez = %d, want 200", got)
	}

	// Recover.
	mustTransition(lifecycle.Ready)
	if got := probe("/healthz", srv.ServeHealthz); got != 200 {
		t.Errorf("Recovered Ready /healthz = %d, want 200", got)
	}

	// Drain.
	mustTransition(lifecycle.Draining)
	if got := probe("/healthz", srv.ServeHealthz); got != 200 {
		t.Errorf("Draining /healthz = %d, want 200", got)
	}
	if got := probe("/livez", srv.ServeLivez); got != 503 {
		t.Errorf("Draining /livez = %d, want 503", got)
	}

	// Stop.
	mustTransition(lifecycle.Stopped)
	if got := probe("/healthz", srv.ServeHealthz); got != 503 {
		t.Errorf("Stopped /healthz = %d, want 503", got)
	}
	if got := probe("/livez", srv.ServeLivez); got != 503 {
		t.Errorf("Stopped /livez = %d, want 503", got)
	}
}
