// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Zero Root AI

package eval

import (
	"context"
	"testing"
	"time"
)

func TestStreamingTrajectoryScorer_SupportsStreaming(t *testing.T) {
	scorer := NewStreamingTrajectoryScorer(TrajectoryOptions{
		ExpectedSteps: []ExpectedStep{
			{Type: "tool", Name: "mytool", Required: true},
		},
		Mode: TrajectoryExactMatch,
	})

	if !scorer.SupportsStreaming() {
		t.Error("StreamingTrajectoryScorer should support streaming")
	}
}

func TestStreamingTrajectoryScorer_ExactMatch_OnTrack(t *testing.T) {
	scorer := NewStreamingTrajectoryScorer(TrajectoryOptions{
		ExpectedSteps: []ExpectedStep{
			{Type: "tool", Name: "mytool", Required: true},
			{Type: "tool", Name: "mytool-b", Required: true},
			{Type: "finding", Name: "", Required: true},
		},
		Mode:          TrajectoryExactMatch,
		PenalizeExtra: 0.1,
	})

	tests := []struct {
		name              string
		steps             []TrajectoryStep
		expectedScore     float64
		expectedAction    RecommendedAction
		expectedStatus    ScoreStatus
		minConfidence     float64
		shouldContainText string
	}{
		{
			name:              "no steps",
			steps:             []TrajectoryStep{},
			expectedScore:     0.0,
			expectedAction:    ActionContinue,
			expectedStatus:    ScoreStatusPending,
			minConfidence:     0.0,
			shouldContainText: "Waiting",
		},
		{
			name: "first step correct",
			steps: []TrajectoryStep{
				{Type: "tool", Name: "mytool"},
			},
			expectedScore:     1.0 / 3.0,
			expectedAction:    ActionContinue,
			expectedStatus:    ScoreStatusPartial,
			minConfidence:     0.3,
			shouldContainText: "On track",
		},
		{
			name: "two steps correct",
			steps: []TrajectoryStep{
				{Type: "tool", Name: "mytool"},
				{Type: "tool", Name: "mytool-b"},
			},
			expectedScore:     2.0 / 3.0,
			expectedAction:    ActionContinue,
			expectedStatus:    ScoreStatusPartial,
			minConfidence:     0.6,
			shouldContainText: "On track",
		},
		{
			name: "all steps correct",
			steps: []TrajectoryStep{
				{Type: "tool", Name: "mytool"},
				{Type: "tool", Name: "mytool-b"},
				{Type: "finding", Name: ""},
			},
			expectedScore:     1.0,
			expectedAction:    ActionContinue,
			expectedStatus:    ScoreStatusPartial,
			minConfidence:     1.0,
			shouldContainText: "complete",
		},
	}

	ctx := context.Background()
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			trajectory := Trajectory{
				Steps:     tt.steps,
				StartTime: time.Now(),
			}

			result, err := scorer.ScorePartial(ctx, trajectory)
			if err != nil {
				t.Fatalf("ScorePartial failed: %v", err)
			}

			if result.Score != tt.expectedScore {
				t.Errorf("Expected score %v, got %v", tt.expectedScore, result.Score)
			}

			if result.Action != tt.expectedAction {
				t.Errorf("Expected action %v, got %v", tt.expectedAction, result.Action)
			}

			if result.Status != tt.expectedStatus {
				t.Errorf("Expected status %v, got %v", tt.expectedStatus, result.Status)
			}

			if result.Confidence < tt.minConfidence {
				t.Errorf("Expected confidence >= %v, got %v", tt.minConfidence, result.Confidence)
			}

			if tt.shouldContainText != "" && !contains(result.Feedback, tt.shouldContainText) {
				t.Errorf("Expected feedback to contain %q, got %q", tt.shouldContainText, result.Feedback)
			}
		})
	}
}

