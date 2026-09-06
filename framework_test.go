// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Zero Root AI

package sdk_test

import (
	"context"
	"errors"
	"testing"

	"github.com/zeroroot-ai/sdk"
)

func mustFramework(t *testing.T) sdk.Framework {
	t.Helper()
	f, err := sdk.NewFramework()
	if err != nil {
		t.Fatalf("NewFramework: %v", err)
	}
	return f
}

// TestSentinelErrors_AgentNotFound verifies that errors.Is correctly matches
// ErrAgentNotFound for Get and Unregister on a missing agent.
func TestSentinelErrors_AgentNotFound(t *testing.T) {
	f := mustFramework(t)

	agents := f.Agents()

	_, err := agents.Get("nonexistent")
	if err == nil {
		t.Fatal("expected error from Get, got nil")
	}
	if !errors.Is(err, sdk.ErrAgentNotFound) {
		t.Errorf("Get: errors.Is(err, ErrAgentNotFound) = false; err = %v", err)
	}

	err = agents.Unregister("nonexistent")
	if err == nil {
		t.Fatal("expected error from Unregister, got nil")
	}
	if !errors.Is(err, sdk.ErrAgentNotFound) {
		t.Errorf("Unregister: errors.Is(err, ErrAgentNotFound) = false; err = %v", err)
	}
}

// TestSentinelErrors_ToolNotFound verifies that errors.Is correctly matches
// ErrToolNotFound for Get on a missing tool.
func TestSentinelErrors_ToolNotFound(t *testing.T) {
	f := mustFramework(t)

	tools := f.Tools()

	_, err := tools.Get("nonexistent")
	if err == nil {
		t.Fatal("expected error from Get, got nil")
	}
	if !errors.Is(err, sdk.ErrToolNotFound) {
		t.Errorf("Get: errors.Is(err, ErrToolNotFound) = false; err = %v", err)
	}
}

// TestSentinelErrors_MissionNotFound verifies that errors.Is correctly matches
// ErrMissionNotFound for StartMission, StopMission, and GetMission on a missing mission.
func TestSentinelErrors_MissionNotFound(t *testing.T) {
	f := mustFramework(t)
	ctx := context.Background()

	err := f.StartMission(ctx, "ghost-mission")
	if err == nil {
		t.Fatal("expected error from StartMission, got nil")
	}
	if !errors.Is(err, sdk.ErrMissionNotFound) {
		t.Errorf("StartMission: errors.Is(err, ErrMissionNotFound) = false; err = %v", err)
	}

	err = f.StopMission(ctx, "ghost-mission")
	if err == nil {
		t.Fatal("expected error from StopMission, got nil")
	}
	if !errors.Is(err, sdk.ErrMissionNotFound) {
		t.Errorf("StopMission: errors.Is(err, ErrMissionNotFound) = false; err = %v", err)
	}

	_, err = f.GetMission(ctx, "ghost-mission")
	if err == nil {
		t.Fatal("expected error from GetMission, got nil")
	}
	if !errors.Is(err, sdk.ErrMissionNotFound) {
		t.Errorf("GetMission: errors.Is(err, ErrMissionNotFound) = false; err = %v", err)
	}
}
