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
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/a2aproject/a2a-go/v2/a2a"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/wrapperspb"

	sdk "github.com/zeroroot-ai/sdk"
	"github.com/zeroroot-ai/sdk/agent"
	graphragpb "github.com/zeroroot-ai/sdk/api/gen/gibson/graphrag/v1"
	"github.com/zeroroot-ai/sdk/finding"
	"github.com/zeroroot-ai/sdk/mission"
	"github.com/zeroroot-ai/sdk/openresponses"
	"github.com/zeroroot-ai/sdk/planning"
	"github.com/zeroroot-ai/sdk/plugin"
	"github.com/zeroroot-ai/sdk/schema"
	"github.com/zeroroot-ai/sdk/tool"
	"github.com/zeroroot-ai/sdk/types"
)

const (
	agentName    = "agent-harness"
	agentVersion = "1.0.0"
	// llmSlot is the one LLM slot the agent uses.
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

// New builds the agent with the options of the sdk.
func New() (agent.Agent, error) {
	a, err := sdk.NewAgent(
		sdk.WithName(agentName),
		sdk.WithVersion(agentVersion),
		sdk.WithDescription("Calls each method of agent.Harness once"),
		sdk.WithTargetTypes("llm_chat"),
		sdk.WithExecuteFunc(execute),
	)
	if err != nil {
		return nil, fmt.Errorf("build the agent: %w", err)
	}
	return a, nil
}

// execute calls each method of agent.Harness. It is the same code for each
// transport: the harness that the caller passes decides where the calls go.
func execute(ctx context.Context, h agent.Harness, task agent.Task) (agent.Result, error) {
	ctx, span := h.Tracer().Start(ctx, "agent-harness.execute")
	defer span.End()

	rep := &Report{}
	callContext(ctx, h, task, rep)
	callLLM(ctx, h, task, rep)
	ids := callTools(ctx, h, task, rep)
	callPlugins(ctx, h, rep)
	callDelegation(ctx, h, rep)
	callWorld(ctx, h, rep, ids)
	callPlanning(ctx, h, rep)
	callWorkspace(ctx, h, rep)
	callKnowledge(ctx, h, rep)
	callMissions(ctx, h, rep)
	callUsage(h, rep)

	return agent.Result{Status: a2a.TaskStateCompleted, Output: rep}, nil
}

// callContext reads the context accessors and asks for authorization.
func callContext(ctx context.Context, h agent.Harness, task agent.Task, rep *Report) {
	m := h.Mission()
	rep.add(Step{Call: "Mission", Value: m, Summary: m.ID + " " + m.Name})
	target := h.Target()
	rep.add(Step{Call: "Target", Value: target, Summary: target.ID + " " + target.Name + " " + target.URL()})
	h.Logger().InfoContext(ctx, "task start", "task", task.ID, "goal", task.Goal,
		"agent_run", task.Run.AgentRunID, "mission_run", task.Run.MissionRunID,
		"max_turns", task.Constraints.MaxTurns, "has_job", task.Job != nil, "metadata_keys", len(task.Metadata))
	recordErr(rep, "Authorize", h.Authorize(ctx, "scan", "target:"+target.ID))
}

func callLLM(ctx context.Context, h agent.Harness, task agent.Task, rep *Report) {
	model, err := h.Model(ctx, llmSlot)
	record(rep, "Model", model, err, func(m *openresponses.Client) string {
		return fmt.Sprintf("model=%s provider=%s context_window=%d max_output_tokens=%d",
			m.Model, m.Provider, m.ContextWindow, m.MaxOutputTokens)
	})
	if err != nil {
		return
	}
	// The pinned Agent Skills come first, then the goal. The agent holds these
	// items from the first message to the last tool result.
	input := append(append([]openresponses.Item(nil), task.SkillItems...), openresponses.NewUserMessage(task.Goal))

	temperature := 0.2
	maxTokens := 256
	// The task limits the tokens of the run. The SDK does not enforce the limits of a
	// task: the agent reads them and applies them.
	if limit := task.Constraints.MaxTokens; limit > 0 && limit < maxTokens {
		maxTokens = limit
	}
	c, err := model.Create(ctx, &openresponses.Request{Input: input, Temperature: &temperature, MaxOutputTokens: &maxTokens})
	record(rep, "Create", c, err, describeResponse)

	tools := []openresponses.Tool{{
		Name:        "lookup",
		Description: "Look up a host",
		Parameters:  map[string]any{"type": "object", "properties": map[string]any{"host": map[string]any{"type": "string"}}},
	}}
	ct, err := model.Create(ctx, &openresponses.Request{
		Input: input, Tools: tools, ToolChoice: openresponses.ToolChoiceFunction("lookup"),
	})
	record(rep, "CreateWithTools", ct, err, describeResponse)

	type verdict struct {
		Safe bool `json:"safe"`
	}
	var v verdict
	_, err = model.CreateStructured(ctx, &openresponses.Request{Input: input}, "verdict", schema.FromType(verdict{}), &v)
	record(rep, "CreateStructured", v, err, func(v verdict) string { return fmt.Sprintf("%+v", v) })

	st, err := model.Stream(ctx, &openresponses.Request{Input: input})
	if err != nil {
		rep.add(Step{Call: "Stream", Err: err})
		return
	}
	defer func() { _ = st.Close() }()
	var text strings.Builder
	final, err := st.Collect(func(d string) { text.WriteString(d) })
	if err != nil {
		rep.add(Step{Call: "Stream", Err: err})
		return
	}
	s := streamed{Text: text.String(), ToolCalls: len(final.FunctionCalls()), Status: final.Status}
	if final.Usage != nil {
		s.Usage = *final.Usage
	}
	rep.add(Step{Call: "Stream", Value: s, Summary: s.String()})
}

// streamed is what the agent read from the events of a stream.
type streamed struct {
	Text      string
	ToolCalls int
	Status    string
	Usage     openresponses.Usage
}

func (s streamed) String() string {
	return fmt.Sprintf("text=%q tool_calls=%d status=%s total=%d", s.Text, s.ToolCalls, s.Status, s.Usage.TotalTokens)
}

func describeResponse(r *openresponses.Response) string {
	fcs := r.FunctionCalls()
	calls := make([]string, 0, len(fcs))
	for _, fc := range fcs {
		calls = append(calls, fc.CallID+":"+fc.Name+":"+fc.Arguments)
	}
	var total int
	if r.Usage != nil {
		total = r.Usage.TotalTokens
	}
	out := fmt.Sprintf("text=%q status=%s calls=[%s] total=%d", r.Text(), r.Status, strings.Join(calls, ","), total)
	// An item or a part of a type that the SDK has no struct for comes as a raw value.
	// The agent can tell which types came back.
	if kinds := rawKinds(r); len(kinds) > 0 {
		out += " raw=" + strings.Join(kinds, ",")
	}
	// A response that ends early says why. The agent must not treat it as complete.
	if r.Status == openresponses.StatusIncomplete && r.IncompleteDetails != nil {
		out += " incomplete=" + r.IncompleteDetails.Reason
	}
	return out
}

// rawKinds lists the types of the raw items and raw parts of a response.
func rawKinds(r *openresponses.Response) []string {
	var kinds []string
	for _, it := range r.Output {
		switch v := it.(type) {
		case openresponses.RawItem:
			kinds = append(kinds, v.Type)
		case openresponses.Message:
			for _, p := range v.Content {
				if raw, ok := p.(openresponses.RawPart); ok {
					kinds = append(kinds, raw.Type)
				}
			}
		}
	}
	return kinds
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
	Index      int
	Output     string
	Err        error
	ToolCallID string
}

func gatherToolResults(ch <-chan agent.QueuedToolResult) []queuedResult {
	var out []queuedResult
	for r := range ch {
		q := queuedResult{Index: r.Index, Err: r.Error, ToolCallID: r.ToolCallID}
		if sv, ok := r.Output.(*wrapperspb.StringValue); ok {
			q.Output = sv.GetValue()
		}
		out = append(out, q)
	}
	return out
}

// toolAllowed applies the tool limits of a task. The SDK does not enforce them.
func toolAllowed(c agent.TaskConstraints, name string) bool {
	if slices.Contains(c.BlockedTools, name) {
		return false
	}
	return len(c.AllowedTools) == 0 || slices.Contains(c.AllowedTools, name)
}

func callTools(ctx context.Context, h agent.Harness, task agent.Task, rep *Report) *toolCallIDs {
	// ReceiveToolCallID gives the id that the daemon recorded for each call. An
	// agent cites it with Finding.CiteToolCalls.
	ids := &toolCallIDs{}
	if !toolAllowed(task.Constraints, "echo") {
		rep.add(Step{Call: "CallToolProto", Err: errors.New("the task does not allow the tool echo")})
		return ids
	}
	reply := &wrapperspb.StringValue{}
	err := h.CallToolProto(ctx, "echo", wrapperspb.String("ping"), reply, agent.ReceiveToolCallID(&ids.Call))
	record(rep, "CallToolProto", reply.GetValue(), err, func(v string) string { return v })

	events := &toolEvents{}
	streamReply := &wrapperspb.StringValue{}
	err = h.CallToolProtoStream(ctx, "echo", wrapperspb.String("stream"), streamReply, events,
		agent.ReceiveToolCallID(&ids.Stream))
	record(rep, "CallToolProtoStream", events, err, func(e *toolEvents) string {
		return fmt.Sprintf("progress=%v partials=%d warnings=%v errors=%v reply=%q",
			e.Progress, e.Partials, e.Warnings, e.Errors, streamReply.GetValue())
	})

	tools, err := h.ListTools(ctx)
	record(rep, "ListTools", tools, err, describeTools)

	job, err := h.QueueToolWork(ctx, "echo", []proto.Message{wrapperspb.String("a"), wrapperspb.String("b")})
	record(rep, "QueueToolWork", job, err, func(id string) string { return id })
	if err != nil {
		return ids
	}
	results := gatherToolResults(h.ToolResults(ctx, job))
	lines := make([]string, 0, len(results))
	for _, r := range results {
		lines = append(lines, fmt.Sprintf("%d:%s:%v", r.Index, r.Output, r.Err))
	}
	rep.add(Step{Call: "ToolResults", Value: results, Summary: strings.Join(lines, ",")})
	for _, r := range results {
		ids.Queued = append(ids.Queued, r.ToolCallID)
	}
	rep.add(Step{Call: "ToolCallIDs", Value: ids, Summary: fmt.Sprintf("%+v", *ids)})
	return ids
}

// toolCallIDs holds the ids that the daemon recorded for the tool calls of the agent.
type toolCallIDs struct {
	Call   string
	Stream string
	Queued []string
}

func callPlugins(ctx context.Context, h agent.Harness, rep *Report) {
	v, err := h.QueryPlugin(ctx, "scanner", "probe", map[string]any{"host": "10.0.0.1"})
	record(rep, "QueryPlugin", v, err, func(v any) string { return fmt.Sprintf("%v", v) })

	ps, err := h.ListPlugins(ctx)
	record(rep, "ListPlugins", ps, err, describePlugins)
}

func callDelegation(ctx context.Context, h agent.Harness, rep *Report) {
	msg := a2a.NewMessage(a2a.MessageRoleUser, a2a.NewTextPart("check the host"))
	task, err := h.Delegate(ctx, "helper", msg)
	record(rep, "Delegate", task, err, describeTask)

	// A bank of always-on agents is a delegation target too.
	banked, err := h.Delegate(ctx, "helper", msg, agent.WithBank("bank-1"))
	record(rep, "DelegateToBank", banked, err, describeTask)
}

func describeTask(t *a2a.Task) string {
	var artifacts []string
	for _, a := range t.Artifacts {
		for _, p := range a.Parts {
			artifacts = append(artifacts, p.Text())
		}
	}
	return fmt.Sprintf("state=%s artifacts=%v", t.Status.State, artifacts)
}

func callWorld(ctx context.Context, h agent.Harness, rep *Report, ids *toolCallIDs) {
	f := &finding.Finding{
		Title:       "Open admin port",
		Description: "Port 8443 answers without a login",
		Category:    "exposure",
		Severity:    finding.SeverityHigh,
		Confidence:  0.9,
	}
	// The tool calls that back the finding. The platform builds recorded evidence from them.
	f.CiteToolCalls(ids.Call, ids.Stream)
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

	cms, err := h.Countermeasures(ctx, "T1190")
	record(rep, "Countermeasures", cms, err, describeCountermeasures)
}

func describeCountermeasures(cs []agent.Countermeasure) string {
	out := make([]string, 0, len(cs))
	for _, c := range cs {
		out = append(out, c.ID+" "+c.Name+" "+c.Tactic)
	}
	return strings.Join(out, "; ")
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

	pending := mission.MissionStatusWorking
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
