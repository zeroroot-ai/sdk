// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Zero Root AI

package plugin

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	componentpb "github.com/zeroroot-ai/sdk/api/gen/gibson/component/v1"
	"github.com/zeroroot-ai/sdk/plugin/lifecycle"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/anypb"

	pluginpb "github.com/zeroroot-ai/sdk/api/gen/gibson/plugin/v1"
)

// ----------------------------------------------------------------------------
// Helpers: fake capability-grant platform (complete discovery + register)
// ----------------------------------------------------------------------------

// fakeCGPlatform serves a protocol-complete capability-grant discovery
// document and register endpoint, unlike fakePlatform which intentionally
// returns a minimal document for failure-path tests.
func fakeCGPlatform(t *testing.T) *httptest.Server {
	t.Helper()
	mux := http.NewServeMux()
	var baseURL atomic.Value

	mux.HandleFunc("/.well-known/agent-configuration", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{
			"protocol_version": "1.0",
			"provider_name":    "test",
			"issuer":           baseURL.Load().(string),
			"supported_modes":  []string{"autonomous"},
			"endpoints": map[string]string{
				"register": baseURL.Load().(string) + "/register",
			},
		})
	})
	mux.HandleFunc("/register", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{
			"agent_id":        "test-agent-id",
			"capabilities":    []any{},
			"component_scope": "plugin:dyn-plugin",
		})
	})

	srv := httptest.NewTLSServer(mux)
	t.Cleanup(srv.Close)
	baseURL.Store(srv.URL)
	return srv
}

// ----------------------------------------------------------------------------
// Helpers: fake in-process gRPC daemon (ComponentService subset)
// ----------------------------------------------------------------------------

// fakeDaemon implements the ComponentService RPCs Serve exercises:
// RegisterComponent, PollWork, SubmitResult, Heartbeat, WatchComponentEvents.
// Work items are fed through workCh; submitted results come out of resultCh;
// component events are fed through eventCh to whichever stream is open.
type fakeDaemon struct {
	componentpb.UnimplementedComponentServiceServer

	mu        sync.Mutex
	registers []*componentpb.RegisterComponentRequest

	workCh   chan *componentpb.PollWorkResponse
	resultCh chan *componentpb.SubmitResultRequest

	// addr is the listen address once startFakeDaemon has bound it.
	addr string

	// eventCh feeds the open WatchComponentEvents stream.
	eventCh chan *componentpb.ComponentEvent
	// watchCount counts WatchComponentEvents calls, so a test can prove a
	// reconnect happened.
	watchCount atomic.Int32
	// failFirstWatch ends the first WatchComponentEvents stream with
	// Unavailable, like a daemon replica going away in a rollout.
	failFirstWatch bool

	// heartbeatIntervalMs is what RegisterComponent tells the plugin. The
	// default is one minute, so a test that does not read heartbeats never
	// sees one.
	heartbeatIntervalMs int32
	// heartbeatCh receives every HeartbeatRequest the plugin sends. The send
	// never blocks: a full buffer drops the request.
	heartbeatCh chan *componentpb.HeartbeatRequest
}

func newFakeDaemon() *fakeDaemon {
	return &fakeDaemon{
		workCh:              make(chan *componentpb.PollWorkResponse, 16),
		resultCh:            make(chan *componentpb.SubmitResultRequest, 16),
		eventCh:             make(chan *componentpb.ComponentEvent, 16),
		heartbeatIntervalMs: 60_000,
		heartbeatCh:         make(chan *componentpb.HeartbeatRequest, 64),
	}
}

func (d *fakeDaemon) RegisterComponent(_ context.Context, req *componentpb.RegisterComponentRequest) (*componentpb.RegisterComponentResponse, error) {
	d.mu.Lock()
	d.registers = append(d.registers, req)
	d.mu.Unlock()
	return &componentpb.RegisterComponentResponse{
		InstanceId:          "inst-test-1",
		PollTimeoutMs:       50,
		HeartbeatIntervalMs: d.heartbeatIntervalMs,
	}, nil
}

