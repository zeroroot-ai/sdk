// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Zero Root AI

package agent

import (
	"testing"

	"github.com/zeroroot-ai/sdk/llm"
)

func TestConfig_SetTargetTypes(t *testing.T) {
	targets := []string{"llm_chat", "rag"}
	cfg := NewConfig().SetTargetTypes(targets)

	if len(cfg.targetTypes) != 2 {
		t.Errorf("len(targetTypes) = %d, want 2", len(cfg.targetTypes))
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
