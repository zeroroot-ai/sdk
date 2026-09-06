// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Zero Root AI

package lsp

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/metric"

	"github.com/zeroroot-ai/sdk/codegen"
)

// languageServer is an internal interface for language server clients.
type languageServer interface {
	openDocument(ctx context.Context, filePath, content string) error
	closeDocument(filePath string) error
	getDiagnostics(ctx context.Context, filePath string, timeout time.Duration) ([]codegen.Diagnostic, error)
	waitForReady(ctx context.Context) error
	shutdown(ctx context.Context) error
}

// lspManager implements LSPManager interface.
type lspManager struct {
	config        LSPConfig
	workspaceRoot string
	servers       map[string]languageServer
	serversMutex  sync.RWMutex
	logger        *slog.Logger
	started       bool
	metrics       *codegen.CodegenMetrics
}

// NewLSPManager creates a new LSP manager with the given configuration.
func NewLSPManager(config LSPConfig, logger *slog.Logger) LSPManager {
	if logger == nil {
		logger = slog.Default()
	}

	// Initialize metrics (non-fatal if it fails)
	metrics, err := codegen.NewCodegenMetrics()
	if err != nil {
		logger.Warn("failed to initialize codegen metrics", "error", err)
		metrics = codegen.NoopCodegenMetrics()
	}

	return &lspManager{
		config:  config,
		servers: make(map[string]languageServer),
		logger:  logger.With("component", "lsp-manager"),
		metrics: metrics,
	}
}

// Start initializes and starts language servers for the given workspace.
func (m *lspManager) Start(ctx context.Context, workspaceRoot string) error {
	startTime := time.Now()
	ctx, span := codegen.StartSpan(ctx, "codegen.lsp.start",
		attribute.String("workspace.root", workspaceRoot))
	defer span.End()

	m.serversMutex.Lock()
	defer m.serversMutex.Unlock()

	if m.started {
		span.SetStatus(codes.Error, "already started")
		return errors.New("lsp manager already started")
	}

	// Validate workspace root
	absRoot, err := filepath.Abs(workspaceRoot)
	if err != nil {
		return fmt.Errorf("invalid workspace root: %w", err)
	}

	stat, err := os.Stat(absRoot)
	if err != nil {
		return fmt.Errorf("workspace root not accessible: %w", err)
	}
	if !stat.IsDir() {
		return fmt.Errorf("workspace root is not a directory: %s", absRoot)
	}

	m.workspaceRoot = absRoot

	// Apply default timeouts if not set
	if m.config.InitTimeout == 0 {
		m.config.InitTimeout = 30 * time.Second
	}
	if m.config.ValidationTimeout == 0 {
		m.config.ValidationTimeout = 10 * time.Second
	}

	// Start enabled language servers
	var startErrors []error

	// Start gopls if enabled
	if m.config.EnableGo {
		if err := m.startGopls(ctx); err != nil {
			startErrors = append(startErrors, fmt.Errorf("failed to start gopls: %w", err))
			m.logger.Error("failed to start gopls", "error", err)
		} else {
			m.logger.Info("started gopls", "workspace", absRoot)
		}
	}

	// Start pyright if enabled
	if m.config.EnablePython {
		if err := m.startPyright(ctx); err != nil {
			startErrors = append(startErrors, fmt.Errorf("failed to start pyright: %w", err))
			m.logger.Error("failed to start pyright", "error", err)
		} else {
			m.logger.Info("started pyright", "workspace", absRoot)
		}
	}

	// Start typescript-language-server if enabled
	if m.config.EnableTypeScript {
		if err := m.startTsserver(ctx); err != nil {
			startErrors = append(startErrors, fmt.Errorf("failed to start typescript-language-server: %w", err))
			m.logger.Error("failed to start typescript-language-server", "error", err)
		} else {
			m.logger.Info("started typescript-language-server", "workspace", absRoot)
		}
	}

	if len(startErrors) > 0 && len(m.servers) == 0 {
		// All language servers failed to start
		duration := time.Since(startTime).Seconds()
		span.SetStatus(codes.Error, "all servers failed to start")
		m.metrics.RecordDuration(ctx, m.metrics.LSPValidationDurationSeconds, duration,
			attribute.String(codegen.MetricAttrOperation, "start"))
		return fmt.Errorf("all language servers failed to start: %v", startErrors)
	}

	duration := time.Since(startTime).Seconds()
	m.started = true
	span.SetStatus(codes.Ok, "LSP started")
	span.SetAttributes(attribute.Int("server.count", len(m.servers)))
	m.metrics.RecordDuration(ctx, m.metrics.LSPValidationDurationSeconds, duration,
		attribute.String(codegen.MetricAttrOperation, "start"))
	return nil
}

