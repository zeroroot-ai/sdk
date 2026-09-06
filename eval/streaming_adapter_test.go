// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Zero Root AI

package eval

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"
)

// streamingTestScorer is a scorer for testing the streaming adapter.
// We define a separate type to avoid conflicts with mockScorer in scorer_test.go
type streamingTestScorer struct {
	name    string
	score   float64
	details map[string]any
	err     error
}

func (m *streamingTestScorer) Score(ctx context.Context, sample Sample) (ScoreResult, error) {
	if m.err != nil {
		return ScoreResult{}, m.err
	}
	return ScoreResult{
		Score:   m.score,
		Details: m.details,
	}, nil
}

func (m *streamingTestScorer) Name() string {
	return m.name
}

var errTestScorer = errors.New("test scorer error")

func TestNewStreamingAdapter(t *testing.T) {
	tests := []struct {
		name        string
		scorer      Scorer
		opts        StreamingAdapterOptions
		wantOpts    StreamingAdapterOptions
		description string
	}{
		{
			name:   "default options",
			scorer: &streamingTestScorer{name: "test", score: 0.8},
			opts:   DefaultStreamingAdapterOptions(),
			wantOpts: StreamingAdapterOptions{
				MinStepsForEval:    1,
				PartialScoreWeight: 0.8,
			},
			description: "should use default options when provided",
		},
		{
			name:   "custom options",
			scorer: &streamingTestScorer{name: "test", score: 0.8},
			opts: StreamingAdapterOptions{
				MinStepsForEval:    5,
				PartialScoreWeight: 0.6,
			},
			wantOpts: StreamingAdapterOptions{
				MinStepsForEval:    5,
				PartialScoreWeight: 0.6,
			},
			description: "should preserve custom options",
		},
		{
			name:   "invalid min steps - zero",
			scorer: &streamingTestScorer{name: "test", score: 0.8},
			opts: StreamingAdapterOptions{
				MinStepsForEval:    0,
				PartialScoreWeight: 0.8,
			},
			wantOpts: StreamingAdapterOptions{
				MinStepsForEval:    1, // Should be corrected to 1
				PartialScoreWeight: 0.8,
			},
			description: "should correct zero MinStepsForEval to 1",
		},
		{
			name:   "invalid min steps - negative",
			scorer: &streamingTestScorer{name: "test", score: 0.8},
			opts: StreamingAdapterOptions{
				MinStepsForEval:    -5,
				PartialScoreWeight: 0.8,
			},
			wantOpts: StreamingAdapterOptions{
				MinStepsForEval:    1, // Should be corrected to 1
				PartialScoreWeight: 0.8,
			},
			description: "should correct negative MinStepsForEval to 1",
		},
		{
			name:   "invalid partial weight - zero",
			scorer: &streamingTestScorer{name: "test", score: 0.8},
			opts: StreamingAdapterOptions{
				MinStepsForEval:    1,
				PartialScoreWeight: 0.0,
			},
			wantOpts: StreamingAdapterOptions{
				MinStepsForEval:    1,
				PartialScoreWeight: 0.8, // Should be corrected to 0.8
			},
			description: "should correct zero PartialScoreWeight to 0.8",
		},
		{
			name:   "invalid partial weight - too high",
			scorer: &streamingTestScorer{name: "test", score: 0.8},
			opts: StreamingAdapterOptions{
				MinStepsForEval:    1,
				PartialScoreWeight: 1.5,
			},
			wantOpts: StreamingAdapterOptions{
				MinStepsForEval:    1,
				PartialScoreWeight: 0.8, // Should be corrected to 0.8
			},
			description: "should correct too-high PartialScoreWeight to 0.8",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			adapter := NewStreamingAdapter(tt.scorer, tt.opts)

			// Verify the adapter implements StreamingScorer
			_, ok := adapter.(StreamingScorer)
			if !ok {
				t.Fatal("NewStreamingAdapter should return a StreamingScorer")
			}

			// Verify internal options (access via type assertion for testing)
			sa := adapter.(*streamingAdapter)
			if sa.opts.MinStepsForEval != tt.wantOpts.MinStepsForEval {
				t.Errorf("MinStepsForEval = %d, want %d",
					sa.opts.MinStepsForEval, tt.wantOpts.MinStepsForEval)
			}
			if sa.opts.PartialScoreWeight != tt.wantOpts.PartialScoreWeight {
				t.Errorf("PartialScoreWeight = %f, want %f",
					sa.opts.PartialScoreWeight, tt.wantOpts.PartialScoreWeight)
			}
		})
	}
}

