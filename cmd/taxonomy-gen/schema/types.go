// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Zero Root AI

// Package schema defines the Go types for parsing taxonomy YAML files.
package schema

import (
	"fmt"
	"strings"
)

// Taxonomy represents the complete taxonomy definition parsed from YAML.
type Taxonomy struct {
	Version string `yaml:"version"`
	Kind    string `yaml:"kind"`              // "core" or "extension"
	Extends string `yaml:"extends,omitempty"` // Base taxonomy reference (extensions only)

	NodeTypes         []NodeType         `yaml:"node_types"`
	RelationshipTypes []RelationshipType `yaml:"relationship_types"`
	// NestedTypes holds embedded value-object types (e.g. ComplianceMapping).
	// These are not graph nodes — they have no identifying_properties, no parent
	// relationships, and do not get domain constructors. They exist solely so
	// that other node types can reference them as field types (e.g. list<ComplianceMapping>).
	NestedTypes map[string]NestedType `yaml:"nested_types,omitempty"`
}

// NestedType is a simple value-object type declared under nested_types:.
// It has a flat list of fields but no graph-node semantics.
type NestedType struct {
	Fields []NestedTypeField `yaml:"fields"`
}

// NestedTypeField is a single field within a NestedType.
type NestedTypeField struct {
	Name        string `yaml:"name"`
	Type        string `yaml:"type"`
	Required    bool   `yaml:"required,omitempty"`
	Description string `yaml:"description,omitempty"`
}

// NodeType represents a node type definition in the taxonomy.
//
// Number is the CoreNodeType enum value, and it is DECLARED, not derived from
// position in the file. It used to be the index plus one, which made the order
// of core.yaml part of the wire contract: deleting a node type renumbered every
// type after it, and inserting one in the middle did the same. Three live types
// moved when compliance_signal was deleted (scope 18->17, credential 19->18,
// account 20->19). Nothing errored. The drift gate did notice the regenerated
// proto differed, but its message says to commit the regenerated output, so it
// turned a silent wire break into a prompted one.
//
// Retired takes a type out of service while HOLDING its number. The enum emits
// `reserved <number>;` and `reserved "<NAME>";` instead of a member, which is
// proto's own mechanism for a value that must never be reused, and no message,
// helper, validator or query binding is generated. A retired type stays in
// core.yaml: that is what reserves the number, and deleting the entry would
// free the number for the next type to reuse.
type NodeType struct {
	Name                  string           `yaml:"name"`
	Number                int              `yaml:"number"`
	Retired               bool             `yaml:"retired,omitempty"`
	Category              string           `yaml:"category"` // execution, asset, finding, attack
	Description           string           `yaml:"description,omitempty"`
	Properties            []Property       `yaml:"properties,omitempty"`
	Parent                *ParentConfig    `yaml:"parent,omitempty"`
	IdentifyingProperties []string         `yaml:"identifying_properties,omitempty"`
	Validations           []ValidationRule `yaml:"validations,omitempty"`
}

// Property represents a property on a node or relationship type.
type Property struct {
	Name        string   `yaml:"name"`
	Type        string   `yaml:"type"` // string, int32, int64, float64, bool, timestamp, bytes, map<string,string>, list<string>
	Required    bool     `yaml:"required,omitempty"`
	Enum        []string `yaml:"enum,omitempty"`
	Description string   `yaml:"description,omitempty"`
	// Volatile marks a property as mutable runtime state (e.g. port state, last-seen)
	// rather than stable identity. Identity-vs-volatile drives the brain's entity
	// resolution (ADR-0102): identity fields are compared, volatile fields are
	// updated-on-match and never compared. Not emitted into the proto; consumed by
	// the ark-component codegen.
	Volatile bool `yaml:"volatile,omitempty"`
	// ReservedKeys defines closed-vocabulary constraints for named keys within
	// map-typed properties. Each entry maps a key name to its allowed values and
	// optional description. Non-reserved keys are unconstrained (free-form).
	// Only meaningful when Type is "map<string,string>".
	ReservedKeys map[string]ReservedKeyDef `yaml:"reserved_keys,omitempty"`
}

// ReservedKeyDef defines the closed vocabulary and documentation for a single
// reserved key within a map-typed property.
type ReservedKeyDef struct {
	// ClosedVocabulary lists the allowed values for this key.
	// A signal whose map contains this key with any other value fails validation.
	ClosedVocabulary []string `yaml:"closed_vocabulary"`
	// Description is human-readable documentation for this reserved key.
	Description string `yaml:"description,omitempty"`
}

// ParentConfig defines the parent relationship for a node type.
type ParentConfig struct {
	Type         string `yaml:"type"`         // Parent node type
	RefField     string `yaml:"ref_field"`    // Field on child holding parent ID (e.g., host_id)
	Relationship string `yaml:"relationship"` // Relationship name (e.g., HAS_PORT)
	Required     bool   `yaml:"required"`     // Must have parent?
}

