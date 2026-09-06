// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Zero Root AI

package eval

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/zeroroot-ai/sdk/agent"
	"github.com/zeroroot-ai/sdk/llm"
)

func TestNewLLMJudgeScorer(t *testing.T) {
	tests := []struct {
		name        string
		opts        LLMJudgeOptions
		expectError bool
		errorMsg    string
	}{
		{
			name: "valid options",
			opts: LLMJudgeOptions{
				Provider: &mockLLMProvider{},
				Rubric:   "Test rubric",
			},
			expectError: false,
		},
		{
			name: "missing provider",
			opts: LLMJudgeOptions{
				Rubric: "Test rubric",
			},
			expectError: true,
			errorMsg:    "Provider is required",
		},
		{
			name: "missing rubric",
			opts: LLMJudgeOptions{
				Provider: &mockLLMProvider{},
			},
			expectError: true,
			errorMsg:    "Rubric is required",
		},
		{
			name: "custom system prompt",
			opts: LLMJudgeOptions{
				Provider:     &mockLLMProvider{},
				Rubric:       "Test rubric",
				SystemPrompt: "Custom prompt",
			},
			expectError: false,
		},
		{
			name: "custom max retries",
			opts: LLMJudgeOptions{
				Provider:   &mockLLMProvider{},
				Rubric:     "Test rubric",
				MaxRetries: 5,
			},
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			scorer, err := NewLLMJudgeScorer(tt.opts)

			if tt.expectError {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.errorMsg)
				assert.Nil(t, scorer)
			} else {
				require.NoError(t, err)
				assert.NotNil(t, scorer)
				assert.Equal(t, "llm_judge", scorer.Name())
			}
		})
	}
}

func TestLLMJudgeScorer_Score_Success(t *testing.T) {
	// Create a mock provider that returns a valid score
	provider := &mockLLMProvider{
		responses: []*llm.CompletionResponse{
			{
				Content: `{"score": 0.85, "reasoning": "The agent successfully completed the task with minor issues."}`,
				Usage: llm.TokenUsage{
					InputTokens:  100,
					OutputTokens: 50,
					TotalTokens:  150,
				},
			},
		},
	}

	tokenTracker := &TokenUsage{}

	scorer, err := NewLLMJudgeScorer(LLMJudgeOptions{
		Provider:     provider,
		Rubric:       "Score based on task completion",
		TokenTracker: tokenTracker,
	})
	require.NoError(t, err)

	sample := Sample{
		ID: "test-1",
		Task: agent.Task{
			ID:      "task-1",
			Context: map[string]any{"objective": "Test task"},
		},
		Result: agent.Result{
			Output: map[string]any{"result": "success"},
		},
	}

	result, err := scorer.Score(context.Background(), sample)
	require.NoError(t, err)

	assert.Equal(t, 0.85, result.Score)
	assert.NotNil(t, result.Details)
	assert.Equal(t, "The agent successfully completed the task with minor issues.", result.Details["reasoning"])
	assert.Equal(t, 150, result.Details["tokens_used"])
	assert.Equal(t, 100, result.Details["input_tokens"])
	assert.Equal(t, 50, result.Details["output_tokens"])

	// Verify token tracker was updated
	assert.Equal(t, 100, tokenTracker.InputTokens)
	assert.Equal(t, 50, tokenTracker.OutputTokens)
	assert.Equal(t, 150, tokenTracker.Total())
}

func TestLLMJudgeScorer_Score_WithMarkdownJSON(t *testing.T) {
	// Test that the scorer can handle JSON wrapped in markdown code blocks
	provider := &mockLLMProvider{
		responses: []*llm.CompletionResponse{
			{
				Content: "```json\n{\"score\": 0.75, \"reasoning\": \"Good performance with room for improvement.\"}\n```",
				Usage: llm.TokenUsage{
					InputTokens:  100,
					OutputTokens: 50,
					TotalTokens:  150,
				},
			},
		},
	}

	scorer, err := NewLLMJudgeScorer(LLMJudgeOptions{
		Provider: provider,
		Rubric:   "Test rubric",
	})
	require.NoError(t, err)

	sample := Sample{
		ID: "test-2",
		Task: agent.Task{
			ID: "task-2",
		},
		Result: agent.Result{
			Output: "test output",
		},
	}

	result, err := scorer.Score(context.Background(), sample)
	require.NoError(t, err)

	assert.Equal(t, 0.75, result.Score)
	assert.Equal(t, "Good performance with room for improvement.", result.Details["reasoning"])
}