// startGopls starts the gopls language server.
func (m *lspManager) startGopls(ctx context.Context) error {
	initCtx, cancel := context.WithTimeout(ctx, m.config.InitTimeout)
	defer cancel()

	client, err := newGoplsClient(initCtx, m.config.GoplsPath, m.workspaceRoot, m.logger)
	if err != nil {
		return err
	}

	// Wait for initialization
	if err := client.waitForReady(initCtx); err != nil {
		client.shutdown(context.Background())
		return fmt.Errorf("gopls initialization timeout: %w", err)
	}

	m.servers["go"] = client
	return nil
}

// startPyright starts the pyright language server.
func (m *lspManager) startPyright(ctx context.Context) error {
	initCtx, cancel := context.WithTimeout(ctx, m.config.InitTimeout)
	defer cancel()

	client, err := newPyrightClient(initCtx, m.config.PyrightPath, m.workspaceRoot, m.logger)
	if err != nil {
		return err
	}

	// Wait for initialization
	if err := client.waitForReady(initCtx); err != nil {
		client.shutdown(context.Background())
		return fmt.Errorf("pyright initialization timeout: %w", err)
	}

	m.servers["python"] = client
	return nil
}

// startTsserver starts the typescript-language-server.
func (m *lspManager) startTsserver(ctx context.Context) error {
	initCtx, cancel := context.WithTimeout(ctx, m.config.InitTimeout)
	defer cancel()

	client, err := newTsserverClient(initCtx, m.config.TypeScriptServerPath, m.workspaceRoot, m.logger)
	if err != nil {
		return err
	}

	// Wait for initialization
	if err := client.waitForReady(initCtx); err != nil {
		client.shutdown(context.Background())
		return fmt.Errorf("typescript-language-server initialization timeout: %w", err)
	}

	// TypeScript server handles both TypeScript and JavaScript
	m.servers["typescript"] = client
	m.servers["javascript"] = client
	return nil
}

// Stop gracefully shuts down all running language servers.
func (m *lspManager) Stop(ctx context.Context) error {
	ctx, span := codegen.StartSpan(ctx, "codegen.lsp.stop")
	defer span.End()

	m.serversMutex.Lock()
	defer m.serversMutex.Unlock()

	if !m.started {
		span.SetStatus(codes.Ok, "not started")
		return nil
	}

	var shutdownErrors []error

	// Shutdown all servers
	for lang, server := range m.servers {
		if err := server.shutdown(ctx); err != nil {
			shutdownErrors = append(shutdownErrors, fmt.Errorf("%s: %w", lang, err))
			m.logger.Error("failed to shutdown language server", "language", lang, "error", err)
		} else {
			m.logger.Info("stopped language server", "language", lang)
		}
	}

	m.servers = make(map[string]languageServer)
	m.started = false

	if len(shutdownErrors) > 0 {
		span.SetStatus(codes.Error, "shutdown had errors")
		return fmt.Errorf("errors during shutdown: %v", shutdownErrors)
	}

	span.SetStatus(codes.Ok, "LSP stopped")
	return nil
}

