// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Zero Root AI

package eval

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/zeroroot-ai/sdk/agent"
)

func TestLoadEvalSet_JSON(t *testing.T) {
	// Create a temporary JSON file
	tmpDir := t.TempDir()
	jsonPath := filepath.Join(tmpDir, "test.json")

	jsonContent := `{
		"name": "test-eval-set",
		"version": "1.0.0",
		"samples": [
			{
				"id": "sample-1",
				"task": {
					"id": "sample-1",
					"objective": "bypass filter",
					"context": {"target": "http://example.com"}
				},
				"tags": ["injection", "basic"]
			},
			{
				"id": "sample-2",
				"task": {
					"id": "sample-2",
					"objective": "jailbreak chatbot",
					"context": {"model": "gpt-4"}
				},
				"tags": ["jailbreak", "advanced"]
			}
		],
		"metadata": {
			"author": "test-author",
			"created": "2025-01-05"
		}
	}`

	err := os.WriteFile(jsonPath, []byte(jsonContent), 0644)
	require.NoError(t, err)

	// Load the eval set
	evalSet, err := LoadEvalSet(jsonPath)
	require.NoError(t, err)
	assert.NotNil(t, evalSet)

	// Verify loaded data
	assert.Equal(t, "test-eval-set", evalSet.Name)
	assert.Equal(t, "1.0.0", evalSet.Version)
	assert.Len(t, evalSet.Samples, 2)
	assert.Equal(t, "sample-1", evalSet.Samples[0].ID)
	assert.Equal(t, "sample-1", evalSet.Samples[0].Task.ID)
	assert.Equal(t, []string{"injection", "basic"}, evalSet.Samples[0].Tags)
	assert.Equal(t, "test-author", evalSet.Metadata["author"])
}

func TestLoadEvalSet_YAML(t *testing.T) {
	// Create a temporary YAML file
	tmpDir := t.TempDir()
	yamlPath := filepath.Join(tmpDir, "test.yaml")

	yamlContent := `name: test-eval-set
version: 1.0.0
samples:
  - id: sample-1
    task:
      id: sample-1
      objective: bypass filter
      context:
        target: http://example.com
    tags:
      - injection
      - basic
  - id: sample-2
    task:
      id: sample-2
      objective: jailbreak chatbot
      context:
        model: gpt-4
    tags:
      - jailbreak
      - advanced
metadata:
  author: test-author
  created: 2025-01-05
`

	err := os.WriteFile(yamlPath, []byte(yamlContent), 0644)
	require.NoError(t, err)

	// Load the eval set
	evalSet, err := LoadEvalSet(yamlPath)
	require.NoError(t, err)
	assert.NotNil(t, evalSet)

	// Verify loaded data
	assert.Equal(t, "test-eval-set", evalSet.Name)
	assert.Equal(t, "1.0.0", evalSet.Version)
	assert.Len(t, evalSet.Samples, 2)
	assert.Equal(t, "sample-1", evalSet.Samples[0].ID)
	assert.Equal(t, "sample-1", evalSet.Samples[0].Task.ID)
	assert.Equal(t, []string{"injection", "basic"}, evalSet.Samples[0].Tags)
}

func TestLoadEvalSet_YMLExtension(t *testing.T) {
	// Create a temporary .yml file
	tmpDir := t.TempDir()
	ymlPath := filepath.Join(tmpDir, "test.yml")

	ymlContent := `name: test-eval-set
version: 1.0.0
samples:
  - id: sample-1
    task:
      id: sample-1
      objective: test goal
`

	err := os.WriteFile(ymlPath, []byte(ymlContent), 0644)
	require.NoError(t, err)

	// Load the eval set
	evalSet, err := LoadEvalSet(ymlPath)
	require.NoError(t, err)
	assert.NotNil(t, evalSet)
	assert.Equal(t, "test-eval-set", evalSet.Name)
}

func TestLoadEvalSet_FileNotFound(t *testing.T) {
	evalSet, err := LoadEvalSet("/nonexistent/path/to/file.json")
	assert.Error(t, err)
	assert.Nil(t, evalSet)
	assert.Contains(t, err.Error(), "not found")
}

func TestLoadEvalSet_UnsupportedFormat(t *testing.T) {
	tmpDir := t.TempDir()
	txtPath := filepath.Join(tmpDir, "test.txt")

	err := os.WriteFile(txtPath, []byte("some content"), 0644)
	require.NoError(t, err)

	evalSet, err := LoadEvalSet(txtPath)
	assert.Error(t, err)
	assert.Nil(t, evalSet)
	assert.Contains(t, err.Error(), "unsupported eval set format")
	assert.Contains(t, err.Error(), ".txt")
}

