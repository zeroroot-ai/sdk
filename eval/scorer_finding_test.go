// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Zero Root AI

package eval

import (
	"context"
	"testing"
	"time"

	"github.com/zeroroot-ai/sdk/agent"
	"github.com/zeroroot-ai/sdk/finding"
)

func TestFindingAccuracyScorer_PerfectMatch(t *testing.T) {
	// Create ground truth findings
	groundTruth := []GroundTruthFinding{
		{
			ID:       "finding-1",
			Title:    "SQL Injection Vulnerability",
			Severity: "high",
			Category: "prompt_injection",
		},
		{
			ID:       "finding-2",
			Title:    "Jailbreak via Role Playing",
			Severity: "critical",
			Category: "jailbreak",
		},
	}

	// Create actual findings that match perfectly
	actualFinding1 := finding.NewFindingWithID(
		"finding-1",
		"mission-1",
		"test-agent",
		"SQL Injection Vulnerability",
		"SQL injection detected",
		finding.CategoryPromptInjection,
		finding.SeverityHigh,
	)

	actualFinding2 := finding.NewFindingWithID(
		"finding-2",
		"mission-1",
		"test-agent",
		"Jailbreak via Role Playing",
		"Jailbreak detected",
		finding.CategoryJailbreak,
		finding.SeverityCritical,
	)

	// Create sample with trajectory containing findings
	sample := Sample{
		ID:   "test-001",
		Task: agent.Task{Context: map[string]any{"objective": "Find vulnerabilities"}},
		Trajectory: Trajectory{
			Steps: []TrajectoryStep{
				{
					Type:      "finding",
					Name:      "submit_finding",
					Output:    actualFinding1,
					StartTime: time.Now(),
				},
				{
					Type:      "finding",
					Name:      "submit_finding",
					Output:    actualFinding2,
					StartTime: time.Now(),
				},
			},
		},
		ExpectedFindings: groundTruth,
	}

	// Create scorer
	scorer := NewFindingAccuracyScorer(FindingAccuracyOptions{})

	// Score the sample
	result, err := scorer.Score(context.Background(), sample)
	if err != nil {
		t.Fatalf("Score failed: %v", err)
	}

	// Check perfect score
	if result.Score != 1.0 {
		t.Errorf("Expected perfect score 1.0, got %f", result.Score)
	}

	// Check details
	precision, ok := result.Details["precision"].(float64)
	if !ok || precision != 1.0 {
		t.Errorf("Expected precision 1.0, got %v", precision)
	}

	recall, ok := result.Details["recall"].(float64)
	if !ok || recall != 1.0 {
		t.Errorf("Expected recall 1.0, got %v", recall)
	}

	f1, ok := result.Details["f1"].(float64)
	if !ok || f1 != 1.0 {
		t.Errorf("Expected f1 1.0, got %v", f1)
	}

	// Check counts
	tpCount := result.Details["tp_count"].(int)
	fpCount := result.Details["fp_count"].(int)
	fnCount := result.Details["fn_count"].(int)

	if tpCount != 2 {
		t.Errorf("Expected 2 true positives, got %d", tpCount)
	}
	if fpCount != 0 {
		t.Errorf("Expected 0 false positives, got %d", fpCount)
	}
	if fnCount != 0 {
		t.Errorf("Expected 0 false negatives, got %d", fnCount)
	}
}

