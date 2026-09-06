// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Zero Root AI

package eval

import (
	"context"
	"testing"
	"time"

	"github.com/zeroroot-ai/sdk/finding"
)

func TestStreamingFindingAccuracyScorer_SupportsStreaming(t *testing.T) {
	scorer := NewStreamingFindingAccuracyScorer(FindingAccuracyOptions{})
	streamingScorer, ok := scorer.(StreamingScorer)
	if !ok {
		t.Fatal("Expected StreamingScorer interface")
	}
	if !streamingScorer.SupportsStreaming() {
		t.Error("Expected SupportsStreaming() to return true")
	}
}

func TestStreamingFindingAccuracyScorer_NoFindings(t *testing.T) {
	groundTruth := []GroundTruthFinding{
		{
			ID:       "finding-1",
			Title:    "SQL Injection",
			Severity: "high",
			Category: "prompt_injection",
		},
	}

	scorer := NewStreamingFindingAccuracyScorer(FindingAccuracyOptions{
		GroundTruth: groundTruth,
	})

	trajectory := Trajectory{
		Steps:     []TrajectoryStep{},
		StartTime: time.Now(),
	}

	result, err := scorer.(StreamingScorer).ScorePartial(context.Background(), trajectory)
	if err != nil {
		t.Fatalf("ScorePartial failed: %v", err)
	}

	// Should be pending with no findings
	if result.Status != ScoreStatusPending {
		t.Errorf("Expected status pending, got %s", result.Status)
	}

	if result.Action != ActionContinue {
		t.Errorf("Expected action continue, got %s", result.Action)
	}

	if result.Confidence != 0.0 {
		t.Errorf("Expected confidence 0.0, got %f", result.Confidence)
	}
}

func TestStreamingFindingAccuracyScorer_OneTruePositive(t *testing.T) {
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

	scorer := NewStreamingFindingAccuracyScorer(FindingAccuracyOptions{
		GroundTruth: groundTruth,
	})

	actualFinding := finding.NewFindingWithID(
		"finding-1",
		"mission-1",
		"test-agent",
		"SQL Injection",
		"SQL injection detected",
		finding.CategoryPromptInjection,
		finding.SeverityHigh,
	)

	trajectory := Trajectory{
		Steps: []TrajectoryStep{
			{
				Type:      "finding",
				Name:      "submit_finding",
				Output:    actualFinding,
				StartTime: time.Now(),
			},
		},
		StartTime: time.Now(),
	}

	result, err := scorer.(StreamingScorer).ScorePartial(context.Background(), trajectory)
	if err != nil {
		t.Fatalf("ScorePartial failed: %v", err)
	}

	// Should be partial with one true positive
	if result.Status != ScoreStatusPartial {
		t.Errorf("Expected status partial, got %s", result.Status)
	}

	if result.Action != ActionContinue {
		t.Errorf("Expected action continue, got %s", result.Action)
	}

	// Precision should be 1.0 (1 TP, 0 FP)
	precision, ok := result.Details["precision"].(float64)
	if !ok || precision != 1.0 {
		t.Errorf("Expected precision 1.0, got %v", precision)
	}

	// Should have 1 TP, 0 FP, 1 FN
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

	// Score should equal precision (1.0)
	if result.Score != 1.0 {
		t.Errorf("Expected score 1.0, got %f", result.Score)
	}

	// Confidence should be low since we only have one finding
	if result.Confidence != 0.3 {
		t.Errorf("Expected confidence 0.3 (one finding), got %f", result.Confidence)
	}
}

