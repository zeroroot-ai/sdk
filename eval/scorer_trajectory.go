// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Zero Root AI

package eval

import (
	"context"
	"fmt"
	"math"
)

// TrajectoryMode defines how trajectory steps are matched against expected steps.
type TrajectoryMode int

const (
	// TrajectoryExactMatch requires exact sequence - all expected steps must match in order,
	// no extra steps allowed. This is the strictest matching mode.
	TrajectoryExactMatch TrajectoryMode = iota

	// TrajectorySubsetMatch requires all required expected steps to be present,
	// but they can appear in any order and extra steps are allowed.
	TrajectorySubsetMatch

	// TrajectoryOrderedSubset requires expected steps to appear in order,
	// but allows extra steps between them. Required steps must maintain
	// their relative ordering from the expected sequence.
	TrajectoryOrderedSubset
)

// ExpectedStep represents a single expected operation in the agent's execution path.
type ExpectedStep struct {
	// Type identifies the kind of operation.
	// Common values: "tool", "llm", "delegate", "finding"
	Type string `json:"type" yaml:"type"`

	// Name is the specific name of the operation.
	// For tools: tool name, for LLM: slot name, for delegate: agent name
	Name string `json:"name" yaml:"name"`

	// Required indicates whether this step must be present in the trajectory.
	// If false, the step is optional and won't penalize the score if missing.
	Required bool `json:"required" yaml:"required"`
}

// TrajectoryOptions configures how trajectory scoring is performed.
type TrajectoryOptions struct {
	// ExpectedSteps defines the sequence of operations expected during execution.
	ExpectedSteps []ExpectedStep `json:"expected_steps" yaml:"expected_steps"`

	// Mode determines how trajectory matching is performed.
	Mode TrajectoryMode `json:"mode" yaml:"mode"`

	// PenalizeExtra is the penalty applied per extra step not in expected sequence.
	// Value should be in range [0, 1]. A value of 0.1 means each extra step
	// reduces the score by 10% of the maximum score.
	PenalizeExtra float64 `json:"penalize_extra" yaml:"penalize_extra"`
}

// trajectoryScorer evaluates agent execution paths against expected trajectories.
type trajectoryScorer struct {
	opts TrajectoryOptions
}

// NewTrajectoryScorer creates a new trajectory scorer with the given options.
//
// The scorer evaluates how well an agent's execution path matches the expected
// sequence of operations. Different matching modes provide flexibility for
// evaluation:
//
//   - ExactMatch: Strictest mode - requires exact sequence, no extras
//   - SubsetMatch: All required steps present, any order, extras allowed
//   - OrderedSubset: Required steps in order, extras allowed between them
//
// Example:
//
//	scorer := NewTrajectoryScorer(TrajectoryOptions{
//	    ExpectedSteps: []ExpectedStep{
//	        {Type: "tool", Name: "mytool", Required: true},
//	        {Type: "tool", Name: "mytool-b", Required: true},
//	        {Type: "finding", Name: "", Required: true},
//	    },
//	    Mode: TrajectoryOrderedSubset,
//	    PenalizeExtra: 0.05, // 5% penalty per extra step
//	})
func NewTrajectoryScorer(opts TrajectoryOptions) Scorer {
	return &trajectoryScorer{opts: opts}
}

// Name returns the scorer identifier.
func (t *trajectoryScorer) Name() string {
	return "trajectory"
}

