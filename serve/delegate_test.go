// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Zero Root AI

package serve

import (
	"context"
	"encoding/json"
	"encoding/pem"
	"io"
	"iter"
	"net"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/a2aproject/a2a-go/v2/a2a"
	"github.com/a2aproject/a2a-go/v2/a2asrv"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/test/bufconn"

	"github.com/zeroroot-ai/sdk/agent"
	harnesspb "github.com/zeroroot-ai/sdk/api/gen/gibson/harness/v1"
)

func testGrantHeaders(_ context.Context, route string) (map[string]string, error) {
	return map[string]string{"x-capability-grant": "grant:" + route}, nil
}

// seenHeaders keeps the headers of each request that reaches a test gateway.
type seenHeaders struct {
	mu   sync.Mutex
	list []http.Header
}

func (s *seenHeaders) add(h http.Header) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.list = append(s.list, h.Clone())
}

func (s *seenHeaders) all() []http.Header {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]http.Header(nil), s.list...)
}

func userMessage(text string) *a2a.Message {
	m := a2a.NewMessage(a2a.MessageRoleUser, a2a.NewTextPart(text))
	return m
}

// TestDelegateThroughAnA2AServer runs the client against the real a2a-go server
// over TLS with a private CA. The agent makes a Task with an artifact.
func TestDelegateThroughAnA2AServer(t *testing.T) {
	exec := a2asrv.AgentExecutorFunc(func(_ context.Context, ec *a2asrv.ExecutorContext) iter.Seq2[a2a.Event, error] {
		return func(yield func(a2a.Event, error) bool) {
			if !yield(a2a.NewSubmittedTask(ec, ec.Message), nil) {
				return
			}
			if !yield(a2a.NewStatusUpdateEvent(ec, a2a.TaskStateWorking, nil), nil) {
				return
			}
			art := a2a.NewArtifactEvent(ec, a2a.NewTextPart("echo: "+ec.Message.Parts[0].Text()))
			if !yield(art, nil) {
				return
			}
			yield(a2a.NewStatusUpdateEvent(ec, a2a.TaskStateCompleted, nil), nil)
		}
	})
	seen := &seenHeaders{}
	inner := a2asrv.NewJSONRPCHandler(a2asrv.NewHandler(exec))
	gw := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		seen.add(r.Header)
		inner.ServeHTTP(w, r)
	}))
	defer gw.Close()
	ca := string(pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: gw.Certificate().Raw}))

	task, err := delegate(context.Background(), &resolvedAgent{
		gatewayURL:   gw.URL + "/a2a/helper",
		routeHeaders: map[string]string{"x-gibson-tenant": "t1", "x-gibson-agent": "helper"},
		gatewayCAPEM: ca,
	}, testGrantHeaders, userMessage("hello"))
	require.NoError(t, err)

	assert.Equal(t, a2a.TaskStateCompleted, task.Status.State)
	require.Len(t, task.Artifacts, 1)
	assert.Equal(t, "echo: hello", task.Artifacts[0].Parts[0].Text())

	hs := seen.all()
	require.NotEmpty(t, hs)
	for _, h := range hs {
		assert.Equal(t, "grant:/a2a/helper", h.Get("x-capability-grant"), "the method claim is the path of the call")
		assert.Equal(t, "t1", h.Get("x-gibson-tenant"))
		assert.Equal(t, "helper", h.Get("x-gibson-agent"))
	}
}

func TestDelegateRefusesAnUnknownCA(t *testing.T) {
	gw := httptest.NewTLSServer(http.NotFoundHandler())
	defer gw.Close()
	_, err := delegate(context.Background(), &resolvedAgent{gatewayURL: gw.URL, gatewayCAPEM: selfSignedPEMForServe(t)},
		testGrantHeaders, userMessage("x"))
	require.Error(t, err)
	_, err = delegate(context.Background(), &resolvedAgent{gatewayURL: "http://plain.example", gatewayCAPEM: selfSignedPEMForServe(t)},
		testGrantHeaders, userMessage("x"))
	require.ErrorContains(t, err, "https")
	_, err = delegate(context.Background(), &resolvedAgent{}, testGrantHeaders, userMessage("x"))
	require.ErrorContains(t, err, "no gateway URL")
}

// rpcServer is a hand-made A2A JSON-RPC server. send answers SendMessage and
// get answers GetTask.
func rpcServer(t *testing.T, send, get func() any) (*httptest.Server, *int) {
	t.Helper()
	gets := new(int)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		var req struct {
			ID     json.RawMessage `json:"id"`
			Method string          `json:"method"`
		}
		_ = json.Unmarshal(body, &req)
		var result any
		switch req.Method {
		case "SendMessage":
			result = send()
		case "GetTask":
			*gets++
			result = get()
		default:
			http.Error(w, "unexpected method "+req.Method, http.StatusBadRequest)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{"jsonrpc": "2.0", "id": req.ID, "result": result})
	}))
	t.Cleanup(srv.Close)
	return srv, gets
}

func taskJSON(state a2a.TaskState) map[string]any {
	return map[string]any{"id": "task-1", "contextId": "ctx-1", "status": map[string]any{"state": state}}
}

