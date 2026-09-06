// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Zero Root AI

package eval

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/zeroroot-ai/sdk/finding"
)

// FindingAccuracyScorer evaluates the accuracy of findings discovered by an agent.
// It compares actual findings against ground truth to calculate precision, recall, and F1 score.
type FindingAccuracyScorer struct {
	options FindingAccuracyOptions
}

// FindingAccuracyOptions configures the finding accuracy scorer.
type FindingAccuracyOptions struct {
	// GroundTruth contains the expected findings that should be discovered.
	// If nil or empty, the scorer will use sample.ExpectedFindings instead.
	GroundTruth []GroundTruthFinding

	// MatchBySeverity enables severity-weighted scoring.
	// When true, true positives are weighted by severity level:
	// critical=4, high=3, medium=2, low=1, info=0.5
	MatchBySeverity bool

	// MatchByCategory requires findings to match on category.
	// When true, findings must match both title/ID and category to count as true positives.
	MatchByCategory bool

	// FuzzyTitleThreshold is the minimum similarity (0.0 to 1.0) for fuzzy title matching.
	// Default is 0.8. Set to 1.0 to require exact title matches.
	FuzzyTitleThreshold float64
}

// NewFindingAccuracyScorer creates a new finding accuracy scorer with the given options.
func NewFindingAccuracyScorer(opts FindingAccuracyOptions) Scorer {
	// Set default fuzzy threshold if not specified
	if opts.FuzzyTitleThreshold == 0.0 {
		opts.FuzzyTitleThreshold = 0.8
	}
	return &FindingAccuracyScorer{
		options: opts,
	}
}

// Name returns the scorer name.
func (s *FindingAccuracyScorer) Name() string {
	return "finding_accuracy"
}

// Score evaluates finding accuracy against ground truth.
func (s *FindingAccuracyScorer) Score(ctx context.Context, sample Sample) (ScoreResult, error) {
	// Get ground truth findings
	groundTruth := s.options.GroundTruth
	if len(groundTruth) == 0 {
		groundTruth = sample.ExpectedFindings
	}

	// If no ground truth, return perfect score (nothing to compare against)
	if len(groundTruth) == 0 {
		return ScoreResult{
			Score: 1.0,
			Details: map[string]any{
				"precision": 1.0,
				"recall":    1.0,
				"f1":        1.0,
				"warning":   "no ground truth findings provided",
			},
		}, nil
	}

	// Extract actual findings from trajectory
	actualFindings, err := s.extractFindings(sample)
	if err != nil {
		return ScoreResult{Score: 0.0}, fmt.Errorf("failed to extract findings: %w", err)
	}

	// Match findings and calculate metrics
	tp, fp, fn := s.matchFindings(actualFindings, groundTruth)

	// Calculate precision, recall, and F1
	var precision, recall, f1 float64

	tpCount := float64(len(tp))
	fpCount := float64(len(fp))
	fnCount := float64(len(fn))

	// Apply severity weighting if enabled
	if s.options.MatchBySeverity {
		tpCount = s.calculateWeightedCount(tp)
		fpCount = s.calculateWeightedCount(fp)
		fnCount = s.calculateWeightedCountGroundTruth(fn)
	}

	// Calculate precision = TP / (TP + FP)
	if tpCount+fpCount > 0 {
		precision = tpCount / (tpCount + fpCount)
	}

	// Calculate recall = TP / (TP + FN)
	if tpCount+fnCount > 0 {
		recall = tpCount / (tpCount + fnCount)
	}

	// Calculate F1 = 2 * (precision * recall) / (precision + recall)
	if precision+recall > 0 {
		f1 = 2.0 * (precision * recall) / (precision + recall)
	}

	// Build details with lists
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
		"recall":             recall,
		"f1":                 f1,
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

	return ScoreResult{
		Score:   f1,
		Details: details,
	}, nil
}

// extractFindings extracts findings from the sample trajectory or metadata.
func (s *FindingAccuracyScorer) extractFindings(sample Sample) ([]*finding.Finding, error) {
	var findings []*finding.Finding

	// First, check if findings are in trajectory steps
	for _, step := range sample.Trajectory.Steps {
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

	// If no findings in trajectory, check metadata
	if len(findings) == 0 {
		if findingsData, ok := sample.Metadata["findings"]; ok {
			// Try to parse metadata findings
			metaFindings, err := s.parseMetadataFindings(findingsData)
			if err == nil {
				findings = metaFindings
			}
		}
	}

	return findings, nil
}

// parseStepFinding parses a finding from a trajectory step.
func (s *FindingAccuracyScorer) parseStepFinding(step TrajectoryStep) (*finding.Finding, error) {
	// Try to unmarshal the output as a finding
	var f finding.Finding

	// Handle different output formats
	switch output := step.Output.(type) {
	case *finding.Finding:
		return output, nil
	case finding.Finding:
		return &output, nil
	case map[string]any:
		// Marshal to JSON and back to get proper types
		data, err := json.Marshal(output)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal finding: %w", err)
		}
		if err := json.Unmarshal(data, &f); err != nil {
			return nil, fmt.Errorf("failed to unmarshal finding: %w", err)
		}
		return &f, nil
	default:
		return nil, fmt.Errorf("unsupported output type: %T", output)
	}
}

