// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Zero Root AI

package serve

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"testing"
	"time"
)

// pickFreePort returns an available TCP port on localhost.
func pickFreePort(t *testing.T) int {
	t.Helper()
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen on 127.0.0.1:0: %v", err)
	}
	port := l.Addr().(*net.TCPAddr).Port
	l.Close()
	return port
}

// fetchHealth GETs an endpoint on the health server and returns status code
// plus the parsed JSON body.
func fetchHealth(t *testing.T, port int, path string) (int, map[string]any) {
	t.Helper()
	url := fmt.Sprintf("http://127.0.0.1:%d%s", port, path)
	resp, err := http.Get(url) //nolint:gosec // local test
	if err != nil {
		t.Fatalf("GET %s: %v", url, err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	var parsed map[string]any
	if len(body) > 0 {
		_ = json.Unmarshal(body, &parsed)
	}
	return resp.StatusCode, parsed
}

func waitForPort(port int, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		conn, err := net.DialTimeout("tcp", fmt.Sprintf("127.0.0.1:%d", port), 50*time.Millisecond)
		if err == nil {
			conn.Close()
			return nil
		}
		time.Sleep(20 * time.Millisecond)
	}
	return fmt.Errorf("port %d not ready after %s", port, timeout)
}

func TestStartComponentHealthServer_Disabled(t *testing.T) {
	cfg := &Config{HealthPort: -1}
	srv, state, stop := startComponentHealthServer(cfg, "tool", "noop")
	defer stop()
	if srv != nil {
		t.Errorf("expected nil server when HealthPort < 0, got %v", srv)
	}
	if state != nil {
		t.Errorf("expected nil state when HealthPort < 0, got %v", state)
	}
}

func TestStartComponentHealthServer_LivenessAlwaysHealthy(t *testing.T) {
	port := pickFreePort(t)
	cfg := &Config{HealthPort: port}
	srv, state, stop := startComponentHealthServer(cfg, "tool", "alive-tool")
	defer stop()
	if srv == nil {
		t.Fatal("expected health server to start")
	}
	if state == nil {
		t.Fatal("expected health state to be created")
	}
	if err := waitForPort(port, 2*time.Second); err != nil {
		t.Fatal(err)
	}

	// Liveness must report healthy even before registration.
	code, body := fetchHealth(t, port, "/healthz")
	if code != http.StatusOK {
		t.Errorf("/healthz before registration: got %d, want 200; body=%v", code, body)
	}
	if body["status"] != "healthy" {
		t.Errorf("/healthz status field = %v, want healthy", body["status"])
	}
}

func TestStartComponentHealthServer_ReadinessFlipsAfterRegistration(t *testing.T) {
	port := pickFreePort(t)
	cfg := &Config{HealthPort: port}
	_, state, stop := startComponentHealthServer(cfg, "tool", "ready-tool")
	defer stop()
	if state == nil {
		t.Fatal("expected health state to be created")
	}
	if err := waitForPort(port, 2*time.Second); err != nil {
		t.Fatal(err)
	}

	// Before registration: 503.
	code, _ := fetchHealth(t, port, "/readyz")
	if code != http.StatusServiceUnavailable {
		t.Errorf("/readyz before registration: got %d, want 503", code)
	}

	// After registration: 200.
	state.markRegistered(10 * time.Second)
	code, body := fetchHealth(t, port, "/readyz")
	if code != http.StatusOK {
		t.Errorf("/readyz after registration: got %d, want 200; body=%v", code, body)
	}
}

func TestStartComponentHealthServer_ReadinessTripsOnHeartbeatStaleness(t *testing.T) {
	port := pickFreePort(t)
	cfg := &Config{HealthPort: port}
	_, state, stop := startComponentHealthServer(cfg, "tool", "stale-tool")
	defer stop()
	if state == nil {
		t.Fatal("expected health state to be created")
	}
	if err := waitForPort(port, 2*time.Second); err != nil {
		t.Fatal(err)
	}

	// Use a tiny heartbeat interval so the staleness window expires quickly.
	state.markRegistered(50 * time.Millisecond)

	// Immediately after registration: healthy.
	code, _ := fetchHealth(t, port, "/readyz")
	if code != http.StatusOK {
		t.Fatalf("/readyz immediately after registration: got %d, want 200", code)
	}

	// Sleep past 3x heartbeat interval (the staleness window).
	time.Sleep(250 * time.Millisecond)

	code, body := fetchHealth(t, port, "/readyz")
	if code != http.StatusServiceUnavailable {
		t.Errorf("/readyz after staleness: got %d, want 503; body=%v", code, body)
	}

	// A fresh heartbeat should restore readiness.
	state.markHeartbeat()
	code, _ = fetchHealth(t, port, "/readyz")
	if code != http.StatusOK {
		t.Errorf("/readyz after fresh heartbeat: got %d, want 200", code)
	}
}

func TestStartComponentHealthServer_StopIsIdempotent(t *testing.T) {
	port := pickFreePort(t)
	cfg := &Config{HealthPort: port}
	_, _, stop := startComponentHealthServer(cfg, "agent", "stop-twice")
	if err := waitForPort(port, 2*time.Second); err != nil {
		t.Fatal(err)
	}
	stop()
	stop() // must not panic or error
	// Server should now reject connections.
	if _, err := net.DialTimeout("tcp", fmt.Sprintf("127.0.0.1:%d", port), 200*time.Millisecond); err == nil {
		t.Error("expected dial to fail after stop, succeeded")
	}
}

func TestStartComponentHealthServer_PortBindFailureNonFatal(t *testing.T) {
	// Hold a port hostage so the health server cannot bind to it.
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer l.Close()
	taken := l.Addr().(*net.TCPAddr).Port

	cfg := &Config{HealthPort: taken}
	srv, state, stop := startComponentHealthServer(cfg, "plugin", "bind-fail")
	defer stop()

	if srv != nil {
		t.Errorf("expected nil server on bind failure, got %v", srv)
	}
	// State is still returned so callers can mark heartbeats without nil
	// checks; only the HTTP server itself is absent.
	if state == nil {
		t.Error("expected non-nil state on bind failure")
	}
}

// Sanity-check the http package import path lines up with the package alias
// we use elsewhere — guards against accidental import drift.
var _ = context.Background
