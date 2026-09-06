// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Zero Root AI

// Package planning provides planning-aware interfaces for agent developers.
//
// This package enables agents to:
//   - Be aware of their position in mission execution
//   - Access budget constraints (step and mission level)
//   - Provide feedback to the planning system
//   - Suggest next steps and recommend replanning
//
// # PlanningContext
//
// The PlanningContext interface provides read-only access to the mission plan state.
// Agents receive this through the harness:
//
//	func (a *MyAgent) Execute(ctx context.Context, task agent.Task, harness harness.AgentHarness) (agent.Result, error) {
//	    planCtx := harness.PlanContext()
//	    if planCtx == nil {
//	        // Planning not enabled, proceed normally
//	        return a.executeNormal(ctx, task, harness)
//	    }
//
//	    // Adapt based on plan position
//	    if planCtx.CurrentStepIndex() == 0 {
//	        // First step - initialize mission state
//	    }
//
//	    if planCtx.StepBudget() < 1000 {
//	        // Low budget - use efficient strategy
//	    }
//
//	    if planCtx.CurrentStepIndex() == planCtx.TotalSteps()-1 {
//	        // Last step - summarize findings
//	    }
//
//	    // ...
//	}
//
// # StepHints
//
// The StepHints builder allows agents to provide feedback to the planner.
// Use the fluent builder pattern to construct hints:
//
//	hints := planning.NewStepHints().
//	    WithConfidence(0.85).
//	    WithKeyFinding("Admin panel discovered at /admin").
//	    WithKeyFinding("Default credentials may be in use").
//	    WithSuggestion("auth_bypass_agent").
//	    RecommendReplan("Target uses custom auth - standard attacks ineffective")
//
//	// Report to the framework
//	harness.ReportStepHints(ctx, hints)
//
// The framework uses these hints to:
//   - Score step execution quality
//   - Decide whether tactical replanning is needed
//   - Inform the next planning cycle
//   - Track agent confidence over time
//
// # Design Principles
//
// This package is designed to be standalone and usable by external agent developers.
// It has no dependencies on Gibson's internal packages and uses only the standard library.
//
// The interfaces are read-only to prevent agents from modifying mission state directly.
// All state changes flow through the harness to maintain consistency.
package planning
