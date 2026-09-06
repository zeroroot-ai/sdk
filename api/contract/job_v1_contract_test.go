// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Zero Root AI

package contract_test

import (
	"testing"

	"google.golang.org/protobuf/types/known/timestamppb"

	commonpb "github.com/zeroroot-ai/sdk/api/gen/gibson/common/v1"
	jobpb "github.com/zeroroot-ai/sdk/api/gen/gibson/job/v1"
)

func TestJobV1_AllMessagesRoundTrip(t *testing.T) {
	roundTripPackage(t, "gibson.job.v1")
}

// fullJobSpec returns a JobSpec with every field set. Shared by the job and
// harness populated-fixture tests.
func fullJobSpec() *jobpb.JobSpec {
	return &jobpb.JobSpec{
		Goal: "Add a health endpoint",
		Repositories: []*jobpb.RepositorySpec{{
			Name:         "api",
			ConnectorRef: "connector/gitlab-main",
			Project:      "group/api",
			BaseBranch:   "main",
			Deliverable:  jobpb.DeliverableKind_DELIVERABLE_KIND_MERGE_REQUEST,
		}},
		CredentialNames: []string{"npm-token"},
		Inputs:          []string{"node-1", "node-2"},
		Acceptance: &jobpb.Acceptance{
			VerifierComponent: "agent/verifier",
			PassingScore:      0.8,
			MaxPasses:         3,
		},
		Context: map[string]*commonpb.TypedValue{
			"ticket": {Kind: &commonpb.TypedValue_StringValue{StringValue: "API-42"}},
		},
	}
}

// TestJobV1_PopulatedJobRoundTrip round-trips a closed Job with every field
// set, including the nested spec, principal, timestamps and deliverables.
func TestJobV1_PopulatedJobRoundTrip(t *testing.T) {
	now := timestamppb.Now()
	roundTrip(t, &jobpb.Job{
		Id:              "job-1",
		BankId:          "bank-1",
		MemberId:        "member-1",
		State:           jobpb.JobState_JOB_STATE_CLOSED,
		Spec:            fullJobSpec(),
		ClaudeSessionId: "sess-1",
		OpenedBy: &commonpb.Principal{
			Kind: commonpb.Principal_KIND_COMPONENT,
			Id:   "agent-principal-1",
		},
		OpenedAt:    now,
		LastInputAt: now,
		ClosedAt:    now,
		Verdict:     jobpb.JobVerdict_JOB_VERDICT_ACCOMPLISHED,
		Score:       0.95,
		Deliverables: []*jobpb.Deliverable{{
			Kind: jobpb.DeliverableKind_DELIVERABLE_KIND_MERGE_REQUEST,
			Ref:  "!12",
			Url:  "https://gitlab.example/group/api/-/merge_requests/12",
		}},
		Attempts: 2,
	})
}

// TestJobV1_PopulatedInputRoundTrip round-trips an Input with its grant, the
// shape SubscribeInput delivers to a member.
func TestJobV1_PopulatedInputRoundTrip(t *testing.T) {
	roundTrip(t, &jobpb.Input{
		Id:      "input-1",
		JobId:   "job-1",
		Message: "Fix the failing test",
		Sender: &commonpb.Principal{
			Kind: commonpb.Principal_KIND_USER,
			Id:   "user-1",
		},
		Grant:  "eyJhbGciOi.example.grant",
		SentAt: timestamppb.Now(),
		Kind:   jobpb.InputKind_INPUT_KIND_ANSWER,
	})
}

// TestJobV1_PopulatedJobEventRoundTrip round-trips one event of each kind
// that carries a payload.
func TestJobV1_PopulatedJobEventRoundTrip(t *testing.T) {
	now := timestamppb.Now()
	events := []*jobpb.JobEvent{
		{Seq: 1, OccurredAt: now, Kind: jobpb.JobEventKind_JOB_EVENT_KIND_OPENED, JobId: "job-1", State: jobpb.JobState_JOB_STATE_OPEN},
		{Seq: 2, OccurredAt: now, Kind: jobpb.JobEventKind_JOB_EVENT_KIND_INPUT, JobId: "job-1", Input: &jobpb.Input{Id: "input-1", JobId: "job-1", Message: "go", Kind: jobpb.InputKind_INPUT_KIND_TURN}},
		{Seq: 3, OccurredAt: now, Kind: jobpb.JobEventKind_JOB_EVENT_KIND_DELIVERABLE, JobId: "job-1", Deliverable: &jobpb.Deliverable{Kind: jobpb.DeliverableKind_DELIVERABLE_KIND_PUSH_BRANCH, Ref: "job/job-1"}},
		{Seq: 4, OccurredAt: now, Kind: jobpb.JobEventKind_JOB_EVENT_KIND_CLOSED, JobId: "job-1", State: jobpb.JobState_JOB_STATE_CLOSED, Verdict: jobpb.JobVerdict_JOB_VERDICT_FAILED, Score: 0.2, Message: "verifier rejected pass 3"},
	}
	for _, ev := range events {
		t.Run(ev.GetKind().String(), func(t *testing.T) {
			roundTrip(t, &jobpb.StreamJobEventsResponse{Event: ev})
		})
	}
}
