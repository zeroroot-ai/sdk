// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Zero Root AI

package serve

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/a2aproject/a2a-go/v2/a2a"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/zeroroot-ai/sdk/agent"
	componentpb "github.com/zeroroot-ai/sdk/api/gen/gibson/component/v1"
)

type resolverComponent struct {
	componentpb.UnimplementedComponentServiceServer
	agentURL string
	bank     *string
	fail     bool
}

func (r *resolverComponent) ResolveAgent(
	_ context.Context, req *componentpb.ResolveAgentRequest,
) (*componentpb.ResolveAgentResponse, error) {
	if r.fail {
		return nil, fmt.Errorf("resolve: %w", status.Error(codes.NotFound, "no such agent"))
	}
	r.bank = req.BankId
	return &componentpb.ResolveAgentResponse{GatewayUrl: r.agentURL, AgentCardUrl: r.agentURL + "/card"}, nil
}

func (r *resolverComponent) ResolveModel(context.Context, *componentpb.ResolveModelRequest) (*componentpb.ResolveModelResponse, error) {
	return nil, fmt.Errorf("resolve: %w", status.Error(codes.PermissionDenied, "no"))
}

func TestPlatformHarnessDelegate(t *testing.T) {
	srv, _ := rpcServer(t, func() any { return map[string]any{"task": taskJSON(a2a.TaskStateCompleted)} }, func() any { return nil })
	res := &resolverComponent{agentURL: srv.URL}
	h, cleanup := setupParityTestWith(t, func(s *grpc.Server) { componentpb.RegisterComponentServiceServer(s, res) })
	defer cleanup()
	h.grant = newTaskGrant(grantJWT(t, time.Now().Add(20*time.Minute)), nil)

	task, err := h.Delegate(context.Background(), "helper", userMessage("go"), agent.WithBank("bank-9"))
	require.NoError(t, err)
	assert.Equal(t, a2a.TaskStateCompleted, task.Status.State)
	require.NotNil(t, res.bank)
	assert.Equal(t, "bank-9", *res.bank)

	// No task grant: the call fails with a clear error.
	h.grant = newTaskGrant("", nil)
	_, err = h.Delegate(context.Background(), "helper", userMessage("go"))
	require.ErrorContains(t, err, "no task grant")

	res.fail = true
	_, err = h.Delegate(context.Background(), "helper", userMessage("go"))
	require.ErrorContains(t, err, "no such agent")
}

func TestPlatformClientResolveGuards(t *testing.T) {
	h, cleanup := setupParityTestWith(t, func(s *grpc.Server) {
		componentpb.RegisterComponentServiceServer(s, &resolverComponent{})
	})
	defer cleanup()
	ctx := context.Background()

	_, err := h.client.ResolveModel(ctx, "", "primary")
	require.ErrorContains(t, err, "workID")
	_, err = h.client.ResolveModel(ctx, "w", "")
	require.ErrorContains(t, err, "slot")
	_, err = h.client.ResolveModel(ctx, "w", "primary")
	require.ErrorContains(t, err, "ResolveModel")
	_, err = h.Model(ctx, "primary")
	require.Error(t, err)

	_, err = h.client.ResolveAgent(ctx, "", "a", "")
	require.ErrorContains(t, err, "workID")
	_, err = h.client.ResolveAgent(ctx, "w", "", "")
	require.ErrorContains(t, err, "name")

	// A client that is not connected refuses each call.
	pc := &PlatformClient{}
	_, err = pc.ResolveModel(ctx, "w", "s")
	require.Error(t, err)
	_, err = pc.ResolveAgent(ctx, "w", "a", "")
	require.Error(t, err)
	_, err = pc.RenewTaskGrant(ctx, "x")
	require.Error(t, err)
}

func TestDelegateRefusesANilMessage(t *testing.T) {
	_, err := delegate(context.Background(), &resolvedAgent{gatewayURL: "http://x"}, testGrantHeaders, nil)
	require.ErrorContains(t, err, "nil")
}

func TestDelegateBadReplies(t *testing.T) {
	srv, _ := rpcServer(t, func() any { return map[string]any{"unexpected": true} }, func() any { return nil })
	_, err := delegate(context.Background(), &resolvedAgent{gatewayURL: srv.URL}, testGrantHeaders, userMessage("x"))
	require.Error(t, err)

	// GetTask fails while the task works.
	old := DelegatePollInterval
	DelegatePollInterval = time.Millisecond
	defer func() { DelegatePollInterval = old }()
	srv2, _ := rpcServer(t,
		func() any { return map[string]any{"task": taskJSON(a2a.TaskStateWorking)} },
		func() any { return "not a task" })
	_, err = delegate(context.Background(), &resolvedAgent{gatewayURL: srv2.URL}, testGrantHeaders, userMessage("x"))
	require.Error(t, err)
}

func TestCallbackGatewayCallsNeedAConnection(t *testing.T) {
	c := &CallbackClient{}
	_, err := c.ResolveModel(context.Background(), "s")
	require.ErrorContains(t, err, "not connected")
	_, err = c.ResolveAgent(context.Background(), "a", "b")
	require.ErrorContains(t, err, "not connected")
	h := &CallbackHarness{client: c, tokenTracker: NewCallbackTokenTracker(), tracer: defaultNoopTracer()}
	_, err = h.Delegate(context.Background(), "a", userMessage("x"))
	require.Error(t, err)
	_, err = h.Model(context.Background(), "s")
	require.Error(t, err)
}

func TestTaskGrantWithoutRenewerKeepsTheGrant(t *testing.T) {
	g := newTaskGrant(grantJWT(t, time.Now().Add(30*time.Second)), nil)
	got, err := g.get(context.Background())
	require.NoError(t, err)
	assert.NotEmpty(t, got)
}
