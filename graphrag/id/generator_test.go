// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Zero Root AI

package id

import (
	"strings"
	"testing"

	"github.com/zeroroot-ai/sdk/graphrag"
)

func TestGenerateDeterminism(t *testing.T) {
	registry := graphrag.NewDefaultNodeTypeRegistry()
	gen := NewGenerator(registry)

	tests := []struct {
		name       string
		nodeType   string
		properties map[string]any
	}{
		{
			name:     "host with ip",
			nodeType: "host",
			properties: map[string]any{
				"ip": "10.0.0.1",
			},
		},
		{
			name:     "port with host_id, number, protocol",
			nodeType: "port",
			properties: map[string]any{
				"host_id":  "host:abc123",
				"number":   443,
				"protocol": "tcp",
			},
		},
		{
			name:     "domain with name",
			nodeType: "domain",
			properties: map[string]any{
				"name": "example.com",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Generate ID multiple times
			id1, err1 := gen.Generate(tt.nodeType, tt.properties)
			id2, err2 := gen.Generate(tt.nodeType, tt.properties)
			id3, err3 := gen.Generate(tt.nodeType, tt.properties)

			// All should succeed
			if err1 != nil {
				t.Fatalf("first generation failed: %v", err1)
			}
			if err2 != nil {
				t.Fatalf("second generation failed: %v", err2)
			}
			if err3 != nil {
				t.Fatalf("third generation failed: %v", err3)
			}

			// All IDs should be identical
			if id1 != id2 {
				t.Errorf("id1 != id2: %q != %q", id1, id2)
			}
			if id1 != id3 {
				t.Errorf("id1 != id3: %q != %q", id1, id3)
			}
			if id2 != id3 {
				t.Errorf("id2 != id3: %q != %q", id2, id3)
			}

			// ID should start with node type
			expectedPrefix := tt.nodeType + ":"
			if !strings.HasPrefix(id1, expectedPrefix) {
				t.Errorf("ID does not start with %q: %q", expectedPrefix, id1)
			}
		})
	}
}

func TestGenerateDifferentInputs(t *testing.T) {
	registry := graphrag.NewDefaultNodeTypeRegistry()
	gen := NewGenerator(registry)

	tests := []struct {
		name        string
		nodeType1   string
		properties1 map[string]any
		nodeType2   string
		properties2 map[string]any
	}{
		{
			name:      "different host IPs",
			nodeType1: "host",
			properties1: map[string]any{
				"ip": "10.0.0.1",
			},
			nodeType2: "host",
			properties2: map[string]any{
				"ip": "10.0.0.2",
			},
		},
		{
			name:      "different port numbers",
			nodeType1: "port",
			properties1: map[string]any{
				"host_id":  "host:abc123",
				"number":   80,
				"protocol": "tcp",
			},
			nodeType2: "port",
			properties2: map[string]any{
				"host_id":  "host:abc123",
				"number":   443,
				"protocol": "tcp",
			},
		},
		{
			name:      "different node types",
			nodeType1: "domain",
			properties1: map[string]any{
				"name": "example.com",
			},
			nodeType2: "subdomain",
			properties2: map[string]any{
				"parent_domain": "example.com",
				"name":          "www",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			id1, err1 := gen.Generate(tt.nodeType1, tt.properties1)
			id2, err2 := gen.Generate(tt.nodeType2, tt.properties2)

			if err1 != nil {
				t.Fatalf("first generation failed: %v", err1)
			}
			if err2 != nil {
				t.Fatalf("second generation failed: %v", err2)
			}

			// IDs should be different
			if id1 == id2 {
				t.Errorf("IDs should be different but both are: %q", id1)
			}
		})
	}
}

func TestGenerateMissingProperties(t *testing.T) {
	registry := graphrag.NewDefaultNodeTypeRegistry()
	gen := NewGenerator(registry)

	tests := []struct {
		name       string
		nodeType   string
		properties map[string]any
		wantErr    string
	}{
		{
			name:       "host missing ip",
			nodeType:   "host",
			properties: map[string]any{},
			wantErr:    "ip",
		},
		{
			name:     "port missing protocol",
			nodeType: "port",
			properties: map[string]any{
				"host_id": "host:abc123",
				"number":  80,
			},
			wantErr: "protocol",
		},
		{
			name:     "service missing name",
			nodeType: "service",
			properties: map[string]any{
				"port_id": "port:xyz789",
			},
			wantErr: "name",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := gen.Generate(tt.nodeType, tt.properties)

			if err == nil {
				t.Fatal("expected error but got none")
			}

			if !strings.Contains(err.Error(), tt.wantErr) {
				t.Errorf("error should mention missing property %q, got: %v", tt.wantErr, err)
			}
		})
	}
}

func TestGenerateUnknownNodeType(t *testing.T) {
	registry := graphrag.NewDefaultNodeTypeRegistry()
	gen := NewGenerator(registry)

	_, err := gen.Generate("unknown_type", map[string]any{"foo": "bar"})

	if err == nil {
		t.Fatal("expected error for unknown node type but got none")
	}

	if !strings.Contains(err.Error(), "unknown_type") {
		t.Errorf("error should mention node type, got: %v", err)
	}
}

