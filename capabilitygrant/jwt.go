// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Zero Root AI

package capabilitygrant

import (
	"crypto/ed25519"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"
)

// jwtExpiry is how far in the future agent+jwt tokens expire.
// Kept under the 60-second server-enforced limit.
const jwtExpiry = 55 * time.Second

// AudienceGibsonDaemon is the stable audience every component agent+jwt names,
// and that ext-authz pins via ComponentConfig.ExpectedAudiences (gibson#1246).
//
// It is a FIXED logical identifier for the daemon — NOT the dialled gRPC
// target. Earlier the SDK set aud to whatever endpoint grpc-go handed it, which
// meant the same audience never appeared twice and could not be pinned to a
// constant; per-call request binding now lives in the `method` claim instead.
// Both the Go and TypeScript SDKs mint this exact string, and ext-authz refuses
// any other value, so it is part of the wire contract — do not change it on one
// side alone.
const AudienceGibsonDaemon = "zeroroot.ai/gibson-daemon"

// AgentKey holds an Ed25519 keypair used to sign a component's per-RPC
// agent+jwt. It is generated at registration; its public key is submitted so
// the platform can verify subsequent per-call JWTs.
//
// Under ADR-0045 lifecycle A (persist & reuse) the key is persisted as part of
// the RuntimeCredential (see runtime.go) so a later process — e.g. a standalone
// `gibson inspect` — can re-sign without re-registering. Reconstruct it from a
// stored seed with AgentKeyFromSeed.
type AgentKey struct {
	// PrivateKey is used to sign JWTs.
	PrivateKey ed25519.PrivateKey

	// PublicKey is registered with the platform during agent registration.
	PublicKey ed25519.PublicKey
}

// GenerateAgentKey creates a fresh Ed25519 keypair.
func GenerateAgentKey() (*AgentKey, error) {
	pub, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		return nil, fmt.Errorf("capabilitygrant: generate agent keypair: %w", err)
	}
	return &AgentKey{
		PrivateKey: priv,
		PublicKey:  pub,
	}, nil
}

// AgentKeyFromSeed reconstructs an AgentKey from a persisted 32-byte Ed25519
// seed (ADR-0045 lifecycle A). The seed is the secret half of the keypair; treat
// it accordingly.
func AgentKeyFromSeed(seed []byte) (*AgentKey, error) {
	if len(seed) != ed25519.SeedSize {
		return nil, fmt.Errorf("capabilitygrant: AgentKeyFromSeed: seed must be %d bytes, got %d", ed25519.SeedSize, len(seed))
	}
	priv := ed25519.NewKeyFromSeed(seed)
	return &AgentKey{
		PrivateKey: priv,
		PublicKey:  priv.Public().(ed25519.PublicKey),
	}, nil
}

// Seed returns the 32-byte Ed25519 seed for persistence (ADR-0045 lifecycle A).
func (k *AgentKey) Seed() []byte { return k.PrivateKey.Seed() }

// PublicKeyJWK returns the agent's public key as a public-only OKP JWK.
// Submit this to the platform during registration.
func (k *AgentKey) PublicKeyJWK() json.RawMessage {
	return marshalPublicJWK(k.PublicKey)
}

