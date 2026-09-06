// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Zero Root AI

package taxonomy

import (
	"testing"
)

func TestParentRelationships_ContainsExpectedChildTypes(t *testing.T) {
	expectedChildTypes := []string{
		"mission_run",
		"tool_execution",
		"subdomain",
		"port",
		"service",
		"endpoint",
		"evidence",
		"compliance_signal",
	}

	for _, childType := range expectedChildTypes {
		t.Run(childType, func(t *testing.T) {
			if _, exists := ParentRelationships[childType]; !exists {
				t.Errorf("Expected child type %q not found in ParentRelationships map", childType)
			}
		})
	}
}

func TestParentRelationships_MapSize(t *testing.T) {
	// v4.0: agent_run and llm_call promoted to root; compliance_signal added as child of agent_run.
	// Net change: -2 + 1 = -1 from v3 count of 9.
	expectedSize := 8
	actualSize := len(ParentRelationships)

	if actualSize != expectedSize {
		t.Errorf("Expected ParentRelationships map to contain %d entries, got %d", expectedSize, actualSize)
	}
}

func TestParentRelationships_CorrectParentTypes(t *testing.T) {
	tests := []struct {
		childType  string
		parentType string
	}{
		{"mission_run", "mission"},
		{"tool_execution", "agent_run"},
		{"subdomain", "domain"},
		{"port", "host"},
		{"service", "port"},
		{"endpoint", "service"},
		{"evidence", "finding"},
		{"compliance_signal", "agent_run"},
	}

	for _, tt := range tests {
		t.Run(tt.childType, func(t *testing.T) {
			rel, exists := ParentRelationships[tt.childType]
			if !exists {
				t.Fatalf("Child type %q not found in ParentRelationships", tt.childType)
			}

			if rel.ParentType != tt.parentType {
				t.Errorf("Expected ParentType=%q for %q, got %q", tt.parentType, tt.childType, rel.ParentType)
			}
		})
	}
}

func TestParentRelationships_ChildTypeMatchesKey(t *testing.T) {
	for key, rel := range ParentRelationships {
		if rel.ChildType != key {
			t.Errorf("Key %q does not match ChildType %q", key, rel.ChildType)
		}
	}
}

func TestParentRelationships_NonEmptyRefField(t *testing.T) {
	for childType, rel := range ParentRelationships {
		if rel.RefField == "" {
			t.Errorf("Child type %q has empty RefField", childType)
		}
	}
}

func TestParentRelationships_ParentFieldIsID(t *testing.T) {
	for childType, rel := range ParentRelationships {
		if rel.ParentField != "id" {
			t.Errorf("Child type %q has ParentField=%q, expected \"id\"", childType, rel.ParentField)
		}
	}
}

func TestParentRelationships_NonEmptyRelationshipName(t *testing.T) {
	for childType, rel := range ParentRelationships {
		if rel.Relationship == "" {
			t.Errorf("Child type %q has empty Relationship name", childType)
		}
	}
}

func TestParentRelationships_RefFieldMatchesExpectedPattern(t *testing.T) {
	tests := []struct {
		childType string
		refField  string
	}{
		{"mission_run", "mission_id"},
		{"tool_execution", "agent_run_id"},
		{"subdomain", "domain_id"},
		{"port", "host_id"},
		{"service", "port_id"},
		{"endpoint", "service_id"},
		{"evidence", "finding_id"},
		{"compliance_signal", "agent_run_id"},
	}

	for _, tt := range tests {
		t.Run(tt.childType, func(t *testing.T) {
			rel, exists := ParentRelationships[tt.childType]
			if !exists {
				t.Fatalf("Child type %q not found in ParentRelationships", tt.childType)
			}

			if rel.RefField != tt.refField {
				t.Errorf("Expected RefField=%q for %q, got %q", tt.refField, tt.childType, rel.RefField)
			}
		})
	}
}

