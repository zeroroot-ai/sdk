// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Zero Root AI

package eval

import (
	"bufio"
	"encoding/json"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/zeroroot-ai/sdk/agent"
)

func TestNewJSONLLogger(t *testing.T) {
	t.Run("creates new file", func(t *testing.T) {
		tmpDir := t.TempDir()
		logPath := filepath.Join(tmpDir, "test.jsonl")

		logger, err := NewJSONLLogger(logPath)
		require.NoError(t, err)
		defer logger.Close()

		// Verify file exists
		_, err = os.Stat(logPath)
		assert.NoError(t, err, "log file should exist")
	})

	t.Run("appends to existing file", func(t *testing.T) {
		tmpDir := t.TempDir()
		logPath := filepath.Join(tmpDir, "test.jsonl")

		// Create initial file with content
		err := os.WriteFile(logPath, []byte(`{"test":"data"}`+"\n"), 0644)
		require.NoError(t, err)

		logger, err := NewJSONLLogger(logPath)
		require.NoError(t, err)
		defer logger.Close()

		// Write a sample
		sample := Sample{
			ID:   "test-001",
			Task: agent.Task{ID: "task-001"},
		}
		result := Result{
			SampleID:     "test-001",
			Scores:       map[string]ScoreResult{"test": {Score: 0.8}},
			OverallScore: 0.8,
			Timestamp:    time.Now(),
			Duration:     100 * time.Millisecond,
		}

		err = logger.Log(sample, result)
		require.NoError(t, err)

		// Read file and verify both lines exist
		data, err := os.ReadFile(logPath)
		require.NoError(t, err)

		lines := 0
		file, err := os.Open(logPath)
		require.NoError(t, err)
		defer file.Close()
		scanner := bufio.NewScanner(file)
		for scanner.Scan() {
			lines++
		}
		assert.Equal(t, 2, lines, "file should have 2 lines (original + new)")
		assert.Contains(t, string(data), `{"test":"data"}`)
		assert.Contains(t, string(data), `"sample_id":"test-001"`)
	})

	t.Run("returns error for invalid path", func(t *testing.T) {
		// Try to create logger in non-existent directory
		logger, err := NewJSONLLogger("/nonexistent/directory/test.jsonl")
		assert.Error(t, err)
		assert.Nil(t, logger)
	})
}

