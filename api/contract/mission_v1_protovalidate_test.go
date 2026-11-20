// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Zero Root AI

package contract_test

// mission_v1_protovalidate_test.go — runtime exercise of the buf.validate
// annotations declared in gibson/mission/v1.
//
// The annotations are static metadata in the .proto until somebody actually
// runs a validator over a message instance. This file does that via
// buf.build/go/protovalidate (the runtime companion to the protovalidate
// proto definitions already vendored in go.mod), asserting for each
// constraint shipped in mission/v1 today:
//
//   - A known-good input passes validation.
//   - A known-bad input is rejected with an error that names the field.
//
// Constraints covered:
//
//   - AgentNodeConfig.max_tokens_per_call    int32 gte: 0
//   - ToolNodeConfig.max_tokens_per_call     int32 gte: 0
//   - PluginNodeConfig.max_tokens_per_call   int32 gte: 0
//   - JoinNodeConfig.wait_for                repeated string min_items: 1
//   - MissionConstraints fix loop fields     duration gte: 0, enum defined_only,
//     restart_window pattern and the rule that ties it to restart_policy
//
// As future buf.validate annotations land in mission/v1, extend this file
// in the same shape: one sub-test per known-good case and one per
// known-bad case, named after the field under test.

import (
	"strings"
	"testing"
	"time"

	"buf.build/go/protovalidate"
	"google.golang.org/protobuf/types/known/durationpb"

	missionpb "github.com/zeroroot-ai/sdk/api/gen/gibson/mission/v1"
)

func newValidator(t *testing.T) protovalidate.Validator {
	t.Helper()
	v, err := protovalidate.New()
	if err != nil {
		t.Fatalf("protovalidate.New: %v", err)
	}
	return v
}

// int32Ptr returns a pointer to the given int32; needed because mission/v1
// uses optional int32 for the max_tokens_per_call fields.
func ptrInt32(v int32) *int32 { return &v }

func TestMissionV1_AgentNodeConfig_MaxTokensPerCall(t *testing.T) {
	v := newValidator(t)

	t.Run("zero is accepted", func(t *testing.T) {
		if err := v.Validate(&missionpb.AgentNodeConfig{
			AgentName:        "scanner",
			MaxTokensPerCall: ptrInt32(0),
		}); err != nil {
			t.Fatalf("expected zero to pass gte:0, got: %v", err)
		}
	})

	t.Run("positive is accepted", func(t *testing.T) {
		if err := v.Validate(&missionpb.AgentNodeConfig{
			AgentName:        "scanner",
			MaxTokensPerCall: ptrInt32(8192),
		}); err != nil {
			t.Fatalf("expected positive value to pass, got: %v", err)
		}
	})

	t.Run("negative is rejected", func(t *testing.T) {
		err := v.Validate(&missionpb.AgentNodeConfig{
			AgentName:        "scanner",
			MaxTokensPerCall: ptrInt32(-1),
		})
		if err == nil {
			t.Fatal("expected -1 to violate gte:0, got nil")
		}
		if !strings.Contains(err.Error(), "max_tokens_per_call") {
			t.Errorf("error should name the offending field: %v", err)
		}
	})
}

