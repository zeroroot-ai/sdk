// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Zero Root AI

package graphrag_test

import (
	"fmt"

	"github.com/zeroroot-ai/sdk/graphrag"
)

// ExampleNewGraphNode demonstrates creating a GraphNode with properties.
func ExampleNewGraphNode() {
	node := graphrag.NewGraphNode("finding").
		WithID("finding-123").
		WithContent("Cross-Site Scripting (XSS) in search parameter").
		WithProperty("severity", "high").
		WithProperty("confidence", 0.92).
		WithProperty("endpoint", "/api/search")

	fmt.Println("Type:", node.Type)
	fmt.Println("ID:", node.ID)
	fmt.Println("Severity:", node.Properties["severity"])
	// Output:
	// Type: finding
	// ID: finding-123
	// Severity: high
}

// ExampleNewRelationship demonstrates creating relationships between nodes.
func ExampleNewRelationship() {
	// Create a relationship linking a finding to a MITRE ATT&CK technique
	rel := graphrag.NewRelationship(
		"finding-123",
		"T1190",
		"USES_TECHNIQUE",
	).WithProperty("confidence", 0.95)

	fmt.Println("From:", rel.FromID)
	fmt.Println("To:", rel.ToID)
	fmt.Println("Type:", rel.Type)
	fmt.Println("Confidence:", rel.Properties["confidence"])
	// Output:
	// From: finding-123
	// To: T1190
	// Type: USES_TECHNIQUE
	// Confidence: 0.95
}

// ExampleRelationship_WithBidirectional demonstrates creating bidirectional relationships.
func ExampleRelationship_WithBidirectional() {
	// Create a bidirectional similarity relationship
	rel := graphrag.NewRelationship(
		"finding-123",
		"finding-456",
		"SIMILAR_TO",
	).WithProperty("similarity", 0.87).
		WithBidirectional(true)

	fmt.Println("Bidirectional:", rel.Bidirectional)
	fmt.Println("Type:", rel.Type)
	// Output:
	// Bidirectional: true
	// Type: SIMILAR_TO
}

// ExampleNewBatch demonstrates batch operations for efficient bulk storage.
func ExampleNewBatch() {
	// Create nodes
	finding1 := *graphrag.NewGraphNode("finding").
		WithID("finding-1").
		WithContent("SQL injection in login")

	finding2 := *graphrag.NewGraphNode("finding").
		WithID("finding-2").
		WithContent("XSS in search")

	technique := *graphrag.NewGraphNode("technique").
		WithID("T1190").
		WithContent("Exploit Public-Facing Application")

	// Create relationships
	rel1 := *graphrag.NewRelationship("finding-1", "T1190", "USES_TECHNIQUE")
	rel2 := *graphrag.NewRelationship("finding-2", "T1190", "USES_TECHNIQUE")

	// Build batch
	batch := graphrag.NewBatch().
		AddNode(finding1).
		AddNode(finding2).
		AddNode(technique).
		AddRelationship(rel1).
		AddRelationship(rel2)

	fmt.Println("Nodes in batch:", len(batch.Nodes))
	fmt.Println("Relationships in batch:", len(batch.Relationships))
	// Output:
	// Nodes in batch: 3
	// Relationships in batch: 2
}

// Example_storingAttackData demonstrates storing attack findings and techniques.
func Example_storingAttackData() {
	// Create finding node
	finding := graphrag.NewGraphNode("finding").
		WithID("finding-001").
		WithContent("SQL injection in /api/login parameter 'username'").
		WithProperty("severity", "critical").
		WithProperty("cvss_score", 9.8).
		WithProperty("endpoint", "/api/login").
		WithProperty("parameter", "username")

	// Create technique node
	technique := graphrag.NewGraphNode("technique").
		WithID("T1190").
		WithContent("Exploit Public-Facing Application").
		WithProperty("tactic", "Initial Access")

	// Link finding to technique
	rel := graphrag.NewRelationship(
		finding.ID,
		"T1190",
		"USES_TECHNIQUE",
	).WithProperty("confidence", 0.95)

	// Validate all components
	if err := finding.Validate(); err != nil {
		fmt.Printf("failed to validate finding: %v\n", err)
		return
	}
	if err := technique.Validate(); err != nil {
		fmt.Printf("failed to validate technique: %v\n", err)
		return
	}
	if err := rel.Validate(); err != nil {
		fmt.Printf("failed to validate relationship: %v\n", err)
		return
	}

	fmt.Println("Finding severity:", finding.Properties["severity"])
	fmt.Println("Technique ID:", technique.ID)
	fmt.Println("Relationship confidence:", rel.Properties["confidence"])
	// Output:
	// Finding severity: critical
	// Technique ID: T1190
	// Relationship confidence: 0.95
}

// Example_buildingAttackChain demonstrates creating an attack chain from multiple findings.
func Example_buildingAttackChain() {
	// Create findings representing attack steps
	recon := graphrag.NewGraphNode("finding").
		WithID("step-1").
		WithContent("Port scan detected open SSH port 22")

	bruteforce := graphrag.NewGraphNode("finding").
		WithID("step-2").
		WithContent("SSH bruteforce successful with weak credentials")

	privesc := graphrag.NewGraphNode("finding").
		WithID("step-3").
		WithContent("Privilege escalation via sudo misconfiguration")

	// Link findings in sequence to form attack chain
	rel1 := graphrag.NewRelationship(recon.ID, bruteforce.ID, "LEADS_TO").
		WithProperty("sequence", 1).
		WithProperty("chain_id", "attack-chain-001")

	rel2 := graphrag.NewRelationship(bruteforce.ID, privesc.ID, "LEADS_TO").
		WithProperty("sequence", 2).
		WithProperty("chain_id", "attack-chain-001")

	fmt.Println("Step 1:", recon.ID, "->", rel1.ToID)
	fmt.Println("Step 2:", bruteforce.ID, "->", rel2.ToID)
	fmt.Println("Chain ID:", rel1.Properties["chain_id"])
	// Output:
	// Step 1: step-1 -> step-2
	// Step 2: step-2 -> step-3
	// Chain ID: attack-chain-001
}
