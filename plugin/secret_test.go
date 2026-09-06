// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Zero Root AI

package plugin

import (
	"context"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	pluginsecrets "github.com/zeroroot-ai/sdk/plugin/secrets"
)

// TestResolveSecret_FromInjectedContext verifies that ResolveSecret recovers the
// client injected by Serve (via secrets.NewContext) and returns its value. This
// is the path a real handler exercises: Serve injects the client, the handler
// calls plugin.ResolveSecret(ctx, name).
func TestResolveSecret_FromInjectedContext(t *testing.T) {
	fake := newFakeSecretsClient(map[string][]byte{
		"cred:api_key": []byte("s3cr3t"),
	})
	ctx := pluginsecrets.NewContext(context.Background(), fake)

	got, err := ResolveSecret(ctx, "cred:api_key")
	require.NoError(t, err)
	assert.Equal(t, []byte("s3cr3t"), got)
}

// TestResolveSecret_NoClientInContext verifies the documented failure mode: a
// bare context (e.g. a handler called directly in a unit test without
// injection) yields ErrNoSecretsClient rather than a nil-pointer panic.
func TestResolveSecret_NoClientInContext(t *testing.T) {
	_, err := ResolveSecret(context.Background(), "cred:api_key")
	require.Error(t, err)
	assert.ErrorIs(t, err, ErrNoSecretsClient)
}

// TestResolveSecret_PropagatesClientError verifies that an error from the
// underlying client (e.g. undeclared or revoked secret) is returned to the
// caller unwrapped enough to inspect.
func TestResolveSecret_PropagatesClientError(t *testing.T) {
	fake := newFakeSecretsClient(map[string][]byte{})
	ctx := pluginsecrets.NewContext(context.Background(), fake)

	_, err := ResolveSecret(ctx, "cred:missing")
	require.Error(t, err)
	assert.NotErrorIs(t, err, ErrNoSecretsClient,
		"a client-side resolve failure must not masquerade as a missing-client error")
}

// TestSecretsFromContext_RoundTrip verifies the lower-level accessor returns the
// exact client that was injected, and reports absence correctly.
func TestSecretsFromContext_RoundTrip(t *testing.T) {
	fake := newFakeSecretsClient(map[string][]byte{})

	_, ok := SecretsFromContext(context.Background())
	assert.False(t, ok, "bare context carries no client")

	ctx := pluginsecrets.NewContext(context.Background(), fake)
	got, ok := SecretsFromContext(ctx)
	require.True(t, ok)
	assert.Same(t, fake, got.(*fakeSecretsClient))
}

// TestErrNoSecretsClient_IsStable guards the sentinel identity so downstream
// plugins can match on it with errors.Is even through a wrapping layer.
func TestErrNoSecretsClient_IsStable(t *testing.T) {
	require.Error(t, ErrNoSecretsClient)
	wrapped := fmt.Errorf("resolving secret: %w", ErrNoSecretsClient)
	assert.ErrorIs(t, wrapped, ErrNoSecretsClient)
}
