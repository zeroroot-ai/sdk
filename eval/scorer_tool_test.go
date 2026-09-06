// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Zero Root AI

package eval

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestToolCorrectnessScorer_PerfectMatch(t *testing.T) {
	scorer := NewToolCorrectnessScorer(ToolCorrectnessOptions{})

	sample := Sample{
		ID: "test-1",
		ExpectedTools: []ExpectedToolCall{
			{
				Name: "mytool",
				Arguments: map[string]any{
					"target": "192.168.1.1",
					"ports":  "80,443",
				},
				Required: true,
			},
			{
				Name: "http-client",
				Arguments: map[string]any{
					"url":    "https://example.com",
					"method": "GET",
				},
				Required: true,
			},
		},
		Trajectory: Trajectory{
			Steps: []TrajectoryStep{
				{
					Type: "tool",
					Name: "mytool",
					Input: map[string]any{
						"target": "192.168.1.1",
						"ports":  "80,443",
					},
					StartTime: time.Now(),
					Duration:  1 * time.Second,
				},
				{
					Type: "tool",
					Name: "http-client",
					Input: map[string]any{
						"url":    "https://example.com",
						"method": "GET",
					},
					StartTime: time.Now(),
					Duration:  500 * time.Millisecond,
				},
			},
		},
	}

	result, err := scorer.Score(context.Background(), sample)
	require.NoError(t, err)
	assert.Equal(t, 1.0, result.Score, "Perfect match should score 1.0")
	assert.Equal(t, 2, result.Details["matched"])
	assert.Equal(t, 0, result.Details["missing"])
	assert.Equal(t, 0, result.Details["extra"])
	assert.Equal(t, 0, result.Details["mismatched"])
}

func TestToolCorrectnessScorer_MissingTools(t *testing.T) {
	scorer := NewToolCorrectnessScorer(ToolCorrectnessOptions{})

	sample := Sample{
		ID: "test-2",
		ExpectedTools: []ExpectedToolCall{
			{Name: "mytool", Required: true},
			{Name: "http-client", Required: true},
			{Name: "sqlmap", Required: true},
		},
		Trajectory: Trajectory{
			Steps: []TrajectoryStep{
				{Type: "tool", Name: "mytool", StartTime: time.Now(), Duration: 1 * time.Second},
			},
		},
	}

	result, err := scorer.Score(context.Background(), sample)
	require.NoError(t, err)

	// Score = matched / max(required, actual) = 1 / max(3, 1) = 1/3 = 0.333...
	assert.InDelta(t, 0.333, result.Score, 0.01, "Should penalize missing tools")
	assert.Equal(t, 1, result.Details["matched"])
	assert.Equal(t, 2, result.Details["missing"])

	missingTools := result.Details["missing_tools"].([]string)
	assert.ElementsMatch(t, []string{"http-client", "sqlmap"}, missingTools)
}

func TestToolCorrectnessScorer_ExtraTools(t *testing.T) {
	scorer := NewToolCorrectnessScorer(ToolCorrectnessOptions{})

	sample := Sample{
		ID: "test-3",
		ExpectedTools: []ExpectedToolCall{
			{Name: "mytool", Required: true},
		},
		Trajectory: Trajectory{
			Steps: []TrajectoryStep{
				{Type: "tool", Name: "mytool", StartTime: time.Now(), Duration: 1 * time.Second},
				{Type: "tool", Name: "http-client", StartTime: time.Now(), Duration: 500 * time.Millisecond},
				{Type: "tool", Name: "sqlmap", StartTime: time.Now(), Duration: 2 * time.Second},
			},
		},
	}

	result, err := scorer.Score(context.Background(), sample)
	require.NoError(t, err)

	// Score = matched / max(required, actual) = 1 / max(1, 3) = 1/3 = 0.333...
	assert.InDelta(t, 0.333, result.Score, 0.01, "Should penalize extra tools")
	assert.Equal(t, 1, result.Details["matched"])
	assert.Equal(t, 2, result.Details["extra"])

	extraTools := result.Details["extra_tools"].([]string)
	assert.ElementsMatch(t, []string{"http-client", "sqlmap"}, extraTools)
}

func TestToolCorrectnessScorer_ArgumentMismatch(t *testing.T) {
	scorer := NewToolCorrectnessScorer(ToolCorrectnessOptions{})

	sample := Sample{
		ID: "test-4",
		ExpectedTools: []ExpectedToolCall{
			{
				Name: "mytool",
				Arguments: map[string]any{
					"target": "192.168.1.1",
					"ports":  "80,443",
				},
				Required: true,
			},
		},
		Trajectory: Trajectory{
			Steps: []TrajectoryStep{
				{
					Type: "tool",
					Name: "mytool",
					Input: map[string]any{
						"target": "192.168.1.2", // Wrong target
						"ports":  "80,443",
					},
					StartTime: time.Now(),
					Duration:  1 * time.Second,
				},
			},
		},
	}

	result, err := scorer.Score(context.Background(), sample)
	require.NoError(t, err)

	// Tool called but with wrong args counts as mismatch
	assert.Equal(t, 0.0, result.Score, "Argument mismatch should result in 0 score")
	assert.Equal(t, 0, result.Details["matched"])
	assert.Equal(t, 1, result.Details["mismatched"])

	mismatches := result.Details["mismatched_tools"].([]map[string]any)
	assert.Len(t, mismatches, 1)
	assert.Equal(t, "mytool", mismatches[0]["tool"])
	assert.Equal(t, "arguments mismatch", mismatches[0]["reason"])
}

