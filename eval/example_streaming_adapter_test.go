// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Zero Root AI

package eval_test

import (
	"context"
	"fmt"
	"time"

	"github.com/zeroroot-ai/sdk/eval"
)

// ExampleNewStreamingAdapter demonstrates how to wrap an existing Scorer
// to support streaming evaluation with partial trajectories.
func ExampleNewStreamingAdapter() {
	// Create a tool correctness scorer (any Scorer implementation)
	baseScorer := eval.NewToolCorrectnessScorer(eval.ToolCorrectnessOptions{
		OrderMatters: false,
	})

	// Wrap it with the streaming adapter
	opts := eval.StreamingAdapterOptions{
		MinStepsForEval:    2,   // Need at least 2 steps before evaluating
		PartialScoreWeight: 0.8, // 80% confidence for partial scores
	}
	streamingScorer := eval.NewStreamingAdapter(baseScorer, opts)

	// Simulate partial trajectory as agent executes
	trajectory := eval.Trajectory{
		Steps: []eval.TrajectoryStep{
			{
				Type:      "tool",
				Name:      "mytool",
				StartTime: time.Now(),
			},
			// Agent hasn't called mytool-b yet...
		},
		StartTime: time.Now(),
	}

	// Score the partial trajectory
	ctx := context.Background()
	result, _ := streamingScorer.ScorePartial(ctx, trajectory)

	fmt.Printf("Score: %.2f\n", result.Score)
	fmt.Printf("Status: %s\n", result.Status)
	fmt.Printf("Action: %s\n", result.Action)
	fmt.Printf("Supports Streaming: %v\n", streamingScorer.SupportsStreaming())

	// Output:
	// Score: 0.00
	// Status: pending
	// Action: continue
	// Supports Streaming: true
}

// ExampleNewStreamingAdapter_withDefaults demonstrates using default options.
func ExampleNewStreamingAdapter_withDefaults() {
	// Create any scorer
	baseScorer := eval.NewToolCorrectnessScorer(eval.ToolCorrectnessOptions{})

	// Use default options (MinStepsForEval: 1, PartialScoreWeight: 0.8)
	streamingScorer := eval.NewStreamingAdapter(
		baseScorer,
		eval.DefaultStreamingAdapterOptions(),
	)

	// The adapter can now handle partial trajectories
	fmt.Printf("Supports Streaming: %v\n", streamingScorer.SupportsStreaming())
	fmt.Printf("Scorer Name: %s\n", streamingScorer.Name())

	// Output:
	// Supports Streaming: true
	// Scorer Name: tool_correctness
}

// ExampleStreamingAdapterOptions demonstrates different configuration options.
func ExampleStreamingAdapterOptions() {
	baseScorer := eval.NewToolCorrectnessScorer(eval.ToolCorrectnessOptions{})

	// High confidence, requires more steps
	strictOpts := eval.StreamingAdapterOptions{
		MinStepsForEval:    5,   // Need 5 steps before evaluating
		PartialScoreWeight: 0.9, // 90% confidence in partial scores
	}
	strictScorer := eval.NewStreamingAdapter(baseScorer, strictOpts)

	// Low confidence, evaluates early
	earlyOpts := eval.StreamingAdapterOptions{
		MinStepsForEval:    1,   // Evaluate after just 1 step
		PartialScoreWeight: 0.6, // 60% confidence (more cautious)
	}
	earlyScorer := eval.NewStreamingAdapter(baseScorer, earlyOpts)

	fmt.Printf("Strict scorer supports streaming: %v\n", strictScorer.SupportsStreaming())
	fmt.Printf("Early scorer supports streaming: %v\n", earlyScorer.SupportsStreaming())

	// Output:
	// Strict scorer supports streaming: true
	// Early scorer supports streaming: true
}
