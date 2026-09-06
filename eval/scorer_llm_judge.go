// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Zero Root AI

package eval

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"strings"
	"time"

	"github.com/zeroroot-ai/sdk/llm"
)

// LLMProvider defines the minimal interface needed for LLM-based evaluation.
// This can be implemented by wrapping a harness or directly by LLM client libraries.
type LLMProvider interface {
	// Complete performs a single LLM completion request.
	// Returns the response or an error if the request fails.
	Complete(ctx context.Context, messages []llm.Message, opts ...llm.CompletionOption) (*llm.CompletionResponse, error)
}

// TokenUsage tracks token consumption for cost analysis.
type TokenUsage struct {
	// InputTokens is the cumulative number of tokens in all input/prompts.
	InputTokens int `json:"input_tokens" yaml:"input_tokens"`

	// OutputTokens is the cumulative number of tokens generated in all responses.
	OutputTokens int `json:"output_tokens" yaml:"output_tokens"`
}

// Add accumulates token usage from another TokenUsage instance.
func (t *TokenUsage) Add(usage llm.TokenUsage) {
	t.InputTokens += usage.InputTokens
	t.OutputTokens += usage.OutputTokens
}

// Total returns the sum of input and output tokens.
func (t *TokenUsage) Total() int {
	return t.InputTokens + t.OutputTokens
}

// LLMJudgeOptions configures an LLM-as-Judge scorer.
type LLMJudgeOptions struct {
	// Provider is the LLM to use for judging (required).
	Provider LLMProvider

	// Rubric defines the evaluation criteria (required).
	// This should be a clear description of what constitutes good vs bad performance.
	// Example: "The agent should discover all SQL injection vulnerabilities and provide
	// working exploit payloads. Score 1.0 for complete success, 0.0 for no findings."
	Rubric string

	// SystemPrompt is an optional custom system prompt.
	// If empty, a default prompt instructing JSON output will be used.
	SystemPrompt string

	// MaxRetries is the number of times to retry on JSON parse failures (default: 3).
	MaxRetries int

	// TokenTracker optionally tracks cumulative token usage across all evaluations.
	// Useful for cost analysis and budget management.
	TokenTracker *TokenUsage

	// Temperature controls randomness in LLM judgments (default: 0.0 for deterministic).
	Temperature float64

	// IncludeTrajectory controls whether to include full trajectory details in the prompt.
	// If false, only a summary is included (default: true).
	IncludeTrajectory bool
}

// llmJudgeScorer implements the Scorer interface using an LLM as a judge.
type llmJudgeScorer struct {
	provider          LLMProvider
	rubric            string
	systemPrompt      string
	maxRetries        int
	tokenTracker      *TokenUsage
	temperature       float64
	includeTrajectory bool
}

// judgeResponse represents the expected JSON response from the LLM judge.
type judgeResponse struct {
	Score     float64 `json:"score"`
	Reasoning string  `json:"reasoning"`
}

// NewLLMJudgeScorer creates a new LLM-as-Judge scorer with the given options.
// Returns an error if Provider or Rubric is not provided.
func NewLLMJudgeScorer(opts LLMJudgeOptions) (Scorer, error) {
	if opts.Provider == nil {
		return nil, errors.New("LLMJudgeOptions.Provider is required")
	}

	if opts.Rubric == "" {
		return nil, errors.New("LLMJudgeOptions.Rubric is required")
	}

	// Set defaults
	maxRetries := opts.MaxRetries
	if maxRetries == 0 {
		maxRetries = 3
	}

	systemPrompt := opts.SystemPrompt
	if systemPrompt == "" {
		systemPrompt = defaultSystemPrompt
	}

	includeTrajectory := opts.IncludeTrajectory
	// Default to true if not explicitly set
	// Note: zero value for bool is false, so we need to check if it was explicitly set
	// For simplicity, we'll default to true
	if !includeTrajectory {
		includeTrajectory = true
	}

	return &llmJudgeScorer{
		provider:          opts.Provider,
		rubric:            opts.Rubric,
		systemPrompt:      systemPrompt,
		maxRetries:        maxRetries,
		tokenTracker:      opts.TokenTracker,
		temperature:       opts.Temperature,
		includeTrajectory: includeTrajectory,
	}, nil
}

