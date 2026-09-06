// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Zero Root AI

package graphrag_test

import (
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/zeroroot-ai/sdk/graphrag"
)

// TestTaxonomyRegistry_ExtensionNames tests the ExtensionNames method
func TestTaxonomyRegistry_ExtensionNames(t *testing.T) {
	core := graphrag.NewSimpleTaxonomy()
	registry := graphrag.NewTaxonomyRegistry(core)

	t.Run("returns empty slice initially", func(t *testing.T) {
		names := registry.ExtensionNames()
		assert.NotNil(t, names, "ExtensionNames should return non-nil slice")
		assert.Empty(t, names, "ExtensionNames should return empty slice initially")
	})

	t.Run("returns single extension after registration", func(t *testing.T) {
		ext := graphrag.TaxonomyExtension{
			NodeTypes: []graphrag.NodeTypeDefinition{
				{
					Name:        "test_node",
					Category:    "test",
					Description: "Test node type",
				},
			},
		}

		err := registry.RegisterExtension("test-ext", ext)
		require.NoError(t, err)

		names := registry.ExtensionNames()
		assert.Len(t, names, 1)
		assert.Contains(t, names, "test-ext")
	})

	t.Run("returns multiple extensions after multiple registrations", func(t *testing.T) {
		ext2 := graphrag.TaxonomyExtension{
			NodeTypes: []graphrag.NodeTypeDefinition{
				{
					Name:     "test_node_2",
					Category: "test",
				},
			},
		}

		err := registry.RegisterExtension("test-ext-2", ext2)
		require.NoError(t, err)

		names := registry.ExtensionNames()
		assert.Len(t, names, 2)
		assert.Contains(t, names, "test-ext")
		assert.Contains(t, names, "test-ext-2")
	})

	t.Run("does not include unregistered extensions", func(t *testing.T) {
		err := registry.UnregisterExtension("test-ext")
		require.NoError(t, err)

		names := registry.ExtensionNames()
		assert.Len(t, names, 1)
		assert.Contains(t, names, "test-ext-2")
		assert.NotContains(t, names, "test-ext")
	})
}

// TestTaxonomyRegistry_ExtensionInfo tests the ExtensionInfo method
func TestTaxonomyRegistry_ExtensionInfo(t *testing.T) {
	core := graphrag.NewSimpleTaxonomy()
	registry := graphrag.NewTaxonomyRegistry(core)

	t.Run("returns nil for non-existent extension", func(t *testing.T) {
		info := registry.ExtensionInfo("non-existent")
		assert.Nil(t, info)
	})

	t.Run("returns nil for empty string", func(t *testing.T) {
		info := registry.ExtensionInfo("")
		assert.Nil(t, info)
	})

	t.Run("returns correct extension info after registration", func(t *testing.T) {
		ext := graphrag.TaxonomyExtension{
			NodeTypes: []graphrag.NodeTypeDefinition{
				{
					Name:        "custom_asset",
					Category:    "asset",
					Description: "Custom asset type",
					Properties: []graphrag.PropertyInfo{
						{
							Name:     "asset_name",
							Type:     "string",
							Required: true,
						},
						{
							Name:        "asset_value",
							Type:        "int32",
							Required:    false,
							Description: "Asset value",
						},
					},
				},
			},
			Relationships: []graphrag.RelationshipDefinition{
				{
					Name:        "CUSTOM_REL",
					Category:    "asset",
					Description: "Custom relationship",
					FromTypes:   []string{"custom_asset"},
					ToTypes:     []string{graphrag.NodeTypeHost},
				},
			},
		}

		err := registry.RegisterExtension("test-agent", ext)
		require.NoError(t, err)

		info := registry.ExtensionInfo("test-agent")
		require.NotNil(t, info)
		assert.Len(t, info.NodeTypes, 1)
		assert.Equal(t, "custom_asset", info.NodeTypes[0].Name)
		assert.Equal(t, "asset", info.NodeTypes[0].Category)
		assert.Equal(t, "Custom asset type", info.NodeTypes[0].Description)
		assert.Len(t, info.NodeTypes[0].Properties, 2)
		assert.Equal(t, "asset_name", info.NodeTypes[0].Properties[0].Name)
		assert.True(t, info.NodeTypes[0].Properties[0].Required)
		assert.Equal(t, "asset_value", info.NodeTypes[0].Properties[1].Name)
		assert.False(t, info.NodeTypes[0].Properties[1].Required)

		assert.Len(t, info.Relationships, 1)
		assert.Equal(t, "CUSTOM_REL", info.Relationships[0].Name)
	})

	t.Run("returns nil after unregistration", func(t *testing.T) {
		err := registry.UnregisterExtension("test-agent")
		require.NoError(t, err)

		info := registry.ExtensionInfo("test-agent")
		assert.Nil(t, info)
	})

	t.Run("returned info is a pointer copy (shallow copy)", func(t *testing.T) {
		ext := graphrag.TaxonomyExtension{
			NodeTypes: []graphrag.NodeTypeDefinition{
				{
					Name:     "mutable_test",
					Category: "test",
				},
			},
		}

		err := registry.RegisterExtension("mutable-test", ext)
		require.NoError(t, err)

		info := registry.ExtensionInfo("mutable-test")
		require.NotNil(t, info)

		// The returned pointer is a shallow copy - modifying struct fields
		// won't affect internal state, but slice elements are shared
		// This is acceptable behavior for performance reasons

		// Verify we get a new pointer each time
		info2 := registry.ExtensionInfo("mutable-test")
		require.NotNil(t, info2)
		assert.NotSame(t, info, info2, "Each call should return a different pointer")

		// Both should have the same content
		assert.Equal(t, info.NodeTypes[0].Name, info2.NodeTypes[0].Name)
	})
}

