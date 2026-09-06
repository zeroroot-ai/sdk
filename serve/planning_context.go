// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Zero Root AI

package serve

import harnesspb "github.com/zeroroot-ai/sdk/api/gen/gibson/harness/v1"

// planContextWrapper wraps a harnesspb.PlanContext and implements planning.PlanningContext.
type planContextWrapper struct {
	proto *harnesspb.PlanContext
}

// CurrentStepIndex returns the 0-based index of the current step in the plan.
func (p *planContextWrapper) CurrentStepIndex() int {
	if p.proto == nil {
		return 0
	}
	return int(p.proto.CurrentStepIndex)
}

// TotalSteps returns the total number of planned steps in the mission.
func (p *planContextWrapper) TotalSteps() int {
	if p.proto == nil {
		return 0
	}
	return int(p.proto.TotalSteps)
}

// RemainingSteps returns the node IDs that will execute after this step.
func (p *planContextWrapper) RemainingSteps() []string {
	if p.proto == nil {
		return nil
	}
	// Return a copy to prevent external modification
	result := make([]string, len(p.proto.RemainingSteps))
	copy(result, p.proto.RemainingSteps)
	return result
}

// StepBudget returns the token budget allocated for this specific step.
func (p *planContextWrapper) StepBudget() int {
	if p.proto == nil {
		return 0
	}
	return int(p.proto.StepBudget)
}

// MissionBudgetRemaining returns the total remaining mission token budget.
func (p *planContextWrapper) MissionBudgetRemaining() int {
	if p.proto == nil {
		return 0
	}
	return int(p.proto.MissionBudgetRemaining)
}