// Score evaluates the trajectory and returns a score in [0.0, 1.0].
//
// The scoring algorithm:
// 1. Extract actual steps from sample.Trajectory
// 2. Match against ExpectedSteps based on Mode
// 3. Calculate base score: matched_required / total_required
// 4. Apply penalty: score -= (extra_count * PenalizeExtra)
// 5. Clamp to [0, 1]
//
// Details map contains:
//   - "matched": []string - List of matched step descriptions
//   - "missing": []string - List of missing required step descriptions
//   - "extra": []string - List of extra step descriptions
//   - "matched_count": int - Number of matched required steps
//   - "required_count": int - Total number of required expected steps
//   - "extra_count": int - Number of extra steps
//   - "mode": string - Matching mode used
func (t *trajectoryScorer) Score(ctx context.Context, sample Sample) (ScoreResult, error) {
	// Extract actual steps from trajectory
	actualSteps := sample.Trajectory.Steps

	// Count required expected steps
	requiredCount := 0
	for _, exp := range t.opts.ExpectedSteps {
		if exp.Required {
			requiredCount++
		}
	}

	// If no required steps, return perfect score
	if requiredCount == 0 {
		return ScoreResult{
			Score: 1.0,
			Details: map[string]any{
				"matched":        []string{},
				"missing":        []string{},
				"extra":          []string{},
				"matched_count":  0,
				"required_count": 0,
				"extra_count":    0,
				"mode":           t.modeString(),
			},
		}, nil
	}

	// Perform matching based on mode
	var matched []string
	var missing []string
	var extra []string
	var matchedCount int
	var extraCount int

	switch t.opts.Mode {
	case TrajectoryExactMatch:
		matched, missing, extra, matchedCount, extraCount = t.exactMatch(actualSteps)
	case TrajectorySubsetMatch:
		matched, missing, extra, matchedCount, extraCount = t.subsetMatch(actualSteps)
	case TrajectoryOrderedSubset:
		matched, missing, extra, matchedCount, extraCount = t.orderedSubsetMatch(actualSteps)
	default:
		return ScoreResult{}, fmt.Errorf("unknown trajectory mode: %v", t.opts.Mode)
	}

	// Calculate base score: matched_required / total_required
	baseScore := float64(matchedCount) / float64(requiredCount)

	// Apply penalty for extra steps
	penalty := float64(extraCount) * t.opts.PenalizeExtra
	finalScore := baseScore - penalty

	// Clamp to [0, 1]
	finalScore = math.Max(0.0, math.Min(1.0, finalScore))

	return ScoreResult{
		Score: finalScore,
		Details: map[string]any{
			"matched":        matched,
			"missing":        missing,
			"extra":          extra,
			"matched_count":  matchedCount,
			"required_count": requiredCount,
			"extra_count":    extraCount,
			"mode":           t.modeString(),
		},
	}, nil
}

// exactMatch implements TrajectoryExactMatch mode.
// Steps must match exactly in order with no extras.
func (t *trajectoryScorer) exactMatch(actualSteps []TrajectoryStep) (matched, missing, extra []string, matchedCount, extraCount int) {
	matched = []string{}
	missing = []string{}
	extra = []string{}

	// Check if lengths match first
	if len(actualSteps) != len(t.opts.ExpectedSteps) {
		// Mark all as missing or extra depending on which is longer
		for i, exp := range t.opts.ExpectedSteps {
			if i < len(actualSteps) {
				actual := actualSteps[i]
				if t.stepsMatch(actual, exp) {
					matched = append(matched, t.stepString(actual))
					if exp.Required {
						matchedCount++
					}
				} else {
					if exp.Required {
						missing = append(missing, t.expectedStepString(exp))
					}
					extra = append(extra, t.stepString(actual))
					extraCount++
				}
			} else {
				if exp.Required {
					missing = append(missing, t.expectedStepString(exp))
				}
			}
		}
		// Any extra actual steps
		for i := len(t.opts.ExpectedSteps); i < len(actualSteps); i++ {
			extra = append(extra, t.stepString(actualSteps[i]))
			extraCount++
		}
		return
	}

	// Same length - check each position
	for i, exp := range t.opts.ExpectedSteps {
		actual := actualSteps[i]
		if t.stepsMatch(actual, exp) {
			matched = append(matched, t.stepString(actual))
			if exp.Required {
				matchedCount++
			}
		} else {
			if exp.Required {
				missing = append(missing, t.expectedStepString(exp))
			}
			extra = append(extra, t.stepString(actual))
			extraCount++
		}
	}

	return
}

