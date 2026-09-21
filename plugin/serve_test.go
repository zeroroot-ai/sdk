// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Zero Root AI

package plugin

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/protobuf/types/known/timestamppb"

	componentpb "github.com/zeroroot-ai/sdk/api/gen/gibson/component/v1"
	pluginpb "github.com/zeroroot-ai/sdk/api/gen/gibson/plugin/v1"
	"github.com/zeroroot-ai/sdk/plugin/dispatch"
	"github.com/zeroroot-ai/sdk/plugin/events"
	"github.com/zeroroot-ai/sdk/plugin/health"
	"github.com/zeroroot-ai/sdk/plugin/lifecycle"
	"github.com/zeroroot-ai/sdk/plugin/manifest"
	pluginsecrets "github.com/zeroroot-ai/sdk/plugin/secrets"
)

// ----------------------------------------------------------------------------
// Helpers: fake manifest on disk
// ----------------------------------------------------------------------------

const testManifestYAML = `
apiVersion: plugin.gibson.zeroroot.ai/v1
kind: Plugin
metadata:
  name: test-plugin
  version: 0.1.0
  description: Test plugin
spec:
  workload_class: plugin
  methods:
    - name: Echo
  runtime: process
`

const testManifestWithSecretsYAML = `
apiVersion: plugin.gibson.zeroroot.ai/v1
kind: Plugin
metadata:
  name: secret-plugin
  version: 0.1.0
spec:
  workload_class: plugin
  secrets:
    - name: cred:api_key
      scope: startup
      rotation: restart
      required: true
  methods:
    - name: Echo
  runtime: process
`

// writeManifest writes YAML to a temp file and returns its path.
func writeManifest(t *testing.T, yaml string) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "plugin.yaml")
	require.NoError(t, os.WriteFile(path, []byte(yaml), 0600))
	return path
}

// ----------------------------------------------------------------------------
// Helpers: fake ComponentService backed by channels
// ----------------------------------------------------------------------------

// fakeComponentClient implements dispatch.ComponentClient for tests.
type fakeComponentClient struct {
	workCh   chan fakeWork
	resultCh chan fakeResult
}

type fakeWork struct {
	workID   string
	workType string
	payload  []byte
}

type fakeResult struct {
	workID  string
	result  []byte
	errInfo *pluginpb.PluginError
}

func newFakeComponentClient() *fakeComponentClient {
	return &fakeComponentClient{
		workCh:   make(chan fakeWork, 16),
		resultCh: make(chan fakeResult, 16),
	}
}

func (f *fakeComponentClient) PollWork(ctx context.Context, timeout time.Duration) (string, string, []byte, error) {
	select {
	case w := <-f.workCh:
		return w.workID, w.workType, w.payload, nil
	case <-ctx.Done():
		return "", "", nil, ctx.Err()
	case <-time.After(min(timeout, 5*time.Millisecond)):
		return "", "", nil, nil
	}
}

func (f *fakeComponentClient) SubmitResult(ctx context.Context, workID string, result []byte, errInfo *pluginpb.PluginError) error {
	select {
	case f.resultCh <- fakeResult{workID: workID, result: result, errInfo: errInfo}:
	case <-ctx.Done():
		return ctx.Err()
	}
	return nil
}

// ----------------------------------------------------------------------------
// Helpers: fake secrets client
// ----------------------------------------------------------------------------

type fakeSecretsClient struct {
	mu       sync.Mutex
	values   map[string][]byte
	revoked  map[string]struct{}
	errOnKey string // return error for this key
}

func newFakeSecretsClient(vals map[string][]byte) *fakeSecretsClient {
	return &fakeSecretsClient{
		values:  vals,
		revoked: make(map[string]struct{}),
	}
}

func (f *fakeSecretsClient) Resolve(_ context.Context, name string, _ ...pluginsecrets.Option) ([]byte, error) {
	if f.isRevoked(name) {
		return nil, errors.New("permission denied")
	}
	if f.errOnKey == name {
		return nil, fmt.Errorf("secret %q is unavailable", name)
	}
	if v, ok := f.values[name]; ok {
		return v, nil
	}
	return nil, fmt.Errorf("secret %q not found", name)
}