func TestLLMJudgeScorer_Score_WithTrajectory(t *testing.T) {
	provider := &mockLLMProvider{
		responses: []*llm.CompletionResponse{
			{
				Content: `{"score": 0.9, "reasoning": "Excellent execution with proper tool usage."}`,
				Usage: llm.TokenUsage{
					InputTokens:  200,
					OutputTokens: 75,
					TotalTokens:  275,
				},
			},
		},
	}

	scorer, err := NewLLMJudgeScorer(LLMJudgeOptions{
		Provider:          provider,
		Rubric:            "Evaluate tool usage",
		IncludeTrajectory: true,
	})
	require.NoError(t, err)

	sample := Sample{
		ID: "test-3",
		Task: agent.Task{
			ID:      "task-3",
			Context: map[string]any{"objective": "Test with trajectory"},
		},
		Result: agent.Result{
			Output: "success",
		},
		Trajectory: Trajectory{
			Steps: []TrajectoryStep{
				{
					Type: "tool",
					Name: "http-client",
					Input: map[string]any{
						"url": "https://example.com",
					},
					Duration: 100 * time.Millisecond,
				},
				{
					Type: "llm",
					Name: "primary",
					Input: []llm.Message{
						{Role: llm.RoleUser, Content: "Analyze response"},
					},
					Duration: 200 * time.Millisecond,
				},
			},
		},
	}

	result, err := scorer.Score(context.Background(), sample)
	require.NoError(t, err)

	assert.Equal(t, 0.9, result.Score)

	// Verify that trajectory was included in the prompt
	require.NotEmpty(t, provider.recordedCalls)
	lastCall := provider.recordedCalls[len(provider.recordedCalls)-1]

	// Find the user message
	var userMsg string
	for _, msg := range lastCall {
		if msg.Role == llm.RoleUser {
			userMsg = msg.Content
			break
		}
	}

	assert.Contains(t, userMsg, "Trajectory")
	assert.Contains(t, userMsg, "http-client")
	assert.Contains(t, userMsg, "llm")
}

func TestLLMJudgeScorer_Score_Retry(t *testing.T) {
	// First response is invalid JSON, second is valid
	provider := &mockLLMProvider{
		responses: []*llm.CompletionResponse{
			{
				Content: "This is not valid JSON",
				Usage: llm.TokenUsage{
					InputTokens:  100,
					OutputTokens: 20,
					TotalTokens:  120,
				},
			},
			{
				Content: `{"score": 0.6, "reasoning": "Corrected response after retry."}`,
				Usage: llm.TokenUsage{
					InputTokens:  150,
					OutputTokens: 40,
					TotalTokens:  190,
				},
			},
		},
	}

	tokenTracker := &TokenUsage{}

	scorer, err := NewLLMJudgeScorer(LLMJudgeOptions{
		Provider:     provider,
		Rubric:       "Test rubric",
		MaxRetries:   3,
		TokenTracker: tokenTracker,
	})
	require.NoError(t, err)

	sample := Sample{
		ID: "test-4",
		Task: agent.Task{
			ID: "task-4",
		},
		Result: agent.Result{
			Output: "test",
		},
	}

	result, err := scorer.Score(context.Background(), sample)
	require.NoError(t, err)

	assert.Equal(t, 0.6, result.Score)
	assert.Equal(t, 1, result.Details["retries"])

	// Verify total token usage (both attempts)
	assert.Equal(t, 310, tokenTracker.Total()) // 120 + 190
}

