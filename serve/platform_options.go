// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Zero Root AI

package serve

import (
	"log/slog"

	"github.com/zeroroot-ai/sdk/types"
	"go.opentelemetry.io/otel/trace"
)

// PlatformHarnessOption is a functional option for configuring PlatformHarness.
// It follows the same pattern as CallbackHarnessOption.
type PlatformHarnessOption func(*PlatformHarness)

// WithPlatformLogger sets the structured logger for the platform harness.
// Defaults to slog.Default() when not supplied.
func WithPlatformLogger(l *slog.Logger) PlatformHarnessOption {
	return func(h *PlatformHarness) {
		h.logger = l
	}
}

// WithPlatformTracer sets the OpenTelemetry tracer for distributed tracing.
// Defaults to a no-op tracer when not supplied.
func WithPlatformTracer(t trace.Tracer) PlatformHarnessOption {
	return func(h *PlatformHarness) {
		h.tracer = t
	}
}

// WithPlatformWorkID sets the work item identifier that routes platform
// billing and callbacks for this harness instance.
func WithPlatformWorkID(id string) PlatformHarnessOption {
	return func(h *PlatformHarness) {
		h.workID = id
	}
}

// WithPlatformMission sets the mission context for the platform harness.
func WithPlatformMission(m types.MissionContext) PlatformHarnessOption {
	return func(h *PlatformHarness) {
		h.mission = m
	}
}

// WithPlatformTarget sets the target information for the platform harness.
func WithPlatformTarget(t types.TargetInfo) PlatformHarnessOption {
	return func(h *PlatformHarness) {
		h.target = t
	}
}
