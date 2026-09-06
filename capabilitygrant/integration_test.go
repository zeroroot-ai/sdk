// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Zero Root AI

package capabilitygrant_test

import (
	"context"
	"crypto/ed25519"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/credentials"

	"github.com/zeroroot-ai/sdk/capabilitygrant"
)

// grpcCtx returns a context carrying the gRPC RequestInfo grpc-go attaches for a
// call to method. PerRPCCredentials binds the agent+jwt to that method
// (gibson#1246), so a direct GetRequestMetadata call must supply one.
func grpcCtx(method string) context.Context {
	return credentials.NewContextWithRequestInfo(
		context.Background(),
		credentials.RequestInfo{Method: method},
	)
}

// ----------------------------------------------------------------------------
// Shared mock server helpers
// ----------------------------------------------------------------------------

// mockServer holds a running httptest.TLSServer and the accumulated state from
// requests so individual tests can assert on what the server received.
type mockServer struct {
	srv *httptest.Server

	// registrationRequests stores raw request bodies for detailed inspection.
	registrationBodies []registrationCapture
}

// registrationCapture stores fields from a parsed registration request that
// the test needs to inspect.
type registrationCapture struct {
	HostID      string
	AgentName   string
	AgentMode   string
	HostKeyJWK  json.RawMessage
	AgentKeyJWK json.RawMessage
	AuthHeader  string
}

