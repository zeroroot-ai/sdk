// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Zero Root AI

package serve_test

import (
	"context"
	"log/slog"
	"os"

	"github.com/zeroroot-ai/sdk/serve"
	"go.opentelemetry.io/otel/trace"
)

// ExampleNewProxyTracerProvider demonstrates creating a TracerProvider
// that exports spans to the orchestrator via the callback client.
func ExampleNewProxyTracerProvider() {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))

	// Create a callback client (normally configured with orchestrator endpoint)
	client, _ := serve.NewCallbackClient("localhost:50051")

	// Create a tracer provider with parent trace context
	traceID := "0123456789abcdef0123456789abcdef"
	parentSpanID := "fedcba9876543210"
	tp := serve.NewProxyTracerProvider(client, traceID, parentSpanID, logger)
	defer tp.Shutdown(context.Background())

	// Create a tracer
	tracer := tp.Tracer("my-service")

	// Use CreateParentContext to inject parent span context
	ctx := serve.CreateParentContext(context.Background(), traceID, parentSpanID)

	// Start a span that will be linked to the orchestrator's parent span
	ctx, span := tracer.Start(ctx, "my-operation")
	defer span.End()

	// The span will be automatically exported to the orchestrator when it ends
	_ = ctx
}

// ExampleCreateParentContext demonstrates creating a context with parent
// span information for distributed tracing.
func ExampleCreateParentContext() {
	// Parent trace context from orchestrator
	traceID := "0123456789abcdef0123456789abcdef"
	parentSpanID := "fedcba9876543210"

	// Create context with parent span
	ctx := serve.CreateParentContext(context.Background(), traceID, parentSpanID)

	// Verify parent span context is injected
	spanCtx := trace.SpanContextFromContext(ctx)
	if spanCtx.IsValid() {
		// The context now contains the parent span information
		// Any spans started with this context will be children of the parent
		_ = spanCtx.TraceID()
		_ = spanCtx.SpanID()
	}

	_ = ctx
}
