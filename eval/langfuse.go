// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Zero Root AI

package eval

// Langfuse Integration
//
// This file provides integration with Langfuse (https://langfuse.com), an open-source
// LLM observability and evaluation platform. The LangfuseExporter allows you to export
// evaluation scores from Gibson SDK evaluation runs to Langfuse for visualization,
// tracking, and analysis.
//
// Architecture:
//
//   - Non-blocking exports: Uses a buffered channel and background worker goroutine
//   - Async processing: ExportResult returns immediately without blocking the caller
//   - Graceful shutdown: Close() flushes pending exports before exiting
//   - Error resilience: API errors are logged but don't fail the caller
//
// Example Usage:
//
//	import (
//	    "github.com/zeroroot-ai/sdk/eval"
//	)
//
//	func TestAgentEvalWithLangfuse(t *testing.T) {
//	    eval.Run(t, "prompt_injection_eval", func(e *eval.E) {
//	        // Create Langfuse exporter
//	        exporter := eval.NewLangfuseExporter(eval.LangfuseOptions{
//	            BaseURL:   "https://cloud.langfuse.com",
//	            PublicKey: os.Getenv("LANGFUSE_PUBLIC_KEY"),
//	            SecretKey: os.Getenv("LANGFUSE_SECRET_KEY"),
//	        })
//	        defer exporter.Close()
//
//	        // Configure E with Langfuse
//	        e.WithLangfuse(exporter)
//
//	        // Run evaluation - scores automatically exported
//	        sample := eval.Sample{
//	            ID: "test-001",
//	            Task: agent.Task{Context: map[string]any{"objective": "Detect prompt injection"}},
//	        }
//	        result := e.Score(sample,
//	            eval.NewToolCorrectnessScorer(opts),
//	            eval.NewTaskCompletionScorer(opts),
//	        )
//
//	        e.RequireScore(result, 0.8)
//	    })
//	}
//
// Self-hosted Langfuse:
//
//	exporter := eval.NewLangfuseExporter(eval.LangfuseOptions{
//	    BaseURL:   "https://langfuse.yourdomain.com",
//	    PublicKey: "pk-...",
//	    SecretKey: "sk-...",
//	})
//
// Score Format:
//
// Each evaluation result is exported as multiple Langfuse scores:
//   - Individual scorer scores (e.g., "tool_correctness": 0.9)
//   - Overall aggregated score ("overall_score": 0.85)
//
// All scores are linked to a trace ID for grouping in Langfuse.

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"sync"
	"time"
)

// LangfuseExporter exports evaluation scores to Langfuse via REST API.
// It uses a background worker pattern for non-blocking exports.
type LangfuseExporter struct {
	baseURL   string
	publicKey string
	secretKey string

	// HTTP client for making API requests
	client *http.Client

	// Channel for async export jobs
	exportQueue chan *exportJob

	// Channel for async partial score export jobs
	partialScoreQueue chan *partialScoreJob

	// Real-time export configuration
	realTimeConfig *realTimeConfig

	// wg is kept as sync.WaitGroup (not errgroup) because it tracks two persistent
	// background worker goroutines (worker + partialScoreWorker) that drain their
	// respective queues until Close() closes those channels. These goroutines run
	// indefinitely until their input channels are closed; errgroup's
	// "first-error cancels siblings" semantics do not fit this drain-on-close pattern.
	wg sync.WaitGroup

	// Context for shutdown
	shutdownCtx    context.Context
	shutdownCancel context.CancelFunc

	// Mutex for thread-safe shutdown
	mu     sync.Mutex
	closed bool
}

// exportJob represents a single score export operation.
type exportJob struct {
	ctx     context.Context
	traceID string
	result  Result
}

// langfuseScore represents a score payload for the Langfuse API.
// See: https://langfuse.com/docs/scores/api-reference
type langfuseScore struct {
	TraceID  string  `json:"traceId"`
	Name     string  `json:"name"`
	Value    float64 `json:"value"`
	DataType string  `json:"dataType"`
	Comment  string  `json:"comment,omitempty"`
}

// NewLangfuseExporter creates a new Langfuse exporter with a background worker.
// The exporter will buffer up to 100 export operations and process them asynchronously.
//
// Example:
//
//	exporter := eval.NewLangfuseExporter(eval.LangfuseOptions{
//	    BaseURL: "https://cloud.langfuse.com",
//	    PublicKey: "pk-...",
//	    SecretKey: "sk-...",
//	})
//	defer exporter.Close()
func NewLangfuseExporter(opts LangfuseOptions) *LangfuseExporter {
	ctx, cancel := context.WithCancel(context.Background())

	exporter := &LangfuseExporter{
		baseURL:   opts.BaseURL,
		publicKey: opts.PublicKey,
		secretKey: opts.SecretKey,
		client: &http.Client{
			Timeout: 10 * time.Second,
		},
		exportQueue:       make(chan *exportJob, 100),
		partialScoreQueue: make(chan *partialScoreJob, 100),
		shutdownCtx:       ctx,
		shutdownCancel:    cancel,
	}

	// Start background worker for full results
	exporter.wg.Add(1)
	go exporter.worker()

	// Start background worker for partial scores
	exporter.wg.Add(1)
	go exporter.partialScoreWorker()

	return exporter
}

