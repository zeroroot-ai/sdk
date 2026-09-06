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
//
// As future buf.validate annotations land in mission/v1, extend this file
// in the same shape: one sub-test per known-good case and one per
// known-bad case, named after the field under test.

import (
	"strings"
	"testing"

	"buf.build/go/protovalidate"

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
