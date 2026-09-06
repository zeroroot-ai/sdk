// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Zero Root AI

package plugin

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	componentpb "github.com/zeroroot-ai/sdk/api/gen/gibson/component/v1"
	"github.com/zeroroot-ai/sdk/plugin/manifest"
	"google.golang.org/grpc"
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

	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	baseURL.Store(srv.URL)
	return srv
}

// ----------------------------------------------------------------------------
// Helpers: fake in-process gRPC daemon (ComponentService subset)
// ----------------------------------------------------------------------------

// fakeDaemon implements the ComponentService RPCs Serve exercises:
// RegisterComponent, PollWork, SubmitResult, Heartbeat. Work items are fed
// through workCh; submitted results come out of resultCh.
type fakeDaemon struct {
	componentpb.UnimplementedComponentServiceServer

	mu        sync.Mutex
	registers []*componentpb.RegisterComponentRequest

	workCh   chan *componentpb.PollWorkResponse
	resultCh chan *componentpb.SubmitResultRequest
}

func newFakeDaemon() *fakeDaemon {
	return &fakeDaemon{
		workCh:   make(chan *componentpb.PollWorkResponse, 16),
		resultCh: make(chan *componentpb.SubmitResultRequest, 16),
	}
}

func (d *fakeDaemon) RegisterComponent(_ context.Context, req *componentpb.RegisterComponentRequest) (*componentpb.RegisterComponentResponse, error) {
	d.mu.Lock()
	d.registers = append(d.registers, req)
	d.mu.Unlock()
	return &componentpb.RegisterComponentResponse{
		InstanceId:          "inst-test-1",
		PollTimeoutMs:       50,
		HeartbeatIntervalMs: 60_000,
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

func (d *fakeDaemon) Heartbeat(_ context.Context, _ *componentpb.HeartbeatRequest) (*componentpb.HeartbeatResponse, error) {
	return &componentpb.HeartbeatResponse{}, nil
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

	t.Setenv("GIBSON_DAEMON_ADDR", lis.Addr().String())
	return daemon
}

// dynamicManifest returns an in-memory manifest with dynamic methods enabled
// and no static method declarations.
func dynamicManifest() *manifest.Manifest {
	return &manifest.Manifest{
		APIVersion: manifest.APIVersionV1,
		Kind:       manifest.KindPlugin,
		Metadata: manifest.ManifestMetadata{
			Name:    "dyn-plugin",
			Version: "0.1.0",
		},
		Spec: manifest.ManifestSpec{
			WorkloadClass:  manifest.WorkloadClassPlugin,
			DynamicMethods: true,
		},
	}
}

// ----------------------------------------------------------------------------
// Manifest validation: dynamic_methods relaxes the non-empty methods rule
// ----------------------------------------------------------------------------

func TestManifest_DynamicMethods_AllowsEmptyMethods(t *testing.T) {
	m, err := manifest.LoadBytes([]byte(`
apiVersion: plugin.gibson.zeroroot.ai/v1
kind: Plugin
metadata:
  name: dyn-plugin
  version: 0.1.0
spec:
  workload_class: plugin
  dynamic_methods: true
`))
	require.NoError(t, err)
	assert.True(t, m.Spec.DynamicMethods)
	assert.Empty(t, m.Spec.Methods)
}

func TestManifest_NoMethodsNoDynamic_Fails(t *testing.T) {
	_, err := manifest.LoadBytes([]byte(`
apiVersion: plugin.gibson.zeroroot.ai/v1
kind: Plugin
metadata:
  name: static-plugin
  version: 0.1.0
spec:
  workload_class: plugin
`))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "spec.methods is required")
}

// ----------------------------------------------------------------------------
// Serve startup validation for method sources
// ----------------------------------------------------------------------------

func TestServe_DynamicMethodsWithoutSource_ReturnsError(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	err := Serve(ctx, WithParsedManifest(dynamicManifest()))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "WithMethodSource")
}

func TestServe_MethodSourceWithoutDynamicMethods_ReturnsError(t *testing.T) {
	path := writeManifest(t, testManifestYAML) // static manifest, no dynamic_methods

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	err := Serve(ctx,
		WithManifest(path),
		WithHandler("Echo", func(_ context.Context, req string) (string, error) {
			return req, nil
		}),
		WithMethodSource(func(_ context.Context) ([]DiscoveredMethod, error) {
			return nil, nil
		}),
	)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "dynamic_methods")
}

func TestServe_ParsedManifestInvalid_ReturnsError(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	m := dynamicManifest()
	m.Metadata.Name = "BAD NAME"
	err := Serve(ctx,
		WithParsedManifest(m),
		WithMethodSource(func(_ context.Context) ([]DiscoveredMethod, error) {
			return nil, nil
		}),
	)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "validate manifest")
}

// ----------------------------------------------------------------------------
// Full Serve: discovered methods are registered and invocable end-to-end
// ----------------------------------------------------------------------------

