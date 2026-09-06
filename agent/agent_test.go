// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Zero Root AI

package agent

import (
	"context"
	"testing"

	"github.com/zeroroot-ai/sdk/llm"
	"github.com/zeroroot-ai/sdk/types"
)

// mockAgent is a simple implementation of the Agent interface for testing.
type mockAgent struct {
	name           string
	version        string
	description    string
	capabilities   []string
	targetTypes    []string
	techniqueTypes []string
	llmSlots       []llm.SlotDefinition
	initCalled     bool
	shutdownCalled bool
}

func (m *mockAgent) Name() string {
	return m.name
}

func (m *mockAgent) Version() string {
	return m.version
}

func (m *mockAgent) Description() string {
	return m.description
}

func (m *mockAgent) Capabilities() []string {
	return m.capabilities
}

func (m *mockAgent) TargetTypes() []string {
	return m.targetTypes
}

func (m *mockAgent) TechniqueTypes() []string {
	return m.techniqueTypes
}

func (m *mockAgent) LLMSlots() []llm.SlotDefinition {
	return m.llmSlots
}

func (m *mockAgent) Execute(ctx context.Context, harness Harness, task Task) (Result, error) {
	return NewSuccessResult("mock execution result"), nil
}

func (m *mockAgent) Initialize(ctx context.Context, config map[string]any) error {
	m.initCalled = true
	return nil
}

func (m *mockAgent) Shutdown(ctx context.Context) error {
	m.shutdownCalled = true
	return nil
}

func (m *mockAgent) Health(ctx context.Context) types.HealthStatus {
	return types.NewHealthyStatus("mock agent is healthy")
}

func TestMockAgent(t *testing.T) {
	agent := &mockAgent{
		name:           "test-agent",
		version:        "1.0.0",
		description:    "A test agent",
		capabilities:   []string{"prompt_injection"},
		targetTypes:    []string{"llm_chat"},
		techniqueTypes: []string{"prompt_injection"},
		llmSlots: []llm.SlotDefinition{
			{
				Name:             "primary",
				Description:      "Primary LLM slot",
				Required:         true,
				MinContextWindow: 8000,
			},
		},
	}

	ctx := context.Background()

	// Test metadata methods
	if agent.Name() != "test-agent" {
		t.Errorf("Name() = %s, want test-agent", agent.Name())
	}
	if agent.Version() != "1.0.0" {
		t.Errorf("Version() = %s, want 1.0.0", agent.Version())
	}
	if agent.Description() != "A test agent" {
		t.Errorf("Description() = %s, want 'A test agent'", agent.Description())
	}

	// Test capabilities
	caps := agent.Capabilities()
	if len(caps) != 1 || caps[0] != "prompt_injection" {
		t.Errorf("Capabilities() = %v, want [%s]", caps, "prompt_injection")
	}

	// Test target types
	targets := agent.TargetTypes()
	if len(targets) != 1 || targets[0] != "llm_chat" {
		t.Errorf("TargetTypes() = %v, want [%s]", targets, "llm_chat")
	}

	// Test technique types
	techniques := agent.TechniqueTypes()
	if len(techniques) != 1 || techniques[0] != "prompt_injection" {
		t.Errorf("TechniqueTypes() = %v, want [%s]", techniques, "prompt_injection")
	}

	// Test LLM slots
	slots := agent.LLMSlots()
	if len(slots) != 1 {
		t.Fatalf("LLMSlots() returned %d slots, want 1", len(slots))
	}
	if slots[0].Name != "primary" {
		t.Errorf("LLMSlots()[0].Name = %s, want primary", slots[0].Name)
	}

	// Test lifecycle methods
	err := agent.Initialize(ctx, map[string]any{"key": "value"})
	if err != nil {
		t.Errorf("Initialize() error = %v, want nil", err)
	}
	if !agent.initCalled {
		t.Error("Initialize() did not set initCalled flag")
	}

	err = agent.Shutdown(ctx)
	if err != nil {
		t.Errorf("Shutdown() error = %v, want nil", err)
	}
	if !agent.shutdownCalled {
		t.Error("Shutdown() did not set shutdownCalled flag")
	}

	// Test health check
	health := agent.Health(ctx)
	if !health.IsHealthy() {
		t.Errorf("Health() = %v, want healthy status", health)
	}

	// Test execution
	task := NewTask("test-task")
	result, err := agent.Execute(ctx, nil, *task)
	if err != nil {
		t.Errorf("Execute() error = %v, want nil", err)
	}
	if result.Status != StatusSuccess {
		t.Errorf("Execute() result.Status = %v, want %v", result.Status, StatusSuccess)
	}
}

// Note: Capability type tests removed as capabilities are now plain strings.
// Domain-specific capability constants moved to Gibson's taxonomy.
