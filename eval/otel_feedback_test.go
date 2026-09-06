// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Zero Root AI

package eval

import (
	"context"
	"testing"
	"time"

	"go.opentelemetry.io/otel/trace/noop"
)

func TestRecordFeedbackSpan_NilTracer(t *testing.T) {
	ctx := context.Background()
	feedback := &Feedback{
		Timestamp: time.Now(),
		StepIndex: 5,
		Scores: map[string]PartialScore{
			"scorer1": {Score: 0.8, Confidence: 0.9, Action: ActionContinue},
		},
		Overall: PartialScore{
			Score:      0.75,
			Confidence: 0.85,
			Action:     ActionContinue,
		},
		Alerts: []Alert{},
	}

	// Should not panic with nil tracer
	resultCtx := RecordFeedbackSpan(ctx, nil, feedback)
	if resultCtx != ctx {
		t.Error("Expected same context when tracer is nil")
	}
}

func TestRecordFeedbackSpan_WithTracer(t *testing.T) {
	ctx := context.Background()
	tracer := noop.NewTracerProvider().Tracer("test")

	feedback := &Feedback{
		Timestamp: time.Now(),
		StepIndex: 5,
		Scores: map[string]PartialScore{
			"scorer1": {Score: 0.8, Confidence: 0.9, Action: ActionContinue},
			"scorer2": {Score: 0.7, Confidence: 0.8, Action: ActionAdjust},
		},
		Overall: PartialScore{
			Score:      0.75,
			Confidence: 0.85,
			Action:     ActionContinue,
		},
		Alerts: []Alert{
			{Level: AlertWarning, Scorer: "scorer2", Score: 0.7, Threshold: 0.8},
		},
	}

	// Should not panic and return context
	resultCtx := RecordFeedbackSpan(ctx, tracer, feedback)
	if resultCtx == nil {
		t.Error("Expected non-nil context")
	}
}

func TestRecordAlertSpan_NilTracer(t *testing.T) {
	ctx := context.Background()
	alert := Alert{
		Level:     AlertWarning,
		Scorer:    "test_scorer",
		Score:     0.65,
		Threshold: 0.8,
		Message:   "Performance below threshold",
		Action:    ActionAdjust,
	}

	// Should not panic with nil tracer
	resultCtx := RecordAlertSpan(ctx, nil, alert)
	if resultCtx != ctx {
		t.Error("Expected same context when tracer is nil")
	}
}

func TestRecordAlertSpan_Warning(t *testing.T) {
	ctx := context.Background()
	tracer := noop.NewTracerProvider().Tracer("test")

	alert := Alert{
		Level:     AlertWarning,
		Scorer:    "test_scorer",
		Score:     0.65,
		Threshold: 0.8,
		Message:   "Performance below threshold",
		Action:    ActionAdjust,
	}

	// Should not panic and return context
	resultCtx := RecordAlertSpan(ctx, tracer, alert)
	if resultCtx == nil {
		t.Error("Expected non-nil context")
	}
}

func TestRecordAlertSpan_Critical(t *testing.T) {
	ctx := context.Background()
	tracer := noop.NewTracerProvider().Tracer("test")

	alert := Alert{
		Level:     AlertCritical,
		Scorer:    "test_scorer",
		Score:     0.3,
		Threshold: 0.5,
		Message:   "Critical performance issue",
		Action:    ActionAbort,
	}

	// Should not panic and return context
	resultCtx := RecordAlertSpan(ctx, tracer, alert)
	if resultCtx == nil {
		t.Error("Expected non-nil context")
	}
}

func TestRecordAlertSpan_UnknownLevel(t *testing.T) {
	ctx := context.Background()
	tracer := noop.NewTracerProvider().Tracer("test")

	alert := Alert{
		Level:     AlertLevel("unknown"),
		Scorer:    "test_scorer",
		Score:     0.5,
		Threshold: 0.8,
		Message:   "Unknown alert level",
		Action:    ActionContinue,
	}

	// Should not panic with unknown alert level
	resultCtx := RecordAlertSpan(ctx, tracer, alert)
	if resultCtx == nil {
		t.Error("Expected non-nil context")
	}
}

// Benchmark for RecordFeedbackSpan with nil tracer (should be very fast)
func BenchmarkRecordFeedbackSpan_NilTracer(b *testing.B) {
	ctx := context.Background()
	feedback := &Feedback{
		Timestamp: time.Now(),
		StepIndex: 5,
		Scores:    map[string]PartialScore{},
		Overall:   PartialScore{Score: 0.75, Confidence: 0.85, Action: ActionContinue},
		Alerts:    []Alert{},
	}

	b.ResetTimer()
	for range b.N {
		RecordFeedbackSpan(ctx, nil, feedback)
	}
}

// Benchmark for RecordFeedbackSpan with noop tracer
func BenchmarkRecordFeedbackSpan_NoopTracer(b *testing.B) {
	ctx := context.Background()
	tracer := noop.NewTracerProvider().Tracer("test")
	feedback := &Feedback{
		Timestamp: time.Now(),
		StepIndex: 5,
		Scores: map[string]PartialScore{
			"scorer1": {Score: 0.8, Confidence: 0.9, Action: ActionContinue},
		},
		Overall: PartialScore{Score: 0.75, Confidence: 0.85, Action: ActionContinue},
		Alerts:  []Alert{},
	}

	b.ResetTimer()
	for range b.N {
		RecordFeedbackSpan(ctx, tracer, feedback)
	}
}

// Benchmark for RecordAlertSpan with nil tracer (should be very fast)
func BenchmarkRecordAlertSpan_NilTracer(b *testing.B) {
	ctx := context.Background()
	alert := Alert{
		Level:     AlertWarning,
		Scorer:    "test_scorer",
		Score:     0.65,
		Threshold: 0.8,
		Message:   "Performance below threshold",
		Action:    ActionAdjust,
	}

	b.ResetTimer()
	for range b.N {
		RecordAlertSpan(ctx, nil, alert)
	}
}

// Benchmark for RecordAlertSpan with noop tracer
func BenchmarkRecordAlertSpan_NoopTracer(b *testing.B) {
	ctx := context.Background()
	tracer := noop.NewTracerProvider().Tracer("test")
	alert := Alert{
		Level:     AlertWarning,
		Scorer:    "test_scorer",
		Score:     0.65,
		Threshold: 0.8,
		Message:   "Performance below threshold",
		Action:    ActionAdjust,
	}

	b.ResetTimer()
	for range b.N {
		RecordAlertSpan(ctx, tracer, alert)
	}
}
