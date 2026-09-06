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

// IsValid checks if the result status is a recognized value.
func (s ResultStatus) IsValid() bool {
	switch s {
	case StatusSuccess, StatusFailed, StatusPartial, StatusCancelled, StatusTimeout:
		return true
	default:
		return false
	}
}

// IsTerminal returns true if the status represents a final state.
// All statuses are terminal except for in-progress states (which don't exist in this enum).
func (s ResultStatus) IsTerminal() bool {
	return s.IsValid()
}

// IsSuccessful returns true if the status indicates successful completion.
// Both complete success and partial success are considered successful.
func (s ResultStatus) IsSuccessful() bool {
	return s == StatusSuccess || s == StatusPartial
}

// NewTask creates a new task with the given ID.
func NewTask(id string) *Task {
	return &Task{
		ID:          id,
		Context:     make(map[string]any),
		Constraints: TaskConstraints{},
		Metadata:    make(map[string]any),
	}
}

// GetContext retrieves a context value by key.
func (t *Task) GetContext(key string) (any, bool) {
	if t.Context == nil {
		return nil, false
	}
	val, ok := t.Context[key]
	return val, ok
}

// SetContext sets a context value.
func (t *Task) SetContext(key string, value any) {
	if t.Context == nil {
		t.Context = make(map[string]any)
	}
	t.Context[key] = value
}

// GetMetadata retrieves a metadata value by key.
func (t *Task) GetMetadata(key string) (any, bool) {
	if t.Metadata == nil {
		return nil, false
	}
	val, ok := t.Metadata[key]
	return val, ok
}

// SetMetadata sets a metadata value.
func (t *Task) SetMetadata(key string, value any) {
	if t.Metadata == nil {
		t.Metadata = make(map[string]any)
	}
	t.Metadata[key] = value
}

// IsToolAllowed checks if a tool is allowed by the constraints.
func (c *TaskConstraints) IsToolAllowed(toolName string) bool {
	// Check blocked tools first
	for _, blocked := range c.BlockedTools {
		if blocked == toolName {
			return false
		}
	}

	// If no allowed tools specified, all tools are allowed (except blocked)
	if len(c.AllowedTools) == 0 {
		return true
	}

	// Check if tool is in allowed list
	for _, allowed := range c.AllowedTools {
		if allowed == toolName {
			return true
		}
	}

	return false
}

// HasTurnLimit returns true if a maximum turn limit is set.
func (c *TaskConstraints) HasTurnLimit() bool {
	return c.MaxTurns > 0
}

// HasTokenLimit returns true if a maximum token limit is set.
func (c *TaskConstraints) HasTokenLimit() bool {
	return c.MaxTokens > 0
}

// NewSuccessResult creates a successful result with the given output.
func NewSuccessResult(output any) Result {
	return Result{
		Status:   StatusSuccess,
		Output:   output,
		Findings: []string{},
		Metadata: make(map[string]any),
	}
}

// NewFailedResult creates a failed result with the given error.
func NewFailedResult(err error) Result {
	return Result{
		Status:    StatusFailed,
		Error:     err,
		ErrorInfo: FromError(err),
		Findings:  []string{},
		Metadata:  make(map[string]any),
	}
}

// NewPartialResult creates a partial result with the given output and error.
func NewPartialResult(output any, err error) Result {
	return Result{
		Status:    StatusPartial,
		Output:    output,
		Error:     err,
		ErrorInfo: FromError(err),
		Findings:  []string{},
		Metadata:  make(map[string]any),
	}
}

// NewCancelledResult creates a cancelled result.
func NewCancelledResult() Result {
	return Result{
		Status:   StatusCancelled,
		Findings: []string{},
		Metadata: make(map[string]any),
	}
}

// NewTimeoutResult creates a timeout result.
func NewTimeoutResult() Result {
	return Result{
		Status:   StatusTimeout,
		Findings: []string{},
		Metadata: make(map[string]any),
	}
}

// AddFinding adds a finding ID to the result.
func (r *Result) AddFinding(findingID string) {
	if r.Findings == nil {
		r.Findings = []string{}
	}
	r.Findings = append(r.Findings, findingID)
}

// GetMetadata retrieves a metadata value by key.
func (r *Result) GetMetadata(key string) (any, bool) {
	if r.Metadata == nil {
		return nil, false
	}
	val, ok := r.Metadata[key]
	return val, ok
}

// SetMetadata sets a metadata value.
func (r *Result) SetMetadata(key string, value any) {
	if r.Metadata == nil {
		r.Metadata = make(map[string]any)
	}
	r.Metadata[key] = value
}

// Fail marks the result as failed and sets both the Error and ErrorInfo fields.
// The Error field is set to the provided error for backwards compatibility.
// The ErrorInfo field is set to a serializable ResultError for transmission.
//
// Example:
//
//	result := agent.Result{}
//	if err := performTask(); err != nil {
//	    result.Fail(err)
//	    return result
//	}
func (r *Result) Fail(err error) {
	r.Status = StatusFailed
	r.Error = err
	r.ErrorInfo = FromError(err)
}
