// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Zero Root AI

package eval

import (
	"context"
	"testing"
	"time"
)

// TestStreamingScorers_EdgeCases tests edge cases that apply to all streaming scorers.
func TestStreamingScorers_EdgeCases(t *testing.T) {
	ctx := context.Background()

	// Create instances of all streaming scorers
	scorers := []struct {
		name   string
		scorer StreamingScorer
	}{
		{
			name: "ToolCorrectnessScorer",
			scorer: NewStreamingToolCorrectnessScorer(ToolCorrectnessOptions{
				ExpectedTools: []ExpectedToolCall{
					{Name: "tool1", Required: true},
					{Name: "tool2", Required: true},
				},
				OrderMatters: true,
			}),
		},
		{
			name: "TrajectoryScorer",
			scorer: NewStreamingTrajectoryScorer(TrajectoryOptions{
				ExpectedSteps: []ExpectedStep{
					{Type: "tool", Name: "tool1", Required: true},
					{Type: "tool", Name: "tool2", Required: true},
				},
				Mode: TrajectoryExactMatch,
			}),
		},
		{
			name: "FindingAccuracyScorer",
			scorer: NewStreamingFindingAccuracyScorer(FindingAccuracyOptions{
				GroundTruth: []GroundTruthFinding{
					{ID: "finding1", Title: "Test Finding", Severity: "high", Category: "test"},
				},
			}),
		},
	}

	t.Run("empty_trajectory", func(t *testing.T) {
		for _, s := range scorers {
			t.Run(s.name, func(t *testing.T) {
				trajectory := Trajectory{
					Steps:     []TrajectoryStep{},
					StartTime: time.Now(),
				}

				result, err := s.scorer.ScorePartial(ctx, trajectory)
				if err != nil {
					t.Fatalf("ScorePartial failed: %v", err)
				}

				// All scorers should handle empty trajectory gracefully
				if result.Status != ScoreStatusPending {
					t.Errorf("Expected status pending for empty trajectory, got %s", result.Status)
				}

				if result.Action != ActionContinue {
					t.Errorf("Expected action continue for empty trajectory, got %s", result.Action)
				}

				if result.Feedback == "" {
					t.Error("Expected non-empty feedback for empty trajectory")
				}
			})
		}
	})

	t.Run("single_step_trajectory", func(t *testing.T) {
		for _, s := range scorers {
			t.Run(s.name, func(t *testing.T) {
				trajectory := Trajectory{
					Steps: []TrajectoryStep{
						{Type: "tool", Name: "tool1", StartTime: time.Now()},
					},
					StartTime: time.Now(),
				}

				result, err := s.scorer.ScorePartial(ctx, trajectory)
				if err != nil {
					t.Fatalf("ScorePartial failed: %v", err)
				}

				// All scorers should handle single step gracefully
				if result.Feedback == "" {
					t.Error("Expected non-empty feedback for single step trajectory")
				}

				// Score should be between 0 and 1
				if result.Score < 0 || result.Score > 1 {
					t.Errorf("Score out of range [0,1]: %f", result.Score)
				}

				// Confidence should be between 0 and 1
				if result.Confidence < 0 || result.Confidence > 1 {
					t.Errorf("Confidence out of range [0,1]: %f", result.Confidence)
				}
			})
		}
	})

	t.Run("context_cancellation", func(t *testing.T) {
		for _, s := range scorers {
			t.Run(s.name, func(t *testing.T) {
				// Create cancelled context
				cancelledCtx, cancel := context.WithCancel(ctx)
				cancel()

				trajectory := Trajectory{
					Steps: []TrajectoryStep{
						{Type: "tool", Name: "tool1", StartTime: time.Now()},
					},
					StartTime: time.Now(),
				}

				// Scorer should respect context cancellation or complete quickly
				_, err := s.scorer.ScorePartial(cancelledCtx, trajectory)
				// Some scorers may not check context for simple operations
				// We're just ensuring it doesn't panic or hang
				_ = err
			})
		}
	})

	t.Run("nil_trajectory_steps", func(t *testing.T) {
		for _, s := range scorers {
			t.Run(s.name, func(t *testing.T) {
				trajectory := Trajectory{
					Steps:     nil, // nil slice
					StartTime: time.Now(),
				}

				result, err := s.scorer.ScorePartial(ctx, trajectory)
				if err != nil {
					t.Fatalf("ScorePartial failed with nil steps: %v", err)
				}

				// Should handle nil steps same as empty slice
				if result.Status != ScoreStatusPending {
					t.Errorf("Expected status pending for nil steps, got %s", result.Status)
				}
			})
		}
	})
}

// TestStreamingScorers_InterfaceCompliance verifies all streaming scorers implement interfaces correctly.
func TestStreamingScorers_InterfaceCompliance(t *testing.T) {
	scorers := []struct {
		name   string
		scorer Scorer
	}{
		{
			name:   "ToolCorrectnessScorer",
			scorer: NewStreamingToolCorrectnessScorer(ToolCorrectnessOptions{}),
		},
		{
			name:   "TrajectoryScorer",
			scorer: NewStreamingTrajectoryScorer(TrajectoryOptions{Mode: TrajectoryExactMatch}),
		},
		{
			name:   "FindingAccuracyScorer",
			scorer: NewStreamingFindingAccuracyScorer(FindingAccuracyOptions{}),
		},
	}

	for _, s := range scorers {
		t.Run(s.name, func(t *testing.T) {
			// Verify implements Scorer
			if s.scorer == nil {
				t.Fatal("Scorer should not be nil")
			}

			// Verify Name() returns non-empty
			if s.scorer.Name() == "" {
				t.Error("Name() should return non-empty string")
			}

			// Verify implements StreamingScorer
			streamingScorer, ok := s.scorer.(StreamingScorer)
			if !ok {
				t.Fatal("Scorer should implement StreamingScorer interface")
			}

			// Verify SupportsStreaming() returns true
			if !streamingScorer.SupportsStreaming() {
				t.Error("SupportsStreaming() should return true")
			}
		})
	}
}

