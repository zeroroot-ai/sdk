// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Zero Root AI

package serve

import (
	"context"
	"fmt"
	"log/slog"
	"sync/atomic"
	"time"

	healthhttp "github.com/zeroroot-ai/sdk/health/http"
	"github.com/zeroroot-ai/sdk/types"
)

// componentHealthState tracks runtime state used by the health checks
// registered against the SDK's HTTP health server.
//
// The platform serve loops are poll-based workers: they connect outbound
// to the daemon's ComponentService, register, then heartbeat and poll for
// work in a loop. Kubernetes-side health checks need a way to inspect
// whether the worker is still successfully talking to the platform; this
// struct is the bridge between the heartbeat loop and the HTTP probe
// handlers.
type componentHealthState struct {
	// registered flips to 1 once the initial RegisterComponent RPC succeeds.
	registered atomic.Bool

	// lastHeartbeatNanos records the wall-clock time of the most recent
	// successful heartbeat as a nanosecond-precision Unix timestamp. It is
	// updated from the heartbeat goroutine and read from the readiness
	// check. Nanosecond precision is required because tests exercise
	// sub-second heartbeat intervals.
	lastHeartbeatNanos atomic.Int64

	// heartbeatInterval is the interval the heartbeat goroutine is using
	// (which may have been overridden by the platform's RegisterResponse).
	// Readiness considers the worker unhealthy if no successful heartbeat
	// has occurred within heartbeatStaleMultiplier * heartbeatInterval.
	heartbeatInterval time.Duration
}

// heartbeatStaleMultiplier defines how many heartbeat intervals may pass
// without a successful heartbeat before the readiness check trips. With
// the default 10s heartbeat this is 30s, which leaves headroom for one
// transient failure but trips well before the daemon's 30s registry TTL.
const heartbeatStaleMultiplier = 3

// markRegistered records that the component has successfully registered
// with the platform. Until this is set, /readyz returns 503.
func (s *componentHealthState) markRegistered(heartbeatInterval time.Duration) {
	s.heartbeatInterval = heartbeatInterval
	// Seed the heartbeat timestamp with "now" so the readiness check does
	// not immediately trip during the first heartbeat interval.
	s.lastHeartbeatNanos.Store(time.Now().UnixNano())
	s.registered.Store(true)
}

// markHeartbeat records a successful heartbeat. Called from the heartbeat
// goroutine in each platform serve loop.
func (s *componentHealthState) markHeartbeat() {
	s.lastHeartbeatNanos.Store(time.Now().UnixNano())
}

// livenessCheck reports the process as alive. Liveness must only fail when
// the process itself is unrecoverable; the daemon's heartbeat staleness
// belongs in readiness, not liveness, so kubelet does not flap the pod on
// transient platform issues.
func (s *componentHealthState) livenessCheck(_ context.Context) types.HealthStatus {
	return types.NewHealthyStatus("process alive")
}

// readinessCheck reports whether the worker is ready to handle work:
//   - it has successfully registered with the platform, and
//   - it has produced a successful heartbeat within the staleness window.
func (s *componentHealthState) readinessCheck(_ context.Context) types.HealthStatus {
	if !s.registered.Load() {
		return types.NewUnhealthyStatus("not yet registered with platform", nil)
	}

	last := s.lastHeartbeatNanos.Load()
	if last == 0 {
		return types.NewUnhealthyStatus("no heartbeat recorded yet", nil)
	}

	if s.heartbeatInterval > 0 {
		staleAfter := time.Duration(heartbeatStaleMultiplier) * s.heartbeatInterval
		age := time.Since(time.Unix(0, last))
		if age > staleAfter {
			return types.NewUnhealthyStatus(
				fmt.Sprintf("last heartbeat %s ago (>%s)", age.Round(time.Millisecond), staleAfter),
				map[string]any{
					"last_heartbeat_age_seconds": age.Seconds(),
					"stale_after_seconds":        staleAfter.Seconds(),
				},
			)
		}
	}

	return types.NewHealthyStatus("registered and heartbeating")
}

// startComponentHealthServer starts the SDK HTTP health server (if enabled
// by cfg.HealthPort) and returns the running server, the health state used
// to drive the readiness check, and a stop function that gracefully shuts
// the server down.
//
// When cfg.HealthPort is < 0 the health server is disabled and this
// function returns nil values plus a no-op stop function. Callers can
// always invoke stop() in a defer regardless of the disabled state.
//
// componentKind is used purely for logging ("tool", "plugin", "agent")
// so operators can correlate startup messages.
func startComponentHealthServer(cfg *Config, componentKind, componentName string) (*healthhttp.Server, *componentHealthState, func()) {
	noop := func() {}

	if cfg == nil || cfg.HealthPort < 0 {
		slog.Info("health HTTP server disabled (HealthPort < 0)",
			"component", componentKind,
			"name", componentName,
		)
		return nil, nil, noop
	}

	port := cfg.HealthPort
	if port == 0 {
		port = 8080
	}

	state := &componentHealthState{}

	hcfg := healthhttp.DefaultConfig()
	hcfg.Port = port

	srv := healthhttp.NewServer(hcfg)
	srv.RegisterLivenessCheck(componentKind, state.livenessCheck)
	srv.RegisterReadinessCheck(componentKind, state.readinessCheck)

	if err := srv.Start(); err != nil {
		// Failing to bind the health port is non-fatal — components are
		// still able to serve work — but it almost certainly means K8s
		// probes will fail. Log loudly and continue rather than crashing
		// the worker, since orphan processes are worse than degraded
		// observability.
		slog.Error("failed to start health HTTP server, K8s probes will fail",
			"component", componentKind,
			"name", componentName,
			"port", port,
			"error", err,
		)
		return nil, state, noop
	}

	slog.Info("health HTTP server listening",
		"component", componentKind,
		"name", componentName,
		"port", port,
		"endpoints", []string{"/healthz", "/readyz"},
	)

	stop := func() {
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := srv.Stop(shutdownCtx); err != nil {
			slog.Warn("health HTTP server shutdown error",
				"component", componentKind,
				"name", componentName,
				"error", err,
			)
		}
	}

	return srv, state, stop
}
