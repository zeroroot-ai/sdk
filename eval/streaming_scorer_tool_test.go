// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Zero Root AI

package eval

import (
	"context"
	"math"
	"testing"
	"time"
)

func TestNewStreamingToolCorrectnessScorer(t *testing.T) {
	scorer := NewStreamingToolCorrectnessScorer(ToolCorrectnessOptions{
		ExpectedTools: []ExpectedToolCall{
			{Name: "mytool", Required: true},
			{Name: "http-client", Required: true},
		},
		OrderMatters: true,
	})

	if scorer == nil {
		t.Fatal("Expected non-nil scorer")
	}

	if !scorer.SupportsStreaming() {
		t.Error("Expected SupportsStreaming() to return true")
	}

	if scorer.Name() != "tool_correctness" {
		t.Errorf("Expected name 'tool_correctness', got %q", scorer.Name())
	}
}

func TestStreamingToolCorrectnessScorer_OrderedPrefixMatch(t *testing.T) {
	tests := []struct {
		name               string
		expected           []ExpectedToolCall
		actual             []TrajectoryStep
		expectedScore      float64
		expectedConfidence float64
		expectedStatus     ScoreStatus
		expectedAction     RecommendedAction
	}{
		{
			name: "no tools called yet",
			expected: []ExpectedToolCall{
				{Name: "mytool", Required: true},
				{Name: "http-client", Required: true},
			},
			actual:             []TrajectoryStep{},
			expectedScore:      0.0,
			expectedConfidence: 0.0,
			expectedStatus:     ScoreStatusPending,
			expectedAction:     ActionContinue,
		},
		{
			name: "first tool correct",
			expected: []ExpectedToolCall{
				{Name: "mytool", Required: true},
				{Name: "http-client", Required: true},
				{Name: "sqlmap", Required: true},
			},
			actual: []TrajectoryStep{
				{Type: "tool", Name: "mytool", Input: map[string]any{}},
			},
			expectedScore:      0.33,
			expectedConfidence: 0.33,
			expectedStatus:     ScoreStatusPartial,
			expectedAction:     ActionContinue,
		},
		{
			name: "two tools correct",
			expected: []ExpectedToolCall{
				{Name: "mytool", Required: true},
				{Name: "http-client", Required: true},
				{Name: "sqlmap", Required: true},
				{Name: "exploit", Required: true},
			},
			actual: []TrajectoryStep{
				{Type: "tool", Name: "mytool", Input: map[string]any{}},
				{Type: "tool", Name: "http-client", Input: map[string]any{}},
			},
			expectedScore:      0.5,
			expectedConfidence: 0.5,
			expectedStatus:     ScoreStatusPartial,
			expectedAction:     ActionContinue,
		},
		{
			name: "wrong tool in sequence",
			expected: []ExpectedToolCall{
				{Name: "mytool", Required: true},
				{Name: "http-client", Required: true},
				{Name: "sqlmap", Required: true},
			},
			actual: []TrajectoryStep{
				{Type: "tool", Name: "mytool", Input: map[string]any{}},
				{Type: "tool", Name: "sqlmap", Input: map[string]any{}},
			},
			expectedScore:      0.33,
			expectedConfidence: 0.67,
			expectedStatus:     ScoreStatusPartial,
			expectedAction:     ActionReconsider, // 1/3 = 33% mismatch rate > 30% threshold
		},
		{
			name: "all tools correct",
			expected: []ExpectedToolCall{
				{Name: "mytool", Required: true},
				{Name: "http-client", Required: true},
			},
			actual: []TrajectoryStep{
				{Type: "tool", Name: "mytool", Input: map[string]any{}},
				{Type: "tool", Name: "http-client", Input: map[string]any{}},
			},
			expectedScore:      1.0,
			expectedConfidence: 1.0,
			expectedStatus:     ScoreStatusComplete,
			expectedAction:     ActionContinue,
		},
		{
			name: "high mismatch rate triggers reconsider",
			expected: []ExpectedToolCall{
				{Name: "mytool", Required: true},
				{Name: "http-client", Required: true},
				{Name: "sqlmap", Required: true},
			},
			actual: []TrajectoryStep{
				{Type: "tool", Name: "wrong-tool", Input: map[string]any{}},
				{Type: "tool", Name: "another-wrong", Input: map[string]any{}},
			},
			expectedScore:      0.0,
			expectedConfidence: 0.67,
			expectedStatus:     ScoreStatusPartial,
			expectedAction:     ActionReconsider,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			scorer := NewStreamingToolCorrectnessScorer(ToolCorrectnessOptions{
				ExpectedTools: tt.expected,
				OrderMatters:  true,
			})

			trajectory := Trajectory{
				Steps:     tt.actual,
				StartTime: time.Now(),
			}

			result, err := scorer.ScorePartial(context.Background(), trajectory)
			if err != nil {
				t.Fatalf("Unexpected error: %v", err)
			}

			// Allow small floating point tolerance
			if math.Abs(result.Score-tt.expectedScore) > 0.01 {
				t.Errorf("Expected score %0.2f, got %0.2f", tt.expectedScore, result.Score)
			}

			if math.Abs(result.Confidence-tt.expectedConfidence) > 0.01 {
				t.Errorf("Expected confidence %0.2f, got %0.2f", tt.expectedConfidence, result.Confidence)
			}

			if result.Status != tt.expectedStatus {
				t.Errorf("Expected status %q, got %q", tt.expectedStatus, result.Status)
			}

			if result.Action != tt.expectedAction {
				t.Errorf("Expected action %q, got %q", tt.expectedAction, result.Action)
			}

			if result.Feedback == "" {
				t.Error("Expected non-empty feedback message")
			}

			t.Logf("Feedback: %s", result.Feedback)
		})
	}
}

