// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Zero Root AI

package eval

import (
	"context"
	"fmt"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/metric"
	"go.opentelemetry.io/otel/trace"
)

// otelMetrics holds the OpenTelemetry metric instruments for the evaluation runner.
// These are created once during WithOTel configuration and reused for all evaluations.
type otelMetrics struct {
	// scoreHistogram records individual evaluation scores (0.0 to 1.0)
	scoreHistogram metric.Float64Histogram

	// durationHistogram records evaluation duration in milliseconds
	durationHistogram metric.Float64Histogram

	// countCounter increments for each evaluation performed
	countCounter metric.Int64Counter
}

// initOTelMetrics creates and initializes all OpenTelemetry metric instruments.
// This is called once when WithOTel is invoked with a valid MeterProvider.
func (e *E) initOTelMetrics() (*otelMetrics, error) {
	if e.otelMeter == nil {
		return nil, nil
	}

	metrics := &otelMetrics{}
	var err error

	// Create score histogram
	metrics.scoreHistogram, err = e.otelMeter.Float64Histogram(
		"eval.score",
		metric.WithDescription("Evaluation score from 0.0 (worst) to 1.0 (best)"),
		metric.WithUnit("1"), // dimensionless ratio
	)
	if err != nil {
		return nil, fmt.Errorf("create score histogram: %w", err)
	}

	// Create duration histogram
	metrics.durationHistogram, err = e.otelMeter.Float64Histogram(
		"eval.duration",
		metric.WithDescription("Evaluation duration in milliseconds"),
		metric.WithUnit("ms"),
	)
	if err != nil {
		return nil, fmt.Errorf("create duration histogram: %w", err)
	}

	// Create count counter
	metrics.countCounter, err = e.otelMeter.Int64Counter(
		"eval.count",
		metric.WithDescription("Number of evaluations performed"),
		metric.WithUnit("1"),
	)
	if err != nil {
		return nil, fmt.Errorf("create count counter: %w", err)
	}

	return metrics, nil
}

// recordOTelScore creates an OpenTelemetry span and records metrics for an evaluation.
// This method is called after scoring is complete to capture observability data.
//
// The span includes:
// - sample.id as an attribute
// - scorer names as attributes
// - overall score as an attribute
// - duration in milliseconds
// - status: OK if score >= threshold, Error if below
//
// Metrics recorded:
// - eval.score: histogram of evaluation scores
// - eval.duration: histogram of evaluation duration
// - eval.count: counter incremented per evaluation
//
// If OTel is not configured (nil tracer/meter), this method returns silently without error.
// If OTel operations fail, errors are logged but not returned to avoid breaking evaluation flow.
func (e *E) recordOTelScore(ctx context.Context, sample Sample, result Result, threshold float64) {
	// Graceful handling: skip if OTel not configured
	if e.otelTracer == nil && e.otelMeter == nil {
		return
	}

	// Create span if tracer configured
	var span trace.Span
	if e.otelTracer != nil {
		ctx, span = e.otelTracer.Start(ctx, "eval.score")
		defer span.End()

		// Add span attributes
		span.SetAttributes(
			attribute.String("sample.id", result.SampleID),
			attribute.Float64("eval.overall_score", result.OverallScore),
			attribute.Float64("eval.duration_ms", float64(result.Duration.Milliseconds())),
			attribute.Int("eval.scorer_count", len(result.Scores)),
		)

		// Add individual scorer scores as attributes
		for scorerName, scoreResult := range result.Scores {
			span.SetAttributes(
				attribute.Float64(fmt.Sprintf("eval.scorer.%s.score", scorerName), scoreResult.Score),
			)
		}

		// Set span status based on threshold
		if result.OverallScore >= threshold {
			span.SetStatus(codes.Ok, fmt.Sprintf("Score %.3f meets threshold %.3f", result.OverallScore, threshold))
		} else {
			span.SetStatus(codes.Error, fmt.Sprintf("Score %.3f below threshold %.3f", result.OverallScore, threshold))
		}

		// Add error if present
		if result.Error != "" {
			span.SetAttributes(attribute.String("error", result.Error))
			span.RecordError(fmt.Errorf("%s", result.Error))
		}
	}

	// Record metrics if meter configured
	if e.otelMeter != nil {
		// Create metric options with common attributes
		opts := metric.WithAttributes(
			attribute.String("sample.id", result.SampleID),
		)

		// Record score histogram
		if e.otelMetrics != nil && e.otelMetrics.scoreHistogram != nil {
			e.otelMetrics.scoreHistogram.Record(ctx, result.OverallScore, opts)

			// Also record individual scorer scores
			for scorerName, scoreResult := range result.Scores {
				scorerOpts := metric.WithAttributes(
					attribute.String("sample.id", result.SampleID),
					attribute.String("scorer", scorerName),
				)
				e.otelMetrics.scoreHistogram.Record(ctx, scoreResult.Score, scorerOpts)
			}
		}

		// Record duration histogram
		if e.otelMetrics != nil && e.otelMetrics.durationHistogram != nil {
			durationMs := float64(result.Duration.Milliseconds())
			e.otelMetrics.durationHistogram.Record(ctx, durationMs, opts)
		}

		// Increment count counter
		if e.otelMetrics != nil && e.otelMetrics.countCounter != nil {
			e.otelMetrics.countCounter.Add(ctx, 1, opts)
		}
	}
}
