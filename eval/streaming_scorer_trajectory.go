// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Zero Root AI

package eval

import (
	"context"
	"fmt"
	"math"
)

// streamingTrajectoryScorer provides real-time evaluation of agent execution paths
// as they are being generated. It extends the standard trajectoryScorer with
// the ability to evaluate partial trajectories.
type streamingTrajectoryScorer struct {
	*trajectoryScorer
}

// NewStreamingTrajectoryScorer creates a new streaming trajectory scorer with the given options.
//
// The scorer can evaluate partial trajectories in real-time, providing feedback
// as the agent executes. Different matching modes behave differently during streaming:
//
//   - ExactMatch: Detects deviations immediately when a step doesn't match the expected prefix
//   - SubsetMatch: Tracks progress toward seeing all required steps (any order)
//   - OrderedSubset: Tracks progress through expected sequence, allowing extras between
//
// Example:
//
//	scorer := NewStreamingTrajectoryScorer(TrajectoryOptions{
//	    ExpectedSteps: []ExpectedStep{
//	        {Type: "tool", Name: "mytool", Required: true},
//	        {Type: "tool", Name: "mytool-b", Required: true},
//	        {Type: "finding", Name: "", Required: true},
//	    },
//	    Mode: TrajectoryOrderedSubset,
//	    PenalizeExtra: 0.05, // 5% penalty per extra step
//	})
func NewStreamingTrajectoryScorer(opts TrajectoryOptions) StreamingScorer {
	return &streamingTrajectoryScorer{
		trajectoryScorer: &trajectoryScorer{opts: opts},
	}
}

// SupportsStreaming indicates this scorer can meaningfully evaluate partial trajectories.
func (s *streamingTrajectoryScorer) SupportsStreaming() bool {
	return true
}

// ScorePartial evaluates a partial trajectory and returns real-time feedback.
//
// The scoring behavior depends on the TrajectoryMode:
//
// ExactMatch Mode:
//   - Checks if current steps match expected steps prefix exactly
//   - If mismatch at any position, immediately flags with "reconsider" action
//   - Score = steps_matched / total_expected
//   - Confidence increases as more steps match correctly
//
// SubsetMatch Mode:
//   - Tracks which expected steps have been seen (any order)
//   - Score = seen_required_steps / total_required_steps
//   - No ordering penalty during execution
//   - Confidence reflects completeness of required steps
//
// OrderedSubset Mode:
//   - Tracks expected steps in order, allows extras between
//   - Score = matched_required_in_order / total_required
//   - Penalizes extra steps based on PenalizeExtra option
//   - Confidence reflects progress through expected sequence
//
// The method returns appropriate feedback messages and recommended actions:
//   - ActionContinue: Agent is on track
//   - ActionAdjust: Minor deviation detected
//   - ActionReconsider: Significant deviation detected
//   - ActionAbort: Critical issue (currently not used for trajectory)
func (s *streamingTrajectoryScorer) ScorePartial(ctx context.Context, trajectory Trajectory) (PartialScore, error) {
	actualSteps := trajectory.Steps

	// Count required expected steps
	requiredCount := 0
	for _, exp := range s.opts.ExpectedSteps {
		if exp.Required {
			requiredCount++
		}
	}

	// If no required steps, return perfect score
	if requiredCount == 0 {
		return PartialScore{
			Score:      1.0,
			Confidence: 1.0,
			Status:     ScoreStatusPartial,
			Feedback:   "No required steps defined - all trajectories are valid",
			Action:     ActionContinue,
			Details: map[string]any{
				"matched":        []string{},
				"missing":        []string{},
				"extra":          []string{},
				"matched_count":  0,
				"required_count": 0,
				"extra_count":    0,
				"mode":           s.modeString(),
			},
		}, nil
	}

	// If no steps yet, return pending state
	if len(actualSteps) == 0 {
		return PartialScore{
			Score:      0.0,
			Confidence: 0.0,
			Status:     ScoreStatusPending,
			Feedback:   "Waiting for first step in trajectory",
			Action:     ActionContinue,
			Details: map[string]any{
				"matched":        []string{},
				"missing":        []string{},
				"extra":          []string{},
				"matched_count":  0,
				"required_count": requiredCount,
				"extra_count":    0,
				"mode":           s.modeString(),
			},
		}, nil
	}

	// Perform mode-specific partial scoring
	switch s.opts.Mode {
	case TrajectoryExactMatch:
		return s.scorePartialExactMatch(actualSteps, requiredCount)
	case TrajectorySubsetMatch:
		return s.scorePartialSubsetMatch(actualSteps, requiredCount)
	case TrajectoryOrderedSubset:
		return s.scorePartialOrderedSubset(actualSteps, requiredCount)
	default:
		return PartialScore{}, fmt.Errorf("unknown trajectory mode: %v", s.opts.Mode)
	}
}

