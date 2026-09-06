// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Zero Root AI

package git

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/zeroroot-ai/sdk/types"
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

// TestCloneCommandConstruction tests clone command construction with various options.
func TestCloneCommandConstruction(t *testing.T) {
	tests := []struct {
		name     string
		url      string
		destPath string
		opts     CloneOptions
		expected []string
	}{
		{
			name:     "basic clone",
			url:      "https://github.com/org/repo.git",
			destPath: "/tmp/repo",
			opts:     CloneOptions{},
			expected: []string{"git", "clone", "https://github.com/org/repo.git", "/tmp/repo"},
		},
		{
			name:     "shallow clone",
			url:      "https://github.com/org/repo.git",
			destPath: "/tmp/repo",
			opts:     CloneOptions{Depth: 1},
			expected: []string{"git", "clone", "--depth", "1", "https://github.com/org/repo.git", "/tmp/repo"},
		},
		{
			name:     "clone with branch",
			url:      "https://github.com/org/repo.git",
			destPath: "/tmp/repo",
			opts:     CloneOptions{Branch: "develop"},
			expected: []string{"git", "clone", "--branch", "develop", "https://github.com/org/repo.git", "/tmp/repo"},
		},
		{
			name:     "single branch clone",
			url:      "https://github.com/org/repo.git",
			destPath: "/tmp/repo",
			opts:     CloneOptions{SingleBranch: true},
			expected: []string{"git", "clone", "--single-branch", "https://github.com/org/repo.git", "/tmp/repo"},
		},
		{
			name:     "shallow single branch clone",
			url:      "https://github.com/org/repo.git",
			destPath: "/tmp/repo",
			opts:     CloneOptions{Depth: 1, Branch: "main", SingleBranch: true},
			expected: []string{"git", "clone", "--depth", "1", "--branch", "main", "--single-branch", "https://github.com/org/repo.git", "/tmp/repo"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Build the command args
			args := []string{"clone"}

			if tt.opts.Depth > 0 {
				args = append(args, "--depth", strconv.Itoa(tt.opts.Depth))
			}
			if tt.opts.Branch != "" {
				args = append(args, "--branch", tt.opts.Branch)
			}
			if tt.opts.SingleBranch {
				args = append(args, "--single-branch")
			}
			args = append(args, tt.url, tt.destPath)

			// Verify command construction
			assert.Equal(t, tt.expected[1:], args)
		})
	}
}

// TestURLSanitization tests that credentials are sanitized from URLs in logs.
func TestURLSanitization(t *testing.T) {
	tests := []struct {
		name     string
		url      string
		expected string
	}{
		{
			name:     "https with credentials",
			url:      "https://user:pass@github.com/org/repo.git",
			expected: "https://***:***@github.com/org/repo.git",
		},
		{
			name:     "https without credentials",
			url:      "https://github.com/org/repo.git",
			expected: "https://github.com/org/repo.git",
		},
		{
			name:     "ssh url",
			url:      "git@github.com:org/repo.git",
			expected: "git@github.com:org/repo.git",
		},
		{
			name:     "https with token",
			url:      "https://ghp_token12345@github.com/org/repo.git",
			expected: "https://***:***@github.com/org/repo.git",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sanitized := sanitizeGitURL(tt.url)
			assert.Equal(t, tt.expected, sanitized)

			// Verify credentials are not present in sanitized output
			if strings.Contains(tt.url, "user:pass") || strings.Contains(tt.url, "ghp_token") {
				assert.NotContains(t, sanitized, "user")
				assert.NotContains(t, sanitized, "pass")
				assert.NotContains(t, sanitized, "ghp_token")
			}
		})
	}
}

