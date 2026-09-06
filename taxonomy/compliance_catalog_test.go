// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Zero Root AI

package taxonomy

import (
	"testing"
)

func TestLoadCatalog_SeedFile(t *testing.T) {
	c, err := LoadCatalog("compliance_rules.yaml")
	if err != nil {
		t.Fatalf("LoadCatalog: %v", err)
	}
	if c.Version == "" {
		t.Errorf("version missing")
	}
	if len(c.Rules) == 0 {
		t.Errorf("no rules loaded")
	}
	if _, ok := c.Frameworks["SOC2"]; !ok {
		t.Errorf("SOC2 framework missing")
	}
	if _, ok := c.Frameworks["NIST_AI_RMF"]; !ok {
		t.Errorf("NIST_AI_RMF framework missing")
	}
	if _, ok := c.Frameworks["PLATFORM"]; !ok {
		t.Errorf("PLATFORM framework missing")
	}
}

func TestLoadCatalog_DuplicateID(t *testing.T) {
	yml := []byte(`
version: "1.0"
frameworks:
  F1: {name: "F"}
rules:
  - id: DUP.1
    framework: F1
    control_id: A
    matcher:
      equals:
        action: tool_call
  - id: DUP.1
    framework: F1
    control_id: B
    matcher:
      equals:
        action: llm_call
`)
	_, err := LoadCatalogFromBytes(yml)
	if err == nil {
		t.Errorf("expected duplicate-id error")
	}
}

func TestLoadCatalog_MissingFramework(t *testing.T) {
	yml := []byte(`
version: "1.0"
frameworks:
  F1: {name: "F"}
rules:
  - id: R.1
    framework: F2
    control_id: A
    matcher:
      equals:
        action: tool_call
`)
	_, err := LoadCatalogFromBytes(yml)
	if err == nil {
		t.Errorf("expected unknown-framework error")
	}
}

func TestLoadCatalog_EmptyMatcher(t *testing.T) {
	yml := []byte(`
version: "1.0"
frameworks:
  F1: {name: "F"}
rules:
  - id: R.1
    framework: F1
    control_id: A
    matcher: {}
`)
	_, err := LoadCatalogFromBytes(yml)
	if err == nil {
		t.Errorf("expected empty-matcher error")
	}
}

func TestMatcher_IsLeaf(t *testing.T) {
	leaf := Matcher{Equals: map[string]string{"action": "tool_call"}}
	if !leaf.IsLeaf() {
		t.Errorf("leaf matcher should report IsLeaf=true")
	}
	nested := Matcher{AnyOf: []Matcher{leaf}}
	if nested.IsLeaf() {
		t.Errorf("nested matcher should report IsLeaf=false")
	}
}

func TestRulesByFramework(t *testing.T) {
	c := &Catalog{
		Version:    "1.0",
		Frameworks: map[string]Framework{"A": {Name: "a"}, "B": {Name: "b"}},
		Rules: []Rule{
			{ID: "A.1", Framework: "A", ControlID: "a1", Matcher: Matcher{Equals: map[string]string{"x": "y"}}},
			{ID: "A.2", Framework: "A", ControlID: "a2", Matcher: Matcher{Equals: map[string]string{"x": "y"}}},
			{ID: "B.1", Framework: "B", ControlID: "b1", Matcher: Matcher{Equals: map[string]string{"x": "y"}}},
		},
	}
	grouped := c.RulesByFramework()
	if len(grouped["A"]) != 2 {
		t.Errorf("A = %d; want 2", len(grouped["A"]))
	}
	if len(grouped["B"]) != 1 {
		t.Errorf("B = %d; want 1", len(grouped["B"]))
	}
}
