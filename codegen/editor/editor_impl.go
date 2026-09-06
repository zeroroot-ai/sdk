// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Zero Root AI

package editor

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"time"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"

	"github.com/zeroroot-ai/sdk/codegen"
	"github.com/zeroroot-ai/sdk/codegen/git"
	"github.com/zeroroot-ai/sdk/codegen/lsp"
)

// DefaultValidationTimeout is the default maximum time to wait for LSP validation.
const DefaultValidationTimeout = 10 * time.Second

// EditorImpl implements the Editor interface with LSP integration and GitOps support.
// It applies SEARCH/REPLACE edits with automatic snapshot/rollback on validation failures.
type EditorImpl struct {
	// workspaceRoot is the absolute path to the workspace directory
	workspaceRoot string

	// git provides Git operations for snapshot/rollback
	git git.GitOps

	// lsp provides language server validation (optional)
	lsp lsp.LSPManager

	// fuzzyThreshold is the minimum similarity score for fuzzy matching (0.0 to 1.0)
	fuzzyThreshold float64

	// validationTimeout is the maximum time to wait for LSP validation
	validationTimeout time.Duration

	// logger for structured logging
	logger *slog.Logger

	// metrics for observability
	metrics *codegen.CodegenMetrics
}

// NewEditor creates a new EditorImpl with GitOps and optional LSP integration.
//
// Parameters:
//   - workspaceRoot: Absolute path to the workspace directory
//   - gitOps: Git operations provider for snapshot/rollback
//   - lspManager: Language server manager for validation (can be nil to disable LSP)
//
// Returns a configured Editor with default settings:
//   - Fuzzy threshold: 0.85 (85% similarity required)
//   - Validation timeout: 10 seconds
func NewEditor(workspaceRoot string, gitOps git.GitOps, lspManager lsp.LSPManager) *EditorImpl {
	logger := slog.Default()

	// Initialize metrics (non-fatal if it fails)
	metrics, err := codegen.NewCodegenMetrics()
	if err != nil {
		logger.Warn("failed to initialize codegen metrics", "error", err)
		metrics = codegen.NoopCodegenMetrics()
	}

	return &EditorImpl{
		workspaceRoot:     workspaceRoot,
		git:               gitOps,
		lsp:               lspManager,
		fuzzyThreshold:    DefaultFuzzyThreshold,
		validationTimeout: DefaultValidationTimeout,
		logger:            logger,
		metrics:           metrics,
	}
}

