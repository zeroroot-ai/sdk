// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Zero Root AI

package serve

import (
	"testing"

	"google.golang.org/protobuf/proto"

	"github.com/zeroroot-ai/sdk/agent"
	harnesspb "github.com/zeroroot-ai/sdk/api/gen/gibson/harness/v1"
)

// TestBetToProto: a Bet carries the hypothesis it is on, the staking agent,
// the staked confidence and the technique it exercises onto the wire
// (ADR-0022, sdk#71). Reputation is keyed on technique × environment, so the
// technique must ride on the bet.
func TestBetToProto(t *testing.T) {
	req := betToProto(agent.Bet{
		HypothesisID: "hyp-123",
		StakingAgent: "triage-agent",
		Confidence:   0.8,
		Technique:    "k8s-rbac-escalation",
	})

	b := req.GetBet()
	if b == nil {
		t.Fatal("expected bet in request")
	}
	if b.HypothesisId != "hyp-123" {
		t.Fatalf("hypothesis id not mapped: %+v", b)
	}
	if b.StakingAgent != "triage-agent" {
		t.Fatalf("staking agent not mapped: %+v", b)
	}
	if b.Confidence != 0.8 {
		t.Fatalf("confidence not mapped: %+v", b)
	}
	if b.Technique != "k8s-rbac-escalation" {
		t.Fatalf("technique not mapped: %+v", b)
	}
	// Scope and tenant are server-side, same as Observe. The request must not
	// carry context.
	if req.Context != nil {
		t.Fatalf("bet should not carry context/scope, got %+v", req.Context)
	}
}

// TestBetToProto_WireRoundTrip: the bet must survive an actual proto
// marshal/unmarshal, not just struct construction — the "round-trips through
// the wire" acceptance criterion (sdk#71).
func TestBetToProto_WireRoundTrip(t *testing.T) {
	req := betToProto(agent.Bet{
		HypothesisID: "hyp-456",
		StakingAgent: "exploit-agent",
		Confidence:   0.35,
		Technique:    "unauthenticated-api-access",
	})

	wire, err := proto.Marshal(req)
	if err != nil {
		t.Fatalf("marshal failed: %v", err)
	}

	got := &harnesspb.PlaceBetRequest{}
	if err := proto.Unmarshal(wire, got); err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}

	b := got.GetBet()
	if b == nil {
		t.Fatal("expected bet after round trip")
	}
	if b.HypothesisId != "hyp-456" || b.StakingAgent != "exploit-agent" {
		t.Fatalf("identity fields lost in round trip: %+v", b)
	}
	if b.Confidence != 0.35 {
		t.Fatalf("confidence lost in round trip: %+v", b)
	}
	if b.Technique != "unauthenticated-api-access" {
		t.Fatalf("technique lost in round trip: %+v", b)
	}
}

// TestPlatformHarness_PlaceBet_Unsupported: PlaceBet is not yet wired in
// platform pull-mode, mirroring Observe's status (ADR-0007/ADR-0022 land on
// the same emit surface, and platform pull-mode has no typed emit endpoint
// yet). It must fail loudly, not silently no-op.
func TestPlatformHarness_PlaceBet_Unsupported(t *testing.T) {
	h := &PlatformHarness{}
	err := h.PlaceBet(nil, agent.Bet{}) //nolint:staticcheck // nil context is fine for this error-path unit test
	if err == nil {
		t.Fatal("expected an error from PlatformHarness.PlaceBet, got nil")
	}
}
