// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Zero Root AI

package generator

import (
	"bytes"
	"fmt"
	"os"
	"strings"
	"text/template"

	"github.com/zeroroot-ai/sdk/cmd/taxonomy-gen/schema"
)

// GenerateDomain generates Go domain types from the taxonomy.
func GenerateDomain(taxonomy *schema.Taxonomy, outputPath, pkgName string) error {
	tmpl, err := template.New("domain").Funcs(domainFuncMap()).Parse(domainTemplate)
	if err != nil {
		return fmt.Errorf("failed to parse template: %w", err)
	}

	data := struct {
		*schema.Taxonomy
		Package    string
		SourcePath string
	}{
		Taxonomy:   taxonomy,
		Package:    pkgName,
		SourcePath: "taxonomy/core.yaml",
	}

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, data); err != nil {
		return fmt.Errorf("failed to execute template: %w", err)
	}

	if err := os.WriteFile(outputPath, buf.Bytes(), 0644); err != nil {
		return fmt.Errorf("failed to write file: %w", err)
	}

	return nil
}

func domainFuncMap() template.FuncMap {
	return template.FuncMap{
		"add":            func(a, b int) int { return a + b },
		"toPascalCase":   toPascalCase,
		"toLowerSnake":   toLowerSnake,
		"toUpperSnake":   toUpperSnake,
		"toCamelCase":    toCamelCase,
		"escapeKeyword":  escapeKeyword,
		"hasParent":      func(nt schema.NodeType) bool { return nt.Parent != nil },
		"hasValidations": func(nt schema.NodeType) bool { return len(nt.Validations) > 0 },
		"join":           strings.Join,
		"toGoComment":    toGoComment,
		// parentRefField returns the snake_case name of the field that holds the parent ID.
		// When ref_field names an existing property, use ref_field; otherwise use "parent_{type}_id".
		"parentRefField": func(nt schema.NodeType) string {
			if nt.Parent == nil {
				return ""
			}
			// Check if ref_field names an existing property.
			if nt.Parent.RefField != "" {
				for _, p := range nt.Properties {
					if p.Name == nt.Parent.RefField {
						return nt.Parent.RefField
					}
				}
			}
			return "parent_" + nt.Parent.Type + "_id"
		},
		// parentRefProtoField returns the PascalCase proto field name for the parent ref.
		// When the parent ref is an explicit property (ref_field names an existing property),
		// we use toPascalCase(ref_field). Otherwise we use the auto-generated Parent{Type}Id name.
		"parentRefProtoField": func(nt schema.NodeType) string {
			if nt.Parent == nil {
				return ""
			}
			// Check if ref_field names an existing property (explicit ref).
			if nt.Parent.RefField != "" {
				for _, p := range nt.Properties {
					if p.Name == nt.Parent.RefField {
						// The explicit property holds the parent ref.
						return toPascalCase(nt.Parent.RefField)
					}
				}
			}
			// No explicit property: use the auto-generated parent_{type}_id field.
			return "Parent" + toPascalCase(nt.Parent.Type) + "Id"
		},
		// parentRefIsExplicitProperty returns true when the parent ref field is already
		// declared as an explicit property in the node type (and thus already handled
		// by the Properties() loop — no extra emit needed).
		// This happens when ref_field names a property that exists in the node's properties list.
		"parentRefIsExplicitProperty": func(nt schema.NodeType) bool {
			if nt.Parent == nil {
				return false
			}
			// Only consider explicit if ref_field is set AND that exact property name exists.
			if nt.Parent.RefField == "" {
				return false
			}
			for _, p := range nt.Properties {
				if p.Name == nt.Parent.RefField {
					return true
				}
			}
			// ref_field is set but the property doesn't exist — fall back to auto-generated.
			return false
		},
		// isNestedListProp returns true for list<SomePascalType> properties (non-primitive element).
		"isNestedListProp": func(p schema.Property) bool {
			if !strings.HasPrefix(p.Type, "list<") || !strings.HasSuffix(p.Type, ">") {
				return false
			}
			inner := p.Type[5 : len(p.Type)-1]
			switch inner {
			case "string", "int32", "int64", "float64", "bool":
				return false
			}
			return true
		},
		// isMapProp returns true for map<...> properties.
		"isMapProp": func(p schema.Property) bool {
			return strings.HasPrefix(p.Type, "map<")
		},
		// protoListElemType returns the proto slice type for a nested list property,
		// e.g. "[]*taxonomypb.ComplianceMapping".
		"protoListElemType": func(p schema.Property) string {
			if !strings.HasPrefix(p.Type, "list<") || !strings.HasSuffix(p.Type, ">") {
				return p.GoType()
			}
			inner := p.Type[5 : len(p.Type)-1]
			return "[]*taxonomypb." + inner
		},
	}
}

