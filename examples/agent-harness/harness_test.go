// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Zero Root AI

package main

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"log/slog"
	"math/big"
	"net"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/metadata"

	"github.com/zeroroot-ai/sdk/agent"
	componentpb "github.com/zeroroot-ai/sdk/api/gen/gibson/component/v1"
	harnesspb "github.com/zeroroot-ai/sdk/api/gen/gibson/harness/v1"
	"github.com/zeroroot-ai/sdk/codegen/workspace"
	"github.com/zeroroot-ai/sdk/fork"
	"github.com/zeroroot-ai/sdk/harness"
	"github.com/zeroroot-ai/sdk/llm"
	"github.com/zeroroot-ai/sdk/mission"
	"github.com/zeroroot-ai/sdk/plugin"
	"github.com/zeroroot-ai/sdk/serve"
	"github.com/zeroroot-ai/sdk/tool"
	"github.com/zeroroot-ai/sdk/types"
)

var taskParamsFixture = serve.TaskContextParams{
	TaskID: "task-1", AgentName: agentName, MissionID: "m-1", TraceID: "trace-1", SpanID: "span-1",
	MissionRunID: "run-1", AgentRunID: "arun-1", RunNumber: 2, ToolExecutionID: "tex-1",
}

var goalTask = agent.Task{ID: "task-1", Goal: "scan the host"}

func reportOf(t *testing.T, res agent.Result) *Report {
	t.Helper()
	rep, ok := res.Output.(*Report)
	require.True(t, ok, "output is %T", res.Output)
	require.Equal(t, agent.StatusSuccess, res.Status)
	return rep
}

func value[T any](t *testing.T, rep *Report, call string) T {
	t.Helper()
	s, ok := rep.Step(call)
	require.True(t, ok, "no step %s", call)
	require.NoError(t, s.Err, "step %s", call)
	v, ok := s.Value.(T)
	require.True(t, ok, "value of %s is %T", call, s.Value)
	return v
}

func TestSlotsAreDeclared(t *testing.T) {
	a, err := New()
	require.NoError(t, err)
	require.Equal(t, []string{"main required=true window=8000 features=tools models=claude"}, SlotSummary(a))
	require.Nil(t, SlotSummary(plainAgent{}))
	require.NoError(t, Shutdown(context.Background(), a))
	require.NoError(t, Shutdown(context.Background(), plainAgent{}))
}

type plainAgent struct{ agent.Agent }

// ---------------------------------------------------------------------------
// serve.CallbackHarness against the fake HarnessCallbackService
// ---------------------------------------------------------------------------

func runCallback(t *testing.T) (*callbackDaemon, *Report) {
	t.Helper()
	d, addr := startCallbackDaemon(t)
	ctx := context.Background()
	client, h, err := Dial(ctx, CallbackOptions{
		Endpoint: addr, Token: "grant-1",
		Mission: types.MissionContext{ID: "m-1", Name: "Mission One"},
	})
	require.NoError(t, err)
	t.Cleanup(func() { _ = client.Close() })

	a, err := New()
	require.NoError(t, err)
	res, err := RunTask(ctx, a, client, h, taskParamsFixture, goalTask)
	require.NoError(t, err)
	return d, reportOf(t, res)
}