// GetDiagnostics retrieves diagnostics for the specified file path.
func (m *lspManager) GetDiagnostics(ctx context.Context, path string) ([]codegen.Diagnostic, error) {
	startTime := time.Now()
	ctx, span := codegen.StartSpan(ctx, "codegen.lsp.get_diagnostics",
		attribute.String("file.path", codegen.SanitizeFilePath(path, m.workspaceRoot)))
	defer span.End()

	m.serversMutex.RLock()
	defer m.serversMutex.RUnlock()

	if !m.started {
		span.SetStatus(codes.Error, "not started")
		m.metrics.RecordError(ctx, m.metrics.LSPValidationsTotal, "get_diagnostics")
		return nil, errors.New("lsp manager not started")
	}

	// Determine file path (make absolute if relative)
	absPath := path
	if !filepath.IsAbs(path) {
		absPath = filepath.Join(m.workspaceRoot, path)
	}

	// Check if file exists
	if _, err := os.Stat(absPath); err != nil {
		return nil, fmt.Errorf("file not found: %w", err)
	}

	// Determine language from file extension
	lang := m.detectLanguage(absPath)
	if lang == "" {
		span.SetStatus(codes.Error, "unsupported file type")
		m.metrics.RecordError(ctx, m.metrics.LSPValidationsTotal, "get_diagnostics")
		return nil, fmt.Errorf("unsupported file type: %s", filepath.Ext(absPath))
	}

	span.SetAttributes(attribute.String(codegen.MetricAttrLanguage, lang))

	// Get the appropriate language server
	server, ok := m.servers[lang]
	if !ok {
		span.SetStatus(codes.Error, "no language server")
		m.metrics.RecordError(ctx, m.metrics.LSPValidationsTotal, "get_diagnostics",
			attribute.String(codegen.MetricAttrLanguage, lang))
		return nil, fmt.Errorf("no language server available for %s", lang)
	}

	// Read file content
	content, err := os.ReadFile(absPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read file: %w", err)
	}

	// Create timeout context
	diagCtx, cancel := context.WithTimeout(ctx, m.config.ValidationTimeout)
	defer cancel()

	// Open document
	if err := server.openDocument(diagCtx, absPath, string(content)); err != nil {
		return nil, fmt.Errorf("failed to open document: %w", err)
	}

	// Get diagnostics (with timeout)
	diagnostics, err := server.getDiagnostics(diagCtx, absPath, m.config.ValidationTimeout)

	// Close document
	if closeErr := server.closeDocument(absPath); closeErr != nil {
		m.logger.Warn("failed to close document", "path", absPath, "error", closeErr)
	}

	duration := time.Since(startTime).Seconds()

	if err != nil {
		span.SetStatus(codes.Error, "diagnostics failed")
		span.RecordError(err)
		m.metrics.RecordError(ctx, m.metrics.LSPValidationsTotal, "get_diagnostics",
			attribute.String(codegen.MetricAttrLanguage, lang))
		m.metrics.RecordDuration(ctx, m.metrics.LSPValidationDurationSeconds, duration,
			attribute.String(codegen.MetricAttrLanguage, lang))
		if errors.Is(err, context.DeadlineExceeded) {
			return nil, ErrLSPTimeout
		}
		return nil, err
	}

	// Count diagnostics by severity
	errorCount, warningCount := countBySeverity(diagnostics)

	span.SetStatus(codes.Ok, "diagnostics retrieved")
	span.SetAttributes(
		attribute.Int("diagnostic.count", len(diagnostics)),
		attribute.Int("diagnostic.error_count", errorCount),
		attribute.Int("diagnostic.warning_count", warningCount))

	m.metrics.RecordSuccess(ctx, m.metrics.LSPValidationsTotal, "get_diagnostics",
		attribute.String(codegen.MetricAttrLanguage, lang))
	m.metrics.RecordDuration(ctx, m.metrics.LSPValidationDurationSeconds, duration,
		attribute.String(codegen.MetricAttrLanguage, lang))

	// Record individual diagnostic counts by severity
	if errorCount > 0 {
		m.metrics.LSPDiagnosticsTotal.Add(ctx, int64(errorCount),
			metric.WithAttributes(
				attribute.String(codegen.MetricAttrLanguage, lang),
				attribute.String(codegen.MetricAttrSeverity, "error")))
	}
	if warningCount > 0 {
		m.metrics.LSPDiagnosticsTotal.Add(ctx, int64(warningCount),
			metric.WithAttributes(
				attribute.String(codegen.MetricAttrLanguage, lang),
				attribute.String(codegen.MetricAttrSeverity, "warning")))
	}

	return diagnostics, nil
}

// countBySeverity tallies error and warning diagnostics for span
// attributes and metrics. Every server adapter (gopls, pyright,
// tsserver) maps raw LSP DiagnosticSeverity wire values (1=Error,
// 2=Warning, 3=Information, 4=Hint) to the codegen.DiagnosticSeverity
// enum at ingest, so Severity is always an enum member here.
// Info/Hint are deliberately uncounted.
func countBySeverity(diagnostics []codegen.Diagnostic) (errorCount, warningCount int) {
	for _, d := range diagnostics {
		//nolint:exhaustive // only Error/Warning feed the span attributes and metrics
		switch d.Severity {
		case codegen.SeverityError:
			errorCount++
		case codegen.SeverityWarning:
			warningCount++
		}
	}
	return errorCount, warningCount
}

// WaitForReady blocks until all language servers are initialized and ready.
func (m *lspManager) WaitForReady(ctx context.Context) error {
	m.serversMutex.RLock()
	defer m.serversMutex.RUnlock()

	if !m.started {
		return errors.New("lsp manager not started")
	}

	// Wait for all servers to be ready
	for lang, server := range m.servers {
		if err := server.waitForReady(ctx); err != nil {
			return fmt.Errorf("language server %s not ready: %w", lang, err)
		}
	}

	return nil
}

// SupportedLanguages returns the list of programming languages this manager can validate.
func (m *lspManager) SupportedLanguages() []string {
	m.serversMutex.RLock()
	defer m.serversMutex.RUnlock()

	languages := make([]string, 0, len(m.servers))
	for lang := range m.servers {
		languages = append(languages, lang)
	}

	return languages
}

// detectLanguage detects the programming language from a file path.
func (m *lspManager) detectLanguage(path string) string {
	ext := strings.ToLower(filepath.Ext(path))

	switch ext {
	case ".go":
		if _, ok := m.servers["go"]; ok {
			return "go"
		}
	case ".py":
		if _, ok := m.servers["python"]; ok {
			return "python"
		}
	case ".ts", ".tsx":
		if _, ok := m.servers["typescript"]; ok {
			return "typescript"
		}
	case ".js", ".jsx":
		if _, ok := m.servers["javascript"]; ok {
			return "javascript"
		}
	}

	return ""
}