// TestTaxonomyRegistry_NodeTypeSource tests the NodeTypeSource method
func TestTaxonomyRegistry_NodeTypeSource(t *testing.T) {
	core := graphrag.NewSimpleTaxonomy()
	registry := graphrag.NewTaxonomyRegistry(core)

	t.Run("returns 'core' for core node types", func(t *testing.T) {
		coreTypes := []string{
			graphrag.NodeTypeHost,
			graphrag.NodeTypePort,
			graphrag.NodeTypeService,
			graphrag.NodeTypeFinding,
			graphrag.NodeTypeDomain,
			graphrag.NodeTypeSubdomain,
			graphrag.NodeTypeEndpoint,
			graphrag.NodeTypeCertificate,
			graphrag.NodeTypeTechnology,
			graphrag.NodeTypeEvidence,
			graphrag.NodeTypeTechnique,
		}

		for _, nodeType := range coreTypes {
			source := registry.NodeTypeSource(nodeType)
			assert.Equal(t, "core", source, "Node type %s should be from core", nodeType)
		}
	})

	t.Run("returns 'unknown' for unrecognized types", func(t *testing.T) {
		unknownTypes := []string{
			"custom_unknown",
			"nonexistent_type",
			"",
			"random_string_123",
		}

		for _, nodeType := range unknownTypes {
			source := registry.NodeTypeSource(nodeType)
			assert.Equal(t, "unknown", source, "Node type %s should be unknown", nodeType)
		}
	})

	t.Run("returns extension name for extension types", func(t *testing.T) {
		ext := graphrag.TaxonomyExtension{
			NodeTypes: []graphrag.NodeTypeDefinition{
				{
					Name:     "ext_type_1",
					Category: "custom",
				},
				{
					Name:     "ext_type_2",
					Category: "custom",
				},
			},
		}

		err := registry.RegisterExtension("my-extension", ext)
		require.NoError(t, err)

		source := registry.NodeTypeSource("ext_type_1")
		assert.Equal(t, "my-extension", source)

		source = registry.NodeTypeSource("ext_type_2")
		assert.Equal(t, "my-extension", source)
	})

	t.Run("distinguishes between different extension sources", func(t *testing.T) {
		ext1 := graphrag.TaxonomyExtension{
			NodeTypes: []graphrag.NodeTypeDefinition{
				{Name: "type_from_ext1", Category: "custom"},
			},
		}

		ext2 := graphrag.TaxonomyExtension{
			NodeTypes: []graphrag.NodeTypeDefinition{
				{Name: "type_from_ext2", Category: "custom"},
			},
		}

		err := registry.RegisterExtension("extension-1", ext1)
		require.NoError(t, err)

		err = registry.RegisterExtension("extension-2", ext2)
		require.NoError(t, err)

		source1 := registry.NodeTypeSource("type_from_ext1")
		assert.Equal(t, "extension-1", source1)

		source2 := registry.NodeTypeSource("type_from_ext2")
		assert.Equal(t, "extension-2", source2)
	})

	t.Run("returns 'unknown' after extension unregistration", func(t *testing.T) {
		err := registry.UnregisterExtension("extension-1")
		require.NoError(t, err)

		source := registry.NodeTypeSource("type_from_ext1")
		assert.Equal(t, "unknown", source)
	})

	t.Run("core types remain 'core' regardless of extensions", func(t *testing.T) {
		ext := graphrag.TaxonomyExtension{
			NodeTypes: []graphrag.NodeTypeDefinition{
				{Name: "another_ext_type", Category: "custom"},
			},
		}

		err := registry.RegisterExtension("another-ext", ext)
		require.NoError(t, err)

		// Core types should still be "core"
		source := registry.NodeTypeSource(graphrag.NodeTypeHost)
		assert.Equal(t, "core", source)

		source = registry.NodeTypeSource(graphrag.NodeTypeFinding)
		assert.Equal(t, "core", source)
	})
}