func TestParentRelationships_RelationshipNames(t *testing.T) {
	tests := []struct {
		childType    string
		relationship string
	}{
		{"mission_run", "BELONGS_TO"},
		{"tool_execution", "USED_TOOL"},
		{"subdomain", "HAS_SUBDOMAIN"},
		{"port", "HAS_PORT"},
		{"service", "RUNS_SERVICE"},
		{"endpoint", "HAS_ENDPOINT"},
		{"evidence", "HAS_EVIDENCE"},
		{"compliance_signal", "EMITTED_SIGNAL"},
	}

	for _, tt := range tests {
		t.Run(tt.childType, func(t *testing.T) {
			rel, exists := ParentRelationships[tt.childType]
			if !exists {
				t.Fatalf("Child type %q not found in ParentRelationships", tt.childType)
			}

			if rel.Relationship != tt.relationship {
				t.Errorf("Expected Relationship=%q for %q, got %q", tt.relationship, tt.childType, rel.Relationship)
			}
		})
	}
}

func TestParentRelationships_RequiredField(t *testing.T) {
	// Most parent relationships are required; compliance_signal is optional.
	tests := []struct {
		childType string
		required  bool
	}{
		{"mission_run", true},
		{"tool_execution", true},
		{"subdomain", true},
		{"port", true},
		{"service", true},
		{"endpoint", true},
		{"evidence", true},
		{"compliance_signal", false},
	}

	for _, tt := range tests {
		t.Run(tt.childType, func(t *testing.T) {
			rel, exists := ParentRelationships[tt.childType]
			if !exists {
				t.Fatalf("Child type %q not found in ParentRelationships", tt.childType)
			}
			if rel.Required != tt.required {
				t.Errorf("Expected Required=%v for %q, got %v", tt.required, tt.childType, rel.Required)
			}
		})
	}
}

func TestRootNodeTypes_ContainsExpectedTypes(t *testing.T) {
	// v4.0: agent_run and llm_call promoted to root (no parent relationship).
	expectedRootTypes := []string{
		"mission",
		"agent_run",
		"llm_call",
		"domain",
		"host",
		"technology",
		"certificate",
		"finding",
		"technique",
		// v4.1 (ecs-brain, sdk#340): scope/credential/account added as root asset types.
		"scope",
		"credential",
		"account",
	}

	// Convert slice to map for easy lookup
	rootTypeMap := make(map[string]bool)
	for _, rt := range RootNodeTypes {
		rootTypeMap[rt] = true
	}

	for _, expectedType := range expectedRootTypes {
		if !rootTypeMap[expectedType] {
			t.Errorf("Expected root type %q not found in RootNodeTypes", expectedType)
		}
	}
}

func TestRootNodeTypes_Size(t *testing.T) {
	// v4.0: agent_run and llm_call added as root types (were child types in v3).
	// v4.1 (ecs-brain, sdk#340): +scope, +credential, +account (root asset types).
	expectedSize := 12
	actualSize := len(RootNodeTypes)

	if actualSize != expectedSize {
		t.Errorf("Expected RootNodeTypes to contain %d entries, got %d", expectedSize, actualSize)
	}
}

func TestRootNodeTypes_DoesNotContainChildTypes(t *testing.T) {
	// Convert RootNodeTypes to map for easy lookup
	rootTypeMap := make(map[string]bool)
	for _, rt := range RootNodeTypes {
		rootTypeMap[rt] = true
	}

	// Check that no child type is in RootNodeTypes
	for childType := range ParentRelationships {
		if rootTypeMap[childType] {
			t.Errorf("Child type %q should not be in RootNodeTypes", childType)
		}
	}
}

func TestRootNodeTypes_NoDuplicates(t *testing.T) {
	seen := make(map[string]bool)
	for _, rt := range RootNodeTypes {
		if seen[rt] {
			t.Errorf("Duplicate root type found: %q", rt)
		}
		seen[rt] = true
	}
}

