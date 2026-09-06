// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Zero Root AI

package planning

import (
	"math"
	"testing"
)

func TestNewStepHints(t *testing.T) {
	hints := NewStepHints()

	if hints == nil {
		t.Fatal("NewStepHints() returned nil")
	}

	if hints.Confidence() != 0.5 {
		t.Errorf("Expected default confidence 0.5, got %f", hints.Confidence())
	}

	if len(hints.SuggestedNext()) != 0 {
		t.Errorf("Expected empty suggestedNext, got %d items", len(hints.SuggestedNext()))
	}

	if len(hints.KeyFindings()) != 0 {
		t.Errorf("Expected empty keyFindings, got %d items", len(hints.KeyFindings()))
	}

	if hints.ReplanReason() != "" {
		t.Errorf("Expected empty replanReason, got %q", hints.ReplanReason())
	}

	if hints.HasReplanRecommendation() {
		t.Error("Expected HasReplanRecommendation() to be false")
	}
}

func TestWithConfidence(t *testing.T) {
	tests := []struct {
		name     string
		input    float64
		expected float64
	}{
		{
			name:     "valid confidence 0.0",
			input:    0.0,
			expected: 0.0,
		},
		{
			name:     "valid confidence 1.0",
			input:    1.0,
			expected: 1.0,
		},
		{
			name:     "valid confidence 0.75",
			input:    0.75,
			expected: 0.75,
		},
		{
			name:     "clamped below 0.0",
			input:    -0.5,
			expected: 0.0,
		},
		{
			name:     "clamped above 1.0",
			input:    1.5,
			expected: 1.0,
		},
		{
			name:     "clamped negative infinity",
			input:    math.Inf(-1),
			expected: 0.0,
		},
		{
			name:     "clamped positive infinity",
			input:    math.Inf(1),
			expected: 1.0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			hints := NewStepHints().WithConfidence(tt.input)
			if hints.Confidence() != tt.expected {
				t.Errorf("Expected confidence %f, got %f", tt.expected, hints.Confidence())
			}
		})
	}
}

func TestWithConfidenceChaining(t *testing.T) {
	hints := NewStepHints().
		WithConfidence(0.8).
		WithSuggestion("next_agent")

	if hints.Confidence() != 0.8 {
		t.Errorf("Expected confidence 0.8 after chaining, got %f", hints.Confidence())
	}

	if len(hints.SuggestedNext()) != 1 {
		t.Fatalf("Expected 1 suggestion after chaining, got %d", len(hints.SuggestedNext()))
	}
}

func TestWithSuggestion(t *testing.T) {
	tests := []struct {
		name        string
		suggestions []string
		expected    []string
	}{
		{
			name:        "single suggestion",
			suggestions: []string{"auth_bypass_agent"},
			expected:    []string{"auth_bypass_agent"},
		},
		{
			name:        "multiple suggestions",
			suggestions: []string{"recon_agent", "exploit_agent", "persistence_agent"},
			expected:    []string{"recon_agent", "exploit_agent", "persistence_agent"},
		},
		{
			name:        "empty string ignored",
			suggestions: []string{"valid", "", "another"},
			expected:    []string{"valid", "another"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			hints := NewStepHints()
			for _, s := range tt.suggestions {
				hints.WithSuggestion(s)
			}

			result := hints.SuggestedNext()
			if len(result) != len(tt.expected) {
				t.Fatalf("Expected %d suggestions, got %d", len(tt.expected), len(result))
			}

			for i, exp := range tt.expected {
				if result[i] != exp {
					t.Errorf("Expected suggestion[%d] = %q, got %q", i, exp, result[i])
				}
			}
		})
	}
}

func TestWithKeyFinding(t *testing.T) {
	tests := []struct {
		name     string
		findings []string
		expected []string
	}{
		{
			name:     "single finding",
			findings: []string{"Admin panel discovered at /admin"},
			expected: []string{"Admin panel discovered at /admin"},
		},
		{
			name: "multiple findings",
			findings: []string{
				"SQL injection in login form",
				"Default credentials work",
				"No rate limiting on API",
			},
			expected: []string{
				"SQL injection in login form",
				"Default credentials work",
				"No rate limiting on API",
			},
		},
		{
			name:     "empty string ignored",
			findings: []string{"valid finding", "", "another finding"},
			expected: []string{"valid finding", "another finding"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			hints := NewStepHints()
			for _, f := range tt.findings {
				hints.WithKeyFinding(f)
			}

			result := hints.KeyFindings()
			if len(result) != len(tt.expected) {
				t.Fatalf("Expected %d findings, got %d", len(tt.expected), len(result))
			}

			for i, exp := range tt.expected {
				if result[i] != exp {
					t.Errorf("Expected finding[%d] = %q, got %q", i, exp, result[i])
				}
			}
		})
	}
}

