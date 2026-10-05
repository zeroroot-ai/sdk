// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Zero Root AI

package agent

import "testing"

// TestBet_Fields: a Bet carries the hypothesis it is on, the staking agent,
// the staked confidence, and the technique it exercises — the reputation key
// is technique × environment, so the technique must ride on the bet
// (ADR-0122, sdk#71 acceptance criteria).
func TestBet_Fields(t *testing.T) {
	b := Bet{
		HypothesisID: "hyp-123",
		StakingAgent: "triage-agent",
		Confidence:   0.8,
		Technique:    "k8s-rbac-escalation",
	}
	if b.HypothesisID != "hyp-123" {
		t.Fatalf("hypothesis id not carried: %+v", b)
	}
	if b.StakingAgent != "triage-agent" {
		t.Fatalf("staking agent not carried: %+v", b)
	}
	if b.Confidence != 0.8 {
		t.Fatalf("confidence not carried: %+v", b)
	}
	if b.Technique != "k8s-rbac-escalation" {
		t.Fatalf("technique not carried: %+v", b)
	}
}
