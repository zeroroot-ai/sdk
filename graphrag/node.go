// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Zero Root AI

package graphrag

import (
	"time"
)

// GraphNode represents a node in the GraphRAG knowledge graph.
// It stores arbitrary data with metadata for mission context and embedding generation.
type GraphNode struct {
	// ID is the unique node identifier. Auto-generated if empty.
	ID string `json:"id"`

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
