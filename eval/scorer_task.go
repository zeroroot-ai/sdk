// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Zero Root AI

package eval

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"strings"

	"github.com/zeroroot-ai/sdk/llm"
)

// TaskCompletionOptions configures the task completion scorer.
// The scorer can operate in multiple modes:
// - Exact/fuzzy comparison against ExpectedOutput
// - LLM-as-judge evaluation using a rubric
// - Binary pass/fail evaluation
type TaskCompletionOptions struct {
	// ExpectedOutput is the expected task output for comparison.
	// If set, the scorer will compare the sample's Result against this value.
	// Comparison can be exact (deep equality) or fuzzy (for strings).
	ExpectedOutput any

	// Rubric contains evaluation criteria for LLM-as-judge scoring.
	// This should describe what constitutes success for the task.
	// Only used when Judge is also set.
	Rubric string

	// Judge is the LLM provider to use for LLM-as-judge evaluation.
	// If set along with Rubric, the scorer will use the LLM to evaluate task completion.
	// If nil, only ExpectedOutput comparison will be used.
	Judge LLMProvider

	// Binary determines whether to round scores to 0 or 1.
	// When true, scores >= 0.5 become 1.0, and scores < 0.5 become 0.0.
	Binary bool

	// FuzzyThreshold controls fuzzy string matching sensitivity (0.0 to 1.0).
	// Only used for string comparisons. Default: 0.8 (80% similarity required).
	FuzzyThreshold float64
}

// taskCompletionScorer evaluates whether an agent successfully completed its task.
type taskCompletionScorer struct {
	opts TaskCompletionOptions
}

// NewTaskCompletionScorer creates a scorer that evaluates task completion.
//
// The scorer supports multiple evaluation modes:
//  1. Exact match: If ExpectedOutput is set, compares Result against it
//  2. LLM-as-judge: If Rubric and Judge are set, uses LLM to evaluate quality
//  3. Combined: Can use both methods and average the scores
//
// Example (exact match):
//
//	scorer := NewTaskCompletionScorer(TaskCompletionOptions{
//	    ExpectedOutput: map[string]any{"status": "success"},
//	    Binary: true,
//	})
//
// Example (LLM-as-judge):
//
//	scorer := NewTaskCompletionScorer(TaskCompletionOptions{
//	    Rubric: "The agent should identify SQL injection vulnerabilities",
//	    Judge: myLLMProvider,
//	})
//
// Example (combined):
//
//	scorer := NewTaskCompletionScorer(TaskCompletionOptions{
//	    ExpectedOutput: "vulnerability found",
//	    Rubric: "Output should describe the vulnerability clearly",
//	    Judge: myLLMProvider,
//	})
func NewTaskCompletionScorer(opts TaskCompletionOptions) Scorer {
	// Set default fuzzy threshold if not specified
	if opts.FuzzyThreshold == 0.0 {
		opts.FuzzyThreshold = 0.8
	}

	return &taskCompletionScorer{
		opts: opts,
	}
}

// Name returns the scorer identifier.
func (s *taskCompletionScorer) Name() string {
	return "task_completion"
}

// Score evaluates whether the agent completed the task successfully.
func (s *taskCompletionScorer) Score(ctx context.Context, sample Sample) (ScoreResult, error) {
	var scores []float64
	details := make(map[string]any)

	// Mode 1: Compare against ExpectedOutput
	if s.opts.ExpectedOutput != nil {
		// Extract the Output field from the Result for comparison
		actualOutput := sample.Result.Output
		score, compDetails, err := s.compareOutput(actualOutput, s.opts.ExpectedOutput)
		if err != nil {
			return ScoreResult{}, fmt.Errorf("output comparison failed: %w", err)
		}
		scores = append(scores, score)
		details["comparison_mode"] = "exact"
		for k, v := range compDetails {
			details[k] = v
		}
	}

	// Mode 2: LLM-as-judge evaluation
	if s.opts.Rubric != "" && s.opts.Judge != nil {
		score, judgeDetails, err := s.llmJudge(ctx, sample)
		if err != nil {
			return ScoreResult{}, fmt.Errorf("LLM-as-judge evaluation failed: %w", err)
		}
		scores = append(scores, score)
		details["judge_mode"] = "llm"
		for k, v := range judgeDetails {
			details[k] = v
		}
	}

	// If neither mode is configured, return an error
	if len(scores) == 0 {
		return ScoreResult{}, errors.New("no evaluation mode configured: set ExpectedOutput or both Rubric and Judge")
	}

	// Calculate average score across all modes
	var finalScore float64
	for _, score := range scores {
		finalScore += score
	}
	finalScore /= float64(len(scores))

	// Apply binary threshold if requested
	if s.opts.Binary {
		if finalScore >= 0.5 {
			finalScore = 1.0
		} else {
			finalScore = 0.0
		}
		details["binary"] = true
	}

	// Validate the final score
	if err := ValidateScore(finalScore); err != nil {
		return ScoreResult{}, fmt.Errorf("score validation failed: %w", err)
	}

	return ScoreResult{
		Score:   finalScore,
		Details: details,
	}, nil
}

