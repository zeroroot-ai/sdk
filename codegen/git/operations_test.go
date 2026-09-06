// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Zero Root AI

package git

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestValidateGitArg tests argument validation for command injection prevention.
func TestValidateGitArg(t *testing.T) {
	tests := []struct {
		name    string
		arg     string
		wantErr bool
	}{
		{"empty arg", "", false},
		{"safe branch name", "feature-branch", false},
		{"safe path", "src/main.go", false},
		{"safe commit message", "Fix bug in parser", false},
		{"email address", "Author <author@example.com>", false}, // < and > allowed for emails
		{"semicolon injection", "test;rm -rf /", true},
		{"pipe injection", "test | cat /etc/passwd", true},
		{"backtick injection", "test`whoami`", true},
		{"dollar injection", "test$HOME", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateGitArg(tt.arg)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

// TestValidateRepoURL tests URL validation.
func TestValidateRepoURL(t *testing.T) {
	tests := []struct {
		name    string
		url     string
		wantErr bool
	}{
		{"https github", "https://github.com/org/repo.git", false},
		{"https gitlab", "https://gitlab.com/org/repo", false},
		{"ssh github", "git@github.com:org/repo.git", false},
		{"invalid scheme", "ftp://example.com/repo", true},
		{"with semicolon", "https://github.com/org/repo;rm", true},
		{"with pipe", "https://github.com/org/repo|cat", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateRepoURL(tt.url)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

// TestGitOpsWithRealRepo tests GitOps with a real Git repository.
func TestGitOpsWithRealRepo(t *testing.T) {
	// Check if git is available
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not available in PATH")
	}

	// Create a temporary directory for the test repo
	tempDir := t.TempDir()
	repoPath := filepath.Join(tempDir, "test-repo")

	// Initialize a new git repository
	cmd := exec.Command("git", "init", repoPath)
	require.NoError(t, cmd.Run())

	// Configure git user for commits
	cmd = exec.Command("git", "config", "user.name", "Test User")
	cmd.Dir = repoPath
	require.NoError(t, cmd.Run())

	cmd = exec.Command("git", "config", "user.email", "test@example.com")
	cmd.Dir = repoPath
	require.NoError(t, cmd.Run())

	// Create initial commit
	testFile := filepath.Join(repoPath, "test.txt")
	require.NoError(t, os.WriteFile(testFile, []byte("initial content\n"), 0644))

	cmd = exec.Command("git", "add", "test.txt")
	cmd.Dir = repoPath
	require.NoError(t, cmd.Run())

	cmd = exec.Command("git", "commit", "-m", "Initial commit")
	cmd.Dir = repoPath
	require.NoError(t, cmd.Run())

	// Create GitOps instance
	gitOps := NewGitOps(repoPath, nil)

	// Test CurrentBranch
	t.Run("CurrentBranch", func(t *testing.T) {
		branch, err := gitOps.CurrentBranch()
		require.NoError(t, err)
		// Could be "master" or "main" depending on git version
		assert.Contains(t, []string{"master", "main"}, branch)
	})

	// Test Status
	t.Run("Status", func(t *testing.T) {
		status, err := gitOps.Status()
		require.NoError(t, err)
		assert.NotEmpty(t, status.Commit)
		assert.Empty(t, status.Staged)
		assert.Empty(t, status.Unstaged)
		assert.Empty(t, status.Untracked)
	})

	// Test CreateBranch and Checkout
	t.Run("CreateBranch and Checkout", func(t *testing.T) {
		ctx := context.Background()

		// Create new branch
		err := gitOps.CreateBranch(ctx, "feature-test")
		require.NoError(t, err)

		// Checkout the branch
		err = gitOps.Checkout(ctx, "feature-test")
		require.NoError(t, err)

		// Verify we're on the new branch
		branch, err := gitOps.CurrentBranch()
		require.NoError(t, err)
		assert.Equal(t, "feature-test", branch)
	})

	// Test Add and Commit
	t.Run("Add and Commit", func(t *testing.T) {
		ctx := context.Background()

		// Modify file
		require.NoError(t, os.WriteFile(testFile, []byte("modified content\n"), 0644))

		// Add file
		err := gitOps.Add(ctx, "test.txt")
		require.NoError(t, err)

		// Commit
		sha, err := gitOps.Commit(ctx, "Update test file", CommitOptions{})
		require.NoError(t, err)
		assert.NotEmpty(t, sha)
		assert.Len(t, sha, 40) // SHA-1 is 40 hex chars
	})

	// Test Commit with options
	t.Run("Commit with options", func(t *testing.T) {
		ctx := context.Background()

		// Modify file again
		require.NoError(t, os.WriteFile(testFile, []byte("another change\n"), 0644))

		// Add and commit with author and timestamp
		err := gitOps.Add(ctx, "test.txt")
		require.NoError(t, err)

		timestamp := time.Date(2024, 1, 1, 12, 0, 0, 0, time.UTC)
		opts := CommitOptions{
			Author:    "Custom Author <custom@example.com>",
			Timestamp: timestamp,
		}

		sha, err := gitOps.Commit(ctx, "Custom commit", opts)
		require.NoError(t, err)
		assert.NotEmpty(t, sha)
	})
}

// TestSnapshotAndRollback tests snapshot and rollback functionality.
func TestSnapshotAndRollback(t *testing.T) {
	// Check if git is available
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not available in PATH")
	}

	// Create a temporary directory for the test repo
	tempDir := t.TempDir()
	repoPath := filepath.Join(tempDir, "test-repo")

	// Initialize repository
	cmd := exec.Command("git", "init", repoPath)
	require.NoError(t, cmd.Run())

	// Configure git user
	cmd = exec.Command("git", "config", "user.name", "Test User")
	cmd.Dir = repoPath
	require.NoError(t, cmd.Run())

	cmd = exec.Command("git", "config", "user.email", "test@example.com")
	cmd.Dir = repoPath
	require.NoError(t, cmd.Run())

	// Create initial commit
	testFile := filepath.Join(repoPath, "test.txt")
	require.NoError(t, os.WriteFile(testFile, []byte("initial\n"), 0644))

	cmd = exec.Command("git", "add", "test.txt")
	cmd.Dir = repoPath
	require.NoError(t, cmd.Run())

	cmd = exec.Command("git", "commit", "-m", "Initial commit")
	cmd.Dir = repoPath
	require.NoError(t, cmd.Run())

	gitOps := NewGitOps(repoPath, nil)
	ctx := context.Background()

	// Modify the file
	require.NoError(t, os.WriteFile(testFile, []byte("modified\n"), 0644))

	// Create snapshot
	snapshotID, err := gitOps.Snapshot(ctx)
	require.NoError(t, err)
	assert.NotEmpty(t, snapshotID)

	// Make more changes
	require.NoError(t, os.WriteFile(testFile, []byte("changed again\n"), 0644))

	// Read content before rollback
	contentBefore, err := os.ReadFile(testFile)
	require.NoError(t, err)
	assert.Equal(t, "changed again\n", string(contentBefore))

	// Rollback to snapshot
	err = gitOps.Rollback(ctx, snapshotID)
	require.NoError(t, err)

	// Verify content is restored to snapshot state
	contentAfter, err := os.ReadFile(testFile)
	require.NoError(t, err)
	assert.Equal(t, "modified\n", string(contentAfter))
}

// TestSnapshotWithUntrackedFiles tests that snapshots include untracked files.
func TestSnapshotWithUntrackedFiles(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not available in PATH")
	}

	tempDir := t.TempDir()
	repoPath := filepath.Join(tempDir, "test-repo")

	// Initialize repository
	cmd := exec.Command("git", "init", repoPath)
	require.NoError(t, cmd.Run())

	cmd = exec.Command("git", "config", "user.name", "Test User")
	cmd.Dir = repoPath
	require.NoError(t, cmd.Run())

	cmd = exec.Command("git", "config", "user.email", "test@example.com")
	cmd.Dir = repoPath
	require.NoError(t, cmd.Run())

	// Create initial commit
	require.NoError(t, os.WriteFile(filepath.Join(repoPath, "tracked.txt"), []byte("tracked\n"), 0644))

	cmd = exec.Command("git", "add", "tracked.txt")
	cmd.Dir = repoPath
	require.NoError(t, cmd.Run())

	cmd = exec.Command("git", "commit", "-m", "Initial")
	cmd.Dir = repoPath
	require.NoError(t, cmd.Run())

	gitOps := NewGitOps(repoPath, nil)
	ctx := context.Background()

	// Create untracked file
	untrackedFile := filepath.Join(repoPath, "untracked.txt")
	require.NoError(t, os.WriteFile(untrackedFile, []byte("untracked content\n"), 0644))

	// Create snapshot
	snapshotID, err := gitOps.Snapshot(ctx)
	require.NoError(t, err)

	// Remove untracked file
	require.NoError(t, os.Remove(untrackedFile))

	// Verify file is gone
	_, err = os.Stat(untrackedFile)
	assert.True(t, os.IsNotExist(err))

	// Rollback
	err = gitOps.Rollback(ctx, snapshotID)
	require.NoError(t, err)

	// Verify untracked file is restored
	content, err := os.ReadFile(untrackedFile)
	require.NoError(t, err)
	assert.Equal(t, "untracked content\n", string(content))
}
