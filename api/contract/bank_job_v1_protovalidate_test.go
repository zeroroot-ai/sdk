// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Zero Root AI

package contract_test

// bank_job_v1_protovalidate_test.go — runtime exercise of the buf.validate
// annotations declared in gibson/bank/v1, gibson/job/v1 and the bank member
// RPCs of gibson/harness/v1 (gibson#1706, sdk#545).
//
// Same shape as mission_v1_protovalidate_test.go: for each rule, one
// known-good input that passes and one known-bad input that is rejected
// with an error that names the field.

import (
	"strings"
	"testing"

	"google.golang.org/protobuf/proto"

	bankpb "github.com/zeroroot-ai/sdk/api/gen/gibson/bank/v1"
	harnesspb "github.com/zeroroot-ai/sdk/api/gen/gibson/harness/v1"
	jobpb "github.com/zeroroot-ai/sdk/api/gen/gibson/job/v1"
	missionpb "github.com/zeroroot-ai/sdk/api/gen/gibson/mission/v1"
)

// validCase is one protovalidate fixture: the message under test, whether it
// must pass, and the field name the error must mention when it must fail.
type validCase struct {
	name  string
	msg   proto.Message
	valid bool
	field string
}

func runValidCases(t *testing.T, cases []validCase) {
	t.Helper()
	v := newValidator(t)
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := v.Validate(tc.msg)
			if tc.valid {
				if err != nil {
					t.Fatalf("want valid, got error: %v", err)
				}
				return
			}
			if err == nil {
				t.Fatalf("want error on %q, got nil", tc.field)
			}
			if !strings.Contains(err.Error(), tc.field) {
				t.Fatalf("error does not name field %q: %v", tc.field, err)
			}
		})
	}
}

func TestBankV1_Protovalidate(t *testing.T) {
	neg := int32(-1)
	badPolicy := bankpb.SpillPolicy(99)
	runValidCases(t, []validCase{
		{"CreateBankRequest/ok", &bankpb.CreateBankRequest{Name: "reviewers", LoginShape: bankpb.LoginShape_LOGIN_SHAPE_ANTHROPIC_API_KEY, ProviderConfigName: "anthropic"}, true, ""},
		{"CreateBankRequest/name_empty", &bankpb.CreateBankRequest{LoginShape: bankpb.LoginShape_LOGIN_SHAPE_SUBSCRIPTION}, false, "name"},
		{"CreateBankRequest/login_shape_unspecified", &bankpb.CreateBankRequest{Name: "x"}, false, "login_shape"},
		{"CreateBankRequest/login_shape_undefined", &bankpb.CreateBankRequest{Name: "x", LoginShape: bankpb.LoginShape(42)}, false, "login_shape"},
		{"CreateBankRequest/desired_count_negative", &bankpb.CreateBankRequest{Name: "x", LoginShape: bankpb.LoginShape_LOGIN_SHAPE_SUBSCRIPTION, DesiredCount: -1}, false, "desired_count"},
		{"CreateBankRequest/max_jobs_negative", &bankpb.CreateBankRequest{Name: "x", LoginShape: bankpb.LoginShape_LOGIN_SHAPE_SUBSCRIPTION, MaxJobsInFlight: -1}, false, "max_jobs_in_flight"},
		{"CreateBankRequest/spill_policy_undefined", &bankpb.CreateBankRequest{Name: "x", LoginShape: bankpb.LoginShape_LOGIN_SHAPE_SUBSCRIPTION, SpillPolicy: badPolicy}, false, "spill_policy"},
		{"GetBankRequest/ok", &bankpb.GetBankRequest{Id: "bank-1"}, true, ""},
		{"GetBankRequest/id_empty", &bankpb.GetBankRequest{}, false, "id"},
		{"ListBanksRequest/ok", &bankpb.ListBanksRequest{PageSize: 10}, true, ""},
		{"ListBanksRequest/page_size_negative", &bankpb.ListBanksRequest{PageSize: -1}, false, "page_size"},
		{"UpdateBankRequest/ok_absent", &bankpb.UpdateBankRequest{Id: "bank-1"}, true, ""},
		{"UpdateBankRequest/id_empty", &bankpb.UpdateBankRequest{}, false, "id"},
		{"UpdateBankRequest/desired_count_negative", &bankpb.UpdateBankRequest{Id: "bank-1", DesiredCount: &neg}, false, "desired_count"},
		{"UpdateBankRequest/spill_policy_undefined", &bankpb.UpdateBankRequest{Id: "bank-1", SpillPolicy: &badPolicy}, false, "spill_policy"},
		{"DeleteBankRequest/id_empty", &bankpb.DeleteBankRequest{}, false, "id"},
		{"ListMembersRequest/ok", &bankpb.ListMembersRequest{BankId: "bank-1"}, true, ""},
		{"ListMembersRequest/bank_id_empty", &bankpb.ListMembersRequest{}, false, "bank_id"},
		{"StartSignInRequest/ok", &bankpb.StartSignInRequest{BankId: "bank-1", MemberId: "m-1"}, true, ""},
		{"StartSignInRequest/member_id_empty", &bankpb.StartSignInRequest{BankId: "bank-1"}, false, "member_id"},
		{"StreamSignInRequest/bank_id_empty", &bankpb.StreamSignInRequest{MemberId: "m-1"}, false, "bank_id"},
		{"SubmitSignInCodeRequest/ok", &bankpb.SubmitSignInCodeRequest{BankId: "bank-1", MemberId: "m-1", Code: "ABCD-1234"}, true, ""},
		{"SubmitSignInCodeRequest/code_empty", &bankpb.SubmitSignInCodeRequest{BankId: "bank-1", MemberId: "m-1"}, false, "code"},
		{"Bank/ok", &bankpb.Bank{DesiredCount: 2, LoginShape: bankpb.LoginShape_LOGIN_SHAPE_BEDROCK, SpillPolicy: bankpb.SpillPolicy_SPILL_POLICY_EPHEMERAL}, true, ""},
		{"Bank/login_shape_undefined", &bankpb.Bank{LoginShape: bankpb.LoginShape(42)}, false, "login_shape"},
		{"MemberStatus/ok", &bankpb.MemberStatus{State: bankpb.MemberState_MEMBER_STATE_IDLE, JobsInFlight: 0, Cap: 1}, true, ""},
		{"MemberStatus/state_undefined", &bankpb.MemberStatus{State: bankpb.MemberState(42)}, false, "state"},
		{"MemberStatus/cap_negative", &bankpb.MemberStatus{State: bankpb.MemberState_MEMBER_STATE_IDLE, Cap: -1}, false, "cap"},
	})
}

