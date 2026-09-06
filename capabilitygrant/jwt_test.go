// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Zero Root AI

package capabilitygrant

import (
	"crypto/ed25519"
	"encoding/base64"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// parseJWT splits a compact JWT and decodes its header and payload.
// It does NOT verify the signature — verification is tested separately.
func parseJWT(t *testing.T, token string) (header map[string]any, payload map[string]any, rawSig []byte) {
	t.Helper()
	parts := strings.Split(token, ".")
	require.Len(t, parts, 3, "JWT must have exactly 3 parts separated by '.'")

	headerJSON, err := base64.RawURLEncoding.DecodeString(parts[0])
	require.NoError(t, err, "header base64url decode")
	require.NoError(t, json.Unmarshal(headerJSON, &header), "header JSON unmarshal")

	payloadJSON, err := base64.RawURLEncoding.DecodeString(parts[1])
	require.NoError(t, err, "payload base64url decode")
	require.NoError(t, json.Unmarshal(payloadJSON, &payload), "payload JSON unmarshal")

	sig, err := base64.RawURLEncoding.DecodeString(parts[2])
	require.NoError(t, err, "signature base64url decode")

	return header, payload, sig
}

func TestGenerateAgentKey(t *testing.T) {
	key, err := GenerateAgentKey()
	require.NoError(t, err)
	require.NotNil(t, key)
	assert.Len(t, key.PublicKey, ed25519.PublicKeySize)
	assert.Len(t, key.PrivateKey, ed25519.PrivateKeySize)
}

func TestAgentKeyPublicKeyJWK(t *testing.T) {
	key, err := GenerateAgentKey()
	require.NoError(t, err)

	jwk := key.PublicKeyJWK()
	var m map[string]string
	require.NoError(t, json.Unmarshal(jwk, &m))

	assert.Equal(t, "OKP", m["kty"])
	assert.Equal(t, "Ed25519", m["crv"])
	assert.NotEmpty(t, m["x"])
	assert.Empty(t, m["d"], "public JWK must not expose private key material")
}

func TestSignAgentJWT_Structure(t *testing.T) {
	key, err := GenerateAgentKey()
	require.NoError(t, err)

	const (
		hostID         = "test-host-id"
		agentID        = "test-agent-id"
		method         = "/gibson.daemon.v1.DaemonService/Execute"
		componentScope = "component:test-agent-installation-1"
	)

	token, err := SignAgentJWT(key, hostID, agentID, method, componentScope)
	require.NoError(t, err)
	assert.NotEmpty(t, token)

	header, payload, _ := parseJWT(t, token)

	// Header assertions.
	assert.Equal(t, "agent+jwt", header["typ"])
	assert.Equal(t, "EdDSA", header["alg"])

	// Payload assertions.
	assert.Equal(t, hostID, payload["iss"])
	assert.Equal(t, agentID, payload["sub"])
	assert.Equal(t, AudienceGibsonDaemon, payload["aud"],
		"aud must be the stable daemon audience, never the dialled target (gibson#1246)")
	assert.Equal(t, method, payload["method"],
		"method must be present to bind the token to a single RPC (gibson#1246)")
	assert.Equal(t, componentScope, payload["component_scope"],
		"component_scope must be present for daemon FGA narrowing")
	assert.NotEmpty(t, payload["jti"], "jti must be present for replay prevention")

	// Time assertions.
	iat, ok := payload["iat"].(float64)
	require.True(t, ok, "iat must be a number")
	exp, ok := payload["exp"].(float64)
	require.True(t, ok, "exp must be a number")

	delta := exp - iat
	assert.InDelta(t, 55, delta, 2, "exp - iat should be ~55 seconds")

	// Token must not already be expired.
	assert.Greater(t, exp, float64(time.Now().Unix()), "token should not be expired immediately after signing")
}

func TestSignAgentJWT_SignatureVerification(t *testing.T) {
	key, err := GenerateAgentKey()
	require.NoError(t, err)

	token, err := SignAgentJWT(key, "host-id", "agent-id", "/gibson.daemon.v1.DaemonService/Ping", "component:c1")
	require.NoError(t, err)

	parts := strings.Split(token, ".")
	require.Len(t, parts, 3)

	signingInput := parts[0] + "." + parts[1]
	sig, err := base64.RawURLEncoding.DecodeString(parts[2])
	require.NoError(t, err)

	valid := ed25519.Verify(key.PublicKey, []byte(signingInput), sig)
	assert.True(t, valid, "signature must verify with the agent's public key")
}

func TestSignAgentJWT_UniqueJTI(t *testing.T) {
	key, err := GenerateAgentKey()
	require.NoError(t, err)

	_, p1, _ := parseJWT(t, mustSignAgentJWT(t, key))
	_, p2, _ := parseJWT(t, mustSignAgentJWT(t, key))

	jti1, _ := p1["jti"].(string)
	jti2, _ := p2["jti"].(string)
	assert.NotEqual(t, jti1, jti2, "each JWT must have a unique jti")
}

func TestSignAgentJWT_NilKey(t *testing.T) {
	_, err := SignAgentJWT(nil, "h", "a", "/pkg.Svc/M", "component:c")
	assert.Error(t, err)
}

func TestSignAgentJWT_EmptyComponentScope(t *testing.T) {
	key, err := GenerateAgentKey()
	require.NoError(t, err)
	_, err = SignAgentJWT(key, "h", "a", "/pkg.Svc/M", "")
	require.Error(t, err, "signing with empty component_scope must fail")
	assert.Contains(t, err.Error(), "componentScope is required",
		"error message should guide callers to Client.ComponentScope()")
}

func TestSignAgentJWT_EmptyMethod(t *testing.T) {
	key, err := GenerateAgentKey()
	require.NoError(t, err)
	_, err = SignAgentJWT(key, "h", "a", "", "component:c")
	require.Error(t, err, "signing without a method must fail — the token has to be bound to an RPC")
	assert.Contains(t, err.Error(), "method is required")
}

// TestSignAgentJWT_BindsMethodAndStableAudience pins the gibson#1246 request
// binding: the token names the exact gRPC method it may be presented at, and a
// FIXED audience (never the dialled target) so ext-authz can pin it to one
// constant. A different method produces a token with a different method claim,
// which is what lets the verifier reject a cross-RPC replay.
func TestSignAgentJWT_BindsMethodAndStableAudience(t *testing.T) {
	key, err := GenerateAgentKey()
	require.NoError(t, err)

	const m1 = "/gibson.daemon.v1.DaemonService/GetMission"
	const m2 = "/gibson.daemon.v1.DaemonService/DeleteMission"

	tok1, err := SignAgentJWT(key, "h", "a", m1, "component:c")
	require.NoError(t, err)
	tok2, err := SignAgentJWT(key, "h", "a", m2, "component:c")
	require.NoError(t, err)

	_, p1, _ := parseJWT(t, tok1)
	_, p2, _ := parseJWT(t, tok2)

	assert.Equal(t, m1, p1["method"])
	assert.Equal(t, m2, p2["method"])
	assert.Equal(t, AudienceGibsonDaemon, p1["aud"])
	assert.Equal(t, AudienceGibsonDaemon, p2["aud"],
		"the audience is a stable constant and does not vary per method")
}

func TestSignHostJWT_Structure(t *testing.T) {
	hostKey, err := GenerateHostKey()
	require.NoError(t, err)

	audience := "https://platform.example.com/agent-auth/register"
	token, err := SignHostJWT(hostKey, audience)
	require.NoError(t, err)
	require.NotEmpty(t, token)

	header, payload, _ := parseJWT(t, token)

	assert.Equal(t, "host+jwt", header["typ"])
	assert.Equal(t, "EdDSA", header["alg"])

	assert.Equal(t, hostKey.ID, payload["iss"])
	assert.Equal(t, audience, payload["aud"])
	assert.NotEmpty(t, payload["jti"])

	iat, _ := payload["iat"].(float64)
	exp, _ := payload["exp"].(float64)
	assert.InDelta(t, 55, exp-iat, 2)
}

func TestSignHostJWT_SignatureVerification(t *testing.T) {
	hostKey, err := GenerateHostKey()
	require.NoError(t, err)

	token, err := SignHostJWT(hostKey, "https://example.com/register")
	require.NoError(t, err)

	parts := strings.Split(token, ".")
	require.Len(t, parts, 3)

	signingInput := parts[0] + "." + parts[1]
	sig, err := base64.RawURLEncoding.DecodeString(parts[2])
	require.NoError(t, err)

	valid := ed25519.Verify(hostKey.PublicKey, []byte(signingInput), sig)
	assert.True(t, valid, "host JWT signature must verify with the host's public key")
}

func TestSignHostJWT_NilKey(t *testing.T) {
	_, err := SignHostJWT(nil, "aud")
	assert.Error(t, err)
}

func TestNewUUID(t *testing.T) {
	u1, err := newUUID()
	require.NoError(t, err)
	u2, err := newUUID()
	require.NoError(t, err)

	assert.NotEqual(t, u1, u2, "UUIDs should be unique")
	// Basic format check: 8-4-4-4-12
	parts := strings.Split(u1, "-")
	require.Len(t, parts, 5)
	assert.Len(t, parts[0], 8)
	assert.Len(t, parts[1], 4)
	assert.Len(t, parts[2], 4)
	assert.Len(t, parts[3], 4)
	assert.Len(t, parts[4], 12)
}

// mustSignAgentJWT is a test helper that fails the test on error.
func mustSignAgentJWT(t *testing.T, key *AgentKey) string {
	t.Helper()
	token, err := SignAgentJWT(key, "h", "a", "/pkg.Svc/M", "component:test")
	require.NoError(t, err)
	return token
}

// TestSignAgentJWT_SetsKidToAgentID pins the gibson#648 / ADR-0045 fix: the
// per-RPC agent+jwt must carry a kid so the verifier can resolve the signing
// key (Verify rejects a missing kid). The kid is the agentID — the daemon
// registers and serves the agent's public key under that id.
func TestSignAgentJWT_SetsKidToAgentID(t *testing.T) {
	key, err := GenerateAgentKey()
	require.NoError(t, err)

	const agentID = "agt_abc123"
	tok, err := SignAgentJWT(key, "host_xyz", agentID, "/pkg.Svc/M", "component:demo")
	require.NoError(t, err)

	header, payload, _ := parseJWT(t, tok)
	assert.Equal(t, agentID, header["kid"], "kid must be the agentID for key resolution")
	assert.Equal(t, "agent+jwt", header["typ"])
	assert.Equal(t, agentID, payload["sub"])
	assert.Equal(t, "component:demo", payload["component_scope"])
}
