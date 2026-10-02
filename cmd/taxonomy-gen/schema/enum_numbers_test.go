// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Zero Root AI

package schema

import (
	"strings"
	"testing"
)

// core returns a minimal valid core taxonomy to mutate per case.
func core(nodes []NodeType, rels []RelationshipType) *Taxonomy {
	return &Taxonomy{
		Version:           "1.0.0",
		Kind:              "core",
		NodeTypes:         nodes,
		RelationshipTypes: rels,
	}
}

func node(name string, number int) NodeType {
	return NodeType{Name: name, Number: number, Category: "asset"}
}

func rel(name string, number int) RelationshipType {
	return RelationshipType{
		Name: name, Number: number,
		FromTypes: []string{"host"}, ToTypes: []string{"*"},
		Cardinality: "one_to_many",
	}
}

// TestNumbersAreRequired is the rule that takes position out of the wire
// contract. Without it the generator has to pick a number, which is how
// position became load-bearing.
func TestNumbersAreRequired(t *testing.T) {
	tax := core([]NodeType{node("host", 1), node("port", 0)}, nil)

	err := tax.Validate()
	if err == nil {
		t.Fatal("a node type with no number was accepted")
	}
	msg := err.Error()
	if !strings.Contains(msg, "port") {
		t.Errorf("the error does not name the offending type: %v", err)
	}
	// It must say which number to use, or the author has to count the file by
	// hand, which is the habit this change removes.
	if !strings.Contains(msg, "lowest free number is 2") {
		t.Errorf("the error does not name the lowest free number: %v", err)
	}
}

// TestEveryMissingNumberIsNamed: reporting one at a time makes a 20-type file
// twenty runs.
func TestEveryMissingNumberIsNamed(t *testing.T) {
	tax := core([]NodeType{node("host", 1), node("port", 0), node("service", 0)}, nil)

	err := tax.Validate()
	if err == nil {
		t.Fatal("node types with no number were accepted")
	}
	for _, want := range []string{"port", "service", "2 entries declare"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("the error does not mention %q: %v", want, err)
		}
	}
}

// TestDuplicateNumbersAreRefused: two names sharing one wire value is the
// corruption that reusing a retired number would cause.
func TestDuplicateNumbersAreRefused(t *testing.T) {
	tax := core([]NodeType{node("host", 1), node("port", 1)}, nil)

	err := tax.Validate()
	if err == nil {
		t.Fatal("two node types with the same number were accepted")
	}
	msg := err.Error()
	for _, want := range []string{"host", "port", "share one wire value"} {
		if !strings.Contains(msg, want) {
			t.Errorf("the error does not mention %q: %v", want, err)
		}
	}
	// The message must point at the fix, because the likely cause is somebody
	// reusing a retired number on purpose.
	if !strings.Contains(msg, "retired entry keeps its number") {
		t.Errorf("the error does not explain that a retired number is not free: %v", err)
	}
}

// TestZeroAndNegativeNumbers: 0 is the UNSPECIFIED member. A node claiming 0
// would collide with it, and a negative number is not a valid enum value.
func TestZeroAndNegativeNumbers(t *testing.T) {
	// 0 is indistinguishable from unset in YAML, so it is reported as missing.
	if err := core([]NodeType{node("host", 0)}, nil).Validate(); err == nil {
		t.Error("number 0 was accepted")
	}
	err := core([]NodeType{node("host", -3)}, nil).Validate()
	if err == nil {
		t.Fatal("a negative number was accepted")
	}
	if !strings.Contains(err.Error(), "0 is the UNSPECIFIED member") {
		t.Errorf("the error does not say why: %v", err)
	}
}

// TestRelationshipNumbersAreCheckedToo: CoreRelationType was numbered by
// position as well, so leaving it unchecked would leave half the hazard.
func TestRelationshipNumbersAreCheckedToo(t *testing.T) {
	tax := core([]NodeType{node("host", 1)}, []RelationshipType{rel("HAS_PORT", 1), rel("RESOLVES_TO", 0)})

	err := tax.Validate()
	if err == nil {
		t.Fatal("a relationship type with no number was accepted")
	}
	if !strings.Contains(err.Error(), "RESOLVES_TO") {
		t.Errorf("the error does not name the relationship: %v", err)
	}

	tax = core([]NodeType{node("host", 1)}, []RelationshipType{rel("HAS_PORT", 2), rel("RESOLVES_TO", 2)})
	if err := tax.Validate(); err == nil {
		t.Fatal("two relationship types with the same number were accepted")
	}
}

// TestAnExtensionIsNotNumbered: an extension has its own enum space and the
// core enums are not extended, so requiring numbers there would break every
// extension for no gain.
func TestAnExtensionIsNotNumbered(t *testing.T) {
	tax := &Taxonomy{
		Version:   "1.0.0",
		Kind:      "extension",
		Extends:   "core",
		NodeTypes: []NodeType{node("widget", 0)},
	}
	if err := tax.Validate(); err != nil {
		t.Errorf("an extension with no numbers was refused: %v", err)
	}
}