// ValidationRule represents a CEL validation rule.
type ValidationRule struct {
	Rule    string `yaml:"rule"`    // CEL expression
	Message string `yaml:"message"` // Error message on failure
}

// RelationshipType represents a relationship type definition.
// RelationshipType represents a relationship type definition.
//
// Number and Retired work exactly as they do on NodeType, and for the same
// reason: CoreRelationType was numbered by position too, so the hazard was
// identical. EMITTED_SIGNAL is the relationship compliance_signal used.
type RelationshipType struct {
	Name        string     `yaml:"name"`
	Number      int        `yaml:"number"`
	Retired     bool       `yaml:"retired,omitempty"`
	Description string     `yaml:"description,omitempty"`
	FromTypes   []string   `yaml:"from_types"`
	ToTypes     []string   `yaml:"to_types"`    // ["*"] for any
	Cardinality string     `yaml:"cardinality"` // one_to_one, one_to_many, many_to_many
	Properties  []Property `yaml:"properties,omitempty"`
}

// Validate performs basic validation on the taxonomy.
func (t *Taxonomy) Validate() error {
	if t.Version == "" {
		return &ValidationError{Field: "version", Message: "version is required"}
	}
	if t.Kind == "" {
		return &ValidationError{Field: "kind", Message: "kind is required"}
	}
	if t.Kind != "core" && t.Kind != "extension" {
		return &ValidationError{Field: "kind", Message: "kind must be 'core' or 'extension'"}
	}
	if t.Kind == "extension" && t.Extends == "" {
		return &ValidationError{Field: "extends", Message: "extensions must specify 'extends'"}
	}
	if len(t.NodeTypes) == 0 {
		return &ValidationError{Field: "node_types", Message: "at least one node type is required"}
	}

	// Validate each node type
	nodeNames := make(map[string]bool)
	for _, nt := range t.NodeTypes {
		if nt.Name == "" {
			return &ValidationError{Field: "node_types[].name", Message: "node type name is required"}
		}
		if nodeNames[nt.Name] {
			return &ValidationError{Field: "node_types[].name", Message: "duplicate node type: " + nt.Name}
		}
		nodeNames[nt.Name] = true

		// Validate properties
		propNames := make(map[string]bool)
		for _, p := range nt.Properties {
			if p.Name == "" {
				return &ValidationError{Field: "node_types[" + nt.Name + "].properties[].name", Message: "property name is required"}
			}
			if p.Type == "" {
				return &ValidationError{Field: "node_types[" + nt.Name + "].properties[" + p.Name + "].type", Message: "property type is required"}
			}
			if propNames[p.Name] {
				return &ValidationError{Field: "node_types[" + nt.Name + "].properties[].name", Message: "duplicate property: " + p.Name}
			}
			propNames[p.Name] = true
		}
	}

	// Validate relationship types
	relNames := make(map[string]bool)
	for _, rt := range t.RelationshipTypes {
		if rt.Name == "" {
			return &ValidationError{Field: "relationship_types[].name", Message: "relationship type name is required"}
		}
		if relNames[rt.Name] {
			return &ValidationError{Field: "relationship_types[].name", Message: "duplicate relationship type: " + rt.Name}
		}
		relNames[rt.Name] = true
	}

	// Enum numbers last, so a missing name is reported as a missing name rather
	// than as "no number: ". An extension has its own enum space and is not
	// numbered, so this applies to the core taxonomy only.
	if t.Kind == "core" {
		if err := validateEnumNumbers(t); err != nil {
			return err
		}
	}
	if err := validateRetiredReferences(t); err != nil {
		return err
	}

	return nil
}

// ValidationError represents a validation error.
type ValidationError struct {
	Field   string
	Message string
}

func (e *ValidationError) Error() string {
	return e.Field + ": " + e.Message
}