// toGoComment formats a possibly multi-line description string as Go comment lines.
// Each line of the input is prefixed with "// ". Trailing newlines are stripped.
func toGoComment(s string) string {
	s = strings.TrimRight(s, "\n\r")
	lines := strings.Split(s, "\n")
	var out []string
	for _, l := range lines {
		out = append(out, "// "+l)
	}
	return strings.Join(out, "\n")
}

// goKeywords is a set of Go reserved keywords that need escaping when used as identifiers.
var goKeywords = map[string]bool{
	"break": true, "case": true, "chan": true, "const": true, "continue": true,
	"default": true, "defer": true, "else": true, "fallthrough": true, "for": true,
	"func": true, "go": true, "goto": true, "if": true, "import": true,
	"interface": true, "map": true, "package": true, "range": true, "return": true,
	"select": true, "struct": true, "switch": true, "type": true, "var": true,
}

// escapeKeyword escapes Go keywords by appending an underscore.
func escapeKeyword(s string) string {
	if goKeywords[s] {
		return s + "_"
	}
	return s
}

// toCamelCase converts a string to camelCase.
// e.g., "host_id" -> "hostId", "mission_run" -> "missionRun"
func toCamelCase(s string) string {
	pascal := toPascalCase(s)
	if len(pascal) > 0 {
		return strings.ToLower(pascal[:1]) + pascal[1:]
	}
	return pascal
}

