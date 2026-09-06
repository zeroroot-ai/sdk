// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Zero Root AI

package eval

import (
	"context"
	"math"
	"testing"
	"time"
)

func TestTrajectoryScorer_ExactMatch(t *testing.T) {
	tests := []struct {
		name          string
		expectedSteps []ExpectedStep
		actualSteps   []TrajectoryStep
		penalizeExtra float64
		wantScore     float64
		wantMatched   int
		wantMissing   int
		wantExtra     int
	}{
		{
			name: "perfect match",
			expectedSteps: []ExpectedStep{
				{Type: "tool", Name: "mytool", Required: true},
				{Type: "tool", Name: "mytool-b", Required: true},
			},
			actualSteps: []TrajectoryStep{
				{Type: "tool", Name: "mytool"},
				{Type: "tool", Name: "mytool-b"},
			},
			penalizeExtra: 0.0,
			wantScore:     1.0,
			wantMatched:   2,
			wantMissing:   0,
			wantExtra:     0,
		},
		{
			name: "missing step",
			expectedSteps: []ExpectedStep{
				{Type: "tool", Name: "mytool", Required: true},
				{Type: "tool", Name: "mytool-b", Required: true},
			},
			actualSteps: []TrajectoryStep{
				{Type: "tool", Name: "mytool"},
			},
			penalizeExtra: 0.0,
			wantScore:     0.5, // 1/2 matched
			wantMatched:   1,
			wantMissing:   1,
			wantExtra:     0,
		},
		{
			name: "extra step with penalty",
			expectedSteps: []ExpectedStep{
				{Type: "tool", Name: "mytool", Required: true},
			},
			actualSteps: []TrajectoryStep{
				{Type: "tool", Name: "mytool"},
				{Type: "tool", Name: "extra"},
			},
			penalizeExtra: 0.1,
			wantScore:     0.9, // 1.0 - (1 * 0.1)
			wantMatched:   1,
			wantMissing:   0,
			wantExtra:     1,
		},
		{
			name: "wrong order",
			expectedSteps: []ExpectedStep{
				{Type: "tool", Name: "mytool", Required: true},
				{Type: "tool", Name: "mytool-b", Required: true},
			},
			actualSteps: []TrajectoryStep{
				{Type: "tool", Name: "mytool-b"},
				{Type: "tool", Name: "mytool"},
			},
			penalizeExtra: 0.0,
			wantScore:     0.0, // No match in exact mode with wrong order
			wantMatched:   0,
			wantMissing:   2,
			wantExtra:     2,
		},
		{
			name: "optional step missing",
			expectedSteps: []ExpectedStep{
				{Type: "tool", Name: "mytool", Required: true},
				{Type: "tool", Name: "optional", Required: false},
			},
			actualSteps: []TrajectoryStep{
				{Type: "tool", Name: "mytool"},
			},
			penalizeExtra: 0.0,
			wantScore:     1.0, // 1/1 required matched
			wantMatched:   1,
			wantMissing:   0,
			wantExtra:     0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			scorer := NewTrajectoryScorer(TrajectoryOptions{
				ExpectedSteps: tt.expectedSteps,
				Mode:          TrajectoryExactMatch,
				PenalizeExtra: tt.penalizeExtra,
			})

			sample := Sample{
				Trajectory: Trajectory{
					Steps: tt.actualSteps,
				},
			}

			result, err := scorer.Score(context.Background(), sample)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if math.Abs(result.Score-tt.wantScore) > 0.0001 {
				t.Errorf("Score = %v, want %v", result.Score, tt.wantScore)
			}

			matchedCount := result.Details["matched_count"].(int)
			if matchedCount != tt.wantMatched {
				t.Errorf("matched_count = %v, want %v", matchedCount, tt.wantMatched)
			}

			missingCount := len(result.Details["missing"].([]string))
			if missingCount != tt.wantMissing {
				t.Errorf("missing count = %v, want %v", missingCount, tt.wantMissing)
			}

			extraCount := result.Details["extra_count"].(int)
			if extraCount != tt.wantExtra {
				t.Errorf("extra_count = %v, want %v", extraCount, tt.wantExtra)
			}
		})
	}
}