func (f *fakeSecretsClient) Invalidate(name string) {
	// no-op for fake
}

func (f *fakeSecretsClient) MarkRevoked(name string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.revoked[name] = struct{}{}
}

func (f *fakeSecretsClient) isRevoked(name string) bool {
	f.mu.Lock()
	defer f.mu.Unlock()
	_, ok := f.revoked[name]
	return ok
}

// ----------------------------------------------------------------------------
// Unit tests: validateMethods
// ----------------------------------------------------------------------------

func TestValidateMethods_HappyPath(t *testing.T) {
	m := &manifest.Manifest{
		Spec: manifest.ManifestSpec{
			Methods: []manifest.MethodDecl{
				{Name: "Echo"},
				{Name: "Ping"},
			},
		},
	}
	handlers := map[string]MethodHandler{
		"Echo": func(_ context.Context, req json.RawMessage) (json.RawMessage, error) { return req, nil },
		"Ping": func(_ context.Context, req json.RawMessage) (json.RawMessage, error) { return req, nil },
	}
	assert.NoError(t, validateMethods(m, handlers))
}

func TestValidateMethods_UndeclaredHandler(t *testing.T) {
	m := &manifest.Manifest{
		Spec: manifest.ManifestSpec{
			Methods: []manifest.MethodDecl{
				{Name: "Echo"},
			},
		},
	}
	handlers := map[string]MethodHandler{
		"Echo":  func(_ context.Context, req json.RawMessage) (json.RawMessage, error) { return req, nil },
		"Rogue": func(_ context.Context, req json.RawMessage) (json.RawMessage, error) { return req, nil },
	}
	err := validateMethods(m, handlers)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "Rogue")
	assert.Contains(t, err.Error(), "undeclared")
}

func TestValidateMethods_UnregisteredDeclaration(t *testing.T) {
	m := &manifest.Manifest{
		Spec: manifest.ManifestSpec{
			Methods: []manifest.MethodDecl{
				{Name: "Echo"},
				{Name: "Ping"},
			},
		},
	}
	handlers := map[string]MethodHandler{
		"Echo": func(_ context.Context, req json.RawMessage) (json.RawMessage, error) { return req, nil },
		// "Ping" not registered
	}
	err := validateMethods(m, handlers)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "Ping")
	assert.Contains(t, err.Error(), "without registered handlers")
}

// ----------------------------------------------------------------------------
// Test: Serve returns startup error for method mismatch (no daemon needed)
// ----------------------------------------------------------------------------

func TestServe_MethodMismatch_ReturnsStartupError(t *testing.T) {
	path := writeManifest(t, testManifestYAML)

	// Register a handler for an undeclared method.
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	err := Serve(ctx,
		WithManifest(path),
		WithHandler("UndeclaredMethod", func(_ context.Context, req string) (string, error) {
			return req, nil
		}),
	)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "UndeclaredMethod")
	assert.Contains(t, err.Error(), "undeclared")
}

func TestServe_MissingManifestPath_ReturnsError(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	err := Serve(ctx,
		WithHandler("Echo", func(_ context.Context, req string) (string, error) {
			return req, nil
		}),
	)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "WithManifest is required")
}

func TestServe_InvalidManifestPath_ReturnsError(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	err := Serve(ctx,
		WithManifest("/nonexistent/path/plugin.yaml"),
		WithHandler("Echo", func(_ context.Context, req string) (string, error) {
			return req, nil
		}),
	)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "load manifest")
}

// ----------------------------------------------------------------------------
// Test: Serve delivers a revocation from WatchComponentEvents to the secrets
// client and the lifecycle state machine. The fake daemon serves the stream,
// the fake secrets client stands in for GetCredential.
// ----------------------------------------------------------------------------

// secretsManifest returns the in-memory form of testManifestWithSecretsYAML.
func secretsManifest(t *testing.T) *manifest.Manifest {
	t.Helper()
	m, err := manifest.LoadBytes([]byte(testManifestWithSecretsYAML))
	require.NoError(t, err)
	return m
}

