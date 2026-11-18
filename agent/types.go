// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Zero Root AI

package agent

import (
	"github.com/a2aproject/a2a-go/v2/a2a"
	jobpb "github.com/zeroroot-ai/sdk/api/gen/gibson/job/v1"
	"github.com/zeroroot-ai/sdk/openresponses"
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

	// PinnedSkills lists the Agent Skills that the mission pins for this task,
	// as canonical ids of the form skill:<name>. They come from the Gibson run
	// envelope of the A2A message.
	PinnedSkills []string

	// ContextID is the A2A context id of the task. The SDK copies it to the
	// A2A Task that it returns.
	ContextID string

	// Message is the A2A message that started this run. It holds every part
	// of the message, for example a file part. Goal holds the text parts.
	Message *a2a.Message

	// Run names the run that the daemon started this task for.
	Run RunContext

	// SkillItems holds the loaded pinned skills as Open Responses items, a
	// function_call and a function_call_output for each skill. The SDK fills
	// it before Execute runs. An agent puts these items at the start of its
	// conversation, before its first model call.
	SkillItems []openresponses.Item
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
	// Status is the A2A Task state of the run. It is one of the a2a.TaskState
	// constants, for example a2a.TaskStateCompleted, a2a.TaskStateFailed,
	// a2a.TaskStateCanceled or a2a.TaskStateInputRequired. An empty status
	// means completed when Error and ErrorInfo are nil, and failed otherwise.
	Status a2a.TaskState

	// Output contains the task result data. A string becomes a text part, a
	// []byte becomes a raw part, and any other value becomes a data part.
	// The SDK puts it in an A2A artifact named "output". For the states
	// input-required and auth-required it becomes the status message instead.
	Output any

	// Artifacts holds A2A artifacts that the agent builds itself. The SDK
	// returns them unchanged, before the artifact of Output.
	Artifacts []*a2a.Artifact

	// History holds messages that the agent adds to the history of the A2A
	// Task. The SDK puts the message that started the run first.
	History []*a2a.Message

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
