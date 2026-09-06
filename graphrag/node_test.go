// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Zero Root AI

package graphrag

import (
	"testing"
	"time"
)

func TestNewGraphNode(t *testing.T) {
	node := NewGraphNode("TestType")

	if node.Type != "TestType" {
		t.Errorf("expected Type to be 'TestType', got %q", node.Type)
	}

	if node.Properties == nil {
		t.Error("expected Properties to be initialized")
	}

	if node.CreatedAt.IsZero() {
		t.Error("expected CreatedAt to be set")
	}

	if node.UpdatedAt.IsZero() {
		t.Error("expected UpdatedAt to be set")
	}
}

func TestGraphNode_BuilderMethods(t *testing.T) {
	// Test method chaining
	node := NewGraphNode("TestType").
		WithID("test-id-123").
		WithProperty("key1", "value1").
		WithProperty("key2", 42).
		WithContent("test content")

	if node.ID != "test-id-123" {
		t.Errorf("expected ID to be 'test-id-123', got %q", node.ID)
	}

	if node.Properties["key1"] != "value1" {
		t.Errorf("expected Properties['key1'] to be 'value1', got %v", node.Properties["key1"])
	}

	if node.Properties["key2"] != 42 {
		t.Errorf("expected Properties['key2'] to be 42, got %v", node.Properties["key2"])
	}

	if node.Content != "test content" {
		t.Errorf("expected Content to be 'test content', got %q", node.Content)
	}
}

func TestGraphNode_WithProperties(t *testing.T) {
	props := map[string]any{
		"name":  "test",
		"count": 10,
	}

	node := NewGraphNode("TestType").WithProperties(props)

	if node.Properties["name"] != "test" {
		t.Errorf("expected Properties['name'] to be 'test', got %v", node.Properties["name"])
	}

	if node.Properties["count"] != 10 {
		t.Errorf("expected Properties['count'] to be 10, got %v", node.Properties["count"])
	}
}

func TestGraphNode_Validate(t *testing.T) {
	tests := []struct {
		name    string
		node    *GraphNode
		wantErr bool
	}{
		{
			name:    "valid node",
			node:    NewGraphNode("ValidType"),
			wantErr: false,
		},
		{
			name: "missing type",
			node: &GraphNode{
				ID:         "test-id",
				Properties: make(map[string]any),
				CreatedAt:  time.Now(),
				UpdatedAt:  time.Now(),
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.node.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestGraphNode_WithProperty_NilMap(t *testing.T) {
	// Test that WithProperty initializes map if nil
	node := &GraphNode{Type: "TestType"}
	node.WithProperty("key", "value")

	if node.Properties == nil {
		t.Error("expected Properties to be initialized")
	}

	if node.Properties["key"] != "value" {
		t.Errorf("expected Properties['key'] to be 'value', got %v", node.Properties["key"])
	}
}