// revocationEvent is the wire message the daemon publishes when the operator
// revokes the plugin's binding to name.
func revocationEvent(name string) *componentpb.ComponentEvent {
	return &componentpb.ComponentEvent{
		Type:       events.EventTypeSecretAccessRevoked,
		SecretName: name,
		Reason:     "operator revoked the binding",
		OccurredAt: timestamppb.Now(),
	}
}

func TestServe_DeclaredSecrets_RevocationMarksDegraded(t *testing.T) {
	t.Setenv("HOME", t.TempDir()) // keep capability-grant host keys out of the real home
	platform := fakeCGPlatform(t)
	t.Setenv("GIBSON_URL", platform.URL)
	daemon := startFakeDaemon(t)

	fakeSecrets := newFakeSecretsClient(map[string][]byte{"cred:api_key": []byte("value")})
	degraded := make(chan string, 1)
	m := secretsManifest(t)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	serveErr := make(chan error, 1)
	go func() {
		serveErr <- Serve(ctx,
			WithParsedManifest(m),
			WithSecretsClient(fakeSecrets),
			WithLifecycle(lifecycle.LifecycleHooks{
				OnDegraded: func(reason string) { degraded <- reason },
			}),
			WithHandler("Echo", func(_ context.Context, req string) (string, error) {
				return req, nil
			}),
			WithHTTPClient(platform.Client()),
			WithHealthAddr(":0"),
		)
	}()

	// A heartbeat first: the SDK must drop it without effect. Then the
	// revocation for the declared secret.
	daemon.eventCh <- &componentpb.ComponentEvent{Type: "heartbeat", OccurredAt: timestamppb.Now()}
	daemon.eventCh <- revocationEvent("cred:api_key")

	select {
	case reason := <-degraded:
		assert.Equal(t, "secret_revoked: cred:api_key", reason)
	case err := <-serveErr:
		t.Fatalf("Serve returned before the revocation was delivered: %v", err)
	case <-ctx.Done():
		t.Fatal("timeout waiting for OnDegraded")
	}
	assert.True(t, fakeSecrets.isRevoked("cred:api_key"), "MarkRevoked must have been called")
	assert.Equal(t, int32(1), daemon.watchCount.Load(), "one subscription for one plugin")

	cancel()
	require.NoError(t, <-serveErr)
}

// TestServe_DeclaredSecrets_RevocationReachesHeartbeat proves sdk#60: the
// heartbeat reports the lifecycle state. Before the revocation every
// heartbeat reads "serving". After the daemon publishes
// secret_access_revoked the plugin turns Degraded, and the next heartbeat
// tick reads "degraded" with the recorded reason. The daemon maps that value
// to PLUGIN_INSTALL_STATUS_DEGRADED (gibson#199), so a plugin that lost its
// secret no longer reads SERVING forever.
func TestServe_DeclaredSecrets_RevocationReachesHeartbeat(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	platform := fakeCGPlatform(t)
	t.Setenv("GIBSON_URL", platform.URL)
	daemon := startFakeDaemon(t)
	const interval = 100 * time.Millisecond
	daemon.heartbeatIntervalMs = int32(interval / time.Millisecond)

	fakeSecrets := newFakeSecretsClient(map[string][]byte{"cred:api_key": []byte("value")})
	degraded := make(chan string, 1)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	serveErr := make(chan error, 1)
	go func() {
		serveErr <- Serve(ctx,
			WithParsedManifest(secretsManifest(t)),
			WithSecretsClient(fakeSecrets),
			WithLifecycle(lifecycle.LifecycleHooks{
				OnDegraded: func(reason string) { degraded <- reason },
			}),
			WithHandler("Echo", func(_ context.Context, req string) (string, error) {
				return req, nil
			}),
			WithHTTPClient(platform.Client()),
			WithHealthAddr(":0"),
		)
	}()

	// The plugin is Ready: the first heartbeat reads serving.
	select {
	case hb := <-daemon.heartbeatCh:
		assert.Equal(t, "inst-test-1", hb.GetInstanceId())
		assert.Equal(t, heartbeatHealthServing, hb.GetHealthStatus())
		assert.Equal(t, "ok", hb.GetHealthMessage())
	case err := <-serveErr:
		t.Fatalf("Serve returned before the first heartbeat: %v", err)
	case <-ctx.Done():
		t.Fatal("timeout waiting for the first heartbeat")
	}

	daemon.eventCh <- revocationEvent("cred:api_key")
	select {
	case reason := <-degraded:
		assert.Equal(t, "secret_revoked: cred:api_key", reason)
	case <-ctx.Done():
		t.Fatal("timeout waiting for OnDegraded")
	}
	degradedAt := time.Now()

	// The tick that fires after the transition reports degraded. One
	// heartbeat may already be in flight with the old state, so at most one
	// serving heartbeat may arrive after the transition, and the degraded
	// one must land within two intervals of it.
	deadline := time.After(2 * interval)
	servingAfter := 0
	for {
		select {
		case hb := <-daemon.heartbeatCh:
			if hb.GetHealthStatus() == heartbeatHealthServing {
				servingAfter++
				require.LessOrEqual(t, servingAfter, 1,
					"a second serving heartbeat after the revocation: the heartbeat ignores the lifecycle state")
				continue
			}
			assert.Equal(t, heartbeatHealthDegraded, hb.GetHealthStatus())
			assert.Equal(t, "secret_revoked: cred:api_key", hb.GetHealthMessage())
			assert.Less(t, time.Since(degradedAt), 2*interval,
				"the degraded heartbeat must land within one interval plus the tick in flight")
			cancel()
			require.NoError(t, <-serveErr)
			return
		case <-deadline:
			t.Fatal("no degraded heartbeat within two intervals of the revocation")
		}
	}
}

