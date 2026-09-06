// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Zero Root AI

package graphrag_test

import (
	"fmt"

	graphragpb "github.com/zeroroot-ai/sdk/api/gen/gibson/graphrag/v1"
	testpb "github.com/zeroroot-ai/sdk/api/gen/gibson/test/v1"
	"github.com/zeroroot-ai/sdk/graphrag"
	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/types/dynamicpb"
)

// ExampleExtractDiscovery_compiledType demonstrates extraction from a compiled tool response.
func ExampleExtractDiscovery_compiledType() {
	// Create a tool response with discovery data
	response := &testpb.GenericDiscoveryResponse{
		Discovery: &graphragpb.DiscoveryResult{
			Hosts: []*graphragpb.Host{
				{Ip: "10.0.0.1", Hostname: stringPtr("database-server")},
			},
		},
	}

	// Extract discovery data
	discovery := graphrag.ExtractDiscovery(response)

	if discovery != nil && len(discovery.Hosts) > 0 {
		fmt.Printf("Found host: %s\n", discovery.Hosts[0].Ip)
	}
	// Output: Found host: 10.0.0.1
}

// ExampleExtractDiscovery_dynamicMessage demonstrates extraction from a dynamic proto message.
// This is useful when the proto type isn't compiled into the binary, such as when
// loading tool responses from file descriptors at runtime.
func ExampleExtractDiscovery_dynamicMessage() {
	// Create discovery data
	discovery := &graphragpb.DiscoveryResult{
		Ports: []*graphragpb.Port{
			{Number: 22, Protocol: "tcp", State: stringPtr("open")},
		},
	}

	// Get descriptor for GenericDiscoveryResponse (has field 100)
	desc := (&testpb.GenericDiscoveryResponse{}).ProtoReflect().Descriptor()

	// Create dynamic message (simulating runtime proto resolution)
	dynamicMsg := dynamicpb.NewMessage(desc)

	// Set field 100 (discovery field) by number
	discoveryField := desc.Fields().ByNumber(100)
	dynamicMsg.Set(discoveryField, protoreflect.ValueOfMessage(discovery.ProtoReflect()))

	// Extract discovery data - works with both compiled and dynamic messages
	extracted := graphrag.ExtractDiscovery(dynamicMsg)

	if extracted != nil && len(extracted.Ports) > 0 {
		fmt.Printf("Found port: %d/%s\n", extracted.Ports[0].Number, extracted.Ports[0].Protocol)
	}
	// Output: Found port: 22/tcp
}

// ExampleExtractDiscoveryReflect demonstrates using the reflection-based extractor directly.
func ExampleExtractDiscoveryReflect() {
	response := &testpb.GenericDiscoveryResponse{
		Discovery: &graphragpb.DiscoveryResult{
			Findings: []*graphragpb.Finding{
				{
					Title:    "SQL Injection",
					Severity: "HIGH",
				},
			},
		},
	}

	// Use reflection-based extraction
	discovery := graphrag.ExtractDiscoveryReflect(response)

	if discovery != nil && len(discovery.Findings) > 0 {
		fmt.Printf("Severity: %s\n", discovery.Findings[0].Severity)
	}
	// Output: Severity: HIGH
}

func stringPtr(s string) *string {
	return &s
}
