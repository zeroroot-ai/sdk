// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Zero Root AI

package sdk

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/zeroroot-ai/sdk/finding"
)

// newFinding is a convenience helper that returns a minimal Finding.
func newFinding(id, title, desc, category string, sev finding.Severity, targetID string) finding.Finding {
	now := time.Now()
	return finding.Finding{
		ID:          id,
		MissionID:   "mission-1",
		AgentName:   "test-agent",
		Title:       title,
		Description: desc,
		Category:    category,
		Severity:    sev,
		Status:      finding.StatusOpen,
		TargetID:    targetID,
		Confidence:  1.0,
		CreatedAt:   now,
		UpdatedAt:   now,
	}
}

func TestBuildSARIF_ThreeFindingsDifferentSeverities_CorrectLevels(t *testing.T) {
	findings := []finding.Finding{
		newFinding("f1", "Critical Issue", "A critical RCE", "injection", finding.SeverityCritical, ""),
		newFinding("f2", "Medium Issue", "A medium info leak", "disclosure", finding.SeverityMedium, ""),
		newFinding("f3", "Low Issue", "A low risk finding", "hardening", finding.SeverityLow, ""),
	}

	log := buildSARIF(findings, "v0.1.0")

	if len(log.Runs) != 1 {
		t.Fatalf("expected 1 run, got %d", len(log.Runs))
	}
	results := log.Runs[0].Results
	if len(results) != 3 {
		t.Fatalf("expected 3 results, got %d", len(results))
	}

	wantLevels := []string{"error", "warning", "note"}
	for i, want := range wantLevels {
		if results[i].Level != want {
			t.Errorf("result[%d].Level = %q, want %q", i, results[i].Level, want)
		}
	}
}

func TestBuildSARIF_HighSeverityMapsToError(t *testing.T) {
	findings := []finding.Finding{
		newFinding("f1", "High Finding", "details", "xss", finding.SeverityHigh, ""),
	}
	log := buildSARIF(findings, "v0.1.0")
	got := log.Runs[0].Results[0].Level
	if got != "error" {
		t.Errorf("high severity: Level = %q, want %q", got, "error")
	}
}

func TestBuildSARIF_InfoSeverityMapsToNote(t *testing.T) {
	findings := []finding.Finding{
		newFinding("f1", "Info Finding", "details", "recon", finding.SeverityInfo, ""),
	}
	log := buildSARIF(findings, "v0.1.0")
	got := log.Runs[0].Results[0].Level
	if got != "note" {
		t.Errorf("info severity: Level = %q, want %q", got, "note")
	}
}

func TestBuildSARIF_TwoFindingsSameCategory_OneRule(t *testing.T) {
	findings := []finding.Finding{
		newFinding("f1", "Finding A", "desc a", "injection", finding.SeverityCritical, ""),
		newFinding("f2", "Finding B", "desc b", "injection", finding.SeverityHigh, ""),
	}

	log := buildSARIF(findings, "v0.1.0")
	rules := log.Runs[0].Tool.Driver.Rules
	if len(rules) != 1 {
		t.Fatalf("expected 1 deduplicated rule, got %d", len(rules))
	}
	if rules[0].ID != "injection" {
		t.Errorf("rule[0].ID = %q, want %q", rules[0].ID, "injection")
	}

	// Both results must reference the same rule.
	for i, r := range log.Runs[0].Results {
		if r.RuleID != "injection" {
			t.Errorf("result[%d].RuleID = %q, want %q", i, r.RuleID, "injection")
		}
	}
}

func TestBuildSARIF_FindingWithTargetID_LocationPresent(t *testing.T) {
	findings := []finding.Finding{
		newFinding("f1", "Host Scan Finding", "details", "network", finding.SeverityHigh, "https://example.com/api"),
	}

	log := buildSARIF(findings, "v0.1.0")
	res := log.Runs[0].Results[0]
	if len(res.Locations) == 0 {
		t.Fatal("expected location for finding with TargetID, got none")
	}
	uri := res.Locations[0].PhysicalLocation.ArtifactLocation.URI
	if uri != "https://example.com/api" {
		t.Errorf("ArtifactLocation.URI = %q, want %q", uri, "https://example.com/api")
	}
}

