// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Zero Root AI

//go:build integration

// Copyright (c) 2026 ZeroRoot
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package codegen_test

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/zeroroot-ai/sdk/codegen"
	"github.com/zeroroot-ai/sdk/codegen/editor"
	"github.com/zeroroot-ai/sdk/codegen/git"
	"github.com/zeroroot-ai/sdk/codegen/lsp"
	"github.com/zeroroot-ai/sdk/codegen/workspace"
	"github.com/zeroroot-ai/sdk/types"
)

// mockCredStore is a simple credential store for testing.
type mockCredStore struct {
	creds map[string]*types.Credential
}

func newMockCredStore() *mockCredStore {
	return &mockCredStore{
		creds: make(map[string]*types.Credential),
	}
}

func (m *mockCredStore) Set(name string, cred *types.Credential) {
	m.creds[name] = cred
}

func (m *mockCredStore) Get(name string) (*types.Credential, error) {
	cred, ok := m.creds[name]
	if !ok {
		return nil, fmt.Errorf("credential not found: %s", name)
	}
	return cred, nil
}

// initGitRepo initializes a Git repository with a basic configuration.
func initGitRepo(t *testing.T, path string) {
	t.Helper()

	cmd := exec.Command("git", "init")
	cmd.Dir = path
	require.NoError(t, cmd.Run(), "git init failed")

	cmd = exec.Command("git", "config", "user.name", "Test User")
	cmd.Dir = path
	require.NoError(t, cmd.Run(), "git config user.name failed")

	cmd = exec.Command("git", "config", "user.email", "test@example.com")
	cmd.Dir = path
	require.NoError(t, cmd.Run(), "git config user.email failed")

	// Create default branch explicitly (for Git 2.28+)
	cmd = exec.Command("git", "checkout", "-b", "main")
	cmd.Dir = path
	_ = cmd.Run() // Ignore error if branch already exists
}

// createInitialCommit creates an initial commit with a README file.
func createInitialCommit(t *testing.T, repoPath string) {
	t.Helper()

	readmePath := filepath.Join(repoPath, "README.md")
	require.NoError(t, os.WriteFile(readmePath, []byte("# Test Repository\n"), 0644))

	cmd := exec.Command("git", "add", "README.md")
	cmd.Dir = repoPath
	require.NoError(t, cmd.Run(), "git add failed")

	cmd = exec.Command("git", "commit", "-m", "Initial commit")
	cmd.Dir = repoPath
	require.NoError(t, cmd.Run(), "git commit failed")
}

// verifyCommitExists checks if a commit SHA exists in the repository.
func verifyCommitExists(t *testing.T, repoPath, commitSHA string) bool {
	t.Helper()

	cmd := exec.Command("git", "cat-file", "-t", commitSHA)
	cmd.Dir = repoPath
	output, err := cmd.CombinedOutput()
	if err != nil {
		return false
	}
	return strings.TrimSpace(string(output)) == "commit"
}

// getCommitMessage returns the commit message for a given SHA.
func getCommitMessage(t *testing.T, repoPath, commitSHA string) string {
	t.Helper()

	cmd := exec.Command("git", "log", "-1", "--format=%B", commitSHA)
	cmd.Dir = repoPath
	output, err := cmd.CombinedOutput()
	require.NoError(t, err, "git log failed")
	return strings.TrimSpace(string(output))
}

// checkCommandAvailable checks if a command is available in PATH.
func checkCommandAvailable(command string) bool {
	_, err := exec.LookPath(command)
	return err == nil
}

