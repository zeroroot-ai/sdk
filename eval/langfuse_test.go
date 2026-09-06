// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Zero Root AI

package eval

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"
)

func TestNewLangfuseExporter(t *testing.T) {
	opts := LangfuseOptions{
		BaseURL:   "https://cloud.langfuse.com",
		PublicKey: "pk-test",
		SecretKey: "sk-test",
	}

	exporter := NewLangfuseExporter(opts)
	defer exporter.Close()

	if exporter == nil {
		t.Fatal("expected non-nil exporter")
	}

	if exporter.baseURL != opts.BaseURL {
		t.Errorf("expected baseURL %s, got %s", opts.BaseURL, exporter.baseURL)
	}

	if exporter.publicKey != opts.PublicKey {
		t.Errorf("expected publicKey %s, got %s", opts.PublicKey, exporter.publicKey)
	}

	if exporter.secretKey != opts.SecretKey {
		t.Errorf("expected secretKey %s, got %s", opts.SecretKey, exporter.secretKey)
	}

	if exporter.client == nil {
		t.Error("expected non-nil HTTP client")
	}

	if exporter.exportQueue == nil {
		t.Error("expected non-nil export queue")
	}
}

func TestLangfuseExporter_ExportResult(t *testing.T) {
	// Create mock HTTP server
	var mu sync.Mutex
	receivedScores := []langfuseScore{}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify request method and path
		if r.Method != http.MethodPost {
			t.Errorf("expected POST method, got %s", r.Method)
		}
		if r.URL.Path != "/api/public/scores" {
			t.Errorf("expected path /api/public/scores, got %s", r.URL.Path)
		}

		// Verify Basic Auth
		username, password, ok := r.BasicAuth()
		if !ok {
			t.Error("expected Basic Auth headers")
		}
		if username != "pk-test" {
			t.Errorf("expected username pk-test, got %s", username)
		}
		if password != "sk-test" {
			t.Errorf("expected password sk-test, got %s", password)
		}

		// Verify Content-Type
		contentType := r.Header.Get("Content-Type")
		if contentType != "application/json" {
			t.Errorf("expected Content-Type application/json, got %s", contentType)
		}

		// Parse request body
		body, err := io.ReadAll(r.Body)
		if err != nil {
			t.Errorf("failed to read request body: %v", err)
			w.WriteHeader(http.StatusBadRequest)
			return
		}

		var score langfuseScore
		if err := json.Unmarshal(body, &score); err != nil {
			t.Errorf("failed to unmarshal score: %v", err)
			w.WriteHeader(http.StatusBadRequest)
			return
		}

		mu.Lock()
		receivedScores = append(receivedScores, score)
		mu.Unlock()

		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	// Create exporter with test server URL
	opts := LangfuseOptions{
		BaseURL:   server.URL,
		PublicKey: "pk-test",
		SecretKey: "sk-test",
	}
	exporter := NewLangfuseExporter(opts)
	defer exporter.Close()

	// Create test result
	result := Result{
		SampleID:     "test-sample-1",
		OverallScore: 0.85,
		Scores: map[string]ScoreResult{
			"tool_correctness": {
				Score: 0.9,
				Details: map[string]any{
					"matched": 9,
					"total":   10,
				},
			},
			"task_completion": {
				Score: 0.8,
				Details: map[string]any{
					"completed": true,
				},
			},
		},
		Duration:  100 * time.Millisecond,
		Timestamp: time.Now(),
	}

	// Export result
	ctx := context.Background()
	traceID := "trace-123"

	err := exporter.ExportResult(ctx, traceID, result)
	if err != nil {
		t.Fatalf("ExportResult failed: %v", err)
	}

	// Wait for async export to complete
	time.Sleep(200 * time.Millisecond)

	// Verify received scores
	mu.Lock()
	defer mu.Unlock()

	expectedScoreCount := 3 // 2 individual scores + 1 overall score
	if len(receivedScores) != expectedScoreCount {
		t.Errorf("expected %d scores, got %d", expectedScoreCount, len(receivedScores))
	}

	// Check that all scores have correct traceID
	for _, score := range receivedScores {
		if score.TraceID != traceID {
			t.Errorf("expected traceID %s, got %s", traceID, score.TraceID)
		}
		if score.DataType != "NUMERIC" {
			t.Errorf("expected dataType NUMERIC, got %s", score.DataType)
		}
	}

	// Check for specific scores
	scoresByName := make(map[string]langfuseScore)
	for _, score := range receivedScores {
		scoresByName[score.Name] = score
	}

	// Verify tool_correctness score
	if score, ok := scoresByName["tool_correctness"]; ok {
		if score.Value != 0.9 {
			t.Errorf("expected tool_correctness score 0.9, got %.2f", score.Value)
		}
	} else {
		t.Error("missing tool_correctness score")
	}

	// Verify task_completion score
	if score, ok := scoresByName["task_completion"]; ok {
		if score.Value != 0.8 {
			t.Errorf("expected task_completion score 0.8, got %.2f", score.Value)
		}
	} else {
		t.Error("missing task_completion score")
	}

	// Verify overall_score
	if score, ok := scoresByName["overall_score"]; ok {
		if score.Value != 0.85 {
			t.Errorf("expected overall_score 0.85, got %.2f", score.Value)
		}
	} else {
		t.Error("missing overall_score")
	}
}