const domainTemplate = `// Code generated by taxonomy-gen from taxonomy YAML. DO NOT EDIT.
// Source: {{.SourcePath}}
// Taxonomy Version: {{.Version}}

package {{.Package}}

import (
	"fmt"

	taxonomypb "github.com/zeroroot-ai/sdk/api/gen/taxonomy/v1"
	"github.com/zeroroot-ai/sdk/graphrag/validation"
)

// ==================== INTERFACES ====================

// GraphNode is implemented by all domain types.
type GraphNode interface {
	// NodeType returns the taxonomy node type string.
	NodeType() string

	// Properties returns all properties as a map.
	Properties() map[string]any

	// IdentifyingProperties returns the natural key properties.
	IdentifyingProperties() map[string]any

	// ParentRef returns the parent reference, or nil for root nodes.
	ParentRef() *NodeRef

	// Validate runs CEL validation rules. Returns nil for custom types.
	Validate() error

	// ToProto converts to the generic GraphNode proto.
	ToProto() *taxonomypb.GraphNode

	// ID returns the node ID (may be empty until stored).
	ID() string

	// SetID sets the node ID (called by harness after storage).
	SetID(id string)
}

// NodeRef references a parent node.
type NodeRef struct {
	NodeType     string
	Properties   map[string]any
	Relationship string
}

// ==================== HELPER FUNCTIONS ====================

func propsToValueMap(props map[string]any) map[string]*taxonomypb.Value {
	result := make(map[string]*taxonomypb.Value, len(props))
	for k, v := range props {
		result[k] = anyToValue(v)
	}
	return result
}

func anyToValue(v any) *taxonomypb.Value {
	if v == nil {
		return nil
	}
	switch val := v.(type) {
	case string:
		return &taxonomypb.Value{Kind: &taxonomypb.Value_StringValue{StringValue: val}}
	case int:
		return &taxonomypb.Value{Kind: &taxonomypb.Value_IntValue{IntValue: int64(val)}}
	case int32:
		return &taxonomypb.Value{Kind: &taxonomypb.Value_IntValue{IntValue: int64(val)}}
	case int64:
		return &taxonomypb.Value{Kind: &taxonomypb.Value_IntValue{IntValue: val}}
	case float64:
		return &taxonomypb.Value{Kind: &taxonomypb.Value_DoubleValue{DoubleValue: val}}
	case float32:
		return &taxonomypb.Value{Kind: &taxonomypb.Value_DoubleValue{DoubleValue: float64(val)}}
	case bool:
		return &taxonomypb.Value{Kind: &taxonomypb.Value_BoolValue{BoolValue: val}}
	case []byte:
		return &taxonomypb.Value{Kind: &taxonomypb.Value_BytesValue{BytesValue: val}}
	default:
		return &taxonomypb.Value{Kind: &taxonomypb.Value_StringValue{StringValue: fmt.Sprintf("%v", val)}}
	}
}

// ==================== NESTED VALUE-OBJECT TYPES ====================
// These types are embedded in node properties. They are plain Go structs
// with no GraphNode interface, no constructor, and no CEL validation.
{{- if .NestedTypes}}
{{- range .SortedNestedTypeNames}}
{{- $nt := index $.NestedTypes .}}

// {{.}} is a nested value-object type.
type {{.}} struct {
{{- range $nt.Fields}}
{{- if .Required}}
	{{.Name | toPascalCase}} {{.GoType}} ` + "`" + `json:"{{.Name}}"` + "`" + `
{{- else}}
	{{.Name | toPascalCase}} *{{.GoType}} ` + "`" + `json:"{{.Name}},omitempty"` + "`" + `
{{- end}}
{{- end}}
}
{{- end}}
{{- end}}

// ==================== GENERATED DOMAIN TYPES ====================
{{range .NodeTypes}}
// ==================== {{.Name | toUpperSnake}} ====================

{{toGoComment (printf "%s represents: %s" (.Name | toPascalCase) .Description)}}
type {{.Name | toPascalCase}} struct {
	proto  *taxonomypb.{{.Name | toPascalCase}}
	parent *NodeRef
}

// New{{.Name | toPascalCase}} creates a new {{.Name | toPascalCase}}.
{{- $type := .}}
{{- $requiredProps := .RequiredProperties}}
{{- if gt (len $requiredProps) 0}}
func New{{.Name | toPascalCase}}({{range $i, $p := $requiredProps}}{{if $i}}, {{end}}{{$p.Name | escapeKeyword}} {{$p.GoType}}{{end}}) *{{.Name | toPascalCase}} {
	return &{{.Name | toPascalCase}}{
		proto: &taxonomypb.{{.Name | toPascalCase}}{
{{- range $requiredProps}}
			{{.Name | toPascalCase}}: {{.Name | escapeKeyword}},
{{- end}}
		},
	}
}
{{- else}}
func New{{.Name | toPascalCase}}() *{{.Name | toPascalCase}} {
	return &{{.Name | toPascalCase}}{
		proto: &taxonomypb.{{.Name | toPascalCase}}{},
	}
}
{{- end}}

// NodeType implements GraphNode.
func (n *{{.Name | toPascalCase}}) NodeType() string { return "{{.Name}}" }

// Properties implements GraphNode.
func (n *{{.Name | toPascalCase}}) Properties() map[string]any {
	props := make(map[string]any)
{{- range .Properties}}
{{- if .IsListType}}
	if len(n.proto.{{.Name | toPascalCase}}) > 0 {
		props["{{.Name}}"] = n.proto.{{.Name | toPascalCase}}
	}
{{- else if .Required}}
	props["{{.Name}}"] = n.proto.{{.Name | toPascalCase}}
{{- else}}
	if n.proto.{{.Name | toPascalCase}} != nil {
		props["{{.Name}}"] = *n.proto.{{.Name | toPascalCase}}
	}
{{- end}}
{{- end}}
{{- if .Parent}}
{{- if parentRefIsExplicitProperty .}}{{/* parent ref field is already emitted by properties loop */}}
{{- else}}
	if n.proto.{{parentRefProtoField .}} != "" {
		props["{{parentRefField .}}"] = n.proto.{{parentRefProtoField .}}
	}
{{- end}}
{{- end}}
	return props
}

// IdentifyingProperties implements GraphNode.
func (n *{{.Name | toPascalCase}}) IdentifyingProperties() map[string]any {
	props := make(map[string]any)
{{- range .IdentifyingProperties}}
{{- $prop := $type.PropertyByName .}}
{{- if $prop}}
{{- if $prop.Required}}
	props["{{.}}"] = n.proto.{{. | toPascalCase}}
{{- else}}
	if n.proto.{{. | toPascalCase}} != nil {
		props["{{.}}"] = *n.proto.{{. | toPascalCase}}
	}
{{- end}}
{{- end}}
{{- end}}
	return props
}

// ParentRef implements GraphNode.
func (n *{{.Name | toPascalCase}}) ParentRef() *NodeRef {
{{- if .Parent}}
	if n.parent != nil {
		return n.parent
	}
{{- if parentRefIsExplicitProperty .}}
	if n.proto.{{parentRefProtoField .}} != nil && *n.proto.{{parentRefProtoField .}} != "" {
		return &NodeRef{
			NodeType:     "{{.Parent.Type}}",
			Properties:   map[string]any{"id": *n.proto.{{parentRefProtoField .}}},
			Relationship: "{{.Parent.Relationship}}",
		}
	}
{{- else}}
	if n.proto.{{parentRefProtoField .}} != "" {
		return &NodeRef{
			NodeType:     "{{.Parent.Type}}",
			Properties:   map[string]any{"id": n.proto.{{parentRefProtoField .}}},
			Relationship: "{{.Parent.Relationship}}",
		}
	}
{{- end}}
{{- end}}
	return nil
}

// Validate implements GraphNode.
func (n *{{.Name | toPascalCase}}) Validate() error {
{{- if .Parent}}
{{- if .Parent.Required}}
{{- if parentRefIsExplicitProperty .}}
	if n.parent == nil && (n.proto.{{parentRefProtoField .}} == nil || *n.proto.{{parentRefProtoField .}} == "") {
		return fmt.Errorf("{{.Name}} requires a parent of type {{.Parent.Type}} (use BelongsTo)")
	}
{{- else}}
	if n.parent == nil && n.proto.{{parentRefProtoField .}} == "" {
		return fmt.Errorf("{{.Name}} requires a parent of type {{.Parent.Type}} (use BelongsTo)")
	}
{{- end}}
{{- end}}
{{- end}}
	return validation.Validate{{.Name | toPascalCase}}(n.proto)
}

// ToProto implements GraphNode.
func (n *{{.Name | toPascalCase}}) ToProto() *taxonomypb.GraphNode {
	node := &taxonomypb.GraphNode{
		Id:         n.proto.Id,
		Type:       "{{.Name}}",
		Properties: propsToValueMap(n.Properties()),
	}
	if n.parent != nil {
		node.ParentType = &n.parent.NodeType
		node.ParentRelationship = &n.parent.Relationship
	}
	return node
}

// ID returns the node ID.
func (n *{{.Name | toPascalCase}}) ID() string { return n.proto.Id }

// SetID sets the node ID.
func (n *{{.Name | toPascalCase}}) SetID(id string) { n.proto.Id = id }
{{if .Parent}}
// BelongsTo sets the parent {{.Parent.Type}}.
func (n *{{.Name | toPascalCase}}) BelongsTo(parent *{{.Parent.Type | toPascalCase}}) *{{.Name | toPascalCase}} {
	n.parent = &NodeRef{
		NodeType:     "{{.Parent.Type}}",
		Properties:   parent.IdentifyingProperties(),
		Relationship: "{{.Parent.Relationship}}",
	}
{{- if parentRefIsExplicitProperty .}}
	parentID := parent.ID()
	n.proto.{{parentRefProtoField .}} = &parentID
{{- else}}
	n.proto.{{parentRefProtoField .}} = parent.ID()
{{- end}}
	return n
}
{{end}}
// --- Typed Accessors ---
{{range .Properties}}
{{- if isNestedListProp .}}
// {{.Name | toPascalCase}} returns the {{.Name}} value.
func (n *{{$type.Name | toPascalCase}}) {{.Name | toPascalCase}}() {{protoListElemType .}} {
	return n.proto.{{.Name | toPascalCase}}
}

// Set{{.Name | toPascalCase}} sets the {{.Name}} value.
func (n *{{$type.Name | toPascalCase}}) Set{{.Name | toPascalCase}}(v {{protoListElemType .}}) *{{$type.Name | toPascalCase}} {
	n.proto.{{.Name | toPascalCase}} = v
	return n
}
{{- else if isMapProp .}}
// {{.Name | toPascalCase}} returns the {{.Name}} value as a JSON-serialized string.
// The map is stored as an optional JSON string in the underlying proto message.
func (n *{{$type.Name | toPascalCase}}) {{.Name | toPascalCase}}() string {
	if n.proto.{{.Name | toPascalCase}} != nil {
		return *n.proto.{{.Name | toPascalCase}}
	}
	return ""
}

// Set{{.Name | toPascalCase}} sets the {{.Name}} value from a JSON-serialized string.
func (n *{{$type.Name | toPascalCase}}) Set{{.Name | toPascalCase}}(v string) *{{$type.Name | toPascalCase}} {
	n.proto.{{.Name | toPascalCase}} = &v
	return n
}
{{- else}}
// {{.Name | toPascalCase}} returns the {{.Name}} value.
func (n *{{$type.Name | toPascalCase}}) {{.Name | toPascalCase}}() {{.GoType}} {
{{- if .IsListType}}
	return n.proto.{{.Name | toPascalCase}}
{{- else if .Required}}
	return n.proto.{{.Name | toPascalCase}}
{{- else}}
	if n.proto.{{.Name | toPascalCase}} != nil {
		return *n.proto.{{.Name | toPascalCase}}
	}
	var zero {{.GoType}}
	return zero
{{- end}}
}

// Set{{.Name | toPascalCase}} sets the {{.Name}} value.
func (n *{{$type.Name | toPascalCase}}) Set{{.Name | toPascalCase}}(v {{.GoType}}) *{{$type.Name | toPascalCase}} {
{{- if .IsListType}}
	n.proto.{{.Name | toPascalCase}} = v
{{- else if .Required}}
	n.proto.{{.Name | toPascalCase}} = v
{{- else}}
	n.proto.{{.Name | toPascalCase}} = &v
{{- end}}
	return n
}
{{- end}}
{{end}}
{{end}}`
