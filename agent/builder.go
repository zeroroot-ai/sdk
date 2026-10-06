// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Zero Root AI

package agent

import (
	"context"
	"errors"
	"fmt"

	"github.com/zeroroot-ai/sdk/llm"
	"github.com/zeroroot-ai/sdk/types"
)

// Config holds configuration for building an agent using the SDK.
// This provides a flexible way to define agent behavior without implementing
// the full Agent interface from scratch.
type Config struct {
	name           string
	version        string
	description    string
	capabilities   []string
	targetSchemas  []types.TargetSchema
	targetTypes    []string
	techniqueTypes []string
	llmSlots       []llm.SlotDefinition
	executeFunc    ExecuteFunc
	initFunc       InitFunc
	shutdownFunc   ShutdownFunc
	healthFunc     HealthFunc
}

// ExecuteFunc is the function signature for agent task execution.
// Implementations should perform the task and return the result.
type ExecuteFunc func(ctx context.Context, harness Harness, task Task) (Result, error)

// InitFunc is the function signature for agent initialization.
// Implementations should prepare the agent for execution.
type InitFunc func(ctx context.Context, config map[string]any) error

// ShutdownFunc is the function signature for agent shutdown.
// Implementations should release resources and perform cleanup.
type ShutdownFunc func(ctx context.Context) error

// HealthFunc is the function signature for health checks.
// Implementations should return the current health status.
type HealthFunc func(ctx context.Context) types.HealthStatus

// NewConfig creates a new agent configuration with default values.
func NewConfig() *Config {
	return &Config{
		capabilities:   []string{},
		targetSchemas:  []types.TargetSchema{},
		targetTypes:    []string{},
		techniqueTypes: []string{},
		llmSlots:       []llm.SlotDefinition{},
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

	// Set defaults for optional functions
	initFunc := cfg.initFunc
	if initFunc == nil {
		initFunc = func(ctx context.Context, config map[string]any) error {
			return nil
		}
	}

	shutdownFunc := cfg.shutdownFunc
	if shutdownFunc == nil {
		shutdownFunc = func(ctx context.Context) error {
			return nil
		}
	}

	healthFunc := cfg.healthFunc
	if healthFunc == nil {
		healthFunc = func(ctx context.Context) types.HealthStatus {
			return types.NewHealthyStatus("agent is operational")
		}
	}

	return &sdkAgent{
		name:           cfg.name,
		version:        cfg.version,
		description:    cfg.description,
		capabilities:   cfg.capabilities,
		targetSchemas:  cfg.targetSchemas,
		targetTypes:    cfg.targetTypes,
		techniqueTypes: cfg.techniqueTypes,
		llmSlots:       cfg.llmSlots,
		executeFunc:    cfg.executeFunc,
		initFunc:       initFunc,
		shutdownFunc:   shutdownFunc,
		healthFunc:     healthFunc,
	}, nil
}

// sdkAgent is the internal implementation of the Agent interface.
// It wraps user-provided functions to implement the full Agent interface.
type sdkAgent struct {
	name           string
	version        string
	description    string
	capabilities   []string
	targetSchemas  []types.TargetSchema
	targetTypes    []string
	techniqueTypes []string
	llmSlots       []llm.SlotDefinition
	executeFunc    ExecuteFunc
	initFunc       InitFunc
	shutdownFunc   ShutdownFunc
	healthFunc     HealthFunc
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
func (a *sdkAgent) Capabilities() []string {
	return a.capabilities
}

// TargetSchemas returns the target schemas this agent supports.
func (a *sdkAgent) TargetSchemas() []types.TargetSchema {
	return a.targetSchemas
}

// TargetTypes returns the types of targets the agent can test.
func (a *sdkAgent) TargetTypes() []string {
	return a.targetTypes
}

// TechniqueTypes returns the attack techniques the agent employs.
func (a *sdkAgent) TechniqueTypes() []string {
	return a.techniqueTypes
}

// LLMSlots returns the LLM slot definitions required by the agent.
func (a *sdkAgent) LLMSlots() []llm.SlotDefinition {
	return a.llmSlots
}

// Execute performs a task using the configured execute function.
func (a *sdkAgent) Execute(ctx context.Context, harness Harness, task Task) (Result, error) {
	return a.executeFunc(ctx, harness, task)
}

// Initialize calls the configured init function.
func (a *sdkAgent) Initialize(ctx context.Context, config map[string]any) error {
	return a.initFunc(ctx, config)
}

// Shutdown calls the configured shutdown function.
func (a *sdkAgent) Shutdown(ctx context.Context) error {
	return a.shutdownFunc(ctx)
}

// Health calls the configured health function.
func (a *sdkAgent) Health(ctx context.Context) types.HealthStatus {
	return a.healthFunc(ctx)
}
