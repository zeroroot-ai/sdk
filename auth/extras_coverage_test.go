// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Zero Root AI

package auth

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestTenantScopedRedisKey(t *testing.T) {
	assert.Equal(t, "tenant:acme:sessions", TenantScopedRedisKey("acme", "sessions"))
}

func TestActingUserContext(t *testing.T) {
	ctx := context.Background()

	_, ok := ActingUserFromContext(ctx)
	assert.False(t, ok)

	// Empty user ID is a no-op.
	assert.Equal(t, ctx, ContextWithActingUser(ctx, ""))

	ctx = ContextWithActingUser(ctx, "user-1")
	got, ok := ActingUserFromContext(ctx)
	assert.True(t, ok)
	assert.Equal(t, "user-1", got)
}

func TestInitiatorUserContext(t *testing.T) {
	ctx := context.Background()

	_, ok := InitiatorUserFromContext(ctx)
	assert.False(t, ok)
	assert.Equal(t, ctx, ContextWithInitiatorUser(ctx, ""))

	ctx = ContextWithInitiatorUser(ctx, "user-2")
	got, ok := InitiatorUserFromContext(ctx)
	assert.True(t, ok)
	assert.Equal(t, "user-2", got)
}

func TestExecutorUserContext(t *testing.T) {
	ctx := context.Background()

	_, ok := ExecutorUserFromContext(ctx)
	assert.False(t, ok)
	assert.Equal(t, ctx, ContextWithExecutorUser(ctx, ""))

	ctx = ContextWithExecutorUser(ctx, "user-3")
	got, ok := ExecutorUserFromContext(ctx)
	assert.True(t, ok)
	assert.Equal(t, "user-3", got)
}

func TestComponentScopeContext(t *testing.T) {
	ctx := context.Background()

	assert.Empty(t, ComponentScopeFromContext(ctx))
	assert.Equal(t, ctx, ContextWithComponentScope(ctx, ""))

	ctx = ContextWithComponentScope(ctx, "component:agent-abc")
	assert.Equal(t, "component:agent-abc", ComponentScopeFromContext(ctx))
}

func TestTenantStringContextHelpers(t *testing.T) {
	ctx := context.Background()

	// No identity on the context -> empty string.
	assert.Empty(t, TenantStringFromContext(ctx))

	ctx = ContextWithTenant(ctx, MustNewTenantID("acme"))
	assert.Equal(t, "acme", TenantStringFromContext(ctx))

	// String convenience: valid value round-trips, invalid is a no-op.
	ctx2 := ContextWithTenantString(context.Background(), "bigcorp")
	assert.Equal(t, "bigcorp", TenantStringFromContext(ctx2))

	ctx3 := ContextWithTenantString(context.Background(), "")
	assert.Empty(t, TenantStringFromContext(ctx3))
}

func TestIdentity_IsZeroAndLogValue(t *testing.T) {
	var zero Identity
	assert.True(t, zero.IsZero())
	assert.NotNil(t, zero.LogValue())

	populated := Identity{
		Subject:  "svc-account",
		Issuer:   "https://issuer.example",
		Tenant:   MustNewTenantID("acme"),
		IssuedAt: time.Unix(1700000000, 0),
	}
	assert.False(t, populated.IsZero())
	assert.NotNil(t, populated.LogValue())
}

func TestTenantID_LogValue(t *testing.T) {
	tid := MustNewTenantID("acme")
	assert.Equal(t, "acme", tid.LogValue())
}
