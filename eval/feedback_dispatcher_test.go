// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Zero Root AI

package eval

import (
	"context"
	"errors"
	"testing"
	"time"
)

// mockStreamingScorer is a test implementation of StreamingScorer.
type mockStreamingScorer struct {
	name           string
	score          PartialScore
	err            error
	delay          time.Duration
	supportsStream bool
}

func (m *mockStreamingScorer) Score(ctx context.Context, sample Sample) (ScoreResult, error) {
	return ScoreResult{Score: m.score.Score}, m.err
}

func (m *mockStreamingScorer) Name() string {
	return m.name
}

func (m *mockStreamingScorer) ScorePartial(ctx context.Context, trajectory Trajectory) (PartialScore, error) {
	if m.delay > 0 {
		select {
		case <-time.After(m.delay):
		case <-ctx.Done():
			return PartialScore{}, ctx.Err()
		}
	}
	return m.score, m.err
}

func (m *mockStreamingScorer) SupportsStreaming() bool {
	return m.supportsStream
}

func TestNewFeedbackDispatcher(t *testing.T) {
	tests := []struct {
		name       string
		scorers    []StreamingScorer
		thresholds ThresholdConfig
		wantWarn   float64
		wantCrit   float64
	}{
		{
			name: "default thresholds",
			scorers: []StreamingScorer{
				&mockStreamingScorer{name: "test"},
			},
			thresholds: ThresholdConfig{},
			wantWarn:   0.5,
			wantCrit:   0.2,
		},
		{
			name: "custom thresholds",
			scorers: []StreamingScorer{
				&mockStreamingScorer{name: "test"},
			},
			thresholds: ThresholdConfig{
				Warning:  0.7,
				Critical: 0.3,
			},
			wantWarn: 0.7,
			wantCrit: 0.3,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fd := NewFeedbackDispatcher(tt.scorers, tt.thresholds)
			if fd == nil {
				t.Fatal("NewFeedbackDispatcher returned nil")
			}
			if fd.thresholds.Warning != tt.wantWarn {
				t.Errorf("Warning threshold = %v, want %v", fd.thresholds.Warning, tt.wantWarn)
			}
			if fd.thresholds.Critical != tt.wantCrit {
				t.Errorf("Critical threshold = %v, want %v", fd.thresholds.Critical, tt.wantCrit)
			}
		})
	}
}

