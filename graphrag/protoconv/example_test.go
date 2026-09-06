// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Zero Root AI

package protoconv_test

import (
	"fmt"
	"log"

	taxonomypb "github.com/zeroroot-ai/sdk/api/gen/taxonomy/v1"
	"github.com/zeroroot-ai/sdk/graphrag/protoconv"
)

// ExampleToProperties demonstrates converting a proto message to a property map.
func ExampleToProperties() {
	// Create a host proto with some properties
	ip := "192.168.1.100"
	hostname := "web-server.local"
	os := "Ubuntu"
	osVersion := "22.04"

	host := &taxonomypb.Host{
		Id:        "host-123", // This will be excluded from properties
		Ip:        &ip,
		Hostname:  &hostname,
		Os:        &os,
		OsVersion: &osVersion,
	}

	// Convert to properties
	props, err := protoconv.ToProperties(host)
	if err != nil {
		log.Fatal(err)
	}

	// Properties only include user-facing fields
	fmt.Printf("IP: %v\n", props["ip"])
	fmt.Printf("Hostname: %v\n", props["hostname"])
	fmt.Printf("OS: %v\n", props["os"])
	fmt.Printf("Contains 'id': %v\n", props["id"] != nil)

	// Output:
	// IP: 192.168.1.100
	// Hostname: web-server.local
	// OS: Ubuntu
	// Contains 'id': false
}

// ExampleIdentifyingProperties demonstrates extracting identifying properties.
func ExampleIdentifyingProperties() {
	// Create a port proto
	state := "open"
	reason := "syn-ack"
	port := &taxonomypb.Port{
		Id:           "port-123",
		Number:       443,
		Protocol:     "tcp",
		State:        &state,
		Reason:       &reason,
		ParentHostId: "host-123",
	}

	// Extract only the identifying properties for a port
	// (ports are identified by number + protocol)
	idProps, err := protoconv.IdentifyingProperties("port", port)
	if err != nil {
		log.Fatal(err)
	}

	fmt.Printf("Number: %v\n", idProps["number"])
	fmt.Printf("Protocol: %v\n", idProps["protocol"])
	fmt.Printf("Contains 'state': %v\n", idProps["state"] != nil)

	// Output:
	// Number: 443
	// Protocol: tcp
	// Contains 'state': false
}

// ExampleToProperties_service demonstrates service property conversion.
func ExampleToProperties_service() {
	product := "nginx"
	version := "1.21.0"
	banner := "nginx/1.21.0"

	service := &taxonomypb.Service{
		Id:           "svc-123",
		Name:         "http",
		Product:      &product,
		Version:      &version,
		Banner:       &banner,
		ParentPortId: "port-123", // Framework field, excluded
	}

	props, err := protoconv.ToProperties(service)
	if err != nil {
		log.Fatal(err)
	}

	fmt.Printf("Name: %v\n", props["name"])
	fmt.Printf("Product: %v\n", props["product"])
	fmt.Printf("Version: %v\n", props["version"])

	// Output:
	// Name: http
	// Product: nginx
	// Version: 1.21.0
}

// ExampleIdentifyingProperties_technology demonstrates technology identification.
func ExampleIdentifyingProperties_technology() {
	version := "3.11.0"
	category := "programming-language"

	tech := &taxonomypb.Technology{
		Id:       "tech-123",
		Name:     "Python",
		Version:  &version,
		Category: &category,
	}

	// Technologies are identified by name + version
	idProps, err := protoconv.IdentifyingProperties("technology", tech)
	if err != nil {
		log.Fatal(err)
	}

	fmt.Printf("Name: %v\n", idProps["name"])
	fmt.Printf("Version: %v\n", idProps["version"])
	fmt.Printf("Contains 'category': %v\n", idProps["category"] != nil)

	// Output:
	// Name: Python
	// Version: 3.11.0
	// Contains 'category': false
}

// ExampleToProperties_optionalFields demonstrates handling of optional fields.
func ExampleToProperties_optionalFields() {
	// Create a host with only required fields
	ip := "10.0.0.1"
	host := &taxonomypb.Host{
		Id: "host-456",
		Ip: &ip,
		// No hostname, os, etc.
	}

	props, err := protoconv.ToProperties(host)
	if err != nil {
		log.Fatal(err)
	}

	// Only set fields are included
	fmt.Printf("Number of properties: %d\n", len(props))
	fmt.Printf("IP: %v\n", props["ip"])
	fmt.Printf("Has hostname: %v\n", props["hostname"] != nil)

	// Output:
	// Number of properties: 1
	// IP: 10.0.0.1
	// Has hostname: false
}