func TestJSONLLogger_Log(t *testing.T) {
	t.Run("writes valid JSON line", func(t *testing.T) {
		tmpDir := t.TempDir()
		logPath := filepath.Join(tmpDir, "test.jsonl")

		logger, err := NewJSONLLogger(logPath)
		require.NoError(t, err)
		defer logger.Close()

		// Create sample and result
		sample := Sample{
			ID: "test-001",
			Task: agent.Task{
				ID:      "task-001",
				Context: map[string]any{"objective": "Test task"},
			},
			Metadata: map[string]any{
				"difficulty": "easy",
			},
			Tags: []string{"test", "unit"},
		}

		result := Result{
			SampleID: "test-001",
			Scores: map[string]ScoreResult{
				"scorer1": {
					Score: 0.8,
					Details: map[string]any{
						"precision": 0.85,
						"recall":    0.75,
					},
				},
				"scorer2": {
					Score: 0.6,
				},
			},
			OverallScore: 0.7,
			Duration:     250 * time.Millisecond,
			Timestamp:    time.Date(2024, 1, 1, 12, 0, 0, 0, time.UTC),
		}

		// Log the result
		err = logger.Log(sample, result)
		require.NoError(t, err)

		// Read and parse the logged entry
		data, err := os.ReadFile(logPath)
		require.NoError(t, err)

		var entry LogEntry
		err = json.Unmarshal(data, &entry)
		require.NoError(t, err)

		// Verify entry fields
		assert.Equal(t, "test-001", entry.SampleID)
		assert.Equal(t, "task-001", entry.TaskID)
		assert.Equal(t, 0.7, entry.OverallScore)
		assert.Equal(t, int64(250), entry.Duration)
		assert.Equal(t, time.Date(2024, 1, 1, 12, 0, 0, 0, time.UTC), entry.Timestamp)

		// Verify scores
		assert.Len(t, entry.Scores, 2)
		assert.Equal(t, 0.8, entry.Scores["scorer1"])
		assert.Equal(t, 0.6, entry.Scores["scorer2"])

		// Verify details
		assert.NotNil(t, entry.Details)
		assert.Contains(t, entry.Details, "scorer1_details")
		assert.Contains(t, entry.Details, "sample_metadata")
		assert.Contains(t, entry.Details, "sample_tags")

		// Verify scorer details
		scorer1Details, ok := entry.Details["scorer1_details"].(map[string]any)
		require.True(t, ok)
		assert.Equal(t, 0.85, scorer1Details["precision"])
		assert.Equal(t, 0.75, scorer1Details["recall"])
	})

	t.Run("includes error in details", func(t *testing.T) {
		tmpDir := t.TempDir()
		logPath := filepath.Join(tmpDir, "test.jsonl")

		logger, err := NewJSONLLogger(logPath)
		require.NoError(t, err)
		defer logger.Close()

		sample := Sample{
			ID:   "test-002",
			Task: agent.Task{ID: "task-002"},
		}

		result := Result{
			SampleID:     "test-002",
			Scores:       map[string]ScoreResult{},
			OverallScore: 0.0,
			Timestamp:    time.Now(),
			Duration:     0,
			Error:        "evaluation failed: timeout",
		}

		err = logger.Log(sample, result)
		require.NoError(t, err)

		// Read and parse
		data, err := os.ReadFile(logPath)
		require.NoError(t, err)

		var entry LogEntry
		err = json.Unmarshal(data, &entry)
		require.NoError(t, err)

		// Verify error is in details
		assert.Contains(t, entry.Details, "error")
		assert.Equal(t, "evaluation failed: timeout", entry.Details["error"])
	})

	t.Run("handles missing task ID", func(t *testing.T) {
		tmpDir := t.TempDir()
		logPath := filepath.Join(tmpDir, "test.jsonl")

		logger, err := NewJSONLLogger(logPath)
		require.NoError(t, err)
		defer logger.Close()

		sample := Sample{
			ID:   "test-003",
			Task: agent.Task{}, // No ID
		}

		result := Result{
			SampleID:     "test-003",
			Scores:       map[string]ScoreResult{"test": {Score: 0.5}},
			OverallScore: 0.5,
			Timestamp:    time.Now(),
			Duration:     100 * time.Millisecond,
		}

		err = logger.Log(sample, result)
		require.NoError(t, err)

		// Read and parse
		data, err := os.ReadFile(logPath)
		require.NoError(t, err)

		var entry LogEntry
		err = json.Unmarshal(data, &entry)
		require.NoError(t, err)

		// TaskID should be empty
		assert.Empty(t, entry.TaskID)
	})

	t.Run("writes multiple entries", func(t *testing.T) {
		tmpDir := t.TempDir()
		logPath := filepath.Join(tmpDir, "test.jsonl")

		logger, err := NewJSONLLogger(logPath)
		require.NoError(t, err)
		defer logger.Close()

		// Write 3 entries
		for i := 1; i <= 3; i++ {
			sample := Sample{
				ID:   "test-" + string(rune('0'+i)),
				Task: agent.Task{ID: "task-" + string(rune('0'+i))},
			}
			result := Result{
				SampleID:     sample.ID,
				Scores:       map[string]ScoreResult{"test": {Score: float64(i) * 0.3}},
				OverallScore: float64(i) * 0.3,
				Timestamp:    time.Now(),
				Duration:     time.Duration(i*100) * time.Millisecond,
			}

			err = logger.Log(sample, result)
			require.NoError(t, err)
		}

		// Read all lines
		file, err := os.Open(logPath)
		require.NoError(t, err)
		defer file.Close()

		scanner := bufio.NewScanner(file)
		lineCount := 0
		for scanner.Scan() {
			lineCount++
			var entry LogEntry
			err := json.Unmarshal(scanner.Bytes(), &entry)
			require.NoError(t, err, "line %d should be valid JSON", lineCount)
		}

		assert.Equal(t, 3, lineCount, "should have 3 lines")
	})
}

