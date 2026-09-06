// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Zero Root AI

package graphrag

import (
	"testing"
)

// ============================================================================
// Query Tests
// ============================================================================

// ============================================================================
// Batch Tests
// ============================================================================

func TestNewBatch(t *testing.T) {
	batch := NewBatch()

	if batch == nil {
		t.Fatal("expected NewBatch to return non-nil")
	}

	if batch.Nodes == nil {
		t.Error("expected Nodes slice to be initialized")
	}

	if batch.Relationships == nil {
		t.Error("expected Relationships slice to be initialized")
	}

	if len(batch.Nodes) != 0 {
		t.Errorf("expected Nodes length to be 0, got %d", len(batch.Nodes))
	}

	if len(batch.Relationships) != 0 {
		t.Errorf("expected Relationships length to be 0, got %d", len(batch.Relationships))
	}
}

func TestBatch_AddNode(t *testing.T) {
	batch := NewBatch()
	node := *NewGraphNode("TestNode").WithID("node-1")

	result := batch.AddNode(node)

	// Check method chaining returns the batch
	if result != batch {
		t.Error("expected AddNode to return the same batch instance for chaining")
	}

	if len(batch.Nodes) != 1 {
		t.Errorf("expected Nodes length to be 1, got %d", len(batch.Nodes))
	}

	if batch.Nodes[0].ID != "node-1" {
		t.Errorf("expected Nodes[0].ID to be 'node-1', got %q", batch.Nodes[0].ID)
	}

	if batch.Nodes[0].Type != "TestNode" {
		t.Errorf("expected Nodes[0].Type to be 'TestNode', got %q", batch.Nodes[0].Type)
	}
}

func TestBatch_AddRelationship(t *testing.T) {
	batch := NewBatch()
	rel := *NewRelationship("node1", "node2", "CONNECTS_TO")

	result := batch.AddRelationship(rel)

	// Check method chaining returns the batch
	if result != batch {
		t.Error("expected AddRelationship to return the same batch instance for chaining")
	}

	if len(batch.Relationships) != 1 {
		t.Errorf("expected Relationships length to be 1, got %d", len(batch.Relationships))
	}

	if batch.Relationships[0].FromID != "node1" {
		t.Errorf("expected Relationships[0].FromID to be 'node1', got %q", batch.Relationships[0].FromID)
	}

	if batch.Relationships[0].ToID != "node2" {
		t.Errorf("expected Relationships[0].ToID to be 'node2', got %q", batch.Relationships[0].ToID)
	}

	if batch.Relationships[0].Type != "CONNECTS_TO" {
		t.Errorf("expected Relationships[0].Type to be 'CONNECTS_TO', got %q", batch.Relationships[0].Type)
	}
}

func TestBatch_Chaining(t *testing.T) {
	// Test that AddNode and AddRelationship can be chained together
	node1 := *NewGraphNode("Node1").WithID("n1")
	node2 := *NewGraphNode("Node2").WithID("n2")
	rel := *NewRelationship("n1", "n2", "RELATED_TO")

	batch := NewBatch().
		AddNode(node1).
		AddNode(node2).
		AddRelationship(rel)

	if len(batch.Nodes) != 2 {
		t.Errorf("expected Nodes length to be 2, got %d", len(batch.Nodes))
	}

	if len(batch.Relationships) != 1 {
		t.Errorf("expected Relationships length to be 1, got %d", len(batch.Relationships))
	}

	// Verify nodes
	if batch.Nodes[0].ID != "n1" {
		t.Errorf("expected Nodes[0].ID to be 'n1', got %q", batch.Nodes[0].ID)
	}

	if batch.Nodes[1].ID != "n2" {
		t.Errorf("expected Nodes[1].ID to be 'n2', got %q", batch.Nodes[1].ID)
	}

	// Verify relationship
	if batch.Relationships[0].FromID != "n1" || batch.Relationships[0].ToID != "n2" {
		t.Errorf("expected relationship from 'n1' to 'n2', got from '%s' to '%s'",
			batch.Relationships[0].FromID, batch.Relationships[0].ToID)
	}
}

func TestBatch_MultipleAdditions(t *testing.T) {
	batch := NewBatch()

	// Add multiple nodes
	for i := range 5 {
		node := *NewGraphNode("Node").WithID("node-" + string(rune('0'+i)))
		batch.AddNode(node)
	}

	// Add multiple relationships
	for i := range 3 {
		rel := *NewRelationship("node-"+string(rune('0'+i)), "node-"+string(rune('1'+i)), "NEXT")
		batch.AddRelationship(rel)
	}

	if len(batch.Nodes) != 5 {
		t.Errorf("expected 5 nodes, got %d", len(batch.Nodes))
	}

	if len(batch.Relationships) != 3 {
		t.Errorf("expected 3 relationships, got %d", len(batch.Relationships))
	}
}

// ============================================================================
// TraversalOptions Tests
// ============================================================================

// ============================================================================
// Result Tests
// ============================================================================

// ============================================================================
// TraversalResult Tests
// ============================================================================
