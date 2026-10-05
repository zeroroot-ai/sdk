// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Zero Root AI

package serve

import (
	"context"
	"errors"
	"fmt"

	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"

	"github.com/zeroroot-ai/sdk/agent"
	harnesspb "github.com/zeroroot-ai/sdk/api/gen/gibson/harness/v1"
)

// betToProto converts a typed agent.Bet into a PlaceBetRequest. Scope is
// intentionally not carried — the daemon derives it from mission context, same
// as Observe (ADR-0107 / ADR-0122).
func betToProto(bet agent.Bet) *harnesspb.PlaceBetRequest {
	return &harnesspb.PlaceBetRequest{
		Bet: &harnesspb.Bet{
			HypothesisId: bet.HypothesisID,
			StakingAgent: bet.StakingAgent,
			Confidence:   bet.Confidence,
			Technique:    bet.Technique,
		},
	}
}

// PlaceBet stakes a calibrated confidence on a Hypothesis via the callback
// channel (ADR-0122). Storage, settlement and scoring of the bet are a
// gibson-side concern; this is the agent's write path only.
func (h *CallbackHarness) PlaceBet(ctx context.Context, bet agent.Bet) error {
	ctx, span := h.tracer.Start(ctx, "gibson.brain.place_bet", trace.WithSpanKind(trace.SpanKindClient))
	defer span.End()

	req := betToProto(bet)
	req.Context = h.client.contextInfo()

	resp, err := h.client.PlaceBet(ctx, req)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return fmt.Errorf("PlaceBet callback failed: %w", err)
	}
	if resp.Error != nil {
		return fmt.Errorf("PlaceBet rejected: %s", resp.Error.Message)
	}
	return nil
}

// PlaceBet is not yet wired in platform pull-mode, mirroring Observe's status:
// that transport emits over the component service, which has no typed bet
// endpoint yet. No existing agent calls PlaceBet (it is new in ADR-0122), so
// this is safe to leave unsupported here.
func (h *PlatformHarness) PlaceBet(_ context.Context, _ agent.Bet) error {
	return errors.New("PlaceBet is not supported in platform pull-mode yet (use the callback harness); tracked for the platform transport")
}
