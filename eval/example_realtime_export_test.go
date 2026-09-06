// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Zero Root AI

package eval_test

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/zeroroot-ai/sdk/eval"
)

// ExampleLangfuseExporter_realTimeExport demonstrates how to use real-time
// partial score export with Langfuse.
func ExampleLangfuseExporter_realTimeExport() {
	// Create Langfuse exporter
	exporter := eval.NewLangfuseExporter(eval.LangfuseOptions{
		BaseURL:   "https://cloud.langfuse.com",
		PublicKey: os.Getenv("LANGFUSE_PUBLIC_KEY"),
		SecretKey: os.Getenv("LANGFUSE_SECRET_KEY"),
	})
	defer exporter.Close()

	// Enable real-time export with custom confidence threshold
	exporter.EnableRealTimeExport(eval.RealTimeExportOptions{
		ExportPartialScores: true,
		MinConfidence:       0.6, // Only export scores with 60%+ confidence
	})

	// During agent execution, export partial scores as they're generated
	ctx := context.Background()
	traceID := "trace-abc-123"

	// Example: Tool correctness scorer generates a partial score
	partialScore := eval.PartialScore{
		Score:      0.75,
		Confidence: 0.8,
		Status:     eval.ScoreStatusPartial,
		Feedback:   "Agent is making progress but has missed one expected tool call",
		Action:     eval.ActionContinue,
	}

	// Export the partial score - this is non-blocking
	err := exporter.ExportPartialScore(ctx, traceID, "tool_correctness", partialScore)
	if err != nil {
		// Handle error (typically just log it)
		_ = err
	}

	// The score will be sent to Langfuse asynchronously in the background
	// with the name "tool_correctness_partial"
}

// TestRealTimeExportIntegration demonstrates a complete mission with real-time
// export enabled in an evaluation context.
func TestRealTimeExportIntegration(t *testing.T) {
	// Skip if Langfuse credentials not set
	if os.Getenv("LANGFUSE_PUBLIC_KEY") == "" {
		t.Skip("LANGFUSE_PUBLIC_KEY not set")
	}

	eval.Run(t, "real_time_export_demo", func(e *eval.E) {
		// Create and configure Langfuse exporter
		exporter := eval.NewLangfuseExporter(eval.LangfuseOptions{
			BaseURL:   "https://cloud.langfuse.com",
			PublicKey: os.Getenv("LANGFUSE_PUBLIC_KEY"),
			SecretKey: os.Getenv("LANGFUSE_SECRET_KEY"),
		})
		defer exporter.Close()

		// Enable real-time export
		exporter.EnableRealTimeExport(eval.RealTimeExportOptions{
			ExportPartialScores: true,
			MinConfidence:       0.5,
		})

		// Attach to evaluation context
		e.WithLangfuse(exporter)

		// Simulate streaming evaluation with partial scores
		ctx := context.Background()
		traceID := "integration-test-trace"

		// Simulate partial scores at different stages
		stages := []struct {
			score      float64
			confidence float64
			status     eval.ScoreStatus
		}{
			{0.3, 0.4, eval.ScoreStatusPending},  // Low confidence, won't export
			{0.5, 0.6, eval.ScoreStatusPartial},  // Medium confidence, will export
			{0.7, 0.8, eval.ScoreStatusPartial},  // High confidence, will export
			{0.9, 1.0, eval.ScoreStatusComplete}, // Final score, will export
		}

		for _, stage := range stages {
			partialScore := eval.PartialScore{
				Score:      stage.score,
				Confidence: stage.confidence,
				Status:     stage.status,
			}

			err := exporter.ExportPartialScore(ctx, traceID, "tool_correctness", partialScore)
			if err != nil {
				t.Logf("Export error (expected for closed queue): %v", err)
			}

			// Small delay to simulate real-time processing
			time.Sleep(100 * time.Millisecond)
		}

		// Exporter will flush all pending exports on Close
		t.Log("Integration test completed - check Langfuse dashboard for real-time scores")
	})
}