func TestStreamingTrajectoryScorer_ExactMatch_Deviation(t *testing.T) {
	scorer := NewStreamingTrajectoryScorer(TrajectoryOptions{
		ExpectedSteps: []ExpectedStep{
			{Type: "tool", Name: "mytool", Required: true},
			{Type: "tool", Name: "mytool-b", Required: true},
		},
		Mode:          TrajectoryExactMatch,
		PenalizeExtra: 0.1,
	})

	ctx := context.Background()

	// Wrong step at position 0
	trajectory := Trajectory{
		Steps: []TrajectoryStep{
			{Type: "tool", Name: "hydra"}, // Should be mytool
		},
		StartTime: time.Now(),
	}

	result, err := scorer.ScorePartial(ctx, trajectory)
	if err != nil {
		t.Fatalf("ScorePartial failed: %v", err)
	}

	if result.Action != ActionReconsider {
		t.Errorf("Expected ActionReconsider for mismatch, got %v", result.Action)
	}

	if !contains(result.Feedback, "mismatch") {
		t.Errorf("Expected feedback to mention mismatch, got %q", result.Feedback)
	}

	details := result.Details
	if extraCount, ok := details["extra_count"].(int); !ok || extraCount != 1 {
		t.Errorf("Expected 1 extra step, got %v", details["extra_count"])
	}
}

func TestStreamingTrajectoryScorer_SubsetMatch(t *testing.T) {
	scorer := NewStreamingTrajectoryScorer(TrajectoryOptions{
		ExpectedSteps: []ExpectedStep{
			{Type: "tool", Name: "mytool", Required: true},
			{Type: "tool", Name: "mytool-b", Required: true},
			{Type: "finding", Name: "", Required: true},
		},
		Mode:          TrajectorySubsetMatch,
		PenalizeExtra: 0.05,
	})

	tests := []struct {
		name              string
		steps             []TrajectoryStep
		expectedScore     float64
		expectedAction    RecommendedAction
		minMatchedCount   int
		shouldContainText string
	}{
		{
			name: "out of order but all present",
			steps: []TrajectoryStep{
				{Type: "finding", Name: ""},
				{Type: "tool", Name: "mytool-b"},
				{Type: "tool", Name: "mytool"},
			},
			expectedScore:     1.0,
			expectedAction:    ActionContinue,
			minMatchedCount:   3,
			shouldContainText: "All 3 required steps found",
		},
		{
			name: "partial with extras",
			steps: []TrajectoryStep{
				{Type: "tool", Name: "mytool"},
				{Type: "tool", Name: "hydra"}, // extra
				{Type: "tool", Name: "mytool-b"},
			},
			expectedScore:     2.0/3.0 - 0.05, // 2 matched, 1 extra with penalty
			expectedAction:    ActionContinue,
			minMatchedCount:   2,
			shouldContainText: "Progress",
		},
		{
			name: "many extras, few matches",
			steps: []TrajectoryStep{
				{Type: "tool", Name: "hydra"},
				{Type: "tool", Name: "sqlmap"},
				{Type: "tool", Name: "mytool"},
			},
			expectedScore:     1.0/3.0 - 2*0.05, // 1 matched, 2 extras
			expectedAction:    ActionAdjust,
			minMatchedCount:   1,
			shouldContainText: "consider focusing",
		},
	}

	ctx := context.Background()
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			trajectory := Trajectory{
				Steps:     tt.steps,
				StartTime: time.Now(),
			}

			result, err := scorer.ScorePartial(ctx, trajectory)
			if err != nil {
				t.Fatalf("ScorePartial failed: %v", err)
			}

			// Allow small floating point variance
			scoreDiff := result.Score - tt.expectedScore
			if scoreDiff < -0.01 || scoreDiff > 0.01 {
				t.Errorf("Expected score ~%v, got %v", tt.expectedScore, result.Score)
			}

			if result.Action != tt.expectedAction {
				t.Errorf("Expected action %v, got %v", tt.expectedAction, result.Action)
			}

			matchedCount := result.Details["matched_count"].(int)
			if matchedCount < tt.minMatchedCount {
				t.Errorf("Expected at least %d matched, got %d", tt.minMatchedCount, matchedCount)
			}

			if !contains(result.Feedback, tt.shouldContainText) {
				t.Errorf("Expected feedback to contain %q, got %q", tt.shouldContainText, result.Feedback)
			}
		})
	}
}

