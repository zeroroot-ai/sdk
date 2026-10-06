// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Zero Root AI

package taxonomy

import (
	"io/fs"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestValidationWarning_String(t *testing.T) {
	t.Run("without suggestions", func(t *testing.T) {
		w := ValidationWarning{Field: "severity", Message: "unusual value"}
		assert.Equal(t, "severity: unusual value", w.String())
	})

	t.Run("with suggestions", func(t *testing.T) {
		w := ValidationWarning{
			Field:       "severity",
			Message:     "unknown value",
			Suggestions: []string{"high", "medium"},
		}
		got := w.String()
		assert.Contains(t, got, "severity: unknown value")
		assert.Contains(t, got, "suggestions: high, medium")
	})
}

func TestEmbeddedOntology(t *testing.T) {
	fsys := EmbeddedOntology()
	require.NotNil(t, fsys)

	// The embedded FS is rooted such that the "ontology" directory is readable.
	entries, err := fs.ReadDir(fsys, "ontology")
	require.NoError(t, err)
	assert.NotEmpty(t, entries)
}
