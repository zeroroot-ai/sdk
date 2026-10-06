// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Zero Root AI

package editor

import (
	"context"
	"os"
	"path/filepath"

	"github.com/zeroroot-ai/sdk/codegen"
	"github.com/zeroroot-ai/sdk/codegen/git"
)

// MockGitOps provides a mock implementation of git.GitOps for testing.
type MockGitOps struct {
	snapshots    map[string]map[string]string // snapshotID -> filepath -> content
	nextID       int
	workspaceDir string
}

func NewMockGitOps() *MockGitOps {
	return &MockGitOps{
		snapshots: make(map[string]map[string]string),
		nextID:    1,
	}
}

func (m *MockGitOps) SetWorkspaceDir(dir string) {
	m.workspaceDir = dir
}

func (m *MockGitOps) CurrentBranch() (string, error) {
	return "main", nil
}

func (m *MockGitOps) CreateBranch(ctx context.Context, name string) error {
	return nil
}

func (m *MockGitOps) Checkout(ctx context.Context, ref string) error {
	return nil
}

func (m *MockGitOps) Add(ctx context.Context, paths ...string) error {
	return nil
}

func (m *MockGitOps) Commit(ctx context.Context, message string, opts git.CommitOptions) (string, error) {
	return "abc123", nil
}

func (m *MockGitOps) Push(ctx context.Context, opts git.PushOptions) error {
	return nil
}

func (m *MockGitOps) Pull(ctx context.Context) error {
	return nil
}

func (m *MockGitOps) Snapshot(ctx context.Context) (string, error) {
	id := "snapshot-" + string(rune(m.nextID+'0'))
	m.nextID++

	// Save current state of all files in workspace
	fileStates := make(map[string]string)

	if m.workspaceDir != "" {
		_ = filepath.Walk(m.workspaceDir, func(path string, info os.FileInfo, err error) error {
			if err != nil || info.IsDir() {
				return nil
			}
			content, err := os.ReadFile(path)
			if err != nil {
				return nil
			}
			relPath, _ := filepath.Rel(m.workspaceDir, path)
			fileStates[relPath] = string(content)
			return nil
		})
	}

	m.snapshots[id] = fileStates
	return id, nil
}

func (m *MockGitOps) Rollback(ctx context.Context, snapshotID string) error {
	fileStates, exists := m.snapshots[snapshotID]
	if !exists {
		return os.ErrNotExist
	}

	// Restore files to snapshot state
	for relPath, content := range fileStates {
		absPath := filepath.Join(m.workspaceDir, relPath)
		if err := os.WriteFile(absPath, []byte(content), 0644); err != nil {
			return err
		}
	}

	return nil
}

// MockLSPManager provides a mock implementation of lsp.LSPManager for testing.
type MockLSPManager struct {
	diagnostics map[string][]codegen.Diagnostic
	shouldError bool
}

func NewMockLSPManager() *MockLSPManager {
	return &MockLSPManager{
		diagnostics: make(map[string][]codegen.Diagnostic),
	}
}

func (m *MockLSPManager) Start(ctx context.Context, workspaceRoot string) error {
	return nil
}

func (m *MockLSPManager) Stop(ctx context.Context) error {
	return nil
}

func (m *MockLSPManager) GetDiagnostics(ctx context.Context, path string) ([]codegen.Diagnostic, error) {
	if m.shouldError {
		return nil, os.ErrNotExist
	}
	return m.diagnostics[path], nil
}

func (m *MockLSPManager) WaitForReady(ctx context.Context) error {
	return nil
}

func (m *MockLSPManager) SupportedLanguages() []string {
	return []string{"go"}
}

func (m *MockLSPManager) AddDiagnostic(path string, diag codegen.Diagnostic) {
	m.diagnostics[path] = append(m.diagnostics[path], diag)
}

// Helper function to check if string contains substring.
func contains(s, substr string) bool {
	return len(s) > 0 && len(substr) > 0 && (s == substr || len(s) > len(substr) && findSubstring(s, substr))
}

func findSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