// validateEnumNumbers checks the declared CoreNodeType and CoreRelationType
// values. The three rules exist because each one, left unchecked, silently
// changes a wire value:
//
//	missing    the generator would have to pick a number, which is how
//	           position became load-bearing in the first place
//	duplicate  two names would share one wire value
//	< 1        0 is CORE_*_UNSPECIFIED and negative is not a valid enum value
//
// A retired type keeps its number. That is the point of retiring: the number is
// reserved so nothing can reuse it.
func validateEnumNumbers(t *Taxonomy) error {
	type entry struct {
		field string
		name  string
		num   int
	}
	check := func(field string, entries []entry) error {
		var missing []string
		seen := map[int]string{}
		for _, e := range entries {
			if e.num == 0 {
				missing = append(missing, e.name)
				continue
			}
			if e.num < 0 {
				return &ValidationError{
					Field:   field,
					Message: fmt.Sprintf("%s has number %d: an enum value must be 1 or more, because 0 is the UNSPECIFIED member", e.name, e.num),
				}
			}
			if other, dup := seen[e.num]; dup {
				return &ValidationError{
					Field: field,
					Message: fmt.Sprintf("%s and %s both declare number %d, so they would share one wire value. "+
						"A retired entry keeps its number: pick the next free one instead of reusing it", other, e.name, e.num),
				}
			}
			seen[e.num] = e.name
		}
		if len(missing) > 0 {
			next := 1
			for seen[next] != "" {
				next++
			}
			return &ValidationError{
				Field: field,
				Message: fmt.Sprintf("%s no number: %s. "+
					"Every entry declares its own enum value, so that position in this file is not part of the wire contract. "+
					"The lowest free number is %d", pluralEntries(len(missing)), strings.Join(missing, ", "), next),
			}
		}
		return nil
	}

	nodes := make([]entry, 0, len(t.NodeTypes))
	for _, nt := range t.NodeTypes {
		nodes = append(nodes, entry{field: "node_types[].number", name: nt.Name, num: nt.Number})
	}
	if err := check("node_types[].number", nodes); err != nil {
		return err
	}

	rels := make([]entry, 0, len(t.RelationshipTypes))
	for _, rt := range t.RelationshipTypes {
		rels = append(rels, entry{field: "relationship_types[].number", name: rt.Name, num: rt.Number})
	}
	return check("relationship_types[].number", rels)
}

func pluralEntries(n int) string {
	if n == 1 {
		return "one entry declares"
	}
	return fmt.Sprintf("%d entries declare", n)
}

// Retire splits a taxonomy into the part that is generated and the part that is
// only reserved.
//
// live holds every entry that is still in service. Every generator except the
// proto enum takes live, so a retired type produces no message, no graphrag
// helper, no validator, no constant and no query binding. Nothing can construct
// one.
//
// retiredNodes and retiredRels hold the retired entries, which the proto enum
// turns into `reserved <number>;` and `reserved "<NAME>";`.
//
// The split happens once, here, rather than as a condition inside each of the
// twenty-odd templates that range over these lists. A condition per template is
// a condition one template can be missing, and a missing one emits a helper for
// a type nothing should be able to build.
func (t *Taxonomy) Retire() (live *Taxonomy, retiredNodes []NodeType, retiredRels []RelationshipType) {
	live = &Taxonomy{}
	*live = *t
	live.NodeTypes = make([]NodeType, 0, len(t.NodeTypes))
	live.RelationshipTypes = make([]RelationshipType, 0, len(t.RelationshipTypes))

	for _, nt := range t.NodeTypes {
		if nt.Retired {
			retiredNodes = append(retiredNodes, nt)
			continue
		}
		live.NodeTypes = append(live.NodeTypes, nt)
	}
	for _, rt := range t.RelationshipTypes {
		if rt.Retired {
			retiredRels = append(retiredRels, rt)
			continue
		}
		live.RelationshipTypes = append(live.RelationshipTypes, rt)
	}
	return live, retiredNodes, retiredRels
}

// validateRetiredReferences refuses a live relationship type whose from_types or
// to_types name a retired node type.
//
// Retiring compliance_signal left TRIGGERED live with from_types
// [compliance_signal], so the generator emitted a query traversal from a node
// nothing can construct. It matched nothing and said nothing. Either the
// relationship is retired with the node, or the node is still in service.
func validateRetiredReferences(t *Taxonomy) error {
	retired := map[string]bool{}
	for _, nt := range t.NodeTypes {
		if nt.Retired {
			retired[nt.Name] = true
		}
	}
	if len(retired) == 0 {
		return nil
	}

	for _, rt := range t.RelationshipTypes {
		if rt.Retired {
			continue
		}
		for _, group := range []struct {
			field string
			types []string
		}{
			{"from_types", rt.FromTypes},
			{"to_types", rt.ToTypes},
		} {
			var dead []string
			live := 0
			for _, name := range group.types {
				switch {
				case retired[name]:
					dead = append(dead, name)
				case name == "*":
					// A wildcard is not a reference to any one type.
					live++
				default:
					live++
				}
			}
			if len(dead) == 0 {
				continue
			}
			if live == 0 {
				return &ValidationError{
					Field: "relationship_types[" + rt.Name + "]." + group.field,
					Message: fmt.Sprintf("every %s entry is retired (%s), so no %s edge can exist. "+
						"Retire %s as well, or put the node type back in service",
						group.field, strings.Join(dead, ", "), rt.Name, rt.Name),
				}
			}
			return &ValidationError{
				Field: "relationship_types[" + rt.Name + "]." + group.field,
				Message: fmt.Sprintf("names the retired node type %s. "+
					"Remove it from %s: a live relationship must not reference a type nothing can construct",
					strings.Join(dead, ", "), group.field),
			}
		}
	}
	return nil
}