// Apply applies a single code edit to a file.
//
// Process:
//  1. Create Git snapshot before any changes
//  2. Read file content
//  3. Attempt exact SEARCH/REPLACE match
//  4. Fall back to fuzzy matching if exact match fails
//  5. Write modified content to file
//  6. Validate with LSP if available
//  7. Rollback on validation errors (not warnings)
//
// Returns an EditResult indicating success/failure and diagnostics.
func (e *EditorImpl) Apply(ctx context.Context, edit Edit) (*EditResult, error) {
	startTime := time.Now()
	ctx, span := codegen.StartSpan(ctx, "codegen.editor.apply",
		attribute.String("file.path", codegen.SanitizeFilePath(edit.FilePath, e.workspaceRoot)))
	defer span.End()

	e.logger.Info("applying edit",
		"file", edit.FilePath,
		"description", edit.Description)

	// Validate input
	if edit.FilePath == "" {
		span.SetStatus(codes.Error, "empty file path")
		e.metrics.RecordError(ctx, e.metrics.EditorEditsTotal, "apply")
		return nil, errors.New("edit file path cannot be empty")
	}
	if edit.SearchBlock == "" {
		span.SetStatus(codes.Error, "empty search block")
		e.metrics.RecordError(ctx, e.metrics.EditorEditsTotal, "apply")
		return nil, errors.New("edit search block cannot be empty")
	}

	// Create snapshot before any changes
	snapshotID, err := e.git.Snapshot(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to create snapshot: %w", err)
	}

	e.logger.Debug("created snapshot", "snapshot_id", snapshotID)

	result := &EditResult{
		FilePath: edit.FilePath,
		Snapshot: snapshotID,
	}

	// Attempt to apply the edit
	if err := e.applyEdit(ctx, edit, result); err != nil {
		// Clean up snapshot on error
		if rollbackErr := e.git.Rollback(ctx, snapshotID); rollbackErr != nil {
			e.logger.Error("failed to rollback after error",
				"error", rollbackErr,
				"original_error", err)
		}
		return nil, err
	}

	// If edit was not applied successfully, no need to validate
	if !result.Applied {
		duration := time.Since(startTime).Seconds()
		e.logger.Warn("edit not applied",
			"file", edit.FilePath,
			"match_type", result.MatchType.String())
		span.SetStatus(codes.Error, "edit not applied")
		span.SetAttributes(attribute.String("match_type", codegen.MatchTypeToString(result.MatchType)))
		e.metrics.RecordError(ctx, e.metrics.EditorEditsTotal, "apply",
			attribute.String(codegen.MetricAttrMatchType, codegen.MatchTypeToString(result.MatchType)))
		e.metrics.RecordDuration(ctx, e.metrics.EditorEditDurationSeconds, duration,
			attribute.String(codegen.MetricAttrMatchType, codegen.MatchTypeToString(result.MatchType)))
		return result, nil
	}

	// Validate with LSP if available
	if e.lsp != nil {
		if err := e.validateEdit(ctx, edit.FilePath, result); err != nil {
			e.logger.Error("validation failed", "error", err)
			// Note: validation timeout is logged but doesn't fail the edit
		}

		// Rollback if validation produced errors (not warnings)
		if result.HasErrors() {
			e.logger.Warn("rolling back due to validation errors",
				"file", edit.FilePath,
				"error_count", result.ErrorCount())

			if err := e.git.Rollback(ctx, snapshotID); err != nil {
				return nil, fmt.Errorf("failed to rollback after validation errors: %w", err)
			}

			result.Applied = false

			duration := time.Since(startTime).Seconds()
			span.SetStatus(codes.Error, "validation failed")
			e.metrics.RecordError(ctx, e.metrics.EditorEditsTotal, "apply",
				attribute.String(codegen.MetricAttrMatchType, codegen.MatchTypeToString(result.MatchType)),
				attribute.String(codegen.MetricAttrValidationStatus, "failed"))
			e.metrics.RecordDuration(ctx, e.metrics.EditorEditDurationSeconds, duration)
			return result, nil
		}
	}

	duration := time.Since(startTime).Seconds()
	e.logger.Info("edit applied successfully",
		"file", edit.FilePath,
		"match_type", result.MatchType.String(),
		"has_warnings", result.HasWarnings())

	span.SetStatus(codes.Ok, "edit applied")
	span.SetAttributes(
		attribute.String("match_type", codegen.MatchTypeToString(result.MatchType)),
		attribute.Int("diagnostic_count", len(result.Diagnostics)),
		attribute.Int("error_count", result.ErrorCount()),
		attribute.Int("warning_count", result.WarningCount()))

	validationStatus := "passed"
	if result.HasWarnings() {
		validationStatus = "warnings"
	}

	e.metrics.RecordSuccess(ctx, e.metrics.EditorEditsTotal, "apply",
		attribute.String(codegen.MetricAttrMatchType, codegen.MatchTypeToString(result.MatchType)),
		attribute.String(codegen.MetricAttrValidationStatus, validationStatus))
	e.metrics.RecordDuration(ctx, e.metrics.EditorEditDurationSeconds, duration,
		attribute.String(codegen.MetricAttrMatchType, codegen.MatchTypeToString(result.MatchType)))

	return result, nil
}