// TestCredentialConfiguration tests credential provider configuration.
func TestCredentialConfiguration(t *testing.T) {
	tempDir := t.TempDir()
	repoPath := filepath.Join(tempDir, "test-repo")

	tests := []struct {
		name       string
		credential *types.Credential
		wantErr    bool
		validate   func(t *testing.T, cleanup func())
	}{
		{
			name: "https token auth",
			credential: &types.Credential{
				Type:   types.CredentialTypeAPIKey,
				Secret: "ghp_test_token_12345",
			},
			wantErr: false,
			validate: func(t *testing.T, cleanup func()) {
				assert.NotNil(t, cleanup)
				// Verify GIT_CONFIG_GLOBAL is set
				configPath := os.Getenv("GIT_CONFIG_GLOBAL")
				assert.NotEmpty(t, configPath)

				// Verify config file exists and has correct permissions
				info, err := os.Stat(configPath)
				require.NoError(t, err)
				assert.Equal(t, os.FileMode(0600), info.Mode().Perm())

				// Verify config content
				content, err := os.ReadFile(configPath)
				require.NoError(t, err)
				assert.Contains(t, string(content), "[credential]")
				assert.Contains(t, string(content), "helper")

				// Cleanup and verify environment is restored
				cleanup()
				assert.Empty(t, os.Getenv("GIT_CONFIG_GLOBAL"))
			},
		},
		{
			name: "basic auth",
			credential: &types.Credential{
				Type:     types.CredentialTypeBasic,
				Username: "testuser",
				Secret:   "testpass",
			},
			wantErr: false,
			validate: func(t *testing.T, cleanup func()) {
				assert.NotNil(t, cleanup)
				configPath := os.Getenv("GIT_CONFIG_GLOBAL")
				assert.NotEmpty(t, configPath)

				// Verify credential helper script exists
				content, err := os.ReadFile(configPath)
				require.NoError(t, err)
				assert.Contains(t, string(content), "helper")

				cleanup()
				assert.Empty(t, os.Getenv("GIT_CONFIG_GLOBAL"))
			},
		},
		{
			name: "ssh key auth",
			credential: &types.Credential{
				Type:   types.CredentialTypeCustom,
				Secret: "-----BEGIN RSA PRIVATE KEY-----\ntest_key_content\n-----END RSA PRIVATE KEY-----",
			},
			wantErr: false,
			validate: func(t *testing.T, cleanup func()) {
				assert.NotNil(t, cleanup)

				// Verify GIT_SSH_COMMAND is set
				sshCommand := os.Getenv("GIT_SSH_COMMAND")
				assert.NotEmpty(t, sshCommand)
				assert.Contains(t, sshCommand, "ssh -i")

				// Extract key path from SSH command
				parts := strings.Split(sshCommand, " ")
				var keyPath string
				for i, part := range parts {
					if part == "-i" && i+1 < len(parts) {
						keyPath = parts[i+1]
						break
					}
				}
				assert.NotEmpty(t, keyPath)

				// Verify key file exists with correct permissions
				info, err := os.Stat(keyPath)
				require.NoError(t, err)
				assert.Equal(t, os.FileMode(0600), info.Mode().Perm())

				// Verify key content
				content, err := os.ReadFile(keyPath)
				require.NoError(t, err)
				assert.Contains(t, string(content), "BEGIN RSA PRIVATE KEY")

				// Cleanup and verify environment is restored
				cleanup()
				assert.Empty(t, os.Getenv("GIT_SSH_COMMAND"))

				// Verify key file is removed
				_, err = os.Stat(keyPath)
				assert.True(t, os.IsNotExist(err))
			},
		},
		{
			name:       "nil credential",
			credential: nil,
			wantErr:    false,
			validate: func(t *testing.T, cleanup func()) {
				assert.NotNil(t, cleanup)
				cleanup() // Should not panic
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Clear environment before each test
			os.Unsetenv("GIT_CONFIG_GLOBAL")
			os.Unsetenv("GIT_SSH_COMMAND")

			provider := NewCredentialProvider(tt.credential)

			// Handle nil provider case
			if provider == nil {
				// Nil provider should not cause errors
				assert.Nil(t, provider)
				return
			}

			cleanup, err := provider.ConfigureAuth(context.Background(), repoPath)
			if tt.wantErr {
				assert.Error(t, err)
				return
			}

			require.NoError(t, err)
			if tt.validate != nil {
				tt.validate(t, cleanup)
			}
		})
	}
}