func TestBuildSARIF_FindingWithoutTargetID_LocationsOmitted(t *testing.T) {
	findings := []finding.Finding{
		newFinding("f1", "No URL Finding", "details", "network", finding.SeverityMedium, ""),
	}

	log := buildSARIF(findings, "v0.1.0")
	res := log.Runs[0].Results[0]
	if len(res.Locations) != 0 {
		t.Errorf("expected no locations for finding without TargetID, got %d", len(res.Locations))
	}
}

func TestBuildSARIF_ValidJSON(t *testing.T) {
	findings := []finding.Finding{
		newFinding("f1", "RCE", "Remote code execution", "injection", finding.SeverityCritical, "https://target.example.com"),
		newFinding("f2", "Info Leak", "Information disclosure", "disclosure", finding.SeverityMedium, ""),
	}

	log := buildSARIF(findings, "v1.0.0")

	data, err := json.Marshal(log)
	if err != nil {
		t.Fatalf("json.Marshal failed: %v", err)
	}

	var decoded sarifLog
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("json.Unmarshal failed: %v", err)
	}

	if decoded.Schema != sarifSchema {
		t.Errorf("$schema = %q, want %q", decoded.Schema, sarifSchema)
	}
	if decoded.Version != sarifVersion {
		t.Errorf("version = %q, want %q", decoded.Version, sarifVersion)
	}
	if len(decoded.Runs) != 1 {
		t.Fatalf("runs = %d, want 1", len(decoded.Runs))
	}
	if decoded.Runs[0].Tool.Driver.Name != "Gibson" {
		t.Errorf("driver.name = %q, want Gibson", decoded.Runs[0].Tool.Driver.Name)
	}
	if decoded.Runs[0].Tool.Driver.Version != "v1.0.0" {
		t.Errorf("driver.version = %q, want v1.0.0", decoded.Runs[0].Tool.Driver.Version)
	}
}

func TestBuildSARIF_EmptyFindings_EmptyResultsAndRules(t *testing.T) {
	log := buildSARIF([]finding.Finding{}, "v0.1.0")

	if len(log.Runs) != 1 {
		t.Fatalf("expected 1 run, got %d", len(log.Runs))
	}
	if len(log.Runs[0].Results) != 0 {
		t.Errorf("expected 0 results, got %d", len(log.Runs[0].Results))
	}
	if len(log.Runs[0].Tool.Driver.Rules) != 0 {
		t.Errorf("expected 0 rules, got %d", len(log.Runs[0].Tool.Driver.Rules))
	}
}

func TestBuildSARIF_MessageText_TitleColonDescription(t *testing.T) {
	findings := []finding.Finding{
		newFinding("f1", "SQL Injection", "User input unsanitized", "injection", finding.SeverityCritical, ""),
	}

	log := buildSARIF(findings, "v0.1.0")
	msg := log.Runs[0].Results[0].Message.Text
	want := "SQL Injection: User input unsanitized"
	if msg != want {
		t.Errorf("message.text = %q, want %q", msg, want)
	}
}

func TestBuildSARIF_RulesPreAllocated_LargeSet(t *testing.T) {
	// Verify pre-allocation works correctly for a larger set without panic.
	const count = 1000
	findings := make([]finding.Finding, count)
	for i := range count {
		findings[i] = newFinding(
			"f"+string(rune('0'+i%10)),
			"Finding",
			"desc",
			"category",
			finding.SeverityLow,
			"",
		)
	}

	log := buildSARIF(findings, "v0.1.0")
	if len(log.Runs[0].Results) != count {
		t.Errorf("expected %d results, got %d", count, len(log.Runs[0].Results))
	}
	// All same category → exactly 1 rule.
	if len(log.Runs[0].Tool.Driver.Rules) != 1 {
		t.Errorf("expected 1 rule, got %d", len(log.Runs[0].Tool.Driver.Rules))
	}
}
