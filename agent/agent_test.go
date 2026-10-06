// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Zero Root AI

package agent

import (
	"context"

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

// Note: Capability type tests removed as capabilities are now plain strings.
// Domain-specific capability constants moved to Gibson's taxonomy.