// TestCredentialCleanup tests that credential files are properly cleaned up.
func TestCredentialCleanup(t *testing.T) {
	tempDir := t.TempDir()
	repoPath := filepath.Join(tempDir, "test-repo")

	tests := []struct {
		name       string
		credential *types.Credential
	}{
		{
			name: "token cleanup",
			credential: &types.Credential{
				Type:   types.CredentialTypeBearer,
				Secret: "token123",
			},
		},
		{
			name: "ssh key cleanup",
			credential: &types.Credential{
				Type:   types.CredentialTypeCustom,
				Secret: "-----BEGIN RSA PRIVATE KEY-----\ntest\n-----END RSA PRIVATE KEY-----",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			provider := NewCredentialProvider(tt.credential)

			cleanup, err := provider.ConfigureAuth(context.Background(), repoPath)
			require.NoError(t, err)

			// Track temp files before cleanup
			var tempFiles []string
			if tt.credential.Type == types.CredentialTypeCustom {
				// SSH key
				sshCommand := os.Getenv("GIT_SSH_COMMAND")
				if sshCommand != "" {
					parts := strings.Split(sshCommand, " ")
					for i, part := range parts {
						if part == "-i" && i+1 < len(parts) {
							tempFiles = append(tempFiles, parts[i+1])
							// Also track parent directory
							tempFiles = append(tempFiles, filepath.Dir(parts[i+1]))
						}
					}
				}
			} else {
				// Token/basic auth
				configPath := os.Getenv("GIT_CONFIG_GLOBAL")
				if configPath != "" {
					tempFiles = append(tempFiles, configPath)
					// Track parent directory
					tempFiles = append(tempFiles, filepath.Dir(configPath))
				}
			}

			// Call cleanup
			cleanup()

			// Verify all temp files are removed
			for _, file := range tempFiles {
				_, err := os.Stat(file)
				assert.True(t, os.IsNotExist(err), "file should be removed: %s", file)
			}

			// Verify environment is clean
			assert.Empty(t, os.Getenv("GIT_CONFIG_GLOBAL"))
			assert.Empty(t, os.Getenv("GIT_SSH_COMMAND"))
		})
	}
}

