// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Zero Root AI

package graphrag_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
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

// TestDefaultTaxonomyRegistry_ExtensionMethods verifies that DefaultTaxonomyRegistry
// correctly implements the new extension query methods.
func TestDefaultTaxonomyRegistry_ExtensionMethods(t *testing.T) {
	core := graphrag.NewSimpleTaxonomy()
	registry := graphrag.NewTaxonomyRegistry(core)

	t.Run("ExtensionNames returns empty initially", func(t *testing.T) {
		names := registry.ExtensionNames()
		assert.NotNil(t, names)
		assert.Empty(t, names)
	})

	t.Run("ExtensionInfo returns nil for non-existent extension", func(t *testing.T) {
		info := registry.ExtensionInfo("non-existent")
		assert.Nil(t, info)
	})

	t.Run("NodeTypeSource returns core for core types", func(t *testing.T) {
		source := registry.NodeTypeSource(graphrag.NodeTypeHost)
		assert.Equal(t, "core", source)

		source = registry.NodeTypeSource(graphrag.NodeTypeService)
		assert.Equal(t, "core", source)
	})

	t.Run("NodeTypeSource returns unknown for unrecognized types", func(t *testing.T) {
		source := registry.NodeTypeSource("custom_type")
		assert.Equal(t, "unknown", source)
	})

	// Register an extension
	ext := graphrag.TaxonomyExtension{
		NodeTypes: []graphrag.NodeTypeDefinition{
			{
				Name:        "custom_asset",
				Category:    "asset",
				Description: "A custom asset type",
				Properties: []graphrag.PropertyInfo{
					{
						Name:     "name",
						Type:     "string",
						Required: true,
					},
				},
			},
			{
				Name:        "custom_finding",
				Category:    "finding",
				Description: "A custom finding type",
			},
		},
		Relationships: []graphrag.RelationshipDefinition{
			{
				Name:        "CUSTOM_RELATION",
				Category:    "asset",
				Description: "A custom relationship",
				FromTypes:   []string{"custom_asset"},
				ToTypes:     []string{graphrag.NodeTypeHost},
			},
		},
	}

	err := registry.RegisterExtension("test-agent", ext)
	require.NoError(t, err)

	t.Run("ExtensionNames returns registered extensions", func(t *testing.T) {
		names := registry.ExtensionNames()
		assert.NotNil(t, names)
		assert.Len(t, names, 1)
		assert.Contains(t, names, "test-agent")
	})

	t.Run("ExtensionInfo returns extension definition", func(t *testing.T) {
		info := registry.ExtensionInfo("test-agent")
		require.NotNil(t, info)
		assert.Len(t, info.NodeTypes, 2)
		assert.Len(t, info.Relationships, 1)
		assert.Equal(t, "custom_asset", info.NodeTypes[0].Name)
		assert.Equal(t, "custom_finding", info.NodeTypes[1].Name)
		assert.Equal(t, "CUSTOM_RELATION", info.Relationships[0].Name)
	})

	t.Run("NodeTypeSource returns extension name for extension types", func(t *testing.T) {
		source := registry.NodeTypeSource("custom_asset")
		assert.Equal(t, "test-agent", source)

		source = registry.NodeTypeSource("custom_finding")
		assert.Equal(t, "test-agent", source)
	})

	t.Run("NodeTypeSource still returns core for core types", func(t *testing.T) {
		source := registry.NodeTypeSource(graphrag.NodeTypeHost)
		assert.Equal(t, "core", source)
	})

	t.Run("NodeTypes includes both core and extension types", func(t *testing.T) {
		types := registry.NodeTypes()
		assert.Contains(t, types, graphrag.NodeTypeHost)
		assert.Contains(t, types, "custom_asset")
		assert.Contains(t, types, "custom_finding")
	})

	t.Run("NodeTypeInfo returns info for extension types", func(t *testing.T) {
		info := registry.NodeTypeInfo("custom_asset")
		require.NotNil(t, info)
		assert.Equal(t, "custom_asset", info.Type)
		assert.Equal(t, "asset", info.Category)
		assert.Equal(t, "A custom asset type", info.Description)
		assert.Len(t, info.Properties, 1)
	})

	t.Run("RelationshipTypes includes extension relationships", func(t *testing.T) {
		types := registry.RelationshipTypes()
		assert.Contains(t, types, "CUSTOM_RELATION")
	})

	t.Run("RelationshipTypeInfo returns info for extension relationships", func(t *testing.T) {
		info := registry.RelationshipTypeInfo("CUSTOM_RELATION")
		require.NotNil(t, info)
		assert.Equal(t, "CUSTOM_RELATION", info.Type)
		assert.Equal(t, "asset", info.Category)
		assert.Contains(t, info.FromTypes, "custom_asset")
		assert.Contains(t, info.ToTypes, graphrag.NodeTypeHost)
	})

	// Register another extension
	ext2 := graphrag.TaxonomyExtension{
		NodeTypes: []graphrag.NodeTypeDefinition{
			{
				Name:        "another_type",
				Category:    "custom",
				Description: "Another custom type",
			},
		},
	}

	err = registry.RegisterExtension("another-agent", ext2)
	require.NoError(t, err)

	t.Run("ExtensionNames returns all registered extensions", func(t *testing.T) {
		names := registry.ExtensionNames()
		assert.Len(t, names, 2)
		assert.Contains(t, names, "test-agent")
		assert.Contains(t, names, "another-agent")
	})

	t.Run("NodeTypeSource distinguishes between different extensions", func(t *testing.T) {
		source := registry.NodeTypeSource("custom_asset")
		assert.Equal(t, "test-agent", source)

		source = registry.NodeTypeSource("another_type")
		assert.Equal(t, "another-agent", source)
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
