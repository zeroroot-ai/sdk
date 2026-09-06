// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Zero Root AI

package eval

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/zeroroot-ai/sdk/agent"
	"github.com/zeroroot-ai/sdk/finding"
)

// TestIntegration_FullEvaluationMission tests the complete evaluation flow:
// 1. Load eval set from JSON
// 2. Filter samples by tags
// 3. Create mock trajectory data
// 4. Run multiple scorers
// 5. Log results to JSONL
// 6. Verify output format
func TestIntegration_FullEvaluationMission(t *testing.T) {
	if os.Getenv("GOEVALS") != "1" {
		t.Skip("GOEVALS=1 not set, skipping integration test")
	}

	// Step 1: Load the sample eval set
	t.Log("Loading sample eval set...")
	evalSetPath := filepath.Join("testdata", "sample_evalset.json")
	evalSet, err := LoadEvalSet(evalSetPath)
	require.NoError(t, err, "Failed to load eval set")
	require.NotNil(t, evalSet, "Eval set should not be nil")

	assert.Equal(t, "Sample Evaluation Set", evalSet.Name)
	assert.Equal(t, "1.0.0", evalSet.Version)
	assert.Len(t, evalSet.Samples, 5, "Should have 5 samples")

	t.Logf("Loaded eval set: %s v%s with %d samples", evalSet.Name, evalSet.Version, len(evalSet.Samples))

	// Step 2: Filter by tags
	t.Log("Filtering samples by tags...")

	// Test filtering by "web" tag
	webSamples := evalSet.FilterByTags([]string{"web"})
	assert.Len(t, webSamples.Samples, 2, "Should have 2 web samples")
	t.Logf("Filtered to %d samples with 'web' tag", len(webSamples.Samples))

	// Test filtering by "critical" tag
	criticalSamples := evalSet.FilterByTags([]string{"critical"})
	assert.Len(t, criticalSamples.Samples, 2, "Should have 2 critical samples")
	t.Logf("Filtered to %d samples with 'critical' tag", len(criticalSamples.Samples))

	// Test filtering by multiple tags (AND logic)
	webCriticalSamples := evalSet.FilterByTags([]string{"web", "critical"})
	assert.Len(t, webCriticalSamples.Samples, 1, "Should have 1 sample with both 'web' and 'critical' tags")

	// Step 3: Create mock trajectory data for each sample
	t.Log("Creating mock trajectory data...")
	samplesWithTrajectories := createMockTrajectories(evalSet.Samples)
	assert.Len(t, samplesWithTrajectories, 5, "Should have trajectories for all samples")

	// Step 4: Run multiple scorers on samples
	t.Log("Running scorers on samples...")

	// Create a temporary log file
	tmpDir := t.TempDir()
	logPath := filepath.Join(tmpDir, "eval_results.jsonl")
	logger, err := NewJSONLLogger(logPath)
	require.NoError(t, err, "Failed to create logger")
	defer logger.Close()

	// Create eval runner with logger
	e := &E{
		T:      t,
		logger: logger,
	}

	// Score each sample with multiple scorers
	var results []Result
	for _, sample := range samplesWithTrajectories {
		t.Logf("Scoring sample: %s", sample.ID)

		// Create scorers based on what the sample expects
		var scorers []Scorer

		// Add tool scorer if sample has expected tools
		if len(sample.ExpectedTools) > 0 {
			toolScorer := NewToolCorrectnessScorer(ToolCorrectnessOptions{
				OrderMatters:     false,
				NumericTolerance: 0.01,
			})
			scorers = append(scorers, toolScorer)
		}

		// Add finding scorer if sample has expected findings
		if len(sample.ExpectedFindings) > 0 {
			findingScorer := NewFindingAccuracyScorer(FindingAccuracyOptions{
				MatchBySeverity:     true,
				MatchByCategory:     true,
				FuzzyTitleThreshold: 0.8,
			})
			scorers = append(scorers, findingScorer)
		}

		// Add trajectory scorer for all samples
		trajectoryScorer := NewTrajectoryScorer(TrajectoryOptions{
			Mode:          TrajectorySubsetMatch,
			PenalizeExtra: 0.1,
			ExpectedSteps: extractExpectedSteps(sample),
		})
		scorers = append(scorers, trajectoryScorer)

		// Score the sample
		result := e.Score(sample, scorers...)
		results = append(results, result)

		// Log individual scores
		t.Logf("  Overall score: %.3f", result.OverallScore)
		for name, scoreResult := range result.Scores {
			t.Logf("    %s: %.3f", name, scoreResult.Score)
		}
	}

	// Verify we got results for all samples
	assert.Len(t, results, 5, "Should have results for all 5 samples")

	// Step 5: Verify JSONL output format
	t.Log("Verifying JSONL output format...")

	// Close logger to flush
	err = logger.Close()
	require.NoError(t, err, "Failed to close logger")

	// Read and verify JSONL file
	logData, err := os.ReadFile(logPath)
	require.NoError(t, err, "Failed to read log file")

	lines := strings.Split(strings.TrimSpace(string(logData)), "\n")
	assert.Len(t, lines, 5, "Should have 5 log entries (one per sample)")

	// Verify each line is valid JSON
	for i, line := range lines {
		t.Logf("Verifying log entry %d...", i+1)

		var entry LogEntry
		err := json.Unmarshal([]byte(line), &entry)
		require.NoError(t, err, "Log entry %d should be valid JSON", i+1)

		// Verify required fields
		assert.NotEmpty(t, entry.SampleID, "Entry %d should have sample_id", i+1)
		assert.NotZero(t, entry.Timestamp, "Entry %d should have timestamp", i+1)
		assert.NotNil(t, entry.Scores, "Entry %d should have scores", i+1)
		assert.GreaterOrEqual(t, entry.OverallScore, 0.0, "Entry %d overall_score should be >= 0", i+1)
		assert.LessOrEqual(t, entry.OverallScore, 1.0, "Entry %d overall_score should be <= 1", i+1)

		t.Logf("  Entry %d: sample=%s, overall_score=%.3f, duration=%dms",
			i+1, entry.SampleID, entry.OverallScore, entry.Duration)
	}

	// Step 6: Calculate and report summary statistics
	t.Log("Calculating summary statistics...")

	var totalScore float64
	var minScore, maxScore = 1.0, 0.0

	for _, result := range results {
		totalScore += result.OverallScore
		if result.OverallScore < minScore {
			minScore = result.OverallScore
		}
		if result.OverallScore > maxScore {
			maxScore = result.OverallScore
		}
	}

	avgScore := totalScore / float64(len(results))

	t.Logf("Summary Statistics:")
	t.Logf("  Total samples: %d", len(results))
	t.Logf("  Average score: %.3f", avgScore)
	t.Logf("  Min score: %.3f", minScore)
	t.Logf("  Max score: %.3f", maxScore)
	t.Logf("  Log file: %s", logPath)

	// Success!
	t.Log("Integration test completed successfully!")
}

