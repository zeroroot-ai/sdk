// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Zero Root AI

package lsp

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"os/exec"
	"path/filepath"
	"sync"
	"time"

	"github.com/zeroroot-ai/sdk/codegen"
)

// goplsClient wraps the base LSP client with gopls-specific functionality.
type goplsClient struct {
	*lspClient
	workspaceRoot string
	diagnostics   map[string][]codegen.Diagnostic
	diagMutex     sync.RWMutex
	readyCh       chan struct{}
}

// newGoplsClient creates a new gopls client for the given workspace.
func newGoplsClient(ctx context.Context, goplsPath, workspaceRoot string, logger *slog.Logger) (*goplsClient, error) {
	// Find gopls binary
	if goplsPath == "" {
		path, err := exec.LookPath("gopls")
		if err != nil {
			return nil, fmt.Errorf("gopls not found in PATH: %w", err)
		}
		goplsPath = path
	}

	// Create command
	cmd := exec.CommandContext(ctx, goplsPath, "serve")

	// Create base client
	baseClient, err := newLSPClient(cmd, logger.With("component", "gopls"))
	if err != nil {
		return nil, err
	}

	client := &goplsClient{
		lspClient:     baseClient,
		workspaceRoot: workspaceRoot,
		diagnostics:   make(map[string][]codegen.Diagnostic),
		readyCh:       make(chan struct{}),
	}

	// Register notification handlers
	baseClient.onNotification("textDocument/publishDiagnostics", client.handlePublishDiagnostics)

	// Perform LSP initialization handshake
	if err := client.initialize(ctx); err != nil {
		baseClient.close()
		return nil, fmt.Errorf("initialization failed: %w", err)
	}

	return client, nil
}

// initialize performs the LSP initialize handshake.
func (c *goplsClient) initialize(ctx context.Context) error {
	// Send initialize request
	initParams := map[string]interface{}{
		"processId": nil, // null means don't track parent process
		"rootUri":   "file://" + c.workspaceRoot,
		"capabilities": map[string]interface{}{
			"textDocument": map[string]interface{}{
				"publishDiagnostics": map[string]interface{}{
					"relatedInformation":     true,
					"versionSupport":         false,
					"codeDescriptionSupport": true,
					"dataSupport":            true,
				},
			},
		},
		"workspaceFolders": []map[string]interface{}{
			{
				"uri":  "file://" + c.workspaceRoot,
				"name": filepath.Base(c.workspaceRoot),
			},
		},
	}

	result, err := c.sendRequest(ctx, "initialize", initParams)
	if err != nil {
		return fmt.Errorf("initialize request failed: %w", err)
	}

	// Parse server capabilities (optional, for logging)
	var initResult map[string]interface{}
	if err := json.Unmarshal(result, &initResult); err == nil {
		c.logger.Debug("gopls initialized", "capabilities", initResult["capabilities"])
	}

	// Send initialized notification
	if err := c.sendNotification("initialized", map[string]interface{}{}); err != nil {
		return fmt.Errorf("initialized notification failed: %w", err)
	}

	c.initialized.Store(true)
	close(c.readyCh)

	return nil
}

