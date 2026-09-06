// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Zero Root AI

package eval

import (
	"context"
	"testing"

	"github.com/zeroroot-ai/sdk/agent"
	"github.com/zeroroot-ai/sdk/llm"
)

func TestTaskCompletionScorer_ExactMatch(t *testing.T) {
	scorer := NewTaskCompletionScorer(TaskCompletionOptions{
		ExpectedOutput: "success",
	})

	sample := Sample{
		ID: "test-1",
		Task: agent.Task{
			Context: map[string]any{"objective": "Complete the task"},
		},
		Result: agent.Result{
			Output: "success",
		},
	}

	result, err := scorer.Score(context.Background(), sample)
	if err != nil {
		t.Fatalf("Score() failed: %v", err)
	}

	if result.Score != 1.0 {
		t.Errorf("Expected score 1.0 for exact match, got %.2f", result.Score)
	}

	if result.Details["matched"] != true {
		t.Errorf("Expected matched=true in details")
	}
}

func TestTaskCompletionScorer_NoMatch(t *testing.T) {
	scorer := NewTaskCompletionScorer(TaskCompletionOptions{
		ExpectedOutput: "success",
	})

	sample := Sample{
		ID: "test-2",
		Task: agent.Task{
			Context: map[string]any{"objective": "Complete the task"},
		},
		Result: agent.Result{
			Output: "failure",
		},
	}

	result, err := scorer.Score(context.Background(), sample)
	if err != nil {
		t.Fatalf("Score() failed: %v", err)
	}

	if result.Score != 0.0 {
		t.Errorf("Expected score 0.0 for no match, got %.2f", result.Score)
	}

	if result.Details["matched"] != false {
		t.Errorf("Expected matched=false in details")
	}
}

func TestTaskCompletionScorer_CaseInsensitive(t *testing.T) {
	scorer := NewTaskCompletionScorer(TaskCompletionOptions{
		ExpectedOutput: "Success",
	})

	sample := Sample{
		ID: "test-3",
		Task: agent.Task{
			Context: map[string]any{"objective": "Complete the task"},
		},
		Result: agent.Result{
			Output: "success",
		},
	}

	result, err := scorer.Score(context.Background(), sample)
	if err != nil {
		t.Fatalf("Score() failed: %v", err)
	}

	if result.Score != 1.0 {
		t.Errorf("Expected score 1.0 for case-insensitive match, got %.2f", result.Score)
	}

	if result.Details["match_type"] != "case_insensitive" {
		t.Errorf("Expected match_type=case_insensitive, got %v", result.Details["match_type"])
	}
}

func TestTaskCompletionScorer_Substring(t *testing.T) {
	scorer := NewTaskCompletionScorer(TaskCompletionOptions{
		ExpectedOutput: "vulnerability",
	})

	sample := Sample{
		ID: "test-4",
		Task: agent.Task{
			Context: map[string]any{"objective": "Find security issues"},
		},
		Result: agent.Result{
			Output: "Found SQL injection vulnerability in login form",
		},
	}

	result, err := scorer.Score(context.Background(), sample)
	if err != nil {
		t.Fatalf("Score() failed: %v", err)
	}

	if result.Score != 0.9 {
		t.Errorf("Expected score 0.9 for substring match, got %.2f", result.Score)
	}

	if result.Details["match_type"] != "substring" {
		t.Errorf("Expected match_type=substring, got %v", result.Details["match_type"])
	}
}

func TestTaskCompletionScorer_Binary(t *testing.T) {
	scorer := NewTaskCompletionScorer(TaskCompletionOptions{
		ExpectedOutput: "vulnerability",
		Binary:         true,
	})

	// Test case 1: Substring match (0.9) should round to 1.0
	sample1 := Sample{
		ID: "test-5a",
		Task: agent.Task{
			Context: map[string]any{"objective": "Find security issues"},
		},
		Result: agent.Result{
			Output: "Found SQL injection vulnerability",
		},
	}

	result1, err := scorer.Score(context.Background(), sample1)
	if err != nil {
		t.Fatalf("Score() failed: %v", err)
	}

	if result1.Score != 1.0 {
		t.Errorf("Expected binary score 1.0, got %.2f", result1.Score)
	}

	// Test case 2: No match (0.0) should stay 0.0
	sample2 := Sample{
		ID: "test-5b",
		Task: agent.Task{
			Context: map[string]any{"objective": "Find security issues"},
		},
		Result: agent.Result{
			Output: "No issues found",
		},
	}

	result2, err := scorer.Score(context.Background(), sample2)
	if err != nil {
		t.Fatalf("Score() failed: %v", err)
	}

	if result2.Score != 0.0 {
		t.Errorf("Expected binary score 0.0, got %.2f", result2.Score)
	}
}

