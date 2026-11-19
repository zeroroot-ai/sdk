// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Zero Root AI

package agent

import (
	"encoding/json"
	"errors"
	"testing"

	"github.com/a2aproject/a2a-go/v2/a2a"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"

	jobpb "github.com/zeroroot-ai/sdk/api/gen/gibson/job/v1"
)

func testJob() *jobpb.JobSpec {
	return &jobpb.JobSpec{
		Goal: "Add a health endpoint",
		Repositories: []*jobpb.RepositorySpec{{
			Name: "api", ConnectorRef: "connector/gitlab-main", Project: "group/api", BaseBranch: "main",
			Deliverable: jobpb.DeliverableKind_DELIVERABLE_KIND_MERGE_REQUEST,
		}},
		CredentialNames: []string{"npm-token"},
	}
}

// wireMessage builds the message that the daemon sends, and sends it through
// JSON, as the work item does.
func wireMessage(t *testing.T) *a2a.Message {
	t.Helper()
	job, err := protojson.Marshal(testJob())
	require.NoError(t, err)
	msg := a2a.NewMessage(a2a.MessageRoleUser, a2a.NewTextPart("scan the host"), a2a.NewTextPart("be quick"))
	msg.TaskID = "task-1"
	msg.ContextID = "ctx-1"
	msg.SetMeta(ExtensionKey, map[string]any{
		"mission_run_id": "mr-1", "agent_run_id": "ar-1", "run_number": 3, "job_id": "job-9",
		"trace_id": "tr", "parent_span_id": "sp",
		"pinned_skills": []string{"skill:review", "skill:triage"},
		"constraints":   map[string]any{"max_turns": 5, "max_tokens": 900, "allowed_tools": []string{"Bash"}},
		"context":       map[string]any{"ticket": "API-42"},
		"metadata":      map[string]any{"priority": "high"},
		"job":           json.RawMessage(job),
	})
	raw, err := json.Marshal(msg)
	require.NoError(t, err)
	var back a2a.Message
	require.NoError(t, json.Unmarshal(raw, &back))
	return &back
}

func TestTaskFromMessage(t *testing.T) {
	task, err := TaskFromMessage(wireMessage(t))
	require.NoError(t, err)

	assert.Equal(t, "task-1", task.ID)
	assert.Equal(t, "ctx-1", task.ContextID)
	assert.Equal(t, "scan the host\nbe quick", task.Goal)
	assert.Equal(t, []string{"skill:review", "skill:triage"}, task.PinnedSkills)
	assert.Equal(t, RunContext{
		MissionRunID: "mr-1", AgentRunID: "ar-1", RunNumber: 3, JobID: "job-9", TraceID: "tr", ParentSpanID: "sp",
	}, task.Run)
	assert.Equal(t, TaskConstraints{MaxTurns: 5, MaxTokens: 900, AllowedTools: []string{"Bash"}}, task.Constraints)
	assert.Equal(t, "API-42", task.Context["ticket"])
	assert.Equal(t, "high", task.Metadata["priority"])
	require.NotNil(t, task.Job)
	assert.True(t, proto.Equal(testJob(), task.Job), "job diverged: %v", task.Job)
	require.NotNil(t, task.Message)
}

func TestTaskFromMessageWithoutEnvelope(t *testing.T) {
	msg := a2a.NewMessage(a2a.MessageRoleUser, a2a.NewTextPart("hi"))
	msg.TaskID, msg.ContextID = "t", "c"
	task, err := TaskFromMessage(msg)
	require.NoError(t, err)
	assert.Equal(t, "hi", task.Goal)
	assert.Nil(t, task.Job, "a task with no job keeps no empty spec")
	assert.Empty(t, task.PinnedSkills)
}

