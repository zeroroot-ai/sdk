// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Zero Root AI

package graphrag_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/zeroroot-ai/sdk/graphrag"
)

// TestDefaultTaxonomyRegistry_TaxonomyIntrospector verifies that DefaultTaxonomyRegistry
// implements the TaxonomyIntrospector interface correctly.
func TestDefaultTaxonomyRegistry_TaxonomyIntrospector(t *testing.T) {
	core := graphrag.NewSimpleTaxonomy()
	registry := graphrag.NewTaxonomyRegistry(core)

	// Verify it implements TaxonomyIntrospector by using it as one
	var introspector graphrag.TaxonomyIntrospector = registry

	t.Run("Version delegates to core", func(t *testing.T) {
		version := introspector.Version()
		assert.NotEmpty(t, version)
		assert.Equal(t, core.Version(), version)
	})

	t.Run("TechniqueIDs delegates to core", func(t *testing.T) {
		ids := introspector.TechniqueIDs("")
		// Should return whatever core returns (may be nil or empty slice)
		// Just verify no panic and type is correct
		_ = ids
	})

	t.Run("TechniqueInfo delegates to core", func(t *testing.T) {
		// Since core doesn't have techniques initialized, this should return nil
		info := introspector.TechniqueInfo("T1234")
		assert.Nil(t, info)
	})
}
