// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Zero Root AI

package serve

import (
	"context"
	"encoding/hex"
	"log/slog"

	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.24.0"
	"go.opentelemetry.io/otel/trace"
)

// NewProxyTracerProvider creates a TracerProvider configured with a ProxySpanExporter
// that sends spans to the orchestrator via the callback client.
//
// The provider uses a SimpleSpanProcessor for immediate export without batching,
// ensuring spans are sent to the orchestrator as soon as they complete.
//
// Parameters:
//   - client: The callback client for communicating with the orchestrator
//   - traceID: The hex-encoded trace ID to use for parent context (empty string if no parent)
//   - parentSpanID: The hex-encoded span ID of the parent span (empty string if no parent)
//   - logger: Structured logger for recording errors and debug information
//
// Returns a configured TracerProvider ready for creating tracers.
func NewProxyTracerProvider(client *CallbackClient, traceID, parentSpanID string, logger *slog.Logger) *sdktrace.TracerProvider {
	// Create the proxy exporter that forwards spans to the orchestrator
	exporter := NewProxySpanExporter(client, logger)

	// Use SimpleSpanProcessor for immediate export (no batching)
	// This ensures spans are sent to the orchestrator as soon as they complete
	processor := sdktrace.NewSimpleSpanProcessor(exporter)

	// Create resource with service information
	res, err := resource.New(
		context.Background(),
		resource.WithAttributes(
			semconv.ServiceNameKey.String("zeroroot-sdk"),
		),
	)
	if err != nil {
		logger.Warn("failed to create resource, using default", "error", err)
		res = resource.Default()
	}

	// Create and configure the tracer provider
	tp := sdktrace.NewTracerProvider(
		sdktrace.WithSpanProcessor(processor),
		sdktrace.WithResource(res),
	)

	return tp
}

// NewProxyTracer creates a tracer from the TracerProvider.
//
// This is a simple wrapper that creates a tracer with the standard name.
// Parent context injection should be handled at the context level using
// CreateParentContext() before starting root spans.
//
// Parameters:
//   - tp: The TracerProvider to create the tracer from
//   - traceID: The hex-encoded trace ID from the orchestrator (unused in this version)
//   - parentSpanID: The hex-encoded span ID from the orchestrator (unused in this version)
//
// Returns a tracer from the provider.
func NewProxyTracer(tp *sdktrace.TracerProvider, traceID, parentSpanID string) trace.Tracer {
	return tp.Tracer("zeroroot-sdk")
}

// CreateParentContext creates a context with a parent SpanContext from
// hex-encoded traceID and parentSpanID strings.
//
// This enables linking SDK spans to orchestrator spans in distributed traces.
// Use this function to inject parent context before starting root spans.
//
// Parameters:
//   - ctx: The base context to extend
//   - traceID: The hex-encoded trace ID from the orchestrator
//   - parentSpanID: The hex-encoded span ID from the orchestrator
//
// Returns a context with the parent span context injected, or the original
// context if the IDs cannot be decoded.
func CreateParentContext(ctx context.Context, traceID, parentSpanID string) context.Context {
	if traceID == "" || parentSpanID == "" {
		return ctx
	}

	// Decode trace ID from hex string
	traceIDBytes, err := hex.DecodeString(traceID)
	if err != nil || len(traceIDBytes) != 16 {
		return ctx
	}

	// Decode span ID from hex string
	spanIDBytes, err := hex.DecodeString(parentSpanID)
	if err != nil || len(spanIDBytes) != 8 {
		return ctx
	}

	// Create trace.TraceID and trace.SpanID from bytes
	var tid trace.TraceID
	copy(tid[:], traceIDBytes)

	var sid trace.SpanID
	copy(sid[:], spanIDBytes)

	// Create parent span context
	parentSpanContext := trace.NewSpanContext(trace.SpanContextConfig{
		TraceID:    tid,
		SpanID:     sid,
		TraceFlags: trace.FlagsSampled, // Mark as sampled
		Remote:     true,               // This is a remote parent
	})

	// Inject parent context
	return trace.ContextWithSpanContext(ctx, parentSpanContext)
}