func TestStreamingToolCorrectnessScorer_UnorderedMatch(t *testing.T) {
	tests := []struct {
		name              string
		expected          []ExpectedToolCall
		actual            []TrajectoryStep
		expectedScore     float64
		expectedAction    RecommendedAction
		wantFeedbackRegex string
	}{
		{
			name: "tools in any order",
			expected: []ExpectedToolCall{
				{Name: "mytool", Required: true},
				{Name: "http-client", Required: true},
				{Name: "sqlmap", Required: true},
			},
			actual: []TrajectoryStep{
				{Type: "tool", Name: "http-client", Input: map[string]any{}},
				{Type: "tool", Name: "mytool", Input: map[string]any{}},
			},
			expectedScore:  0.67,
			expectedAction: ActionContinue,
		},
		{
			name: "argument mismatch",
			expected: []ExpectedToolCall{
				{Name: "mytool", Required: true, Arguments: map[string]any{"target": "192.168.1.1"}},
				{Name: "http-client", Required: true},
			},
			actual: []TrajectoryStep{
				{Type: "tool", Name: "mytool", Input: map[string]any{"target": "10.0.0.1"}},
			},
			expectedScore:  0.0,
			expectedAction: ActionReconsider, // 1/2 = 50% mismatch rate > 30% threshold
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			scorer := NewStreamingToolCorrectnessScorer(ToolCorrectnessOptions{
				ExpectedTools: tt.expected,
				OrderMatters:  false,
			})

			trajectory := Trajectory{
				Steps:     tt.actual,
				StartTime: time.Now(),
			}

			result, err := scorer.ScorePartial(context.Background(), trajectory)
			if err != nil {
				t.Fatalf("Unexpected error: %v", err)
			}

			if math.Abs(result.Score-tt.expectedScore) > 0.01 {
				t.Errorf("Expected score %0.2f, got %0.2f", tt.expectedScore, result.Score)
			}

			if result.Action != tt.expectedAction {
				t.Errorf("Expected action %q, got %q", tt.expectedAction, result.Action)
			}

			if result.Feedback == "" {
				t.Error("Expected non-empty feedback message")
			}

			t.Logf("Feedback: %s", result.Feedback)
		})
	}
}

func TestStreamingToolCorrectnessScorer_MixedStepTypes(t *testing.T) {
	scorer := NewStreamingToolCorrectnessScorer(ToolCorrectnessOptions{
		ExpectedTools: []ExpectedToolCall{
			{Name: "mytool", Required: true},
			{Name: "http-client", Required: true},
		},
		OrderMatters: true,
	})

	trajectory := Trajectory{
		Steps: []TrajectoryStep{
			{Type: "llm", Name: "primary", Input: "analyze target"},
			{Type: "tool", Name: "mytool", Input: map[string]any{}},
			{Type: "memory", Name: "set", Input: "key"},
			{Type: "tool", Name: "http-client", Input: map[string]any{}},
			{Type: "finding", Name: "submit", Input: "vulnerability"},
		},
		StartTime: time.Now(),
	}

	result, err := scorer.ScorePartial(context.Background(), trajectory)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	// Should only evaluate tool steps
	if result.Score != 1.0 {
		t.Errorf("Expected score 1.0, got %0.2f (should ignore non-tool steps)", result.Score)
	}

	if result.Status != ScoreStatusComplete {
		t.Errorf("Expected complete status, got %q", result.Status)
	}
}

