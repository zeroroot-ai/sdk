// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Zero Root AI

package main

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"google.golang.org/grpc/credentials"
	"google.golang.org/protobuf/proto"

	"github.com/zeroroot-ai/sdk/agent"
	harnesspb "github.com/zeroroot-ai/sdk/api/gen/gibson/harness/v1"
	"github.com/zeroroot-ai/sdk/fork"
	"github.com/zeroroot-ai/sdk/serve"
	"github.com/zeroroot-ai/sdk/types"
)

// CallbackOptions say how the agent reaches the HarnessCallbackService of the
// daemon. Each field maps to one option of the callback client.
type CallbackOptions struct {
	// Endpoint is the gRPC address of the daemon.
	Endpoint string
	// Token is the capability grant that the client sends on each call.
	Token string
	// TLS is the TLS config of the connection.
	TLS *tls.Config
	// Transport replaces TLS with ready transport credentials, for example the
	// mTLS credentials of SPIFFE.
	Transport credentials.TransportCredentials
	// PerRPC puts a grant on each call. A client with PerRPC cannot claim a fork.
	PerRPC credentials.PerRPCCredentials
	// Mission is the mission context that the harness reports.
	Mission types.MissionContext
	// MissionRunID is the mission run that the daemon gave the sandbox. The
	// client sends it on each call, and Authorize needs it.
	MissionRunID string
}

func (o CallbackOptions) clientOptions() []serve.CallbackClientOption {
	var opts []serve.CallbackClientOption
	if o.Token != "" {
		opts = append(opts, serve.WithCallbackToken(o.Token))
	}
	if o.TLS != nil {
		opts = append(opts, serve.WithCallbackTLS(o.TLS))
	}
	if o.Transport != nil {
		opts = append(opts, serve.WithCallbackTransportCredentials(o.Transport))
	}
	if o.PerRPC != nil {
		opts = append(opts, serve.WithCallbackCredentials(o.PerRPC))
	}
	return opts
}

// Dial connects to the daemon and returns the client and the Harness on it.
func Dial(ctx context.Context, o CallbackOptions) (*serve.CallbackClient, *serve.CallbackHarness, error) {
	client, err := serve.NewCallbackClient(o.Endpoint, o.clientOptions()...)
	if err != nil {
		return nil, nil, fmt.Errorf("make the callback client: %w", err)
	}
	if err := client.Connect(ctx); err != nil {
		return nil, nil, fmt.Errorf("connect the callback client: %w", err)
	}
	//nolint:contextcheck // NewCallbackHarness reads the taxonomy under its own timeout
	if o.MissionRunID != "" {
		client.SetFullContext(serve.TaskContextParams{MissionID: o.Mission.ID, MissionRunID: o.MissionRunID})
	}
	return client, serve.NewCallbackHarness(client, serve.WithCallbackMission(o.Mission)), nil
}

// RunTask runs one task of the agent on a callback Harness. It first tells the
// client which task, mission and run the next calls belong to.
func RunTask(
	ctx context.Context, a agent.Agent, client *serve.CallbackClient, h agent.Harness,
	p serve.TaskContextParams, task agent.Task,
) (agent.Result, error) {
	client.SetFullContext(p)
	res, err := a.Execute(ctx, h, task)
	if err != nil {
		return agent.Result{}, fmt.Errorf("run task %s: %w", task.ID, err)
	}
	return res, nil
}

// Extras calls the RPCs of the callback client that agent.Harness does not
// wrap: the plan context, the spans, a credential by name and the taxonomy
// checks. It records the outcome of each call.
func Extras(ctx context.Context, client *serve.CallbackClient) *Report {
	rep := &Report{}

	plan, err := client.GetPlanContext(ctx, &harnesspb.GetPlanContextRequest{})
	recordProto(rep, "GetPlanContext", plan, err)

	spans, err := client.RecordSpans(ctx, &harnesspb.RecordSpansRequest{})
	recordProto(rep, "RecordSpans", spans, err)

	cred, err := client.GetCredential(ctx, &harnesspb.GetCredentialRequest{Name: "scanner-api"})
	recordProto(rep, "GetCredential", cred, err)

	id, err := client.GenerateNodeID(ctx, &harnesspb.GenerateNodeIDRequest{NodeType: "host"})
	recordProto(rep, "GenerateNodeID", id, err)

	vf, err := client.ValidateFinding(ctx, &harnesspb.ValidateFindingRequest{})
	recordProto(rep, "ValidateFinding", vf, err)

	vn, err := client.ValidateGraphNode(ctx, &harnesspb.ValidateGraphNodeRequest{NodeType: "host"})
	recordProto(rep, "ValidateGraphNode", vn, err)

	vr, err := client.ValidateRelationship(ctx, &harnesspb.ValidateRelationshipRequest{RelationshipType: "RUNS"})
	recordProto(rep, "ValidateRelationship", vr, err)

	return rep
}