// ----------------------------------------------------------------------------
// Test: runHeartbeat reads the state machine on every tick (no Serve call)
// ----------------------------------------------------------------------------

// heartbeatRecorder is a ComponentServiceClient that records Heartbeat calls
// and panics on every other RPC.
type heartbeatRecorder struct {
	componentpb.ComponentServiceClient
	reqs chan *componentpb.HeartbeatRequest
}

func (r *heartbeatRecorder) Heartbeat(_ context.Context, req *componentpb.HeartbeatRequest, _ ...grpc.CallOption) (*componentpb.HeartbeatResponse, error) {
	r.reqs <- req
	return &componentpb.HeartbeatResponse{}, nil
}

func TestRunHeartbeat_ReportsLifecycleStateOnEveryTick(t *testing.T) {
	sm := readyStateMachine(t, lifecycle.LifecycleHooks{})
	rec := &heartbeatRecorder{reqs: make(chan *componentpb.HeartbeatRequest, 64)}
	srv := health.New(sm, ":0", time.Minute)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	done := make(chan error, 1)
	go func() { done <- runHeartbeat(ctx, rec, "inst-1", 5*time.Millisecond, sm, srv) }()

	next := func() *componentpb.HeartbeatRequest {
		select {
		case hb := <-rec.reqs:
			return hb
		case <-ctx.Done():
			t.Fatal("timeout waiting for a heartbeat")
			return nil
		}
	}

	hb := next()
	assert.Equal(t, "inst-1", hb.GetInstanceId())
	assert.Equal(t, heartbeatHealthServing, hb.GetHealthStatus())
	assert.Equal(t, "ok", hb.GetHealthMessage())

	require.NoError(t, sm.MarkDegraded("secret_revoked: cred:api_key"))
	// Skip the heartbeat that may have read the state before the transition.
	for hb = next(); hb.GetHealthStatus() == heartbeatHealthServing; hb = next() {
	}
	assert.Equal(t, heartbeatHealthDegraded, hb.GetHealthStatus())
	assert.Equal(t, "secret_revoked: cred:api_key", hb.GetHealthMessage())

	// Recovery: back to Ready reports serving again with no stale reason.
	require.NoError(t, sm.Transition(lifecycle.Ready))
	for hb = next(); hb.GetHealthStatus() == heartbeatHealthDegraded; hb = next() {
	}
	assert.Equal(t, heartbeatHealthServing, hb.GetHealthStatus())
	assert.Equal(t, "ok", hb.GetHealthMessage())

	cancel()
	require.NoError(t, <-done)
}