func TestStreamingToolCorrectnessScorer_NoExpectedTools(t *testing.T) {
	scorer := NewStreamingToolCorrectnessScorer(ToolCorrectnessOptions{
		ExpectedTools: []ExpectedToolCall{},
	})

	trajectory := Trajectory{
		Steps: []TrajectoryStep{
			{Type: "tool", Name: "mytool", Input: map[string]any{}},
		},
		StartTime: time.Now(),
	}

	result, err := scorer.ScorePartial(context.Background(), trajectory)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	if result.Score != 1.0 {
		t.Errorf("Expected score 1.0 when no tools expected, got %0.2f", result.Score)
	}

	if result.Status != ScoreStatusPending {
		t.Errorf("Expected pending status, got %q", result.Status)
	}
}

func TestStreamingToolCorrectnessScorer_ArgumentsWithTolerance(t *testing.T) {
	scorer := NewStreamingToolCorrectnessScorer(ToolCorrectnessOptions{
		ExpectedTools: []ExpectedToolCall{
			{
				Name:     "measure",
				Required: true,
				Arguments: map[string]any{
					"threshold": 0.85,
				},
			},
		},
		OrderMatters:     true,
		NumericTolerance: 0.1,
	})

	trajectory := Trajectory{
		Steps: []TrajectoryStep{
			{
				Type: "tool",
				Name: "measure",
				Input: map[string]any{
					"threshold": 0.88, // Within tolerance
				},
			},
		},
		StartTime: time.Now(),
	}

	result, err := scorer.ScorePartial(context.Background(), trajectory)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	if result.Score != 1.0 {
		t.Errorf("Expected score 1.0 (within tolerance), got %0.2f", result.Score)
	}
}

func TestStreamingToolCorrectnessScorer_DetailsContent(t *testing.T) {
	scorer := NewStreamingToolCorrectnessScorer(ToolCorrectnessOptions{
		ExpectedTools: []ExpectedToolCall{
			{Name: "mytool", Required: true},
			{Name: "http-client", Required: true},
			{Name: "sqlmap", Required: true},
		},
		OrderMatters: true,
	})

	trajectory := Trajectory{
		Steps: []TrajectoryStep{
			{Type: "tool", Name: "mytool", Input: map[string]any{}},
		},
		StartTime: time.Now(),
	}

	result, err := scorer.ScorePartial(context.Background(), trajectory)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	details := result.Details
	if details == nil {
		t.Fatal("Expected non-nil details")
	}

	// Check expected keys
	expectedKeys := []string{"matched", "mismatched", "required_total", "progress", "order_matters"}
	for _, key := range expectedKeys {
		if _, exists := details[key]; !exists {
			t.Errorf("Expected details key %q not found", key)
		}
	}

	if details["matched"] != 1 {
		t.Errorf("Expected matched=1, got %v", details["matched"])
	}

	if details["required_total"] != 3 {
		t.Errorf("Expected required_total=3, got %v", details["required_total"])
	}

	if details["order_matters"] != true {
		t.Errorf("Expected order_matters=true, got %v", details["order_matters"])
	}
}

func TestStreamingToolCorrectnessScorer_BackwardCompatibility(t *testing.T) {
	// Verify the streaming scorer implements the base Scorer interface
	scorer := NewStreamingToolCorrectnessScorer(ToolCorrectnessOptions{
		ExpectedTools: []ExpectedToolCall{
			{Name: "mytool", Required: true},
		},
	})

	// Should work as a regular scorer too
	sample := Sample{
		ID: "test",
		Trajectory: Trajectory{
			Steps: []TrajectoryStep{
				{Type: "tool", Name: "mytool", Input: map[string]any{}},
			},
		},
	}

	result, err := scorer.Score(context.Background(), sample)
	if err != nil {
		t.Fatalf("Unexpected error from Score(): %v", err)
	}

	if result.Score != 1.0 {
		t.Errorf("Expected score 1.0, got %0.2f", result.Score)
	}
}