func TestTaskFromMessageRefusals(t *testing.T) {
	_, err := TaskFromMessage(nil)
	require.Error(t, err)
	_, err = TaskFromMessage(a2a.NewMessage(a2a.MessageRoleUser, a2a.NewTextPart("x")))
	require.ErrorContains(t, err, "taskId")
	m := a2a.NewMessage(a2a.MessageRoleUser)
	m.TaskID = "t"
	_, err = TaskFromMessage(m)
	require.ErrorContains(t, err, "contextId")

	bad := a2a.NewMessage(a2a.MessageRoleUser)
	bad.TaskID, bad.ContextID = "t", "c"
	bad.SetMeta(ExtensionKey, map[string]any{"job": map[string]any{"nope": 1}})
	_, err = TaskFromMessage(bad)
	require.ErrorContains(t, err, "job")
}

func TestResultToTaskCompleted(t *testing.T) {
	task, err := TaskFromMessage(wireMessage(t))
	require.NoError(t, err)

	got, err := ResultToTask(task, Result{
		Status:   a2a.TaskStateCompleted,
		Output:   map[string]any{"open_ports": []int{22, 443}},
		Findings: []string{"f1"},
		Metadata: map[string]any{"took": "3s"},
		Artifacts: []*a2a.Artifact{
			{ID: "a1", Name: "report", Parts: a2a.ContentParts{a2a.NewTextPart("report")}},
		},
		History: []*a2a.Message{a2a.NewMessage(a2a.MessageRoleAgent, a2a.NewTextPart("working on it"))},
	})
	require.NoError(t, err)

	assert.Equal(t, a2a.TaskID("task-1"), got.ID)
	assert.Equal(t, "ctx-1", got.ContextID)
	assert.Equal(t, a2a.TaskStateCompleted, got.Status.State)
	require.Len(t, got.History, 2, "the message that started the run comes first")
	assert.Equal(t, "scan the host\nbe quick", messageText(got.History[0]))
	require.Len(t, got.Artifacts, 2, "the artifacts of the agent come before the output")
	assert.Equal(t, a2a.ArtifactID("a1"), got.Artifacts[0].ID)
	assert.Equal(t, "output", got.Artifacts[1].Name)

	// The Task goes over the wire as A2A JSON and comes back whole.
	raw, err := json.Marshal(got)
	require.NoError(t, err)
	var back a2a.Task
	require.NoError(t, json.Unmarshal(raw, &back))
	assert.Equal(t, a2a.TaskStateCompleted, back.Status.State)
	assert.Contains(t, string(raw), "TASK_STATE_COMPLETED")
	env, err := json.Marshal(back.Metadata[ExtensionKey])
	require.NoError(t, err)
	assert.JSONEq(t, `{"finding_ids":["f1"],"metadata":{"took":"3s"}}`, string(env))
}

func TestResultToTaskStates(t *testing.T) {
	task := Task{ID: "t", ContextID: "c"}
	cases := []struct {
		name string
		in   Result
		want a2a.TaskState
	}{
		{"empty status, no error", Result{}, a2a.TaskStateCompleted},
		{"empty status, error", Result{Error: errors.New("boom")}, a2a.TaskStateFailed},
		{"canceled", Result{Status: a2a.TaskStateCanceled}, a2a.TaskStateCanceled},
		{"rejected", Result{Status: a2a.TaskStateRejected}, a2a.TaskStateRejected},
		{"input required", Result{Status: a2a.TaskStateInputRequired, Output: "which host?"}, a2a.TaskStateInputRequired},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := ResultToTask(task, tc.in)
			require.NoError(t, err)
			assert.Equal(t, tc.want, got.Status.State)
		})
	}
}