// ApplyBatch applies multiple edits in sequence as a single transaction.
// All edits are applied to the same snapshot. If any edit fails validation,
// the entire batch is rolled back.
func (e *EditorImpl) ApplyBatch(ctx context.Context, edits []Edit) (*BatchEditResult, error) {
	startTime := time.Now()
	ctx, span := codegen.StartSpan(ctx, "codegen.editor.apply_batch",
		attribute.Int("edit_count", len(edits)))
	defer span.End()

	e.logger.Info("applying batch edits", "count", len(edits))

	// Create snapshot before any changes
	snapshotID, err := e.git.Snapshot(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to create snapshot: %w", err)
	}

	e.logger.Debug("created batch snapshot", "snapshot_id", snapshotID)

	result := &BatchEditResult{
		Snapshot: snapshotID,
		Applied:  true,
		Results:  make([]EditResult, 0, len(edits)),
	}

	// Track all modified files for batch validation
	modifiedFiles := make(map[string]bool)

	// Apply each edit
	for i, edit := range edits {
		e.logger.Debug("applying batch edit",
			"index", i+1,
			"total", len(edits),
			"file", edit.FilePath)

		editResult := EditResult{
			FilePath: edit.FilePath,
			Snapshot: snapshotID,
		}

		// Attempt to apply the edit
		if err := e.applyEdit(ctx, edit, &editResult); err != nil {
			// Rollback entire batch on error
			if rollbackErr := e.git.Rollback(ctx, snapshotID); rollbackErr != nil {
				e.logger.Error("failed to rollback batch after error",
					"error", rollbackErr,
					"original_error", err)
			}
			result.Applied = false
			return result, fmt.Errorf("failed to apply edit %d/%d: %w", i+1, len(edits), err)
		}

		result.Results = append(result.Results, editResult)

		// If any edit fails to apply, mark batch as failed but continue
		// to collect all results for debugging
		if !editResult.Applied {
			result.Applied = false
			e.logger.Warn("batch edit not applied",
				"index", i+1,
				"file", edit.FilePath,
				"match_type", editResult.MatchType.String())
		} else {
			modifiedFiles[edit.FilePath] = true
		}
	}

	// If no edits were actually applied, no need to validate
	if !result.Applied {
		e.logger.Warn("batch contained no successfully applied edits")
		return result, nil
	}

	// Validate all modified files with LSP if available
	if e.lsp != nil {
		e.logger.Debug("validating batch", "file_count", len(modifiedFiles))

		// Collect diagnostics from all modified files
		allDiagnostics := []codegen.Diagnostic{}
		hasErrors := false

		for filePath := range modifiedFiles {
			diagnostics, err := e.validateFile(ctx, filePath)
			if err != nil {
				e.logger.Warn("validation error for file",
					"file", filePath,
					"error", err)
				// Continue validation for other files
				continue
			}

			allDiagnostics = append(allDiagnostics, diagnostics...)

			// Check for errors
			for _, d := range diagnostics {
				if d.IsError() {
					hasErrors = true
					break
				}
			}
		}

		result.Diagnostics = allDiagnostics

		// Determine validation status
		if hasErrors {
			result.ValidationStatus = codegen.ValidationFailed
		} else if len(allDiagnostics) > 0 {
			result.ValidationStatus = codegen.ValidationWarnings
		} else {
			result.ValidationStatus = codegen.ValidationPassed
		}

		// Rollback entire batch if validation produced errors
		if hasErrors {
			e.logger.Warn("rolling back batch due to validation errors",
				"error_count", result.ErrorCount())

			if err := e.git.Rollback(ctx, snapshotID); err != nil {
				return nil, fmt.Errorf("failed to rollback batch after validation errors: %w", err)
			}

			result.Applied = false

			duration := time.Since(startTime).Seconds()
			span.SetStatus(codes.Error, "batch validation failed")
			e.metrics.RecordError(ctx, e.metrics.EditorEditsTotal, "apply_batch",
				attribute.String(codegen.MetricAttrValidationStatus, "failed"))
			e.metrics.RecordDuration(ctx, e.metrics.EditorEditDurationSeconds, duration)
			return result, nil
		}
	} else {
		result.ValidationStatus = codegen.ValidationPassed
	}

	duration := time.Since(startTime).Seconds()
	e.logger.Info("batch edits applied successfully",
		"total", len(edits),
		"successful", result.SuccessfulEdits(),
		"failed", result.FailedEdits(),
		"validation", result.ValidationStatus.String())

	span.SetStatus(codes.Ok, "batch edits applied")
	span.SetAttributes(
		attribute.Int("successful_edits", result.SuccessfulEdits()),
		attribute.Int("failed_edits", result.FailedEdits()),
		attribute.String("validation_status", codegen.ValidationStatusToString(result.ValidationStatus)),
		attribute.Int("diagnostic_count", len(result.Diagnostics)))

	e.metrics.RecordSuccess(ctx, e.metrics.EditorEditsTotal, "apply_batch",
		attribute.String(codegen.MetricAttrValidationStatus, codegen.ValidationStatusToString(result.ValidationStatus)))
	e.metrics.RecordDuration(ctx, e.metrics.EditorEditDurationSeconds, duration)

	return result, nil
}

// Validate checks a file for LSP diagnostics without making any changes.
func (e *EditorImpl) Validate(ctx context.Context, path string) ([]codegen.Diagnostic, error) {
	ctx, span := codegen.StartSpan(ctx, "codegen.editor.validate",
		attribute.String("file.path", codegen.SanitizeFilePath(path, e.workspaceRoot)))
	defer span.End()

	if e.lsp == nil {
		span.SetStatus(codes.Error, "LSP not available")
		return nil, errors.New("LSP validation not available (LSP manager is nil)")
	}

	// Create timeout context for validation
	validationCtx, cancel := context.WithTimeout(ctx, e.validationTimeout)
	defer cancel()

	diagnostics, err := e.lsp.GetDiagnostics(validationCtx, path)
	if err != nil {
		span.SetStatus(codes.Error, "validation failed")
		span.RecordError(err)
		return nil, fmt.Errorf("failed to get diagnostics for %s: %w", path, err)
	}

	span.SetStatus(codes.Ok, "validation complete")
	span.SetAttributes(attribute.Int("diagnostic_count", len(diagnostics)))
	return diagnostics, nil
}

