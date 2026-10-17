// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Zero Root AI

// Command agent-harness is a first-party Go agent that calls each method of
// agent.Harness once and reads each result type it gets back.
//
// It has two jobs. It shows an agent author what the Harness offers, and it is
// the first-party consumer of the Go Harness (D86): the unwired-code guard of
// the sdk counts these reads, and harness_test.go runs the agent against a fake
// daemon through both serve.CallbackHarness and serve.PlatformHarness.
//
// The agent does not stop at the first error. A Harness call that the
// transport does not offer (the platform pull transport has no WorldView, for
// example) is recorded in the Report with its error, and the agent goes on.
package main

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/wrapperspb"

	sdk "github.com/zeroroot-ai/sdk"
	"github.com/zeroroot-ai/sdk/agent"
	graphragpb "github.com/zeroroot-ai/sdk/api/gen/gibson/graphrag/v1"
	"github.com/zeroroot-ai/sdk/finding"
	"github.com/zeroroot-ai/sdk/llm"
	"github.com/zeroroot-ai/sdk/mission"
	"github.com/zeroroot-ai/sdk/planning"
	"github.com/zeroroot-ai/sdk/plugin"
	"github.com/zeroroot-ai/sdk/tool"
	"github.com/zeroroot-ai/sdk/types"
)

const (
	agentName    = "agent-harness"
	agentVersion = "1.0.0"
	// llmSlot is the one LLM slot the agent declares and uses.
	llmSlot = "main"
)

// Step is one Harness call: its name, the value it returned, a one-line
// summary of the fields the agent read from that value, and its error.
type Step struct {
	Call    string
	Value   any
	Summary string
	Err     error
}

// Report is the Output of the result of the agent. Steps are in call order.
type Report struct {
	Steps []Step
}

// Step returns the first step with the given call name.
func (r *Report) Step(call string) (Step, bool) {
	for _, s := range r.Steps {
		if s.Call == call {
			return s, true
		}
	}
	return Step{}, false
}

func (r *Report) add(s Step) { r.Steps = append(r.Steps, s) }

// record adds the outcome of a call that returns a value.
func record[T any](r *Report, call string, v T, err error, describe func(T) string) {
	if err != nil {
		r.add(Step{Call: call, Err: err})
		return
	}
	r.add(Step{Call: call, Value: v, Summary: describe(v)})
}

// recordErr adds the outcome of a call that returns only an error.
func recordErr(r *Report, call string, err error) {
	r.add(Step{Call: call, Err: err})
}

// slotDeclarer is implemented by the agent that sdk.NewAgent builds.
type slotDeclarer interface {
	LLMSlots() []llm.SlotDefinition
}

// New builds the agent with the options of the sdk.
func New() (agent.Agent, error) {
	a, err := sdk.NewAgent(
		sdk.WithName(agentName),
		sdk.WithVersion(agentVersion),
		sdk.WithDescription("Calls each method of agent.Harness once"),
		sdk.WithTargetTypes("llm_chat"),
		sdk.WithLLMSlot(llmSlot, llm.SlotRequirements{
			MinContextWindow: 8000,
			PreferredModels:  []string{"claude"},
		}),
		sdk.WithExecuteFunc(execute),
	)
	if err != nil {
		return nil, fmt.Errorf("build the agent: %w", err)
	}
	return a, nil
}

// SlotSummary names the LLM slots that the agent declares and what each needs.
// The agent calls it at start, so a slot that execute uses is declared.
func SlotSummary(a agent.Agent) []string {
	d, ok := a.(slotDeclarer)
	if !ok {
		return nil
	}
	var out []string
	for _, s := range d.LLMSlots() {
		out = append(out, fmt.Sprintf("%s required=%t window=%d models=%s",
			s.Name, s.Required, s.MinContextWindow,
			strings.Join(s.PreferredModels, ",")))
	}
	return out
}

// execute calls each method of agent.Harness. It is the same code for each
// transport: the harness that the caller passes decides where the calls go.
func execute(ctx context.Context, h agent.Harness, task agent.Task) (agent.Result, error) {
	ctx, span := h.Tracer().Start(ctx, "agent-harness.execute")
	defer span.End()

	rep := &Report{}
	callContext(ctx, h, task, rep)
	callLLM(ctx, h, task, rep)
	callTools(ctx, h, rep)
	callPlugins(ctx, h, rep)
	callDelegation(ctx, h, rep)
	callWorld(ctx, h, rep)
	callPlanning(ctx, h, rep)
	callWorkspace(ctx, h, rep)
	callKnowledge(ctx, h, rep)
	callMissions(ctx, h, rep)
	callUsage(h, rep)

	return agent.Result{Status: agent.StatusSuccess, Output: rep}, nil
}