// scorePartialExactMatch evaluates partial trajectory in ExactMatch mode.
// Checks if current steps match expected steps prefix exactly.
func (s *streamingTrajectoryScorer) scorePartialExactMatch(actualSteps []TrajectoryStep, requiredCount int) (PartialScore, error) {
	matched := []string{}
	missing := []string{}
	extra := []string{}
	matchedCount := 0
	extraCount := 0

	// Check each actual step against expected prefix
	for i, actual := range actualSteps {
		if i >= len(s.opts.ExpectedSteps) {
			// More steps than expected - flag as extra
			extra = append(extra, s.stepString(actual))
			extraCount++
			continue
		}

		expected := s.opts.ExpectedSteps[i]
		if s.stepsMatch(actual, expected) {
			matched = append(matched, s.stepString(actual))
			if expected.Required {
				matchedCount++
			}
		} else {
			// Mismatch detected - flag immediately
			extra = append(extra, s.stepString(actual))
			extraCount++

			// This is a critical deviation in ExactMatch mode
			score := float64(matchedCount) / float64(requiredCount)
			confidence := math.Min(1.0, float64(len(actualSteps))/float64(len(s.opts.ExpectedSteps)))

			feedback := fmt.Sprintf("Step mismatch at position %d: expected %s, got %s",
				i, s.expectedStepString(expected), s.stepString(actual))

			return PartialScore{
				Score:      score,
				Confidence: confidence,
				Status:     ScoreStatusPartial,
				Feedback:   feedback,
				Action:     ActionReconsider,
				Details: map[string]any{
					"matched":        matched,
					"missing":        missing,
					"extra":          extra,
					"matched_count":  matchedCount,
					"required_count": requiredCount,
					"extra_count":    extraCount,
					"mode":           s.modeString(),
					"mismatch_index": i,
				},
			}, nil
		}
	}

	// All steps so far match prefix
	score := float64(matchedCount) / float64(requiredCount)
	confidence := math.Min(1.0, float64(len(actualSteps))/float64(len(s.opts.ExpectedSteps)))

	// Determine remaining expected steps
	for i := len(actualSteps); i < len(s.opts.ExpectedSteps); i++ {
		exp := s.opts.ExpectedSteps[i]
		if exp.Required {
			missing = append(missing, s.expectedStepString(exp))
		}
	}

	var feedback string
	var action RecommendedAction

	if len(actualSteps) < len(s.opts.ExpectedSteps) {
		feedback = fmt.Sprintf("On track: %d/%d required steps completed", matchedCount, requiredCount)
		action = ActionContinue
	} else if len(actualSteps) == len(s.opts.ExpectedSteps) {
		feedback = "Trajectory complete and matches expected sequence exactly"
		action = ActionContinue
	} else {
		feedback = fmt.Sprintf("Extra steps detected: %d more than expected", extraCount)
		action = ActionReconsider
	}

	return PartialScore{
		Score:      score,
		Confidence: confidence,
		Status:     ScoreStatusPartial,
		Feedback:   feedback,
		Action:     action,
		Details: map[string]any{
			"matched":        matched,
			"missing":        missing,
			"extra":          extra,
			"matched_count":  matchedCount,
			"required_count": requiredCount,
			"extra_count":    extraCount,
			"mode":           s.modeString(),
		},
	}, nil
}

