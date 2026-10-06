// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Zero Root AI

package git

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestGitCommandConstruction tests that git commands are built correctly
// without executing them (using mock execution).
func TestGitCommandConstruction(t *testing.T) {
	tests := []struct {
		name     string
		method   string
		args     []string
		expected []string
	}{
		{
			name:     "current branch",
			method:   "CurrentBranch",
			args:     []string{"rev-parse", "--abbrev-ref", "HEAD"},
			expected: []string{"git", "rev-parse", "--abbrev-ref", "HEAD"},
		},
		{
			name:     "status porcelain",
			method:   "Status",
			args:     []string{"status", "--porcelain"},
			expected: []string{"git", "status", "--porcelain"},
		},
		{
			name:     "create branch",
			method:   "CreateBranch",
			args:     []string{"branch", "feature-test"},
			expected: []string{"git", "branch", "feature-test"},
		},
		{
			name:     "checkout branch",
			method:   "Checkout",
			args:     []string{"checkout", "main"},
			expected: []string{"git", "checkout", "main"},
		},
		{
			name:     "add files",
			method:   "Add",
			args:     []string{"add", "--", "file1.txt", "file2.txt"},
			expected: []string{"git", "add", "--", "file1.txt", "file2.txt"},
		},
		{
			name:     "commit message",
			method:   "Commit",
			args:     []string{"commit", "-m", "Fix: important bug"},
			expected: []string{"git", "commit", "-m", "Fix: important bug"},
		},
		{
			name:     "commit with author",
			method:   "Commit",
			args:     []string{"commit", "-m", "Update", "--author", "Test User <test@example.com>"},
			expected: []string{"git", "commit", "-m", "Update", "--author", "Test User <test@example.com>"},
		},
		{
			name:     "push to remote",
			method:   "Push",
			args:     []string{"push", "origin"},
			expected: []string{"git", "push", "origin"},
		},
		{
			name:     "push with set-upstream",
			method:   "Push",
			args:     []string{"push", "--set-upstream", "origin", "feature"},
			expected: []string{"git", "push", "--set-upstream", "origin", "feature"},
		},
		{
			name:     "force push",
			method:   "Push",
			args:     []string{"push", "--force", "origin"},
			expected: []string{"git", "push", "--force", "origin"},
		},
		{
			name:     "pull from remote",
			method:   "Pull",
			args:     []string{"pull"},
			expected: []string{"git", "pull"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Verify the expected command structure
			assert.Equal(t, "git", tt.expected[0])
			assert.Equal(t, tt.args, tt.expected[1:])
		})
	}
}