func TestResultToTaskFailureAndInputRequired(t *testing.T) {
	got, err := ResultToTask(Task{ID: "t", ContextID: "c"}, Result{
		Status:    a2a.TaskStateFailed,
		ErrorInfo: &ResultError{Code: "EXECUTION_FAILED", Message: "probe failed", Retryable: true},
	})
	require.NoError(t, err)
	require.NotNil(t, got.Status.Message)
	assert.Contains(t, messageText(got.Status.Message), "probe failed")
	raw, err := json.Marshal(got.Metadata[ExtensionKey])
	require.NoError(t, err)
	assert.Contains(t, string(raw), `"retryable":true`)

	got, err = ResultToTask(Task{ID: "t", ContextID: "c"}, Result{Status: a2a.TaskStateInputRequired, Output: "which host?"})
	require.NoError(t, err)
	assert.Empty(t, got.Artifacts, "the prompt is the status message, not an artifact")
	assert.Equal(t, "which host?", messageText(got.Status.Message))
}

func TestOutputParts(t *testing.T) {
	p, err := outputPart("text")
	require.NoError(t, err)
	assert.Equal(t, "text", p.Text())
	p, err = outputPart([]byte{1, 2})
	require.NoError(t, err)
	assert.Equal(t, []byte{1, 2}, p.Raw())
	_, err = outputPart(make(chan int))
	require.Error(t, err)
}

type cardAgent struct{ Agent }

func (cardAgent) CardSkills() []CardSkill {
	return []CardSkill{{ID: "scan", Name: "Scan", Description: "scans", Tags: []string{"net"}}}
}
func (cardAgent) DefaultInputModes() []string  { return []string{"text/plain"} }
func (cardAgent) DefaultOutputModes() []string { return nil }

type plainCardAgent struct{ Agent }

func (plainCardAgent) Name() string             { return "scanner" }
func (plainCardAgent) Description() string      { return "scans hosts" }
func (plainCardAgent) Capabilities() []string   { return []string{"port-scan", "dns"} }
func (plainCardAgent) TargetTypes() []string    { return []string{"host"} }
func (plainCardAgent) TechniqueTypes() []string { return []string{"T1046"} }

func TestCardOf(t *testing.T) {
	skills, in, out := CardOf(cardAgent{})
	assert.Equal(t, "scan", skills[0].ID)
	assert.Equal(t, []string{"text/plain"}, in)
	assert.Equal(t, DefaultCardModes, out, "empty modes fall back to the defaults")

	skills, in, out = CardOf(plainCardAgent{})
	require.Len(t, skills, 2, "one card skill for each capability")
	assert.Equal(t, "port-scan", skills[0].ID)
	assert.Equal(t, []string{"host", "T1046"}, skills[0].Tags)
	assert.Equal(t, "scans hosts", skills[1].Description)
	assert.Equal(t, DefaultCardModes, in)
	assert.Equal(t, DefaultCardModes, out)
}

func TestResultToTaskCarriesDeliverablesAndSession(t *testing.T) {
	got, err := ResultToTask(Task{ID: "t", ContextID: "c"}, Result{
		Deliverables: []*jobpb.Deliverable{{
			Kind: jobpb.DeliverableKind_DELIVERABLE_KIND_MERGE_REQUEST, Ref: "!12", Url: "https://gitlab.example/mr/12",
		}},
		ClaudeSessionID: "sess-1",
	})
	require.NoError(t, err)
	raw, err := json.Marshal(got.Metadata[ExtensionKey])
	require.NoError(t, err)
	var env ResultEnvelope
	require.NoError(t, json.Unmarshal(raw, &env))
	assert.Equal(t, "sess-1", env.ClaudeSessionID)
	require.Len(t, env.Deliverables, 1)
	var d jobpb.Deliverable
	require.NoError(t, protojson.Unmarshal(env.Deliverables[0], &d))
	assert.Equal(t, "!12", d.GetRef())
	assert.Equal(t, jobpb.DeliverableKind_DELIVERABLE_KIND_MERGE_REQUEST, d.GetKind())

	// A run with none of them has no extension key.
	plain, err := ResultToTask(Task{ID: "t", ContextID: "c"}, Result{})
	require.NoError(t, err)
	assert.NotContains(t, plain.Metadata, ExtensionKey)
}