func TestStreamingFindingAccuracyScorer_FalsePositive(t *testing.T) {
	groundTruth := []GroundTruthFinding{
		{
			ID:       "finding-1",
			Title:    "SQL Injection",
			Severity: "high",
			Category: "prompt_injection",
		},
	}

	scorer := NewStreamingFindingAccuracyScorer(FindingAccuracyOptions{
		GroundTruth: groundTruth,
	})

	// Create a false positive finding
	falsePositiveFinding := finding.NewFinding(
		"mission-1",
		"test-agent",
		"Unrelated Finding",
		"This was not expected",
		finding.CategoryDOS,
		finding.SeverityLow,
	)

	trajectory := Trajectory{
		Steps: []TrajectoryStep{
			{
				Type:      "finding",
				Name:      "submit_finding",
				Output:    falsePositiveFinding,
				StartTime: time.Now(),
			},
		},
		StartTime: time.Now(),
	}

	result, err := scorer.(StreamingScorer).ScorePartial(context.Background(), trajectory)
	if err != nil {
		t.Fatalf("ScorePartial failed: %v", err)
	}

	// Should detect false positive and suggest adjustment
	if result.Action != ActionAdjust {
		t.Errorf("Expected action adjust for false positive, got %s", result.Action)
	}

	// Precision should be 0.0 (0 TP, 1 FP)
	precision, ok := result.Details["precision"].(float64)
	if !ok || precision != 0.0 {
		t.Errorf("Expected precision 0.0, got %v", precision)
	}

	// Should have 0 TP, 1 FP, 1 FN
	tpCount := result.Details["tp_count"].(int)
	fpCount := result.Details["fp_count"].(int)
	fnCount := result.Details["fn_count"].(int)

	if tpCount != 0 {
		t.Errorf("Expected 0 true positives, got %d", tpCount)
	}
	if fpCount != 1 {
		t.Errorf("Expected 1 false positive, got %d", fpCount)
	}
	if fnCount != 1 {
		t.Errorf("Expected 1 false negative, got %d", fnCount)
	}

	// Score should be 0.0
	if result.Score != 0.0 {
		t.Errorf("Expected score 0.0, got %f", result.Score)
	}
}

func TestStreamingFindingAccuracyScorer_MixedFindings(t *testing.T) {
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

	scorer := NewStreamingFindingAccuracyScorer(FindingAccuracyOptions{
		GroundTruth: groundTruth,
	})

	// One true positive
	trueFinding := finding.NewFindingWithID(
		"finding-1",
		"mission-1",
		"test-agent",
		"SQL Injection",
		"SQL injection detected",
		finding.CategoryPromptInjection,
		finding.SeverityHigh,
	)

	// One false positive
	falseFinding := finding.NewFinding(
		"mission-1",
		"test-agent",
		"Unrelated Finding",
		"This was not expected",
		finding.CategoryDOS,
		finding.SeverityLow,
	)

	trajectory := Trajectory{
		Steps: []TrajectoryStep{
			{
				Type:      "finding",
				Name:      "submit_finding",
				Output:    trueFinding,
				StartTime: time.Now(),
			},
			{
				Type:      "finding",
				Name:      "submit_finding",
				Output:    falseFinding,
				StartTime: time.Now().Add(time.Second),
			},
		},
		StartTime: time.Now(),
	}

	result, err := scorer.(StreamingScorer).ScorePartial(context.Background(), trajectory)
	if err != nil {
		t.Fatalf("ScorePartial failed: %v", err)
	}

	// Precision should be 0.5 (1 TP, 1 FP)
	precision, ok := result.Details["precision"].(float64)
	if !ok || precision != 0.5 {
		t.Errorf("Expected precision 0.5, got %v", precision)
	}

	// Should have 1 TP, 1 FP, 1 FN
	tpCount := result.Details["tp_count"].(int)
	fpCount := result.Details["fp_count"].(int)
	fnCount := result.Details["fn_count"].(int)

	if tpCount != 1 {
		t.Errorf("Expected 1 true positive, got %d", tpCount)
	}
	if fpCount != 1 {
		t.Errorf("Expected 1 false positive, got %d", fpCount)
	}
	if fnCount != 1 {
		t.Errorf("Expected 1 false negative, got %d", fnCount)
	}

	// Score should equal precision (0.5)
	if result.Score != 0.5 {
		t.Errorf("Expected score 0.5, got %f", result.Score)
	}

	// Should suggest adjustment due to false positive
	if result.Action != ActionAdjust {
		t.Errorf("Expected action adjust, got %s", result.Action)
	}

	// Confidence should be 0.5 (two findings)
	if result.Confidence != 0.5 {
		t.Errorf("Expected confidence 0.5 (two findings), got %f", result.Confidence)
	}
}

