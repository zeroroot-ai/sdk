// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Zero Root AI

package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"google.golang.org/genproto/googleapis/rpc/errdetails"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/wrapperspb"

	componentpb "github.com/zeroroot-ai/sdk/api/gen/gibson/component/v1"
	graphragpb "github.com/zeroroot-ai/sdk/api/gen/gibson/graphrag/v1"
	harnesspb "github.com/zeroroot-ai/sdk/api/gen/gibson/harness/v1"
	typespb "github.com/zeroroot-ai/sdk/api/gen/gibson/types/v1"
	"github.com/zeroroot-ai/sdk/fork"
	"github.com/zeroroot-ai/sdk/mission"
	"github.com/zeroroot-ai/sdk/serve"
)

// call is one RPC that reached a fake daemon.
type call struct {
	method string
	req    proto.Message
	md     metadata.MD
}

// recorder keeps the RPCs that reach a fake daemon, unary and streaming.
type recorder struct {
	mu    sync.Mutex
	calls []call
}

func (r *recorder) add(ctx context.Context, full string, req proto.Message) {
	md, _ := metadata.FromIncomingContext(ctx)
	r.mu.Lock()
	defer r.mu.Unlock()
	r.calls = append(r.calls, call{method: full[strings.LastIndex(full, "/")+1:], req: req, md: md})
}

func (r *recorder) unary(
	ctx context.Context, req any, info *grpc.UnaryServerInfo, next grpc.UnaryHandler,
) (any, error) {
	if m, ok := req.(proto.Message); ok {
		r.add(ctx, info.FullMethod, m)
	}
	return next(ctx, req)
}

type recordingStream struct {
	grpc.ServerStream
	rec  *recorder
	full string
	once sync.Once
}

func (s *recordingStream) RecvMsg(m any) error {
	if err := s.ServerStream.RecvMsg(m); err != nil {
		return fmt.Errorf("recv: %w", err)
	}
	if pm, ok := m.(proto.Message); ok {
		s.once.Do(func() { s.rec.add(s.Context(), s.full, pm) })
	}
	return nil
}

func (r *recorder) stream(srv any, ss grpc.ServerStream, info *grpc.StreamServerInfo, next grpc.StreamHandler) error {
	return next(srv, &recordingStream{ServerStream: ss, rec: r, full: info.FullMethod})
}

// named returns the calls of one RPC.
func (r *recorder) named(method string) []call {
	r.mu.Lock()
	defer r.mu.Unlock()
	var out []call
	for _, c := range r.calls {
		if c.method == method {
			out = append(out, c)
		}
	}
	return out
}

func (r *recorder) all() []call {
	r.mu.Lock()
	defer r.mu.Unlock()
	return append([]call(nil), r.calls...)
}

// only returns the request of the one call of an RPC.
func only[T proto.Message](t *testing.T, r *recorder, method string) T {
	t.Helper()
	cs := r.named(method)
	require.Len(t, cs, 1, "calls of %s", method)
	got, ok := cs[0].req.(T)
	require.True(t, ok, "request type of %s is %T", method, cs[0].req)
	return got
}

func (r *recorder) serve(t *testing.T, register func(*grpc.Server), extra ...grpc.ServerOption) string {
	t.Helper()
	lis, err := (&net.ListenConfig{}).Listen(context.Background(), "tcp", "127.0.0.1:0")
	require.NoError(t, err)
	opts := append([]grpc.ServerOption{grpc.UnaryInterceptor(r.unary), grpc.StreamInterceptor(r.stream)}, extra...)
	srv := grpc.NewServer(opts...)
	register(srv)
	go func() { _ = srv.Serve(lis) }()
	t.Cleanup(srv.Stop)
	return lis.Addr().String()
}

// ---------------------------------------------------------------------------
// The fake daemon: HarnessCallbackService
// ---------------------------------------------------------------------------

type callbackDaemon struct {
	harnesspb.UnimplementedHarnessCallbackServiceServer
	recorder

	// claims is the answer of ClaimFork, in order. The last one repeats.
	noAuthorize bool

	claimMu sync.Mutex
	claims  []func() (*harnesspb.ClaimForkResponse, error)
}

func startCallbackDaemon(t *testing.T, extra ...grpc.ServerOption) (d *callbackDaemon, addr string) {
	t.Helper()
	d = &callbackDaemon{}
	addr = d.serve(t, func(s *grpc.Server) { harnesspb.RegisterHarnessCallbackServiceServer(s, d) }, extra...)
	return d, addr
}

