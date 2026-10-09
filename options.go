// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Zero Root AI

package sdk

import (
	"github.com/zeroroot-ai/sdk/agent"
	"github.com/zeroroot-ai/sdk/llm"
)

// AgentOption configures an Agent.
type AgentOption func(*agent.Config)

// WithName sets the agent's unique identifier.
// The name should be a kebab-case string (e.g., "prompt-injector").
func WithName(name string) AgentOption {
	return func(c *agent.Config) {
		c.SetName(name)
	}
}

// WithVersion sets the agent's semantic version.
// Should follow semantic versioning format (e.g., "1.0.0").
func WithVersion(version string) AgentOption {
	return func(c *agent.Config) {
		c.SetVersion(version)
	}
}

// WithDescription sets the agent's human-readable description.
// This should explain what the agent does and its purpose.
func WithDescription(desc string) AgentOption {
	return func(c *agent.Config) {
		c.SetDescription(desc)
	}
}

// WithTargetTypes sets the types of target systems the agent can test.
// This helps the framework match agents to appropriate targets.
func WithTargetTypes(targetTypes ...string) AgentOption {
	return func(c *agent.Config) {
		c.SetTargetTypes(targetTypes)
	}
}

// WithLLMSlot adds an LLM slot requirement to the agent.
// The framework will provision an LLM that meets these requirements.
//
// Example:
//
//	WithLLMSlot("primary", llm.SlotRequirements{
//	    MinContextWindow: 8000,
//	    RequiredFeatures: []string{"function_calling"},
//	})
func WithLLMSlot(name string, requirements llm.SlotRequirements) AgentOption {
	return func(c *agent.Config) {
		c.AddLLMSlot(name, requirements)
	}
}

// WithExecuteFunc sets the function that executes agent tasks.
// This is the core agent logic and is required.
func WithExecuteFunc(fn agent.ExecuteFunc) AgentOption {
	return func(c *agent.Config) {
		c.SetExecuteFunc(fn)
	}
}