func TestTrajectoryScorer_SubsetMatch(t *testing.T) {
	tests := []struct {
		name          string
		expectedSteps []ExpectedStep
		actualSteps   []TrajectoryStep
		penalizeExtra float64
		wantScore     float64
		wantMatched   int
		wantMissing   int
		wantExtra     int
	}{
		{
			name: "all required present, any order",
			expectedSteps: []ExpectedStep{
				{Type: "tool", Name: "mytool", Required: true},
				{Type: "tool", Name: "mytool-b", Required: true},
			},
			actualSteps: []TrajectoryStep{
				{Type: "tool", Name: "mytool-b"},
				{Type: "tool", Name: "mytool"},
			},
			penalizeExtra: 0.0,
			wantScore:     1.0,
			wantMatched:   2,
			wantMissing:   0,
			wantExtra:     0,
		},
		{
			name: "required present with extras",
			expectedSteps: []ExpectedStep{
				{Type: "tool", Name: "mytool", Required: true},
			},
			actualSteps: []TrajectoryStep{
				{Type: "tool", Name: "extra1"},
				{Type: "tool", Name: "mytool"},
				{Type: "tool", Name: "extra2"},
			},
			penalizeExtra: 0.1,
			wantScore:     0.8, // 1.0 - (2 * 0.1)
			wantMatched:   1,
			wantMissing:   0,
			wantExtra:     2,
		},
		{
			name: "missing required step",
			expectedSteps: []ExpectedStep{
				{Type: "tool", Name: "mytool", Required: true},
				{Type: "tool", Name: "mytool-b", Required: true},
			},
			actualSteps: []TrajectoryStep{
				{Type: "tool", Name: "mytool"},
				{Type: "tool", Name: "other"},
			},
			penalizeExtra: 0.1,
			wantScore:     0.4, // 0.5 - (1 * 0.1)
			wantMatched:   1,
			wantMissing:   1,
			wantExtra:     1,
		},
		{
			name: "duplicate steps",
			expectedSteps: []ExpectedStep{
				{Type: "tool", Name: "mytool", Required: true},
			},
			actualSteps: []TrajectoryStep{
				{Type: "tool", Name: "mytool"},
				{Type: "tool", Name: "mytool"},
			},
			penalizeExtra: 0.0,
			wantScore:     1.0,
			wantMatched:   1,
			wantMissing:   0,
			wantExtra:     1, // Second mytool is extra
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			scorer := NewTrajectoryScorer(TrajectoryOptions{
				ExpectedSteps: tt.expectedSteps,
				Mode:          TrajectorySubsetMatch,
				PenalizeExtra: tt.penalizeExtra,
			})

			sample := Sample{
				Trajectory: Trajectory{
					Steps: tt.actualSteps,
				},
			}

			result, err := scorer.Score(context.Background(), sample)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if math.Abs(result.Score-tt.wantScore) > 0.0001 {
				t.Errorf("Score = %v, want %v", result.Score, tt.wantScore)
			}

			matchedCount := result.Details["matched_count"].(int)
			if matchedCount != tt.wantMatched {
				t.Errorf("matched_count = %v, want %v", matchedCount, tt.wantMatched)
			}

			missingCount := len(result.Details["missing"].([]string))
			if missingCount != tt.wantMissing {
				t.Errorf("missing count = %v, want %v", missingCount, tt.wantMissing)
			}

			extraCount := result.Details["extra_count"].(int)
			if extraCount != tt.wantExtra {
				t.Errorf("extra_count = %v, want %v", extraCount, tt.wantExtra)
			}
		})
	}
}

