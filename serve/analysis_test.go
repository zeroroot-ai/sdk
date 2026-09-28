// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Zero Root AI

package serve

import (
	"testing"

	"google.golang.org/protobuf/proto"

	"github.com/zeroroot-ai/sdk/agent"
	harnesspb "github.com/zeroroot-ai/sdk/api/gen/gibson/harness/v1"
)

// TestObservationToProto_Analysis: an analysis artifact carries its
// conclusion, confidence, and the evidence it used from the graph onto the
// wire (sdk#74). It rides the normal Observe emit-only surface — one more
// observation kind, never a raw graph write (ADR-0007).
func TestObservationToProto_Analysis(t *testing.T) {
	req, err := observationToProto(agent.AnalysisObservation{
		Agent:      "triage-agent",
		Conclusion: "the exposed admin API is the root cause of the breach",
		Confidence: 0.9,
		Evidence: []agent.ReferencedEntity{
			{Label: "Finding", IDProperties: map[string]string{"brain_id": "f-1"}},
		},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	a := req.GetAnalysis()
	if a == nil {
		t.Fatal("expected analysis observation in request")
	}
	if a.Agent != "triage-agent" {
		t.Fatalf("agent not mapped: %+v", a)
	}
	if a.Conclusion != "the exposed admin API is the root cause of the breach" {
		t.Fatalf("conclusion not mapped: %+v", a)
	}
	if a.Confidence != 0.9 {
		t.Fatalf("confidence not mapped: %+v", a)
	}
	if len(a.Evidence) != 1 || a.Evidence[0].Label != "Finding" {
		t.Fatalf("evidence not mapped: %+v", a.Evidence)
	}
	if req.Context != nil {
		t.Fatalf("observation should not carry context/scope, got %+v", req.Context)
	}
}

// TestObservationToProto_Analysis_WireRoundTrip: the analysis artifact must
// survive an actual proto marshal/unmarshal, not just struct construction.
// This deliberately mirrors every other Observation variant's own
// marshal/unmarshal round-trip test (Hypothesis, ReasoningStep); each variant
// needs this same wire-survival check, not a shared copy-paste bug.
//
//nolint:dupl // see comment above
func TestObservationToProto_Analysis_WireRoundTrip(t *testing.T) {
	req, err := observationToProto(agent.AnalysisObservation{
		Agent:      "exploit-agent",
		Conclusion: "the lodash dependency is exploitable via prototype pollution",
		Confidence: 0.72,
		Evidence: []agent.ReferencedEntity{
			{Label: "Package", IDProperties: map[string]string{"purl": "pkg:npm/lodash@4.17.20"}},
			{Label: "Finding", IDProperties: map[string]string{"brain_id": "f-42"}},
		},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	wire, err := proto.Marshal(req)
	if err != nil {
		t.Fatalf("marshal failed: %v", err)
	}

	got := &harnesspb.ObserveRequest{}
	if err := proto.Unmarshal(wire, got); err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}

	a := got.GetAnalysis()
	if a == nil {
		t.Fatal("expected analysis observation after round trip")
	}
	if a.Agent != "exploit-agent" || a.Confidence != 0.72 {
		t.Fatalf("agent/confidence lost in round trip: %+v", a)
	}
	if a.Conclusion != "the lodash dependency is exploitable via prototype pollution" {
		t.Fatalf("conclusion lost in round trip: %+v", a)
	}
	if len(a.Evidence) != 2 {
		t.Fatalf("evidence lost in round trip: %+v", a.Evidence)
	}
	if a.Evidence[0].Label != "Package" || a.Evidence[0].IdProperties["purl"] != "pkg:npm/lodash@4.17.20" {
		t.Fatalf("first evidence ref lost in round trip: %+v", a.Evidence[0])
	}
	if a.Evidence[1].Label != "Finding" || a.Evidence[1].IdProperties["brain_id"] != "f-42" {
		t.Fatalf("second evidence ref lost in round trip: %+v", a.Evidence[1])
	}
}

// TestObservationToProto_Analysis_EmptyEvidenceStaysEmpty: an analysis with
// no evidence refs must not acquire a phantom one from a nil slice.
func TestObservationToProto_Analysis_EmptyEvidenceStaysEmpty(t *testing.T) {
	req, err := observationToProto(agent.AnalysisObservation{
		Agent:      "triage-agent",
		Conclusion: "no root cause identified yet",
		Confidence: 0.1,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	a := req.GetAnalysis()
	if a == nil {
		t.Fatal("expected analysis observation in request")
	}
	if len(a.Evidence) != 0 {
		t.Fatalf("expected no evidence, got %+v", a.Evidence)
	}
}
