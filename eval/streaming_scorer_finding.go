// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Zero Root AI

package eval

import (
	"context"
	"fmt"

	"github.com/zeroroot-ai/sdk/finding"
)

// streamingFindingAccuracyScorer is a streaming version of FindingAccuracyScorer
// that can evaluate partial trajectories as findings are discovered.
type streamingFindingAccuracyScorer struct {
	*FindingAccuracyScorer
}

// NewStreamingFindingAccuracyScorer creates a new streaming finding accuracy scorer.
func NewStreamingFindingAccuracyScorer(opts FindingAccuracyOptions) StreamingScorer {
	// Set default fuzzy threshold if not specified
	if opts.FuzzyTitleThreshold == 0.0 {
		opts.FuzzyTitleThreshold = 0.8
	}
	return &streamingFindingAccuracyScorer{
		FindingAccuracyScorer: &FindingAccuracyScorer{
			options: opts,
		},
	}
}

// ScorePartial evaluates a partial trajectory and returns streaming feedback.
// It calculates precision on discovered findings so far and flags false positives.
func (s *streamingFindingAccuracyScorer) ScorePartial(ctx context.Context, trajectory Trajectory) (PartialScore, error) {
	// Get ground truth findings
	groundTruth := s.options.GroundTruth
	if len(groundTruth) == 0 {
		// No ground truth - can't evaluate yet
		return PartialScore{
			Score:      1.0,
			Confidence: 0.0,
			Status:     ScoreStatusPending,
			Action:     ActionContinue,
			Feedback:   "No ground truth findings available for evaluation",
			Details: map[string]any{
				"warning": "no ground truth findings provided",
			},
		}, nil
	}

	// Extract findings from trajectory steps
	actualFindings, err := s.extractFindingsFromTrajectory(trajectory)
	if err != nil {
		return PartialScore{}, fmt.Errorf("failed to extract findings: %w", err)
	}

	// If no findings yet, return pending status
	if len(actualFindings) == 0 {
		return PartialScore{
			Score:      0.5,
			Confidence: 0.0,
			Status:     ScoreStatusPending,
			Action:     ActionContinue,
			Feedback:   "No findings discovered yet. Continue searching.",
			Details: map[string]any{
				"actual_count":       0,
				"ground_truth_count": len(groundTruth),
			},
		}, nil
	}

	// Match findings against ground truth
	tp, fp, fn := s.matchFindings(actualFindings, groundTruth)

	// Calculate counts (with optional severity weighting)
	var tpCount, fpCount, fnCount float64

	if s.options.MatchBySeverity {
		tpCount = s.calculateWeightedCount(tp)
		fpCount = s.calculateWeightedCount(fp)
		fnCount = s.calculateWeightedCountGroundTruth(fn)
	} else {
		tpCount = float64(len(tp))
		fpCount = float64(len(fp))
		fnCount = float64(len(fn))
	}

	// Calculate precision - this is reliable on partial data
	var precision float64
	if tpCount+fpCount > 0 {
		precision = tpCount / (tpCount + fpCount)
	}

	// Calculate partial recall - confidence is LOW since we don't know if more findings will be discovered
	var partialRecall float64
	if tpCount+fnCount > 0 {
		partialRecall = tpCount / (tpCount + fnCount)
	}

	// Score is primarily based on precision (since recall is uncertain)
	// We use precision as the score during streaming
	score := precision

	// Confidence calculation:
	// - High confidence if we have several findings to evaluate
	// - Low confidence if we only have 1-2 findings
	// - Medium confidence for recall (since trajectory is incomplete)
	confidence := s.calculateConfidence(len(actualFindings), len(tp), len(fp))

	// Determine status
	status := ScoreStatusPartial
	if trajectory.EndTime.IsZero() {
		status = ScoreStatusPartial
	} else {
		status = ScoreStatusComplete
	}

	// Determine action and feedback
	action, feedback := s.determineActionAndFeedback(precision, partialRecall, len(tp), len(fp), len(fn), status)

	// Build details
	tpList := make([]map[string]any, len(tp))
	for i, f := range tp {
		tpList[i] = s.findingToMap(f)
	}

	fpList := make([]map[string]any, len(fp))
	for i, f := range fp {
		fpList[i] = s.findingToMap(f)
	}

	fnList := make([]map[string]any, len(fn))
	for i, gt := range fn {
		fnList[i] = map[string]any{
			"id":       gt.ID,
			"title":    gt.Title,
			"severity": gt.Severity,
			"category": gt.Category,
		}
	}

	details := map[string]any{
		"precision":          precision,
		"partial_recall":     partialRecall,
		"recall_confidence":  "low", // Always low for partial trajectories
		"true_positives":     tpList,
		"false_positives":    fpList,
		"false_negatives":    fnList,
		"tp_count":           len(tp),
		"fp_count":           len(fp),
		"fn_count":           len(fn),
		"ground_truth_count": len(groundTruth),
		"actual_count":       len(actualFindings),
	}

	if s.options.MatchBySeverity {
		details["weighted_tp_count"] = tpCount
		details["weighted_fp_count"] = fpCount
		details["weighted_fn_count"] = fnCount
	}

	return PartialScore{
		Score:      score,
		Confidence: confidence,
		Status:     status,
		Action:     action,
		Feedback:   feedback,
		Details:    details,
	}, nil
}

