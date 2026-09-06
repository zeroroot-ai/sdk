// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Zero Root AI

package query_test

import (
	"fmt"

	taxonomypb "github.com/zeroroot-ai/sdk/api/gen/taxonomy/v1"
	"github.com/zeroroot-ai/sdk/graphrag/query"
	"google.golang.org/protobuf/proto"
)

// ExampleMapRowToProto demonstrates mapping a Neo4j result row to a proto message.
func ExampleMapRowToProto() {
	// Simulate a Neo4j query result row
	row := map[string]any{
		"id":       "host-1",
		"ip":       "192.168.1.100",
		"hostname": "webserver.internal",
		"os":       "Linux",
		"state":    "up",
	}

	// Create a new Host proto message
	host := &taxonomypb.Host{}

	// Map the row to the proto
	err := query.MapRowToProto(row, host)
	if err != nil {
		panic(err)
	}

	// Access the mapped fields
	fmt.Printf("Host ID: %s\n", host.Id)
	fmt.Printf("IP: %s\n", host.GetIp())
	fmt.Printf("Hostname: %s\n", host.GetHostname())
	fmt.Printf("OS: %s\n", host.GetOs())

	// Output:
	// Host ID: host-1
	// IP: 192.168.1.100
	// Hostname: webserver.internal
	// OS: Linux
}

// ExampleMapRowsToProtos demonstrates mapping multiple Neo4j result rows to proto messages.
func ExampleMapRowsToProtos() {
	// Simulate Neo4j query results with multiple rows
	rows := []map[string]any{
		{
			"id":             "port-1",
			"number":         int64(80), // Neo4j returns int64
			"protocol":       "tcp",
			"state":          "open",
			"parent_host_id": "host-1",
		},
		{
			"id":             "port-2",
			"number":         int64(443),
			"protocol":       "tcp",
			"state":          "open",
			"parent_host_id": "host-1",
		},
		{
			"id":             "port-3",
			"number":         int64(22),
			"protocol":       "tcp",
			"state":          "filtered",
			"parent_host_id": "host-1",
		},
	}

	// Map all rows to Port protos
	ports, err := query.MapRowsToProtos(rows, func() *taxonomypb.Port {
		return &taxonomypb.Port{}
	})
	if err != nil {
		panic(err)
	}

	// Process the results
	fmt.Printf("Found %d ports:\n", len(ports))
	for _, port := range ports {
		fmt.Printf("- Port %d/%s (%s)\n", port.Number, port.Protocol, port.GetState())
	}

	// Output:
	// Found 3 ports:
	// - Port 80/tcp (open)
	// - Port 443/tcp (open)
	// - Port 22/tcp (filtered)
}

// ExampleMapFieldsFromProto demonstrates extracting fields from a proto message.
func ExampleMapFieldsFromProto() {
	// Create a Finding proto
	finding := &taxonomypb.Finding{
		Id:          "finding-1",
		Title:       "SQL Injection Vulnerability",
		Severity:    "high",
		Confidence:  proto.Float64(0.95),
		Category:    proto.String("injection"),
		CvssScore:   proto.Float64(8.5),
		Description: proto.String("Potential SQL injection in login form"),
	}

	// Extract specific fields for use in Neo4j parameters
	params := query.MapFieldsFromProto(finding, []string{"id", "title", "severity", "cvss_score"})

	fmt.Printf("Query parameters:\n")
	for k, v := range params {
		fmt.Printf("  %s: %v\n", k, v)
	}

	// Output would contain (order may vary):
	// Query parameters:
	//   id: finding-1
	//   title: SQL Injection Vulnerability
	//   severity: high
	//   cvss_score: 8.5
}

// ExampleExtractIDFields demonstrates extracting ID-related fields from a proto message.
func ExampleExtractIDFields() {
	// Create a Service proto with parent reference
	service := &taxonomypb.Service{
		Id:           "service-1",
		Name:         "http",
		Product:      proto.String("nginx"),
		Version:      proto.String("1.18.0"),
		ParentPortId: "port-80-tcp",
	}

	// Extract only ID-related fields
	idFields := query.ExtractIDFields(service)

	fmt.Printf("ID fields:\n")
	for k, v := range idFields {
		fmt.Printf("  %s: %v\n", k, v)
	}

	// Output would contain (order may vary):
	// ID fields:
	//   id: service-1
	//   parent_port_id: port-80-tcp
}

// ExampleMapRowToProto_typeConversion demonstrates Neo4j type conversion.
func ExampleMapRowToProto_typeConversion() {
	// Neo4j returns all integers as int64 and all floats as float64
	row := map[string]any{
		"id":         "finding-1",
		"title":      "XSS Vulnerability",
		"severity":   "medium",
		"confidence": float64(0.87), // Neo4j float64
		"cvss_score": float64(6.1),  // Neo4j float64
	}

	finding := &taxonomypb.Finding{}
	err := query.MapRowToProto(row, finding)
	if err != nil {
		panic(err)
	}

	// Values are correctly converted to proto types
	fmt.Printf("Finding: %s\n", finding.Title)
	fmt.Printf("Confidence: %.2f\n", finding.GetConfidence())
	fmt.Printf("CVSS Score: %.1f\n", finding.GetCvssScore())

	// Output:
	// Finding: XSS Vulnerability
	// Confidence: 0.87
	// CVSS Score: 6.1
}

// ExampleMapRowToProto_optionalFields demonstrates handling of optional fields.
func ExampleMapRowToProto_optionalFields() {
	// Row with some fields missing
	row := map[string]any{
		"id": "host-1",
		"ip": "10.0.0.1",
		// No hostname, os, or other optional fields
	}

	host := &taxonomypb.Host{}
	err := query.MapRowToProto(row, host)
	if err != nil {
		panic(err)
	}

	// Required fields are set
	fmt.Printf("Host ID: %s\n", host.Id)
	fmt.Printf("IP: %s\n", host.GetIp())

	// Optional fields are not set (using Get* returns zero value)
	fmt.Printf("Has hostname: %v\n", host.Hostname != nil)
	fmt.Printf("Has OS: %v\n", host.Os != nil)

	// Output:
	// Host ID: host-1
	// IP: 10.0.0.1
	// Has hostname: false
	// Has OS: false
}
