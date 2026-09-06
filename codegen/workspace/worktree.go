// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Zero Root AI

package workspace

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/zeroroot-ai/sdk/codegen/git"
	"github.com/zeroroot-ai/sdk/codegen/lsp"
)

// CreateWorktree creates a Git worktree for agent isolation.
// Worktrees allow multiple agents to work on the same repository concurrently
// by creating separate working directories that share the same object store.
//
// This is more efficient than cloning multiple times because:
// - All worktrees share the same .git/objects directory
// - Reduces disk space (no duplicate objects)
// - Faster creation (no need to fetch from remote)
//
// Returns a new Workspace instance for the worktree.
func (m *workspaceManager) CreateWorktree(ctx context.Context, repoName, branchName, agentID string) (Workspace, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Get the base repository workspace
	baseWs, ok := m.workspaces[repoName]
	if !ok {
		return nil, fmt.Errorf("repository %s not found", repoName)
	}

	if baseWs.isWorktree {
		return nil, errors.New("cannot create worktree from another worktree")
	}

	// Validate branch name
	if branchName == "" {
		return nil, errors.New("branch name cannot be empty")
	}

	// Validate agent ID
	if agentID == "" {
		return nil, errors.New("agent ID cannot be empty")
	}

	// Create worktree directory name
	worktreeName := fmt.Sprintf("%s-worktree-%s", repoName, agentID)
	worktreePath := filepath.Join(m.config.Settings.BaseDirectory, worktreeName)

	// Check if worktree directory already exists
	if _, err := os.Stat(worktreePath); err == nil {
		return nil, fmt.Errorf("worktree directory already exists: %s", worktreePath)
	}

	m.logger.Info("creating worktree",
		"repository", repoName,
		"branch", branchName,
		"agent", agentID,
		"path", worktreePath)

	// Create the worktree using git worktree add
	if err := createGitWorktree(ctx, baseWs.path, worktreePath, branchName); err != nil {
		return nil, fmt.Errorf("failed to create worktree: %w", err)
	}

	m.logger.Info("successfully created worktree",
		"repository", repoName,
		"path", worktreePath)

	// Create workspace instance for the worktree
	// The worktree shares the same credential provider as the base workspace
	ws := &workspaceImpl{
		name:       fmt.Sprintf("%s:%s", repoName, agentID),
		path:       worktreePath,
		gitOps:     git.NewGitOps(worktreePath, baseWs.credProv),
		credProv:   baseWs.credProv,
		logger:     m.logger.With("workspace", worktreeName),
		isWorktree: true,
	}

	// Initialize LSP for the worktree if enabled
	if m.config.Settings.LSPEnabled {
		if err := m.initializeWorktreeLSP(ctx, ws); err != nil {
			// Log warning but don't fail - LSP is optional
			m.logger.Warn("failed to initialize LSP for worktree",
				"worktree", worktreeName,
				"error", err)
		}
	}

	// Store worktree in workspaces map
	m.workspaces[ws.name] = ws

	return ws, nil
}

// initializeWorktreeLSP starts an LSP server for a worktree workspace.
func (m *workspaceManager) initializeWorktreeLSP(ctx context.Context, ws *workspaceImpl) error {
	lspConfig := lsp.LSPConfig{
		InitTimeout:       m.config.Settings.LSPTimeout,
		ValidationTimeout: m.config.Settings.LSPTimeout,
		EnableGo:          true,
		EnablePython:      true,
		EnableTypeScript:  true,
	}

	lspMgr := lsp.NewLSPManager(lspConfig, m.logger)

	if err := lspMgr.Start(ctx, ws.path); err != nil {
		return fmt.Errorf("failed to start LSP: %w", err)
	}

	// Wait for LSP to be ready
	if err := lspMgr.WaitForReady(ctx); err != nil {
		lspMgr.Stop(ctx)
		return fmt.Errorf("LSP failed to become ready: %w", err)
	}

	m.lspManagers[ws.name] = lspMgr
	ws.lspManager = lspMgr

	m.logger.Info("initialized LSP for worktree",
		"worktree", ws.name,
		"languages", lspMgr.SupportedLanguages())

	return nil
}

// RemoveWorktree removes a Git worktree and cleans up its resources.
func (m *workspaceManager) RemoveWorktree(ctx context.Context, worktreeName string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	ws, ok := m.workspaces[worktreeName]
	if !ok {
		return fmt.Errorf("worktree %s not found", worktreeName)
	}

	if !ws.isWorktree {
		return fmt.Errorf("%s is not a worktree", worktreeName)
	}

	m.logger.Info("removing worktree", "name", worktreeName, "path", ws.path)

	// Stop LSP server if running
	if lspMgr, ok := m.lspManagers[worktreeName]; ok {
		if err := lspMgr.Stop(ctx); err != nil {
			m.logger.Warn("failed to stop LSP for worktree",
				"worktree", worktreeName,
				"error", err)
		}
		delete(m.lspManagers, worktreeName)
	}

	// Extract base repository name from worktree name (format: "repo:agentID")
	parts := strings.SplitN(worktreeName, ":", 2)
	if len(parts) != 2 {
		return fmt.Errorf("invalid worktree name format: %s", worktreeName)
	}
	baseRepoName := parts[0]

	// Get base repository to run git worktree remove
	baseWs, ok := m.workspaces[baseRepoName]
	if !ok {
		// Base repository not found, manually remove directory
		m.logger.Warn("base repository not found, manually removing worktree directory",
			"base_repo", baseRepoName,
			"worktree", worktreeName)

		if err := os.RemoveAll(ws.path); err != nil {
			return fmt.Errorf("failed to remove worktree directory: %w", err)
		}
	} else {
		// Use git worktree remove for clean removal
		if err := removeGitWorktree(ctx, baseWs.path, ws.path); err != nil {
			m.logger.Warn("git worktree remove failed, manually removing directory",
				"worktree", worktreeName,
				"error", err)

			// Fallback to manual removal
			if err := os.RemoveAll(ws.path); err != nil {
				return fmt.Errorf("failed to remove worktree directory: %w", err)
			}
		}
	}

	// Remove from workspaces map
	delete(m.workspaces, worktreeName)

	m.logger.Info("successfully removed worktree", "name", worktreeName)

	return nil
}

