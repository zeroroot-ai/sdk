// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Zero Root AI

package agent

import "testing"

// TestHypothesisObservation_IsObservation: a HypothesisObservation is a valid
// member of the closed Observation sum type (ADR-0021), so it must satisfy the
// interface like every sighting variant.
func TestHypothesisObservation_IsObservation(t *testing.T) {
	var obs Observation = HypothesisObservation{
		Proposer:   "agent-1",
		Confidence: 0.75,
		Claim:      "port 6443 on 10.0.0.5 is unauthenticated",
		References: []ReferencedEntity{
			{Label: "Host", IDProperties: map[string]string{"address": "10.0.0.5"}},
		},
	}
	if _, ok := obs.(HypothesisObservation); !ok {
		t.Fatalf("expected HypothesisObservation to satisfy Observation, got %T", obs)
	}
}

// TestHypothesisObservation_Fields: the type carries proposer, confidence, claim,
// the technique it exercises, and the referenced entities the claim is about
// (sdk#70 acceptance criteria; Technique added for gibson#333/#284 — reputation
// keys on technique x environment, so it must ride on the Hypothesis, matching
// Bet.Technique).
func TestHypothesisObservation_Fields(t *testing.T) {
	h := HypothesisObservation{
		HypothesisID: "hyp-lodash-proto-pollution",
		Proposer:     "triage-agent",
		Confidence:   0.4,
		Claim:        "the lodash dependency is exploitable via prototype pollution",
		Technique:    "dependency-cve-match",
		References: []ReferencedEntity{
			{Label: "Package", IDProperties: map[string]string{"purl": "pkg:npm/lodash@4.17.20"}},
			{Label: "Application", IDProperties: map[string]string{"key": "customer-portal"}},
		},
	}
	if h.HypothesisID != "hyp-lodash-proto-pollution" {
		t.Fatalf("hypothesis id not carried: %+v", h)
	}
	if h.Proposer != "triage-agent" {
		t.Fatalf("proposer not carried: %+v", h)
	}
	if h.Confidence != 0.4 {
		t.Fatalf("confidence not carried: %+v", h)
	}
	if h.Claim == "" {
		t.Fatalf("claim not carried: %+v", h)
	}
	if h.Technique != "dependency-cve-match" {
		t.Fatalf("technique not carried: %+v", h)
	}
	if len(h.References) != 2 {
		t.Fatalf("expected 2 referenced entities, got %d: %+v", len(h.References), h.References)
	}
	if h.References[0].Label != "Package" || h.References[0].IDProperties["purl"] != "pkg:npm/lodash@4.17.20" {
		t.Fatalf("first reference not mapped: %+v", h.References[0])
	}
	if h.References[1].Label != "Application" || h.References[1].IDProperties["key"] != "customer-portal" {
		t.Fatalf("second reference not mapped: %+v", h.References[1])
	}
}