// TestFullMission tests the complete mission: clone, edit, validate, commit.
func TestFullMission(t *testing.T) {
	if !checkCommandAvailable("git") {
		t.Skip("git not available in PATH")
	}

	ctx := context.Background()
	tmpDir := t.TempDir()

	// Create a test repository to clone
	sourceRepo := filepath.Join(tmpDir, "source-repo")
	require.NoError(t, os.MkdirAll(sourceRepo, 0755))
	initGitRepo(t, sourceRepo)

	// Create a Go file with a bug
	goFile := filepath.Join(sourceRepo, "main.go")
	buggyCode := `package main

import "fmt"

func calculate(x int) int {
	return x * 2  // Bug: should be * 3
}

func main() {
	result := calculate(5)
	fmt.Println("Result:", result)
}
`
	require.NoError(t, os.WriteFile(goFile, []byte(buggyCode), 0644))

	cmd := exec.Command("git", "add", "main.go")
	cmd.Dir = sourceRepo
	require.NoError(t, cmd.Run())

	cmd = exec.Command("git", "commit", "-m", "Add buggy calculator")
	cmd.Dir = sourceRepo
	require.NoError(t, cmd.Run())

	// Create workspace manager
	credStore := newMockCredStore()
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelDebug}))
	wm := workspace.NewWorkspaceManager(credStore, logger)

	// Initialize workspace with the test repository (local path works for testing)
	config := workspace.WorkspaceConfig{
		Repositories: []workspace.RepositoryConfig{
			{
				Name:   "test-repo",
				URL:    sourceRepo,
				Branch: "main",
			},
		},
		Settings: workspace.WorkspaceSettings{
			BaseDirectory:     filepath.Join(tmpDir, "workspaces"),
			CleanupOnComplete: true,
			LSPEnabled:        false, // Disable LSP for basic mission test
		},
	}

	require.NoError(t, wm.Initialize(ctx, config), "workspace initialization failed")
	defer wm.Cleanup(ctx)

	// Get the workspace
	ws := wm.Primary()
	require.NotNil(t, ws, "primary workspace should not be nil")
	assert.Equal(t, "test-repo", ws.Name())

	// Read the buggy file
	content, err := ws.ReadFile(ctx, "main.go")
	require.NoError(t, err)
	assert.Contains(t, string(content), "x * 2")

	// Apply a fix using the editor
	gitOps := git.NewGitOps(ws.Path(), nil)
	ed := editor.NewEditor(ws.Path(), gitOps, nil)

	edit := editor.Edit{
		FilePath: "main.go",
		SearchBlock: `func calculate(x int) int {
	return x * 2  // Bug: should be * 3
}`,
		ReplaceBlock: `func calculate(x int) int {
	return x * 3  // Fixed: multiplier changed to 3
}`,
		Description: "Fix calculation bug",
	}

	result, err := ed.Apply(ctx, edit)
	require.NoError(t, err, "edit apply failed")
	assert.True(t, result.Applied, "edit should be applied")
	assert.Equal(t, codegen.MatchExact, result.MatchType, "should use exact match")

	// Verify file was modified
	modifiedContent, err := ws.ReadFile(ctx, "main.go")
	require.NoError(t, err)
	assert.Contains(t, string(modifiedContent), "x * 3")
	assert.Contains(t, string(modifiedContent), "Fixed: multiplier changed to 3")

	// Stage and commit the changes
	require.NoError(t, gitOps.Add(ctx, "main.go"))

	commitSHA, err := gitOps.Commit(ctx, "Fix: correct calculation multiplier", git.CommitOptions{
		Author: "Test User <test@example.com>",
	})
	require.NoError(t, err, "commit failed")
	assert.NotEmpty(t, commitSHA, "commit SHA should not be empty")
	assert.Len(t, commitSHA, 40, "commit SHA should be 40 characters")

	// Verify commit exists in history
	assert.True(t, verifyCommitExists(t, ws.Path(), commitSHA), "commit should exist")

	// Verify commit message
	commitMsg := getCommitMessage(t, ws.Path(), commitSHA)
	assert.Contains(t, commitMsg, "Fix: correct calculation multiplier")

	t.Logf("Successfully completed full mission with commit SHA: %s", commitSHA)
}

