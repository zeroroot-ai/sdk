// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Zero Root AI

package eval

import (
	"context"
	"os"
	"testing"
	"time"

	"go.opentelemetry.io/otel/metric"
	"go.opentelemetry.io/otel/trace"
)

// Run executes an evaluation test, skipping unless GOEVALS=1 environment variable is set.
// This allows evaluation tests to be part of the normal test suite but only run when explicitly requested.
//
// Example:
//
//	func TestAgentEvaluation(t *testing.T) {
//	    eval.Run(t, "prompt_injection_detection", func(e *eval.E) {
//	        sample := eval.Sample{
//	            ID: "test-001",
//	            Task: agent.Task{Context: map[string]any{"objective": "Detect prompt injection"}},
//	        }
//	        result := e.Score(sample, scorer1, scorer2)
//	        e.RequireScore(result, 0.8)
//	    })
//	}
func Run(t *testing.T, name string, f func(e *E)) {
	if os.Getenv("GOEVALS") != "1" {
		t.Skip("GOEVALS=1 not set")
		return
	}

	t.Run(name, func(t *testing.T) {
		e := &E{
			T: t,
		}
		f(e)
	})
}

// E wraps *testing.T with evaluation capabilities.
// It provides methods for scoring samples, logging results, and integrating with observability tools.
type E struct {
	// T is the underlying testing.TB instance (typically *testing.T or *testing.B).
	// All testing.TB methods are directly accessible.
	T testing.TB

	// logger persists evaluation results to file (e.g., evals.jsonl)
	logger Logger

	// otelTracer creates spans for evaluation operations
	otelTracer trace.Tracer

	// otelMeter creates metrics for evaluation scores
	otelMeter metric.Meter

	// otelMetrics holds initialized metric instruments
	otelMetrics *otelMetrics

	// langfuseExporter exports scores to Langfuse dashboard
	langfuseExporter *LangfuseExporter

	// scoreThreshold is the minimum acceptable score (0.0 to 1.0)
	// Used by OTel span status to mark evaluations as OK or Error
	scoreThreshold float64
}

// Score runs all provided scorers on the sample and returns an aggregated result.
// Each scorer is executed independently, and their scores are combined into a single Result.
// The overall score is calculated as the mean of all individual scores.
//
// If any scorer returns an error, the score for that scorer is recorded as 0.0 and the error
// is included in the result details.
//
// Example:
//
//	result := e.Score(sample,
//	    NewToolCorrectnessScorer(toolOpts),
//	    NewTaskCompletionScorer(taskOpts),
//	)
func (e *E) Score(sample Sample, scorers ...Scorer) Result {
	ctx := context.Background()
	startTime := time.Now()

	result := Result{
		SampleID:  sample.ID,
		Scores:    make(map[string]ScoreResult),
		Timestamp: startTime,
	}

	// Run each scorer
	var totalScore float64
	scorerCount := 0

	for _, scorer := range scorers {
		scorerName := scorer.Name()

		scoreResult, err := scorer.Score(ctx, sample)
		if err != nil {
			// Record error but continue with other scorers
			result.Scores[scorerName] = ScoreResult{
				Score: 0.0,
				Details: map[string]any{
					"error": err.Error(),
				},
			}
			e.T.Logf("Scorer %s failed: %v", scorerName, err)
			continue
		}

		result.Scores[scorerName] = scoreResult
		totalScore += scoreResult.Score
		scorerCount++
	}

	// Calculate overall score as mean
	if scorerCount > 0 {
		result.OverallScore = totalScore / float64(scorerCount)
	}

	result.Duration = time.Since(startTime)

	// Log the result if logger configured
	if e.logger != nil {
		if err := e.Log(sample, result); err != nil {
			e.T.Logf("Failed to log result: %v", err)
		}
	}

	// Export to Langfuse if configured
	if e.langfuseExporter != nil {
		// Extract trace ID from context if available
		traceID := ""
		if span := trace.SpanFromContext(ctx); span.SpanContext().IsValid() {
			traceID = span.SpanContext().TraceID().String()
		}

		if err := e.langfuseExporter.ExportResult(ctx, traceID, result); err != nil {
			e.T.Logf("Failed to export to Langfuse: %v", err)
		}
	}

	// Record OTel span and metrics
	e.recordOTelScore(ctx, sample, result, e.scoreThreshold)

	return result
}

