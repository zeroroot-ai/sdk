// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Zero Root AI

package query_test

import (
	"fmt"

	"github.com/zeroroot-ai/sdk/graphrag/query"
)

// ExampleBuildMatch demonstrates building a MATCH clause for a node.
func ExampleBuildMatch() {
	cypher := query.BuildMatch("Host", "h")
	fmt.Println(cypher)
	// Output: MATCH (h:Host)
}

// ExampleBuildWhere demonstrates building a WHERE clause with predicates.
func ExampleBuildWhere() {
	predicates := []query.Predicate{
		{Field: "ip", Op: query.Eq, Value: "192.168.1.1"},
		{Field: "port", Op: query.Gt, Value: 1000},
	}

	whereClause, params := query.BuildWhere(predicates, "h")
	fmt.Println(whereClause)
	fmt.Printf("Parameters: %v\n", params)
	// Output:
	// WHERE h.ip = $p0 AND h.port > $p1
	// Parameters: map[p0:192.168.1.1 p1:1000]
}

// ExampleBuildWhere_nullChecks demonstrates null checking predicates.
func ExampleBuildWhere_nullChecks() {
	predicates := []query.Predicate{
		{Field: "banner", Op: query.IsNotNull},
		{Field: "cve_id", Op: query.IsNull},
	}

	whereClause, params := query.BuildWhere(predicates, "v")
	fmt.Println(whereClause)
	fmt.Printf("Parameters: %v\n", params)
	// Output:
	// WHERE v.banner IS NOT NULL AND v.cve_id IS NULL
	// Parameters: map[]
}

// ExampleBuildReturn demonstrates building a RETURN clause.
func ExampleBuildReturn() {
	// Return entire node
	returnAll := query.BuildReturn("h", nil)
	fmt.Println(returnAll)

	// Return specific fields
	returnFields := query.BuildReturn("h", []string{"ip", "hostname"})
	fmt.Println(returnFields)

	// Output:
	// RETURN h
	// RETURN h.ip, h.hostname
}

// ExampleBuildTraversal demonstrates building relationship traversal patterns.
func ExampleBuildTraversal() {
	// Outbound traversal
	outbound := query.Traversal{
		Relationship: "RUNS_ON",
		TargetType:   "Host",
		Direction:    "out",
	}
	fmt.Println(query.BuildTraversal(outbound, "s", "h"))

	// Inbound traversal
	inbound := query.Traversal{
		Relationship: "HAS_PORT",
		TargetType:   "Port",
		Direction:    "in",
	}
	fmt.Println(query.BuildTraversal(inbound, "h", "p"))

	// Bidirectional traversal
	both := query.Traversal{
		Relationship: "CONNECTED_TO",
		TargetType:   "Host",
		Direction:    "both",
	}
	fmt.Println(query.BuildTraversal(both, "h1", "h2"))

	// Output:
	// (s)-[:RUNS_ON]->(h:Host)
	// (h)<-[:HAS_PORT]-(p:Port)
	// (h1)-[:CONNECTED_TO]-(h2:Host)
}

// Example_fullQuery demonstrates building a complete Cypher query.
func Example_fullQuery() {
	// Query: Find all open TCP ports > 1000 on production hosts
	// MATCH (p:Port)<-[:HAS_PORT]-(h:Host)
	// WHERE p.state = $p0 AND p.protocol = $p1 AND p.number > $p2 AND h.environment = $p3
	// RETURN p.number, p.protocol, h.ip

	// Build the MATCH clause
	match := query.BuildMatch("Port", "p")

	// Build the relationship traversal
	traversal := query.Traversal{
		Relationship: "HAS_PORT",
		TargetType:   "Host",
		Direction:    "in",
	}
	pattern := query.BuildTraversal(traversal, "p", "h")

	// Build the WHERE clause with predicates
	predicates := []query.Predicate{
		{Field: "state", Op: query.Eq, Value: "open"},
		{Field: "protocol", Op: query.Eq, Value: "tcp"},
		{Field: "number", Op: query.Gt, Value: 1000},
	}
	where, params := query.BuildWhere(predicates, "p")

	// Add host environment filter manually
	// (In a real implementation, you'd merge predicates or use multiple BuildWhere calls)
	hostWhere := " AND h.environment = $p3"
	params["p3"] = "production"

	// Build the RETURN clause
	returnClause := "RETURN p.number, p.protocol, h.ip"

	// Combine into full query
	fullQuery := fmt.Sprintf("%s %s %s%s %s", match, pattern, where, hostWhere, returnClause)
	fmt.Println(fullQuery)
	fmt.Printf("Parameters: %v\n", params)

	// Output:
	// MATCH (p:Port) (p)<-[:HAS_PORT]-(h:Host) WHERE p.state = $p0 AND p.protocol = $p1 AND p.number > $p2 AND h.environment = $p3 RETURN p.number, p.protocol, h.ip
	// Parameters: map[p0:open p1:tcp p2:1000 p3:production]
}

// ExampleBuildWhere_stringOperations demonstrates string matching operations.
func ExampleBuildWhere_stringOperations() {
	predicates := []query.Predicate{
		{Field: "domain", Op: query.StartsWith, Value: "prod-"},
		{Field: "hostname", Op: query.EndsWith, Value: ".mil"},
		{Field: "description", Op: query.Contains, Value: "vulnerability"},
	}

	whereClause, _ := query.BuildWhere(predicates, "h")
	fmt.Println(whereClause)
	// Output:
	// WHERE h.domain STARTS WITH $p0 AND h.hostname ENDS WITH $p1 AND h.description CONTAINS $p2
}

// ExampleBuildWhere_inOperator demonstrates the IN operator for list matching.
func ExampleBuildWhere_inOperator() {
	predicates := []query.Predicate{
		{Field: "protocol", Op: query.In, Value: []string{"tcp", "udp"}},
		{Field: "severity", Op: query.In, Value: []int{7, 8, 9, 10}},
	}

	whereClause, _ := query.BuildWhere(predicates, "v")
	fmt.Println(whereClause)
	// Output:
	// WHERE v.protocol IN $p0 AND v.severity IN $p1
}
