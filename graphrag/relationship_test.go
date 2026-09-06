// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Zero Root AI

package graphrag

import (
	"testing"
)

func TestNewRelationship(t *testing.T) {
	rel := NewRelationship("node1", "node2", "ELICITED")

	if rel.FromID != "node1" {
		t.Errorf("expected FromID to be 'node1', got '%s'", rel.FromID)
	}
	if rel.ToID != "node2" {
		t.Errorf("expected ToID to be 'node2', got '%s'", rel.ToID)
	}
	if rel.Type != "ELICITED" {
		t.Errorf("expected Type to be 'ELICITED', got '%s'", rel.Type)
	}
	if rel.Bidirectional {
		t.Error("expected Bidirectional to be false by default")
	}
	if rel.Properties == nil {
		t.Error("expected Properties map to be initialized")
	}
}

func TestRelationshipBuilderChaining(t *testing.T) {
	rel := NewRelationship("node1", "node2", "SIMILAR_TO").
		WithProperty("score", 0.95).
		WithProperty("method", "embedding").
		WithBidirectional(true)

	if rel.Properties["score"] != 0.95 {
		t.Errorf("expected score property to be 0.95, got %v", rel.Properties["score"])
	}
	if rel.Properties["method"] != "embedding" {
		t.Errorf("expected method property to be 'embedding', got %v", rel.Properties["method"])
	}
	if !rel.Bidirectional {
		t.Error("expected Bidirectional to be true")
	}
}

func TestRelationshipWithProperties(t *testing.T) {
	props := map[string]any{
		"weight":    0.8,
		"timestamp": "2025-12-29",
	}

	rel := NewRelationship("node1", "node2", "PART_OF").
		WithProperties(props)

	if len(rel.Properties) != 2 {
		t.Errorf("expected 2 properties, got %d", len(rel.Properties))
	}
	if rel.Properties["weight"] != 0.8 {
		t.Errorf("expected weight to be 0.8, got %v", rel.Properties["weight"])
	}
}

func TestRelationshipValidate(t *testing.T) {
	tests := []struct {
		name         string
		relationship *Relationship
		expectError  bool
		errorMsg     string
	}{
		{
			name:         "valid relationship",
			relationship: NewRelationship("node1", "node2", "ELICITED"),
			expectError:  false,
		},
		{
			name:         "empty FromID",
			relationship: &Relationship{FromID: "", ToID: "node2", Type: "ELICITED"},
			expectError:  true,
			errorMsg:     "FromID cannot be empty",
		},
		{
			name:         "empty ToID",
			relationship: &Relationship{FromID: "node1", ToID: "", Type: "ELICITED"},
			expectError:  true,
			errorMsg:     "ToID cannot be empty",
		},
		{
			name:         "empty Type",
			relationship: &Relationship{FromID: "node1", ToID: "node2", Type: ""},
			expectError:  true,
			errorMsg:     "Type cannot be empty",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.relationship.Validate()
			if tt.expectError {
				if err == nil {
					t.Errorf("expected error containing '%s', got nil", tt.errorMsg)
				}
			} else {
				if err != nil {
					t.Errorf("expected no error, got: %v", err)
				}
			}
		})
	}
}
