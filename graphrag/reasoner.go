// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Zero Root AI

package graphrag

// Reasoner performs ontology-aware graph reasoning over a loaded
// OntologyExtension. The SDK declares this interface so the daemon and any
// SDK-level tooling can share a contract without the implementation living
// here.
//
// The implementation lives in the gibson daemon
// (enterprise/platform/gibson/internal/graphrag/reasoner.go). The SDK only
// publishes the contract.
//
// All IRI arguments use prefix:localname form (e.g. "soc2:CC6.1",
// "mitre:T1190.001"). Methods return nil/empty slices rather than errors for
// unknown IRIs — callers should treat an empty result as "not found in this
// ontology", not as a hard failure.
type Reasoner interface {
	// Ancestors returns all transitive parent IRIs of the given IRI in the
	// subClassOf hierarchy. The root node (which has no parent) is included as
	// the final element. The slice is ordered from closest ancestor to farthest.
	// Returns an empty slice if iri is unknown or has no parents.
	Ancestors(iri string) []string

	// Descendants returns all transitive child IRIs of the given IRI in the
	// subClassOf hierarchy. Returns an empty slice if iri is a leaf or unknown.
	Descendants(iri string) []string

	// Equivalents returns all IRIs that are asserted to be equivalent to iri
	// via sameAs pairs, including transitive closure. The input iri is NOT
	// included in the result. Returns an empty slice if iri has no equivalences.
	Equivalents(iri string) []string

	// IsSubclassOf reports whether child is a (direct or transitive) subclass
	// of parent according to the loaded ontology. Returns false if either IRI
	// is unknown.
	IsSubclassOf(child, parent string) bool

	// IFPsForType returns the property names declared as inverse-functional for
	// the given nodeType. Returns an empty slice if nodeType has no IFP
	// declarations.
	IFPsForType(nodeType string) []string
}