// TestTaxonomyRegistry_RegisterExtension tests extension registration
func TestTaxonomyRegistry_RegisterExtension(t *testing.T) {
	t.Run("successful registration of valid extension", func(t *testing.T) {
		core := graphrag.NewSimpleTaxonomy()
		registry := graphrag.NewTaxonomyRegistry(core)

		ext := graphrag.TaxonomyExtension{
			NodeTypes: []graphrag.NodeTypeDefinition{
				{Name: "test_type", Category: "test"},
			},
		}

		err := registry.RegisterExtension("test-ext", ext)
		assert.NoError(t, err)

		// Verify registration
		names := registry.ExtensionNames()
		assert.Contains(t, names, "test-ext")
	})

	t.Run("error on duplicate registration", func(t *testing.T) {
		core := graphrag.NewSimpleTaxonomy()
		registry := graphrag.NewTaxonomyRegistry(core)

		ext := graphrag.TaxonomyExtension{
			NodeTypes: []graphrag.NodeTypeDefinition{
				{Name: "test_type", Category: "test"},
			},
		}

		err := registry.RegisterExtension("duplicate-ext", ext)
		require.NoError(t, err)

		// Try to register again with same name
		err = registry.RegisterExtension("duplicate-ext", ext)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "already registered")
	})

	t.Run("can register extension with empty node types", func(t *testing.T) {
		core := graphrag.NewSimpleTaxonomy()
		registry := graphrag.NewTaxonomyRegistry(core)

		ext := graphrag.TaxonomyExtension{
			NodeTypes: []graphrag.NodeTypeDefinition{},
			Relationships: []graphrag.RelationshipDefinition{
				{Name: "TEST_REL", Category: "test"},
			},
		}

		err := registry.RegisterExtension("empty-nodes", ext)
		assert.NoError(t, err)
	})

	t.Run("can register extension with empty relationships", func(t *testing.T) {
		core := graphrag.NewSimpleTaxonomy()
		registry := graphrag.NewTaxonomyRegistry(core)

		ext := graphrag.TaxonomyExtension{
			NodeTypes: []graphrag.NodeTypeDefinition{
				{Name: "test_type", Category: "test"},
			},
			Relationships: []graphrag.RelationshipDefinition{},
		}

		err := registry.RegisterExtension("empty-rels", ext)
		assert.NoError(t, err)
	})
}

