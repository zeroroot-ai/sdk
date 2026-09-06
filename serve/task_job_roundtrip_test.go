// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Zero Root AI

package serve

// task_job_roundtrip_test.go — Task.job survives the SDK <-> proto boundary.
//
// The daemon reads Task.job (repositories, credential names, World inputs,
// acceptance) BEFORE it mints the per-turn grant (gibson#1706, sdk#546). If a
// converter drops the field, the daemon mints a grant that covers nothing and
// the job fails with no obvious cause. These tests hold that line.

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"

	"github.com/zeroroot-ai/sdk/agent"
	jobpb "github.com/zeroroot-ai/sdk/api/gen/gibson/job/v1"
)

func testJobSpec() *jobpb.JobSpec {
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
		Inputs:          []string{"node-1"},
		Acceptance: &jobpb.Acceptance{
			VerifierComponent: "agent/verifier",
			PassingScore:      0.8,
			MaxPasses:         3,
		},
	}
}

// TestTaskToProto_CarriesJob checks the SDK -> proto direction.
func TestTaskToProto_CarriesJob(t *testing.T) {
	task := agent.Task{ID: "t-1", Goal: "ship it", Job: testJobSpec()}

	got := TaskToProto(task)

	require.NotNil(t, got.GetJob(), "job must survive TaskToProto")
	assert.True(t, proto.Equal(testJobSpec(), got.GetJob()), "job diverged: %v", got.GetJob())
	assert.Equal(t, "ship it", got.GetGoal(), "goal stays the goal")
}

// TestProtoToTask_CarriesJob checks the proto -> SDK direction.
func TestProtoToTask_CarriesJob(t *testing.T) {
	pt := TaskToProto(agent.Task{ID: "t-1", Job: testJobSpec()})

	got := ProtoToTask(pt)

	require.NotNil(t, got.Job, "job must survive ProtoToTask")
	assert.True(t, proto.Equal(testJobSpec(), got.Job), "job diverged: %v", got.Job)
}

// TestTaskJob_FullRoundTrip runs both directions and compares against the
// input, including every declared sub-field.
func TestTaskJob_FullRoundTrip(t *testing.T) {
	in := agent.Task{
		ID:          "t-1",
		Goal:        "ship it",
		Context:     map[string]any{"ticket": "API-42"},
		Constraints: agent.TaskConstraints{MaxTurns: 20, MaxTokens: 1000, AllowedTools: []string{"Bash"}},
		Job:         testJobSpec(),
	}

	out := ProtoToTask(TaskToProto(in))

	assert.Equal(t, in.ID, out.ID)
	assert.Equal(t, in.Goal, out.Goal)
	assert.Equal(t, in.Constraints, out.Constraints, "bounds ride on the envelope, not on the spec")
	require.NotNil(t, out.Job)
	assert.True(t, proto.Equal(in.Job, out.Job))
	assert.Equal(t, "connector/gitlab-main", out.Job.GetRepositories()[0].GetConnectorRef())
	assert.Equal(t, []string{"npm-token"}, out.Job.GetCredentialNames())
	assert.InDelta(t, 0.8, out.Job.GetAcceptance().GetPassingScore(), 0.0001)
}

// TestTaskJob_NilStaysNil keeps a task with no job free of an empty spec.
// An empty JobSpec is not the same as no job: it would declare zero
// repositories and zero credentials rather than "this task opens no job".
func TestTaskJob_NilStaysNil(t *testing.T) {
	assert.Nil(t, TaskToProto(agent.Task{ID: "t-1"}).GetJob())
	assert.Nil(t, ProtoToTask(TaskToProto(agent.Task{ID: "t-1"})).Job)
}

// TestTaskJob_NotAliased checks the converters copy the spec. A shared
// pointer would let a caller mutate a task after dispatch and change the
// message already on the wire.
func TestTaskJob_NotAliased(t *testing.T) {
	in := agent.Task{ID: "t-1", Job: testJobSpec()}

	pt := TaskToProto(in)
	in.Job.Repositories[0].ConnectorRef = "connector/mutated"

	assert.Equal(t, "connector/gitlab-main", pt.GetJob().GetRepositories()[0].GetConnectorRef(),
		"TaskToProto must copy the spec, not alias it")

	back := ProtoToTask(pt)
	pt.Job.Repositories[0].ConnectorRef = "connector/mutated-again"
	assert.Equal(t, "connector/gitlab-main", back.Job.GetRepositories()[0].GetConnectorRef(),
		"ProtoToTask must copy the spec, not alias it")
}