func TestStreamingAdapter_Score(t *testing.T) {
	tests := []struct {
		name      string
		scorer    *streamingTestScorer
		sample    Sample
		wantScore float64
		wantErr   bool
	}{
		{
			name:      "delegates to inner scorer",
			scorer:    &streamingTestScorer{name: "test", score: 0.85, details: map[string]any{"key": "value"}},
			sample:    Sample{ID: "test-1"},
			wantScore: 0.85,
			wantErr:   false,
		},
		{
			name:      "propagates errors from inner scorer",
			scorer:    &streamingTestScorer{name: "test", score: 0.0, err: errTestScorer},
			sample:    Sample{ID: "test-2"},
			wantScore: 0.0,
			wantErr:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			adapter := NewStreamingAdapter(tt.scorer, DefaultStreamingAdapterOptions())
			ctx := context.Background()

			result, err := adapter.Score(ctx, tt.sample)

			if (err != nil) != tt.wantErr {
				t.Errorf("Score() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if !tt.wantErr && result.Score != tt.wantScore {
				t.Errorf("Score() score = %v, want %v", result.Score, tt.wantScore)
			}
		})
	}
}

func TestStreamingAdapter_Name(t *testing.T) {
	scorer := &streamingTestScorer{name: "my-custom-scorer", score: 0.5}
	adapter := NewStreamingAdapter(scorer, DefaultStreamingAdapterOptions())

	if got := adapter.Name(); got != "my-custom-scorer" {
		t.Errorf("Name() = %v, want %v", got, "my-custom-scorer")
	}
}

func TestStreamingAdapter_SupportsStreaming(t *testing.T) {
	scorer := &streamingTestScorer{name: "test", score: 0.5}
	adapter := NewStreamingAdapter(scorer, DefaultStreamingAdapterOptions())

	if !adapter.SupportsStreaming() {
		t.Error("SupportsStreaming() should return true for streaming adapter")
	}
}

