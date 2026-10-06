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
