// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Zero Root AI

package agent

import (
	"testing"
)

func TestConfig_SetTargetTypes(t *testing.T) {
	targets := []string{"llm_chat", "rag"}
	cfg := NewConfig().SetTargetTypes(targets)

	if len(cfg.targetTypes) != 2 {
		t.Errorf("len(targetTypes) = %d, want 2", len(cfg.targetTypes))
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
