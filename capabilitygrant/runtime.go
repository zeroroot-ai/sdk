// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Zero Root AI

package capabilitygrant

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"

	"google.golang.org/grpc/credentials"
)

// RuntimeCredential is the persisted material a registered component needs to
// authenticate per-RPC WITHOUT re-registering (ADR-0045 lifecycle A — persist &
// reuse). A long-running serve loop and a standalone `gibson inspect` both load
// it and sign their own agent+jwt.
//
// It is DISTINCT from the enrollment credential (client_id / bootstrap token):
// the enrollment credential authorizes first registration; this is the runtime
// signing material produced BY registration. The AgentKeySeed is the secret half
// of the agent key — persist it 0600 and treat it like a private key.
type RuntimeCredential struct {
	// HostID is the registered host's JWK thumbprint (agent+jwt iss).
	HostID string `json:"host_id"`
	// AgentID is the platform-assigned agent id — the agent+jwt sub AND kid
	// (ext-authz resolves the verifying key + principal descriptor by it).
	AgentID string `json:"agent_id"`
	// ComponentScope is the FGA component identifier bound at registration;
	// SignAgentJWT requires it.
	ComponentScope string `json:"component_scope"`
	// AgentKeySeed is the 32-byte Ed25519 seed of the agent signing key.
	AgentKeySeed []byte `json:"agent_key_seed"`
}

// Valid reports whether the credential has all fields needed to sign per-RPC
// tokens.
func (rc RuntimeCredential) Valid() error {
	switch {
	case rc.HostID == "":
		return errors.New("capabilitygrant: RuntimeCredential: host_id is empty")
	case rc.AgentID == "":
		return errors.New("capabilitygrant: RuntimeCredential: agent_id is empty")
	case rc.ComponentScope == "":
		return errors.New("capabilitygrant: RuntimeCredential: component_scope is empty")
	case len(rc.AgentKeySeed) == 0:
		return errors.New("capabilitygrant: RuntimeCredential: agent_key_seed is empty")
	}
	return nil
}

// DecodeRuntimeCredential parses bytes produced by Encode.
func DecodeRuntimeCredential(data []byte) (RuntimeCredential, error) {
	var rc RuntimeCredential
	if err := json.Unmarshal(data, &rc); err != nil {
		return RuntimeCredential{}, fmt.Errorf("capabilitygrant: decode RuntimeCredential: %w", err)
	}
	return rc, nil
}

// DecodeRuntimeCredentialBase64 parses a base64(JSON) blob — the form carried in
// an env var (e.g. GIBSON_AGENT_KEY) for CI / k8s-Secret-as-env (ADR-0045
// lifecycle A, file + env-override backends). Accepts std or raw base64.
func DecodeRuntimeCredentialBase64(s string) (RuntimeCredential, error) {
	dec, err := base64.StdEncoding.DecodeString(s)
	if err != nil {
		if dec, err = base64.RawStdEncoding.DecodeString(s); err != nil {
			return RuntimeCredential{}, fmt.Errorf("capabilitygrant: decode RuntimeCredential base64: %w", err)
		}
	}
	return DecodeRuntimeCredential(dec)
}

// PerRPCCredentials returns a gRPC credentials.PerRPCCredentials that signs a
// fresh agent+jwt for every call from this stored credential — no Client and no
// re-registration required (ADR-0045 lifecycle A). This is what `gibson inspect`
// and a re-launched serve loop use.
func (rc RuntimeCredential) PerRPCCredentials() (credentials.PerRPCCredentials, error) {
	if err := rc.Valid(); err != nil {
		return nil, err
	}
	key, err := AgentKeyFromSeed(rc.AgentKeySeed)
	if err != nil {
		return nil, err
	}
	return &runtimeCredentials{
		key:            key,
		hostID:         rc.HostID,
		agentID:        rc.AgentID,
		componentScope: rc.ComponentScope,
	}, nil
}

// runtimeCredentials implements credentials.PerRPCCredentials from persisted
// material (no live Client).
type runtimeCredentials struct {
	key            *AgentKey
	hostID         string
	agentID        string
	componentScope string
}

func (r *runtimeCredentials) GetRequestMetadata(ctx context.Context, _ ...string) (map[string]string, error) {
	method, err := requestMethodFromContext(ctx)
	if err != nil {
		return nil, err
	}
	return perRPCMetadata(r.key, r.hostID, r.agentID, r.componentScope, method)
}

// RequireTransportSecurity returns false so the credential works over both TLS
// and plaintext (tests / in-cluster mTLS-terminated hops). Production traffic is
// TLS at the gateway.
func (r *runtimeCredentials) RequireTransportSecurity() bool { return false }