var usage = &harnesspb.TokenUsage{InputTokens: 11, OutputTokens: 7, TotalTokens: 18}

func (d *callbackDaemon) GetTaxonomySchema(context.Context, *harnesspb.GetTaxonomySchemaRequest) (
	*harnesspb.GetTaxonomySchemaResponse, error,
) {
	return &harnesspb.GetTaxonomySchemaResponse{Version: "tax-1"}, nil
}

func (d *callbackDaemon) LLMComplete(context.Context, *harnesspb.LLMCompleteRequest) (*harnesspb.LLMCompleteResponse, error) {
	return &harnesspb.LLMCompleteResponse{Content: "hello", FinishReason: "stop", Usage: usage}, nil
}

func (d *callbackDaemon) LLMCompleteWithTools(
	context.Context, *harnesspb.LLMCompleteWithToolsRequest,
) (*harnesspb.LLMCompleteWithToolsResponse, error) {
	return &harnesspb.LLMCompleteWithToolsResponse{
		FinishReason: "tool_use", Usage: usage,
		ToolCalls: []*harnesspb.ToolCall{{Id: "tc-1", Name: "lookup", Arguments: `{"host":"10.0.0.1"}`}},
	}, nil
}

func (d *callbackDaemon) LLMCompleteStructured(
	context.Context, *harnesspb.LLMCompleteStructuredRequest,
) (*harnesspb.LLMCompleteStructuredResponse, error) {
	return &harnesspb.LLMCompleteStructuredResponse{
		Result: serve.ToTypedValue(map[string]any{"safe": true}), Usage: usage,
	}, nil
}

func (d *callbackDaemon) LLMStream(_ *harnesspb.LLMStreamRequest, s grpc.ServerStreamingServer[harnesspb.LLMStreamResponse]) error {
	for _, c := range []*harnesspb.LLMStreamResponse{
		{Delta: "he"},
		{Delta: "llo", ToolCalls: []*harnesspb.ToolCall{{Id: "tc-2", Name: "lookup"}}},
		{FinishReason: "stop", Usage: usage},
	} {
		if err := s.Send(c); err != nil {
			return fmt.Errorf("send: %w", err)
		}
	}
	return nil
}

func (d *callbackDaemon) CallToolProto(context.Context, *harnesspb.CallToolProtoRequest) (*harnesspb.CallToolProtoResponse, error) {
	return &harnesspb.CallToolProtoResponse{OutputJson: []byte(`"pong"`)}, nil
}

func (d *callbackDaemon) CallToolProtoStream(
	_ *harnesspb.CallToolProtoStreamRequest, s grpc.ServerStreamingServer[harnesspb.CallToolProtoStreamResponse],
) error {
	for _, m := range []*harnesspb.CallToolProtoStreamResponse{
		{Payload: &harnesspb.CallToolProtoStreamResponse_Progress{
			Progress: &harnesspb.ToolProgressEvent{Percent: 50, Stage: "scan", Message: "halfway"}}},
		{Payload: &harnesspb.CallToolProtoStreamResponse_Partial{
			Partial: &harnesspb.ToolPartialResultEvent{OutputJson: []byte(`"part"`)}}},
		{Payload: &harnesspb.CallToolProtoStreamResponse_Warning{
			Warning: &harnesspb.ToolWarningEvent{Message: "slow", Code: "W1"}}},
		{Payload: &harnesspb.CallToolProtoStreamResponse_Complete{
			Complete: &harnesspb.ToolCompleteEvent{OutputJson: []byte(`"done"`)}}},
	} {
		if err := s.Send(m); err != nil {
			return fmt.Errorf("send: %w", err)
		}
	}
	return nil
}

func (d *callbackDaemon) ListTools(context.Context, *harnesspb.ListToolsRequest) (*harnesspb.ListToolsResponse, error) {
	return &harnesspb.ListToolsResponse{Tools: []*harnesspb.HarnessToolDescriptor{{Name: "echo", Description: "Echo"}}}, nil
}

func (d *callbackDaemon) QueueToolWork(context.Context, *harnesspb.QueueToolWorkRequest) (*harnesspb.QueueToolWorkResponse, error) {
	return &harnesspb.QueueToolWorkResponse{JobId: "job-7"}, nil
}

