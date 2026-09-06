// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Zero Root AI

package lsp_test

import (
	"context"
	"fmt"
	"log"
	"log/slog"
	"os"
	"path/filepath"
	"time"

	"github.com/zeroroot-ai/sdk/codegen/lsp"
)

// Example demonstrates basic usage of the LSP manager.
func Example() {
	// Create a temporary workspace
	tempDir, err := os.MkdirTemp("", "lsp-example-*")
	if err != nil {
		log.Fatal(err)
	}
	defer os.RemoveAll(tempDir)

	// Create a Go file with an error
	goFile := filepath.Join(tempDir, "example.go")
	code := `package main

func main() {
	undefinedFunction()
}
`
	if err := os.WriteFile(goFile, []byte(code), 0644); err != nil {
		log.Fatal(err)
	}

	// Create go.mod
	goMod := filepath.Join(tempDir, "go.mod")
	if err := os.WriteFile(goMod, []byte("module example\n\ngo 1.21\n"), 0644); err != nil {
		log.Fatal(err)
	}

	// Create LSP manager
	config := lsp.DefaultLSPConfig()
	config.ValidationTimeout = 15 * time.Second

	manager := lsp.NewLSPManager(config, slog.Default())

	ctx := context.Background()

	// Start language servers
	if err := manager.Start(ctx, tempDir); err != nil {
		log.Printf("Failed to start LSP manager (gopls may not be installed): %v", err)
		return
	}
	defer manager.Stop(ctx)

	// Wait for servers to be ready
	if err := manager.WaitForReady(ctx); err != nil {
		log.Fatal(err)
	}

	// Get diagnostics for the file
	diagnostics, err := manager.GetDiagnostics(ctx, goFile)
	if err != nil {
		log.Fatal(err)
	}

	// Print diagnostics
	fmt.Printf("Found %d diagnostic(s):\n", len(diagnostics))
	for _, d := range diagnostics {
		fmt.Printf("  [%s] Line %d: %s\n", d.Severity, d.Line, d.Message)
	}
}

// ExampleLSPConfig demonstrates configuration options.
func ExampleLSPConfig() {
	config := lsp.LSPConfig{
		// Specify custom gopls path (optional)
		GoplsPath: "/usr/local/bin/gopls",

		// Configure timeouts
		InitTimeout:       30 * time.Second,
		ValidationTimeout: 10 * time.Second,

		// Enable only Go validation
		EnableGo:         true,
		EnablePython:     false,
		EnableTypeScript: false,
	}

	manager := lsp.NewLSPManager(config, slog.Default())
	fmt.Printf("Created manager with custom config: %v\n", manager.SupportedLanguages())
}

// ExampleLSPManager_GetDiagnostics demonstrates diagnostic retrieval.
func ExampleLSPManager_GetDiagnostics() {
	// Create a temporary workspace
	tempDir, err := os.MkdirTemp("", "lsp-diagnostics-*")
	if err != nil {
		log.Fatal(err)
	}
	defer os.RemoveAll(tempDir)

	// Create a valid Go file
	goFile := filepath.Join(tempDir, "valid.go")
	code := `package main

import "fmt"

func main() {
	fmt.Println("Hello, World!")
}
`
	if err := os.WriteFile(goFile, []byte(code), 0644); err != nil {
		log.Fatal(err)
	}

	// Create go.mod
	goMod := filepath.Join(tempDir, "go.mod")
	if err := os.WriteFile(goMod, []byte("module example\n\ngo 1.21\n"), 0644); err != nil {
		log.Fatal(err)
	}

	// Setup manager
	manager := lsp.NewLSPManager(lsp.DefaultLSPConfig(), slog.Default())
	ctx := context.Background()

	if err := manager.Start(ctx, tempDir); err != nil {
		log.Printf("Failed to start LSP manager: %v", err)
		return
	}
	defer manager.Stop(ctx)

	if err := manager.WaitForReady(ctx); err != nil {
		log.Fatal(err)
	}

	// Get diagnostics
	diagnostics, err := manager.GetDiagnostics(ctx, goFile)
	if err != nil {
		log.Fatal(err)
	}

	// Check for errors
	hasErrors := false
	for _, d := range diagnostics {
		if d.IsError() {
			hasErrors = true
			fmt.Printf("Error at line %d: %s\n", d.Line, d.Message)
		}
	}

	if !hasErrors {
		fmt.Println("No errors found - code is valid!")
	}
}

// ExampleDefaultLSPConfig shows the default configuration.
func ExampleDefaultLSPConfig() {
	config := lsp.DefaultLSPConfig()

	fmt.Printf("Init Timeout: %v\n", config.InitTimeout)
	fmt.Printf("Validation Timeout: %v\n", config.ValidationTimeout)
	fmt.Printf("Go Enabled: %v\n", config.EnableGo)
	fmt.Printf("Python Enabled: %v\n", config.EnablePython)
	fmt.Printf("TypeScript Enabled: %v\n", config.EnableTypeScript)

	// Output:
	// Init Timeout: 30s
	// Validation Timeout: 10s
	// Go Enabled: true
	// Python Enabled: true
	// TypeScript Enabled: true
}
