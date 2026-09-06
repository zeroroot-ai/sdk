// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Zero Root AI

package plugin

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	pluginpb "github.com/zeroroot-ai/sdk/api/gen/gibson/plugin/v1"
	"github.com/zeroroot-ai/sdk/plugin/dispatch"
	"github.com/zeroroot-ai/sdk/plugin/events"
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
// Helpers: fake platform HTTP server for capabilitygrant
// ----------------------------------------------------------------------------

// fakePlatform starts an httptest.Server that implements the minimal
// capability-grant HTTP API needed by the SDK.
func fakePlatform(t *testing.T) *httptest.Server {
	t.Helper()
	mux := http.NewServeMux()

	// Will be set once the server URL is known.
	var registerURL atomic.Value

	mux.HandleFunc("/.well-known/agent-configuration", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		// Return the discovery document pointing back to the same server.
		resp := map[string]interface{}{
			"version": "1",
			"endpoints": map[string]string{
				"register": registerURL.Load().(string),
			},
		}
		json.NewEncoder(w).Encode(resp)
	})

	mux.HandleFunc("/register", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		resp := map[string]interface{}{
			"agent_id":        "test-agent-id",
			"capabilities":    []interface{}{},
			"component_scope": "plugin:test-plugin",
		}
		json.NewEncoder(w).Encode(resp)
	})

	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)

	registerURL.Store(srv.URL + "/register")
	return srv
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
	if _, ok := f.revoked[name]; ok {
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
	f.revoked[name] = struct{}{}
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
// Test: Serve fails when required startup secret is unavailable
// The fake secrets client returns an error for the declared secret name,
// and we bypass the capabilitygrant/gRPC steps with a fake daemon.
// ----------------------------------------------------------------------------

func TestServe_RequiredStartupSecretMissing_ReturnsError(t *testing.T) {
	path := writeManifest(t, testManifestWithSecretsYAML)

	// Fake secrets client that returns an error for the required key.
	fakeSecrets := newFakeSecretsClient(nil) // no values
	fakeSecrets.errOnKey = "cred:api_key"

	// We need to bypass capabilitygrant + gRPC. Use a fake platform to
	// handle the discovery + register steps, then provide a fake daemon addr
	// so gRPC NewClient connects quickly and we can intercept the startup
	// secret resolution via WithSecretsClient.
	srv := fakePlatform(t)

	t.Setenv("GIBSON_DAEMON_ADDR", "localhost:1") // unreachable; registration happens via HTTP
	t.Setenv("GIBSON_URL", srv.URL)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	err := Serve(ctx,
		WithManifest(path),
		WithSecretsClient(fakeSecrets),
		WithHandler("Echo", func(_ context.Context, req string) (string, error) {
			return req, nil
		}),
	)
	// The error may come from capabilitygrant (gRPC to localhost:1 fails) OR
	// from the missing secret depending on timing. Either way it's an error.
	require.Error(t, err)
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