func TestIsRootNodeType_ReturnsTrueForRootTypes(t *testing.T) {
	// v4.0: agent_run and llm_call are root types.
	tests := []struct {
		nodeType string
		expected bool
	}{
		{"mission", true},
		{"agent_run", true},
		{"llm_call", true},
		{"domain", true},
		{"host", true},
		{"technology", true},
		{"certificate", true},
		{"finding", true},
		{"technique", true},
	}

	for _, tt := range tests {
		t.Run(tt.nodeType, func(t *testing.T) {
			result := IsRootNodeType(tt.nodeType)
			if result != tt.expected {
				t.Errorf("IsRootNodeType(%q) = %v, want %v", tt.nodeType, result, tt.expected)
			}
		})
	}
}

func TestIsRootNodeType_ReturnsFalseForChildTypes(t *testing.T) {
	// v4.0: agent_run and llm_call are NOT in this list — they are root types.
	tests := []struct {
		nodeType string
		expected bool
	}{
		{"mission_run", false},
		{"tool_execution", false},
		{"subdomain", false},
		{"port", false},
		{"service", false},
		{"endpoint", false},
		{"evidence", false},
		{"compliance_signal", false},
	}

	for _, tt := range tests {
		t.Run(tt.nodeType, func(t *testing.T) {
			result := IsRootNodeType(tt.nodeType)
			if result != tt.expected {
				t.Errorf("IsRootNodeType(%q) = %v, want %v", tt.nodeType, result, tt.expected)
			}
		})
	}
}

func TestIsRootNodeType_ReturnsFalseForUnknownTypes(t *testing.T) {
	tests := []string{
		"unknown_type",
		"nonexistent",
		"",
		"MISSION", // case sensitive
	}

	for _, nodeType := range tests {
		t.Run(nodeType, func(t *testing.T) {
			result := IsRootNodeType(nodeType)
			if result {
				t.Errorf("IsRootNodeType(%q) = true, want false for unknown type", nodeType)
			}
		})
	}
}

func TestGetParentRelationship_ReturnsCorrectRelationship(t *testing.T) {
	// v4.0: agent_run and llm_call are root nodes (wantNil=true).
	// compliance_signal added as child of agent_run.
	tests := []struct {
		childType    string
		wantNil      bool
		wantParent   string
		wantRefField string
	}{
		{"mission_run", false, "mission", "mission_id"},
		{"agent_run", true, "", ""},
		{"llm_call", true, "", ""},
		{"tool_execution", false, "agent_run", "agent_run_id"},
		{"subdomain", false, "domain", "domain_id"},
		{"port", false, "host", "host_id"},
		{"service", false, "port", "port_id"},
		{"endpoint", false, "service", "service_id"},
		{"evidence", false, "finding", "finding_id"},
		{"compliance_signal", false, "agent_run", "agent_run_id"},
	}

	for _, tt := range tests {
		t.Run(tt.childType, func(t *testing.T) {
			rel := GetParentRelationship(tt.childType)

			if tt.wantNil {
				if rel != nil {
					t.Errorf("GetParentRelationship(%q) = %v, want nil", tt.childType, rel)
				}
				return
			}

			if rel == nil {
				t.Fatalf("GetParentRelationship(%q) = nil, want non-nil", tt.childType)
			}

			if rel.ParentType != tt.wantParent {
				t.Errorf("GetParentRelationship(%q).ParentType = %q, want %q", tt.childType, rel.ParentType, tt.wantParent)
			}

			if rel.RefField != tt.wantRefField {
				t.Errorf("GetParentRelationship(%q).RefField = %q, want %q", tt.childType, rel.RefField, tt.wantRefField)
			}

			if rel.ChildType != tt.childType {
				t.Errorf("GetParentRelationship(%q).ChildType = %q, want %q", tt.childType, rel.ChildType, tt.childType)
			}
		})
	}
}