func TestHeartbeatHealth(t *testing.T) {
	tests := []struct {
		state       lifecycle.State
		reason      string
		wantStatus  string
		wantMessage string
	}{
		{lifecycle.Ready, "", heartbeatHealthServing, "ok"},
		{lifecycle.Degraded, "secret_revoked: cred:api_key", heartbeatHealthDegraded, "secret_revoked: cred:api_key"},
		{lifecycle.Degraded, "", heartbeatHealthDegraded, "degraded"},
		{lifecycle.Draining, "", heartbeatHealthServing, "ok"},
	}
	for _, tc := range tests {
		status, message := heartbeatHealth(tc.state, tc.reason)
		assert.Equal(t, tc.wantStatus, status, "state %s", tc.state)
		assert.Equal(t, tc.wantMessage, message, "state %s", tc.state)
	}
}

// ----------------------------------------------------------------------------
// Test: componentEventStream adapter (unit-level, no Serve call)
// ----------------------------------------------------------------------------

// readyStateMachine returns a state machine walked to Ready, the state the
// subscriber sees in production when an event arrives.
func readyStateMachine(t *testing.T, hooks lifecycle.LifecycleHooks) *lifecycle.StateMachine {
	t.Helper()
	sm := lifecycle.New(hooks)
	require.NoError(t, sm.Transition(lifecycle.Registering))
	require.NoError(t, sm.Transition(lifecycle.ResolvingSecrets))
	require.NoError(t, sm.Transition(lifecycle.Starting))
	require.NoError(t, sm.RunOnStart(context.Background()))
	require.Equal(t, lifecycle.Ready, sm.Current())
	return sm
}

func TestComponentEventStream_DropsHeartbeatAndMapsEvent(t *testing.T) {
	daemon := startFakeDaemon(t)
	stream := newComponentEventStream(dialFakeDaemon(t, daemon), "secret-plugin")

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	occurred := time.Date(2026, 9, 18, 12, 0, 0, 0, time.UTC)
	daemon.eventCh <- &componentpb.ComponentEvent{Type: "heartbeat", OccurredAt: timestamppb.Now()}
	daemon.eventCh <- &componentpb.ComponentEvent{
		Type:       events.EventTypeSecretRotated,
		SecretName: "cred:api_key",
		Version:    7,
		OccurredAt: timestamppb.New(occurred),
	}

	// The first event Recv returns is the rotation: the heartbeat before it
	// never reaches the caller.
	ev, err := stream.Recv(ctx)
	require.NoError(t, err)
	assert.Equal(t, events.Event{
		Type:       events.EventTypeSecretRotated,
		Name:       "cred:api_key",
		Version:    7,
		OccurredAt: occurred,
	}, ev)

	daemon.eventCh <- revocationEvent("cred:api_key")
	ev, err = stream.Recv(ctx)
	require.NoError(t, err)
	assert.Equal(t, events.EventTypeSecretAccessRevoked, ev.Type)
	assert.Equal(t, "cred:api_key", ev.Name)
	assert.Equal(t, "operator revoked the binding", ev.Reason)
	assert.False(t, ev.OccurredAt.IsZero())
}

func TestComponentEventStream_RevocationThroughSubscriber(t *testing.T) {
	daemon := startFakeDaemon(t)
	stream := newComponentEventStream(dialFakeDaemon(t, daemon), "secret-plugin")

	fakeSecrets := newFakeSecretsClient(map[string][]byte{"cred:api_key": []byte("value")})
	degraded := make(chan string, 1)
	sm := readyStateMachine(t, lifecycle.LifecycleHooks{
		OnDegraded: func(reason string) { degraded <- reason },
	})
	sub := events.NewWithDrainer(stream, fakeSecrets, sm, nil, secretsManifest(t))

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	runErr := make(chan error, 1)
	go func() { runErr <- sub.Run(ctx) }()

	daemon.eventCh <- revocationEvent("cred:api_key")

	select {
	case reason := <-degraded:
		assert.Equal(t, "secret_revoked: cred:api_key", reason)
	case <-ctx.Done():
		t.Fatal("timeout waiting for OnDegraded")
	}
	assert.Equal(t, lifecycle.Degraded, sm.Current())
	assert.True(t, fakeSecrets.isRevoked("cred:api_key"))

	cancel()
	require.NoError(t, <-runErr, "cancellation is a clean exit")
}

