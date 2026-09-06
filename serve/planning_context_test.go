// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Zero Root AI

package serve

import (
	"testing"

	"github.com/stretchr/testify/assert"
	harnesspb "github.com/zeroroot-ai/sdk/api/gen/gibson/harness/v1"
)

// TestPlanContextWrapper tests the planContextWrapper implementation.
func TestPlanContextWrapper(t *testing.T) {
	protoCtx := &harnesspb.PlanContext{
		CurrentStepIndex:       2,
		TotalSteps:             5,
		RemainingSteps:         []string{"step3", "step4", "step5"},
		StepBudget:             1000,
		MissionBudgetRemaining: 5000,
	}

	wrapper := &planContextWrapper{proto: protoCtx}

	assert.Equal(t, 2, wrapper.CurrentStepIndex())
	assert.Equal(t, 5, wrapper.TotalSteps())
	assert.Equal(t, []string{"step3", "step4", "step5"}, wrapper.RemainingSteps())
	assert.Equal(t, 1000, wrapper.StepBudget())
	assert.Equal(t, 5000, wrapper.MissionBudgetRemaining())
}

// TestPlanContextWrapperNil tests the wrapper with nil proto.
func TestPlanContextWrapperNil(t *testing.T) {
	wrapper := &planContextWrapper{proto: nil}

	assert.Equal(t, 0, wrapper.CurrentStepIndex())
	assert.Equal(t, 0, wrapper.TotalSteps())
	assert.Nil(t, wrapper.RemainingSteps())
	assert.Equal(t, 0, wrapper.StepBudget())
	assert.Equal(t, 0, wrapper.MissionBudgetRemaining())
}

// TestPlanContextWrapperRemainingStepsCopy tests that RemainingSteps returns a copy.
func TestPlanContextWrapperRemainingStepsCopy(t *testing.T) {
	protoCtx := &harnesspb.PlanContext{
		RemainingSteps: []string{"step1", "step2"},
	}

	wrapper := &planContextWrapper{proto: protoCtx}

	// Get the remaining steps
	steps := wrapper.RemainingSteps()

	// Modify the returned slice
	steps[0] = "modified"

	// Verify the original is unchanged
	steps2 := wrapper.RemainingSteps()
	assert.Equal(t, "step1", steps2[0])
	assert.NotEqual(t, "modified", steps2[0])
}
