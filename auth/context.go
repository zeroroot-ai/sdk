// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Zero Root AI

package auth

import (
	"context"
	"errors"
)

// contextKey is the unexported context-key type used by this package.
// Using a local type prevents accidental collisions with keys defined
// in other packages — even if another package happens to use the
// integer constant 0 as its key, the type system keeps them separate.
type contextKey int

const (
	identityKey contextKey = iota
)

// ErrIdentityNotInContext is returned by IdentityFromContext when no
// identity has been placed on the context. It is wrapped with errors.Is
// support so callers can match on it without comparing strings.
var ErrIdentityNotInContext = errors.New("auth: identity not in context")

// WithIdentity returns a derived context that carries id. The interceptor
// (interceptor.go) calls this at the top of each request after
// constructing the Identity from headers; handlers retrieve it via
// IdentityFromContext.
//
// WithIdentity is also the supported escape hatch for tests and for
// in-process dispatch: when a code path needs to invoke a handler with
// a synthetic identity (a unit test, a mission-orchestrator dispatch
// to an in-process subsystem, a CLI's local-development mode), it
// constructs an Identity and calls WithIdentity. There is intentionally
// no production-code path that bypasses the interceptor — see the
// design's "Trust Boundaries" section.
func WithIdentity(ctx context.Context, id Identity) context.Context {
	if ctx == nil {
		ctx = context.Background()
	}
	return context.WithValue(ctx, identityKey, id)
}

// IdentityFromContext retrieves the Identity placed by WithIdentity. It
// returns ErrIdentityNotInContext when the context carries no identity
// — a state that should never occur in handler code (the interceptor
// rejects identity-less requests before the handler runs) but is
// possible in tests or in middleware that runs before the interceptor.
//
// Callers that just want the tenant should prefer TenantFromContext for
// brevity.
func IdentityFromContext(ctx context.Context) (Identity, error) {
	if ctx == nil {
		return Identity{}, ErrIdentityNotInContext
	}
	v := ctx.Value(identityKey)
	if v == nil {
		return Identity{}, ErrIdentityNotInContext
	}
	id, ok := v.(Identity)
	if !ok {
		// Should be impossible: only WithIdentity writes this key, and
		// it writes Identity. A type mismatch indicates someone reused
		// our context key (a bug). Return the missing-identity error so
		// callers fail-closed rather than crashing.
		return Identity{}, ErrIdentityNotInContext
	}
	return id, nil
}

// TenantFromContext returns the tenant from the context's Identity.
// The bool reports presence: true if an Identity was found, false if
// none was placed on the context. The caller MUST treat false as
// "fail-closed" — there is no defaulting to SystemTenant or any other
// fallback. This is the closure of audit findings C11/C12: there is no
// path by which an identity-less request reaches a handler and gets a
// non-zero tenant.
//
// In handler code, the canonical pattern is:
//
//	tenant, ok := auth.TenantFromContext(ctx)
//	if !ok {
//	    return nil, status.Error(codes.PermissionDenied, "no tenant in context")
//	}
//	conn, err := s.pool.For(ctx, tenant)
//	...
//
// A handler that reaches `pool.For` with the zero TenantID will receive
// an error from the pool; this layered defense means a missing-tenant
// bug is caught at two points.
func TenantFromContext(ctx context.Context) (TenantID, bool) {
	id, err := IdentityFromContext(ctx)
	if err != nil {
		return TenantID{}, false
	}
	if id.Tenant.IsZero() {
		return TenantID{}, false
	}
	return id.Tenant, true
}

// WithTenant returns ctx scoped to tenant t. When ctx already carries an
// Identity, that identity is kept (subject, issuer, credential type, scopes)
// and only its Tenant is replaced: code downstream that must know WHO asked,
// such as a per-dispatch authorization check, still sees the caller after a
// tenant scope change. Before this, WithTenant installed a bare Identity
// carrying only the tenant, and every ContextWithTenant/ContextWithTenantString
// caller silently dropped the subject; a mission context built that way was
// then refused at dispatch with "no caller identity". With no identity in ctx
// it installs a minimal Identity carrying only the tenant, which is what unit
// tests of TenantFromContext callers rely on.
func WithTenant(ctx context.Context, t TenantID) context.Context {
	if id, err := IdentityFromContext(ctx); err == nil {
		id.Tenant = t
		return WithIdentity(ctx, id)
	}
	return WithIdentity(ctx, Identity{Tenant: t})
}