func TestLoadEvalSet_MalformedJSON(t *testing.T) {
	tmpDir := t.TempDir()
	jsonPath := filepath.Join(tmpDir, "malformed.json")

	malformedJSON := `{
		"name": "test",
		"samples": [
			{"id": "sample-1"  // missing closing brace
		]
	}`

	err := os.WriteFile(jsonPath, []byte(malformedJSON), 0644)
	require.NoError(t, err)

	evalSet, err := LoadEvalSet(jsonPath)
	assert.Error(t, err)
	assert.Nil(t, evalSet)
	assert.Contains(t, err.Error(), "failed to parse JSON")
}

func TestLoadEvalSet_MalformedYAML(t *testing.T) {
	tmpDir := t.TempDir()
	yamlPath := filepath.Join(tmpDir, "malformed.yaml")

	malformedYAML := `name: test
samples:
  - id: sample-1
    task:
      type: test
    invalid indentation here
`

	err := os.WriteFile(yamlPath, []byte(malformedYAML), 0644)
	require.NoError(t, err)

	evalSet, err := LoadEvalSet(yamlPath)
	assert.Error(t, err)
	assert.Nil(t, evalSet)
	assert.Contains(t, err.Error(), "failed to parse YAML")
}

func TestLoadEvalSet_EmptySet(t *testing.T) {
	tmpDir := t.TempDir()
	jsonPath := filepath.Join(tmpDir, "empty.json")

	emptyJSON := `{
		"name": "empty-set",
		"version": "1.0.0",
		"samples": []
	}`

	err := os.WriteFile(jsonPath, []byte(emptyJSON), 0644)
	require.NoError(t, err)

	// Empty sets are valid
	evalSet, err := LoadEvalSet(jsonPath)
	require.NoError(t, err)
	assert.NotNil(t, evalSet)
	assert.Equal(t, "empty-set", evalSet.Name)
	assert.Empty(t, evalSet.Samples)
}

func TestValidate_MissingID(t *testing.T) {
	evalSet := &EvalSet{
		Name:    "test",
		Version: "1.0.0",
		Samples: []Sample{
			{
				ID: "sample-1",
				Task: agent.Task{
					ID:      "task-id",
					Context: map[string]any{"objective": "test goal"},
				},
			},
			{
				// Missing ID
				Task: agent.Task{
					ID:      "task-id",
					Context: map[string]any{"objective": "test goal"},
				},
			},
		},
	}

	err := evalSet.Validate()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "missing required field 'id'")
	assert.Contains(t, err.Error(), "index 1")
}

func TestValidate_MissingTaskID(t *testing.T) {
	evalSet := &EvalSet{
		Name:    "test",
		Version: "1.0.0",
		Samples: []Sample{
			{
				ID: "sample-1",
				Task: agent.Task{
					// Missing ID
					Context: map[string]any{"objective": "test goal"},
				},
			},
		},
	}

	err := evalSet.Validate()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "missing required field 'task.id'")
	assert.Contains(t, err.Error(), "sample-1")
}

func TestValidate_DuplicateIDs(t *testing.T) {
	evalSet := &EvalSet{
		Name:    "test",
		Version: "1.0.0",
		Samples: []Sample{
			{
				ID: "duplicate-id",
				Task: agent.Task{
					ID:      "task-id",
					Context: map[string]any{"objective": "test goal"},
				},
			},
			{
				ID: "unique-id",
				Task: agent.Task{
					ID:      "task-id",
					Context: map[string]any{"objective": "test goal"},
				},
			},
			{
				ID: "duplicate-id", // Duplicate
				Task: agent.Task{
					ID:      "task-id",
					Context: map[string]any{"objective": "test goal"},
				},
			},
		},
	}

	err := evalSet.Validate()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "duplicate sample ID")
	assert.Contains(t, err.Error(), "duplicate-id")
}

func TestValidate_Valid(t *testing.T) {
	evalSet := &EvalSet{
		Name:    "test",
		Version: "1.0.0",
		Samples: []Sample{
			{
				ID: "sample-1",
				Task: agent.Task{
					ID:      "task-id",
					Context: map[string]any{"objective": "test goal 1"},
				},
			},
			{
				ID: "sample-2",
				Task: agent.Task{
					ID:      "task-id",
					Context: map[string]any{"objective": "test goal 2"},
				},
			},
		},
	}

	err := evalSet.Validate()
	assert.NoError(t, err)
}