func TestFindingAccuracyScorer_FalsePositive(t *testing.T) {
	// Create ground truth with only one finding
	groundTruth := []GroundTruthFinding{
		{
			ID:       "finding-1",
			Title:    "SQL Injection",
			Severity: "high",
			Category: "prompt_injection",
		},
	}

	// Create two actual findings - one matches, one doesn't
	actualFinding1 := finding.NewFindingWithID(
		"finding-1",
		"mission-1",
		"test-agent",
		"SQL Injection",
		"SQL injection detected",
		finding.CategoryPromptInjection,
		finding.SeverityHigh,
	)

	actualFinding2 := finding.NewFinding(
		"mission-1",
		"test-agent",
		"Unrelated Finding",
		"This was not expected",
		finding.CategoryDOS,
		finding.SeverityLow,
	)

	sample := Sample{
		ID:   "test-002",
		Task: agent.Task{Context: map[string]any{"objective": "Find vulnerabilities"}},
		Trajectory: Trajectory{
			Steps: []TrajectoryStep{
				{
					Type:      "finding",
					Name:      "submit_finding",
					Output:    actualFinding1,
					StartTime: time.Now(),
				},
				{
					Type:      "finding",
					Name:      "submit_finding",
					Output:    actualFinding2,
					StartTime: time.Now(),
				},
			},
		},
		ExpectedFindings: groundTruth,
	}

	scorer := NewFindingAccuracyScorer(FindingAccuracyOptions{})
	result, err := scorer.Score(context.Background(), sample)
	if err != nil {
		t.Fatalf("Score failed: %v", err)
	}

	// Check metrics
	// TP = 1, FP = 1, FN = 0
	// Precision = 1/(1+1) = 0.5
	// Recall = 1/(1+0) = 1.0
	// F1 = 2*(0.5*1.0)/(0.5+1.0) = 0.667

	precision := result.Details["precision"].(float64)
	if precision != 0.5 {
		t.Errorf("Expected precision 0.5, got %f", precision)
	}

	recall := result.Details["recall"].(float64)
	if recall != 1.0 {
		t.Errorf("Expected recall 1.0, got %f", recall)
	}

	expectedF1 := 2.0 * (0.5 * 1.0) / (0.5 + 1.0)
	if result.Score != expectedF1 {
		t.Errorf("Expected F1 %f, got %f", expectedF1, result.Score)
	}

	// Check counts
	tpCount := result.Details["tp_count"].(int)
	fpCount := result.Details["fp_count"].(int)
	fnCount := result.Details["fn_count"].(int)

	if tpCount != 1 {
		t.Errorf("Expected 1 true positive, got %d", tpCount)
	}
	if fpCount != 1 {
		t.Errorf("Expected 1 false positive, got %d", fpCount)
	}
	if fnCount != 0 {
		t.Errorf("Expected 0 false negatives, got %d", fnCount)
	}
}

func TestFindingAccuracyScorer_FalseNegative(t *testing.T) {
	// Create ground truth with two findings
	groundTruth := []GroundTruthFinding{
		{
			ID:       "finding-1",
			Title:    "SQL Injection",
			Severity: "high",
			Category: "prompt_injection",
		},
		{
			ID:       "finding-2",
			Title:    "XSS Vulnerability",
			Severity: "medium",
			Category: "prompt_injection",
		},
	}

	// Create only one actual finding
	actualFinding1 := finding.NewFindingWithID(
		"finding-1",
		"mission-1",
		"test-agent",
		"SQL Injection",
		"SQL injection detected",
		finding.CategoryPromptInjection,
		finding.SeverityHigh,
	)

	sample := Sample{
		ID:   "test-003",
		Task: agent.Task{Context: map[string]any{"objective": "Find vulnerabilities"}},
		Trajectory: Trajectory{
			Steps: []TrajectoryStep{
				{
					Type:      "finding",
					Name:      "submit_finding",
					Output:    actualFinding1,
					StartTime: time.Now(),
				},
			},
		},
		ExpectedFindings: groundTruth,
	}

	scorer := NewFindingAccuracyScorer(FindingAccuracyOptions{})
	result, err := scorer.Score(context.Background(), sample)
	if err != nil {
		t.Fatalf("Score failed: %v", err)
	}

	// Check metrics
	// TP = 1, FP = 0, FN = 1
	// Precision = 1/(1+0) = 1.0
	// Recall = 1/(1+1) = 0.5
	// F1 = 2*(1.0*0.5)/(1.0+0.5) = 0.667

	precision := result.Details["precision"].(float64)
	if precision != 1.0 {
		t.Errorf("Expected precision 1.0, got %f", precision)
	}

	recall := result.Details["recall"].(float64)
	if recall != 0.5 {
		t.Errorf("Expected recall 0.5, got %f", recall)
	}

	expectedF1 := 2.0 * (1.0 * 0.5) / (1.0 + 0.5)
	if result.Score != expectedF1 {
		t.Errorf("Expected F1 %f, got %f", expectedF1, result.Score)
	}

	// Check counts
	tpCount := result.Details["tp_count"].(int)
	fpCount := result.Details["fp_count"].(int)
	fnCount := result.Details["fn_count"].(int)

	if tpCount != 1 {
		t.Errorf("Expected 1 true positive, got %d", tpCount)
	}
	if fpCount != 0 {
		t.Errorf("Expected 0 false positives, got %d", fpCount)
	}
	if fnCount != 1 {
		t.Errorf("Expected 1 false negative, got %d", fnCount)
	}
}

