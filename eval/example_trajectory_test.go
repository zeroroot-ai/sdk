// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Zero Root AI

package eval_test

import (
	"context"
	"fmt"
	"time"

	"github.com/zeroroot-ai/sdk/eval"
)

// ExampleNewTrajectoryScorer demonstrates basic trajectory scoring.
func ExampleNewTrajectoryScorer() {
	// Define expected execution path
	opts := eval.TrajectoryOptions{
		ExpectedSteps: []eval.ExpectedStep{
			{Type: "tool", Name: "mytool", Required: true},
			{Type: "tool", Name: "mytool-b", Required: true},
			{Type: "finding", Name: "", Required: true}, // Any finding
		},
		Mode:          eval.TrajectoryOrderedSubset,
		PenalizeExtra: 0.1, // 10% penalty per extra step
	}

	scorer := eval.NewTrajectoryScorer(opts)

	// Create a sample with actual execution trajectory
	sample := eval.Sample{
		ID: "test-001",
		Trajectory: eval.Trajectory{
			Steps: []eval.TrajectoryStep{
				{Type: "tool", Name: "mytool", StartTime: time.Now(), Duration: 2 * time.Second},
				{Type: "llm", Name: "primary", StartTime: time.Now(), Duration: 100 * time.Millisecond},
				{Type: "tool", Name: "mytool-b", StartTime: time.Now(), Duration: 5 * time.Second},
				{Type: "finding", Name: "CVE-2024-1234", StartTime: time.Now(), Duration: 10 * time.Millisecond},
			},
		},
	}

	// Score the trajectory
	result, err := scorer.Score(context.Background(), sample)
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		return
	}

	fmt.Printf("Score: %.2f\n", result.Score)
	fmt.Printf("Matched: %v\n", result.Details["matched_count"])
	fmt.Printf("Missing: %v\n", len(result.Details["missing"].([]string)))
	fmt.Printf("Extra: %v\n", result.Details["extra_count"])

	// Output:
	// Score: 0.90
	// Matched: 3
	// Missing: 0
	// Extra: 1
}

// ExampleNewTrajectoryScorer_exactMatch demonstrates exact matching mode.
func ExampleNewTrajectoryScorer_exactMatch() {
	opts := eval.TrajectoryOptions{
		ExpectedSteps: []eval.ExpectedStep{
			{Type: "tool", Name: "mytool", Required: true},
			{Type: "tool", Name: "mytool-b", Required: true},
		},
		Mode:          eval.TrajectoryExactMatch,
		PenalizeExtra: 0.0,
	}

	scorer := eval.NewTrajectoryScorer(opts)

	// Perfect match
	sample := eval.Sample{
		Trajectory: eval.Trajectory{
			Steps: []eval.TrajectoryStep{
				{Type: "tool", Name: "mytool"},
				{Type: "tool", Name: "mytool-b"},
			},
		},
	}

	result, _ := scorer.Score(context.Background(), sample)
	fmt.Printf("Perfect match score: %.2f\n", result.Score)

	// Wrong order - fails in exact mode
	sample.Trajectory.Steps = []eval.TrajectoryStep{
		{Type: "tool", Name: "mytool-b"},
		{Type: "tool", Name: "mytool"},
	}

	result, _ = scorer.Score(context.Background(), sample)
	fmt.Printf("Wrong order score: %.2f\n", result.Score)

	// Output:
	// Perfect match score: 1.00
	// Wrong order score: 0.00
}

// ExampleNewTrajectoryScorer_subsetMatch demonstrates subset matching mode.
func ExampleNewTrajectoryScorer_subsetMatch() {
	opts := eval.TrajectoryOptions{
		ExpectedSteps: []eval.ExpectedStep{
			{Type: "tool", Name: "mytool", Required: true},
			{Type: "tool", Name: "mytool-b", Required: true},
		},
		Mode:          eval.TrajectorySubsetMatch,
		PenalizeExtra: 0.05,
	}

	scorer := eval.NewTrajectoryScorer(opts)

	// All required present, any order, with extras
	sample := eval.Sample{
		Trajectory: eval.Trajectory{
			Steps: []eval.TrajectoryStep{
				{Type: "llm", Name: "primary"},   // extra
				{Type: "tool", Name: "mytool-b"}, // required (out of order is OK)
				{Type: "tool", Name: "mytool"},   // required
				{Type: "tool", Name: "sqlmap"},   // extra
			},
		},
	}

	result, _ := scorer.Score(context.Background(), sample)
	fmt.Printf("Score: %.2f\n", result.Score)
	fmt.Printf("Matched: %v\n", result.Details["matched_count"])
	fmt.Printf("Extra: %v\n", result.Details["extra_count"])

	// Output:
	// Score: 0.90
	// Matched: 2
	// Extra: 2
}

// ExampleNewTrajectoryScorer_optionalSteps demonstrates optional step handling.
func ExampleNewTrajectoryScorer_optionalSteps() {
	opts := eval.TrajectoryOptions{
		ExpectedSteps: []eval.ExpectedStep{
			{Type: "tool", Name: "mytool", Required: true},
			{Type: "llm", Name: "primary", Required: false}, // optional
			{Type: "finding", Name: "", Required: true},
		},
		Mode:          eval.TrajectorySubsetMatch,
		PenalizeExtra: 0.0,
	}

	scorer := eval.NewTrajectoryScorer(opts)

	// Optional step present
	sample1 := eval.Sample{
		Trajectory: eval.Trajectory{
			Steps: []eval.TrajectoryStep{
				{Type: "tool", Name: "mytool"},
				{Type: "llm", Name: "primary"},
				{Type: "finding", Name: "vuln-001"},
			},
		},
	}

	result, _ := scorer.Score(context.Background(), sample1)
	fmt.Printf("With optional: %.2f\n", result.Score)

	// Optional step missing - still perfect score (only required steps count)
	sample2 := eval.Sample{
		Trajectory: eval.Trajectory{
			Steps: []eval.TrajectoryStep{
				{Type: "tool", Name: "mytool"},
				{Type: "finding", Name: "vuln-001"},
			},
		},
	}

	result, _ = scorer.Score(context.Background(), sample2)
	fmt.Printf("Without optional: %.2f\n", result.Score)

	// Output:
	// With optional: 1.00
	// Without optional: 1.00
}