// subsetMatch implements TrajectorySubsetMatch mode.
// All required expected steps must be present, any order, extras allowed.
func (t *trajectoryScorer) subsetMatch(actualSteps []TrajectoryStep) (matched, missing, extra []string, matchedCount, extraCount int) {
	matched = []string{}
	missing = []string{}
	extra = []string{}

	// Track which actual steps have been matched
	usedActual := make([]bool, len(actualSteps))

	// For each expected step, find a matching actual step
	for _, exp := range t.opts.ExpectedSteps {
		found := false
		for i, actual := range actualSteps {
			if !usedActual[i] && t.stepsMatch(actual, exp) {
				matched = append(matched, t.stepString(actual))
				usedActual[i] = true
				found = true
				if exp.Required {
					matchedCount++
				}
				break
			}
		}
		if !found && exp.Required {
			missing = append(missing, t.expectedStepString(exp))
		}
	}

	// Any unused actual steps are extras
	for i, actual := range actualSteps {
		if !usedActual[i] {
			extra = append(extra, t.stepString(actual))
			extraCount++
		}
	}

	return
}

// orderedSubsetMatch implements TrajectoryOrderedSubset mode.
// Required steps must appear in order, extras allowed between them.
func (t *trajectoryScorer) orderedSubsetMatch(actualSteps []TrajectoryStep) (matched, missing, extra []string, matchedCount, extraCount int) {
	matched = []string{}
	missing = []string{}
	extra = []string{}

	// Track which actual steps have been matched
	usedActual := make([]bool, len(actualSteps))

	// Track position in actual steps
	actualIdx := 0

	// For each expected step, find the next matching actual step
	for _, exp := range t.opts.ExpectedSteps {
		found := false

		// Search from current position forward
		for actualIdx < len(actualSteps) {
			actual := actualSteps[actualIdx]
			if t.stepsMatch(actual, exp) {
				matched = append(matched, t.stepString(actual))
				usedActual[actualIdx] = true
				found = true
				if exp.Required {
					matchedCount++
				}
				actualIdx++ // Move past this matched step
				break
			}
			// This step doesn't match - continue searching
			actualIdx++
		}

		if !found && exp.Required {
			missing = append(missing, t.expectedStepString(exp))
		}
	}

	// Any unused actual steps are extras
	for i, actual := range actualSteps {
		if !usedActual[i] {
			extra = append(extra, t.stepString(actual))
			extraCount++
		}
	}

	return
}

// stepsMatch checks if an actual step matches an expected step.
func (t *trajectoryScorer) stepsMatch(actual TrajectoryStep, expected ExpectedStep) bool {
	// Type must match
	if actual.Type != expected.Type {
		return false
	}

	// Name must match (if expected name is specified)
	if expected.Name != "" && actual.Name != expected.Name {
		return false
	}

	return true
}

// stepString formats a trajectory step for display.
func (t *trajectoryScorer) stepString(step TrajectoryStep) string {
	if step.Name != "" {
		return fmt.Sprintf("%s:%s", step.Type, step.Name)
	}
	return step.Type
}

// expectedStepString formats an expected step for display.
func (t *trajectoryScorer) expectedStepString(step ExpectedStep) string {
	if step.Name != "" {
		return fmt.Sprintf("%s:%s", step.Type, step.Name)
	}
	return step.Type
}

// modeString returns a string representation of the matching mode.
func (t *trajectoryScorer) modeString() string {
	switch t.opts.Mode {
	case TrajectoryExactMatch:
		return "exact_match"
	case TrajectorySubsetMatch:
		return "subset_match"
	case TrajectoryOrderedSubset:
		return "ordered_subset"
	default:
		return "unknown"
	}
}