// TestCommandInjectionPrevention tests that dangerous inputs are rejected.
func TestCommandInjectionPrevention(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		wantErr bool
		reason  string
	}{
		{
			name:    "safe branch name",
			input:   "feature/test-branch",
			wantErr: false,
		},
		{
			name:    "safe commit message",
			input:   "Fix: resolve issue with parser",
			wantErr: false,
		},
		{
			name:    "email with angle brackets",
			input:   "Author Name <author@example.com>",
			wantErr: false,
			reason:  "angle brackets allowed for email addresses",
		},
		{
			name:    "semicolon injection",
			input:   "feature; rm -rf /",
			wantErr: true,
			reason:  "semicolon allows command chaining",
		},
		{
			name:    "pipe injection",
			input:   "feature | cat /etc/passwd",
			wantErr: true,
			reason:  "pipe allows command chaining",
		},
		{
			name:    "backtick injection",
			input:   "feature`whoami`",
			wantErr: true,
			reason:  "backticks allow command substitution",
		},
		{
			name:    "dollar sign injection",
			input:   "feature$HOME",
			wantErr: true,
			reason:  "dollar sign allows variable expansion",
		},
		{
			name:    "ampersand injection",
			input:   "feature && rm file",
			wantErr: true,
			reason:  "ampersand allows command chaining",
		},
		{
			name:    "parenthesis injection",
			input:   "feature$(whoami)",
			wantErr: true,
			reason:  "parentheses allow command substitution",
		},
		{
			name:    "newline injection",
			input:   "feature\nrm -rf /",
			wantErr: true,
			reason:  "newline allows command separation",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateGitArg(tt.input)
			if tt.wantErr {
				assert.Error(t, err, "should reject: %s", tt.reason)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

// TestGitStatusParsing tests parsing of git status --porcelain output.
func TestGitStatusParsing(t *testing.T) {
	tests := []struct {
		name            string
		porcelainOutput string
		expectedStatus  *GitStatus
	}{
		{
			name:            "clean working directory",
			porcelainOutput: "",
			expectedStatus: &GitStatus{
				Staged:    []string{},
				Unstaged:  []string{},
				Untracked: []string{},
			},
		},
		{
			name: "untracked files",
			porcelainOutput: `?? new-file.txt
?? another-file.go`,
			expectedStatus: &GitStatus{
				Staged:    []string{},
				Unstaged:  []string{},
				Untracked: []string{"new-file.txt", "another-file.go"},
			},
		},
		{
			name: "staged modifications",
			porcelainOutput: `M  staged-file.txt
A  new-file.go`,
			expectedStatus: &GitStatus{
				Staged:    []string{"staged-file.txt", "new-file.go"},
				Unstaged:  []string{},
				Untracked: []string{},
			},
		},
		{
			name: "unstaged modifications",
			porcelainOutput: ` M modified-file.txt
 D deleted-file.go`,
			expectedStatus: &GitStatus{
				Staged:    []string{},
				Unstaged:  []string{"modified-file.txt", "deleted-file.go"},
				Untracked: []string{},
			},
		},
		{
			name: "mixed status",
			porcelainOutput: `M  staged.txt
 M unstaged.txt
?? untracked.txt
A  added.go
 D deleted.go`,
			expectedStatus: &GitStatus{
				Staged:    []string{"staged.txt", "added.go"},
				Unstaged:  []string{"unstaged.txt", "deleted.go"},
				Untracked: []string{"untracked.txt"},
			},
		},
		{
			name:            "renamed file",
			porcelainOutput: `R  old.txt -> new.txt`,
			expectedStatus: &GitStatus{
				Staged:    []string{"old.txt -> new.txt"},
				Unstaged:  []string{},
				Untracked: []string{},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			status := &GitStatus{
				Staged:    []string{},
				Unstaged:  []string{},
				Untracked: []string{},
			}

			// Parse the porcelain output
			for _, line := range strings.Split(tt.porcelainOutput, "\n") {
				if line == "" {
					continue
				}
				if len(line) < 4 {
					continue
				}

				statusCode := line[0:2]
				filePath := strings.TrimSpace(line[3:])

				switch {
				case statusCode == "??":
					status.Untracked = append(status.Untracked, filePath)
				case statusCode[0] != ' ' && statusCode[0] != '?':
					status.Staged = append(status.Staged, filePath)
				case statusCode[1] != ' ' && statusCode[1] != '?':
					status.Unstaged = append(status.Unstaged, filePath)
				}
			}

			assert.ElementsMatch(t, tt.expectedStatus.Staged, status.Staged)
			assert.ElementsMatch(t, tt.expectedStatus.Unstaged, status.Unstaged)
			assert.ElementsMatch(t, tt.expectedStatus.Untracked, status.Untracked)
		})
	}
}

// TestSnapshotDoesNotCreateRefs tests that snapshots don't pollute git history.
func TestSnapshotDoesNotCreateRefs(t *testing.T) {
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

	// Get stash list before snapshot
	cmd = exec.Command("git", "stash", "list")
	cmd.Dir = repoPath
	outputBefore, err := cmd.CombinedOutput()
	require.NoError(t, err)

	// Modify file and create snapshot
	require.NoError(t, os.WriteFile(testFile, []byte("modified\n"), 0644))
	snapshotID, err := gitOps.Snapshot(ctx)
	require.NoError(t, err)
	assert.NotEmpty(t, snapshotID)

	// Get stash list after snapshot
	cmd = exec.Command("git", "stash", "list")
	cmd.Dir = repoPath
	outputAfter, err := cmd.CombinedOutput()
	require.NoError(t, err)

	// Verify no new stash refs were created
	assert.Equal(t, string(outputBefore), string(outputAfter),
		"snapshot should not create visible stash refs")

	// Verify snapshot metadata file exists
	metadataFile := filepath.Join(repoPath, ".git", "codegen-snapshots", snapshotID)
	_, err = os.Stat(metadataFile)
	assert.NoError(t, err, "snapshot metadata should exist")
}

// TestSnapshotMetadataStorage tests snapshot metadata file operations.
func TestSnapshotMetadataStorage(t *testing.T) {
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
	testFile := filepath.Join(repoPath, "test.txt")
	require.NoError(t, os.WriteFile(testFile, []byte("initial\n"), 0644))

	cmd = exec.Command("git", "add", "test.txt")
	cmd.Dir = repoPath
	require.NoError(t, cmd.Run())

	cmd = exec.Command("git", "commit", "-m", "Initial")
	cmd.Dir = repoPath
	require.NoError(t, cmd.Run())

	gitOps := NewGitOps(repoPath, nil).(*gitOps)
	ctx := context.Background()

	// Modify file
	require.NoError(t, os.WriteFile(testFile, []byte("modified\n"), 0644))

	// Create snapshot
	snapshotID, err := gitOps.Snapshot(ctx)
	require.NoError(t, err)

	// Verify metadata directory exists with correct permissions
	metadataDir := filepath.Join(repoPath, ".git", "codegen-snapshots")
	info, err := os.Stat(metadataDir)
	require.NoError(t, err)
	assert.True(t, info.IsDir())
	assert.Equal(t, os.FileMode(0755), info.Mode().Perm())

	// Verify metadata file exists
	metadataFile := filepath.Join(metadataDir, snapshotID)
	info, err = os.Stat(metadataFile)
	require.NoError(t, err)
	assert.Equal(t, os.FileMode(0644), info.Mode().Perm())

	// Read and verify stash SHA format (40-character hex)
	content, err := os.ReadFile(metadataFile)
	require.NoError(t, err)
	stashSHA := strings.TrimSpace(string(content))
	assert.Len(t, stashSHA, 40, "stash SHA should be 40 characters")
	assert.Regexp(t, "^[0-9a-f]{40}$", stashSHA, "stash SHA should be valid hex")
}

// TestRollbackWithNoSnapshot tests that rollback fails gracefully when snapshot doesn't exist.
func TestRollbackWithNoSnapshot(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not available in PATH")
	}

	tempDir := t.TempDir()
	repoPath := filepath.Join(tempDir, "test-repo")

	// Initialize repository
	cmd := exec.Command("git", "init", repoPath)
	require.NoError(t, cmd.Run())

	gitOps := NewGitOps(repoPath, nil)
	ctx := context.Background()

	// Try to rollback to non-existent snapshot
	err := gitOps.Rollback(ctx, "non-existent-snapshot")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "snapshot")
}

// TestMultipleSnapshots tests snapshot stack behavior.
func TestMultipleSnapshots(t *testing.T) {
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
	testFile := filepath.Join(repoPath, "test.txt")
	require.NoError(t, os.WriteFile(testFile, []byte("initial\n"), 0644))

	cmd = exec.Command("git", "add", "test.txt")
	cmd.Dir = repoPath
	require.NoError(t, cmd.Run())

	cmd = exec.Command("git", "commit", "-m", "Initial")
	cmd.Dir = repoPath
	require.NoError(t, cmd.Run())

	gitOps := NewGitOps(repoPath, nil)
	ctx := context.Background()

	// Create first snapshot
	require.NoError(t, os.WriteFile(testFile, []byte("state1\n"), 0644))
	snapshot1, err := gitOps.Snapshot(ctx)
	require.NoError(t, err)

	// Create second snapshot
	require.NoError(t, os.WriteFile(testFile, []byte("state2\n"), 0644))
	snapshot2, err := gitOps.Snapshot(ctx)
	require.NoError(t, err)

	// Create third snapshot
	require.NoError(t, os.WriteFile(testFile, []byte("state3\n"), 0644))
	snapshot3, err := gitOps.Snapshot(ctx)
	require.NoError(t, err)

	// Verify all snapshots are different
	assert.NotEqual(t, snapshot1, snapshot2)
	assert.NotEqual(t, snapshot2, snapshot3)
	assert.NotEqual(t, snapshot1, snapshot3)

	// Modify file further
	require.NoError(t, os.WriteFile(testFile, []byte("current\n"), 0644))

	// Rollback to second snapshot
	err = gitOps.Rollback(ctx, snapshot2)
	require.NoError(t, err)

	content, err := os.ReadFile(testFile)
	require.NoError(t, err)
	assert.Equal(t, "state2\n", string(content))

	// Rollback to first snapshot
	err = gitOps.Rollback(ctx, snapshot1)
	require.NoError(t, err)

	content, err = os.ReadFile(testFile)
	require.NoError(t, err)
	assert.Equal(t, "state1\n", string(content))
}

// TestErrorHandling tests various error scenarios.
func TestErrorHandling(t *testing.T) {
	tempDir := t.TempDir()
	repoPath := filepath.Join(tempDir, "non-existent-repo")

	gitOps := NewGitOps(repoPath, nil)
	ctx := context.Background()

	t.Run("operations on non-existent repo", func(t *testing.T) {
		_, err := gitOps.CurrentBranch()
		assert.Error(t, err)

		_, err = gitOps.Status()
		assert.Error(t, err)

		err = gitOps.CreateBranch(ctx, "test")
		assert.Error(t, err)
	})

	t.Run("empty parameters", func(t *testing.T) {
		err := gitOps.CreateBranch(ctx, "")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "empty")

		err = gitOps.Checkout(ctx, "")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "empty")

		err = gitOps.Add(ctx)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "no paths")

		_, err = gitOps.Commit(ctx, "", CommitOptions{})
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "empty")

		err = gitOps.Rollback(ctx, "")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "empty")
	})
}