func TestTaskCompletionScorer_LLMJudge(t *testing.T) {
	mockJudge := &mockLLMProvider{
		responses: []*llm.CompletionResponse{
			{
				Content:      `{"score": 0.85, "reasoning": "Good but not perfect"}`,
				FinishReason: "stop",
			},
		},
	}

	scorer := NewTaskCompletionScorer(TaskCompletionOptions{
		Rubric: "The agent should identify SQL injection vulnerabilities",
		Judge:  mockJudge,
	})

	sample := Sample{
		ID: "test-6",
		Task: agent.Task{
			Context: map[string]any{"objective": "Test login form for SQL injection"},
		},
		Result: agent.Result{
			Output: "Found SQL injection in username field",
		},
	}

	result, err := scorer.Score(context.Background(), sample)
	if err != nil {
		t.Fatalf("Score() failed: %v", err)
	}

	if result.Score != 0.85 {
		t.Errorf("Expected score 0.85 from LLM judge, got %.2f", result.Score)
	}

	if result.Details["judge_reasoning"] != "Good but not perfect" {
		t.Errorf("Expected judge reasoning in details, got %v", result.Details["judge_reasoning"])
	}
}

func TestTaskCompletionScorer_LLMJudgeWithMarkdown(t *testing.T) {
	mockJudge := &mockLLMProvider{
		responses: []*llm.CompletionResponse{
			{
				Content:      "```json\n{\"score\": 0.75, \"reasoning\": \"Partial success\"}\n```",
				FinishReason: "stop",
			},
		},
	}

	scorer := NewTaskCompletionScorer(TaskCompletionOptions{
		Rubric: "The agent should identify all vulnerabilities",
		Judge:  mockJudge,
	})

	sample := Sample{
		ID: "test-7",
		Task: agent.Task{
			Context: map[string]any{"objective": "Comprehensive security test"},
		},
		Result: agent.Result{
			Output: "Found 2 out of 3 vulnerabilities",
		},
	}

	result, err := scorer.Score(context.Background(), sample)
	if err != nil {
		t.Fatalf("Score() failed: %v", err)
	}

	if result.Score != 0.75 {
		t.Errorf("Expected score 0.75, got %.2f", result.Score)
	}
}

func TestTaskCompletionScorer_CombinedMode(t *testing.T) {
	mockJudge := &mockLLMProvider{
		responses: []*llm.CompletionResponse{
			{
				Content:      `{"score": 0.8, "reasoning": "Good match"}`,
				FinishReason: "stop",
			},
		},
	}

	scorer := NewTaskCompletionScorer(TaskCompletionOptions{
		ExpectedOutput: "success",
		Rubric:         "Output should indicate success",
		Judge:          mockJudge,
	})

	sample := Sample{
		ID: "test-8",
		Task: agent.Task{
			Context: map[string]any{"objective": "Complete the task"},
		},
		Result: agent.Result{
			Output: "success",
		},
	}

	result, err := scorer.Score(context.Background(), sample)
	if err != nil {
		t.Fatalf("Score() failed: %v", err)
	}

	// Should average exact match (1.0) and LLM judge (0.8) = 0.9
	expected := 0.9
	if result.Score != expected {
		t.Errorf("Expected combined score %.2f, got %.2f", expected, result.Score)
	}

	// Both modes should be present in details
	if result.Details["comparison_mode"] != "exact" {
		t.Errorf("Expected comparison_mode in details")
	}
	if result.Details["judge_mode"] != "llm" {
		t.Errorf("Expected judge_mode in details")
	}
}

