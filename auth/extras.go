// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Zero Root AI

package auth

// This file extends the auth package with helpers needed by the
// Gibson daemon to express cross-cutting auth concepts that go beyond
// the bare Identity carrier:
//
//   - ActingUser:    the end user on whose behalf a platform service
//                    is acting (dashboard pod making a tenant-scoped
//                    call on behalf of a logged-in user).
//   - InitiatorUser: the user who triggered a mission. Stable across
//                    the entire mission lifetime, including sub-agent
//                    delegation, scheduled runs, and checkpoint
//                    resume — span attribution follows the spend.
//   - ExecutorUser:  the owner of the agent currently executing a
//                    mission step. May differ from InitiatorUser when
//                    a parent agent delegates to a sub-agent owned by
//                    a different user.
//   - ComponentScope: a "component:<id>" string identifying the
//                    capability-grant subject for a request — used by
//                    the FGA batch-check to apply component-scoped
//                    relations.
//
// These helpers are not part of the Identity struct because they are
// orthogonal to subject identity and may be set/cleared independently
// during a single request's lifetime (e.g., the orchestrator sets
// ExecutorUser at sub-agent dispatch time without re-setting Identity).

import (
	"context"

	"github.com/zeroroot-ai/sdk/auth/internal/sysconst"
)

// SystemTenantString is the literal string identifier for the
// platform-operator tenant. Exposed alongside the typed SystemTenant
// constant for the rare callers that need the raw string (e.g. log
// message construction, key-prefix hand-rolling). Production code
// should prefer SystemTenant + .String().
const SystemTenantString = sysconst.Reserved

// TenantScopedRedisKey generates a tenant-scoped Redis key in the
// canonical format "tenant:<tenant>:<key>". Used by daemon code that
// stores per-tenant data outside the per-tenant database model
// (event streams, in-process caches that need a stable key).
//
// The tenant argument is plain string to accept legacy tenant values
// during migration; new code SHOULD use TenantID.String().
//
// Spec: when the data-plane spec lands, every per-tenant Redis
// access goes through pool.For(tenant).Redis which is bound to the
// tenant's logical DB and does not need prefixing. This helper is
// retained only for migration ergonomics.
func TenantScopedRedisKey(tenant, key string) string {
	return "tenant:" + tenant + ":" + key
}

// ---------------------------------------------------------------------
// Acting/initiator/executor user context plumbing.
//
// These three carry user-IDs distinct from Identity.Subject because
// they describe principal *intent* across delegation boundaries that
// the underlying Identity does not capture.
// ---------------------------------------------------------------------

type actingUserKey struct{}
type initiatorUserKey struct{}
type executorUserKey struct{}
type componentScopeKey struct{}

// ActingUserFromContext returns the user ID a platform service is
// acting on behalf of, plus ok=true when set. Empty + ok=false
// otherwise.
//
// Used by the dashboard pod when it makes a daemon RPC on behalf of
// a logged-in human. The Identity carries the dashboard's own
// service-account subject; the acting-user header tells the daemon
// the human's user ID for audit attribution.
func ActingUserFromContext(ctx context.Context) (string, bool) {
	if v, ok := ctx.Value(actingUserKey{}).(string); ok && v != "" {
		return v, true
	}
	return "", false
}

// ContextWithActingUser stores the acting-user ID on ctx.
func ContextWithActingUser(ctx context.Context, userID string) context.Context {
	if userID == "" {
		return ctx
	}
	return context.WithValue(ctx, actingUserKey{}, userID)
}

// InitiatorUserFromContext returns the user who triggered the mission,
// plus ok=true when set. Stable across the entire mission span tree
// — sub-agent delegation, scheduled runs, checkpoint/resume.
func InitiatorUserFromContext(ctx context.Context) (string, bool) {
	if v, ok := ctx.Value(initiatorUserKey{}).(string); ok && v != "" {
		return v, true
	}
	return "", false
}

// ContextWithInitiatorUser stores the mission-initiator user ID on
// ctx. Called once at mission start; subsequent dispatches inherit
// it via context.
func ContextWithInitiatorUser(ctx context.Context, userID string) context.Context {
	if userID == "" {
		return ctx
	}
	return context.WithValue(ctx, initiatorUserKey{}, userID)
}

// ExecutorUserFromContext returns the owner of the currently-
// executing agent, plus ok=true when set. May differ from
// InitiatorUser under cross-user delegation.
func ExecutorUserFromContext(ctx context.Context) (string, bool) {
	if v, ok := ctx.Value(executorUserKey{}).(string); ok && v != "" {
		return v, true
	}
	return "", false
}

// ContextWithExecutorUser stores the executing-agent owner user ID
// on ctx. Set at sub-agent dispatch time so attribution follows the
// code path through delegation.
func ContextWithExecutorUser(ctx context.Context, userID string) context.Context {
	if userID == "" {
		return ctx
	}
	return context.WithValue(ctx, executorUserKey{}, userID)
}

// ComponentScopeFromContext returns the component scope (e.g.
// "component:agent-abc123") for the current request, or "" if the
// request was not authenticated via a capability grant. The FGA
// batch-check branches on this value to apply component-scoped
// relations.
func ComponentScopeFromContext(ctx context.Context) string {
	if v, ok := ctx.Value(componentScopeKey{}).(string); ok {
		return v
	}
	return ""
}

// ContextWithComponentScope stores the component scope on ctx.
func ContextWithComponentScope(ctx context.Context, scope string) context.Context {
	if scope == "" {
		return ctx
	}
	return context.WithValue(ctx, componentScopeKey{}, scope)
}

// ---------------------------------------------------------------------
// ContextWithTenant alias (for legacy callers using ContextWithTenant
// instead of WithTenant). Identical semantics.
// ---------------------------------------------------------------------

// ContextWithTenant is an alias for WithTenant kept for naming-
// consistency with the other Context* helpers in this file.
func ContextWithTenant(ctx context.Context, tenant TenantID) context.Context {
	return WithTenant(ctx, tenant)
}

// TenantStringFromContext returns the context tenant as its string
// representation, or "" when no identity is on the context. This is
// the migration replacement for the legacy gibson
// internal/identity.TenantFromContext call sites.
//
// NOTE on semantic difference from the legacy: legacy returned
// "_system" on absence; this helper returns "". Call sites that need
// the system-tenant fallback behaviour MUST handle it explicitly:
//
//	tenant := auth.TenantStringFromContext(ctx)
//	if tenant == "" { tenant = auth.SystemTenantString }
//
// New code SHOULD prefer TenantFromContext directly so the missing-
// identity case is a typed bool (better for fail-closed handler logic)
// rather than a magic empty string.
func TenantStringFromContext(ctx context.Context) string {
	t, ok := TenantFromContext(ctx)
	if !ok {
		return ""
	}
	return t.String()
}

// ContextWithTenantString is a string-input convenience wrapping
// ContextWithTenant. Used by tests and migration scaffolding that
// have a known-valid tenant identifier as a string. Returns ctx
// unchanged when s fails NewTenantID validation — production code
// SHOULD construct the typed TenantID via NewTenantID and surface
// the validation error rather than silently swallowing it.
func ContextWithTenantString(ctx context.Context, s string) context.Context {
	t, err := NewTenantID(s)
	if err != nil {
		return ctx
	}
	return ContextWithTenant(ctx, t)
}