// handlePublishDiagnostics processes textDocument/publishDiagnostics notifications.
func (c *goplsClient) handlePublishDiagnostics(params json.RawMessage) {
	var diagnosticsParams struct {
		URI         string `json:"uri"`
		Version     int    `json:"version"`
		Diagnostics []struct {
			Range struct {
				Start struct {
					Line      int `json:"line"`
					Character int `json:"character"`
				} `json:"start"`
				End struct {
					Line      int `json:"line"`
					Character int `json:"character"`
				} `json:"end"`
			} `json:"range"`
			Severity int    `json:"severity"`
			Source   string `json:"source"`
			Message  string `json:"message"`
		} `json:"diagnostics"`
	}

	if err := json.Unmarshal(params, &diagnosticsParams); err != nil {
		c.logger.Error("failed to parse publishDiagnostics", "error", err)
		return
	}

	// Convert URI to file path (strip "file://" prefix)
	filePath := diagnosticsParams.URI
	if len(filePath) > 7 && filePath[:7] == "file://" {
		filePath = filePath[7:]
	}

	// Convert LSP diagnostics to our format
	diagnostics := make([]codegen.Diagnostic, 0, len(diagnosticsParams.Diagnostics))
	for _, d := range diagnosticsParams.Diagnostics {
		severity := codegen.SeverityInfo
		switch d.Severity {
		case 1:
			severity = codegen.SeverityError
		case 2:
			severity = codegen.SeverityWarning
		case 3:
			severity = codegen.SeverityInfo
		case 4:
			severity = codegen.SeverityHint
		}

		diagnostics = append(diagnostics, codegen.Diagnostic{
			Path:      filePath,
			Line:      d.Range.Start.Line + 1,      // LSP is 0-based, our API is 1-based
			Column:    d.Range.Start.Character + 1, // LSP is 0-based, our API is 1-based
			EndLine:   d.Range.End.Line + 1,
			EndColumn: d.Range.End.Character + 1,
			Severity:  severity,
			Message:   d.Message,
			Source:    "gopls",
		})
	}

	// Store diagnostics
	c.diagMutex.Lock()
	c.diagnostics[filePath] = diagnostics
	c.diagMutex.Unlock()

	c.logger.Debug("received diagnostics",
		"file", filePath,
		"count", len(diagnostics))
}

// openDocument notifies gopls that a document has been opened.
func (c *goplsClient) openDocument(ctx context.Context, filePath, content string) error {
	if !c.initialized.Load() {
		return ErrLSPNotInitialized
	}

	params := map[string]interface{}{
		"textDocument": map[string]interface{}{
			"uri":        "file://" + filePath,
			"languageId": "go",
			"version":    1,
			"text":       content,
		},
	}

	return c.sendNotification("textDocument/didOpen", params)
}

// closeDocument notifies gopls that a document has been closed.
func (c *goplsClient) closeDocument(filePath string) error {
	if !c.initialized.Load() {
		return ErrLSPNotInitialized
	}

	params := map[string]interface{}{
		"textDocument": map[string]interface{}{
			"uri": "file://" + filePath,
		},
	}

	return c.sendNotification("textDocument/didClose", params)
}

// getDiagnostics retrieves diagnostics for a specific file with timeout.
// It waits for diagnostics to be published by gopls after opening the document.
func (c *goplsClient) getDiagnostics(ctx context.Context, filePath string, timeout time.Duration) ([]codegen.Diagnostic, error) {
	if !c.initialized.Load() {
		return nil, ErrLSPNotInitialized
	}

	// Clear any existing diagnostics for this file
	c.diagMutex.Lock()
	delete(c.diagnostics, filePath)
	c.diagMutex.Unlock()

	// Wait for diagnostics with timeout
	deadline := time.Now().Add(timeout)
	ticker := time.NewTicker(50 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return nil, ErrLSPTimeout
		case <-ticker.C:
			c.diagMutex.RLock()
			diagnostics, exists := c.diagnostics[filePath]
			c.diagMutex.RUnlock()

			if exists {
				// Diagnostics received
				return diagnostics, nil
			}

			if time.Now().After(deadline) {
				// Timeout - return empty diagnostics (not an error, might be no issues)
				return []codegen.Diagnostic{}, nil
			}
		}
	}
}

// waitForReady blocks until the gopls client is initialized and ready.
func (c *goplsClient) waitForReady(ctx context.Context) error {
	select {
	case <-c.readyCh:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

// shutdown performs graceful shutdown of the gopls client.
func (c *goplsClient) shutdown(ctx context.Context) error {
	if !c.initialized.Load() {
		return c.close()
	}

	// Send shutdown request
	_, err := c.sendRequest(ctx, "shutdown", nil)
	if err != nil {
		c.logger.Warn("shutdown request failed", "error", err)
	}

	// Send exit notification
	if err := c.sendNotification("exit", nil); err != nil {
		c.logger.Warn("exit notification failed", "error", err)
	}

	// Close the client
	return c.close()
}
