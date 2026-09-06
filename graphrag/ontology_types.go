// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Zero Root AI

package graphrag

// OntologyExtension carries the parsed content of an ontology YAML file in a
// form ready for the gibson daemon's Reasoner to consume. The SDK declares
// this type so both the daemon and any SDK tooling can share a common data
// shape without requiring an import of the daemon.
//
// An OntologyExtension is produced by converting a taxonomy.Ontology value
// (the YAML-parsed representation) into this GraphRAG-facing struct. The
// conversion is the daemon's responsibility; the SDK provides the target type.
//
// RawTriples is an optional Turtle (*.ttl) payload for power users who need
// to express semantics beyond the YAML authoring surface. The initial daemon
// implementation may store RawTriples without parsing them; full Turtle
// ingestion is deferred to a follow-up milestone.
type OntologyExtension struct {
	// Prefixes maps short prefix names to base IRIs, mirroring the prefixes
	// block of the source ontology YAML.
	Prefixes map[string]string

	// Hierarchies is the ordered list of subClassOf assertions parsed from the
	// YAML. Each entry binds a node type, a child IRI label, and an optional
	// parent IRI.
	Hierarchies []HierarchyDef

	// Equivalences is the list of sameAs pairs. Each [2]string is [iriA, iriB]
	// asserting that both IRIs identify the same concept.
	Equivalences [][2]string

	// IFPs is the list of inverse-functional property declarations. An IFP
	// uniquely identifies a node of a given node type by the value of one
	// property.
	IFPs []IFPDef

	// RawTriples holds optional Turtle (*.ttl) content supplied alongside the
	// ontology YAML by power users. The daemon SHOULD store this verbatim and
	// MAY parse it in a future milestone. SDK callers MUST NOT rely on the
	// daemon having parsed RawTriples.
	RawTriples []byte
}

// HierarchyDef is a single subClassOf assertion within an OntologyExtension.
// It mirrors taxonomy.HierarchyDef and is duplicated here to keep the graphrag
// package free of a dependency on the taxonomy package.
type HierarchyDef struct {
	// NodeType is the GraphRAG node type this hierarchy entry applies to.
	NodeType string

	// Label is the IRI of the child class (prefix:localname form).
	Label string

	// SubClassOf is the IRI of the parent class. Empty string denotes a root
	// node with no parent in this ontology.
	SubClassOf string
}

// IFPDef declares an inverse-functional property within an OntologyExtension.
// It mirrors taxonomy.IFPDef and is duplicated here for the same reason.
type IFPDef struct {
	// NodeType is the GraphRAG node type this IFP applies to.
	NodeType string

	// Property is the name of the identity-bearing property on that node type.
	Property string
}
