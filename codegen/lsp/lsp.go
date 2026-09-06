// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Zero Root AI

package lsp

import (
	"context"
	"time"

	"github.com/zeroroot-ai/sdk/codegen"
)

// LSPManager manages language server processes and provides code validation.
// It handles the lifecycle of LSP servers for multiple programming languages,
// enabling real-time diagnostics and validation of code changes.
//
// The manager starts language servers for a workspace and routes validation
// requests to the appropriate server based on file extensions. It supports
// concurrent validation requests and graceful shutdown.
type LSPManager interface {
	// Start initializes and starts language servers for the given workspace.
	// The workspaceRoot should be an absolute path to the workspace directory.
	// Language servers are started based on the detected project languages and
	// LSPConfig settings.
	//
	// Returns an error if:
	// - workspaceRoot is invalid or inaccessible
	// - Language server binaries are not found
	// - Server initialization fails
	Start(ctx context.Context, workspaceRoot string) error

	// Stop gracefully shuts down all running language servers.
	// It waits for ongoing validation requests to complete before stopping.
	//
	// Returns an error if any server fails to stop cleanly.
	Stop(ctx context.Context) error

	// GetDiagnostics retrieves diagnostics for the specified file path.
	// The path should be absolute or relative to the workspace root.
	//
	// Returns:
	// - Slice of diagnostics (may be empty if no issues found)
	// - Error if the file cannot be validated or the server is not ready
	//
	// If the context deadline is exceeded, returns ErrLSPTimeout.
	GetDiagnostics(ctx context.Context, path string) ([]codegen.Diagnostic, error)

	// WaitForReady blocks until all language servers are initialized and ready.
	// This should be called after Start() and before any validation requests.
	//
	// Returns an error if:
	// - The context is cancelled or deadline exceeded
	// - Server initialization fails permanently
	WaitForReady(ctx context.Context) error

	// SupportedLanguages returns the list of programming languages this manager
	// can validate. Common values include: "go", "python", "typescript", "javascript".
	//
	// The returned slice is based on the LSPConfig and available language server binaries.
	SupportedLanguages() []string
}

// LSPConfig configures language server settings and timeouts.
// It specifies paths to language server binaries and operational parameters.
type LSPConfig struct {
	// GoplsPath is the path to the gopls binary for Go language support.
	// If empty, gopls will be searched in PATH.
	// Example: "/usr/local/bin/gopls"
	GoplsPath string

	// PyrightPath is the path to the pyright-langserver binary for Python support.
	// If empty, pyright-langserver will be searched in PATH.
	// Example: "/usr/local/bin/pyright-langserver"
	PyrightPath string

	// TypeScriptServerPath is the path to typescript-language-server for TypeScript/JavaScript support.
	// If empty, typescript-language-server will be searched in PATH.
	// Example: "/usr/local/bin/typescript-language-server"
	TypeScriptServerPath string

	// InitTimeout is the maximum time to wait for language servers to initialize.
	// Default: 30 seconds
	InitTimeout time.Duration

	// ValidationTimeout is the maximum time to wait for diagnostic validation requests.
	// If a validation exceeds this timeout, GetDiagnostics returns ErrLSPTimeout.
	// Default: 10 seconds
	ValidationTimeout time.Duration

	// EnableGo enables the Go language server (gopls) if true.
	// Default: true
	EnableGo bool

	// EnablePython enables the Python language server (pyright) if true.
	// Default: true
	EnablePython bool

	// EnableTypeScript enables the TypeScript/JavaScript language server if true.
	// Default: true
	EnableTypeScript bool
}

// DefaultLSPConfig returns an LSPConfig with sensible defaults.
// Language server binaries are expected to be in PATH.
// All supported languages are enabled with reasonable timeout values.
func DefaultLSPConfig() LSPConfig {
	return LSPConfig{
		GoplsPath:            "", // Search in PATH
		PyrightPath:          "", // Search in PATH
		TypeScriptServerPath: "", // Search in PATH
		InitTimeout:          30 * time.Second,
		ValidationTimeout:    10 * time.Second,
		EnableGo:             true,
		EnablePython:         true,
		EnableTypeScript:     true,
	}
}
