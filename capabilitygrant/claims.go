// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Zero Root AI

package capabilitygrant

import (
	"errors"
	"fmt"
	"slices"
	"time"

	"github.com/zeroroot-ai/sdk/auth"
)

// Claims is the typed payload of a capability-grant JWT (CG-JWT) minted
// by the Gibson daemon at mission task dispatch and presented by an
// agent on every harness callback during that task. ext-authz verifies
// these JWTs against the daemon's published JWKS and, when the
// requested method appears in AllowedRPCs and the tenant matches, may
// short-circuit the OpenFGA call.
//
// CG-JWTs are EdDSA (Ed25519)–signed by a key held in the configured
// KMS (Vault Transit, AWS KMS, Azure Key Vault, GCP KMS). The daemon
// never holds the private signing material in plaintext outside the
// KMS process boundary; the public counterpart is published on the
// daemon's /.well-known/jwks.json endpoint (served via Envoy) for
// ext-authz to fetch and cache.
//
// Spec: unified-identity-and-authorization Requirement 5.
type Claims struct {
	// Issuer is the JWT iss claim. It is the canonical CG-JWT authority
	// URL — typically the Envoy-exposed daemon URL. ext-authz validates
	// this against its configured expected issuer.
	Issuer string

	// Audience is the JWT aud claim. It is the daemon identifier — the
	// CG-JWT is intended only for callbacks consumed by this daemon and
	// MUST be rejected by any other consumer.
	Audience string

	// Subject is the JWT sub claim — the agent's Zitadel service-
	// account identifier. Should match the primary identity's Subject
	// presented in the same request.
	Subject string

	// Tenant is the validated tenant scope. ext-authz checks this
	// against the tenant derived from the primary identity (the
	// `x-gibson-identity-tenant` header it would otherwise emit) — a
	// CG-JWT for a different tenant than the calling identity is
	// rejected.
	Tenant auth.TenantID

	// MissionID names the mission whose dispatch produced this CG-JWT.
	// Required and non-empty.
	MissionID string

	// TaskID names the specific mission task. Required and non-empty —
	// each task gets its own CG-JWT so revocation, audit, and replay
	// detection are per-task.
	TaskID string

	// AllowedRPCs is the set of fully-qualified gRPC method names the
	// holder of this CG-JWT may invoke without further FGA evaluation.
	// Method names use the slash-prefixed format
	// `/<package>.<Service>/<Method>` (matching grpc.UnaryServerInfo's
	// FullMethod). ext-authz performs an O(N) substring match; the
	// expected size of AllowedRPCs is small (the agent's
	// component.yaml capability set, typically <20 entries).
	AllowedRPCs []string

	// IssuedAt is the JWT iat claim, second precision UTC.
	IssuedAt time.Time

	// ExpiresAt is the JWT exp claim, second precision UTC. Per
	// Requirement 5.2, ExpiresAt MUST NOT exceed IssuedAt + 30 minutes.
	ExpiresAt time.Time

	// JTI is the JWT jti claim — a unique identifier for this CG-JWT.
	// Used by ext-authz's optional replay-detection cache and by audit
	// logs. Required and non-empty (the daemon's mint code generates a
	// random UUIDv4).
	JTI string
}

// AllowsMethod reports whether method is present in AllowedRPCs. The
// comparison is exact: method strings use the
// `/<package>.<Service>/<Method>` form supplied by gRPC's
// UnaryServerInfo.FullMethod and StreamServerInfo.FullMethod.
func (c Claims) AllowsMethod(method string) bool {
	return slices.Contains(c.AllowedRPCs, method)
}

// IsExpired reports whether the CG-JWT is past its exp at the supplied
// reference time. Pass time.Now() for "now" semantics; allow tests to
// inject a fixed clock.
func (c Claims) IsExpired(now time.Time) bool {
	return now.After(c.ExpiresAt)
}

// MaxLifetime is the upper bound enforced by the daemon's mint code
// per Requirement 5.2. Verifiers also enforce it: a CG-JWT whose
// (exp - iat) exceeds MaxLifetime is rejected as malformed.
const MaxLifetime = 30 * time.Minute

// Validate performs structural validation of the claims independent
// of signature checks. Verify (in verify.go) calls this after JWS
// signature verification; tests can call it directly with synthesized
// claims.
//
// Returns ErrClaimsInvalid wrapping a descriptive cause on failure.
func (c Claims) Validate(now time.Time, expectedIssuer, expectedAudience string) error {
	if c.Issuer == "" {
		return fmt.Errorf("%w: missing iss", ErrClaimsInvalid)
	}
	if expectedIssuer != "" && c.Issuer != expectedIssuer {
		return fmt.Errorf("%w: iss %q does not match expected %q", ErrClaimsInvalid, c.Issuer, expectedIssuer)
	}
	if c.Audience == "" {
		return fmt.Errorf("%w: missing aud", ErrClaimsInvalid)
	}
	if expectedAudience != "" && c.Audience != expectedAudience {
		return fmt.Errorf("%w: aud %q does not match expected %q", ErrClaimsInvalid, c.Audience, expectedAudience)
	}
	if c.Subject == "" {
		return fmt.Errorf("%w: missing sub", ErrClaimsInvalid)
	}
	if c.Tenant.IsZero() {
		return fmt.Errorf("%w: missing tenant", ErrClaimsInvalid)
	}
	if c.MissionID == "" {
		return fmt.Errorf("%w: missing mission_id", ErrClaimsInvalid)
	}
	if c.TaskID == "" {
		return fmt.Errorf("%w: missing task_id", ErrClaimsInvalid)
	}
	if c.JTI == "" {
		return fmt.Errorf("%w: missing jti", ErrClaimsInvalid)
	}
	if c.IssuedAt.IsZero() {
		return fmt.Errorf("%w: missing iat", ErrClaimsInvalid)
	}
	if c.ExpiresAt.IsZero() {
		return fmt.Errorf("%w: missing exp", ErrClaimsInvalid)
	}
	if c.ExpiresAt.Before(c.IssuedAt) {
		return fmt.Errorf("%w: exp before iat", ErrClaimsInvalid)
	}
	if c.ExpiresAt.Sub(c.IssuedAt) > MaxLifetime {
		return fmt.Errorf("%w: lifetime %s exceeds MaxLifetime %s", ErrClaimsInvalid, c.ExpiresAt.Sub(c.IssuedAt), MaxLifetime)
	}
	if c.IsExpired(now) {
		return fmt.Errorf("%w: expired at %s (now %s)", ErrExpired, c.ExpiresAt, now)
	}
	return nil
}

// ErrClaimsInvalid is returned when a CG-JWT's claims fail structural
// validation (missing required field, exp before iat, lifetime exceeds
// MaxLifetime, etc.). Wrapped with errors.Is.
var ErrClaimsInvalid = errors.New("capabilitygrant: claims invalid")

// ErrExpired is returned when a CG-JWT is past its exp. Wrapped with
// errors.Is. Distinguished from ErrClaimsInvalid so callers can
// trigger a renewal RPC instead of treating it as a hard failure.
var ErrExpired = errors.New("capabilitygrant: expired")
