// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Zero Root AI

package eval

import (
	"context"
	"fmt"
)

// streamingToolCorrectnessScorer is a streaming version of the tool correctness scorer
// that can evaluate partial trajectories in real-time.
type streamingToolCorrectnessScorer struct {
	*toolCorrectnessScorer
}

// NewStreamingToolCorrectnessScorer creates a streaming scorer that evaluates tool call
// correctness as the trajectory is being generated.
//
// For ordered mode:
//   - Evaluates tool calls as a PREFIX match against expected sequence
//   - Detects wrong tool choices early (mismatches in expected order)
//   - Score = tools_matched_in_order / total_expected_required_tools
//   - Returns "continue" action if tools match expected prefix
//   - Returns "adjust" or "reconsider" if wrong tool detected
//
// For unordered mode:
//   - Evaluates tools called so far against expected tools (any order)
//   - Score reflects partial progress toward calling all expected tools
//   - Cannot detect wrong choices as easily (tool might be called later)
//
// Example (ordered mode):
//
//	Expected: [mytool, http-client, sqlmap, exploit]
//	Called: [mytool, http-client]
//	Result: Score=0.5, Confidence=0.5, Action=continue
//
//	Expected: [mytool, http-client, sqlmap]
//	Called: [mytool, sqlmap]  // Wrong order!
//	Result: Score=0.33, Confidence=0.67, Action=adjust
//
// Confidence increases as more of the expected sequence is evaluated.
func NewStreamingToolCorrectnessScorer(opts ToolCorrectnessOptions) StreamingScorer {
	return &streamingToolCorrectnessScorer{
		toolCorrectnessScorer: &toolCorrectnessScorer{opts: opts},
	}
}

// SupportsStreaming returns true to indicate streaming evaluation is supported.
func (s *streamingToolCorrectnessScorer) SupportsStreaming() bool {
	return true
}

// ScorePartial evaluates a partial trajectory and provides real-time feedback.
// This allows the framework to detect tool selection issues early and provide
// corrective guidance before the full trajectory completes.
func (s *streamingToolCorrectnessScorer) ScorePartial(ctx context.Context, trajectory Trajectory) (PartialScore, error) {
	// Extract tool calls from partial trajectory
	actualTools := extractToolCalls(trajectory)

	// Get expected tools from options
	expectedTools := s.opts.ExpectedTools

	// If no expected tools or actual tools yet, return pending status
	if len(expectedTools) == 0 {
		return PartialScore{
			Score:      1.0,
			Confidence: 1.0,
			Status:     ScoreStatusPending,
			Feedback:   "No expected tools defined for evaluation",
			Action:     ActionContinue,
		}, nil
	}

	if len(actualTools) == 0 {
		return PartialScore{
			Score:      0.0,
			Confidence: 0.0,
			Status:     ScoreStatusPending,
			Feedback:   "No tool calls recorded yet",
			Action:     ActionContinue,
		}, nil
	}

	// Evaluate based on ordering mode
	if s.opts.OrderMatters {
		return s.scorePartialOrdered(expectedTools, actualTools)
	}
	return s.scorePartialUnordered(expectedTools, actualTools)
}

// scorePartialOrdered evaluates ordered tool calls using PREFIX matching.
// It checks if the tools called so far match the beginning of the expected sequence.
func (s *streamingToolCorrectnessScorer) scorePartialOrdered(expected []ExpectedToolCall, actual []TrajectoryStep) (PartialScore, error) {
	requiredCount := countRequiredTools(expected)
	if requiredCount == 0 {
		return PartialScore{
			Score:      1.0,
			Confidence: 1.0,
			Status:     ScoreStatusComplete,
			Feedback:   "No required tools to evaluate",
			Action:     ActionContinue,
		}, nil
	}

	// Track matching progress through expected sequence
	matched := 0
	mismatched := 0
	expectedIdx := 0

	// Walk through actual tools and try to match against expected sequence
	for _, actualTool := range actual {
		// Skip to next required tool in expected sequence
		for expectedIdx < len(expected) && !expected[expectedIdx].Required {
			expectedIdx++
		}

		if expectedIdx >= len(expected) {
			// Called more tools than expected - these are extras
			break
		}

		expectedTool := expected[expectedIdx]

		// Check if this tool matches the expected tool at this position
		if actualTool.Name == expectedTool.Name {
			// Tool name matches, check arguments
			if s.argumentsMatch(expectedTool.Arguments, actualTool.Input) {
				matched++
				expectedIdx++
			} else {
				// Wrong arguments for expected tool
				mismatched++
				expectedIdx++
			}
		} else {
			// Wrong tool at this position in sequence
			mismatched++
			// Don't advance expectedIdx - this tool is missing
		}
	}

	// Calculate partial score based on prefix match
	score := float64(matched) / float64(requiredCount)

	// Validate score
	if err := ValidateScore(score); err != nil {
		return PartialScore{}, fmt.Errorf("invalid streaming tool correctness score: %w", err)
	}

	// Calculate confidence based on how much of sequence we've evaluated
	// Confidence increases as we evaluate more of the expected sequence
	toolsEvaluated := matched + mismatched
	confidence := float64(toolsEvaluated) / float64(requiredCount)
	if confidence > 1.0 {
		confidence = 1.0
	}

	// Determine status
	status := ScoreStatusPartial
	if toolsEvaluated >= requiredCount {
		status = ScoreStatusComplete
	}

	// Determine action based on score and mismatches
	action := s.determineAction(score, mismatched, requiredCount)

	// Build feedback message
	feedback := s.buildOrderedFeedback(matched, mismatched, requiredCount, expected, expectedIdx)

	return PartialScore{
		Score:      score,
		Confidence: confidence,
		Status:     status,
		Feedback:   feedback,
		Action:     action,
		Details: map[string]any{
			"matched":        matched,
			"mismatched":     mismatched,
			"required_total": requiredCount,
			"progress":       fmt.Sprintf("%d/%d", matched, requiredCount),
			"order_matters":  true,
		},
	}, nil
}

