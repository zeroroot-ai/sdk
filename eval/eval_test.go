// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Zero Root AI

package eval

import (
	"context"
	"errors"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/otel/metric/noop"
	nooptrace "go.opentelemetry.io/otel/trace/noop"

	"github.com/zeroroot-ai/sdk/agent"
)

// TestRunSkipsWithoutEnvVar tests that Run() skips the test when GOEVALS is not set.
func TestRunSkipsWithoutEnvVar(t *testing.T) {
	// Ensure GOEVALS is not set
	os.Unsetenv("GOEVALS")

	executed := false
	Run(t, "should_skip", func(e *E) {
		executed = true
	})

	// The function should not have executed
	assert.False(t, executed, "Run should skip without GOEVALS=1")
}

// TestRunExecutesWithEnvVar tests that Run() executes when GOEVALS=1.
func TestRunExecutesWithEnvVar(t *testing.T) {
	// Set GOEVALS=1
	os.Setenv("GOEVALS", "1")
	defer os.Unsetenv("GOEVALS")

	executed := false
	Run(t, "should_execute", func(e *E) {
		executed = true
		assert.NotNil(t, e)
		assert.NotNil(t, e.T)
	})

	// The function should have executed
	assert.True(t, executed, "Run should execute with GOEVALS=1")
}

// Note: mockScorer is defined in scorer_test.go

// TestEScore tests that E.Score() runs all scorers and aggregates results.
func TestEScore(t *testing.T) {
	e := &E{T: t}

	sample := Sample{
		ID: "test-sample-001",
		Task: agent.Task{
			Context: map[string]any{"objective": "Test task"},
		},
	}

	scorer1 := &mockScorer{name: "scorer1", score: 0.8}
	scorer2 := &mockScorer{name: "scorer2", score: 0.6}

	result := e.Score(sample, scorer1, scorer2)

	// Verify result structure
	assert.Equal(t, "test-sample-001", result.SampleID)
	assert.Len(t, result.Scores, 2)
	assert.Contains(t, result.Scores, "scorer1")
	assert.Contains(t, result.Scores, "scorer2")
	assert.Equal(t, 0.8, result.Scores["scorer1"].Score)
	assert.Equal(t, 0.6, result.Scores["scorer2"].Score)

	// Verify overall score is the mean
	expectedOverall := (0.8 + 0.6) / 2
	assert.Equal(t, expectedOverall, result.OverallScore)

	// Verify timestamp and duration
	assert.False(t, result.Timestamp.IsZero())
	assert.Greater(t, result.Duration, time.Duration(0))
}

// TestEScoreSingleScorer tests E.Score() with a single scorer.
func TestEScoreSingleScorer(t *testing.T) {
	e := &E{T: t}

	sample := Sample{ID: "test-sample-002"}
	scorer := &mockScorer{name: "single_scorer", score: 0.9}

	result := e.Score(sample, scorer)

	assert.Equal(t, 0.9, result.OverallScore)
	assert.Len(t, result.Scores, 1)
}

// TestEScoreWithError tests that E.Score() handles scorer errors gracefully.
func TestEScoreWithError(t *testing.T) {
	e := &E{T: t}

	sample := Sample{ID: "test-sample-003"}

	scorer1 := &mockScorer{name: "failing_scorer", err: errors.New("mock scorer error")}
	scorer2 := &mockScorer{name: "passing_scorer", score: 0.7}

	result := e.Score(sample, scorer1, scorer2)

	// Verify the failing scorer is recorded with score 0.0
	require.Contains(t, result.Scores, "failing_scorer")
	assert.Equal(t, 0.0, result.Scores["failing_scorer"].Score)
	assert.Contains(t, result.Scores["failing_scorer"].Details, "error")
	assert.Equal(t, "mock scorer error", result.Scores["failing_scorer"].Details["error"])

	// Verify the passing scorer succeeded
	require.Contains(t, result.Scores, "passing_scorer")
	assert.Equal(t, 0.7, result.Scores["passing_scorer"].Score)

	// Verify overall score only includes successful scorers
	assert.Equal(t, 0.7, result.OverallScore)
}

