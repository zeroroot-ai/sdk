// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Zero Root AI

package serve

import (
	"testing"

	"google.golang.org/protobuf/proto"

	"github.com/zeroroot-ai/sdk/agent"
	harnesspb "github.com/zeroroot-ai/sdk/api/gen/gibson/harness/v1"
)

// TestObservationToProto_ReasoningStep: a reasoning step carries the agent,
// the plan it belongs to, its position, its content, and the entities it
// reasons about onto the wire (sdk#73). It rides the normal Observe emit-only
// surface — one more observation kind, never a raw graph write (ADR-0107) —
// so it lands on the Timeline exactly like every other Observation.
func TestObservationToProto_ReasoningStep(t *testing.T) {
	req, err := observationToProto(agent.ReasoningStepObservation{
		Agent:     "triage-agent",
		PlanID:    "plan-1",
		StepIndex: 3,
		Step:      "the exposed service on port 6443 is the most promising lead",
		References: []agent.ReferencedEntity{
			{Label: "Host", IDProperties: map[string]string{"address": "10.0.0.5"}},
		},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	s := req.GetReasoningStep()
	if s == nil {
		t.Fatal("expected reasoning step observation in request")
	}
	if s.Agent != "triage-agent" {
		t.Fatalf("agent not mapped: %+v", s)
	}
	if s.PlanId != "plan-1" {
		t.Fatalf("plan id not mapped: %+v", s)
	}
	if s.StepIndex != 3 {
		t.Fatalf("step index not mapped: %+v", s)
	}
	if s.Step != "the exposed service on port 6443 is the most promising lead" {
		t.Fatalf("step content not mapped: %+v", s)
	}
	if len(s.References) != 1 || s.References[0].Label != "Host" {
		t.Fatalf("references not mapped: %+v", s.References)
	}
	if req.Context != nil {
		t.Fatalf("observation should not carry context/scope, got %+v", req.Context)
	}
}

// TestObservationToProto_ReasoningStep_WireRoundTrip: the step must survive an
// actual proto marshal/unmarshal, not just struct construction.
func TestObservationToProto_ReasoningStep_WireRoundTrip(t *testing.T) {
	req, err := observationToProto(agent.ReasoningStepObservation{
		Agent:     "exploit-agent",
		PlanID:    "plan-7",
		StepIndex: 0,
		Step:      "start from the unauthenticated admin API",
		References: []agent.ReferencedEntity{
			{Label: "Endpoint", IDProperties: map[string]string{"path": "/admin"}},
			{Label: "Host", IDProperties: map[string]string{"address": "10.0.0.9"}},
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

	s := got.GetReasoningStep()
	if s == nil {
		t.Fatal("expected reasoning step observation after round trip")
	}
	if s.Agent != "exploit-agent" || s.PlanId != "plan-7" || s.StepIndex != 0 {
		t.Fatalf("identity fields lost in round trip: %+v", s)
	}
	if s.Step != "start from the unauthenticated admin API" {
		t.Fatalf("step content lost in round trip: %+v", s)
	}
	if len(s.References) != 2 {
		t.Fatalf("references lost in round trip: %+v", s.References)
	}
}

// TestObservationToProto_ReasoningStep_EmptyReferencesStayEmpty: a reasoning
// step need not name entities (some steps are pure deduction), and a nil
// slice must not become a phantom reference.
func TestObservationToProto_ReasoningStep_EmptyReferencesStayEmpty(t *testing.T) {
	req, err := observationToProto(agent.ReasoningStepObservation{
		Agent:     "triage-agent",
		PlanID:    "plan-1",
		StepIndex: 1,
		Step:      "no direct evidence yet; widening the search",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	s := req.GetReasoningStep()
	if s == nil {
		t.Fatal("expected reasoning step observation in request")
	}
	if len(s.References) != 0 {
		t.Fatalf("expected no references, got %+v", s.References)
	}
}