func TestCallbackHarness_EveryCallReachesTheDaemon(t *testing.T) {
	d, rep := runCallback(t)

	for _, s := range rep.Steps {
		require.NoError(t, s.Err, "step %s", s.Call)
	}

	t.Run("llm", func(t *testing.T) {
		req := only[*harnesspb.LLMCompleteRequest](t, &d.recorder, "LLMComplete")
		assert.Equal(t, "main", req.GetSlot())
		assert.Equal(t, "scan the host", req.GetMessages()[0].GetContent())
		assert.Equal(t, "user", req.GetMessages()[0].GetRole())
		assert.InDelta(t, 0.2, req.GetTemperature(), 0.001)
		assert.Equal(t, int32(256), req.GetMaxTokens())
		c := value[*llm.CompletionResponse](t, rep, "Complete")
		assert.Equal(t, "hello", c.Content)
		assert.Equal(t, "stop", c.FinishReason)
		assert.Equal(t, 18, c.Usage.TotalTokens)

		wt := only[*harnesspb.LLMCompleteWithToolsRequest](t, &d.recorder, "LLMCompleteWithTools")
		assert.Equal(t, "lookup", wt.GetTools()[0].GetName())
		ct := value[*llm.CompletionResponse](t, rep, "CompleteWithTools")
		assert.Equal(t, "tc-1", ct.ToolCalls[0].ID)
		assert.JSONEq(t, `{"host":"10.0.0.1"}`, ct.ToolCalls[0].Arguments)

		structured := d.named("LLMCompleteStructured")
		require.Len(t, structured, 2, "CompleteStructured and CompleteStructuredAny")
		assert.Contains(t, structured[0].req.(*harnesspb.LLMCompleteStructuredRequest).GetSchemaJson(), "safe")
		assert.Equal(t, map[string]any{"safe": true}, value[any](t, rep, "CompleteStructured"))
		assert.Equal(t, map[string]any{"safe": true}, value[any](t, rep, "CompleteStructuredAny"))

		st := only[*harnesspb.LLMStreamRequest](t, &d.recorder, "LLMStream")
		assert.Equal(t, "main", st.GetSlot())
		s := value[streamed](t, rep, "Stream")
		assert.Equal(t, streamed{Text: "hello", ToolCalls: 1, Finish: "stop", Usage: llm.TokenUsage{
			InputTokens: 11, OutputTokens: 7, TotalTokens: 18}}, s)
	})

	t.Run("tools", func(t *testing.T) {
		req := only[*harnesspb.CallToolProtoRequest](t, &d.recorder, "CallToolProto")
		assert.Equal(t, "echo", req.GetName())
		assert.JSONEq(t, `"ping"`, string(req.GetInputJson()))
		assert.Equal(t, "google.protobuf.StringValue", req.GetInputType())
		assert.Equal(t, "pong", value[string](t, rep, "CallToolProto"))

		sreq := only[*harnesspb.CallToolProtoStreamRequest](t, &d.recorder, "CallToolProtoStream")
		assert.JSONEq(t, `"stream"`, string(sreq.GetInputJson()))
		ev := value[*toolEvents](t, rep, "CallToolProtoStream")
		assert.Equal(t, []string{"50:scan:halfway"}, ev.Progress)
		assert.Equal(t, 1, ev.Partials)
		assert.Equal(t, []string{"slow:W1"}, ev.Warnings)
		assert.Empty(t, ev.Errors)

		only[*harnesspb.ListToolsRequest](t, &d.recorder, "ListTools")
		assert.Equal(t, "echo", value[[]tool.Descriptor](t, rep, "ListTools")[0].Name)

		q := only[*harnesspb.QueueToolWorkRequest](t, &d.recorder, "QueueToolWork")
		assert.Equal(t, "echo", q.GetToolName())
		assert.Equal(t, []string{`"a"`, `"b"`}, q.GetInputJsons())
		assert.Equal(t, "job-7", value[string](t, rep, "QueueToolWork"))
		assert.Equal(t, "job-7", only[*harnesspb.ToolResultsRequest](t, &d.recorder, "ToolResults").GetJobId())
		assert.Equal(t, []queuedResult{{Index: 0, Output: "A"}, {Index: 1, Output: "B"}},
			value[[]queuedResult](t, rep, "ToolResults"))
	})

	t.Run("plugins and agents", func(t *testing.T) {
		req := only[*harnesspb.QueryPluginRequest](t, &d.recorder, "QueryPlugin")
		assert.Equal(t, "scanner", req.GetName())
		assert.Equal(t, "probe", req.GetMethod())
		assert.Equal(t, map[string]any{"host": "10.0.0.1"}, serve.FromTypedMap(req.GetParams()))
		assert.Equal(t, map[string]any{"open": true}, value[any](t, rep, "QueryPlugin"))

		only[*harnesspb.ListPluginsRequest](t, &d.recorder, "ListPlugins")
		assert.Contains(t, mustStep(t, rep, "ListPlugins").Summary, "scanner 2.0")

		del := only[*harnesspb.DelegateToAgentRequest](t, &d.recorder, "DelegateToAgent")
		assert.Equal(t, "helper", del.GetName())
		assert.Equal(t, "sub-1", del.GetTask().GetId())
		res := value[agent.Result](t, rep, "DelegateToAgent")
		assert.Equal(t, agent.StatusPartial, res.Status)
		assert.Equal(t, "helped", res.Output)
		require.Error(t, res.Error)
		assert.Contains(t, res.Error.Error(), "half done")

		only[*harnesspb.ListAgentsRequest](t, &d.recorder, "ListAgents")
		ag := value[[]agent.Descriptor](t, rep, "ListAgents")[0]
		assert.Equal(t, agent.Descriptor{
			Name: "helper", Version: "3.1", Description: "Helps", Capabilities: []string{"web"},
			TargetTypes: []string{"llm_chat"}, TechniqueTypes: []string{"T1190"},
		}, ag)
	})

	t.Run("world", func(t *testing.T) {
		sf := only[*harnesspb.SubmitFindingRequest](t, &d.recorder, "SubmitFinding")
		assert.Equal(t, "Open admin port", sf.GetFinding().GetTitle())
		assert.NotEmpty(t, sf.GetIdempotencyKey())
		assert.Equal(t, "example.com", only[*harnesspb.ObserveRequest](t, &d.recorder, "Observe").GetDomain().GetName())
		bet := only[*harnesspb.PlaceBetRequest](t, &d.recorder, "PlaceBet").GetBet()
		assert.Equal(t, "hyp-1", bet.GetHypothesisId())
		assert.Equal(t, "T1190", bet.GetTechnique())
		assert.InDelta(t, 0.7, bet.GetConfidence(), 0.001)

		wv := only[*harnesspb.WorldViewRequest](t, &d.recorder, "WorldView")
		assert.Equal(t, []string{"host:10.0.0.1"}, wv.GetFocus())
		view := value[agent.WorldView](t, rep, "WorldView")
		assert.True(t, view.Truncated)
		assert.Equal(t, agent.WorldEntity{
			Handle: "host:10.0.0.1", Kind: agent.EntityKindHost, Label: "web-1", Attributes: map[string]string{"os": "linux"},
		}, view.Entities[0])
	})

	t.Run("planning and workspace", func(t *testing.T) {
		assert.NotNil(t, only[*harnesspb.ReportStepHintsRequest](t, &d.recorder, "ReportStepHints").GetHints())
		assert.Equal(t, "no plan", mustStep(t, rep, "PlanContext").Summary)

		assert.Len(t, d.named("WorkspaceGetInfo"), 1)
		assert.Equal(t, "README.md", only[*harnesspb.WorkspaceReadFileRequest](t, &d.recorder, "WorkspaceReadFile").GetPath())
		assert.Equal(t, `main /work/main "# readme"`, mustStep(t, rep, "Workspace").Summary)
		only[*harnesspb.WorkspaceListRequest](t, &d.recorder, "WorkspaceList")
		assert.Len(t, value[map[string]workspace.Workspace](t, rep, "Workspaces"), 2)
	})

	t.Run("knowledge", func(t *testing.T) {
		only[*harnesspb.QueryNodesRequest](t, &d.recorder, "QueryNodes")
		assert.Equal(t, "2", mustStep(t, rep, "QueryNodes").Summary)
		fa := only[*harnesspb.FindSimilarAttacksRequest](t, &d.recorder, "FindSimilarAttacks")
		assert.Equal(t, "sql injection", fa.GetContent())
		assert.Equal(t, int32(3), fa.GetTopK())
		ac := only[*harnesspb.GetAttackChainsRequest](t, &d.recorder, "GetAttackChains")
		assert.Equal(t, "T1190", ac.GetTechniqueId())
		assert.Equal(t, int32(2), ac.GetMaxDepth())
		assert.Equal(t, "2", mustStep(t, rep, "GetAttackChains").Summary)
		sf := only[*harnesspb.FindSimilarFindingsRequest](t, &d.recorder, "FindSimilarFindings")
		assert.Equal(t, "f-1", sf.GetFindingId())
		assert.Equal(t, int32(4), sf.GetTopK())
		assert.Equal(t, "f-1", only[*harnesspb.GetRelatedFindingsRequest](t, &d.recorder, "GetRelatedFindings").GetFindingId())
		assert.Equal(t, "2", mustStep(t, rep, "GetRelatedFindings").Summary)

		gf := only[*harnesspb.GetFindingsRequest](t, &d.recorder, "GetFindings").GetFilter()
		assert.Equal(t, "mission-1", gf.GetMissionId())
		assert.Equal(t, []string{"web"}, gf.GetTags())
		assert.Equal(t, "Open port:", mustStep(t, rep, "GetFindings").Summary[:10])
		rf := only[*harnesspb.GetRunFindingsRequest](t, &d.recorder, "GetRunFindings")
		assert.Equal(t, harnesspb.RunScope_RUN_SCOPE_PREVIOUS, rf.GetScope())
		only[*harnesspb.GetMissionRunHistoryRequest](t, &d.recorder, "GetMissionRunHistory")
		runs := value[[]types.MissionRunSummary](t, rep, "GetMissionRunHistory")
		assert.Equal(t, 3, runs[0].FindingsCount)
		require.NotNil(t, runs[0].CompletedAt)

		app := only[*harnesspb.ApplicationFindingsRequest](t, &d.recorder, "ApplicationFindings")
		assert.Equal(t, "payments", app.GetApplication())
		assert.Equal(t, []string{"open"}, app.GetStatuses())
		assert.Equal(t, int32(10), app.GetLimit())
		af := value[[]agent.ApplicationFinding](t, rep, "ApplicationFindings")[0]
		assert.Equal(t, agent.ApplicationFinding{
			FindingID: "af-1", Status: "open", Severity: "high", VulnerabilityID: "CVE-1", PlaceLabel: "Service",
			PlaceKey: "pay", Reachable: true, Exposed: true, DeploymentKey: "dep-1", ImageKey: "img-1",
			Priority: "p1", PriorityRule: "rule-1", PriorityReason: "exposed",
		}, af)
	})

	t.Run("missions", func(t *testing.T) {
		cm := only[*harnesspb.CreateMissionRequest](t, &d.recorder, "CreateMission")
		assert.Equal(t, "target-1", cm.GetTargetId())
		assert.Equal(t, "child", cm.GetName())
		assert.Equal(t, []string{"d86"}, cm.GetTags())
		assert.Equal(t, harnesspb.OriginationStart_ORIGINATION_START_CALLER_STATE, cm.GetStartsFrom())
		assert.JSONEq(t, `{"name":"child"}`, string(cm.GetMissionDefinitionJson()))
		info := value[*mission.MissionInfo](t, rep, "CreateMission")
		assert.Equal(t, "child-1", info.ID)
		assert.Equal(t, mission.MissionStatusRunning, info.Status)

		rm := only[*harnesspb.RunMissionRequest](t, &d.recorder, "RunMission")
		assert.Equal(t, "child-1", rm.GetMissionId())
		assert.True(t, rm.GetWait())
		assert.Equal(t, int64(30000), rm.GetTimeoutMs())
		assert.Equal(t, "child-1", only[*harnesspb.GetMissionStatusRequest](t, &d.recorder, "GetMissionStatus").GetMissionId())
		status := value[*mission.MissionStatusInfo](t, rep, "GetMissionStatus")
		assert.Equal(t, "scan", status.Phase)
		assert.Equal(t, 2500*time.Millisecond, status.Duration)
		assert.Equal(t, map[string]int{"high": 2}, status.FindingCounts)

		wm := only[*harnesspb.WaitForMissionRequest](t, &d.recorder, "WaitForMission")
		assert.Equal(t, int64(5000), wm.GetTimeoutMs())
		waited := value[*mission.MissionResult](t, rep, "WaitForMission")
		assert.Equal(t, int64(99), waited.Metrics.TokensUsed)
		assert.Equal(t, 4, waited.Metrics.ToolCalls)
		assert.Equal(t, "Child finding", waited.Findings[0].Title)

		lm := only[*harnesspb.ListMissionsRequest](t, &d.recorder, "ListMissions")
		assert.Equal(t, harnesspb.MissionStatus_MISSION_STATUS_RUNNING, lm.GetFilter().GetStatus())
		assert.Equal(t, []string{"d86"}, lm.GetFilter().GetTags())
		assert.Equal(t, int32(5), lm.GetPageSize())
		assert.Len(t, value[[]*mission.MissionInfo](t, rep, "ListMissions"), 1)

		assert.Equal(t, "child-1", only[*harnesspb.CancelMissionRequest](t, &d.recorder, "CancelMission").GetMissionId())
		assert.Equal(t, "child-1", only[*harnesspb.GetMissionResultsRequest](t, &d.recorder, "GetMissionResults").GetMissionId())
		assert.Equal(t, mission.MissionStatusCompleted, value[*mission.MissionResult](t, rep, "GetMissionResults").Status)
	})

	t.Run("context", func(t *testing.T) {
		assert.Equal(t, "m-1 Mission One", mustStep(t, rep, "Mission").Summary)
		assert.Len(t, d.named("GetTaxonomySchema"), 1, "the harness reads the taxonomy at start")
		// Authorize reaches the daemon with the mission run of the task.
		az := only[*harnesspb.AuthorizeRequest](t, &d.recorder, "Authorize")
		assert.Equal(t, "run-1", az.GetRunId())
		assert.Equal(t, "scan", az.GetAction())
		assert.Equal(t, "target:", az.GetResource())
		require.NoError(t, mustStep(t, rep, "Authorize").Err)

		total := value[llm.TokenUsage](t, rep, "TokenUsage")
		assert.Positive(t, total.TotalTokens)
		assert.Contains(t, mustStep(t, rep, "TokenUsage").Summary, "slots=[main]")
	})

	t.Run("every call carries the grant and the task context", func(t *testing.T) {
		for _, c := range d.all() {
			assert.Equal(t, []string{"Bearer grant-1"}, c.md.Get("authorization"), c.method)
		}
		sf := only[*harnesspb.SubmitFindingRequest](t, &d.recorder, "SubmitFinding")
		_ = sf
		cm := only[*harnesspb.CreateMissionRequest](t, &d.recorder, "CreateMission")
		assert.Equal(t, "task-1", cm.GetContext().GetTaskId())
		assert.Equal(t, "arun-1", cm.GetContext().GetAgentRunId())
		assert.Equal(t, int32(2), cm.GetContext().GetRunNumber())
		assert.Equal(t, "trace-1", cm.GetContext().GetTraceId())
		assert.Equal(t, "tex-1", cm.GetContext().GetToolExecutionId())
	})
}

