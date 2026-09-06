// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Zero Root AI

// Package health implements the HTTP health-probe server for the Gibson plugin
// SDK.
//
// Two endpoints are served:
//
//   - /healthz — startup probe: returns 200 once the plugin has reached
//     [lifecycle.Ready], [lifecycle.Degraded], or [lifecycle.Draining]; 503
//     otherwise. Orchestrators (Kubernetes startupProbe, systemd
//     ReadinessGate) poll this until it flips to 200 before routing traffic.
//
//   - /livez — liveness probe: returns 200 only when the plugin is in
//     [lifecycle.Ready] or [lifecycle.Degraded] AND the most recent daemon
//     heartbeat completed within the configured livenessWindow; 503 otherwise.
//     Kubernetes livenessProbe restarts the pod when this returns 503.
//
// The server is non-blocking: [Server.Start] launches the listener in a
// background goroutine and returns immediately. The server shuts down
// gracefully when the supplied context is cancelled.
//
// Usage:
//
//	sm := lifecycle.New(hooks)
//	srv := health.New(sm, ":8080", 30*time.Second)
//	if err := srv.Start(ctx); err != nil {
//	    log.Fatal(err)
//	}
package health

import (
	"context"
	"encoding/json"
	"log/slog"
	"net"
	"net/http"
	"sync"
	"sync/atomic"
	"time"

	"github.com/zeroroot-ai/sdk/plugin/lifecycle"
)

// probeResponse is the JSON body returned by both probe endpoints.
type probeResponse struct {
	// State is the human-readable lifecycle state name at time of the probe.
	State string `json:"state"`
	// Timestamp is the RFC 3339 UTC time at which the probe was evaluated.
	Timestamp string `json:"timestamp"`
	// Reason is an optional human-readable explanation for a 503 response.
	Reason string `json:"reason,omitempty"`
}

// Server is a non-blocking HTTP health-probe server. Construct it with [New];
// the zero value is not usable.
type Server struct {
	state          *lifecycle.StateMachine
	addr           string
	livenessWindow time.Duration

	// lastHeartbeat holds the Unix nanosecond timestamp of the most recent
	// successful daemon heartbeat. Zero means no heartbeat has been recorded.
	// Updated via atomic store/load so RecordHeartbeat is lock-free.
	lastHeartbeat atomic.Int64

	// now is the time source used by probe handlers; overridden in tests.
	now func() time.Time
}

// New constructs a [Server] that is ready to be started.
//
// state is the plugin's lifecycle state machine; the server reads its current
// state on every probe request.
//
// addr is the TCP listen address, e.g. ":8080". An empty string defaults to
// ":8080".
//
// livenessWindow is the maximum age of the last daemon heartbeat before
// /livez returns 503. This is typically sourced from the manifest's
// spec.health.liveness_interval; the Kubernetes livenessProbe
// periodSeconds/failureThreshold should be set to match so the probe does
// not fire before the plugin has had a chance to reconnect.
func New(state *lifecycle.StateMachine, addr string, livenessWindow time.Duration) *Server {
	if addr == "" {
		addr = ":8080"
	}
	return &Server{
		state:          state,
		addr:           addr,
		livenessWindow: livenessWindow,
		now:            time.Now,
	}
}

// RecordHeartbeat updates the last-heartbeat timestamp to now. This should be
// called by the daemon-callback subscriber each time it successfully receives
// any message on the component callback stream (heartbeat or other event).
func (s *Server) RecordHeartbeat() {
	s.lastHeartbeat.Store(s.now().UnixNano())
}

