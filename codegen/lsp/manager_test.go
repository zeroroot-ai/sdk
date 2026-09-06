// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Zero Root AI

package lsp

import (
	"context"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/zeroroot-ai/sdk/codegen"
)

// skipIfNoLSPBinaries skips the test when none of the LSP server binaries
// (gopls, pyright-langserver, typescript-language-server) are present in PATH.
// These are integration tests that require external tooling. Pre-existing
// infrastructure requirement; not caused by zitadel-envoy-gateway-migration spec.
func skipIfNoLSPBinaries(t *testing.T) {
	t.Helper()
	binaries := []string{"gopls", "pyright-langserver", "typescript-language-server"}
	for _, bin := range binaries {
		if _, err := exec.LookPath(bin); err == nil {
			return // at least one binary is available
		}
	}
	t.Skip("skipping LSP integration test: gopls, pyright-langserver, and typescript-language-server not found in PATH")
}

// TestNewLSPManager verifies manager creation.
func TestNewLSPManager(t *testing.T) {
	config := DefaultLSPConfig()
	manager := NewLSPManager(config, slog.Default())

	assert.NotNil(t, manager)
	assert.Equal(t, []string{}, manager.SupportedLanguages())
}

// TestManagerStartStop verifies basic lifecycle.
func TestManagerStartStop(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}
	skipIfNoLSPBinaries(t)

	// Create a temporary workspace
	tempDir := t.TempDir()

	// Create a simple Go file
	goFile := filepath.Join(tempDir, "test.go")
	err := os.WriteFile(goFile, []byte(`package main

import "fmt"

func main() {
	fmt.Println("Hello, World!")
}
`), 0644)
	require.NoError(t, err)

	// Create go.mod to make it a valid Go workspace
	goMod := filepath.Join(tempDir, "go.mod")
	err = os.WriteFile(goMod, []byte(`module testworkspace

go 1.21
`), 0644)
	require.NoError(t, err)

	config := DefaultLSPConfig()
	config.InitTimeout = 30 * time.Second
	manager := NewLSPManager(config, slog.Default())

	ctx := context.Background()

	// Start the manager
	err = manager.Start(ctx, tempDir)
	require.NoError(t, err)

	// Verify gopls is running
	langs := manager.SupportedLanguages()
	assert.Contains(t, langs, "go")

	// Wait for ready
	err = manager.WaitForReady(ctx)
	require.NoError(t, err)

	// Stop the manager
	err = manager.Stop(ctx)
	assert.NoError(t, err)
}

// TestGetDiagnosticsValidCode tests diagnostics for valid Go code.
func TestGetDiagnosticsValidCode(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}
	skipIfNoLSPBinaries(t)

	// Create a temporary workspace
	tempDir := t.TempDir()

	// Create a valid Go file
	goFile := filepath.Join(tempDir, "valid.go")
	err := os.WriteFile(goFile, []byte(`package main

import "fmt"

func main() {
	fmt.Println("Hello, World!")
}
`), 0644)
	require.NoError(t, err)

	// Create go.mod
	goMod := filepath.Join(tempDir, "go.mod")
	err = os.WriteFile(goMod, []byte(`module testworkspace

go 1.21
`), 0644)
	require.NoError(t, err)

	config := DefaultLSPConfig()
	config.ValidationTimeout = 15 * time.Second
	manager := NewLSPManager(config, slog.Default())

	ctx := context.Background()

	// Start the manager
	err = manager.Start(ctx, tempDir)
	require.NoError(t, err)
	defer manager.Stop(ctx)

	// Wait for ready
	err = manager.WaitForReady(ctx)
	require.NoError(t, err)

	// Get diagnostics
	diagnostics, err := manager.GetDiagnostics(ctx, goFile)
	require.NoError(t, err)

	// Should have no errors for valid code
	hasErrors := false
	for _, d := range diagnostics {
		if d.IsError() {
			hasErrors = true
			t.Logf("Unexpected error: %s at line %d: %s", d.Source, d.Line, d.Message)
		}
	}
	assert.False(t, hasErrors, "valid code should not produce errors")
}

// TestGetDiagnosticsInvalidCode tests diagnostics for invalid Go code.
func TestGetDiagnosticsInvalidCode(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}
	skipIfNoLSPBinaries(t)

	// Create a temporary workspace
	tempDir := t.TempDir()

	// Create an invalid Go file (undefined variable)
	goFile := filepath.Join(tempDir, "invalid.go")
	err := os.WriteFile(goFile, []byte(`package main

func main() {
	undefinedVariable()
}
`), 0644)
	require.NoError(t, err)

	// Create go.mod
	goMod := filepath.Join(tempDir, "go.mod")
	err = os.WriteFile(goMod, []byte(`module testworkspace

go 1.21
`), 0644)
	require.NoError(t, err)

	config := DefaultLSPConfig()
	config.ValidationTimeout = 15 * time.Second
	manager := NewLSPManager(config, slog.Default())

	ctx := context.Background()

	// Start the manager
	err = manager.Start(ctx, tempDir)
	require.NoError(t, err)
	defer manager.Stop(ctx)

	// Wait for ready
	err = manager.WaitForReady(ctx)
	require.NoError(t, err)

	// Get diagnostics
	diagnostics, err := manager.GetDiagnostics(ctx, goFile)
	require.NoError(t, err)

	// Should have at least one error
	hasErrors := false
	for _, d := range diagnostics {
		if d.IsError() {
			hasErrors = true
			t.Logf("Expected error found: %s at line %d: %s", d.Source, d.Line, d.Message)
		}
	}
	assert.True(t, hasErrors, "invalid code should produce errors")
}

