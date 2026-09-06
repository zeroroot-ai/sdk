// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Zero Root AI

// Package prometheus provides a Prometheus-backed implementation of
// [metrics.Recorder] for the Gibson plugin SDK. Import this package explicitly
// when you want Gibson plugin metrics to appear in your Prometheus registry.
// Importing this package alone does NOT register anything — call [New] and
// pass the result to [metrics.SetDefault].
//
// Example:
//
//	import (
//	    "github.com/prometheus/client_golang/prometheus"
//	    pluginprom "github.com/zeroroot-ai/sdk/plugin/metrics/prometheus"
//	    "github.com/zeroroot-ai/sdk/plugin/metrics"
//	)
//
//	reg := prometheus.NewRegistry()
//	metrics.SetDefault(pluginprom.New(reg))
package prometheus

import (
	"time"

	"github.com/prometheus/client_golang/prometheus"

	"github.com/zeroroot-ai/sdk/plugin/lifecycle"
	"github.com/zeroroot-ai/sdk/plugin/metrics"
)

// startupBuckets covers the NFR Performance startup budget (5s) plus a small
// margin for the long tail.
var startupBuckets = prometheus.ExponentialBuckets(0.05, 2, 9) // 50ms..~12.8s

// invokeBuckets covers per-invocation handler durations up to ~40s.
var invokeBuckets = prometheus.ExponentialBuckets(0.005, 2, 14) // 5ms..~40s

// rotationBuckets covers the rotation-propagation 5s p95 SLO.
var rotationBuckets = prometheus.ExponentialBuckets(0.05, 2, 9) // 50ms..~12.8s

// Recorder implements [metrics.Recorder] using Prometheus metric vectors
// registered with a caller-supplied [prometheus.Registerer].
type Recorder struct {
	startupSeconds             *prometheus.HistogramVec
	invokeDurationSeconds      *prometheus.HistogramVec
	rotationPropagationSeconds *prometheus.HistogramVec
	invokeTotal                *prometheus.CounterVec
	lifecycleTransitionTotal   *prometheus.CounterVec
	stateGauge                 *prometheus.GaugeVec
}

var _ metrics.Recorder = (*Recorder)(nil)

// New allocates and registers all Gibson plugin metric vectors with reg,
// then returns a Recorder backed by them. Panics via reg.MustRegister if
// any name collides — use a fresh [prometheus.Registry] per process or
// per test to avoid conflicts.
func New(reg prometheus.Registerer) *Recorder {
	r := &Recorder{
		startupSeconds: prometheus.NewHistogramVec(
			prometheus.HistogramOpts{
				Name:    "gibson_plugin_startup_seconds",
				Help:    "Plugin startup time, measured from plugin.Serve entry to the first transition into the Ready lifecycle state.",
				Buckets: startupBuckets,
			},
			[]string{"plugin"},
		),
		invokeDurationSeconds: prometheus.NewHistogramVec(
			prometheus.HistogramOpts{
				Name:    "gibson_plugin_invoke_duration_seconds",
				Help:    "Per-invocation handler duration in seconds, labeled by plugin, method, and result outcome.",
				Buckets: invokeBuckets,
			},
			[]string{"plugin", "method", "result"},
		),
		rotationPropagationSeconds: prometheus.NewHistogramVec(
			prometheus.HistogramOpts{
				Name:    "gibson_plugin_rotation_propagation_seconds",
				Help:    "End-to-end secret rotation propagation latency, measured from the secret_rotated event server-side timestamp to the plugin's cache invalidation. SLO: p95 <= 5s.",
				Buckets: rotationBuckets,
			},
			[]string{"plugin"},
		),
		invokeTotal: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Name: "gibson_plugin_invoke_total",
				Help: "Total plugin method invocations, labeled by plugin, method, and result outcome.",
			},
			[]string{"plugin", "method", "result"},
		),
		lifecycleTransitionTotal: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Name: "gibson_plugin_lifecycle_transition_total",
				Help: "Total plugin lifecycle state transitions, labeled by plugin and the from / to state names.",
			},
			[]string{"plugin", "from", "to"},
		),
		stateGauge: prometheus.NewGaugeVec(
			prometheus.GaugeOpts{
				Name: "gibson_plugin_state",
				Help: "Current plugin lifecycle state, encoded as 0..7 (Bootstrapping=0, Registering=1, ResolvingSecrets=2, Starting=3, Ready=4, Degraded=5, Draining=6, Stopped=7).",
			},
			[]string{"plugin", "install_id"},
		),
	}
	reg.MustRegister(
		r.startupSeconds,
		r.invokeDurationSeconds,
		r.rotationPropagationSeconds,
		r.invokeTotal,
		r.lifecycleTransitionTotal,
		r.stateGauge,
	)
	return r
}

// ObserveStartup records a startup duration sample for plugin.
func (r *Recorder) ObserveStartup(plugin string, t0 time.Time) {
	r.startupSeconds.WithLabelValues(plugin).Observe(time.Since(t0).Seconds())
}

// ObserveInvocation records handler duration and increments the invocation counter.
func (r *Recorder) ObserveInvocation(plugin, method string, result metrics.Result, duration time.Duration) {
	r.invokeDurationSeconds.
		WithLabelValues(plugin, method, string(result)).
		Observe(duration.Seconds())
	r.invokeTotal.
		WithLabelValues(plugin, method, string(result)).
		Inc()
}

// ObserveRotationPropagation records the lag between a secret_rotated event
// and cache invalidation. Negative lag (clock skew) is clamped to zero.
func (r *Recorder) ObserveRotationPropagation(plugin string, lag time.Duration) {
	if lag < 0 {
		lag = 0
	}
	r.rotationPropagationSeconds.WithLabelValues(plugin).Observe(lag.Seconds())
}

// RecordTransition bumps the lifecycle transition counter and updates the
// per-install state gauge.
func (r *Recorder) RecordTransition(plugin, installID string, from, to lifecycle.State) {
	r.lifecycleTransitionTotal.
		WithLabelValues(plugin, from.String(), to.String()).
		Inc()
	if installID == "" {
		return
	}
	r.stateGauge.WithLabelValues(plugin, installID).Set(float64(to))
}

// SetState updates the per-(plugin, install_id) gauge without bumping the
// transition counter.
func (r *Recorder) SetState(plugin, installID string, state lifecycle.State) {
	if installID == "" {
		return
	}
	r.stateGauge.WithLabelValues(plugin, installID).Set(float64(state))
}
