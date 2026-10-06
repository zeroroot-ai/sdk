// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Zero Root AI

package plugin

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestMethodDescriptor_Capabilities asserts that capabilities on a
// MethodDescriptor are preserved in order.
func TestMethodDescriptor_Capabilities(t *testing.T) {
	caps := []string{"cache", "rate_limit:tier1", "audit"}
	md := MethodDescriptor{
		Name:         "DoThing",
		Capabilities: caps,
	}

	require.Len(t, md.Capabilities, 3)
	assert.Equal(t, "cache", md.Capabilities[0])
	assert.Equal(t, "rate_limit:tier1", md.Capabilities[1])
	assert.Equal(t, "audit", md.Capabilities[2])
}
