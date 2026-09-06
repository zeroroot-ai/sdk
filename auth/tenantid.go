// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Zero Root AI

// Package auth provides the SDK-side identity and authorization primitives
// used by every Gibson Go service: the daemon, the tenant-operator, the
// gibson-tool-runner runtime, and any future component that participates in
// the auth chain.
//
// The package is deliberately narrow. Authentication (JWT signature, issuer,
// audience, expiry) is the job of Envoy's jwt_authn filter against the
// configured OIDC provider's JWKS. Authorization (relation × tenant × method)
// is the job of ext-authz against OpenFGA. The SDK auth package owns three things:
//
//  1. The TenantID sealed type — non-empty, validated, type-enforced; the
//     single source of tenant identity in handler code (see tenantid.go).
//  2. The Identity carrier — a request-scoped struct populated by the
//     interceptor from the verified `x-gibson-identity-*` headers
//     ext-authz emits (see identity.go).
//  3. The gRPC interceptor — reads the headers, builds the Identity, places
//     it on the context (see interceptor.go).
//
// The daemon never validates JWTs, talks to the configured OIDC provider,
// talks to OpenFGA, or holds any signing keys. All of that lives upstream.
//
// Spec: unified-identity-and-authorization (entire spec).
package auth

import (
	"errors"
	"fmt"
	"regexp"
	"strings"

	"github.com/zeroroot-ai/sdk/auth/internal/sysconst"
)

// MaxTenantIDLen caps the length of a tenant identifier. The cap is well
// above any human-friendly tenant name (slugs, UUIDs, ULIDs, account
// identifiers from upstream IdPs all fit easily) and below limits for safe
// embedding in Postgres database names, Redis logical-DB index lookups,
// Neo4j database names, and vector-store collection names. The exact value
// is set conservatively at 128 — this is a hard ceiling enforced by
// NewTenantID.
const MaxTenantIDLen = 128

// tenantIDPattern is the validation regular expression for tenant
// identifiers. The grammar is intentionally restrictive:
//
//   - Must start with a lowercase letter (so it is safe as a database
//     name, a Postgres role name, a Redis key namespace, a Neo4j database
//     name, and a vector-store collection name).
//   - May contain lowercase ASCII letters, digits, and a single hyphen or
//     underscore separator.
//   - Must end with a lowercase letter or digit.
//
// This rules out: leading digits (some Postgres tooling rejects), uppercase
// letters (many naming systems lower-case unconditionally), trailing
// separators (creates `tenant_` style ambiguities), and any non-ASCII
// codepoint (avoids Unicode-normalisation footguns when the same identifier
// must round-trip through multiple stores).
//
// Tenant identifiers that come from upstream identity providers (organization
// IDs, customer IdP federated IDs) are sanitised by the tenant-operator
// before being persisted; this validation enforces the invariant at every
// SDK consumer.
var tenantIDPattern = regexp.MustCompile(`^[a-z][a-z0-9]*([-_][a-z0-9]+)*$`)

// TenantID is a sealed, validated, non-empty tenant identifier. Construction
// is the only way to obtain a usable value — the unexported field cannot be
// set by struct literal from outside this package. The zero TenantID{} is
// invalid and reported by IsZero.
//
// Every storage operation in Gibson takes a TenantID as a typed parameter
// (in the data-plane spec, this becomes the connection-pool selector
// `pool.For(tenant)`). Forgetting to pass a tenant, or passing an empty
// string, is a compile error. This is the structural foundation of Gibson's
// tenant isolation model — see `database-per-tenant-data-plane` for the
// downstream half.
//
// TenantID is comparable, immutable, and safe to use as a map key.
type TenantID struct {
	s string
}

