// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Zero Root AI

package query

import (
	"reflect"
	"testing"
)

func TestBuildMatch(t *testing.T) {
	tests := []struct {
		name     string
		nodeType string
		alias    string
		want     string
	}{
		{
			name:     "simple host match",
			nodeType: "Host",
			alias:    "h",
			want:     "MATCH (h:Host)",
		},
		{
			name:     "port match",
			nodeType: "Port",
			alias:    "p",
			want:     "MATCH (p:Port)",
		},
		{
			name:     "service match with longer alias",
			nodeType: "Service",
			alias:    "svc",
			want:     "MATCH (svc:Service)",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := BuildMatch(tt.nodeType, tt.alias)
			if got != tt.want {
				t.Errorf("BuildMatch() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestBuildWhere(t *testing.T) {
	tests := []struct {
		name       string
		predicates []Predicate
		alias      string
		wantWhere  string
		wantParams map[string]any
	}{
		{
			name:       "empty predicates",
			predicates: nil,
			alias:      "h",
			wantWhere:  "",
			wantParams: nil,
		},
		{
			name: "single equality predicate",
			predicates: []Predicate{
				{Field: "ip", Op: Eq, Value: "192.168.1.1"},
			},
			alias:      "h",
			wantWhere:  "WHERE h.ip = $p0",
			wantParams: map[string]any{"p0": "192.168.1.1"},
		},
		{
			name: "multiple predicates",
			predicates: []Predicate{
				{Field: "ip", Op: Eq, Value: "192.168.1.1"},
				{Field: "port", Op: Gt, Value: 1000},
			},
			alias:      "h",
			wantWhere:  "WHERE h.ip = $p0 AND h.port > $p1",
			wantParams: map[string]any{"p0": "192.168.1.1", "p1": 1000},
		},
		{
			name: "inequality predicate",
			predicates: []Predicate{
				{Field: "status", Op: Neq, Value: "closed"},
			},
			alias:      "p",
			wantWhere:  "WHERE p.status <> $p0",
			wantParams: map[string]any{"p0": "closed"},
		},
		{
			name: "less than predicate",
			predicates: []Predicate{
				{Field: "port", Op: Lt, Value: 1024},
			},
			alias:      "p",
			wantWhere:  "WHERE p.port < $p0",
			wantParams: map[string]any{"p0": 1024},
		},
		{
			name: "less than or equal predicate",
			predicates: []Predicate{
				{Field: "severity", Op: Lte, Value: 5},
			},
			alias:      "v",
			wantWhere:  "WHERE v.severity <= $p0",
			wantParams: map[string]any{"p0": 5},
		},
		{
			name: "greater than or equal predicate",
			predicates: []Predicate{
				{Field: "score", Op: Gte, Value: 7.5},
			},
			alias:      "f",
			wantWhere:  "WHERE f.score >= $p0",
			wantParams: map[string]any{"p0": 7.5},
		},
		{
			name: "contains predicate",
			predicates: []Predicate{
				{Field: "description", Op: Contains, Value: "SQL injection"},
			},
			alias:      "v",
			wantWhere:  "WHERE v.description CONTAINS $p0",
			wantParams: map[string]any{"p0": "SQL injection"},
		},
		{
			name: "starts with predicate",
			predicates: []Predicate{
				{Field: "hostname", Op: StartsWith, Value: "prod-"},
			},
			alias:      "h",
			wantWhere:  "WHERE h.hostname STARTS WITH $p0",
			wantParams: map[string]any{"p0": "prod-"},
		},
		{
			name: "ends with predicate",
			predicates: []Predicate{
				{Field: "domain", Op: EndsWith, Value: ".mil"},
			},
			alias:      "h",
			wantWhere:  "WHERE h.domain ENDS WITH $p0",
			wantParams: map[string]any{"p0": ".mil"},
		},
		{
			name: "in predicate",
			predicates: []Predicate{
				{Field: "protocol", Op: In, Value: []string{"tcp", "udp"}},
			},
			alias:      "p",
			wantWhere:  "WHERE p.protocol IN $p0",
			wantParams: map[string]any{"p0": []string{"tcp", "udp"}},
		},
		{
			name: "is null predicate",
			predicates: []Predicate{
				{Field: "banner", Op: IsNull},
			},
			alias:      "p",
			wantWhere:  "WHERE p.banner IS NULL",
			wantParams: map[string]any{},
		},
		{
			name: "is not null predicate",
			predicates: []Predicate{
				{Field: "cve_id", Op: IsNotNull},
			},
			alias:      "v",
			wantWhere:  "WHERE v.cve_id IS NOT NULL",
			wantParams: map[string]any{},
		},
		{
			name: "mixed predicates with null checks",
			predicates: []Predicate{
				{Field: "status", Op: Eq, Value: "open"},
				{Field: "banner", Op: IsNotNull},
				{Field: "port", Op: Gte, Value: 80},
			},
			alias:      "p",
			wantWhere:  "WHERE p.status = $p0 AND p.banner IS NOT NULL AND p.port >= $p2",
			wantParams: map[string]any{"p0": "open", "p2": 80},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotWhere, gotParams := BuildWhere(tt.predicates, tt.alias)
			if gotWhere != tt.wantWhere {
				t.Errorf("BuildWhere() where = %v, want %v", gotWhere, tt.wantWhere)
			}
			if !reflect.DeepEqual(gotParams, tt.wantParams) {
				t.Errorf("BuildWhere() params = %v, want %v", gotParams, tt.wantParams)
			}
		})
	}
}

func TestBuildReturn(t *testing.T) {
	tests := []struct {
		name   string
		alias  string
		fields []string
		want   string
	}{
		{
			name:   "return entire node (nil fields)",
			alias:  "h",
			fields: nil,
			want:   "RETURN h",
		},
		{
			name:   "return entire node (empty fields)",
			alias:  "h",
			fields: []string{},
			want:   "RETURN h",
		},
		{
			name:   "return single field",
			alias:  "h",
			fields: []string{"ip"},
			want:   "RETURN h.ip",
		},
		{
			name:   "return multiple fields",
			alias:  "h",
			fields: []string{"ip", "hostname", "os"},
			want:   "RETURN h.ip, h.hostname, h.os",
		},
		{
			name:   "return port fields",
			alias:  "p",
			fields: []string{"number", "protocol", "state"},
			want:   "RETURN p.number, p.protocol, p.state",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := BuildReturn(tt.alias, tt.fields)
			if got != tt.want {
				t.Errorf("BuildReturn() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestBuildTraversal(t *testing.T) {
	tests := []struct {
		name      string
		traversal Traversal
		fromAlias string
		toAlias   string
		want      string
	}{
		{
			name: "outbound traversal",
			traversal: Traversal{
				Relationship: "RUNS_ON",
				TargetType:   "Host",
				Direction:    "out",
			},
			fromAlias: "s",
			toAlias:   "h",
			want:      "(s)-[:RUNS_ON]->(h:Host)",
		},
		{
			name: "inbound traversal",
			traversal: Traversal{
				Relationship: "HAS_PORT",
				TargetType:   "Host",
				Direction:    "in",
			},
			fromAlias: "p",
			toAlias:   "h",
			want:      "(p)<-[:HAS_PORT]-(h:Host)",
		},
		{
			name: "bidirectional traversal",
			traversal: Traversal{
				Relationship: "CONNECTED_TO",
				TargetType:   "Host",
				Direction:    "both",
			},
			fromAlias: "h1",
			toAlias:   "h2",
			want:      "(h1)-[:CONNECTED_TO]-(h2:Host)",
		},
		{
			name: "vulnerability to service",
			traversal: Traversal{
				Relationship: "AFFECTS",
				TargetType:   "Service",
				Direction:    "out",
			},
			fromAlias: "v",
			toAlias:   "s",
			want:      "(v)-[:AFFECTS]->(s:Service)",
		},
		{
			name: "invalid direction defaults to out",
			traversal: Traversal{
				Relationship: "RELATED_TO",
				TargetType:   "Node",
				Direction:    "invalid",
			},
			fromAlias: "n1",
			toAlias:   "n2",
			want:      "(n1)-[:RELATED_TO]->(n2:Node)",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := BuildTraversal(tt.traversal, tt.fromAlias, tt.toAlias)
			if got != tt.want {
				t.Errorf("BuildTraversal() = %v, want %v", got, tt.want)
			}
		})
	}
}

// TestFullQueryConstruction demonstrates building a complete Cypher query
func TestFullQueryConstruction(t *testing.T) {
	// Build a query: Find all open ports > 1000 on hosts in production
	nodeType := "Port"
	alias := "p"

	predicates := []Predicate{
		{Field: "state", Op: Eq, Value: "open"},
		{Field: "number", Op: Gt, Value: 1000},
	}

	traversal := Traversal{
		Relationship: "HAS_PORT",
		TargetType:   "Host",
		Direction:    "in",
	}

	// Build query parts
	match := BuildMatch(nodeType, alias)
	where, params := BuildWhere(predicates, alias)
	traversalPattern := BuildTraversal(traversal, alias, "h")
	returnClause := BuildReturn(alias, []string{"number", "protocol", "state"})

	// Construct full query
	fullQuery := match + " " + traversalPattern + " " + where + " " + returnClause

	expectedQuery := "MATCH (p:Port) (p)<-[:HAS_PORT]-(h:Host) WHERE p.state = $p0 AND p.number > $p1 RETURN p.number, p.protocol, p.state"
	if fullQuery != expectedQuery {
		t.Errorf("Full query = %v, want %v", fullQuery, expectedQuery)
	}

	expectedParams := map[string]any{
		"p0": "open",
		"p1": 1000,
	}
	if !reflect.DeepEqual(params, expectedParams) {
		t.Errorf("Params = %v, want %v", params, expectedParams)
	}
}

// TestParameterSafety ensures that parameter binding prevents injection
func TestParameterSafety(t *testing.T) {
	// Attempt to inject malicious Cypher via predicate value
	maliciousValue := "'; DROP DATABASE; --"

	predicates := []Predicate{
		{Field: "hostname", Op: Eq, Value: maliciousValue},
	}

	where, params := BuildWhere(predicates, "h")

	// The value should be safely bound as a parameter, not interpolated
	expectedWhere := "WHERE h.hostname = $p0"
	if where != expectedWhere {
		t.Errorf("BuildWhere() where = %v, want %v", where, expectedWhere)
	}

	// The malicious value should be in params, not in the query string
	if params["p0"] != maliciousValue {
		t.Errorf("Parameter value = %v, want %v", params["p0"], maliciousValue)
	}

	// Verify the malicious string is NOT in the WHERE clause
	if contains(where, maliciousValue) {
		t.Errorf("WHERE clause contains malicious value directly: %v", where)
	}
}

// contains checks if s contains substr
func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > len(substr) && (s[:len(substr)] == substr || s[len(s)-len(substr):] == substr || containsInMiddle(s, substr)))
}

func containsInMiddle(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
