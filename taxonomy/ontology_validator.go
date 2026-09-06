// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Zero Root AI

package taxonomy

import (
	"errors"
	"fmt"
	"strings"
)

// Validate performs semantic validation on a parsed Ontology.
//
// Checks performed:
//   - All prefixes used in IRI fields are declared in the Prefixes map.
//   - All IRI fields are syntactically valid (prefix:localname form).
//   - No subClassOf cycle exists in the hierarchy graph.
//   - IFP property declarations reference node types that appear somewhere in
//     the hierarchy (an IFP for an undeclared node type is likely a typo).
//   - Equivalence pairs are each syntactically valid IRIs with declared prefixes.
//
// TODO(ontology-extension-system): Add richer consistency checks — for example,
// detecting equivalence chains that contradict known disjointness constraints.
// This initial cut enforces cycle freedom and prefix resolution only, which is
// sufficient for the launch milestone. A future PR will add the reasoner-side
// closure check once the daemon Reasoner implementation ships.
func (o *Ontology) Validate() error {
	if o.Version == "" {
		return errors.New("ontology: version field is required")
	}

	// Build the set of declared prefixes for O(1) lookup.
	declaredPrefixes := make(map[string]bool, len(o.Prefixes))
	for p := range o.Prefixes {
		declaredPrefixes[p] = true
	}

	// Collect node types seen in hierarchies for IFP cross-reference.
	hierarchyNodeTypes := make(map[string]bool)

	// Validate hierarchy entries.
	for i, h := range o.Hierarchies {
		if h.NodeType == "" {
			return fmt.Errorf("ontology: hierarchy[%d]: node_type is required", i)
		}
		if h.Label == "" {
			return fmt.Errorf("ontology: hierarchy[%d]: label is required", i)
		}
		if !isValidIRI(h.Label) {
			return fmt.Errorf("ontology: hierarchy[%d]: label %q is not a valid IRI (expected prefix:localname)", i, h.Label)
		}
		if err := checkPrefixDeclared(h.Label, declaredPrefixes); err != nil {
			return fmt.Errorf("ontology: hierarchy[%d]: label %w", i, err)
		}
		if h.SubClassOf != "" {
			if !isValidIRI(h.SubClassOf) {
				return fmt.Errorf("ontology: hierarchy[%d]: sub_class_of %q is not a valid IRI (expected prefix:localname)", i, h.SubClassOf)
			}
			if err := checkPrefixDeclared(h.SubClassOf, declaredPrefixes); err != nil {
				return fmt.Errorf("ontology: hierarchy[%d]: sub_class_of %w", i, err)
			}
		}
		hierarchyNodeTypes[h.NodeType] = true
	}

	// Detect subClassOf cycles.
	if err := detectCycles(o.Hierarchies); err != nil {
		return err
	}

	// Validate equivalence pairs.
	for i, eq := range o.Equivalences {
		for j, iri := range eq {
			if !isValidIRI(iri) {
				return fmt.Errorf("ontology: equivalence[%d][%d]: %q is not a valid IRI", i, j, iri)
			}
			if err := checkPrefixDeclared(iri, declaredPrefixes); err != nil {
				return fmt.Errorf("ontology: equivalence[%d][%d]: %w", i, j, err)
			}
		}
	}

	// Validate IFP declarations.
	for i, ifp := range o.IFPs {
		if ifp.NodeType == "" {
			return fmt.Errorf("ontology: ifps[%d]: node_type is required", i)
		}
		if ifp.Property == "" {
			return fmt.Errorf("ontology: ifps[%d]: property is required", i)
		}
		// An IFP must reference a node_type that appears in at least one
		// hierarchy entry, or the ontology is incomplete / contains a typo.
		if len(o.Hierarchies) > 0 && !hierarchyNodeTypes[ifp.NodeType] {
			return fmt.Errorf("ontology: ifps[%d]: node_type %q is not referenced by any hierarchy entry", i, ifp.NodeType)
		}
	}

	return nil
}

// checkPrefixDeclared returns an error if the prefix extracted from iri is not
// in the declared map. It is a no-op when declared is empty (the caller has no
// prefixes to check against).
func checkPrefixDeclared(iri string, declared map[string]bool) error {
	if len(declared) == 0 {
		return nil
	}
	p := iriPrefix(iri)
	if !declared[p] {
		return fmt.Errorf("prefix %q in IRI %q is not declared in the prefixes map", p, iri)
	}
	return nil
}

// detectCycles walks the hierarchy graph and returns an error if any
// subClassOf cycle is found. The detection uses iterative DFS with a
// per-node colour (white/grey/black) to identify back edges.
//
// Only label→subClassOf edges within the same node_type are followed;
// cross-type relationships are not considered a cycle.
func detectCycles(hierarchies []HierarchyDef) error {
	// Build adjacency: for each (nodeType, label) → subClassOf parent IRI.
	// Key is "nodeType\x00label" to namespace per node type.
	type key = string
	parent := make(map[key]string)
	for _, h := range hierarchies {
		if h.SubClassOf == "" {
			continue
		}
		k := h.NodeType + "\x00" + h.Label
		parent[k] = h.NodeType + "\x00" + h.SubClassOf
	}

	// Collect all nodes (labels that appear in the hierarchy).
	nodes := make(map[key]bool)
	for _, h := range hierarchies {
		nodes[h.NodeType+"\x00"+h.Label] = true
		if h.SubClassOf != "" {
			nodes[h.NodeType+"\x00"+h.SubClassOf] = true
		}
	}

	const (
		white = 0
		grey  = 1
		black = 2
	)
	colour := make(map[key]int, len(nodes))

	var visit func(node key) error
	visit = func(node key) error {
		colour[node] = grey
		if p, ok := parent[node]; ok {
			switch colour[p] {
			case grey:
				// Extract readable IRI from the compound key for the error message.
				iri := p[strings.IndexByte(p, '\x00')+1:]
				return fmt.Errorf("ontology: subClassOf cycle detected involving %q", iri)
			case white:
				if err := visit(p); err != nil {
					return err
				}
			}
		}
		colour[node] = black
		return nil
	}

	for node := range nodes {
		if colour[node] == white {
			if err := visit(node); err != nil {
				return err
			}
		}
	}
	return nil
}
