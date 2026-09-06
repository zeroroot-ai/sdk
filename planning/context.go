// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Zero Root AI

// Package planning provides planning-aware interfaces for agent developers.
// This package enables agents to be aware of their position in mission execution,
// budget constraints, and to provide feedback to the planning system.
package planning

// PlanningContext provides read-only access to mission planning state.
// This allows agents to be aware of their position in the execution plan
// and make decisions based on remaining steps and budget.
//
// Example usage:
//
//	func (a *MyAgent) Execute(ctx context.Context, task agent.Task, harness harness.AgentHarness) (agent.Result, error) {
//	    planCtx := harness.PlanContext()
//	    if planCtx != nil {
//	        if planCtx.StepBudget() < 1000 {
//	            // Use efficient strategy
//	        }
//	        if planCtx.CurrentStepIndex() == planCtx.TotalSteps()-1 {
//	            // Last step - summarize findings
//	        }
//	    }
//	    // ...
//	}
type PlanningContext interface {
	// CurrentStepIndex returns the 0-based index of the current step in the plan.
	CurrentStepIndex() int

	// TotalSteps returns the total number of planned steps in the mission.
	TotalSteps() int

	// RemainingSteps returns the node IDs that will execute after this step.
	RemainingSteps() []string

	// StepBudget returns the token budget allocated for this specific step.
	// Returns 0 if no specific budget is set (unlimited).
	StepBudget() int

	// MissionBudgetRemaining returns the total remaining mission token budget.
	// Returns 0 if no budget tracking is enabled.
	MissionBudgetRemaining() int
}