func TestComponentEventStream_ReconnectsAfterStreamError(t *testing.T) {
	daemon := startFakeDaemon(t)
	daemon.failFirstWatch = true
	stream := newComponentEventStream(dialFakeDaemon(t, daemon), "secret-plugin")
	stream.initialBackoff = 5 * time.Millisecond
	stream.maxBackoff = 20 * time.Millisecond

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	daemon.eventCh <- revocationEvent("cred:api_key")

	ev, err := stream.Recv(ctx)
	require.NoError(t, err)
	assert.Equal(t, events.EventTypeSecretAccessRevoked, ev.Type)
	assert.Equal(t, int32(2), daemon.watchCount.Load(), "the first stream failed, the second delivered")
}

func TestComponentEventStream_CancelReturnsContextError(t *testing.T) {
	daemon := startFakeDaemon(t)
	stream := newComponentEventStream(dialFakeDaemon(t, daemon), "secret-plugin")

	ctx, cancel := context.WithCancel(context.Background())
	type result struct {
		ev  events.Event
		err error
	}
	got := make(chan result, 1)
	go func() {
		ev, err := stream.Recv(ctx)
		got <- result{ev, err}
	}()

	// Wait for the subscription, then cancel: the adapter must not reconnect.
	require.Eventually(t, func() bool { return daemon.watchCount.Load() == 1 },
		5*time.Second, 5*time.Millisecond)
	cancel()

	select {
	case r := <-got:
		require.ErrorIs(t, r.err, context.Canceled)
	case <-time.After(5 * time.Second):
		t.Fatal("Recv did not return after cancellation")
	}
	assert.Equal(t, int32(1), daemon.watchCount.Load(), "cancellation must not reconnect")
}

func TestComponentEventStream_BackoffDoublesToCap(t *testing.T) {
	stream := newComponentEventStream(nil, "secret-plugin")
	stream.initialBackoff = time.Millisecond
	stream.maxBackoff = 4 * time.Millisecond

	ctx := context.Background()
	cause := errors.New("stream broke")
	require.NoError(t, stream.waitBackoff(ctx, "recv", cause))
	assert.Equal(t, 2*time.Millisecond, stream.backoff)
	require.NoError(t, stream.waitBackoff(ctx, "recv", cause))
	assert.Equal(t, 4*time.Millisecond, stream.backoff)
	require.NoError(t, stream.waitBackoff(ctx, "recv", cause))
	assert.Equal(t, 4*time.Millisecond, stream.backoff, "capped at maxBackoff")

	cancelled, cancel := context.WithCancel(ctx)
	cancel()
	require.ErrorIs(t, stream.waitBackoff(cancelled, "recv", cause), context.Canceled)
}

// ----------------------------------------------------------------------------
// Test: dispatch.Dispatcher drain integration (unit-level, no Serve call)
// Verifies that the Drainer interface wired between events.Subscriber and
// dispatch.Dispatcher behaves correctly.
// ----------------------------------------------------------------------------

