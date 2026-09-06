// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Zero Root AI

package codegen

import (
	"testing"
	"time"
)

func TestMatchTypeString(t *testing.T) {
	tests := []struct {
		name      string
		matchType MatchType
		want      string
	}{
		{"exact match", MatchExact, "exact"},
		{"fuzzy match", MatchFuzzy, "fuzzy"},
		{"failed match", MatchFailed, "failed"},
		{"unknown match", MatchType(99), "unknown"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.matchType.String(); got != tt.want {
				t.Errorf("MatchType.String() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestValidationStatusString(t *testing.T) {
	tests := []struct {
		name   string
		status ValidationStatus
		want   string
	}{
		{"passed", ValidationPassed, "passed"},
		{"warnings", ValidationWarnings, "warnings"},
		{"failed", ValidationFailed, "failed"},
		{"unknown", ValidationStatus(99), "unknown"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.status.String(); got != tt.want {
				t.Errorf("ValidationStatus.String() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestDiagnosticSeverityString(t *testing.T) {
	tests := []struct {
		name     string
		severity DiagnosticSeverity
		want     string
	}{
		{"error", SeverityError, "error"},
		{"warning", SeverityWarning, "warning"},
		{"info", SeverityInfo, "info"},
		{"hint", SeverityHint, "hint"},
		{"unknown", DiagnosticSeverity(99), "unknown"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.severity.String(); got != tt.want {
				t.Errorf("DiagnosticSeverity.String() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestDiagnosticIsError(t *testing.T) {
	tests := []struct {
		name     string
		severity DiagnosticSeverity
		want     bool
	}{
		{"error severity", SeverityError, true},
		{"warning severity", SeverityWarning, false},
		{"info severity", SeverityInfo, false},
		{"hint severity", SeverityHint, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			d := &Diagnostic{Severity: tt.severity}
			if got := d.IsError(); got != tt.want {
				t.Errorf("Diagnostic.IsError() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestDiagnosticIsWarning(t *testing.T) {
	tests := []struct {
		name     string
		severity DiagnosticSeverity
		want     bool
	}{
		{"error severity", SeverityError, false},
		{"warning severity", SeverityWarning, true},
		{"info severity", SeverityInfo, false},
		{"hint severity", SeverityHint, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			d := &Diagnostic{Severity: tt.severity}
			if got := d.IsWarning(); got != tt.want {
				t.Errorf("Diagnostic.IsWarning() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestCodeChangeSetStructure(t *testing.T) {
	// Test that CodeChangeSet can be created with all fields
	now := time.Now()
	changeset := CodeChangeSet{
		ID:            "test-id",
		WorkspaceName: "test-repo",
		FindingIDs:    []string{"finding-1", "finding-2"},
		Patches: []AppliedPatch{
			{
				FilePath:     "main.go",
				SearchBlock:  "old code",
				ReplaceBlock: "new code",
				MatchType:    MatchExact,
				LinesChanged: 5,
			},
		},
		CommitSHA:        "abc123",
		BranchName:       "fix/security-issue",
		ValidationStatus: ValidationPassed,
		Diagnostics: []Diagnostic{
			{
				Path:     "main.go",
				Line:     10,
				Column:   5,
				Severity: SeverityWarning,
				Message:  "unused variable",
				Source:   "gopls",
			},
		},
		CreatedAt: now,
		CreatedBy: "test-agent",
	}

	// Verify fields are accessible
	if changeset.ID != "test-id" {
		t.Errorf("expected ID to be 'test-id', got %s", changeset.ID)
	}
	if len(changeset.Patches) != 1 {
		t.Errorf("expected 1 patch, got %d", len(changeset.Patches))
	}
	if changeset.Patches[0].MatchType != MatchExact {
		t.Errorf("expected MatchExact, got %v", changeset.Patches[0].MatchType)
	}
}

func TestAppliedPatchStructure(t *testing.T) {
	patch := AppliedPatch{
		FilePath:     "test.go",
		SearchBlock:  "search",
		ReplaceBlock: "replace",
		MatchType:    MatchFuzzy,
		LinesChanged: 3,
	}

	if patch.MatchType != MatchFuzzy {
		t.Errorf("expected MatchFuzzy, got %v", patch.MatchType)
	}
	if patch.LinesChanged != 3 {
		t.Errorf("expected 3 lines changed, got %d", patch.LinesChanged)
	}
}

func TestDiagnosticStructure(t *testing.T) {
	diag := Diagnostic{
		Path:      "main.go",
		Line:      10,
		Column:    5,
		EndLine:   10,
		EndColumn: 15,
		Severity:  SeverityError,
		Message:   "syntax error",
		Source:    "gopls",
	}

	if !diag.IsError() {
		t.Error("expected diagnostic to be an error")
	}
	if diag.IsWarning() {
		t.Error("expected diagnostic not to be a warning")
	}
	if diag.Message != "syntax error" {
		t.Errorf("expected message 'syntax error', got %s", diag.Message)
	}
}
