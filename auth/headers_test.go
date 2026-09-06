// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Zero Root AI

package auth

import (
	"errors"
	"strconv"
	"testing"
	"time"

	"google.golang.org/grpc/metadata"
)

// withFreshnessSkew temporarily overrides the freshness skew for a test.
func withFreshnessSkew(t *testing.T, d time.Duration) {
	t.Helper()
	prev := identityFreshnessSkewForTest
	identityFreshnessSkewForTest = &d
	t.Cleanup(func() { identityFreshnessSkewForTest = prev })
}

func validHeaders() metadata.MD {
	return metadata.New(map[string]string{
		HeaderSubject:        "user-42",
		HeaderIssuer:         string(IssuerOIDC),
		HeaderCredentialType: string(CredentialOIDCUser),
		HeaderTenant:         "acme",
		HeaderIssuedAt:       strconv.FormatInt(time.Now().Unix(), 10),
	})
}

func TestIdentityFromMetadata_HappyPath(t *testing.T) {
	md := validHeaders()
	id, err := IdentityFromMetadata(md)
	if err != nil {
		t.Fatalf("expected ok, got %v", err)
	}
	if id.Subject != "user-42" {
		t.Errorf("Subject: %q", id.Subject)
	}
	if id.Issuer != IssuerOIDC {
		t.Errorf("Issuer: %q", id.Issuer)
	}
	if id.CredentialType != CredentialOIDCUser {
		t.Errorf("CredentialType: %q", id.CredentialType)
	}
	if id.Tenant.String() != "acme" {
		t.Errorf("Tenant: %q", id.Tenant.String())
	}
	if id.IssuedAt.IsZero() {
		t.Error("IssuedAt zero")
	}
}

func TestIdentityFromMetadata_NilMetadata(t *testing.T) {
	_, err := IdentityFromMetadata(nil)
	if !errors.Is(err, ErrMissingIdentity) {
		t.Fatalf("expected ErrMissingIdentity, got %v", err)
	}
}

func TestIdentityFromMetadata_MissingHeaders(t *testing.T) {
	for _, hdr := range []string{HeaderSubject, HeaderIssuer, HeaderCredentialType, HeaderTenant, HeaderIssuedAt} {
		t.Run("missing_"+hdr, func(t *testing.T) {
			md := validHeaders()
			md.Delete(hdr)
			_, err := IdentityFromMetadata(md)
			if !errors.Is(err, ErrMissingIdentity) {
				t.Fatalf("expected ErrMissingIdentity, got %v", err)
			}
		})
	}
}

func TestIdentityFromMetadata_UnknownIssuer(t *testing.T) {
	md := validHeaders()
	md.Set(HeaderIssuer, "homemade-jwt")
	_, err := IdentityFromMetadata(md)
	if !errors.Is(err, ErrInvalidIdentity) {
		t.Fatalf("expected ErrInvalidIdentity, got %v", err)
	}
}

func TestIdentityFromMetadata_InvalidTenant(t *testing.T) {
	md := validHeaders()
	md.Set(HeaderTenant, "")
	_, err := IdentityFromMetadata(md)
	if !errors.Is(err, ErrMissingIdentity) {
		t.Fatalf("expected ErrMissingIdentity for empty tenant header (treated as missing), got %v", err)
	}

	md.Set(HeaderTenant, "INVALID-CASE")
	_, err = IdentityFromMetadata(md)
	if !errors.Is(err, ErrInvalidIdentity) {
		t.Fatalf("expected ErrInvalidIdentity for malformed tenant, got %v", err)
	}
	if !errors.Is(err, ErrInvalidTenant) {
		t.Fatalf("expected wrapped ErrInvalidTenant, got %v", err)
	}
}

func TestIdentityFromMetadata_BadIssuedAt(t *testing.T) {
	md := validHeaders()
	md.Set(HeaderIssuedAt, "not-a-number")
	_, err := IdentityFromMetadata(md)
	if !errors.Is(err, ErrInvalidIdentity) {
		t.Fatalf("expected ErrInvalidIdentity, got %v", err)
	}
}

func TestIdentityToMetadata_RoundTrip(t *testing.T) {
	// Use time.Now() truncated to second precision so the round-trip
	// passes the freshness window check.
	now := time.Now().UTC().Truncate(time.Second)
	id := Identity{
		Subject:        "svc-acct-1",
		Issuer:         IssuerOIDC,
		CredentialType: CredentialClientCredentials,
		Tenant:         MustNewTenantID("bigcorp"),
		IssuedAt:       now,
	}
	md := metadata.MD{}
	IdentityToMetadata(md, id)
	got, err := IdentityFromMetadata(md)
	if err != nil {
		t.Fatalf("expected ok on round-trip, got %v", err)
	}
	if got != id {
		t.Fatalf("round-trip mismatch:\n  got %+v\n  want %+v", got, id)
	}
}