func TestDrainerIntegration_RotationRestartCallsDrainThenExit(t *testing.T) {
	m, err := manifest.LoadBytes([]byte(testManifestWithSecretsYAML))
	require.NoError(t, err)

	// Track the fake exiter call.
	var exitCode int
	var exitCalled atomic.Bool
	origExiter := dispatch.SetExiterForTest(func(code int) {
		exitCode = code
		exitCalled.Store(true)
	})
	defer dispatch.SetExiterForTest(origExiter)

	fakeClient := newFakeComponentClient()
	disp := dispatch.New(fakeClient, dispatch.Config{
		Handlers: map[string]dispatch.MethodHandler{
			"Echo": func(_ context.Context, req json.RawMessage) (json.RawMessage, error) {
				return req, nil
			},
		},
	})

	fakeSecrets := newFakeSecretsClient(map[string][]byte{
		"cred:api_key": []byte("test-key"),
	})
	sm := lifecycle.New(lifecycle.LifecycleHooks{})

	// Wire the subscriber with the dispatcher as Drainer.
	eventCh := make(chan events.Event, 1)
	stream := &chanEventStream{ch: eventCh}
	sub := events.NewWithDrainer(stream, fakeSecrets, sm, disp, m)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Run subscriber in background.
	go sub.Run(ctx)

	// Send a rotation=restart event for the declared secret.
	eventCh <- events.Event{
		Type:       events.EventTypeSecretRotated,
		Name:       "cred:api_key",
		Version:    2,
		OccurredAt: time.Now(),
	}

	// Wait for exit to be called.
	deadline := time.After(2 * time.Second)
	for !exitCalled.Load() {
		select {
		case <-deadline:
			t.Fatal("timeout waiting for DrainThenExit to be called")
		case <-time.After(5 * time.Millisecond):
		}
	}

	assert.True(t, exitCalled.Load(), "DrainThenExit should have been called")
	assert.Equal(t, 75, exitCode, "exit code should be 75 (rotation-restart sentinel)")
}

// chanEventStream is a test EventStream backed by a channel.
type chanEventStream struct {
	ch <-chan events.Event
}

func (s *chanEventStream) Recv(ctx context.Context) (events.Event, error) {
	select {
	case ev, ok := <-s.ch:
		if !ok {
			return events.Event{}, errors.New("event stream closed")
		}
		return ev, nil
	case <-ctx.Done():
		return events.Event{}, ctx.Err()
	}
}

// ----------------------------------------------------------------------------
// Test: SIGTERM drain path (unit-level via gracefulShutdown)
// Verifies that gracefulShutdown transitions the lifecycle SM and drains
// the dispatcher correctly.
// ----------------------------------------------------------------------------

func TestGracefulShutdown_TransitionsAndDrains(t *testing.T) {
	var onStopCalled atomic.Bool
	var onStartCalled atomic.Bool

	sm := lifecycle.New(lifecycle.LifecycleHooks{
		OnStart: func(_ context.Context) error {
			onStartCalled.Store(true)
			return nil
		},
		OnStop: func(_ context.Context) error {
			onStopCalled.Store(true)
			return nil
		},
	})

	// Walk SM to Ready state so gracefulShutdown can transition to Draining.
	require.NoError(t, sm.Transition(lifecycle.Registering))
	require.NoError(t, sm.Transition(lifecycle.ResolvingSecrets))
	require.NoError(t, sm.Transition(lifecycle.Starting))
	require.NoError(t, sm.RunOnStart(context.Background()))
	assert.True(t, onStartCalled.Load())
	assert.Equal(t, lifecycle.Ready, sm.Current())

	fakeClient := newFakeComponentClient()
	disp := dispatch.New(fakeClient, dispatch.Config{
		Handlers: map[string]dispatch.MethodHandler{
			"Echo": func(_ context.Context, req json.RawMessage) (json.RawMessage, error) {
				return req, nil
			},
		},
	})

	ctx := context.Background()
	err := gracefulShutdown(ctx, sm, disp, 100*time.Millisecond, "test-plugin")
	require.NoError(t, err)

	assert.True(t, onStopCalled.Load(), "OnStop should have been called")
	assert.Equal(t, lifecycle.Stopped, sm.Current(), "SM should be in Stopped state")
}

// ----------------------------------------------------------------------------
// Test: validateMethods with empty handlers map and empty manifest
// ----------------------------------------------------------------------------

func TestValidateMethods_BothEmpty(t *testing.T) {
	m := &manifest.Manifest{
		Spec: manifest.ManifestSpec{},
	}
	err := validateMethods(m, nil)
	assert.NoError(t, err, "no declared methods and no handlers is not a mismatch")
}

// ----------------------------------------------------------------------------
// Test: WithOptions ergonomics
// ----------------------------------------------------------------------------

func TestOptions_Defaults(t *testing.T) {
	c := &config{}
	c.defaults()
	assert.Equal(t, ":8080", c.healthAddr)
	assert.Equal(t, 30*time.Second, c.drainTimeout)
	assert.NotNil(t, c.handlers)
}