func TestStreamingAdapter_ScorePartial(t *testing.T) {
	baseTime := time.Now()

	tests := []struct {
		name              string
		scorer            *streamingTestScorer
		opts              StreamingAdapterOptions
		trajectory        Trajectory
		wantStatus        ScoreStatus
		wantAction        RecommendedAction
		wantMinConfidence float64
		wantMaxConfidence float64
		wantScoreRange    [2]float64 // [min, max]
	}{
		{
			name:   "insufficient steps - pending",
			scorer: &streamingTestScorer{name: "test", score: 0.8},
			opts: StreamingAdapterOptions{
				MinStepsForEval:    3,
				PartialScoreWeight: 0.8,
			},
			trajectory: Trajectory{
				Steps: []TrajectoryStep{
					{Type: "tool", Name: "mytool", StartTime: baseTime},
				},
				StartTime: baseTime,
			},
			wantStatus:        ScoreStatusPending,
			wantAction:        ActionContinue,
			wantMinConfidence: 0.2,
			wantMaxConfidence: 0.4,
			wantScoreRange:    [2]float64{0.0, 0.0}, // Score is 0 when pending
		},
		{
			name:   "minimum steps met - partial evaluation",
			scorer: &streamingTestScorer{name: "test", score: 0.75},
			opts: StreamingAdapterOptions{
				MinStepsForEval:    2,
				PartialScoreWeight: 0.8,
			},
			trajectory: Trajectory{
				Steps: []TrajectoryStep{
					{Type: "tool", Name: "mytool", StartTime: baseTime},
					{Type: "tool", Name: "mytool-b", StartTime: baseTime.Add(time.Second)},
				},
				StartTime: baseTime,
			},
			wantStatus:        ScoreStatusPartial,
			wantAction:        ActionContinue,
			wantMinConfidence: 0.5,
			wantMaxConfidence: 0.8,
			wantScoreRange:    [2]float64{0.75, 0.75}, // Exact score from mock
		},
		{
			name:   "many steps - higher confidence",
			scorer: &streamingTestScorer{name: "test", score: 0.9},
			opts: StreamingAdapterOptions{
				MinStepsForEval:    2,
				PartialScoreWeight: 0.8,
			},
			trajectory: Trajectory{
				Steps: []TrajectoryStep{
					{Type: "tool", Name: "mytool", StartTime: baseTime},
					{Type: "tool", Name: "mytool-b", StartTime: baseTime.Add(time.Second)},
					{Type: "tool", Name: "sqlmap", StartTime: baseTime.Add(2 * time.Second)},
					{Type: "tool", Name: "hydra", StartTime: baseTime.Add(3 * time.Second)},
					{Type: "tool", Name: "metasploit", StartTime: baseTime.Add(4 * time.Second)},
				},
				StartTime: baseTime,
			},
			wantStatus:        ScoreStatusPartial,
			wantAction:        ActionContinue,
			wantMinConfidence: 0.7,
			wantMaxConfidence: 0.85,
			wantScoreRange:    [2]float64{0.9, 0.9},
		},
		{
			name:   "low score - recommend reconsider",
			scorer: &streamingTestScorer{name: "test", score: 0.2},
			opts: StreamingAdapterOptions{
				MinStepsForEval:    1,
				PartialScoreWeight: 0.9, // High weight for strong recommendation
			},
			trajectory: Trajectory{
				Steps: []TrajectoryStep{
					{Type: "tool", Name: "wrong-tool", StartTime: baseTime},
					{Type: "tool", Name: "wrong-tool-2", StartTime: baseTime.Add(time.Second)},
					{Type: "tool", Name: "wrong-tool-3", StartTime: baseTime.Add(2 * time.Second)},
					{Type: "tool", Name: "wrong-tool-4", StartTime: baseTime.Add(3 * time.Second)},
				},
				StartTime: baseTime,
			},
			wantStatus:        ScoreStatusPartial,
			wantAction:        ActionReconsider,
			wantMinConfidence: 0.7,
			wantMaxConfidence: 1.0,
			wantScoreRange:    [2]float64{0.2, 0.2},
		},
		{
			name:   "medium-low score - recommend adjust",
			scorer: &streamingTestScorer{name: "test", score: 0.4},
			opts: StreamingAdapterOptions{
				MinStepsForEval:    1,
				PartialScoreWeight: 0.9,
			},
			trajectory: Trajectory{
				Steps: []TrajectoryStep{
					{Type: "tool", Name: "tool1", StartTime: baseTime},
					{Type: "tool", Name: "tool2", StartTime: baseTime.Add(time.Second)},
					{Type: "tool", Name: "tool3", StartTime: baseTime.Add(2 * time.Second)},
					{Type: "tool", Name: "tool4", StartTime: baseTime.Add(3 * time.Second)},
				},
				StartTime: baseTime,
			},
			wantStatus:        ScoreStatusPartial,
			wantAction:        ActionAdjust,
			wantMinConfidence: 0.7,
			wantMaxConfidence: 1.0,
			wantScoreRange:    [2]float64{0.4, 0.4},
		},
		{
			name:   "just above minimum - reduced confidence",
			scorer: &streamingTestScorer{name: "test", score: 0.8},
			opts: StreamingAdapterOptions{
				MinStepsForEval:    3,
				PartialScoreWeight: 0.8,
			},
			trajectory: Trajectory{
				Steps: []TrajectoryStep{
					{Type: "tool", Name: "tool1", StartTime: baseTime},
					{Type: "tool", Name: "tool2", StartTime: baseTime.Add(time.Second)},
					{Type: "tool", Name: "tool3", StartTime: baseTime.Add(2 * time.Second)},
				},
				StartTime: baseTime,
			},
			wantStatus:        ScoreStatusPartial,
			wantAction:        ActionContinue,
			wantMinConfidence: 0.5, // Reduced due to being just at minimum
			wantMaxConfidence: 0.75,
			wantScoreRange:    [2]float64{0.8, 0.8},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			adapter := NewStreamingAdapter(tt.scorer, tt.opts)
			ctx := context.Background()

			result, err := adapter.ScorePartial(ctx, tt.trajectory)
			if err != nil {
				t.Fatalf("ScorePartial() unexpected error: %v", err)
			}

			// Check status
			if result.Status != tt.wantStatus {
				t.Errorf("Status = %v, want %v", result.Status, tt.wantStatus)
			}

			// Check action
			if result.Action != tt.wantAction {
				t.Errorf("Action = %v, want %v", result.Action, tt.wantAction)
			}

			// Check confidence range
			if result.Confidence < tt.wantMinConfidence {
				t.Errorf("Confidence = %v, want >= %v", result.Confidence, tt.wantMinConfidence)
			}
			if result.Confidence > tt.wantMaxConfidence {
				t.Errorf("Confidence = %v, want <= %v", result.Confidence, tt.wantMaxConfidence)
			}

			// Check score range
			if result.Score < tt.wantScoreRange[0] {
				t.Errorf("Score = %v, want >= %v", result.Score, tt.wantScoreRange[0])
			}
			if result.Score > tt.wantScoreRange[1] {
				t.Errorf("Score = %v, want <= %v", result.Score, tt.wantScoreRange[1])
			}

			// Check feedback is not empty
			if result.Feedback == "" {
				t.Error("Feedback should not be empty")
			}

			// Verify feedback contains useful information
			t.Logf("Feedback: %s", result.Feedback)
		})
	}
}

