// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Zero Root AI

package codegen

import (
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/metric"
	"go.opentelemetry.io/otel/trace"
)

// Package-level tracer and meter for codegen instrumentation.
var (
	tracer trace.Tracer
	meter  metric.Meter
)

func init() {
	tracer = otel.Tracer("github.com/zeroroot-ai/sdk/codegen")
	meter = otel.Meter("github.com/zeroroot-ai/sdk/codegen")
}
