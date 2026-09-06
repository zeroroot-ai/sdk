// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Zero Root AI

package editor

import (
	"context"
	"time"

	"github.com/zeroroot-ai/sdk/codegen"
)

// Editor provides intelligent code editing capabilities using SEARCH/REPLACE blocks.
// It applies code changes with automatic snapshot/rollback and optional LSP validation.
//
// The Editor uses a line-free SEARCH/REPLACE approach where LLMs provide blocks of
// code to find and replace, without specifying line numbers. This is more robust than
// line-based edits since line numbers can become stale as code changes.
//
// Example usage:
//
//	edit := Edit{
//	    FilePath:     "main.go",
//	    SearchBlock:  "func main() {\n\tprintln(\"hello\")\n}",
//	    ReplaceBlock: "func main() {\n\tfmt.Println(\"hello world\")\n}",
//	    Description:  "Update to use fmt.Println",
//	}
//	result, err := editor.Apply(ctx, edit)
type Editor interface {
	// Apply applies a single code edit to a file.
	// It creates a Git snapshot before the edit, applies the SEARCH/REPLACE,
	// and validates the result with LSP if configured.
	//
	// If the search block is not found exactly, fuzzy matching is attempted
	// using the configured threshold. If validation fails, the edit is
	// automatically rolled back to the snapshot.
	//
	// Returns an EditResult indicating success/failure, match type, and any
	// diagnostics from LSP validation.
	Apply(ctx context.Context, edit Edit) (*EditResult, error)

	// ApplyBatch applies multiple edits in sequence as a single transaction.
	// All edits are applied to the same snapshot. If any edit fails validation,
	// the entire batch is rolled back.
	//
	// This is more efficient than calling Apply() multiple times since only
	// one snapshot and one LSP validation pass are needed for the entire batch.
	ApplyBatch(ctx context.Context, edits []Edit) (*BatchEditResult, error)

	// Validate checks a file for LSP diagnostics without making any changes.
	// This is useful for checking if a file has errors before attempting edits.
	//
	// Returns diagnostics (errors, warnings, hints) from the language server.
	// If LSP is not available or times out, returns an empty slice and an error.
	Validate(ctx context.Context, path string) ([]codegen.Diagnostic, error)

	// SetFuzzyThreshold configures the similarity threshold for fuzzy matching.
	// The threshold is a value between 0.0 and 1.0, where 1.0 requires exact
	// matches and lower values allow more tolerance for whitespace and minor
	// differences.
	//
	// Default: 0.85 (85% similarity required)
	SetFuzzyThreshold(threshold float64)

	// SetValidationTimeout configures the maximum time to wait for LSP validation.
	// If validation takes longer than this timeout, the edit is applied with a
	// warning but is not rolled back.
	//
	// Default: 10 seconds
	SetValidationTimeout(timeout time.Duration)
}

// Edit represents a single SEARCH/REPLACE code modification.
// This is the line-free edit format where the LLM provides blocks of code
// to find and replace, without specifying line numbers.
type Edit struct {
	// FilePath is the path to the file to edit, relative to workspace root.
	// Example: "cmd/server/main.go"
	FilePath string

	// SearchBlock is the original code to find in the file.
	// Must be a contiguous block of lines. Whitespace differences are tolerated
	// with fuzzy matching, but the overall structure should match.
	//
	// Example:
	//   func calculate(x int) int {
	//       return x * 2
	//   }
	SearchBlock string

	// ReplaceBlock is the replacement code to insert in place of SearchBlock.
	// Should have the same or similar indentation level as the search block.
	//
	// Example:
	//   func calculate(x int) int {
	//       return x * 3
	//   }
	ReplaceBlock string

	// Description is an optional human-readable explanation of why this change
	// is being made. Used for logging and debugging.
	//
	// Example: "Fix calculation bug by changing multiplier from 2 to 3"
	Description string
}

