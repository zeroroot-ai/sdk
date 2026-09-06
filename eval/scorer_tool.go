// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Zero Root AI

package eval

import (
	"context"
	"fmt"
	"math"
	"reflect"
)

// ToolCorrectnessOptions configures the Tool Correctness Scorer behavior.
type ToolCorrectnessOptions struct {
	// ExpectedTools lists the expected tool calls to match against.
	// If empty, uses sample.ExpectedTools instead.
	ExpectedTools []ExpectedToolCall

	// OrderMatters determines if tool calls must occur in the expected order.
	// If true, tools must be called in the exact sequence specified.
	// If false, tools can be called in any order.
	OrderMatters bool

	// NumericTolerance is the tolerance for comparing numeric arguments.
	// Two numbers are considered equal if |a - b| <= NumericTolerance.
	// Default: 0.0 (exact equality required).
	NumericTolerance float64
}

// toolCorrectnessScorer evaluates whether an agent called the correct tools
// with the correct arguments during task execution.
type toolCorrectnessScorer struct {
	opts ToolCorrectnessOptions
}

// NewToolCorrectnessScorer creates a scorer that evaluates tool call correctness.
// It compares actual tool calls from the trajectory against expected tool calls.
//
// The scorer extracts tool calls from trajectory steps (where Type == "tool"),
// matches them against expected tools by name, and compares arguments.
// Numeric arguments are compared with tolerance, other arguments use deep equality.
//
// Score calculation:
//   - Score = matched / max(len(expected), len(actual))
//   - Only required tools contribute to scoring
//   - Optional tools don't penalize if missing
//
// Details returned:
//   - matched: Number of correctly matched tool calls
//   - missing: Number of expected tools not called
//   - extra: Number of unexpected tool calls
//   - mismatched: Number of tools called with wrong arguments
//   - matched_tools: List of successfully matched tool names
//   - missing_tools: List of expected tool names not found
//   - extra_tools: List of unexpected tool names called
func NewToolCorrectnessScorer(opts ToolCorrectnessOptions) Scorer {
	return &toolCorrectnessScorer{opts: opts}
}

// Name returns the scorer identifier.
func (s *toolCorrectnessScorer) Name() string {
	return "tool_correctness"
}

// Score evaluates tool call correctness for the given sample.
func (s *toolCorrectnessScorer) Score(ctx context.Context, sample Sample) (ScoreResult, error) {
	// Determine which expected tools to use
	expectedTools := s.opts.ExpectedTools
	if len(expectedTools) == 0 {
		expectedTools = sample.ExpectedTools
	}

	// Extract actual tool calls from trajectory
	actualTools := extractToolCalls(sample.Trajectory)

	// Match tools and calculate score
	matched, mismatched, missing, extra := s.matchTools(expectedTools, actualTools)

	// Calculate score: matched / max(expected, actual)
	// Only count required tools in expected count
	requiredCount := countRequiredTools(expectedTools)
	maxCount := max(requiredCount, len(actualTools))

	var score float64
	if maxCount == 0 {
		// No tools expected or called
		score = 1.0
	} else {
		score = float64(len(matched)) / float64(maxCount)
	}

	// Validate score is in range
	if err := ValidateScore(score); err != nil {
		return ScoreResult{}, fmt.Errorf("invalid tool correctness score: %w", err)
	}

	// Build details
	details := map[string]any{
		"matched":       len(matched),
		"mismatched":    len(mismatched),
		"missing":       len(missing),
		"extra":         len(extra),
		"matched_tools": extractToolNames(matched),
	}

	if len(missing) > 0 {
		details["missing_tools"] = extractExpectedToolNames(missing)
	}

	if len(extra) > 0 {
		details["extra_tools"] = extractToolNames(extra)
	}

	if len(mismatched) > 0 {
		details["mismatched_tools"] = buildMismatchDetails(mismatched)
	}

	return ScoreResult{
		Score:   score,
		Details: details,
	}, nil
}

// matchTools compares expected and actual tool calls and categorizes them.
// Returns matched tool calls, mismatched (wrong args), missing (not called), and extra (unexpected).
func (s *toolCorrectnessScorer) matchTools(expected []ExpectedToolCall, actual []TrajectoryStep) (
	matched []TrajectoryStep,
	mismatched []mismatchInfo,
	missing []ExpectedToolCall,
	extra []TrajectoryStep,
) {
	if s.opts.OrderMatters {
		return s.matchToolsOrdered(expected, actual)
	}
	return s.matchToolsUnordered(expected, actual)
}

// matchToolsUnordered matches tools without considering order.
func (s *toolCorrectnessScorer) matchToolsUnordered(expected []ExpectedToolCall, actual []TrajectoryStep) (
	matched []TrajectoryStep,
	mismatched []mismatchInfo,
	missing []ExpectedToolCall,
	extra []TrajectoryStep,
) {
	// Track which actual tools have been matched
	actualMatched := make([]bool, len(actual))

	// Try to match each expected tool
	for _, exp := range expected {
		found := false
		for i, act := range actual {
			if actualMatched[i] {
				continue
			}

			if act.Name == exp.Name {
				// Found matching tool name, check arguments
				if s.argumentsMatch(exp.Arguments, act.Input) {
					matched = append(matched, act)
					actualMatched[i] = true
					found = true
					break
				} else {
					// Tool called but arguments don't match
					if exp.Required {
						mismatched = append(mismatched, mismatchInfo{
							expected: exp,
							actual:   act,
							reason:   "arguments mismatch",
						})
						actualMatched[i] = true
						found = true
						break
					}
				}
			}
		}

		if !found && exp.Required {
			missing = append(missing, exp)
		}
	}

	// Collect unmatched actual tools as extras
	for i, act := range actual {
		if !actualMatched[i] {
			extra = append(extra, act)
		}
	}

	return
}

