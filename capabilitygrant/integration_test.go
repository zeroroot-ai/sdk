// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Zero Root AI

package capabilitygrant_test

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

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