func TestGetParentRelationship_ReturnsNilForRootTypes(t *testing.T) {
	// v4.0: agent_run and llm_call are root types with no parent relationship.
	rootTypes := []string{
		"mission",
		"agent_run",
		"llm_call",
		"domain",
		"host",
		"technology",
		"certificate",
		"finding",
		"technique",
	}

	for _, rootType := range rootTypes {
		t.Run(rootType, func(t *testing.T) {
			rel := GetParentRelationship(rootType)
			if rel != nil {
				t.Errorf("GetParentRelationship(%q) = %v, want nil for root type", rootType, rel)
			}
		})
	}
}

func TestGetParentRelationship_ReturnsNilForUnknownTypes(t *testing.T) {
	tests := []string{
		"unknown_type",
		"nonexistent",
		"",
	}

	for _, childType := range tests {
		t.Run(childType, func(t *testing.T) {
			rel := GetParentRelationship(childType)
			if rel != nil {
				t.Errorf("GetParentRelationship(%q) = %v, want nil for unknown type", childType, rel)
			}
		})
	}
}

func TestGetParentRelationship_ReturnsCopy(t *testing.T) {
	// Verify that modifying the returned pointer doesn't affect the original
	rel1 := GetParentRelationship("port")
	if rel1 == nil {
		t.Fatal("GetParentRelationship(\"port\") returned nil")
	}

	originalParentType := rel1.ParentType
	rel1.ParentType = "modified"

	rel2 := GetParentRelationship("port")
	if rel2 == nil {
		t.Fatal("GetParentRelationship(\"port\") returned nil on second call")
	}

	if rel2.ParentType != originalParentType {
		t.Errorf("Modifying returned relationship affected original: got %q, want %q", rel2.ParentType, originalParentType)
	}
}

func TestParentRelationships_NoCircularReferences(t *testing.T) {
	// Build a graph of parent relationships
	graph := make(map[string]string) // childType -> parentType
	for childType, rel := range ParentRelationships {
		graph[childType] = rel.ParentType
	}

	// For each child type, walk up the parent chain and ensure we don't loop
	for childType := range ParentRelationships {
		visited := make(map[string]bool)
		current := childType

		for {
			if visited[current] {
				t.Errorf("Circular reference detected starting from %q: visited %q twice", childType, current)
				break
			}
			visited[current] = true

			parent, hasParent := graph[current]
			if !hasParent {
				// Reached a root type or unknown type, no circle
				break
			}
			current = parent

			// Safety: if we've visited more than 100 nodes, something is wrong
			if len(visited) > 100 {
				t.Errorf("Infinite loop detected starting from %q", childType)
				break
			}
		}
	}
}

func TestParentRelationships_AllParentTypesExist(t *testing.T) {
	// Build a set of all valid node types (roots + children)
	validTypes := make(map[string]bool)
	for _, rt := range RootNodeTypes {
		validTypes[rt] = true
	}
	for childType := range ParentRelationships {
		validTypes[childType] = true
	}

	// Verify each parent type is valid
	for childType, rel := range ParentRelationships {
		if !validTypes[rel.ParentType] {
			t.Errorf("Child type %q references unknown parent type %q", childType, rel.ParentType)
		}
	}
}

func TestParentRelationships_UniqueRelationshipNames(t *testing.T) {
	// Track relationship names and which child types use them
	relationshipNames := make(map[string][]string)

	for childType, rel := range ParentRelationships {
		relationshipNames[rel.Relationship] = append(relationshipNames[rel.Relationship], childType)
	}

	// Check for duplicates
	for relName, childTypes := range relationshipNames {
		if len(childTypes) > 1 {
			t.Errorf("Relationship name %q is used by multiple child types: %v", relName, childTypes)
		}
	}
}

func BenchmarkIsRootNodeType(b *testing.B) {
	testTypes := []string{"mission", "port", "unknown", "finding", "service"}

	b.ResetTimer()
	for i := range b.N {
		IsRootNodeType(testTypes[i%len(testTypes)])
	}
}

func BenchmarkGetParentRelationship(b *testing.B) {
	testTypes := []string{"port", "service", "endpoint", "evidence", "unknown"}

	b.ResetTimer()
	for i := range b.N {
		GetParentRelationship(testTypes[i%len(testTypes)])
	}
}
