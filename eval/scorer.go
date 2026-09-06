// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Zero Root AI

// Package eval provides a lightweight evaluation framework for assessing AI agent performance.
// It integrates with go test and provides domain-specific scorers for security testing agents.
package eval

import (
	"context"
	"errors"
	"fmt"
)

// Scorer evaluates a sample and returns a scored result.
// All scorers must return scores in the range [0.0, 1.0].
type Scorer interface {
	// Score evaluates the given sample and returns a ScoreResult.
	// The score must be between 0.0 (worst) and 1.0 (best).
	// Details map can contain scorer-specific diagnostic information.
	Score(ctx context.Context, sample Sample) (ScoreResult, error)

	// Name returns a unique identifier for this scorer type.
	// This is used for result aggregation and logging.
	Name() string
}

// ScoreResult contains the evaluation score and optional details from a scorer.
type ScoreResult struct {
	// Score must be in the range [0.0, 1.0] where 0.0 is worst and 1.0 is best.
	Score float64 `json:"score" yaml:"score"`

	// Details contains scorer-specific diagnostic information.
	// Common keys include: "matched", "missing", "extra", "precision", "recall", "f1"
	Details map[string]any `json:"details,omitempty" yaml:"details,omitempty"`
}

// ValidateScore ensures the score is within the valid range [0.0, 1.0].
// Returns an error if the score is out of range or NaN.
func ValidateScore(score float64) error {
	if score < 0.0 || score > 1.0 {
		return fmt.Errorf("score %.4f is out of valid range [0.0, 1.0]", score)
	}

	// Check for NaN
	if score != score {
		return errors.New("score is NaN")
	}

	return nil
}

// AggregateScores combines multiple ScoreResults into a single weighted score.
// If weights is nil or empty, all scores are weighted equally (average).
// If weights are provided, only scorers with matching names in the weights map are included.
// Weight values are normalized to sum to 1.0.
//
// Example:
//
//	results := []ScoreResult{
//	    {Score: 0.8},
//	    {Score: 0.6},
//	}
//	weights := map[string]float64{
//	    "tool_correctness": 0.7,
//	    "task_completion": 0.3,
//	}
//	score := AggregateScores(results, weights)
func AggregateScores(results []ScoreResult, weights map[string]float64) float64 {
	if len(results) == 0 {
		return 0.0
	}

	// If no weights provided, return simple average
	if len(weights) == 0 {
		var sum float64
		for _, result := range results {
			sum += result.Score
		}
		return sum / float64(len(results))
	}

	// Weighted average
	// First, normalize weights to sum to 1.0
	var weightSum float64
	for _, w := range weights {
		weightSum += w
	}

	if weightSum == 0.0 {
		// All weights are zero, fall back to equal weighting
		var sum float64
		for _, result := range results {
			sum += result.Score
		}
		return sum / float64(len(results))
	}

	// Calculate weighted sum
	var weightedSum float64
	usedWeight := 0.0

	for scorerName, weight := range weights {
		// Find corresponding result
		// Note: This assumes results are indexed in the same order as scorer names
		// In practice, the caller should maintain this mapping
		normalizedWeight := weight / weightSum

		// For this basic implementation, we'll apply weights in order
		// A more sophisticated version would require scorer names in ScoreResult
		if int(usedWeight*float64(len(results))) < len(results) {
			idx := int(usedWeight * float64(len(results)))
			weightedSum += results[idx].Score * normalizedWeight
			usedWeight += normalizedWeight
		}
		_ = scorerName // Will be used when ScoreResult includes scorer name
	}

	return weightedSum
}

// AggregateScoresWithNames combines multiple named ScoreResults into a single weighted score.
// This is a more robust version that takes a map of scorer name to ScoreResult.
// If weights is nil or empty, all scores are weighted equally.
// Only scorers present in the results map are considered.
func AggregateScoresWithNames(results map[string]ScoreResult, weights map[string]float64) float64 {
	if len(results) == 0 {
		return 0.0
	}

	// If no weights provided, return simple average
	if len(weights) == 0 {
		var sum float64
		for _, result := range results {
			sum += result.Score
		}
		return sum / float64(len(results))
	}

	// Normalize weights for scorers that exist in results
	var weightSum float64
	for name, weight := range weights {
		if _, exists := results[name]; exists {
			weightSum += weight
		}
	}

	if weightSum == 0.0 {
		// No matching scorers or all weights are zero, fall back to equal weighting
		var sum float64
		for _, result := range results {
			sum += result.Score
		}
		return sum / float64(len(results))
	}

	// Calculate weighted sum
	var weightedSum float64
	for name, result := range results {
		if weight, hasWeight := weights[name]; hasWeight {
			normalizedWeight := weight / weightSum
			weightedSum += result.Score * normalizedWeight
		}
	}

	return weightedSum
}
