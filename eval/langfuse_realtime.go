// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Zero Root AI

package eval

// Langfuse Real-Time Export Integration
//
// This file extends the LangfuseExporter with real-time partial score export capabilities.
// It allows streaming evaluation scores to Langfuse as agents execute, providing live
// feedback on agent performance.
//
// Architecture:
//
//   - Uses the same non-blocking export queue as the base exporter
//   - Confidence filtering to avoid exporting low-confidence partial scores
//   - Integrates with StreamingScorer interface for real-time evaluation
//
// Example Usage:
//
//	import (
//	    "github.com/zeroroot-ai/sdk/eval"
//	)
//
//	func TestAgentWithRealTimeFeedback(t *testing.T) {
//	    eval.Run(t, "streaming_eval", func(e *eval.E) {
//	        // Create Langfuse exporter with real-time export
//	        exporter := eval.NewLangfuseExporter(eval.LangfuseOptions{
//	            BaseURL:   "https://cloud.langfuse.com",
//	            PublicKey: os.Getenv("LANGFUSE_PUBLIC_KEY"),
//	            SecretKey: os.Getenv("LANGFUSE_SECRET_KEY"),
//	        })
//	        defer exporter.Close()
//
//	        // Enable real-time export with filtering
//	        exporter.EnableRealTimeExport(eval.RealTimeExportOptions{
//	            ExportPartialScores: true,
//	            MinConfidence:       0.6, // Only export scores with 60%+ confidence
//	        })
//
//	        e.WithLangfuse(exporter)
//
//	        // During agent execution, partial scores are automatically exported
//	        // to Langfuse as the agent generates trajectory steps
//	    })
//	}
//
// Confidence Filtering:
//
// Partial scores are only exported if their confidence meets the MinConfidence threshold.
// This prevents noisy or unreliable scores from cluttering the Langfuse dashboard.
//
//	opts := eval.RealTimeExportOptions{
//	    MinConfidence: 0.5, // Default: only export scores with 50%+ confidence
//	}
//
// Score Naming:
//
// Real-time scores are sent with a "_partial" suffix to distinguish them from final scores:
//   - "tool_correctness_partial" vs "tool_correctness"
//   - "task_completion_partial" vs "task_completion"

import (
	"context"
	"errors"
	"fmt"
)

// RealTimeExportOptions configures real-time partial score export behavior.
type RealTimeExportOptions struct {
	// ExportPartialScores enables real-time export of streaming evaluation scores.
	// When false, only final scores from completed evaluations are exported.
	ExportPartialScores bool

	// MinConfidence is the minimum confidence threshold for exporting partial scores.
	// Scores with confidence below this value will be filtered out.
	// Valid range: 0.0 to 1.0. Default: 0.5.
	MinConfidence float64
}

// realTimeConfig holds the internal real-time export configuration.
type realTimeConfig struct {
	enabled       bool
	minConfidence float64
}

// EnableRealTimeExport configures the exporter to send partial scores in real-time.
// This should be called after creating the exporter and before any export operations.
//
// Real-time export uses the same non-blocking export queue as standard exports,
// ensuring that partial score exports don't block agent execution.
//
// Example:
//
//	exporter := eval.NewLangfuseExporter(opts)
//	exporter.EnableRealTimeExport(eval.RealTimeExportOptions{
//	    ExportPartialScores: true,
//	    MinConfidence:       0.6,
//	})
func (l *LangfuseExporter) EnableRealTimeExport(opts RealTimeExportOptions) {
	l.mu.Lock()
	defer l.mu.Unlock()

	// Set default MinConfidence if not specified
	minConfidence := opts.MinConfidence
	if minConfidence == 0 {
		minConfidence = 0.5
	}

	// Initialize real-time config if needed
	if l.realTimeConfig == nil {
		l.realTimeConfig = &realTimeConfig{}
	}

	l.realTimeConfig.enabled = opts.ExportPartialScores
	l.realTimeConfig.minConfidence = minConfidence
}

// ExportPartialScore exports a single partial score to Langfuse in real-time.
// This is called by streaming scorers during agent execution to provide live feedback.
//
// The score is only exported if:
//   - Real-time export is enabled via EnableRealTimeExport
//   - The score's confidence meets or exceeds MinConfidence threshold
//
// The export is non-blocking - it queues the score for background processing and returns
// immediately without waiting for the API call to complete.
//
// Example:
//
//	partialScore := PartialScore{
//	    Score:      0.75,
//	    Confidence: 0.8,
//	    Status:     ScoreStatusPartial,
//	}
//	err := exporter.ExportPartialScore(ctx, "trace-123", "tool_correctness", partialScore)
//	if err != nil {
//	    log.Printf("Failed to export partial score: %v", err)
//	}
//
// Parameters:
//   - ctx: Context for cancellation and timeout
//   - traceID: Langfuse trace ID to link this score to
//   - scorer: Name of the scorer that generated this score
//   - score: The partial score to export
func (l *LangfuseExporter) ExportPartialScore(ctx context.Context, traceID string, scorer string, score PartialScore) error {
	l.mu.Lock()
	defer l.mu.Unlock()

	// Check if exporter is closed
	if l.closed {
		return errors.New("langfuse exporter is closed")
	}

	// Check if real-time export is enabled
	if l.realTimeConfig == nil || !l.realTimeConfig.enabled {
		// Silently skip if real-time export is not enabled
		return nil
	}

	// Check confidence threshold
	if score.Confidence < l.realTimeConfig.minConfidence {
		// Silently skip scores below confidence threshold
		return nil
	}

	// Create partial score export job
	job := &partialScoreJob{
		ctx:     ctx,
		traceID: traceID,
		scorer:  scorer,
		score:   score,
	}

	// Non-blocking send - drop if queue is full
	select {
	case l.partialScoreQueue <- job:
		return nil
	default:
		return fmt.Errorf("partial score queue full, dropping export for trace %s", traceID)
	}
}

// partialScoreJob represents a single partial score export operation.
type partialScoreJob struct {
	ctx     context.Context
	traceID string
	scorer  string
	score   PartialScore
}

// sendPartialScore sends a partial score to the Langfuse API.
// This is called by the background worker for each partial score export job.
func (l *LangfuseExporter) sendPartialScore(ctx context.Context, traceID string, scorer string, score PartialScore) error {
	// Build score name with "_partial" suffix to distinguish from final scores
	scoreName := scorer + "_partial"

	// Build comment with metadata about the partial score
	comment := fmt.Sprintf("Partial score - status: %s, confidence: %.2f", score.Status, score.Confidence)

	// Create Langfuse score payload
	langfuseScore := langfuseScore{
		TraceID:  traceID,
		Name:     scoreName,
		Value:    score.Score,
		DataType: "NUMERIC",
		Comment:  comment,
	}

	// Send to Langfuse API
	if err := l.sendScore(ctx, langfuseScore); err != nil {
		return fmt.Errorf("failed to send partial score: %w", err)
	}

	return nil
}
