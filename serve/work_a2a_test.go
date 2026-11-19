// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Zero Root AI

package serve

import (
	"context"
	"encoding/json"
	"sync"
	"testing"

	"github.com/a2aproject/a2a-go/v2/a2a"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"

	"github.com/zeroroot-ai/sdk/agent"
	componentpb "github.com/zeroroot-ai/sdk/api/gen/gibson/component/v1"
)

// workServer records the results and the tool calls of an agent in pull mode.
type workServer struct {
	componentpb.UnimplementedComponentServiceServer
	mu        sync.Mutex
	submitted []*componentpb.SubmitResultRequest
	toolCalls []*componentpb.CallToolRequest
	failTool  string
}

func (s *workServer) SubmitResult(
	_ context.Context, req *componentpb.SubmitResultRequest,
) (*componentpb.SubmitResultResponse, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.submitted = append(s.submitted, req)
	return &componentpb.SubmitResultResponse{}, nil
}

func (s *workServer) CallTool(
	_ context.Context, req *componentpb.CallToolRequest,
) (*componentpb.CallToolResponse, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.toolCalls = append(s.toolCalls, req)
	if s.failTool != "" && json.Valid([]byte(req.GetInputJson())) {
		var in map[string]string
		_ = json.Unmarshal([]byte(req.GetInputJson()), &in)
		if in["id"] == s.failTool {
			return &componentpb.CallToolResponse{Error: &componentpb.ComponentError{Code: "NOT_FOUND", Message: "no such skill"}}, nil
		}
	}
	return &componentpb.CallToolResponse{OutputJson: `{"body":"skill text"}`}, nil
}

type workAgent struct {
	agent.Agent
	got  agent.Task
	exec func(agent.Task) (agent.Result, error)
}

func (a *workAgent) Name() string             { return "work-agent" }
func (a *workAgent) Description() string      { return "works" }
func (a *workAgent) Capabilities() []string   { return []string{"work"} }
func (a *workAgent) TargetTypes() []string    { return nil }
func (a *workAgent) TechniqueTypes() []string { return nil }
func (a *workAgent) Execute(_ context.Context, _ agent.Harness, t agent.Task) (agent.Result, error) {
	a.got = t
	return a.exec(t)
}

func startWorkServer(t *testing.T, srv *workServer) *PlatformClient {
	t.Helper()
	h, cleanup := setupParityTestWith(t, func(s *grpc.Server) { componentpb.RegisterComponentServiceServer(s, srv) })
	t.Cleanup(cleanup)
	return h.client
}

func workItem(t *testing.T, ext map[string]any) *WorkItem {
	t.Helper()
	msg := a2a.NewMessage(a2a.MessageRoleUser, a2a.NewTextPart("scan the host"))
	msg.TaskID, msg.ContextID = "task-1", "ctx-1"
	if ext != nil {
		msg.SetMeta(agent.ExtensionKey, ext)
	}
	raw, err := json.Marshal(msg)
	require.NoError(t, err)
	return &WorkItem{WorkID: "work-1", WorkType: "execute", Payload: raw}
}

func TestWorkItemIsAnA2AMessageAndTheResultIsAnA2ATask(t *testing.T) {
	srv := &workServer{}
	pc := startWorkServer(t, srv)
	a := &workAgent{exec: func(agent.Task) (agent.Result, error) {
		return agent.Result{Status: a2a.TaskStateInputRequired, Output: "which port?"}, nil
	}}

	executePlatformAgentWork(context.Background(), pc, a, workItem(t, map[string]any{
		"pinned_skills": []string{"skill:review"}, "agent_run_id": "ar-1",
	}))

	assert.Equal(t, "scan the host", a.got.Goal)
	assert.Equal(t, "ar-1", a.got.Run.AgentRunID)
	require.Len(t, a.got.SkillItems, 2, "a function_call and a function_call_output")

	require.Len(t, srv.toolCalls, 1)
	assert.Equal(t, "invoke_tool", srv.toolCalls[0].GetToolName())
	assert.JSONEq(t, `{"id":"skill:review"}`, srv.toolCalls[0].GetInputJson())

	require.Len(t, srv.submitted, 1)
	assert.Nil(t, srv.submitted[0].GetError())
	var task a2a.Task
	require.NoError(t, json.Unmarshal(srv.submitted[0].GetResult(), &task))
	assert.Equal(t, a2a.TaskID("task-1"), task.ID)
	assert.Equal(t, "ctx-1", task.ContextID)
	assert.Equal(t, a2a.TaskStateInputRequired, task.Status.State)
}

func TestPinnedSkillLoadFailureStopsTheRun(t *testing.T) {
	srv := &workServer{failTool: "skill:gone"}
	pc := startWorkServer(t, srv)
	ran := false
	a := &workAgent{exec: func(agent.Task) (agent.Result, error) { ran = true; return agent.Result{}, nil }}

	executePlatformAgentWork(context.Background(), pc, a, workItem(t, map[string]any{
		"pinned_skills": []string{"skill:gone"},
	}))

	assert.False(t, ran, "the agent must not start")
	require.Len(t, srv.submitted, 1)
	assert.Equal(t, "PINNED_SKILL_LOAD_FAILED", srv.submitted[0].GetError().GetCode())
	assert.Contains(t, srv.submitted[0].GetError().GetMessage(), "skill:gone")
}

func TestBadPayloadIsRefused(t *testing.T) {
	srv := &workServer{}
	pc := startWorkServer(t, srv)
	a := &workAgent{exec: func(agent.Task) (agent.Result, error) { return agent.Result{}, nil }}

	executePlatformAgentWork(context.Background(), pc, a, &WorkItem{WorkID: "w", Payload: []byte("not json")})
	executePlatformAgentWork(context.Background(), pc, a, &WorkItem{WorkID: "w2", Payload: []byte(`{"messageId":"m","role":"ROLE_USER","parts":[]}`)})

	require.Len(t, srv.submitted, 2)
	for _, s := range srv.submitted {
		assert.Equal(t, "DESERIALIZATION_ERROR", s.GetError().GetCode())
	}
}

func TestRegisterSendsTheCard(t *testing.T) {
	var got *componentpb.RegisterComponentRequest
	h, cleanup := setupParityTestWith(t, func(s *grpc.Server) {
		componentpb.RegisterComponentServiceServer(s, &registerServer{on: func(r *componentpb.RegisterComponentRequest) { got = r }})
	})
	defer cleanup()
	skills, in, out := agent.CardOf(&workAgent{})
	_, err := h.client.Register(context.Background(), ComponentInfo{
		Kind: "agent", Name: "work-agent", CardSkills: skills, DefaultInputModes: in, DefaultOutputModes: out,
	})
	require.NoError(t, err)
	require.NotNil(t, got)
	require.Len(t, got.GetCardSkills(), 1)
	assert.Equal(t, []string{"text/plain", "application/json"}, got.GetDefaultInputModes())
	assert.Equal(t, []string{"text/plain", "application/json"}, got.GetDefaultOutputModes())
}

type registerServer struct {
	componentpb.UnimplementedComponentServiceServer
	on func(*componentpb.RegisterComponentRequest)
}

func (s *registerServer) RegisterComponent(
	_ context.Context, r *componentpb.RegisterComponentRequest,
) (*componentpb.RegisterComponentResponse, error) {
	s.on(r)
	return &componentpb.RegisterComponentResponse{InstanceId: "i-1"}, nil
}