// SetFuzzyThreshold configures the similarity threshold for fuzzy matching.
func (e *EditorImpl) SetFuzzyThreshold(threshold float64) {
	if threshold < 0.0 {
		threshold = 0.0
	}
	if threshold > 1.0 {
		threshold = 1.0
	}
	e.fuzzyThreshold = threshold
	e.logger.Debug("fuzzy threshold updated", "threshold", threshold)
}

// SetValidationTimeout configures the maximum time to wait for LSP validation.
func (e *EditorImpl) SetValidationTimeout(timeout time.Duration) {
	e.validationTimeout = timeout
	e.logger.Debug("validation timeout updated", "timeout", timeout)
}

// applyEdit performs the actual SEARCH/REPLACE operation on a file.
// It attempts exact matching first, then falls back to fuzzy matching.
// Results are written to the provided EditResult.
func (e *EditorImpl) applyEdit(ctx context.Context, edit Edit, result *EditResult) error {
	// Construct absolute file path
	absPath := filepath.Join(e.workspaceRoot, edit.FilePath)

	// Read current file content
	content, err := os.ReadFile(absPath)
	if err != nil {
		return fmt.Errorf("failed to read file %s: %w", edit.FilePath, err)
	}

	fileContent := string(content)

	// Attempt exact match first
	sr := NewSearchReplace(fileContent, edit.SearchBlock, edit.ReplaceBlock)
	newContent, matchResult := sr.Apply()

	if matchResult.Found {
		result.Applied = true
		result.MatchType = codegen.MatchExact
		result.FuzzyMatchSimilarity = 1.0

		e.logger.Debug("exact match found",
			"file", edit.FilePath,
			"start_line", matchResult.StartLine,
			"end_line", matchResult.EndLine)

		// Write modified content
		if err := os.WriteFile(absPath, []byte(newContent), 0644); err != nil {
			return fmt.Errorf("failed to write file %s: %w", edit.FilePath, err)
		}

		return nil
	}

	// Exact match failed, try fuzzy matching
	e.logger.Debug("exact match failed, trying fuzzy match",
		"file", edit.FilePath,
		"threshold", e.fuzzyThreshold)

	fm := NewFuzzyMatcher(fileContent, edit.SearchBlock).WithThreshold(e.fuzzyThreshold)
	fuzzyResult := fm.FindBestMatch()

	if fuzzyResult.Found && fuzzyResult.Similarity >= e.fuzzyThreshold {
		// Apply the replacement using the fuzzy match positions
		newContent = fileContent[:fuzzyResult.StartPos] + edit.ReplaceBlock + fileContent[fuzzyResult.EndPos:]

		result.Applied = true
		result.MatchType = codegen.MatchFuzzy
		result.FuzzyMatchSimilarity = fuzzyResult.Similarity

		e.logger.Debug("fuzzy match found",
			"file", edit.FilePath,
			"similarity", fuzzyResult.Similarity,
			"start_line", fuzzyResult.StartLine,
			"end_line", fuzzyResult.EndLine)

		// Write modified content
		if err := os.WriteFile(absPath, []byte(newContent), 0644); err != nil {
			return fmt.Errorf("failed to write file %s: %w", edit.FilePath, err)
		}

		return nil
	}

	// No match found
	result.Applied = false
	result.MatchType = codegen.MatchFailed
	result.FuzzyMatchSimilarity = fuzzyResult.Similarity

	// Provide debugging information about the closest match
	if fuzzyResult.Similarity > 0.0 {
		// Limit content to 500 characters for debugging
		closestContent := fuzzyResult.MatchedContent
		if len(closestContent) > 500 {
			closestContent = closestContent[:500] + "..."
		}

		result.ClosestMatch = &ClosestMatchInfo{
			StartLine:  fuzzyResult.StartLine,
			EndLine:    fuzzyResult.EndLine,
			Similarity: fuzzyResult.Similarity,
			Content:    closestContent,
		}

		e.logger.Debug("closest match found",
			"file", edit.FilePath,
			"similarity", fuzzyResult.Similarity,
			"start_line", fuzzyResult.StartLine,
			"end_line", fuzzyResult.EndLine)
	}

	return nil
}

