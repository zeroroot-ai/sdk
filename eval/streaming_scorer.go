// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Zero Root AI

package eval

import (
	"context"
)

// StreamingScorer extends the Scorer interface with the ability to evaluate
// partial trajectories as they are being generated. This enables real-time
// feedback during agent execution.
type StreamingScorer interface {
	// Scorer embeds the standard scorer interface for backward compatibility.
	Scorer

	// ScorePartial evaluates a partial trajectory and returns a PartialScore.
	// This is called during agent execution to provide real-time feedback.
	// The trajectory may be incomplete, so scorers should handle partial data.
	ScorePartial(ctx context.Context, trajectory Trajectory) (PartialScore, error)

	// SupportsStreaming indicates whether this scorer can meaningfully
	// evaluate partial trajectories. Scorers that require complete trajectories
	// should return false.
	SupportsStreaming() bool
}

// PartialScore represents a streaming evaluation result with confidence
// and recommended actions.
type PartialScore struct {
	// Score must be in the range [0.0, 1.0] where 0.0 is worst and 1.0 is best.
	Score float64 `json:"score" yaml:"score"`

	// Confidence indicates how confident the scorer is in this partial score (0.0 to 1.0).
	// Low confidence scores (< 0.5) should be interpreted cautiously.
	Confidence float64 `json:"confidence" yaml:"confidence"`

	// Status indicates the evaluation state for this partial trajectory.
	Status ScoreStatus `json:"status" yaml:"status"`

	// Feedback is a human-readable message explaining the score and any issues.
	// This can be injected into the agent's context to guide behavior.
	Feedback string `json:"feedback,omitempty" yaml:"feedback,omitempty"`

	// Action is the recommended action based on this partial evaluation.
	Action RecommendedAction `json:"action" yaml:"action"`

	// Details contains scorer-specific diagnostic information.
	// Common keys include: "matched", "missing", "extra", "precision", "recall"
	Details map[string]any `json:"details,omitempty" yaml:"details,omitempty"`
}

// ScoreStatus represents the state of a partial evaluation.
type ScoreStatus string

const (
	// ScoreStatusPending indicates evaluation cannot yet be performed
	// (e.g., insufficient trajectory data).
	ScoreStatusPending ScoreStatus = "pending"

	// ScoreStatusPartial indicates evaluation is in progress with partial data.
	ScoreStatusPartial ScoreStatus = "partial"

	// ScoreStatusComplete indicates evaluation is complete (trajectory finished).
	ScoreStatusComplete ScoreStatus = "complete"
)

// RecommendedAction suggests what the agent should do based on the evaluation.
type RecommendedAction string

const (
	// ActionContinue indicates the agent is performing well and should proceed.
	ActionContinue RecommendedAction = "continue"

	// ActionAdjust suggests the agent should modify its approach slightly.
	ActionAdjust RecommendedAction = "adjust"

	// ActionReconsider suggests the agent should significantly change its strategy.
	ActionReconsider RecommendedAction = "reconsider"

	// ActionAbort suggests the agent should stop execution due to critical issues.
	ActionAbort RecommendedAction = "abort"
)
