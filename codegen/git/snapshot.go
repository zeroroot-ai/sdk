// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Zero Root AI

package git

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"

	"github.com/zeroroot-ai/sdk/codegen"
)

// Snapshot creates a snapshot of the current working directory state.
//
// Implementation details:
// - Uses `git stash create` to create a tree object representing the current state
// - Does NOT create a stash ref, so it doesn't pollute git history
// - Returns the tree SHA which can be used with `git stash apply` for rollback
// - Includes staged, unstaged, and untracked files
// - No commits are created, no refs are visible in `git stash list`
// - Push/pull operations will NOT include the snapshot data
func (g *gitOps) Snapshot(ctx context.Context) (string, error) {
	startTime := time.Now()
	ctx, span := codegen.StartSpan(ctx, "codegen.git.snapshot")
	defer span.End()

	// First, add untracked files to the index temporarily so they're included in the stash
	// We need to identify untracked files first
	status, err := g.Status()
	if err != nil {
		span.SetStatus(codes.Error, "failed to get status")
		span.RecordError(err)
		g.metrics.RecordError(ctx, g.metrics.GitOperationsTotal, "snapshot")
		return "", fmt.Errorf("failed to get status for snapshot: %w", err)
	}

	// If there are untracked files, we need to add them temporarily
	hasUntracked := len(status.Untracked) > 0

	// Create a snapshot ID based on current timestamp
	snapshotID := fmt.Sprintf("snapshot-%d", time.Now().UnixNano())

	if hasUntracked {
		// Stage untracked files temporarily
		if err := g.Add(ctx, "."); err != nil {
			return "", fmt.Errorf("failed to stage untracked files for snapshot: %w", err)
		}
	}

	// Create the stash without creating a ref
	// `git stash create` creates a commit object but doesn't create any refs
	// It returns the SHA of the stash commit
	stashSHA, err := g.execGit(ctx, "stash", "create")
	if err != nil {
		duration := time.Since(startTime).Seconds()
		span.SetStatus(codes.Error, "stash create failed")
		span.RecordError(err)
		g.metrics.RecordError(ctx, g.metrics.GitOperationsTotal, "snapshot")
		g.metrics.RecordDuration(ctx, g.metrics.GitOperationDurationSeconds, duration,
			attribute.String(codegen.MetricAttrOperation, "snapshot"))
		return "", fmt.Errorf("failed to create snapshot: %w", err)
	}

	// If no changes to snapshot, git stash create returns empty
	if stashSHA == "" {
		// Check if we have a clean working directory
		if !hasUntracked && len(status.Staged) == 0 && len(status.Unstaged) == 0 {
			// For a clean working directory, record current HEAD commit as the snapshot
			// This allows us to rollback to this clean state if needed
			headCommit, err := g.execGit(ctx, "rev-parse", "HEAD")
			if err != nil {
				return "", fmt.Errorf("failed to get HEAD commit for clean snapshot: %w", err)
			}
			stashSHA = strings.TrimSpace(headCommit)
			if stashSHA == "" {
				return "", errors.New("no HEAD commit found (empty repository?)")
			}
			// Save with special marker to indicate this is a HEAD snapshot, not a stash
			if err := g.saveSnapshotMetadata(snapshotID, "HEAD:"+stashSHA); err != nil {
				return "", fmt.Errorf("failed to save snapshot metadata: %w", err)
			}
			return snapshotID, nil
		}

		// If we have changes but stash create returned empty, something went wrong
		return "", errors.New("failed to create snapshot: no stash SHA returned")
	}

	// Reset the index if we staged untracked files
	if hasUntracked {
		// Reset the staging area to HEAD (unstage everything we just added)
		if _, err := g.execGit(ctx, "reset", "HEAD"); err != nil {
			return "", fmt.Errorf("failed to reset after snapshot: %w", err)
		}
	}

	// Store the snapshot SHA in a temporary directory for reference
	if err := g.saveSnapshotMetadata(snapshotID, stashSHA); err != nil {
		duration := time.Since(startTime).Seconds()
		span.SetStatus(codes.Error, "failed to save metadata")
		span.RecordError(err)
		g.metrics.RecordError(ctx, g.metrics.GitOperationsTotal, "snapshot")
		g.metrics.RecordDuration(ctx, g.metrics.GitOperationDurationSeconds, duration,
			attribute.String(codegen.MetricAttrOperation, "snapshot"))
		return "", fmt.Errorf("failed to save snapshot metadata: %w", err)
	}

	duration := time.Since(startTime).Seconds()
	span.SetStatus(codes.Ok, "snapshot created")
	span.SetAttributes(attribute.String("snapshot.id", snapshotID))
	g.metrics.RecordSuccess(ctx, g.metrics.GitOperationsTotal, "snapshot")
	g.metrics.RecordDuration(ctx, g.metrics.GitOperationDurationSeconds, duration,
		attribute.String(codegen.MetricAttrOperation, "snapshot"))
	return snapshotID, nil
}