func (d *callbackDaemon) ToolResults(_ *harnesspb.ToolResultsRequest, s grpc.ServerStreamingServer[harnesspb.ToolResultsResponse]) error {
	typ := string((&wrapperspb.StringValue{}).ProtoReflect().Descriptor().FullName())
	for _, m := range []*harnesspb.ToolResultsResponse{
		{Index: 0, OutputJson: `"A"`, OutputType: typ},
		{Index: 1, OutputJson: `"B"`, OutputType: typ, IsFinal: true},
	} {
		if err := s.Send(m); err != nil {
			return fmt.Errorf("send: %w", err)
		}
	}
	return nil
}

func (d *callbackDaemon) QueryPlugin(context.Context, *harnesspb.QueryPluginRequest) (*harnesspb.QueryPluginResponse, error) {
	return &harnesspb.QueryPluginResponse{Result: serve.ToTypedValue(map[string]any{"open": true})}, nil
}

func (d *callbackDaemon) ListPlugins(context.Context, *harnesspb.ListPluginsRequest) (*harnesspb.ListPluginsResponse, error) {
	return &harnesspb.ListPluginsResponse{Plugins: []*harnesspb.HarnessPluginDescriptor{
		{Name: "scanner", Version: "2.0", Description: "Scans", Methods: []string{"probe"}},
	}}, nil
}

func (d *callbackDaemon) DelegateToAgent(context.Context, *harnesspb.DelegateToAgentRequest) (*harnesspb.DelegateToAgentResponse, error) {
	return &harnesspb.DelegateToAgentResponse{Result: &typespb.Result{
		Status: typespb.ResultStatus_RESULT_STATUS_PARTIAL,
		Output: serve.ToTypedValue("helped"),
		Error:  &typespb.ResultError{Message: "half done"},
	}}, nil
}

func (d *callbackDaemon) ListAgents(context.Context, *harnesspb.ListAgentsRequest) (*harnesspb.ListAgentsResponse, error) {
	return &harnesspb.ListAgentsResponse{Agents: []*harnesspb.HarnessAgentDescriptor{{
		Name: "helper", Version: "3.1", Description: "Helps", Capabilities: []string{"web"},
		TargetTypes: []string{"llm_chat"}, TechniqueTypes: []string{"T1190"},
	}}}, nil
}

func (d *callbackDaemon) SubmitFinding(context.Context, *harnesspb.SubmitFindingRequest) (*harnesspb.SubmitFindingResponse, error) {
	return &harnesspb.SubmitFindingResponse{}, nil
}

func (d *callbackDaemon) Observe(context.Context, *harnesspb.ObserveRequest) (*harnesspb.ObserveResponse, error) {
	return &harnesspb.ObserveResponse{}, nil
}

func (d *callbackDaemon) PlaceBet(context.Context, *harnesspb.PlaceBetRequest) (*harnesspb.PlaceBetResponse, error) {
	return &harnesspb.PlaceBetResponse{}, nil
}

func (d *callbackDaemon) WorldView(context.Context, *harnesspb.WorldViewRequest) (*harnesspb.WorldViewResponse, error) {
	return &harnesspb.WorldViewResponse{Truncated: true, Entities: []*harnesspb.WorldEntity{{
		Handle: "host:10.0.0.1", Kind: harnesspb.WorldEntityKind_WORLD_ENTITY_KIND_HOST,
		Label: "web-1", Attributes: map[string]string{"os": "linux"},
	}}}, nil
}

func (d *callbackDaemon) ReportStepHints(context.Context, *harnesspb.ReportStepHintsRequest) (*harnesspb.ReportStepHintsResponse, error) {
	return &harnesspb.ReportStepHintsResponse{}, nil
}

func (d *callbackDaemon) GetPlanContext(context.Context, *harnesspb.GetPlanContextRequest) (*harnesspb.GetPlanContextResponse, error) {
	return &harnesspb.GetPlanContextResponse{}, nil
}

func (d *callbackDaemon) RecordSpans(context.Context, *harnesspb.RecordSpansRequest) (*harnesspb.RecordSpansResponse, error) {
	return &harnesspb.RecordSpansResponse{}, nil
}

