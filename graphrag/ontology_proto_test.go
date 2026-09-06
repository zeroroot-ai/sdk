// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Zero Root AI

package graphrag_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	graphragpb "github.com/zeroroot-ai/sdk/api/gen/gibson/graphrag/v1"
	"github.com/zeroroot-ai/sdk/graphrag"
)

func TestOntologyExtensionToProto_FullRoundTrip(t *testing.T) {
	t.Parallel()

	in := graphrag.OntologyExtension{
		Prefixes: map[string]string{
			"mycorp": "https://mycorp.example/sec/",
			"cwe":    "https://cwe.mitre.org/data/definitions/",
		},
		Hierarchies: []graphrag.HierarchyDef{
			{NodeType: "finding", Label: "mycorp:LeakedSecret", SubClassOf: "mycorp:ProprietaryFinding"},
			{NodeType: "finding", Label: "mycorp:HardcodedKey", SubClassOf: "mycorp:LeakedSecret"},
		},
		Equivalences: [][2]string{
			{"mycorp:LeakedSecret", "gibson:disclosure"},
			{"mycorp:HardcodedKey", "cwe:CWE-798"},
		},
		IFPs: []graphrag.IFPDef{
			{NodeType: "my_custom_artifact", Property: "content_hash"},
		},
		RawTriples: []byte("@prefix ex: <https://example/> .\nex:a ex:b ex:c .\n"),
	}

	out := graphrag.OntologyExtensionFromProto(graphrag.OntologyExtensionToProto(in))
	assert.Equal(t, in, out, "OntologyExtension must round-trip through the proto form unchanged")
}

func TestOntologyExtensionFromProto_NilReturnsZero(t *testing.T) {
	t.Parallel()

	got := graphrag.OntologyExtensionFromProto(nil)
	assert.True(t, got.IsZero(), "nil proto must produce zero OntologyExtension")
}

func TestOntologyExtension_IsZero(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		ext  graphrag.OntologyExtension
		zero bool
	}{
		{"empty", graphrag.OntologyExtension{}, true},
		{"prefixes-only", graphrag.OntologyExtension{Prefixes: map[string]string{"a": "b"}}, false},
		{"hierarchies-only", graphrag.OntologyExtension{Hierarchies: []graphrag.HierarchyDef{{Label: "x"}}}, false},
		{"equivalences-only", graphrag.OntologyExtension{Equivalences: [][2]string{{"a", "b"}}}, false},
		{"ifps-only", graphrag.OntologyExtension{IFPs: []graphrag.IFPDef{{NodeType: "n", Property: "p"}}}, false},
		{"raw-triples-only", graphrag.OntologyExtension{RawTriples: []byte("x")}, false},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.zero, tc.ext.IsZero())
		})
	}
}

func TestOntologyExtensionToProto_EmptySlicesAndMaps(t *testing.T) {
	t.Parallel()

	// Zero-value input must produce a non-nil proto with empty (not nil) repeated fields,
	// so the wire form is stable regardless of how the caller initialised the Go struct.
	p := graphrag.OntologyExtensionToProto(graphrag.OntologyExtension{})
	require.NotNil(t, p)
	assert.Empty(t, p.GetPrefixes())
	assert.Empty(t, p.GetHierarchies())
	assert.Empty(t, p.GetEquivalences())
	assert.Empty(t, p.GetIfps())
	assert.Empty(t, p.GetRawTriples())
}

func TestOntologyExtensionFromProto_RegressionShape(t *testing.T) {
	t.Parallel()

	// Catch silent rename: assert proto field types are exactly what
	// OntologyExtensionFromProto expects. If a future proto edit renames a
	// field or changes a type, this test fails clearly at compile time rather
	// than producing subtly wrong runtime behaviour.
	p := &graphragpb.OntologyExtension{
		Prefixes: map[string]string{"a": "b"},
		Hierarchies: []*graphragpb.HierarchyDef{
			{NodeType: "n", Label: "l", SubClassOf: "p"},
		},
		Equivalences: []*graphragpb.SameAsPair{
			{IriA: "x", IriB: "y"},
		},
		Ifps: []*graphragpb.IFPDef{
			{NodeType: "n", Property: "prop"},
		},
		RawTriples: []byte("rt"),
	}
	got := graphrag.OntologyExtensionFromProto(p)

	assert.Equal(t, map[string]string{"a": "b"}, got.Prefixes)
	assert.Equal(t, []graphrag.HierarchyDef{{NodeType: "n", Label: "l", SubClassOf: "p"}}, got.Hierarchies)
	assert.Equal(t, [][2]string{{"x", "y"}}, got.Equivalences)
	assert.Equal(t, []graphrag.IFPDef{{NodeType: "n", Property: "prop"}}, got.IFPs)
	assert.Equal(t, []byte("rt"), got.RawTriples)
}