func TestDelegateWaitsForTheTaskToEnd(t *testing.T) {
	old := DelegatePollInterval
	DelegatePollInterval = 5 * time.Millisecond
	defer func() { DelegatePollInterval = old }()

	n := 0
	srv, gets := rpcServer(t,
		func() any { return map[string]any{"task": taskJSON(a2a.TaskStateWorking)} },
		func() any {
			n++
			if n < 3 {
				return taskJSON(a2a.TaskStateWorking)
			}
			return taskJSON(a2a.TaskStateCompleted)
		})

	task, err := delegate(context.Background(), &resolvedAgent{gatewayURL: srv.URL}, testGrantHeaders, userMessage("x"))
	require.NoError(t, err)
	assert.Equal(t, a2a.TaskStateCompleted, task.Status.State)
	assert.Equal(t, 3, *gets)
}

func TestDelegateStopsOnInputRequiredAndOnAFailedTask(t *testing.T) {
	for _, state := range []a2a.TaskState{a2a.TaskStateInputRequired, a2a.TaskStateAuthRequired, a2a.TaskStateFailed, a2a.TaskStateRejected, a2a.TaskStateCanceled} {
		srv, gets := rpcServer(t, func() any { return map[string]any{"task": taskJSON(state)} }, func() any { return nil })
		task, err := delegate(context.Background(), &resolvedAgent{gatewayURL: srv.URL}, testGrantHeaders, userMessage("x"))
		require.NoError(t, err, state)
		assert.Equal(t, state, task.Status.State)
		assert.Zerof(t, *gets, "no poll for %s", state)
	}
}

func TestDelegateWrapsAPlainMessageReply(t *testing.T) {
	srv, _ := rpcServer(t, func() any {
		return map[string]any{"message": map[string]any{
			"messageId": "m1", "role": "ROLE_AGENT", "parts": []any{map[string]any{"text": "pong"}},
		}}
	}, func() any { return nil })
	task, err := delegate(context.Background(), &resolvedAgent{gatewayURL: srv.URL}, testGrantHeaders, userMessage("ping"))
	require.NoError(t, err)
	assert.Equal(t, a2a.TaskStateCompleted, task.Status.State)
	require.NotNil(t, task.Status.Message)
	assert.Equal(t, "pong", task.Status.Message.Parts[0].Text())
	assert.NotEmpty(t, task.ID)
}

func TestDelegateStopsWhenTheContextEnds(t *testing.T) {
	old := DelegatePollInterval
	DelegatePollInterval = time.Millisecond
	defer func() { DelegatePollInterval = old }()
	srv, _ := rpcServer(t,
		func() any { return map[string]any{"task": taskJSON(a2a.TaskStateWorking)} },
		func() any { return taskJSON(a2a.TaskStateWorking) })
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()
	_, err := delegate(ctx, &resolvedAgent{gatewayURL: srv.URL}, testGrantHeaders, userMessage("x"))
	require.Error(t, err)
}

func TestDelegateGrantError(t *testing.T) {
	srv, _ := rpcServer(t, func() any { return nil }, func() any { return nil })
	_, err := delegate(context.Background(), &resolvedAgent{gatewayURL: srv.URL}, grantHeaders(staticGrant("")), userMessage("x"))
	require.ErrorContains(t, err, "no task grant")
}

type agentResolver struct {
	harnesspb.UnimplementedHarnessCallbackServiceServer
	url string
	req *harnesspb.ResolveAgentRequest
}

func (a *agentResolver) ResolveAgent(
	_ context.Context, req *harnesspb.ResolveAgentRequest,
) (*harnesspb.ResolveAgentResponse, error) {
	a.req = req
	return &harnesspb.ResolveAgentResponse{GatewayUrl: a.url}, nil
}

func TestDelegateWithABank(t *testing.T) {
	srv, _ := rpcServer(t, func() any { return map[string]any{"task": taskJSON(a2a.TaskStateCompleted)} }, func() any { return nil })
	res := &agentResolver{url: srv.URL}
	lis := bufconn.Listen(1 << 20)
	gs := grpc.NewServer()
	harnesspb.RegisterHarnessCallbackServiceServer(gs, res)
	go func() { _ = gs.Serve(lis) }()
	defer gs.Stop()
	//nolint:staticcheck
	conn, err := grpc.DialContext(context.Background(), "bufnet",
		grpc.WithContextDialer(func(context.Context, string) (net.Conn, error) { return lis.Dial() }),
		grpc.WithTransportCredentials(insecure.NewCredentials()))
	require.NoError(t, err)
	defer func() { _ = conn.Close() }()
	cc := &CallbackClient{conn: conn, client: harnesspb.NewHarnessCallbackServiceClient(conn), connected: true, token: "task-grant"}
	h := &CallbackHarness{client: cc, tokenTracker: NewCallbackTokenTracker(), tracer: defaultNoopTracer()}

	task, err := h.Delegate(context.Background(), "worker", userMessage("go"), agent.WithBank("bank-1"))
	require.NoError(t, err)
	assert.Equal(t, a2a.TaskStateCompleted, task.Status.State)
	assert.Equal(t, "worker", res.req.GetName())
	assert.Equal(t, "bank-1", res.req.GetBankId())

	_, err = h.Delegate(context.Background(), "worker", userMessage("go"))
	require.NoError(t, err)
	assert.Nil(t, res.req.BankId, "no bank option: the field stays unset")
}