func (d *callbackDaemon) GetCredential(context.Context, *harnesspb.GetCredentialRequest) (*harnesspb.GetCredentialResponse, error) {
	return &harnesspb.GetCredentialResponse{Credential: &harnesspb.Credential{Name: "scanner-api"}}, nil
}

func (d *callbackDaemon) GenerateNodeID(context.Context, *harnesspb.GenerateNodeIDRequest) (*harnesspb.GenerateNodeIDResponse, error) {
	return &harnesspb.GenerateNodeIDResponse{NodeId: "host:abc"}, nil
}

func (d *callbackDaemon) ValidateFinding(context.Context, *harnesspb.ValidateFindingRequest) (*harnesspb.ValidateFindingResponse, error) {
	return &harnesspb.ValidateFindingResponse{Valid: true}, nil
}

func (d *callbackDaemon) ValidateGraphNode(context.Context, *harnesspb.ValidateGraphNodeRequest) (*harnesspb.ValidateGraphNodeResponse, error) {
	return &harnesspb.ValidateGraphNodeResponse{Valid: true}, nil
}

func (d *callbackDaemon) ValidateRelationship(
	context.Context, *harnesspb.ValidateRelationshipRequest,
) (*harnesspb.ValidateRelationshipResponse, error) {
	return &harnesspb.ValidateRelationshipResponse{Valid: true}, nil
}

func (d *callbackDaemon) WorkspaceGetInfo(context.Context, *harnesspb.WorkspaceGetInfoRequest) (*harnesspb.WorkspaceGetInfoResponse, error) {
	return &harnesspb.WorkspaceGetInfoResponse{Workspace: &harnesspb.WorkspaceInfo{Name: "main", Path: "/work/main"}}, nil
}

func (d *callbackDaemon) WorkspaceList(context.Context, *harnesspb.WorkspaceListRequest) (*harnesspb.WorkspaceListResponse, error) {
	return &harnesspb.WorkspaceListResponse{Workspaces: []*harnesspb.WorkspaceInfo{
		{Name: "main", Path: "/work/main"}, {Name: "aux", Path: "/work/aux"},
	}}, nil
}

func (d *callbackDaemon) WorkspaceReadFile(
	context.Context, *harnesspb.WorkspaceReadFileRequest,
) (*harnesspb.WorkspaceReadFileResponse, error) {
	return &harnesspb.WorkspaceReadFileResponse{Content: []byte("# readme")}, nil
}

func (d *callbackDaemon) QueryNodes(context.Context, *harnesspb.QueryNodesRequest) (*harnesspb.QueryNodesResponse, error) {
	return &harnesspb.QueryNodesResponse{Results: []*graphragpb.QueryResult{{Score: 0.5}, {Score: 0.4}}}, nil
}

func (d *callbackDaemon) FindSimilarAttacks(
	context.Context, *harnesspb.FindSimilarAttacksRequest,
) (*harnesspb.FindSimilarAttacksResponse, error) {
	return &harnesspb.FindSimilarAttacksResponse{Results: []*graphragpb.AttackPattern{{Id: "ap-1"}}}, nil
}

func (d *callbackDaemon) GetAttackChains(context.Context, *harnesspb.GetAttackChainsRequest) (*harnesspb.GetAttackChainsResponse, error) {
	return &harnesspb.GetAttackChainsResponse{Results: []*graphragpb.AttackChain{{Id: "ac-1"}, {Id: "ac-2"}}}, nil
}

func (d *callbackDaemon) FindSimilarFindings(
	context.Context, *harnesspb.FindSimilarFindingsRequest,
) (*harnesspb.FindSimilarFindingsResponse, error) {
	return &harnesspb.FindSimilarFindingsResponse{Results: []*graphragpb.FindingNode{{Id: "fn-1"}}}, nil
}

func (d *callbackDaemon) GetRelatedFindings(
	context.Context, *harnesspb.GetRelatedFindingsRequest,
) (*harnesspb.GetRelatedFindingsResponse, error) {
	return &harnesspb.GetRelatedFindingsResponse{Results: []*graphragpb.FindingNode{{Id: "fn-2"}, {Id: "fn-3"}}}, nil
}

func (d *callbackDaemon) GetFindings(context.Context, *harnesspb.GetFindingsRequest) (*harnesspb.GetFindingsResponse, error) {
	return &harnesspb.GetFindingsResponse{Findings: []*typespb.Finding{{Id: "f-1", Title: "Open port"}}}, nil
}