// scorePartialSubsetMatch evaluates partial trajectory in SubsetMatch mode.
// Tracks which expected steps have been seen in any order.
func (s *streamingTrajectoryScorer) scorePartialSubsetMatch(actualSteps []TrajectoryStep, requiredCount int) (PartialScore, error) {
	matched := []string{}
	missing := []string{}
	extra := []string{}
	matchedCount := 0
	extraCount := 0

	// Track which actual steps have been matched
	usedActual := make([]bool, len(actualSteps))

	// For each expected step, find a matching actual step
	for _, exp := range s.opts.ExpectedSteps {
		found := false
		for i, actual := range actualSteps {
			if !usedActual[i] && s.stepsMatch(actual, exp) {
				matched = append(matched, s.stepString(actual))
				usedActual[i] = true
				found = true
				if exp.Required {
					matchedCount++
				}
				break
			}
		}
		if !found && exp.Required {
			missing = append(missing, s.expectedStepString(exp))
		}
	}

	// Any unused actual steps are extras
	for i, actual := range actualSteps {
		if !usedActual[i] {
			extra = append(extra, s.stepString(actual))
			extraCount++
		}
	}

	// Calculate score and confidence
	baseScore := float64(matchedCount) / float64(requiredCount)
	penalty := float64(extraCount) * s.opts.PenalizeExtra
	score := math.Max(0.0, math.Min(1.0, baseScore-penalty))

	// Confidence reflects completeness - how close we are to seeing all required steps
	confidence := float64(matchedCount) / float64(requiredCount)

	// Determine feedback and action
	var feedback string
	var action RecommendedAction

	if matchedCount == requiredCount {
		feedback = fmt.Sprintf("All %d required steps found", requiredCount)
		action = ActionContinue
		if extraCount > 0 {
			feedback += fmt.Sprintf(" (with %d extra steps)", extraCount)
			if extraCount > requiredCount {
				action = ActionAdjust
			}
		}
	} else {
		missingCount := requiredCount - matchedCount
		feedback = fmt.Sprintf("Progress: %d/%d required steps found, %d remaining",
			matchedCount, requiredCount, missingCount)
		action = ActionContinue

		// If we have many extra steps but haven't found required ones, suggest adjustment
		if extraCount > requiredCount/2 && float64(matchedCount) < float64(requiredCount)*0.5 {
			action = ActionAdjust
			feedback += " - consider focusing on required steps"
		}
	}

	return PartialScore{
		Score:      score,
		Confidence: confidence,
		Status:     ScoreStatusPartial,
		Feedback:   feedback,
		Action:     action,
		Details: map[string]any{
			"matched":        matched,
			"missing":        missing,
			"extra":          extra,
			"matched_count":  matchedCount,
			"required_count": requiredCount,
			"extra_count":    extraCount,
			"mode":           s.modeString(),
		},
	}, nil
}

// scorePartialOrderedSubset evaluates partial trajectory in OrderedSubset mode.
// Tracks expected steps in order, allows extras between them.
func (s *streamingTrajectoryScorer) scorePartialOrderedSubset(actualSteps []TrajectoryStep, requiredCount int) (PartialScore, error) {
	matched := []string{}
	missing := []string{}
	extra := []string{}
	matchedCount := 0
	extraCount := 0

	// Track which actual steps have been matched
	usedActual := make([]bool, len(actualSteps))

	// Track position in actual steps
	actualIdx := 0
	expectedIdx := 0

	// For each expected step, find the next matching actual step
	for expectedIdx < len(s.opts.ExpectedSteps) {
		exp := s.opts.ExpectedSteps[expectedIdx]
		found := false

		// Search from current position forward
		for actualIdx < len(actualSteps) {
			actual := actualSteps[actualIdx]
			if s.stepsMatch(actual, exp) {
				matched = append(matched, s.stepString(actual))
				usedActual[actualIdx] = true
				found = true
				if exp.Required {
					matchedCount++
				}
				actualIdx++ // Move past this matched step
				break
			}
			// This step doesn't match - mark as extra and continue searching
			actualIdx++
		}

		if found {
			expectedIdx++
		} else {
			// Haven't found this expected step yet
			if exp.Required {
				missing = append(missing, s.expectedStepString(exp))
			}
			expectedIdx++
		}
	}

	// Any unused actual steps are extras
	for i, actual := range actualSteps {
		if !usedActual[i] {
			extra = append(extra, s.stepString(actual))
			extraCount++
		}
	}

	// Calculate score and confidence
	baseScore := float64(matchedCount) / float64(requiredCount)
	penalty := float64(extraCount) * s.opts.PenalizeExtra
	score := math.Max(0.0, math.Min(1.0, baseScore-penalty))

	// Confidence reflects how far we've progressed through the expected sequence
	// We're more confident the more required steps we've matched in order
	confidence := float64(matchedCount) / float64(requiredCount)

	// Determine feedback and action
	var feedback string
	var action RecommendedAction

	if matchedCount == requiredCount {
		feedback = fmt.Sprintf("All %d required steps found in order", requiredCount)
		action = ActionContinue
		if extraCount > 0 {
			feedback += fmt.Sprintf(" (with %d extra steps)", extraCount)
			if extraCount > requiredCount {
				action = ActionAdjust
			}
		}
	} else {
		missingCount := requiredCount - matchedCount
		feedback = fmt.Sprintf("Progress: %d/%d required steps found in order, %d remaining",
			matchedCount, requiredCount, missingCount)
		action = ActionContinue

		// If we have many extra steps with low progress, suggest adjustment
		if extraCount > requiredCount/2 && float64(matchedCount) < float64(requiredCount)*0.5 {
			action = ActionAdjust
			feedback += " - many extra steps detected, consider more focused approach"
		}
	}

	return PartialScore{
		Score:      score,
		Confidence: confidence,
		Status:     ScoreStatusPartial,
		Feedback:   feedback,
		Action:     action,
		Details: map[string]any{
			"matched":        matched,
			"missing":        missing,
			"extra":          extra,
			"matched_count":  matchedCount,
			"required_count": requiredCount,
			"extra_count":    extraCount,
			"mode":           s.modeString(),
		},
	}, nil
}