func TestStreamingAdapter_ScorePartial_InnerScorerError(t *testing.T) {
	scorer := &streamingTestScorer{
		name:  "failing-scorer",
		score: 0.0,
		err:   errTestScorer,
	}

	adapter := NewStreamingAdapter(scorer, DefaultStreamingAdapterOptions())
	ctx := context.Background()

	trajectory := Trajectory{
		Steps: []TrajectoryStep{
			{Type: "tool", Name: "mytool", StartTime: time.Now()},
		},
		StartTime: time.Now(),
	}

	_, err := adapter.ScorePartial(ctx, trajectory)
	if err == nil {
		t.Fatal("ScorePartial() should return error when inner scorer fails")
	}

	// Error should mention inner scorer failure
	errMsg := err.Error()
	if errMsg == "" {
		t.Error("Error message should not be empty")
	}
	t.Logf("Error message: %s", errMsg)
}

func TestStreamingAdapter_ConfidenceCalculation(t *testing.T) {
	baseTime := time.Now()

	// Test confidence decreases for trajectories just above minimum
	scorer := &streamingTestScorer{name: "test", score: 0.8}
	opts := StreamingAdapterOptions{
		MinStepsForEval:    5,
		PartialScoreWeight: 0.9,
	}

	testCases := []struct {
		numSteps          int
		wantMinConfidence float64
		wantMaxConfidence float64
	}{
		{5, 0.6, 0.75},  // Just at minimum (3 steps short of +3)
		{6, 0.7, 0.82},  // 1 step above
		{7, 0.75, 0.88}, // 2 steps above
		{8, 0.8, 0.92},  // 3 steps above (no reduction)
	}

	for _, tc := range testCases {
		t.Run(fmt.Sprintf("%d_steps", tc.numSteps), func(t *testing.T) {
			adapter := NewStreamingAdapter(scorer, opts)
			ctx := context.Background()

			// Build trajectory with specified number of steps
			steps := make([]TrajectoryStep, tc.numSteps)
			for i := range tc.numSteps {
				steps[i] = TrajectoryStep{
					Type:      "tool",
					Name:      fmt.Sprintf("tool-%d", i),
					StartTime: baseTime.Add(time.Duration(i) * time.Second),
				}
			}

			trajectory := Trajectory{
				Steps:     steps,
				StartTime: baseTime,
			}

			result, err := adapter.ScorePartial(ctx, trajectory)
			if err != nil {
				t.Fatalf("ScorePartial() unexpected error: %v", err)
			}

			if result.Confidence < tc.wantMinConfidence {
				t.Errorf("Confidence = %v, want >= %v", result.Confidence, tc.wantMinConfidence)
			}
			if result.Confidence > tc.wantMaxConfidence {
				t.Errorf("Confidence = %v, want <= %v", result.Confidence, tc.wantMaxConfidence)
			}

			t.Logf("Steps: %d, Confidence: %.3f", tc.numSteps, result.Confidence)
		})
	}
}

func TestStreamingAdapter_ActionRecommendations(t *testing.T) {
	baseTime := time.Now()
	opts := StreamingAdapterOptions{
		MinStepsForEval:    1,
		PartialScoreWeight: 0.9, // High confidence for clear recommendations
	}

	// Create trajectory with sufficient steps for high confidence
	trajectory := Trajectory{
		Steps: []TrajectoryStep{
			{Type: "tool", Name: "tool1", StartTime: baseTime},
			{Type: "tool", Name: "tool2", StartTime: baseTime.Add(time.Second)},
			{Type: "tool", Name: "tool3", StartTime: baseTime.Add(2 * time.Second)},
			{Type: "tool", Name: "tool4", StartTime: baseTime.Add(3 * time.Second)},
		},
		StartTime: baseTime,
	}

	testCases := []struct {
		score      float64
		wantAction RecommendedAction
	}{
		{0.1, ActionReconsider},
		{0.25, ActionReconsider},
		{0.35, ActionAdjust},
		{0.45, ActionAdjust},
		{0.55, ActionContinue},
		{0.75, ActionContinue},
		{0.95, ActionContinue},
	}

	for _, tc := range testCases {
		t.Run(fmt.Sprintf("score_%.2f", tc.score), func(t *testing.T) {
			scorer := &streamingTestScorer{name: "test", score: tc.score}
			adapter := NewStreamingAdapter(scorer, opts)
			ctx := context.Background()

			result, err := adapter.ScorePartial(ctx, trajectory)
			if err != nil {
				t.Fatalf("ScorePartial() unexpected error: %v", err)
			}

			if result.Action != tc.wantAction {
				t.Errorf("Action = %v, want %v (score: %.2f, confidence: %.2f)",
					result.Action, tc.wantAction, result.Score, result.Confidence)
			}

			t.Logf("Score: %.2f, Confidence: %.2f, Action: %s, Feedback: %s",
				result.Score, result.Confidence, result.Action, result.Feedback)
		})
	}
}