// callContext reads the context accessors and asks for authorization.
func callContext(ctx context.Context, h agent.Harness, task agent.Task, rep *Report) {
	m := h.Mission()
	rep.add(Step{Call: "Mission", Value: m, Summary: m.ID + " " + m.Name})
	target := h.Target()
	rep.add(Step{Call: "Target", Value: target, Summary: target.ID + " " + target.Name + " " + target.URL()})
	h.Logger().InfoContext(ctx, "task start", "task", task.ID, "goal", task.Goal)
	recordErr(rep, "Authorize", h.Authorize(ctx, "scan", "target:"+target.ID))
}

func callLLM(ctx context.Context, h agent.Harness, task agent.Task, rep *Report) {
	msgs := []llm.Message{{Role: llm.RoleUser, Content: task.Goal}}

	c, err := h.Complete(ctx, llmSlot, msgs, llm.WithTemperature(0.2), llm.WithMaxTokens(256))
	record(rep, "Complete", c, err, describeCompletion)

	tools := []llm.ToolDef{{
		Name:        "lookup",
		Description: "Look up a host",
		Parameters:  map[string]any{"type": "object", "properties": map[string]any{"host": map[string]any{"type": "string"}}},
	}}
	ct, err := h.CompleteWithTools(ctx, llmSlot, msgs, tools)
	record(rep, "CompleteWithTools", ct, err, describeCompletion)

	type verdict struct {
		Safe bool `json:"safe"`
	}
	cs, err := h.CompleteStructured(ctx, llmSlot, msgs, verdict{})
	record(rep, "CompleteStructured", cs, err, func(v any) string { return fmt.Sprintf("%v", v) })
	ca, err := h.CompleteStructuredAny(ctx, llmSlot, msgs, verdict{})
	record(rep, "CompleteStructuredAny", ca, err, func(v any) string { return fmt.Sprintf("%v", v) })

	ch, err := h.Stream(ctx, llmSlot, msgs)
	if err != nil {
		rep.add(Step{Call: "Stream", Err: err})
		return
	}
	s := drainStream(ch)
	rep.add(Step{Call: "Stream", Value: s, Summary: s.String()})
}

// streamed is what the agent read from the chunks of a stream.
type streamed struct {
	Text      string
	ToolCalls int
	Finish    string
	Usage     llm.TokenUsage
}

func (s streamed) String() string {
	return fmt.Sprintf("text=%q tool_calls=%d finish=%s total=%d", s.Text, s.ToolCalls, s.Finish, s.Usage.TotalTokens)
}

func drainStream(ch <-chan llm.StreamChunk) streamed {
	var s streamed
	var text strings.Builder
	for chunk := range ch {
		text.WriteString(chunk.Delta)
		s.ToolCalls += len(chunk.ToolCalls)
		if chunk.FinishReason != "" {
			s.Finish = chunk.FinishReason
		}
		if chunk.Usage != nil {
			s.Usage = *chunk.Usage
		}
	}
	s.Text = text.String()
	return s
}

func describeCompletion(c *llm.CompletionResponse) string {
	calls := make([]string, 0, len(c.ToolCalls))
	for _, tc := range c.ToolCalls {
		calls = append(calls, tc.ID+":"+tc.Name+":"+tc.Arguments)
	}
	return fmt.Sprintf("content=%q finish=%s calls=[%s] in=%d out=%d total=%d",
		c.Content, c.FinishReason, strings.Join(calls, ","),
		c.Usage.InputTokens, c.Usage.OutputTokens, c.Usage.TotalTokens)
}

// toolEvents records what a tool stream tells the agent.
type toolEvents struct {
	Progress []string
	Partials int
	Warnings []string
	Errors   []string
}

func (e *toolEvents) OnProgress(percent int, phase, message string) {
	e.Progress = append(e.Progress, fmt.Sprintf("%d:%s:%s", percent, phase, message))
}

func (e *toolEvents) OnPartial(_ proto.Message, _ bool) { e.Partials++ }

func (e *toolEvents) OnWarning(message, warnContext string) {
	e.Warnings = append(e.Warnings, message+":"+warnContext)
}

func (e *toolEvents) OnError(err error, fatal bool) {
	e.Errors = append(e.Errors, fmt.Sprintf("%v fatal=%t", err, fatal))
}

// queuedResult is what the agent read from a queued tool result.
type queuedResult struct {
	Index  int
	Output string
	Err    error
}

