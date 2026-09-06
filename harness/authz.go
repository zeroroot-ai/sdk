// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Zero Root AI

// Package harness provides authorization constants and error types for Gibson
// components (tools, plugins, agents) that use the SDK Harness interface.
//
// The constants in this package define the stable vocabulary for component
// authorization checks. They correspond directly to FGA relations in the
// Gibson authorization model (prefixed with "can_" by the daemon handler).
//
// Typical usage:
//
//	if err := h.Authorize(ctx, harness.ActionExecute, "tool:mytool-a"); err != nil {
//	    slog.Error("authz denied", "action", harness.ActionExecute, "resource", "tool:mytool-a", "error", err)
//	    return nil, err
//	}
package harness

import (
	"context"
	"errors"
)

// Action constants define the allowed verbs for harness.Authorize calls.
// These map to FGA relations by the daemon prefixing them with "can_":
// ActionExecute → "can_execute", ActionRead → "can_read", etc.
const (
	// ActionExecute authorizes running or invoking a component.
	// Use before initiating scans, process execution, or tool invocation.
	ActionExecute = "execute"

	// ActionConfigure authorizes changing configuration or settings.
	// Use before modifying runtime parameters of a component or system.
	ActionConfigure = "configure"

	// ActionRead authorizes reading or querying data from an external system.
	// Use before reading from APIs, file systems, or data stores.
	ActionRead = "read"

	// ActionWrite authorizes writing, creating, or mutating data.
	// Use before modifying records, writing files, or sending network traffic.
	ActionWrite = "write"
)

// Sentinel errors returned by harness.Authorize and its gRPC implementation.
// Use errors.Is to match these errors in component code.
var (
	// ErrUnauthorized is returned when the FGA check explicitly denies the action.
	// Components must not proceed with the operation and should return
	// a PERMISSION_DENIED result to the caller.
	ErrUnauthorized = errors.New("not authorized")

	// ErrAuthzServiceUnavailable is returned when the daemon or FGA is unreachable.
	// Fail-closed behavior (default): treat as deny.
	// Fail-open behavior (dev mode): proceed but log a WARN and increment the
	// gibson_component_authz_fail_open_total counter.
	ErrAuthzServiceUnavailable = errors.New("authorization service unavailable")

	// ErrInvalidAction is returned when action or resource is empty or malformed.
	// Resource must be in "<type>:<name>" format (e.g. "tool:mytool-a").
	ErrInvalidAction = errors.New("invalid action or resource")

	// ErrWorkExpired is returned by the SDK serve loop when an AuthzContext TTL
	// has elapsed. The work item is rejected and the mission sees a failed step.
	ErrWorkExpired = errors.New("work envelope expired")
)

// ============================================================================
// Authorizer — narrow interface for context injection
// ============================================================================

// Authorizer is a narrow interface exposing only the authorization check.
// Tools retrieve an Authorizer from their execution context via AuthorizerFromContext
// rather than receiving the full agent.Harness (which would create an import cycle
// between tool packages and the agent package).
//
// The CallbackHarness in the serve package satisfies this interface.
// When no authorizer is present in the context, AuthorizerFromContext returns
// a fail-closed deny-all authorizer; permissive behavior must be opted into
// explicitly via ContextWithAllowAllAuthorizer.
type Authorizer interface {
	Authorize(ctx context.Context, action, resource string) error
}

// authorizerContextKey is the unexported key used to store an Authorizer in a
// context. Using a private type prevents key collisions with third-party code.
type authorizerContextKey struct{}

// ContextWithAuthorizer returns a derived context carrying the given Authorizer.
// Call this in the SDK serve loop before invoking ExecuteProto so that tools
// can retrieve the authorizer via AuthorizerFromContext.
func ContextWithAuthorizer(ctx context.Context, a Authorizer) context.Context {
	return context.WithValue(ctx, authorizerContextKey{}, a)
}

// AuthorizerFromContext retrieves the Authorizer stored by ContextWithAuthorizer.
//
// Fail-closed by default: when NO authorizer is present in the context, it
// returns a deny-all authorizer whose Authorize always returns ErrUnauthorized.
// The SDK serve loop always injects a real Authorizer before invoking a tool
// (see ContextWithAuthorizer in serve), so the deny default only surfaces for
// out-of-band invocations (ad-hoc calls, tests) that forgot to wire one — which
// is exactly the missing-provider mistake we want to fail rather than silently
// allow. Code that genuinely wants permissive behavior (local dev, unit tests)
// must opt in explicitly with ContextWithAllowAllAuthorizer.
func AuthorizerFromContext(ctx context.Context) Authorizer {
	if a, ok := ctx.Value(authorizerContextKey{}).(Authorizer); ok && a != nil {
		return a
	}
	return denyAuthorizer{}
}

// denyAuthorizer is the fail-closed default returned when no authorizer is
// present in the context. It denies every check.
type denyAuthorizer struct{}

func (denyAuthorizer) Authorize(_ context.Context, _, _ string) error {
	return ErrUnauthorized
}

// AllowAllAuthorizer is an explicit, permissive Authorizer for local dev and
// tests. It allows every check. It is never installed implicitly — callers must
// opt in via ContextWithAllowAllAuthorizer so that a missing real authorizer
// fails closed instead of silently allowing.
type AllowAllAuthorizer struct{}

func (AllowAllAuthorizer) Authorize(_ context.Context, _, _ string) error { return nil }

// ContextWithAllowAllAuthorizer derives a context carrying the permissive
// AllowAllAuthorizer. Use only in local-dev or test code that deliberately
// wants to bypass authorization; never on a production code path.
func ContextWithAllowAllAuthorizer(ctx context.Context) context.Context {
	return ContextWithAuthorizer(ctx, AllowAllAuthorizer{})
}
