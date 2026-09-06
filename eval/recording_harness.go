// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Zero Root AI

// Package eval provides evaluation capabilities for the Gibson SDK.
// This file implements RecordingHarness, a transparent wrapper around agent.Harness
// that records all operations for trajectory-based evaluation.
package eval

import (
	"context"
	"log/slog"
	"sync"
	"time"

	"github.com/zeroroot-ai/sdk/agent"
	"github.com/zeroroot-ai/sdk/finding"
	"github.com/zeroroot-ai/sdk/llm"
	"github.com/zeroroot-ai/sdk/planning"
	"github.com/zeroroot-ai/sdk/plugin"
	"github.com/zeroroot-ai/sdk/tool"
	"github.com/zeroroot-ai/sdk/types"
	"go.opentelemetry.io/otel/trace"
	"google.golang.org/protobuf/proto"
)

// RecordingHarness wraps an agent.Harness and records all operations as trajectory steps.
// It implements the full agent.Harness interface by delegating to an inner harness
// while capturing inputs, outputs, timing, and errors for evaluation.
type RecordingHarness struct {
	inner      agent.Harness
	trajectory Trajectory
	mu         sync.Mutex
}

// NewRecordingHarness creates a new recording harness that wraps the given inner harness.
// All method calls will be delegated to the inner harness while recording trajectory steps.
func NewRecordingHarness(inner agent.Harness) *RecordingHarness {
	return &RecordingHarness{
		inner: inner,
		trajectory: Trajectory{
			Steps:     make([]TrajectoryStep, 0),
			StartTime: time.Now(),
		},
	}
}

// recordStep adds a trajectory step to the recording in a thread-safe manner.
func (r *RecordingHarness) recordStep(step TrajectoryStep) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.trajectory.Steps = append(r.trajectory.Steps, step)
}

// Trajectory returns the recorded trajectory of operations.
// This returns a copy to prevent external modification.
func (r *RecordingHarness) Trajectory() Trajectory {
	r.mu.Lock()
	defer r.mu.Unlock()

	// Update end time
	r.trajectory.EndTime = time.Now()

	// Return a copy with copied steps slice
	trajCopy := r.trajectory
	trajCopy.Steps = make([]TrajectoryStep, len(r.trajectory.Steps))
	copy(trajCopy.Steps, r.trajectory.Steps)

	return trajCopy
}

// Reset clears the recorded trajectory and starts a new recording session.
func (r *RecordingHarness) Reset() {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.trajectory = Trajectory{
		Steps:     make([]TrajectoryStep, 0),
		StartTime: time.Now(),
	}
}

// Complete performs a single LLM completion request and records it.
func (r *RecordingHarness) Complete(ctx context.Context, slot string, messages []llm.Message, opts ...llm.CompletionOption) (*llm.CompletionResponse, error) {
	startTime := time.Now()

	// Delegate to inner harness
	resp, err := r.inner.Complete(ctx, slot, messages, opts...)

	// Record the step
	duration := time.Since(startTime)
	step := TrajectoryStep{
		Type:      "llm",
		Name:      slot,
		Input:     messages,
		Output:    resp,
		StartTime: startTime,
		Duration:  duration,
	}
	if err != nil {
		step.Error = err.Error()
	}
	r.recordStep(step)

	return resp, err
}

// CompleteWithTools performs a completion with tool calling enabled and records it.
func (r *RecordingHarness) CompleteWithTools(ctx context.Context, slot string, messages []llm.Message, tools []llm.ToolDef) (*llm.CompletionResponse, error) {
	startTime := time.Now()

	// Delegate to inner harness
	resp, err := r.inner.CompleteWithTools(ctx, slot, messages, tools)

	// Record the step with tools in input
	duration := time.Since(startTime)
	step := TrajectoryStep{
		Type: "llm",
		Name: slot,
		Input: map[string]any{
			"messages": messages,
			"tools":    tools,
		},
		Output:    resp,
		StartTime: startTime,
		Duration:  duration,
	}
	if err != nil {
		step.Error = err.Error()
	}
	r.recordStep(step)

	return resp, err
}

// Stream performs a streaming completion request and records it.
func (r *RecordingHarness) Stream(ctx context.Context, slot string, messages []llm.Message) (<-chan llm.StreamChunk, error) {
	startTime := time.Now()

	// Delegate to inner harness
	ch, err := r.inner.Stream(ctx, slot, messages)

	// Record the step (note: output will be incomplete since streaming is async)
	duration := time.Since(startTime)
	step := TrajectoryStep{
		Type:      "llm",
		Name:      slot,
		Input:     messages,
		Output:    "streaming",
		StartTime: startTime,
		Duration:  duration,
	}
	if err != nil {
		step.Error = err.Error()
	}
	r.recordStep(step)

	return ch, err
}

// CallToolProto invokes a tool with proto messages and records the invocation.
func (r *RecordingHarness) CallToolProto(ctx context.Context, name string, request proto.Message, response proto.Message) error {
	startTime := time.Now()

	// Delegate to inner harness
	err := r.inner.CallToolProto(ctx, name, request, response)

	// Record the step
	duration := time.Since(startTime)
	step := TrajectoryStep{
		Type:      "tool",
		Name:      name,
		Input:     request,
		Output:    response,
		StartTime: startTime,
		Duration:  duration,
	}
	if err != nil {
		step.Error = err.Error()
	}
	r.recordStep(step)

	return err
}

