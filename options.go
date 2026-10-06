// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Zero Root AI

package sdk

import (
	"context"

	"github.com/zeroroot-ai/sdk/agent"
	"github.com/zeroroot-ai/sdk/llm"
	"github.com/zeroroot-ai/sdk/tool"
	"google.golang.org/protobuf/proto"
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

// ToolOption configures a Tool.
type ToolOption func(*tool.Config)

// WithToolName sets the tool's unique identifier.
// The name should be descriptive and unique within the system.
func WithToolName(name string) ToolOption {
	return func(c *tool.Config) {
		c.SetName(name)
	}
}

// WithToolVersion sets the tool's semantic version.
// Should follow semantic versioning format (e.g., "1.0.0").
func WithToolVersion(version string) ToolOption {
	return func(c *tool.Config) {
		c.SetVersion(version)
	}
}

// WithToolDescription sets the tool's human-readable description.
// This should explain what the tool does and how to use it.
func WithToolDescription(desc string) ToolOption {
	return func(c *tool.Config) {
		c.SetDescription(desc)
	}
}

// WithToolTags sets categorization tags for the tool.
// Tags help with discovery and filtering of tools.
func WithToolTags(tags ...string) ToolOption {
	return func(c *tool.Config) {
		c.SetTags(tags)
	}
}

// WithInputMessageType sets the proto message type for tool input.
// The messageType should be a fully-qualified proto message type name.
// Example: "zero_day.tools.http.HttpRequest"
func WithInputMessageType(messageType string) ToolOption {
	return func(c *tool.Config) {
		c.SetInputMessageType(messageType)
	}
}

// WithOutputMessageType sets the proto message type for tool output.
// The messageType should be a fully-qualified proto message type name.
// Example: "zero_day.tools.http.HttpResponse"
func WithOutputMessageType(messageType string) ToolOption {
	return func(c *tool.Config) {
		c.SetOutputMessageType(messageType)
	}
}

// WithExecuteProtoHandler sets the function that executes the tool with proto messages.
// This function implements the tool's core functionality and is required.
func WithExecuteProtoHandler(fn func(ctx context.Context, input proto.Message) (proto.Message, error)) ToolOption {
	return func(c *tool.Config) {
		c.SetExecuteProtoFunc(fn)
	}
}