// TestGoCodeWithLSP tests Go code editing with gopls validation.
func TestGoCodeWithLSP(t *testing.T) {
	if !checkCommandAvailable("git") {
		t.Skip("git not available in PATH")
	}
	if !checkCommandAvailable("gopls") {
		t.Skip("gopls not available in PATH")
	}

	ctx := context.Background()
	tmpDir := t.TempDir()

	// Create a test Go repository
	sourceRepo := filepath.Join(tmpDir, "go-repo")
	require.NoError(t, os.MkdirAll(sourceRepo, 0755))
	initGitRepo(t, sourceRepo)

	// Create go.mod
	goMod := filepath.Join(sourceRepo, "go.mod")
	require.NoError(t, os.WriteFile(goMod, []byte("module testmod\n\ngo 1.21\n"), 0644))

	// Create Go file with syntax error
	goFile := filepath.Join(sourceRepo, "main.go")
	invalidCode := `package main

import "fmt"

func broken() {
	fmt.Println("Missing closing brace"
// Syntax error: missing }

func main() {
	broken()
}
`
	require.NoError(t, os.WriteFile(goFile, []byte(invalidCode), 0644))

	cmd := exec.Command("git", "add", ".")
	cmd.Dir = sourceRepo
	require.NoError(t, cmd.Run())

	cmd = exec.Command("git", "commit", "-m", "Add broken code")
	cmd.Dir = sourceRepo
	require.NoError(t, cmd.Run())

	// Create workspace with LSP enabled
	credStore := newMockCredStore()
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelWarn}))
	wm := workspace.NewWorkspaceManager(credStore, logger)

	config := workspace.WorkspaceConfig{
		Repositories: []workspace.RepositoryConfig{
			{
				Name:   "go-repo",
				URL:    sourceRepo,
				Branch: "main",
			},
		},
		Settings: workspace.WorkspaceSettings{
			BaseDirectory:     filepath.Join(tmpDir, "workspaces"),
			CleanupOnComplete: true,
			LSPEnabled:        true,
			LSPTimeout:        30 * time.Second,
		},
	}

	require.NoError(t, wm.Initialize(ctx, config), "workspace initialization failed")
	defer wm.Cleanup(ctx)

	ws := wm.Primary()
	require.NotNil(t, ws)

	// Create LSP manager
	lspConfig := lsp.LSPConfig{
		InitTimeout:       30 * time.Second,
		ValidationTimeout: 10 * time.Second,
		EnableGo:          true,
		EnablePython:      false,
		EnableTypeScript:  false,
	}
	lspMgr := lsp.NewLSPManager(lspConfig, logger)
	require.NoError(t, lspMgr.Start(ctx, ws.Path()))
	defer lspMgr.Stop(ctx)
	require.NoError(t, lspMgr.WaitForReady(ctx))

	// Create editor with LSP
	gitOps := git.NewGitOps(ws.Path(), nil)
	ed := editor.NewEditor(ws.Path(), gitOps, lspMgr)

	// Apply edit that fixes the syntax error
	edit := editor.Edit{
		FilePath: "main.go",
		SearchBlock: `func broken() {
	fmt.Println("Missing closing brace"
// Syntax error: missing }`,
		ReplaceBlock: `func broken() {
	fmt.Println("Fixed - added closing brace")
}`,
		Description: "Fix syntax error",
	}

	result, err := ed.Apply(ctx, edit)
	require.NoError(t, err)
	assert.True(t, result.Applied, "edit should be applied")

	// Verify no errors (warnings are acceptable)
	assert.False(t, result.HasErrors(), "should not have errors after fix")

	t.Logf("Successfully fixed Go syntax error with LSP validation")
}

// TestPythonCodeWithLSP tests Python code editing with pyright validation.
func TestPythonCodeWithLSP(t *testing.T) {
	if !checkCommandAvailable("git") {
		t.Skip("git not available in PATH")
	}
	if !checkCommandAvailable("pyright-langserver") {
		t.Skip("pyright-langserver not available in PATH")
	}

	ctx := context.Background()
	tmpDir := t.TempDir()

	// Create a test Python repository
	sourceRepo := filepath.Join(tmpDir, "py-repo")
	require.NoError(t, os.MkdirAll(sourceRepo, 0755))
	initGitRepo(t, sourceRepo)

	// Create Python file with SQL injection vulnerability pattern
	pyFile := filepath.Join(sourceRepo, "app.py")
	vulnerableCode := `import sqlite3

def get_user(username):
    conn = sqlite3.connect("users.db")
    cursor = conn.cursor()
    # SQL injection vulnerability
    query = "SELECT * FROM users WHERE username = '" + username + "'"
    cursor.execute(query)
    return cursor.fetchone()

def main():
    user = get_user("admin")
    print(user)

if __name__ == "__main__":
    main()
`
	require.NoError(t, os.WriteFile(pyFile, []byte(vulnerableCode), 0644))

	cmd := exec.Command("git", "add", "app.py")
	cmd.Dir = sourceRepo
	require.NoError(t, cmd.Run())

	cmd = exec.Command("git", "commit", "-m", "Add vulnerable code")
	cmd.Dir = sourceRepo
	require.NoError(t, cmd.Run())

	// Create workspace with LSP enabled
	credStore := newMockCredStore()
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelWarn}))
	wm := workspace.NewWorkspaceManager(credStore, logger)

	config := workspace.WorkspaceConfig{
		Repositories: []workspace.RepositoryConfig{
			{
				Name:   "py-repo",
				URL:    sourceRepo,
				Branch: "main",
			},
		},
		Settings: workspace.WorkspaceSettings{
			BaseDirectory:     filepath.Join(tmpDir, "workspaces"),
			CleanupOnComplete: true,
			LSPEnabled:        true,
			LSPTimeout:        30 * time.Second,
		},
	}

	require.NoError(t, wm.Initialize(ctx, config))
	defer wm.Cleanup(ctx)

	ws := wm.Primary()
	require.NotNil(t, ws)

	// Create LSP manager
	lspConfig := lsp.LSPConfig{
		InitTimeout:       30 * time.Second,
		ValidationTimeout: 10 * time.Second,
		EnableGo:          false,
		EnablePython:      true,
		EnableTypeScript:  false,
	}
	lspMgr := lsp.NewLSPManager(lspConfig, logger)
	require.NoError(t, lspMgr.Start(ctx, ws.Path()))
	defer lspMgr.Stop(ctx)
	require.NoError(t, lspMgr.WaitForReady(ctx))

	// Create editor with LSP
	gitOps := git.NewGitOps(ws.Path(), nil)
	ed := editor.NewEditor(ws.Path(), gitOps, lspMgr)

	// Apply edit that fixes SQL injection
	edit := editor.Edit{
		FilePath: "app.py",
		SearchBlock: `def get_user(username):
    conn = sqlite3.connect("users.db")
    cursor = conn.cursor()
    # SQL injection vulnerability
    query = "SELECT * FROM users WHERE username = '" + username + "'"
    cursor.execute(query)
    return cursor.fetchone()`,
		ReplaceBlock: `def get_user(username):
    conn = sqlite3.connect("users.db")
    cursor = conn.cursor()
    # Fixed: using parameterized query
    query = "SELECT * FROM users WHERE username = ?"
    cursor.execute(query, (username,))
    return cursor.fetchone()`,
		Description: "Fix SQL injection vulnerability",
	}

	result, err := ed.Apply(ctx, edit)
	require.NoError(t, err)
	assert.True(t, result.Applied, "edit should be applied")

	// Verify file was modified
	content, err := ws.ReadFile(ctx, "app.py")
	require.NoError(t, err)
	assert.Contains(t, string(content), "parameterized query")
	assert.Contains(t, string(content), "cursor.execute(query, (username,))")

	t.Logf("Successfully fixed Python SQL injection with LSP validation")
}

