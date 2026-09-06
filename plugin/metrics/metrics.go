// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Zero Root AI

// Package metrics defines the observability interface the Gibson plugin SDK
// emits through. The SDK ships with a no-op default so customers that do not
// care about metrics get zero-cost, zero-panic behaviour by default.
//
// To record real metrics, wire in a concrete implementation before calling
// plugin.Serve. The sister package plugin/metrics/prometheus provides a
// ready-made Prometheus implementation:
//
//	import pluginprom "github.com/zeroroot-ai/sdk/plugin/metrics/prometheus"
//
//	func main() {
//	    reg := prometheus.NewRegistry()
//	    metrics.SetDefault(pluginprom.New(reg))
//	    // ... plugin.Serve(...)
//	}
//
// Metric names and label semantics when a Prometheus recorder is used:
//
//   - gibson_plugin_startup_seconds{plugin}                 Histogram
//   - gibson_plugin_invoke_duration_seconds{plugin,method,result}  Histogram
//   - gibson_plugin_rotation_propagation_seconds{plugin}    Histogram
//   - gibson_plugin_invoke_total{plugin,method,result}      Counter
//   - gibson_plugin_lifecycle_transition_total{plugin,from,to}  Counter
//   - gibson_plugin_state{plugin,install_id}                Gauge
package metrics

import (
	"time"

	"github.com/zeroroot-ai/sdk/plugin/lifecycle"
)

// Result is the bounded enumeration recorded as the result label on
// invocation metrics. Callers MUST use these constants rather than free-form
// strings to keep cardinality bounded.
type Result string

const (
	ResultOK               Result = "ok"
	ResultError            Result = "error"
	ResultDeadlineExceeded Result = "deadline_exceeded"
	ResultPanic            Result = "panic"
	ResultMethodNotFound   Result = "method_not_found"
)

// Recorder is the observability surface the plugin SDK emits metrics through.
// A nil Recorder is safe: all call sites guard with if r == nil.
// Use SetDefault to replace the package-level no-op with a real implementation.
type Recorder interface {
	ObserveStartup(plugin string, t0 time.Time)
	ObserveInvocation(plugin, method string, result Result, duration time.Duration)
	ObserveRotationPropagation(plugin string, lag time.Duration)
	RecordTransition(plugin, installID string, from, to lifecycle.State)
	SetState(plugin, installID string, state lifecycle.State)
}

// Default is the package-level Recorder. It starts as a no-op so that
// importing this package never mutates any global Prometheus registry.
// Replace it with SetDefault before calling plugin.Serve.
var Default Recorder = nopRecorder{}

// SetDefault replaces the package-level Default recorder. It is not
// goroutine-safe; call it once during program initialisation before any
// plugin.Serve call.
func SetDefault(r Recorder) {
	if r == nil {
		r = nopRecorder{}
	}
	Default = r
}

// nopRecorder is the zero-cost default. Every method is a no-op.
type nopRecorder struct{}

func (nopRecorder) ObserveStartup(_ string, _ time.Time)                                      {}
func (nopRecorder) ObserveInvocation(_ string, _ string, _ Result, _ time.Duration)           {}
func (nopRecorder) ObserveRotationPropagation(_ string, _ time.Duration)                      {}
func (nopRecorder) RecordTransition(_ string, _ string, _ lifecycle.State, _ lifecycle.State) {}
func (nopRecorder) SetState(_ string, _ string, _ lifecycle.State)                            {}