// recordProto adds the outcome of a call that returns a proto message.
func recordProto[T proto.Message](r *Report, call string, m T, err error) {
	record(r, call, m, err, func(m T) string { return fmt.Sprint(m) })
}

// ForkOptions tune how a forked process waits for its dispatch.
type ForkOptions struct {
	// ReadSandboxID reads the sandbox id of the process. A fork reads a new id.
	// Nil means the hostname.
	ReadSandboxID func() (string, error)
	// PollInterval is the wait between two checks. Zero means the default.
	PollInterval time.Duration
}

// ParkAsFork is the driver of a oneshot agent that the platform forks (D74,
// D80). A process that is not forkable returns no claim. A forkable process
// parks until its sandbox id changes, which means a snapshot restored it as a
// fork. It then claims its dispatch, with the setec identity token and no
// grant, and the grant of the claim replaces the old grant of the client.
func ParkAsFork(ctx context.Context, client *serve.CallbackClient, o ForkOptions) (*fork.Claim, error) {
	if !fork.Forkable() {
		return nil, nil
	}
	timeout, err := fork.ParkTimeout()
	if err != nil {
		return nil, fmt.Errorf("read the park timeout: %w", err)
	}
	read := o.ReadSandboxID
	if read == nil {
		read = fork.SandboxID
	}
	watcher, err := fork.NewWatcherWith(read)
	if err != nil {
		return nil, fmt.Errorf("watch the sandbox id: %w", err)
	}

	deadline := time.Now().Add(timeout)
	for {
		claim, err := fork.Park(ctx, watcher, client, fork.ParkOptions{Timeout: timeout, PollInterval: o.PollInterval})
		switch {
		case err == nil && claim == nil:
			return nil, nil
		case err == nil:
			if err := client.ApplyClaim(claim); err != nil {
				return nil, fmt.Errorf("apply the claim: %w", err)
			}
			return claim, nil
		case fork.IsForkUnclaimed(err) && time.Now().Before(deadline):
			// The daemon has not recorded the dispatch of this fork yet.
			select {
			case <-ctx.Done():
				return nil, fmt.Errorf("wait for the dispatch: %w", ctx.Err())
			case <-time.After(o.PollInterval):
			}
		default:
			return nil, fmt.Errorf("claim the fork: %w", err)
		}
	}
}

// ClaimSummary names what the fork learned from its claim.
func ClaimSummary(c *fork.Claim) string {
	return fmt.Sprintf("sandbox=%s mission=%s run=%s agent_run=%s node=%s model=%s task=%s",
		c.SandboxID, c.MissionID, c.MissionRunID, c.AgentRunID, c.NodeID, c.Model, c.Task.GetId())
}

// ForkTask is the task that the claim dispatched.
func ForkTask(c *fork.Claim) (agent.Task, error) {
	if c == nil || c.Task == nil {
		return agent.Task{}, errors.New("the claim has no task")
	}
	return serve.ProtoToTask(c.Task), nil
}

// taskParams turns a claim into the context of the calls of the fork.
func taskParams(c *fork.Claim, name string) serve.TaskContextParams {
	return serve.TaskContextParams{
		TaskID:       c.Task.GetId(),
		AgentName:    name,
		MissionID:    c.MissionID,
		MissionRunID: c.MissionRunID,
		AgentRunID:   c.AgentRunID,
	}
}

// RunFork parks as a fork and, if the platform dispatched a task, runs it.
func RunFork(ctx context.Context, a agent.Agent, o CallbackOptions, fo ForkOptions) (string, error) {
	client, h, err := Dial(ctx, o)
	if err != nil {
		return "", err
	}
	defer func() {
		_ = client.Close()
		_ = Shutdown(ctx, a)
	}()

	claim, err := ParkAsFork(ctx, client, fo)
	if err != nil {
		return "", err
	}
	if claim == nil {
		return "no fork", nil
	}
	task, err := ForkTask(claim)
	if err != nil {
		return "", err
	}
	res, err := RunTask(ctx, a, client, h, taskParams(claim, a.Name()), task)
	if err != nil {
		return "", err
	}
	rep, ok := res.Output.(*Report)
	if !ok {
		return "", errors.New("the agent returned no report")
	}
	extras := Extras(ctx, client)
	out := ClaimSummary(claim) + " steps=" + strconv.Itoa(len(rep.Steps)+len(extras.Steps))
	if s, found := rep.Step("TokenUsage"); found {
		out += " usage=" + s.Summary
	}
	if f := Failures(rep) + Failures(extras); f != "" {
		out += " failed=" + f
	}
	return out + "\n" + Describe(rep) + Describe(extras), nil
}

// failed lists the calls of a report that returned an error other than an
// unavailable call.
func failed(r *Report) []string {
	var out []string
	for _, s := range r.Steps {
		if s.Err != nil && !s.Unavailable() {
			out = append(out, s.Call+": "+s.Err.Error())
		}
	}
	return out
}

// Failures joins the failed calls into one line.
func Failures(r *Report) string { return strings.Join(failed(r), "; ") }
