// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Zero Root AI

package id_test

import (
	"fmt"

	"github.com/zeroroot-ai/sdk/graphrag"
	"github.com/zeroroot-ai/sdk/graphrag/id"
)

// ExampleNewGenerator demonstrates how to create and use the deterministic ID generator.
func ExampleNewGenerator() {
	// Create a registry with all canonical node types
	registry := graphrag.NewDefaultNodeTypeRegistry()

	// Create the ID generator
	gen := id.NewGenerator(registry)

	// Generate IDs for different node types
	hostID, _ := gen.Generate("host", map[string]any{
		"ip": "10.0.0.1",
	})
	fmt.Println("Host ID:", hostID)

	portID, _ := gen.Generate("port", map[string]any{
		"host_id":  hostID,
		"number":   443,
		"protocol": "tcp",
	})
	fmt.Println("Port ID:", portID)

	// The same properties always produce the same ID
	hostID2, _ := gen.Generate("host", map[string]any{
		"ip": "10.0.0.1",
	})
	fmt.Println("Same host ID:", hostID == hostID2)

	// Output:
	// Host ID: host:7GFL93dZcGubGnAF
	// Port ID: port:wYuyVrjQqsthmxUw
	// Same host ID: true
}

// ExampleDeterministicGenerator_Generate demonstrates ID generation for various node types.
func ExampleDeterministicGenerator_Generate() {
	registry := graphrag.NewDefaultNodeTypeRegistry()
	gen := id.NewGenerator(registry)

	// Domain node
	domainID, _ := gen.Generate("domain", map[string]any{
		"name": "example.com",
	})
	fmt.Printf("Domain: %s\n", domainID)

	// Subdomain node
	subdomainID, _ := gen.Generate("subdomain", map[string]any{
		"parent_domain": "example.com",
		"name":          "www",
	})
	fmt.Printf("Subdomain: %s\n", subdomainID)

	// Service node
	serviceID, _ := gen.Generate("service", map[string]any{
		"port_id": "port:xyz789",
		"name":    "https",
	})
	fmt.Printf("Service: %s\n", serviceID)

	// Finding node
	findingID, _ := gen.Generate("finding", map[string]any{
		"mission_id":  "mission:abc123",
		"fingerprint": "sql-injection-login-form",
	})
	fmt.Printf("Finding: %s\n", findingID)

	// Output:
	// Domain: domain:652KfmDGdzCge9o3
	// Subdomain: subdomain:ZLC6cUUHsRL8qpr3
	// Service: service:NIUf09FIdghf7MH-
	// Finding: finding:1i0cRJcWgYSZZNvW
}

// ExampleDeterministicGenerator_Generate_caseInsensitive demonstrates that
// string properties are normalized to lowercase for consistent ID generation.
func ExampleDeterministicGenerator_Generate_caseInsensitive() {
	registry := graphrag.NewDefaultNodeTypeRegistry()
	gen := id.NewGenerator(registry)

	// Different cases produce the same ID
	id1, _ := gen.Generate("domain", map[string]any{
		"name": "Example.COM",
	})

	id2, _ := gen.Generate("domain", map[string]any{
		"name": "example.com",
	})

	fmt.Println("IDs match:", id1 == id2)

	// Output:
	// IDs match: true
}

// ExampleDeterministicGenerator_Generate_whitespaceNormalization demonstrates
// that whitespace is trimmed from string properties.
func ExampleDeterministicGenerator_Generate_whitespaceNormalization() {
	registry := graphrag.NewDefaultNodeTypeRegistry()
	gen := id.NewGenerator(registry)

	// Whitespace is trimmed
	id1, _ := gen.Generate("domain", map[string]any{
		"name": "  example.com  ",
	})

	id2, _ := gen.Generate("domain", map[string]any{
		"name": "example.com",
	})

	fmt.Println("IDs match:", id1 == id2)

	// Output:
	// IDs match: true
}

// ExampleDeterministicGenerator_Generate_missingProperties demonstrates
// error handling when required properties are missing.
func ExampleDeterministicGenerator_Generate_missingProperties() {
	registry := graphrag.NewDefaultNodeTypeRegistry()
	gen := id.NewGenerator(registry)

	// Missing required property
	_, err := gen.Generate("host", map[string]any{
		// "ip" is required but missing
	})

	if err != nil {
		fmt.Println("Error:", err)
	}

	// Output:
	// Error: validation failed for node type "host": missing identifying properties for node type 'host': [ip] (missing: [ip])
}

// ExampleDeterministicGenerator_Generate_unknownNodeType demonstrates
// error handling for unregistered node types.
func ExampleDeterministicGenerator_Generate_unknownNodeType() {
	registry := graphrag.NewDefaultNodeTypeRegistry()
	gen := id.NewGenerator(registry)

	_, err := gen.Generate("custom_type", map[string]any{
		"foo": "bar",
	})

	if err != nil {
		fmt.Println("Error occurred:", err != nil)
	}

	// Output:
	// Error occurred: true
}
