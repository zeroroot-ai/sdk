// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Zero Root AI

package auth

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestWithIdentity_RoundTrip(t *testing.T) {
	id := Identity{
		Subject:        "u-1",
		Issuer:         IssuerOIDC,
		CredentialType: CredentialOIDCUser,
		Tenant:         MustNewTenantID("acme"),
		IssuedAt:       time.Unix(1700000000, 0).UTC(),
	}
	ctx := WithIdentity(context.Background(), id)
	got, err := IdentityFromContext(ctx)
	if err != nil {
		t.Fatalf("expected ok, got %v", err)
	}
	if got != id {
		t.Fatalf("round-trip mismatch")
	}
}

func TestIdentityFromContext_Empty(t *testing.T) {
	_, err := IdentityFromContext(context.Background())
	if !errors.Is(err, ErrIdentityNotInContext) {
		t.Fatalf("expected ErrIdentityNotInContext, got %v", err)
	}
}

func TestIdentityFromContext_Nil(t *testing.T) {
	//nolint:staticcheck // intentionally passing nil to verify defensive behavior
	_, err := IdentityFromContext(nil)
	if !errors.Is(err, ErrIdentityNotInContext) {
		t.Fatalf("expected ErrIdentityNotInContext on nil ctx, got %v", err)
	}
}

func TestTenantFromContext_Present(t *testing.T) {
	id := Identity{Tenant: MustNewTenantID("acme")}
	ctx := WithIdentity(context.Background(), id)
	tid, ok := TenantFromContext(ctx)
	if !ok {
		t.Fatal("expected tenant present")
	}
	if tid.String() != "acme" {
		t.Errorf("got %q", tid.String())
	}
}

func TestTenantFromContext_Absent(t *testing.T) {
	_, ok := TenantFromContext(context.Background())
	if ok {
		t.Fatal("expected absence")
	}
}

func TestTenantFromContext_ZeroTenantOnIdentity(t *testing.T) {
	// An Identity with a zero tenant must NOT be reported as a present
	// tenant — this is Requirement 8.6 closure. There is no fallback to
	// SystemTenant or empty-string-treated-as-tenant.
	id := Identity{Subject: "u-1", Issuer: IssuerOIDC}
	ctx := WithIdentity(context.Background(), id)
	_, ok := TenantFromContext(ctx)
	if ok {
		t.Fatal("zero tenant must be reported absent")
	}
}

func TestWithTenant_TestHelper(t *testing.T) {
	ctx := WithTenant(context.Background(), MustNewTenantID("test"))
	tid, ok := TenantFromContext(ctx)
	if !ok || tid.String() != "test" {
		t.Fatalf("WithTenant helper failed: ok=%v tid=%q", ok, tid.String())
	}
}

// TestWithTenant_KeepsExistingIdentity is the regression test for the
// dropped subject: scoping a context that already carries a caller identity
// to a tenant must keep that caller and change only the tenant.
func TestWithTenant_KeepsExistingIdentity(t *testing.T) {
	caller := Identity{
		Subject:        "user-42",
		Issuer:         Issuer("https://idp.example"),
		CredentialType: CredentialType("jwt"),
		Tenant:         MustNewTenantID("acme"),
	}
	ctx := WithTenant(WithIdentity(context.Background(), caller), MustNewTenantID("globex"))
	got, err := IdentityFromContext(ctx)
	if err != nil {
		t.Fatalf("IdentityFromContext: %v", err)
	}
	if got.Subject != caller.Subject || got.Issuer != caller.Issuer || got.CredentialType != caller.CredentialType {
		t.Fatalf("caller identity was not kept: got %+v", got)
	}
	if got.Tenant.String() != "globex" {
		t.Fatalf("tenant = %q, want globex", got.Tenant.String())
	}
	if tid, ok := TenantFromContext(ctx); !ok || tid.String() != "globex" {
		t.Fatalf("TenantFromContext = %q ok=%v, want globex", tid.String(), ok)
	}
}

func TestWithIdentity_NilContext(t *testing.T) {
	id := Identity{Tenant: MustNewTenantID("acme")}
	//nolint:staticcheck // intentionally passing nil to verify it returns a usable ctx
	ctx := WithIdentity(nil, id)
	if ctx == nil {
		t.Fatal("WithIdentity(nil, ...) should return a usable context")
	}
	got, err := IdentityFromContext(ctx)
	if err != nil {
		t.Fatalf("expected ok after WithIdentity(nil), got %v", err)
	}
	if got != id {
		t.Fatal("identity round-trip mismatch")
	}
}