// TestCallbackHarness_AuthorizeFailsClosed proves that Authorize allows only on
// an explicit allow from the daemon, and that it refuses every other case.
func TestCallbackHarness_AuthorizeFailsClosed(t *testing.T) {
	newHarness := func(t *testing.T, addr string, serveTask bool) agent.Harness {
		t.Helper()
		client, h, err := Dial(context.Background(), CallbackOptions{Endpoint: addr})
		require.NoError(t, err)
		t.Cleanup(func() { _ = client.Close() })
		if serveTask {
			client.SetFullContext(taskParamsFixture)
		}
		return h
	}

	t.Run("a task is served and the daemon allows", func(t *testing.T) {
		d, addr := startCallbackDaemon(t)
		h := newHarness(t, addr, true)
		require.NoError(t, h.Authorize(context.Background(), "scan", "target:web"))
		assert.Equal(t, "run-1", only[*harnesspb.AuthorizeRequest](t, &d.recorder, "Authorize").GetRunId())
	})
	t.Run("no mission run is refused and nothing is sent", func(t *testing.T) {
		d, addr := startCallbackDaemon(t)
		h := newHarness(t, addr, false)
		require.ErrorIs(t, h.Authorize(context.Background(), "scan", "target:web"), harness.ErrNoRunContext)
		assert.Empty(t, d.named("Authorize"))
	})
	t.Run("a denial is refused", func(t *testing.T) {
		d, addr := startCallbackDaemon(t)
		h := newHarness(t, addr, true)
		require.ErrorIs(t, h.Authorize(context.Background(), "scan", "target:deny"), harness.ErrUnauthorized)
	})
	t.Run("a daemon with no Authorize RPC is refused", func(t *testing.T) {
		d, addr := startCallbackDaemon(t)
		d.noAuthorize = true
		h := newHarness(t, addr, true)
		require.ErrorIs(t, h.Authorize(context.Background(), "scan", "target:web"), harness.ErrAuthzServiceUnavailable)
	})
	t.Run("an empty action is refused", func(t *testing.T) {
		d, addr := startCallbackDaemon(t)
		h := newHarness(t, addr, true)
		require.ErrorIs(t, h.Authorize(context.Background(), "", "target:web"), harness.ErrInvalidAction)
		assert.Empty(t, d.named("Authorize"))
	})
}