func TestTrajectoryScorer_OrderedSubset(t *testing.T) {
	tests := []struct {
		name          string
		expectedSteps []ExpectedStep
		actualSteps   []TrajectoryStep
		penalizeExtra float64
		wantScore     float64
		wantMatched   int
		wantMissing   int
		wantExtra     int
	}{
		{
			name: "ordered with extras between",
			expectedSteps: []ExpectedStep{
				{Type: "tool", Name: "mytool", Required: true},
				{Type: "tool", Name: "mytool-b", Required: true},
			},
			actualSteps: []TrajectoryStep{
				{Type: "tool", Name: "mytool"},
				{Type: "tool", Name: "extra1"},
				{Type: "tool", Name: "mytool-b"},
				{Type: "tool", Name: "extra2"},
			},
			penalizeExtra: 0.0,
			wantScore:     1.0,
			wantMatched:   2,
			wantMissing:   0,
			wantExtra:     2,
		},
		{
			name: "ordered with penalty",
			expectedSteps: []ExpectedStep{
				{Type: "tool", Name: "mytool", Required: true},
				{Type: "tool", Name: "mytool-b", Required: true},
			},
			actualSteps: []TrajectoryStep{
				{Type: "tool", Name: "mytool"},
				{Type: "tool", Name: "extra"},
				{Type: "tool", Name: "mytool-b"},
			},
			penalizeExtra: 0.1,
			wantScore:     0.9, // 1.0 - (1 * 0.1)
			wantMatched:   2,
			wantMissing:   0,
			wantExtra:     1,
		},
		{
			name: "wrong order fails",
			expectedSteps: []ExpectedStep{
				{Type: "tool", Name: "mytool", Required: true},
				{Type: "tool", Name: "mytool-b", Required: true},
			},
			actualSteps: []TrajectoryStep{
				{Type: "tool", Name: "mytool-b"},
				{Type: "tool", Name: "mytool"},
			},
			penalizeExtra: 0.0,
			wantScore:     0.5, // Only first match (mytool-b won't match mytool)
			wantMatched:   1,   // mytool found after mytool-b
			wantMissing:   1,   // mytool-b not found after mytool
			wantExtra:     1,   // mytool-b at position 0
		},
		{
			name: "partial order match",
			expectedSteps: []ExpectedStep{
				{Type: "tool", Name: "step1", Required: true},
				{Type: "tool", Name: "step2", Required: true},
				{Type: "tool", Name: "step3", Required: true},
			},
			actualSteps: []TrajectoryStep{
				{Type: "tool", Name: "step1"},
				{Type: "tool", Name: "step2"},
			},
			penalizeExtra: 0.0,
			wantScore:     0.666667, // 2/3 matched
			wantMatched:   2,
			wantMissing:   1,
			wantExtra:     0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			scorer := NewTrajectoryScorer(TrajectoryOptions{
				ExpectedSteps: tt.expectedSteps,
				Mode:          TrajectoryOrderedSubset,
				PenalizeExtra: tt.penalizeExtra,
			})

			sample := Sample{
				Trajectory: Trajectory{
					Steps: tt.actualSteps,
				},
			}

			result, err := scorer.Score(context.Background(), sample)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if math.Abs(result.Score-tt.wantScore) > 0.0001 {
				t.Errorf("Score = %v, want %v", result.Score, tt.wantScore)
			}

			matchedCount := result.Details["matched_count"].(int)
			if matchedCount != tt.wantMatched {
				t.Errorf("matched_count = %v, want %v", matchedCount, tt.wantMatched)
			}

			missingCount := len(result.Details["missing"].([]string))
			if missingCount != tt.wantMissing {
				t.Errorf("missing count = %v, want %v", missingCount, tt.wantMissing)
			}

			extraCount := result.Details["extra_count"].(int)
			if extraCount != tt.wantExtra {
				t.Errorf("extra_count = %v, want %v", extraCount, tt.wantExtra)
			}
		})
	}
}

