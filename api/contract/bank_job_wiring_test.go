// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Zero Root AI

package contract_test

// bank_job_wiring_test.go — the fields that wire banks and jobs into the
// existing surfaces (gibson#1706, sdk#546, sdk#547, sdk#548):
//
//   - gibson.types.v1.Task.job
//   - gibson.agent.v1.ExecuteRequest.job_id, ExecuteResponse deliverables
//   - gibson.mission.v1 NODE_TYPE_JOB and JobNodeConfig
//   - gibson.harness.v1.DelegateToAgentRequest.target
//   - gibson.component.v1.HeartbeatRequest.member

import (
	"testing"

	"google.golang.org/protobuf/proto"

	agentpb "github.com/zeroroot-ai/sdk/api/gen/gibson/agent/v1"
	bankpb "github.com/zeroroot-ai/sdk/api/gen/gibson/bank/v1"
	componentpb "github.com/zeroroot-ai/sdk/api/gen/gibson/component/v1"
	harnesspb "github.com/zeroroot-ai/sdk/api/gen/gibson/harness/v1"
	jobpb "github.com/zeroroot-ai/sdk/api/gen/gibson/job/v1"
	missionpb "github.com/zeroroot-ai/sdk/api/gen/gibson/mission/v1"
	typespb "github.com/zeroroot-ai/sdk/api/gen/gibson/types/v1"
)

func wiringSpec() *jobpb.JobSpec {
	return &jobpb.JobSpec{
		Goal: "fix the build",
		Repositories: []*jobpb.RepositorySpec{{
			Name:         "api",
			ConnectorRef: "connector/gitlab",
			Project:      "group/api",
			Deliverable:  jobpb.DeliverableKind_DELIVERABLE_KIND_MERGE_REQUEST,
		}},
		CredentialNames: []string{"npm-token"},
		Acceptance:      &jobpb.Acceptance{VerifierComponent: "agent/verifier", PassingScore: 0.8, MaxPasses: 2},
	}
}

// TestTypesV1_TaskCarriesJobSpec round-trips a Task with a job and its own
// constraints. The bounds ride on the envelope; the spec carries none.
func TestTypesV1_TaskCarriesJobSpec(t *testing.T) {
	task := &typespb.Task{
		Id:          "task-1",
		Goal:        "fix the build",
		Constraints: &typespb.TaskConstraints{MaxTurns: 10, MaxTokens: 5000},
		Job:         wiringSpec(),
	}
	roundTrip(t, task)
	if task.GetJob().GetRepositories()[0].GetConnectorRef() != "connector/gitlab" {
		t.Fatalf("connector_ref lost: %v", task.GetJob())
	}
}

// TestAgentV1_ExecuteCarriesJobResults round-trips the one-shot and bank-job
// result path: a job id in, deliverables and the session id out.
func TestAgentV1_ExecuteCarriesJobResults(t *testing.T) {
	roundTrip(t, &agentpb.ExecuteRequest{
		Task:  &typespb.Task{Id: "task-1", Job: wiringSpec()},
		JobId: "job-1",
	})
	roundTrip(t, &agentpb.ExecuteResponse{
		Result: &typespb.Result{Status: typespb.ResultStatus_RESULT_STATUS_SUCCESS},
		Deliverables: []*jobpb.Deliverable{{
			Kind: jobpb.DeliverableKind_DELIVERABLE_KIND_MERGE_REQUEST,
			Ref:  "!12",
			Url:  "https://gitlab.example/group/api/-/merge_requests/12",
		}},
		ClaudeSessionId: "sess-1",
	})
}