func mustStep(t *testing.T, rep *Report, call string) Step {
	t.Helper()
	s, ok := rep.Step(call)
	require.True(t, ok, "no step %s", call)
	return s
}

func TestCallbackClient_Extras(t *testing.T) {
	d, addr := startCallbackDaemon(t)
	client, _, err := Dial(context.Background(), CallbackOptions{Endpoint: addr})
	require.NoError(t, err)
	t.Cleanup(func() { _ = client.Close() })
	client.SetFullContext(taskParamsFixture)

	rep := Extras(context.Background(), client)
	for _, s := range rep.Steps {
		require.NoError(t, s.Err, s.Call)
	}
	assert.Equal(t, "scanner-api", only[*harnesspb.GetCredentialRequest](t, &d.recorder, "GetCredential").GetName())
	assert.Equal(t, "host", only[*harnesspb.GenerateNodeIDRequest](t, &d.recorder, "GenerateNodeID").GetNodeType())
	assert.Equal(t, "host", only[*harnesspb.ValidateGraphNodeRequest](t, &d.recorder, "ValidateGraphNode").GetNodeType())
	assert.Equal(t, "RUNS", only[*harnesspb.ValidateRelationshipRequest](t, &d.recorder, "ValidateRelationship").GetRelationshipType())
	assert.NotNil(t, only[*harnesspb.ValidateFindingRequest](t, &d.recorder, "ValidateFinding"))
	assert.NotNil(t, only[*harnesspb.GetPlanContextRequest](t, &d.recorder, "GetPlanContext"))
	assert.NotNil(t, only[*harnesspb.RecordSpansRequest](t, &d.recorder, "RecordSpans"))
	assert.Equal(t, "host:abc", value[*harnesspb.GenerateNodeIDResponse](t, rep, "GenerateNodeID").GetNodeId())
	assert.Equal(t, "task-1", d.named("GenerateNodeID")[0].req.(*harnesspb.GenerateNodeIDRequest).GetContext().GetTaskId())
}