// TestTaxonomyRegistry_UnregisterExtension tests extension unregistration
func TestTaxonomyRegistry_UnregisterExtension(t *testing.T) {
	t.Run("successful unregistration of existing extension", func(t *testing.T) {
		core := graphrag.NewSimpleTaxonomy()
		registry := graphrag.NewTaxonomyRegistry(core)

		ext := graphrag.TaxonomyExtension{
			NodeTypes: []graphrag.NodeTypeDefinition{
				{Name: "test_type", Category: "test"},
			},
		}

		err := registry.RegisterExtension("test-ext", ext)
		require.NoError(t, err)

		err = registry.UnregisterExtension("test-ext")
		assert.NoError(t, err)

		// Verify unregistration
		names := registry.ExtensionNames()
		assert.NotContains(t, names, "test-ext")
	})

	t.Run("error on unregistering non-existent extension", func(t *testing.T) {
		core := graphrag.NewSimpleTaxonomy()
		registry := graphrag.NewTaxonomyRegistry(core)

		err := registry.UnregisterExtension("non-existent")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "not registered")
	})

	t.Run("can re-register after unregistration", func(t *testing.T) {
		core := graphrag.NewSimpleTaxonomy()
		registry := graphrag.NewTaxonomyRegistry(core)

		ext := graphrag.TaxonomyExtension{
			NodeTypes: []graphrag.NodeTypeDefinition{
				{Name: "test_type", Category: "test"},
			},
		}

		// Register
		err := registry.RegisterExtension("test-ext", ext)
		require.NoError(t, err)

		// Unregister
		err = registry.UnregisterExtension("test-ext")
		require.NoError(t, err)

		// Re-register with same name
		err = registry.RegisterExtension("test-ext", ext)
		assert.NoError(t, err)

		names := registry.ExtensionNames()
		assert.Contains(t, names, "test-ext")
	})

	t.Run("unregistration removes node type mappings", func(t *testing.T) {
		core := graphrag.NewSimpleTaxonomy()
		registry := graphrag.NewTaxonomyRegistry(core)

		ext := graphrag.TaxonomyExtension{
			NodeTypes: []graphrag.NodeTypeDefinition{
				{Name: "temp_type", Category: "temp"},
			},
		}

		err := registry.RegisterExtension("temp-ext", ext)
		require.NoError(t, err)

		// Verify type is from extension
		source := registry.NodeTypeSource("temp_type")
		assert.Equal(t, "temp-ext", source)

		// Unregister
		err = registry.UnregisterExtension("temp-ext")
		require.NoError(t, err)

		// Verify type is now unknown
		source = registry.NodeTypeSource("temp_type")
		assert.Equal(t, "unknown", source)
	})
}

