// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Zero Root AI

package graphrag_test

import (
	"fmt"
	"log"

	"github.com/zeroroot-ai/sdk/graphrag"
)

// ExampleRegistry demonstrates the global registry access pattern.
func ExampleRegistry() {
	registry := graphrag.Registry()

	// Check if a node type is registered
	if registry.IsRegistered(graphrag.NodeTypeHost) {
		fmt.Println("Host node type is registered")
	}

	// Get identifying properties
	props, err := registry.GetIdentifyingProperties(graphrag.NodeTypeHost)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("Host identifying properties: %v\n", props)

	// Output:
	// Host node type is registered
	// Host identifying properties: [ip]
}

// ExampleNodeTypeRegistry_GetIdentifyingProperties demonstrates retrieving
// identifying properties for different node types.
func ExampleNodeTypeRegistry_GetIdentifyingProperties() {
	registry := graphrag.NewDefaultNodeTypeRegistry()

	// Get identifying properties for a host node
	hostProps, _ := registry.GetIdentifyingProperties(graphrag.NodeTypeHost)
	fmt.Printf("Host: %v\n", hostProps)

	// Get identifying properties for a port node
	portProps, _ := registry.GetIdentifyingProperties(graphrag.NodeTypePort)
	fmt.Printf("Port: %v\n", portProps)

	// Get identifying properties for a finding node
	findingProps, _ := registry.GetIdentifyingProperties(graphrag.NodeTypeFinding)
	fmt.Printf("Finding: %v\n", findingProps)

	// Output:
	// Host: [ip]
	// Port: [host_id number protocol]
	// Finding: [mission_id fingerprint]
}

// ExampleNodeTypeRegistry_ValidateProperties demonstrates property validation
// for node creation.
func ExampleNodeTypeRegistry_ValidateProperties() {
	registry := graphrag.NewDefaultNodeTypeRegistry()

	// Valid host properties
	hostProps := map[string]any{
		"ip": "10.0.0.1",
	}

	missing, err := registry.ValidateProperties(graphrag.NodeTypeHost, hostProps)
	if err == nil {
		fmt.Println("Host properties are valid")
	}

	// Invalid port properties (missing protocol)
	portProps := map[string]any{
		"host_id": "host-123",
		"number":  443,
		// Missing protocol
	}

	missing, err = registry.ValidateProperties(graphrag.NodeTypePort, portProps)
	if err != nil {
		fmt.Printf("Port validation failed: missing %v\n", missing)
	}

	// Output:
	// Host properties are valid
	// Port validation failed: missing [protocol]
}

// ExampleNodeTypeRegistry_AllNodeTypes demonstrates listing all registered node types.
func ExampleNodeTypeRegistry_AllNodeTypes() {
	registry := graphrag.NewDefaultNodeTypeRegistry()

	// Get all registered node types
	types := registry.AllNodeTypes()

	// Show first 5 types (alphabetically sorted)
	fmt.Printf("First 5 node types: %v\n", types[:5])
	fmt.Printf("Total registered types: %d\n", len(types))

	// Output:
	// First 5 node types: [agent_run certificate domain endpoint evidence]
	// Total registered types: 16
}

// ExampleNodeTypeRegistry_IsRegistered demonstrates checking if a node type exists.
func ExampleNodeTypeRegistry_IsRegistered() {
	registry := graphrag.NewDefaultNodeTypeRegistry()

	// Check canonical types
	fmt.Printf("host registered: %v\n", registry.IsRegistered(graphrag.NodeTypeHost))
	fmt.Printf("port registered: %v\n", registry.IsRegistered(graphrag.NodeTypePort))

	// Check custom type
	fmt.Printf("custom_type registered: %v\n", registry.IsRegistered("custom_type"))

	// Output:
	// host registered: true
	// port registered: true
	// custom_type registered: false
}

// Example_nodeCreationWithValidation demonstrates using the registry to validate
// node properties before creation.
func Example_nodeCreationWithValidation() {
	registry := graphrag.Registry()

	// Define properties for a new port node
	properties := map[string]any{
		"host_id":  "host-abc123",
		"number":   443,
		"protocol": "tcp",
		"state":    "open",
	}

	// Validate before creating the node
	missing, err := registry.ValidateProperties(graphrag.NodeTypePort, properties)
	if err != nil {
		log.Fatalf("Invalid properties: %v missing: %v", err, missing)
	}

	// Create the node after validation passes
	node := graphrag.NewGraphNode(graphrag.NodeTypePort).
		WithProperties(properties)

	fmt.Printf("Created %s node with %d properties\n", node.Type, len(node.Properties))

	// Output:
	// Created port node with 4 properties
}

// Example_validationErrorHandling demonstrates proper error handling during validation.
func Example_validationErrorHandling() {
	registry := graphrag.Registry()

	// Properties missing the required 'ip' field
	properties := map[string]any{
		"name": "my-host",
	}

	// Attempt validation
	missing, err := registry.ValidateProperties(graphrag.NodeTypeHost, properties)
	if err != nil {
		fmt.Printf("Validation error: %v\n", err)
		fmt.Printf("Missing properties: %v\n", missing)
	}

	// Output:
	// Validation error: missing identifying properties for node type 'host': [ip]
	// Missing properties: [ip]
}

// Example_multiNodeTypeValidation demonstrates validating multiple node types.
func Example_multiNodeTypeValidation() {
	registry := graphrag.Registry()

	// Test data for different node types
	testCases := []struct {
		nodeType   string
		properties map[string]any
	}{
		{
			nodeType: graphrag.NodeTypeHost,
			properties: map[string]any{
				"ip": "192.168.1.1",
			},
		},
		{
			nodeType: graphrag.NodeTypeDomain,
			properties: map[string]any{
				"name": "example.com",
			},
		},
		{
			nodeType: graphrag.NodeTypeTechnology,
			properties: map[string]any{
				"name":    "nginx",
				"version": "1.21.0",
			},
		},
	}

	// Validate each node type
	for _, tc := range testCases {
		_, err := registry.ValidateProperties(tc.nodeType, tc.properties)
		if err == nil {
			fmt.Printf("%s: valid\n", tc.nodeType)
		}
	}

	// Output:
	// host: valid
	// domain: valid
	// technology: valid
}