// validateEdit validates a file with LSP and populates diagnostics in the result.
func (e *EditorImpl) validateEdit(ctx context.Context, filePath string, result *EditResult) error {
	diagnostics, err := e.validateFile(ctx, filePath)
	if err != nil {
		return err
	}

	result.Diagnostics = diagnostics
	return nil
}

// validateFile performs LSP validation on a single file with timeout.
func (e *EditorImpl) validateFile(ctx context.Context, filePath string) ([]codegen.Diagnostic, error) {
	// Create timeout context for validation
	validationCtx, cancel := context.WithTimeout(ctx, e.validationTimeout)
	defer cancel()

	// Construct absolute path for LSP
	absPath := filepath.Join(e.workspaceRoot, filePath)

	diagnostics, err := e.lsp.GetDiagnostics(validationCtx, absPath)
	if err != nil {
		// Check if timeout occurred
		if validationCtx.Err() == context.DeadlineExceeded {
			e.logger.Warn("LSP validation timeout",
				"file", filePath,
				"timeout", e.validationTimeout)
			return []codegen.Diagnostic{}, fmt.Errorf("LSP validation timeout after %v", e.validationTimeout)
		}
		return nil, err
	}

	return diagnostics, nil
}

// WithLogger sets a custom logger for the editor.
func (e *EditorImpl) WithLogger(logger *slog.Logger) *EditorImpl {
	e.logger = logger
	return e
}

// WorkspaceRoot returns the workspace root path.
func (e *EditorImpl) WorkspaceRoot() string {
	return e.workspaceRoot
}

// Stats returns editor statistics (for monitoring/debugging).
type EditorStats struct {
	FuzzyThreshold    float64
	ValidationTimeout time.Duration
	LSPEnabled        bool
	WorkspaceRoot     string
}

// Stats returns current editor configuration and statistics.
func (e *EditorImpl) Stats() EditorStats {
	return EditorStats{
		FuzzyThreshold:    e.fuzzyThreshold,
		ValidationTimeout: e.validationTimeout,
		LSPEnabled:        e.lsp != nil,
		WorkspaceRoot:     e.workspaceRoot,
	}
}

// FormatEditSummary returns a human-readable summary of an edit result.
func FormatEditSummary(result *EditResult) string {
	var sb strings.Builder

	sb.WriteString(fmt.Sprintf("File: %s\n", result.FilePath))
	sb.WriteString(fmt.Sprintf("Applied: %v\n", result.Applied))
	sb.WriteString(fmt.Sprintf("Match Type: %s\n", result.MatchType.String()))

	if result.MatchType == codegen.MatchFuzzy {
		sb.WriteString(fmt.Sprintf("Similarity: %.2f%%\n", result.FuzzyMatchSimilarity*100))
	}

	if result.ClosestMatch != nil && !result.Applied {
		sb.WriteString("\nClosest Match:\n")
		sb.WriteString(fmt.Sprintf("  Lines: %d-%d\n", result.ClosestMatch.StartLine, result.ClosestMatch.EndLine))
		sb.WriteString(fmt.Sprintf("  Similarity: %.2f%%\n", result.ClosestMatch.Similarity*100))
	}

	if len(result.Diagnostics) > 0 {
		sb.WriteString(fmt.Sprintf("\nDiagnostics: %d\n", len(result.Diagnostics)))
		sb.WriteString(fmt.Sprintf("  Errors: %d\n", result.ErrorCount()))
		sb.WriteString(fmt.Sprintf("  Warnings: %d\n", result.WarningCount()))
	}

	return sb.String()
}

// FormatBatchSummary returns a human-readable summary of a batch edit result.
func FormatBatchSummary(result *BatchEditResult) string {
	var sb strings.Builder

	sb.WriteString(fmt.Sprintf("Batch Applied: %v\n", result.Applied))
	sb.WriteString(fmt.Sprintf("Total Edits: %d\n", len(result.Results)))
	sb.WriteString(fmt.Sprintf("Successful: %d\n", result.SuccessfulEdits()))
	sb.WriteString(fmt.Sprintf("Failed: %d\n", result.FailedEdits()))
	sb.WriteString(fmt.Sprintf("Validation: %s\n", result.ValidationStatus.String()))

	if len(result.Diagnostics) > 0 {
		sb.WriteString(fmt.Sprintf("\nTotal Diagnostics: %d\n", len(result.Diagnostics)))
		sb.WriteString(fmt.Sprintf("  Errors: %d\n", result.ErrorCount()))
		sb.WriteString(fmt.Sprintf("  Warnings: %d\n", result.WarningCount()))
	}

	return sb.String()
}