func (d *callbackDaemon) GetRunFindings(context.Context, *harnesspb.GetRunFindingsRequest) (*harnesspb.GetRunFindingsResponse, error) {
	return &harnesspb.GetRunFindingsResponse{Findings: []*typespb.Finding{{Id: "f-2", Title: "Old port"}}}, nil
}

func (d *callbackDaemon) GetMissionRunHistory(
	context.Context, *harnesspb.GetMissionRunHistoryRequest,
) (*harnesspb.GetMissionRunHistoryResponse, error) {
	return &harnesspb.GetMissionRunHistoryResponse{Runs: []*typespb.MissionRunSummary{
		{MissionId: "m-1", RunNumber: 1, Status: "completed", FindingsCount: 3, CreatedAt: 1000, CompletedAt: 2000},
	}}, nil
}

func (d *callbackDaemon) ApplicationFindings(
	context.Context, *harnesspb.ApplicationFindingsRequest,
) (*harnesspb.ApplicationFindingsResponse, error) {
	return &harnesspb.ApplicationFindingsResponse{Findings: []*harnesspb.ApplicationFinding{{
		FindingId: "af-1", Status: "open", Severity: "high", VulnerabilityId: "CVE-1", PlaceLabel: "Service",
		PlaceKey: "pay", Reachable: true, Exposed: true, DeploymentKey: "dep-1", ImageKey: "img-1",
		Priority: "p1", PriorityRule: "rule-1", PriorityReason: "exposed",
	}}}, nil
}

func (d *callbackDaemon) CreateMission(context.Context, *harnesspb.CreateMissionRequest) (*harnesspb.CreateMissionResponse, error) {
	return &harnesspb.CreateMissionResponse{Mission: missionInfo()}, nil
}

func missionInfo() *harnesspb.MissionInfo {
	return &harnesspb.MissionInfo{
		Id: "child-1", Name: "child", Status: harnesspb.MissionStatus_MISSION_STATUS_RUNNING,
		TargetId: "target-1", ParentMissionId: "m-1", CreatedAt: 5000, Tags: []string{"d86"},
	}
}

func missionResult() *harnesspb.MissionResult {
	return &harnesspb.MissionResult{
		MissionId: "child-1", Status: harnesspb.MissionStatus_MISSION_STATUS_COMPLETED,
		Findings:    []*typespb.Finding{{Id: "f-9", Title: "Child finding"}},
		Metrics:     &harnesspb.MissionMetrics{DurationMs: 1500, TokensUsed: 99, ToolCalls: 4, AgentCalls: 2, FindingsCount: 1},
		CompletedAt: 9000,
	}
}

func (d *callbackDaemon) RunMission(context.Context, *harnesspb.RunMissionRequest) (*harnesspb.RunMissionResponse, error) {
	return &harnesspb.RunMissionResponse{}, nil
}

func (d *callbackDaemon) GetMissionStatus(context.Context, *harnesspb.GetMissionStatusRequest) (*harnesspb.GetMissionStatusResponse, error) {
	return &harnesspb.GetMissionStatusResponse{Status: &harnesspb.MissionStatusInfo{
		Status: harnesspb.MissionStatus_MISSION_STATUS_RUNNING, Progress: 0.5, Phase: "scan",
		FindingCounts: map[string]int32{"high": 2}, TokenUsage: 42, DurationMs: 2500,
	}}, nil
}

func (d *callbackDaemon) WaitForMission(context.Context, *harnesspb.WaitForMissionRequest) (*harnesspb.WaitForMissionResponse, error) {
	return &harnesspb.WaitForMissionResponse{Result: missionResult()}, nil
}

func (d *callbackDaemon) ListMissions(context.Context, *harnesspb.ListMissionsRequest) (*harnesspb.ListMissionsResponse, error) {
	return &harnesspb.ListMissionsResponse{Missions: []*harnesspb.MissionInfo{missionInfo()}}, nil
}

func (d *callbackDaemon) CancelMission(context.Context, *harnesspb.CancelMissionRequest) (*harnesspb.CancelMissionResponse, error) {
	return &harnesspb.CancelMissionResponse{}, nil
}

func (d *callbackDaemon) GetMissionResults(
	context.Context, *harnesspb.GetMissionResultsRequest,
) (*harnesspb.GetMissionResultsResponse, error) {
	return &harnesspb.GetMissionResultsResponse{Result: missionResult()}, nil
}

