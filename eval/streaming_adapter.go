// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Zero Root AI

package eval

import (
	"context"
	"fmt"

	"github.com/zeroroot-ai/sdk/agent"
)

// StreamingAdapterOptions configures how a Scorer is adapted for streaming evaluation.
type StreamingAdapterOptions struct {
	// MinStepsForEval is the minimum number of trajectory steps required
	// before attempting evaluation. If the trajectory has fewer steps,
	// ScorePartial returns a pending status with low confidence.
	// Default: 1
	MinStepsForEval int

	// PartialScoreWeight is the weight applied to partial scores vs final scores.
	// This value (0.0 to 1.0) represents how confident we are in partial evaluations.
	// A value of 0.8 means partial scores are weighted at 80% of final scores.
	// Default: 0.8
	PartialScoreWeight float64
}

// DefaultStreamingAdapterOptions returns the default configuration for streaming adapters.
func DefaultStreamingAdapterOptions() StreamingAdapterOptions {
	return StreamingAdapterOptions{
		MinStepsForEval:    1,
		PartialScoreWeight: 0.8,
	}
}

// NewStreamingAdapter wraps an existing Scorer to support streaming evaluation.
// This allows any standard scorer to be used for partial trajectory evaluation
// with appropriate confidence adjustments.
//
// The adapter creates a minimal Sample from the partial trajectory and delegates
// to the inner scorer's Score method. Confidence is calculated based on trajectory
// completeness relative to the configured options.
//
// Example:
//
//	baseScorer := eval.NewToolCorrectnessScorer()
//	streamingScorer := eval.NewStreamingAdapter(baseScorer, eval.DefaultStreamingAdapterOptions())
//	partialScore, err := streamingScorer.ScorePartial(ctx, trajectory)
func NewStreamingAdapter(scorer Scorer, opts StreamingAdapterOptions) StreamingScorer {
	// Apply defaults if needed
	if opts.MinStepsForEval <= 0 {
		opts.MinStepsForEval = 1
	}
	if opts.PartialScoreWeight <= 0.0 || opts.PartialScoreWeight > 1.0 {
		opts.PartialScoreWeight = 0.8
	}

	return &streamingAdapter{
		inner: scorer,
		opts:  opts,
	}
}

// streamingAdapter wraps a standard Scorer to provide streaming capabilities.
type streamingAdapter struct {
	inner Scorer
	opts  StreamingAdapterOptions
}

// Score delegates to the inner scorer for standard (complete) evaluation.
func (s *streamingAdapter) Score(ctx context.Context, sample Sample) (ScoreResult, error) {
	return s.inner.Score(ctx, sample)
}

// Name returns the name of the inner scorer.
func (s *streamingAdapter) Name() string {
	return s.inner.Name()
}

// ScorePartial evaluates a partial trajectory by creating a minimal Sample
// and delegating to the inner scorer's Score method.
//
// The method handles three cases:
//
//  1. Insufficient steps: Returns pending status with low confidence
//  2. Partial trajectory: Evaluates with reduced confidence based on completeness
//  3. Error handling: Wraps scorer errors with appropriate context
//
// Confidence calculation:
//   - Base confidence starts at PartialScoreWeight (e.g., 0.8)
//   - Reduced further if trajectory is very short relative to MinStepsForEval
//   - For trajectories with < MinStepsForEval steps, confidence is capped at 0.3
func (s *streamingAdapter) ScorePartial(ctx context.Context, trajectory Trajectory) (PartialScore, error) {
	// Check if we have enough trajectory data for meaningful evaluation
	if len(trajectory.Steps) < s.opts.MinStepsForEval {
		return PartialScore{
			Score:      0.0,
			Confidence: 0.3, // Low confidence due to insufficient data
			Status:     ScoreStatusPending,
			Feedback: fmt.Sprintf(
				"Waiting for more trajectory data (have %d steps, need %d)",
				len(trajectory.Steps),
				s.opts.MinStepsForEval,
			),
			Action: ActionContinue,
			Details: map[string]any{
				"current_steps":  len(trajectory.Steps),
				"required_steps": s.opts.MinStepsForEval,
			},
		}, nil
	}

	// Create a minimal Sample from the partial trajectory
	// We use a synthetic Sample that contains just the trajectory data
	sample := Sample{
		ID:         "streaming-eval", // Synthetic ID for partial evaluation
		Task:       agent.Task{},     // Empty task - scorer should handle gracefully
		Trajectory: trajectory,
	}

	// Delegate to the inner scorer
	result, err := s.inner.Score(ctx, sample)
	if err != nil {
		return PartialScore{}, fmt.Errorf("inner scorer failed: %w", err)
	}

	// Calculate confidence based on trajectory completeness
	// For partial trajectories, we reduce confidence based on how much data we have
	confidence := s.opts.PartialScoreWeight

	// Further reduce confidence if we're just barely above the minimum threshold
	stepsAboveMin := len(trajectory.Steps) - s.opts.MinStepsForEval
	if stepsAboveMin < 3 {
		// Reduce confidence by 10% for each missing step (up to 30% reduction)
		reduction := float64(3-stepsAboveMin) * 0.1
		confidence = confidence * (1.0 - reduction)
	}

	// Determine recommended action based on score and confidence
	action := s.determineAction(result.Score, confidence)

	// Generate human-readable feedback
	feedback := s.generateFeedback(result.Score, confidence, len(trajectory.Steps))

	return PartialScore{
		Score:      result.Score,
		Confidence: confidence,
		Status:     ScoreStatusPartial,
		Feedback:   feedback,
		Action:     action,
		Details:    result.Details,
	}, nil
}

// SupportsStreaming always returns true since this adapter enables streaming.
func (s *streamingAdapter) SupportsStreaming() bool {
	return true
}

// determineAction recommends an action based on the score and confidence.
func (s *streamingAdapter) determineAction(score, confidence float64) RecommendedAction {
	// If confidence is very low, just continue and wait for more data
	if confidence < 0.5 {
		return ActionContinue
	}

	// For high confidence scores, provide stronger recommendations
	if confidence >= 0.7 {
		if score < 0.3 {
			return ActionReconsider // Very poor performance
		}
		if score < 0.5 {
			return ActionAdjust // Below average performance
		}
	}

	// Default to continue for acceptable performance
	return ActionContinue
}

// generateFeedback creates a human-readable message about the evaluation.
func (s *streamingAdapter) generateFeedback(score, confidence float64, steps int) string {
	if confidence < 0.5 {
		return fmt.Sprintf(
			"Early evaluation based on %d steps (confidence: %.2f). Score: %.2f. Continuing to gather data.",
			steps,
			confidence,
			score,
		)
	}

	if score < 0.3 {
		return fmt.Sprintf(
			"Partial evaluation shows low performance (score: %.2f, confidence: %.2f). Consider adjusting approach.",
			score,
			confidence,
		)
	}

	if score < 0.5 {
		return fmt.Sprintf(
			"Partial evaluation shows room for improvement (score: %.2f, confidence: %.2f).",
			score,
			confidence,
		)
	}

	return fmt.Sprintf(
		"Partial evaluation shows good progress (score: %.2f, confidence: %.2f). Continue current approach.",
		score,
		confidence,
	)
}