func TestWithHealthAddr(t *testing.T) {
	c := &config{}
	WithHealthAddr(":9090")(c)
	assert.Equal(t, ":9090", c.healthAddr)
}

func TestWithDrainTimeout(t *testing.T) {
	c := &config{}
	WithDrainTimeout(10 * time.Second)(c)
	assert.Equal(t, 10*time.Second, c.drainTimeout)
}

func TestWithHandler_RegistersHandler(t *testing.T) {
	c := &config{}
	handler := func(_ context.Context, req string) (string, error) {
		return "ok", nil
	}
	WithHandler("Echo", handler)(c)
	require.NotNil(t, c.handlers["Echo"])
}

func TestWithSecretsClient_SetsClient(t *testing.T) {
	c := &config{}
	fake := newFakeSecretsClient(nil)
	WithSecretsClient(fake)(c)
	assert.Equal(t, fake, c.secretsClient)
}

func TestWithManifest_SetsPath(t *testing.T) {
	c := &config{}
	WithManifest("/some/path/plugin.yaml")(c)
	assert.Equal(t, "/some/path/plugin.yaml", c.manifestPath)
}

// min is a local helper for older Go compat in test helper (also stdlib since 1.21).
func min(a, b time.Duration) time.Duration {
	if a < b {
		return a
	}
	return b
}

func TestResolveDaemonAddr_PortedPlatformURL(t *testing.T) {
	t.Setenv("GIBSON_DAEMON_ADDR", "")
	cases := map[string]string{
		// Explicit port wins — the dial authority (and so the CG-JWT audience)
		// follows the configured URL (sdk#452).
		"https://api.zeroroot.ai:30443":        "api.zeroroot.ai:30443",
		"http://gibson-gibson-workloads:50051": "gibson-gibson-workloads:50051",
		"http://gibson.gibson.svc:8080/path":   "gibson.gibson.svc:8080",
		"https://api.zeroroot.ai:30443/":       "api.zeroroot.ai:30443",
		// No port → scheme default, mirroring agent.Connect's normalizeTarget.
		"https://api.zeroroot.ai":        "api.zeroroot.ai:443",
		"https://api.zeroroot.ai/":       "api.zeroroot.ai:443",
		"https://api.zeroroot.ai/gibson": "api.zeroroot.ai:443",
		"http://gibson.gibson.svc":       "gibson.gibson.svc:80",
		// Bare host, no scheme → :443 like normalizeTarget.
		"gibson-workloads.gibson.svc": "gibson-workloads.gibson.svc:443",
	}
	for in, want := range cases {
		if got := resolveDaemonAddr(in); got != want {
			t.Errorf("resolveDaemonAddr(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestResolveDaemonAddr_EnvOverride(t *testing.T) {
	t.Setenv("GIBSON_DAEMON_ADDR", "localhost:50001")
	if got := resolveDaemonAddr("https://api.zeroroot.ai:30443"); got != "localhost:50001" {
		t.Errorf("env override not honored: got %q", got)
	}
}

func TestDaemonTransportCredentials_DefaultInsecure(t *testing.T) {
	t.Setenv("GIBSON_DAEMON_TLS", "")
	if got := daemonTransportCredentials(); got.Info().SecurityProtocol != "insecure" {
		t.Errorf("default should be insecure, got %q", got.Info().SecurityProtocol)
	}
}

func TestDaemonTransportCredentials_TLSWhenEnabled(t *testing.T) {
	for _, v := range []string{"1", "true", "yes"} {
		t.Setenv("GIBSON_DAEMON_TLS", v)
		if got := daemonTransportCredentials(); got.Info().SecurityProtocol != "tls" {
			t.Errorf("GIBSON_DAEMON_TLS=%q should select tls, got %q", v, got.Info().SecurityProtocol)
		}
	}
	for _, v := range []string{"", "0", "false"} {
		t.Setenv("GIBSON_DAEMON_TLS", v)
		if got := daemonTransportCredentials(); got.Info().SecurityProtocol != "insecure" {
			t.Errorf("GIBSON_DAEMON_TLS=%q should stay insecure, got %q", v, got.Info().SecurityProtocol)
		}
	}
}