func TestJobV1_Protovalidate(t *testing.T) {
	okRepo := &jobpb.RepositorySpec{Name: "api", ConnectorRef: "connector/gitlab", Project: "group/api", Deliverable: jobpb.DeliverableKind_DELIVERABLE_KIND_PUSH_BRANCH}
	runValidCases(t, []validCase{
		{"RepositorySpec/ok", okRepo, true, ""},
		{"RepositorySpec/name_empty", &jobpb.RepositorySpec{ConnectorRef: "connector/gitlab", Project: "group/api", Deliverable: jobpb.DeliverableKind_DELIVERABLE_KIND_NONE}, false, "name"},
		{"RepositorySpec/connector_ref_empty", &jobpb.RepositorySpec{Name: "api", Project: "group/api", Deliverable: jobpb.DeliverableKind_DELIVERABLE_KIND_NONE}, false, "connector_ref"},
		{"RepositorySpec/project_empty", &jobpb.RepositorySpec{Name: "api", ConnectorRef: "connector/gitlab", Deliverable: jobpb.DeliverableKind_DELIVERABLE_KIND_NONE}, false, "project"},
		{"RepositorySpec/deliverable_unspecified", &jobpb.RepositorySpec{Name: "api", ConnectorRef: "connector/gitlab", Project: "group/api"}, false, "deliverable"},
		{"Acceptance/ok", &jobpb.Acceptance{VerifierComponent: "agent/verifier", PassingScore: 0.8, MaxPasses: 3}, true, ""},
		{"Acceptance/passing_score_above_one", &jobpb.Acceptance{PassingScore: 1.5}, false, "passing_score"},
		{"Acceptance/max_passes_negative", &jobpb.Acceptance{MaxPasses: -1}, false, "max_passes"},
		{"JobSpec/ok_goal_only", &jobpb.JobSpec{Goal: "say hello"}, true, ""},
		{"JobSpec/nested_repository_rejected", &jobpb.JobSpec{Goal: "x", Repositories: []*jobpb.RepositorySpec{{Name: "api"}}}, false, "connector_ref"},
		{"Job/ok", &jobpb.Job{State: jobpb.JobState_JOB_STATE_CLOSED, Verdict: jobpb.JobVerdict_JOB_VERDICT_ACCOMPLISHED, Score: 1.0}, true, ""},
		{"Job/score_negative", &jobpb.Job{Score: -0.1}, false, "score"},
		{"Job/state_undefined", &jobpb.Job{State: jobpb.JobState(42)}, false, "state"},
		{"Input/kind_undefined", &jobpb.Input{Kind: jobpb.InputKind(42)}, false, "kind"},
		{"OpenJobRequest/ok", &jobpb.OpenJobRequest{BankId: "bank-1", Spec: &jobpb.JobSpec{Goal: "x"}}, true, ""},
		{"OpenJobRequest/bank_id_empty", &jobpb.OpenJobRequest{Spec: &jobpb.JobSpec{Goal: "x"}}, false, "bank_id"},
		{"OpenJobRequest/spec_missing", &jobpb.OpenJobRequest{BankId: "bank-1"}, false, "spec"},
		{"SendInputRequest/ok", &jobpb.SendInputRequest{JobId: "job-1", Message: "go", Kind: jobpb.InputKind_INPUT_KIND_ANSWER}, true, ""},
		{"SendInputRequest/message_empty", &jobpb.SendInputRequest{JobId: "job-1"}, false, "message"},
		{"SendInputRequest/wrap_up_rejected", &jobpb.SendInputRequest{JobId: "job-1", Message: "go", Kind: jobpb.InputKind_INPUT_KIND_WRAP_UP}, false, "kind"},
		{"CloseJobRequest/ok", &jobpb.CloseJobRequest{JobId: "job-1", Verdict: jobpb.JobVerdict_JOB_VERDICT_FAILED, Score: 0.3}, true, ""},
		{"CloseJobRequest/verdict_unspecified", &jobpb.CloseJobRequest{JobId: "job-1"}, false, "verdict"},
		{"CloseJobRequest/score_above_one", &jobpb.CloseJobRequest{JobId: "job-1", Verdict: jobpb.JobVerdict_JOB_VERDICT_ACCOMPLISHED, Score: 2}, false, "score"},
		{"GetJobRequest/job_id_empty", &jobpb.GetJobRequest{}, false, "job_id"},
		{"ListJobsRequest/ok_no_filter", &jobpb.ListJobsRequest{}, true, ""},
		{"ListJobsRequest/state_undefined", &jobpb.ListJobsRequest{State: jobpb.JobState(42)}, false, "state"},
		{"StreamJobEventsRequest/job_id_empty", &jobpb.StreamJobEventsRequest{}, false, "job_id"},
		{"JobEvent/kind_undefined", &jobpb.JobEvent{Kind: jobpb.JobEventKind(42)}, false, "kind"},
	})
}

