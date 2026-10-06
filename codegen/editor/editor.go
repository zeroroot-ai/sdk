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
}

// EditResult represents the outcome of applying a single Edit.
type EditResult struct {
}

// BatchEditResult represents the outcome of applying multiple edits.
type BatchEditResult struct {
}