// TestEScoreNoScorers tests E.Score() with no scorers.
func TestEScoreNoScorers(t *testing.T) {
	e := &E{T: t}

	sample := Sample{ID: "test-sample-004"}
	result := e.Score(sample)

	// Overall score should be 0 when no scorers succeed
	assert.Equal(t, 0.0, result.OverallScore)
	assert.Empty(t, result.Scores)
}

// TestEScoreAllFailures tests E.Score() when all scorers fail.
func TestEScoreAllFailures(t *testing.T) {
	e := &E{T: t}

	sample := Sample{ID: "test-sample-005"}

	scorer1 := &mockScorer{name: "fail1", err: errors.New("error 1")}
	scorer2 := &mockScorer{name: "fail2", err: errors.New("error 2")}

	result := e.Score(sample, scorer1, scorer2)

	// Overall score should be 0 when all scorers fail
	assert.Equal(t, 0.0, result.OverallScore)
	assert.Len(t, result.Scores, 2)
}

// TestEScoreAll tests that E.ScoreAll() processes multiple samples.
func TestEScoreAll(t *testing.T) {
	e := &E{T: t}

	samples := []Sample{
		{ID: "sample-1"},
		{ID: "sample-2"},
		{ID: "sample-3"},
	}

	scorer := &mockScorer{name: "test_scorer", score: 0.85}

	results := e.ScoreAll(samples, scorer)

	// Verify we got results for all samples
	assert.Len(t, results, 3)
	assert.Equal(t, "sample-1", results[0].SampleID)
	assert.Equal(t, "sample-2", results[1].SampleID)
	assert.Equal(t, "sample-3", results[2].SampleID)

	// Verify all results have the same score
	for _, result := range results {
		assert.Equal(t, 0.85, result.OverallScore)
	}
}

// TestEScoreAllEmpty tests E.ScoreAll() with no samples.
func TestEScoreAllEmpty(t *testing.T) {
	e := &E{T: t}

	samples := []Sample{}
	scorer := &mockScorer{name: "test_scorer", score: 0.85}

	results := e.ScoreAll(samples, scorer)

	assert.Empty(t, results)
}

// TestERequireScorePass tests that E.RequireScore() passes when score is above threshold.
func TestERequireScorePass(t *testing.T) {
	e := &E{T: t}

	result := Result{
		SampleID:     "test-sample-006",
		OverallScore: 0.85,
	}

	// Should not call t.Errorf (score above threshold)
	e.RequireScore(result, 0.8)
}

// TestERequireScoreExactThreshold tests E.RequireScore() when score equals threshold.
func TestERequireScoreExactThreshold(t *testing.T) {
	e := &E{T: t}

	result := Result{
		SampleID:     "test-sample-008",
		OverallScore: 0.8,
	}

	// Should not call t.Errorf (score equals threshold)
	e.RequireScore(result, 0.8)
}

// TestEWithLogger tests that E.WithLogger() configures the logger.
func TestEWithLogger(t *testing.T) {
	e := &E{T: t}

	// Create a mock logger
	logger := &mockLogger{}

	// Configure logger
	e.WithLogger(logger)

	// Verify logger is set
	assert.Equal(t, logger, e.logger)
}

// TestEWithOTel tests that E.WithOTel() configures OpenTelemetry.
func TestEWithOTel(t *testing.T) {
	e := &E{T: t}

	tracer := nooptrace.NewTracerProvider().Tracer("test")
	meterProvider := noop.NewMeterProvider()

	e.WithOTel(OTelOptions{
		Tracer:        tracer,
		MeterProvider: meterProvider,
	})

	// Verify OTel components are set
	assert.Equal(t, tracer, e.otelTracer)
	assert.NotNil(t, e.otelMeter)
	assert.NotNil(t, e.otelMetrics)
}

