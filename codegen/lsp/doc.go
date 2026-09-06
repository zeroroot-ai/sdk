// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Zero Root AI

// Package lsp provides Language Server Protocol integration for code validation.
//
// This package manages language server processes and provides real-time
// code validation through LSP diagnostics. It supports multiple programming
// languages including Go, Python, and TypeScript/JavaScript.
//
// # Overview
//
// The LSP manager provides:
//   - Automatic language server startup and initialization
//   - Multi-language support (Go, Python, TypeScript/JavaScript)
//   - Diagnostic retrieval for code validation
//   - Graceful server lifecycle management
//   - Timeout handling for responsive operations
//
// # Supported Languages
//
// Go (gopls):
//
//	Language: Go
//	Server: gopls
//	Installation: go install golang.org/x/tools/gopls@latest
//	Features:
//	  - Syntax error detection
//	  - Type checking
//	  - Undefined variable detection
//	  - Import statement validation
//	  - Interface compliance checking
//
// Python (pyright):
//
//	Language: Python
//	Server: pyright-langserver
//	Installation: npm install -g pyright
//	Features:
//	  - Syntax error detection
//	  - Type checking (with type hints)
//	  - Import resolution
//	  - Undefined name detection
//	  - Common Python mistakes
//
// TypeScript/JavaScript (tsserver):
//
//	Languages: TypeScript, JavaScript
//	Server: typescript-language-server
//	Installation: npm install -g typescript-language-server typescript
//	Features:
//	  - Syntax error detection
//	  - Type checking (TypeScript)
//	  - Module resolution
//	  - JSDoc validation
//	  - ES6+ feature validation
//
// # Basic Usage
//
// Create and Start LSP Manager:
//
//	import (
//	    "github.com/zeroroot-ai/sdk/codegen/lsp"
//	    "log/slog"
//	)
//
//	// Create manager with default config
//	config := lsp.DefaultLSPConfig()
//	logger := slog.Default()
//	manager := lsp.NewLSPManager(config, logger)
//
//	// Start language servers for workspace
//	if err := manager.Start(ctx, workspaceRoot); err != nil {
//	    return fmt.Errorf("failed to start LSP: %w", err)
//	}
//	defer manager.Stop(ctx)
//
//	// Wait for servers to be ready
//	if err := manager.WaitForReady(ctx); err != nil {
//	    return fmt.Errorf("LSP initialization failed: %w", err)
//	}
//
// Get Diagnostics for a File:
//
//	manager := lsp.NewLSPManager(config, logger)
//	manager.Start(ctx, workspaceRoot)
//	defer manager.Stop(ctx)
//	manager.WaitForReady(ctx)
//
//	// Get diagnostics for a specific file
//	filePath := filepath.Join(workspaceRoot, "main.go")
//	diagnostics, err := manager.GetDiagnostics(ctx, filePath)
//	if err != nil {
//	    return err
//	}
//
//	// Check for errors
//	for _, diag := range diagnostics {
//	    if diag.IsError() {
//	        log.Printf("Error at line %d: %s", diag.Line, diag.Message)
//	    }
//	}
//
// # Configuration
//
// Default Configuration:
//
//	config := lsp.DefaultLSPConfig()
//	// All languages enabled
//	// 30s initialization timeout
//	// 10s validation timeout
//	// Server binaries searched in PATH
//
// Custom Configuration:
//
//	config := lsp.LSPConfig{
//	    // Specify custom binary paths
//	    GoplsPath:            "/usr/local/bin/gopls",
//	    PyrightPath:          "/usr/local/bin/pyright-langserver",
//	    TypeScriptServerPath: "/usr/local/bin/typescript-language-server",
//
//	    // Configure timeouts
//	    InitTimeout:       30 * time.Second,
//	    ValidationTimeout: 10 * time.Second,
//
//	    // Enable specific languages
//	    EnableGo:         true,
//	    EnablePython:     true,
//	    EnableTypeScript: true,
//	}
//	manager := lsp.NewLSPManager(config, logger)
//
// Enable Only Go:
//
//	config := lsp.LSPConfig{
//	    EnableGo:         true,
//	    EnablePython:     false,
//	    EnableTypeScript: false,
//	    InitTimeout:      30 * time.Second,
//	    ValidationTimeout: 10 * time.Second,
//	}
//
// Longer Timeouts for Large Projects:
//
//	config := lsp.DefaultLSPConfig()
//	config.InitTimeout = 60 * time.Second       // Large workspace
//	config.ValidationTimeout = 30 * time.Second // Complex analysis
//
// # Integration with Editor
//
// The editor automatically uses LSP when configured:
//
//	// LSP is configured in workspace settings
//	editor := workspace.Editor()
//
//	// Apply edit - automatically validates with LSP
//	result, err := editor.Apply(ctx, edit)
//
//	// Check diagnostics
//	if result.HasErrors() {
//	    for _, diag := range result.Diagnostics {
//	        log.Printf("[%s] %s:%d - %s",
//	            diag.Severity,
//	            diag.Path,
//	            diag.Line,
//	            diag.Message)
//	    }
//	}
//
// Manual Validation:
//
//	// Validate a file without making changes
//	editor := workspace.Editor()
//	diagnostics, err := editor.Validate(ctx, "main.go")
//	if err != nil {
//	    return err
//	}
//
//	if len(diagnostics) > 0 {
//	    log.Printf("File has %d diagnostic(s)", len(diagnostics))
//	}
//
// # Diagnostic Types
//
// Diagnostics come in four severity levels:
//
// Error (SeverityError):
//
//	Fatal issues that prevent compilation or execution.
//	Examples:
//	  - Syntax errors
//	  - Undefined variables
//	  - Type mismatches
//	  - Missing imports
//
//	if diag.IsError() {
//	    log.Printf("ERROR: %s", diag.Message)
//	}
//
// Warning (SeverityWarning):
//
//	Non-fatal issues that should be addressed.
//	Examples:
//	  - Unused variables
//	  - Deprecated API usage
//	  - Potential bugs
//	  - Code smells
//
//	if diag.IsWarning() {
//	    log.Printf("WARNING: %s", diag.Message)
//	}
//
// Info (SeverityInfo):
//
//	Informational messages.
//	Examples:
//	  - Code optimization suggestions
//	  - Best practice recommendations
//
// Hint (SeverityHint):
//
//	Style suggestions and optional improvements.
//	Examples:
//	  - Formatting suggestions
//	  - Naming conventions
//	  - Simplification opportunities
//
// # Diagnostic Information
//
// Each diagnostic contains:
//
//	type Diagnostic struct {
//	    Path      string              // File path
//	    Line      int                 // Starting line (1-based)
//	    Column    int                 // Starting column (1-based)
//	    EndLine   int                 // Ending line
//	    EndColumn int                 // Ending column
//	    Severity  DiagnosticSeverity  // Error, Warning, Info, Hint
//	    Message   string              // Human-readable message
//	    Source    string              // Server that produced it (e.g., "gopls")
//	}
//
// Filter Errors Only:
//
//	var errors []codegen.Diagnostic
//	for _, diag := range diagnostics {
//	    if diag.IsError() {
//	        errors = append(errors, diag)
//	    }
//	}
//	if len(errors) > 0 {
//	    return fmt.Errorf("code has %d errors", len(errors))
//	}
//
// Group by Severity:
//
//	errorCount := 0
//	warningCount := 0
//	for _, diag := range diagnostics {
//	    switch diag.Severity {
//	    case codegen.SeverityError:
//	        errorCount++
//	    case codegen.SeverityWarning:
//	        warningCount++
//	    }
//	}
//	log.Printf("Found %d errors, %d warnings", errorCount, warningCount)
//
// # Language Server Lifecycle
//
// Startup:
//
//  1. Manager detects project languages (by file extensions)
//  2. Starts appropriate language servers
//  3. Initializes each server with workspace root
//  4. Waits for "initialized" notification
//  5. Ready to serve diagnostic requests
//
// Runtime:
//
//  1. Editor makes changes to files
//  2. Manager requests diagnostics for changed files
//  3. Language server analyzes code
//  4. Returns diagnostics within timeout
//  5. Editor processes diagnostics
//
// Shutdown:
//
//  1. Manager sends shutdown request to each server
//  2. Waits for graceful shutdown
//  3. Terminates server processes
//  4. Cleans up resources
//
// # Error Handling
//
// Server Not Found:
//
//	Causes:
//	  - Language server binary not installed
//	  - Binary not in PATH
//	  - Incorrect path in config
//
//	Solutions:
//	  - Install language server (see installation commands above)
//	  - Add binary to PATH
//	  - Specify full path in LSPConfig
//	  - Disable language if not needed
//
// Timeout Errors (ErrLSPTimeout):
//
//	Causes:
//	  - Large file taking too long to analyze
//	  - Language server is overloaded
//	  - Server is unresponsive
//
//	Solutions:
//	  - Increase ValidationTimeout in config
//	  - Restart language server
//	  - Check server logs for issues
//	  - Disable LSP if not critical
//
// Initialization Failed:
//
//	Causes:
//	  - Invalid workspace root
//	  - Server crashed during startup
//	  - Incompatible server version
//
//	Solutions:
//	  - Verify workspace root exists and is accessible
//	  - Check server version compatibility
//	  - Review server logs for errors
//	  - Try with DefaultLSPConfig()
//
// # Performance Considerations
//
// Server Startup Time:
//
//   - Go (gopls): ~2-5 seconds
//
//   - Python (pyright): ~3-8 seconds
//
//   - TypeScript: ~5-10 seconds
//
//     Tips:
//
//   - Start servers early in mission initialization
//
//   - Keep servers running for duration of mission
//
//   - Use WaitForReady() to ensure servers are initialized
//
// Validation Time:
//
//   - Small file (<100 lines): ~100-500ms
//
//   - Medium file (100-1000 lines): ~500ms-2s
//
//   - Large file (>1000 lines): ~2-5s
//
//     Tips:
//
//   - Set appropriate ValidationTimeout (10-30s)
//
//   - Language servers cache analysis results
//
//   - Subsequent validations are faster
//
// Resource Usage:
//
//	Per language server:
//	  - Memory: 50-200MB
//	  - CPU: Varies with analysis complexity
//	  - Disk: Minimal (caches in memory)
//
//	Tips:
//	  - Enable only needed languages
//	  - Stop servers when done (defer manager.Stop())
//	  - Monitor resource usage in production
//
// # Advanced Usage
//
// Check Supported Languages:
//
//	manager := lsp.NewLSPManager(config, logger)
//	manager.Start(ctx, workspaceRoot)
//	defer manager.Stop(ctx)
//
//	languages := manager.SupportedLanguages()
//	log.Printf("LSP supports: %v", languages)
//	// Output: [go python typescript]
//
// Validate Multiple Files:
//
//	manager := lsp.NewLSPManager(config, logger)
//	manager.Start(ctx, workspaceRoot)
//	defer manager.Stop(ctx)
//	manager.WaitForReady(ctx)
//
//	files := []string{
//	    "main.go",
//	    "handler.go",
//	    "utils.go",
//	}
//
//	var allDiagnostics []codegen.Diagnostic
//	for _, file := range files {
//	    filePath := filepath.Join(workspaceRoot, file)
//	    diags, err := manager.GetDiagnostics(ctx, filePath)
//	    if err != nil {
//	        log.Printf("Failed to validate %s: %v", file, err)
//	        continue
//	    }
//	    allDiagnostics = append(allDiagnostics, diags...)
//	}
//
//	log.Printf("Total diagnostics: %d", len(allDiagnostics))
//
// Conditional Validation:
//
//	// Only validate if LSP is available
//	editor := workspace.Editor()
//
//	result, err := editor.Apply(ctx, edit)
//	if err != nil {
//	    return err
//	}
//
//	// Check if validation was performed
//	if len(result.Diagnostics) == 0 {
//	    log.Println("No LSP validation (server not available)")
//	} else {
//	    log.Printf("LSP validation: %d diagnostics", len(result.Diagnostics))
//	}
//
// # Language-Specific Notes
//
// Go (gopls):
//
//	Requirements:
//	  - go.mod file in workspace root
//	  - Valid Go module structure
//
//	Common diagnostics:
//	  - "undeclared name: X"
//	  - "cannot use X as Y value"
//	  - "imported and not used: X"
//
//	Tips:
//	  - Ensure go.mod exists
//	  - Run "go mod tidy" before validation
//	  - gopls caches module information
//
// Python (pyright):
//
//	Requirements:
//	  - Python files (.py)
//	  - Optional: pyrightconfig.json or pyproject.toml
//
//	Common diagnostics:
//	  - "X is not defined"
//	  - "Argument of type X cannot be assigned to Y"
//	  - "Import X could not be resolved"
//
//	Tips:
//	  - Use type hints for better validation
//	  - Configure Python version in pyrightconfig.json
//	  - Virtual environment affects import resolution
//
// TypeScript/JavaScript (tsserver):
//
//	Requirements:
//	  - package.json in workspace root
//	  - Optional: tsconfig.json for TypeScript
//
//	Common diagnostics:
//	  - "Cannot find name 'X'"
//	  - "Type 'X' is not assignable to type 'Y'"
//	  - "Module 'X' has no exported member 'Y'"
//
//	Tips:
//	  - Run "npm install" before validation
//	  - Configure TypeScript options in tsconfig.json
//	  - JavaScript validation is more lenient than TypeScript
//
// # Mission Configuration
//
// Enable LSP in mission YAML:
//
//	workspace:
//	  settings:
//	    lsp_enabled: true
//	    lsp_timeout: 10s
//
//	This automatically:
//	  - Starts language servers during initialization
//	  - Enables validation in editor operations
//	  - Stops servers during cleanup
//
// Disable LSP for specific missions:
//
//	workspace:
//	  settings:
//	    lsp_enabled: false
//
//	Useful when:
//	  - Language servers not available
//	  - Fast iteration without validation
//	  - Working with unsupported languages
//
// # Best Practices
//
// Always Use Defer for Cleanup:
//
//	manager := lsp.NewLSPManager(config, logger)
//	if err := manager.Start(ctx, workspaceRoot); err != nil {
//	    return err
//	}
//	defer manager.Stop(ctx) // Ensure cleanup
//
// Wait for Initialization:
//
//	manager.Start(ctx, workspaceRoot)
//	defer manager.Stop(ctx)
//
//	// Always wait for ready before validation
//	if err := manager.WaitForReady(ctx); err != nil {
//	    return err
//	}
//
//	// Now safe to get diagnostics
//	diagnostics, err := manager.GetDiagnostics(ctx, filePath)
//
// Handle Timeouts Gracefully:
//
//	diagnostics, err := manager.GetDiagnostics(ctx, filePath)
//	if errors.Is(err, codegen.ErrLSPTimeout) {
//	    log.Println("Validation timed out, proceeding without validation")
//	    // Continue with operation
//	} else if err != nil {
//	    return err
//	}
//
// Configure Appropriate Timeouts:
//
//	Small projects:
//	  InitTimeout: 20s
//	  ValidationTimeout: 5s
//
//	Medium projects:
//	  InitTimeout: 30s
//	  ValidationTimeout: 10s
//
//	Large projects:
//	  InitTimeout: 60s
//	  ValidationTimeout: 30s
//
// # Thread Safety
//
// The LSP manager is safe for concurrent use:
//
//	// Safe: Concurrent diagnostics for different files
//	go manager.GetDiagnostics(ctx, "file1.go")
//	go manager.GetDiagnostics(ctx, "file2.go")
//
//	// Safe: Multiple goroutines using same manager
//	for _, file := range files {
//	    go func(f string) {
//	        manager.GetDiagnostics(ctx, f)
//	    }(file)
//	}
//
// Internal locking ensures safe concurrent access to language servers.
//
// # Troubleshooting
//
// Server Not Starting:
//
//  1. Check if binary is installed:
//     which gopls
//     which pyright-langserver
//     which typescript-language-server
//
//  2. Test binary manually:
//     gopls version
//     pyright-langserver --version
//     typescript-language-server --version
//
//  3. Check logs for error messages
//
//  4. Try with full path in config
//
// No Diagnostics Returned:
//
//  1. Ensure WaitForReady() was called
//  2. Check file path is correct (absolute path)
//  3. Verify file extension matches enabled language
//  4. Check server logs for errors
//  5. Increase ValidationTimeout
//
// Slow Validation:
//
//  1. Increase ValidationTimeout
//  2. Check system resources (CPU, memory)
//  3. Restart language server
//  4. Reduce workspace size
//  5. Enable only needed languages
//
// # Examples
//
// See example_test.go for detailed examples:
//   - Example: Basic LSP manager usage
//   - ExampleLSPConfig: Configuration options
//   - ExampleLSPManager_GetDiagnostics: Diagnostic retrieval
//   - ExampleDefaultLSPConfig: Default configuration
//
// See integration_test.go for complete missions:
//   - TestGoCodeWithLSP: Go code validation with gopls
//   - TestPythonCodeWithLSP: Python validation with pyright
//   - TestTypeScriptCodeWithLSP: TypeScript validation with tsserver
//   - TestLSPValidationIntegration: Full validation mission
package lsp
