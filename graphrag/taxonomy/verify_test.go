// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Zero Root AI

package taxonomy

import "testing"

func TestParentRelationshipsExist(t *testing.T) {
	tests := []struct {
		childType        string
		expectedParent   string
		expectedRefField string
		expectedRel      string
	}{
		{"port", "host", "host_id", "HAS_PORT"},
		{"service", "port", "port_id", "RUNS_SERVICE"},
		{"endpoint", "service", "service_id", "HAS_ENDPOINT"},
		{"subdomain", "domain", "domain_id", "HAS_SUBDOMAIN"},
		{"evidence", "finding", "finding_id", "HAS_EVIDENCE"},
		{"mission_run", "mission", "mission_id", "BELONGS_TO"},
		{"tool_execution", "agent_run", "agent_run_id", "USED_TOOL"},
		// compliance_signal has a parent relationship to agent_run (EMITTED_SIGNAL).
		// agent_run and llm_call are root nodes in v4.0 (parent: null).
		{"compliance_signal", "agent_run", "agent_run_id", "EMITTED_SIGNAL"},
	}

	for _, tt := range tests {
		t.Run(tt.childType, func(t *testing.T) {
			rel := GetParentRelationship(tt.childType)
			if rel == nil {
				t.Fatalf("GetParentRelationship(%q) returned nil", tt.childType)
			}
			if rel.ParentType != tt.expectedParent {
				t.Errorf("ParentType = %q, want %q", rel.ParentType, tt.expectedParent)
			}
			if rel.RefField != tt.expectedRefField {
				t.Errorf("RefField = %q, want %q", rel.RefField, tt.expectedRefField)
			}
			if rel.Relationship != tt.expectedRel {
				t.Errorf("Relationship = %q, want %q", rel.Relationship, tt.expectedRel)
			}
			if rel.ParentField != "id" {
				t.Errorf("ParentField = %q, want \"id\"", rel.ParentField)
			}
		})
	}
}

func TestRootNodeTypes(t *testing.T) {
	// In v4.0: agent_run and llm_call are root nodes (mission_run_id / agent_run_id
	// are stored as plain properties, not parent relationships).
	rootTypes := []string{
		"host", "domain", "technology", "certificate", "finding",
		"mission", "technique", "agent_run", "llm_call",
	}
	for _, nodeType := range rootTypes {
		t.Run(nodeType, func(t *testing.T) {
			if !IsRootNodeType(nodeType) {
				t.Errorf("IsRootNodeType(%q) = false, want true", nodeType)
			}
			if rel := GetParentRelationship(nodeType); rel != nil {
				t.Errorf("GetParentRelationship(%q) = %+v, want nil", nodeType, rel)
			}
		})
	}
}

func TestNonRootNodeTypes(t *testing.T) {
	// In v4.0: agent_run and llm_call are root; compliance_signal has agent_run as parent.
	nonRootTypes := []string{
		"port", "service", "endpoint", "subdomain", "evidence",
		"mission_run", "tool_execution", "compliance_signal",
	}
	for _, nodeType := range nonRootTypes {
		t.Run(nodeType, func(t *testing.T) {
			if IsRootNodeType(nodeType) {
				t.Errorf("IsRootNodeType(%q) = true, want false", nodeType)
			}
			if rel := GetParentRelationship(nodeType); rel == nil {
				t.Errorf("GetParentRelationship(%q) = nil, want non-nil", nodeType)
			}
		})
	}
}
