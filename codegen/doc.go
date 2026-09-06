// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Zero Root AI

// Package codegen provides SDK primitives for intelligent code generation,
// modification, and validation within the Gibson agent framework.
//
// The codegen SDK enables agents to:
//   - Clone and work with Git repositories as mission-scoped workspaces
//   - Generate and apply code changes with language-server validation
//   - Create branches and commits with proper attribution
//   - Rollback changes when validation fails
//   - Support multi-repository missions
//
// # Architecture
//
// The codegen SDK is organized into several sub-packages:
//
//   - codegen/workspace: Manages Git repository clones and provides isolated working directories
//   - codegen/editor: Applies code changes using SEARCH/REPLACE blocks with LSP validation
//   - codegen/git: Provides Git operations including clone, branch, commit, push, and snapshot/rollback
//   - codegen/lsp: Manages language server processes and provides code validation
//
// # Core Types
//
// The main types exported by this package are:
//
//   - CodeChangeSet: Represents a collection of code changes applied during a mission
//   - AppliedPatch: Represents a single SEARCH/REPLACE edit that was applied
//   - Diagnostic: Represents a language server diagnostic (error, warning, or hint)
//   - ValidationStatus: Indicates the result of LSP validation
//   - DiagnosticSeverity: Represents the severity level of a diagnostic
//
// # Error Types
//
// The package defines several sentinel errors:
//
//   - ErrCloneFailed: Repository clone failed
//   - ErrCredentialMissing: Required credential not found
//   - ErrSearchNotFound: SEARCH block not found in file
//   - ErrValidationFailed: LSP validation returned errors
//   - ErrPushConflict: Remote has diverged, pull required
//   - ErrWorkspaceNotReady: Workspace not initialized
//   - ErrLSPTimeout: LSP validation timed out
//
// # Quick Start
//
// Agents typically interact with the codegen SDK through the harness interface:
//
//	// Get the primary workspace (single-repo missions)
//	ws := harness.Workspace()
//
//	// Or get a specific workspace by name (multi-repo missions)
//	ws, ok := harness.GetWorkspace("frontend")
//	if !ok {
//	    return errors.New("frontend workspace not found")
//	}
//
//	// Apply a code change
//	editor := ws.Editor()
//	edit := editor.Edit{
//	    FilePath: "main.go",
//	    SearchBlock: "func main() {\n\tfmt.Println(\"Hello\")\n}",
//	    ReplaceBlock: "func main() {\n\tfmt.Println(\"Hello, World!\")\n}",
//	    Description: "Update greeting message",
//	}
//	result, err := editor.Apply(ctx, edit)
//	if err != nil {
//	    return fmt.Errorf("failed to apply edit: %w", err)
//	}
//
//	if !result.Applied {
//	    log.Printf("Edit was not applied: %d errors, %d warnings",
//	        result.ErrorCount(), result.WarningCount())
//	    return errors.New("edit failed validation")
//	}
//
//	// Commit the changes
//	git := ws.Git()
//	if err := git.Add(ctx, "main.go"); err != nil {
//	    return fmt.Errorf("failed to stage changes: %w", err)
//	}
//
//	commitSHA, err := git.Commit(ctx, "Update greeting message", git.CommitOptions{
//	    Author: "Agent <agent@example.com>",
//	})
//	if err != nil {
//	    return fmt.Errorf("failed to commit: %w", err)
//	}
//
//	log.Printf("Changes committed: %s", commitSHA)
//
// # Common Usage Patterns
//
// Apply Multiple Edits:
//
//	editor := ws.Editor()
//	edits := []editor.Edit{
//	    {FilePath: "file1.go", SearchBlock: "...", ReplaceBlock: "..."},
//	    {FilePath: "file2.go", SearchBlock: "...", ReplaceBlock: "..."},
//	}
//	result, err := editor.ApplyBatch(ctx, edits)
//	if err != nil {
//	    return err
//	}
//	// All edits are applied atomically or rolled back together
//
// Handle Fuzzy Matching:
//
//	editor := ws.Editor()
//	editor.SetFuzzyThreshold(0.85) // 85% similarity required
//	result, err := editor.Apply(ctx, edit)
//	if result.MatchType == codegen.MatchFuzzy {
//	    log.Printf("Used fuzzy match with %.0f%% similarity",
//	        result.FuzzyMatchSimilarity*100)
//	}
//
// Create Branch and Push:
//
//	git := ws.Git()
//	branchName := "fix/vulnerability-123"
//	if err := git.CreateBranch(ctx, branchName); err != nil {
//	    return err
//	}
//	if err := git.Checkout(ctx, branchName); err != nil {
//	    return err
//	}
//
//	// Make changes and commit...
//
//	err = git.Push(ctx, git.PushOptions{
//	    SetUpstream: true, // Set tracking branch
//	})
//	if err != nil {
//	    return err
//	}
//
// Snapshot and Rollback:
//
//	git := ws.Git()
//
//	// Create a snapshot before risky changes
//	snapshotID, err := git.Snapshot(ctx)
//	if err != nil {
//	    return err
//	}
//
//	// Make experimental changes...
//	result, err := editor.Apply(ctx, riskyEdit)
//
//	// Rollback if something went wrong
//	if err != nil || !result.Applied {
//	    if err := git.Rollback(ctx, snapshotID); err != nil {
//	        log.Printf("Rollback failed: %v", err)
//	    }
//	}
//
// # Design Principles
//
//   - No line numbers: SEARCH/REPLACE uses exact string matching (with fuzzy fallback)
//   - Validation first: LSP validation prevents committing broken code
//   - Automatic rollback: Failed validation triggers automatic snapshot restore
//   - Credential security: Credentials are managed through the harness, never exposed
//   - Observability: All operations are traced via OpenTelemetry
//   - Atomic operations: Batch edits succeed or fail together
//
// # Multi-Repository Support
//
// For missions that work with multiple repositories:
//
//	// Access all workspaces
//	workspaces := harness.AllWorkspaces()
//	for name, ws := range workspaces {
//	    log.Printf("Workspace: %s at %s", name, ws.Path())
//	}
//
//	// Work with specific repositories
//	frontend, _ := harness.GetWorkspace("frontend")
//	backend, _ := harness.GetWorkspace("backend")
//
//	// Apply coordinated changes
//	frontendEdit := editor.Edit{...}
//	backendEdit := editor.Edit{...}
//
//	if _, err := frontend.Editor().Apply(ctx, frontendEdit); err != nil {
//	    return err
//	}
//	if _, err := backend.Editor().Apply(ctx, backendEdit); err != nil {
//	    return err
//	}
//
// # LSP Validation
//
// Language server validation is automatically enabled when configured:
//
//	// In mission YAML:
//	// workspace:
//	//   settings:
//	//     lsp_enabled: true
//	//     lsp_timeout: 10s
//
//	// Edits are automatically validated:
//	result, err := editor.Apply(ctx, edit)
//	if result.HasErrors() {
//	    for _, diag := range result.Diagnostics {
//	        log.Printf("[%s] Line %d: %s",
//	            diag.Severity, diag.Line, diag.Message)
//	    }
//	}
//
// Supported languages: Go (gopls), Python (pyright), TypeScript/JavaScript (tsserver)
//
// # Performance Considerations
//
//   - Shallow clones: Use shallow: true in repo config for large repositories
//   - Worktrees: Enable use_worktrees for agent isolation in concurrent missions
//   - LSP caching: Language servers cache diagnostics for faster validation
//   - Batch edits: Use ApplyBatch() for multiple changes to reduce validation overhead
//
// # Configuration
//
// Workspaces are configured in the mission YAML:
//
//	workspace:
//	  repositories:
//	    - name: frontend
//	      url: https://github.com/org/frontend.git
//	      branch: main
//	      credential: github-token
//	      shallow: false
//
//	    - name: backend
//	      url: https://github.com/org/backend.git
//	      branch: main
//	      credential: github-token
//	      depends_on:
//	        - frontend
//
//	  settings:
//	    cleanup_on_complete: true
//	    use_worktrees: false
//	    lsp_enabled: true
//	    lsp_timeout: 10s
//	    base_directory: /tmp/gibson-workspaces
//
// # Additional Resources
//
// For detailed documentation on each sub-package:
//   - workspace/doc.go - Workspace management and configuration
//   - editor/doc.go - SEARCH/REPLACE operations and fuzzy matching
//   - git/doc.go - Git operations and snapshot/rollback
//   - lsp/doc.go - Language server integration
//
// For design documentation and specifications:
//   - .spec-mission/specs/codegen-sdk/ - Architecture and design docs
//
// For more examples:
//   - codegen/integration_test.go - Complete mission examples
//   - editor/example_test.go - Editor usage examples
//   - lsp/example_test.go - LSP validation examples
package codegen