func gatherToolResults(ch <-chan agent.QueuedToolResult) []queuedResult {
	var out []queuedResult
	for r := range ch {
		q := queuedResult{Index: r.Index, Err: r.Error}
		if sv, ok := r.Output.(*wrapperspb.StringValue); ok {
			q.Output = sv.GetValue()
		}
		out = append(out, q)
	}
	return out
}

func callTools(ctx context.Context, h agent.Harness, rep *Report) {
	reply := &wrapperspb.StringValue{}
	err := h.CallToolProto(ctx, "echo", wrapperspb.String("ping"), reply)
	record(rep, "CallToolProto", reply.GetValue(), err, func(v string) string { return v })

	events := &toolEvents{}
	streamReply := &wrapperspb.StringValue{}
	err = h.CallToolProtoStream(ctx, "echo", wrapperspb.String("stream"), streamReply, events)
	record(rep, "CallToolProtoStream", events, err, func(e *toolEvents) string {
		return fmt.Sprintf("progress=%v partials=%d warnings=%v errors=%v reply=%q",
			e.Progress, e.Partials, e.Warnings, e.Errors, streamReply.GetValue())
	})

	tools, err := h.ListTools(ctx)
	record(rep, "ListTools", tools, err, describeTools)

	job, err := h.QueueToolWork(ctx, "echo", []proto.Message{wrapperspb.String("a"), wrapperspb.String("b")})
	record(rep, "QueueToolWork", job, err, func(id string) string { return id })
	if err != nil {
		return
	}
	results := gatherToolResults(h.ToolResults(ctx, job))
	lines := make([]string, 0, len(results))
	for _, r := range results {
		lines = append(lines, fmt.Sprintf("%d:%s:%v", r.Index, r.Output, r.Err))
	}
	rep.add(Step{Call: "ToolResults", Value: results, Summary: strings.Join(lines, ",")})
}

func callPlugins(ctx context.Context, h agent.Harness, rep *Report) {
	v, err := h.QueryPlugin(ctx, "scanner", "probe", map[string]any{"host": "10.0.0.1"})
	record(rep, "QueryPlugin", v, err, func(v any) string { return fmt.Sprintf("%v", v) })

	ps, err := h.ListPlugins(ctx)
	record(rep, "ListPlugins", ps, err, describePlugins)
}

func callDelegation(ctx context.Context, h agent.Harness, rep *Report) {
	res, err := h.DelegateToAgent(ctx, "helper", agent.Task{ID: "sub-1", Goal: "check the host"})
	record(rep, "DelegateToAgent", res, err, describeResult)

	as, err := h.ListAgents(ctx)
	record(rep, "ListAgents", as, err, describeAgents)
}

func describeResult(r agent.Result) string {
	msg := ""
	if r.Error != nil {
		msg = r.Error.Error()
	}
	return fmt.Sprintf("status=%s output=%v findings=%v error=%q", r.Status, r.Output, r.Findings, msg)
}

func describeAgents(ds []agent.Descriptor) string {
	out := make([]string, 0, len(ds))
	for _, d := range ds {
		out = append(out, fmt.Sprintf("%s %s %q caps=%v targets=%v techniques=%v",
			d.Name, d.Version, d.Description, d.Capabilities, d.TargetTypes, d.TechniqueTypes))
	}
	return strings.Join(out, "; ")
}

func callWorld(ctx context.Context, h agent.Harness, rep *Report) {
	f := &finding.Finding{
		Title:       "Open admin port",
		Description: "Port 8443 answers without a login",
		Category:    "exposure",
		Severity:    finding.SeverityHigh,
		Confidence:  0.9,
	}
	recordErr(rep, "SubmitFinding", h.SubmitFinding(ctx, f))
	recordErr(rep, "Observe", h.Observe(ctx, agent.DomainObservation{Name: "example.com"}))
	recordErr(rep, "PlaceBet", h.PlaceBet(ctx, agent.Bet{
		HypothesisID: "hyp-1", StakingAgent: agentName, Confidence: 0.7, Technique: "T1190",
	}))

	view, err := h.WorldView(ctx, agent.Handle("host:10.0.0.1"))
	record(rep, "WorldView", view, err, describeWorld)
}

func describeWorld(v agent.WorldView) string {
	out := make([]string, 0, len(v.Entities))
	for _, e := range v.Entities {
		out = append(out, fmt.Sprintf("%s %s %q %v", e.Handle, e.Kind, e.Label, e.Attributes))
	}
	return fmt.Sprintf("%s truncated=%t", strings.Join(out, "; "), v.Truncated)
}