// TestTypeScriptCodeWithLSP tests TypeScript code editing with tsserver validation.
func TestTypeScriptCodeWithLSP(t *testing.T) {
	if !checkCommandAvailable("git") {
		t.Skip("git not available in PATH")
	}
	if !checkCommandAvailable("typescript-language-server") {
		t.Skip("typescript-language-server not available in PATH")
	}

	ctx := context.Background()
	tmpDir := t.TempDir()

	// Create a test TypeScript repository
	sourceRepo := filepath.Join(tmpDir, "ts-repo")
	require.NoError(t, os.MkdirAll(sourceRepo, 0755))
	initGitRepo(t, sourceRepo)

	// Create package.json
	packageJSON := filepath.Join(sourceRepo, "package.json")
	require.NoError(t, os.WriteFile(packageJSON, []byte(`{
  "name": "test-app",
  "version": "1.0.0",
  "dependencies": {}
}
`), 0644))

	// Create TypeScript file with XSS vulnerability pattern
	tsFile := filepath.Join(sourceRepo, "app.ts")
	vulnerableCode := `export function renderUser(username: string): string {
  // XSS vulnerability: direct HTML injection
  return "<div>Welcome " + username + "!</div>";
}

export function displayUser(username: string): void {
  const html = renderUser(username);
  document.body.innerHTML = html;
}
`
	require.NoError(t, os.WriteFile(tsFile, []byte(vulnerableCode), 0644))

	cmd := exec.Command("git", "add", ".")
	cmd.Dir = sourceRepo
	require.NoError(t, cmd.Run())

	cmd = exec.Command("git", "commit", "-m", "Add vulnerable code")
	cmd.Dir = sourceRepo
	require.NoError(t, cmd.Run())

	// Create workspace with LSP enabled
	credStore := newMockCredStore()
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelWarn}))
	wm := workspace.NewWorkspaceManager(credStore, logger)

	config := workspace.WorkspaceConfig{
		Repositories: []workspace.RepositoryConfig{
			{
				Name:   "ts-repo",
				URL:    sourceRepo,
				Branch: "main",
			},
		},
		Settings: workspace.WorkspaceSettings{
			BaseDirectory:     filepath.Join(tmpDir, "workspaces"),
			CleanupOnComplete: true,
			LSPEnabled:        true,
			LSPTimeout:        30 * time.Second,
		},
	}

	require.NoError(t, wm.Initialize(ctx, config))
	defer wm.Cleanup(ctx)

	ws := wm.Primary()
	require.NotNil(t, ws)

	// Create LSP manager
	lspConfig := lsp.LSPConfig{
		InitTimeout:       30 * time.Second,
		ValidationTimeout: 10 * time.Second,
		EnableGo:          false,
		EnablePython:      false,
		EnableTypeScript:  true,
	}
	lspMgr := lsp.NewLSPManager(lspConfig, logger)
	require.NoError(t, lspMgr.Start(ctx, ws.Path()))
	defer lspMgr.Stop(ctx)
	require.NoError(t, lspMgr.WaitForReady(ctx))

	// Create editor with LSP
	gitOps := git.NewGitOps(ws.Path(), nil)
	ed := editor.NewEditor(ws.Path(), gitOps, lspMgr)

	// Apply edit that fixes XSS vulnerability
	edit := editor.Edit{
		FilePath: "app.ts",
		SearchBlock: `export function renderUser(username: string): string {
  // XSS vulnerability: direct HTML injection
  return "<div>Welcome " + username + "!</div>";
}`,
		ReplaceBlock: `export function renderUser(username: string): string {
  // Fixed: escape HTML to prevent XSS
  const escaped = username.replace(/[&<>"']/g, (char) => {
    const escapeMap: { [key: string]: string } = {
      '&': '&amp;',
      '<': '&lt;',
      '>': '&gt;',
      '"': '&quot;',
      "'": '&#39;'
    };
    return escapeMap[char];
  });
  return "<div>Welcome " + escaped + "!</div>";
}`,
		Description: "Fix XSS vulnerability",
	}

	result, err := ed.Apply(ctx, edit)
	require.NoError(t, err)
	assert.True(t, result.Applied, "edit should be applied")

	// Verify file was modified
	content, err := ws.ReadFile(ctx, "app.ts")
	require.NoError(t, err)
	assert.Contains(t, string(content), "escape HTML to prevent XSS")
	assert.Contains(t, string(content), "escapeMap")

	t.Logf("Successfully fixed TypeScript XSS vulnerability with LSP validation")
}