func TestFeedbackDispatcher_Evaluate(t *testing.T) {
	tests := []struct {
		name         string
		scorers      []StreamingScorer
		thresholds   ThresholdConfig
		trajectory   Trajectory
		wantErr      bool
		wantOverall  float64
		wantAlerts   int
		wantAction   RecommendedAction
		checkDetails bool
	}{
		{
			name:    "empty scorer list",
			scorers: []StreamingScorer{},
			trajectory: Trajectory{
				Steps: []TrajectoryStep{{Type: "test"}},
			},
			wantErr: true,
		},
		{
			name: "all scorers succeed - high score",
			scorers: []StreamingScorer{
				&mockStreamingScorer{
					name: "scorer1",
					score: PartialScore{
						Score:      0.9,
						Confidence: 0.8,
						Status:     ScoreStatusPartial,
						Action:     ActionContinue,
					},
					supportsStream: true,
				},
				&mockStreamingScorer{
					name: "scorer2",
					score: PartialScore{
						Score:      0.85,
						Confidence: 0.9,
						Status:     ScoreStatusPartial,
						Action:     ActionContinue,
					},
					supportsStream: true,
				},
			},
			thresholds: ThresholdConfig{Warning: 0.5, Critical: 0.2},
			trajectory: Trajectory{
				Steps: []TrajectoryStep{{Type: "test"}},
			},
			wantErr:     false,
			wantOverall: 0.875, // (0.9 + 0.85) / 2
			wantAlerts:  0,
			wantAction:  ActionContinue,
		},
		{
			name: "warning threshold breach",
			scorers: []StreamingScorer{
				&mockStreamingScorer{
					name: "scorer1",
					score: PartialScore{
						Score:      0.4,
						Confidence: 0.8,
						Status:     ScoreStatusPartial,
						Action:     ActionAdjust,
					},
					supportsStream: true,
				},
			},
			thresholds: ThresholdConfig{Warning: 0.5, Critical: 0.2},
			trajectory: Trajectory{
				Steps: []TrajectoryStep{{Type: "test"}},
			},
			wantErr:     false,
			wantOverall: 0.4,
			wantAlerts:  2, // Overall + individual
			wantAction:  ActionAdjust,
		},
		{
			name: "critical threshold breach",
			scorers: []StreamingScorer{
				&mockStreamingScorer{
					name: "scorer1",
					score: PartialScore{
						Score:      0.15,
						Confidence: 0.8,
						Status:     ScoreStatusPartial,
						Action:     ActionReconsider,
					},
					supportsStream: true,
				},
			},
			thresholds: ThresholdConfig{Warning: 0.5, Critical: 0.2},
			trajectory: Trajectory{
				Steps: []TrajectoryStep{{Type: "test"}},
			},
			wantErr:     false,
			wantOverall: 0.15,
			wantAlerts:  2, // Overall + individual
			wantAction:  ActionReconsider,
		},
		{
			name: "some scorers fail",
			scorers: []StreamingScorer{
				&mockStreamingScorer{
					name: "scorer1",
					score: PartialScore{
						Score:      0.8,
						Confidence: 0.9,
						Status:     ScoreStatusPartial,
						Action:     ActionContinue,
					},
					supportsStream: true,
				},
				&mockStreamingScorer{
					name:           "scorer2",
					err:            errors.New("scorer error"),
					supportsStream: true,
				},
			},
			thresholds: ThresholdConfig{Warning: 0.5, Critical: 0.2},
			trajectory: Trajectory{
				Steps: []TrajectoryStep{{Type: "test"}},
			},
			wantErr:      false,
			wantOverall:  0.8, // Only successful scorer
			wantAlerts:   0,
			wantAction:   ActionContinue,
			checkDetails: true,
		},
		{
			name: "all scorers timeout",
			scorers: []StreamingScorer{
				&mockStreamingScorer{
					name:           "scorer1",
					delay:          10 * time.Second, // Longer than timeout
					supportsStream: true,
				},
			},
			thresholds: ThresholdConfig{Warning: 0.5, Critical: 0.2},
			trajectory: Trajectory{
				Steps: []TrajectoryStep{{Type: "test"}},
			},
			wantErr: true,
		},
		{
			name: "low confidence scores excluded from average",
			scorers: []StreamingScorer{
				&mockStreamingScorer{
					name: "scorer1",
					score: PartialScore{
						Score:      0.9,
						Confidence: 0.9,
						Status:     ScoreStatusPartial,
						Action:     ActionContinue,
					},
					supportsStream: true,
				},
				&mockStreamingScorer{
					name: "scorer2",
					score: PartialScore{
						Score:      0.1, // Low score
						Confidence: 0.3, // Low confidence - should be excluded
						Status:     ScoreStatusPending,
						Action:     ActionContinue,
					},
					supportsStream: true,
				},
			},
			thresholds: ThresholdConfig{Warning: 0.5, Critical: 0.2},
			trajectory: Trajectory{
				Steps: []TrajectoryStep{{Type: "test"}},
			},
			wantErr:     false,
			wantOverall: 0.9, // Only high-confidence scorer
			wantAlerts:  0,
			wantAction:  ActionContinue,
		},
		{
			name: "abort action propagates",
			scorers: []StreamingScorer{
				&mockStreamingScorer{
					name: "scorer1",
					score: PartialScore{
						Score:      0.1,
						Confidence: 0.9,
						Status:     ScoreStatusPartial,
						Action:     ActionAbort,
					},
					supportsStream: true,
				},
			},
			thresholds: ThresholdConfig{Warning: 0.5, Critical: 0.2},
			trajectory: Trajectory{
				Steps: []TrajectoryStep{{Type: "test"}},
			},
			wantErr:     false,
			wantOverall: 0.1,
			wantAlerts:  2,
			wantAction:  ActionAbort,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fd := NewFeedbackDispatcher(tt.scorers, tt.thresholds)
			ctx := context.Background()

			feedback, err := fd.Evaluate(ctx, tt.trajectory)

			if (err != nil) != tt.wantErr {
				t.Errorf("Evaluate() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if tt.wantErr {
				return
			}

			if feedback == nil {
				t.Fatal("Evaluate() returned nil feedback")
			}

			// Check overall score (with tolerance for floating point)
			const tolerance = 0.01
			if abs(feedback.Overall.Score-tt.wantOverall) > tolerance {
				t.Errorf("Overall score = %v, want %v", feedback.Overall.Score, tt.wantOverall)
			}

			// Check alerts
			if len(feedback.Alerts) != tt.wantAlerts {
				t.Errorf("Alerts count = %v, want %v", len(feedback.Alerts), tt.wantAlerts)
			}

			// Check action
			if feedback.Overall.Action != tt.wantAction {
				t.Errorf("Overall action = %v, want %v", feedback.Overall.Action, tt.wantAction)
			}

			// Check details if requested
			if tt.checkDetails {
				if _, ok := feedback.Overall.Details["failed_scorers"]; !ok {
					t.Error("Expected failed_scorers in details")
				}
			}

			// Check feedback is not consumed initially
			if feedback.Consumed {
				t.Error("Feedback should not be consumed initially")
			}

			// Check timestamp is recent
			if time.Since(feedback.Timestamp) > time.Second {
				t.Error("Feedback timestamp is not recent")
			}

			// Check step index
			if feedback.StepIndex != len(tt.trajectory.Steps) {
				t.Errorf("StepIndex = %v, want %v", feedback.StepIndex, len(tt.trajectory.Steps))
			}
		})
	}
}