// TestTaxonomyRegistry_ThreadSafety tests concurrent access to the registry
func TestTaxonomyRegistry_ThreadSafety(t *testing.T) {
	core := graphrag.NewSimpleTaxonomy()
	registry := graphrag.NewTaxonomyRegistry(core)

	const numGoroutines = 50
	const numIterations = 100

	t.Run("concurrent registrations", func(t *testing.T) {
		var wg sync.WaitGroup
		errors := make(chan error, numGoroutines)

		for i := range numGoroutines {
			wg.Add(1)
			go func(id int) {
				defer wg.Done()

				ext := graphrag.TaxonomyExtension{
					NodeTypes: []graphrag.NodeTypeDefinition{
						{
							Name:     "type_" + string(rune(id)),
							Category: "test",
						},
					},
				}

				err := registry.RegisterExtension("ext-"+string(rune(id)), ext)
				if err != nil {
					errors <- err
				}
			}(i)
		}

		wg.Wait()
		close(errors)

		// Check for errors
		for err := range errors {
			t.Errorf("Concurrent registration error: %v", err)
		}
	})

	t.Run("concurrent reads during writes", func(t *testing.T) {
		var wg sync.WaitGroup

		// Writers
		for i := range 10 {
			wg.Add(1)
			go func(id int) {
				defer wg.Done()

				for j := range numIterations {
					ext := graphrag.TaxonomyExtension{
						NodeTypes: []graphrag.NodeTypeDefinition{
							{Name: "concurrent_type", Category: "test"},
						},
					}

					extName := "writer-ext-" + string(rune(id)) + "-" + string(rune(j))
					_ = registry.RegisterExtension(extName, ext)
				}
			}(i)
		}

		// Readers
		for range 40 {
			wg.Add(1)
			go func() {
				defer wg.Done()

				for range numIterations {
					_ = registry.ExtensionNames()
					_ = registry.NodeTypeSource(graphrag.NodeTypeHost)
					_ = registry.NodeTypeSource("concurrent_type")
					_ = registry.ExtensionInfo("some-ext")
				}
			}()
		}

		wg.Wait()
	})

	t.Run("concurrent register and unregister", func(t *testing.T) {
		testRegistry := graphrag.NewTaxonomyRegistry(core)
		var wg sync.WaitGroup

		// Pre-register some extensions
		for i := range 20 {
			ext := graphrag.TaxonomyExtension{
				NodeTypes: []graphrag.NodeTypeDefinition{
					{Name: "type_" + string(rune(i)), Category: "test"},
				},
			}
			_ = testRegistry.RegisterExtension("pre-ext-"+string(rune(i)), ext)
		}

		// Concurrent register/unregister
		for i := range 20 {
			wg.Add(2)

			// Register new extensions
			go func(id int) {
				defer wg.Done()
				ext := graphrag.TaxonomyExtension{
					NodeTypes: []graphrag.NodeTypeDefinition{
						{Name: "new_type_" + string(rune(id)), Category: "test"},
					},
				}
				_ = testRegistry.RegisterExtension("new-ext-"+string(rune(id)), ext)
			}(i)

			// Unregister pre-existing extensions
			go func(id int) {
				defer wg.Done()
				_ = testRegistry.UnregisterExtension("pre-ext-" + string(rune(id)))
			}(i)
		}

		wg.Wait()

		// Verify registry is still functional
		names := testRegistry.ExtensionNames()
		assert.NotNil(t, names)
	})

	t.Run("concurrent ExtensionInfo calls", func(t *testing.T) {
		testRegistry := graphrag.NewTaxonomyRegistry(core)

		ext := graphrag.TaxonomyExtension{
			NodeTypes: []graphrag.NodeTypeDefinition{
				{Name: "shared_type", Category: "test"},
			},
		}
		_ = testRegistry.RegisterExtension("shared-ext", ext)

		var wg sync.WaitGroup
		for range numGoroutines {
			wg.Add(1)
			go func() {
				defer wg.Done()
				for range numIterations {
					info := testRegistry.ExtensionInfo("shared-ext")
					assert.NotNil(t, info)
				}
			}()
		}

		wg.Wait()
	})
}

// TestSimpleTaxonomy_ExtensionMethods_EdgeCases tests edge cases for SimpleTaxonomy
func TestSimpleTaxonomy_ExtensionMethods_EdgeCases(t *testing.T) {
	taxonomy := graphrag.NewSimpleTaxonomy()

	t.Run("ExtensionNames returns non-nil empty slice", func(t *testing.T) {
		names := taxonomy.ExtensionNames()
		assert.NotNil(t, names)
		assert.Empty(t, names)
		assert.IsType(t, []string{}, names)
	})

	t.Run("ExtensionInfo always returns nil", func(t *testing.T) {
		testCases := []string{
			"",
			"test",
			"core",
			"unknown",
			"very-long-extension-name-that-does-not-exist",
		}

		for _, name := range testCases {
			info := taxonomy.ExtensionInfo(name)
			assert.Nil(t, info, "ExtensionInfo(%q) should return nil", name)
		}
	})

	t.Run("NodeTypeSource handles all core types correctly", func(t *testing.T) {
		// Test all node types from constants
		allTypes := graphrag.AllNodeTypes
		for _, nodeType := range allTypes {
			source := taxonomy.NodeTypeSource(nodeType)
			assert.Equal(t, "core", source, "NodeTypeSource(%q) should return 'core'", nodeType)
		}
	})

	t.Run("NodeTypeSource returns unknown for non-core types", func(t *testing.T) {
		nonCoreTypes := []string{
			"",
			"invalid_type",
			"custom_node",
			"extension_type",
			"123",
			"type-with-dashes",
		}

		for _, nodeType := range nonCoreTypes {
			source := taxonomy.NodeTypeSource(nodeType)
			assert.Equal(t, "unknown", source, "NodeTypeSource(%q) should return 'unknown'", nodeType)
		}
	})
}

