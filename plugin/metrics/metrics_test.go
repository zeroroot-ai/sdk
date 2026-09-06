// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Zero Root AI

package metrics_test

import (
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus"

	"github.com/zeroroot-ai/sdk/plugin/lifecycle"
	"github.com/zeroroot-ai/sdk/plugin/metrics"
)

// TestDefaultIsNop verifies that the package-level Default is a no-op and
// never panics — including after a nil check is removed.
func TestDefaultIsNop(t *testing.T) {
	t.Parallel()
	r := metrics.Default
	if r == nil {
		t.Fatal("Default must not be nil")
	}
	// All calls must be silent no-ops.
	r.ObserveStartup("p", time.Now())
	r.ObserveInvocation("p", "M", metrics.ResultOK, time.Millisecond)
	r.ObserveRotationPropagation("p", 100*time.Millisecond)
	r.RecordTransition("p", "i-1", lifecycle.Bootstrapping, lifecycle.Registering)
	r.SetState("p", "i-1", lifecycle.Ready)
}

// TestNoGlobalRegistryPollution is the core invariant of sdk#130.
// Importing plugin/metrics must NOT register any collectors on the default
// Prometheus registry. If this test fails, an init() is silently mutating the
// global registry on import.
func TestNoGlobalRegistryPollution(t *testing.T) {
	t.Parallel()
	mfs, err := prometheus.DefaultGatherer.Gather()
	if err != nil {
		t.Skipf("could not gather default registry: %v", err)
	}
	for _, mf := range mfs {
		name := mf.GetName()
		if len(name) >= 7 && name[:7] == "gibson_" {
			t.Errorf("found gibson metric %q in default Prometheus registry; plugin/metrics must not auto-register on import", name)
		}
		if len(name) >= 16 && name[:16] == "capability_grant" {
			t.Errorf("found capability grant metric %q in default Prometheus registry; must not auto-register on import", name)
		}
	}
}

// TestSetDefault verifies that SetDefault replaces Default and that passing
// nil reverts to a no-op recorder.
func TestSetDefault(t *testing.T) {
	original := metrics.Default
	t.Cleanup(func() { metrics.SetDefault(original) })

	called := false
	metrics.SetDefault(spyRecorder{onObserve: func() { called = true }})
	metrics.Default.ObserveStartup("p", time.Now())
	if !called {
		t.Error("SetDefault: custom recorder not called")
	}

	// Passing nil must not panic and must revert to a no-op.
	metrics.SetDefault(nil)
	metrics.Default.ObserveStartup("p", time.Now()) // must not panic
}

// spyRecorder is a test-only Recorder that calls onObserve on every method.
type spyRecorder struct {
	onObserve func()
}

func (s spyRecorder) ObserveStartup(_ string, _ time.Time) { s.onObserve() }
func (s spyRecorder) ObserveInvocation(_ string, _ string, _ metrics.Result, _ time.Duration) {
	s.onObserve()
}
func (s spyRecorder) ObserveRotationPropagation(_ string, _ time.Duration) { s.onObserve() }
func (s spyRecorder) RecordTransition(_ string, _ string, _ lifecycle.State, _ lifecycle.State) {
	s.onObserve()
}
func (s spyRecorder) SetState(_ string, _ string, _ lifecycle.State) { s.onObserve() }