func TestServe_MethodSource_RegistersAndRoundTrips(t *testing.T) {
	t.Setenv("HOME", t.TempDir()) // keep capability-grant host keys out of the real home
	platform := fakeCGPlatform(t)
	t.Setenv("GIBSON_URL", platform.URL)
	daemon := startFakeDaemon(t)

	echoHandler := func(_ context.Context, req json.RawMessage) (json.RawMessage, error) {
		var in struct {
			Msg string `json:"msg"`
		}
		if err := json.Unmarshal(req, &in); err != nil {
			return nil, err
		}
		return json.Marshal(struct {
			Echoed string `json:"echoed"`
		}{Echoed: in.Msg})
	}

	source := func(ctx context.Context) ([]DiscoveredMethod, error) {
		return []DiscoveredMethod{
			{Name: "vendor_echo", Description: "echoes its input", Handler: echoHandler},
		}, nil
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	serveErr := make(chan error, 1)
	go func() {
		serveErr <- Serve(ctx,
			WithParsedManifest(dynamicManifest()),
			WithMethodSource(source),
			WithHealthAddr(":0"),
		)
	}()

	// Enqueue a plugin_invoke for the discovered method. The Go-first wire
	// format carries the JSON request in the Any's value.
	invoke := &pluginpb.PluginInvokeRequest{
		PluginName: "dyn-plugin",
		Method:     "vendor_echo",
		Request:    &anypb.Any{TypeUrl: "json:vendor_echo_request", Value: []byte(`{"msg":"hello"}`)},
		DeadlineMs: 5000,
	}
	payload, err := proto.Marshal(invoke)
	require.NoError(t, err)
	daemon.workCh <- &componentpb.PollWorkResponse{
		WorkId:   "work-1",
		WorkType: "plugin_invoke",
		Payload:  payload,
	}

	// Await the submitted result.
	var result *componentpb.SubmitResultRequest
	select {
	case result = <-daemon.resultCh:
	case <-ctx.Done():
		t.Fatal("timeout waiting for SubmitResult")
	}

	require.Nil(t, result.GetError(), "handler should not error: %v", result.GetError())
	// The result is the handler's raw JSON response, submitted verbatim.
	var resp struct {
		Echoed string `json:"echoed"`
	}
	require.NoError(t, json.Unmarshal(result.GetResult(), &resp))
	assert.Equal(t, "hello", resp.Echoed)

	// The discovered method must be in the RegisterComponent declaration.
	assert.Contains(t, daemon.registeredMethods(t), "vendor_echo")

	// gibson#997: the plugin:* metadata keys the daemon reads to populate the
	// ComponentInstall record are now forwarded (previously written by nobody).
	md := daemon.registeredMetadata(t)
	assert.Equal(t, "process", md["plugin:runtime_mode"], "runtime mode forwarded")
	assert.Equal(t, "false", md["plugin:setec_required"], "setec_required forwarded")
	assert.Equal(t, "trusted", md["plugin:content_trust"], "content_trust forwarded")
	assert.NotEmpty(t, md["plugin:host_id"], "host_id thumbprint forwarded (keys per-host install uniqueness)")

	cancel()
	require.NoError(t, <-serveErr)
}

func TestServe_MethodSource_CollisionWithStatic_ReturnsError(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	platform := fakeCGPlatform(t)
	t.Setenv("GIBSON_URL", platform.URL)
	startFakeDaemon(t)

	m := dynamicManifest()
	m.Spec.Methods = []manifest.MethodDecl{{Name: "vendor_echo"}}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	err := Serve(ctx,
		WithParsedManifest(m),
		WithHandler("vendor_echo", func(_ context.Context, req string) (string, error) {
			return req, nil
		}),
		WithMethodSource(func(_ context.Context) ([]DiscoveredMethod, error) {
			return []DiscoveredMethod{
				{Name: "vendor_echo", Handler: func(_ context.Context, req json.RawMessage) (json.RawMessage, error) {
					return req, nil
				}},
			}, nil
		}),
	)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "collides")
}

func TestServe_MethodSource_EmptySetNoStatic_ReturnsError(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	platform := fakeCGPlatform(t)
	t.Setenv("GIBSON_URL", platform.URL)
	startFakeDaemon(t)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	err := Serve(ctx,
		WithParsedManifest(dynamicManifest()),
		WithMethodSource(func(_ context.Context) ([]DiscoveredMethod, error) {
			return nil, nil
		}),
	)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "no methods to register")
}

func TestManifestHashFromPath(t *testing.T) {
	// Empty path → empty hash (in-memory parsed manifest case).
	if got := manifestHashFromPath(""); got != "" {
		t.Errorf("empty path: got %q, want empty", got)
	}
	// Missing file → empty hash (best-effort).
	if got := manifestHashFromPath(filepath.Join(t.TempDir(), "nope.yaml")); got != "" {
		t.Errorf("missing file: got %q, want empty", got)
	}
	// Real file → deterministic SHA-256 hex of its bytes.
	dir := t.TempDir()
	p := filepath.Join(dir, "plugin.yaml")
	content := []byte("apiVersion: x\nkind: Plugin\n")
	if err := os.WriteFile(p, content, 0o600); err != nil {
		t.Fatal(err)
	}
	sum := sha256.Sum256(content)
	want := hex.EncodeToString(sum[:])
	if got := manifestHashFromPath(p); got != want {
		t.Errorf("hash mismatch: got %q, want %q", got, want)
	}
}