// createMockTrajectories creates mock trajectory and result data for evaluation samples.
// This simulates what would happen during actual agent execution.
func createMockTrajectories(samples []Sample) []Sample {
	mockSamples := make([]Sample, len(samples))

	for i, sample := range samples {
		// Copy the sample
		mockSample := sample

		// Create mock trajectory steps based on expected tools
		var steps []TrajectoryStep
		startTime := time.Now()

		for j, expectedTool := range sample.ExpectedTools {
			step := TrajectoryStep{
				Type:      "tool",
				Name:      expectedTool.Name,
				Input:     expectedTool.Arguments,
				Output:    map[string]any{"status": "success", "result": "mock output"},
				StartTime: startTime.Add(time.Duration(j) * time.Second),
				Duration:  time.Millisecond * 500,
			}
			steps = append(steps, step)
		}

		// Add LLM step
		steps = append(steps, TrajectoryStep{
			Type:      "llm",
			Name:      "primary",
			Input:     map[string]any{"prompt": "Analyze results"},
			Output:    map[string]any{"content": "Analysis complete"},
			StartTime: startTime.Add(time.Duration(len(steps)) * time.Second),
			Duration:  time.Millisecond * 300,
		})

		// Add finding submission steps based on expected findings
		for j, expectedFinding := range sample.ExpectedFindings {
			step := TrajectoryStep{
				Type:      "finding",
				Name:      "submit",
				Input:     expectedFinding,
				Output:    map[string]any{"finding_id": expectedFinding.ID},
				StartTime: startTime.Add(time.Duration(len(steps)+j) * time.Second),
				Duration:  time.Millisecond * 100,
			}
			steps = append(steps, step)
		}

		mockSample.Trajectory = Trajectory{
			Steps:     steps,
			StartTime: startTime,
			EndTime:   startTime.Add(time.Duration(len(steps)) * time.Second),
		}

		// Create mock result
		mockSample.Result = agent.Result{
			Status:   agent.StatusSuccess,
			Output:   sample.ExpectedOutput,
			Findings: extractFindingIDs(sample.ExpectedFindings),
		}

		mockSamples[i] = mockSample
	}

	return mockSamples
}

