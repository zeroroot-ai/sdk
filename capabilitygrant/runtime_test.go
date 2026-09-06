// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Zero Root AI

package capabilitygrant

import (
	"context"
	"encoding/base64"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func sampleRuntimeCredential(t *testing.T) RuntimeCredential {
	t.Helper()
	key, err := GenerateAgentKey()
	require.NoError(t, err)
	return RuntimeCredential{
		HostID:         "host-thumb",
		AgentID:        "agent-xyz",
		ComponentScope: "component:hello",
		AgentKeySeed:   key.Seed(),
	}
}

func TestAgentKeyFromSeed_RoundTrip(t *testing.T) {
	orig, err := GenerateAgentKey()
	require.NoError(t, err)
	got, err := AgentKeyFromSeed(orig.Seed())
	require.NoError(t, err)
	assert.Equal(t, orig.PrivateKey, got.PrivateKey)
	assert.Equal(t, orig.PublicKey, got.PublicKey)

	_, err = AgentKeyFromSeed([]byte("too-short"))
	assert.Error(t, err)
}

func TestRuntimeCredential_SaveLoadRoundTrip(t *testing.T) {
	rc := sampleRuntimeCredential(t)
	path := filepath.Join(t.TempDir(), "nested", "runtime.json")

	require.NoError(t, SaveRuntimeCredential(path, rc))
	got, err := LoadRuntimeCredential(path)
	require.NoError(t, err)
	assert.Equal(t, rc, got)
}

func TestRuntimeCredential_Base64EnvRoundTrip(t *testing.T) {
	rc := sampleRuntimeCredential(t)
	enc, err := rc.Encode()
	require.NoError(t, err)
	b64 := base64.StdEncoding.EncodeToString(enc)

	got, err := DecodeRuntimeCredentialBase64(b64)
	require.NoError(t, err)
	assert.Equal(t, rc, got)
}

func TestRuntimeCredential_ValidRejectsIncomplete(t *testing.T) {
	assert.Error(t, RuntimeCredential{}.Valid())
	assert.Error(t, SaveRuntimeCredential(filepath.Join(t.TempDir(), "x.json"), RuntimeCredential{AgentID: "a"}))
}

func TestRuntimeCredential_PerRPCCredentials_SignsCGHeader(t *testing.T) {
	rc := sampleRuntimeCredential(t)
	creds, err := rc.PerRPCCredentials()
	require.NoError(t, err)
	assert.False(t, creds.RequireTransportSecurity())

	md, err := creds.GetRequestMetadata(grpcCtx("/gibson.daemon.v1.DaemonService/Rpc"), "https://daemon")
	require.NoError(t, err)
	assert.Empty(t, md["authorization"], "runtime CG-JWT must not use Authorization")
	assert.NotEmpty(t, md[MetadataHeaderCapabilityGrant], "runtime CG-JWT goes in x-capability-grant")

	// A second call mints a distinct token (fresh jti).
	md2, err := creds.GetRequestMetadata(grpcCtx("/gibson.daemon.v1.DaemonService/Rpc"), "https://daemon")
	require.NoError(t, err)
	assert.NotEqual(t, md[MetadataHeaderCapabilityGrant], md2[MetadataHeaderCapabilityGrant])
}

func TestRuntimeCredential_PerRPCCredentials_RequiresRequestInfo(t *testing.T) {
	rc := sampleRuntimeCredential(t)
	creds, err := rc.PerRPCCredentials()
	require.NoError(t, err)

	// Without gRPC RequestInfo in the context there is no method to bind the
	// token to, so signing must fail rather than emit an unbound token.
	_, err = creds.GetRequestMetadata(context.Background(), "https://daemon")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "RequestInfo")
}

func TestClient_RuntimeCredential_RequiresRegistration(t *testing.T) {
	dir := t.TempDir()
	client, err := NewClient(ClientConfig{
		PlatformURL: "https://example.com",
		AgentName:   "hello",
		HostKeyPath: filepath.Join(dir, "host_key.json"),
	})
	require.NoError(t, err)

	// Not yet registered → error.
	_, err = client.RuntimeCredential()
	assert.Error(t, err)

	// Simulate a completed registration.
	client.mu.Lock()
	client.agentID = "agent-xyz"
	client.componentScope = "component:hello"
	client.mu.Unlock()

	rc, err := client.RuntimeCredential()
	require.NoError(t, err)
	assert.Equal(t, "agent-xyz", rc.AgentID)
	assert.Equal(t, "component:hello", rc.ComponentScope)
	assert.NotEmpty(t, rc.AgentKeySeed)
	assert.Equal(t, client.hostID, rc.HostID)

	// The persisted seed reproduces the client's live signing key.
	roundTrip, err := rc.PerRPCCredentials()
	require.NoError(t, err)
	_, err = roundTrip.GetRequestMetadata(grpcCtx("/pkg.Svc/M"), "aud")
	require.NoError(t, err)
}