func TestHarnessV1_BankMemberProtovalidate(t *testing.T) {
	runValidCases(t, []validCase{
		{"ReportJobStateRequest/ok_working", &harnesspb.ReportJobStateRequest{JobId: "job-1", State: jobpb.JobState_JOB_STATE_WORKING}, true, ""},
		{"ReportJobStateRequest/ok_waiting", &harnesspb.ReportJobStateRequest{JobId: "job-1", State: jobpb.JobState_JOB_STATE_WAITING}, true, ""},
		{"ReportJobStateRequest/closed_rejected", &harnesspb.ReportJobStateRequest{JobId: "job-1", State: jobpb.JobState_JOB_STATE_CLOSED}, false, "state"},
		{"ReportJobStateRequest/job_id_empty", &harnesspb.ReportJobStateRequest{State: jobpb.JobState_JOB_STATE_WORKING}, false, "job_id"},
		{"ReportDeliverableRequest/ok", &harnesspb.ReportDeliverableRequest{JobId: "job-1", Deliverable: &jobpb.Deliverable{Kind: jobpb.DeliverableKind_DELIVERABLE_KIND_PUSH_BRANCH, Ref: "job/job-1"}}, true, ""},
		{"ReportDeliverableRequest/deliverable_missing", &harnesspb.ReportDeliverableRequest{JobId: "job-1"}, false, "deliverable"},
		{"OpenJobRequest/spec_missing", &harnesspb.OpenJobRequest{BankId: "bank-1"}, false, "spec"},
		{"OpenJobRequest/ok", &harnesspb.OpenJobRequest{BankId: "bank-1", Spec: &jobpb.JobSpec{Goal: "x"}}, true, ""},
		{"SendInputRequest/wrap_up_rejected", &harnesspb.SendInputRequest{JobId: "job-1", Message: "go", Kind: jobpb.InputKind_INPUT_KIND_WRAP_UP}, false, "kind"},
		{"CloseJobRequest/verdict_unspecified", &harnesspb.CloseJobRequest{JobId: "job-1"}, false, "verdict"},
		{"CloseJobRequest/ok", &harnesspb.CloseJobRequest{JobId: "job-1", Verdict: jobpb.JobVerdict_JOB_VERDICT_ACCOMPLISHED, Score: 1}, true, ""},
	})
}

// TestMissionV1_JobNodeProtovalidate covers the job node's own rules. A node
// with no bank and no spec must be rejected at submit time, not at run time.
func TestMissionV1_JobNodeProtovalidate(t *testing.T) {
	okSpec := &jobpb.JobSpec{Goal: "fix the build"}
	runValidCases(t, []validCase{
		{"JobNodeConfig/ok", &missionpb.JobNodeConfig{BankRef: "reviewers", Spec: okSpec}, true, ""},
		{"JobNodeConfig/bank_ref_empty", &missionpb.JobNodeConfig{Spec: okSpec}, false, "bank_ref"},
		{"JobNodeConfig/spec_missing", &missionpb.JobNodeConfig{BankRef: "reviewers"}, false, "spec"},
		{"JobNodeConfig/nested_acceptance_checked", &missionpb.JobNodeConfig{
			BankRef: "reviewers",
			Spec:    &jobpb.JobSpec{Goal: "x", Acceptance: &jobpb.Acceptance{PassingScore: 1.5}},
		}, false, "passing_score"},
	})
}