func TestStreamingFindingAccuracyScorer_HighConfidence(t *testing.T) {
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
		{
			ID:       "finding-3",
			Title:    "CSRF Token Missing",
			Severity: "high",
			Category: "prompt_injection",
		},
		{
			ID:       "finding-4",
			Title:    "Weak Password Policy",
			Severity: "low",
			Category: "information_disclosure",
		},
		{
			ID:       "finding-5",
			Title:    "Missing HTTPS",
			Severity: "medium",
			Category: "information_disclosure",
		},
	}

	scorer := NewStreamingFindingAccuracyScorer(FindingAccuracyOptions{
		GroundTruth: groundTruth,
	})

	// Create 5 true positive findings
	findings := []*finding.Finding{
		finding.NewFindingWithID("finding-1", "mission-1", "test-agent", "SQL Injection", "desc", finding.CategoryPromptInjection, finding.SeverityHigh),
		finding.NewFindingWithID("finding-2", "mission-1", "test-agent", "XSS Vulnerability", "desc", finding.CategoryPromptInjection, finding.SeverityMedium),
		finding.NewFindingWithID("finding-3", "mission-1", "test-agent", "CSRF Token Missing", "desc", finding.CategoryPromptInjection, finding.SeverityHigh),
		finding.NewFindingWithID("finding-4", "mission-1", "test-agent", "Weak Password Policy", "desc", finding.CategoryInformationDisclosure, finding.SeverityLow),
		finding.NewFindingWithID("finding-5", "mission-1", "test-agent", "Missing HTTPS", "desc", finding.CategoryInformationDisclosure, finding.SeverityMedium),
	}

	steps := make([]TrajectoryStep, len(findings))
	for i, f := range findings {
		steps[i] = TrajectoryStep{
			Type:      "finding",
			Name:      "submit_finding",
			Output:    f,
			StartTime: time.Now().Add(time.Duration(i) * time.Second),
		}
	}

	trajectory := Trajectory{
		Steps:     steps,
		StartTime: time.Now(),
	}

	result, err := scorer.(StreamingScorer).ScorePartial(context.Background(), trajectory)
	if err != nil {
		t.Fatalf("ScorePartial failed: %v", err)
	}

	// Precision should be 1.0 (5 TP, 0 FP)
	precision, ok := result.Details["precision"].(float64)
	if !ok || precision != 1.0 {
		t.Errorf("Expected precision 1.0, got %v", precision)
	}

	// Score should be 1.0
	if result.Score != 1.0 {
		t.Errorf("Expected score 1.0, got %f", result.Score)
	}

	// Confidence should be high (0.85) with 5+ findings
	if result.Confidence != 0.85 {
		t.Errorf("Expected confidence 0.85 (5+ findings), got %f", result.Confidence)
	}
}

func TestStreamingFindingAccuracyScorer_NoGroundTruth(t *testing.T) {
	scorer := NewStreamingFindingAccuracyScorer(FindingAccuracyOptions{})

	trajectory := Trajectory{
		Steps:     []TrajectoryStep{},
		StartTime: time.Now(),
	}

	result, err := scorer.(StreamingScorer).ScorePartial(context.Background(), trajectory)
	if err != nil {
		t.Fatalf("ScorePartial failed: %v", err)
	}

	// Should return pending with warning
	if result.Status != ScoreStatusPending {
		t.Errorf("Expected status pending, got %s", result.Status)
	}

	if result.Confidence != 0.0 {
		t.Errorf("Expected confidence 0.0, got %f", result.Confidence)
	}

	warning, ok := result.Details["warning"]
	if !ok {
		t.Error("Expected warning in details")
	}
	if warning != "no ground truth findings provided" {
		t.Errorf("Unexpected warning: %v", warning)
	}
}
