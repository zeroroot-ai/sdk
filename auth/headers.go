// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Zero Root AI

package auth

import (
	"errors"
	"fmt"
	"os"
	"strconv"
	"time"

	"google.golang.org/grpc/metadata"
)

// Header names emitted by ext-authz on every authenticated request and
// consumed by the SDK auth interceptor. The names are part of the wire
// contract and MUST match those produced by ext-authz exactly. They are
// intentionally lowercase to match gRPC metadata canonicalisation.
//
// These headers are NOT HMAC-signed. Channel security is provided by
// SPIFFE mTLS between Envoy and the daemon (see Component A in
// design.md). The daemon's TLS listener is configured to accept only
// Envoy's SPIFFE peer SVID — any other connecting party is rejected at
// the TLS handshake before headers are read.
//
// Spec: unified-identity-and-authorization Requirement 4.5.
const (
	HeaderSubject        = "x-gibson-identity-subject"
	HeaderIssuer         = "x-gibson-identity-issuer"
	HeaderCredentialType = "x-gibson-identity-credential-type"
	HeaderTenant         = "x-gibson-identity-tenant"
	HeaderIssuedAt       = "x-gibson-identity-issued-at"
)

// ErrMissingIdentity is returned by IdentityFromMetadata when the
// required identity headers are absent. The interceptor maps this to
// codes.PermissionDenied — a request reaching the daemon without
// identity headers is structurally impossible (only Envoy can connect),
// so the right response is rejection, not internal-error.
var ErrMissingIdentity = errors.New("auth: identity headers absent")

// ErrInvalidIdentity is returned by IdentityFromMetadata when one of
// the identity headers fails validation (malformed timestamp, unknown
// issuer, invalid tenant, or stale/future-dated issued-at). The
// interceptor maps this to codes.PermissionDenied with the underlying
// message.
var ErrInvalidIdentity = errors.New("auth: identity header invalid")

// defaultFreshnessSkewSeconds is the maximum allowed deviation (in
// seconds) between the x-gibson-identity-issued-at header value and the
// daemon's time.Now() at the moment the request is processed.
//
// Configurable at process startup via GIBSON_IDENTITY_FRESHNESS_SKEW_SEC.
// Hardened deployments should NOT raise this above 300 seconds.
//
// Spec: admin-services-completion Requirement 6.2.
const defaultFreshnessSkewSeconds = 60

// identityFreshnessSkew is the process-level skew threshold, initialized
// once from the environment. It is package-level so tests can substitute
// a controlled value via identityFreshnessSkewForTest.
var identityFreshnessSkew = func() time.Duration {
	const envKey = "GIBSON_IDENTITY_FRESHNESS_SKEW_SEC"
	raw := os.Getenv(envKey)
	if raw == "" {
		return defaultFreshnessSkewSeconds * time.Second
	}
	n, err := strconv.ParseInt(raw, 10, 64)
	if err != nil || n <= 0 {
		return defaultFreshnessSkewSeconds * time.Second
	}
	return time.Duration(n) * time.Second
}()

// identityFreshnessSkewForTest overrides identityFreshnessSkew in tests.
// Call it from a test helper; restore the original value in t.Cleanup.
// Exported only for the auth package's own tests.
var identityFreshnessSkewForTest *time.Duration

