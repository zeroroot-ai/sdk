// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Zero Root AI

package graphrag

import (
	"errors"
	"time"
)

// GraphNode represents a node in the GraphRAG knowledge graph.
// It stores arbitrary data with metadata for mission context and embedding generation.
type GraphNode struct {
	// ID is the unique node identifier. Auto-generated if empty.
	ID string `json:"id"`

	// Type is the custom node type (e.g., "AttackAttempt", "Conversation").
	// Required field.
	Type string `json:"type"`

	// Properties contains arbitrary key-value properties for the node.
	Properties map[string]any `json:"properties,omitempty"`

	// Content is the text content used for embedding generation (optional).
	Content string `json:"content,omitempty"`

	// MissionID is auto-populated by the harness.
	MissionID string `json:"mission_id,omitempty"`

	// AgentName is auto-populated by the harness.
	AgentName string `json:"agent_name,omitempty"`

	// CreatedAt is the timestamp when the node was created.
	CreatedAt time.Time `json:"created_at"`

	// UpdatedAt is the timestamp when the node was last updated.
	UpdatedAt time.Time `json:"updated_at"`
}

// NewGraphNode creates a new GraphNode with the specified type and sensible defaults.
// The timestamps are set to the current time, and Properties map is initialized.
func NewGraphNode(nodeType string) *GraphNode {
	now := time.Now()
	return &GraphNode{
		Type:       nodeType,
		Properties: make(map[string]any),
		CreatedAt:  now,
		UpdatedAt:  now,
	}
}

// WithID sets the node ID and returns the node for method chaining.
func (n *GraphNode) WithID(id string) *GraphNode {
	n.ID = id
	return n
}

// WithProperty sets a single property and returns the node for method chaining.
// If the Properties map is nil, it will be initialized.
func (n *GraphNode) WithProperty(key string, value any) *GraphNode {
	if n.Properties == nil {
		n.Properties = make(map[string]any)
	}
	n.Properties[key] = value
	return n
}

// WithProperties sets multiple properties and returns the node for method chaining.
// This replaces the entire Properties map.
func (n *GraphNode) WithProperties(props map[string]any) *GraphNode {
	n.Properties = props
	return n
}

// WithContent sets the content field and returns the node for method chaining.
func (n *GraphNode) WithContent(content string) *GraphNode {
	n.Content = content
	return n
}

// Validate checks that the node has all required fields set correctly.
// Returns an error if Type is empty.
func (n *GraphNode) Validate() error {
	if n.Type == "" {
		return errors.New("node type is required")
	}
	return nil
}