func callPlanning(ctx context.Context, h agent.Harness, rep *Report) {
	if pc := h.PlanContext(); pc != nil {
		rep.add(Step{Call: "PlanContext", Value: pc, Summary: fmt.Sprintf("step %d of %d budget %d left %d next %v",
			pc.CurrentStepIndex(), pc.TotalSteps(), pc.StepBudget(), pc.MissionBudgetRemaining(), pc.RemainingSteps())})
	} else {
		rep.add(Step{Call: "PlanContext", Summary: "no plan"})
	}
	recordErr(rep, "ReportStepHints", h.ReportStepHints(ctx, &planning.StepHints{}))
}

func callWorkspace(ctx context.Context, h agent.Harness, rep *Report) {
	ws := h.Workspace()
	if ws == nil {
		rep.add(Step{Call: "Workspace", Summary: "none"})
	} else {
		data, err := ws.ReadFile(ctx, "README.md")
		record(rep, "Workspace", data, err, func(b []byte) string {
			return fmt.Sprintf("%s %s %q", ws.Name(), ws.Path(), b)
		})
	}
	all := h.Workspaces()
	rep.add(Step{Call: "Workspaces", Value: all, Summary: strconv.Itoa(len(all))})
}

func callKnowledge(ctx context.Context, h agent.Harness, rep *Report) {
	nodes, err := h.QueryNodes(ctx, &graphragpb.GraphQuery{})
	record(rep, "QueryNodes", nodes, err, func(v []*graphragpb.QueryResult) string { return strconv.Itoa(len(v)) })

	attacks, err := h.FindSimilarAttacks(ctx, "sql injection", 3)
	record(rep, "FindSimilarAttacks", attacks, err, func(v []*graphragpb.AttackPattern) string { return strconv.Itoa(len(v)) })

	chains, err := h.GetAttackChains(ctx, "T1190", 2)
	record(rep, "GetAttackChains", chains, err, func(v []*graphragpb.AttackChain) string { return strconv.Itoa(len(v)) })

	similar, err := h.FindSimilarFindings(ctx, "f-1", 4)
	record(rep, "FindSimilarFindings", similar, err, func(v []*graphragpb.FindingNode) string { return strconv.Itoa(len(v)) })

	related, err := h.GetRelatedFindings(ctx, "f-1")
	record(rep, "GetRelatedFindings", related, err, func(v []*graphragpb.FindingNode) string { return strconv.Itoa(len(v)) })

	callFindings(ctx, h, rep)
}

func callFindings(ctx context.Context, h agent.Harness, rep *Report) {
	filter := finding.Filter{MissionID: "mission-1", AgentName: agentName, Tags: []string{"web"}}
	fs, err := h.GetFindings(ctx, filter)
	record(rep, "GetFindings", fs, err, describeFindings)
	rs, err := h.GetRunFindings(ctx, agent.RunScopePrevious, filter)
	record(rep, "GetRunFindings", rs, err, describeFindings)

	runs, err := h.GetMissionRunHistory(ctx)
	record(rep, "GetMissionRunHistory", runs, err, describeRuns)

	apps, err := h.ApplicationFindings(ctx, "payments", []string{"open"}, 10)
	record(rep, "ApplicationFindings", apps, err, describeApplicationFindings)
}

func describeFindings(fs []*finding.Finding) string {
	out := make([]string, 0, len(fs))
	for _, f := range fs {
		out = append(out, f.Title+":"+f.Severity.String())
	}
	return strings.Join(out, ",")
}

func describeApplicationFindings(fs []agent.ApplicationFinding) string {
	out := make([]string, 0, len(fs))
	for _, f := range fs {
		out = append(out, fmt.Sprintf("%s %s %s %s place=%s/%s reach=%t exposed=%t deploy=%s image=%s prio=%s/%s/%s",
			f.FindingID, f.Status, f.Severity, f.VulnerabilityID, f.PlaceLabel, f.PlaceKey,
			f.Reachable, f.Exposed, f.DeploymentKey, f.ImageKey, f.Priority, f.PriorityRule, f.PriorityReason))
	}
	return strings.Join(out, "; ")
}

