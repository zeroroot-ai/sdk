// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Zero Root AI

package agent

import (
	"context"

	"github.com/zeroroot-ai/sdk/llm"
	"github.com/zeroroot-ai/sdk/types"
)

// Agent is the interface that all SDK agents must implement.
// Agents are autonomous components that execute security testing tasks
// using LLMs, tools, and plugins provided by the harness.
type Agent interface {
	// Name returns the unique identifier for this agent.
	// This should be a short, kebab-case name (e.g., "prompt-injector").
	Name() string

	// Version returns the semantic version of this agent.
	// Format: "major.minor.patch" (e.g., "1.0.0").
	Version() string

	// Description returns a human-readable description of what this agent does.
	// This should explain the agent's purpose and capabilities.
	Description() string

	// Capabilities returns the security testing capabilities this agent provides.
	// These indicate what types of vulnerabilities the agent can discover.
	// Returns a list of capability identifiers as strings.
	Capabilities() []string

	// TargetSchemas returns the target schemas this agent supports.
	// This defines the connection parameter requirements for each target type.
	// Returns nil or empty slice to accept any target type (opt-out validation).
	TargetSchemas() []types.TargetSchema

	// TargetTypes returns the types of target systems this agent can test.
	// This helps the framework match agents to appropriate targets.
	// Returns a list of target type identifiers as strings.
	TargetTypes() []string

	// TechniqueTypes returns the attack techniques this agent employs.
	// This categorizes the agent's testing methodology.
	// Returns a list of technique identifiers as strings.
	TechniqueTypes() []string

	// LLMSlots returns the LLM slot definitions required by this agent.
	// The framework will provision LLMs that meet these requirements.
	LLMSlots() []llm.SlotDefinition

	// Execute performs a task using the provided harness.
	// The harness provides access to LLMs, tools, plugins, and other resources.
	// The context can be used for cancellation and timeout control.
	Execute(ctx context.Context, harness Harness, task Task) (Result, error)

	// Initialize prepares the agent for execution.
	// This is called once when the agent is loaded, before any tasks are executed.
	// The config map contains agent-specific configuration parameters.
	Initialize(ctx context.Context, config map[string]any) error

	// Shutdown gracefully stops the agent and releases resources.
	// This is called when the agent is being unloaded or the system is shutting down.
	Shutdown(ctx context.Context) error

	// Health returns the current health status of the agent.
	// This is used for monitoring and diagnostics.
	Health(ctx context.Context) types.HealthStatus
}
