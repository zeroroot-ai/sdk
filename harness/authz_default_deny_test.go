// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Zero Root AI

package harness

import (
	"context"
	"errors"
	"testing"
)

// Regression for the fail-open-by-omission default: a context with no
// Authorizer must DENY, not silently allow. Permissive behavior requires an
// explicit opt-in via ContextWithAllowAllAuthorizer.
func TestAuthorizerFromContext_DefaultsToDeny(t *testing.T) {
	az := AuthorizerFromContext(context.Background())
	err := az.Authorize(context.Background(), "use", "tool:example")
	if !errors.Is(err, ErrUnauthorized) {
		t.Fatalf("expected ErrUnauthorized when no authorizer in context, got %v", err)
	}
}

func TestContextWithAllowAllAuthorizer_Allows(t *testing.T) {
	ctx := ContextWithAllowAllAuthorizer(context.Background())
	az := AuthorizerFromContext(ctx)
	if err := az.Authorize(ctx, "use", "tool:example"); err != nil {
		t.Fatalf("expected allow with AllowAllAuthorizer, got %v", err)
	}
}

func TestAuthorizerFromContext_RealAuthorizerWins(t *testing.T) {
	ctx := ContextWithAuthorizer(context.Background(), AllowAllAuthorizer{})
	if err := AuthorizerFromContext(ctx).Authorize(ctx, "x", "y:z"); err != nil {
		t.Fatalf("injected authorizer should be used, got %v", err)
	}
}
