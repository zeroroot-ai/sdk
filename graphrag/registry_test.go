// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Zero Root AI

package graphrag_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/zeroroot-ai/sdk/graphrag"
)

// TestNewDefaultNodeTypeRegistry verifies that the registry is properly initialized
// with all canonical node types from the taxonomy.
func TestNewDefaultNodeTypeRegistry(t *testing.T) {
	registry := graphrag.NewDefaultNodeTypeRegistry()
	require.NotNil(t, registry)

	// Verify all canonical node types are registered
	expectedTypes := []string{
		// Asset types
		graphrag.NodeTypeHost,
		graphrag.NodeTypePort,
		graphrag.NodeTypeService,
		graphrag.NodeTypeEndpoint,
		graphrag.NodeTypeDomain,
		graphrag.NodeTypeSubdomain,
		graphrag.NodeTypeTechnology,
		graphrag.NodeTypeCertificate,

		// Finding types
		graphrag.NodeTypeFinding,
		graphrag.NodeTypeEvidence,

		// Execution types
		graphrag.NodeTypeMission,
		graphrag.NodeTypeMissionRun,
		graphrag.NodeTypeAgentRun,
		graphrag.NodeTypeToolExecution,
		graphrag.NodeTypeLlmCall,

		// Attack types
		graphrag.NodeTypeTechnique,
	}

	for _, nodeType := range expectedTypes {
		assert.True(t, registry.IsRegistered(nodeType),
			"Expected node type %s to be registered", nodeType)
	}
}

// TestGetIdentifyingProperties_ValidTypes verifies that identifying properties
// are correctly returned for all canonical node types.
func TestGetIdentifyingProperties_ValidTypes(t *testing.T) {
	registry := graphrag.NewDefaultNodeTypeRegistry()

	tests := []struct {
		nodeType      string
		expectedProps []string
		description   string
	}{
		{
			nodeType:      graphrag.NodeTypeHost,
			expectedProps: []string{"ip"},
			description:   "Host identified by IP address",
		},
		{
			nodeType:      graphrag.NodeTypePort,
			expectedProps: []string{"host_id", "number", "protocol"},
			description:   "Port identified by host_id, port number, and protocol",
		},
		{
			nodeType:      graphrag.NodeTypeService,
			expectedProps: []string{"port_id", "name"},
			description:   "Service identified by port_id and service name",
		},
		{
			nodeType:      graphrag.NodeTypeEndpoint,
			expectedProps: []string{"service_id", "url", "method"},
			description:   "Endpoint identified by service_id, URL, and HTTP method",
		},
		{
			nodeType:      graphrag.NodeTypeDomain,
			expectedProps: []string{"name"},
			description:   "Domain identified by name",
		},
		{
			nodeType:      graphrag.NodeTypeSubdomain,
			expectedProps: []string{"parent_domain", "name"},
			description:   "Subdomain identified by parent_domain and name",
		},
		{
			nodeType:      graphrag.NodeTypeTechnology,
			expectedProps: []string{"name", "version"},
			description:   "Technology identified by name and version",
		},
		{
			nodeType:      graphrag.NodeTypeCertificate,
			expectedProps: []string{"fingerprint"},
			description:   "Certificate identified by fingerprint",
		},
		{
			nodeType:      graphrag.NodeTypeFinding,
			expectedProps: []string{"mission_id", "fingerprint"},
			description:   "Finding identified by mission_id and fingerprint",
		},
		{
			nodeType:      graphrag.NodeTypeEvidence,
			expectedProps: []string{"finding_id", "type", "fingerprint"},
			description:   "Evidence identified by finding_id, type, and fingerprint",
		},
		{
			nodeType:      graphrag.NodeTypeMission,
			expectedProps: []string{"name", "timestamp"},
			description:   "Mission identified by name and timestamp",
		},
		{
			nodeType:      graphrag.NodeTypeMissionRun,
			expectedProps: []string{"mission_id", "run_number"},
			description:   "Mission run identified by mission_id and run_number",
		},
		{
			nodeType:      graphrag.NodeTypeAgentRun,
			expectedProps: []string{"mission_run_id", "agent_name"},
			description:   "Agent run identified by mission_run_id and agent_name",
		},
		{
			nodeType:      graphrag.NodeTypeToolExecution,
			expectedProps: []string{"agent_run_id", "tool_name", "sequence"},
			description:   "Tool execution identified by agent_run_id, tool_name, and sequence",
		},
		{
			nodeType:      graphrag.NodeTypeLlmCall,
			expectedProps: []string{"agent_run_id", "sequence"},
			description:   "LLM call identified by agent_run_id and sequence",
		},
		{
			nodeType:      graphrag.NodeTypeTechnique,
			expectedProps: []string{"technique_id"},
			description:   "Technique identified by technique_id",
		},
	}

	for _, tt := range tests {
		t.Run(tt.nodeType, func(t *testing.T) {
			props, err := registry.GetIdentifyingProperties(tt.nodeType)
			require.NoError(t, err, tt.description)
			assert.ElementsMatch(t, tt.expectedProps, props,
				"Identifying properties mismatch for %s: %s", tt.nodeType, tt.description)
		})
	}
}

