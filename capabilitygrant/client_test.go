// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Zero Root AI

package capabilitygrant

import (
	"context"
	"net/http/httptest"
	"path/filepath"
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