// TestTaxonomyRegistry_GetExtension tests the GetExtension method
func TestTaxonomyRegistry_GetExtension(t *testing.T) {
	core := graphrag.NewSimpleTaxonomy()
	registry := graphrag.NewTaxonomyRegistry(core)

	t.Run("returns false for non-existent extension", func(t *testing.T) {
		ext, ok := registry.GetExtension("non-existent")
		assert.False(t, ok)
		assert.Equal(t, graphrag.TaxonomyExtension{}, ext)
	})

	t.Run("returns true and extension for registered extension", func(t *testing.T) {
		testExt := graphrag.TaxonomyExtension{
			NodeTypes: []graphrag.NodeTypeDefinition{
				{Name: "test_type", Category: "test"},
			},
		}

		err := registry.RegisterExtension("test-ext", testExt)
		require.NoError(t, err)

		ext, ok := registry.GetExtension("test-ext")
		assert.True(t, ok)
		assert.Len(t, ext.NodeTypes, 1)
		assert.Equal(t, "test_type", ext.NodeTypes[0].Name)
	})

	t.Run("returns false after unregistration", func(t *testing.T) {
		err := registry.UnregisterExtension("test-ext")
		require.NoError(t, err)

		ext, ok := registry.GetExtension("test-ext")
		assert.False(t, ok)
		assert.Equal(t, graphrag.TaxonomyExtension{}, ext)
	})
}

// TestTaxonomyRegistry_AllExtensions tests the AllExtensions method
func TestTaxonomyRegistry_AllExtensions(t *testing.T) {
	core := graphrag.NewSimpleTaxonomy()
	registry := graphrag.NewTaxonomyRegistry(core)

	t.Run("returns empty map initially", func(t *testing.T) {
		exts := registry.AllExtensions()
		assert.NotNil(t, exts)
		assert.Empty(t, exts)
	})

	t.Run("returns all registered extensions", func(t *testing.T) {
		ext1 := graphrag.TaxonomyExtension{
			NodeTypes: []graphrag.NodeTypeDefinition{
				{Name: "type1", Category: "test"},
			},
		}
		ext2 := graphrag.TaxonomyExtension{
			NodeTypes: []graphrag.NodeTypeDefinition{
				{Name: "type2", Category: "test"},
			},
		}

		err := registry.RegisterExtension("ext1", ext1)
		require.NoError(t, err)

		err = registry.RegisterExtension("ext2", ext2)
		require.NoError(t, err)

		exts := registry.AllExtensions()
		assert.Len(t, exts, 2)
		assert.Contains(t, exts, "ext1")
		assert.Contains(t, exts, "ext2")
		assert.Equal(t, "type1", exts["ext1"].NodeTypes[0].Name)
		assert.Equal(t, "type2", exts["ext2"].NodeTypes[0].Name)
	})

	t.Run("returned map is a copy", func(t *testing.T) {
		exts := registry.AllExtensions()
		require.Len(t, exts, 2)

		// Modify returned map
		delete(exts, "ext1")

		// Verify internal state unchanged
		exts2 := registry.AllExtensions()
		assert.Len(t, exts2, 2)
		assert.Contains(t, exts2, "ext1")
	})
}
