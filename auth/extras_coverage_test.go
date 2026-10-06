// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Zero Root AI

package auth

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestTenantScopedRedisKey(t *testing.T) {
	assert.Equal(t, "tenant:acme:sessions", TenantScopedRedisKey("acme", "sessions"))
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