func TestIdentityFromMetadata_NoHMAC(t *testing.T) {
	// Documents that the parser does NOT require any HMAC/signature
	// header. If a future commit adds HMAC enforcement (regressing
	// Requirement 8.4), this test fails to flag it.
	md := validHeaders()
	// Inject a bogus signature header to make sure the parser ignores it.
	md.Set("x-gibson-identity-signature", "deadbeef")
	if _, err := IdentityFromMetadata(md); err != nil {
		t.Fatalf("parser should ignore extra headers, got %v", err)
	}
}

// TestIdentityFromMetadata_Freshness covers Spec admin-services-completion
// Requirement 6.2: the parser rejects headers whose issued-at value falls
// outside the ±skew window around now.
func TestIdentityFromMetadata_Freshness(t *testing.T) {
	// Use a very small skew so we can easily craft timestamps outside the
	// window without sleeping.
	withFreshnessSkew(t, 5*time.Second)

	t.Run("in_window_accept", func(t *testing.T) {
		md := validHeaders() // validHeaders uses time.Now() — within any sensible window
		if _, err := IdentityFromMetadata(md); err != nil {
			t.Fatalf("expected accept for fresh timestamp, got %v", err)
		}
	})

	t.Run("past_skew_reject", func(t *testing.T) {
		md := validHeaders()
		// Issued 120 seconds ago — outside a 5s window.
		past := time.Now().Add(-120 * time.Second).Unix()
		md.Set(HeaderIssuedAt, strconv.FormatInt(past, 10))
		_, err := IdentityFromMetadata(md)
		if !errors.Is(err, ErrInvalidIdentity) {
			t.Fatalf("expected ErrInvalidIdentity for past-skew timestamp, got %v", err)
		}
	})

	t.Run("future_skew_reject", func(t *testing.T) {
		md := validHeaders()
		// Issued 120 seconds in the future — outside a 5s window.
		future := time.Now().Add(120 * time.Second).Unix()
		md.Set(HeaderIssuedAt, strconv.FormatInt(future, 10))
		_, err := IdentityFromMetadata(md)
		if !errors.Is(err, ErrInvalidIdentity) {
			t.Fatalf("expected ErrInvalidIdentity for future-skew timestamp, got %v", err)
		}
	})

	t.Run("missing_header", func(t *testing.T) {
		md := validHeaders()
		md.Delete(HeaderIssuedAt)
		_, err := IdentityFromMetadata(md)
		if !errors.Is(err, ErrMissingIdentity) {
			t.Fatalf("expected ErrMissingIdentity for absent issued-at header, got %v", err)
		}
	})

	t.Run("malformed_header", func(t *testing.T) {
		md := validHeaders()
		md.Set(HeaderIssuedAt, "not-a-timestamp")
		_, err := IdentityFromMetadata(md)
		if !errors.Is(err, ErrInvalidIdentity) {
			t.Fatalf("expected ErrInvalidIdentity for malformed issued-at header, got %v", err)
		}
	})
}

// TestIdentityFromMetadata_OIDCIssuerAccepted verifies that "oidc" is accepted
// and the old "zitadel" wire value is rejected. This is a regression guard for
// Spec agent-service-credentials Requirement 6.1.
func TestIdentityFromMetadata_OIDCIssuerAccepted(t *testing.T) {
	md := validHeaders()
	md.Set(HeaderIssuer, string(IssuerOIDC))
	if _, err := IdentityFromMetadata(md); err != nil {
		t.Fatalf("expected IssuerOIDC accepted, got %v", err)
	}
}

// TestIdentityFromMetadata_ZitadelIssuerRejected is the corresponding
// rejection check — the old "zitadel" wire value MUST now be invalid.
func TestIdentityFromMetadata_ZitadelIssuerRejected(t *testing.T) {
	md := validHeaders()
	md.Set(HeaderIssuer, "zitadel")
	_, err := IdentityFromMetadata(md)
	if !errors.Is(err, ErrInvalidIdentity) {
		t.Fatalf("expected ErrInvalidIdentity for old zitadel issuer value, got %v", err)
	}
}