// TestMissionV1_JobNode round-trips a job node and checks NODE_TYPE_JOB is a
// distinct enum value that did not displace an existing one.
func TestMissionV1_JobNode(t *testing.T) {
	if got := missionpb.NodeType_NODE_TYPE_JOB; got != 7 {
		t.Fatalf("NODE_TYPE_JOB = %d, want 7 (append-only enum)", got)
	}
	if missionpb.NodeType_NODE_TYPE_JOIN != 6 {
		t.Fatal("NODE_TYPE_JOIN moved; the enum must stay append-only")
	}

	node := &missionpb.MissionNode{
		Id:   "verify-and-fix",
		Type: missionpb.NodeType_NODE_TYPE_JOB,
		Name: "Fix the build",
		Config: &missionpb.MissionNode_JobConfig{
			JobConfig: &missionpb.JobNodeConfig{
				BankRef:     "reviewers",
				Spec:        wiringSpec(),
				Constraints: &typespb.TaskConstraints{MaxTurns: 20},
			},
		},
		Dependencies: []string{"build"},
	}
	roundTrip(t, node)

	cfg, ok := node.GetConfig().(*missionpb.MissionNode_JobConfig)
	if !ok {
		t.Fatalf("config oneof lost the job node: %T", node.GetConfig())
	}
	if cfg.JobConfig.GetSpec().GetAcceptance().GetMaxPasses() != 2 {
		t.Fatal("the verify loop bound must come from spec.acceptance.max_passes")
	}
}

// TestHarnessV1_DelegateTarget round-trips each arm of the dispatch target
// selector. An absent selector is today's ephemeral launch.
func TestHarnessV1_DelegateTarget(t *testing.T) {
	cases := map[string]*harnesspb.DelegateToAgentRequest{
		"absent":    {Name: "worker", Task: &typespb.Task{Id: "t-1"}},
		"ephemeral": {Name: "worker", Target: &harnesspb.DelegateToAgentRequest_Ephemeral{Ephemeral: true}},
		"bank":      {Name: "worker", Target: &harnesspb.DelegateToAgentRequest_BankId{BankId: "bank-1"}},
		"job":       {Name: "worker", Target: &harnesspb.DelegateToAgentRequest_JobId{JobId: "job-1"}},
	}
	for name, req := range cases {
		t.Run(name, func(t *testing.T) { roundTrip(t, req) })
	}
	if cases["absent"].GetTarget() != nil {
		t.Fatal("an absent selector must stay absent: it means ephemeral, today's behavior")
	}

	roundTrip(t, &harnesspb.DelegateToAgentResponse{
		Result:   &typespb.Result{Status: typespb.ResultStatus_RESULT_STATUS_SUCCESS},
		JobId:    "job-1",
		MemberId: "member-1",
	})
}

// TestComponentV1_HeartbeatCarriesMemberStatus round-trips a heartbeat with
// the member status the daemon routes queued jobs on. health_status stays: it
// reports whether the process is well, not whether it is busy.
func TestComponentV1_HeartbeatCarriesMemberStatus(t *testing.T) {
	hb := &componentpb.HeartbeatRequest{
		InstanceId:   "inst-1",
		HealthStatus: "healthy",
		Member: &bankpb.MemberStatus{
			State:         bankpb.MemberState_MEMBER_STATE_BUSY,
			JobsInFlight:  1,
			Cap:           1,
			ActiveJobIds:  []string{"job-1"},
			ClaudeVersion: "2.0.0",
		},
	}
	roundTrip(t, hb)

	if hb.GetHealthStatus() != "healthy" || hb.GetMember().GetState() != bankpb.MemberState_MEMBER_STATE_BUSY {
		t.Fatal("health and member status must both survive")
	}

	plain := &componentpb.HeartbeatRequest{InstanceId: "inst-2", HealthStatus: "healthy"}
	roundTrip(t, plain)
	if plain.GetMember() != nil {
		t.Fatal("a component that is not a bank member must report no member status")
	}
	if !proto.Equal(plain, &componentpb.HeartbeatRequest{InstanceId: "inst-2", HealthStatus: "healthy"}) {
		t.Fatal("the existing heartbeat shape changed")
	}
}
