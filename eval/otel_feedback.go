// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Zero Root AI

package eval

import (
	"context"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
)

// RecordFeedbackSpan creates an OpenTelemetry span for evaluation feedback.
// This captures aggregated feedback metrics at a specific point during agent execution.
//
// The span includes:
// - eval.step_index: trajectory step index when feedback was generated
// - eval.overall_score: aggregated score across all scorers
// - eval.overall_confidence: confidence in the overall score
// - eval.overall_action: recommended action (continue/adjust/reconsider/abort)
// - eval.scorer_count: number of individual scorers that contributed
// - eval.alert_count: number of threshold breach alerts
//
// Returns the context with the new span. The span must be ended by the caller.
// If tracer is nil, returns the original context without creating a span.
func RecordFeedbackSpan(ctx context.Context, tracer trace.Tracer, feedback *Feedback) context.Context {
	// Graceful nil handling - don't panic if tracer is nil
	if tracer == nil {
		return ctx
	}

	// Create span for feedback event
	ctx, span := tracer.Start(ctx, "eval.feedback")
	defer span.End()

	// Add feedback attributes
	span.SetAttributes(
		attribute.Int("eval.step_index", feedback.StepIndex),
		attribute.Float64("eval.overall_score", feedback.Overall.Score),
		attribute.Float64("eval.overall_confidence", feedback.Overall.Confidence),
		attribute.String("eval.overall_action", string(feedback.Overall.Action)),
		attribute.Int("eval.scorer_count", len(feedback.Scores)),
		attribute.Int("eval.alert_count", len(feedback.Alerts)),
	)

	// Add individual scorer scores as attributes
	for scorerName, score := range feedback.Scores {
		span.SetAttributes(
			attribute.Float64("eval.scorer."+scorerName+".score", score.Score),
			attribute.Float64("eval.scorer."+scorerName+".confidence", score.Confidence),
			attribute.String("eval.scorer."+scorerName+".action", string(score.Action)),
		)
	}

	// Set span status to OK (feedback is informational, not an error)
	span.SetStatus(codes.Ok, "Feedback generated")

	return ctx
}

// RecordAlertSpan creates an OpenTelemetry span for an evaluation alert.
// Alerts represent threshold breaches that require attention.
//
// The span includes:
// - eval.alert.level: warning or critical
// - eval.alert.scorer: name of the scorer that triggered the alert
// - eval.alert.score: the score that breached the threshold
// - eval.alert.threshold: the threshold value that was breached
// - eval.alert.action: recommended action in response to the alert
//
// Span status is set to Error for critical alerts, OK for warnings.
// Returns the context with the new span. The span must be ended by the caller.
// If tracer is nil, returns the original context without creating a span.
func RecordAlertSpan(ctx context.Context, tracer trace.Tracer, alert Alert) context.Context {
	// Graceful nil handling - don't panic if tracer is nil
	if tracer == nil {
		return ctx
	}

	// Create span for alert event
	ctx, span := tracer.Start(ctx, "eval.alert")
	defer span.End()

	// Add alert attributes
	span.SetAttributes(
		attribute.String("eval.alert.level", string(alert.Level)),
		attribute.String("eval.alert.scorer", alert.Scorer),
		attribute.Float64("eval.alert.score", alert.Score),
		attribute.Float64("eval.alert.threshold", alert.Threshold),
		attribute.String("eval.alert.action", string(alert.Action)),
	)

	// Set span status based on alert level
	switch alert.Level {
	case AlertCritical:
		span.SetStatus(codes.Error, alert.Message)
	case AlertWarning:
		span.SetStatus(codes.Ok, alert.Message)
	default:
		// Unknown alert level - treat as OK but note it
		span.SetStatus(codes.Ok, alert.Message)
	}

	return ctx
}
