// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Zero Root AI

package agent

import (
	"context"
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

	// TargetTypes returns the types of target systems this agent can test.
	// This helps the framework match agents to appropriate targets.
	// Returns a list of target type identifiers as strings.
	TargetTypes() []string

	// TechniqueTypes returns the attack techniques this agent employs.
	// This categorizes the agent's testing methodology.
	// Returns a list of technique identifiers as strings.
	TechniqueTypes() []string

	// Execute performs a task using the provided harness.
	// The harness provides access to LLMs, tools, plugins, and other resources.
	// The context can be used for cancellation and timeout control.
	Execute(ctx context.Context, harness Harness, task Task) (Result, error)
}
