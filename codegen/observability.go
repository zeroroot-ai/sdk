// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Zero Root AI

package codegen

import (
	"context"
	"log/slog"
	"strings"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
	"go.opentelemetry.io/otel/trace"
)

// Package-level tracer and meter for codegen instrumentation.
var (
	tracer trace.Tracer
	meter  metric.Meter
)

func init() {
	tracer = otel.Tracer("github.com/zeroroot-ai/sdk/codegen")
	meter = otel.Meter("github.com/zeroroot-ai/sdk/codegen")
}

// Metric attribute key constants for consistent labeling.
const (
	// MetricAttrOperation identifies the operation type (e.g., "clone", "commit", "apply")
	MetricAttrOperation = "operation"

	// MetricAttrStatus represents the outcome status (e.g., "success", "error")
	MetricAttrStatus = "status"

	// MetricAttrMatchType distinguishes between exact, fuzzy, or failed matches
	MetricAttrMatchType = "match_type"

	// MetricAttrLanguage identifies the programming language
	MetricAttrLanguage = "language"

	// MetricAttrSeverity represents diagnostic severity (error, warning, info)
	MetricAttrSeverity = "severity"

	// MetricAttrValidationStatus represents validation outcome (passed, warnings, failed)
	MetricAttrValidationStatus = "validation_status"
)

// CodegenMetrics holds all metric instruments for codegen operations.
type CodegenMetrics struct {
	// Counters track cumulative counts
	WorkspaceOperationsTotal metric.Int64Counter
	EditorEditsTotal         metric.Int64Counter
	GitOperationsTotal       metric.Int64Counter
	LSPValidationsTotal      metric.Int64Counter
	LSPDiagnosticsTotal      metric.Int64Counter

	// Histograms track distributions of values
	WorkspaceCloneDurationSeconds metric.Float64Histogram
	EditorEditDurationSeconds     metric.Float64Histogram
	GitOperationDurationSeconds   metric.Float64Histogram
	LSPValidationDurationSeconds  metric.Float64Histogram
}

// NewCodegenMetrics creates and registers all metric instruments.
// Returns a no-op recorder if meter is nil.
func NewCodegenMetrics() (*CodegenMetrics, error) {
	if meter == nil {
		slog.Warn("nil MeterProvider in codegen, returning no-op metrics")
		return &CodegenMetrics{}, nil
	}

	m := &CodegenMetrics{}
	var err error

	// Create counters
	m.WorkspaceOperationsTotal, err = meter.Int64Counter(
		"codegen.workspace.operations.total",
		metric.WithDescription("Total workspace operations"),
	)
	if err != nil {
		return nil, err
	}

	m.EditorEditsTotal, err = meter.Int64Counter(
		"codegen.editor.edits.total",
		metric.WithDescription("Total editor edit operations"),
	)
	if err != nil {
		return nil, err
	}

	m.GitOperationsTotal, err = meter.Int64Counter(
		"codegen.git.operations.total",
		metric.WithDescription("Total git operations"),
	)
	if err != nil {
		return nil, err
	}

	m.LSPValidationsTotal, err = meter.Int64Counter(
		"codegen.lsp.validations.total",
		metric.WithDescription("Total LSP validation operations"),
	)
	if err != nil {
		return nil, err
	}

	m.LSPDiagnosticsTotal, err = meter.Int64Counter(
		"codegen.lsp.diagnostics.total",
		metric.WithDescription("Total LSP diagnostics reported"),
	)
	if err != nil {
		return nil, err
	}

	// Create histograms with appropriate bucket boundaries
	// Workspace clone: 1s to 5 minutes (typical range for git clone)
	m.WorkspaceCloneDurationSeconds, err = meter.Float64Histogram(
		"codegen.workspace.clone.duration.seconds",
		metric.WithDescription("Workspace clone duration distribution"),
		metric.WithUnit("s"),
		metric.WithExplicitBucketBoundaries(1, 5, 10, 30, 60, 120, 300),
	)
	if err != nil {
		return nil, err
	}

	// Editor edits: 10ms to 10s (typical range for file edits)
	m.EditorEditDurationSeconds, err = meter.Float64Histogram(
		"codegen.editor.edit.duration.seconds",
		metric.WithDescription("Editor edit operation duration distribution"),
		metric.WithUnit("s"),
		metric.WithExplicitBucketBoundaries(0.01, 0.05, 0.1, 0.5, 1, 5, 10),
	)
	if err != nil {
		return nil, err
	}

	// Git operations: 100ms to 30s (typical range for git commands)
	m.GitOperationDurationSeconds, err = meter.Float64Histogram(
		"codegen.git.operation.duration.seconds",
		metric.WithDescription("Git operation duration distribution"),
		metric.WithUnit("s"),
		metric.WithExplicitBucketBoundaries(0.1, 0.5, 1, 5, 10, 30),
	)
	if err != nil {
		return nil, err
	}

	// LSP validation: 100ms to 10s (typical range for LSP diagnostics)
	m.LSPValidationDurationSeconds, err = meter.Float64Histogram(
		"codegen.lsp.validation.duration.seconds",
		metric.WithDescription("LSP validation duration distribution"),
		metric.WithUnit("s"),
		metric.WithExplicitBucketBoundaries(0.1, 0.5, 1, 2, 5, 10),
	)
	if err != nil {
		return nil, err
	}

	slog.Debug("created codegen metrics with all instruments")
	return m, nil
}