func TestDial_Refusals(t *testing.T) {
	_, _, err := Dial(context.Background(), CallbackOptions{})
	require.ErrorContains(t, err, "endpoint")
}

// ---------------------------------------------------------------------------
// serve.PlatformHarness against the fake ComponentService
// ---------------------------------------------------------------------------

func TestPlatformHarness_EveryCallReachesTheDaemon(t *testing.T) {
	d, addr := startComponentDaemon(t)
	ctx := context.Background()
	pc, err := serve.NewPlatformClientWithOptions("http://"+addr, "",
		serve.WithPlatformPollInterval(time.Second), serve.WithPlatformHeartbeatInterval(time.Second))
	require.NoError(t, err)
	require.NoError(t, pc.Connect(ctx))
	t.Cleanup(func() { _ = pc.Close() })

	h := serve.NewPlatformHarness(pc,
		serve.WithPlatformWorkID("work-1"),
		serve.WithPlatformMission(types.MissionContext{ID: "m-1", Name: "Mission One"}),
		serve.WithPlatformTarget(types.TargetInfo{ID: "t-1", Name: "web", Connection: map[string]any{"url": "https://t.example"}}),
		serve.WithPlatformLogger(slog.New(slog.DiscardHandler)),
	)
	a, err := New()
	require.NoError(t, err)
	res, err := a.Execute(ctx, h, goalTask)
	require.NoError(t, err)
	rep := reportOf(t, res)

	// The pull transport has no WorldView, Observe, PlaceBet or knowledge reads. The agent records that.
	unavailable := map[string]bool{
		"Observe": true, "PlaceBet": true, "WorldView": true, "QueryNodes": true, "FindSimilarAttacks": true,
		"GetAttackChains": true, "FindSimilarFindings": true, "GetRelatedFindings": true, "GetFindings": true,
		"GetRunFindings": true, "GetMissionRunHistory": true, "ApplicationFindings": true,
	}
	for _, s := range rep.Steps {
		if unavailable[s.Call] {
			assert.True(t, s.Unavailable(), "%s: %v", s.Call, s.Err)
			continue
		}
		require.NoError(t, s.Err, "step %s", s.Call)
	}
	assert.Empty(t, Failures(rep))

	for _, c := range d.all() {
		wid, ok := workID(c)
		require.True(t, ok, c.method)
		assert.Equal(t, "work-1", wid, c.method)
	}

	cr := only[*componentpb.CompleteRequest](t, &d.recorder, "Complete")
	assert.Equal(t, "main", cr.GetSlot())
	assert.Equal(t, "scan the host", cr.GetMessages()[0].GetContent())
	c := value[*llm.CompletionResponse](t, rep, "Complete")
	assert.Equal(t, "hello", c.Content)
	assert.Equal(t, 18, c.Usage.TotalTokens)
	ct := value[*llm.CompletionResponse](t, rep, "CompleteWithTools")
	assert.Equal(t, "lookup", ct.ToolCalls[0].Name)
	assert.Equal(t, "tool_use", ct.FinishReason)
	assert.Equal(t, map[string]any{"safe": true}, value[any](t, rep, "CompleteStructured"))
	assert.Len(t, d.named("CompleteStructured"), 2)
	assert.Equal(t, streamed{Text: "hello", Finish: "stop", Usage: llm.TokenUsage{
		InputTokens: 11, OutputTokens: 7, TotalTokens: 18}}, value[streamed](t, rep, "Stream"))

	assert.Equal(t, "pong", value[string](t, rep, "CallToolProto"))
	ev := value[*toolEvents](t, rep, "CallToolProtoStream")
	assert.Equal(t, []string{"50:scan:halfway"}, ev.Progress)
	assert.Equal(t, []string{"slow:W1"}, ev.Warnings)
	assert.Equal(t, 2, ev.Partials, "one partial and the final result")
	assert.Equal(t, "echo", value[[]tool.Descriptor](t, rep, "ListTools")[0].Name)
	assert.Equal(t, "job-7", value[string](t, rep, "QueueToolWork"))
	results := value[[]queuedResult](t, rep, "ToolResults")
	assert.Equal(t, []int{0, 1}, []int{results[0].Index, results[1].Index})
	assert.Equal(t, map[string]any{"open": true}, value[any](t, rep, "QueryPlugin"))
	assert.Empty(t, value[[]plugin.Descriptor](t, rep, "ListPlugins"))
	assert.Equal(t, agent.StatusPartial, value[agent.Result](t, rep, "DelegateToAgent").Status)
	assert.Equal(t, "helper", value[[]agent.Descriptor](t, rep, "ListAgents")[0].Name)
	assert.Equal(t, "child-1", value[*mission.MissionInfo](t, rep, "CreateMission").ID)
	assert.Equal(t, "scan", value[*mission.MissionStatusInfo](t, rep, "GetMissionStatus").Phase)
	assert.Equal(t, mission.MissionStatusCompleted, value[*mission.MissionResult](t, rep, "WaitForMission").Status)
	assert.Equal(t, "no plan", mustStep(t, rep, "PlanContext").Summary)
	assert.Equal(t, "none", mustStep(t, rep, "Workspace").Summary)
	assert.Equal(t, "m-1 Mission One", mustStep(t, rep, "Mission").Summary)
	assert.Equal(t, "t-1 web https://t.example", mustStep(t, rep, "Target").Summary)
	assert.Positive(t, value[llm.TokenUsage](t, rep, "TokenUsage").TotalTokens)

	// Each RPC that PlatformHarness uses reached the fake once or twice, with the right request.
	for _, m := range []string{
		"Complete", "CompleteWithTools", "CompleteStream", "CallTool", "CallToolStream", "ListTools",
		"QueueToolWork", "ToolResults", "QueryPlugin", "DelegateToAgent", "ListAgents", "SubmitFinding",
		"ReportStepHints", "CreateMission", "RunMission", "GetMissionStatus", "WaitMission", "ListMissions",
		"CancelMission", "GetMissionResults",
	} {
		assert.Len(t, d.named(m), 1, m)
	}
	ctReq := only[*componentpb.CallToolRequest](t, &d.recorder, "CallTool")
	assert.Equal(t, "echo", ctReq.GetToolName())
	assert.JSONEq(t, `"ping"`, ctReq.GetInputJson())
	assert.Equal(t, []string{`"a"`, `"b"`}, only[*componentpb.QueueToolWorkRequest](t, &d.recorder, "QueueToolWork").GetInputsJson())
	assert.Equal(t, "job-7", only[*componentpb.ToolResultsRequest](t, &d.recorder, "ToolResults").GetJobId())
	qp := only[*componentpb.QueryPluginRequest](t, &d.recorder, "QueryPlugin")
	assert.Equal(t, "scanner", qp.GetPluginName())
	assert.JSONEq(t, `{"host":"10.0.0.1"}`, qp.GetParamsJson())
	assert.Equal(t, "helper", only[*componentpb.DelegateToAgentRequest](t, &d.recorder, "DelegateToAgent").GetAgentName())
	assert.Contains(t, string(only[*componentpb.SubmitFindingRequest](t, &d.recorder, "SubmitFinding").GetFinding()), "Open admin port")
	assert.Equal(t, "target-1", only[*componentpb.CreateMissionRequest](t, &d.recorder, "CreateMission").GetTargetId())
	assert.Equal(t, int64(5000), only[*componentpb.WaitMissionRequest](t, &d.recorder, "WaitMission").GetTimeoutMs())
	assert.NotEmpty(t, only[*componentpb.ReportStepHintsRequest](t, &d.recorder, "ReportStepHints").GetHintsJson())
	assert.Empty(t, d.named("Authorize"), "the pull transport allows Authorize locally")
}

