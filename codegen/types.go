// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Zero Root AI

package codegen

import "time"

// CodeChangeSet represents a collection of code changes applied during a mission.
// It is stored in mission memory to share code context between agents.
// This enables downstream agents to understand what upstream agents modified.
type CodeChangeSet struct {
	// ID is a unique identifier for this changeset.
	ID string

	// WorkspaceName identifies which repository these changes apply to.
	WorkspaceName string

	// FindingIDs are optional references to security findings that motivated these changes.
	// This links code modifications to the vulnerabilities they address.
	FindingIDs []string

	// Patches contains the individual code edits that were applied.
	Patches []AppliedPatch

	// CommitSHA is the Git commit hash if the changes were committed.
	// Empty if changes are uncommitted.
	CommitSHA string

	// BranchName is the Git branch where changes were made or committed.
	BranchName string

	// ValidationStatus indicates whether the changes passed LSP validation.
	ValidationStatus ValidationStatus

	// Diagnostics contains any LSP diagnostics (errors, warnings) from validation.
	Diagnostics []Diagnostic

	// CreatedAt is the timestamp when this changeset was created.
	CreatedAt time.Time

	// CreatedBy identifies the agent that created this changeset.
	CreatedBy string
}

// AppliedPatch represents a single SEARCH/REPLACE edit that was successfully applied.
type AppliedPatch struct {
	// FilePath is the path to the file that was modified (relative to workspace root).
	FilePath string

	// SearchBlock is the original code that was found and replaced.
	SearchBlock string

	// ReplaceBlock is the replacement code that was inserted.
	ReplaceBlock string

	// MatchType indicates how the search block was matched (exact, fuzzy, or failed).
	MatchType MatchType

	// LinesChanged is the number of lines modified by this patch.
	LinesChanged int
}

// MatchType indicates how a SEARCH block was matched in the target file.
type MatchType int

const (
	// MatchExact indicates the search block was found with exact string matching.
	MatchExact MatchType = iota

	// MatchFuzzy indicates the search block was found using fuzzy matching
	// (Levenshtein similarity above the configured threshold).
	MatchFuzzy

	// MatchFailed indicates the search block could not be found in the file.
	MatchFailed
)

// String returns a human-readable representation of the MatchType.
func (m MatchType) String() string {
	switch m {
	case MatchExact:
		return "exact"
	case MatchFuzzy:
		return "fuzzy"
	case MatchFailed:
		return "failed"
	default:
		return "unknown"
	}
}

// ValidationStatus indicates the result of LSP validation for code changes.
type ValidationStatus int

const (
	// ValidationPassed indicates all changes passed validation with no errors or warnings.
	ValidationPassed ValidationStatus = iota

	// ValidationWarnings indicates changes passed but generated non-fatal warnings.
	ValidationWarnings

	// ValidationFailed indicates changes produced errors and should be rolled back.
	ValidationFailed
)

// String returns a human-readable representation of the ValidationStatus.
func (v ValidationStatus) String() string {
	switch v {
	case ValidationPassed:
		return "passed"
	case ValidationWarnings:
		return "warnings"
	case ValidationFailed:
		return "failed"
	default:
		return "unknown"
	}
}

// Diagnostic represents a language server diagnostic (error, warning, or hint).
// These are produced by LSP servers during code validation.
type Diagnostic struct {
	// Path is the file path where the diagnostic was reported.
	Path string

	// Line is the starting line number (1-based).
	Line int

	// Column is the starting column number (1-based).
	Column int

	// EndLine is the ending line number for multi-line diagnostics (1-based).
	EndLine int

	// EndColumn is the ending column number (1-based).
	EndColumn int

	// Severity indicates the diagnostic severity level.
	Severity DiagnosticSeverity

	// Message is the human-readable diagnostic message.
	Message string

	// Source identifies the tool that produced this diagnostic (e.g., "gopls", "pyright").
	Source string
}

// DiagnosticSeverity represents the severity level of a diagnostic.
type DiagnosticSeverity int

const (
	// SeverityError indicates a fatal error that prevents code compilation or execution.
	SeverityError DiagnosticSeverity = iota

	// SeverityWarning indicates a non-fatal issue that should be addressed.
	SeverityWarning

	// SeverityInfo indicates informational feedback that doesn't require action.
	SeverityInfo

	// SeverityHint indicates a style suggestion or optional improvement.
	SeverityHint
)

// String returns a human-readable representation of the DiagnosticSeverity.
func (d DiagnosticSeverity) String() string {
	switch d {
	case SeverityError:
		return "error"
	case SeverityWarning:
		return "warning"
	case SeverityInfo:
		return "info"
	case SeverityHint:
		return "hint"
	default:
		return "unknown"
	}
}

// IsError returns true if the diagnostic is an error severity.
func (d *Diagnostic) IsError() bool {
	return d.Severity == SeverityError
}

// IsWarning returns true if the diagnostic is a warning severity.
func (d *Diagnostic) IsWarning() bool {
	return d.Severity == SeverityWarning
}