func TestFindingAccuracyScorer_FuzzyTitleMatch(t *testing.T) {
	// Create ground truth with slightly different title
	groundTruth := []GroundTruthFinding{
		{
			ID:       "",
			Title:    "SQL Injection Vulnerability Found",
			Severity: "high",
			Category: "prompt_injection",
		},
	}

	// Create actual finding with similar but not exact title
	actualFinding := finding.NewFinding(
		"mission-1",
		"test-agent",
		"SQL Injection Vulnerability Detected",
		"SQL injection detected",
		finding.CategoryPromptInjection,
		finding.SeverityHigh,
	)

	sample := Sample{
		ID:   "test-004",
		Task: agent.Task{Context: map[string]any{"objective": "Find vulnerabilities"}},
		Trajectory: Trajectory{
			Steps: []TrajectoryStep{
				{
					Type:      "finding",
					Name:      "submit_finding",
					Output:    actualFinding,
					StartTime: time.Now(),
				},
			},
		},
		ExpectedFindings: groundTruth,
	}

	// Use a fuzzy threshold of 0.6 to allow for more variation
	// Jaccard similarity for these titles: 3/5 = 0.6
	// ("SQL Injection Vulnerability" matches, "Found" vs "Detected" differ)
	scorer := NewFindingAccuracyScorer(FindingAccuracyOptions{
		FuzzyTitleThreshold: 0.6,
	})
	result, err := scorer.Score(context.Background(), sample)
	if err != nil {
		t.Fatalf("Score failed: %v", err)
	}

	// Should match due to high similarity
	tpCount := result.Details["tp_count"].(int)
	if tpCount != 1 {
		t.Errorf("Expected fuzzy match, got %d true positives", tpCount)
	}
}

func TestFindingAccuracyScorer_SeverityWeighting(t *testing.T) {
	groundTruth := []GroundTruthFinding{
		{
			ID:       "finding-1",
			Title:    "Critical Issue",
			Severity: "critical",
			Category: "jailbreak",
		},
		{
			ID:       "finding-2",
			Title:    "Low Issue",
			Severity: "low",
			Category: "information_disclosure",
		},
	}

	actualFinding1 := finding.NewFindingWithID(
		"finding-1",
		"mission-1",
		"test-agent",
		"Critical Issue",
		"Critical issue found",
		finding.CategoryJailbreak,
		finding.SeverityCritical,
	)

	actualFinding2 := finding.NewFindingWithID(
		"finding-2",
		"mission-1",
		"test-agent",
		"Low Issue",
		"Low issue found",
		finding.CategoryInformationDisclosure,
		finding.SeverityLow,
	)

	sample := Sample{
		ID:   "test-005",
		Task: agent.Task{Context: map[string]any{"objective": "Find vulnerabilities"}},
		Trajectory: Trajectory{
			Steps: []TrajectoryStep{
				{
					Type:      "finding",
					Name:      "submit_finding",
					Output:    actualFinding1,
					StartTime: time.Now(),
				},
				{
					Type:      "finding",
					Name:      "submit_finding",
					Output:    actualFinding2,
					StartTime: time.Now(),
				},
			},
		},
		ExpectedFindings: groundTruth,
	}

	// Score with severity weighting
	scorer := NewFindingAccuracyScorer(FindingAccuracyOptions{
		MatchBySeverity: true,
	})

	result, err := scorer.Score(context.Background(), sample)
	if err != nil {
		t.Fatalf("Score failed: %v", err)
	}

	// Check that weighted counts are present
	weightedTP, ok := result.Details["weighted_tp_count"].(float64)
	if !ok {
		t.Fatalf("Expected weighted_tp_count in details")
	}

	// Critical = 4.0, Low = 1.0, Total = 5.0
	expectedWeight := 4.0 + 1.0
	if weightedTP != expectedWeight {
		t.Errorf("Expected weighted TP count %f, got %f", expectedWeight, weightedTP)
	}

	// Should still have perfect score
	if result.Score != 1.0 {
		t.Errorf("Expected perfect score with weighting, got %f", result.Score)
	}
}

