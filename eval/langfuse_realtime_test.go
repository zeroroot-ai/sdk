// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Zero Root AI

package eval

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestRealTimeExportOptions_DefaultMinConfidence verifies that EnableRealTimeExport
// sets a default MinConfidence of 0.5 when not specified.
func TestRealTimeExportOptions_DefaultMinConfidence(t *testing.T) {
	exporter := NewLangfuseExporter(LangfuseOptions{
		BaseURL:   "https://cloud.langfuse.com",
		PublicKey: "test-public",
		SecretKey: "test-secret",
	})
	defer exporter.Close()

	// Enable with zero MinConfidence - should default to 0.5
	exporter.EnableRealTimeExport(RealTimeExportOptions{
		ExportPartialScores: true,
		MinConfidence:       0,
	})

	require.NotNil(t, exporter.realTimeConfig)
	assert.True(t, exporter.realTimeConfig.enabled)
	assert.Equal(t, 0.5, exporter.realTimeConfig.minConfidence)
}

// TestRealTimeExportOptions_CustomMinConfidence verifies that EnableRealTimeExport
// respects custom MinConfidence values.
func TestRealTimeExportOptions_CustomMinConfidence(t *testing.T) {
	exporter := NewLangfuseExporter(LangfuseOptions{
		BaseURL:   "https://cloud.langfuse.com",
		PublicKey: "test-public",
		SecretKey: "test-secret",
	})
	defer exporter.Close()

	exporter.EnableRealTimeExport(RealTimeExportOptions{
		ExportPartialScores: true,
		MinConfidence:       0.75,
	})

	require.NotNil(t, exporter.realTimeConfig)
	assert.True(t, exporter.realTimeConfig.enabled)
	assert.Equal(t, 0.75, exporter.realTimeConfig.minConfidence)
}

// TestExportPartialScore_Disabled verifies that ExportPartialScore is a no-op
// when real-time export is not enabled.
func TestExportPartialScore_Disabled(t *testing.T) {
	exporter := NewLangfuseExporter(LangfuseOptions{
		BaseURL:   "https://cloud.langfuse.com",
		PublicKey: "test-public",
		SecretKey: "test-secret",
	})
	defer exporter.Close()

	// Do not enable real-time export
	ctx := context.Background()
	score := PartialScore{
		Score:      0.8,
		Confidence: 0.9,
		Status:     ScoreStatusPartial,
	}

	err := exporter.ExportPartialScore(ctx, "trace-123", "tool_correctness", score)
	assert.NoError(t, err, "ExportPartialScore should not error when disabled")

	// Queue should be empty
	assert.Empty(t, exporter.partialScoreQueue)
}

// TestExportPartialScore_BelowConfidenceThreshold verifies that scores below
// the confidence threshold are not exported.
func TestExportPartialScore_BelowConfidenceThreshold(t *testing.T) {
	exporter := NewLangfuseExporter(LangfuseOptions{
		BaseURL:   "https://cloud.langfuse.com",
		PublicKey: "test-public",
		SecretKey: "test-secret",
	})
	defer exporter.Close()

	exporter.EnableRealTimeExport(RealTimeExportOptions{
		ExportPartialScores: true,
		MinConfidence:       0.7,
	})

	ctx := context.Background()
	score := PartialScore{
		Score:      0.8,
		Confidence: 0.6, // Below threshold of 0.7
		Status:     ScoreStatusPartial,
	}

	err := exporter.ExportPartialScore(ctx, "trace-123", "tool_correctness", score)
	assert.NoError(t, err, "ExportPartialScore should not error for low confidence")

	// Queue should be empty (score filtered out)
	assert.Empty(t, exporter.partialScoreQueue)
}

// TestExportPartialScore_AboveConfidenceThreshold verifies that scores above
// the confidence threshold are queued for export.
func TestExportPartialScore_AboveConfidenceThreshold(t *testing.T) {
	exporter := NewLangfuseExporter(LangfuseOptions{
		BaseURL:   "https://cloud.langfuse.com",
		PublicKey: "test-public",
		SecretKey: "test-secret",
	})
	defer exporter.Close()

	exporter.EnableRealTimeExport(RealTimeExportOptions{
		ExportPartialScores: true,
		MinConfidence:       0.7,
	})

	ctx := context.Background()
	score := PartialScore{
		Score:      0.8,
		Confidence: 0.9, // Above threshold of 0.7
		Status:     ScoreStatusPartial,
	}

	err := exporter.ExportPartialScore(ctx, "trace-123", "tool_correctness", score)
	assert.NoError(t, err)

	// Queue should have one item
	assert.Len(t, exporter.partialScoreQueue, 1)
}

// TestExportPartialScore_Closed verifies that ExportPartialScore returns an error
// when called on a closed exporter.
func TestExportPartialScore_Closed(t *testing.T) {
	exporter := NewLangfuseExporter(LangfuseOptions{
		BaseURL:   "https://cloud.langfuse.com",
		PublicKey: "test-public",
		SecretKey: "test-secret",
	})

	exporter.EnableRealTimeExport(RealTimeExportOptions{
		ExportPartialScores: true,
		MinConfidence:       0.5,
	})

	// Close the exporter
	err := exporter.Close()
	require.NoError(t, err)

	ctx := context.Background()
	score := PartialScore{
		Score:      0.8,
		Confidence: 0.9,
		Status:     ScoreStatusPartial,
	}

	err = exporter.ExportPartialScore(ctx, "trace-123", "tool_correctness", score)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "closed")
}