func TestRecommendReplan(t *testing.T) {
	tests := []struct {
		name   string
		reason string
	}{
		{
			name:   "valid reason",
			reason: "Target uses custom auth - standard attacks ineffective",
		},
		{
			name:   "empty reason",
			reason: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			hints := NewStepHints().RecommendReplan(tt.reason)

			if hints.ReplanReason() != tt.reason {
				t.Errorf("Expected replan reason %q, got %q", tt.reason, hints.ReplanReason())
			}

			expectedHasReplan := tt.reason != ""
			if hints.HasReplanRecommendation() != expectedHasReplan {
				t.Errorf("Expected HasReplanRecommendation() = %v, got %v",
					expectedHasReplan, hints.HasReplanRecommendation())
			}
		})
	}
}

func TestHasReplanRecommendation(t *testing.T) {
	tests := []struct {
		name     string
		reason   string
		expected bool
	}{
		{
			name:     "with reason",
			reason:   "Need to change strategy",
			expected: true,
		},
		{
			name:     "without reason",
			reason:   "",
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			hints := NewStepHints()
			if tt.reason != "" {
				hints.RecommendReplan(tt.reason)
			}

			if hints.HasReplanRecommendation() != tt.expected {
				t.Errorf("Expected HasReplanRecommendation() = %v, got %v",
					tt.expected, hints.HasReplanRecommendation())
			}
		})
	}
}

func TestBuilderChaining(t *testing.T) {
	// Test fluent builder pattern with all methods
	hints := NewStepHints().
		WithConfidence(0.85).
		WithKeyFinding("Admin panel discovered at /admin").
		WithKeyFinding("Default credentials may be in use").
		WithSuggestion("auth_bypass_agent").
		WithSuggestion("credential_stuffing_agent").
		RecommendReplan("Target uses custom auth - standard attacks ineffective")

	// Verify all fields are set correctly
	if hints.Confidence() != 0.85 {
		t.Errorf("Expected confidence 0.85, got %f", hints.Confidence())
	}

	findings := hints.KeyFindings()
	if len(findings) != 2 {
		t.Fatalf("Expected 2 findings, got %d", len(findings))
	}
	if findings[0] != "Admin panel discovered at /admin" {
		t.Errorf("Unexpected finding[0]: %q", findings[0])
	}
	if findings[1] != "Default credentials may be in use" {
		t.Errorf("Unexpected finding[1]: %q", findings[1])
	}

	suggestions := hints.SuggestedNext()
	if len(suggestions) != 2 {
		t.Fatalf("Expected 2 suggestions, got %d", len(suggestions))
	}
	if suggestions[0] != "auth_bypass_agent" {
		t.Errorf("Unexpected suggestion[0]: %q", suggestions[0])
	}
	if suggestions[1] != "credential_stuffing_agent" {
		t.Errorf("Unexpected suggestion[1]: %q", suggestions[1])
	}

	if !hints.HasReplanRecommendation() {
		t.Error("Expected HasReplanRecommendation() to be true")
	}
	expectedReason := "Target uses custom auth - standard attacks ineffective"
	if hints.ReplanReason() != expectedReason {
		t.Errorf("Expected replan reason %q, got %q", expectedReason, hints.ReplanReason())
	}
}

func TestGettersReturnCopies(t *testing.T) {
	// Verify that SuggestedNext and KeyFindings return copies, not references
	hints := NewStepHints().
		WithSuggestion("agent1").
		WithSuggestion("agent2").
		WithKeyFinding("finding1").
		WithKeyFinding("finding2")

	// Get initial slices
	suggestions1 := hints.SuggestedNext()
	findings1 := hints.KeyFindings()

	// Modify the slices
	if len(suggestions1) > 0 {
		suggestions1[0] = "modified"
	}
	if len(findings1) > 0 {
		findings1[0] = "modified"
	}

	// Get new slices and verify they're unchanged
	suggestions2 := hints.SuggestedNext()
	findings2 := hints.KeyFindings()

	if suggestions2[0] == "modified" {
		t.Error("SuggestedNext() returned a reference, not a copy")
	}
	if suggestions2[0] != "agent1" {
		t.Errorf("Expected suggestion[0] = 'agent1', got %q", suggestions2[0])
	}

	if findings2[0] == "modified" {
		t.Error("KeyFindings() returned a reference, not a copy")
	}
	if findings2[0] != "finding1" {
		t.Errorf("Expected finding[0] = 'finding1', got %q", findings2[0])
	}
}

func TestMultipleConfidenceUpdates(t *testing.T) {
	// Test that confidence can be updated multiple times
	hints := NewStepHints().
		WithConfidence(0.3).
		WithConfidence(0.7).
		WithConfidence(0.9)

	if hints.Confidence() != 0.9 {
		t.Errorf("Expected final confidence 0.9, got %f", hints.Confidence())
	}
}

func TestEmptyHints(t *testing.T) {
	// Test that an empty hints object behaves correctly
	hints := NewStepHints()

	if len(hints.SuggestedNext()) != 0 {
		t.Error("Expected empty suggestions")
	}

	if len(hints.KeyFindings()) != 0 {
		t.Error("Expected empty findings")
	}

	if hints.HasReplanRecommendation() {
		t.Error("Expected no replan recommendation")
	}

	if hints.ReplanReason() != "" {
		t.Error("Expected empty replan reason")
	}
}
