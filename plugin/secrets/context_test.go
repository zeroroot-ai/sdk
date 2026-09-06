// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Zero Root AI

package secrets_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/zeroroot-ai/sdk/plugin/manifest"
	"github.com/zeroroot-ai/sdk/plugin/secrets"
)

// TestContext_RoundTrip verifies NewContext stores a client that FromContext
// recovers, and that a bare context reports absence rather than returning a nil
// client as present.
func TestContext_RoundTrip(t *testing.T) {
	_, ok := secrets.FromContext(context.Background())
	assert.False(t, ok, "bare context must report no client")

	m := &manifest.Manifest{}
	client := secrets.New(m, func(context.Context, string) ([]byte, error) {
		return nil, nil
	}, secrets.CacheConfig{})

	ctx := secrets.NewContext(context.Background(), client)
	got, ok := secrets.FromContext(ctx)
	require.True(t, ok)
	assert.Equal(t, client, got)
}

// TestContext_Isolation verifies that the secrets context key does not collide
// with a sibling value stored under a different key on the same context.
func TestContext_Isolation(t *testing.T) {
	type otherKey struct{}
	m := &manifest.Manifest{}
	client := secrets.New(m, func(context.Context, string) ([]byte, error) {
		return nil, nil
	}, secrets.CacheConfig{})

	ctx := context.WithValue(context.Background(), otherKey{}, "unrelated")
	ctx = secrets.NewContext(ctx, client)

	got, ok := secrets.FromContext(ctx)
	require.True(t, ok)
	assert.Equal(t, client, got)
	assert.Equal(t, "unrelated", ctx.Value(otherKey{}))
}