// scorePartialUnordered evaluates unordered tool calls.
// It matches tools called so far against expected tools in any order.
func (s *streamingToolCorrectnessScorer) scorePartialUnordered(expected []ExpectedToolCall, actual []TrajectoryStep) (PartialScore, error) {
	requiredCount := countRequiredTools(expected)
	if requiredCount == 0 {
		return PartialScore{
			Score:      1.0,
			Confidence: 1.0,
			Status:     ScoreStatusComplete,
			Feedback:   "No required tools to evaluate",
			Action:     ActionContinue,
		}, nil
	}

	// Match tools using the same logic as the base scorer
	matched, mismatched, missing, extra := s.matchToolsUnordered(expected, actual)

	// Calculate score
	score := float64(len(matched)) / float64(requiredCount)

	// Validate score
	if err := ValidateScore(score); err != nil {
		return PartialScore{}, fmt.Errorf("invalid streaming tool correctness score: %w", err)
	}

	// Calculate confidence based on how many required tools we've seen
	// In unordered mode, we can't be fully confident until all tools are called
	toolsSeen := len(matched) + len(mismatched)
	confidence := float64(toolsSeen) / float64(requiredCount)
	if confidence > 1.0 {
		confidence = 1.0
	}
	// Reduce confidence in unordered mode since we can't be sure about missing tools yet
	confidence *= 0.8

	// Determine status
	status := ScoreStatusPartial
	if len(missing) == 0 && toolsSeen >= requiredCount {
		status = ScoreStatusComplete
	}

	// Determine action
	action := s.determineAction(score, len(mismatched), requiredCount)

	// Build feedback message
	feedback := s.buildUnorderedFeedback(len(matched), len(mismatched), len(missing), requiredCount)

	return PartialScore{
		Score:      score,
		Confidence: confidence,
		Status:     status,
		Feedback:   feedback,
		Action:     action,
		Details: map[string]any{
			"matched":        len(matched),
			"mismatched":     len(mismatched),
			"missing":        len(missing),
			"extra":          len(extra),
			"required_total": requiredCount,
			"progress":       fmt.Sprintf("%d/%d", len(matched), requiredCount),
			"order_matters":  false,
		},
	}, nil
}

// determineAction recommends an action based on score and mismatches.
func (s *streamingToolCorrectnessScorer) determineAction(score float64, mismatches, total int) RecommendedAction {
	// If we have mismatches, agent needs to adjust
	if mismatches > 0 {
		// High mismatch rate = reconsider strategy
		if float64(mismatches)/float64(total) > 0.3 {
			return ActionReconsider
		}
		// Some mismatches = adjust approach
		return ActionAdjust
	}

	// No mismatches = agent is on the right track, continue
	return ActionContinue
}

// buildOrderedFeedback creates a feedback message for ordered tool evaluation.
func (s *streamingToolCorrectnessScorer) buildOrderedFeedback(matched, mismatched, total int, expected []ExpectedToolCall, expectedIdx int) string {
	if mismatched > 0 {
		// Provide guidance about which tool was expected
		nextRequired := ""
		for i := expectedIdx; i < len(expected); i++ {
			if expected[i].Required {
				nextRequired = expected[i].Name
				break
			}
		}

		if nextRequired != "" {
			return fmt.Sprintf("Tool sequence mismatch detected. Matched %d/%d required tools correctly. "+
				"Expected next tool: %s. Review the expected tool sequence and adjust your approach.",
				matched, total, nextRequired)
		}

		return fmt.Sprintf("Tool sequence mismatch detected. Matched %d/%d required tools correctly. "+
			"Review the expected tool sequence and adjust your approach.",
			matched, total)
	}

	if matched == total {
		return fmt.Sprintf("All %d required tools called correctly in sequence. Excellent!", matched)
	}

	// No mismatches but not complete - still in progress
	nextRequired := ""
	for i := expectedIdx; i < len(expected); i++ {
		if expected[i].Required {
			nextRequired = expected[i].Name
			break
		}
	}

	if nextRequired != "" {
		return fmt.Sprintf("Good progress: %d/%d required tools called correctly. Next expected tool: %s",
			matched, total, nextRequired)
	}

	return fmt.Sprintf("Good progress: %d/%d required tools called correctly. Continue executing the sequence.",
		matched, total)
}

// buildUnorderedFeedback creates a feedback message for unordered tool evaluation.
func (s *streamingToolCorrectnessScorer) buildUnorderedFeedback(matched, mismatched, missing, total int) string {
	if mismatched > 0 {
		return fmt.Sprintf("Some tools called with incorrect arguments. Matched %d/%d required tools. "+
			"%d tool(s) with argument mismatches. Review tool parameters and adjust.",
			matched, total, mismatched)
	}

	if missing == 0 && matched == total {
		return fmt.Sprintf("All %d required tools called correctly. Excellent!", matched)
	}

	if matched > 0 {
		return fmt.Sprintf("Progress: %d/%d required tools called correctly. %d tool(s) still needed.",
			matched, total, missing)
	}

	return fmt.Sprintf("No required tools called yet. %d required tool(s) expected.", total)
}