// TestMultiRepoScenario tests working with multiple repositories.
func TestMultiRepoScenario(t *testing.T) {
	if !checkCommandAvailable("git") {
		t.Skip("git not available in PATH")
	}

	ctx := context.Background()
	tmpDir := t.TempDir()

	// Create two test repositories
	repo1 := filepath.Join(tmpDir, "repo1")
	repo2 := filepath.Join(tmpDir, "repo2")

	for _, repo := range []string{repo1, repo2} {
		require.NoError(t, os.MkdirAll(repo, 0755))
		initGitRepo(t, repo)
		createInitialCommit(t, repo)
	}

	// Add files to each repo
	file1 := filepath.Join(repo1, "config.txt")
	file2 := filepath.Join(repo2, "settings.txt")

	require.NoError(t, os.WriteFile(file1, []byte("config=value1\n"), 0644))
	require.NoError(t, os.WriteFile(file2, []byte("setting=value2\n"), 0644))

	for _, repo := range []string{repo1, repo2} {
		cmd := exec.Command("git", "add", ".")
		cmd.Dir = repo
		require.NoError(t, cmd.Run())

		cmd = exec.Command("git", "commit", "-m", "Add config")
		cmd.Dir = repo
		require.NoError(t, cmd.Run())
	}

	// Create workspace manager with both repos
	credStore := newMockCredStore()
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelWarn}))
	wm := workspace.NewWorkspaceManager(credStore, logger)

	config := workspace.WorkspaceConfig{
		Repositories: []workspace.RepositoryConfig{
			{
				Name:   "repo1",
				URL:    repo1,
				Branch: "main",
			},
			{
				Name:   "repo2",
				URL:    repo2,
				Branch: "main",
			},
		},
		Settings: workspace.WorkspaceSettings{
			BaseDirectory:     filepath.Join(tmpDir, "workspaces"),
			CleanupOnComplete: true,
			LSPEnabled:        false,
		},
	}

	require.NoError(t, wm.Initialize(ctx, config))
	defer wm.Cleanup(ctx)

	// Verify both workspaces are accessible
	ws1, ok := wm.Get("repo1")
	require.True(t, ok, "repo1 should be accessible")
	require.NotNil(t, ws1)

	ws2, ok := wm.Get("repo2")
	require.True(t, ok, "repo2 should be accessible")
	require.NotNil(t, ws2)

	// Verify all workspaces
	allWorkspaces := wm.All()
	assert.Len(t, allWorkspaces, 2, "should have 2 workspaces")

	// Apply edits to both repos
	for i, ws := range []workspace.Workspace{ws1, ws2} {
		fileName := fmt.Sprintf("config.txt")
		if i == 1 {
			fileName = "settings.txt"
		}

		gitOps := git.NewGitOps(ws.Path(), nil)
		ed := editor.NewEditor(ws.Path(), gitOps, nil)

		edit := editor.Edit{
			FilePath:     fileName,
			SearchBlock:  fmt.Sprintf("value%d", i+1),
			ReplaceBlock: fmt.Sprintf("updated_value%d", i+1),
			Description:  fmt.Sprintf("Update %s", ws.Name()),
		}

		result, err := ed.Apply(ctx, edit)
		require.NoError(t, err)
		assert.True(t, result.Applied)

		// Commit to each repo independently
		require.NoError(t, gitOps.Add(ctx, fileName))
		commitSHA, err := gitOps.Commit(ctx, fmt.Sprintf("Update %s", fileName), git.CommitOptions{})
		require.NoError(t, err)
		assert.NotEmpty(t, commitSHA)

		t.Logf("Committed to %s: %s", ws.Name(), commitSHA)
	}

	t.Logf("Successfully completed multi-repo scenario")
}

