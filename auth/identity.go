// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Zero Root AI

package auth

import "time"

// Issuer enumerates the identity issuers Gibson recognizes. The list is
// closed: a new issuer requires a code change in the SDK plus a matching
// configuration in Envoy's jwt_authn filter and ext-authz registry.
//
// Spec: unified-identity-and-authorization Requirement 1,
// agent-service-credentials Requirement 6.
type Issuer string

const (
	// IssuerOIDC identifies tokens minted by the configured OIDC identity
	// provider. Both human users (OIDC authorization-code flow) and machine
	// subjects (OAuth2 client_credentials grant) carry the "oidc" issuer,
	// regardless of which OIDC provider is deployed. The concrete provider
	// is an implementation detail isolated to the daemon's internal/idp
	// package; the SDK and ext-authz surface only this vendor-neutral value.
	IssuerOIDC Issuer = "oidc"

	// IssuerCapabilityGrant identifies the ephemeral capability-grant
	// JWTs minted by the Gibson daemon at mission task dispatch and
	// presented by agents on callbacks. ext-authz verifies these against
	// the daemon's published JWKS and may short-circuit FGA when the
	// CG-JWT covers the requested method.
	IssuerCapabilityGrant Issuer = "capability-grant"
)

// CredentialType describes the wire credential class an Identity was
// derived from. Useful for audit logging and per-class policy decisions
// that go beyond the IdentityClass bitfield.
type CredentialType string

const (
	// CredentialOIDCUser is a user JWT obtained from the configured OIDC
	// provider via the authorization-code flow (humans through the dashboard).
	CredentialOIDCUser CredentialType = "oidc-user"

	// CredentialClientCredentials is a JWT obtained from the configured OIDC
	// provider via the OAuth2 client_credentials grant (services, agents,
	// tool runners, the dashboard pod's own admin RPCs).
	CredentialClientCredentials CredentialType = "client-credentials"

	// CredentialCapabilityGrant is a per-task capability-grant JWT minted
	// by the daemon and signed via the configured KMS. Always presented
	// as a second token alongside an OIDC or client-credentials token —
	// never standalone.
	CredentialCapabilityGrant CredentialType = "capability-grant"
)

// Identity is the request-scoped record of who is calling. It is built by
// the auth interceptor (interceptor.go) from the verified
// `x-gibson-identity-*` headers ext-authz emits, and placed on the
// request context via WithIdentity. Handlers consume it via
// IdentityFromContext or — for the common case of "what tenant is this
// for" — via TenantFromContext.
//
// Identity is immutable after construction and safe to share across
// goroutines. The Subject + Tenant + IssuedAt triple is sufficient for
// audit logging; ext-authz handles all authorization decisions before the
// handler runs.
//
// Per Requirement 8: the daemon does NO JWT validation. The Identity
// fields here are the post-validation projection. They can be trusted
// because the channel between Envoy and the daemon is SPIFFE-pinned mTLS
// (see Component E in design.md) — only Envoy can deliver these headers.
type Identity struct {
	// Subject is the stable identifier of the calling principal. For
	// IssuerOIDC, this is the user ID (humans) or service account ID
	// (services, agents) as issued by the configured OIDC provider. It is
	// suitable for audit logs and for use as a key in caches keyed by subject.
	Subject string

	// Issuer is the identity provider. Always IssuerOIDC for primary
	// authentication; IssuerCapabilityGrant only appears in the
	// capability-grant claims, never in the primary Identity.
	Issuer Issuer

	// CredentialType describes the wire credential class. See
	// CredentialOIDCUser, CredentialClientCredentials.
	CredentialType CredentialType

	// Tenant is the validated tenant scope of the request. Always
	// non-zero in a properly-built Identity — the interceptor refuses
	// requests whose `x-gibson-identity-tenant` header is absent or
	// fails NewTenantID validation. The SystemTenant value appears only
	// for platform-operator subjects.
	Tenant TenantID

	// IssuedAt is the Unix-second-precision timestamp at which the
	// underlying JWT was issued, as emitted by the upstream IdP. Useful
	// for token-age telemetry; not used for authorization (ext-authz
	// validates exp upstream).
	IssuedAt time.Time
}

// IsZero reports whether i is the zero Identity (no fields set).
// Handler code should never see a zero Identity from
// IdentityFromContext — the interceptor either populates a non-zero
// value or rejects the request before the handler runs. IsZero exists
// for defensive checks and for tests that exercise context propagation
// without going through the interceptor.
func (i Identity) IsZero() bool {
	return i.Subject == "" && i.Issuer == "" && i.Tenant.IsZero() && i.IssuedAt.IsZero()
}

// LogValue implements slog.LogValuer so structured log records embed an
// Identity as a small object rather than the default reflection-based
// rendering. The output excludes any field that could be considered
// sensitive (none today, but future fields like token JTI would be
// excluded here).
func (i Identity) LogValue() interface{} {
	return struct {
		Subject        string
		Issuer         Issuer
		CredentialType CredentialType
		Tenant         TenantID
		IssuedAt       time.Time
	}{i.Subject, i.Issuer, i.CredentialType, i.Tenant, i.IssuedAt}
}
