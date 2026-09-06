// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Zero Root AI

package eval

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/zeroroot-ai/sdk/agent"
	"go.opentelemetry.io/otel/metric/noop"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
)

func TestOTelIntegration_Tracer(t *testing.T) {
	// Create a test tracer
	tp := sdktrace.NewTracerProvider()
	defer tp.Shutdown(context.Background())
	tracer := tp.Tracer("test")

	// Create E instance with OTel configured
	e := &E{
		T: t,
	}
	e.WithOTel(OTelOptions{
		Tracer: tracer,
	})

	// Create test sample and result
	sample := Sample{
		ID: "test-001",
		Task: agent.Task{
			Context: map[string]any{"objective": "Test goal"},
		},
	}

	result := Result{
		SampleID:     "test-001",
		OverallScore: 0.85,
		Duration:     100 * time.Millisecond,
		Scores: map[string]ScoreResult{
			"scorer1": {Score: 0.9},
			"scorer2": {Score: 0.8},
		},
	}

	// Record OTel score - should not panic
	e.recordOTelScore(context.Background(), sample, result, 0.8)

	// Verify tracer was set
	assert.NotNil(t, e.otelTracer)
}

func TestOTelIntegration_Metrics(t *testing.T) {
	// Create a test meter provider
	meterProvider := noop.NewMeterProvider()

	// Create E instance with OTel configured
	e := &E{
		T: t,
	}
	e.WithOTel(OTelOptions{
		MeterProvider: meterProvider,
	})

	// Verify meter was set
	assert.NotNil(t, e.otelMeter)
	assert.NotNil(t, e.otelMetrics)

	// Create test sample and result
	sample := Sample{
		ID: "test-002",
		Task: agent.Task{
			Context: map[string]any{"objective": "Test goal"},
		},
	}

	result := Result{
		SampleID:     "test-002",
		OverallScore: 0.75,
		Duration:     200 * time.Millisecond,
		Scores: map[string]ScoreResult{
			"scorer1": {Score: 0.7},
			"scorer2": {Score: 0.8},
		},
	}

	// Record OTel score - should not panic
	e.recordOTelScore(context.Background(), sample, result, 0.8)
}

func TestOTelIntegration_BothTracerAndMetrics(t *testing.T) {
	// Create both tracer and meter provider
	tp := sdktrace.NewTracerProvider()
	defer tp.Shutdown(context.Background())
	tracer := tp.Tracer("test")
	meterProvider := noop.NewMeterProvider()

	// Create E instance with both configured
	e := &E{
		T: t,
	}
	e.WithOTel(OTelOptions{
		Tracer:        tracer,
		MeterProvider: meterProvider,
	})

	// Verify both were set
	assert.NotNil(t, e.otelTracer)
	assert.NotNil(t, e.otelMeter)
	assert.NotNil(t, e.otelMetrics)

	// Create test sample and result
	sample := Sample{
		ID: "test-003",
		Task: agent.Task{
			Context: map[string]any{"objective": "Test goal"},
		},
	}

	result := Result{
		SampleID:     "test-003",
		OverallScore: 0.65,
		Duration:     150 * time.Millisecond,
		Scores: map[string]ScoreResult{
			"scorer1": {Score: 0.6},
			"scorer2": {Score: 0.7},
		},
	}

	// Record OTel score - should not panic
	e.recordOTelScore(context.Background(), sample, result, 0.8)
}

func TestOTelIntegration_GracefulDegradation_NilOTel(t *testing.T) {
	// Create E instance without OTel
	e := &E{
		T: t,
	}

	// Create test sample and result
	sample := Sample{
		ID: "test-004",
		Task: agent.Task{
			Context: map[string]any{"objective": "Test goal"},
		},
	}

	result := Result{
		SampleID:     "test-004",
		OverallScore: 0.9,
		Duration:     50 * time.Millisecond,
		Scores: map[string]ScoreResult{
			"scorer1": {Score: 0.9},
		},
	}

	// Record OTel score with no OTel configured - should not panic
	e.recordOTelScore(context.Background(), sample, result, 0.8)

	// Verify nothing was set
	assert.Nil(t, e.otelTracer)
	assert.Nil(t, e.otelMeter)
	assert.Nil(t, e.otelMetrics)
}

func TestOTelIntegration_ScoreMethod(t *testing.T) {
	// Create tracer and meter provider
	tp := sdktrace.NewTracerProvider()
	defer tp.Shutdown(context.Background())
	tracer := tp.Tracer("test")
	meterProvider := noop.NewMeterProvider()

	// Create E instance with OTel
	e := &E{
		T:              t,
		scoreThreshold: 0.8,
	}
	e.WithOTel(OTelOptions{
		Tracer:        tracer,
		MeterProvider: meterProvider,
	})

	// Create a simple scorer
	scorer := &mockScorer{
		name:  "test-scorer",
		score: 0.85,
	}

	// Create test sample
	sample := Sample{
		ID: "test-005",
		Task: agent.Task{
			Context: map[string]any{"objective": "Test OTel integration"},
		},
	}

	// Score the sample - this should call recordOTelScore internally
	result := e.Score(sample, scorer)

	// Verify result
	assert.Equal(t, "test-005", result.SampleID)
	assert.Equal(t, 0.85, result.OverallScore)
	assert.Contains(t, result.Scores, "test-scorer")
}

func TestInitOTelMetrics_Success(t *testing.T) {
	// Create a test meter provider
	meterProvider := noop.NewMeterProvider()

	e := &E{
		T:         t,
		otelMeter: meterProvider.Meter("test"),
	}

	// Initialize metrics
	metrics, err := e.initOTelMetrics()
	require.NoError(t, err)
	require.NotNil(t, metrics)

	// Verify all metrics were created
	assert.NotNil(t, metrics.scoreHistogram)
	assert.NotNil(t, metrics.durationHistogram)
	assert.NotNil(t, metrics.countCounter)
}

func TestInitOTelMetrics_NilMeter(t *testing.T) {
	e := &E{
		T:         t,
		otelMeter: nil,
	}

	// Initialize metrics with nil meter - should return nil
	metrics, err := e.initOTelMetrics()
	assert.NoError(t, err)
	assert.Nil(t, metrics)
}