func workID(c call) (string, bool) {
	w, ok := c.req.(interface{ GetWorkId() string })
	if !ok {
		return "", false
	}
	return w.GetWorkId(), true
}

// ---------------------------------------------------------------------------
// The options of the callback client
// ---------------------------------------------------------------------------

type perRPC struct{ token string }

func (p perRPC) GetRequestMetadata(context.Context, ...string) (map[string]string, error) {
	return map[string]string{"authorization": "Bearer " + p.token}, nil
}
func (perRPC) RequireTransportSecurity() bool { return false }

// selfSigned makes a TLS server certificate for 127.0.0.1 and the pool that trusts it.
func selfSigned(t *testing.T) (tls.Certificate, *x509.CertPool) {
	t.Helper()
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	require.NoError(t, err)
	tmpl := &x509.Certificate{
		SerialNumber: big.NewInt(1), Subject: pkix.Name{CommonName: "daemon"},
		NotBefore: time.Now().Add(-time.Hour), NotAfter: time.Now().Add(time.Hour),
		KeyUsage: x509.KeyUsageDigitalSignature, ExtKeyUsage: []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		IPAddresses: []net.IP{net.ParseIP("127.0.0.1")}, BasicConstraintsValid: true, IsCA: true,
	}
	der, err := x509.CreateCertificate(rand.Reader, tmpl, tmpl, &key.PublicKey, key)
	require.NoError(t, err)
	leaf, err := x509.ParseCertificate(der)
	require.NoError(t, err)
	pool := x509.NewCertPool()
	pool.AddCert(leaf)
	return tls.Certificate{Certificate: [][]byte{der}, PrivateKey: key}, pool
}