func TestStreamingTrajectoryScorer_OrderedSubset(t *testing.T) {
	scorer := NewStreamingTrajectoryScorer(TrajectoryOptions{
		ExpectedSteps: []ExpectedStep{
			{Type: "tool", Name: "mytool", Required: true},
			{Type: "tool", Name: "mytool-b", Required: true},
			{Type: "finding", Name: "", Required: true},
		},
		Mode:          TrajectoryOrderedSubset,
		PenalizeExtra: 0.05,
	})

	tests := []struct {
		name              string
		steps             []TrajectoryStep
		expectedScore     float64
		expectedAction    RecommendedAction
		minMatchedCount   int
		shouldContainText string
	}{
		{
			name: "in order with extras",
			steps: []TrajectoryStep{
				{Type: "tool", Name: "mytool"},
				{Type: "tool", Name: "hydra"}, // extra
				{Type: "tool", Name: "mytool-b"},
				{Type: "tool", Name: "sqlmap"}, // extra
				{Type: "finding", Name: ""},
			},
			expectedScore:     1.0 - 2*0.05, // All matched, 2 extras
			expectedAction:    ActionContinue,
			minMatchedCount:   3,
			shouldContainText: "All 3 required steps found in order",
		},
		{
			name: "partial progress",
			steps: []TrajectoryStep{
				{Type: "tool", Name: "mytool"},
				{Type: "tool", Name: "hydra"}, // extra
			},
			expectedScore:     1.0/3.0 - 0.05, // 1 matched, 1 extra
			expectedAction:    ActionContinue,
			minMatchedCount:   1,
			shouldContainText: "Progress",
		},
		{
			name: "many extras with low progress",
			steps: []TrajectoryStep{
				{Type: "tool", Name: "mytool"},
				{Type: "tool", Name: "hydra"},
				{Type: "tool", Name: "sqlmap"},
			},
			expectedScore:     1.0/3.0 - 2*0.05, // 1 matched, 2 extras
			expectedAction:    ActionAdjust,
			minMatchedCount:   1,
			shouldContainText: "many extra steps",
		},
	}

	ctx := context.Background()
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			trajectory := Trajectory{
				Steps:     tt.steps,
				StartTime: time.Now(),
			}

			result, err := scorer.ScorePartial(ctx, trajectory)
			if err != nil {
				t.Fatalf("ScorePartial failed: %v", err)
			}

			// Allow small floating point variance
			scoreDiff := result.Score - tt.expectedScore
			if scoreDiff < -0.01 || scoreDiff > 0.01 {
				t.Errorf("Expected score ~%v, got %v", tt.expectedScore, result.Score)
			}

			if result.Action != tt.expectedAction {
				t.Errorf("Expected action %v, got %v", tt.expectedAction, result.Action)
			}

			matchedCount := result.Details["matched_count"].(int)
			if matchedCount < tt.minMatchedCount {
				t.Errorf("Expected at least %d matched, got %d", tt.minMatchedCount, matchedCount)
			}

			if !contains(result.Feedback, tt.shouldContainText) {
				t.Errorf("Expected feedback to contain %q, got %q", tt.shouldContainText, result.Feedback)
			}
		})
	}
}

func TestStreamingTrajectoryScorer_NoRequiredSteps(t *testing.T) {
	scorer := NewStreamingTrajectoryScorer(TrajectoryOptions{
		ExpectedSteps: []ExpectedStep{
			{Type: "tool", Name: "mytool", Required: false},
		},
		Mode: TrajectoryExactMatch,
	})

	ctx := context.Background()
	trajectory := Trajectory{
		Steps: []TrajectoryStep{
			{Type: "tool", Name: "hydra"},
		},
		StartTime: time.Now(),
	}

	result, err := scorer.ScorePartial(ctx, trajectory)
	if err != nil {
		t.Fatalf("ScorePartial failed: %v", err)
	}

	if result.Score != 1.0 {
		t.Errorf("Expected perfect score with no required steps, got %v", result.Score)
	}

	if result.Confidence != 1.0 {
		t.Errorf("Expected full confidence with no required steps, got %v", result.Confidence)
	}

	if result.Action != ActionContinue {
		t.Errorf("Expected ActionContinue, got %v", result.Action)
	}
}