// ListTools returns descriptors for all available tools.
func (r *RecordingHarness) ListTools(ctx context.Context) ([]tool.Descriptor, error) {
	// No recording for list operations
	return r.inner.ListTools(ctx)
}

// QueryPlugin sends a query to a plugin and records it.
func (r *RecordingHarness) QueryPlugin(ctx context.Context, name string, method string, params map[string]any) (any, error) {
	startTime := time.Now()

	// Delegate to inner harness
	result, err := r.inner.QueryPlugin(ctx, name, method, params)

	// Record the step
	duration := time.Since(startTime)
	step := TrajectoryStep{
		Type: "plugin",
		Name: name,
		Input: map[string]any{
			"method": method,
			"params": params,
		},
		Output:    result,
		StartTime: startTime,
		Duration:  duration,
	}
	if err != nil {
		step.Error = err.Error()
	}
	r.recordStep(step)

	return result, err
}

// ListPlugins returns descriptors for all available plugins.
func (r *RecordingHarness) ListPlugins(ctx context.Context) ([]plugin.Descriptor, error) {
	// No recording for list operations
	return r.inner.ListPlugins(ctx)
}

// DelegateToAgent assigns a task to another agent and records the delegation.
func (r *RecordingHarness) DelegateToAgent(ctx context.Context, name string, task agent.Task) (agent.Result, error) {
	startTime := time.Now()

	// Delegate to inner harness
	result, err := r.inner.DelegateToAgent(ctx, name, task)

	// Record the step
	duration := time.Since(startTime)
	step := TrajectoryStep{
		Type:      "delegate",
		Name:      name,
		Input:     task,
		Output:    result,
		StartTime: startTime,
		Duration:  duration,
	}
	if err != nil {
		step.Error = err.Error()
	}
	r.recordStep(step)

	return result, err
}

// ListAgents returns descriptors for all available agents.
func (r *RecordingHarness) ListAgents(ctx context.Context) ([]agent.Descriptor, error) {
	// No recording for list operations
	return r.inner.ListAgents(ctx)
}

// SubmitFinding records a new security finding and records the submission.
func (r *RecordingHarness) SubmitFinding(ctx context.Context, f *finding.Finding) error {
	startTime := time.Now()

	// Delegate to inner harness
	err := r.inner.SubmitFinding(ctx, f)

	// Record the step
	duration := time.Since(startTime)
	step := TrajectoryStep{
		Type:      "finding",
		Name:      "submit",
		Input:     f,
		StartTime: startTime,
		Duration:  duration,
	}
	if err != nil {
		step.Error = err.Error()
	}
	r.recordStep(step)

	return err
}

// Mission returns the current mission context.
func (r *RecordingHarness) Mission() types.MissionContext {
	// No recording for context access
	return r.inner.Mission()
}

// Target returns information about the target being tested.
func (r *RecordingHarness) Target() types.TargetInfo {
	// No recording for context access
	return r.inner.Target()
}

// Tracer returns an OpenTelemetry tracer for distributed tracing.
func (r *RecordingHarness) Tracer() trace.Tracer {
	// No recording for observability access
	return r.inner.Tracer()
}

// Logger returns a structured logger for the agent.
func (r *RecordingHarness) Logger() *slog.Logger {
	// No recording for observability access
	return r.inner.Logger()
}

// TokenUsage returns the token usage tracker for this execution.
func (r *RecordingHarness) TokenUsage() llm.TokenTracker {
	// No recording for observability access
	return r.inner.TokenUsage()
}

// WorldView delegates to the inner harness without recording a step. A world
// read is not an action the agent took, so putting it in the trajectory would
// score the agent for looking rather than for doing.
func (r *RecordingHarness) WorldView(ctx context.Context, focus ...agent.Handle) (agent.WorldView, error) {
	return r.inner.WorldView(ctx, focus...)
}

// Observe records a typed observation emit and delegates to the inner harness.
func (r *RecordingHarness) Observe(ctx context.Context, obs agent.Observation) error {
	startTime := time.Now()

	err := r.inner.Observe(ctx, obs)

	step := TrajectoryStep{
		Type:      "observation",
		Name:      "observe",
		Input:     obs,
		StartTime: startTime,
		Duration:  time.Since(startTime),
	}
	if err != nil {
		step.Error = err.Error()
	}
	r.recordStep(step)

	return err
}

// ============================================================================
// Planning Operations
// ============================================================================

// PlanContext returns the planning context for the current execution.
func (r *RecordingHarness) PlanContext() planning.PlanningContext {
	// No recording for context access
	return r.inner.PlanContext()
}

// ReportStepHints allows agents to provide feedback to the planning system and records it.
func (r *RecordingHarness) ReportStepHints(ctx context.Context, hints *planning.StepHints) error {
	startTime := time.Now()

	err := r.inner.ReportStepHints(ctx, hints)

	duration := time.Since(startTime)
	step := TrajectoryStep{
		Type:      "planning",
		Name:      "report_step_hints",
		Input:     hints,
		StartTime: startTime,
		Duration:  duration,
	}
	if err != nil {
		step.Error = err.Error()
	}
	r.recordStep(step)

	return err
}