func TestLLMJudgeScorer_Score_ExhaustedRetries(t *testing.T) {
	// All responses are invalid
	provider := &mockLLMProvider{
		responses: []*llm.CompletionResponse{
			{Content: "invalid1", Usage: llm.TokenUsage{InputTokens: 10, OutputTokens: 5, TotalTokens: 15}},
			{Content: "invalid2", Usage: llm.TokenUsage{InputTokens: 10, OutputTokens: 5, TotalTokens: 15}},
			{Content: "invalid3", Usage: llm.TokenUsage{InputTokens: 10, OutputTokens: 5, TotalTokens: 15}},
			{Content: "invalid4", Usage: llm.TokenUsage{InputTokens: 10, OutputTokens: 5, TotalTokens: 15}},
		},
	}

	scorer, err := NewLLMJudgeScorer(LLMJudgeOptions{
		Provider:   provider,
		Rubric:     "Test rubric",
		MaxRetries: 3,
	})
	require.NoError(t, err)

	sample := Sample{
		ID: "test-5",
		Task: agent.Task{
			ID: "task-5",
		},
		Result: agent.Result{
			Output: "test",
		},
	}

	_, err = scorer.Score(context.Background(), sample)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed after")
	assert.Contains(t, err.Error(), "attempts")
}

func TestLLMJudgeScorer_Score_LLMError(t *testing.T) {
	provider := &mockLLMProvider{
		shouldError: true,
	}

	scorer, err := NewLLMJudgeScorer(LLMJudgeOptions{
		Provider:   provider,
		Rubric:     "Test rubric",
		MaxRetries: 2,
	})
	require.NoError(t, err)

	sample := Sample{
		ID: "test-6",
		Task: agent.Task{
			ID: "task-6",
		},
		Result: agent.Result{
			Output: "test",
		},
	}

	_, err = scorer.Score(context.Background(), sample)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "mock LLM error")
}

func TestLLMJudgeScorer_Score_InvalidScore(t *testing.T) {
	tests := []struct {
		name     string
		response string
	}{
		{
			name:     "score too high",
			response: `{"score": 1.5, "reasoning": "Invalid score > 1.0"}`,
		},
		{
			name:     "score too low",
			response: `{"score": -0.1, "reasoning": "Invalid score < 0.0"}`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Provide multiple responses since retry logic will keep trying
			provider := &mockLLMProvider{
				responses: []*llm.CompletionResponse{
					{
						Content: tt.response,
						Usage:   llm.TokenUsage{InputTokens: 10, OutputTokens: 5, TotalTokens: 15},
					},
					{
						Content: tt.response,
						Usage:   llm.TokenUsage{InputTokens: 10, OutputTokens: 5, TotalTokens: 15},
					},
					{
						Content: tt.response,
						Usage:   llm.TokenUsage{InputTokens: 10, OutputTokens: 5, TotalTokens: 15},
					},
					{
						Content: tt.response,
						Usage:   llm.TokenUsage{InputTokens: 10, OutputTokens: 5, TotalTokens: 15},
					},
				},
			}

			scorer, err := NewLLMJudgeScorer(LLMJudgeOptions{
				Provider:   provider,
				Rubric:     "Test rubric",
				MaxRetries: 3,
			})
			require.NoError(t, err)

			sample := Sample{
				ID: "test-invalid",
				Task: agent.Task{
					ID: "task-invalid",
				},
				Result: agent.Result{
					Output: "test",
				},
			}

			_, err = scorer.Score(context.Background(), sample)
			require.Error(t, err)
			assert.Contains(t, err.Error(), "invalid score")
		})
	}
}