func TestTaskCompletionScorer_NoConfiguration(t *testing.T) {
	scorer := NewTaskCompletionScorer(TaskCompletionOptions{})

	sample := Sample{
		ID: "test-9",
		Task: agent.Task{
			Context: map[string]any{"objective": "Test task"},
		},
		Result: agent.Result{
			Output: "result",
		},
	}

	_, err := scorer.Score(context.Background(), sample)
	if err == nil {
		t.Fatal("Expected error when no evaluation mode is configured")
	}

	expectedErr := "no evaluation mode configured"
	if !contains(err.Error(), expectedErr) {
		t.Errorf("Expected error to contain %q, got %q", expectedErr, err.Error())
	}
}

func TestTaskCompletionScorer_InvalidJudgeScore(t *testing.T) {
	mockJudge := &mockLLMProvider{
		responses: []*llm.CompletionResponse{
			{
				Content:      `{"score": 1.5, "reasoning": "Invalid score"}`,
				FinishReason: "stop",
			},
		},
	}

	scorer := NewTaskCompletionScorer(TaskCompletionOptions{
		Rubric: "Test rubric",
		Judge:  mockJudge,
	})

	sample := Sample{
		ID: "test-10",
		Task: agent.Task{
			Context: map[string]any{"objective": "Test task"},
		},
		Result: agent.Result{
			Output: "result",
		},
	}

	_, err := scorer.Score(context.Background(), sample)
	if err == nil {
		t.Fatal("Expected error for invalid LLM judge score")
	}

	if !contains(err.Error(), "invalid score") {
		t.Errorf("Expected error about invalid score, got: %v", err)
	}
}

func TestTaskCompletionScorer_MalformedJudgeResponse(t *testing.T) {
	mockJudge := &mockLLMProvider{
		responses: []*llm.CompletionResponse{
			{
				Content:      `This is not valid JSON`,
				FinishReason: "stop",
			},
		},
	}

	scorer := NewTaskCompletionScorer(TaskCompletionOptions{
		Rubric: "Test rubric",
		Judge:  mockJudge,
	})

	sample := Sample{
		ID: "test-11",
		Task: agent.Task{
			Context: map[string]any{"objective": "Test task"},
		},
		Result: agent.Result{
			Output: "result",
		},
	}

	_, err := scorer.Score(context.Background(), sample)
	if err == nil {
		t.Fatal("Expected error for malformed LLM judge response")
	}

	if !contains(err.Error(), "parse") {
		t.Errorf("Expected error about parsing, got: %v", err)
	}
}

func TestTaskCompletionScorer_Name(t *testing.T) {
	scorer := NewTaskCompletionScorer(TaskCompletionOptions{
		ExpectedOutput: "test",
	})

	name := scorer.Name()
	if name != "task_completion" {
		t.Errorf("Expected name 'task_completion', got %q", name)
	}
}

func TestTaskCompletionScorer_StringSimilarity(t *testing.T) {
	scorer := &taskCompletionScorer{
		opts: TaskCompletionOptions{},
	}

	tests := []struct {
		name     string
		a        string
		b        string
		expected float64
	}{
		{
			name:     "identical strings",
			a:        "hello",
			b:        "hello",
			expected: 1.0,
		},
		{
			name:     "empty strings",
			a:        "",
			b:        "",
			expected: 1.0,
		},
		{
			name:     "one empty",
			a:        "hello",
			b:        "",
			expected: 0.0,
		},
		{
			name:     "completely different",
			a:        "abc",
			b:        "xyz",
			expected: 0.0,
		},
		{
			name:     "partial overlap",
			a:        "hello world",
			b:        "hello there",
			expected: 0.5, // Approximate
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			similarity := scorer.stringSimilarity(tt.a, tt.b)

			// For approximate tests, allow some tolerance
			if tt.name == "partial overlap" {
				if similarity < 0.4 || similarity > 0.6 {
					t.Errorf("Expected similarity around %.2f, got %.2f", tt.expected, similarity)
				}
			} else {
				if similarity != tt.expected {
					t.Errorf("Expected similarity %.2f, got %.2f", tt.expected, similarity)
				}
			}
		})
	}
}