// Authorize allows an action, except on a resource named "...:deny". A daemon
// with noAuthorize set answers Unimplemented, as an old daemon does.
func (d *callbackDaemon) Authorize(_ context.Context, req *harnesspb.AuthorizeRequest) (*harnesspb.AuthorizeResponse, error) {
	if d.noAuthorize {
		return nil, fmt.Errorf("authorize: %w", status.Error(codes.Unimplemented, "no Authorize"))
	}
	switch {
	case strings.HasSuffix(req.GetResource(), ":invalid"):
		return nil, fmt.Errorf("authorize: %w", status.Error(codes.InvalidArgument, "resource is not kind-qualified"))
	case strings.HasSuffix(req.GetResource(), ":forbidden"):
		return nil, fmt.Errorf("authorize: %w", status.Error(codes.PermissionDenied, "no"))
	case strings.HasSuffix(req.GetResource(), ":anon"):
		return nil, fmt.Errorf("authorize: %w", status.Error(codes.Unauthenticated, "no grant"))
	}
	if strings.HasSuffix(req.GetResource(), ":deny") {
		return &harnesspb.AuthorizeResponse{Allowed: false, Reason: "denied by the fake"}, nil
	}
	return &harnesspb.AuthorizeResponse{Allowed: true}, nil
}

func (d *callbackDaemon) ClaimFork(context.Context, *harnesspb.ClaimForkRequest) (*harnesspb.ClaimForkResponse, error) {
	d.claimMu.Lock()
	defer d.claimMu.Unlock()
	if len(d.claims) == 0 {
		return nil, fmt.Errorf("claim: %w", status.Error(codes.Unimplemented, "no claim set"))
	}
	next := d.claims[0]
	if len(d.claims) > 1 {
		d.claims = d.claims[1:]
	}
	return next()
}

// forkDispatch is the answer of the daemon to the claim of a fork.
func forkDispatch() (*harnesspb.ClaimForkResponse, error) {
	return &harnesspb.ClaimForkResponse{
		Grant: "fork-grant", MissionId: "child-1", MissionRunId: "child-run-1", AgentRunId: "fork-run-1",
		NodeId: "node-b", Model: "model-x", Task: &typespb.Task{Id: "task-b", Goal: "scan the next host"},
	}, nil
}

// forkUnclaimed is the answer of a daemon that has not recorded the dispatch.
func forkUnclaimed() (*harnesspb.ClaimForkResponse, error) {
	st, err := status.New(codes.FailedPrecondition, "unclaimed").WithDetails(
		&errdetails.ErrorInfo{Reason: fork.ReasonForkUnclaimed, Domain: fork.ErrorDomain})
	if err != nil {
		return nil, fmt.Errorf("details: %w", err)
	}
	return nil, fmt.Errorf("unclaimed: %w", st.Err())
}

// ---------------------------------------------------------------------------
// The fake daemon: ComponentService (the pull transport of PlatformHarness)
// ---------------------------------------------------------------------------

type componentDaemon struct {
	componentpb.UnimplementedComponentServiceServer
	recorder
}

func startComponentDaemon(t *testing.T) (d *componentDaemon, addr string) {
	t.Helper()
	d = &componentDaemon{}
	addr = d.serve(t, func(s *grpc.Server) { componentpb.RegisterComponentServiceServer(s, d) })
	return d, addr
}

var compUsage = &componentpb.TokenUsage{InputTokens: 11, OutputTokens: 7}

func (d *componentDaemon) Complete(context.Context, *componentpb.CompleteRequest) (*componentpb.CompleteResponse, error) {
	return &componentpb.CompleteResponse{Response: &componentpb.LLMMessage{Role: "assistant", Content: "hello"}, Usage: compUsage}, nil
}

func (d *componentDaemon) CompleteWithTools(
	context.Context, *componentpb.CompleteWithToolsRequest,
) (*componentpb.CompleteWithToolsResponse, error) {
	return &componentpb.CompleteWithToolsResponse{
		Response: &componentpb.LLMMessage{Role: "assistant", Content: "calling"}, Usage: compUsage, FinishReason: "tool_use",
		ToolCalls: []*componentpb.ToolCallResult{{Id: "tc-1", Name: "lookup", ArgumentsJson: `{"host":"10.0.0.1"}`}},
	}, nil
}

