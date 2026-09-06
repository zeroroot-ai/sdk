// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Zero Root AI

package query

import (
	"fmt"
	"strings"
)

// BuildMatch generates a MATCH clause for a node with the given type and alias.
// Returns a Cypher MATCH clause like: MATCH (alias:NodeType)
//
// Example:
//
//	BuildMatch("Host", "h") // Returns: "MATCH (h:Host)"
func BuildMatch(nodeType string, alias string) string {
	return fmt.Sprintf("MATCH (%s:%s)", alias, nodeType)
}

// BuildWhere generates a WHERE clause from predicates with parameterized values.
// Returns the WHERE clause string and a map of parameter names to values.
// Parameters are named $p0, $p1, etc. to prevent SQL/Cypher injection.
//
// Returns empty string and nil params if predicates is empty or nil.
//
// Example:
//
//	predicates := []Predicate{
//	    {Field: "ip", Op: Eq, Value: "192.168.1.1"},
//	    {Field: "port", Op: Gt, Value: 1000},
//	}
//	where, params := BuildWhere(predicates, "h")
//	// Returns: "WHERE h.ip = $p0 AND h.port > $p1"
//	// params: {"p0": "192.168.1.1", "p1": 1000}
func BuildWhere(predicates []Predicate, alias string) (string, map[string]any) {
	if len(predicates) == 0 {
		return "", nil
	}

	params := make(map[string]any)
	var conditions []string

	for i, pred := range predicates {
		paramName := fmt.Sprintf("p%d", i)
		condition := buildCondition(pred, alias, paramName)
		conditions = append(conditions, condition)

		// Add parameter value if the operation requires one
		if requiresValue(pred.Op) {
			params[paramName] = pred.Value
		}
	}

	whereClause := "WHERE " + strings.Join(conditions, " AND ")
	return whereClause, params
}

// buildCondition constructs a single WHERE condition for a predicate.
func buildCondition(pred Predicate, alias string, paramName string) string {
	fieldRef := fmt.Sprintf("%s.%s", alias, pred.Field)

	switch pred.Op {
	case Eq:
		return fmt.Sprintf("%s = $%s", fieldRef, paramName)
	case Neq:
		return fmt.Sprintf("%s <> $%s", fieldRef, paramName)
	case Lt:
		return fmt.Sprintf("%s < $%s", fieldRef, paramName)
	case Lte:
		return fmt.Sprintf("%s <= $%s", fieldRef, paramName)
	case Gt:
		return fmt.Sprintf("%s > $%s", fieldRef, paramName)
	case Gte:
		return fmt.Sprintf("%s >= $%s", fieldRef, paramName)
	case Contains:
		return fmt.Sprintf("%s CONTAINS $%s", fieldRef, paramName)
	case StartsWith:
		return fmt.Sprintf("%s STARTS WITH $%s", fieldRef, paramName)
	case EndsWith:
		return fmt.Sprintf("%s ENDS WITH $%s", fieldRef, paramName)
	case In:
		return fmt.Sprintf("%s IN $%s", fieldRef, paramName)
	case IsNull:
		return fieldRef + " IS NULL"
	case IsNotNull:
		return fieldRef + " IS NOT NULL"
	default:
		// Unknown operation - return a safe default
		return fmt.Sprintf("%s = $%s", fieldRef, paramName)
	}
}

// requiresValue returns true if the operation requires a parameter value.
// IsNull and IsNotNull operations do not require values.
func requiresValue(op Op) bool {
	return op != IsNull && op != IsNotNull
}

// BuildReturn generates a RETURN clause with the specified alias and optional fields.
// If fields is empty, returns the entire node.
// Otherwise, returns only the specified fields.
//
// Examples:
//
//	BuildReturn("h", nil)              // Returns: "RETURN h"
//	BuildReturn("h", []string{})       // Returns: "RETURN h"
//	BuildReturn("h", []string{"ip"})   // Returns: "RETURN h.ip"
//	BuildReturn("h", []string{"ip", "port"}) // Returns: "RETURN h.ip, h.port"
func BuildReturn(alias string, fields []string) string {
	if len(fields) == 0 {
		return "RETURN " + alias
	}

	var fieldRefs []string
	for _, field := range fields {
		fieldRefs = append(fieldRefs, fmt.Sprintf("%s.%s", alias, field))
	}

	return "RETURN " + strings.Join(fieldRefs, ", ")
}

// BuildTraversal generates a Cypher pattern for traversing a relationship.
// The direction determines the arrow direction in the pattern:
//   - "out": (fromAlias)-[:REL]->(toAlias:TargetType)
//   - "in":  (fromAlias)<-[:REL]-(toAlias:TargetType)
//   - "both": (fromAlias)-[:REL]-(toAlias:TargetType)
//
// Example:
//
//	t := Traversal{
//	    Relationship: "RUNS_ON",
//	    TargetType: "Host",
//	    Direction: "out",
//	}
//	BuildTraversal(t, "s", "h")
//	// Returns: "(s)-[:RUNS_ON]->(h:Host)"
func BuildTraversal(t Traversal, fromAlias string, toAlias string) string {
	rel := fmt.Sprintf("[:%s]", t.Relationship)
	target := fmt.Sprintf("%s:%s", toAlias, t.TargetType)

	switch t.Direction {
	case "out":
		return fmt.Sprintf("(%s)-%s->(%s)", fromAlias, rel, target)
	case "in":
		return fmt.Sprintf("(%s)<-%s-(%s)", fromAlias, rel, target)
	case "both":
		return fmt.Sprintf("(%s)-%s-(%s)", fromAlias, rel, target)
	default:
		// Default to outbound if direction is invalid
		return fmt.Sprintf("(%s)-%s->(%s)", fromAlias, rel, target)
	}
}