// matchToolsOrdered matches tools considering their order.
func (s *toolCorrectnessScorer) matchToolsOrdered(expected []ExpectedToolCall, actual []TrajectoryStep) (
	matched []TrajectoryStep,
	mismatched []mismatchInfo,
	missing []ExpectedToolCall,
	extra []TrajectoryStep,
) {
	actualIdx := 0

	for _, exp := range expected {
		found := false

		// Look for this expected tool in remaining actual tools
		for i := actualIdx; i < len(actual); i++ {
			if actual[i].Name == exp.Name {
				// Check arguments
				if s.argumentsMatch(exp.Arguments, actual[i].Input) {
					matched = append(matched, actual[i])
					// Mark all skipped tools as extra
					for j := actualIdx; j < i; j++ {
						extra = append(extra, actual[j])
					}
					actualIdx = i + 1
					found = true
					break
				} else if exp.Required {
					mismatched = append(mismatched, mismatchInfo{
						expected: exp,
						actual:   actual[i],
						reason:   "arguments mismatch",
					})
					// Mark skipped tools as extra
					for j := actualIdx; j < i; j++ {
						extra = append(extra, actual[j])
					}
					actualIdx = i + 1
					found = true
					break
				}
			}
		}

		if !found && exp.Required {
			missing = append(missing, exp)
		}
	}

	// Remaining actual tools are extras
	for i := actualIdx; i < len(actual); i++ {
		extra = append(extra, actual[i])
	}

	return
}

// argumentsMatch compares expected arguments against actual input.
// Handles numeric tolerance for float comparisons.
func (s *toolCorrectnessScorer) argumentsMatch(expected map[string]any, actual any) bool {
	// If no expected arguments, any input is acceptable
	if len(expected) == 0 {
		return true
	}

	// Convert actual to map if possible
	actualMap, ok := actual.(map[string]any)
	if !ok {
		return false
	}

	// Check each expected argument
	for key, expectedVal := range expected {
		actualVal, exists := actualMap[key]
		if !exists {
			return false
		}

		if !s.valuesMatch(expectedVal, actualVal) {
			return false
		}
	}

	return true
}

// valuesMatch compares two values with numeric tolerance support.
func (s *toolCorrectnessScorer) valuesMatch(expected, actual any) bool {
	// Try numeric comparison first
	if s.opts.NumericTolerance > 0 {
		expNum, expOk := toFloat64(expected)
		actNum, actOk := toFloat64(actual)
		if expOk && actOk {
			return math.Abs(expNum-actNum) <= s.opts.NumericTolerance
		}
	}

	// Fall back to deep equality
	return reflect.DeepEqual(expected, actual)
}

// extractToolCalls extracts all tool call steps from a trajectory.
func extractToolCalls(trajectory Trajectory) []TrajectoryStep {
	var tools []TrajectoryStep
	for _, step := range trajectory.Steps {
		if step.Type == "tool" {
			tools = append(tools, step)
		}
	}
	return tools
}

// countRequiredTools counts how many tools are marked as required.
func countRequiredTools(tools []ExpectedToolCall) int {
	count := 0
	for _, tool := range tools {
		if tool.Required {
			count++
		}
	}
	return count
}

// extractToolNames extracts tool names from trajectory steps.
func extractToolNames(steps []TrajectoryStep) []string {
	names := make([]string, len(steps))
	for i, step := range steps {
		names[i] = step.Name
	}
	return names
}

// extractExpectedToolNames extracts tool names from expected tool calls.
func extractExpectedToolNames(tools []ExpectedToolCall) []string {
	names := make([]string, len(tools))
	for i, tool := range tools {
		names[i] = tool.Name
	}
	return names
}

// mismatchInfo holds details about a tool call that didn't match expectations.
type mismatchInfo struct {
	expected ExpectedToolCall
	actual   TrajectoryStep
	reason   string
}

// buildMismatchDetails creates human-readable mismatch information.
func buildMismatchDetails(mismatches []mismatchInfo) []map[string]any {
	details := make([]map[string]any, len(mismatches))
	for i, mm := range mismatches {
		details[i] = map[string]any{
			"tool":          mm.expected.Name,
			"reason":        mm.reason,
			"expected_args": mm.expected.Arguments,
			"actual_args":   mm.actual.Input,
		}
	}
	return details
}

// toFloat64 attempts to convert a value to float64.
// Returns the float64 value and true if successful, 0 and false otherwise.
func toFloat64(v any) (float64, bool) {
	switch val := v.(type) {
	case float64:
		return val, true
	case float32:
		return float64(val), true
	case int:
		return float64(val), true
	case int8:
		return float64(val), true
	case int16:
		return float64(val), true
	case int32:
		return float64(val), true
	case int64:
		return float64(val), true
	case uint:
		return float64(val), true
	case uint8:
		return float64(val), true
	case uint16:
		return float64(val), true
	case uint32:
		return float64(val), true
	case uint64:
		return float64(val), true
	default:
		return 0, false
	}
}