func TestCallbackOptions(t *testing.T) {
	cert, pool := selfSigned(t)
	serverTLS := grpc.Creds(credentials.NewTLS(&tls.Config{Certificates: []tls.Certificate{cert}, MinVersion: tls.VersionTLS13}))
	clientTLS := &tls.Config{RootCAs: pool, MinVersion: tls.VersionTLS13}

	cases := []struct {
		name   string
		secure bool
		opts   func() CallbackOptions
		auth   []string
	}{
		{"token", false, func() CallbackOptions { return CallbackOptions{Token: "tok"} }, []string{"Bearer tok"}},
		{"per-rpc credentials", false, func() CallbackOptions { return CallbackOptions{PerRPC: perRPC{"rpc"}} }, []string{"Bearer rpc"}},
		{"tls config", true, func() CallbackOptions { return CallbackOptions{TLS: clientTLS, Token: "tok"} }, []string{"Bearer tok"}},
		{"transport credentials", true, func() CallbackOptions {
			return CallbackOptions{Transport: credentials.NewTLS(clientTLS), Token: "tok"}
		}, []string{"Bearer tok"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var extra []grpc.ServerOption
			if tc.secure {
				extra = append(extra, serverTLS)
			}
			d, addr := startCallbackDaemon(t, extra...)
			o := tc.opts()
			o.Endpoint = addr
			client, h, err := Dial(context.Background(), o)
			require.NoError(t, err)
			t.Cleanup(func() { _ = client.Close() })

			_, err = h.Complete(context.Background(), "main", []llm.Message{{Role: llm.RoleUser, Content: "hi"}})
			require.NoError(t, err)
			cs := d.named("LLMComplete")
			require.Len(t, cs, 1)
			assert.Equal(t, tc.auth, cs[0].md.Get("authorization"))
		})
	}
}

func TestTLSClientAgainstPlainServerFails(t *testing.T) {
	_, addr := startCallbackDaemon(t)
	_, pool := selfSigned(t)
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	client, h, err := Dial(ctx, CallbackOptions{Endpoint: addr, TLS: &tls.Config{RootCAs: pool, MinVersion: tls.VersionTLS13}})
	if err == nil {
		t.Cleanup(func() { _ = client.Close() })
		_, err = h.Complete(ctx, "main", nil)
	}
	require.Error(t, err)
}

// ---------------------------------------------------------------------------
// The fork claim (D74, D80)
// ---------------------------------------------------------------------------

func forkEnv(t *testing.T, timeout string) *fakeIdentity {
	t.Helper()
	id := serveIdentity(t)
	t.Setenv(fork.EnvForkable, "1")
	t.Setenv(fork.EnvParkTimeout, timeout)
	return id
}