func TestToolCorrectnessScorer_NumericTolerance(t *testing.T) {
	scorer := NewToolCorrectnessScorer(ToolCorrectnessOptions{
		NumericTolerance: 0.01,
	})

	sample := Sample{
		ID: "test-5",
		ExpectedTools: []ExpectedToolCall{
			{
				Name: "measure",
				Arguments: map[string]any{
					"threshold": 0.5,
				},
				Required: true,
			},
		},
		Trajectory: Trajectory{
			Steps: []TrajectoryStep{
				{
					Type: "tool",
					Name: "measure",
					Input: map[string]any{
						"threshold": 0.505, // Within tolerance
					},
					StartTime: time.Now(),
					Duration:  100 * time.Millisecond,
				},
			},
		},
	}

	result, err := scorer.Score(context.Background(), sample)
	require.NoError(t, err)
	assert.Equal(t, 1.0, result.Score, "Should accept values within numeric tolerance")
	assert.Equal(t, 1, result.Details["matched"])
}

func TestToolCorrectnessScorer_NumericToleranceExceeded(t *testing.T) {
	scorer := NewToolCorrectnessScorer(ToolCorrectnessOptions{
		NumericTolerance: 0.01,
	})

	sample := Sample{
		ID: "test-6",
		ExpectedTools: []ExpectedToolCall{
			{
				Name: "measure",
				Arguments: map[string]any{
					"threshold": 0.5,
				},
				Required: true,
			},
		},
		Trajectory: Trajectory{
			Steps: []TrajectoryStep{
				{
					Type: "tool",
					Name: "measure",
					Input: map[string]any{
						"threshold": 0.52, // Exceeds tolerance
					},
					StartTime: time.Now(),
					Duration:  100 * time.Millisecond,
				},
			},
		},
	}

	result, err := scorer.Score(context.Background(), sample)
	require.NoError(t, err)
	assert.Equal(t, 0.0, result.Score, "Should reject values outside tolerance")
	assert.Equal(t, 1, result.Details["mismatched"])
}

func TestToolCorrectnessScorer_OrderMatters(t *testing.T) {
	scorer := NewToolCorrectnessScorer(ToolCorrectnessOptions{
		OrderMatters: true,
	})

	sample := Sample{
		ID: "test-7",
		ExpectedTools: []ExpectedToolCall{
			{Name: "mytool", Required: true},
			{Name: "http-client", Required: true},
			{Name: "sqlmap", Required: true},
		},
		Trajectory: Trajectory{
			Steps: []TrajectoryStep{
				{Type: "tool", Name: "mytool", StartTime: time.Now(), Duration: 1 * time.Second},
				{Type: "tool", Name: "http-client", StartTime: time.Now(), Duration: 500 * time.Millisecond},
				{Type: "tool", Name: "sqlmap", StartTime: time.Now(), Duration: 2 * time.Second},
			},
		},
	}

	result, err := scorer.Score(context.Background(), sample)
	require.NoError(t, err)
	assert.Equal(t, 1.0, result.Score, "Correct order should score perfectly")
	assert.Equal(t, 3, result.Details["matched"])
}

func TestToolCorrectnessScorer_OrderMattersWrongOrder(t *testing.T) {
	scorer := NewToolCorrectnessScorer(ToolCorrectnessOptions{
		OrderMatters: true,
	})

	sample := Sample{
		ID: "test-8",
		ExpectedTools: []ExpectedToolCall{
			{Name: "mytool", Required: true},
			{Name: "http-client", Required: true},
			{Name: "sqlmap", Required: true},
		},
		Trajectory: Trajectory{
			Steps: []TrajectoryStep{
				{Type: "tool", Name: "http-client", StartTime: time.Now(), Duration: 500 * time.Millisecond},
				{Type: "tool", Name: "mytool", StartTime: time.Now(), Duration: 1 * time.Second},
				{Type: "tool", Name: "sqlmap", StartTime: time.Now(), Duration: 2 * time.Second},
			},
		},
	}

	result, err := scorer.Score(context.Background(), sample)
	require.NoError(t, err)

	// First tool (mytool) expected, but http-client came first - it's marked as extra
	// Then mytool found and matched
	// Then http-client expected but already passed - missing
	// Then sqlmap matched
	assert.Less(t, result.Score, 1.0, "Wrong order should reduce score")
	assert.Equal(t, 2, result.Details["matched"]) // mytool and sqlmap
	assert.Equal(t, 1, result.Details["extra"])   // http-client out of order
	assert.Equal(t, 1, result.Details["missing"]) // http-client in correct position
}

