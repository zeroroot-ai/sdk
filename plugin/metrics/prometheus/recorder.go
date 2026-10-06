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

// 50ms..~12.8s

// 5ms..~40s

// 50ms..~12.8s

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