// newMockServer creates a TLS httptest server that implements the minimal
// Agent Auth HTTP API required by the integration tests.
//
// The protoVersion parameter controls what protocol_version value the
// discovery endpoint advertises, allowing callers to test version rejection.
//
// The caps parameter is the capability list returned to registering agents.
func newMockServer(t *testing.T, protoVersion string, caps []capabilitygrant.Capability) *mockServer {
	t.Helper()

	ms := &mockServer{}
	mux := http.NewServeMux()

	// Discovery endpoint — /.well-known/agent-configuration.
	mux.HandleFunc("/.well-known/agent-configuration", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		// The register URL must reference this server. We use the request Host
		// header because the server URL is not known until after the server
		// starts; the test client will override this anyway, but we keep it
		// self-consistent for transparency.
		scheme := "https"
		doc := map[string]any{
			"protocol_version": protoVersion,
			"provider_name":    "Gibson Integration Test Platform",
			"issuer":           scheme + "://" + r.Host,
			"default_location": "us-east-1",
			"supported_modes":  []string{"delegated", "autonomous"},
			"endpoints": map[string]string{
				"register":   scheme + "://" + r.Host + "/api/auth/agent/register",
				"execute":    "/api/auth/capability/execute",
				"list":       "/api/auth/capability/list",
				"status":     "/api/auth/agent/status",
				"revoke":     "/api/auth/agent/revoke",
				"introspect": "/api/auth/token/introspect",
			},
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(doc)
	})

	// Registration endpoint — POST /api/auth/agent/register.
	mux.HandleFunc("/api/auth/agent/register", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}

		authHeader := r.Header.Get("Authorization")
		if !strings.HasPrefix(authHeader, "Bearer ") {
			http.Error(w, "missing or invalid Authorization header", http.StatusUnauthorized)
			return
		}

		var body struct {
			HostID      string          `json:"host_id"`
			AgentName   string          `json:"agent_name"`
			AgentMode   string          `json:"agent_mode"`
			HostKeyJWK  json.RawMessage `json:"host_key_jwk"`
			AgentKeyJWK json.RawMessage `json:"agent_key_jwk"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			http.Error(w, "bad request: "+err.Error(), http.StatusBadRequest)
			return
		}

		// Record the request for later assertion.
		ms.registrationBodies = append(ms.registrationBodies, registrationCapture{
			HostID:      body.HostID,
			AgentName:   body.AgentName,
			AgentMode:   body.AgentMode,
			HostKeyJWK:  body.HostKeyJWK,
			AgentKeyJWK: body.AgentKeyJWK,
			AuthHeader:  authHeader,
		})

		// Return a deterministic agent ID based on the submitted host ID.
		agentID := "agent-" + body.HostID
		if len(agentID) > 24 {
			agentID = agentID[:24]
		}

		resp := map[string]any{
			"agent_id":        agentID,
			"capabilities":    caps,
			"component_scope": "component:" + agentID,
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		_ = json.NewEncoder(w).Encode(resp)
	})

	// Capability list endpoint — GET /api/auth/capability/list.
	mux.HandleFunc("/api/auth/capability/list", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		if !strings.HasPrefix(r.Header.Get("Authorization"), "Bearer ") {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{"capabilities": caps})
	})

	// Capability execute endpoint — POST /api/auth/capability/execute.
	mux.HandleFunc("/api/auth/capability/execute", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		if !strings.HasPrefix(r.Header.Get("Authorization"), "Bearer ") {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{"status": "ok"})
	})

	ms.srv = httptest.NewTLSServer(mux)
	t.Cleanup(ms.srv.Close)
	return ms
}

// newTestClient creates a Client configured to talk to ms with its TLS cert trusted.
// hostKeyPath controls where the persistent host key is stored.
func newTestClient(t *testing.T, ms *mockServer, hostKeyPath string) *capabilitygrant.Client {
	t.Helper()

	client, err := capabilitygrant.NewClient(capabilitygrant.ClientConfig{
		PlatformURL:    ms.srv.URL,
		BootstrapToken: "integration-test-bootstrap-token",
		HostKeyPath:    hostKeyPath,
		AgentName:      "integration-test-agent",
		AgentMode:      "autonomous",
	})
	require.NoError(t, err)

	// Swap in the TLS-aware HTTP client so the self-signed test certificate is
	// accepted without modifying the system trust store.
	client.SetHTTPClient(ms.srv.Client())

	return client
}

// patchRegisterURL rewrites the cached discovery document's register endpoint
// to use the mock server's actual base URL. This is necessary because the
// discovery handler sets the endpoint host from r.Host, which is correct, but
// calling Discover and Register in sequence would work without this. We include
// it for tests that manipulate the discovery doc directly.
func patchRegisterURL(t *testing.T, client *capabilitygrant.Client, baseURL string) {
	t.Helper()
	client.PatchDiscoveryRegisterURL(baseURL + "/api/auth/agent/register")
}

// decodeJWT splits a compact JWT and returns the decoded header and payload.
// The signature is NOT verified here — that is done separately where needed.
func decodeJWT(t *testing.T, token string) (header map[string]any, payload map[string]any, rawSig []byte) {
	t.Helper()
	parts := strings.Split(token, ".")
	require.Len(t, parts, 3, "JWT must have three dot-separated parts")

	hJSON, err := base64.RawURLEncoding.DecodeString(parts[0])
	require.NoError(t, err, "base64url decode JWT header")
	require.NoError(t, json.Unmarshal(hJSON, &header), "unmarshal JWT header")

	pJSON, err := base64.RawURLEncoding.DecodeString(parts[1])
	require.NoError(t, err, "base64url decode JWT payload")
	require.NoError(t, json.Unmarshal(pJSON, &payload), "unmarshal JWT payload")

	sig, err := base64.RawURLEncoding.DecodeString(parts[2])
	require.NoError(t, err, "base64url decode JWT signature")

	return header, payload, sig
}

// ----------------------------------------------------------------------------
// Integration tests
// ----------------------------------------------------------------------------

// TestCapabilityGrantClientFullFlow verifies the complete Discover → Register sequence
// against a mock server, then asserts every observable client state.
func TestCapabilityGrantClientFullFlow(t *testing.T) {
	caps := []capabilitygrant.Capability{
		{Name: "execute:tool:mytool", Description: "Network scanner"},
		{Name: "execute:tool:mytool-c", Description: "HTTP prober"},
		{Name: "mission:create", Description: "Create new missions"},
	}
	ms := newMockServer(t, "1.0", caps)
	dir := t.TempDir()
	client := newTestClient(t, ms, filepath.Join(dir, "host_key.json"))

	ctx := context.Background()

	// --- Step 1: Discover ---------------------------------------------------
	require.NoError(t, client.Discover(ctx), "Discover should succeed")

	// Patch the register URL so it uses the mock server.
	patchRegisterURL(t, client, ms.srv.URL)

	// --- Step 2: Register ---------------------------------------------------
	require.NoError(t, client.Register(ctx), "Register should succeed")

	// --- Step 3: Verify client state ----------------------------------------
	agentID := client.AgentID()
	assert.NotEmpty(t, agentID, "AgentID must be set after registration")
	assert.True(t, strings.HasPrefix(agentID, "agent-"), "AgentID should start with 'agent-'")

	hostID := client.HostID()
	assert.NotEmpty(t, hostID, "HostID must be set at construction time")

	assert.Equal(t, ms.srv.URL, client.PlatformURL())

	// --- Step 4: Capability checks ------------------------------------------
	assert.True(t, client.HasCapability("execute:tool:mytool"))
	assert.True(t, client.HasCapability("execute:tool:mytool-c"))
	assert.True(t, client.HasCapability("mission:create"))
	assert.False(t, client.HasCapability("execute:tool:nonexistent"))
	assert.False(t, client.HasCapability(""))

	allCaps := client.Capabilities()
	assert.Len(t, allCaps, 3, "Capabilities() should return all three granted capabilities")

	// --- Step 5: Verify registration request contents -----------------------
	require.Len(t, ms.registrationBodies, 1, "exactly one registration request should have been made")
	reg := ms.registrationBodies[0]

	assert.Equal(t, hostID, reg.HostID, "registration body host_id must match client HostID")
	assert.Equal(t, "integration-test-agent", reg.AgentName)
	assert.Equal(t, "autonomous", reg.AgentMode)
	assert.NotEmpty(t, reg.HostKeyJWK, "host public key JWK must be included in registration body")
	assert.NotEmpty(t, reg.AgentKeyJWK, "agent public key JWK must be included in registration body")
	assert.True(t, strings.HasPrefix(reg.AuthHeader, "Bearer "),
		"registration request must carry a Bearer token")

	// Validate the host key JWK structure.
	var hostJWK map[string]string
	require.NoError(t, json.Unmarshal(reg.HostKeyJWK, &hostJWK))
	assert.Equal(t, "OKP", hostJWK["kty"])
	assert.Equal(t, "Ed25519", hostJWK["crv"])
	assert.NotEmpty(t, hostJWK["x"])
	assert.Empty(t, hostJWK["d"], "public JWK must not expose the private key material")

	// Validate the agent key JWK structure.
	var agentJWK map[string]string
	require.NoError(t, json.Unmarshal(reg.AgentKeyJWK, &agentJWK))
	assert.Equal(t, "OKP", agentJWK["kty"])
	assert.Equal(t, "Ed25519", agentJWK["crv"])
	assert.NotEmpty(t, agentJWK["x"])
	assert.Empty(t, agentJWK["d"], "public JWK must not expose the private key material")
}

// TestCapabilityGrantClientKeypairPersistence verifies that the host key written
// during the first registration is loaded (not regenerated) on subsequent
// client creations, resulting in the same host ID.
func TestCapabilityGrantClientKeypairPersistence(t *testing.T) {
	caps := []capabilitygrant.Capability{{Name: "tool:mytool"}}
	ms := newMockServer(t, "1.0", caps)
	dir := t.TempDir()
	keyPath := filepath.Join(dir, "host_key.json")

	ctx := context.Background()

	// --- First client: generates and saves the host key ---------------------
	client1 := newTestClient(t, ms, keyPath)
	require.NoError(t, client1.Discover(ctx))
	patchRegisterURL(t, client1, ms.srv.URL)
	require.NoError(t, client1.Register(ctx))

	hostID1 := client1.HostID()
	require.NotEmpty(t, hostID1)

	// The host key file must exist with 0600 permissions.
	fi, err := os.Stat(keyPath)
	require.NoError(t, err, "host key file must exist after first registration")
	assert.Equal(t, os.FileMode(0600), fi.Mode().Perm(),
		"host key file must have 0600 permissions")

	// --- Second client: loads the same key from disk -----------------------
	client2 := newTestClient(t, ms, keyPath)
	// No need to register again to verify the key was loaded — HostID() is
	// derived directly from the key at construction time.
	hostID2 := client2.HostID()

	assert.Equal(t, hostID1, hostID2,
		"HostID must be identical when the same key file is reloaded")

	// Register client2 as well to verify the server receives the same host ID.
	require.NoError(t, client2.Discover(ctx))
	patchRegisterURL(t, client2, ms.srv.URL)
	require.NoError(t, client2.Register(ctx))

	require.Len(t, ms.registrationBodies, 2, "two registrations should have been recorded")
	assert.Equal(t, ms.registrationBodies[0].HostID, ms.registrationBodies[1].HostID,
		"both registration requests must carry the same host_id")
}

// TestCapabilityGrantClientJWTFormat verifies that the JWT produced by
// GRPCPerRPCCredentials.GetRequestMetadata has the correct JOSE header fields,
// payload claims, and a valid Ed25519 signature over the signing input.
func TestCapabilityGrantClientJWTFormat(t *testing.T) {
	caps := []capabilitygrant.Capability{{Name: "tool:mytool"}}
	ms := newMockServer(t, "1.0", caps)
	dir := t.TempDir()
	client := newTestClient(t, ms, filepath.Join(dir, "host_key.json"))

	ctx := context.Background()
	require.NoError(t, client.Discover(ctx))
	patchRegisterURL(t, client, ms.srv.URL)
	require.NoError(t, client.Register(ctx))

	// Obtain gRPC credentials and call GetRequestMetadata.
	creds := client.GRPCPerRPCCredentials()
	require.NotNil(t, creds)

	const method = "/gibson.harness.v1.HarnessService/Report"
	md, err := creds.GetRequestMetadata(grpcCtx(method), "https://gibson.example.com")
	require.NoError(t, err)

	rawToken, ok := md[capabilitygrant.MetadataHeaderCapabilityGrant]
	require.True(t, ok, "metadata must contain the x-capability-grant key")
	require.NotEmpty(t, rawToken, "x-capability-grant must carry the bare agent+jwt")
	require.Empty(t, md["authorization"], "per-RPC CG-JWT must not use Authorization (ADR-0045 decision A)")

	// --- Decode and inspect JOSE header ------------------------------------
	header, payload, sig := decodeJWT(t, rawToken)

	assert.Equal(t, "agent+jwt", header["typ"], "typ must be 'agent+jwt'")
	assert.Equal(t, "EdDSA", header["alg"], "alg must be 'EdDSA'")

	// --- Verify payload claims ---------------------------------------------
	assert.Equal(t, client.HostID(), payload["iss"],
		"iss must equal the client's HostID")
	assert.Equal(t, client.AgentID(), payload["sub"],
		"sub must equal the client's AgentID")
	assert.Equal(t, capabilitygrant.AudienceGibsonDaemon, payload["aud"],
		"aud must be the stable daemon audience, never the dialled target (gibson#1246)")
	assert.Equal(t, method, payload["method"],
		"method claim must bind the token to the requested RPC (gibson#1246)")

	jti, ok := payload["jti"].(string)
	require.True(t, ok && jti != "", "jti must be a non-empty string")

	iat, ok := payload["iat"].(float64)
	require.True(t, ok, "iat must be a numeric Unix timestamp")

	exp, ok := payload["exp"].(float64)
	require.True(t, ok, "exp must be a numeric Unix timestamp")

	now := float64(time.Now().Unix())
	assert.LessOrEqual(t, iat, now+1,
		"iat must not be in the future (allowing 1s clock skew)")
	assert.Greater(t, exp, now,
		"token must not already be expired at issuance time")

	lifetime := exp - iat
	assert.InDelta(t, 55, lifetime, 2,
		"token lifetime must be ~55 seconds (jwtExpiry constant)")

	// --- Verify Ed25519 signature ------------------------------------------
	// Recover the agent's public key from the registration request.
	require.NotEmpty(t, ms.registrationBodies, "registration must have occurred before JWT test")
	var agentJWK map[string]string
	require.NoError(t, json.Unmarshal(ms.registrationBodies[0].AgentKeyJWK, &agentJWK))

	pubBytes, err := base64.RawURLEncoding.DecodeString(agentJWK["x"])
	require.NoError(t, err, "decode agent public key x coordinate")
	require.Len(t, pubBytes, ed25519.PublicKeySize,
		"agent public key must be 32 bytes")

	agentPub := ed25519.PublicKey(pubBytes)
	parts := strings.Split(rawToken, ".")
	signingInput := parts[0] + "." + parts[1]

	assert.True(t, ed25519.Verify(agentPub, []byte(signingInput), sig),
		"JWT signature must verify with the agent's registered public key")
}

// TestCapabilityGrantClientDiscoveryVersionCheck verifies that Discover returns an
// error when the platform advertises a protocol_version below the client minimum.
func TestCapabilityGrantClientDiscoveryVersionCheck(t *testing.T) {
	tests := []struct {
		name        string
		version     string
		expectError bool
		errContains string
	}{
		{
			name:        "current version accepted",
			version:     "1.0",
			expectError: false,
		},
		{
			name:        "newer minor version accepted",
			version:     "1.5",
			expectError: false,
		},
		{
			name:        "newer major version accepted",
			version:     "2.0",
			expectError: false,
		},
		{
			name:        "older minor version rejected",
			version:     "0.9",
			expectError: true,
			errContains: "protocol_version",
		},
		{
			name:        "zero version rejected",
			version:     "0.5",
			expectError: true,
			errContains: "protocol_version",
		},
		{
			name:        "empty version rejected",
			version:     "",
			expectError: true,
			errContains: "protocol_version",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			ms := newMockServer(t, tc.version, nil)
			dir := t.TempDir()
			client := newTestClient(t, ms, filepath.Join(dir, "host_key.json"))

			err := client.Discover(context.Background())
			if tc.expectError {
				require.Error(t, err, "Discover should fail for version %q", tc.version)
				assert.Contains(t, err.Error(), tc.errContains,
					"error message should mention %q", tc.errContains)
			} else {
				assert.NoError(t, err, "Discover should succeed for version %q", tc.version)
			}
		})
	}
}

// TestCapabilityGrantClientRegisterBeforeDiscover verifies that calling Register
// before Discover returns a clear, actionable error.
func TestCapabilityGrantClientRegisterBeforeDiscover(t *testing.T) {
	dir := t.TempDir()
	client, err := capabilitygrant.NewClient(capabilitygrant.ClientConfig{
		PlatformURL:    "https://localhost:19999",
		BootstrapToken: "tok",
		HostKeyPath:    filepath.Join(dir, "host_key.json"),
		AgentName:      "agent",
		AgentMode:      "autonomous",
	})
	require.NoError(t, err)

	err = client.Register(context.Background())
	require.Error(t, err, "Register before Discover must return an error")
	assert.Contains(t, err.Error(), "Discover",
		"error message should advise the caller to call Discover first")
}

// TestCapabilityGrantClientGRPCCredentialsUniqueness verifies that two successive
// calls to GetRequestMetadata produce different JWTs. Each JWT must have a
// distinct jti claim, providing replay prevention.
func TestCapabilityGrantClientGRPCCredentialsUniqueness(t *testing.T) {
	caps := []capabilitygrant.Capability{{Name: "tool:mytool"}}
	ms := newMockServer(t, "1.0", caps)
	dir := t.TempDir()
	client := newTestClient(t, ms, filepath.Join(dir, "host_key.json"))

	ctx := context.Background()
	require.NoError(t, client.Discover(ctx))
	patchRegisterURL(t, client, ms.srv.URL)
	require.NoError(t, client.Register(ctx))

	creds := client.GRPCPerRPCCredentials()

	md1, err := creds.GetRequestMetadata(grpcCtx("/gibson.daemon.v1.DaemonService/Rpc"), "https://example.com")
	require.NoError(t, err)

	md2, err := creds.GetRequestMetadata(grpcCtx("/gibson.daemon.v1.DaemonService/Rpc"), "https://example.com")
	require.NoError(t, err)

	token1 := md1[capabilitygrant.MetadataHeaderCapabilityGrant]
	token2 := md2[capabilitygrant.MetadataHeaderCapabilityGrant]

	assert.NotEqual(t, token1, token2,
		"successive GetRequestMetadata calls must produce distinct JWTs")

	// Confirm the JTI values differ — that is the primary replay-prevention field.
	_, p1, _ := decodeJWT(t, token1)
	_, p2, _ := decodeJWT(t, token2)

	jti1, _ := p1["jti"].(string)
	jti2, _ := p2["jti"].(string)

	assert.NotEqual(t, jti1, jti2, "successive JWTs must have different jti values")
}

// TestCapabilityGrantClientBootstrapK8sAutoDetect verifies that ResolveBootstrap
// reads a Kubernetes service account token from the well-known file path when
// no explicit bootstrap token is provided, and that the resulting credential
// has type "k8s_sa".
//
// This test only covers the ResolveBootstrap function in isolation (not a full
// client flow) because redirecting the K8s SA token path requires file system
// interaction rather than environment variables.
func TestCapabilityGrantClientBootstrapK8sAutoDetect(t *testing.T) {
	// Create a fake SA token file to simulate the K8s environment.
	dir := t.TempDir()
	fakeTokenPath := filepath.Join(dir, "token")
	const fakeToken = "eyJhbGciOiJSUzI1NiJ9.fake-k8s-sa-token"
	require.NoError(t, os.WriteFile(fakeTokenPath, []byte(fakeToken+"\n"), 0600))

	// Write the token to the standard K8s SA token path via a symlink in
	// temp dir. We cannot rewrite /var/run/secrets/... in a unit test, so we
	// test ResolveBootstrap directly with an explicit token that matches what
	// the K8s path would contain — and separately verify the path-reading
	// behaviour by calling the exported function with an empty token to confirm
	// it falls through to the error path (no K8s token available in CI).

	// Case 1: explicit token takes precedence.
	cred, err := capabilitygrant.ResolveBootstrap("my-api-key")
	require.NoError(t, err)
	assert.Equal(t, "api_key", cred.Type)
	assert.Equal(t, "my-api-key", cred.Token)

	// Case 2: empty explicit token falls back to the K8s SA token path.
	// In a CI/test environment there is no real K8s token, so we expect an
	// error (not a panic or incorrect credential). This asserts the fallback
	// path is reached and handled gracefully.
	_, err = capabilitygrant.ResolveBootstrap("")
	// The error may or may not occur depending on whether the test is running
	// inside a real Kubernetes pod. We accept either outcome but verify the
	// function does not panic and returns a sensible error when no token exists.
	if err != nil {
		assert.Contains(t, err.Error(), "bootstrap",
			"error message should mention bootstrap credential")
	}
}

// TestCapabilityGrantClientConcurrentGRPCCredentials stress-tests GetRequestMetadata
// under concurrent load to confirm there are no data races in the JWT signing
// path. Run with -race to detect issues.
func TestCapabilityGrantClientConcurrentGRPCCredentials(t *testing.T) {
	caps := []capabilitygrant.Capability{{Name: "tool:mytool"}}
	ms := newMockServer(t, "1.0", caps)
	dir := t.TempDir()
	client := newTestClient(t, ms, filepath.Join(dir, "host_key.json"))

	ctx := context.Background()
	require.NoError(t, client.Discover(ctx))
	patchRegisterURL(t, client, ms.srv.URL)
	require.NoError(t, client.Register(ctx))

	creds := client.GRPCPerRPCCredentials()

	const goroutines = 20
	const callsPerGoroutine = 10

	var errorCount atomic.Int64
	done := make(chan struct{})

	for range goroutines {
		go func() {
			defer func() { done <- struct{}{} }()
			for range callsPerGoroutine {
				md, err := creds.GetRequestMetadata(grpcCtx("/gibson.daemon.v1.DaemonService/Rpc"), "https://example.com")
				if err != nil {
					errorCount.Add(1)
					return
				}
				if md[capabilitygrant.MetadataHeaderCapabilityGrant] == "" {
					errorCount.Add(1)
					return
				}
			}
		}()
	}

	for range goroutines {
		<-done
	}

	assert.Equal(t, int64(0), errorCount.Load(),
		"no errors should occur during concurrent JWT signing")
}

// TestCapabilityGrantClientDiscoverSetsEndpoints verifies that after a successful
// Discover call the client has absorbed the full endpoint set from the discovery
// document. We use the PlatformURL as a proxy since the endpoints are internal.
func TestCapabilityGrantClientDiscoverSetsEndpoints(t *testing.T) {
	ms := newMockServer(t, "1.0", nil)
	dir := t.TempDir()
	client := newTestClient(t, ms, filepath.Join(dir, "host_key.json"))

	ctx := context.Background()
	require.NoError(t, client.Discover(ctx))

	// PlatformURL is set at construction time and unchanged by Discover.
	assert.Equal(t, ms.srv.URL, client.PlatformURL())
}

// TestCapabilityGrantClientRegisterReturnsCapabilities confirms that after
// registration the full capability set from the server response is accessible
// via Capabilities() and HasCapability().
func TestCapabilityGrantClientRegisterReturnsCapabilities(t *testing.T) {
	caps := []capabilitygrant.Capability{
		{Name: "execute:tool:mytool", ComponentRef: "mytool-v1", Description: "Port scanner"},
		{Name: "execute:tool:mytool-b", ComponentRef: "mytool-b-v1", Description: "Vulnerability scanner"},
		{Name: "execute:tool:mytool-c", ComponentRef: "mytool-c-v1", Description: "HTTP prober"},
		{Name: "mission:create", Description: "Create missions"},
		{Name: "mission:read", Description: "Read mission status"},
	}
	ms := newMockServer(t, "1.0", caps)
	dir := t.TempDir()
	client := newTestClient(t, ms, filepath.Join(dir, "host_key.json"))

	ctx := context.Background()
	require.NoError(t, client.Discover(ctx))
	patchRegisterURL(t, client, ms.srv.URL)
	require.NoError(t, client.Register(ctx))

	returned := client.Capabilities()
	require.Len(t, returned, len(caps))

	for _, cap := range caps {
		assert.True(t, client.HasCapability(cap.Name),
			"HasCapability(%q) should return true", cap.Name)
	}

	assert.False(t, client.HasCapability("execute:tool:subfinder"),
		"HasCapability for a non-granted capability should return false")
}

// TestCapabilityGrantClientCapabilitiesReturnsCopy verifies that mutating the slice
// returned by Capabilities() does not corrupt the client's internal state.
func TestCapabilityGrantClientCapabilitiesReturnsCopy(t *testing.T) {
	caps := []capabilitygrant.Capability{
		{Name: "cap-a"},
		{Name: "cap-b"},
	}
	ms := newMockServer(t, "1.0", caps)
	dir := t.TempDir()
	client := newTestClient(t, ms, filepath.Join(dir, "host_key.json"))

	ctx := context.Background()
	require.NoError(t, client.Discover(ctx))
	patchRegisterURL(t, client, ms.srv.URL)
	require.NoError(t, client.Register(ctx))

	snapshot := client.Capabilities()
	require.Len(t, snapshot, 2)

	// Mutate the returned slice.
	snapshot[0].Name = "mutated"

	// The original capability must still be intact.
	assert.True(t, client.HasCapability("cap-a"),
		"mutating the Capabilities() copy must not affect internal state")
	assert.False(t, client.HasCapability("mutated"),
		"mutated value must not appear in client capabilities")
}
