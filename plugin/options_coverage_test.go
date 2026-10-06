// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Zero Root AI

package plugin

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/zeroroot-ai/sdk/plugin/lifecycle"
)

func TestWithLifecycle(t *testing.T) {
	called := false
	hooks := lifecycle.LifecycleHooks{
		OnStart: func(_ context.Context) error { called = true; return nil },
	}

	c := &config{}
	WithLifecycle(hooks)(c)

	require.NotNil(t, c.hooks.OnStart)
	require.NoError(t, c.hooks.OnStart(context.Background()))
	assert.True(t, called)
}