// TestWorktreeIsolation tests Git worktree isolation.
func TestWorktreeIsolation(t *testing.T) {
	if !checkCommandAvailable("git") {
		t.Skip("git not available in PATH")
	}

	ctx := context.Background()
	tmpDir := t.TempDir()

	// Create main repository
	mainRepo := filepath.Join(tmpDir, "main-repo")
	require.NoError(t, os.MkdirAll(mainRepo, 0755))
	initGitRepo(t, mainRepo)
	createInitialCommit(t, mainRepo)

	// Add a file
	testFile := filepath.Join(mainRepo, "data.txt")
	require.NoError(t, os.WriteFile(testFile, []byte("original data\n"), 0644))

	cmd := exec.Command("git", "add", "data.txt")
	cmd.Dir = mainRepo
	require.NoError(t, cmd.Run())

	cmd = exec.Command("git", "commit", "-m", "Add data file")
	cmd.Dir = mainRepo
	require.NoError(t, cmd.Run())

	// Create workspace with worktree support
	credStore := newMockCredStore()
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelWarn}))
	wm := workspace.NewWorkspaceManager(credStore, logger)

	config := workspace.WorkspaceConfig{
		Repositories: []workspace.RepositoryConfig{
			{
				Name:   "main",
				URL:    mainRepo,
				Branch: "main",
			},
		},
		Settings: workspace.WorkspaceSettings{
			BaseDirectory:     filepath.Join(tmpDir, "workspaces"),
			CleanupOnComplete: true,
			UseWorktrees:      true,
			LSPEnabled:        false,
		},
	}

	require.NoError(t, wm.Initialize(ctx, config))
	defer wm.Cleanup(ctx)

	ws := wm.Primary()
	require.NotNil(t, ws)

	// Make changes in workspace
	gitOps := git.NewGitOps(ws.Path(), nil)
	ed := editor.NewEditor(ws.Path(), gitOps, nil)

	edit := editor.Edit{
		FilePath:     "data.txt",
		SearchBlock:  "original data",
		ReplaceBlock: "modified data",
		Description:  "Modify data",
	}

	result, err := ed.Apply(ctx, edit)
	require.NoError(t, err)
	assert.True(t, result.Applied)

	// Verify workspace file is modified
	wsContent, err := ws.ReadFile(ctx, "data.txt")
	require.NoError(t, err)
	assert.Contains(t, string(wsContent), "modified data")

	// Verify main repo is unaffected
	mainContent, err := os.ReadFile(testFile)
	require.NoError(t, err)
	assert.Contains(t, string(mainContent), "original data")
	assert.NotContains(t, string(mainContent), "modified data")

	t.Logf("Successfully verified worktree isolation")
}