func (d *fakeDaemon) PollWork(ctx context.Context, req *componentpb.PollWorkRequest) (*componentpb.PollWorkResponse, error) {
	select {
	case w := <-d.workCh:
		return w, nil
	case <-ctx.Done():
		return nil, ctx.Err()
	case <-time.After(time.Duration(req.GetTimeoutMs()) * time.Millisecond):
		return &componentpb.PollWorkResponse{}, nil
	}
}

func (d *fakeDaemon) SubmitResult(_ context.Context, req *componentpb.SubmitResultRequest) (*componentpb.SubmitResultResponse, error) {
	d.resultCh <- req
	return &componentpb.SubmitResultResponse{}, nil
}

func (d *fakeDaemon) Heartbeat(_ context.Context, req *componentpb.HeartbeatRequest) (*componentpb.HeartbeatResponse, error) {
	select {
	case d.heartbeatCh <- req:
	default:
	}
	return &componentpb.HeartbeatResponse{}, nil
}

// WatchComponentEvents forwards eventCh to the caller until the stream's
// context ends. With failFirstWatch set, the first call fails at once.
func (d *fakeDaemon) WatchComponentEvents(_ *componentpb.WatchComponentEventsRequest, stream componentpb.ComponentService_WatchComponentEventsServer) error {
	n := d.watchCount.Add(1)
	if d.failFirstWatch && n == 1 {
		return fmt.Errorf("fake daemon: %w", status.Error(codes.Unavailable, "daemon replica is rolling out"))
	}
	for {
		select {
		case ev := <-d.eventCh:
			if err := stream.Send(ev); err != nil {
				return fmt.Errorf("fake daemon: send: %w", err)
			}
		case <-stream.Context().Done():
			return fmt.Errorf("fake daemon: %w", stream.Context().Err())
		}
	}
}

func (d *fakeDaemon) registeredMethods(t *testing.T) []string {
	t.Helper()
	d.mu.Lock()
	defer d.mu.Unlock()
	require.Len(t, d.registers, 1, "expected exactly one RegisterComponent call")
	return d.registers[0].GetMethods()
}

func (d *fakeDaemon) registeredMetadata(t *testing.T) map[string]string {
	t.Helper()
	d.mu.Lock()
	defer d.mu.Unlock()
	require.Len(t, d.registers, 1, "expected exactly one RegisterComponent call")
	return d.registers[0].GetMetadata()
}

// startFakeDaemon serves the fake ComponentService on a random localhost port
// and points GIBSON_DAEMON_ADDR at it.
func startFakeDaemon(t *testing.T) *fakeDaemon {
	t.Helper()
	lis, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)

	daemon := newFakeDaemon()
	srv := grpc.NewServer()
	componentpb.RegisterComponentServiceServer(srv, daemon)
	go srv.Serve(lis)
	t.Cleanup(srv.Stop)

	daemon.addr = lis.Addr().String()
	t.Setenv("GIBSON_DAEMON_ADDR", daemon.addr)
	return daemon
}

// dialFakeDaemon returns a ComponentService client connected to daemon.
func dialFakeDaemon(t *testing.T, daemon *fakeDaemon) componentpb.ComponentServiceClient {
	t.Helper()
	conn, err := grpc.NewClient(daemon.addr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	require.NoError(t, err)
	t.Cleanup(func() { _ = conn.Close() })
	return componentpb.NewComponentServiceClient(conn)
}

// ----------------------------------------------------------------------------
// The declaration in code (ADR-0097): name, version and at least one handler.
// ----------------------------------------------------------------------------

func echoOption() Option {
	return WithHandler("Echo", "echoes the message back", func(_ context.Context, req echoReq) (echoResp, error) {
		return echoResp{Echoed: req.Msg}, nil
	})
}

type echoReq struct {
	Msg string `json:"msg"`
}

type echoResp struct {
	Echoed string `json:"echoed"`
}

func TestServe_DeclarationIsRequired(t *testing.T) {
	ctx := context.Background()
	cases := []struct {
		name string
		opts []Option
		want string
	}{
		{"no name", []Option{WithVersion("0.1.0"), echoOption()}, "WithName is required"},
		{"no version", []Option{WithName("p"), echoOption()}, "WithVersion is required"},
		{"no handler", []Option{WithName("p"), WithVersion("0.1.0")}, "no methods to register"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := Serve(ctx, tc.opts...)
			require.Error(t, err)
			assert.Contains(t, err.Error(), tc.want)
		})
	}
}