// EditResult represents the outcome of applying a single Edit.
type EditResult struct {
	// Applied indicates whether the edit was successfully applied.
	// False means either the search block wasn't found or validation failed.
	Applied bool

	// FilePath is the file that was edited (copied from Edit.FilePath).
	FilePath string

	// MatchType indicates how the search block was matched.
	// One of: MatchExact, MatchFuzzy, or MatchFailed.
	MatchType codegen.MatchType

	// Diagnostics contains any LSP diagnostics (errors, warnings, hints)
	// produced after applying the edit. Empty if LSP validation was not
	// performed or if no issues were found.
	Diagnostics []codegen.Diagnostic

	// Snapshot is the Git snapshot ID created before this edit.
	// Can be used to manually rollback if needed via GitOps.Rollback().
	Snapshot string

	// FuzzyMatchSimilarity is the similarity score if fuzzy matching was used.
	// Value between 0.0 and 1.0. Only set when MatchType is MatchFuzzy.
	FuzzyMatchSimilarity float64

	// ClosestMatch contains information about the closest match found when
	// the search block could not be matched (MatchFailed).
	// Useful for debugging why a search block wasn't found.
	ClosestMatch *ClosestMatchInfo
}

// BatchEditResult represents the outcome of applying multiple edits.
type BatchEditResult struct {
	// Applied indicates whether all edits were successfully applied.
	// False means at least one edit failed or validation failed.
	Applied bool

	// Results contains the individual EditResult for each edit in the batch.
	// The order matches the order of the input edits.
	Results []EditResult

	// Snapshot is the Git snapshot ID created before the batch.
	// If Applied is false, all changes were rolled back to this snapshot.
	Snapshot string

	// ValidationStatus indicates the overall validation result for the batch.
	// One of: ValidationPassed, ValidationWarnings, or ValidationFailed.
	ValidationStatus codegen.ValidationStatus

	// Diagnostics contains all LSP diagnostics across all edited files.
	// Aggregated from individual EditResults.
	Diagnostics []codegen.Diagnostic
}

// ClosestMatchInfo provides debugging information when a search block cannot be found.
// It shows the closest match that was found and its similarity score.
type ClosestMatchInfo struct {
	// StartLine is the starting line number of the closest match (1-based).
	StartLine int

	// EndLine is the ending line number of the closest match (1-based).
	EndLine int

	// Similarity is the similarity score between 0.0 and 1.0.
	Similarity float64

	// Content is the actual content of the closest match.
	// Limited to 500 characters to avoid bloating error messages.
	Content string
}

// HasErrors returns true if any diagnostic in the EditResult is an error.
func (r *EditResult) HasErrors() bool {
	for _, d := range r.Diagnostics {
		if d.IsError() {
			return true
		}
	}
	return false
}

// HasWarnings returns true if any diagnostic in the EditResult is a warning.
func (r *EditResult) HasWarnings() bool {
	for _, d := range r.Diagnostics {
		if d.IsWarning() {
			return true
		}
	}
	return false
}

// ErrorCount returns the number of error-level diagnostics.
func (r *EditResult) ErrorCount() int {
	count := 0
	for _, d := range r.Diagnostics {
		if d.IsError() {
			count++
		}
	}
	return count
}

// WarningCount returns the number of warning-level diagnostics.
func (r *EditResult) WarningCount() int {
	count := 0
	for _, d := range r.Diagnostics {
		if d.IsWarning() {
			count++
		}
	}
	return count
}

// HasErrors returns true if any edit result in the batch has errors.
func (b *BatchEditResult) HasErrors() bool {
	for _, r := range b.Results {
		if r.HasErrors() {
			return true
		}
	}
	return false
}

// HasWarnings returns true if any edit result in the batch has warnings.
func (b *BatchEditResult) HasWarnings() bool {
	for _, r := range b.Results {
		if r.HasWarnings() {
			return true
		}
	}
	return false
}

// ErrorCount returns the total number of error-level diagnostics across all edits.
func (b *BatchEditResult) ErrorCount() int {
	count := 0
	for _, d := range b.Diagnostics {
		if d.IsError() {
			count++
		}
	}
	return count
}

// WarningCount returns the total number of warning-level diagnostics across all edits.
func (b *BatchEditResult) WarningCount() int {
	count := 0
	for _, d := range b.Diagnostics {
		if d.IsWarning() {
			count++
		}
	}
	return count
}

// SuccessfulEdits returns the count of edits that were successfully applied.
func (b *BatchEditResult) SuccessfulEdits() int {
	count := 0
	for _, r := range b.Results {
		if r.Applied {
			count++
		}
	}
	return count
}

// FailedEdits returns the count of edits that failed to apply.
func (b *BatchEditResult) FailedEdits() int {
	return len(b.Results) - b.SuccessfulEdits()
}