// extractExpectedSteps creates expected trajectory steps from a sample's expected tools and findings.
func extractExpectedSteps(sample Sample) []ExpectedStep {
	var steps []ExpectedStep

	// Add expected tool calls as steps
	for _, tool := range sample.ExpectedTools {
		steps = append(steps, ExpectedStep{
			Type:     "tool",
			Name:     tool.Name,
			Required: tool.Required,
		})
	}

	// Add expected findings as steps
	for range sample.ExpectedFindings {
		steps = append(steps, ExpectedStep{
			Type:     "finding",
			Name:     "submit",
			Required: true,
		})
	}

	return steps
}

// extractFindingIDs extracts finding IDs from ground truth findings.
func extractFindingIDs(findings []GroundTruthFinding) []string {
	ids := make([]string, len(findings))
	for i, f := range findings {
		ids[i] = f.ID
	}
	return ids
}

// TestIntegration_LoadAndValidate tests loading and validation of the eval set.
func TestIntegration_LoadAndValidate(t *testing.T) {
	if os.Getenv("GOEVALS") != "1" {
		t.Skip("GOEVALS=1 not set, skipping integration test")
	}

	evalSetPath := filepath.Join("testdata", "sample_evalset.json")

	// Test successful load
	evalSet, err := LoadEvalSet(evalSetPath)
	require.NoError(t, err)
	require.NotNil(t, evalSet)

	// Verify structure
	assert.Equal(t, "Sample Evaluation Set", evalSet.Name)
	assert.Len(t, evalSet.Samples, 5)

	// Verify each sample has required fields
	for i, sample := range evalSet.Samples {
		assert.NotEmpty(t, sample.ID, "Sample %d should have ID", i)
		// Goal field removed - samples use task.id now
		assert.NotEmpty(t, sample.Tags, "Sample %d should have tags", i)

		t.Logf("Sample %d: id=%s, tags=%v, expected_tools=%d, expected_findings=%d",
			i+1, sample.ID, sample.Tags, len(sample.ExpectedTools), len(sample.ExpectedFindings))
	}
}

// TestIntegration_FilterByTags tests tag filtering functionality.
func TestIntegration_FilterByTags(t *testing.T) {
	if os.Getenv("GOEVALS") != "1" {
		t.Skip("GOEVALS=1 not set, skipping integration test")
	}

	evalSetPath := filepath.Join("testdata", "sample_evalset.json")
	evalSet, err := LoadEvalSet(evalSetPath)
	require.NoError(t, err)

	tests := []struct {
		name          string
		tags          []string
		expectedCount int
		description   string
	}{
		{
			name:          "no_filter",
			tags:          nil,
			expectedCount: 5,
			description:   "No filter should return all samples",
		},
		{
			name:          "web_tag",
			tags:          []string{"web"},
			expectedCount: 2,
			description:   "Should find SQL injection and XSS samples",
		},
		{
			name:          "critical_tag",
			tags:          []string{"critical"},
			expectedCount: 2,
			description:   "Should find SQL injection and multi-step samples",
		},
		{
			name:          "llm_tag",
			tags:          []string{"llm"},
			expectedCount: 1,
			description:   "Should find prompt injection sample",
		},
		{
			name:          "multiple_tags",
			tags:          []string{"web", "critical"},
			expectedCount: 1,
			description:   "Should find only SQL injection sample (AND logic)",
		},
		{
			name:          "nonexistent_tag",
			tags:          []string{"nonexistent"},
			expectedCount: 0,
			description:   "Should find no samples",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			filtered := evalSet.FilterByTags(tt.tags)
			assert.Len(t, filtered.Samples, tt.expectedCount, tt.description)

			// Log filtered sample IDs
			if len(filtered.Samples) > 0 {
				ids := make([]string, len(filtered.Samples))
				for i, s := range filtered.Samples {
					ids[i] = s.ID
				}
				t.Logf("Filtered samples: %v", ids)
			}
		})
	}
}