func TestFork_ClaimUsesTheIdentityTokenAndNoGrantAndTheNewGrantReplacesTheOld(t *testing.T) {
	id := forkEnv(t, "10s")
	d, addr := startCallbackDaemon(t)
	d.claims = []func() (*harnesspb.ClaimForkResponse, error){forkUnclaimed, forkDispatch}
	h := &host{name: "sbx-parent"}
	time.AfterFunc(60*time.Millisecond, func() { h.set("sbx-fork-1") })

	a, err := New()
	require.NoError(t, err)
	// The context carries the expired grant of the source. The claim must not send it.
	ctx := metadata.AppendToOutgoingContext(context.Background(), "authorization", "Bearer expired-source-grant")
	summary, err := RunFork(ctx, a,
		CallbackOptions{Endpoint: addr, Token: "parent-grant"},
		ForkOptions{ReadSandboxID: h.read, PollInterval: 10 * time.Millisecond})
	require.NoError(t, err)

	claims := d.named("ClaimFork")
	require.Len(t, claims, 2, "one unclaimed answer, then the dispatch")
	for _, c := range claims {
		assert.Equal(t, "sbx-fork-1", c.req.(*harnesspb.ClaimForkRequest).GetSandboxId())
		assert.Empty(t, c.md.Get("authorization"), "a claim carries no grant")
		require.Len(t, c.md.Get(fork.MetadataSandboxIdentity), 1, "a claim carries the identity token")
		assert.Contains(t, c.md.Get(fork.MetadataSandboxIdentity)[0], "sbx-id-")
	}
	assert.NotEqual(t, claims[0].md.Get(fork.MetadataSandboxIdentity), claims[1].md.Get(fork.MetadataSandboxIdentity),
		"each call asks the socket for a new token")
	assert.GreaterOrEqual(t, id.requests, 2)

	assert.Contains(t, summary, "node=node-b")
	assert.Contains(t, summary, "model=model-x")
	assert.Contains(t, summary, "task=task-b")
	assert.Contains(t, summary, "Complete: *llm.CompletionResponse")
	assert.NotContains(t, summary, "failed=")

	after := 0
	for _, c := range d.all() {
		if c.method == "ClaimFork" || c.method == "GetTaxonomySchema" {
			continue
		}
		after++
		assert.Equal(t, []string{"Bearer fork-grant"}, c.md.Get("authorization"), "%s uses the new grant", c.method)
	}
	assert.Positive(t, after)
	cm := only[*harnesspb.CreateMissionRequest](t, &d.recorder, "CreateMission")
	assert.Equal(t, "task-b", cm.GetContext().GetTaskId())
	assert.Equal(t, "child-1", cm.GetContext().GetMissionId())
	assert.Equal(t, "fork-run-1", cm.GetContext().GetAgentRunId())
	assert.Equal(t, "scan the next host", d.named("LLMComplete")[0].req.(*harnesspb.LLMCompleteRequest).GetMessages()[0].GetContent())
}

func TestFork_NotForkableClaimsNothing(t *testing.T) {
	t.Setenv(fork.EnvForkable, "")
	d, addr := startCallbackDaemon(t)
	a, err := New()
	require.NoError(t, err)
	summary, err := RunFork(context.Background(), a, CallbackOptions{Endpoint: addr}, ForkOptions{})
	require.NoError(t, err)
	assert.Equal(t, "no fork", summary)
	assert.Empty(t, d.named("ClaimFork"))
}

func TestFork_TheParentTimesOutWithoutAClaim(t *testing.T) {
	forkEnv(t, "100ms")
	d, addr := startCallbackDaemon(t)
	a, err := New()
	require.NoError(t, err)
	summary, err := RunFork(context.Background(), a, CallbackOptions{Endpoint: addr},
		ForkOptions{ReadSandboxID: (&host{name: "sbx-parent"}).read, PollInterval: 10 * time.Millisecond})
	require.NoError(t, err)
	assert.Equal(t, "no fork", summary)
	assert.Empty(t, d.named("ClaimFork"))
}

func TestFork_Refusals(t *testing.T) {
	t.Run("a bad park timeout", func(t *testing.T) {
		forkEnv(t, "soon")
		_, addr := startCallbackDaemon(t)
		a, err := New()
		require.NoError(t, err)
		_, err = RunFork(context.Background(), a, CallbackOptions{Endpoint: addr}, ForkOptions{})
		require.ErrorContains(t, err, fork.EnvParkTimeout)
	})
	t.Run("a client with per-rpc credentials cannot claim", func(t *testing.T) {
		forkEnv(t, "5s")
		d, addr := startCallbackDaemon(t)
		h := &host{name: "sbx-parent"}
		time.AfterFunc(30*time.Millisecond, func() { h.set("sbx-fork-1") })
		a, err := New()
		require.NoError(t, err)
		_, err = RunFork(context.Background(), a, CallbackOptions{Endpoint: addr, PerRPC: perRPC{"rpc"}},
			ForkOptions{ReadSandboxID: h.read, PollInterval: 10 * time.Millisecond})
		require.ErrorContains(t, err, "per-RPC credentials")
		assert.Empty(t, d.named("ClaimFork"))
	})
	t.Run("a process with no identity socket cannot claim", func(t *testing.T) {
		forkEnv(t, "5s")
		t.Setenv(fork.EnvIdentitySocket, "")
		_, addr := startCallbackDaemon(t)
		h := &host{name: "sbx-parent"}
		time.AfterFunc(30*time.Millisecond, func() { h.set("sbx-fork-1") })
		a, err := New()
		require.NoError(t, err)
		_, err = RunFork(context.Background(), a, CallbackOptions{Endpoint: addr},
			ForkOptions{ReadSandboxID: h.read, PollInterval: 10 * time.Millisecond})
		require.ErrorIs(t, err, fork.ErrNoSandboxIdentity)
	})
	t.Run("a claim with no task", func(t *testing.T) {
		_, err := ForkTask(nil)
		require.Error(t, err)
	})
}

func TestRun(t *testing.T) {
	t.Setenv("GIBSON_URL", "")
	t.Setenv("GIBSON_BOOTSTRAP_TOKEN", "")
	t.Run("pull transport without a platform url", func(t *testing.T) {
		err := run(context.Background(), func(string) string { return "" })
		require.Error(t, err)
	})
	t.Run("callback transport, not forkable", func(t *testing.T) {
		t.Setenv(fork.EnvForkable, "")
		_, addr := startCallbackDaemon(t)
		env := map[string]string{envCallbackEndpoint: addr, envCallbackToken: "g", "GIBSON_MISSION_ID": "m-1"}
		require.NoError(t, run(context.Background(), func(k string) string { return env[k] }))
	})
}