// TestEWithOTelTracerOnly tests WithOTel with only a tracer (no meter provider).
func TestEWithOTelTracerOnly(t *testing.T) {
	e := &E{T: t}

	tracer := nooptrace.NewTracerProvider().Tracer("test")

	e.WithOTel(OTelOptions{
		Tracer: tracer,
	})

	// Verify tracer is set but meter is not
	assert.Equal(t, tracer, e.otelTracer)
	assert.Nil(t, e.otelMeter)
	assert.Nil(t, e.otelMetrics)
}

// TestEWithOTelMeterOnly tests WithOTel with only a meter provider (no tracer).
func TestEWithOTelMeterOnly(t *testing.T) {
	e := &E{T: t}

	meterProvider := noop.NewMeterProvider()

	e.WithOTel(OTelOptions{
		MeterProvider: meterProvider,
	})

	// Verify meter is set but tracer is not
	assert.Nil(t, e.otelTracer)
	assert.NotNil(t, e.otelMeter)
	assert.NotNil(t, e.otelMetrics)
}

// TestEWithLangfuse tests that E.WithLangfuse() configures the Langfuse exporter.
func TestEWithLangfuse(t *testing.T) {
	e := &E{T: t}

	// Create a mock Langfuse exporter
	exporter := &LangfuseExporter{}

	e.WithLangfuse(exporter)

	// Verify exporter is set
	assert.Equal(t, exporter, e.langfuseExporter)
}

// TestEScoreWithLogger tests that E.Score() calls the logger.
func TestEScoreWithLogger(t *testing.T) {
	e := &E{T: t}

	logger := &mockLogger{}
	e.WithLogger(logger)

	sample := Sample{ID: "test-sample-009"}
	scorer := &mockScorer{name: "test_scorer", score: 0.9}

	e.Score(sample, scorer)

	// Verify logger was called
	assert.True(t, logger.logCalled)
	assert.Equal(t, sample.ID, logger.lastSample.ID)
	assert.Equal(t, 0.9, logger.lastResult.OverallScore)
}

// TestEScoreWithLoggerError tests that E.Score() handles logger errors gracefully.
func TestEScoreWithLoggerError(t *testing.T) {
	e := &E{T: t}

	logger := &mockLogger{shouldFail: true}
	e.WithLogger(logger)

	sample := Sample{ID: "test-sample-010"}
	scorer := &mockScorer{name: "test_scorer", score: 0.9}

	// Should not panic even if logger fails
	result := e.Score(sample, scorer)

	// Score should still succeed
	assert.Equal(t, 0.9, result.OverallScore)
}

// TestEScoreWithOTel tests that E.Score() records OTel metrics.
func TestEScoreWithOTel(t *testing.T) {
	e := &E{T: t}

	tracer := nooptrace.NewTracerProvider().Tracer("test")
	meterProvider := noop.NewMeterProvider()

	e.WithOTel(OTelOptions{
		Tracer:        tracer,
		MeterProvider: meterProvider,
	})

	sample := Sample{ID: "test-sample-011"}
	scorer := &mockScorer{name: "test_scorer", score: 0.9}

	// Should not panic
	result := e.Score(sample, scorer)

	assert.Equal(t, 0.9, result.OverallScore)
}

// TestEScoreChaining tests that WithLogger and WithOTel can be chained.
func TestEScoreChaining(t *testing.T) {
	e := &E{T: t}

	logger := &mockLogger{}
	tracer := nooptrace.NewTracerProvider().Tracer("test")
	meterProvider := noop.NewMeterProvider()

	// Chain configuration methods
	e.WithLogger(logger).
		WithOTel(OTelOptions{
			Tracer:        tracer,
			MeterProvider: meterProvider,
		})

	// Verify all components are set
	assert.Equal(t, logger, e.logger)
	assert.Equal(t, tracer, e.otelTracer)
	assert.NotNil(t, e.otelMeter)
}