// TestGetIdentifyingProperties_UnknownType verifies that an error is returned
// for unknown node types.
func TestGetIdentifyingProperties_UnknownType(t *testing.T) {
	registry := graphrag.NewDefaultNodeTypeRegistry()

	props, err := registry.GetIdentifyingProperties("unknown_type")
	require.Error(t, err)
	assert.Nil(t, props)
	require.ErrorIs(t, err, graphrag.ErrNodeTypeNotRegistered,
		"Expected ErrNodeTypeNotRegistered")
	assert.Contains(t, err.Error(), "unknown_type",
		"Error message should contain the unknown type name")
}

// TestIsRegistered verifies the IsRegistered method correctly identifies
// registered and unregistered node types.
func TestIsRegistered(t *testing.T) {
	registry := graphrag.NewDefaultNodeTypeRegistry()

	tests := []struct {
		nodeType   string
		registered bool
	}{
		{graphrag.NodeTypeHost, true},
		{graphrag.NodeTypePort, true},
		{graphrag.NodeTypeFinding, true},
		{"custom_type", false},
		{"unknown_type", false},
		{"", false},
	}

	for _, tt := range tests {
		t.Run(tt.nodeType, func(t *testing.T) {
			result := registry.IsRegistered(tt.nodeType)
			assert.Equal(t, tt.registered, result,
				"IsRegistered(%s) = %v, want %v", tt.nodeType, result, tt.registered)
		})
	}
}

