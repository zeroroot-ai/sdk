// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Zero Root AI

package serve

import (
	"github.com/zeroroot-ai/sdk/types"
	"go.opentelemetry.io/otel/trace"
	"go.opentelemetry.io/otel/trace/noop"
)

// CallbackHarnessOption is a functional option for configuring CallbackHarness.
// It follows the same pattern as EventBusOption in the daemon package.
type CallbackHarnessOption func(*CallbackHarness)

// WithCallbackMission sets the mission context for the harness.
func WithCallbackMission(m types.MissionContext) CallbackHarnessOption {
	return func(h *CallbackHarness) {
		h.mission = m
	}
}

// defaultNoopTracer returns a no-op OpenTelemetry tracer suitable as the
// default when no tracer option is supplied.
func defaultNoopTracer() trace.Tracer {
	return noop.NewTracerProvider().Tracer("gibson/sdk/serve")
}