// TestIntegration_ScorerAccuracy tests that scorers produce reasonable results.
func TestIntegration_ScorerAccuracy(t *testing.T) {
	if os.Getenv("GOEVALS") != "1" {
		t.Skip("GOEVALS=1 not set, skipping integration test")
	}

	// Create a sample with perfect execution (all tools called, all findings found)
	perfectSample := Sample{
		ID: "perfect-execution",
		Task: agent.Task{
			ID:      "test-001",
			Context: map[string]any{"objective": "Test perfect execution"},
		},
		ExpectedTools: []ExpectedToolCall{
			{
				Name: "mytool",
				Arguments: map[string]any{
					"target": "example.com",
				},
				Required: true,
			},
		},
		ExpectedFindings: []GroundTruthFinding{
			{
				ID:       "finding-001",
				Severity: "high",
				Category: "information_disclosure",
				Title:    "Test Finding",
			},
		},
		Trajectory: Trajectory{
			Steps: []TrajectoryStep{
				{
					Type: "tool",
					Name: "mytool",
					Input: map[string]any{
						"target": "example.com",
					},
					Output: map[string]any{
						"result": "success",
					},
					StartTime: time.Now(),
					Duration:  time.Millisecond * 500,
				},
				{
					Type: "finding",
					Name: "submit",
					Output: finding.NewFindingWithID(
						"finding-001",
						"mission-001",
						"test-agent",
						"Test Finding",
						"Description",
						finding.CategoryInformationDisclosure,
						finding.SeverityHigh,
					),
					StartTime: time.Now(),
					Duration:  time.Millisecond * 100,
				},
			},
			StartTime: time.Now(),
			EndTime:   time.Now().Add(time.Second),
		},
		Result: agent.Result{
			Status:   agent.StatusSuccess,
			Findings: []string{"finding-001"},
		},
	}

	ctx := context.Background()

	// Test tool scorer
	t.Run("tool_scorer", func(t *testing.T) {
		scorer := NewToolCorrectnessScorer(ToolCorrectnessOptions{
			OrderMatters: false,
		})

		result, err := scorer.Score(ctx, perfectSample)
		require.NoError(t, err)

		// Should get perfect score for perfect execution
		assert.Equal(t, 1.0, result.Score, "Perfect tool execution should score 1.0")
		t.Logf("Tool scorer: %.3f (details: %+v)", result.Score, result.Details)
	})

	// Test finding scorer
	t.Run("finding_scorer", func(t *testing.T) {
		scorer := NewFindingAccuracyScorer(FindingAccuracyOptions{
			MatchBySeverity: true,
			MatchByCategory: true,
		})

		result, err := scorer.Score(ctx, perfectSample)
		require.NoError(t, err)

		// Should get high score for finding the expected finding
		assert.Greater(t, result.Score, 0.5, "Finding correct finding should score > 0.5")
		t.Logf("Finding scorer: %.3f (details: %+v)", result.Score, result.Details)
	})

	// Test trajectory scorer
	t.Run("trajectory_scorer", func(t *testing.T) {
		scorer := NewTrajectoryScorer(TrajectoryOptions{
			Mode: TrajectorySubsetMatch,
			ExpectedSteps: []ExpectedStep{
				{Type: "tool", Name: "mytool", Required: true},
				{Type: "finding", Name: "submit", Required: true},
			},
		})

		result, err := scorer.Score(ctx, perfectSample)
		require.NoError(t, err)

		// Should get perfect score for matching all expected steps
		assert.Equal(t, 1.0, result.Score, "Matching all expected steps should score 1.0")
		t.Logf("Trajectory scorer: %.3f (details: %+v)", result.Score, result.Details)
	})
}

// TestIntegration_JSONLFormat verifies the JSONL output format in detail.
func TestIntegration_JSONLFormat(t *testing.T) {
	if os.Getenv("GOEVALS") != "1" {
		t.Skip("GOEVALS=1 not set, skipping integration test")
	}

	// Create a simple sample
	sample := Sample{
		ID: "jsonl-test",
		Task: agent.Task{
			ID:      "test-001",
			Context: map[string]any{"objective": "Test JSONL format"},
		},
		Tags: []string{"test"},
		Metadata: map[string]any{
			"test_key": "test_value",
		},
	}

	// Create temporary log file
	tmpDir := t.TempDir()
	logPath := filepath.Join(tmpDir, "test.jsonl")
	logger, err := NewJSONLLogger(logPath)
	require.NoError(t, err)
	defer logger.Close()

	// Create a result
	result := Result{
		SampleID: sample.ID,
		Scores: map[string]ScoreResult{
			"test_scorer": {
				Score: 0.75,
				Details: map[string]any{
					"detail_key": "detail_value",
				},
			},
		},
		OverallScore: 0.75,
		Duration:     time.Millisecond * 100,
		Timestamp:    time.Now(),
	}

	// Log the result
	err = logger.Log(sample, result)
	require.NoError(t, err)

	// Close logger to flush
	err = logger.Close()
	require.NoError(t, err)

	// Read and verify
	data, err := os.ReadFile(logPath)
	require.NoError(t, err)

	var entry LogEntry
	err = json.Unmarshal(data, &entry)
	require.NoError(t, err)

	assert.Equal(t, "jsonl-test", entry.SampleID)
	assert.Equal(t, "test-001", entry.TaskID)
	assert.Equal(t, 0.75, entry.OverallScore)
	assert.NotEmpty(t, entry.Timestamp)
	assert.Contains(t, entry.Scores, "test_scorer")
	assert.Equal(t, 0.75, entry.Scores["test_scorer"])

	t.Logf("JSONL entry: %+v", entry)
}
