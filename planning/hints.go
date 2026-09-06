// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Zero Root AI

package planning

import "math"

// StepHints allows agents to provide feedback to the planning system.
// Use the builder pattern to construct hints fluently.
//
// Example usage:
//
//	hints := planning.NewStepHints().
//	    WithConfidence(0.85).
//	    WithKeyFinding("Admin panel discovered at /admin").
//	    WithKeyFinding("Default credentials may be in use").
//	    WithSuggestion("auth_bypass_agent").
//	    RecommendReplan("Target uses custom auth - standard attacks ineffective")
//
//	harness.ReportStepHints(ctx, hints)
type StepHints struct {
	// confidence is the agent's self-assessed confidence in its results (0.0-1.0)
	confidence float64

	// suggestedNext contains agent recommendations for next steps
	suggestedNext []string

	// replanReason explains why the agent thinks replanning may be needed
	replanReason string

	// keyFindings is a summary of important discoveries made during execution
	keyFindings []string
}

// NewStepHints creates a new StepHints with default values.
// Default confidence is 0.5 (neutral), no suggestions or findings.
func NewStepHints() *StepHints {
	return &StepHints{
		confidence:    0.5,
		suggestedNext: make([]string, 0),
		keyFindings:   make([]string, 0),
		replanReason:  "",
	}
}

// WithConfidence sets the agent's self-assessed confidence in its results.
// Confidence should be between 0.0 (no confidence) and 1.0 (fully confident).
// Values outside this range are clamped.
func (h *StepHints) WithConfidence(c float64) *StepHints {
	// Clamp confidence to [0.0, 1.0]
	h.confidence = math.Max(0.0, math.Min(1.0, c))
	return h
}

// WithSuggestion adds a suggested next step to the hints.
// Multiple suggestions can be added by chaining calls.
func (h *StepHints) WithSuggestion(step string) *StepHints {
	if step != "" {
		h.suggestedNext = append(h.suggestedNext, step)
	}
	return h
}

// RecommendReplan sets the reason why replanning may be needed.
// If called, the step scorer will consider triggering tactical replanning.
func (h *StepHints) RecommendReplan(reason string) *StepHints {
	h.replanReason = reason
	return h
}

// WithKeyFinding adds a key finding to the hints.
// Key findings are important discoveries that should influence planning.
func (h *StepHints) WithKeyFinding(finding string) *StepHints {
	if finding != "" {
		h.keyFindings = append(h.keyFindings, finding)
	}
	return h
}

// ─── Getter Methods ──────────────────────────────────────────────────────────
// These are used by the framework to read the hints.

// Confidence returns the agent's self-assessed confidence.
func (h *StepHints) Confidence() float64 {
	return h.confidence
}

// SuggestedNext returns the list of suggested next steps.
func (h *StepHints) SuggestedNext() []string {
	// Return a copy to prevent external modification
	result := make([]string, len(h.suggestedNext))
	copy(result, h.suggestedNext)
	return result
}

// ReplanReason returns the reason for recommended replanning, or empty string.
func (h *StepHints) ReplanReason() string {
	return h.replanReason
}

// KeyFindings returns the list of key findings.
func (h *StepHints) KeyFindings() []string {
	// Return a copy to prevent external modification
	result := make([]string, len(h.keyFindings))
	copy(result, h.keyFindings)
	return result
}

// HasReplanRecommendation returns true if replanning was recommended.
func (h *StepHints) HasReplanRecommendation() bool {
	return h.replanReason != ""
}
