// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Zero Root AI

package graphrag_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/zeroroot-ai/sdk/graphrag"
)

// TestSimpleTaxonomy_ExtensionMethods verifies that SimpleTaxonomy correctly
// implements the new extension query methods.
func TestSimpleTaxonomy_ExtensionMethods(t *testing.T) {
	taxonomy := graphrag.NewSimpleTaxonomy()

	t.Run("ExtensionNames returns empty slice", func(t *testing.T) {
		names := taxonomy.ExtensionNames()
		assert.NotNil(t, names)
		assert.Empty(t, names)
	})

	t.Run("ExtensionInfo returns nil for any name", func(t *testing.T) {
		info := taxonomy.ExtensionInfo("some-extension")
		assert.Nil(t, info)

		info = taxonomy.ExtensionInfo("")
		assert.Nil(t, info)
	})

	t.Run("NodeTypeSource returns core for known types", func(t *testing.T) {
		source := taxonomy.NodeTypeSource(graphrag.NodeTypeHost)
		assert.Equal(t, "core", source)

		source = taxonomy.NodeTypeSource(graphrag.NodeTypePort)
		assert.Equal(t, "core", source)

		source = taxonomy.NodeTypeSource(graphrag.NodeTypeFinding)
		assert.Equal(t, "core", source)
	})

	t.Run("NodeTypeSource returns unknown for unknown types", func(t *testing.T) {
		source := taxonomy.NodeTypeSource("custom_node_type")
		assert.Equal(t, "unknown", source)

		source = taxonomy.NodeTypeSource("nonexistent")
		assert.Equal(t, "unknown", source)
	})
}

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
