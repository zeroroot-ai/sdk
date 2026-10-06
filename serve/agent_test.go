// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Zero Root AI

package serve

import (
	"context"

	"github.com/zeroroot-ai/sdk/agent"
	"github.com/zeroroot-ai/sdk/types"
)

// mockAgent is a mock implementation of agent.Agent for testing.
type mockAgent struct {
	name        string
	version     string
	description string
	health      types.HealthStatus
	executeFunc func(ctx context.Context, harness agent.Harness, task agent.Task) (agent.Result, error)
}

func (m *mockAgent) Name() string        { return m.name }
func (m *mockAgent) Version() string     { return m.version }
func (m *mockAgent) Description() string { return m.description }

func (m *mockAgent) Capabilities() []string {
	return []string{"prompt_injection"}
}

func (m *mockAgent) TargetSchemas() []types.TargetSchema {
	return []types.TargetSchema{}
}

func (m *mockAgent) TargetTypes() []string {
	return []string{"llm_chat"}
}

func (m *mockAgent) TechniqueTypes() []string {
	return []string{"prompt_injection"}
}

func (m *mockAgent) Initialize(ctx context.Context, config map[string]any) error {
	return nil
}

func (m *mockAgent) Shutdown(ctx context.Context) error {
	return nil
}

func (m *mockAgent) Health(ctx context.Context) types.HealthStatus {
	if m.health.Status == "" {
		return types.NewHealthyStatus("Agent is healthy")
	}
	return m.health
}
