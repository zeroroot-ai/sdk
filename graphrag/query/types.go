// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Zero Root AI

// Package query provides a type-safe query builder for Neo4j graph queries.
// It generates Cypher queries and returns protocol buffer types directly.
package query

import (
	"context"
	"fmt"
)

// Op represents a comparison or filter operation in a query predicate.
type Op int

const (
	// Eq represents equality comparison (=)
	Eq Op = iota
	// Neq represents inequality comparison (<>)
	Neq
	// Lt represents less than comparison (<)
	Lt
	// Lte represents less than or equal comparison (<=)
	Lte
	// Gt represents greater than comparison (>)
	Gt
	// Gte represents greater than or equal comparison (>=)
	Gte
	// Contains represents string containment check (CONTAINS)
	Contains
	// StartsWith represents string prefix check (STARTS WITH)
	StartsWith
	// EndsWith represents string suffix check (ENDS WITH)
	EndsWith
	// In represents membership check (IN)
	In
	// IsNull represents null check (IS NULL)
	IsNull
	// IsNotNull represents non-null check (IS NOT NULL)
	IsNotNull
)

// String returns the string representation of the operation for debugging.
func (o Op) String() string {
	switch o {
	case Eq:
		return "="
	case Neq:
		return "<>"
	case Lt:
		return "<"
	case Lte:
		return "<="
	case Gt:
		return ">"
	case Gte:
		return ">="
	case Contains:
		return "CONTAINS"
	case StartsWith:
		return "STARTS WITH"
	case EndsWith:
		return "ENDS WITH"
	case In:
		return "IN"
	case IsNull:
		return "IS NULL"
	case IsNotNull:
		return "IS NOT NULL"
	default:
		return fmt.Sprintf("Op(%d)", int(o))
	}
}

// Predicate represents a filter condition in a graph query.
// It combines a field name, comparison operator, and value.
type Predicate struct {
	// Field is the property name to filter on
	Field string
	// Op is the comparison operation to perform
	Op Op
	// Value is the comparison value (may be nil for IsNull/IsNotNull)
	Value any
}

// Traversal represents a graph relationship traversal.
// It defines how to navigate from one node type to another via a relationship.
type Traversal struct {
	// Relationship is the relationship type to traverse
	Relationship string
	// TargetType is the target node label to match
	TargetType string
	// Direction specifies traversal direction: "out", "in", or "both"
	Direction string
}

// GraphClient provides the interface for executing Cypher queries against Neo4j.
// Implementations handle connection management, parameter binding, and result parsing.
type GraphClient interface {
	// Query executes a Cypher query with parameters and returns raw results.
	// Each result row is returned as a map of column names to values.
	Query(ctx context.Context, cypher string, params map[string]any) ([]map[string]any, error)
}