func (d *componentDaemon) CompleteStructured(
	context.Context, *componentpb.CompleteStructuredRequest,
) (*componentpb.CompleteStructuredResponse, error) {
	return &componentpb.CompleteStructuredResponse{ResultJson: `{"safe":true}`}, nil
}

func (d *componentDaemon) CompleteStream(
	_ *componentpb.CompleteStreamRequest, s grpc.ServerStreamingServer[componentpb.CompleteStreamResponse],
) error {
	for _, m := range []*componentpb.CompleteStreamResponse{
		{Content: "he"}, {Content: "llo"}, {Done: true, Usage: compUsage},
	} {
		if err := s.Send(m); err != nil {
			return fmt.Errorf("send: %w", err)
		}
	}
	return nil
}

func (d *componentDaemon) CallTool(context.Context, *componentpb.CallToolRequest) (*componentpb.CallToolResponse, error) {
	return &componentpb.CallToolResponse{OutputJson: `"pong"`}, nil
}

func (d *componentDaemon) CallToolStream(
	_ *componentpb.CallToolStreamRequest, s grpc.ServerStreamingServer[componentpb.CallToolStreamResponse],
) error {
	for _, m := range []*componentpb.CallToolStreamResponse{
		{EventType: "progress", PayloadJson: `{"percent":50,"phase":"scan","message":"halfway"}`},
		{EventType: "partial", PayloadJson: `"part"`},
		{EventType: "warning", PayloadJson: `{"message":"slow","context":"W1"}`},
		{EventType: "result", PayloadJson: `"done"`, Done: true},
	} {
		if err := s.Send(m); err != nil {
			return fmt.Errorf("send: %w", err)
		}
	}
	return nil
}

func (d *componentDaemon) ListTools(context.Context, *componentpb.ListToolsRequest) (*componentpb.ListToolsResponse, error) {
	return &componentpb.ListToolsResponse{Tools: []*componentpb.ToolDescriptorProto{{
		Name: "echo", Version: "1.0", Description: "Echo", Tags: []string{"util"},
		InputMessageType: "google.protobuf.StringValue", OutputMessageType: "google.protobuf.StringValue",
	}}}, nil
}

func (d *componentDaemon) QueueToolWork(context.Context, *componentpb.QueueToolWorkRequest) (*componentpb.QueueToolWorkResponse, error) {
	return &componentpb.QueueToolWorkResponse{JobId: "job-7"}, nil
}

func (d *componentDaemon) ToolResults(_ *componentpb.ToolResultsRequest, s grpc.ServerStreamingServer[componentpb.ToolResultsResponse]) error {
	for _, m := range []*componentpb.ToolResultsResponse{
		{Index: 0, OutputJson: `"A"`}, {Index: 1, OutputJson: `"B"`, Done: true},
	} {
		if err := s.Send(m); err != nil {
			return fmt.Errorf("send: %w", err)
		}
	}
	return nil
}

func (d *componentDaemon) QueryPlugin(context.Context, *componentpb.QueryPluginRequest) (*componentpb.QueryPluginResponse, error) {
	return &componentpb.QueryPluginResponse{ResultJson: `{"open":true}`}, nil
}

func (d *componentDaemon) DelegateToAgent(context.Context, *componentpb.DelegateToAgentRequest) (*componentpb.DelegateToAgentResponse, error) {
	b, err := json.Marshal(map[string]any{"Status": "partial", "Output": "helped", "Findings": []string{"f-1"}})
	if err != nil {
		return nil, fmt.Errorf("marshal: %w", err)
	}
	return &componentpb.DelegateToAgentResponse{ResultJson: b}, nil
}

func (d *componentDaemon) ListAgents(context.Context, *componentpb.ListAgentsRequest) (*componentpb.ListAgentsResponse, error) {
	return &componentpb.ListAgentsResponse{Agents: []*componentpb.AgentDescriptorProto{{
		Name: "helper", Version: "3.1", Description: "Helps", Capabilities: []string{"web"}, TargetTypes: []string{"llm_chat"},
	}}}, nil
}

func (d *componentDaemon) SubmitFinding(context.Context, *componentpb.SubmitFindingRequest) (*componentpb.SubmitFindingResponse, error) {
	return &componentpb.SubmitFindingResponse{FindingId: "f-new"}, nil
}