func TestStreamingTrajectoryScorer_Confidence(t *testing.T) {
	scorer := NewStreamingTrajectoryScorer(TrajectoryOptions{
		ExpectedSteps: []ExpectedStep{
			{Type: "tool", Name: "mytool", Required: true},
			{Type: "tool", Name: "mytool-b", Required: true},
			{Type: "tool", Name: "sqlmap", Required: true},
			{Type: "finding", Name: "", Required: true},
		},
		Mode:          TrajectoryExactMatch,
		PenalizeExtra: 0.0,
	})

	tests := []struct {
		name          string
		steps         []TrajectoryStep
		minConfidence float64
		maxConfidence float64
	}{
		{
			name:          "empty trajectory",
			steps:         []TrajectoryStep{},
			minConfidence: 0.0,
			maxConfidence: 0.0,
		},
		{
			name: "25% complete",
			steps: []TrajectoryStep{
				{Type: "tool", Name: "mytool"},
			},
			minConfidence: 0.2,
			maxConfidence: 0.3,
		},
		{
			name: "50% complete",
			steps: []TrajectoryStep{
				{Type: "tool", Name: "mytool"},
				{Type: "tool", Name: "mytool-b"},
			},
			minConfidence: 0.4,
			maxConfidence: 0.6,
		},
		{
			name: "100% complete",
			steps: []TrajectoryStep{
				{Type: "tool", Name: "mytool"},
				{Type: "tool", Name: "mytool-b"},
				{Type: "tool", Name: "sqlmap"},
				{Type: "finding", Name: ""},
			},
			minConfidence: 1.0,
			maxConfidence: 1.0,
		},
	}

	ctx := context.Background()
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			trajectory := Trajectory{
				Steps:     tt.steps,
				StartTime: time.Now(),
			}

			result, err := scorer.ScorePartial(ctx, trajectory)
			if err != nil {
				t.Fatalf("ScorePartial failed: %v", err)
			}

			if result.Confidence < tt.minConfidence || result.Confidence > tt.maxConfidence {
				t.Errorf("Expected confidence in [%v, %v], got %v",
					tt.minConfidence, tt.maxConfidence, result.Confidence)
			}
		})
	}
}

func TestStreamingTrajectoryScorer_Details(t *testing.T) {
	scorer := NewStreamingTrajectoryScorer(TrajectoryOptions{
		ExpectedSteps: []ExpectedStep{
			{Type: "tool", Name: "mytool", Required: true},
			{Type: "tool", Name: "mytool-b", Required: true},
		},
		Mode:          TrajectoryOrderedSubset,
		PenalizeExtra: 0.1,
	})

	ctx := context.Background()
	trajectory := Trajectory{
		Steps: []TrajectoryStep{
			{Type: "tool", Name: "mytool"},
			{Type: "tool", Name: "hydra"}, // extra
		},
		StartTime: time.Now(),
	}

	result, err := scorer.ScorePartial(ctx, trajectory)
	if err != nil {
		t.Fatalf("ScorePartial failed: %v", err)
	}

	details := result.Details

	// Verify all expected keys are present
	requiredKeys := []string{"matched", "missing", "extra", "matched_count", "required_count", "extra_count", "mode"}
	for _, key := range requiredKeys {
		if _, exists := details[key]; !exists {
			t.Errorf("Expected details to contain key %q", key)
		}
	}

	// Verify types
	if _, ok := details["matched"].([]string); !ok {
		t.Errorf("Expected matched to be []string, got %T", details["matched"])
	}

	if matchedCount, ok := details["matched_count"].(int); !ok {
		t.Errorf("Expected matched_count to be int, got %T", details["matched_count"])
	} else if matchedCount != 1 {
		t.Errorf("Expected 1 matched, got %d", matchedCount)
	}

	if extraCount, ok := details["extra_count"].(int); !ok {
		t.Errorf("Expected extra_count to be int, got %T", details["extra_count"])
	} else if extraCount != 1 {
		t.Errorf("Expected 1 extra, got %d", extraCount)
	}
}