// TestCleanup tests workspace cleanup functionality.
func TestCleanup(t *testing.T) {
	if !checkCommandAvailable("git") {
		t.Skip("git not available in PATH")
	}

	ctx := context.Background()
	tmpDir := t.TempDir()

	// Create test repository
	sourceRepo := filepath.Join(tmpDir, "source")
	require.NoError(t, os.MkdirAll(sourceRepo, 0755))
	initGitRepo(t, sourceRepo)
	createInitialCommit(t, sourceRepo)

	// Create workspace
	credStore := newMockCredStore()
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelWarn}))
	wm := workspace.NewWorkspaceManager(credStore, logger)

	workspacesDir := filepath.Join(tmpDir, "workspaces")
	config := workspace.WorkspaceConfig{
		Repositories: []workspace.RepositoryConfig{
			{
				Name:   "test",
				URL:    sourceRepo,
				Branch: "main",
			},
		},
		Settings: workspace.WorkspaceSettings{
			BaseDirectory:     workspacesDir,
			CleanupOnComplete: true,
			LSPEnabled:        false,
		},
	}

	require.NoError(t, wm.Initialize(ctx, config))

	ws := wm.Primary()
	require.NotNil(t, ws)
	workspacePath := ws.Path()

	// Apply some changes
	testFile := filepath.Join(workspacePath, "test.txt")
	require.NoError(t, os.WriteFile(testFile, []byte("test data\n"), 0644))

	// Verify workspace exists
	_, err := os.Stat(workspacePath)
	require.NoError(t, err, "workspace should exist before cleanup")

	// Call cleanup
	require.NoError(t, wm.Cleanup(ctx))

	// Verify workspace is removed
	_, err = os.Stat(workspacePath)
	assert.True(t, os.IsNotExist(err), "workspace should be removed after cleanup")

	// Verify base directory is removed
	_, err = os.Stat(workspacesDir)
	assert.True(t, os.IsNotExist(err), "base directory should be removed after cleanup")

	t.Logf("Successfully verified cleanup")
}

// TestLSPValidationIntegration tests LSP validation with rollback.
func TestLSPValidationIntegration(t *testing.T) {
	if !checkCommandAvailable("git") {
		t.Skip("git not available in PATH")
	}
	if !checkCommandAvailable("gopls") {
		t.Skip("gopls not available in PATH")
	}

	ctx := context.Background()
	tmpDir := t.TempDir()

	// Create test repository with valid Go code
	sourceRepo := filepath.Join(tmpDir, "source")
	require.NoError(t, os.MkdirAll(sourceRepo, 0755))
	initGitRepo(t, sourceRepo)

	// Create go.mod
	goMod := filepath.Join(sourceRepo, "go.mod")
	require.NoError(t, os.WriteFile(goMod, []byte("module testmod\n\ngo 1.21\n"), 0644))

	// Create valid Go file
	goFile := filepath.Join(sourceRepo, "main.go")
	validCode := `package main

import "fmt"

func greet(name string) {
	fmt.Println("Hello", name)
}

func main() {
	greet("World")
}
`
	require.NoError(t, os.WriteFile(goFile, []byte(validCode), 0644))

	cmd := exec.Command("git", "add", ".")
	cmd.Dir = sourceRepo
	require.NoError(t, cmd.Run())

	cmd = exec.Command("git", "commit", "-m", "Add valid code")
	cmd.Dir = sourceRepo
	require.NoError(t, cmd.Run())

	// Create workspace with LSP
	credStore := newMockCredStore()
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelWarn}))
	wm := workspace.NewWorkspaceManager(credStore, logger)

	config := workspace.WorkspaceConfig{
		Repositories: []workspace.RepositoryConfig{
			{
				Name:   "test",
				URL:    sourceRepo,
				Branch: "main",
			},
		},
		Settings: workspace.WorkspaceSettings{
			BaseDirectory:     filepath.Join(tmpDir, "workspaces"),
			CleanupOnComplete: true,
			LSPEnabled:        true,
			LSPTimeout:        30 * time.Second,
		},
	}

	require.NoError(t, wm.Initialize(ctx, config))
	defer wm.Cleanup(ctx)

	ws := wm.Primary()
	require.NotNil(t, ws)

	// Create LSP manager
	lspConfig := lsp.LSPConfig{
		InitTimeout:       30 * time.Second,
		ValidationTimeout: 10 * time.Second,
		EnableGo:          true,
		EnablePython:      false,
		EnableTypeScript:  false,
	}
	lspMgr := lsp.NewLSPManager(lspConfig, logger)
	require.NoError(t, lspMgr.Start(ctx, ws.Path()))
	defer lspMgr.Stop(ctx)
	require.NoError(t, lspMgr.WaitForReady(ctx))

	gitOps := git.NewGitOps(ws.Path(), nil)
	ed := editor.NewEditor(ws.Path(), gitOps, lspMgr)

	// Test 1: Apply edit that introduces an error
	t.Run("introduce error and rollback", func(t *testing.T) {
		// Read original content
		originalContent, err := ws.ReadFile(ctx, "main.go")
		require.NoError(t, err)

		edit := editor.Edit{
			FilePath: "main.go",
			SearchBlock: `func greet(name string) {
	fmt.Println("Hello", name)
}`,
			ReplaceBlock: `func greet(name string) {
	fmt.Println("Hello", undefinedVar)
}`,
			Description: "Introduce error",
		}

		result, err := ed.Apply(ctx, edit)
		require.NoError(t, err)

		// Edit should be rolled back due to error
		assert.False(t, result.Applied, "edit should be rolled back")
		assert.True(t, result.HasErrors(), "should have validation errors")

		// Verify file was rolled back
		currentContent, err := ws.ReadFile(ctx, "main.go")
		require.NoError(t, err)
		assert.Equal(t, string(originalContent), string(currentContent), "file should be rolled back")
	})

	// Test 2: Apply valid edit
	t.Run("apply valid edit", func(t *testing.T) {
		edit := editor.Edit{
			FilePath: "main.go",
			SearchBlock: `func greet(name string) {
	fmt.Println("Hello", name)
}`,
			ReplaceBlock: `func greet(name string) {
	message := "Hello, " + name + "!"
	fmt.Println(message)
}`,
			Description: "Refactor greeting",
		}

		result, err := ed.Apply(ctx, edit)
		require.NoError(t, err)
		assert.True(t, result.Applied, "valid edit should be applied")
		assert.False(t, result.HasErrors(), "should not have errors")

		// Verify file was modified
		content, err := ws.ReadFile(ctx, "main.go")
		require.NoError(t, err)
		assert.Contains(t, string(content), `message := "Hello, " + name + "!"`)
	})

	t.Logf("Successfully tested LSP validation integration")
}