func TestTrajectoryScorer_EmptyExpectedSteps(t *testing.T) {
	scorer := NewTrajectoryScorer(TrajectoryOptions{
		ExpectedSteps: []ExpectedStep{},
		Mode:          TrajectoryExactMatch,
		PenalizeExtra: 0.0,
	})

	sample := Sample{
		Trajectory: Trajectory{
			Steps: []TrajectoryStep{
				{Type: "tool", Name: "mytool"},
			},
		},
	}

	result, err := scorer.Score(context.Background(), sample)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// No required steps means perfect score
	if result.Score != 1.0 {
		t.Errorf("Score = %v, want 1.0", result.Score)
	}
}

func TestTrajectoryScorer_EmptyActualSteps(t *testing.T) {
	scorer := NewTrajectoryScorer(TrajectoryOptions{
		ExpectedSteps: []ExpectedStep{
			{Type: "tool", Name: "mytool", Required: true},
		},
		Mode:          TrajectorySubsetMatch,
		PenalizeExtra: 0.0,
	})

	sample := Sample{
		Trajectory: Trajectory{
			Steps: []TrajectoryStep{},
		},
	}

	result, err := scorer.Score(context.Background(), sample)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Missing all required steps
	if result.Score != 0.0 {
		t.Errorf("Score = %v, want 0.0", result.Score)
	}
}

func TestTrajectoryScorer_MatchWithoutName(t *testing.T) {
	scorer := NewTrajectoryScorer(TrajectoryOptions{
		ExpectedSteps: []ExpectedStep{
			{Type: "finding", Name: "", Required: true}, // Any finding
		},
		Mode:          TrajectorySubsetMatch,
		PenalizeExtra: 0.0,
	})

	sample := Sample{
		Trajectory: Trajectory{
			Steps: []TrajectoryStep{
				{Type: "finding", Name: "CVE-2024-1234"},
			},
		},
	}

	result, err := scorer.Score(context.Background(), sample)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.Score != 1.0 {
		t.Errorf("Score = %v, want 1.0", result.Score)
	}
}

func TestTrajectoryScorer_ScoreClamping(t *testing.T) {
	scorer := NewTrajectoryScorer(TrajectoryOptions{
		ExpectedSteps: []ExpectedStep{
			{Type: "tool", Name: "mytool", Required: true},
		},
		Mode:          TrajectorySubsetMatch,
		PenalizeExtra: 0.5, // High penalty
	})

	sample := Sample{
		Trajectory: Trajectory{
			Steps: []TrajectoryStep{
				{Type: "tool", Name: "mytool"},
				{Type: "tool", Name: "extra1"},
				{Type: "tool", Name: "extra2"},
				{Type: "tool", Name: "extra3"},
			},
		},
	}

	result, err := scorer.Score(context.Background(), sample)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Score should be clamped to 0.0
	// Base score: 1.0, Penalty: 3 * 0.5 = 1.5, Final: -0.5 -> 0.0
	if result.Score != 0.0 {
		t.Errorf("Score = %v, want 0.0 (clamped)", result.Score)
	}
}

func TestTrajectoryScorer_Name(t *testing.T) {
	scorer := NewTrajectoryScorer(TrajectoryOptions{
		ExpectedSteps: []ExpectedStep{},
		Mode:          TrajectoryExactMatch,
		PenalizeExtra: 0.0,
	})

	if scorer.Name() != "trajectory" {
		t.Errorf("Name() = %v, want 'trajectory'", scorer.Name())
	}
}

