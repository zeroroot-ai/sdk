// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Zero Root AI

package capabilitygrant

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/credentials"
)

// grpcCtx returns a context carrying the gRPC RequestInfo grpc-go would attach
// for a call to method. PerRPCCredentials.GetRequestMetadata reads the method
// from here to bind the agent+jwt to it (gibson#1246), so every direct test
// call must supply one.
func grpcCtx(method string) context.Context {
	return credentials.NewContextWithRequestInfo(
		context.Background(),
		credentials.RequestInfo{Method: method},
	)
}

// buildMockPlatform creates an httptest.Server that responds to the discovery
// and registration endpoints. It returns the server and a pointer to a slice
// that accumulates registration request bodies for inspection.
func buildMockPlatform(t *testing.T, capabilities []Capability) (*httptest.Server, *[]registrationRequest) {
	t.Helper()

	var reqs []registrationRequest

	mux := http.NewServeMux()

	// Discovery endpoint.
	mux.HandleFunc("/.well-known/agent-configuration", func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodGet, r.Method)
		w.Header().Set("Content-Type", "application/json")
		// The register URL is filled in after the server starts, so use a
		// placeholder; the test overrides it in the registration handler itself.
		doc := map[string]any{
			"protocol_version": "1.0",
			"provider_name":    "Gibson Test Platform",
			"issuer":           r.Host,
			"default_location": "us-east-1",
			"supported_modes":  []string{"delegated", "autonomous"},
			"endpoints": map[string]string{
				"register":   "REPLACE_REGISTER_URL",
				"execute":    "/agent-auth/execute",
				"list":       "/agent-auth/agents",
				"status":     "/agent-auth/status",
				"revoke":     "/agent-auth/revoke",
				"introspect": "/agent-auth/introspect",
			},
		}
		_ = json.NewEncoder(w).Encode(doc)
	})

	// Registration endpoint.
	mux.HandleFunc("/agent-auth/register", func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodPost, r.Method)
		assert.Contains(t, r.Header.Get("Authorization"), "Bearer ")

		var req registrationRequest
		require.NoError(t, json.NewDecoder(r.Body).Decode(&req))
		reqs = append(reqs, req)

		agentID := "agent-" + req.HostID[:8]
		resp := registrationResponse{
			AgentID:        agentID,
			Capabilities:   capabilities,
			ComponentScope: "component:" + agentID,
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		_ = json.NewEncoder(w).Encode(resp)
	})

	srv := httptest.NewTLSServer(mux)
	t.Cleanup(srv.Close)

	return srv, &reqs
}

// newTestClient creates a Client wired to a mock server. It patches the
// discovery document's register URL to point at the mock server.
func newTestClient(t *testing.T, srv *httptest.Server) *Client {
	t.Helper()

	dir := t.TempDir()

	client, err := NewClient(ClientConfig{
		PlatformURL:    srv.URL,
		BootstrapToken: "test-bootstrap-token",
		HostKeyPath:    filepath.Join(dir, "host_key.json"),
		AgentName:      "test-agent",
		AgentMode:      "autonomous",
	})
	require.NoError(t, err)

	// Use the test server's TLS-aware HTTP client so self-signed cert is accepted.
	client.httpClient = srv.Client()

	return client
}

func TestClientDiscover(t *testing.T) {
	srv, _ := buildMockPlatform(t, nil)
	client := newTestClient(t, srv)

	require.NoError(t, client.Discover(context.Background()))

	client.mu.RLock()
	doc := client.discovery
	client.mu.RUnlock()

	require.NotNil(t, doc)
	assert.Equal(t, "1.0", doc.ProtocolVersion)
	assert.Equal(t, "Gibson Test Platform", doc.ProviderName)
	assert.Contains(t, doc.SupportedModes, "autonomous")
}

func TestClientRegister(t *testing.T) {
	caps := []Capability{
		{Name: "tool:mytool", Description: "Network scanner"},
		{Name: "mission:create", Description: "Create missions"},
	}
	srv, reqs := buildMockPlatform(t, caps)
	client := newTestClient(t, srv)

	// Patch the discovery document directly so the register URL points at the mock.
	client.mu.Lock()
	client.discovery = &DiscoveryDocument{
		ProtocolVersion: "1.0",
		Endpoints: struct {
			Register   string `json:"register"`
			Execute    string `json:"execute"`
			List       string `json:"list"`
			Status     string `json:"status"`
			Revoke     string `json:"revoke"`
			Introspect string `json:"introspect"`
		}{
			Register: srv.URL + "/agent-auth/register",
		},
	}
	client.mu.Unlock()

	require.NoError(t, client.Register(context.Background()))

	// Verify agent ID was set.
	agentID := client.AgentID()
	assert.NotEmpty(t, agentID)
	assert.True(t, strings.HasPrefix(agentID, "agent-"), "agent ID should start with 'agent-'")

	// Verify capabilities.
	assert.True(t, client.HasCapability("tool:mytool"))
	assert.True(t, client.HasCapability("mission:create"))
	assert.False(t, client.HasCapability("tool:mytool-b"))

	allCaps := client.Capabilities()
	assert.Len(t, allCaps, 2)

	// Verify the request body contained keys.
	require.Len(t, *reqs, 1)
	req := (*reqs)[0]
	assert.Equal(t, client.hostID, req.HostID)
	assert.Equal(t, "test-agent", req.AgentName)
	assert.Equal(t, "autonomous", req.AgentMode)
	assert.NotEmpty(t, req.HostKeyJWK)
	assert.NotEmpty(t, req.AgentKeyJWK)
}

