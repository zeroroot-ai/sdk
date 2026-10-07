// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Zero Root AI

package agent

import (
	jobpb "github.com/zeroroot-ai/sdk/api/gen/gibson/job/v1"
)

// Task represents a unit of work assigned to an agent.
// It contains all information needed for the agent to execute the task.
type Task struct {
	// ID is a unique identifier for this task.
	ID string

	// Goal is the primary objective for this task.
	// This is a convenience field that is also stored in Context["goal"].
	Goal string

	// Context provides additional information needed to complete the task.
	// This can include target details, previous findings, or mission context.
	Context map[string]any

	// Constraints defines limits and rules for task execution.
	Constraints TaskConstraints

	// Metadata stores additional task-specific information.
	// This can include priority, timeout, dependencies, etc.
	Metadata map[string]any

	// Job names the typed resources this task needs and how it is judged.
	// It is set when the task opens or continues a job on a bank of
	// always-on agents. Context stays free-form; anything the daemon must
	// enforce (repositories, credential names, World inputs, acceptance)
	// lives here. Nil for a task that needs none of them.
	Job *jobpb.JobSpec
}

// TaskConstraints defines operational limits for task execution.
// These constraints ensure the agent operates within acceptable boundaries.
type TaskConstraints struct {
	// MaxTurns limits the number of LLM interaction turns allowed.
	// Zero value means no limit.
	MaxTurns int

	// MaxTokens limits the total number of tokens that can be consumed.
	// This includes both input and output tokens across all LLM calls.
	// Zero value means no limit.
	MaxTokens int

	// AllowedTools lists the tools the agent is permitted to use.
	// If empty, all available tools are allowed.
	AllowedTools []string

	// BlockedTools lists the tools the agent must not use.
	// This takes precedence over AllowedTools.
	BlockedTools []string
}

// Result represents the outcome of task execution.
// It contains the agent's output, findings, and execution status.
type Result struct {
	// Status indicates whether the task completed successfully.
	Status ResultStatus

	// Output contains the task result data.
	// The structure depends on the specific task and agent implementation.
	Output any

	// Findings contains IDs of security findings discovered during task execution.
	// These IDs reference findings submitted to the harness.
	Findings []string

	// Metadata stores additional result information.
	// This can include execution time, resource usage, intermediate results, etc.
	Metadata map[string]any

	// Error contains error information if the task failed.
	// This should be nil for successful tasks.
	// This field is not serialized to JSON - use ErrorInfo instead for serialization.
	Error error `json:"-"`

	// ErrorInfo contains structured, JSON-serializable error information.
	// This field is populated automatically when Fail() is called.
	// It preserves error details across process boundaries and serialization.
	ErrorInfo *ResultError `json:"error,omitempty"`
}

// ResultStatus indicates the outcome of task execution.
type ResultStatus string

const (
	// StatusSuccess indicates the task completed successfully.
	StatusSuccess ResultStatus = "success"

	// StatusFailed indicates the task failed to complete.
	StatusFailed ResultStatus = "failed"

	// StatusPartial indicates the task completed with partial results.
	// Some objectives were achieved but not all.
	StatusPartial ResultStatus = "partial"

	// StatusCancelled indicates the task was cancelled before completion.
	StatusCancelled ResultStatus = "cancelled"

	// StatusTimeout indicates the task exceeded time or resource limits.
	StatusTimeout ResultStatus = "timeout"
)

// String returns the string representation of the result status.
func (s ResultStatus) String() string {
	return string(s)
}
