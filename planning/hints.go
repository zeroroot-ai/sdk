// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Zero Root AI

package planning

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