// TestValidateProperties_ValidProperties verifies that validation passes
// when all identifying properties are present.
func TestValidateProperties_ValidProperties(t *testing.T) {
	registry := graphrag.NewDefaultNodeTypeRegistry()

	tests := []struct {
		nodeType   string
		properties map[string]any
	}{
		{
			nodeType:   graphrag.NodeTypeHost,
			properties: map[string]any{"ip": "10.0.0.1"},
		},
		{
			nodeType: graphrag.NodeTypePort,
			properties: map[string]any{
				"host_id":  "host-123",
				"number":   443,
				"protocol": "tcp",
			},
		},
		{
			nodeType: graphrag.NodeTypeService,
			properties: map[string]any{
				"port_id": "port-123",
				"name":    "https",
			},
		},
		{
			nodeType: graphrag.NodeTypeEndpoint,
			properties: map[string]any{
				"service_id": "service-123",
				"url":        "/api/users",
				"method":     "GET",
			},
		},
		{
			nodeType: graphrag.NodeTypeFinding,
			properties: map[string]any{
				"mission_id":  "mission-123",
				"fingerprint": "abc123def456",
			},
		},
		{
			nodeType: graphrag.NodeTypeAgentRun,
			properties: map[string]any{
				"mission_run_id": "run-123",
				"agent_name":     "recon-agent",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.nodeType, func(t *testing.T) {
			missing, err := registry.ValidateProperties(tt.nodeType, tt.properties)
			assert.NoError(t, err, "Validation should pass with all properties present")
			assert.Nil(t, missing, "No properties should be missing")
		})
	}
}

// TestValidateProperties_MissingProperties verifies that validation fails
// when identifying properties are missing.
func TestValidateProperties_MissingProperties(t *testing.T) {
	registry := graphrag.NewDefaultNodeTypeRegistry()

	tests := []struct {
		name            string
		nodeType        string
		properties      map[string]any
		expectedMissing []string
	}{
		{
			name:            "Host missing IP",
			nodeType:        graphrag.NodeTypeHost,
			properties:      map[string]any{},
			expectedMissing: []string{"ip"},
		},
		{
			name:     "Port missing host_id",
			nodeType: graphrag.NodeTypePort,
			properties: map[string]any{
				"number":   443,
				"protocol": "tcp",
			},
			expectedMissing: []string{"host_id"},
		},
		{
			name:     "Port missing protocol and number",
			nodeType: graphrag.NodeTypePort,
			properties: map[string]any{
				"host_id": "host-123",
			},
			expectedMissing: []string{"number", "protocol"},
		},
		{
			name:            "Service missing all properties",
			nodeType:        graphrag.NodeTypeService,
			properties:      map[string]any{},
			expectedMissing: []string{"port_id", "name"},
		},
		{
			name:     "Finding missing fingerprint",
			nodeType: graphrag.NodeTypeFinding,
			properties: map[string]any{
				"mission_id": "mission-123",
			},
			expectedMissing: []string{"fingerprint"},
		},
		{
			name:     "Agent run missing agent_name",
			nodeType: graphrag.NodeTypeAgentRun,
			properties: map[string]any{
				"mission_run_id": "run-123",
			},
			expectedMissing: []string{"agent_name"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			missing, err := registry.ValidateProperties(tt.nodeType, tt.properties)
			require.Error(t, err, "Validation should fail with missing properties")
			require.ErrorIs(t, err, graphrag.ErrMissingIdentifyingProperties,
				"Expected ErrMissingIdentifyingProperties")
			assert.ElementsMatch(t, tt.expectedMissing, missing,
				"Missing properties list mismatch")
			assert.Contains(t, err.Error(), tt.nodeType,
				"Error message should contain node type")
		})
	}
}

// TestValidateProperties_NilAndEmptyValues verifies that nil and empty string values
// are treated as missing properties.
func TestValidateProperties_NilAndEmptyValues(t *testing.T) {
	registry := graphrag.NewDefaultNodeTypeRegistry()

	tests := []struct {
		name            string
		properties      map[string]any
		expectedMissing []string
	}{
		{
			name: "Nil IP value",
			properties: map[string]any{
				"ip": nil,
			},
			expectedMissing: []string{"ip"},
		},
		{
			name: "Empty string IP",
			properties: map[string]any{
				"ip": "",
			},
			expectedMissing: []string{"ip"},
		},
		{
			name: "Whitespace-only IP",
			properties: map[string]any{
				"ip": "   ",
			},
			expectedMissing: []string{"ip"},
		},
		{
			name: "Valid IP",
			properties: map[string]any{
				"ip": "10.0.0.1",
			},
			expectedMissing: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			missing, err := registry.ValidateProperties(graphrag.NodeTypeHost, tt.properties)
			if tt.expectedMissing == nil {
				assert.NoError(t, err)
				assert.Nil(t, missing)
			} else {
				require.Error(t, err)
				assert.ElementsMatch(t, tt.expectedMissing, missing)
			}
		})
	}
}

// TestValidateProperties_UnknownNodeType verifies that validation returns
// an error for unknown node types.
func TestValidateProperties_UnknownNodeType(t *testing.T) {
	registry := graphrag.NewDefaultNodeTypeRegistry()

	properties := map[string]any{"some_prop": "value"}
	missing, err := registry.ValidateProperties("unknown_type", properties)

	require.Error(t, err)
	assert.Nil(t, missing)
	assert.ErrorIs(t, err, graphrag.ErrNodeTypeNotRegistered,
		"Expected ErrNodeTypeNotRegistered")
}

// TestAllNodeTypes verifies that all registered node types are returned
// in sorted order.
func TestAllNodeTypes(t *testing.T) {
	registry := graphrag.NewDefaultNodeTypeRegistry()

	types := registry.AllNodeTypes()
	require.NotEmpty(t, types, "Should have registered node types")

	// Verify at least the expected count (16 types in new taxonomy)
	assert.GreaterOrEqual(t, len(types), 16,
		"Should have at least 16 node types registered")

	// Verify sorted order
	for i := 1; i < len(types); i++ {
		assert.Less(t, types[i-1], types[i],
			"Node types should be sorted alphabetically")
	}

	// Verify some expected types are present
	assert.Contains(t, types, graphrag.NodeTypeHost)
	assert.Contains(t, types, graphrag.NodeTypePort)
	assert.Contains(t, types, graphrag.NodeTypeFinding)
	assert.Contains(t, types, graphrag.NodeTypeMission)
}