func TestTrajectoryScorer_DetailsContent(t *testing.T) {
	scorer := NewTrajectoryScorer(TrajectoryOptions{
		ExpectedSteps: []ExpectedStep{
			{Type: "tool", Name: "mytool", Required: true},
			{Type: "tool", Name: "mytool-b", Required: true},
		},
		Mode:          TrajectorySubsetMatch,
		PenalizeExtra: 0.1,
	})

	sample := Sample{
		Trajectory: Trajectory{
			Steps: []TrajectoryStep{
				{Type: "tool", Name: "mytool"},
				{Type: "tool", Name: "extra"},
			},
		},
	}

	result, err := scorer.Score(context.Background(), sample)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Check details structure
	if _, ok := result.Details["matched"]; !ok {
		t.Error("Details missing 'matched' key")
	}
	if _, ok := result.Details["missing"]; !ok {
		t.Error("Details missing 'missing' key")
	}
	if _, ok := result.Details["extra"]; !ok {
		t.Error("Details missing 'extra' key")
	}
	if _, ok := result.Details["matched_count"]; !ok {
		t.Error("Details missing 'matched_count' key")
	}
	if _, ok := result.Details["required_count"]; !ok {
		t.Error("Details missing 'required_count' key")
	}
	if _, ok := result.Details["extra_count"]; !ok {
		t.Error("Details missing 'extra_count' key")
	}
	if _, ok := result.Details["mode"]; !ok {
		t.Error("Details missing 'mode' key")
	}

	// Check matched list
	matched := result.Details["matched"].([]string)
	if len(matched) != 1 {
		t.Errorf("matched length = %v, want 1", len(matched))
	}
	if matched[0] != "tool:mytool" {
		t.Errorf("matched[0] = %v, want 'tool:mytool'", matched[0])
	}

	// Check missing list
	missing := result.Details["missing"].([]string)
	if len(missing) != 1 {
		t.Errorf("missing length = %v, want 1", len(missing))
	}
	if missing[0] != "tool:mytool-b" {
		t.Errorf("missing[0] = %v, want 'tool:mytool-b'", missing[0])
	}

	// Check extra list
	extra := result.Details["extra"].([]string)
	if len(extra) != 1 {
		t.Errorf("extra length = %v, want 1", len(extra))
	}
	if extra[0] != "tool:extra" {
		t.Errorf("extra[0] = %v, want 'tool:extra'", extra[0])
	}
}

func TestTrajectoryScorer_RealWorldScenario(t *testing.T) {
	// Simulate a real agent trajectory for reconnaissance
	scorer := NewTrajectoryScorer(TrajectoryOptions{
		ExpectedSteps: []ExpectedStep{
			{Type: "tool", Name: "mytool", Required: true},
			{Type: "llm", Name: "primary", Required: true},
			{Type: "tool", Name: "mytool-b", Required: true},
			{Type: "finding", Name: "", Required: true},
		},
		Mode:          TrajectoryOrderedSubset,
		PenalizeExtra: 0.05,
	})

	sample := Sample{
		ID: "recon-001",
		Trajectory: Trajectory{
			Steps: []TrajectoryStep{
				{
					Type:      "llm",
					Name:      "primary",
					StartTime: time.Now(),
					Duration:  100 * time.Millisecond,
				},
				{
					Type:      "tool",
					Name:      "mytool",
					StartTime: time.Now(),
					Duration:  2 * time.Second,
				},
				{
					Type:      "llm",
					Name:      "primary",
					StartTime: time.Now(),
					Duration:  150 * time.Millisecond,
				},
				{
					Type:      "tool",
					Name:      "mytool-b",
					StartTime: time.Now(),
					Duration:  5 * time.Second,
				},
				{
					Type:      "llm",
					Name:      "primary",
					StartTime: time.Now(),
					Duration:  200 * time.Millisecond,
				},
				{
					Type:      "finding",
					Name:      "CVE-2024-1234",
					StartTime: time.Now(),
					Duration:  10 * time.Millisecond,
				},
			},
			StartTime: time.Now(),
			EndTime:   time.Now().Add(8 * time.Second),
		},
	}

	result, err := scorer.Score(context.Background(), sample)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// All 4 required steps matched, 2 extra LLM calls
	// Score = 1.0 - (2 * 0.05) = 0.9
	expectedScore := 0.9
	if math.Abs(result.Score-expectedScore) > 0.0001 {
		t.Errorf("Score = %v, want %v", result.Score, expectedScore)
	}

	matchedCount := result.Details["matched_count"].(int)
	if matchedCount != 4 {
		t.Errorf("matched_count = %v, want 4", matchedCount)
	}

	extraCount := result.Details["extra_count"].(int)
	if extraCount != 2 {
		t.Errorf("extra_count = %v, want 2", extraCount)
	}
}
