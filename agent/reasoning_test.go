// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Zero Root AI

package agent

import "testing"

// TestReasoningStepObservation_IsObservation: a ReasoningStepObservation is a
// valid member of the closed Observation sum type (ADR-0107), so a completed
// multi-step reasoning trace lands on the Timeline the same way every other
// sighting does (sdk#73).
func TestReasoningStepObservation_IsObservation(t *testing.T) {
	var obs Observation = ReasoningStepObservation{
		Agent:     "triage-agent",
		PlanID:    "plan-1",
		StepIndex: 0,
		Step:      "the exposed service on port 6443 is the most promising lead",
		References: []ReferencedEntity{
			{Label: "Host", IDProperties: map[string]string{"address": "10.0.0.5"}},
		},
	}
	if _, ok := obs.(ReasoningStepObservation); !ok {
		t.Fatalf("expected ReasoningStepObservation to satisfy Observation, got %T", obs)
	}
}

// TestReasoningStepObservation_Fields: the type carries the agent, a PlanID
// that groups the steps of one reasoning trace, the step's position, its
// content, and the graph entities it reasons about (sdk#73 acceptance
// criteria: structured multi-step reasoning over graph state).
func TestReasoningStepObservation_Fields(t *testing.T) {
	s := ReasoningStepObservation{
		Agent:     "triage-agent",
		PlanID:    "plan-42",
		StepIndex: 2,
		Step:      "ruling out the admin panel: it requires a client certificate",
		References: []ReferencedEntity{
			{Label: "Endpoint", IDProperties: map[string]string{"path": "/admin"}},
		},
	}
	if s.Agent != "triage-agent" {
		t.Fatalf("agent not carried: %+v", s)
	}
	if s.PlanID != "plan-42" {
		t.Fatalf("plan id not carried: %+v", s)
	}
	if s.StepIndex != 2 {
		t.Fatalf("step index not carried: %+v", s)
	}
	if s.Step == "" {
		t.Fatalf("step content not carried: %+v", s)
	}
	if len(s.References) != 1 || s.References[0].Label != "Endpoint" {
		t.Fatalf("references not carried: %+v", s.References)
	}
}