// parseMetadataFindings parses findings from metadata.
func (s *FindingAccuracyScorer) parseMetadataFindings(data any) ([]*finding.Finding, error) {
	// Marshal to JSON and back to get proper types
	jsonData, err := json.Marshal(data)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal findings: %w", err)
	}

	var findings []*finding.Finding
	if err := json.Unmarshal(jsonData, &findings); err != nil {
		return nil, fmt.Errorf("failed to unmarshal findings: %w", err)
	}

	return findings, nil
}

// matchFindings matches actual findings against ground truth.
// Returns (true positives, false positives, false negatives).
func (s *FindingAccuracyScorer) matchFindings(
	actual []*finding.Finding,
	groundTruth []GroundTruthFinding,
) ([]*finding.Finding, []*finding.Finding, []GroundTruthFinding) {
	var truePositives []*finding.Finding
	var falsePositives []*finding.Finding
	var falseNegatives []GroundTruthFinding

	// Track which ground truth findings have been matched
	matchedGT := make(map[int]bool)

	// For each actual finding, try to match against ground truth
	for _, actualFinding := range actual {
		matched := false

		for i, gt := range groundTruth {
			if matchedGT[i] {
				// This ground truth already matched
				continue
			}

			if s.isMatch(actualFinding, gt) {
				// Found a match
				truePositives = append(truePositives, actualFinding)
				matchedGT[i] = true
				matched = true
				break
			}
		}

		if !matched {
			// No match found - this is a false positive
			falsePositives = append(falsePositives, actualFinding)
		}
	}

	// Any unmatched ground truth findings are false negatives
	for i, gt := range groundTruth {
		if !matchedGT[i] {
			falseNegatives = append(falseNegatives, gt)
		}
	}

	return truePositives, falsePositives, falseNegatives
}

// isMatch determines if an actual finding matches a ground truth finding.
func (s *FindingAccuracyScorer) isMatch(actual *finding.Finding, gt GroundTruthFinding) bool {
	// First try exact ID match
	if gt.ID != "" && actual.ID == gt.ID {
		return true
	}

	// Try fuzzy title match
	titleMatch := s.fuzzyTitleMatch(actual.Title, gt.Title)
	if !titleMatch {
		return false
	}

	// If category matching is required, check category
	if s.options.MatchByCategory {
		if string(actual.Category) != gt.Category {
			return false
		}
	}

	return true
}

// fuzzyTitleMatch performs fuzzy string matching on titles.
func (s *FindingAccuracyScorer) fuzzyTitleMatch(actual, expected string) bool {
	// Normalize strings
	actual = strings.ToLower(strings.TrimSpace(actual))
	expected = strings.ToLower(strings.TrimSpace(expected))

	// Exact match
	if actual == expected {
		return true
	}

	// Calculate simple similarity score using Jaccard similarity on words
	similarity := s.jaccardSimilarity(actual, expected)
	return similarity >= s.options.FuzzyTitleThreshold
}

// jaccardSimilarity calculates Jaccard similarity between two strings.
// It splits strings into words and computes intersection over union.
func (s *FindingAccuracyScorer) jaccardSimilarity(s1, s2 string) float64 {
	// Split into word sets
	words1 := strings.Fields(s1)
	words2 := strings.Fields(s2)

	if len(words1) == 0 && len(words2) == 0 {
		return 1.0
	}
	if len(words1) == 0 || len(words2) == 0 {
		return 0.0
	}

	// Build sets
	set1 := make(map[string]bool)
	for _, w := range words1 {
		set1[w] = true
	}

	set2 := make(map[string]bool)
	for _, w := range words2 {
		set2[w] = true
	}

	// Calculate intersection
	intersection := 0
	for w := range set1 {
		if set2[w] {
			intersection++
		}
	}

	// Calculate union
	union := len(set1) + len(set2) - intersection

	if union == 0 {
		return 0.0
	}

	return float64(intersection) / float64(union)
}

// calculateWeightedCount calculates severity-weighted count of findings.
func (s *FindingAccuracyScorer) calculateWeightedCount(findings []*finding.Finding) float64 {
	var total float64
	for _, f := range findings {
		total += s.severityWeight(f.Severity)
	}
	return total
}

// severityWeight returns the weight for a severity level.
func (s *FindingAccuracyScorer) severityWeight(sev finding.Severity) float64 {
	switch sev {
	case finding.SeverityCritical:
		return 4.0
	case finding.SeverityHigh:
		return 3.0
	case finding.SeverityMedium:
		return 2.0
	case finding.SeverityLow:
		return 1.0
	case finding.SeverityInfo:
		return 0.5
	default:
		return 1.0
	}
}

// calculateWeightedCountGroundTruth calculates severity-weighted count of ground truth findings.
func (s *FindingAccuracyScorer) calculateWeightedCountGroundTruth(findings []GroundTruthFinding) float64 {
	var total float64
	for _, f := range findings {
		// Parse severity from string
		sev, err := finding.ParseSeverity(f.Severity)
		if err != nil {
			// If parsing fails, use default weight of 1.0
			total += 1.0
			continue
		}
		total += s.severityWeight(sev)
	}
	return total
}

// findingToMap converts a finding to a map for JSON serialization in details.
func (s *FindingAccuracyScorer) findingToMap(f *finding.Finding) map[string]any {
	return map[string]any{
		"id":       f.ID,
		"title":    f.Title,
		"severity": string(f.Severity),
		"category": string(f.Category),
	}
}