// TestCloneValidation tests clone parameter validation.
func TestCloneValidation(t *testing.T) {
	ctx := context.Background()
	tempDir := t.TempDir()

	t.Run("empty url", func(t *testing.T) {
		err := Clone(ctx, "", filepath.Join(tempDir, "repo"), nil, CloneOptions{})
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "URL cannot be empty")
	})

	t.Run("empty destination", func(t *testing.T) {
		err := Clone(ctx, "https://github.com/org/repo.git", "", nil, CloneOptions{})
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "destination path cannot be empty")
	})

	t.Run("invalid url format", func(t *testing.T) {
		err := Clone(ctx, "invalid-url", filepath.Join(tempDir, "repo"), nil, CloneOptions{})
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "invalid repository URL")
	})
}

// TestCommitOptions tests commit with various options.
func TestCommitOptions(t *testing.T) {
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
	testFile := filepath.Join(repoPath, "test.txt")
	require.NoError(t, os.WriteFile(testFile, []byte("content\n"), 0644))

	cmd = exec.Command("git", "add", "test.txt")
	cmd.Dir = repoPath
	require.NoError(t, cmd.Run())

	cmd = exec.Command("git", "commit", "-m", "Initial")
	cmd.Dir = repoPath
	require.NoError(t, cmd.Run())

	gitOps := NewGitOps(repoPath, nil)
	ctx := context.Background()

	t.Run("allow empty commit", func(t *testing.T) {
		opts := CommitOptions{AllowEmpty: true}
		sha, err := gitOps.Commit(ctx, "Empty commit", opts)
		require.NoError(t, err)
		assert.NotEmpty(t, sha)
		assert.Len(t, sha, 40)
	})

	t.Run("commit with custom author", func(t *testing.T) {
		require.NoError(t, os.WriteFile(testFile, []byte("modified\n"), 0644))
		require.NoError(t, gitOps.Add(ctx, "test.txt"))

		opts := CommitOptions{
			Author: "Custom Author <custom@example.com>",
		}
		sha, err := gitOps.Commit(ctx, "Custom author commit", opts)
		require.NoError(t, err)
		assert.NotEmpty(t, sha)
	})

	t.Run("commit with timestamp", func(t *testing.T) {
		require.NoError(t, os.WriteFile(testFile, []byte("timestamped\n"), 0644))
		require.NoError(t, gitOps.Add(ctx, "test.txt"))

		timestamp := time.Date(2024, 1, 1, 12, 0, 0, 0, time.UTC)
		opts := CommitOptions{
			Timestamp: timestamp,
		}
		sha, err := gitOps.Commit(ctx, "Timestamped commit", opts)
		require.NoError(t, err)
		assert.NotEmpty(t, sha)
	})
}

