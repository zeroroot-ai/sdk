// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Zero Root AI

package graphrag

// Batch represents a collection of nodes and relationships to be created or updated together.
// It supports builder pattern methods for easy construction.
type Batch struct {
	// Nodes contains all nodes to be processed in this batch
	Nodes []GraphNode `json:"nodes"`

	// Relationships contains all relationships to be processed in this batch
	Relationships []Relationship `json:"relationships"`
}

// NewBatch creates a new empty Batch with initialized slices.
func NewBatch() *Batch {
	return &Batch{
		Nodes:         make([]GraphNode, 0),
		Relationships: make([]Relationship, 0),
	}
}

// AddNode adds a node to the batch and returns the batch for method chaining.
func (b *Batch) AddNode(n GraphNode) *Batch {
	b.Nodes = append(b.Nodes, n)
	return b
}

// AddRelationship adds a relationship to the batch and returns the batch for method chaining.
func (b *Batch) AddRelationship(r Relationship) *Batch {
	b.Relationships = append(b.Relationships, r)
	return b
}
