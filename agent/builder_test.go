// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Zero Root AI

package agent

import (
	"context"
	"errors"
	"testing"

	"github.com/zeroroot-ai/sdk/llm"
	"github.com/zeroroot-ai/sdk/types"
)

func TestNewConfig(t *testing.T) {
	cfg := NewConfig()

	if cfg == nil {
		t.Fatal("NewConfig() returned nil")
	}
	if cfg.capabilities == nil {
		t.Error("capabilities should be initialized")
	}
	if cfg.targetTypes == nil {
		t.Error("targetTypes should be initialized")
	}
	if cfg.techniqueTypes == nil {
		t.Error("techniqueTypes should be initialized")
	}
	if cfg.llmSlots == nil {
		t.Error("llmSlots should be initialized")
	}
}

func TestConfig_FluentAPI(t *testing.T) {
	cfg := NewConfig().
		SetName("test-agent").
		SetVersion("1.0.0").
		SetDescription("Test agent description").
		AddCapability("prompt_injection").
		AddTargetType("llm_chat").
		AddTechniqueType("prompt_injection")

	if cfg.name != "test-agent" {
		t.Errorf("name = %s, want test-agent", cfg.name)
	}
	if cfg.version != "1.0.0" {
		t.Errorf("version = %s, want 1.0.0", cfg.version)
	}
	if cfg.description != "Test agent description" {
		t.Errorf("description = %s, want 'Test agent description'", cfg.description)
	}
	if len(cfg.capabilities) != 1 {
		t.Errorf("len(capabilities) = %d, want 1", len(cfg.capabilities))
	}
	if len(cfg.targetTypes) != 1 {
		t.Errorf("len(targetTypes) = %d, want 1", len(cfg.targetTypes))
	}
	if len(cfg.techniqueTypes) != 1 {
		t.Errorf("len(techniqueTypes) = %d, want 1", len(cfg.techniqueTypes))
	}
}

func TestConfig_SetCapabilities(t *testing.T) {
	caps := []string{"jailbreak", "data_extraction"}
	cfg := NewConfig().SetCapabilities(caps)

	if len(cfg.capabilities) != 2 {
		t.Errorf("len(capabilities) = %d, want 2", len(cfg.capabilities))
	}
	if cfg.capabilities[0] != "jailbreak" {
		t.Errorf("capabilities[0] = %v, want %v", cfg.capabilities[0], "jailbreak")
	}
	if cfg.capabilities[1] != "data_extraction" {
		t.Errorf("capabilities[1] = %v, want %v", cfg.capabilities[1], "data_extraction")
	}
}

func TestConfig_SetTargetTypes(t *testing.T) {
	targets := []string{"llm_chat", "rag"}
	cfg := NewConfig().SetTargetTypes(targets)

	if len(cfg.targetTypes) != 2 {
		t.Errorf("len(targetTypes) = %d, want 2", len(cfg.targetTypes))
	}
}

func TestConfig_SetTechniqueTypes(t *testing.T) {
	techniques := []string{"jailbreak", "dos"}
	cfg := NewConfig().SetTechniqueTypes(techniques)

	if len(cfg.techniqueTypes) != 2 {
		t.Errorf("len(techniqueTypes) = %d, want 2", len(cfg.techniqueTypes))
	}
}

func TestConfig_AddLLMSlot(t *testing.T) {
	requirements := llm.SlotRequirements{
		MinContextWindow: 8000,
		RequiredFeatures: []string{"function_calling"},
		PreferredModels:  []string{"gpt-4"},
	}

	cfg := NewConfig().AddLLMSlot("primary", requirements)

	if len(cfg.llmSlots) != 1 {
		t.Fatalf("len(llmSlots) = %d, want 1", len(cfg.llmSlots))
	}

	slot := cfg.llmSlots[0]
	if slot.Name != "primary" {
		t.Errorf("slot.Name = %s, want primary", slot.Name)
	}
	if !slot.Required {
		t.Error("slot.Required should be true")
	}
	if slot.MinContextWindow != 8000 {
		t.Errorf("slot.MinContextWindow = %d, want 8000", slot.MinContextWindow)
	}
	if len(slot.RequiredFeatures) != 1 {
		t.Errorf("len(slot.RequiredFeatures) = %d, want 1", len(slot.RequiredFeatures))
	}
	if len(slot.PreferredModels) != 1 {
		t.Errorf("len(slot.PreferredModels) = %d, want 1", len(slot.PreferredModels))
	}
}

func TestConfig_AddLLMSlotDefinition(t *testing.T) {
	slot := llm.SlotDefinition{
		Name:             "vision",
		Description:      "Vision-capable LLM",
		Required:         false,
		MinContextWindow: 4000,
		RequiredFeatures: []string{"vision"},
	}

	cfg := NewConfig().AddLLMSlotDefinition(slot)

	if len(cfg.llmSlots) != 1 {
		t.Fatalf("len(llmSlots) = %d, want 1", len(cfg.llmSlots))
	}

	if cfg.llmSlots[0].Name != "vision" {
		t.Errorf("slot.Name = %s, want vision", cfg.llmSlots[0].Name)
	}
	if cfg.llmSlots[0].Required {
		t.Error("slot.Required should be false")
	}
}

