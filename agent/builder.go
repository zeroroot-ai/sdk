// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Zero Root AI

package agent

import (
	"context"
	"errors"
	"fmt"

	"github.com/zeroroot-ai/sdk/llm"
)

// Config holds configuration for building an agent using the SDK.
// This provides a flexible way to define agent behavior without implementing
// the full Agent interface from scratch.
type Config struct {
	name        string
	version     string
	description string
	targetTypes []string
	llmSlots    []llm.SlotDefinition
	executeFunc ExecuteFunc
}

// ExecuteFunc is the function signature for agent task execution.
// Implementations should perform the task and return the result.
type ExecuteFunc func(ctx context.Context, harness Harness, task Task) (Result, error)

// NewConfig creates a new agent configuration with default values.
func NewConfig() *Config {
	return &Config{
		targetTypes: []string{},
		llmSlots:    []llm.SlotDefinition{},
	}
}

// SetName sets the agent name.
// The name should be a unique, kebab-case identifier.
func (c *Config) SetName(name string) *Config {
	c.name = name
	return c
}

// SetVersion sets the agent version.
// Should follow semantic versioning (e.g., "1.0.0").
func (c *Config) SetVersion(version string) *Config {
	c.version = version
	return c
}

// SetDescription sets the agent description.
// Should explain what the agent does and its purpose.
func (c *Config) SetDescription(desc string) *Config {
	c.description = desc
	return c
}

// SetTargetTypes sets the types of targets the agent can test.
func (c *Config) SetTargetTypes(types []string) *Config {
	c.targetTypes = types
	return c
}

// AddLLMSlot adds an LLM slot definition to the agent.
// The name identifies the slot (e.g., "primary", "vision").
// The requirements specify what capabilities the LLM must have.
func (c *Config) AddLLMSlot(name string, requirements llm.SlotRequirements) *Config {
	slot := llm.SlotDefinition{
		Name:             name,
		Required:         true,
		MinContextWindow: requirements.MinContextWindow,
		RequiredFeatures: requirements.RequiredFeatures,
		PreferredModels:  requirements.PreferredModels,
	}
	c.llmSlots = append(c.llmSlots, slot)
	return c
}

// SetExecuteFunc sets the function that executes tasks.
// This is the core agent logic.
func (c *Config) SetExecuteFunc(fn ExecuteFunc) *Config {
	c.executeFunc = fn
	return c
}

// Validate checks if the configuration is valid and complete.
func (c *Config) Validate() error {
	if c.name == "" {
		return errors.New("agent name is required")
	}
	if c.version == "" {
		return errors.New("agent version is required")
	}
	if c.description == "" {
		return errors.New("agent description is required")
	}
	if c.executeFunc == nil {
		return errors.New("execute function is required")
	}
	return nil
}

// New creates a new agent from the configuration.
// Returns an error if the configuration is invalid.
func New(cfg *Config) (Agent, error) {
	if err := cfg.Validate(); err != nil {
		return nil, fmt.Errorf("invalid agent config: %w", err)
	}

	return &sdkAgent{
		name:        cfg.name,
		version:     cfg.version,
		description: cfg.description,
		targetTypes: cfg.targetTypes,
		llmSlots:    cfg.llmSlots,
		executeFunc: cfg.executeFunc,
	}, nil
}

// sdkAgent is the internal implementation of the Agent interface.
// It wraps user-provided functions to implement the full Agent interface.
type sdkAgent struct {
	name        string
	version     string
	description string
	targetTypes []string
	llmSlots    []llm.SlotDefinition
	executeFunc ExecuteFunc
}

// Name returns the agent's unique identifier.
func (a *sdkAgent) Name() string {
	return a.name
}

// Version returns the agent's semantic version.
func (a *sdkAgent) Version() string {
	return a.version
}

// Description returns a description of what the agent does.
func (a *sdkAgent) Description() string {
	return a.description
}

// Capabilities returns the security testing capabilities the agent provides.
// An agent built with New declares none.
func (a *sdkAgent) Capabilities() []string {
	return []string{}
}

// TargetTypes returns the types of targets the agent can test.
func (a *sdkAgent) TargetTypes() []string {
	return a.targetTypes
}

// TechniqueTypes returns the attack techniques the agent employs.
// An agent built with New declares none.
func (a *sdkAgent) TechniqueTypes() []string {
	return []string{}
}

// Execute performs a task using the configured execute function.
func (a *sdkAgent) Execute(ctx context.Context, harness Harness, task Task) (Result, error) {
	return a.executeFunc(ctx, harness, task)
}

// Shutdown has nothing to release for an agent built with New.
func (a *sdkAgent) Shutdown(_ context.Context) error {
	return nil
}