// defaultSystemPrompt is used when no custom system prompt is provided.
const defaultSystemPrompt = `You are an expert evaluation judge for AI agent performance. Your task is to assess the agent's execution based on the provided rubric.

You must respond with valid JSON in the following format:
{"score": <float between 0.0 and 1.0>, "reasoning": "<detailed explanation>"}

Guidelines:
- Score 1.0 indicates perfect performance according to the rubric
- Score 0.0 indicates complete failure
- Provide detailed reasoning explaining your score
- Be objective and consistent in your evaluations
- Consider both successes and failures in your assessment`

// Name returns the scorer's identifier.
func (s *llmJudgeScorer) Name() string {
	return "llm_judge"
}

// Score evaluates the sample using the LLM as a judge.
func (s *llmJudgeScorer) Score(ctx context.Context, sample Sample) (ScoreResult, error) {
	// Build the evaluation prompt
	userPrompt := s.buildEvaluationPrompt(sample)

	messages := []llm.Message{
		{Role: llm.RoleSystem, Content: s.systemPrompt},
		{Role: llm.RoleUser, Content: userPrompt},
	}

	// Attempt to get a valid score with retries
	var lastErr error
	var totalTokens llm.TokenUsage

	for attempt := 0; attempt <= s.maxRetries; attempt++ {
		// Call the LLM
		resp, err := s.provider.Complete(ctx, messages, llm.WithTemperature(s.temperature))
		if err != nil {
			lastErr = fmt.Errorf("LLM completion failed (attempt %d/%d): %w", attempt+1, s.maxRetries+1, err)

			// Exponential backoff before retry
			if attempt < s.maxRetries {
				backoff := time.Duration(math.Pow(2, float64(attempt))) * 100 * time.Millisecond
				select {
				case <-time.After(backoff):
					continue
				case <-ctx.Done():
					return ScoreResult{}, ctx.Err()
				}
			}
			continue
		}

		// Track token usage
		totalTokens = totalTokens.Add(resp.Usage)
		if s.tokenTracker != nil {
			s.tokenTracker.Add(resp.Usage)
		}

		// Parse the JSON response
		score, reasoning, err := s.parseJudgeResponse(resp.Content)
		if err != nil {
			lastErr = fmt.Errorf("failed to parse LLM response (attempt %d/%d): %w", attempt+1, s.maxRetries+1, err)

			// Add feedback to help the LLM correct its response
			if attempt < s.maxRetries {
				messages = append(messages, llm.Message{
					Role:    llm.RoleAssistant,
					Content: resp.Content,
				})
				messages = append(messages, llm.Message{
					Role:    llm.RoleUser,
					Content: fmt.Sprintf("Invalid JSON format. Error: %v\nPlease respond with valid JSON: {\"score\": <0.0-1.0>, \"reasoning\": \"<explanation>\"}", err),
				})

				// Exponential backoff
				backoff := time.Duration(math.Pow(2, float64(attempt))) * 100 * time.Millisecond
				select {
				case <-time.After(backoff):
					continue
				case <-ctx.Done():
					return ScoreResult{}, ctx.Err()
				}
			}
			continue
		}

		// Validate the score
		if err := ValidateScore(score); err != nil {
			lastErr = fmt.Errorf("invalid score from LLM judge: %w", err)
			continue
		}

		// Success - return the result
		details := map[string]any{
			"reasoning":     reasoning,
			"tokens_used":   totalTokens.TotalTokens,
			"input_tokens":  totalTokens.InputTokens,
			"output_tokens": totalTokens.OutputTokens,
		}

		if attempt > 0 {
			details["retries"] = attempt
		}

		return ScoreResult{
			Score:   score,
			Details: details,
		}, nil
	}

	// All retries exhausted
	return ScoreResult{}, fmt.Errorf("LLM judge scoring failed after %d attempts: %w", s.maxRetries+1, lastErr)
}