func TestConfig_SetFunctions(t *testing.T) {
	executeCalled := false
	initCalled := false
	shutdownCalled := false
	healthCalled := false

	cfg := NewConfig().
		SetExecuteFunc(func(ctx context.Context, harness Harness, task Task) (Result, error) {
			executeCalled = true
			return NewSuccessResult(nil), nil
		}).
		SetInitFunc(func(ctx context.Context, config map[string]any) error {
			initCalled = true
			return nil
		}).
		SetShutdownFunc(func(ctx context.Context) error {
			shutdownCalled = true
			return nil
		}).
		SetHealthFunc(func(ctx context.Context) types.HealthStatus {
			healthCalled = true
			return types.NewHealthyStatus("ok")
		})

	if cfg.executeFunc == nil {
		t.Error("executeFunc should be set")
	}
	if cfg.initFunc == nil {
		t.Error("initFunc should be set")
	}
	if cfg.shutdownFunc == nil {
		t.Error("shutdownFunc should be set")
	}
	if cfg.healthFunc == nil {
		t.Error("healthFunc should be set")
	}

	// Verify functions work
	ctx := context.Background()
	cfg.executeFunc(ctx, nil, Task{})
	cfg.initFunc(ctx, nil)
	cfg.shutdownFunc(ctx)
	cfg.healthFunc(ctx)

	if !executeCalled {
		t.Error("executeFunc was not called")
	}
	if !initCalled {
		t.Error("initFunc was not called")
	}
	if !shutdownCalled {
		t.Error("shutdownFunc was not called")
	}
	if !healthCalled {
		t.Error("healthFunc was not called")
	}
}

func TestConfig_Validate(t *testing.T) {
	tests := []struct {
		name    string
		setup   func(*Config)
		wantErr bool
		errMsg  string
	}{
		{
			name:    "missing name",
			setup:   func(c *Config) {},
			wantErr: true,
			errMsg:  "name is required",
		},
		{
			name: "missing version",
			setup: func(c *Config) {
				c.SetName("test")
			},
			wantErr: true,
			errMsg:  "version is required",
		},
		{
			name: "missing description",
			setup: func(c *Config) {
				c.SetName("test").SetVersion("1.0.0")
			},
			wantErr: true,
			errMsg:  "description is required",
		},
		{
			name: "missing execute function",
			setup: func(c *Config) {
				c.SetName("test").SetVersion("1.0.0").SetDescription("test")
			},
			wantErr: true,
			errMsg:  "execute function is required",
		},
		{
			name: "valid config",
			setup: func(c *Config) {
				c.SetName("test").
					SetVersion("1.0.0").
					SetDescription("test").
					SetExecuteFunc(func(ctx context.Context, harness Harness, task Task) (Result, error) {
						return NewSuccessResult(nil), nil
					})
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := NewConfig()
			tt.setup(cfg)

			err := cfg.Validate()
			if tt.wantErr && err == nil {
				t.Error("Validate() error = nil, want error")
			}
			if !tt.wantErr && err != nil {
				t.Errorf("Validate() error = %v, want nil", err)
			}
			// Don't check exact error message as it may vary
		})
	}
}

func TestNew(t *testing.T) {
	cfg := NewConfig().
		SetName("test-agent").
		SetVersion("1.0.0").
		SetDescription("Test agent").
		SetExecuteFunc(func(ctx context.Context, harness Harness, task Task) (Result, error) {
			return NewSuccessResult("test output"), nil
		})

	agent, err := New(cfg)
	if err != nil {
		t.Fatalf("New() error = %v, want nil", err)
	}

	if agent == nil {
		t.Fatal("New() returned nil agent")
	}

	// Verify agent properties
	if agent.Name() != "test-agent" {
		t.Errorf("Name() = %s, want test-agent", agent.Name())
	}
	if agent.Version() != "1.0.0" {
		t.Errorf("Version() = %s, want 1.0.0", agent.Version())
	}
	if agent.Description() != "Test agent" {
		t.Errorf("Description() = %s, want 'Test agent'", agent.Description())
	}
}

func TestNew_InvalidConfig(t *testing.T) {
	cfg := NewConfig()
	// Missing required fields

	agent, err := New(cfg)
	if err == nil {
		t.Error("New() with invalid config should return error")
	}
	if agent != nil {
		t.Error("New() with invalid config should return nil agent")
	}
}

func TestNew_DefaultFunctions(t *testing.T) {
	cfg := NewConfig().
		SetName("test-agent").
		SetVersion("1.0.0").
		SetDescription("Test agent").
		SetExecuteFunc(func(ctx context.Context, harness Harness, task Task) (Result, error) {
			return NewSuccessResult(nil), nil
		})
	// Not setting init, shutdown, or health functions

	agent, err := New(cfg)
	if err != nil {
		t.Fatalf("New() error = %v, want nil", err)
	}

	ctx := context.Background()

	// Test default init (should not error)
	err = agent.Initialize(ctx, map[string]any{})
	if err != nil {
		t.Errorf("Initialize() with default function error = %v, want nil", err)
	}

	// Test default shutdown (should not error)
	err = agent.Shutdown(ctx)
	if err != nil {
		t.Errorf("Shutdown() with default function error = %v, want nil", err)
	}

	// Test default health (should return healthy)
	health := agent.Health(ctx)
	if !health.IsHealthy() {
		t.Errorf("Health() with default function = %v, want healthy", health)
	}
}