// Rollback restores the working directory to the state captured by the given snapshot.
//
// Implementation details:
// - Uses `git stash apply` with the snapshot SHA to restore the state
// - Does NOT create any commits or modify Git history
// - Restores all files (tracked and untracked) to their snapshot state
// - Preserves the current HEAD/branch position
func (g *gitOps) Rollback(ctx context.Context, snapshotID string) error {
	startTime := time.Now()
	ctx, span := codegen.StartSpan(ctx, "codegen.git.rollback",
		attribute.String("snapshot.id", snapshotID))
	defer span.End()

	if snapshotID == "" {
		span.SetStatus(codes.Error, "empty snapshot ID")
		g.metrics.RecordError(ctx, g.metrics.GitOperationsTotal, "rollback")
		return errors.New("snapshot ID cannot be empty")
	}

	// Load the snapshot metadata to get the stash SHA
	stashSHA, err := g.loadSnapshotMetadata(snapshotID)
	if err != nil {
		span.SetStatus(codes.Error, "failed to load metadata")
		span.RecordError(err)
		g.metrics.RecordError(ctx, g.metrics.GitOperationsTotal, "rollback")
		return fmt.Errorf("failed to load snapshot metadata: %w", err)
	}

	if stashSHA == "" {
		span.SetStatus(codes.Error, "snapshot not found")
		g.metrics.RecordError(ctx, g.metrics.GitOperationsTotal, "rollback")
		return fmt.Errorf("snapshot %s not found", snapshotID)
	}

	// Check if this is a HEAD snapshot (clean working directory snapshot)
	if strings.HasPrefix(stashSHA, "HEAD:") {
		// This was a clean working directory snapshot
		// Just reset to HEAD and clean
		if _, err := g.execGit(ctx, "reset", "--hard", "HEAD"); err != nil {
			return fmt.Errorf("failed to reset working directory: %w", err)
		}
		if _, err := g.execGit(ctx, "clean", "-fd"); err != nil {
			return fmt.Errorf("failed to clean untracked files: %w", err)
		}
		return nil
	}

	// First, reset the working directory to HEAD to clear any modifications
	if _, err := g.execGit(ctx, "reset", "--hard", "HEAD"); err != nil {
		return fmt.Errorf("failed to reset working directory: %w", err)
	}

	// Clean untracked files
	if _, err := g.execGit(ctx, "clean", "-fd"); err != nil {
		return fmt.Errorf("failed to clean untracked files: %w", err)
	}

	// Apply the stash to restore the snapshot state
	// Using --index to restore both working directory and index state
	_, err = g.execGit(ctx, "stash", "apply", "--index", stashSHA)
	if err != nil {
		duration := time.Since(startTime).Seconds()
		span.SetStatus(codes.Error, "stash apply failed")
		span.RecordError(err)
		g.metrics.RecordError(ctx, g.metrics.GitOperationsTotal, "rollback")
		g.metrics.RecordDuration(ctx, g.metrics.GitOperationDurationSeconds, duration,
			attribute.String(codegen.MetricAttrOperation, "rollback"))
		// If apply fails, it might be due to conflicts or invalid SHA
		return fmt.Errorf("failed to apply snapshot: %w", err)
	}

	duration := time.Since(startTime).Seconds()
	span.SetStatus(codes.Ok, "rollback complete")
	g.metrics.RecordSuccess(ctx, g.metrics.GitOperationsTotal, "rollback")
	g.metrics.RecordDuration(ctx, g.metrics.GitOperationDurationSeconds, duration,
		attribute.String(codegen.MetricAttrOperation, "rollback"))
	return nil
}

// saveSnapshotMetadata saves the snapshot SHA to a metadata file.
// This allows us to retrieve the snapshot later using its ID.
func (g *gitOps) saveSnapshotMetadata(snapshotID, stashSHA string) error {
	metadataDir := filepath.Join(g.repoPath, ".git", "codegen-snapshots")
	if err := os.MkdirAll(metadataDir, 0755); err != nil {
		return fmt.Errorf("failed to create metadata directory: %w", err)
	}

	metadataFile := filepath.Join(metadataDir, snapshotID)
	if err := os.WriteFile(metadataFile, []byte(stashSHA), 0644); err != nil {
		return fmt.Errorf("failed to write metadata file: %w", err)
	}

	return nil
}

// loadSnapshotMetadata loads the snapshot SHA from a metadata file.
func (g *gitOps) loadSnapshotMetadata(snapshotID string) (string, error) {
	metadataFile := filepath.Join(g.repoPath, ".git", "codegen-snapshots", snapshotID)

	data, err := os.ReadFile(metadataFile)
	if err != nil {
		if os.IsNotExist(err) {
			return "", errors.New("snapshot not found")
		}
		return "", fmt.Errorf("failed to read metadata file: %w", err)
	}

	return strings.TrimSpace(string(data)), nil
}

// CleanupSnapshot removes the metadata for a snapshot after it's no longer needed.
// This is optional and used for cleanup purposes only - the snapshot data itself
// (the stash commit) remains in git's object database until garbage collected.
func (g *gitOps) CleanupSnapshot(snapshotID string) error {
	if snapshotID == "" {
		return errors.New("snapshot ID cannot be empty")
	}

	metadataFile := filepath.Join(g.repoPath, ".git", "codegen-snapshots", snapshotID)
	if err := os.Remove(metadataFile); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("failed to remove snapshot metadata: %w", err)
	}

	return nil
}

// ListSnapshots returns a list of all available snapshot IDs for this repository.
func (g *gitOps) ListSnapshots() ([]string, error) {
	metadataDir := filepath.Join(g.repoPath, ".git", "codegen-snapshots")

	entries, err := os.ReadDir(metadataDir)
	if err != nil {
		if os.IsNotExist(err) {
			return []string{}, nil
		}
		return nil, fmt.Errorf("failed to read snapshots directory: %w", err)
	}

	snapshots := make([]string, 0, len(entries))
	for _, entry := range entries {
		if !entry.IsDir() {
			snapshots = append(snapshots, entry.Name())
		}
	}

	return snapshots, nil
}