func TestLLMJudgeScorer_Score_ContextCancellation(t *testing.T) {
	// Provider that takes too long
	provider := &mockLLMProvider{
		responses: []*llm.CompletionResponse{
			{Content: "invalid", Usage: llm.TokenUsage{InputTokens: 10, OutputTokens: 5, TotalTokens: 15}},
		},
	}

	scorer, err := NewLLMJudgeScorer(LLMJudgeOptions{
		Provider:   provider,
		Rubric:     "Test rubric",
		MaxRetries: 10, // Many retries so we can cancel during retry
	})
	require.NoError(t, err)

	sample := Sample{
		ID: "test-cancel",
		Task: agent.Task{
			ID: "task-cancel",
		},
		Result: agent.Result{
			Output: "test",
		},
	}

	ctx, cancel := context.WithCancel(context.Background())

	// Cancel after first attempt
	go func() {
		time.Sleep(50 * time.Millisecond)
		cancel()
	}()

	_, err = scorer.Score(ctx, sample)
	require.Error(t, err)
	assert.Equal(t, context.Canceled, err)
}

func TestTokenUsage_Add(t *testing.T) {
	tracker := &TokenUsage{}

	tracker.Add(llm.TokenUsage{
		InputTokens:  100,
		OutputTokens: 50,
		TotalTokens:  150,
	})

	assert.Equal(t, 100, tracker.InputTokens)
	assert.Equal(t, 50, tracker.OutputTokens)

	tracker.Add(llm.TokenUsage{
		InputTokens:  200,
		OutputTokens: 75,
		TotalTokens:  275,
	})

	assert.Equal(t, 300, tracker.InputTokens)
	assert.Equal(t, 125, tracker.OutputTokens)
	assert.Equal(t, 425, tracker.Total())
}

func TestParseJudgeResponse_EdgeCases(t *testing.T) {
	scorer := &llmJudgeScorer{}

	tests := []struct {
		name        string
		content     string
		expectError bool
		expectScore float64
	}{
		{
			name:        "clean JSON",
			content:     `{"score": 0.8, "reasoning": "Good"}`,
			expectError: false,
			expectScore: 0.8,
		},
		{
			name:        "JSON with markdown",
			content:     "```json\n{\"score\": 0.7, \"reasoning\": \"OK\"}\n```",
			expectError: false,
			expectScore: 0.7,
		},
		{
			name:        "JSON with extra text before",
			content:     "Here's my evaluation:\n{\"score\": 0.9, \"reasoning\": \"Great\"}",
			expectError: false,
			expectScore: 0.9,
		},
		{
			name:        "JSON with extra text after",
			content:     `{"score": 0.6, "reasoning": "Acceptable"} - This is my final assessment.`,
			expectError: false,
			expectScore: 0.6,
		},
		{
			name:        "missing reasoning",
			content:     `{"score": 0.5}`,
			expectError: true,
		},
		{
			name:        "no JSON",
			content:     "This is just text without any JSON",
			expectError: true,
		},
		{
			name:        "malformed JSON",
			content:     `{"score": 0.5, "reasoning": "Missing closing brace"`,
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			score, reasoning, err := scorer.parseJudgeResponse(tt.content)

			if tt.expectError {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
				assert.Equal(t, tt.expectScore, score)
				assert.NotEmpty(t, reasoning)
			}
		})
	}
}

func TestLLMJudgeScorer_BuildPrompt(t *testing.T) {
	scorer := &llmJudgeScorer{
		rubric:            "Test rubric for evaluation",
		includeTrajectory: true,
	}

	sample := Sample{
		ID: "test-prompt",
		Task: agent.Task{
			ID:      "task-prompt",
			Context: map[string]any{"objective": "Test task for prompt building"},
		},
		Result: agent.Result{
			Output: map[string]any{
				"findings": []string{"finding1", "finding2"},
				"status":   "success",
			},
		},
		Trajectory: Trajectory{
			Steps: []TrajectoryStep{
				{Type: "tool", Name: "mytool"},
				{Type: "llm", Name: "primary"},
			},
		},
	}

	prompt := scorer.buildEvaluationPrompt(sample)

	// Verify all components are included
	assert.Contains(t, prompt, "Task:")
	assert.Contains(t, prompt, "Test task for prompt building")
	assert.Contains(t, prompt, "Agent Output:")
	assert.Contains(t, prompt, "findings")
	assert.Contains(t, prompt, "Trajectory")
	assert.Contains(t, prompt, "mytool")
	assert.Contains(t, prompt, "Evaluation Rubric:")
	assert.Contains(t, prompt, "Test rubric for evaluation")
	assert.Contains(t, prompt, "Respond with valid JSON")
}

