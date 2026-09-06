// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Zero Root AI

package eval_test

import (
	"context"
	"fmt"
	"time"

	"github.com/zeroroot-ai/sdk/eval"
)

// Example demonstrating streaming tool correctness evaluation with ordered matching.
func ExampleNewStreamingToolCorrectnessScorer_ordered() {
	// Create a streaming scorer that expects tools in order
	scorer := eval.NewStreamingToolCorrectnessScorer(eval.ToolCorrectnessOptions{
		ExpectedTools: []eval.ExpectedToolCall{
			{Name: "mytool", Required: true},
			{Name: "http-client", Required: true},
			{Name: "sqlmap", Required: true},
		},
		OrderMatters: true,
	})

	// Simulate partial trajectory after first tool call
	trajectory := eval.Trajectory{
		Steps: []eval.TrajectoryStep{
			{Type: "tool", Name: "mytool", Input: map[string]any{}},
		},
		StartTime: time.Now(),
	}

	// Evaluate partial trajectory
	result, _ := scorer.ScorePartial(context.Background(), trajectory)

	fmt.Printf("Score: %.2f\n", result.Score)
	fmt.Printf("Confidence: %.2f\n", result.Confidence)
	fmt.Printf("Status: %s\n", result.Status)
	fmt.Printf("Action: %s\n", result.Action)
	// Output:
	// Score: 0.33
	// Confidence: 0.33
	// Status: partial
	// Action: continue
}

// Example demonstrating early detection of wrong tool in sequence.
func ExampleNewStreamingToolCorrectnessScorer_wrongTool() {
	scorer := eval.NewStreamingToolCorrectnessScorer(eval.ToolCorrectnessOptions{
		ExpectedTools: []eval.ExpectedToolCall{
			{Name: "mytool", Required: true},
			{Name: "http-client", Required: true},
			{Name: "sqlmap", Required: true},
		},
		OrderMatters: true,
	})

	// Agent called mytool correctly, but then skipped to sqlmap (wrong order)
	trajectory := eval.Trajectory{
		Steps: []eval.TrajectoryStep{
			{Type: "tool", Name: "mytool", Input: map[string]any{}},
			{Type: "tool", Name: "sqlmap", Input: map[string]any{}},
		},
		StartTime: time.Now(),
	}

	result, _ := scorer.ScorePartial(context.Background(), trajectory)

	fmt.Printf("Score: %.2f\n", result.Score)
	fmt.Printf("Action: %s\n", result.Action)
	// Feedback indicates mismatch detected
	fmt.Printf("Has feedback: %v\n", len(result.Feedback) > 0)
	// Output:
	// Score: 0.33
	// Action: reconsider
	// Has feedback: true
}

// Example demonstrating unordered tool matching.
func ExampleNewStreamingToolCorrectnessScorer_unordered() {
	scorer := eval.NewStreamingToolCorrectnessScorer(eval.ToolCorrectnessOptions{
		ExpectedTools: []eval.ExpectedToolCall{
			{Name: "mytool", Required: true},
			{Name: "http-client", Required: true},
			{Name: "sqlmap", Required: true},
		},
		OrderMatters: false, // Tools can be called in any order
	})

	// Agent called tools in different order
	trajectory := eval.Trajectory{
		Steps: []eval.TrajectoryStep{
			{Type: "tool", Name: "http-client", Input: map[string]any{}},
			{Type: "tool", Name: "mytool", Input: map[string]any{}},
		},
		StartTime: time.Now(),
	}

	result, _ := scorer.ScorePartial(context.Background(), trajectory)

	fmt.Printf("Score: %.2f\n", result.Score)
	fmt.Printf("Action: %s\n", result.Action)
	// Output:
	// Score: 0.67
	// Action: continue
}