func TestFilterByTags_NoTags(t *testing.T) {
	evalSet := &EvalSet{
		Name:    "test",
		Version: "1.0.0",
		Samples: []Sample{
			{
				ID:   "sample-1",
				Task: agent.Task{Context: map[string]any{"objective": "test goal"}},
				Tags: []string{"tag1", "tag2"},
			},
			{
				ID:   "sample-2",
				Task: agent.Task{Context: map[string]any{"objective": "test goal"}},
				Tags: []string{"tag3"},
			},
		},
	}

	// No tags specified should return all samples
	filtered := evalSet.FilterByTags(nil)
	assert.Len(t, filtered.Samples, 2)
	assert.Equal(t, "test", filtered.Name)
	assert.Equal(t, "1.0.0", filtered.Version)

	// Verify it's a copy, not the same instance
	assert.NotSame(t, evalSet, filtered)
}

func TestFilterByTags_SingleTag(t *testing.T) {
	evalSet := &EvalSet{
		Name:    "test",
		Version: "1.0.0",
		Samples: []Sample{
			{
				ID:   "sample-1",
				Task: agent.Task{Context: map[string]any{"objective": "test goal"}},
				Tags: []string{"injection", "basic"},
			},
			{
				ID:   "sample-2",
				Task: agent.Task{Context: map[string]any{"objective": "test goal"}},
				Tags: []string{"jailbreak", "basic"},
			},
			{
				ID:   "sample-3",
				Task: agent.Task{Context: map[string]any{"objective": "test goal"}},
				Tags: []string{"injection", "advanced"},
			},
		},
	}

	// Filter by "injection" tag
	filtered := evalSet.FilterByTags([]string{"injection"})
	assert.Len(t, filtered.Samples, 2)
	assert.Equal(t, "sample-1", filtered.Samples[0].ID)
	assert.Equal(t, "sample-3", filtered.Samples[1].ID)
}

func TestFilterByTags_MultipleTags(t *testing.T) {
	evalSet := &EvalSet{
		Name:    "test",
		Version: "1.0.0",
		Samples: []Sample{
			{
				ID:   "sample-1",
				Task: agent.Task{Context: map[string]any{"objective": "test goal"}},
				Tags: []string{"injection", "basic", "web"},
			},
			{
				ID:   "sample-2",
				Task: agent.Task{Context: map[string]any{"objective": "test goal"}},
				Tags: []string{"jailbreak", "basic"},
			},
			{
				ID:   "sample-3",
				Task: agent.Task{Context: map[string]any{"objective": "test goal"}},
				Tags: []string{"injection", "advanced", "web"},
			},
			{
				ID:   "sample-4",
				Task: agent.Task{Context: map[string]any{"objective": "test goal"}},
				Tags: []string{"injection", "web"},
			},
		},
	}

	// Filter by multiple tags - must have ALL tags
	filtered := evalSet.FilterByTags([]string{"injection", "web"})
	assert.Len(t, filtered.Samples, 3)
	assert.Equal(t, "sample-1", filtered.Samples[0].ID)
	assert.Equal(t, "sample-3", filtered.Samples[1].ID)
	assert.Equal(t, "sample-4", filtered.Samples[2].ID)

	// Filter by three tags
	filtered = evalSet.FilterByTags([]string{"injection", "basic", "web"})
	assert.Len(t, filtered.Samples, 1)
	assert.Equal(t, "sample-1", filtered.Samples[0].ID)
}

func TestFilterByTags_NoMatches(t *testing.T) {
	evalSet := &EvalSet{
		Name:    "test",
		Version: "1.0.0",
		Samples: []Sample{
			{
				ID:   "sample-1",
				Task: agent.Task{Context: map[string]any{"objective": "test goal"}},
				Tags: []string{"tag1", "tag2"},
			},
			{
				ID:   "sample-2",
				Task: agent.Task{Context: map[string]any{"objective": "test goal"}},
				Tags: []string{"tag3"},
			},
		},
	}

	// Filter by tag that doesn't exist
	filtered := evalSet.FilterByTags([]string{"nonexistent"})
	assert.Empty(t, filtered.Samples)
	assert.Equal(t, "test", filtered.Name)
	assert.Equal(t, "1.0.0", filtered.Version)
}

