// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Zero Root AI

package eval_test

import (
	"context"
	"fmt"
	"time"

	"github.com/zeroroot-ai/sdk/eval"
)

func ExampleNewToolCorrectnessScorer() {
	// Create a scorer with default options
	scorer := eval.NewToolCorrectnessScorer(eval.ToolCorrectnessOptions{})

	// Create a sample with expected tools
	sample := eval.Sample{
		ID: "example-1",
		ExpectedTools: []eval.ExpectedToolCall{
			{
				Name: "mytool",
				Arguments: map[string]any{
					"target": "192.168.1.1",
					"ports":  "80,443",
				},
				Required: true,
			},
			{
				Name: "http-client",
				Arguments: map[string]any{
					"url":    "https://example.com",
					"method": "GET",
				},
				Required: true,
			},
		},
		Trajectory: eval.Trajectory{
			Steps: []eval.TrajectoryStep{
				{
					Type: "tool",
					Name: "mytool",
					Input: map[string]any{
						"target": "192.168.1.1",
						"ports":  "80,443",
					},
					StartTime: time.Now(),
					Duration:  2 * time.Second,
				},
				{
					Type: "tool",
					Name: "http-client",
					Input: map[string]any{
						"url":    "https://example.com",
						"method": "GET",
					},
					StartTime: time.Now(),
					Duration:  1 * time.Second,
				},
			},
		},
	}

	// Score the sample
	result, err := scorer.Score(context.Background(), sample)
	if err != nil {
		fmt.Printf("failed to score sample: %v\n", err)
		return
	}

	fmt.Printf("Score: %.2f\n", result.Score)
	fmt.Printf("Matched: %d\n", result.Details["matched"])
	fmt.Printf("Missing: %d\n", result.Details["missing"])
	fmt.Printf("Extra: %d\n", result.Details["extra"])
	// Output:
	// Score: 1.00
	// Matched: 2
	// Missing: 0
	// Extra: 0
}

func ExampleNewToolCorrectnessScorer_withNumericTolerance() {
	// Create a scorer with numeric tolerance for fuzzy matching
	scorer := eval.NewToolCorrectnessScorer(eval.ToolCorrectnessOptions{
		NumericTolerance: 0.01,
	})

	sample := eval.Sample{
		ID: "example-2",
		ExpectedTools: []eval.ExpectedToolCall{
			{
				Name: "threshold-check",
				Arguments: map[string]any{
					"threshold": 0.95,
				},
				Required: true,
			},
		},
		Trajectory: eval.Trajectory{
			Steps: []eval.TrajectoryStep{
				{
					Type: "tool",
					Name: "threshold-check",
					Input: map[string]any{
						"threshold": 0.951, // Within 0.01 tolerance
					},
					StartTime: time.Now(),
					Duration:  500 * time.Millisecond,
				},
			},
		},
	}

	result, err := scorer.Score(context.Background(), sample)
	if err != nil {
		fmt.Printf("failed to score sample: %v\n", err)
		return
	}

	fmt.Printf("Score: %.2f\n", result.Score)
	fmt.Printf("Matched: %d\n", result.Details["matched"])
	// Output:
	// Score: 1.00
	// Matched: 1
}

func ExampleNewToolCorrectnessScorer_withOrderMatters() {
	// Create a scorer that requires tools to be called in order
	scorer := eval.NewToolCorrectnessScorer(eval.ToolCorrectnessOptions{
		OrderMatters: true,
	})

	sample := eval.Sample{
		ID: "example-3",
		ExpectedTools: []eval.ExpectedToolCall{
			{Name: "recon", Required: true},
			{Name: "exploit", Required: true},
			{Name: "exfiltrate", Required: true},
		},
		Trajectory: eval.Trajectory{
			Steps: []eval.TrajectoryStep{
				{Type: "tool", Name: "recon", StartTime: time.Now(), Duration: 1 * time.Second},
				{Type: "tool", Name: "exploit", StartTime: time.Now(), Duration: 2 * time.Second},
				{Type: "tool", Name: "exfiltrate", StartTime: time.Now(), Duration: 1 * time.Second},
			},
		},
	}

	result, err := scorer.Score(context.Background(), sample)
	if err != nil {
		fmt.Printf("failed to score sample: %v\n", err)
		return
	}

	fmt.Printf("Score: %.2f\n", result.Score)
	fmt.Printf("Matched: %d\n", result.Details["matched"])
	// Output:
	// Score: 1.00
	// Matched: 3
}
