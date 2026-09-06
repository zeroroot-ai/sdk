// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Zero Root AI

// Package taxonomy holds the ontology extension parser and types.
//
// Ontology files are YAML documents that extend the Gibson security graph
// with external vocabulary hierarchies (MITRE ATT&CK, CWE, SOC2, NIST AI
// RMF, ATLAS, and author-contributed hierarchies). The format uses a simple
// prefix:localname IRI scheme to keep authoring approachable without
// requiring Turtle expertise.
//
// Entry point: Parse parses raw YAML bytes into an Ontology value. Callers
// that want richer semantic validation should follow up with Validate.
package taxonomy

import (
	"errors"
	"fmt"
	"strings"

	"gopkg.in/yaml.v3"
)

// Ontology is the top-level structure parsed from an ontology YAML file.
//
// Example YAML:
//
//	version: "1.0"
//	prefixes:
//	  soc2: "https://trust.aicpa.org/soc2#"
//	  mitre: "https://attack.mitre.org/techniques/"
//	hierarchies:
//	  - node_type: control
//	    label: soc2:CC6.1
//	    subClassOf: soc2:CC6
//	equivalences:
//	  - [soc2:CC6.1, mitre:T1190]
//	ifps:
//	  - node_type: control
//	    property: control_id
type Ontology struct {
	// Version is the ontology schema version string (e.g. "1.0").
	Version string `yaml:"version"`

	// Prefixes maps short prefix names to base IRIs. All prefix:localname
	// references in the file must resolve against an entry here.
	Prefixes map[string]string `yaml:"prefixes"`

	// Hierarchies is the ordered list of subClassOf assertions.
	Hierarchies []HierarchyDef `yaml:"hierarchies"`

	// Equivalences is the list of sameAs pairs. Each entry is a two-element
	// array [iriA, iriB] asserting that both IRIs identify the same concept.
	Equivalences [][2]string `yaml:"equivalences"`

	// IFPs is the list of inverse-functional property declarations. An IFP
	// uniquely identifies a node of a given type by the value of one property.
	IFPs []IFPDef `yaml:"ifps"`
}

// HierarchyDef is a single subClassOf assertion: "label is a subclass of
// subClassOf within the context of node_type".
type HierarchyDef struct {
	// NodeType is the GraphRAG node type this hierarchy entry applies to
	// (e.g. "control", "technique", "weakness").
	NodeType string `yaml:"node_type"`

	// Label is the IRI of the child class (prefix:localname form).
	Label string `yaml:"label"`

	// SubClassOf is the IRI of the parent class. Empty string is valid for
	// root nodes that have no parent in this ontology.
	SubClassOf string `yaml:"sub_class_of,omitempty"`
}

// IFPDef declares an inverse-functional property: within nodes of NodeType,
// the value of Property uniquely identifies the node.
type IFPDef struct {
	// NodeType is the GraphRAG node type this IFP applies to.
	NodeType string `yaml:"node_type"`

	// Property is the name of the identity-bearing property on that node type.
	Property string `yaml:"property"`
}

// rawEquivalence is used during YAML parsing to handle the two-element list
// form before converting to the [2]string pair form in Ontology.
type rawEquivalence []string

// Parse parses an ontology from raw YAML bytes.
//
// The returned Ontology is structurally valid (all required fields present
// and IRI syntax correct) but semantic validation (cycle detection, prefix
// resolution) requires a subsequent call to Validate.
func Parse(data []byte) (Ontology, error) {
	// rawOntology mirrors Ontology but uses [][]string for equivalences
	// so the YAML decoder handles arbitrary-length lists gracefully before
	// we enforce the two-element constraint.
	var raw struct {
		Version      string            `yaml:"version"`
		Prefixes     map[string]string `yaml:"prefixes"`
		Hierarchies  []HierarchyDef    `yaml:"hierarchies"`
		Equivalences []rawEquivalence  `yaml:"equivalences"`
		IFPs         []IFPDef          `yaml:"ifps"`
	}

	if err := yaml.Unmarshal(data, &raw); err != nil {
		return Ontology{}, fmt.Errorf("parse ontology yaml: %w", err)
	}

	if raw.Version == "" {
		return Ontology{}, errors.New("ontology: version field is required")
	}

	// Convert raw equivalences to typed pairs, enforcing exactly two elements.
	pairs := make([][2]string, 0, len(raw.Equivalences))
	for i, eq := range raw.Equivalences {
		if len(eq) != 2 {
			return Ontology{}, fmt.Errorf("ontology: equivalence[%d] must have exactly 2 IRIs, got %d", i, len(eq))
		}
		pairs = append(pairs, [2]string{eq[0], eq[1]})
	}

	o := Ontology{
		Version:      raw.Version,
		Prefixes:     raw.Prefixes,
		Hierarchies:  raw.Hierarchies,
		Equivalences: pairs,
		IFPs:         raw.IFPs,
	}

	return o, nil
}

// isValidIRI reports whether s is a syntactically valid prefix:localname IRI.
// Both the prefix and localname must be non-empty and free of whitespace.
// This is intentionally lenient about localname characters to accommodate
// a wide range of external vocabulary identifiers (e.g. "mitre:T1190.001").
func isValidIRI(s string) bool {
	if s == "" {
		return false
	}
	idx := strings.IndexByte(s, ':')
	if idx <= 0 {
		return false
	}
	prefix := s[:idx]
	local := s[idx+1:]
	if local == "" {
		return false
	}
	// Neither part may contain whitespace.
	if strings.ContainsAny(prefix, " \t\n\r") || strings.ContainsAny(local, " \t\n\r") {
		return false
	}
	return true
}

// iriPrefix extracts the prefix portion from a prefix:localname IRI.
// The caller must ensure isValidIRI(s) returns true before calling this.
func iriPrefix(s string) string {
	return s[:strings.IndexByte(s, ':')]
}
