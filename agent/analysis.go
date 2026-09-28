// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Zero Root AI

package agent

// AnalysisObservation reports a typed analysis artifact (sdk#74): a
// conclusion an agent draws FROM evidence already on the graph, with the
// confidence it holds and the evidence it used — the bridge between raw
// findings and reportable output.
//
// Unlike a Hypothesis (an unproven, forward-looking claim awaiting
// settlement, ADR-0021), an Analysis is backward-looking: it synthesizes what
// is already known on the graph rather than proposing something new to test.
// It feeds hypotheses and findings as a typed artifact, never as free text.
//
// Like every Observation, this is an emit: the daemon folds it onto the
// Timeline (flight recorder, ADR-0007).
type AnalysisObservation struct {
	// Agent identifies the agent producing the analysis.
	Agent string
	// Conclusion states the typed conclusion in a form the brain can fold
	// into a Finding or a Hypothesis, not free text.
	Conclusion string
	// Confidence is the agent's calibrated confidence in the conclusion, in
	// [0,1].
	Confidence float64
	// Evidence names the entities on the graph this conclusion is based on,
	// by label and identity (the same scheme HypothesisObservation.References
	// uses) — the link from the conclusion back to the evidence it used.
	Evidence []ReferencedEntity
}

func (AnalysisObservation) isObservation() {}