// TestRetireHoldsTheNumberAndRemovesTheType is the behaviour the whole change
// exists for: retiring must not move any live number.
func TestRetireHoldsTheNumberAndRemovesTheType(t *testing.T) {
	tax := core([]NodeType{
		node("technique", 16),
		{Name: "compliance_signal", Number: 17, Category: "finding", Retired: true},
		node("scope", 18),
		node("credential", 19),
	}, []RelationshipType{
		rel("USED_TOOL", 1),
		{Name: "EMITTED_SIGNAL", Number: 3, Retired: true,
			FromTypes: []string{"host"}, ToTypes: []string{"*"}, Cardinality: "one_to_many"},
	})

	if err := tax.Validate(); err != nil {
		t.Fatalf("a taxonomy with retired entries was refused: %v", err)
	}

	live, retiredNodes, retiredRels := tax.Retire()

	// The live list holds no retired entry, so no generator can see one.
	for _, nt := range live.NodeTypes {
		if nt.Name == "compliance_signal" {
			t.Error("a retired node type reached the live taxonomy, so helpers would be generated for it")
		}
	}
	for _, rt := range live.RelationshipTypes {
		if rt.Name == "EMITTED_SIGNAL" {
			t.Error("a retired relationship type reached the live taxonomy")
		}
	}

	// And every live number is unchanged. This is the assertion the issue
	// asked for: scope, credential and account used to move when
	// compliance_signal was deleted.
	want := map[string]int{"technique": 16, "scope": 18, "credential": 19}
	for _, nt := range live.NodeTypes {
		if w, ok := want[nt.Name]; ok && nt.Number != w {
			t.Errorf("%s = %d after retiring, want %d", nt.Name, nt.Number, w)
		}
		delete(want, nt.Name)
	}
	if len(want) > 0 {
		t.Errorf("live taxonomy lost node types: %v", want)
	}

	if len(retiredNodes) != 1 || retiredNodes[0].Number != 17 {
		t.Errorf("retiredNodes = %+v, want one entry holding 17", retiredNodes)
	}
	if len(retiredRels) != 1 || retiredRels[0].Number != 3 {
		t.Errorf("retiredRels = %+v, want one entry holding 3", retiredRels)
	}
}

// TestRetireDoesNotMutateTheInput: main.go keeps the full taxonomy as well, and
// a shared backing array would corrupt it.
func TestRetireDoesNotMutateTheInput(t *testing.T) {
	tax := core([]NodeType{
		node("host", 1),
		{Name: "gone", Number: 2, Category: "asset", Retired: true},
		node("port", 3),
	}, nil)

	before := len(tax.NodeTypes)
	live, _, _ := tax.Retire()

	if len(tax.NodeTypes) != before {
		t.Errorf("Retire changed the input: %d node types, was %d", len(tax.NodeTypes), before)
	}
	if len(live.NodeTypes) != before-1 {
		t.Errorf("live has %d node types, want %d", len(live.NodeTypes), before-1)
	}
	// Writing through the live slice must not reach the input.
	live.NodeTypes[0].Name = "clobbered"
	if tax.NodeTypes[0].Name != "host" {
		t.Error("the live and input slices share backing storage")
	}
}

// TestRetiringEverythingIsRefused: a taxonomy with no live node type generates
// nothing, and the existing "at least one node type" rule must still catch it.
func TestRetiringEverythingIsRefused(t *testing.T) {
	tax := core([]NodeType{{Name: "gone", Number: 1, Category: "asset", Retired: true}}, nil)
	live, _, _ := tax.Retire()
	if err := live.Validate(); err == nil {
		t.Error("a taxonomy whose every node type is retired was accepted")
	}
}

// TestALiveRelationshipMayNotReferenceARetiredType is the case that slipped
// through the first version of this change. Retiring compliance_signal left
// TRIGGERED live with from_types [compliance_signal], and the query generator
// emitted a traversal from a node nothing can construct. It matched nothing and
// reported nothing.
func TestALiveRelationshipMayNotReferenceARetiredType(t *testing.T) {
	retiredNode := NodeType{Name: "compliance_signal", Number: 17, Category: "finding", Retired: true}

	t.Run("every endpoint retired", func(t *testing.T) {
		tax := core([]NodeType{node("finding", 1), retiredNode}, []RelationshipType{{
			Name: "TRIGGERED", Number: 2,
			FromTypes: []string{"compliance_signal"}, ToTypes: []string{"finding"},
			Cardinality: "one_to_many",
		}})
		err := tax.Validate()
		if err == nil {
			t.Fatal("a live relationship whose only source is retired was accepted")
		}
		for _, want := range []string{"TRIGGERED", "compliance_signal", "no TRIGGERED edge can exist"} {
			if !strings.Contains(err.Error(), want) {
				t.Errorf("the error does not mention %q: %v", want, err)
			}
		}
	})

	t.Run("one endpoint of several retired", func(t *testing.T) {
		tax := core([]NodeType{node("finding", 1), node("host", 2), retiredNode}, []RelationshipType{{
			Name: "AFFECTS", Number: 2,
			FromTypes: []string{"finding"}, ToTypes: []string{"host", "compliance_signal"},
			Cardinality: "one_to_many",
		}})
		err := tax.Validate()
		if err == nil {
			t.Fatal("a live relationship naming a retired type among live ones was accepted")
		}
		if !strings.Contains(err.Error(), "to_types") {
			t.Errorf("the error does not name the offending field: %v", err)
		}
	})

	t.Run("retiring the relationship too is accepted", func(t *testing.T) {
		tax := core([]NodeType{node("finding", 1), retiredNode}, []RelationshipType{{
			Name: "TRIGGERED", Number: 2, Retired: true,
			FromTypes: []string{"compliance_signal"}, ToTypes: []string{"finding"},
			Cardinality: "one_to_many",
		}})
		if err := tax.Validate(); err != nil {
			t.Errorf("retiring both was refused: %v", err)
		}
	})

	t.Run("a wildcard is not a reference", func(t *testing.T) {
		tax := core([]NodeType{node("finding", 1), retiredNode}, []RelationshipType{{
			Name: "LEADS_TO", Number: 2,
			FromTypes: []string{"finding"}, ToTypes: []string{"*"},
			Cardinality: "many_to_many",
		}})
		if err := tax.Validate(); err != nil {
			t.Errorf("a wildcard target was refused because a type is retired: %v", err)
		}
	})
}