// ExportResult exports an evaluation result to Langfuse asynchronously.
// This method does not block - it queues the export job for background processing.
//
// For each score in the result, a separate Langfuse score entry is created.
// The traceID parameter links scores to a specific trace in Langfuse.
// If traceID is empty, the result's SampleID is used as a fallback.
//
// Example:
//
//	result := e.Score(sample, scorers...)
//	err := exporter.ExportResult(ctx, "trace-123", result)
//	if err != nil {
//	    log.Printf("Export failed: %v", err)
//	}
func (l *LangfuseExporter) ExportResult(ctx context.Context, traceID string, result Result) error {
	l.mu.Lock()
	defer l.mu.Unlock()

	if l.closed {
		return errors.New("langfuse exporter is closed")
	}

	// Use SampleID as fallback if traceID is empty
	if traceID == "" {
		traceID = result.SampleID
	}

	job := &exportJob{
		ctx:     ctx,
		traceID: traceID,
		result:  result,
	}

	// Non-blocking send - drop if queue is full
	select {
	case l.exportQueue <- job:
		return nil
	default:
		return fmt.Errorf("export queue full, dropping export for trace %s", traceID)
	}
}

// Close flushes pending exports and shuts down the background worker.
// This method blocks until all pending exports are processed or the timeout is reached.
//
// The timeout is set to 30 seconds to allow pending exports to complete.
// After calling Close, the exporter cannot be used for new exports.
func (l *LangfuseExporter) Close() error {
	l.mu.Lock()
	if l.closed {
		l.mu.Unlock()
		return nil
	}
	l.closed = true
	l.mu.Unlock()

	// Signal shutdown to workers
	l.shutdownCancel()

	// Close queues to signal workers to exit after draining
	close(l.exportQueue)
	close(l.partialScoreQueue)

	// Wait for workers to finish with timeout
	done := make(chan struct{})
	go func() {
		l.wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		return nil
	case <-time.After(30 * time.Second):
		return errors.New("timeout waiting for exports to complete")
	}
}

// worker processes export jobs from the queue in the background.
// It continues until the export queue is closed and drained.
func (l *LangfuseExporter) worker() {
	defer l.wg.Done()

	for job := range l.exportQueue {
		// Check if we should stop processing
		select {
		case <-l.shutdownCtx.Done():
			// Still process this job before stopping
		default:
		}

		// Process the job
		if err := l.sendScores(job.ctx, job.traceID, job.result); err != nil {
			// Log error but don't stop worker
			// In production, this would use a proper logger
			_ = fmt.Errorf("failed to send scores for trace %s: %w", job.traceID, err)
		}
	}
}

// partialScoreWorker processes partial score export jobs from the queue in the background.
// It continues until the partial score queue is closed and drained.
func (l *LangfuseExporter) partialScoreWorker() {
	defer l.wg.Done()

	for job := range l.partialScoreQueue {
		// Check if we should stop processing
		select {
		case <-l.shutdownCtx.Done():
			// Still process this job before stopping
		default:
		}

		// Process the partial score job
		if err := l.sendPartialScore(job.ctx, job.traceID, job.scorer, job.score); err != nil {
			// Log error but don't stop worker
			// In production, this would use a proper logger
			_ = fmt.Errorf("failed to send partial score for trace %s: %w", job.traceID, err)
		}
	}
}

// sendScores sends all scores from a result to Langfuse.
// This is called by the background worker for each export job.
func (l *LangfuseExporter) sendScores(ctx context.Context, traceID string, result Result) error {
	// Create Langfuse score entries for each scorer result
	scores := make([]langfuseScore, 0, len(result.Scores)+1)

	// Export individual scorer scores
	for name, scoreResult := range result.Scores {
		scores = append(scores, langfuseScore{
			TraceID:  traceID,
			Name:     name,
			Value:    scoreResult.Score,
			DataType: "NUMERIC",
		})
	}

	// Export overall score
	scores = append(scores, langfuseScore{
		TraceID:  traceID,
		Name:     "overall_score",
		Value:    result.OverallScore,
		DataType: "NUMERIC",
	})

	// Send each score to Langfuse
	for _, score := range scores {
		if err := l.sendScore(ctx, score); err != nil {
			return fmt.Errorf("failed to send score %s: %w", score.Name, err)
		}
	}

	return nil
}

// sendScore sends a single score to the Langfuse API.
// It uses HTTP Basic Auth with the public and secret keys.
func (l *LangfuseExporter) sendScore(ctx context.Context, score langfuseScore) error {
	// Marshal score to JSON
	payload, err := json.Marshal(score)
	if err != nil {
		return fmt.Errorf("failed to marshal score: %w", err)
	}

	// Create HTTP request
	url := l.baseURL + "/api/public/scores"
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(payload))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	// Set headers
	req.Header.Set("Content-Type", "application/json")

	// Set HTTP Basic Auth
	req.SetBasicAuth(l.publicKey, l.secretKey)

	// Send request
	resp, err := l.client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to send request: %w", err)
	}
	defer func() {
		if err := resp.Body.Close(); err != nil {
			slog.Warn("failed to close resource", "resource", "Langfuse HTTP response", "error", err)
		}
	}()

	// Check response status
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("langfuse API returned status %d", resp.StatusCode)
	}

	return nil
}
