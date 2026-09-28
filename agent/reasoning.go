// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Zero Root AI

package agent

// ReasoningStepObservation reports one step of an agent's own structured,
// multi-step reasoning over graph state (sdk#73): what it inferred, and the
// entities its inference is about. Like a Hypothesis (ADR-0021), it is the
// agent's own deduction, not a sighting; unlike a Hypothesis it is not a
// proposed claim awaiting settlement, but one step in the argument that leads
// to one.
//
// It builds on the planning package's step model: StepIndex is typically the
// mission's current step (see planning.PlanningContext.CurrentStepIndex), so
// a reasoning trace aligns with the mission's own plan rather than keeping a
// disconnected sequence of its own.
//
// Like every Observation, this is an emit: the daemon folds it onto the
// Timeline (flight recorder), so state-of-the-art offensive reasoning is
// expressed in the SDK and is inspectable after the fact, not buried in a
// prompt. A completed multi-step reasoning trace reads back in order by
// grouping on PlanID.
type ReasoningStepObservation struct {
	// Agent identifies the agent doing the reasoning.
	Agent string
	// PlanID groups every step of one reasoning trace. Agent-assigned: the SDK
	// does not read node ids back, so a plan does not get one from the daemon.
	PlanID string
	// StepIndex orders steps within PlanID.
	StepIndex int
	// Step states what the agent reasoned or decided at this step.
	Step string
	// References names the entities this step's reasoning is about, by label
	// and identity (the same scheme HypothesisObservation.References uses).
	References []ReferencedEntity
}

func (ReasoningStepObservation) isObservation() {}