// Helper function to check if a string contains a substring.
func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(substr) == 0 ||
		(len(s) > 0 && (s[0:1] == substr[0:1] && contains(s[1:], substr[1:])) ||
			(len(s) > 0 && contains(s[1:], substr))))
}

func TestBuildJudgePrompt(t *testing.T) {
	scorer := &taskCompletionScorer{
		opts: TaskCompletionOptions{
			Rubric: "Test rubric",
		},
	}

	sample := Sample{
		Task: agent.Task{
			Context: map[string]any{"objective": "Find vulnerabilities"},
		},
		Result: agent.Result{
			Output: "Found SQL injection",
		},
	}

	prompt := scorer.buildJudgePrompt(sample)

	// Verify prompt contains key elements
	if !jsonContains(prompt, "Find vulnerabilities") {
		t.Error("Prompt should contain task goal")
	}
	if !jsonContains(prompt, "Found SQL injection") {
		t.Error("Prompt should contain result output")
	}
	if !jsonContains(prompt, "Test rubric") {
		t.Error("Prompt should contain rubric")
	}
}

// Helper to check if string contains substring (simple implementation)
func jsonContains(s, substr string) bool {
	return len(s) > 0 && len(substr) > 0 &&
		(s == substr ||
			(len(s) >= len(substr) && s[:len(substr)] == substr) ||
			(len(s) > len(substr) && jsonContains(s[1:], substr)))
}

func TestTaskCompletionScorer_ComplexOutput(t *testing.T) {
	// Test with map output
	scorer := NewTaskCompletionScorer(TaskCompletionOptions{
		ExpectedOutput: map[string]any{
			"status": "success",
			"count":  3,
		},
	})

	sample := Sample{
		ID: "test-12",
		Task: agent.Task{
			Context: map[string]any{"objective": "Count vulnerabilities"},
		},
		Result: agent.Result{
			Output: map[string]any{
				"status": "success",
				"count":  3,
			},
		},
	}

	result, err := scorer.Score(context.Background(), sample)
	if err != nil {
		t.Fatalf("Score() failed: %v", err)
	}

	if result.Score != 1.0 {
		t.Errorf("Expected score 1.0 for exact map match, got %.2f", result.Score)
	}
}

func TestJudgeResponseParsing(t *testing.T) {
	tests := []struct {
		name      string
		content   string
		wantErr   bool
		wantScore float64
	}{
		{
			name:      "plain JSON",
			content:   `{"score": 0.8, "reasoning": "Good"}`,
			wantErr:   false,
			wantScore: 0.8,
		},
		{
			name:      "JSON in code block",
			content:   "```json\n{\"score\": 0.9, \"reasoning\": \"Great\"}\n```",
			wantErr:   false,
			wantScore: 0.9,
		},
		{
			name:      "JSON in generic code block",
			content:   "```\n{\"score\": 0.7, \"reasoning\": \"OK\"}\n```",
			wantErr:   false,
			wantScore: 0.7,
		},
		{
			name:    "invalid JSON",
			content: "not json",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockJudge := &mockLLMProvider{
				responses: []*llm.CompletionResponse{
					{
						Content:      tt.content,
						FinishReason: "stop",
					},
				},
			}

			scorer := NewTaskCompletionScorer(TaskCompletionOptions{
				Rubric: "Test",
				Judge:  mockJudge,
			})

			sample := Sample{
				Task:   agent.Task{Context: map[string]any{"objective": "Test"}},
				Result: agent.Result{Output: "result"},
			}

			result, err := scorer.Score(context.Background(), sample)

			if tt.wantErr {
				if err == nil {
					t.Error("Expected error but got none")
				}
			} else {
				if err != nil {
					t.Errorf("Unexpected error: %v", err)
				}
				if result.Score != tt.wantScore {
					t.Errorf("Expected score %.2f, got %.2f", tt.wantScore, result.Score)
				}
			}
		})
	}
}
