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
	"time"
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
	// Branch is the name of the current branch, or empty if HEAD is detached
	Branch string

	// Commit is the current HEAD commit SHA
	Commit string

	// Staged contains paths of files that have been staged for commit
	Staged []string

	// Unstaged contains paths of files that have modifications but are not staged
	Unstaged []string

	// Untracked contains paths of files that are not tracked by Git
	Untracked []string

	// Ahead is the number of commits the current branch is ahead of its upstream
	Ahead int

	// Behind is the number of commits the current branch is behind its upstream
	Behind int

	// HasConflicts indicates whether there are unresolved merge conflicts
	HasConflicts bool
}

// CommitOptions configures commit behavior.
type CommitOptions struct {
	// Author specifies the commit author in the format "Name <email>".
	// If empty, Git's configured user.name and user.email will be used.
	Author string

	// AllowEmpty allows creating a commit even if there are no changes.
	// Default is false.
	AllowEmpty bool

	// Timestamp specifies the commit timestamp.
	// If zero, the current time will be used.
	Timestamp time.Time

	// Amend modifies the previous commit instead of creating a new one.
	// Default is false.
	Amend bool
}

// PushOptions configures push behavior.
type PushOptions struct {
	// Remote specifies the remote repository name.
	// Default is "origin" if empty.
	Remote string

	// Force enables force push (use with caution).
	// Default is false.
	Force bool

	// SetUpstream sets the upstream tracking branch for the current branch.
	// This is equivalent to git push -u or git push --set-upstream.
	// Default is false.
	SetUpstream bool

	// RefSpec allows specifying custom refspecs for the push operation.
	// If empty, the current branch will be pushed to its upstream.
	// Example: "refs/heads/main:refs/heads/main"
	RefSpec string

	// Tags controls whether to push tags along with the commits.
	// Default is false (tags are not pushed).
	Tags bool
}

// SnapshotMetadata contains information about a snapshot.
// This is used internally by implementations to track snapshot state.
type SnapshotMetadata struct {
	// ID is the unique identifier for this snapshot
	ID string

	// CreatedAt is the timestamp when the snapshot was created
	CreatedAt time.Time

	// Branch is the branch that was active when the snapshot was created
	Branch string

	// Commit is the HEAD commit SHA when the snapshot was created
	Commit string

	// Description is an optional human-readable description of the snapshot
	Description string
}
