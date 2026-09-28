// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Zero Root AI

package agent

// Bet is an agent's calibrated stake on a Hypothesis (ADR-0022): a stake that
// later settles, not a bare confidence number. Being wrong costs standing.
//
// Betting is a first-class SDK primitive, parallel to Observe: an agent places
// a bet through the same emit-only surface, and storage, settlement and
// scoring are a gibson-side concern (see the settlement slice, ADR-0023).
//
// Reputation attaches to a technique × environment pair, never to an agent
// (Claude members are interchangeable and ephemeral), so the technique the
// bet exercises must ride on the bet itself.
type Bet struct {
	// HypothesisID names the Hypothesis this bet is on (see
	// HypothesisObservation, ADR-0021).
	HypothesisID string
	// StakingAgent identifies the agent placing the bet.
	StakingAgent string
	// Confidence is the staked calibrated confidence, in [0,1].
	Confidence float64
	// Technique names the technique this bet exercises. The reputation key is
	// technique × environment, so this must ride on the bet.
	Technique string
}