func TestMissionV1_ToolNodeConfig_MaxTokensPerCall(t *testing.T) {
	v := newValidator(t)

	t.Run("positive is accepted", func(t *testing.T) {
		if err := v.Validate(&missionpb.ToolNodeConfig{
			ToolName:         "nmap",
			MaxTokensPerCall: ptrInt32(4096),
		}); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	t.Run("negative is rejected", func(t *testing.T) {
		err := v.Validate(&missionpb.ToolNodeConfig{
			ToolName:         "nmap",
			MaxTokensPerCall: ptrInt32(-1),
		})
		if err == nil {
			t.Fatal("expected -1 to violate gte:0, got nil")
		}
		if !strings.Contains(err.Error(), "max_tokens_per_call") {
			t.Errorf("error should name the offending field: %v", err)
		}
	})
}

func TestMissionV1_PluginNodeConfig_MaxTokensPerCall(t *testing.T) {
	v := newValidator(t)

	t.Run("positive is accepted", func(t *testing.T) {
		if err := v.Validate(&missionpb.PluginNodeConfig{
			PluginName:       "trivy",
			Method:           "scan",
			MaxTokensPerCall: ptrInt32(2048),
		}); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	t.Run("negative is rejected", func(t *testing.T) {
		err := v.Validate(&missionpb.PluginNodeConfig{
			PluginName:       "trivy",
			Method:           "scan",
			MaxTokensPerCall: ptrInt32(-5),
		})
		if err == nil {
			t.Fatal("expected -5 to violate gte:0, got nil")
		}
		if !strings.Contains(err.Error(), "max_tokens_per_call") {
			t.Errorf("error should name the offending field: %v", err)
		}
	})
}

func TestMissionV1_JoinNodeConfig_WaitFor(t *testing.T) {
	v := newValidator(t)

	t.Run("single dependency is accepted", func(t *testing.T) {
		if err := v.Validate(&missionpb.JoinNodeConfig{
			WaitFor: []string{"agent-1"},
		}); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	t.Run("multiple dependencies are accepted", func(t *testing.T) {
		if err := v.Validate(&missionpb.JoinNodeConfig{
			WaitFor: []string{"agent-1", "agent-2", "tool-3"},
		}); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	t.Run("empty list is rejected", func(t *testing.T) {
		err := v.Validate(&missionpb.JoinNodeConfig{
			WaitFor: nil,
		})
		if err == nil {
			t.Fatal("expected empty wait_for to violate min_items:1, got nil")
		}
		if !strings.Contains(err.Error(), "wait_for") {
			t.Errorf("error should name the offending field: %v", err)
		}
	})
}

func TestMissionV1_MissionConstraints_FixLoop(t *testing.T) {
	v := newValidator(t)
	window := &missionpb.RestartWindow{Start: "02:00", End: "04:30"}

	accepted := map[string]*missionpb.MissionConstraints{
		"empty constraints": {},
		"restarts allowed": {
			HealthHold:    durationpb.New(10 * time.Minute),
			RestartPolicy: missionpb.RestartPolicy_RESTART_POLICY_ALLOWED,
			RestartLimit:  durationpb.New(5 * time.Minute),
		},
		"restarts forbidden": {RestartPolicy: missionpb.RestartPolicy_RESTART_POLICY_FORBIDDEN},
		"restarts in a window": {
			RestartPolicy: missionpb.RestartPolicy_RESTART_POLICY_IN_WINDOW,
			RestartWindow: window,
		},
		"a window that crosses midnight": {
			RestartPolicy: missionpb.RestartPolicy_RESTART_POLICY_IN_WINDOW,
			RestartWindow: &missionpb.RestartWindow{Start: "23:00", End: "01:00"},
		},
	}
	for name, c := range accepted {
		t.Run("accepts "+name, func(t *testing.T) {
			if err := v.Validate(c); err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
		})
	}

	rejected := map[string]struct {
		c    *missionpb.MissionConstraints
		want string
	}{
		"a negative health_hold": {
			&missionpb.MissionConstraints{HealthHold: durationpb.New(-time.Second)}, "health_hold",
		},
		"a negative restart_limit": {
			&missionpb.MissionConstraints{RestartLimit: durationpb.New(-time.Second)}, "restart_limit",
		},
		"an unknown restart_policy": {
			&missionpb.MissionConstraints{RestartPolicy: missionpb.RestartPolicy(9)}, "restart_policy",
		},
		"the window policy with no window": {
			&missionpb.MissionConstraints{RestartPolicy: missionpb.RestartPolicy_RESTART_POLICY_IN_WINDOW},
			"restart_window",
		},
		"a window with another policy": {
			&missionpb.MissionConstraints{
				RestartPolicy: missionpb.RestartPolicy_RESTART_POLICY_ALLOWED,
				RestartWindow: window,
			},
			"restart_window",
		},
		"a window with a bad time": {
			&missionpb.MissionConstraints{
				RestartPolicy: missionpb.RestartPolicy_RESTART_POLICY_IN_WINDOW,
				RestartWindow: &missionpb.RestartWindow{Start: "25:00", End: "04:00"},
			},
			"start",
		},
		"a window with no end": {
			&missionpb.MissionConstraints{
				RestartPolicy: missionpb.RestartPolicy_RESTART_POLICY_IN_WINDOW,
				RestartWindow: &missionpb.RestartWindow{Start: "02:00"},
			},
			"end",
		},
	}
	for name, tc := range rejected {
		t.Run("rejects "+name, func(t *testing.T) {
			err := v.Validate(tc.c)
			if err == nil {
				t.Fatal("expected a validation error, got nil")
			}
			if !strings.Contains(err.Error(), tc.want) {
				t.Errorf("error should name %q: %v", tc.want, err)
			}
		})
	}
}