// compareOutput compares the actual result against the expected output.
// Returns a score in [0.0, 1.0] and details about the comparison.
func (s *taskCompletionScorer) compareOutput(actual, expected any) (float64, map[string]any, error) {
	details := make(map[string]any)

	// Convert both to comparable formats
	actualStr := fmt.Sprintf("%v", actual)
	expectedStr := fmt.Sprintf("%v", expected)

	// Try exact equality first
	if reflect.DeepEqual(actual, expected) {
		details["match_type"] = "exact"
		details["matched"] = true
		return 1.0, details, nil
	}

	// For string types, try fuzzy matching
	actualStrClean := strings.TrimSpace(strings.ToLower(actualStr))
	expectedStrClean := strings.TrimSpace(strings.ToLower(expectedStr))

	if actualStrClean == expectedStrClean {
		details["match_type"] = "case_insensitive"
		details["matched"] = true
		return 1.0, details, nil
	}

	// Check if expected is a substring of actual
	if strings.Contains(actualStrClean, expectedStrClean) {
		details["match_type"] = "substring"
		details["matched"] = true
		return 0.9, details, nil
	}

	// Calculate similarity score for strings
	similarity := s.stringSimilarity(actualStrClean, expectedStrClean)
	details["match_type"] = "fuzzy"
	details["similarity"] = similarity
	details["matched"] = similarity >= s.opts.FuzzyThreshold

	if similarity >= s.opts.FuzzyThreshold {
		return similarity, details, nil
	}

	// No match
	details["matched"] = false
	details["actual"] = actualStr
	details["expected"] = expectedStr
	return 0.0, details, nil
}

// llmJudge uses an LLM to evaluate task completion based on a rubric.
func (s *taskCompletionScorer) llmJudge(ctx context.Context, sample Sample) (float64, map[string]any, error) {
	details := make(map[string]any)

	// Construct the evaluation prompt
	prompt := s.buildJudgePrompt(sample)

	// Create messages for the LLM
	messages := []llm.Message{
		{
			Role:    "user",
			Content: prompt,
		},
	}

	// Request JSON output with low temperature for consistency
	temp := 0.0
	maxTokens := 500
	resp, err := s.opts.Judge.Complete(ctx, messages,
		llm.WithTemperature(temp),
		llm.WithMaxTokens(maxTokens),
	)
	if err != nil {
		return 0.0, nil, fmt.Errorf("LLM completion failed: %w", err)
	}

	// Parse the JSON response
	var judgeResult struct {
		Score     float64 `json:"score"`
		Reasoning string  `json:"reasoning"`
	}

	// Try to extract JSON from the response
	content := strings.TrimSpace(resp.Content)

	// Handle markdown code blocks
	if strings.Contains(content, "```json") {
		start := strings.Index(content, "```json") + 7
		end := strings.Index(content[start:], "```")
		if end > 0 {
			content = strings.TrimSpace(content[start : start+end])
		}
	} else if strings.Contains(content, "```") {
		start := strings.Index(content, "```") + 3
		end := strings.Index(content[start:], "```")
		if end > 0 {
			content = strings.TrimSpace(content[start : start+end])
		}
	}

	if err := json.Unmarshal([]byte(content), &judgeResult); err != nil {
		return 0.0, nil, fmt.Errorf("failed to parse LLM judge response: %w\nResponse: %s", err, resp.Content)
	}

	// Validate score is in valid range
	if judgeResult.Score < 0.0 || judgeResult.Score > 1.0 {
		return 0.0, nil, fmt.Errorf("LLM judge returned invalid score: %.2f (must be 0.0-1.0)", judgeResult.Score)
	}

	details["judge_score"] = judgeResult.Score
	details["judge_reasoning"] = judgeResult.Reasoning
	details["llm_finish_reason"] = resp.FinishReason

	return judgeResult.Score, details, nil
}

// buildJudgePrompt constructs the prompt for LLM-as-judge evaluation.
func (s *taskCompletionScorer) buildJudgePrompt(sample Sample) string {
	// Use the task's Context["objective"] field as the description
	taskDesc := ""
	if objective, ok := sample.Task.Context["objective"]; ok {
		taskDesc = fmt.Sprintf("%v", objective)
	}
	if taskDesc == "" {
		// Fallback to full context if objective is empty
		if len(sample.Task.Context) > 0 {
			taskDesc = fmt.Sprintf("%v", sample.Task.Context)
		} else {
			taskDesc = "No task description provided"
		}
	}

	resultStr := fmt.Sprintf("%v", sample.Result)

	return fmt.Sprintf(`Evaluate whether the agent's output achieves the task goal.

Task: %s

Agent Output: %s

Rubric: %s

Respond with JSON containing:
- "score": A number from 0.0 (complete failure) to 1.0 (perfect success)
- "reasoning": A brief explanation of your evaluation (1-2 sentences)

Example response:
{"score": 0.8, "reasoning": "The agent successfully identified the main vulnerability but missed secondary issues."}

Your evaluation:`, taskDesc, resultStr, s.opts.Rubric)
}

// stringSimilarity calculates the similarity between two strings.
// Returns a value in [0.0, 1.0] where 1.0 is identical.
// Uses a simple character overlap metric (Jaccard similarity).
func (s *taskCompletionScorer) stringSimilarity(a, b string) float64 {
	if a == b {
		return 1.0
	}
	if a == "" || b == "" {
		return 0.0
	}

	// Convert to character sets
	setA := make(map[rune]int)
	setB := make(map[rune]int)

	for _, ch := range a {
		setA[ch]++
	}
	for _, ch := range b {
		setB[ch]++
	}

	// Calculate intersection and union
	intersection := 0
	union := 0

	// Count intersection
	for ch, countA := range setA {
		if countB, exists := setB[ch]; exists {
			intersection += min(countA, countB)
		}
	}

	// Count union (total unique characters)
	for ch, countA := range setA {
		countB := setB[ch]
		union += max(countA, countB)
	}
	for ch, countB := range setB {
		if _, exists := setA[ch]; !exists {
			union += countB
		}
	}

	if union == 0 {
		return 0.0
	}

	return float64(intersection) / float64(union)
}

// min returns the minimum of two integers.
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// max returns the maximum of two integers.
func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
