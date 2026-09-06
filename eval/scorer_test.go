// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Zero Root AI

package eval

import (
	"context"
	"math"
	"testing"
)

// mockScorer is a simple scorer for testing
type mockScorer struct {
	name  string
	score float64
	err   error
}

func (m *mockScorer) Score(ctx context.Context, sample Sample) (ScoreResult, error) {
	if m.err != nil {
		return ScoreResult{}, m.err
	}
	return ScoreResult{Score: m.score}, nil
}

func (m *mockScorer) Name() string {
	return m.name
}

func TestValidateScore(t *testing.T) {
	tests := []struct {
		name    string
		score   float64
		wantErr bool
	}{
		{
			name:    "valid score 0.0",
			score:   0.0,
			wantErr: false,
		},
		{
			name:    "valid score 1.0",
			score:   1.0,
			wantErr: false,
		},
		{
			name:    "valid score 0.5",
			score:   0.5,
			wantErr: false,
		},
		{
			name:    "invalid score below 0",
			score:   -0.1,
			wantErr: true,
		},
		{
			name:    "invalid score above 1",
			score:   1.1,
			wantErr: true,
		},
		{
			name:    "invalid score NaN",
			score:   math.NaN(),
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateScore(tt.score)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateScore() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestAggregateScores(t *testing.T) {
	tests := []struct {
		name    string
		results []ScoreResult
		weights map[string]float64
		want    float64
	}{
		{
			name:    "empty results",
			results: []ScoreResult{},
			weights: nil,
			want:    0.0,
		},
		{
			name: "single result no weights",
			results: []ScoreResult{
				{Score: 0.8},
			},
			weights: nil,
			want:    0.8,
		},
		{
			name: "multiple results equal weights",
			results: []ScoreResult{
				{Score: 0.8},
				{Score: 0.6},
			},
			weights: nil,
			want:    0.7, // (0.8 + 0.6) / 2
		},
		{
			name: "multiple results all zeros",
			results: []ScoreResult{
				{Score: 0.0},
				{Score: 0.0},
			},
			weights: nil,
			want:    0.0,
		},
		{
			name: "multiple results all ones",
			results: []ScoreResult{
				{Score: 1.0},
				{Score: 1.0},
			},
			weights: nil,
			want:    1.0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := AggregateScores(tt.results, tt.weights)
			if math.Abs(got-tt.want) > 0.0001 {
				t.Errorf("AggregateScores() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestAggregateScoresWithNames(t *testing.T) {
	tests := []struct {
		name    string
		results map[string]ScoreResult
		weights map[string]float64
		want    float64
	}{
		{
			name:    "empty results",
			results: map[string]ScoreResult{},
			weights: nil,
			want:    0.0,
		},
		{
			name: "single result no weights",
			results: map[string]ScoreResult{
				"tool": {Score: 0.8},
			},
			weights: nil,
			want:    0.8,
		},
		{
			name: "multiple results equal weights",
			results: map[string]ScoreResult{
				"tool": {Score: 0.8},
				"task": {Score: 0.6},
			},
			weights: nil,
			want:    0.7, // (0.8 + 0.6) / 2
		},
		{
			name: "weighted average",
			results: map[string]ScoreResult{
				"tool": {Score: 0.8},
				"task": {Score: 0.6},
			},
			weights: map[string]float64{
				"tool": 0.7,
				"task": 0.3,
			},
			want: 0.74, // (0.8 * 0.7 + 0.6 * 0.3)
		},
		{
			name: "weighted average with normalization",
			results: map[string]ScoreResult{
				"tool": {Score: 0.8},
				"task": {Score: 0.6},
			},
			weights: map[string]float64{
				"tool": 7.0,
				"task": 3.0,
			},
			want: 0.74, // Same as above after normalization
		},
		{
			name: "weights for non-existent scorers ignored",
			results: map[string]ScoreResult{
				"tool": {Score: 0.8},
			},
			weights: map[string]float64{
				"tool":    0.5,
				"missing": 0.5,
			},
			want: 0.8, // Only tool result exists
		},
		{
			name: "all weights zero falls back to equal",
			results: map[string]ScoreResult{
				"tool": {Score: 0.8},
				"task": {Score: 0.6},
			},
			weights: map[string]float64{
				"tool": 0.0,
				"task": 0.0,
			},
			want: 0.7, // Equal weighting fallback
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := AggregateScoresWithNames(tt.results, tt.weights)
			if math.Abs(got-tt.want) > 0.0001 {
				t.Errorf("AggregateScoresWithNames() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestMockScorer(t *testing.T) {
	ctx := context.Background()

	scorer := &mockScorer{
		name:  "test",
		score: 0.85,
	}

	result, err := scorer.Score(ctx, Sample{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.Score != 0.85 {
		t.Errorf("expected score 0.85, got %v", result.Score)
	}

	if scorer.Name() != "test" {
		t.Errorf("expected name 'test', got %v", scorer.Name())
	}
}