func (d *componentDaemon) ReportStepHints(context.Context, *componentpb.ReportStepHintsRequest) (*componentpb.ReportStepHintsResponse, error) {
	return &componentpb.ReportStepHintsResponse{}, nil
}

func jsonOf(t any) []byte {
	b, err := json.Marshal(t)
	if err != nil {
		panic(err)
	}
	return b
}

func (d *componentDaemon) CreateMission(context.Context, *componentpb.CreateMissionRequest) (*componentpb.CreateMissionResponse, error) {
	return &componentpb.CreateMissionResponse{MissionJson: jsonOf(mission.MissionInfo{
		ID: "child-1", Name: "child", Status: mission.MissionStatusRunning, TargetID: "target-1", Tags: []string{"d86"},
	})}, nil
}

func (d *componentDaemon) RunMission(context.Context, *componentpb.RunMissionRequest) (*componentpb.RunMissionResponse, error) {
	return &componentpb.RunMissionResponse{}, nil
}

func (d *componentDaemon) GetMissionStatus(context.Context, *componentpb.GetMissionStatusRequest) (*componentpb.GetMissionStatusResponse, error) {
	return &componentpb.GetMissionStatusResponse{StatusJson: jsonOf(mission.MissionStatusInfo{
		Status: mission.MissionStatusRunning, Progress: 0.5, Phase: "scan", TokenUsage: 42,
	})}, nil
}

func (d *componentDaemon) WaitMission(context.Context, *componentpb.WaitMissionRequest) (*componentpb.WaitMissionResponse, error) {
	return &componentpb.WaitMissionResponse{ResultJson: jsonOf(mission.MissionResult{
		MissionID: "child-1", Status: mission.MissionStatusCompleted,
	})}, nil
}

func (d *componentDaemon) ListMissions(context.Context, *componentpb.ListMissionsRequest) (*componentpb.ListMissionsResponse, error) {
	return &componentpb.ListMissionsResponse{MissionsJson: jsonOf([]mission.MissionInfo{{ID: "child-1", Name: "child"}})}, nil
}

func (d *componentDaemon) CancelMission(context.Context, *componentpb.CancelMissionRequest) (*componentpb.CancelMissionResponse, error) {
	return &componentpb.CancelMissionResponse{}, nil
}

func (d *componentDaemon) GetMissionResults(
	context.Context, *componentpb.GetMissionResultsRequest,
) (*componentpb.GetMissionResultsResponse, error) {
	return &componentpb.GetMissionResultsResponse{ResultJson: jsonOf(mission.MissionResult{
		MissionID: "child-1", Status: mission.MissionStatusCompleted,
	})}, nil
}

// ---------------------------------------------------------------------------
// The fake setec identity socket (D74, D80)
// ---------------------------------------------------------------------------

type fakeIdentity struct {
	mu       sync.Mutex
	requests int
}

func (f *fakeIdentity) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.requests++
	if r.URL.Query().Get("audience") != fork.SandboxIdentityAudience {
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte(`{"error":"wrong audience"}`))
		return
	}
	_ = json.NewEncoder(w).Encode(map[string]any{"token": fmt.Sprintf("sbx-id-%d", f.requests)})
}

// serveIdentity serves a fake identity socket and names it in
// SETEC_IDENTITY_SOCKET for one test.
func serveIdentity(t *testing.T) *fakeIdentity {
	t.Helper()
	dir, err := os.MkdirTemp("", "id") // a Unix socket path has a short limit
	require.NoError(t, err)
	t.Cleanup(func() { _ = os.RemoveAll(dir) })
	path := filepath.Join(dir, "identity.sock")
	lis, err := (&net.ListenConfig{}).Listen(context.Background(), "unix", path)
	require.NoError(t, err)
	f := &fakeIdentity{}
	srv := &http.Server{Handler: f, ReadHeaderTimeout: time.Second}
	go func() { _ = srv.Serve(lis) }()
	t.Cleanup(func() { _ = srv.Close() })
	t.Setenv(fork.EnvIdentitySocket, path)
	return f
}

// host is a sandbox id that a test changes, as a fork changes it.
type host struct {
	mu   sync.Mutex
	name string
}

func (h *host) set(n string) { h.mu.Lock(); h.name = n; h.mu.Unlock() }
func (h *host) read() (string, error) {
	h.mu.Lock()
	defer h.mu.Unlock()
	return h.name, nil
}
