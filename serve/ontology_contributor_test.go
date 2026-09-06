// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Zero Root AI

package serve

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/zeroroot-ai/sdk/graphrag"
)

// nonContributor does not implement OntologyContributor.
type nonContributor struct{}

// contributor implements OntologyContributor with a populated extension.
type contributor struct{}

func (contributor) OntologyExtension() graphrag.OntologyExtension {
	return graphrag.OntologyExtension{
		Prefixes: map[string]string{"mycorp": "https://mycorp.example/"},
		Hierarchies: []graphrag.HierarchyDef{
			{NodeType: "finding", Label: "mycorp:Secret", SubClassOf: "mycorp:Sensitive"},
		},
	}
}

// emptyContributor implements OntologyContributor but returns the zero value.
// ontologyExtensionFrom must return that zero value verbatim — callers detect
// "nothing to send" via OntologyExtension.IsZero, not via the type-assertion.
type emptyContributor struct{}

func (emptyContributor) OntologyExtension() graphrag.OntologyExtension {
	return graphrag.OntologyExtension{}
}

func TestOntologyExtensionFrom_NonContributorReturnsZero(t *testing.T) {
	t.Parallel()
	got := ontologyExtensionFrom(nonContributor{})
	assert.True(t, got.IsZero(), "component without interface must produce zero ext")
}

func TestOntologyExtensionFrom_ContributorReturnsPopulated(t *testing.T) {
	t.Parallel()
	got := ontologyExtensionFrom(contributor{})
	assert.False(t, got.IsZero())
	assert.Len(t, got.Hierarchies, 1)
	assert.Equal(t, "mycorp:Secret", got.Hierarchies[0].Label)
}

func TestOntologyExtensionFrom_EmptyContributorReturnsZero(t *testing.T) {
	t.Parallel()
	got := ontologyExtensionFrom(emptyContributor{})
	assert.True(t, got.IsZero(), "contributor returning zero value must produce IsZero ext")
}

func TestOntologyExtensionFrom_NilInterfaceReturnsZero(t *testing.T) {
	t.Parallel()
	// A nil interface (not a typed nil — those would panic on method call) is
	// the realistic case when an early-return path forgets to pass a component.
	got := ontologyExtensionFrom(nil)
	assert.True(t, got.IsZero())
}
