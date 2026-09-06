// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Zero Root AI

package eval

import (
	"encoding/json"
	"fmt"
	"os"
	"sync"
	"time"
)

// LogEntry represents a single evaluation result entry in JSONL format.
// Each entry captures the sample ID, task information, scores, and execution metrics.
type LogEntry struct {
	// Timestamp is when the evaluation was performed.
	Timestamp time.Time `json:"timestamp"`

	// SampleID identifies the evaluated sample.
	SampleID string `json:"sample_id"`

	// TaskID is the task identifier from the sample, if available.
	// This is extracted from sample.Task.ID.
	TaskID string `json:"task_id,omitempty"`

	// Scores contains simplified score values keyed by scorer name.
	// This flattens the ScoreResult structure to just the numeric scores.
	Scores map[string]float64 `json:"scores"`

	// OverallScore is the aggregated score across all scorers (0.0 to 1.0).
	OverallScore float64 `json:"overall_score"`

	// Duration is the total time taken for evaluation in milliseconds.
	Duration int64 `json:"duration_ms"`

	// Details contains additional diagnostic information.
	// This can include scorer-specific details, error messages, or metadata.
	Details map[string]any `json:"details,omitempty"`
}

// JSONLLogger implements Logger by writing evaluation results to a JSONL file.
// Each result is written as a single JSON line for easy streaming and analysis.
// The logger is thread-safe and can be used concurrently from multiple goroutines.
type JSONLLogger struct {
	// path is the file path for the JSONL log file.
	path string

	// file is the underlying file handle.
	file *os.File

	// mu protects concurrent writes to the file.
	mu sync.Mutex
}

// NewJSONLLogger creates a new JSONL logger that writes to the specified file path.
// The file is opened in append mode (O_APPEND) and will be created if it doesn't exist.
// The returned logger must be closed when done to ensure all data is flushed.
//
// Example:
//
//	logger, err := eval.NewJSONLLogger("evals.jsonl")
//	if err != nil {
//	    return err
//	}
//	defer logger.Close()
func NewJSONLLogger(path string) (Logger, error) {
	// Open file in append mode, create if not exists
	file, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return nil, fmt.Errorf("failed to open log file %s: %w", path, err)
	}

	return &JSONLLogger{
		path: path,
		file: file,
	}, nil
}

// Log writes a sample and its result to the JSONL log file.
// The entry is written as a single JSON line followed by a newline character.
// The file is flushed after each write to ensure data is persisted immediately.
//
// This method is thread-safe and can be called concurrently.
func (l *JSONLLogger) Log(sample Sample, result Result) error {
	l.mu.Lock()
	defer l.mu.Unlock()

	// Extract task ID from sample if available
	taskID := sample.Task.ID

	// Flatten scores from map[string]ScoreResult to map[string]float64
	scores := make(map[string]float64, len(result.Scores))
	details := make(map[string]any)

	for name, scoreResult := range result.Scores {
		scores[name] = scoreResult.Score

		// Include scorer details if present
		if len(scoreResult.Details) > 0 {
			details[name+"_details"] = scoreResult.Details
		}
	}

	// Include error if present
	if result.Error != "" {
		details["error"] = result.Error
	}

	// Include sample metadata if present
	if len(sample.Metadata) > 0 {
		details["sample_metadata"] = sample.Metadata
	}

	// Include sample tags if present
	if len(sample.Tags) > 0 {
		details["sample_tags"] = sample.Tags
	}

	// Create log entry
	entry := LogEntry{
		Timestamp:    result.Timestamp,
		SampleID:     result.SampleID,
		TaskID:       taskID,
		Scores:       scores,
		OverallScore: result.OverallScore,
		Duration:     result.Duration.Milliseconds(),
		Details:      details,
	}

	// Marshal to JSON
	data, err := json.Marshal(entry)
	if err != nil {
		return fmt.Errorf("failed to marshal log entry: %w", err)
	}

	// Write JSON line
	_, err = l.file.Write(append(data, '\n'))
	if err != nil {
		return fmt.Errorf("failed to write log entry: %w", err)
	}

	// Flush to ensure data is persisted
	if err := l.file.Sync(); err != nil {
		return fmt.Errorf("failed to flush log file: %w", err)
	}

	return nil
}

// Close flushes any buffered data and closes the underlying file.
// This should be called when the logger is no longer needed, typically via defer.
//
// Example:
//
//	logger, err := eval.NewJSONLLogger("evals.jsonl")
//	if err != nil {
//	    return err
//	}
//	defer logger.Close()
func (l *JSONLLogger) Close() error {
	l.mu.Lock()
	defer l.mu.Unlock()

	// Sync any remaining data
	if err := l.file.Sync(); err != nil {
		return fmt.Errorf("failed to flush log file before close: %w", err)
	}

	// Close the file
	if err := l.file.Close(); err != nil {
		return fmt.Errorf("failed to close log file: %w", err)
	}

	return nil
}