func TestFilterByTags_SamplesWithoutTags(t *testing.T) {
	evalSet := &EvalSet{
		Name:    "test",
		Version: "1.0.0",
		Samples: []Sample{
			{
				ID:   "sample-1",
				Task: agent.Task{Context: map[string]any{"objective": "test goal"}},
				Tags: []string{"tag1"},
			},
			{
				ID:   "sample-2",
				Task: agent.Task{Context: map[string]any{"objective": "test goal"}},
				// No tags
			},
		},
	}

	// Filter by tag - sample without tags should not match
	filtered := evalSet.FilterByTags([]string{"tag1"})
	assert.Len(t, filtered.Samples, 1)
	assert.Equal(t, "sample-1", filtered.Samples[0].ID)
}

func TestFilterByTags_PreservesMetadata(t *testing.T) {
	evalSet := &EvalSet{
		Name:    "test",
		Version: "1.0.0",
		Samples: []Sample{
			{
				ID:   "sample-1",
				Task: agent.Task{Context: map[string]any{"objective": "test goal"}},
				Tags: []string{"tag1"},
			},
		},
		Metadata: map[string]any{
			"author": "test-author",
			"date":   "2025-01-05",
		},
	}

	// Filter should preserve metadata
	filtered := evalSet.FilterByTags([]string{"tag1"})
	assert.Equal(t, evalSet.Metadata, filtered.Metadata)
}

func TestHasAllTags(t *testing.T) {
	tests := []struct {
		name         string
		sampleTags   []string
		requiredTags []string
		expected     bool
	}{
		{
			name:         "all tags present",
			sampleTags:   []string{"tag1", "tag2", "tag3"},
			requiredTags: []string{"tag1", "tag2"},
			expected:     true,
		},
		{
			name:         "missing one tag",
			sampleTags:   []string{"tag1", "tag3"},
			requiredTags: []string{"tag1", "tag2"},
			expected:     false,
		},
		{
			name:         "no required tags",
			sampleTags:   []string{"tag1", "tag2"},
			requiredTags: []string{},
			expected:     true,
		},
		{
			name:         "no sample tags",
			sampleTags:   []string{},
			requiredTags: []string{"tag1"},
			expected:     false,
		},
		{
			name:         "exact match",
			sampleTags:   []string{"tag1", "tag2"},
			requiredTags: []string{"tag1", "tag2"},
			expected:     true,
		},
		{
			name:         "order doesn't matter",
			sampleTags:   []string{"tag2", "tag1", "tag3"},
			requiredTags: []string{"tag3", "tag1"},
			expected:     true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := hasAllTags(tt.sampleTags, tt.requiredTags)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestLoadEvalSet_Integration(t *testing.T) {
	// Create a realistic eval set file
	tmpDir := t.TempDir()
	jsonPath := filepath.Join(tmpDir, "integration.json")

	jsonContent := `{
		"name": "prompt-injection-eval",
		"version": "2.1.0",
		"samples": [
			{
				"id": "pi-001",
				"task": {
					"id": "pi-001",
					"objective": "extract system prompt",
					"context": {
						"target": "http://example.com/chat",
						"timeout": 300
					}
				},
				"expected_output": {
					"success": true,
					"extracted_prompt": "You are a helpful assistant..."
				},
				"expected_tools": [
					{
						"name": "http-client",
						"arguments": {
							"method": "POST",
							"url": "http://example.com/chat"
						},
						"required": true
					}
				],
				"tags": ["injection", "basic", "web"],
				"metadata": {
					"difficulty": "easy",
					"author": "security-team"
				}
			}
		],
		"metadata": {
			"author": "gibson-team",
			"created": "2025-01-05",
			"description": "Prompt injection evaluation suite"
		}
	}`

	err := os.WriteFile(jsonPath, []byte(jsonContent), 0644)
	require.NoError(t, err)

	// Load and validate
	evalSet, err := LoadEvalSet(jsonPath)
	require.NoError(t, err)
	assert.Equal(t, "prompt-injection-eval", evalSet.Name)
	assert.Equal(t, "2.1.0", evalSet.Version)
	assert.Len(t, evalSet.Samples, 1)

	sample := evalSet.Samples[0]
	assert.Equal(t, "pi-001", sample.ID)
	// Goal field removed - using context instead
	assert.Contains(t, sample.Tags, "injection")
	assert.Contains(t, sample.Tags, "basic")
	assert.Contains(t, sample.Tags, "web")

	// Test filtering
	filtered := evalSet.FilterByTags([]string{"injection", "web"})
	assert.Len(t, filtered.Samples, 1)

	filtered = evalSet.FilterByTags([]string{"advanced"})
	assert.Empty(t, filtered.Samples)
}
