// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Zero Root AI

package contract_test

import (
	"testing"

	"google.golang.org/protobuf/types/known/durationpb"
	"google.golang.org/protobuf/types/known/timestamppb"

	bankpb "github.com/zeroroot-ai/sdk/api/gen/gibson/bank/v1"
	commonpb "github.com/zeroroot-ai/sdk/api/gen/gibson/common/v1"
)

func TestBankV1_AllMessagesRoundTrip(t *testing.T) {
	roundTripPackage(t, "gibson.bank.v1")
}

// TestBankV1_PopulatedBankRoundTrip round-trips a Bank with every field set,
// so a wire regression on the nested Principal, Duration or Timestamp fields
// cannot hide behind the zero-form package sweep.
func TestBankV1_PopulatedBankRoundTrip(t *testing.T) {
	roundTrip(t, &bankpb.Bank{
		Id:       "bank-1",
		TenantId: "tenant-1",
		Owner: &commonpb.Principal{
			Kind: commonpb.Principal_KIND_USER,
			Id:   "user-1",
		},
		Name:               "reviewers",
		DesiredCount:       2,
		LoginShape:         bankpb.LoginShape_LOGIN_SHAPE_SUBSCRIPTION,
		ProviderConfigName: "",
		AgentName:          "claude",
		Model:              "claude-sonnet-4-5",
		MaxJobsInFlight:    1,
		StaleLimit:         durationpb.New(3600e9),
		SpillPolicy:        bankpb.SpillPolicy_SPILL_POLICY_QUEUE,
		CreatedAt:          timestamppb.New(timestamppb.Now().AsTime()),
		UpdatedAt:          timestamppb.New(timestamppb.Now().AsTime()),
	})
}

// TestBankV1_PopulatedMemberRoundTrip round-trips a Member with a full
// MemberStatus, the shape the daemon returns from ListMembers.
func TestBankV1_PopulatedMemberRoundTrip(t *testing.T) {
	roundTrip(t, &bankpb.Member{
		Id:           "member-1",
		BankId:       "bank-1",
		MissionId:    "mission-1",
		MissionRunId: "run-1",
		AgentRunId:   "agent-run-1",
		SandboxId:    "sbx-1",
		Status: &bankpb.MemberStatus{
			State:         bankpb.MemberState_MEMBER_STATE_BUSY,
			JobsInFlight:  1,
			Cap:           1,
			ActiveJobIds:  []string{"job-1"},
			ClaudeVersion: "2.0.0",
		},
		LastHeartbeat: timestamppb.New(timestamppb.Now().AsTime()),
	})
}

// TestBankV1_UpdateBankRequestPresence checks that the optional fields of
// UpdateBankRequest keep "absent" apart from "zero" across the wire. The
// daemon reads absent as "keep the value", so a zero that arrives as absent
// would silently drop a scale-to-zero.
func TestBankV1_UpdateBankRequestPresence(t *testing.T) {
	zero := int32(0)
	policy := bankpb.SpillPolicy_SPILL_POLICY_EPHEMERAL
	req := &bankpb.UpdateBankRequest{
		Id:           "bank-1",
		DesiredCount: &zero,
		SpillPolicy:  &policy,
	}
	roundTrip(t, req)
	if req.DesiredCount == nil || req.GetDesiredCount() != 0 {
		t.Fatalf("desired_count: want present zero, got %v", req.DesiredCount)
	}
	if req.MaxJobsInFlight != nil {
		t.Fatalf("max_jobs_in_flight: want absent, got %v", req.MaxJobsInFlight)
	}
	if req.SpillPolicy == nil || req.GetSpillPolicy() != bankpb.SpillPolicy_SPILL_POLICY_EPHEMERAL {
		t.Fatalf("spill_policy: want present EPHEMERAL, got %v", req.SpillPolicy)
	}
}