func callMissions(ctx context.Context, h agent.Harness, rep *Report) {
	info, err := h.CreateMission(ctx, map[string]any{"name": "child"}, "target-1", &mission.CreateMissionOpts{
		Name: "child", Tags: []string{"d86"}, StartsFromCallerState: true,
	})
	record(rep, "CreateMission", info, err, describeMission)
	if err != nil {
		return
	}
	recordErr(rep, "RunMission", h.RunMission(ctx, info.ID, &mission.RunMissionOpts{Wait: true, Timeout: 30 * time.Second}))

	st, err := h.GetMissionStatus(ctx, info.ID)
	record(rep, "GetMissionStatus", st, err, func(s *mission.MissionStatusInfo) string {
		return fmt.Sprintf("%s progress=%.1f phase=%s counts=%v tokens=%d took=%s err=%q",
			s.Status, s.Progress, s.Phase, s.FindingCounts, s.TokenUsage, s.Duration, s.Error)
	})

	waited, err := h.WaitForMission(ctx, info.ID, 5*time.Second)
	record(rep, "WaitForMission", waited, err, describeMissionResult)

	pending := mission.MissionStatusRunning
	list, err := h.ListMissions(ctx, &mission.MissionFilter{Status: &pending, Tags: []string{"d86"}, Limit: 5})
	record(rep, "ListMissions", list, err, func(ms []*mission.MissionInfo) string {
		out := make([]string, 0, len(ms))
		for _, m := range ms {
			out = append(out, describeMission(m))
		}
		return strings.Join(out, "; ")
	})

	recordErr(rep, "CancelMission", h.CancelMission(ctx, info.ID))

	results, err := h.GetMissionResults(ctx, info.ID)
	record(rep, "GetMissionResults", results, err, describeMissionResult)
}

func describeMission(m *mission.MissionInfo) string {
	return fmt.Sprintf("%s %q %s target=%s parent=%s tags=%v", m.ID, m.Name, m.Status, m.TargetID, m.ParentMissionID, m.Tags)
}

func describeMissionResult(r *mission.MissionResult) string {
	return fmt.Sprintf("%s %s findings=%d output=%v err=%q tokens=%d calls=%d/%d count=%d took=%s",
		r.MissionID, r.Status, len(r.Findings), r.Output, r.Error,
		r.Metrics.TokensUsed, r.Metrics.ToolCalls, r.Metrics.AgentCalls, r.Metrics.FindingsCount, r.Metrics.Duration)
}

func callUsage(h agent.Harness, rep *Report) {
	usage := h.TokenUsage()
	total := usage.Total()
	rep.add(Step{Call: "TokenUsage", Value: total, Summary: fmt.Sprintf("total=%d slots=%v main=%d",
		total.TotalTokens, usage.Slots(), usage.BySlot(llmSlot).TotalTokens)})
}

// Unavailable reports whether a step failed because the transport lacks the
// call, and not because the daemon refused it.
func (s Step) Unavailable() bool {
	return errors.Is(s.Err, agent.ErrKnowledgeUnavailable) ||
		(s.Err != nil && strings.Contains(s.Err.Error(), "not supported in platform pull-mode"))
}

func describeTools(ts []tool.Descriptor) string {
	out := make([]string, 0, len(ts))
	for _, t := range ts {
		out = append(out, fmt.Sprintf("%s %s %q tags=%v in=%s out=%s",
			t.Name, t.Version, t.Description, t.Tags, t.InputMessageType, t.OutputMessageType))
	}
	return strings.Join(out, "; ")
}

func describePlugins(ps []plugin.Descriptor) string {
	out := make([]string, 0, len(ps))
	for _, p := range ps {
		methods := make([]string, 0, len(p.Methods))
		for _, m := range p.Methods {
			methods = append(methods, m.Name)
		}
		out = append(out, fmt.Sprintf("%s %s %q methods=%v", p.Name, p.Version, p.Description, methods))
	}
	return strings.Join(out, "; ")
}

func describeRuns(rs []types.MissionRunSummary) string {
	out := make([]string, 0, len(rs))
	for _, r := range rs {
		done := "open"
		if r.CompletedAt != nil {
			done = "done"
		}
		out = append(out, fmt.Sprintf("%s #%d %s findings=%d %s", r.MissionID, r.RunNumber, r.Status, r.FindingsCount, done))
	}
	return strings.Join(out, "; ")
}

// shutdowner is implemented by the agent that sdk.NewAgent builds.
type shutdowner interface {
	Shutdown(ctx context.Context) error
}

// Shutdown releases what the agent holds, if it holds anything.
func Shutdown(ctx context.Context, a agent.Agent) error {
	s, ok := a.(shutdowner)
	if !ok {
		return nil
	}
	if err := s.Shutdown(ctx); err != nil {
		return fmt.Errorf("shut the agent down: %w", err)
	}
	return nil
}

// Describe writes one line for each step: the call, the type of the value, the
// summary and the error.
func Describe(r *Report) string {
	var b strings.Builder
	for _, s := range r.Steps {
		fmt.Fprintf(&b, "%s: %T %s", s.Call, s.Value, s.Summary)
		if s.Err != nil {
			fmt.Fprintf(&b, " error=%v", s.Err)
		}
		b.WriteByte('\n')
	}
	return b.String()
}
