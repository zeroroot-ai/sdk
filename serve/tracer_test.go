// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Zero Root AI

package serve

import (
	"context"
	"encoding/hex"
	"log/slog"
	"os"
	"testing"

	"go.opentelemetry.io/otel/trace"
)

func TestNewProxyTracerProvider(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	// Use a real CallbackClient (not connected)
	client, err := NewCallbackClient("localhost:50051")
	if err != nil {
		t.Fatal(err)
	}

	tp := NewProxyTracerProvider(client, "0123456789abcdef0123456789abcdef", "0123456789abcdef", logger)
	if tp == nil {
		t.Fatal("NewProxyTracerProvider returned nil")
	}

	// Verify we can create a tracer from the provider
	tracer := tp.Tracer("test")
	if tracer == nil {
		t.Fatal("TracerProvider.Tracer returned nil")
	}

	// Verify we can start a span
	ctx, span := tracer.Start(context.Background(), "test-span")
	if span == nil {
		t.Fatal("Tracer.Start returned nil span")
	}
	span.End()

	// Verify span context is in the returned context
	spanCtx := trace.SpanContextFromContext(ctx)
	if !spanCtx.IsValid() {
		t.Error("Expected valid span context after starting span")
	}
}

func TestNewProxyTracer(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	client, err := NewCallbackClient("localhost:50051")
	if err != nil {
		t.Fatal(err)
	}

	tp := NewProxyTracerProvider(client, "", "", logger)
	tracer := NewProxyTracer(tp, "0123456789abcdef0123456789abcdef", "0123456789abcdef")

	if tracer == nil {
		t.Fatal("NewProxyTracer returned nil")
	}

	// Verify we can start a span
	ctx, span := tracer.Start(context.Background(), "test-span")
	if span == nil {
		t.Fatal("Tracer.Start returned nil span")
	}
	defer span.End()

	// Verify span context is valid
	spanCtx := trace.SpanContextFromContext(ctx)
	if !spanCtx.IsValid() {
		t.Error("Expected valid span context after starting span")
	}
}

func TestCreateParentContext(t *testing.T) {
	tests := []struct {
		name         string
		traceID      string
		parentSpanID string
		expectValid  bool
	}{
		{
			name:         "valid trace and span IDs",
			traceID:      "0123456789abcdef0123456789abcdef",
			parentSpanID: "0123456789abcdef",
			expectValid:  true,
		},
		{
			name:         "empty trace ID",
			traceID:      "",
			parentSpanID: "0123456789abcdef",
			expectValid:  false,
		},
		{
			name:         "empty span ID",
			traceID:      "0123456789abcdef0123456789abcdef",
			parentSpanID: "",
			expectValid:  false,
		},
		{
			name:         "invalid trace ID (too short)",
			traceID:      "0123456789abcdef",
			parentSpanID: "0123456789abcdef",
			expectValid:  false,
		},
		{
			name:         "invalid span ID (too short)",
			traceID:      "0123456789abcdef0123456789abcdef",
			parentSpanID: "01234567",
			expectValid:  false,
		},
		{
			name:         "invalid hex in trace ID",
			traceID:      "0123456789abcdefxyz3456789abcdef",
			parentSpanID: "0123456789abcdef",
			expectValid:  false,
		},
		{
			name:         "invalid hex in span ID",
			traceID:      "0123456789abcdef0123456789abcdef",
			parentSpanID: "0123456789abcdxz",
			expectValid:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := CreateParentContext(context.Background(), tt.traceID, tt.parentSpanID)
			spanCtx := trace.SpanContextFromContext(ctx)

			if tt.expectValid {
				if !spanCtx.IsValid() {
					t.Error("Expected valid span context, got invalid")
				}

				// Verify the trace ID matches what we provided
				expectedTraceID, _ := hex.DecodeString(tt.traceID)
				if spanCtx.TraceID().String() != hex.EncodeToString(expectedTraceID) {
					t.Errorf("Expected trace ID %s, got %s", tt.traceID, spanCtx.TraceID().String())
				}

				// Verify the span ID matches what we provided
				expectedSpanID, _ := hex.DecodeString(tt.parentSpanID)
				if spanCtx.SpanID().String() != hex.EncodeToString(expectedSpanID) {
					t.Errorf("Expected span ID %s, got %s", tt.parentSpanID, spanCtx.SpanID().String())
				}

				// Verify flags
				if !spanCtx.IsSampled() {
					t.Error("Expected span to be sampled")
				}

				if !spanCtx.IsRemote() {
					t.Error("Expected span to be marked as remote")
				}
			} else {
				if spanCtx.IsValid() {
					t.Error("Expected invalid span context, got valid")
				}
			}
		})
	}
}

func TestCreateParentContextIntegration(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	client, err := NewCallbackClient("localhost:50051")
	if err != nil {
		t.Fatal(err)
	}

	// Create a tracer provider and tracer
	tp := NewProxyTracerProvider(client, "", "", logger)
	tracer := tp.Tracer("test")

	// Create parent context with specific IDs
	traceID := "0123456789abcdef0123456789abcdef"
	parentSpanID := "fedcba9876543210"
	parentCtx := CreateParentContext(context.Background(), traceID, parentSpanID)

	// Start a span with the parent context
	ctx, span := tracer.Start(parentCtx, "child-span")
	defer span.End()

	// Verify the span has the correct trace ID
	spanCtx := trace.SpanContextFromContext(ctx)
	if !spanCtx.IsValid() {
		t.Fatal("Expected valid span context")
	}

	// The child span should have the same trace ID as the parent
	expectedTraceID, _ := hex.DecodeString(traceID)
	if spanCtx.TraceID().String() != hex.EncodeToString(expectedTraceID) {
		t.Errorf("Expected child span to have trace ID %s, got %s",
			traceID, spanCtx.TraceID().String())
	}

	// The child span should have a different span ID than the parent
	expectedParentSpanID, _ := hex.DecodeString(parentSpanID)
	if spanCtx.SpanID().String() == hex.EncodeToString(expectedParentSpanID) {
		t.Error("Expected child span to have different span ID than parent")
	}
}