// TestStreamingScorers_ScoreConsistency verifies that scoring is deterministic and consistent.
func TestStreamingScorers_ScoreConsistency(t *testing.T) {
	ctx := context.Background()
	baseTime := time.Now()

	trajectory := Trajectory{
		Steps: []TrajectoryStep{
			{Type: "tool", Name: "tool1", StartTime: baseTime},
			{Type: "tool", Name: "tool2", StartTime: baseTime.Add(time.Second)},
		},
		StartTime: baseTime,
	}

	scorers := []struct {
		name   string
		scorer StreamingScorer
	}{
		{
			name: "ToolCorrectnessScorer",
			scorer: NewStreamingToolCorrectnessScorer(ToolCorrectnessOptions{
				ExpectedTools: []ExpectedToolCall{
					{Name: "tool1", Required: true},
					{Name: "tool2", Required: true},
				},
			}),
		},
		{
			name: "TrajectoryScorer",
			scorer: NewStreamingTrajectoryScorer(TrajectoryOptions{
				ExpectedSteps: []ExpectedStep{
					{Type: "tool", Name: "tool1", Required: true},
					{Type: "tool", Name: "tool2", Required: true},
				},
				Mode: TrajectoryExactMatch,
			}),
		},
	}

	for _, s := range scorers {
		t.Run(s.name, func(t *testing.T) {
			// Score the same trajectory multiple times
			results := make([]PartialScore, 3)
			for i := range 3 {
				result, err := s.scorer.ScorePartial(ctx, trajectory)
				if err != nil {
					t.Fatalf("ScorePartial failed on iteration %d: %v", i, err)
				}
				results[i] = result
			}

			// Verify all results are identical
			for i := 1; i < len(results); i++ {
				if results[i].Score != results[0].Score {
					t.Errorf("Score inconsistent: iteration %d = %f, iteration 0 = %f",
						i, results[i].Score, results[0].Score)
				}

				if results[i].Confidence != results[0].Confidence {
					t.Errorf("Confidence inconsistent: iteration %d = %f, iteration 0 = %f",
						i, results[i].Confidence, results[0].Confidence)
				}

				if results[i].Status != results[0].Status {
					t.Errorf("Status inconsistent: iteration %d = %s, iteration 0 = %s",
						i, results[i].Status, results[0].Status)
				}

				if results[i].Action != results[0].Action {
					t.Errorf("Action inconsistent: iteration %d = %s, iteration 0 = %s",
						i, results[i].Action, results[0].Action)
				}
			}
		})
	}
}

// TestStreamingScorers_ProgressiveScoring verifies that scores change appropriately as trajectory grows.
func TestStreamingScorers_ProgressiveScoring(t *testing.T) {
	ctx := context.Background()
	baseTime := time.Now()

	scorer := NewStreamingToolCorrectnessScorer(ToolCorrectnessOptions{
		ExpectedTools: []ExpectedToolCall{
			{Name: "tool1", Required: true},
			{Name: "tool2", Required: true},
			{Name: "tool3", Required: true},
		},
		OrderMatters: true,
	})

	// Build trajectory progressively
	trajectories := []Trajectory{
		{Steps: []TrajectoryStep{}, StartTime: baseTime},
		{
			Steps: []TrajectoryStep{
				{Type: "tool", Name: "tool1", StartTime: baseTime},
			},
			StartTime: baseTime,
		},
		{
			Steps: []TrajectoryStep{
				{Type: "tool", Name: "tool1", StartTime: baseTime},
				{Type: "tool", Name: "tool2", StartTime: baseTime.Add(time.Second)},
			},
			StartTime: baseTime,
		},
		{
			Steps: []TrajectoryStep{
				{Type: "tool", Name: "tool1", StartTime: baseTime},
				{Type: "tool", Name: "tool2", StartTime: baseTime.Add(time.Second)},
				{Type: "tool", Name: "tool3", StartTime: baseTime.Add(2 * time.Second)},
			},
			StartTime: baseTime,
		},
	}

	var prevScore float64 = -1
	for i, traj := range trajectories {
		result, err := scorer.ScorePartial(ctx, traj)
		if err != nil {
			t.Fatalf("ScorePartial failed for trajectory %d: %v", i, err)
		}

		t.Logf("Trajectory %d: %d steps, score=%f, confidence=%f, status=%s",
			i, len(traj.Steps), result.Score, result.Confidence, result.Status)

		// Score should increase as more correct tools are added (or stay same for empty)
		if i > 0 && result.Score < prevScore {
			t.Errorf("Score decreased from %f to %f when adding correct tool",
				prevScore, result.Score)
		}

		prevScore = result.Score
	}

	// Final score should be 1.0 (all tools correct)
	finalResult, _ := scorer.ScorePartial(ctx, trajectories[len(trajectories)-1])
	if finalResult.Score != 1.0 {
		t.Errorf("Expected final score 1.0, got %f", finalResult.Score)
	}
}
