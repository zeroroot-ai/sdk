// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Zero Root AI

// Package git provides Git operations for the CodeGen SDK.
//
// This package defines interfaces and types for Git operations including branching,
// committing, pushing, and snapshot/rollback functionality. The snapshot and rollback
// operations are designed to NOT pollute Git history - they do not create commits or
// visible refs in the repository.
package git

import (
	"context"
)

// GitOps provides Git operations for a repository workspace.
//
// All operations support context cancellation and timeout control.
// The Snapshot and Rollback methods provide a way to save and restore
// working directory state without creating commits or visible refs.
type GitOps interface {
	// CurrentBranch returns the name of the currently checked out branch.
	// Returns an error if HEAD is detached or if the repository state cannot be read.
	CurrentBranch() (string, error)

	// Status returns the current repository status including staged, unstaged,
	// and untracked files.
	Status() (*GitStatus, error)

	// CreateBranch creates a new branch with the given name from the current HEAD.
	// Returns an error if the branch already exists or if the repository is in an invalid state.
	CreateBranch(ctx context.Context, name string) error

	// Checkout switches to the specified branch or commit ref.
	// The ref can be a branch name, tag, or commit SHA.
	Checkout(ctx context.Context, ref string) error

	// Add stages the specified paths for commit.
	// Use "." to stage all changes in the working directory.
	Add(ctx context.Context, paths ...string) error

	// Commit creates a new commit with the given message and options.
	// Returns the commit SHA on success.
	Commit(ctx context.Context, message string, opts CommitOptions) (string, error)

	// Push pushes commits to the remote repository.
	// Returns an error if the remote has diverged or if authentication fails.
	Push(ctx context.Context, opts PushOptions) error

	// Pull fetches and merges changes from the remote tracking branch.
	// Returns an error if there are merge conflicts or if the remote cannot be reached.
	Pull(ctx context.Context) error

	// Snapshot creates a snapshot of the current working directory state.
	// The snapshot is stored in a way that does NOT pollute Git history:
	// - No commits are created
	// - No refs (branches/tags) are visible
	// - No push/pull operations will include the snapshot data
	//
	// Returns a snapshot ID that can be used with Rollback.
	// The snapshot includes staged and unstaged changes as well as untracked files.
	Snapshot(ctx context.Context) (string, error)

	// Rollback restores the working directory to the state captured by the given snapshot.
	// This operation:
	// - Restores all tracked files to their snapshot state
	// - Restores untracked files that were present in the snapshot
	// - Removes files that were not present in the snapshot
	// - Does NOT create any commits or modify Git history
	//
	// Returns an error if the snapshot ID is invalid or if the restore fails.
	Rollback(ctx context.Context, snapshotID string) error
}

// GitStatus represents the status of a Git repository.
type GitStatus struct {
}

// CommitOptions configures commit behavior.
type CommitOptions struct {
}

// PushOptions configures push behavior.
type PushOptions struct {
}
