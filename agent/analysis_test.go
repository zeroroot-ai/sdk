// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Zero Root AI

package agent

import "testing"

// TestAnalysisObservation_IsObservation: an AnalysisObservation is a valid
// member of the closed Observation sum type (ADR-0007), so a typed
// conclusion lands on the Timeline the same way every other sighting does
// (sdk#74).
func TestAnalysisObservation_IsObservation(t *testing.T) {
	var obs Observation = AnalysisObservation{
		Agent:      "triage-agent",
		Conclusion: "the exposed admin API is the root cause of the breach",
		Confidence: 0.9,
		Evidence: []ReferencedEntity{
			{Label: "Finding", IDProperties: map[string]string{"brain_id": "f-1"}},
		},
	}
	if _, ok := obs.(AnalysisObservation); !ok {
		t.Fatalf("expected AnalysisObservation to satisfy Observation, got %T", obs)
	}
}

// TestAnalysisObservation_Fields: the type carries a typed conclusion,
// evidence refs, and a confidence — the bridge between raw findings and
// reportable output (sdk#74 acceptance criteria). It links to the evidence
// it used on the graph rather than asserting free text.
func TestAnalysisObservation_Fields(t *testing.T) {
	a := AnalysisObservation{
		Agent:      "triage-agent",
		Conclusion: "the lodash dependency is exploitable via prototype pollution",
		Confidence: 0.85,
		Evidence: []ReferencedEntity{
			{Label: "Package", IDProperties: map[string]string{"purl": "pkg:npm/lodash@4.17.20"}},
			{Label: "Finding", IDProperties: map[string]string{"brain_id": "f-42"}},
		},
	}
	if a.Agent != "triage-agent" {
		t.Fatalf("agent not carried: %+v", a)
	}
	if a.Conclusion == "" {
		t.Fatalf("conclusion not carried: %+v", a)
	}
	if a.Confidence != 0.85 {
		t.Fatalf("confidence not carried: %+v", a)
	}
	if len(a.Evidence) != 2 {
		t.Fatalf("expected 2 evidence refs, got %d: %+v", len(a.Evidence), a.Evidence)
	}
	if a.Evidence[0].Label != "Package" || a.Evidence[0].IDProperties["purl"] != "pkg:npm/lodash@4.17.20" {
		t.Fatalf("first evidence ref not mapped: %+v", a.Evidence[0])
	}
	if a.Evidence[1].Label != "Finding" || a.Evidence[1].IDProperties["brain_id"] != "f-42" {
		t.Fatalf("second evidence ref not mapped: %+v", a.Evidence[1])
	}
}