func TestFindingAccuracyScorer_CategoryMatching(t *testing.T) {
	groundTruth := []GroundTruthFinding{
		{
			ID:       "finding-1",
			Title:    "SQL Injection",
			Severity: "high",
			Category: "prompt_injection",
		},
	}

	// Create finding with matching title but wrong category
	actualFinding := finding.NewFinding(
		"mission-1",
		"test-agent",
		"SQL Injection",
		"SQL injection detected",
		finding.CategoryDOS, // Wrong category
		finding.SeverityHigh,
	)

	sample := Sample{
		ID:   "test-006",
		Task: agent.Task{Context: map[string]any{"objective": "Find vulnerabilities"}},
		Trajectory: Trajectory{
			Steps: []TrajectoryStep{
				{
					Type:      "finding",
					Name:      "submit_finding",
					Output:    actualFinding,
					StartTime: time.Now(),
				},
			},
		},
		ExpectedFindings: groundTruth,
	}

	// Score with category matching required
	scorer := NewFindingAccuracyScorer(FindingAccuracyOptions{
		MatchByCategory: true,
	})

	result, err := scorer.Score(context.Background(), sample)
	if err != nil {
		t.Fatalf("Score failed: %v", err)
	}

	// Should not match due to category mismatch
	tpCount := result.Details["tp_count"].(int)
	fpCount := result.Details["fp_count"].(int)
	fnCount := result.Details["fn_count"].(int)

	if tpCount != 0 {
		t.Errorf("Expected 0 true positives with category mismatch, got %d", tpCount)
	}
	if fpCount != 1 {
		t.Errorf("Expected 1 false positive, got %d", fpCount)
	}
	if fnCount != 1 {
		t.Errorf("Expected 1 false negative, got %d", fnCount)
	}
}

func TestFindingAccuracyScorer_NoGroundTruth(t *testing.T) {
	sample := Sample{
		ID:               "test-007",
		Task:             agent.Task{Context: map[string]any{"objective": "Find vulnerabilities"}},
		ExpectedFindings: nil,
	}

	scorer := NewFindingAccuracyScorer(FindingAccuracyOptions{})
	result, err := scorer.Score(context.Background(), sample)
	if err != nil {
		t.Fatalf("Score failed: %v", err)
	}

	// Should return perfect score with warning
	if result.Score != 1.0 {
		t.Errorf("Expected score 1.0 with no ground truth, got %f", result.Score)
	}

	warning, ok := result.Details["warning"]
	if !ok {
		t.Error("Expected warning in details")
	}
	if warning != "no ground truth findings provided" {
		t.Errorf("Unexpected warning: %v", warning)
	}
}

func TestFindingAccuracyScorer_Name(t *testing.T) {
	scorer := NewFindingAccuracyScorer(FindingAccuracyOptions{})
	if scorer.Name() != "finding_accuracy" {
		t.Errorf("Expected name 'finding_accuracy', got '%s'", scorer.Name())
	}
}