// TestGetDiagnosticsSyntaxError tests diagnostics for syntax errors.
func TestGetDiagnosticsSyntaxError(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}
	skipIfNoLSPBinaries(t)

	// Create a temporary workspace
	tempDir := t.TempDir()

	// Create a Go file with syntax error
	goFile := filepath.Join(tempDir, "syntax_error.go")
	err := os.WriteFile(goFile, []byte(`package main

func main() {
	fmt.Println("missing closing quote)
}
`), 0644)
	require.NoError(t, err)

	// Create go.mod
	goMod := filepath.Join(tempDir, "go.mod")
	err = os.WriteFile(goMod, []byte(`module testworkspace

go 1.21
`), 0644)
	require.NoError(t, err)

	config := DefaultLSPConfig()
	config.ValidationTimeout = 15 * time.Second
	manager := NewLSPManager(config, slog.Default())

	ctx := context.Background()

	// Start the manager
	err = manager.Start(ctx, tempDir)
	require.NoError(t, err)
	defer manager.Stop(ctx)

	// Wait for ready
	err = manager.WaitForReady(ctx)
	require.NoError(t, err)

	// Get diagnostics
	diagnostics, err := manager.GetDiagnostics(ctx, goFile)
	require.NoError(t, err)

	// Should have syntax errors
	assert.NotEmpty(t, diagnostics, "syntax error should produce diagnostics")

	// Log all diagnostics
	for _, d := range diagnostics {
		t.Logf("Diagnostic [%s] at line %d: %s", d.Severity, d.Line, d.Message)
	}
}

// TestDiagnosticSeverity tests diagnostic severity classification.
func TestDiagnosticSeverity(t *testing.T) {
	tests := []struct {
		name     string
		severity codegen.DiagnosticSeverity
		isError  bool
		isWarn   bool
	}{
		{
			name:     "error severity",
			severity: codegen.SeverityError,
			isError:  true,
			isWarn:   false,
		},
		{
			name:     "warning severity",
			severity: codegen.SeverityWarning,
			isError:  false,
			isWarn:   true,
		},
		{
			name:     "info severity",
			severity: codegen.SeverityInfo,
			isError:  false,
			isWarn:   false,
		},
		{
			name:     "hint severity",
			severity: codegen.SeverityHint,
			isError:  false,
			isWarn:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			diag := codegen.Diagnostic{
				Severity: tt.severity,
			}

			assert.Equal(t, tt.isError, diag.IsError())
			assert.Equal(t, tt.isWarn, diag.IsWarning())
		})
	}
}

// TestManagerInvalidWorkspace tests error handling for invalid workspace.
func TestManagerInvalidWorkspace(t *testing.T) {
	config := DefaultLSPConfig()
	manager := NewLSPManager(config, slog.Default())

	ctx := context.Background()

	// Try to start with non-existent workspace
	err := manager.Start(ctx, "/nonexistent/path")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "workspace root not accessible")
}

// TestManagerDoubleStart tests that starting twice returns an error.
func TestManagerDoubleStart(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}
	skipIfNoLSPBinaries(t)

	tempDir := t.TempDir()

	config := DefaultLSPConfig()
	manager := NewLSPManager(config, slog.Default())

	ctx := context.Background()

	// First start should succeed
	err := manager.Start(ctx, tempDir)
	require.NoError(t, err)
	defer manager.Stop(ctx)

	// Second start should fail
	err = manager.Start(ctx, tempDir)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "already started")
}

// TestDefaultLSPConfig verifies default configuration values.
func TestDefaultLSPConfig(t *testing.T) {
	config := DefaultLSPConfig()

	assert.Empty(t, config.GoplsPath)
	assert.Empty(t, config.PyrightPath)
	assert.Empty(t, config.TypeScriptServerPath)
	assert.Equal(t, 30*time.Second, config.InitTimeout)
	assert.Equal(t, 10*time.Second, config.ValidationTimeout)
	assert.True(t, config.EnableGo)
	assert.True(t, config.EnablePython)
	assert.True(t, config.EnableTypeScript)
}

// TestCountBySeverity is the regression test for issue #404: the
// severity tally must switch on codegen.DiagnosticSeverity enum
// members (SeverityError=0, SeverityWarning=1), not raw LSP wire
// values (1=Error, 2=Warning). Before the fix, errors went uncounted
// and warnings were tallied as errors.
func TestCountBySeverity(t *testing.T) {
	diags := []codegen.Diagnostic{
		{Severity: codegen.SeverityError},
		{Severity: codegen.SeverityError},
		{Severity: codegen.SeverityWarning},
		{Severity: codegen.SeverityInfo},
		{Severity: codegen.SeverityHint},
	}

	errorCount, warningCount := countBySeverity(diags)

	assert.Equal(t, 2, errorCount, "SeverityError diagnostics must be counted as errors")
	assert.Equal(t, 1, warningCount, "SeverityWarning diagnostics must be counted as warnings")
}

// TestCountBySeverity_Empty verifies the zero case.
func TestCountBySeverity_Empty(t *testing.T) {
	errorCount, warningCount := countBySeverity(nil)
	assert.Zero(t, errorCount)
	assert.Zero(t, warningCount)
}