// TestRegistry_GlobalInstance verifies that the global Registry() function
// returns a valid, initialized registry.
func TestRegistry_GlobalInstance(t *testing.T) {
	registry := graphrag.Registry()
	require.NotNil(t, registry, "Global registry should be initialized")

	// Verify it's functional
	assert.True(t, registry.IsRegistered(graphrag.NodeTypeHost))
	props, err := registry.GetIdentifyingProperties(graphrag.NodeTypeHost)
	require.NoError(t, err)
	assert.NotEmpty(t, props)
}

// TestSetRegistry_CustomImplementation verifies that a custom registry
// can be set and used globally.
func TestSetRegistry_CustomImplementation(t *testing.T) {
	// Save original registry
	originalRegistry := graphrag.Registry()
	defer graphrag.SetRegistry(originalRegistry)

	// Create a mock registry
	mockRegistry := &mockNodeTypeRegistry{
		types: map[string][]string{
			"custom_type": {"custom_prop1", "custom_prop2"},
		},
	}

	// Set custom registry
	graphrag.SetRegistry(mockRegistry)

	// Verify custom registry is now in use
	registry := graphrag.Registry()
	assert.True(t, registry.IsRegistered("custom_type"))
	assert.False(t, registry.IsRegistered(graphrag.NodeTypeHost))

	props, err := registry.GetIdentifyingProperties("custom_type")
	require.NoError(t, err)
	assert.Equal(t, []string{"custom_prop1", "custom_prop2"}, props)
}

// TestConcurrentAccess verifies that the registry is thread-safe
// for concurrent reads.
func TestConcurrentAccess(t *testing.T) {
	registry := graphrag.NewDefaultNodeTypeRegistry()

	const numGoroutines = 100
	const numOpsPerGoroutine = 100

	done := make(chan bool, numGoroutines)

	for range numGoroutines {
		go func() {
			for range numOpsPerGoroutine {
				// Mix of different operations
				registry.IsRegistered(graphrag.NodeTypeHost)
				registry.GetIdentifyingProperties(graphrag.NodeTypePort)
				registry.AllNodeTypes()
				registry.ValidateProperties(graphrag.NodeTypeService, map[string]any{
					"port_id": "port-123",
					"name":    "https",
				})
			}
			done <- true
		}()
	}

	// Wait for all goroutines to complete
	for range numGoroutines {
		<-done
	}
}

// TestGetIdentifyingProperties_ReturnsCopy verifies that the returned slice
// is a copy and modifications don't affect the internal registry.
func TestGetIdentifyingProperties_ReturnsCopy(t *testing.T) {
	registry := graphrag.NewDefaultNodeTypeRegistry()

	props1, err := registry.GetIdentifyingProperties(graphrag.NodeTypeHost)
	require.NoError(t, err)
	require.Len(t, props1, 1)

	// Modify the returned slice
	props1[0] = "modified"

	// Get properties again and verify they're unchanged
	props2, err := registry.GetIdentifyingProperties(graphrag.NodeTypeHost)
	require.NoError(t, err)
	assert.Equal(t, "ip", props2[0],
		"Internal registry should not be affected by external modifications")
}

// mockNodeTypeRegistry is a simple mock implementation for testing.
type mockNodeTypeRegistry struct {
	types map[string][]string
}

func (m *mockNodeTypeRegistry) GetIdentifyingProperties(nodeType string) ([]string, error) {
	props, ok := m.types[nodeType]
	if !ok {
		return nil, graphrag.ErrNodeTypeNotRegistered
	}
	return props, nil
}

func (m *mockNodeTypeRegistry) IsRegistered(nodeType string) bool {
	_, ok := m.types[nodeType]
	return ok
}

func (m *mockNodeTypeRegistry) ValidateProperties(nodeType string, properties map[string]any) ([]string, error) {
	props, err := m.GetIdentifyingProperties(nodeType)
	if err != nil {
		return nil, err
	}

	var missing []string
	for _, prop := range props {
		if _, ok := properties[prop]; !ok {
			missing = append(missing, prop)
		}
	}

	if len(missing) > 0 {
		return missing, graphrag.ErrMissingIdentifyingProperties
	}

	return nil, nil
}

func (m *mockNodeTypeRegistry) AllNodeTypes() []string {
	types := make([]string, 0, len(m.types))
	for nodeType := range m.types {
		types = append(types, nodeType)
	}
	return types
}