func TestJSONLLogger_Concurrent(t *testing.T) {
	t.Run("handles concurrent writes safely", func(t *testing.T) {
		tmpDir := t.TempDir()
		logPath := filepath.Join(tmpDir, "test.jsonl")

		logger, err := NewJSONLLogger(logPath)
		require.NoError(t, err)
		defer logger.Close()

		// Write 100 entries concurrently
		const numWrites = 100
		var wg sync.WaitGroup
		wg.Add(numWrites)

		for i := range numWrites {
			go func(id int) {
				defer wg.Done()

				sample := Sample{
					ID:   "test-" + string(rune('0'+id)),
					Task: agent.Task{ID: "task-" + string(rune('0'+id))},
				}
				result := Result{
					SampleID:     sample.ID,
					Scores:       map[string]ScoreResult{"test": {Score: 0.5}},
					OverallScore: 0.5,
					Timestamp:    time.Now(),
					Duration:     100 * time.Millisecond,
				}

				err := logger.Log(sample, result)
				assert.NoError(t, err)
			}(i)
		}

		wg.Wait()

		// Count lines
		file, err := os.Open(logPath)
		require.NoError(t, err)
		defer file.Close()

		scanner := bufio.NewScanner(file)
		lineCount := 0
		for scanner.Scan() {
			lineCount++
			// Verify each line is valid JSON
			var entry LogEntry
			err := json.Unmarshal(scanner.Bytes(), &entry)
			assert.NoError(t, err, "line %d should be valid JSON", lineCount)
		}

		assert.Equal(t, numWrites, lineCount, "should have %d lines", numWrites)
	})
}

func TestJSONLLogger_Close(t *testing.T) {
	t.Run("flushes and closes file", func(t *testing.T) {
		tmpDir := t.TempDir()
		logPath := filepath.Join(tmpDir, "test.jsonl")

		logger, err := NewJSONLLogger(logPath)
		require.NoError(t, err)

		// Write an entry
		sample := Sample{
			ID:   "test-001",
			Task: agent.Task{ID: "task-001"},
		}
		result := Result{
			SampleID:     "test-001",
			Scores:       map[string]ScoreResult{"test": {Score: 0.8}},
			OverallScore: 0.8,
			Timestamp:    time.Now(),
			Duration:     100 * time.Millisecond,
		}

		err = logger.Log(sample, result)
		require.NoError(t, err)

		// Close the logger
		err = logger.Close()
		require.NoError(t, err)

		// Verify file is readable and contains data
		data, err := os.ReadFile(logPath)
		require.NoError(t, err)
		assert.NotEmpty(t, data)
		assert.Contains(t, string(data), `"sample_id":"test-001"`)
	})

	t.Run("can be called multiple times safely", func(t *testing.T) {
		tmpDir := t.TempDir()
		logPath := filepath.Join(tmpDir, "test.jsonl")

		logger, err := NewJSONLLogger(logPath)
		require.NoError(t, err)

		// Close multiple times should not panic
		err1 := logger.Close()
		err2 := logger.Close()

		// First close should succeed, second might error (file already closed)
		assert.NoError(t, err1)
		// We don't assert on err2 since it's implementation-dependent
		_ = err2
	})
}

func TestLogEntry_Serialization(t *testing.T) {
	t.Run("marshals and unmarshals correctly", func(t *testing.T) {
		entry := LogEntry{
			Timestamp:    time.Date(2024, 1, 1, 12, 0, 0, 0, time.UTC),
			SampleID:     "test-001",
			TaskID:       "task-001",
			Scores:       map[string]float64{"scorer1": 0.8, "scorer2": 0.6},
			OverallScore: 0.7,
			Duration:     250,
			Details: map[string]any{
				"precision": 0.85,
				"recall":    0.75,
			},
		}

		// Marshal
		data, err := json.Marshal(entry)
		require.NoError(t, err)

		// Unmarshal
		var decoded LogEntry
		err = json.Unmarshal(data, &decoded)
		require.NoError(t, err)

		// Verify
		assert.Equal(t, entry.SampleID, decoded.SampleID)
		assert.Equal(t, entry.TaskID, decoded.TaskID)
		assert.Equal(t, entry.Scores, decoded.Scores)
		assert.Equal(t, entry.OverallScore, decoded.OverallScore)
		assert.Equal(t, entry.Duration, decoded.Duration)
		assert.Equal(t, entry.Timestamp.Unix(), decoded.Timestamp.Unix())
	})

	t.Run("handles optional fields", func(t *testing.T) {
		entry := LogEntry{
			Timestamp:    time.Now(),
			SampleID:     "test-001",
			Scores:       map[string]float64{"test": 0.5},
			OverallScore: 0.5,
			Duration:     100,
			// TaskID and Details omitted
		}

		data, err := json.Marshal(entry)
		require.NoError(t, err)

		var decoded LogEntry
		err = json.Unmarshal(data, &decoded)
		require.NoError(t, err)

		assert.Empty(t, decoded.TaskID)
		assert.Nil(t, decoded.Details)
	})
}
