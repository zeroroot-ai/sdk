// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Zero Root AI

package taxonomy

import (
	"io/fs"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestEmbeddedOntology(t *testing.T) {
	fsys := EmbeddedOntology()
	require.NotNil(t, fsys)

	// The embedded FS is rooted such that the "ontology" directory is readable.
	entries, err := fs.ReadDir(fsys, "ontology")
	require.NoError(t, err)
	assert.NotEmpty(t, entries)
}
