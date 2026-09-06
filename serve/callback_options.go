// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Zero Root AI

package serve

import (
	"log/slog"

	"github.com/zeroroot-ai/sdk/types"
	"go.opentelemetry.io/otel/trace"
	"go.opentelemetry.io/otel/trace/noop"
)

// CallbackHarnessOption is a functional option for configuring CallbackHarness.
// It follows the same pattern as EventBusOption in the daemon package.
type CallbackHarnessOption func(*CallbackHarness)

// WithCallbackLogger sets the structured logger for the harness.
// Defaults to slog.Default() when not supplied.
func WithCallbackLogger(l *slog.Logger) CallbackHarnessOption {
	return func(h *CallbackHarness) {
		h.logger = l
	}
}

// WithCallbackTracer sets the OpenTelemetry tracer for distributed tracing.
// Defaults to a no-op tracer when not supplied.
func WithCallbackTracer(t trace.Tracer) CallbackHarnessOption {
	return func(h *CallbackHarness) {
		h.tracer = t
	}
}

// WithCallbackMission sets the mission context for the harness.
func WithCallbackMission(m types.MissionContext) CallbackHarnessOption {
	return func(h *CallbackHarness) {
		h.mission = m
	}
}

// WithCallbackTarget sets the target information for the harness.
func WithCallbackTarget(t types.TargetInfo) CallbackHarnessOption {
	return func(h *CallbackHarness) {
		h.target = t
	}
}

// defaultNoopTracer returns a no-op OpenTelemetry tracer suitable as the
// default when no tracer option is supplied.
func defaultNoopTracer() trace.Tracer {
	return noop.NewTracerProvider().Tracer("gibson/sdk/serve")
}
