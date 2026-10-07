// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Zero Root AI

package graphrag_test

// mockNodeTypeRegistry is a simple mock implementation for testing.
type mockNodeTypeRegistry struct {
	types map[string][]string
}

func (m *mockNodeTypeRegistry) IsRegistered(nodeType string) bool {
	_, ok := m.types[nodeType]
	return ok
}

func (m *mockNodeTypeRegistry) AllNodeTypes() []string {
	types := make([]string, 0, len(m.types))
	for nodeType := range m.types {
		types = append(types, nodeType)
	}
	return types
}
