// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Zero Root AI

package editor

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

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

func (m *MockGitOps) Status() (*git.GitStatus, error) {
	return &git.GitStatus{Branch: "main"}, nil
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

// TestEditorExactMatch tests exact SEARCH/REPLACE matching.
func TestEditorExactMatch(t *testing.T) {
	// Create temporary workspace
	tmpDir := t.TempDir()

	// Create test file
	testFile := filepath.Join(tmpDir, "test.go")
	content := `package main

func hello() {
	println("hello")
}

func main() {
	hello()
}
`
	if err := os.WriteFile(testFile, []byte(content), 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	// Create editor
	gitOps := NewMockGitOps()
	gitOps.SetWorkspaceDir(tmpDir)
	editor := NewEditor(tmpDir, gitOps, nil)

	// Apply edit
	edit := Edit{
		FilePath: "test.go",
		SearchBlock: `func hello() {
	println("hello")
}`,
		ReplaceBlock: `func hello() {
	fmt.Println("hello world")
}`,
		Description: "Update to use fmt.Println",
	}

	result, err := editor.Apply(context.Background(), edit)
	if err != nil {
		t.Fatalf("Apply failed: %v", err)
	}

	// Verify result
	if !result.Applied {
		t.Errorf("Edit was not applied")
	}
	if result.MatchType != codegen.MatchExact {
		t.Errorf("Expected exact match, got %s", result.MatchType)
	}
	if result.Snapshot == "" {
		t.Errorf("Snapshot ID is empty")
	}

	// Verify file was modified
	modified, err := os.ReadFile(testFile)
	if err != nil {
		t.Fatalf("Failed to read modified file: %v", err)
	}

	expectedContent := `package main

func hello() {
	fmt.Println("hello world")
}

func main() {
	hello()
}
`
	if string(modified) != expectedContent {
		t.Errorf("File content mismatch.\nExpected:\n%s\nGot:\n%s", expectedContent, string(modified))
	}
}

// TestEditorFuzzyMatch tests fuzzy SEARCH/REPLACE matching.
func TestEditorFuzzyMatch(t *testing.T) {
	tmpDir := t.TempDir()

	testFile := filepath.Join(tmpDir, "test.go")
	content := `package main

func calculate(x int) int {
	return x * 2
}
`
	if err := os.WriteFile(testFile, []byte(content), 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	gitOps := NewMockGitOps()
	gitOps.SetWorkspaceDir(tmpDir)
	editor := NewEditor(tmpDir, gitOps, nil)
	editor.SetFuzzyThreshold(0.85)

	// Search block has slightly different value
	edit := Edit{
		FilePath: "test.go",
		SearchBlock: `func calculate(x int) int {
	return x * 3
}`,
		ReplaceBlock: `func calculate(x int) int {
	return x * 10
}`,
		Description: "Change multiplier",
	}

	result, err := editor.Apply(context.Background(), edit)
	if err != nil {
		t.Fatalf("Apply failed: %v", err)
	}

	if !result.Applied {
		t.Errorf("Edit was not applied")
	}
	if result.MatchType != codegen.MatchFuzzy {
		t.Errorf("Expected fuzzy match, got %s", result.MatchType)
	}
	if result.FuzzyMatchSimilarity < 0.85 {
		t.Errorf("Similarity too low: %f", result.FuzzyMatchSimilarity)
	}
}

// TestEditorMatchFailed tests when no match is found.
func TestEditorMatchFailed(t *testing.T) {
	tmpDir := t.TempDir()

	testFile := filepath.Join(tmpDir, "test.go")
	content := `package main

func hello() {
	println("hello")
}
`
	if err := os.WriteFile(testFile, []byte(content), 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	gitOps := NewMockGitOps()
	gitOps.SetWorkspaceDir(tmpDir)
	editor := NewEditor(tmpDir, gitOps, nil)

	edit := Edit{
		FilePath:     "test.go",
		SearchBlock:  "func nonexistent() {}",
		ReplaceBlock: "func replacement() {}",
		Description:  "This should fail",
	}

	result, err := editor.Apply(context.Background(), edit)
	if err != nil {
		t.Fatalf("Apply failed: %v", err)
	}

	if result.Applied {
		t.Errorf("Edit should not have been applied")
	}
	if result.MatchType != codegen.MatchFailed {
		t.Errorf("Expected match failed, got %s", result.MatchType)
	}
	if result.ClosestMatch == nil {
		t.Errorf("Expected closest match info for debugging")
	}

	// Verify file was not modified
	modified, err := os.ReadFile(testFile)
	if err != nil {
		t.Fatalf("Failed to read file: %v", err)
	}
	if string(modified) != content {
		t.Errorf("File should not have been modified")
	}
}

// TestEditorWithLSPValidation tests LSP validation integration.
func TestEditorWithLSPValidation(t *testing.T) {
	tmpDir := t.TempDir()

	testFile := filepath.Join(tmpDir, "test.go")
	content := `package main

func hello() {
	println("hello")
}
`
	if err := os.WriteFile(testFile, []byte(content), 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	gitOps := NewMockGitOps()
	gitOps.SetWorkspaceDir(tmpDir)
	lspManager := NewMockLSPManager()

	// Add an error diagnostic
	absPath := filepath.Join(tmpDir, "test.go")
	lspManager.AddDiagnostic(absPath, codegen.Diagnostic{
		Path:     absPath,
		Line:     3,
		Column:   1,
		Severity: codegen.SeverityError,
		Message:  "undefined: fmt",
		Source:   "gopls",
	})

	editor := NewEditor(tmpDir, gitOps, lspManager)

	edit := Edit{
		FilePath: "test.go",
		SearchBlock: `func hello() {
	println("hello")
}`,
		ReplaceBlock: `func hello() {
	fmt.Println("hello")
}`,
		Description: "This will fail validation",
	}

	result, err := editor.Apply(context.Background(), edit)
	if err != nil {
		t.Fatalf("Apply failed: %v", err)
	}

	// Edit should be rolled back due to error
	if result.Applied {
		t.Errorf("Edit should have been rolled back due to validation error")
	}
	if !result.HasErrors() {
		t.Errorf("Expected validation errors")
	}
	if result.ErrorCount() != 1 {
		t.Errorf("Expected 1 error, got %d", result.ErrorCount())
	}

	// Verify file was rolled back
	modified, err := os.ReadFile(testFile)
	if err != nil {
		t.Fatalf("Failed to read file: %v", err)
	}
	if string(modified) != content {
		t.Errorf("File should have been rolled back to original content")
	}
}

// TestEditorWithLSPWarnings tests that warnings don't trigger rollback.
func TestEditorWithLSPWarnings(t *testing.T) {
	tmpDir := t.TempDir()

	testFile := filepath.Join(tmpDir, "test.go")
	content := `package main

func hello() {
	println("hello")
}
`
	if err := os.WriteFile(testFile, []byte(content), 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	gitOps := NewMockGitOps()
	gitOps.SetWorkspaceDir(tmpDir)
	lspManager := NewMockLSPManager()

	// Add a warning diagnostic (not an error)
	absPath := filepath.Join(tmpDir, "test.go")
	lspManager.AddDiagnostic(absPath, codegen.Diagnostic{
		Path:     absPath,
		Line:     3,
		Column:   1,
		Severity: codegen.SeverityWarning,
		Message:  "unused parameter",
		Source:   "gopls",
	})

	editor := NewEditor(tmpDir, gitOps, lspManager)

	edit := Edit{
		FilePath: "test.go",
		SearchBlock: `func hello() {
	println("hello")
}`,
		ReplaceBlock: `func hello() {
	fmt.Println("hello")
}`,
		Description: "This will have warnings but should still apply",
	}

	result, err := editor.Apply(context.Background(), edit)
	if err != nil {
		t.Fatalf("Apply failed: %v", err)
	}

	// Edit should be applied despite warnings
	if !result.Applied {
		t.Errorf("Edit should have been applied despite warnings")
	}
	if !result.HasWarnings() {
		t.Errorf("Expected validation warnings")
	}
	if result.WarningCount() != 1 {
		t.Errorf("Expected 1 warning, got %d", result.WarningCount())
	}
}

// TestEditorBatch tests batch edit operations.
func TestEditorBatch(t *testing.T) {
	tmpDir := t.TempDir()

	// Create multiple test files
	file1 := filepath.Join(tmpDir, "file1.go")
	file2 := filepath.Join(tmpDir, "file2.go")

	content1 := `package main

func foo() {
	return 1
}
`
	content2 := `package main

func bar() {
	return 2
}
`

	if err := os.WriteFile(file1, []byte(content1), 0644); err != nil {
		t.Fatalf("Failed to create file1: %v", err)
	}
	if err := os.WriteFile(file2, []byte(content2), 0644); err != nil {
		t.Fatalf("Failed to create file2: %v", err)
	}

	gitOps := NewMockGitOps()
	gitOps.SetWorkspaceDir(tmpDir)
	editor := NewEditor(tmpDir, gitOps, nil)

	edits := []Edit{
		{
			FilePath: "file1.go",
			SearchBlock: `func foo() {
	return 1
}`,
			ReplaceBlock: `func foo() {
	return 10
}`,
		},
		{
			FilePath: "file2.go",
			SearchBlock: `func bar() {
	return 2
}`,
			ReplaceBlock: `func bar() {
	return 20
}`,
		},
	}

	result, err := editor.ApplyBatch(context.Background(), edits)
	if err != nil {
		t.Fatalf("ApplyBatch failed: %v", err)
	}

	if !result.Applied {
		t.Errorf("Batch should have been applied")
	}
	if result.SuccessfulEdits() != 2 {
		t.Errorf("Expected 2 successful edits, got %d", result.SuccessfulEdits())
	}
	if result.FailedEdits() != 0 {
		t.Errorf("Expected 0 failed edits, got %d", result.FailedEdits())
	}

	// Verify both files were modified
	modified1, _ := os.ReadFile(file1)
	modified2, _ := os.ReadFile(file2)

	if !contains(string(modified1), "return 10") {
		t.Errorf("file1 was not modified correctly")
	}
	if !contains(string(modified2), "return 20") {
		t.Errorf("file2 was not modified correctly")
	}
}

// TestEditorBatchRollback tests that batch edits are rolled back on error.
func TestEditorBatchRollback(t *testing.T) {
	tmpDir := t.TempDir()

	testFile := filepath.Join(tmpDir, "test.go")
	content := `package main

func hello() {
	println("hello")
}
`
	if err := os.WriteFile(testFile, []byte(content), 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	gitOps := NewMockGitOps()
	gitOps.SetWorkspaceDir(tmpDir)
	lspManager := NewMockLSPManager()

	// Add error diagnostic
	absPath := filepath.Join(tmpDir, "test.go")
	lspManager.AddDiagnostic(absPath, codegen.Diagnostic{
		Path:     absPath,
		Line:     3,
		Column:   1,
		Severity: codegen.SeverityError,
		Message:  "compilation error",
		Source:   "gopls",
	})

	editor := NewEditor(tmpDir, gitOps, lspManager)

	edits := []Edit{
		{
			FilePath: "test.go",
			SearchBlock: `func hello() {
	println("hello")
}`,
			ReplaceBlock: `func hello() {
	INVALID CODE
}`,
		},
	}

	result, err := editor.ApplyBatch(context.Background(), edits)
	if err != nil {
		t.Fatalf("ApplyBatch failed: %v", err)
	}

	// Batch should be rolled back due to errors
	if result.Applied {
		t.Errorf("Batch should have been rolled back")
	}
	if result.ValidationStatus != codegen.ValidationFailed {
		t.Errorf("Expected validation failed, got %s", result.ValidationStatus)
	}

	// Verify file was rolled back
	modified, err := os.ReadFile(testFile)
	if err != nil {
		t.Fatalf("Failed to read file: %v", err)
	}
	if string(modified) != content {
		t.Errorf("File should have been rolled back")
	}
}

// TestEditorConfiguration tests configuration methods.
func TestEditorConfiguration(t *testing.T) {
	tmpDir := t.TempDir()
	gitOps := NewMockGitOps()
	gitOps.SetWorkspaceDir(tmpDir)
	editor := NewEditor(tmpDir, gitOps, nil)

	// Test fuzzy threshold
	editor.SetFuzzyThreshold(0.75)
	stats := editor.Stats()
	if stats.FuzzyThreshold != 0.75 {
		t.Errorf("Fuzzy threshold not updated: %f", stats.FuzzyThreshold)
	}

	// Test validation timeout
	editor.SetValidationTimeout(5 * time.Second)
	stats = editor.Stats()
	if stats.ValidationTimeout != 5*time.Second {
		t.Errorf("Validation timeout not updated: %v", stats.ValidationTimeout)
	}

	// Test workspace root
	if stats.WorkspaceRoot != tmpDir {
		t.Errorf("Workspace root mismatch: %s", stats.WorkspaceRoot)
	}

	// Test LSP enabled flag
	if stats.LSPEnabled {
		t.Errorf("LSP should not be enabled")
	}

	gitOpsWithLSP := NewMockGitOps()
	gitOpsWithLSP.SetWorkspaceDir(tmpDir)
	editorWithLSP := NewEditor(tmpDir, gitOpsWithLSP, NewMockLSPManager())
	statsWithLSP := editorWithLSP.Stats()
	if !statsWithLSP.LSPEnabled {
		t.Errorf("LSP should be enabled")
	}
}

// TestFormatEditSummary tests the summary formatting function.
func TestFormatEditSummary(t *testing.T) {
	result := &EditResult{
		Applied:   true,
		FilePath:  "test.go",
		MatchType: codegen.MatchExact,
	}

	summary := FormatEditSummary(result)
	if !contains(summary, "test.go") {
		t.Errorf("Summary should contain file path")
	}
	if !contains(summary, "exact") {
		t.Errorf("Summary should contain match type")
	}
}

// TestFormatBatchSummary tests the batch summary formatting function.
func TestFormatBatchSummary(t *testing.T) {
	result := &BatchEditResult{
		Applied: true,
		Results: []EditResult{
			{Applied: true},
			{Applied: true},
		},
		ValidationStatus: codegen.ValidationPassed,
	}

	summary := FormatBatchSummary(result)
	if !contains(summary, "2") {
		t.Errorf("Summary should contain total count")
	}
	if !contains(summary, "passed") {
		t.Errorf("Summary should contain validation status")
	}
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