func TestToolCorrectnessScorer_OptionalTools(t *testing.T) {
	scorer := NewToolCorrectnessScorer(ToolCorrectnessOptions{})

	sample := Sample{
		ID: "test-9",
		ExpectedTools: []ExpectedToolCall{
			{Name: "mytool", Required: true},
			{Name: "http-client", Required: false}, // Optional
		},
		Trajectory: Trajectory{
			Steps: []TrajectoryStep{
				{Type: "tool", Name: "mytool", StartTime: time.Now(), Duration: 1 * time.Second},
				// http-client not called, but it's optional
			},
		},
	}

	result, err := scorer.Score(context.Background(), sample)
	require.NoError(t, err)

	// Score = matched / max(required, actual) = 1 / max(1, 1) = 1.0
	assert.Equal(t, 1.0, result.Score, "Missing optional tools shouldn't penalize")
	assert.Equal(t, 1, result.Details["matched"])
	assert.Equal(t, 0, result.Details["missing"]) // Optional tools don't show as missing
}

func TestToolCorrectnessScorer_NoToolsExpectedOrCalled(t *testing.T) {
	scorer := NewToolCorrectnessScorer(ToolCorrectnessOptions{})

	sample := Sample{
		ID:            "test-10",
		ExpectedTools: []ExpectedToolCall{},
		Trajectory: Trajectory{
			Steps: []TrajectoryStep{
				{Type: "llm", Name: "primary", StartTime: time.Now(), Duration: 2 * time.Second},
			},
		},
	}

	result, err := scorer.Score(context.Background(), sample)
	require.NoError(t, err)
	assert.Equal(t, 1.0, result.Score, "No tools expected or called should score perfectly")
}

func TestToolCorrectnessScorer_MixedStepTypes(t *testing.T) {
	scorer := NewToolCorrectnessScorer(ToolCorrectnessOptions{})

	sample := Sample{
		ID: "test-11",
		ExpectedTools: []ExpectedToolCall{
			{Name: "mytool", Required: true},
		},
		Trajectory: Trajectory{
			Steps: []TrajectoryStep{
				{Type: "llm", Name: "primary", StartTime: time.Now(), Duration: 1 * time.Second},
				{Type: "tool", Name: "mytool", StartTime: time.Now(), Duration: 2 * time.Second},
				{Type: "memory", Name: "working", StartTime: time.Now(), Duration: 100 * time.Millisecond},
				{Type: "finding", Name: "vuln-1", StartTime: time.Now(), Duration: 50 * time.Millisecond},
			},
		},
	}

	result, err := scorer.Score(context.Background(), sample)
	require.NoError(t, err)
	assert.Equal(t, 1.0, result.Score, "Should only consider tool steps")
	assert.Equal(t, 1, result.Details["matched"])
}

func TestToolCorrectnessScorer_OptionsOverrideSample(t *testing.T) {
	scorer := NewToolCorrectnessScorer(ToolCorrectnessOptions{
		ExpectedTools: []ExpectedToolCall{
			{Name: "custom-tool", Required: true},
		},
	})

	sample := Sample{
		ID: "test-12",
		ExpectedTools: []ExpectedToolCall{
			{Name: "mytool", Required: true}, // Should be ignored
		},
		Trajectory: Trajectory{
			Steps: []TrajectoryStep{
				{Type: "tool", Name: "custom-tool", StartTime: time.Now(), Duration: 1 * time.Second},
			},
		},
	}

	result, err := scorer.Score(context.Background(), sample)
	require.NoError(t, err)
	assert.Equal(t, 1.0, result.Score, "Options should override sample expected tools")
	assert.Equal(t, 1, result.Details["matched"])
}

func TestToolCorrectnessScorer_IntegerTypeConversion(t *testing.T) {
	scorer := NewToolCorrectnessScorer(ToolCorrectnessOptions{
		NumericTolerance: 0.1,
	})

	sample := Sample{
		ID: "test-13",
		ExpectedTools: []ExpectedToolCall{
			{
				Name: "scanner",
				Arguments: map[string]any{
					"port": 80, // int
				},
				Required: true,
			},
		},
		Trajectory: Trajectory{
			Steps: []TrajectoryStep{
				{
					Type: "tool",
					Name: "scanner",
					Input: map[string]any{
						"port": 80.0, // float64
					},
					StartTime: time.Now(),
					Duration:  1 * time.Second,
				},
			},
		},
	}

	result, err := scorer.Score(context.Background(), sample)
	require.NoError(t, err)
	assert.Equal(t, 1.0, result.Score, "Should handle int/float conversions")
	assert.Equal(t, 1, result.Details["matched"])
}

func TestToolCorrectnessScorer_Name(t *testing.T) {
	scorer := NewToolCorrectnessScorer(ToolCorrectnessOptions{})
	assert.Equal(t, "tool_correctness", scorer.Name())
}