// SupportsStreaming returns true since this scorer can evaluate partial trajectories.
func (s *streamingFindingAccuracyScorer) SupportsStreaming() bool {
	return true
}

// extractFindingsFromTrajectory extracts findings from trajectory steps.
func (s *streamingFindingAccuracyScorer) extractFindingsFromTrajectory(trajectory Trajectory) ([]*finding.Finding, error) {
	var findings []*finding.Finding

	for _, step := range trajectory.Steps {
		if step.Type == "finding" {
			// Try to parse the output as a finding
			f, err := s.parseStepFinding(step)
			if err != nil {
				// Log but don't fail - might be a different format
				continue
			}
			findings = append(findings, f)
		}
	}

	return findings, nil
}

// calculateConfidence determines confidence level based on the number of findings evaluated.
// Confidence is higher when we have more findings to evaluate.
func (s *streamingFindingAccuracyScorer) calculateConfidence(actualCount, tpCount, fpCount int) float64 {
	// Base confidence on number of findings evaluated
	totalEvaluated := tpCount + fpCount

	switch {
	case totalEvaluated == 0:
		return 0.0 // No findings yet
	case totalEvaluated == 1:
		return 0.3 // Very low confidence with just one finding
	case totalEvaluated == 2:
		return 0.5 // Low confidence with two findings
	case totalEvaluated <= 4:
		return 0.7 // Medium confidence with 3-4 findings
	default:
		return 0.85 // High confidence with 5+ findings
		// Note: Never 1.0 confidence during streaming since trajectory is incomplete
	}
}

// determineActionAndFeedback determines the recommended action and feedback message.
func (s *streamingFindingAccuracyScorer) determineActionAndFeedback(
	precision, recall float64,
	tpCount, fpCount, fnCount int,
	status ScoreStatus,
) (RecommendedAction, string) {
	// Handle different scenarios

	// Scenario 1: High false positive rate (precision < 50%)
	if precision < 0.5 && fpCount > 0 {
		feedback := fmt.Sprintf(
			"Warning: High false positive rate detected. Found %d false positive(s) vs %d true positive(s). "+
				"Precision: %.1f%%. Please verify findings against ground truth and refine detection criteria.",
			fpCount, tpCount, precision*100,
		)
		return ActionAdjust, feedback
	}

	// Scenario 2: Some false positives but reasonable precision (50-80%)
	if precision >= 0.5 && precision < 0.8 && fpCount > 0 {
		feedback := fmt.Sprintf(
			"Moderate false positive rate. Found %d false positive(s) vs %d true positive(s). "+
				"Precision: %.1f%%. Consider refining detection to improve accuracy.",
			fpCount, tpCount, precision*100,
		)
		return ActionAdjust, feedback
	}

	// Scenario 3: Good precision (80%+) with few false positives
	if precision >= 0.8 {
		if status == ScoreStatusComplete {
			// Trajectory is complete, evaluate recall
			if fnCount > 0 {
				feedback := fmt.Sprintf(
					"Good precision (%.1f%%) but missed %d expected finding(s). "+
						"Found %d/%d expected findings. Consider expanding search coverage.",
					precision*100, fnCount, tpCount, tpCount+fnCount,
				)
				return ActionContinue, feedback
			}
			feedback := fmt.Sprintf(
				"Excellent! Achieved %.1f%% precision with %d true positive(s) and minimal false positives. "+
					"All expected findings discovered.",
				precision*100, tpCount,
			)
			return ActionContinue, feedback
		}

		// Trajectory is partial
		if fnCount > 0 {
			feedback := fmt.Sprintf(
				"Good progress! Precision: %.1f%% (%d true positives, %d false positives). "+
					"Still missing %d expected finding(s). Continue searching.",
				precision*100, tpCount, fpCount, fnCount,
			)
			return ActionContinue, feedback
		}
		feedback := fmt.Sprintf(
			"Excellent precision (%.1f%%) with %d true positive(s) found. Continue to ensure comprehensive coverage.",
			precision*100, tpCount,
		)
		return ActionContinue, feedback
	}

	// Scenario 4: No findings or perfect (rare)
	if tpCount == 0 && fpCount == 0 {
		feedback := "No findings discovered yet. Continue searching for vulnerabilities."
		return ActionContinue, feedback
	}

	// Default case
	feedback := fmt.Sprintf(
		"Current precision: %.1f%% (%d true positives, %d false positives). Continue evaluation.",
		precision*100, tpCount, fpCount,
	)
	return ActionContinue, feedback
}
