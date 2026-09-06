// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Zero Root AI

package graphrag

import "errors"

// Relationship represents a connection between two nodes in the graph.
// It supports both unidirectional and bidirectional relationships with optional properties.
type Relationship struct {
	// FromID is the source node ID
	FromID string `json:"from_id"`

	// ToID is the target node ID
	ToID string `json:"to_id"`

	// Type describes the relationship type (e.g., "ELICITED", "PART_OF", "SIMILAR_TO")
	Type string `json:"type"`

	// Properties contains optional relationship metadata
	Properties map[string]any `json:"properties,omitempty"`

	// Bidirectional indicates if the relationship should be created in both directions
	Bidirectional bool `json:"bidirectional"`
}

// NewRelationship creates a new Relationship with the specified source, target, and type.
// The relationship is unidirectional by default with no properties.
func NewRelationship(fromID, toID, relType string) *Relationship {
	return &Relationship{
		FromID:     fromID,
		ToID:       toID,
		Type:       relType,
		Properties: make(map[string]any),
	}
}

// WithProperty adds a single property to the relationship and returns the relationship for chaining.
func (r *Relationship) WithProperty(key string, value any) *Relationship {
	if r.Properties == nil {
		r.Properties = make(map[string]any)
	}
	r.Properties[key] = value
	return r
}

// WithProperties sets multiple properties on the relationship and returns the relationship for chaining.
// This replaces any existing properties.
func (r *Relationship) WithProperties(props map[string]any) *Relationship {
	r.Properties = props
	return r
}

// WithBidirectional sets whether the relationship should be created in both directions
// and returns the relationship for chaining.
func (r *Relationship) WithBidirectional(bi bool) *Relationship {
	r.Bidirectional = bi
	return r
}

// Validate checks that the relationship has all required fields populated.
// Returns an error if FromID, ToID, or Type are empty.
func (r *Relationship) Validate() error {
	if r.FromID == "" {
		return errors.New("relationship FromID cannot be empty")
	}
	if r.ToID == "" {
		return errors.New("relationship ToID cannot be empty")
	}
	if r.Type == "" {
		return errors.New("relationship Type cannot be empty")
	}
	return nil
}
