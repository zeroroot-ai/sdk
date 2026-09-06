// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Zero Root AI

package eval

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"
)

// ThresholdConfig defines score thresholds for generating alerts.
type ThresholdConfig struct {
	// Warning is the score below which a warning alert is generated.
	// Default: 0.5
	Warning float64

	// Critical is the score below which a critical alert is generated.
	// Default: 0.2
	Critical float64
}

// DefaultThresholdConfig returns the default threshold configuration.
func DefaultThresholdConfig() ThresholdConfig {
	return ThresholdConfig{
		Warning:  0.5,
		Critical: 0.2,
	}
}

// FeedbackDispatcher orchestrates parallel evaluation of a trajectory
// across multiple streaming scorers and aggregates the results into
// actionable feedback.
type FeedbackDispatcher struct {
	scorers    []StreamingScorer
	thresholds ThresholdConfig
	timeout    time.Duration
}

// NewFeedbackDispatcher creates a new FeedbackDispatcher with the given
// scorers and threshold configuration.
func NewFeedbackDispatcher(scorers []StreamingScorer, thresholds ThresholdConfig) *FeedbackDispatcher {
	// Apply defaults if thresholds are zero
	if thresholds.Warning == 0 {
		thresholds.Warning = 0.5
	}
	if thresholds.Critical == 0 {
		thresholds.Critical = 0.2
	}

	return &FeedbackDispatcher{
		scorers:    scorers,
		thresholds: thresholds,
		timeout:    5 * time.Second, // Default 5 second timeout per scorer
	}
}

// scorerResult holds the result from a single scorer evaluation.
type scorerResult struct {
	name  string
	score PartialScore
	err   error
}

// Evaluate runs all scorers in parallel against the trajectory and aggregates
// the results into a Feedback object. It handles scorer timeouts gracefully
// and generates alerts when scores breach configured thresholds.
//
// Edge cases:
// - All scorers timeout: returns nil feedback with error
// - Some scorers fail: aggregates successful ones, notes failures in details
// - Empty scorer list: returns nil feedback
func (fd *FeedbackDispatcher) Evaluate(ctx context.Context, trajectory Trajectory) (*Feedback, error) {
	// Handle empty scorer list
	if len(fd.scorers) == 0 {
		return nil, errors.New("no scorers configured")
	}

	// Create context with timeout for all scorers
	evalCtx, cancel := context.WithTimeout(ctx, fd.timeout)
	defer cancel()

	// Channel to collect results
	results := make(chan scorerResult, len(fd.scorers))

	// WaitGroup for coordination
	var wg sync.WaitGroup
	wg.Add(len(fd.scorers))

	// Launch goroutines for each scorer
	for _, scorer := range fd.scorers {
		go func(s StreamingScorer) {
			defer wg.Done()

			// Evaluate with timeout context
			score, err := s.ScorePartial(evalCtx, trajectory)
			results <- scorerResult{
				name:  s.Name(),
				score: score,
				err:   err,
			}
		}(scorer)
	}

	// Wait for all scorers to complete in a separate goroutine
	go func() {
		wg.Wait()
		close(results)
	}()

	// Collect results
	scores := make(map[string]PartialScore)
	failures := make(map[string]error)

	for result := range results {
		if result.err != nil {
			failures[result.name] = result.err
			continue
		}
		scores[result.name] = result.score
	}

	// Check if all scorers failed
	if len(scores) == 0 {
		if len(failures) > 0 {
			return nil, fmt.Errorf("all scorers failed or timed out: %v", failures)
		}
		return nil, errors.New("all scorers failed or timed out")
	}

	// Calculate overall score from confident scores (confidence > 0.5)
	overallScore := fd.calculateOverallScore(scores)

	// Generate alerts based on thresholds
	alerts := fd.generateAlerts(overallScore, scores)

	// Determine overall recommended action
	action := fd.determineOverallAction(overallScore, scores)

	// Build overall feedback message
	feedbackMsg := fd.buildOverallFeedback(overallScore, scores, failures)

	// Construct feedback
	feedback := &Feedback{
		Timestamp: time.Now(),
		StepIndex: len(trajectory.Steps),
		Scores:    scores,
		Overall: PartialScore{
			Score:      overallScore,
			Confidence: fd.calculateOverallConfidence(scores),
			Status:     fd.determineOverallStatus(scores),
			Feedback:   feedbackMsg,
			Action:     action,
			Details:    map[string]any{},
		},
		Alerts:   alerts,
		Consumed: false,
	}

	// Add failure information to details if any
	if len(failures) > 0 {
		failureNames := make([]string, 0, len(failures))
		for name := range failures {
			failureNames = append(failureNames, name)
		}
		feedback.Overall.Details["failed_scorers"] = failureNames
	}

	return feedback, nil
}

