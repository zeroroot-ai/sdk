// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Zero Root AI

package eval_test

import (
	"context"
	"fmt"

	"github.com/zeroroot-ai/sdk/agent"
	"github.com/zeroroot-ai/sdk/eval"
)

// ExampleNewTaskCompletionScorer_exactMatch demonstrates exact output matching.
func ExampleNewTaskCompletionScorer_exactMatch() {
	// Create a scorer that checks for exact match
	scorer := eval.NewTaskCompletionScorer(eval.TaskCompletionOptions{
		ExpectedOutput: "vulnerability found",
	})

	sample := eval.Sample{
		ID: "test-1",
		Task: agent.Task{
			Context: map[string]any{"objective": "Find security vulnerabilities"},
		},
		Result: agent.Result{
			Output: "vulnerability found",
		},
	}

	result, err := scorer.Score(context.Background(), sample)
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		return
	}

	fmt.Printf("Score: %.2f\n", result.Score)
	fmt.Printf("Match: %v\n", result.Details["matched"])
	// Output:
	// Score: 1.00
	// Match: true
}

// ExampleNewTaskCompletionScorer_binary demonstrates binary (pass/fail) scoring.
func ExampleNewTaskCompletionScorer_binary() {
	// Create a binary scorer - only returns 0.0 or 1.0
	scorer := eval.NewTaskCompletionScorer(eval.TaskCompletionOptions{
		ExpectedOutput: "sql injection",
		Binary:         true,
	})

	// Test case 1: Contains expected substring
	sample1 := eval.Sample{
		ID: "test-2a",
		Task: agent.Task{
			Context: map[string]any{"objective": "Identify vulnerability type"},
		},
		Result: agent.Result{
			Output: "Found SQL injection vulnerability in login form",
		},
	}

	result1, _ := scorer.Score(context.Background(), sample1)
	fmt.Printf("Contains substring - Binary Score: %.0f\n", result1.Score)

	// Test case 2: Does not contain expected substring
	sample2 := eval.Sample{
		ID: "test-2b",
		Task: agent.Task{
			Context: map[string]any{"objective": "Identify vulnerability type"},
		},
		Result: agent.Result{
			Output: "Found XSS vulnerability",
		},
	}

	result2, _ := scorer.Score(context.Background(), sample2)
	fmt.Printf("No match - Binary Score: %.0f\n", result2.Score)
	// Output:
	// Contains substring - Binary Score: 1
	// No match - Binary Score: 0
}

// ExampleNewTaskCompletionScorer_fuzzyMatch demonstrates case-insensitive and substring matching.
func ExampleNewTaskCompletionScorer_fuzzyMatch() {
	scorer := eval.NewTaskCompletionScorer(eval.TaskCompletionOptions{
		ExpectedOutput: "success",
		FuzzyThreshold: 0.7, // Accept 70% similarity or higher
	})

	sample := eval.Sample{
		ID: "test-3",
		Task: agent.Task{
			Context: map[string]any{"objective": "Complete the operation"},
		},
		Result: agent.Result{
			Output: "Operation completed successfully!", // Contains "success"
		},
	}

	result, _ := scorer.Score(context.Background(), sample)
	fmt.Printf("Score: %.2f\n", result.Score)
	fmt.Printf("Match Type: %v\n", result.Details["match_type"])
	// Output:
	// Score: 0.90
	// Match Type: substring
}

// ExampleNewTaskCompletionScorer_complexOutput demonstrates matching complex data structures.
func ExampleNewTaskCompletionScorer_complexOutput() {
	scorer := eval.NewTaskCompletionScorer(eval.TaskCompletionOptions{
		ExpectedOutput: map[string]any{
			"status":          "complete",
			"vulnerabilities": 3,
		},
	})

	sample := eval.Sample{
		ID: "test-4",
		Task: agent.Task{
			Context: map[string]any{"objective": "Count vulnerabilities"},
		},
		Result: agent.Result{
			Output: map[string]any{
				"status":          "complete",
				"vulnerabilities": 3,
			},
		},
	}

	result, _ := scorer.Score(context.Background(), sample)
	fmt.Printf("Score: %.2f\n", result.Score)
	fmt.Printf("Match Type: %v\n", result.Details["match_type"])
	// Output:
	// Score: 1.00
	// Match Type: exact
}