// TestEdgeCases tests various edge cases and error conditions.
func TestEdgeCases(t *testing.T) {
	if !checkCommandAvailable("git") {
		t.Skip("git not available in PATH")
	}

	ctx := context.Background()
	tmpDir := t.TempDir()

	// Create test repository
	sourceRepo := filepath.Join(tmpDir, "source")
	require.NoError(t, os.MkdirAll(sourceRepo, 0755))
	initGitRepo(t, sourceRepo)
	createInitialCommit(t, sourceRepo)

	credStore := newMockCredStore()
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelWarn}))

	t.Run("empty repository list", func(t *testing.T) {
		wm := workspace.NewWorkspaceManager(credStore, logger)
		config := workspace.WorkspaceConfig{
			Repositories: []workspace.RepositoryConfig{},
			Settings: workspace.WorkspaceSettings{
				BaseDirectory:     filepath.Join(tmpDir, "ws1"),
				CleanupOnComplete: true,
			},
		}
		err := wm.Initialize(ctx, config)
		assert.Error(t, err, "should fail with empty repository list")
	})

	t.Run("duplicate repository names", func(t *testing.T) {
		wm := workspace.NewWorkspaceManager(credStore, logger)
		config := workspace.WorkspaceConfig{
			Repositories: []workspace.RepositoryConfig{
				{Name: "same", URL: sourceRepo, Branch: "main"},
				{Name: "same", URL: sourceRepo, Branch: "main"},
			},
			Settings: workspace.WorkspaceSettings{
				BaseDirectory:     filepath.Join(tmpDir, "ws2"),
				CleanupOnComplete: true,
			},
		}
		err := wm.Initialize(ctx, config)
		assert.Error(t, err, "should fail with duplicate repository names")
	})

	t.Run("invalid repository URL", func(t *testing.T) {
		wm := workspace.NewWorkspaceManager(credStore, logger)
		config := workspace.WorkspaceConfig{
			Repositories: []workspace.RepositoryConfig{
				{Name: "invalid", URL: "/nonexistent/path", Branch: "main"},
			},
			Settings: workspace.WorkspaceSettings{
				BaseDirectory:     filepath.Join(tmpDir, "ws3"),
				CleanupOnComplete: true,
			},
		}
		err := wm.Initialize(ctx, config)
		assert.Error(t, err, "should fail with invalid repository URL")
	})

	t.Run("missing credential", func(t *testing.T) {
		wm := workspace.NewWorkspaceManager(credStore, logger)
		config := workspace.WorkspaceConfig{
			Repositories: []workspace.RepositoryConfig{
				{
					Name:           "test",
					URL:            "https://github.com/private/repo.git",
					Branch:         "main",
					CredentialName: "nonexistent",
				},
			},
			Settings: workspace.WorkspaceSettings{
				BaseDirectory:     filepath.Join(tmpDir, "ws4"),
				CleanupOnComplete: true,
			},
		}
		err := wm.Initialize(ctx, config)
		assert.Error(t, err, "should fail with missing credential")
	})
}
