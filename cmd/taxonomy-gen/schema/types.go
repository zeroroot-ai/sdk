// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Zero Root AI

// Package schema defines the Go types for parsing taxonomy YAML files.
package schema

import (
	"sort"
	"strings"
)

// Taxonomy represents the complete taxonomy definition parsed from YAML.
type Taxonomy struct {
	Version string `yaml:"version"`
	Kind    string `yaml:"kind"`              // "core" or "extension"
	Extends string `yaml:"extends,omitempty"` // Base taxonomy reference (extensions only)

	NodeTypes         []NodeType         `yaml:"node_types"`
	RelationshipTypes []RelationshipType `yaml:"relationship_types"`
	Techniques        []Technique        `yaml:"techniques,omitempty"`
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

// GoType returns the Go type for this nested-type field.
func (f *NestedTypeField) GoType() string {
	switch f.Type {
	case "string":
		return "string"
	case "int32":
		return "int32"
	case "int64":
		return "int64"
	case "float64":
		return "float64"
	case "bool":
		return "bool"
	case "timestamp":
		return "int64"
	case "bytes":
		return "[]byte"
	default:
		return "any"
	}
}

// ProtoType returns the protobuf type for this nested-type field.
func (f *NestedTypeField) ProtoType() string {
	switch f.Type {
	case "string":
		return "string"
	case "int32":
		return "int32"
	case "int64":
		return "int64"
	case "float64":
		return "double"
	case "bool":
		return "bool"
	case "timestamp":
		return "int64"
	case "bytes":
		return "bytes"
	default:
		return "string"
	}
}

// SortedNestedTypeNames returns the nested type names in sorted order for
// deterministic code generation.
func (t *Taxonomy) SortedNestedTypeNames() []string {
	names := make([]string, 0, len(t.NestedTypes))
	for name := range t.NestedTypes {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

// NodeType represents a node type definition in the taxonomy.
type NodeType struct {
	Name                  string           `yaml:"name"`
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
	// resolution (ADR-0002): identity fields are compared, volatile fields are
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
type RelationshipType struct {
	Name        string     `yaml:"name"`
	Description string     `yaml:"description,omitempty"`
	FromTypes   []string   `yaml:"from_types"`
	ToTypes     []string   `yaml:"to_types"`    // ["*"] for any
	Cardinality string     `yaml:"cardinality"` // one_to_one, one_to_many, many_to_many
	Properties  []Property `yaml:"properties,omitempty"`
}

// Technique represents an attack technique (MITRE/Gibson).
type Technique struct {
	ID          string `yaml:"id"` // e.g., "GIB-T1001"
	Name        string `yaml:"name"`
	Taxonomy    string `yaml:"taxonomy"` // mitre_attack, mitre_atlas, gibson
	Tactic      string `yaml:"tactic"`
	Description string `yaml:"description,omitempty"`
	URL         string `yaml:"url,omitempty"`
}

// HasRequiredParent returns true if this node type requires a parent.
func (n *NodeType) HasRequiredParent() bool {
	return n.Parent != nil && n.Parent.Required
}

// HasParent returns true if this node type has any parent configuration.
func (n *NodeType) HasParent() bool {
	return n.Parent != nil
}

// IsRoot returns true if this node type is a root node (no parent).
func (n *NodeType) IsRoot() bool {
	return n.Parent == nil
}

// RequiredProperties returns only the required properties.
func (n *NodeType) RequiredProperties() []Property {
	var required []Property
	for _, p := range n.Properties {
		if p.Required {
			required = append(required, p)
		}
	}
	return required
}

// OptionalProperties returns only the optional properties.
func (n *NodeType) OptionalProperties() []Property {
	var optional []Property
	for _, p := range n.Properties {
		if !p.Required {
			optional = append(optional, p)
		}
	}
	return optional
}

// PropertyByName returns a property by name, or nil if not found.
func (n *NodeType) PropertyByName(name string) *Property {
	for i := range n.Properties {
		if n.Properties[i].Name == name {
			return &n.Properties[i]
		}
	}
	return nil
}

// IsMap returns true if the property type is a map type.
func (p *Property) IsMap() bool {
	return p.Type == "map<string,string>"
}

// HasReservedKeys returns true if the property has any reserved key definitions.
func (p *Property) HasReservedKeys() bool {
	return len(p.ReservedKeys) > 0
}

// SortedReservedKeys returns the reserved keys in sorted (deterministic) order.
// This ensures that generator output is byte-identical across runs.
func (p *Property) SortedReservedKeys() []string {
	keys := make([]string, 0, len(p.ReservedKeys))
	for k := range p.ReservedKeys {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

// IsTimestamp returns true if the property type is timestamp.
func (p *Property) IsTimestamp() bool {
	return p.Type == "timestamp"
}

// IsNumeric returns true if the property type is numeric.
func (p *Property) IsNumeric() bool {
	return p.Type == "int32" || p.Type == "int64" || p.Type == "float64"
}

// IsString returns true if the property type is string.
func (p *Property) IsString() bool {
	return p.Type == "string"
}

// IsBool returns true if the property type is bool.
func (p *Property) IsBool() bool {
	return p.Type == "bool"
}

// IsBytes returns true if the property type is bytes.
func (p *Property) IsBytes() bool {
	return p.Type == "bytes"
}

// HasEnum returns true if the property has enum values.
func (p *Property) HasEnum() bool {
	return len(p.Enum) > 0
}

// GoType returns the Go type for this property type.
func (p *Property) GoType() string {
	switch p.Type {
	case "string":
		return "string"
	case "int32":
		return "int32"
	case "int64":
		return "int64"
	case "float64":
		return "float64"
	case "bool":
		return "bool"
	case "timestamp":
		return "int64"
	case "bytes":
		return "[]byte"
	default:
		// Handle list<TypeName> — e.g. list<ComplianceMapping>, list<string>
		if strings.HasPrefix(p.Type, "list<") && strings.HasSuffix(p.Type, ">") {
			inner := p.Type[len("list<") : len(p.Type)-1]
			// Primitive types get a plain slice; message types get a pointer slice.
			switch inner {
			case "string":
				return "[]string"
			case "int32":
				return "[]int32"
			case "int64":
				return "[]int64"
			case "float64":
				return "[]float64"
			case "bool":
				return "[]bool"
			default:
				// Named message / nested type
				return "[]*" + inner
			}
		}
		return "any"
	}
}

// ProtoType returns the protobuf type for this property type.
// For list<TypeName> properties this returns only the element type; the
// "repeated" keyword is emitted by the template separately via IsListType().
func (p *Property) ProtoType() string {
	switch p.Type {
	case "string":
		return "string"
	case "int32":
		return "int32"
	case "int64":
		return "int64"
	case "float64":
		return "double"
	case "bool":
		return "bool"
	case "timestamp":
		return "int64"
	case "bytes":
		return "bytes"
	default:
		// Handle list<TypeName> — return the inner message type name.
		if strings.HasPrefix(p.Type, "list<") && strings.HasSuffix(p.Type, ">") {
			return p.Type[len("list<") : len(p.Type)-1]
		}
		return "string"
	}
}

// IsListType returns true when this property holds a repeated/list value.
func (p *Property) IsListType() bool {
	return strings.HasPrefix(p.Type, "list<") && strings.HasSuffix(p.Type, ">")
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

	return nil
}

// NodeTypeByName returns a node type by name, or nil if not found.
func (t *Taxonomy) NodeTypeByName(name string) *NodeType {
	for i := range t.NodeTypes {
		if t.NodeTypes[i].Name == name {
			return &t.NodeTypes[i]
		}
	}
	return nil
}

// RelationshipTypeByName returns a relationship type by name, or nil if not found.
func (t *Taxonomy) RelationshipTypeByName(name string) *RelationshipType {
	for i := range t.RelationshipTypes {
		if t.RelationshipTypes[i].Name == name {
			return &t.RelationshipTypes[i]
		}
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
