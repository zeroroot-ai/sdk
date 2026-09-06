// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Zero Root AI

//go:build standalone
// +build standalone

// Copyright (c) 2026 ZeroRoot
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package eval

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/zeroroot-ai/sdk/agent"
)

// Simple standalone test that verifies logger basic functionality
// Run with: go test -tags standalone -run TestLoggerStandalone
func TestLoggerStandalone(t *testing.T) {
	tmpDir := t.TempDir()
	logPath := filepath.Join(tmpDir, "test.jsonl")

	// Create logger
	logger, err := NewJSONLLogger(logPath)
	if err != nil {
		t.Fatalf("Failed to create logger: %v", err)
	}
	defer logger.Close()

	// Create a sample and result
	sample := Sample{
		ID: "test-001",
		Task: agent.Task{
			ID:      "task-001",
			Context: map[string]any{"objective": "Test task"},
		},
		Tags: []string{"test"},
	}

	result := Result{
		SampleID: "test-001",
		Scores: map[string]ScoreResult{
			"test_scorer": {
				Score: 0.85,
				Details: map[string]any{
					"precision": 0.9,
					"recall":    0.8,
				},
			},
		},
		OverallScore: 0.85,
		Duration:     250 * time.Millisecond,
		Timestamp:    time.Now(),
	}

	// Log it
	if err := logger.Log(sample, result); err != nil {
		t.Fatalf("Failed to log: %v", err)
	}

	// Close and read back
	if err := logger.Close(); err != nil {
		t.Fatalf("Failed to close: %v", err)
	}

	// Read and verify
	data, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatalf("Failed to read log file: %v", err)
	}

	var entry LogEntry
	if err := json.Unmarshal(data, &entry); err != nil {
		t.Fatalf("Failed to parse log entry: %v", err)
	}

	// Verify fields
	if entry.SampleID != "test-001" {
		t.Errorf("Expected SampleID test-001, got %s", entry.SampleID)
	}
	if entry.TaskID != "task-001" {
		t.Errorf("Expected TaskID task-001, got %s", entry.TaskID)
	}
	if entry.OverallScore != 0.85 {
		t.Errorf("Expected OverallScore 0.85, got %f", entry.OverallScore)
	}
	if entry.Duration != 250 {
		t.Errorf("Expected Duration 250ms, got %d", entry.Duration)
	}
	if score, ok := entry.Scores["test_scorer"]; !ok || score != 0.85 {
		t.Errorf("Expected test_scorer score 0.85, got %f", score)
	}

	t.Logf("Logger test passed! Entry: %+v", entry)
}