func TestPropertyOrderIndependence(t *testing.T) {
	registry := graphrag.NewDefaultNodeTypeRegistry()
	gen := NewGenerator(registry)

	// Port node has multiple properties: host_id, number, protocol
	// Test that order doesn't matter
	props1 := map[string]any{
		"host_id":  "host:abc123",
		"number":   80,
		"protocol": "tcp",
	}

	props2 := map[string]any{
		"protocol": "tcp",
		"host_id":  "host:abc123",
		"number":   80,
	}

	props3 := map[string]any{
		"number":   80,
		"protocol": "tcp",
		"host_id":  "host:abc123",
	}

	id1, err1 := gen.Generate("port", props1)
	id2, err2 := gen.Generate("port", props2)
	id3, err3 := gen.Generate("port", props3)

	if err1 != nil || err2 != nil || err3 != nil {
		t.Fatalf("generation failed: %v, %v, %v", err1, err2, err3)
	}

	// All IDs should be identical regardless of property order
	if id1 != id2 {
		t.Errorf("id1 != id2: %q != %q", id1, id2)
	}
	if id1 != id3 {
		t.Errorf("id1 != id3: %q != %q", id1, id3)
	}
}

func TestValueNormalization(t *testing.T) {
	registry := graphrag.NewDefaultNodeTypeRegistry()
	gen := NewGenerator(registry)

	tests := []struct {
		name        string
		nodeType    string
		props1      map[string]any
		props2      map[string]any
		shouldMatch bool
		description string
	}{
		{
			name:     "string case normalization",
			nodeType: "host",
			props1: map[string]any{
				"ip": "10.0.0.1",
			},
			props2: map[string]any{
				"ip": "10.0.0.1",
			},
			shouldMatch: true,
			description: "lowercase strings should match",
		},
		{
			name:     "string whitespace normalization",
			nodeType: "domain",
			props1: map[string]any{
				"name": "example.com",
			},
			props2: map[string]any{
				"name": "  example.com  ",
			},
			shouldMatch: true,
			description: "trimmed strings should match",
		},
		{
			name:     "string case difference",
			nodeType: "domain",
			props1: map[string]any{
				"name": "Example.com",
			},
			props2: map[string]any{
				"name": "example.com",
			},
			shouldMatch: true,
			description: "case-insensitive comparison",
		},
		{
			name:     "integer normalization",
			nodeType: "port",
			props1: map[string]any{
				"host_id":  "host:abc",
				"number":   int(80),
				"protocol": "tcp",
			},
			props2: map[string]any{
				"host_id":  "host:abc",
				"number":   int64(80),
				"protocol": "tcp",
			},
			shouldMatch: true,
			description: "int and int64 with same value should match",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			id1, err1 := gen.Generate(tt.nodeType, tt.props1)
			id2, err2 := gen.Generate(tt.nodeType, tt.props2)

			if err1 != nil {
				t.Fatalf("first generation failed: %v", err1)
			}
			if err2 != nil {
				t.Fatalf("second generation failed: %v", err2)
			}

			if tt.shouldMatch {
				if id1 != id2 {
					t.Errorf("%s: IDs should match but differ: %q != %q", tt.description, id1, id2)
				}
			} else {
				if id1 == id2 {
					t.Errorf("%s: IDs should differ but match: %q", tt.description, id1)
				}
			}
		})
	}
}

func TestNormalizeValue(t *testing.T) {
	gen := &DeterministicGenerator{}

	tests := []struct {
		name     string
		input    any
		expected string
	}{
		{"string lowercase", "HELLO", "hello"},
		{"string trim", "  hello  ", "hello"},
		{"string trim and lowercase", "  HELLO  ", "hello"},
		{"int", 42, "42"},
		{"int64", int64(9223372036854775807), "9223372036854775807"},
		{"float64", 3.14159, "3.141590"},
		{"bool true", true, "true"},
		{"bool false", false, "false"},
		{"nil", nil, "null"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := gen.normalizeValue(tt.input)
			if err != nil {
				t.Fatalf("normalization failed: %v", err)
			}

			if result != tt.expected {
				t.Errorf("expected %q, got %q", tt.expected, result)
			}
		})
	}
}

func TestIDFormat(t *testing.T) {
	registry := graphrag.NewDefaultNodeTypeRegistry()
	gen := NewGenerator(registry)

	id, err := gen.Generate("host", map[string]any{"ip": "10.0.0.1"})
	if err != nil {
		t.Fatalf("generation failed: %v", err)
	}

	// ID should be in format: nodeType:base64url(hash[:12])
	parts := strings.Split(id, ":")
	if len(parts) != 2 {
		t.Fatalf("ID should have format nodeType:hash, got: %q", id)
	}

	if parts[0] != "host" {
		t.Errorf("expected node type 'host', got %q", parts[0])
	}

	// Base64url encoded 12 bytes should be 16 characters (without padding)
	if len(parts[1]) != 16 {
		t.Errorf("expected hash part to be 16 characters, got %d: %q", len(parts[1]), parts[1])
	}

	// Should not contain padding characters
	if strings.Contains(parts[1], "=") {
		t.Errorf("hash should not contain padding: %q", parts[1])
	}

	// Should only contain base64url characters
	validChars := "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789-_"
	for _, char := range parts[1] {
		if !strings.ContainsRune(validChars, char) {
			t.Errorf("invalid character in hash: %c", char)
		}
	}
}

func TestGenerateWithExtraProperties(t *testing.T) {
	registry := graphrag.NewDefaultNodeTypeRegistry()
	gen := NewGenerator(registry)

	// Properties with extra non-identifying fields
	props := map[string]any{
		"ip":          "10.0.0.1",
		"hostname":    "server1", // Not an identifying property for host
		"description": "Web server",
	}

	id, err := gen.Generate("host", props)
	if err != nil {
		t.Fatalf("generation should succeed with extra properties: %v", err)
	}

	// ID should only depend on identifying property (ip)
	propsMinimal := map[string]any{
		"ip": "10.0.0.1",
	}

	idMinimal, err := gen.Generate("host", propsMinimal)
	if err != nil {
		t.Fatalf("generation failed: %v", err)
	}

	// Both should produce the same ID since only "ip" is identifying
	if id != idMinimal {
		t.Errorf("IDs should match (extra properties ignored): %q != %q", id, idMinimal)
	}
}
