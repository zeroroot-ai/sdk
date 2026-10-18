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