func TestFeedbackDispatcher_ParallelExecution(t *testing.T) {
	// Create scorers with different delays to ensure parallel execution
	scorers := []StreamingScorer{
		&mockStreamingScorer{
			name:  "slow",
			delay: 100 * time.Millisecond,
			score: PartialScore{
				Score:      0.8,
				Confidence: 0.9,
				Status:     ScoreStatusPartial,
				Action:     ActionContinue,
			},
			supportsStream: true,
		},
		&mockStreamingScorer{
			name:  "fast",
			delay: 10 * time.Millisecond,
			score: PartialScore{
				Score:      0.9,
				Confidence: 0.9,
				Status:     ScoreStatusPartial,
				Action:     ActionContinue,
			},
			supportsStream: true,
		},
	}

	fd := NewFeedbackDispatcher(scorers, DefaultThresholdConfig())
	ctx := context.Background()

	start := time.Now()
	feedback, err := fd.Evaluate(ctx, Trajectory{Steps: []TrajectoryStep{{Type: "test"}}})
	duration := time.Since(start)

	if err != nil {
		t.Fatalf("Evaluate() error = %v", err)
	}

	if feedback == nil {
		t.Fatal("Evaluate() returned nil feedback")
	}

	// If parallel, should take ~100ms (slow scorer time)
	// If sequential, would take ~110ms (slow + fast)
	// Allow some margin for test overhead
	if duration > 200*time.Millisecond {
		t.Errorf("Evaluation took too long (%v), scorers may not be running in parallel", duration)
	}

	// Both scorers should have results
	if len(feedback.Scores) != 2 {
		t.Errorf("Expected 2 scorer results, got %v", len(feedback.Scores))
	}
}

func TestDefaultThresholdConfig(t *testing.T) {
	config := DefaultThresholdConfig()

	if config.Warning != 0.5 {
		t.Errorf("DefaultThresholdConfig().Warning = %v, want 0.5", config.Warning)
	}
	if config.Critical != 0.2 {
		t.Errorf("DefaultThresholdConfig().Critical = %v, want 0.2", config.Critical)
	}
}

// Helper functions

func abs(x float64) float64 {
	if x < 0 {
		return -x
	}
	return x
}