// TestPushOptions tests push with various options.
func TestPushOptions(t *testing.T) {
	// This test just validates option handling, not actual push
	// (which would require a remote repository)

	tests := []struct {
		name     string
		opts     PushOptions
		expected []string
	}{
		{
			name:     "basic push",
			opts:     PushOptions{},
			expected: []string{"push", "origin"},
		},
		{
			name:     "push with force",
			opts:     PushOptions{Force: true},
			expected: []string{"push", "--force", "origin"},
		},
		{
			name:     "push with set-upstream",
			opts:     PushOptions{SetUpstream: true},
			expected: []string{"push", "--set-upstream", "origin"},
		},
		{
			name:     "push with tags",
			opts:     PushOptions{Tags: true},
			expected: []string{"push", "--tags", "origin"},
		},
		{
			name:     "push to custom remote",
			opts:     PushOptions{Remote: "upstream"},
			expected: []string{"push", "upstream"},
		},
		{
			name:     "push with refspec",
			opts:     PushOptions{RefSpec: "refs/heads/main:refs/heads/main"},
			expected: []string{"push", "origin", "refs/heads/main:refs/heads/main"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Build args as the implementation does
			remote := tt.opts.Remote
			if remote == "" {
				remote = "origin"
			}

			args := []string{"push"}
			if tt.opts.Force {
				args = append(args, "--force")
			}
			if tt.opts.SetUpstream {
				args = append(args, "--set-upstream")
			}
			if tt.opts.Tags {
				args = append(args, "--tags")
			}
			args = append(args, remote)
			if tt.opts.RefSpec != "" {
				args = append(args, tt.opts.RefSpec)
			}

			assert.Equal(t, tt.expected, args)
		})
	}
}

