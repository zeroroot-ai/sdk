// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Zero Root AI

package prometheus_test

import (
	"strings"
	"testing"
	"time"

	promclient "github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/testutil"

	"github.com/zeroroot-ai/sdk/plugin/lifecycle"
	"github.com/zeroroot-ai/sdk/plugin/metrics"
	pluginprom "github.com/zeroroot-ai/sdk/plugin/metrics/prometheus"
)

func newRecorder(t *testing.T) (*pluginprom.Recorder, *promclient.Registry) {
	t.Helper()
	reg := promclient.NewRegistry()
	return pluginprom.New(reg), reg
}

// TestObserveStartup checks that the startup histogram receives one observation.
func TestObserveStartup(t *testing.T) {
	t.Parallel()
	r, reg := newRecorder(t)
	r.ObserveStartup("scaffold-test", time.Now().Add(-150*time.Millisecond))
	if got := testutil.CollectAndCount(reg); got == 0 {
		t.Error("expected at least one collected metric")
	}
}

// TestObserveInvocation checks that duration histogram and counter are updated.
func TestObserveInvocation(t *testing.T) {
	t.Parallel()
	r, _ := newRecorder(t)

	cases := []struct {
		method string
		result metrics.Result
		dur    time.Duration
	}{
		{"ScanRepo", metrics.ResultOK, 12 * time.Millisecond},
		{"ScanRepo", metrics.ResultError, 8 * time.Millisecond},
		{"ScanRepo", metrics.ResultDeadlineExceeded, 60 * time.Second},
		{"ScanRepo", metrics.ResultPanic, 1 * time.Millisecond},
		{"DoesNotExist", metrics.ResultMethodNotFound, 1 * time.Millisecond},
	}

	for _, c := range cases {
		r.ObserveInvocation("plugin", c.method, c.result, c.dur)
	}
}

// TestObserveInvocation_Accumulates verifies counter increments are additive.
func TestObserveInvocation_Accumulates(t *testing.T) {
	t.Parallel()
	r, reg := newRecorder(t)

	for range 5 {
		r.ObserveInvocation("p", "M", metrics.ResultOK, time.Millisecond)
	}
	for range 2 {
		r.ObserveInvocation("p", "M", metrics.ResultError, time.Millisecond)
	}

	want := `
		# HELP gibson_plugin_invoke_total Total plugin method invocations, labeled by plugin, method, and result outcome.
		# TYPE gibson_plugin_invoke_total counter
		gibson_plugin_invoke_total{method="M",plugin="p",result="error"} 2
		gibson_plugin_invoke_total{method="M",plugin="p",result="ok"} 5
	`
	if err := testutil.GatherAndCompare(reg, strings.NewReader(want), "gibson_plugin_invoke_total"); err != nil {
		t.Error(err)
	}
}

// TestRecordTransition checks the counter and gauge.
func TestRecordTransition(t *testing.T) {
	t.Parallel()
	r, reg := newRecorder(t)

	r.RecordTransition("p", "i-1", lifecycle.Bootstrapping, lifecycle.Registering)
	r.RecordTransition("p", "i-1", lifecycle.Registering, lifecycle.ResolvingSecrets)
	r.RecordTransition("p", "i-1", lifecycle.ResolvingSecrets, lifecycle.Starting)
	r.RecordTransition("p", "i-1", lifecycle.Starting, lifecycle.Ready)

	mfs, err := reg.Gather()
	if err != nil {
		t.Fatalf("Gather: %v", err)
	}
	seen := map[string]bool{}
	for _, mf := range mfs {
		seen[mf.GetName()] = true
	}
	for _, want := range []string{"gibson_plugin_lifecycle_transition_total", "gibson_plugin_state"} {
		if !seen[want] {
			t.Errorf("metric %q not found after RecordTransition", want)
		}
	}
}

// TestRecordTransition_EmptyInstallID confirms an empty install_id leaves
// the gauge untouched but still bumps the counter.
func TestRecordTransition_EmptyInstallID(t *testing.T) {
	t.Parallel()
	r, reg := newRecorder(t)

	r.RecordTransition("p", "", lifecycle.Bootstrapping, lifecycle.Registering)

	mfs, _ := reg.Gather()
	for _, mf := range mfs {
		if mf.GetName() == "gibson_plugin_state" {
			t.Error("state gauge must not have series when install_id is empty")
		}
	}
}

// TestSetState updates the gauge without bumping the transition counter.
func TestSetState(t *testing.T) {
	t.Parallel()
	r, reg := newRecorder(t)

	r.SetState("p", "i-1", lifecycle.Degraded)

	mfs, _ := reg.Gather()
	for _, mf := range mfs {
		if mf.GetName() == "gibson_plugin_lifecycle_transition_total" {
			t.Error("transition counter must not be bumped by SetState")
		}
	}
	// State gauge must exist.
	found := false
	for _, mf := range mfs {
		if mf.GetName() == "gibson_plugin_state" {
			found = true
		}
	}
	if !found {
		t.Error("gibson_plugin_state gauge not found after SetState")
	}
}

// TestObserveRotationPropagation records a sample and clamps negative lag.
func TestObserveRotationPropagation(t *testing.T) {
	t.Parallel()
	r, _ := newRecorder(t)
	r.ObserveRotationPropagation("p", 200*time.Millisecond)
	r.ObserveRotationPropagation("p", -5*time.Millisecond) // clamped to zero
}

// TestAllSixMetricsPresent confirms all six metric families are registered.
func TestAllSixMetricsPresent(t *testing.T) {
	t.Parallel()
	r, reg := newRecorder(t)

	r.ObserveStartup("p", time.Now().Add(-100*time.Millisecond))
	r.ObserveInvocation("p", "M", metrics.ResultOK, time.Millisecond)
	r.ObserveRotationPropagation("p", 100*time.Millisecond)
	r.RecordTransition("p", "i-1", lifecycle.Starting, lifecycle.Ready)

	mfs, err := reg.Gather()
	if err != nil {
		t.Fatalf("Gather: %v", err)
	}
	want := []string{
		"gibson_plugin_startup_seconds",
		"gibson_plugin_invoke_duration_seconds",
		"gibson_plugin_rotation_propagation_seconds",
		"gibson_plugin_invoke_total",
		"gibson_plugin_lifecycle_transition_total",
		"gibson_plugin_state",
	}
	seen := map[string]bool{}
	for _, mf := range mfs {
		seen[mf.GetName()] = true
	}
	for _, w := range want {
		if !seen[w] {
			t.Errorf("metric %q missing", w)
		}
	}
}

// TestImplementsInterface is a compile-time assertion.
func TestImplementsInterface(t *testing.T) {
	t.Parallel()
	reg := promclient.NewRegistry()
	var _ metrics.Recorder = pluginprom.New(reg)
}
