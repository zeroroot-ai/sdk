// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Zero Root AI

package eval

import (
	"time"

	"github.com/zeroroot-ai/sdk/agent"
)

// Sample represents a single evaluation case for testing agent performance.
// It contains the task, expected results, and metadata for evaluation.
type Sample struct {
	// ID is a unique identifier for this sample.
	ID string `json:"id" yaml:"id"`

	// Task is the agent task to execute for this evaluation.
	Task agent.Task `json:"task" yaml:"task"`

	// Result is the actual result from executing the task.
	// This is populated after execution.
	Result agent.Result `json:"result,omitempty" yaml:"result,omitempty"`

	// Trajectory is the recorded execution path taken by the agent.
	// This is populated by RecordingHarness during execution.
	Trajectory Trajectory `json:"trajectory,omitempty" yaml:"trajectory,omitempty"`

	// ExpectedOutput is the expected task output for comparison.
	// The structure depends on the task type.
	ExpectedOutput any `json:"expected_output,omitempty" yaml:"expected_output,omitempty"`

	// ExpectedTools lists the tools the agent should call during execution.
	ExpectedTools []ExpectedToolCall `json:"expected_tools,omitempty" yaml:"expected_tools,omitempty"`

	// ExpectedFindings lists the security findings the agent should discover.
	ExpectedFindings []GroundTruthFinding `json:"expected_findings,omitempty" yaml:"expected_findings,omitempty"`

	// Metadata stores additional sample-specific information.
	// This can include difficulty level, author, creation date, etc.
	Metadata map[string]any `json:"metadata,omitempty" yaml:"metadata,omitempty"`

	// Tags are labels for categorization and filtering.
	Tags []string `json:"tags,omitempty" yaml:"tags,omitempty"`
}

// Result contains aggregated evaluation results for a sample.
// It includes individual scorer results and an overall score.
type Result struct {
	// SampleID identifies the evaluated sample.
	SampleID string `json:"sample_id" yaml:"sample_id"`

	// Scores contains individual results from each scorer, keyed by scorer name.
	Scores map[string]ScoreResult `json:"scores" yaml:"scores"`

	// OverallScore is the aggregated score across all scorers (0.0 to 1.0).
	OverallScore float64 `json:"overall_score" yaml:"overall_score"`

	// Duration is the total time taken for evaluation.
	Duration time.Duration `json:"duration" yaml:"duration"`

	// Timestamp is when the evaluation was performed.
	Timestamp time.Time `json:"timestamp" yaml:"timestamp"`

	// Error contains error information if evaluation failed.
	// This is serialized as a string since error type isn't JSON-serializable.
	Error string `json:"error,omitempty" yaml:"error,omitempty"`
}

// Trajectory represents the recorded execution path of an agent.
// It captures all operations performed during task execution.
type Trajectory struct {
	// Steps contains the sequence of operations performed.
	Steps []TrajectoryStep `json:"steps" yaml:"steps"`

	// StartTime is when the trajectory recording began.
	StartTime time.Time `json:"start_time" yaml:"start_time"`

	// EndTime is when the trajectory recording ended.
	EndTime time.Time `json:"end_time" yaml:"end_time"`
}

// TrajectoryStep represents a single operation in the agent's execution path.
// This could be a tool call, LLM completion, finding submission, etc.
type TrajectoryStep struct {
	// Type identifies the kind of operation.
	// Common values: "tool", "llm", "delegate", "finding", "memory"
	Type string `json:"type" yaml:"type"`

	// Name is the specific name of the operation.
	// For tools: tool name, for LLM: slot name, for delegate: agent name
	Name string `json:"name" yaml:"name"`

	// Input contains the input data for this operation.
	// The structure depends on the operation type.
	Input any `json:"input,omitempty" yaml:"input,omitempty"`

	// Output contains the output data from this operation.
	// The structure depends on the operation type.
	Output any `json:"output,omitempty" yaml:"output,omitempty"`

	// Error contains error information if the operation failed.
	// This is serialized as a string since error type isn't JSON-serializable.
	Error string `json:"error,omitempty" yaml:"error,omitempty"`

	// StartTime is when this operation began.
	StartTime time.Time `json:"start_time" yaml:"start_time"`

	// Duration is how long this operation took to complete.
	Duration time.Duration `json:"duration" yaml:"duration"`
}

// EvalSet is a collection of evaluation samples with metadata.
// It represents a test suite that can be loaded from a file.
type EvalSet struct {
	// Name identifies this evaluation set.
	Name string `json:"name" yaml:"name"`

	// Version tracks the evaluation set version for reproducibility.
	Version string `json:"version" yaml:"version"`

	// Samples contains the individual evaluation cases.
	Samples []Sample `json:"samples" yaml:"samples"`

	// Metadata stores additional evaluation set information.
	// This can include author, creation date, purpose, etc.
	Metadata map[string]any `json:"metadata,omitempty" yaml:"metadata,omitempty"`
}

// ExpectedToolCall represents an expected tool invocation during task execution.
// It specifies the tool name, arguments, and whether it's required.
type ExpectedToolCall struct {
	// Name is the tool name that should be called.
	Name string `json:"name" yaml:"name"`

	// Arguments are the expected tool arguments.
	// Use map[string]any for flexible argument matching.
	Arguments map[string]any `json:"arguments,omitempty" yaml:"arguments,omitempty"`

	// Required indicates whether this tool call is mandatory.
	// If false, the tool is optional and won't be penalized if not called.
	Required bool `json:"required" yaml:"required"`
}

// GroundTruthFinding represents an expected security finding.
// It defines what the agent should discover for finding accuracy evaluation.
type GroundTruthFinding struct {
	// ID is a unique identifier for this ground truth finding.
	ID string `json:"id" yaml:"id"`

	// Severity is the expected finding severity level.
	Severity string `json:"severity" yaml:"severity"`

	// Category is the expected finding category.
	Category string `json:"category" yaml:"category"`

	// Title is the expected finding title.
	// This can be used for fuzzy matching when ID matching fails.
	Title string `json:"title" yaml:"title"`
}