func TestLLMJudgeScorer_CustomTemperature(t *testing.T) {
	provider := &mockLLMProvider{
		responses: []*llm.CompletionResponse{
			{
				Content: `{"score": 0.8, "reasoning": "Test"}`,
				Usage:   llm.TokenUsage{InputTokens: 10, OutputTokens: 5, TotalTokens: 15},
			},
		},
	}

	scorer, err := NewLLMJudgeScorer(LLMJudgeOptions{
		Provider:    provider,
		Rubric:      "Test",
		Temperature: 0.7,
	})
	require.NoError(t, err)

	sample := Sample{
		ID:   "test-temp",
		Task: agent.Task{ID: "task-temp"},
		Result: agent.Result{
			Output: "test",
		},
	}

	_, err = scorer.Score(context.Background(), sample)
	require.NoError(t, err)

	// The temperature is passed via options, so we can't directly verify it was used
	// but we can ensure the scorer was created with it
	typedScorer, ok := scorer.(*llmJudgeScorer)
	require.True(t, ok)
	assert.Equal(t, 0.7, typedScorer.temperature)
}

// Benchmark tests
func BenchmarkLLMJudgeScorer_Score(b *testing.B) {
	provider := &mockLLMProvider{
		responses: []*llm.CompletionResponse{
			{
				Content: `{"score": 0.85, "reasoning": "Benchmark test evaluation"}`,
				Usage:   llm.TokenUsage{InputTokens: 100, OutputTokens: 50, TotalTokens: 150},
			},
		},
	}

	scorer, err := NewLLMJudgeScorer(LLMJudgeOptions{
		Provider: provider,
		Rubric:   "Benchmark rubric",
	})
	require.NoError(b, err)

	sample := Sample{
		ID: "bench-1",
		Task: agent.Task{
			ID:      "task-bench",
			Context: map[string]any{"objective": "Benchmark task"},
		},
		Result: agent.Result{
			Output: "test output",
		},
	}

	b.ResetTimer()
	for range b.N {
		provider.callCount = 0 // Reset for each iteration
		_, _ = scorer.Score(context.Background(), sample)
	}
}

// Example usage
func ExampleNewLLMJudgeScorer() {
	// Create a mock provider (in real usage, use an actual LLM provider)
	provider := &mockLLMProvider{
		responses: []*llm.CompletionResponse{
			{
				Content: `{"score": 0.9, "reasoning": "Excellent task completion with comprehensive findings."}`,
				Usage:   llm.TokenUsage{InputTokens: 150, OutputTokens: 75, TotalTokens: 225},
			},
		},
	}

	// Create scorer with evaluation rubric
	scorer, err := NewLLMJudgeScorer(LLMJudgeOptions{
		Provider: provider,
		Rubric: `Score the agent based on:
- Task completion (0.4): Did it fully complete the assigned task?
- Finding quality (0.3): Are the findings accurate and actionable?
- Efficiency (0.3): Did it minimize unnecessary steps?`,
		MaxRetries: 3,
	})
	if err != nil {
		fmt.Printf("failed to create LLM judge scorer: %v\n", err)
		return
	}

	// Evaluate a sample
	sample := Sample{
		ID: "example-1",
		Task: agent.Task{
			ID:      "sql-injection-test",
			Context: map[string]any{"objective": "Test the login form for SQL injection vulnerabilities"},
		},
		Result: agent.Result{
			Output: map[string]any{
				"vulnerabilities_found": 2,
				"payloads_tested":       15,
			},
		},
	}

	result, err := scorer.Score(context.Background(), sample)
	if err != nil {
		fmt.Printf("failed to score sample: %v\n", err)
		return
	}

	fmt.Printf("Score: %.2f\n", result.Score)
	fmt.Printf("Reasoning: %s\n", result.Details["reasoning"])
	// Output:
	// Score: 0.90
	// Reasoning: Excellent task completion with comprehensive findings.
}