// IdentityFromMetadata reads the `x-gibson-identity-*` headers from
// gRPC metadata and constructs a validated Identity.
//
// Required headers (any missing → ErrMissingIdentity):
//   - x-gibson-identity-subject
//   - x-gibson-identity-issuer
//   - x-gibson-identity-credential-type
//   - x-gibson-identity-tenant
//   - x-gibson-identity-issued-at
//
// Validation:
//   - tenant is constructed via NewTenantID; if validation fails, the
//     wrapped ErrInvalidTenant is returned (which itself wraps to
//     ErrInvalidIdentity for callers that prefer the higher-level
//     match).
//   - issued-at must parse as a Unix-second integer; the resulting
//     time is in UTC.
//   - issuer must be one of the known Issuer constants; an unknown
//     issuer indicates a misconfigured ext-authz emit and yields
//     ErrInvalidIdentity.
//   - credential-type is accepted as-is (forwards-compatible); the
//     known constants are documented in identity.go.
//
// IdentityFromMetadata performs NO HMAC verification. See the
// HeaderSubject doc comment for the channel-security rationale.
//
// Spec: unified-identity-and-authorization Requirement 4.5, 8.4, 8.5.
func IdentityFromMetadata(md metadata.MD) (Identity, error) {
	if md == nil {
		return Identity{}, ErrMissingIdentity
	}

	subject := first(md, HeaderSubject)
	issuerStr := first(md, HeaderIssuer)
	credTypeStr := first(md, HeaderCredentialType)
	tenantStr := first(md, HeaderTenant)
	issuedAtStr := first(md, HeaderIssuedAt)

	if subject == "" || issuerStr == "" || credTypeStr == "" || tenantStr == "" || issuedAtStr == "" {
		// Be specific in the error so the operator can diagnose a
		// misconfigured ext-authz emit. We do not log header values
		// (they're audit material, but logging them on a bad-request
		// path inflates audit volume disproportionately).
		missing := []string{}
		if subject == "" {
			missing = append(missing, HeaderSubject)
		}
		if issuerStr == "" {
			missing = append(missing, HeaderIssuer)
		}
		if credTypeStr == "" {
			missing = append(missing, HeaderCredentialType)
		}
		if tenantStr == "" {
			missing = append(missing, HeaderTenant)
		}
		if issuedAtStr == "" {
			missing = append(missing, HeaderIssuedAt)
		}
		return Identity{}, fmt.Errorf("%w: missing %v", ErrMissingIdentity, missing)
	}

	issuer := Issuer(issuerStr)
	switch issuer {
	case IssuerOIDC, IssuerCapabilityGrant:
		// known
	default:
		return Identity{}, fmt.Errorf("%w: unknown issuer %q", ErrInvalidIdentity, issuerStr)
	}

	tenant, err := NewTenantID(tenantStr)
	if err != nil {
		// Caller can match either ErrInvalidIdentity (loose) or
		// ErrInvalidTenant (strict, via errors.Is). Multi-wrap requires
		// fmt.Errorf with two %w verbs (Go 1.20+).
		return Identity{}, fmt.Errorf("%w: %s: %w", ErrInvalidIdentity, HeaderTenant, err)
	}

	issuedAtUnix, err := strconv.ParseInt(issuedAtStr, 10, 64)
	if err != nil {
		return Identity{}, fmt.Errorf("%w: %s not unix seconds: %w", ErrInvalidIdentity, HeaderIssuedAt, err)
	}

	// Freshness check: reject requests whose issued-at header deviates
	// from now by more than the configured skew window. This bounds the
	// replay value of captured identity headers.
	//
	// Spec: admin-services-completion Requirement 6.2.
	skew := identityFreshnessSkew
	if identityFreshnessSkewForTest != nil {
		skew = *identityFreshnessSkewForTest
	}
	issuedAt := time.Unix(issuedAtUnix, 0).UTC()
	now := time.Now().UTC()
	// Use absolute difference: past-dated (now > issuedAt) gives positive delta;
	// future-dated (issuedAt > now) gives negative delta. Both exceed skew
	// if their magnitude is large.
	delta := now.Sub(issuedAt)
	if delta < 0 {
		delta = -delta
	}
	if delta > skew {
		rawDelta := now.Sub(issuedAt)
		return Identity{}, fmt.Errorf(
			"%w: %s is outside freshness window (delta=%v, max=%v)",
			ErrInvalidIdentity, HeaderIssuedAt, rawDelta.Round(time.Second), skew,
		)
	}

	return Identity{
		Subject:        subject,
		Issuer:         issuer,
		CredentialType: CredentialType(credTypeStr),
		Tenant:         tenant,
		IssuedAt:       issuedAt,
	}, nil
}

// IdentityToMetadata writes the identity into a metadata.MD for use by
// callers that synthesize identity for testing or for in-process
// dispatch (e.g. the daemon's own internal calls that need to assert
// identity for subsystems that read it from context). Production code
// paths NEVER call this — identity headers come from ext-authz, not
// from the daemon.
func IdentityToMetadata(md metadata.MD, id Identity) {
	md.Set(HeaderSubject, id.Subject)
	md.Set(HeaderIssuer, string(id.Issuer))
	md.Set(HeaderCredentialType, string(id.CredentialType))
	md.Set(HeaderTenant, id.Tenant.String())
	md.Set(HeaderIssuedAt, strconv.FormatInt(id.IssuedAt.Unix(), 10))
}

// first returns the first value of the named metadata key, or "".
// gRPC metadata is a map[string][]string; identity headers are
// single-valued, so the first value is canonical.
func first(md metadata.MD, key string) string {
	vs := md.Get(key)
	if len(vs) == 0 {
		return ""
	}
	return vs[0]
}
