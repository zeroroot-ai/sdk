// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Zero Root AI

package graphrag

import (
	"testing"
)

func TestSimpleTaxonomyAndRegistry(t *testing.T) {
	core := NewSimpleTaxonomy()
	reg := NewTaxonomyRegistry(core)

	for name, tx := range map[string]TaxonomyIntrospector{"simple": core, "registry": reg} {
		t.Run(name, func(t *testing.T) {
			if tx.Version() == "" {
				t.Fatal("Version is empty")
			}
			if len(tx.NodeTypes()) != len(AllNodeTypes) {
				t.Fatalf("NodeTypes = %d, want %d", len(tx.NodeTypes()), len(AllNodeTypes))
			}
			host := tx.NodeTypeInfo(NodeTypeHost)
			if host == nil || host.Type != NodeTypeHost || host.Category != "asset" {
				t.Fatalf("NodeTypeInfo(host) = %+v", host)
			}
			if tx.NodeTypeInfo("no_such_type") != nil {
				t.Fatal("an unknown node type must have no info")
			}
			if len(tx.RelationshipTypes()) != len(AllRelationshipTypes) {
				t.Fatalf("RelationshipTypes = %d, want %d", len(tx.RelationshipTypes()), len(AllRelationshipTypes))
			}
			rel := AllRelationshipTypes[0]
			if info := tx.RelationshipTypeInfo(rel); info == nil || info.Type != rel {
				t.Fatalf("RelationshipTypeInfo(%s) = %+v", rel, info)
			}
			if tx.RelationshipTypeInfo("NO_SUCH_REL") != nil {
				t.Fatal("an unknown relationship must have no info")
			}
			if len(tx.TechniqueIDs("")) != 0 || len(tx.TechniqueIDs("mitre")) != 0 {
				t.Fatal("the generated taxonomy declares no techniques")
			}
			if tx.TechniqueInfo("T1000") != nil {
				t.Fatal("an unknown technique must have no info")
			}
		})
	}
}

func TestGeneratedHelpers(t *testing.T) {
	if !IsCoreType(NodeTypeHost) {
		t.Fatal("host is a core type")
	}
	if IsCoreType("custom_thing") {
		t.Fatal("custom_thing is not a core type")
	}
	req, ok := GetParentRequirement(NodeTypePort)
	if !ok || req.ParentType != NodeTypeHost || !req.Required {
		t.Fatalf("GetParentRequirement(port) = %+v, %v", req, ok)
	}
	if _, ok := GetParentRequirement(NodeTypeMission); ok {
		t.Fatal("mission has no parent")
	}
}