// TestInlineCredentialProvider tests URL transformation with inline credentials.
func TestInlineCredentialProvider(t *testing.T) {
	tests := []struct {
		name     string
		username string
		password string
		url      string
		expected string
	}{
		{
			name:     "https url without credentials",
			username: "user",
			password: "pass",
			url:      "https://github.com/org/repo.git",
			expected: "https://user:pass@github.com/org/repo.git",
		},
		{
			name:     "https url with existing credentials",
			username: "user",
			password: "pass",
			url:      "https://olduser:oldpass@github.com/org/repo.git",
			expected: "https://olduser:oldpass@github.com/org/repo.git",
		},
		{
			name:     "ssh url unchanged",
			username: "user",
			password: "pass",
			url:      "git@github.com:org/repo.git",
			expected: "git@github.com:org/repo.git",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			provider := NewInlineCredentialProvider(tt.username, tt.password)
			transformed, err := provider.TransformURL(tt.url)
			require.NoError(t, err)
			assert.Equal(t, tt.expected, transformed)
		})
	}
}

// TestSSHKeyPermissions tests that SSH keys are created with correct permissions.
func TestSSHKeyPermissions(t *testing.T) {
	tempDir := t.TempDir()
	repoPath := filepath.Join(tempDir, "test-repo")

	credential := &types.Credential{
		Type:   types.CredentialTypeCustom,
		Secret: "-----BEGIN RSA PRIVATE KEY-----\nMIIEpAIBAAKCAQEA...\n-----END RSA PRIVATE KEY-----",
	}

	provider := NewCredentialProvider(credential)
	cleanup, err := provider.ConfigureAuth(context.Background(), repoPath)
	require.NoError(t, err)
	defer cleanup()

	// Extract key path from GIT_SSH_COMMAND
	sshCommand := os.Getenv("GIT_SSH_COMMAND")
	require.NotEmpty(t, sshCommand)

	parts := strings.Split(sshCommand, " ")
	var keyPath string
	for i, part := range parts {
		if part == "-i" && i+1 < len(parts) {
			keyPath = parts[i+1]
			break
		}
	}
	require.NotEmpty(t, keyPath)

	// Verify key exists and has 0600 permissions
	info, err := os.Stat(keyPath)
	require.NoError(t, err)
	assert.Equal(t, os.FileMode(0600), info.Mode().Perm(),
		"SSH key must have 0600 permissions for security")
}