// buildEvaluationPrompt constructs the prompt for the LLM judge.
func (s *llmJudgeScorer) buildEvaluationPrompt(sample Sample) string {
	var sb strings.Builder

	// Task description
	sb.WriteString("Task:\n")
	if objective, ok := sample.Task.Context["objective"]; ok {
		sb.WriteString(fmt.Sprintf("%v", objective))
	} else {
		sb.WriteString("ID: " + sample.Task.ID)
	}

	// Add task context if available
	if len(sample.Task.Context) > 0 {
		if ctxJSON, err := json.MarshalIndent(sample.Task.Context, "", "  "); err == nil {
			sb.WriteString("\nContext: ")
			sb.Write(ctxJSON)
		}
	}
	sb.WriteString("\n\n")

	// Agent output
	sb.WriteString("Agent Output:\n")
	if sample.Result.Output != nil {
		// Try to format the output nicely
		if outputJSON, err := json.MarshalIndent(sample.Result.Output, "", "  "); err == nil {
			sb.Write(outputJSON)
		} else {
			sb.WriteString(fmt.Sprintf("%v", sample.Result.Output))
		}
	} else if sample.Result.Error != nil {
		sb.WriteString("ERROR: " + sample.Result.Error.Error())
	} else {
		sb.WriteString("(no output)")
	}
	sb.WriteString("\n\n")

	// Trajectory summary
	if s.includeTrajectory && len(sample.Trajectory.Steps) > 0 {
		sb.WriteString("Trajectory (steps taken by agent):\n")
		for i, step := range sample.Trajectory.Steps {
			sb.WriteString(fmt.Sprintf("%d. %s: %s", i+1, step.Type, step.Name))
			if step.Error != "" {
				sb.WriteString(fmt.Sprintf(" [ERROR: %s]", step.Error))
			}
			sb.WriteString("\n")
		}
		sb.WriteString("\n")
	}

	// Evaluation rubric
	sb.WriteString("Evaluation Rubric:\n")
	sb.WriteString(s.rubric)
	sb.WriteString("\n\n")

	// Final instruction
	sb.WriteString("Respond with valid JSON: {\"score\": <0.0-1.0>, \"reasoning\": \"<explanation>\"}")

	return sb.String()
}

// parseJudgeResponse extracts the score and reasoning from the LLM's response.
func (s *llmJudgeScorer) parseJudgeResponse(content string) (float64, string, error) {
	// Clean up the content - sometimes LLMs wrap JSON in markdown code blocks
	content = strings.TrimSpace(content)

	// Remove markdown code blocks if present
	if strings.HasPrefix(content, "```json") {
		content = strings.TrimPrefix(content, "```json")
		content = strings.TrimSuffix(content, "```")
		content = strings.TrimSpace(content)
	} else if strings.HasPrefix(content, "```") {
		content = strings.TrimPrefix(content, "```")
		content = strings.TrimSuffix(content, "```")
		content = strings.TrimSpace(content)
	}

	// Try to find JSON object if there's extra text
	startIdx := strings.Index(content, "{")
	endIdx := strings.LastIndex(content, "}")

	if startIdx == -1 || endIdx == -1 || endIdx < startIdx {
		return 0, "", fmt.Errorf("no JSON object found in response: %s", content)
	}

	jsonStr := content[startIdx : endIdx+1]

	// Parse the JSON
	var response judgeResponse
	if err := json.Unmarshal([]byte(jsonStr), &response); err != nil {
		return 0, "", fmt.Errorf("failed to unmarshal JSON: %w (content: %s)", err, jsonStr)
	}

	// Validate that we got the required fields
	if response.Reasoning == "" {
		return 0, "", errors.New("missing 'reasoning' field in response")
	}

	return response.Score, response.Reasoning, nil
}
