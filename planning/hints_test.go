// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Zero Root AI

package planning

import (
	"reflect"
	"testing"
)

func TestStepHintsGetters(t *testing.T) {
	h := &StepHints{
		confidence:    0.5,
		suggestedNext: []string{"a"},
		replanReason:  "why",
		keyFindings:   []string{"k"},
	}
	if h.Confidence() != 0.5 || h.ReplanReason() != "why" {
		t.Fatalf("Confidence/ReplanReason = %v/%q", h.Confidence(), h.ReplanReason())
	}
	next := h.SuggestedNext()
	if !reflect.DeepEqual(next, []string{"a"}) {
		t.Fatalf("SuggestedNext = %v", next)
	}
	next[0] = "changed"
	if h.SuggestedNext()[0] != "a" {
		t.Fatal("SuggestedNext must return a copy")
	}
	found := h.KeyFindings()
	if !reflect.DeepEqual(found, []string{"k"}) {
		t.Fatalf("KeyFindings = %v", found)
	}
	found[0] = "changed"
	if h.KeyFindings()[0] != "k" {
		t.Fatal("KeyFindings must return a copy")
	}
}