// NewTenantID returns a validated TenantID from s. It refuses:
//   - the empty string
//   - strings consisting only of whitespace
//   - strings whose trimmed length exceeds MaxTenantIDLen
//   - strings that do not match tenantIDPattern
//
// On success, the returned TenantID's underlying string is the trimmed
// input. On failure, the zero TenantID and a descriptive error are
// returned. The error wraps ErrInvalidTenant so callers can match on it
// without parsing strings.
//
// NewTenantID is the only path to a non-system TenantID. To obtain the
// reserved system-tenant value, use the SystemTenant package variable —
// this is intentionally segregated so that platform-operator code paths
// are easy to find and audit.
func NewTenantID(s string) (TenantID, error) {
	trimmed := strings.TrimSpace(s)
	if trimmed == "" {
		return TenantID{}, fmt.Errorf("%w: empty or whitespace-only", ErrInvalidTenant)
	}
	if len(trimmed) > MaxTenantIDLen {
		return TenantID{}, fmt.Errorf("%w: length %d exceeds max %d", ErrInvalidTenant, len(trimmed), MaxTenantIDLen)
	}
	if trimmed == sysconst.Reserved {
		// The system tenant is reserved; it can only be obtained through
		// the SystemTenant variable. We refuse to construct it via the
		// public path so that audit-grep for SystemTenant gives the
		// complete list of platform-operator code paths.
		return TenantID{}, fmt.Errorf("%w: %q is reserved; use auth.SystemTenant", ErrInvalidTenant, trimmed)
	}
	if !tenantIDPattern.MatchString(trimmed) {
		return TenantID{}, fmt.Errorf("%w: %q does not match required pattern (lowercase letter start, [a-z0-9_-] body, [a-z0-9] end)", ErrInvalidTenant, trimmed)
	}
	return TenantID{s: trimmed}, nil
}

// MustNewTenantID is the panic-on-error variant of NewTenantID. It is
// intended for tests and for top-level program initialization where a
// hard-coded tenant identifier is known to be valid. Production code that
// receives tenant identifiers from any external source — headers, request
// bodies, configuration files — MUST use NewTenantID and handle the error.
func MustNewTenantID(s string) TenantID {
	t, err := NewTenantID(s)
	if err != nil {
		panic(fmt.Errorf("auth: MustNewTenantID(%q): %w", s, err))
	}
	return t
}

// SystemTenant is the reserved tenant identifier used by platform-operator
// code paths (cross-tenant analytics, billing aggregation, fleet metrics).
// It is the only TenantID whose underlying string equals
// sysconst.Reserved (`_system`). All such code paths live under
// `gibson/internal/admin/` per the data-plane spec; the SystemTenant
// constant is the single audit-grep handle for finding them.
//
// SystemTenant is initialized at package init time and is the only place
// where the sealed constructor is bypassed.
var SystemTenant = TenantID{s: sysconst.Reserved}

// String returns the underlying tenant identifier. The returned string is
// safe to log, embed in error messages, and pass to storage layers as a
// path component (after engine-specific sanitization). It is NOT safe to
// reconstruct a TenantID from the returned string by struct literal — use
// NewTenantID, which re-runs validation.
func (t TenantID) String() string {
	return t.s
}

// IsZero reports whether t is the zero value. A zero TenantID is invalid
// for any storage operation; the data-plane pool refuses it at runtime.
// Use IsZero to distinguish "no tenant in context" from "tenant is the
// system tenant" — the latter is non-zero.
func (t TenantID) IsZero() bool {
	return t.s == ""
}

// IsSystem reports whether t is the reserved SystemTenant. Platform-
// operator code paths use this to gate cross-tenant operations.
func (t TenantID) IsSystem() bool {
	return t.s == sysconst.Reserved
}

// MarshalText implements encoding.TextMarshaler so TenantID interoperates
// with text-based serialization (logs, structured events, JSON tags that
// flow through TextMarshaler). The output is the underlying tenant
// identifier.
//
// The corresponding UnmarshalText is intentionally NOT implemented:
// deserializers that want to reconstruct a TenantID from text must call
// NewTenantID explicitly so that validation runs. This prevents accidental
// "round-trip a malformed tenant through JSON" bugs.
func (t TenantID) MarshalText() ([]byte, error) {
	return []byte(t.s), nil
}

// LogValue implements slog.LogValuer so structured logging records the
// tenant cleanly without leaking the unexported field name.
func (t TenantID) LogValue() interface{} {
	return t.s
}

// Equal reports whether t and other refer to the same tenant. Use this
// instead of `==` if you want to insulate callers from the unexported
// struct shape (the operators happen to work today, but a future
// refactor could add unexported metadata fields that would break it).
func (t TenantID) Equal(other TenantID) bool {
	return t.s == other.s
}

// ErrInvalidTenant is returned by NewTenantID when the input fails
// validation. Callers can match with errors.Is.
var ErrInvalidTenant = errors.New("auth: invalid tenant identifier")