// createGitWorktree creates a new git worktree at the specified path.
// If the branch doesn't exist, it will be created from the current HEAD.
func createGitWorktree(ctx context.Context, repoPath, worktreePath, branchName string) error {
	// Check if branch exists
	checkBranchCmd := exec.CommandContext(ctx, "git", "rev-parse", "--verify", branchName)
	checkBranchCmd.Dir = repoPath
	branchExists := checkBranchCmd.Run() == nil

	var args []string
	if branchExists {
		// Branch exists, check it out in the worktree
		args = []string{"worktree", "add", worktreePath, branchName}
	} else {
		// Branch doesn't exist, create it
		args = []string{"worktree", "add", "-b", branchName, worktreePath}
	}

	// Create the worktree
	cmd := exec.CommandContext(ctx, "git", args...)
	cmd.Dir = repoPath

	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("git worktree add failed: %w (output: %s)",
			err, strings.TrimSpace(string(output)))
	}

	return nil
}

// removeGitWorktree removes a git worktree at the specified path.
func removeGitWorktree(ctx context.Context, repoPath, worktreePath string) error {
	// First, try to remove the worktree using git worktree remove
	cmd := exec.CommandContext(ctx, "git", "worktree", "remove", worktreePath)
	cmd.Dir = repoPath

	output, err := cmd.CombinedOutput()
	if err != nil {
		// If the worktree has uncommitted changes, git will refuse to remove it
		// Try with --force flag
		cmdForce := exec.CommandContext(ctx, "git", "worktree", "remove", "--force", worktreePath)
		cmdForce.Dir = repoPath

		outputForce, errForce := cmdForce.CombinedOutput()
		if errForce != nil {
			return fmt.Errorf("git worktree remove failed: %w (output: %s, force output: %s)",
				err, strings.TrimSpace(string(output)), strings.TrimSpace(string(outputForce)))
		}
	}

	return nil
}

// ListWorktrees returns a list of all active worktrees for a repository.
func (m *workspaceManager) ListWorktrees(repoName string) ([]string, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	// Get base repository
	baseWs, ok := m.workspaces[repoName]
	if !ok {
		return nil, fmt.Errorf("repository %s not found", repoName)
	}

	if baseWs.isWorktree {
		return nil, fmt.Errorf("%s is a worktree, not a base repository", repoName)
	}

	// List all worktrees using git worktree list
	cmd := exec.Command("git", "worktree", "list", "--porcelain")
	cmd.Dir = baseWs.path

	output, err := cmd.CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("failed to list worktrees: %w (output: %s)",
			err, strings.TrimSpace(string(output)))
	}

	// Parse worktree list output
	// Format:
	// worktree /path/to/worktree
	// HEAD <sha>
	// branch refs/heads/branch-name
	//
	// worktree /path/to/another
	// ...

	worktrees := make([]string, 0)
	lines := strings.Split(string(output), "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "worktree ") {
			worktreePath := strings.TrimPrefix(line, "worktree ")

			// Skip the main repository (it's also listed as a worktree)
			if worktreePath == baseWs.path {
				continue
			}

			worktrees = append(worktrees, worktreePath)
		}
	}

	return worktrees, nil
}

// GetWorktreeInfo returns information about a worktree.
type WorktreeInfo struct {
	Path   string
	Branch string
	HEAD   string
}

// GetWorktreeInfo retrieves information about a specific worktree.
func (m *workspaceManager) GetWorktreeInfo(repoName, worktreePath string) (*WorktreeInfo, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	// Get base repository
	baseWs, ok := m.workspaces[repoName]
	if !ok {
		return nil, fmt.Errorf("repository %s not found", repoName)
	}

	// List all worktrees and find the matching one
	cmd := exec.Command("git", "worktree", "list", "--porcelain")
	cmd.Dir = baseWs.path

	output, err := cmd.CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("failed to list worktrees: %w", err)
	}

	// Parse output to find the specific worktree
	lines := strings.Split(string(output), "\n")
	var info *WorktreeInfo

	for i := range lines {
		line := strings.TrimSpace(lines[i])

		if strings.HasPrefix(line, "worktree ") {
			path := strings.TrimPrefix(line, "worktree ")

			if path == worktreePath {
				info = &WorktreeInfo{Path: path}

				// Read next lines for HEAD and branch
				for j := i + 1; j < len(lines) && j < i+5; j++ {
					nextLine := strings.TrimSpace(lines[j])

					if strings.HasPrefix(nextLine, "HEAD ") {
						info.HEAD = strings.TrimPrefix(nextLine, "HEAD ")
					} else if strings.HasPrefix(nextLine, "branch ") {
						branchRef := strings.TrimPrefix(nextLine, "branch ")
						// Extract branch name from refs/heads/branch-name
						if strings.HasPrefix(branchRef, "refs/heads/") {
							info.Branch = strings.TrimPrefix(branchRef, "refs/heads/")
						} else {
							info.Branch = branchRef
						}
					} else if nextLine == "" {
						break
					}
				}

				break
			}
		}
	}

	if info == nil {
		return nil, fmt.Errorf("worktree %s not found", worktreePath)
	}

	return info, nil
}
