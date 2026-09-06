// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Zero Root AI

package bumper_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/zeroroot-ai/sdk/cmd/sdk-bump/bumper"
)

func TestRealRunner(t *testing.T) {
	run := bumper.RealRunner()
	require.NotNil(t, run)

	t.Run("captures stdout", func(t *testing.T) {
		out, err := run(context.Background(), "", "echo", "hello")
		require.NoError(t, err)
		assert.Contains(t, string(out), "hello")
	})

	t.Run("returns error for missing binary", func(t *testing.T) {
		_, err := run(context.Background(), "", "definitely-not-a-real-binary-xyz")
		assert.Error(t, err)
	})
}
