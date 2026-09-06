// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Zero Root AI

package plugin

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/zeroroot-ai/sdk/plugin/manifest"
)

// synthesizeManifest builds a manifest.Manifest with two methods for use in
// tests. It bypasses manifest.Load so tests do not require a file on disk.
func synthesizeManifest(name, version string, methods []manifest.MethodDecl) *manifest.Manifest {
	return &manifest.Manifest{
		APIVersion: manifest.APIVersionV1,
		Kind:       manifest.KindPlugin,
		Metadata: manifest.ManifestMetadata{
			Name:        name,
			Version:     version,
			Description: "Test plugin",
			Author:      "test-author",
		},
		Spec: manifest.ManifestSpec{
			WorkloadClass: manifest.WorkloadClassPlugin,
			Methods:       methods,
			Runtime:       manifest.DefaultRuntime,
		},
	}
}

// TestFromManifest asserts that FromManifest maps all manifest fields correctly.
func TestFromManifest(t *testing.T) {
	m := synthesizeManifest("my-plugin", "1.2.3", []manifest.MethodDecl{
		{
			Name:        "Echo",
			Description: "Returns input unchanged",
		},
		{
			Name:        "Health",
			Description: "Health check method",
		},
	})
	m.Metadata.Description = "A great plugin"

	d := FromManifest(m)

	assert.Equal(t, "my-plugin", d.Name)
	assert.Equal(t, "1.2.3", d.Version)
	assert.Equal(t, "A great plugin", d.Description)
	require.Len(t, d.Methods, 2)

	assert.Equal(t, "Echo", d.Methods[0].Name)
	assert.Equal(t, "Returns input unchanged", d.Methods[0].Description)
	// FromManifest sees only the manifest; per-method schema is derived from the
	// registered Go handler, so it is empty here.
	assert.Empty(t, d.Methods[0].InputSchema)
	assert.Empty(t, d.Methods[0].OutputSchema)

	assert.Equal(t, "Health", d.Methods[1].Name)
	assert.Equal(t, "Health check method", d.Methods[1].Description)
}

// TestFromManifest_EmptyMethods asserts that a manifest with zero methods
// produces a non-nil empty Methods slice, not nil.
func TestFromManifest_EmptyMethods(t *testing.T) {
	m := synthesizeManifest("no-methods-plugin", "0.1.0", []manifest.MethodDecl{})
	// Override the methods to empty to bypass Validate's requirement of ≥1 method.
	m.Spec.Methods = []manifest.MethodDecl{}

	d := FromManifest(m)

	assert.NotNil(t, d.Methods, "Methods must be non-nil even when manifest declares no methods")
	assert.Empty(t, d.Methods)
}

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

// TestFromManifest_PreservesMethodOrder asserts that method order from the
// manifest is preserved in the returned Descriptor.Methods slice.
func TestFromManifest_PreservesMethodOrder(t *testing.T) {
	names := []string{"Alpha", "Beta", "Gamma"}
	methods := make([]manifest.MethodDecl, len(names))
	for i, n := range names {
		methods[i] = manifest.MethodDecl{Name: n}
	}
	m := synthesizeManifest("ordered-plugin", "1.0.0", methods)

	d := FromManifest(m)

	require.Len(t, d.Methods, 3)
	for i, name := range names {
		assert.Equal(t, name, d.Methods[i].Name, "method at index %d should be %s", i, name)
	}
}