// SignAgentJWT creates a short-lived agent+jwt Bearer token bound to a single
// gRPC method (gibson#1246). The token is minted per RPC and is useless at any
// other method, so a token captured in flight cannot be replayed against a
// different endpoint.
//
// Token characteristics:
//   - typ: agent+jwt
//   - alg: EdDSA
//   - iss: hostID
//   - sub: agentID
//   - aud: AudienceGibsonDaemon (the stable daemon audience, always)
//   - iat: current time (Unix seconds)
//   - exp: iat + 55 seconds
//   - jti: random UUID for replay prevention
//   - method: the full gRPC method the token is bound to, in grpc-go's
//     "/<proto.package.Service>/<Method>" form — exactly the value ext-authz
//     reads from the request `:path`. Required; the verifier rejects a token
//     whose method claim is absent or does not match the request.
//   - component_scope: the component identifier the dashboard assigned at
//     registration (agent-authoring-and-tenant-entitlements R2). Required;
//     the daemon's FGA interceptor denies any RPC whose JWT lacks this
//     claim, so callers must pass a non-empty value obtained from
//     Client.ComponentScope() after Register.
//
// The token is produced as a compact JWT (header.payload.signature) using
// stdlib only — no third-party JWT library.
func SignAgentJWT(key *AgentKey, hostID, agentID, method, componentScope string) (string, error) {
	if key == nil {
		return "", errors.New("capabilitygrant: SignAgentJWT: key is nil")
	}
	if componentScope == "" {
		return "", errors.New("capabilitygrant: SignAgentJWT: componentScope is required (obtain from Client.ComponentScope())")
	}
	if method == "" {
		return "", errors.New("capabilitygrant: SignAgentJWT: method is required (the full gRPC method the token is bound to)")
	}

	header := map[string]string{
		"typ": "agent+jwt",
		"alg": "EdDSA",
		// kid lets the verifier resolve the signing key without the body. It is
		// the agentID — the daemon registers (and serves) the agent's Ed25519
		// public key under that id, so ext-authz fetches it by kid (ADR-0045,
		// gibson#648). Verify rejects a missing kid, so this is required.
		"kid": agentID,
	}

	now := time.Now()
	jti, err := newUUID()
	if err != nil {
		return "", fmt.Errorf("capabilitygrant: generate jti: %w", err)
	}

	payload := map[string]any{
		"iss":             hostID,
		"sub":             agentID,
		"aud":             AudienceGibsonDaemon,
		"iat":             now.Unix(),
		"exp":             now.Add(jwtExpiry).Unix(),
		"jti":             jti,
		"method":          method,
		"component_scope": componentScope,
	}

	return signJWT(key.PrivateKey, header, payload)
}

// SignHostJWT creates a host+jwt Bearer token for host registration.
//
// Token characteristics:
//   - typ: host+jwt
//   - alg: EdDSA
//   - iss: host ID (JWK thumbprint)
//   - aud: registration endpoint URL
//   - iat: current time (Unix seconds)
//   - exp: iat + 55 seconds
//   - jti: random UUID
func SignHostJWT(key *HostKey, audience string) (string, error) {
	if key == nil {
		return "", errors.New("capabilitygrant: SignHostJWT: key is nil")
	}

	header := map[string]string{
		"typ": "host+jwt",
		"alg": "EdDSA",
	}

	now := time.Now()
	jti, err := newUUID()
	if err != nil {
		return "", fmt.Errorf("capabilitygrant: generate jti: %w", err)
	}

	payload := map[string]any{
		"iss": key.ID,
		"aud": audience,
		"iat": now.Unix(),
		"exp": now.Add(jwtExpiry).Unix(),
		"jti": jti,
	}

	return signJWT(key.PrivateKey, header, payload)
}

// signJWT builds a compact JWT by manually constructing the JSON header and
// payload, base64url-encoding them, and signing with Ed25519. No third-party
// library is used.
func signJWT(priv ed25519.PrivateKey, header map[string]string, payload map[string]any) (string, error) {
	headerJSON, err := json.Marshal(header)
	if err != nil {
		return "", fmt.Errorf("capabilitygrant: marshal JWT header: %w", err)
	}

	payloadJSON, err := json.Marshal(payload)
	if err != nil {
		return "", fmt.Errorf("capabilitygrant: marshal JWT payload: %w", err)
	}

	encodedHeader := base64.RawURLEncoding.EncodeToString(headerJSON)
	encodedPayload := base64.RawURLEncoding.EncodeToString(payloadJSON)

	signingInput := encodedHeader + "." + encodedPayload

	sig := ed25519.Sign(priv, []byte(signingInput))
	encodedSig := base64.RawURLEncoding.EncodeToString(sig)

	return strings.Join([]string{encodedHeader, encodedPayload, encodedSig}, "."), nil
}

// newUUID generates a random UUID v4 string using crypto/rand.
func newUUID() (string, error) {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "", err
	}
	// Set version 4 and variant bits per RFC 4122.
	b[6] = (b[6] & 0x0f) | 0x40
	b[8] = (b[8] & 0x3f) | 0x80
	return fmt.Sprintf("%08x-%04x-%04x-%04x-%012x",
		b[0:4], b[4:6], b[6:8], b[8:10], b[10:16]), nil
}