// ----------------------------------------------------------------------------
// Full Serve: the handlers are the method set, registered and invocable.
// ----------------------------------------------------------------------------

func TestServe_RegistersHandlersAndRoundTrips(t *testing.T) {
	t.Setenv("HOME", t.TempDir()) // keep capability-grant host keys out of the real home
	platform := fakeCGPlatform(t)
	t.Setenv("GIBSON_URL", platform.URL)
	daemon := startFakeDaemon(t)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	serveErr := make(chan error, 1)
	go func() {
		serveErr <- Serve(ctx,
			WithName("echo-plugin"),
			WithVersion("0.1.0"),
			echoOption(),
			testHTTPClient(platform.Client()),
			testHealthAddr(":0"),
		)
	}()

	// Enqueue a plugin_invoke. The Go-first wire format carries the JSON
	// request in the Any's value.
	invoke := &pluginpb.PluginInvokeRequest{
		PluginName: "echo-plugin",
		Method:     "Echo",
		Request:    &anypb.Any{TypeUrl: "json:Echo_request", Value: []byte(`{"msg":"hello"}`)},
		DeadlineMs: 5000,
	}
	payload, err := proto.Marshal(invoke)
	require.NoError(t, err)
	daemon.workCh <- &componentpb.PollWorkResponse{
		WorkId:   "work-1",
		WorkType: "plugin_invoke",
		Payload:  payload,
	}

	var result *componentpb.SubmitResultRequest
	select {
	case result = <-daemon.resultCh:
	case <-ctx.Done():
		t.Fatal("timeout waiting for SubmitResult")
	}
	require.Nil(t, result.GetError(), "handler should not error: %v", result.GetError())
	var resp echoResp
	require.NoError(t, json.Unmarshal(result.GetResult(), &resp))
	assert.Equal(t, "hello", resp.Echoed)

	assert.Equal(t, []string{"Echo"}, daemon.registeredMethods(t))

	// plugin:host_id is the one metadata key. No key declares a secret, a
	// runtime, a trust level or a manifest hash (sdk#129): the daemon decides
	// each of those itself.
	md := daemon.registeredMetadata(t)
	assert.NotEmpty(t, md["plugin:host_id"], "host_id thumbprint forwarded (keys per-host install uniqueness)")
	assert.Len(t, md, 1, "metadata = %v", md)

	cancel()
	require.NoError(t, <-serveErr)
}

// A plugin that needs a secret it was not granted fails at boot with an error
// that names the secret. The plugin resolves the secret in OnStart; no
// declaration exists for the SDK to interpret (sdk#129).
func TestServe_UngrantedStartupSecret_FailsAtBootNamingTheSecret(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	platform := fakeCGPlatform(t)
	t.Setenv("GIBSON_URL", platform.URL)
	startFakeDaemon(t)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	err := Serve(ctx,
		WithName("needs-a-secret"),
		WithVersion("0.1.0"),
		echoOption(),
		testHTTPClient(platform.Client()),
		testHealthAddr(":0"),
		testSecretsClient(newFakeSecretsClient(nil)),
		WithLifecycle(lifecycle.LifecycleHooks{
			OnStart: func(ctx context.Context) error {
				if _, err := ResolveSecret(ctx, "cred:vendor_token"); err != nil {
					return fmt.Errorf("resolve startup secret %q: %w", "cred:vendor_token", err)
				}
				return nil
			},
		}),
	)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "cred:vendor_token")
	assert.Contains(t, err.Error(), "OnStart hook failed")
}
