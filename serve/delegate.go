// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Zero Root AI

package serve

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/a2aproject/a2a-go/v2/a2a"
	"github.com/a2aproject/a2a-go/v2/a2aclient"

	"github.com/zeroroot-ai/sdk/internal/gwhttp"
)

// DelegatePollInterval is the time between two GetTask calls while Delegate
// waits for a task to end. The A2A call blocks until the task ends or needs
// input when the agent supports that, so the poll is the fallback.
var DelegatePollInterval = time.Second

// resolvedAgent is the answer of the daemon to a ResolveAgent call. Both
// harnesses use it.
type resolvedAgent struct {
	gatewayURL   string
	routeHeaders map[string]string
	gatewayCAPEM string
	cardURL      string
}

// delegate sends one A2A message to an agent through the Agent gateway
// (JSON-RPC binding) and waits for the task to end. It returns the A2A Task.
//
// The call sends the route headers unchanged and the grant of the agent in
// x-capability-grant. It trusts exactly the CA bundle of the gateway, as the
// model client does.
//
// The wait ends when the task is in a terminal state, or needs input or
// authorization from the caller (input-required, auth-required). An agent that
// answers with a plain message and no task gets a Task in the completed state
// that holds the message as its status message.
func delegate(
	ctx context.Context, ra *resolvedAgent, headers gwhttp.HeaderFunc, msg *a2a.Message,
) (*a2a.Task, error) {
	if ra.gatewayURL == "" {
		return nil, errors.New("the daemon returned no gateway URL for the agent")
	}
	if msg == nil {
		return nil, errors.New("delegate: message is nil")
	}
	hc, err := gwhttp.NewClient(ra.gatewayURL, ra.gatewayCAPEM, ra.routeHeaders, headers)
	if err != nil {
		return nil, fmt.Errorf("delegate: %w", err)
	}
	client, err := a2aclient.NewFromEndpoints(ctx,
		[]*a2a.AgentInterface{a2a.NewAgentInterface(ra.gatewayURL, a2a.TransportProtocolJSONRPC)},
		a2aclient.WithConfig(a2aclient.Config{PreferredTransports: []a2a.TransportProtocol{a2a.TransportProtocolJSONRPC}}),
		a2aclient.WithJSONRPCTransport(hc),
	)
	if err != nil {
		return nil, fmt.Errorf("delegate: create the a2a client: %w", err)
	}
	defer func() { _ = client.Destroy() }()

	m := *msg
	if m.ID == "" {
		m.ID = a2a.NewMessageID()
	}
	if m.Role == "" {
		m.Role = a2a.MessageRoleUser
	}
	res, err := client.SendMessage(ctx, &a2a.SendMessageRequest{Message: &m})
	if err != nil {
		return nil, fmt.Errorf("delegate: send message: %w", err)
	}
	switch r := res.(type) {
	case *a2a.Message:
		return taskOfMessage(r), nil
	case *a2a.Task:
		return waitForTask(ctx, client, r)
	default:
		return nil, fmt.Errorf("delegate: unexpected reply type %T", res)
	}
}

// taskOfMessage wraps the plain reply of an agent that made no task.
func taskOfMessage(m *a2a.Message) *a2a.Task {
	id, ctxID := m.TaskID, m.ContextID
	if id == "" {
		id = a2a.NewTaskID()
	}
	if ctxID == "" {
		ctxID = a2a.NewContextID()
	}
	return &a2a.Task{ID: id, ContextID: ctxID, Status: a2a.TaskStatus{State: a2a.TaskStateCompleted, Message: m}}
}

// waitForTask polls GetTask until the task ends or needs the caller.
func waitForTask(ctx context.Context, c *a2aclient.Client, t *a2a.Task) (*a2a.Task, error) {
	for !settled(t.Status.State) {
		select {
		case <-ctx.Done():
			return t, fmt.Errorf("delegate: wait for task %s: %w", t.ID, ctx.Err())
		case <-time.After(DelegatePollInterval):
		}
		next, err := c.GetTask(ctx, &a2a.GetTaskRequest{ID: t.ID})
		if err != nil {
			return t, fmt.Errorf("delegate: get task %s: %w", t.ID, err)
		}
		t = next
	}
	return t, nil
}

// settled reports whether the caller has nothing more to wait for.
func settled(s a2a.TaskState) bool {
	return s.Terminal() || s == a2a.TaskStateInputRequired || s == a2a.TaskStateAuthRequired
}
