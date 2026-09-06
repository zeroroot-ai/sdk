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

// NodeQueryData holds query builder data for a node type.
type NodeQueryData struct {
	NodeType     schema.NodeType
	OutboundRels []RelationshipQueryData
	InboundRels  []RelationshipQueryData
}

// RelationshipQueryData holds data for a relationship traversal.
type RelationshipQueryData struct {
	Relationship schema.RelationshipType
	TargetType   string
	Direction    string
}

// GenerateQueryBuilders generates type-safe query builders from the taxonomy.
func GenerateQueryBuilders(taxonomy *schema.Taxonomy, outputPath, pkgName string) error {
	tmpl, err := template.New("query").Funcs(queryFuncMap()).Parse(queryTemplate)
	if err != nil {
		return fmt.Errorf("failed to parse template: %w", err)
	}

	// Compute relationship data for each node type

	nodeData := make([]NodeQueryData, 0, len(taxonomy.NodeTypes))
	for _, nt := range taxonomy.NodeTypes {
		nqd := NodeQueryData{
			NodeType: nt,
		}

		// Compute outbound relationships
		for _, rel := range taxonomy.RelationshipTypes {
			for _, fromType := range rel.FromTypes {
				if fromType == nt.Name {
					targetType := "GraphNode"
					if len(rel.ToTypes) > 0 && rel.ToTypes[0] != "*" {
						targetType = rel.ToTypes[0]
					}
					nqd.OutboundRels = append(nqd.OutboundRels, RelationshipQueryData{
						Relationship: rel,
						TargetType:   targetType,
						Direction:    "out",
					})
					break
				}
			}
		}

		// Compute inbound relationships
		inboundSeen := make(map[string]bool)
		for _, rel := range taxonomy.RelationshipTypes {
			for _, toType := range rel.ToTypes {
				if toType == nt.Name || toType == "*" {
					sourceType := "GraphNode"
					if len(rel.FromTypes) > 0 {
						sourceType = rel.FromTypes[0]
					}
					// Deduplicate by relationship + target type
					key := rel.Name + ":" + sourceType
					if !inboundSeen[key] {
						nqd.InboundRels = append(nqd.InboundRels, RelationshipQueryData{
							Relationship: rel,
							TargetType:   sourceType,
							Direction:    "in",
						})
						inboundSeen[key] = true
					}
					break
				}
			}
		}

		nodeData = append(nodeData, nqd)
	}

	data := struct {
		Taxonomy   *schema.Taxonomy
		NodeData   []NodeQueryData
		Package    string
		SourcePath string
	}{
		Taxonomy:   taxonomy,
		NodeData:   nodeData,
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

func queryFuncMap() template.FuncMap {
	return template.FuncMap{
		"toPascalCase":        toPascalCase,
		"toCamelCase":         toCamelCase,
		"pluralize":           pluralize,
		"isEnumProperty":      isEnumProperty,
		"traversalMethodName": traversalMethodName,
		// isListStringType returns true for list<string> properties.
		"isListStringType": func(p schema.Property) bool {
			return p.Type == "list<string>"
		},
		// isMapStringType returns true for map<string,string> or map<string,any> properties.
		"isMapStringType": func(p schema.Property) bool {
			return strings.HasPrefix(p.Type, "map<")
		},
		// isNestedListType returns true for list<SomeNestedType> (non-primitive list).
		"isNestedListType": func(p schema.Property) bool {
			if !strings.HasPrefix(p.Type, "list<") {
				return false
			}
			inner := strings.TrimPrefix(p.Type, "list<")
			inner = strings.TrimSuffix(inner, ">")
			// Primitive list types have lowercase names.
			return inner != "string" && inner != "int32" && inner != "int64" &&
				inner != "float64" && inner != "bool"
		},
		// parentRefProtoField returns the PascalCase field name for the parent ref on the node proto.
		"parentRefProtoField": func(nt schema.NodeType) string {
			if nt.Parent == nil {
				return ""
			}
			if nt.Parent.RefField != "" {
				for _, p := range nt.Properties {
					if p.Name == nt.Parent.RefField {
						return toPascalCase(nt.Parent.RefField)
					}
				}
			}
			return "Parent" + toPascalCase(nt.Parent.Type) + "Id"
		},
		// parentRefIsOptional returns true when the parent ref field is an optional proto field.
		"parentRefIsOptional": func(nt schema.NodeType) bool {
			if nt.Parent == nil || nt.Parent.RefField == "" {
				return false
			}
			for _, p := range nt.Properties {
				if p.Name == nt.Parent.RefField {
					return !p.Required
				}
			}
			return false
		},
		// parentRefFieldName returns the snake_case Neo4j property name for the parent ref.
		"parentRefFieldName": func(nt schema.NodeType) string {
			if nt.Parent == nil {
				return ""
			}
			if nt.Parent.RefField != "" {
				for _, p := range nt.Properties {
					if p.Name == nt.Parent.RefField {
						return nt.Parent.RefField
					}
				}
			}
			return "parent_" + nt.Parent.Type + "_id"
		},
	}
}

// traversalMethodName generates a unique method name for a traversal based on relationship and target type.
// Examples:
//   - HAS_PORT -> Port -> "WithPort"
//   - USES_TECHNOLOGY -> technology -> "WithTechnology"
//   - DELEGATED_TO -> agent_run -> "FromAgentRunViaDelegatedTo"
func traversalMethodName(relName string, targetType string, direction string) string {
	// Convert relationship name to PascalCase and then lowercase it for proper comparison
	// HAS_PORT -> HasPort, DELEGATED_TO -> DelegatedTo
	relPascal := toPascalCase(strings.ToLower(relName))

	// Clean up target type
	targetPascal := toPascalCase(targetType)

	// For simple relationships like HAS_X, RUNS_X, just use the target type
	if direction == "out" {
		if strings.HasPrefix(relPascal, "Has") || strings.HasPrefix(relPascal, "Runs") {
			return "With" + targetPascal
		}
		// For other relationships, use the full relationship name
		return relPascal
	} else {
		// For inbound, use "From" + target + "Via" + relationship
		return "From" + targetPascal + "Via" + relPascal
	}
}

// pluralize converts a singular noun to plural form (simple heuristic).
func pluralize(s string) string {
	if strings.HasSuffix(s, "s") || strings.HasSuffix(s, "x") ||
		strings.HasSuffix(s, "ch") || strings.HasSuffix(s, "sh") {
		return s + "es"
	}
	if strings.HasSuffix(s, "y") {
		return strings.TrimSuffix(s, "y") + "ies"
	}
	return s + "s"
}

// isEnumProperty returns true if the property has enum values.
func isEnumProperty(prop schema.Property) bool {
	return len(prop.Enum) > 0
}

const queryTemplate = `// Code generated by taxonomy-gen from taxonomy YAML. DO NOT EDIT.
// Source: {{.SourcePath}}
// Taxonomy Version: {{.Taxonomy.Version}}

package {{.Package}}

import (
	"context"
	"fmt"

	taxonomypb "github.com/zeroroot-ai/sdk/api/gen/taxonomy/v1"
)

// ==================== QUERY BUILDERS ====================
{{range .NodeData}}
// ==================== {{.NodeType.Name | toPascalCase}} Query ====================

// {{.NodeType.Name | toPascalCase}}Query provides type-safe query building for {{.NodeType.Name}} nodes.
type {{.NodeType.Name | toPascalCase}}Query struct {
	predicates []Predicate
	traversals []Traversal
	client     GraphClient
	limit      int
	offset     int
}

// {{pluralize (.NodeType.Name | toPascalCase)}} creates a new {{.NodeType.Name | toPascalCase}}Query.
func {{pluralize (.NodeType.Name | toPascalCase)}}(client GraphClient) *{{.NodeType.Name | toPascalCase}}Query {
	return &{{.NodeType.Name | toPascalCase}}Query{
		client: client,
	}
}

// Where methods for each property
{{- $queryType := .NodeType.Name | toPascalCase}}
{{- $nodeType := .NodeType}}
{{- range .NodeType.Properties}}
{{- if isNestedListType .}}
{{- /* Skip Where methods for nested list types — not queryable via simple predicates */}}
{{- else if isMapStringType .}}
{{- /* Skip Where methods for map types — not queryable via simple predicates */}}
{{- else}}

// Where{{.Name | toPascalCase}} adds a predicate on the {{.Name}} property.
func (q *{{$queryType}}Query) Where{{.Name | toPascalCase}}(op Op, value {{.GoType}}) *{{$queryType}}Query {
	q.predicates = append(q.predicates, Predicate{
		Field: "{{.Name}}",
		Op:    op,
		Value: value,
	})
	return q
}
{{- if isEnumProperty .}}

// Where{{.Name | toPascalCase}}In adds an IN predicate for {{.Name}} enum values.
func (q *{{$queryType}}Query) Where{{.Name | toPascalCase}}In(values ...string) *{{$queryType}}Query {
	q.predicates = append(q.predicates, Predicate{
		Field: "{{.Name}}",
		Op:    In,
		Value: values,
	})
	return q
}
{{- end}}
{{- end}}
{{- end}}

// Traversal methods for relationships
{{- if gt (len .OutboundRels) 0}}

// --- Outbound Traversals ---
{{- range .OutboundRels}}

// {{traversalMethodName .Relationship.Name .TargetType .Direction}} traverses the {{.Relationship.Name}} relationship to {{.TargetType}} nodes.
func (q *{{$queryType}}Query) {{traversalMethodName .Relationship.Name .TargetType .Direction}}() *{{$queryType}}Query {
	q.traversals = append(q.traversals, Traversal{
		Relationship: "{{.Relationship.Name}}",
		TargetType:   "{{.TargetType}}",
		Direction:    "{{.Direction}}",
	})
	return q
}
{{- end}}
{{- end}}

{{- if gt (len .InboundRels) 0}}

// --- Inbound Traversals ---
{{- range .InboundRels}}

// {{traversalMethodName .Relationship.Name .TargetType .Direction}} traverses the {{.Relationship.Name}} relationship from {{.TargetType}} nodes.
func (q *{{$queryType}}Query) {{traversalMethodName .Relationship.Name .TargetType .Direction}}() *{{$queryType}}Query {
	q.traversals = append(q.traversals, Traversal{
		Relationship: "{{.Relationship.Name}}",
		TargetType:   "{{.TargetType}}",
		Direction:    "{{.Direction}}",
	})
	return q
}
{{- end}}
{{- end}}

// Terminal methods

// All executes the query and returns all matching {{.NodeType.Name}} nodes.
func (q *{{$queryType}}Query) All(ctx context.Context) ([]*taxonomypb.{{.NodeType.Name | toPascalCase}}, error) {
	cypher, params := q.buildCypher()

	results, err := q.client.Query(ctx, cypher, params)
	if err != nil {
		return nil, fmt.Errorf("query failed: %w", err)
	}

	nodes := make([]*taxonomypb.{{.NodeType.Name | toPascalCase}}, 0, len(results))
	for _, result := range results {
		node, err := parseNodeTo{{.NodeType.Name | toPascalCase}}(result)
		if err != nil {
			return nil, fmt.Errorf("failed to parse result: %w", err)
		}
		nodes = append(nodes, node)
	}

	return nodes, nil
}

// First executes the query and returns the first matching {{.NodeType.Name}} node.
func (q *{{$queryType}}Query) First(ctx context.Context) (*taxonomypb.{{.NodeType.Name | toPascalCase}}, error) {
	q.limit = 1
	nodes, err := q.All(ctx)
	if err != nil {
		return nil, err
	}
	if len(nodes) == 0 {
		return nil, fmt.Errorf("no {{.NodeType.Name}} found")
	}
	return nodes[0], nil
}

// Count executes the query and returns the count of matching {{.NodeType.Name}} nodes.
func (q *{{$queryType}}Query) Count(ctx context.Context) (int, error) {
	cypher, params := q.buildCypherCount()

	results, err := q.client.Query(ctx, cypher, params)
	if err != nil {
		return 0, fmt.Errorf("count query failed: %w", err)
	}

	if len(results) == 0 {
		return 0, nil
	}

	count, ok := results[0]["count"].(int64)
	if !ok {
		return 0, fmt.Errorf("invalid count result")
	}

	return int(count), nil
}

// Limit sets the maximum number of results to return.
func (q *{{$queryType}}Query) Limit(n int) *{{$queryType}}Query {
	q.limit = n
	return q
}

// Offset sets the number of results to skip.
func (q *{{$queryType}}Query) Offset(n int) *{{$queryType}}Query {
	q.offset = n
	return q
}

// buildCypher builds the Cypher query for this {{.NodeType.Name}} query.
func (q *{{$queryType}}Query) buildCypher() (string, map[string]any) {
	alias := "n"
	cypher := BuildMatch("{{.NodeType.Name | toPascalCase}}", alias)

	// Add traversals
	for i, trav := range q.traversals {
		travAlias := fmt.Sprintf("t%d", i)
		cypher += " " + BuildTraversal(trav, alias, travAlias)
	}

	// Add predicates
	if len(q.predicates) > 0 {
		whereClause, params := BuildWhere(q.predicates, alias)
		cypher += " " + whereClause

		// Add return clause
		cypher += " " + BuildReturn(alias, nil)

		// Add pagination
		if q.limit > 0 {
			cypher += fmt.Sprintf(" LIMIT %d", q.limit)
		}
		if q.offset > 0 {
			cypher += fmt.Sprintf(" SKIP %d", q.offset)
		}

		return cypher, params
	}

	// No predicates - just return
	cypher += " " + BuildReturn(alias, nil)

	// Add pagination
	if q.limit > 0 {
		cypher += fmt.Sprintf(" LIMIT %d", q.limit)
	}
	if q.offset > 0 {
		cypher += fmt.Sprintf(" SKIP %d", q.offset)
	}

	return cypher, nil
}

// buildCypherCount builds the COUNT Cypher query for this {{.NodeType.Name}} query.
func (q *{{$queryType}}Query) buildCypherCount() (string, map[string]any) {
	alias := "n"
	cypher := BuildMatch("{{.NodeType.Name | toPascalCase}}", alias)

	// Add traversals
	for i, trav := range q.traversals {
		travAlias := fmt.Sprintf("t%d", i)
		cypher += " " + BuildTraversal(trav, alias, travAlias)
	}

	// Add predicates
	if len(q.predicates) > 0 {
		whereClause, params := BuildWhere(q.predicates, alias)
		cypher += " " + whereClause + " RETURN count(" + alias + ") as count"
		return cypher, params
	}

	cypher += " RETURN count(" + alias + ") as count"
	return cypher, nil
}

// parseNodeTo{{.NodeType.Name | toPascalCase}} parses a raw query result into a {{.NodeType.Name | toPascalCase}} proto.
func parseNodeTo{{.NodeType.Name | toPascalCase}}(result map[string]any) (*taxonomypb.{{.NodeType.Name | toPascalCase}}, error) {
	nodeData, ok := result["n"].(map[string]any)
	if !ok {
		return nil, fmt.Errorf("invalid node data")
	}

	node := &taxonomypb.{{.NodeType.Name | toPascalCase}}{}

	// Parse ID
	if id, ok := nodeData["id"].(string); ok {
		node.Id = id
	}

	// Parse properties
{{- $nodeTypeRef := .NodeType}}
{{- range .NodeType.Properties}}
{{- if isListStringType .}}
	if val, ok := nodeData["{{.Name}}"]; ok {
		if strs, ok := val.([]string); ok {
			node.{{.Name | toPascalCase}} = strs
		} else if ifaces, ok := val.([]interface{}); ok {
			for _, v := range ifaces {
				if s, ok := v.(string); ok {
					node.{{.Name | toPascalCase}} = append(node.{{.Name | toPascalCase}}, s)
				}
			}
		}
	}
{{- else if isMapStringType .}}
	// {{.Name}} is a map type — stored as JSON string in Neo4j, skip deserialization here.
	if _, ok := nodeData["{{.Name}}"]; ok {
		// map deserialization not generated; access via raw Properties() if needed.
		_ = nodeData["{{.Name}}"]
	}
{{- else if isNestedListType .}}
	// {{.Name}} is a list of nested objects — skip deserialization here.
	if _, ok := nodeData["{{.Name}}"]; ok {
		// nested list deserialization not generated; access via raw Properties() if needed.
		_ = nodeData["{{.Name}}"]
	}
{{- else if eq .Type "string"}}
	if val, ok := nodeData["{{.Name}}"]; ok {
		if s, ok := val.(string); ok {
{{- if .Required}}
			node.{{.Name | toPascalCase}} = s
{{- else}}
			node.{{.Name | toPascalCase}} = &s
{{- end}}
		}
	}
{{- else if eq .Type "int32"}}
	if val, ok := nodeData["{{.Name}}"]; ok {
		if i, ok := val.(int64); ok {
			i32 := int32(i)
{{- if .Required}}
			node.{{.Name | toPascalCase}} = i32
{{- else}}
			node.{{.Name | toPascalCase}} = &i32
{{- end}}
		}
	}
{{- else if eq .Type "int64"}}
	if val, ok := nodeData["{{.Name}}"]; ok {
		if i, ok := val.(int64); ok {
{{- if .Required}}
			node.{{.Name | toPascalCase}} = i
{{- else}}
			node.{{.Name | toPascalCase}} = &i
{{- end}}
		}
	}
{{- else if eq .Type "float64"}}
	if val, ok := nodeData["{{.Name}}"]; ok {
		if f, ok := val.(float64); ok {
{{- if .Required}}
			node.{{.Name | toPascalCase}} = f
{{- else}}
			node.{{.Name | toPascalCase}} = &f
{{- end}}
		}
	}
{{- else if eq .Type "bool"}}
	if val, ok := nodeData["{{.Name}}"]; ok {
		if b, ok := val.(bool); ok {
{{- if .Required}}
			node.{{.Name | toPascalCase}} = b
{{- else}}
			node.{{.Name | toPascalCase}} = &b
{{- end}}
		}
	}
{{- else if eq .Type "timestamp"}}
	if val, ok := nodeData["{{.Name}}"]; ok {
		if t, ok := val.(int64); ok {
{{- if .Required}}
			node.{{.Name | toPascalCase}} = t
{{- else}}
			node.{{.Name | toPascalCase}} = &t
{{- end}}
		}
	}
{{- else if eq .Type "bytes"}}
	if val, ok := nodeData["{{.Name}}"]; ok {
		if b, ok := val.([]byte); ok {
{{- if .Required}}
			node.{{.Name | toPascalCase}} = b
{{- else}}
			node.{{.Name | toPascalCase}} = &b
{{- end}}
		}
	}
{{- end}}
{{- end}}

{{- if .NodeType.Parent}}
	// Parse parent reference
	if parentId, ok := nodeData["{{parentRefFieldName .NodeType}}"]; ok {
		if s, ok := parentId.(string); ok {
{{- if parentRefIsOptional .NodeType}}
			node.{{parentRefProtoField .NodeType}} = &s
{{- else}}
			node.{{parentRefProtoField .NodeType}} = s
{{- end}}
		}
	}
{{- end}}

	return node, nil
}
{{end}}
`