// NoopCodegenMetrics returns a no-op metrics recorder.
func NoopCodegenMetrics() *CodegenMetrics {
	return &CodegenMetrics{}
}

// StartSpan is a helper to start a span with common attributes.
func StartSpan(ctx context.Context, name string, attrs ...attribute.KeyValue) (context.Context, trace.Span) {
	if tracer == nil {
		// Return no-op span
		return ctx, trace.SpanFromContext(ctx)
	}
	return tracer.Start(ctx, name, trace.WithAttributes(attrs...))
}

// RecordSuccess records a successful operation with metrics.
func (m *CodegenMetrics) RecordSuccess(ctx context.Context, counter metric.Int64Counter, operation string, attrs ...attribute.KeyValue) {
	if counter == nil {
		return
	}

	allAttrs := append([]attribute.KeyValue{
		attribute.String(MetricAttrOperation, operation),
		attribute.String(MetricAttrStatus, "success"),
	}, attrs...)

	counter.Add(ctx, 1, metric.WithAttributes(allAttrs...))
}

// RecordError records a failed operation with metrics.
func (m *CodegenMetrics) RecordError(ctx context.Context, counter metric.Int64Counter, operation string, attrs ...attribute.KeyValue) {
	if counter == nil {
		return
	}

	allAttrs := append([]attribute.KeyValue{
		attribute.String(MetricAttrOperation, operation),
		attribute.String(MetricAttrStatus, "error"),
	}, attrs...)

	counter.Add(ctx, 1, metric.WithAttributes(allAttrs...))
}

// RecordDuration records operation duration in a histogram.
func (m *CodegenMetrics) RecordDuration(ctx context.Context, histogram metric.Float64Histogram, durationSeconds float64, attrs ...attribute.KeyValue) {
	if histogram == nil {
		return
	}

	histogram.Record(ctx, durationSeconds, metric.WithAttributes(attrs...))
}

// SanitizeFilePath removes sensitive information from file paths.
// Converts absolute paths to relative paths from workspace root.
func SanitizeFilePath(path, workspaceRoot string) string {
	if workspaceRoot != "" && strings.HasPrefix(path, workspaceRoot) {
		// Strip workspace root, keep relative path
		rel := strings.TrimPrefix(path, workspaceRoot)
		rel = strings.TrimPrefix(rel, "/")
		return rel
	}

	// Return last component only for absolute paths
	if strings.Contains(path, "/") {
		parts := strings.Split(path, "/")
		return parts[len(parts)-1]
	}

	return path
}

// SanitizeRepoURL removes credentials from repository URLs.
func SanitizeRepoURL(url string) string {
	// Handle HTTPS URLs with credentials (e.g., https://user:pass@github.com/repo)
	if strings.HasPrefix(url, "https://") || strings.HasPrefix(url, "http://") {
		// Find @ symbol
		atIndex := strings.Index(url, "@")
		if atIndex == -1 {
			return url
		}

		// Find protocol separator
		protocolEnd := strings.Index(url, "://")
		if protocolEnd == -1 || atIndex <= protocolEnd+3 {
			return url
		}

		// Replace credentials with ***:***
		protocol := url[:protocolEnd+3]
		hostPath := url[atIndex:]
		return protocol + "***:***" + hostPath
	}

	// SSH URLs don't typically contain credentials
	return url
}

// MatchTypeToString converts a match type to a string label.
func MatchTypeToString(mt MatchType) string {
	switch mt {
	case MatchExact:
		return "exact"
	case MatchFuzzy:
		return "fuzzy"
	case MatchFailed:
		return "failed"
	default:
		return "unknown"
	}
}

// ValidationStatusToString converts a validation status to a string label.
func ValidationStatusToString(vs ValidationStatus) string {
	switch vs {
	case ValidationPassed:
		return "passed"
	case ValidationWarnings:
		return "warnings"
	case ValidationFailed:
		return "failed"
	default:
		return "unknown"
	}
}