// mockLogger is a mock implementation of Logger for testing.
type mockLogger struct {
	logCalled  bool
	lastSample Sample
	lastResult Result
	shouldFail bool
}

func (m *mockLogger) Log(sample Sample, result Result) error {
	m.logCalled = true
	m.lastSample = sample
	m.lastResult = result
	if m.shouldFail {
		return errors.New("mock logger error")
	}
	return nil
}

func (m *mockLogger) Close() error {
	return nil
}

// TestOTelMetricsInitialization tests that OTel metrics are properly initialized.
func TestOTelMetricsInitialization(t *testing.T) {
	e := &E{T: t}

	// Use a noop meter provider
	meterProvider := noop.NewMeterProvider()
	meter := meterProvider.Meter("github.com/zeroroot-ai/sdk/eval")
	e.otelMeter = meter

	metrics, err := e.initOTelMetrics()
	require.NoError(t, err)
	require.NotNil(t, metrics)

	// Verify all metrics were created
	assert.NotNil(t, metrics.scoreHistogram)
	assert.NotNil(t, metrics.durationHistogram)
	assert.NotNil(t, metrics.countCounter)
}

// TestOTelMetricsInitializationWithNilMeter tests that initOTelMetrics handles nil meter.
func TestOTelMetricsInitializationWithNilMeter(t *testing.T) {
	e := &E{T: t}
	e.otelMeter = nil

	metrics, err := e.initOTelMetrics()
	assert.NoError(t, err)
	assert.Nil(t, metrics)
}

// TestRecordOTelScoreWithNoOTel tests that recordOTelScore handles missing OTel configuration.
func TestRecordOTelScoreWithNoOTel(t *testing.T) {
	e := &E{T: t}

	sample := Sample{ID: "test-sample"}
	result := Result{
		SampleID:     "test-sample",
		OverallScore: 0.9,
		Duration:     100 * time.Millisecond,
	}

	// Should not panic
	e.recordOTelScore(context.Background(), sample, result, 0.8)
}

// TestRecordOTelScoreWithTracer tests that recordOTelScore creates spans.
func TestRecordOTelScoreWithTracer(t *testing.T) {
	e := &E{T: t}

	tracer := nooptrace.NewTracerProvider().Tracer("test")
	e.otelTracer = tracer

	sample := Sample{ID: "test-sample"}
	result := Result{
		SampleID:     "test-sample",
		OverallScore: 0.9,
		Duration:     100 * time.Millisecond,
		Scores: map[string]ScoreResult{
			"scorer1": {Score: 0.9},
		},
	}

	// Should not panic
	e.recordOTelScore(context.Background(), sample, result, 0.8)
}

// TestRecordOTelScoreBelowThreshold tests span status when score is below threshold.
func TestRecordOTelScoreBelowThreshold(t *testing.T) {
	e := &E{T: t}

	tracer := nooptrace.NewTracerProvider().Tracer("test")
	e.otelTracer = tracer

	sample := Sample{ID: "test-sample"}
	result := Result{
		SampleID:     "test-sample",
		OverallScore: 0.5,
		Duration:     100 * time.Millisecond,
	}

	// Should not panic
	e.recordOTelScore(context.Background(), sample, result, 0.8)
}

// TestRecordOTelScoreWithError tests that recordOTelScore handles errors in results.
func TestRecordOTelScoreWithError(t *testing.T) {
	e := &E{T: t}

	tracer := nooptrace.NewTracerProvider().Tracer("test")
	e.otelTracer = tracer

	sample := Sample{ID: "test-sample"}
	result := Result{
		SampleID:     "test-sample",
		OverallScore: 0.5,
		Duration:     100 * time.Millisecond,
		Error:        "evaluation failed",
	}

	// Should not panic
	e.recordOTelScore(context.Background(), sample, result, 0.8)
}