// Start launches the HTTP server in a background goroutine and returns
// immediately. It returns the bound listen address (e.g. "127.0.0.1:8080") so
// callers can obtain the actual port when addr was ":0". The server shuts down
// gracefully when ctx is cancelled.
//
// Start returns a non-nil error only if the TCP listener cannot be bound.
func (s *Server) Start(ctx context.Context) (string, error) {
	ln, err := net.Listen("tcp", s.addr)
	if err != nil {
		return "", err
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/healthz", s.ServeHealthz)
	mux.HandleFunc("/livez", s.ServeLivez)

	httpsrv := &http.Server{
		Handler: mux,
		// Conservative timeouts prevent resource exhaustion from slow clients.
		ReadHeaderTimeout: 5 * time.Second,
		WriteTimeout:      10 * time.Second,
		IdleTimeout:       60 * time.Second,
	}

	boundAddr := ln.Addr().String()

	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		slog.Info("plugin health server listening", "addr", boundAddr)
		if serveErr := httpsrv.Serve(ln); serveErr != nil && serveErr != http.ErrServerClosed {
			slog.Error("plugin health server error", "err", serveErr)
		}
	}()

	go func() {
		<-ctx.Done()
		slog.Info("plugin health server shutting down")
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if shutdownErr := httpsrv.Shutdown(shutdownCtx); shutdownErr != nil {
			slog.Warn("plugin health server shutdown error", "err", shutdownErr)
		}
		wg.Wait()
	}()

	return boundAddr, nil
}

// ServeHealthz is the http.HandlerFunc for the /healthz startup probe.
//
// Returns 200 when the current lifecycle state is [lifecycle.Ready],
// [lifecycle.Degraded], or [lifecycle.Draining] — i.e., the plugin has
// successfully completed registration and startup-secret resolution.
// Returns 503 for all other states (Bootstrapping, Registering,
// ResolvingSecrets, Starting, Stopped).
//
// This method is exported so it can be registered on a custom mux in tests
// or embedded deployments; [Server.Start] registers it automatically.
func (s *Server) ServeHealthz(w http.ResponseWriter, r *http.Request) {
	cur := s.state.Current()
	now := s.now()

	healthy := isStartupReady(cur)
	resp := probeResponse{
		State:     cur.String(),
		Timestamp: now.UTC().Format(time.RFC3339),
	}
	if !healthy {
		resp.Reason = "plugin has not yet reached ready state"
	}

	writeProbe(w, healthy, resp)
}

// ServeLivez is the http.HandlerFunc for the /livez liveness probe.
//
// Returns 200 when:
//   - the current lifecycle state is [lifecycle.Ready] or [lifecycle.Degraded], AND
//   - the time elapsed since the last daemon heartbeat is less than livenessWindow.
//
// Returns 503 otherwise. A zero last-heartbeat (no heartbeat recorded yet)
// is treated as a stale heartbeat and returns 503.
//
// This method is exported so it can be registered on a custom mux in tests
// or embedded deployments; [Server.Start] registers it automatically.
func (s *Server) ServeLivez(w http.ResponseWriter, r *http.Request) {
	cur := s.state.Current()
	now := s.now()

	resp := probeResponse{
		State:     cur.String(),
		Timestamp: now.UTC().Format(time.RFC3339),
	}

	if !isLivenessReady(cur) {
		resp.Reason = "plugin is not in a serving state"
		writeProbe(w, false, resp)
		return
	}

	lastNano := s.lastHeartbeat.Load()
	if lastNano == 0 {
		resp.Reason = "no daemon heartbeat recorded yet"
		writeProbe(w, false, resp)
		return
	}

	age := now.Sub(time.Unix(0, lastNano))
	if age >= s.livenessWindow {
		resp.Reason = "daemon heartbeat is stale"
		writeProbe(w, false, resp)
		return
	}

	writeProbe(w, true, resp)
}

// isStartupReady reports whether state represents a successfully started
// plugin for the /healthz probe.
func isStartupReady(s lifecycle.State) bool {
	switch s {
	case lifecycle.Ready, lifecycle.Degraded, lifecycle.Draining:
		return true
	default:
		return false
	}
}

// isLivenessReady reports whether state represents an actively serving plugin
// for the /livez probe.
func isLivenessReady(s lifecycle.State) bool {
	switch s {
	case lifecycle.Ready, lifecycle.Degraded:
		return true
	default:
		return false
	}
}

// writeProbe writes the probe response with the appropriate status code and
// Content-Type header. Serialisation errors are swallowed; the probe handler
// must remain non-panicking.
func writeProbe(w http.ResponseWriter, healthy bool, resp probeResponse) {
	w.Header().Set("Content-Type", "application/json")
	if healthy {
		w.WriteHeader(http.StatusOK)
	} else {
		w.WriteHeader(http.StatusServiceUnavailable)
	}
	_ = json.NewEncoder(w).Encode(resp)
}