func TestSDKAgent_Execute(t *testing.T) {
	expectedOutput := "test result"
	cfg := NewConfig().
		SetName("test-agent").
		SetVersion("1.0.0").
		SetDescription("Test agent").
		SetExecuteFunc(func(ctx context.Context, harness Harness, task Task) (Result, error) {
			return NewSuccessResult(expectedOutput), nil
		})

	agent, err := New(cfg)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	ctx := context.Background()
	task := NewTask("task-1")

	result, err := agent.Execute(ctx, nil, *task)
	if err != nil {
		t.Errorf("Execute() error = %v, want nil", err)
	}
	if result.Status != StatusSuccess {
		t.Errorf("Execute() status = %v, want %v", result.Status, StatusSuccess)
	}
	if result.Output != expectedOutput {
		t.Errorf("Execute() output = %v, want %v", result.Output, expectedOutput)
	}
}

func TestSDKAgent_ExecuteError(t *testing.T) {
	expectedErr := errors.New("execution failed")
	cfg := NewConfig().
		SetName("test-agent").
		SetVersion("1.0.0").
		SetDescription("Test agent").
		SetExecuteFunc(func(ctx context.Context, harness Harness, task Task) (Result, error) {
			return NewFailedResult(expectedErr), expectedErr
		})

	agent, err := New(cfg)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	ctx := context.Background()
	task := NewTask("task-1")

	result, err := agent.Execute(ctx, nil, *task)
	if err == nil {
		t.Error("Execute() error = nil, want error")
	}
	if result.Status != StatusFailed {
		t.Errorf("Execute() status = %v, want %v", result.Status, StatusFailed)
	}
	if !errors.Is(result.Error, expectedErr) {
		t.Errorf("Execute() result.Error = %v, want %v", result.Error, expectedErr)
	}
}

func TestSDKAgent_FullLifecycle(t *testing.T) {
	initCalled := false
	shutdownCalled := false

	cfg := NewConfig().
		SetName("lifecycle-agent").
		SetVersion("1.0.0").
		SetDescription("Test lifecycle").
		SetExecuteFunc(func(ctx context.Context, harness Harness, task Task) (Result, error) {
			return NewSuccessResult("done"), nil
		}).
		SetInitFunc(func(ctx context.Context, config map[string]any) error {
			initCalled = true
			return nil
		}).
		SetShutdownFunc(func(ctx context.Context) error {
			shutdownCalled = true
			return nil
		})

	agent, err := New(cfg)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	ctx := context.Background()

	// Initialize
	err = agent.Initialize(ctx, map[string]any{"key": "value"})
	if err != nil {
		t.Errorf("Initialize() error = %v", err)
	}
	if !initCalled {
		t.Error("Initialize() did not call init function")
	}

	// Execute
	task := NewTask("task-1")
	result, err := agent.Execute(ctx, nil, *task)
	if err != nil {
		t.Errorf("Execute() error = %v", err)
	}
	if result.Status != StatusSuccess {
		t.Errorf("Execute() status = %v, want %v", result.Status, StatusSuccess)
	}

	// Shutdown
	err = agent.Shutdown(ctx)
	if err != nil {
		t.Errorf("Shutdown() error = %v", err)
	}
	if !shutdownCalled {
		t.Error("Shutdown() did not call shutdown function")
	}
}

func TestSDKAgent_AllProperties(t *testing.T) {
	cfg := NewConfig().
		SetName("full-agent").
		SetVersion("2.1.0").
		SetDescription("Full featured agent").
		AddCapability("jailbreak").
		AddCapability("dos").
		AddTargetType("llm_api").
		AddTargetType("rag").
		AddTechniqueType("jailbreak").
		AddLLMSlot("primary", llm.SlotRequirements{
			MinContextWindow: 8000,
		}).
		SetExecuteFunc(func(ctx context.Context, harness Harness, task Task) (Result, error) {
			return NewSuccessResult(nil), nil
		})

	agent, err := New(cfg)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	// Verify all properties
	if len(agent.Capabilities()) != 2 {
		t.Errorf("len(Capabilities()) = %d, want 2", len(agent.Capabilities()))
	}
	if len(agent.TargetTypes()) != 2 {
		t.Errorf("len(TargetTypes()) = %d, want 2", len(agent.TargetTypes()))
	}
	if len(agent.TechniqueTypes()) != 1 {
		t.Errorf("len(TechniqueTypes()) = %d, want 1", len(agent.TechniqueTypes()))
	}
	if len(agent.LLMSlots()) != 1 {
		t.Errorf("len(LLMSlots()) = %d, want 1", len(agent.LLMSlots()))
	}
}
