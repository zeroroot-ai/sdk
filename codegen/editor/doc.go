// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Zero Root AI

// Package editor provides intelligent code editing using SEARCH/REPLACE blocks.
//
// The editor implements a line-free editing approach where LLMs provide blocks
// of code to find and replace, without specifying line numbers. This is more
// robust than line-based edits since line numbers become stale as code changes.
//
// # Overview
//
// The editor supports:
//   - Exact string matching for precise edits
//   - Fuzzy matching with configurable similarity threshold
//   - LSP validation to prevent breaking code
//   - Automatic snapshot and rollback on validation failure
//   - Batch operations for atomic multi-file edits
//
// # Key Features
//
// No Line Numbers:
//
//	Traditional diff-based editing requires line numbers:
//	  @@ -42,3 +42,3 @@
//	  -    return x * 2
//	  +    return x * 3
//
//	SEARCH/REPLACE doesn't need line numbers:
//	  SEARCH:
//	  func calculate(x int) int {
//	      return x * 2
//	  }
//
//	  REPLACE:
//	  func calculate(x int) int {
//	      return x * 3
//	  }
//
//	This is more robust because:
//	  - Line numbers change as code is edited
//	  - LLMs don't need to track file state
//	  - Edits are self-documenting (show what changes)
//
// Fuzzy Matching:
//
//	Exact matching can fail due to minor differences:
//	  - Different whitespace (tabs vs spaces)
//	  - Extra/missing blank lines
//	  - Comment variations
//	  - Formatting differences
//
//	Fuzzy matching uses Levenshtein distance to find similar blocks:
//	  - Configurable similarity threshold (default 85%)
//	  - Tolerates whitespace differences
//	  - Reports similarity score
//	  - Falls back gracefully when exact match fails
//
// LSP Validation:
//
//	Before applying edits permanently, the editor:
//	  1. Creates a Git snapshot
//	  2. Applies the SEARCH/REPLACE edit
//	  3. Asks LSP server for diagnostics
//	  4. Checks for errors
//	  5. Commits or rolls back based on validation
//
//	This prevents committing broken code.
//
// # Basic Usage
//
// Apply a Single Edit:
//
//	editor := workspace.Editor()
//
//	edit := editor.Edit{
//	    FilePath: "main.go",
//	    SearchBlock: `func calculate(x int) int {
//	    return x * 2
//	}`,
//	    ReplaceBlock: `func calculate(x int) int {
//	    return x * 3
//	}`,
//	    Description: "Fix calculation multiplier",
//	}
//
//	result, err := editor.Apply(ctx, edit)
//	if err != nil {
//	    return fmt.Errorf("failed to apply edit: %w", err)
//	}
//
//	if !result.Applied {
//	    log.Printf("Edit was not applied: %d errors, %d warnings",
//	        result.ErrorCount(), result.WarningCount())
//	    for _, diag := range result.Diagnostics {
//	        log.Printf("[%s] Line %d: %s",
//	            diag.Severity, diag.Line, diag.Message)
//	    }
//	    return errors.New("edit failed validation")
//	}
//
//	log.Printf("Edit applied successfully")
//	if result.MatchType == codegen.MatchFuzzy {
//	    log.Printf("Used fuzzy match with %.0f%% similarity",
//	        result.FuzzyMatchSimilarity*100)
//	}
//
// Apply Multiple Edits Atomically:
//
//	editor := workspace.Editor()
//
//	edits := []editor.Edit{
//	    {
//	        FilePath:     "pkg/server/handler.go",
//	        SearchBlock:  "old implementation",
//	        ReplaceBlock: "new implementation",
//	        Description:  "Update server handler",
//	    },
//	    {
//	        FilePath:     "pkg/client/client.go",
//	        SearchBlock:  "old client code",
//	        ReplaceBlock: "new client code",
//	        Description:  "Update client",
//	    },
//	}
//
//	result, err := editor.ApplyBatch(ctx, edits)
//	if err != nil {
//	    return err
//	}
//
//	if !result.Applied {
//	    // All edits were rolled back
//	    log.Printf("Batch failed: %d errors", result.ErrorCount())
//	    return errors.New("batch edit failed")
//	}
//
//	// All edits succeeded
//	log.Printf("Applied %d edits successfully", result.SuccessfulEdits())
//
// # Configuration
//
// Fuzzy Matching Threshold:
//
//	editor := workspace.Editor()
//	editor.SetFuzzyThreshold(0.90) // Require 90% similarity
//
//	Default: 0.85 (85% similarity)
//	Range: 0.0 to 1.0
//	  - 1.0 = exact match only
//	  - 0.9 = very strict (minor differences tolerated)
//	  - 0.85 = balanced (default)
//	  - 0.7 = lenient (major differences tolerated)
//	  - <0.6 = too lenient (may match wrong code)
//
// LSP Validation Timeout:
//
//	editor := workspace.Editor()
//	editor.SetValidationTimeout(15 * time.Second)
//
//	Default: 10 seconds
//	Range: 1s to 60s
//	  - Too short: Timeouts on large files
//	  - Too long: Slow edit operations
//	  - 5-10s: Good for most cases
//	  - 15-30s: Large files or slow servers
//
// # Edit Structure
//
// The Edit type represents a single SEARCH/REPLACE operation:
//
//	type Edit struct {
//	    FilePath     string  // Path relative to workspace root
//	    SearchBlock  string  // Code to find
//	    ReplaceBlock string  // Code to insert
//	    Description  string  // Human-readable explanation (optional)
//	}
//
// FilePath:
//
//	Must be relative to workspace root
//	Examples:
//	  - "main.go"
//	  - "pkg/server/handler.go"
//	  - "src/components/Button.tsx"
//
// SearchBlock:
//
//	Must be a contiguous block of lines from the file
//	Whitespace differences are tolerated with fuzzy matching
//	Should be unique enough to identify the correct location
//
//	Good (specific):
//	  func calculate(x int) int {
//	      return x * 2
//	  }
//
//	Bad (too generic, may match multiple locations):
//	  return x * 2
//
// ReplaceBlock:
//
//	Should have similar indentation to the search block
//	Can be shorter or longer than search block
//	Can be empty to delete code
//
// Description:
//
//	Optional human-readable explanation
//	Used for logging and debugging
//	Examples:
//	  - "Fix SQL injection vulnerability"
//	  - "Update API endpoint URL"
//	  - "Refactor error handling"
//
// # Edit Results
//
// EditResult contains the outcome of a single edit:
//
//	type EditResult struct {
//	    Applied              bool               // Was the edit applied?
//	    FilePath             string             // File that was edited
//	    MatchType            codegen.MatchType  // How was the match found?
//	    Diagnostics          []codegen.Diagnostic // LSP diagnostics
//	    Snapshot             string             // Snapshot ID for rollback
//	    FuzzyMatchSimilarity float64            // Similarity score (if fuzzy)
//	    ClosestMatch         *ClosestMatchInfo  // Info about closest match (if failed)
//	}
//
// Check Result Status:
//
//	if result.Applied {
//	    log.Println("Edit succeeded")
//	} else {
//	    log.Println("Edit failed or was rolled back")
//	}
//
// Check Match Type:
//
//	switch result.MatchType {
//	case codegen.MatchExact:
//	    log.Println("Used exact match")
//	case codegen.MatchFuzzy:
//	    log.Printf("Used fuzzy match: %.0f%% similar",
//	        result.FuzzyMatchSimilarity*100)
//	case codegen.MatchFailed:
//	    log.Println("No match found")
//	    if result.ClosestMatch != nil {
//	        log.Printf("Closest match at lines %d-%d (%.0f%% similar)",
//	            result.ClosestMatch.StartLine,
//	            result.ClosestMatch.EndLine,
//	            result.ClosestMatch.Similarity*100)
//	        log.Printf("Content: %s", result.ClosestMatch.Content)
//	    }
//	}
//
// Check Diagnostics:
//
//	if result.HasErrors() {
//	    log.Printf("Edit produced %d errors", result.ErrorCount())
//	    for _, diag := range result.Diagnostics {
//	        if diag.IsError() {
//	            log.Printf("Error at line %d: %s", diag.Line, diag.Message)
//	        }
//	    }
//	}
//
//	if result.HasWarnings() {
//	    log.Printf("Edit produced %d warnings", result.WarningCount())
//	}
//
// # Batch Results
//
// BatchEditResult contains the outcome of multiple edits:
//
//	type BatchEditResult struct {
//	    Applied          bool                     // Were all edits applied?
//	    Results          []EditResult             // Individual results
//	    Snapshot         string                   // Snapshot ID for rollback
//	    ValidationStatus codegen.ValidationStatus // Overall validation result
//	    Diagnostics      []codegen.Diagnostic     // All diagnostics
//	}
//
// Check Batch Status:
//
//	if result.Applied {
//	    log.Printf("Applied %d edits successfully", result.SuccessfulEdits())
//	} else {
//	    log.Printf("Batch failed: %d succeeded, %d failed",
//	        result.SuccessfulEdits(), result.FailedEdits())
//	}
//
// Iterate Individual Results:
//
//	for i, editResult := range result.Results {
//	    if editResult.Applied {
//	        log.Printf("Edit %d: SUCCESS", i+1)
//	    } else {
//	        log.Printf("Edit %d: FAILED - %d errors",
//	            i+1, editResult.ErrorCount())
//	    }
//	}
//
// # Fuzzy Matching
//
// The editor uses Levenshtein distance for fuzzy matching:
//
// Algorithm:
//
//  1. Normalize search block and file content:
//     - Convert line endings to \n
//     - Normalize whitespace (but preserve structure)
//
//  2. Try exact match first:
//     - Fast string.Contains check
//     - If found, return immediately
//
//  3. Try fuzzy match:
//     - Split file into blocks of same line count as search block
//     - Calculate Levenshtein distance for each block
//     - Find block with highest similarity
//     - Return if similarity >= threshold
//
//  4. Return closest match info for debugging
//
// Similarity Calculation:
//
//	similarity = 1 - (levenshtein_distance / max_length)
//
//	Examples:
//	  - Identical strings: similarity = 1.0 (100%)
//	  - One character different: similarity ~0.99
//	  - Whitespace differences: similarity ~0.95
//	  - Major differences: similarity <0.80
//
// Performance:
//
//   - Exact match: O(n) where n = file size
//   - Fuzzy match: O(n * m) where m = search block size
//   - Typical file: <100ms for fuzzy matching
//   - Large file (>5000 lines): May take 1-2 seconds
//
// # LSP Validation
//
// When LSP is enabled, the editor validates changes:
//
// Validation Mission:
//
//  1. Create Git snapshot
//  2. Apply SEARCH/REPLACE edit
//  3. Wait for LSP diagnostics (with timeout)
//  4. Check for errors:
//     - If errors: Rollback to snapshot, return failure
//     - If warnings: Keep changes, return success with warnings
//     - If clean: Keep changes, return success
//
// Diagnostic Filtering:
//
//	The editor only checks files that were actually modified:
//
//	// Edit main.go
//	result, _ := editor.Apply(ctx, edit)
//
//	// Diagnostics only include errors from main.go
//	// Ignores pre-existing errors in other files
//
// Timeout Handling:
//
//	If LSP validation times out:
//	  - Changes are NOT rolled back
//	  - Result.Applied = true
//	  - Warning logged
//	  - Timeout error returned
//
//	Rationale:
//	  - Timeout may be due to server load, not actual errors
//	  - Better to allow changes than block agent progress
//	  - Agent can check result and decide to rollback manually
//
// # Error Handling
//
// Common errors and solutions:
//
// Search Block Not Found (ErrSearchNotFound):
//
//	Causes:
//	  - Code has already been modified
//	  - Search block is not unique
//	  - Whitespace differences
//	  - File was changed by another process
//
//	Solutions:
//	  - Read current file content first
//	  - Use more specific search blocks
//	  - Lower fuzzy threshold (carefully)
//	  - Check ClosestMatch info for hints
//
// Validation Failed (ErrValidationFailed):
//
//	Causes:
//	  - Edit introduced syntax errors
//	  - Edit broke type checking
//	  - Import statements missing
//	  - Logic errors
//
//	Solutions:
//	  - Review Diagnostics in result
//	  - Fix the ReplaceBlock
//	  - Ensure imports are correct
//	  - Test changes locally first
//
// LSP Timeout (ErrLSPTimeout):
//
//	Causes:
//	  - Language server is slow
//	  - Large file or complex analysis
//	  - Server is overloaded
//
//	Solutions:
//	  - Increase validation timeout
//	  - Split large edits into smaller ones
//	  - Restart LSP server
//	  - Disable LSP validation if not critical
//
// # Best Practices
//
// Writing Search Blocks:
//
//	Good:
//	  - Include enough context to be unique
//	  - Use complete statements or blocks
//	  - Include function signatures
//	  - Match file's indentation style
//
//	Bad:
//	  - Single lines (may match multiple places)
//	  - Partial statements
//	  - Too much code (harder to match)
//	  - Mixed indentation
//
// Writing Replace Blocks:
//
//	Good:
//	  - Preserve indentation from search block
//	  - Keep similar structure
//	  - Include necessary imports
//	  - Use clear, readable code
//
//	Bad:
//	  - Change indentation drastically
//	  - Mix formatting styles
//	  - Forget to update related code
//	  - Leave debug statements
//
// Batch Operations:
//
//	Use ApplyBatch() when:
//	  - Multiple files need coordinated changes
//	  - Changes depend on each other
//	  - Want atomic all-or-nothing behavior
//
//	Use individual Apply() when:
//	  - Changes are independent
//	  - Want to continue on partial failure
//	  - Need custom error handling per edit
//
// Error Recovery:
//
//	// Save snapshot ID for manual rollback
//	result, err := editor.Apply(ctx, edit)
//	snapshotID := result.Snapshot
//
//	// Later, if something goes wrong:
//	if needsRollback {
//	    git := workspace.Git()
//	    if err := git.Rollback(ctx, snapshotID); err != nil {
//	        log.Printf("Rollback failed: %v", err)
//	    }
//	}
//
// # Performance Optimization
//
// Minimize File Reads:
//
//	// Bad: Read file multiple times
//	for _, edit := range edits {
//	    editor.Apply(ctx, edit)
//	}
//
//	// Good: Use batch operation
//	editor.ApplyBatch(ctx, edits)
//
// Use Specific Search Blocks:
//
//	// Bad: Generic, requires full file scan
//	SearchBlock: "return x"
//
//	// Good: Specific, faster to find
//	SearchBlock: `func calculate(x int) int {
//	    return x * 2
//	}`
//
// Adjust Fuzzy Threshold:
//
//	// If you know exact match will work:
//	editor.SetFuzzyThreshold(1.0) // Disable fuzzy matching
//
//	// If you need fuzzy matching:
//	editor.SetFuzzyThreshold(0.85) // Keep default
//
// # Thread Safety
//
// The editor is safe for concurrent use with different files,
// but edits to the same file should be serialized:
//
//	// Safe: Editing different files
//	go editor.Apply(ctx, edit1) // edit1.FilePath = "file1.go"
//	go editor.Apply(ctx, edit2) // edit2.FilePath = "file2.go"
//
//	// Unsafe: Editing same file
//	go editor.Apply(ctx, edit1) // edit1.FilePath = "main.go"
//	go editor.Apply(ctx, edit2) // edit2.FilePath = "main.go"
//
//	// Safe: Use batch operation
//	editor.ApplyBatch(ctx, []editor.Edit{edit1, edit2})
//
// # Examples
//
// See example_test.go for detailed examples:
//   - ExampleSearchReplace_exactMatch: Exact string matching
//   - ExampleFuzzyMatcher_minorDifferences: Fuzzy matching
//   - ExampleFuzzyMatcher_whitespaceTolerance: Whitespace handling
//   - ExampleSearchReplace_lineEndings: Line ending normalization
//   - ExampleFuzzyMatcher_withThreshold: Threshold configuration
//
// See integration_test.go for complete missions:
//   - TestFullMission: Complete edit, validate, commit mission
//   - TestGoCodeWithLSP: Go code editing with gopls
//   - TestPythonCodeWithLSP: Python code editing with pyright
//   - TestTypeScriptCodeWithLSP: TypeScript editing with tsserver
//   - TestLSPValidationIntegration: LSP validation and rollback
package editor