// calculateOverallScore computes the mean score of all confident scores (confidence > 0.5).
func (fd *FeedbackDispatcher) calculateOverallScore(scores map[string]PartialScore) float64 {
	var sum float64
	count := 0

	for _, score := range scores {
		// Only include scores with confidence > 0.5
		if score.Confidence > 0.5 {
			sum += score.Score
			count++
		}
	}

	if count == 0 {
		// No confident scores, return average of all scores
		for _, score := range scores {
			sum += score.Score
			count++
		}
	}

	if count == 0 {
		return 0.0
	}

	return sum / float64(count)
}

// calculateOverallConfidence computes the average confidence across all scorers.
func (fd *FeedbackDispatcher) calculateOverallConfidence(scores map[string]PartialScore) float64 {
	if len(scores) == 0 {
		return 0.0
	}

	var sum float64
	for _, score := range scores {
		sum += score.Confidence
	}

	return sum / float64(len(scores))
}

// determineOverallStatus determines the overall status based on individual scorer statuses.
func (fd *FeedbackDispatcher) determineOverallStatus(scores map[string]PartialScore) ScoreStatus {
	hasPartial := false
	allComplete := true

	for _, score := range scores {
		switch score.Status {
		case ScoreStatusPending:
			allComplete = false
		case ScoreStatusPartial:
			hasPartial = true
			allComplete = false
		case ScoreStatusComplete:
			// At least one is complete
		}
	}

	if allComplete {
		return ScoreStatusComplete
	}
	if hasPartial {
		return ScoreStatusPartial
	}
	return ScoreStatusPending
}

// generateAlerts creates alerts for threshold breaches.
func (fd *FeedbackDispatcher) generateAlerts(overallScore float64, scores map[string]PartialScore) []Alert {
	var alerts []Alert

	// Check overall score thresholds
	if overallScore < fd.thresholds.Critical {
		alerts = append(alerts, Alert{
			Level:     AlertCritical,
			Scorer:    "",
			Score:     overallScore,
			Threshold: fd.thresholds.Critical,
			Message:   fmt.Sprintf("Overall score %.2f is below critical threshold %.2f", overallScore, fd.thresholds.Critical),
			Action:    ActionReconsider,
		})
	} else if overallScore < fd.thresholds.Warning {
		alerts = append(alerts, Alert{
			Level:     AlertWarning,
			Scorer:    "",
			Score:     overallScore,
			Threshold: fd.thresholds.Warning,
			Message:   fmt.Sprintf("Overall score %.2f is below warning threshold %.2f", overallScore, fd.thresholds.Warning),
			Action:    ActionAdjust,
		})
	}

	// Check individual scorer thresholds (only for confident scores)
	for name, score := range scores {
		if score.Confidence > 0.5 {
			if score.Score < fd.thresholds.Critical {
				alerts = append(alerts, Alert{
					Level:     AlertCritical,
					Scorer:    name,
					Score:     score.Score,
					Threshold: fd.thresholds.Critical,
					Message:   fmt.Sprintf("%s score %.2f is critically low", name, score.Score),
					Action:    score.Action,
				})
			} else if score.Score < fd.thresholds.Warning {
				alerts = append(alerts, Alert{
					Level:     AlertWarning,
					Scorer:    name,
					Score:     score.Score,
					Threshold: fd.thresholds.Warning,
					Message:   fmt.Sprintf("%s score %.2f is below expected", name, score.Score),
					Action:    score.Action,
				})
			}
		}
	}

	return alerts
}

// determineOverallAction determines the recommended action based on scores.
func (fd *FeedbackDispatcher) determineOverallAction(overallScore float64, scores map[string]PartialScore) RecommendedAction {
	// Check if any individual scorer recommends abort (highest priority)
	for _, score := range scores {
		if score.Action == ActionAbort && score.Confidence > 0.5 {
			return ActionAbort
		}
	}

	// If overall score is critically low, recommend reconsider
	if overallScore < fd.thresholds.Critical {
		return ActionReconsider
	}

	// Check if any individual scorer recommends reconsider
	for _, score := range scores {
		if score.Action == ActionReconsider && score.Confidence > 0.5 {
			return ActionReconsider
		}
	}

	// If overall score is below warning, recommend adjust
	if overallScore < fd.thresholds.Warning {
		return ActionAdjust
	}

	// Check if any individual scorer recommends adjust
	for _, score := range scores {
		if score.Action == ActionAdjust && score.Confidence > 0.5 {
			return ActionAdjust
		}
	}

	// Default to continue
	return ActionContinue
}

// buildOverallFeedback constructs a feedback message summarizing the evaluation.
func (fd *FeedbackDispatcher) buildOverallFeedback(overallScore float64, scores map[string]PartialScore, failures map[string]error) string {
	if overallScore >= 0.8 {
		return "Execution is proceeding well."
	}

	if overallScore >= fd.thresholds.Warning {
		return "Execution is acceptable but could be improved."
	}

	if overallScore >= fd.thresholds.Critical {
		return "Execution quality is below expected. Review the individual scorer feedback."
	}

	return "Execution quality is critically low. Consider changing your approach."
}