func TestClientRegisterBeforeDiscover(t *testing.T) {
	dir := t.TempDir()
	client, err := NewClient(ClientConfig{
		PlatformURL:    "https://localhost:9999",
		BootstrapToken: "tok",
		HostKeyPath:    filepath.Join(dir, "host_key.json"),
		AgentName:      "agent",
		AgentMode:      "autonomous",
	})
	require.NoError(t, err)

	err = client.Register(context.Background())
	assert.Error(t, err, "Register before Discover should return an error")
	assert.Contains(t, err.Error(), "Discover")
}

func TestClientFullFlow(t *testing.T) {
	caps := []Capability{
		{Name: "tool:mytool-c", ComponentRef: "mytool-c-v1", Description: "HTTP probe"},
	}
	srv, _ := buildMockPlatform(t, caps)
	client := newTestClient(t, srv)

	ctx := context.Background()

	// Step 1: Discover.
	require.NoError(t, client.Discover(ctx))

	// Patch the register URL to include the test server base.
	client.mu.Lock()
	client.discovery.Endpoints.Register = srv.URL + "/agent-auth/register"
	client.mu.Unlock()

	// Step 2: Register.
	require.NoError(t, client.Register(ctx))

	// Verify full state.
	assert.NotEmpty(t, client.AgentID())
	assert.NotEmpty(t, client.HostID())
	assert.True(t, client.HasCapability("tool:mytool-c"))
	assert.Equal(t, srv.URL, client.PlatformURL())
}

func TestGRPCPerRPCCredentials(t *testing.T) {
	caps := []Capability{{Name: "tool:mytool"}}
	srv, _ := buildMockPlatform(t, caps)
	client := newTestClient(t, srv)

	// Manually set agentID and componentScope as if registration already happened.
	client.mu.Lock()
	client.agentID = "agent-abc123"
	client.componentScope = "component:agent-abc123"
	client.mu.Unlock()

	creds := client.GRPCPerRPCCredentials()
	require.NotNil(t, creds)

	// Each call should produce a fresh JWT under the dedicated CG header
	// (ADR-0045 decision A — NOT Authorization).
	md1, err := creds.GetRequestMetadata(grpcCtx("/gibson.daemon.v1.DaemonService/Rpc"), "https://example.com")
	require.NoError(t, err)
	assert.Empty(t, md1["authorization"], "per-RPC CG-JWT must not use Authorization")
	assert.NotEmpty(t, md1[MetadataHeaderCapabilityGrant], "per-RPC CG-JWT goes in x-capability-grant")

	md2, err := creds.GetRequestMetadata(grpcCtx("/gibson.daemon.v1.DaemonService/Rpc"), "https://example.com")
	require.NoError(t, err)

	// Tokens should be different (different jti).
	assert.NotEqual(t, md1[MetadataHeaderCapabilityGrant], md2[MetadataHeaderCapabilityGrant],
		"each gRPC call should get a unique JWT")
}

func TestGRPCPerRPCCredentials_RequiresRequestInfo(t *testing.T) {
	caps := []Capability{{Name: "tool:mytool"}}
	srv, _ := buildMockPlatform(t, caps)
	client := newTestClient(t, srv)
	client.mu.Lock()
	client.agentID = "agent-abc123"
	client.componentScope = "component:agent-abc123"
	client.mu.Unlock()

	creds := client.GRPCPerRPCCredentials()

	// A context without gRPC RequestInfo cannot name the method the token must
	// bind to, so signing must fail rather than mint an unbound token.
	_, err := creds.GetRequestMetadata(context.Background(), "https://example.com")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "RequestInfo")
}

func TestGRPCPerRPCCredentials_RequireTransportSecurity(t *testing.T) {
	dir := t.TempDir()
	client, err := NewClient(ClientConfig{
		PlatformURL: "https://localhost:9999",
		HostKeyPath: filepath.Join(dir, "host_key.json"),
		AgentName:   "agent",
		AgentMode:   "autonomous",
	})
	require.NoError(t, err)

	creds := client.GRPCPerRPCCredentials()
	assert.False(t, creds.RequireTransportSecurity())
}

func TestClientNewClient_MissingPlatformURL(t *testing.T) {
	_, err := NewClient(ClientConfig{AgentName: "agent"})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "PlatformURL")
}

func TestClientNewClient_MissingAgentName(t *testing.T) {
	_, err := NewClient(ClientConfig{PlatformURL: "https://example.com"})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "AgentName")
}

func TestClientCapabilities_ReturnsCopy(t *testing.T) {
	dir := t.TempDir()
	client, err := NewClient(ClientConfig{
		PlatformURL: "https://example.com",
		HostKeyPath: filepath.Join(dir, "host_key.json"),
		AgentName:   "agent",
		AgentMode:   "autonomous",
	})
	require.NoError(t, err)

	client.mu.Lock()
	client.capabilities = []Capability{{Name: "cap1"}, {Name: "cap2"}}
	client.mu.Unlock()

	caps := client.Capabilities()
	require.Len(t, caps, 2)

	// Mutating the returned slice must not affect the internal state.
	caps[0].Name = "mutated"
	assert.True(t, client.HasCapability("cap1"), "internal capabilities should be unchanged after mutating copy")
}