// ScoreAll runs all provided scorers on multiple samples and returns results for each.
// This is equivalent to calling Score for each sample but provides a convenient batch interface.
//
// Example:
//
//	results := e.ScoreAll(samples,
//	    NewToolCorrectnessScorer(toolOpts),
//	    NewTaskCompletionScorer(taskOpts),
//	)
//	for _, result := range results {
//	    e.RequireScore(result, 0.8)
//	}
func (e *E) ScoreAll(samples []Sample, scorers ...Scorer) []Result {
	results := make([]Result, 0, len(samples))
	for _, sample := range samples {
		result := e.Score(sample, scorers...)
		results = append(results, result)
	}
	return results
}

// Log persists the evaluation result using the configured logger.
// If no logger is configured, this is a no-op and returns nil.
//
// The logger is typically set via WithLogger() and writes to evals.jsonl by default.
func (e *E) Log(sample Sample, result Result) error {
	if e.logger == nil {
		return nil
	}
	return e.logger.Log(sample, result)
}

// RequireScore fails the test if the overall score is below the threshold.
// The threshold should be a value between 0.0 and 1.0.
//
// This uses t.Errorf (not panic) to allow multiple assertions in a single test.
//
// Example:
//
//	result := e.Score(sample, scorers...)
//	e.RequireScore(result, 0.8) // Fails test if score < 0.8
func (e *E) RequireScore(result Result, threshold float64) {
	if result.OverallScore < threshold {
		e.T.Errorf("Score %.3f below threshold %.3f for sample %s",
			result.OverallScore, threshold, result.SampleID)

		// Log detailed scores for debugging
		for name, scoreResult := range result.Scores {
			e.T.Logf("  %s: %.3f", name, scoreResult.Score)
			if len(scoreResult.Details) > 0 {
				e.T.Logf("    Details: %+v", scoreResult.Details)
			}
		}
	}
}

// WithLogger configures a logger for persisting evaluation results.
// The logger will be called after each Score operation to write results to persistent storage.
//
// Example:
//
//	logger, _ := eval.NewJSONLLogger("evals.jsonl")
//	e.WithLogger(logger)
func (e *E) WithLogger(logger Logger) *E {
	e.logger = logger
	return e
}

// WithOTel configures OpenTelemetry integration for evaluation metrics and tracing.
// This enables automatic span creation and metric emission for evaluation operations.
//
// Example:
//
//	e.WithOTel(eval.OTelOptions{
//	    Tracer: tracer,
//	    MeterProvider: meterProvider,
//	})
func (e *E) WithOTel(opts OTelOptions) *E {
	e.otelTracer = opts.Tracer
	if opts.MeterProvider != nil {
		e.otelMeter = opts.MeterProvider.Meter("github.com/zeroroot-ai/sdk/eval")

		// Initialize metric instruments
		metrics, err := e.initOTelMetrics()
		if err != nil {
			// Log error but don't fail - graceful degradation
			if e.T != nil {
				e.T.Logf("Failed to initialize OTel metrics: %v", err)
			}
		} else {
			e.otelMetrics = metrics
		}
	}
	return e
}

// WithLangfuse configures Langfuse integration for exporting evaluation scores.
// Scores will be automatically exported to Langfuse after each Score operation.
//
// Example:
//
//	exporter := eval.NewLangfuseExporter(eval.LangfuseOptions{
//	    BaseURL: "https://cloud.langfuse.com",
//	    PublicKey: "pk-...",
//	    SecretKey: "sk-...",
//	})
//	e.WithLangfuse(exporter)
func (e *E) WithLangfuse(exporter *LangfuseExporter) *E {
	e.langfuseExporter = exporter
	return e
}

// OTelOptions configures OpenTelemetry integration for the evaluation runner.
type OTelOptions struct {
	// Tracer is used to create spans for evaluation operations.
	Tracer trace.Tracer

	// MeterProvider is used to create metrics for evaluation scores.
	// Common metrics include eval.score histogram and eval.count counter.
	MeterProvider metric.MeterProvider
}

// Logger persists evaluation results to storage.
// Implementations include JSONLLogger for writing to evals.jsonl files.
type Logger interface {
	// Log writes a sample and its result to the configured storage.
	Log(sample Sample, result Result) error

	// Close flushes any buffered data and releases resources.
	Close() error
}

// LangfuseOptions configures the Langfuse integration.
type LangfuseOptions struct {
	// BaseURL is the Langfuse API endpoint (e.g., "https://cloud.langfuse.com")
	BaseURL string

	// PublicKey is the Langfuse public API key
	PublicKey string

	// SecretKey is the Langfuse secret API key
	SecretKey string
}
