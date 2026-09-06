// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Zero Root AI

package agent

import (
	"context"
	"log/slog"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/zeroroot-ai/sdk/finding"
	"github.com/zeroroot-ai/sdk/mission"
	"github.com/zeroroot-ai/sdk/planning"
	"github.com/zeroroot-ai/sdk/types"
)

// TestBaseHarness_SatisfiesInterface verifies the compile-time assertion that
// *BaseHarness satisfies Harness. If BaseHarness ever drifts from the interface
// this test will fail to compile rather than at runtime.
func TestBaseHarness_SatisfiesInterface(t *testing.T) {
	bh := NewBaseHarness(slog.Default())
	var _ Harness = &bh
}

// TestBaseHarness_DefaultMethods calls every method defined on the Harness
// interface through a *BaseHarness value and asserts that:
//   - methods returning (T, error) return a non-nil error
//   - methods returning a plain error return a non-nil error
//   - methods returning interface types return nil
//   - Logger() returns a non-nil *slog.Logger
//   - value-type returns (struct, map, channel) return their zero / empty equivalents
func TestBaseHarness_DefaultMethods(t *testing.T) {
	bh := NewBaseHarness(slog.Default())
	h := &bh
	ctx := context.Background()

	// --- LLM access ---

	t.Run("Complete", func(t *testing.T) {
		resp, err := h.Complete(ctx, "primary", nil)
		assert.Nil(t, resp)
		require.Error(t, err)
	})

	t.Run("CompleteWithTools", func(t *testing.T) {
		resp, err := h.CompleteWithTools(ctx, "primary", nil, nil)
		assert.Nil(t, resp)
		require.Error(t, err)
	})

	t.Run("Stream", func(t *testing.T) {
		ch, err := h.Stream(ctx, "primary", nil)
		assert.Nil(t, ch)
		require.Error(t, err)
	})

	t.Run("CompleteStructured", func(t *testing.T) {
		result, err := h.CompleteStructured(ctx, "primary", nil, struct{}{})
		assert.Nil(t, result)
		require.Error(t, err)
	})

	t.Run("CompleteStructuredAny", func(t *testing.T) {
		result, err := h.CompleteStructuredAny(ctx, "primary", nil, struct{}{})
		assert.Nil(t, result)
		require.Error(t, err)
	})

	// --- Tool access ---

	t.Run("CallToolProto", func(t *testing.T) {
		err := h.CallToolProto(ctx, "mytool", nil, nil)
		require.Error(t, err)
	})

	t.Run("CallToolProtoStream", func(t *testing.T) {
		err := h.CallToolProtoStream(ctx, "mytool", nil, nil, nil)
		require.Error(t, err)
	})

	t.Run("ListTools", func(t *testing.T) {
		tools, err := h.ListTools(ctx)
		assert.Nil(t, tools)
		require.Error(t, err)
	})

	t.Run("QueueToolWork", func(t *testing.T) {
		jobID, err := h.QueueToolWork(ctx, "mytool", nil)
		assert.Empty(t, jobID)
		require.Error(t, err)
	})

	t.Run("ToolResults_returns_closed_channel", func(t *testing.T) {
		ch := h.ToolResults(ctx, "some-job-id")
		require.NotNil(t, ch)
		// A ranging loop over a closed channel exits immediately with zero iterations.
		count := 0
		for range ch {
			count++
		}
		assert.Equal(t, 0, count, "ToolResults should return a closed channel with no items")
	})

	// --- Plugin access ---

	t.Run("QueryPlugin", func(t *testing.T) {
		result, err := h.QueryPlugin(ctx, "gitlab", "list_projects", nil)
		assert.Nil(t, result)
		require.Error(t, err)
	})

	t.Run("ListPlugins", func(t *testing.T) {
		plugins, err := h.ListPlugins(ctx)
		assert.Nil(t, plugins)
		require.Error(t, err)
	})

	// --- Agent delegation ---

	t.Run("DelegateToAgent", func(t *testing.T) {
		result, err := h.DelegateToAgent(ctx, "recon-agent", Task{})
		assert.Equal(t, Result{}, result)
		require.Error(t, err)
	})

	t.Run("ListAgents", func(t *testing.T) {
		agents, err := h.ListAgents(ctx)
		assert.Nil(t, agents)
		require.Error(t, err)
	})

	// --- Finding management ---

	t.Run("SubmitFinding", func(t *testing.T) {
		err := h.SubmitFinding(ctx, &finding.Finding{})
		require.Error(t, err)
	})

	// --- Context accessors (value returns, no error) ---

	t.Run("Mission_returns_zero_value", func(t *testing.T) {
		assert.Equal(t, types.MissionContext{}, h.Mission())
	})

	t.Run("Target_returns_zero_value", func(t *testing.T) {
		assert.Equal(t, types.TargetInfo{}, h.Target())
	})

	// --- Observability ---

	t.Run("Tracer_returns_noop_non_nil", func(t *testing.T) {
		assert.NotNil(t, h.Tracer())
	})

	t.Run("Logger_returns_non_nil", func(t *testing.T) {
		assert.NotNil(t, h.Logger())
	})

	t.Run("TokenUsage_returns_nil", func(t *testing.T) {
		assert.Nil(t, h.TokenUsage())
	})

	// --- Observation emit ---

	t.Run("Observe", func(t *testing.T) {
		err := h.Observe(ctx, HostObservation{Address: "10.0.0.1"})
		require.Error(t, err)
	})

	// --- Planning context ---

	t.Run("PlanContext_returns_nil", func(t *testing.T) {
		assert.Nil(t, h.PlanContext())
	})

	t.Run("ReportStepHints", func(t *testing.T) {
		err := h.ReportStepHints(ctx, &planning.StepHints{})
		require.Error(t, err)
	})

	// --- MissionManager methods ---

	t.Run("CreateMission", func(t *testing.T) {
		info, err := h.CreateMission(ctx, nil, "target-id", &mission.CreateMissionOpts{})
		assert.Nil(t, info)
		require.Error(t, err)
	})

	t.Run("RunMission", func(t *testing.T) {
		err := h.RunMission(ctx, "mission-id", &mission.RunMissionOpts{})
		require.Error(t, err)
	})

	t.Run("GetMissionStatus", func(t *testing.T) {
		status, err := h.GetMissionStatus(ctx, "mission-id")
		assert.Nil(t, status)
		require.Error(t, err)
	})

	t.Run("WaitForMission", func(t *testing.T) {
		result, err := h.WaitForMission(ctx, "mission-id", 5*time.Second)
		assert.Nil(t, result)
		require.Error(t, err)
	})

	t.Run("ListMissions", func(t *testing.T) {
		missions, err := h.ListMissions(ctx, &mission.MissionFilter{})
		assert.Nil(t, missions)
		require.Error(t, err)
	})

	t.Run("CancelMission", func(t *testing.T) {
		err := h.CancelMission(ctx, "mission-id")
		require.Error(t, err)
	})

	t.Run("GetMissionResults", func(t *testing.T) {
		result, err := h.GetMissionResults(ctx, "mission-id")
		assert.Nil(t, result)
		require.Error(t, err)
	})

	// --- Workspace access ---

	t.Run("Workspace_returns_nil", func(t *testing.T) {
		assert.Nil(t, h.Workspace())
	})

	t.Run("Workspaces_returns_empty_map", func(t *testing.T) {
		ws := h.Workspaces()
		require.NotNil(t, ws)
		assert.Empty(t, ws)
	})
}