func TestLangfuseExporter_ExportResult_EmptyTraceID(t *testing.T) {
	// Create mock HTTP server
	var mu sync.Mutex
	receivedScores := []langfuseScore{}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		var score langfuseScore
		json.Unmarshal(body, &score)

		mu.Lock()
		receivedScores = append(receivedScores, score)
		mu.Unlock()

		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	// Create exporter
	opts := LangfuseOptions{
		BaseURL:   server.URL,
		PublicKey: "pk-test",
		SecretKey: "sk-test",
	}
	exporter := NewLangfuseExporter(opts)
	defer exporter.Close()

	// Create test result
	result := Result{
		SampleID:     "test-sample-1",
		OverallScore: 0.85,
		Scores: map[string]ScoreResult{
			"test_scorer": {Score: 0.85},
		},
	}

	// Export with empty traceID - should use SampleID
	ctx := context.Background()
	err := exporter.ExportResult(ctx, "", result)
	if err != nil {
		t.Fatalf("ExportResult failed: %v", err)
	}

	// Wait for async export
	time.Sleep(200 * time.Millisecond)

	// Verify traceID is set to SampleID
	mu.Lock()
	defer mu.Unlock()

	for _, score := range receivedScores {
		if score.TraceID != result.SampleID {
			t.Errorf("expected traceID to fallback to SampleID %s, got %s", result.SampleID, score.TraceID)
		}
	}
}

func TestLangfuseExporter_Close(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	opts := LangfuseOptions{
		BaseURL:   server.URL,
		PublicKey: "pk-test",
		SecretKey: "sk-test",
	}
	exporter := NewLangfuseExporter(opts)

	// Close should not error
	err := exporter.Close()
	if err != nil {
		t.Errorf("Close failed: %v", err)
	}

	// Subsequent close should also not error
	err = exporter.Close()
	if err != nil {
		t.Errorf("second Close failed: %v", err)
	}

	// Export after close should error
	result := Result{
		SampleID:     "test",
		OverallScore: 0.5,
		Scores:       map[string]ScoreResult{},
	}
	err = exporter.ExportResult(context.Background(), "trace", result)
	if err == nil {
		t.Error("expected error when exporting after close")
	}
}

func TestLangfuseExporter_APIError(t *testing.T) {
	// Create mock server that returns errors
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
	}))
	defer server.Close()

	opts := LangfuseOptions{
		BaseURL:   server.URL,
		PublicKey: "pk-invalid",
		SecretKey: "sk-invalid",
	}
	exporter := NewLangfuseExporter(opts)
	defer exporter.Close()

	result := Result{
		SampleID:     "test",
		OverallScore: 0.5,
		Scores: map[string]ScoreResult{
			"test": {Score: 0.5},
		},
	}

	// Export should not block even if API fails
	err := exporter.ExportResult(context.Background(), "trace", result)
	if err != nil {
		t.Errorf("ExportResult should not error synchronously: %v", err)
	}

	// Wait for background processing
	time.Sleep(200 * time.Millisecond)

	// Exporter should still be operational
	if exporter.closed {
		t.Error("exporter should not be closed after API error")
	}
}

func TestLangfuseExporter_FullQueueDrops(t *testing.T) {
	// Create slow server to fill up queue
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(100 * time.Millisecond)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	opts := LangfuseOptions{
		BaseURL:   server.URL,
		PublicKey: "pk-test",
		SecretKey: "sk-test",
	}
	exporter := NewLangfuseExporter(opts)
	defer exporter.Close()

	result := Result{
		SampleID:     "test",
		OverallScore: 0.5,
		Scores:       map[string]ScoreResult{},
	}

	// Try to fill queue beyond capacity (100 slots)
	var dropCount int
	for range 150 {
		err := exporter.ExportResult(context.Background(), "trace", result)
		if err != nil {
			dropCount++
		}
	}

	// Should have dropped some exports when queue was full
	if dropCount == 0 {
		t.Error("expected some exports to be dropped when queue is full")
	}
}

func TestLangfuseExporter_ConcurrentExports(t *testing.T) {
	var mu sync.Mutex
	receivedCount := 0

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		receivedCount++
		mu.Unlock()
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	opts := LangfuseOptions{
		BaseURL:   server.URL,
		PublicKey: "pk-test",
		SecretKey: "sk-test",
	}
	exporter := NewLangfuseExporter(opts)
	defer exporter.Close()

	// Export concurrently from multiple goroutines
	const numGoroutines = 10
	const exportsPerGoroutine = 3

	var wg sync.WaitGroup
	for i := range numGoroutines {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			for range exportsPerGoroutine {
				result := Result{
					SampleID:     "test",
					OverallScore: 0.5,
					Scores: map[string]ScoreResult{
						"test": {Score: 0.5},
					},
				}
				exporter.ExportResult(context.Background(), "trace", result)
			}
		}(i)
	}

	wg.Wait()

	// Wait for all exports to complete
	time.Sleep(500 * time.Millisecond)

	// Should have received all scores (each result has 2 scores: test + overall_score)
	mu.Lock()
	expectedCount := numGoroutines * exportsPerGoroutine * 2
	if receivedCount != expectedCount {
		t.Errorf("expected %d scores, got %d", expectedCount, receivedCount)
	}
	mu.Unlock()
}
