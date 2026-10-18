// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Zero Root AI

package schema

import "testing"

type cycleNode struct {
	Name string     `json:"name"`
	Next *cycleNode `json:"next,omitempty"`
}

type cycleTree struct {
	Label    string      `json:"label"`
	Children []cycleTree `json:"children"`
}

// sdk#302: a self-referential type overflowed the stack.
func TestFromType_DirectCycleStaysFinite(t *testing.T) {
	got := FromType(cycleNode{})
	if got.Type != "object" {
		t.Fatalf("root type = %q, want object", got.Type)
	}
	next, ok := got.Properties["next"]
	if !ok {
		t.Fatalf("no next property: %+v", got.Properties)
	}
	if next.Type != "object" || next.Properties != nil {
		t.Fatalf("next = %+v, want a bare object at the cycle", next)
	}
}

func TestFromType_CycleThroughASliceStaysFinite(t *testing.T) {
	got := FromType(cycleTree{})
	children := got.Properties["children"]
	if children.Type != "array" || children.Items == nil {
		t.Fatalf("children = %+v, want an array", children)
	}
	if children.Items.Type != "object" || children.Items.Properties != nil {
		t.Fatalf("children items = %+v, want a bare object at the cycle", *children.Items)
	}
}

// The same struct type twice side by side is not a cycle and keeps its
// properties both times.
func TestFromType_RepeatedTypeIsNotACycle(t *testing.T) {
	type pair struct {
		Left  cycleTree `json:"left"`
		Right cycleTree `json:"right"`
	}
	got := FromType(pair{})
	for _, k := range []string{"left", "right"} {
		if got.Properties[k].Properties["label"].Type != "string" {
			t.Fatalf("%s lost its properties: %+v", k, got.Properties[k])
		}
	}
}

type mutualA struct {
	B *mutualB `json:"b"`
}

type mutualB struct {
	A []mutualA `json:"a"`
}

type selfSlice []selfSlice

type selfPtr *selfPtr

type holdsSelfPtr struct {
	P selfPtr `json:"p"`
}

// A mutual cycle (A to B to A) stops at the second A.
func TestFromType_MutualCycleStaysFinite(t *testing.T) {
	got := FromType(mutualA{})
	a := got.Properties["b"].Properties["a"]
	if a.Type != "array" || a.Items == nil || a.Items.Type != "object" || a.Items.Properties != nil {
		t.Fatalf("b.a = %+v, want an array of bare objects", a)
	}
}

// Named slice and pointer types can refer to themselves too.
func TestFromType_SelfReferentialSliceAndPointerStayFinite(t *testing.T) {
	s := FromType(selfSlice{})
	if s.Type != "array" || s.Items == nil || s.Items.Type != "array" || s.Items.Items != nil {
		t.Fatalf("selfSlice = %+v, want an array of bare arrays", s)
	}
	p := FromType(holdsSelfPtr{})
	if p.Properties["p"].Type != "" {
		t.Fatalf("p = %+v, want the empty schema at the cycle", p.Properties["p"])
	}
}
