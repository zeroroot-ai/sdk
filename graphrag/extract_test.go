// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Zero Root AI

package graphrag

import (
	"testing"

	graphragpb "github.com/zeroroot-ai/sdk/api/gen/gibson/graphrag/v1"
	testpb "github.com/zeroroot-ai/sdk/api/gen/gibson/test/v1"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protodesc"
	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/reflect/protoregistry"
	"google.golang.org/protobuf/types/descriptorpb"
	"google.golang.org/protobuf/types/dynamicpb"
)

// TestExtractDiscovery_DirectDiscoveryResult tests extraction when message is DiscoveryResult itself
func TestExtractDiscovery_DirectDiscoveryResult(t *testing.T) {
	discovery := &graphragpb.DiscoveryResult{
		Hosts: []*graphragpb.Host{
			{Ip: "10.0.0.1", Hostname: stringPtr("test-host")},
		},
	}

	extracted := ExtractDiscovery(discovery)
	if extracted == nil {
		t.Fatal("expected non-nil result")
	}

	if len(extracted.Hosts) != 1 {
		t.Errorf("expected 1 host, got %d", len(extracted.Hosts))
	}

	if extracted.Hosts[0].Ip != "10.0.0.1" {
		t.Errorf("expected IP 10.0.0.1, got %s", extracted.Hosts[0].Ip)
	}
}

// TestExtractDiscovery_CompiledTypeWithDiscoveryField tests extraction from compiled tool response
func TestExtractDiscovery_CompiledTypeWithDiscoveryField(t *testing.T) {
	discovery := &graphragpb.DiscoveryResult{
		Hosts: []*graphragpb.Host{
			{Ip: "192.168.1.100", Hostname: stringPtr("web-server")},
		},
		Services: []*graphragpb.Service{
			{Name: "http"},
		},
	}

	// Use a real compiled tool response (NmapResponse has field 100)
	nmapResponse := &testpb.GenericDiscoveryResponse{
		Discovery: discovery,
	}

	extracted := ExtractDiscovery(nmapResponse)
	if extracted == nil {
		t.Fatal("expected non-nil result from compiled type")
	}

	if len(extracted.Hosts) != 1 {
		t.Errorf("expected 1 host, got %d", len(extracted.Hosts))
	}

	if extracted.Hosts[0].Ip != "192.168.1.100" {
		t.Errorf("expected IP 192.168.1.100, got %s", extracted.Hosts[0].Ip)
	}
}

// TestExtractDiscovery_DynamicMessage tests extraction from dynamicpb.Message
func TestExtractDiscovery_DynamicMessage(t *testing.T) {
	// Create a DiscoveryResult with test data
	discovery := &graphragpb.DiscoveryResult{
		Hosts: []*graphragpb.Host{
			{Ip: "172.16.0.50", Hostname: stringPtr("dynamic-host")},
		},
		Ports: []*graphragpb.Port{
			{Number: 443, Protocol: "tcp", State: stringPtr("open")},
		},
	}

	// Get the descriptor for NmapResponse (which has field 100)
	nmapResponseDesc := (&testpb.GenericDiscoveryResponse{}).ProtoReflect().Descriptor()

	// Create a dynamic message
	dynamicMsg := dynamicpb.NewMessage(nmapResponseDesc)

	// Get field descriptor for field 100 (discovery)
	discoveryField := nmapResponseDesc.Fields().ByNumber(100)
	if discoveryField == nil {
		t.Fatal("field 100 not found in NmapResponse descriptor")
	}

	// Set the discovery field
	dynamicMsg.Set(discoveryField, protoreflect.ValueOfMessage(discovery.ProtoReflect()))

	// Extract discovery from the dynamic message
	extracted := ExtractDiscovery(dynamicMsg)
	if extracted == nil {
		t.Fatal("expected non-nil result from dynamic message")
	}

	if len(extracted.Hosts) != 1 {
		t.Errorf("expected 1 host, got %d", len(extracted.Hosts))
	}

	if len(extracted.Ports) != 1 {
		t.Errorf("expected 1 port, got %d", len(extracted.Ports))
	}

	if extracted.Hosts[0].Ip != "172.16.0.50" {
		t.Errorf("expected IP 172.16.0.50, got %s", extracted.Hosts[0].Ip)
	}

	if extracted.Ports[0].Number != 443 {
		t.Errorf("expected port 443, got %d", extracted.Ports[0].Number)
	}
}

// TestExtractDiscovery_DynamicMessageFieldNumber100 explicitly tests field number lookup
func TestExtractDiscovery_DynamicMessageFieldNumber100(t *testing.T) {
	discovery := &graphragpb.DiscoveryResult{
		Findings: []*graphragpb.Finding{
			{
				Title:       "Test Finding",
				Severity:    "HIGH",
				Description: stringPtr("A test security finding"),
			},
		},
	}

	// Create dynamic message from HttpxResponse
	httpxResponseDesc := (&testpb.GenericDiscoveryResponse{}).ProtoReflect().Descriptor()
	dynamicMsg := dynamicpb.NewMessage(httpxResponseDesc)

	// Set field 100 directly by number
	discoveryField := httpxResponseDesc.Fields().ByNumber(100)
	if discoveryField == nil {
		t.Fatal("field 100 not found in HttpxResponse descriptor")
	}

	dynamicMsg.Set(discoveryField, protoreflect.ValueOfMessage(discovery.ProtoReflect()))

	// Extract and verify
	extracted := ExtractDiscovery(dynamicMsg)
	if extracted == nil {
		t.Fatal("expected non-nil result from dynamic message with field 100")
	}

	if len(extracted.Findings) != 1 {
		t.Errorf("expected 1 finding, got %d", len(extracted.Findings))
	}

	if extracted.Findings[0].Title != "Test Finding" {
		t.Errorf("expected title 'Test Finding', got %s", extracted.Findings[0].Title)
	}
}

// TestExtractDiscovery_Nil tests nil message handling
func TestExtractDiscovery_Nil(t *testing.T) {
	result := ExtractDiscovery(nil)
	if result != nil {
		t.Errorf("expected nil for nil message, got %v", result)
	}
}

// TestExtractDiscovery_Empty tests empty discovery result
func TestExtractDiscovery_Empty(t *testing.T) {
	discovery := &graphragpb.DiscoveryResult{}

	extracted := ExtractDiscovery(discovery)
	if extracted == nil {
		t.Fatal("expected non-nil result for empty discovery")
	}

	if len(extracted.Hosts) != 0 {
		t.Errorf("expected 0 hosts, got %d", len(extracted.Hosts))
	}
}

// TestExtractDiscovery_MessageWithoutDiscoveryField tests message without field 100
func TestExtractDiscovery_MessageWithoutDiscoveryField(t *testing.T) {
	// Use GraphQuery which doesn't have a discovery field
	query := &graphragpb.GraphQuery{
		Text: "test query",
		TopK: 10,
	}

	result := ExtractDiscovery(query)
	if result != nil {
		t.Errorf("expected nil for message without discovery field, got %v", result)
	}
}

// TestExtractDiscovery_MessageWithUnsetDiscoveryField tests when field 100 exists but is not set
func TestExtractDiscovery_MessageWithUnsetDiscoveryField(t *testing.T) {
	// Create NmapResponse with discovery field unset
	nmapResponse := &testpb.GenericDiscoveryResponse{
		// Discovery field intentionally not set
	}

	result := ExtractDiscovery(nmapResponse)
	if result != nil {
		t.Errorf("expected nil for unset discovery field, got %v", result)
	}
}

// TestExtractDiscoveryReflect_DirectCall tests calling ExtractDiscoveryReflect directly
func TestExtractDiscoveryReflect_DirectCall(t *testing.T) {
	discovery := &graphragpb.DiscoveryResult{
		Domains: []*graphragpb.Domain{
			{Name: "example.com"},
		},
	}

	nucleiResponse := &testpb.GenericDiscoveryResponse{
		Discovery: discovery,
	}

	extracted := ExtractDiscoveryReflect(nucleiResponse)
	if extracted == nil {
		t.Fatal("expected non-nil result from ExtractDiscoveryReflect")
	}

	if len(extracted.Domains) != 1 {
		t.Errorf("expected 1 domain, got %d", len(extracted.Domains))
	}

	if extracted.Domains[0].Name != "example.com" {
		t.Errorf("expected domain 'example.com', got %s", extracted.Domains[0].Name)
	}
}

// TestExtractDiscoveryReflect_NilMessage tests nil handling in reflect function
func TestExtractDiscoveryReflect_NilMessage(t *testing.T) {
	result := ExtractDiscoveryReflect(nil)
	if result != nil {
		t.Errorf("expected nil for nil message in ExtractDiscoveryReflect, got %v", result)
	}
}

// TestExtractDiscovery_MultipleToolTypes tests extraction works across different tool response types
func TestExtractDiscovery_MultipleToolTypes(t *testing.T) {
	testCases := []struct {
		name     string
		msg      proto.Message
		expected bool
	}{
		{
			name: "with hosts (was NmapResponse)",
			msg: &testpb.GenericDiscoveryResponse{
				Discovery: &graphragpb.DiscoveryResult{
					Hosts: []*graphragpb.Host{{Ip: "10.0.0.1"}},
				},
			},
			expected: true,
		},
		{
			name: "with endpoints (was HttpxResponse)",
			msg: &testpb.GenericDiscoveryResponse{
				Discovery: &graphragpb.DiscoveryResult{
					Endpoints: []*graphragpb.Endpoint{{Url: "https://example.com"}},
				},
			},
			expected: true,
		},
		{
			name: "with findings (was NucleiResponse)",
			msg: &testpb.GenericDiscoveryResponse{
				Discovery: &graphragpb.DiscoveryResult{
					Findings: []*graphragpb.Finding{{Title: "XSS"}},
				},
			},
			expected: true,
		},
		{
			name: "with technologies (was WappalyzerResponse)",
			msg: &testpb.GenericDiscoveryResponse{
				Discovery: &graphragpb.DiscoveryResult{
					Technologies: []*graphragpb.Technology{{Name: "nginx"}},
				},
			},
			expected: true,
		},
		{
			name: "with certificates (was TestsslResponse)",
			msg: &testpb.GenericDiscoveryResponse{
				Discovery: &graphragpb.DiscoveryResult{
					Certificates: []*graphragpb.Certificate{{Subject: stringPtr("CN=example.com")}},
				},
			},
			expected: true,
		},
		{
			name:     "empty discovery response",
			msg:      &testpb.GenericDiscoveryResponse{},
			expected: false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			extracted := ExtractDiscovery(tc.msg)
			if tc.expected && extracted == nil {
				t.Errorf("expected non-nil result for %s", tc.name)
			}
			if !tc.expected && extracted != nil {
				t.Errorf("expected nil result for %s, got %v", tc.name, extracted)
			}
		})
	}
}

// TestExtractDiscovery_TableDriven is a comprehensive table-driven test covering all scenarios
func TestExtractDiscovery_TableDriven(t *testing.T) {
	testCases := []struct {
		name            string
		msg             proto.Message
		expectNil       bool
		validateContent func(*testing.T, *graphragpb.DiscoveryResult)
	}{
		{
			name:      "nil message returns nil",
			msg:       nil,
			expectNil: true,
		},
		{
			name: "compiled message WITH field 100 populated",
			msg: &testpb.GenericDiscoveryResponse{
				Discovery: &graphragpb.DiscoveryResult{
					Hosts: []*graphragpb.Host{
						{Ip: "192.168.1.1", Hostname: stringPtr("test-host")},
					},
					Ports: []*graphragpb.Port{
						{Number: 22, Protocol: "tcp", State: stringPtr("open")},
					},
				},
			},
			expectNil: false,
			validateContent: func(t *testing.T, dr *graphragpb.DiscoveryResult) {
				if len(dr.Hosts) != 1 {
					t.Errorf("expected 1 host, got %d", len(dr.Hosts))
				}
				if len(dr.Ports) != 1 {
					t.Errorf("expected 1 port, got %d", len(dr.Ports))
				}
				if dr.Hosts[0].Ip != "192.168.1.1" {
					t.Errorf("expected IP 192.168.1.1, got %s", dr.Hosts[0].Ip)
				}
			},
		},
		{
			name: "compiled message WITHOUT field 100 (not set)",
			msg:  &testpb.GenericDiscoveryResponse{
				// Discovery field intentionally not set
			},
			expectNil: true,
		},
		{
			name: "message without discovery field (GraphQuery)",
			msg: &graphragpb.GraphQuery{
				Text: "test query",
				TopK: 10,
			},
			expectNil: true,
		},
		{
			name: "direct DiscoveryResult",
			msg: &graphragpb.DiscoveryResult{
				Services: []*graphragpb.Service{
					{Name: "http", PortId: "port-123"},
				},
			},
			expectNil: false,
			validateContent: func(t *testing.T, dr *graphragpb.DiscoveryResult) {
				if len(dr.Services) != 1 {
					t.Errorf("expected 1 service, got %d", len(dr.Services))
				}
				if dr.Services[0].Name != "http" {
					t.Errorf("expected service name 'http', got %s", dr.Services[0].Name)
				}
			},
		},
		{
			name: "empty DiscoveryResult (default values)",
			msg:  &graphragpb.DiscoveryResult{
				// All fields at default/empty values
			},
			expectNil: false,
			validateContent: func(t *testing.T, dr *graphragpb.DiscoveryResult) {
				if len(dr.Hosts) != 0 {
					t.Errorf("expected 0 hosts, got %d", len(dr.Hosts))
				}
				if len(dr.Services) != 0 {
					t.Errorf("expected 0 services, got %d", len(dr.Services))
				}
			},
		},
		{
			name: "multiple discovery fields populated",
			msg: &testpb.GenericDiscoveryResponse{
				Discovery: &graphragpb.DiscoveryResult{
					Hosts:        []*graphragpb.Host{{Ip: "10.0.0.1"}},
					Endpoints:    []*graphragpb.Endpoint{{Url: "https://test.com"}},
					Technologies: []*graphragpb.Technology{{Name: "nginx"}},
					Findings:     []*graphragpb.Finding{{Title: "Test", Severity: "LOW"}},
				},
			},
			expectNil: false,
			validateContent: func(t *testing.T, dr *graphragpb.DiscoveryResult) {
				if len(dr.Hosts) != 1 {
					t.Errorf("expected 1 host, got %d", len(dr.Hosts))
				}
				if len(dr.Endpoints) != 1 {
					t.Errorf("expected 1 endpoint, got %d", len(dr.Endpoints))
				}
				if len(dr.Technologies) != 1 {
					t.Errorf("expected 1 technology, got %d", len(dr.Technologies))
				}
				if len(dr.Findings) != 1 {
					t.Errorf("expected 1 finding, got %d", len(dr.Findings))
				}
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := ExtractDiscovery(tc.msg)

			if tc.expectNil {
				if result != nil {
					t.Errorf("expected nil result, got %+v", result)
				}
				return
			}

			if result == nil {
				t.Fatal("expected non-nil result, got nil")
			}

			if tc.validateContent != nil {
				tc.validateContent(t, result)
			}
		})
	}
}

// TestExtractDiscovery_WrongFieldType tests when field 100 exists but is not a message type
func TestExtractDiscovery_WrongFieldType(t *testing.T) {
	// Create a custom message descriptor with field 100 as a string (wrong type)
	fileDesc := &descriptorpb.FileDescriptorProto{
		Name:    proto.String("test.proto"),
		Package: proto.String("test"),
		MessageType: []*descriptorpb.DescriptorProto{
			{
				Name: proto.String("TestMessage"),
				Field: []*descriptorpb.FieldDescriptorProto{
					{
						Name:   proto.String("discovery"),
						Number: proto.Int32(100),
						Type:   descriptorpb.FieldDescriptorProto_TYPE_STRING.Enum(),
						Label:  descriptorpb.FieldDescriptorProto_LABEL_OPTIONAL.Enum(),
					},
				},
			},
		},
	}

	// Build the file descriptor
	files := &protoregistry.Files{}
	fd, err := protodesc.NewFile(fileDesc, files)
	if err != nil {
		t.Fatalf("failed to create file descriptor: %v", err)
	}

	// Get the message descriptor
	msgDesc := fd.Messages().Get(0)

	// Create a dynamic message
	dynamicMsg := dynamicpb.NewMessage(msgDesc)

	// Set field 100 to a string value (wrong type for discovery)
	field100 := msgDesc.Fields().ByNumber(100)
	dynamicMsg.Set(field100, protoreflect.ValueOfString("this is not a message"))

	// Try to extract discovery - should return nil with a warning log
	result := ExtractDiscovery(dynamicMsg)
	if result != nil {
		t.Errorf("expected nil for wrong field type, got %+v", result)
	}
}

// TestExtractDiscovery_MalformedDynamicMessage tests extraction from a dynamic message with incompatible structure
func TestExtractDiscovery_MalformedDynamicMessage(t *testing.T) {
	// Create a custom message descriptor with field 100 as a message, but with incompatible structure
	fileDesc := &descriptorpb.FileDescriptorProto{
		Name:    proto.String("malformed.proto"),
		Package: proto.String("malformed"),
		MessageType: []*descriptorpb.DescriptorProto{
			{
				Name: proto.String("IncompatibleDiscovery"),
				Field: []*descriptorpb.FieldDescriptorProto{
					{
						Name:   proto.String("wrong_field"),
						Number: proto.Int32(1),
						Type:   descriptorpb.FieldDescriptorProto_TYPE_STRING.Enum(),
						Label:  descriptorpb.FieldDescriptorProto_LABEL_OPTIONAL.Enum(),
					},
				},
			},
			{
				Name: proto.String("TestResponse"),
				Field: []*descriptorpb.FieldDescriptorProto{
					{
						Name:     proto.String("discovery"),
						Number:   proto.Int32(100),
						Type:     descriptorpb.FieldDescriptorProto_TYPE_MESSAGE.Enum(),
						TypeName: proto.String(".malformed.IncompatibleDiscovery"),
						Label:    descriptorpb.FieldDescriptorProto_LABEL_OPTIONAL.Enum(),
					},
				},
			},
		},
	}

	// Build the file descriptor
	files := &protoregistry.Files{}
	fd, err := protodesc.NewFile(fileDesc, files)
	if err != nil {
		t.Fatalf("failed to create file descriptor: %v", err)
	}

	// Get the message descriptors
	incompatibleDiscoveryDesc := fd.Messages().ByName("IncompatibleDiscovery")
	testResponseDesc := fd.Messages().ByName("TestResponse")

	// Create a dynamic message for IncompatibleDiscovery
	incompatibleMsg := dynamicpb.NewMessage(incompatibleDiscoveryDesc)
	field1 := incompatibleDiscoveryDesc.Fields().ByNumber(1)
	incompatibleMsg.Set(field1, protoreflect.ValueOfString("this will fail to unmarshal"))

	// Create a dynamic message for TestResponse
	dynamicMsg := dynamicpb.NewMessage(testResponseDesc)
	field100 := testResponseDesc.Fields().ByNumber(100)
	dynamicMsg.Set(field100, protoreflect.ValueOfMessage(incompatibleMsg.ProtoReflect()))

	// Try to extract discovery - should return nil due to unmarshal error
	result := ExtractDiscovery(dynamicMsg)
	if result != nil {
		t.Errorf("expected nil for incompatible message structure, got %+v", result)
	}
}

// TestExtractDiscovery_DynamicMessageTableDriven tests dynamic message scenarios
func TestExtractDiscovery_DynamicMessageTableDriven(t *testing.T) {
	testCases := []struct {
		name            string
		setupMsg        func() proto.Message
		expectNil       bool
		validateContent func(*testing.T, *graphragpb.DiscoveryResult)
	}{
		{
			name: "dynamicpb.Message WITH field 100 populated",
			setupMsg: func() proto.Message {
				discovery := &graphragpb.DiscoveryResult{
					Hosts: []*graphragpb.Host{
						{Ip: "172.16.0.1", Hostname: stringPtr("dynamic-test")},
					},
				}

				nmapResponseDesc := (&testpb.GenericDiscoveryResponse{}).ProtoReflect().Descriptor()
				dynamicMsg := dynamicpb.NewMessage(nmapResponseDesc)
				discoveryField := nmapResponseDesc.Fields().ByNumber(100)
				dynamicMsg.Set(discoveryField, protoreflect.ValueOfMessage(discovery.ProtoReflect()))

				return dynamicMsg
			},
			expectNil: false,
			validateContent: func(t *testing.T, dr *graphragpb.DiscoveryResult) {
				if len(dr.Hosts) != 1 {
					t.Errorf("expected 1 host, got %d", len(dr.Hosts))
				}
				if dr.Hosts[0].Ip != "172.16.0.1" {
					t.Errorf("expected IP 172.16.0.1, got %s", dr.Hosts[0].Ip)
				}
			},
		},
		{
			name: "dynamicpb.Message WITHOUT field 100 set",
			setupMsg: func() proto.Message {
				nmapResponseDesc := (&testpb.GenericDiscoveryResponse{}).ProtoReflect().Descriptor()
				dynamicMsg := dynamicpb.NewMessage(nmapResponseDesc)
				// Don't set field 100
				return dynamicMsg
			},
			expectNil: true,
		},
		{
			name: "dynamicpb.Message with all discovery types",
			setupMsg: func() proto.Message {
				discovery := &graphragpb.DiscoveryResult{
					Hosts:        []*graphragpb.Host{{Ip: "10.1.1.1"}},
					Ports:        []*graphragpb.Port{{Number: 443, Protocol: "tcp"}},
					Services:     []*graphragpb.Service{{Name: "https"}},
					Endpoints:    []*graphragpb.Endpoint{{Url: "https://api.test.com"}},
					Domains:      []*graphragpb.Domain{{Name: "test.com"}},
					Technologies: []*graphragpb.Technology{{Name: "React"}},
					Findings:     []*graphragpb.Finding{{Title: "Open Port", Severity: "INFO"}},
				}

				httpxResponseDesc := (&testpb.GenericDiscoveryResponse{}).ProtoReflect().Descriptor()
				dynamicMsg := dynamicpb.NewMessage(httpxResponseDesc)
				discoveryField := httpxResponseDesc.Fields().ByNumber(100)
				dynamicMsg.Set(discoveryField, protoreflect.ValueOfMessage(discovery.ProtoReflect()))

				return dynamicMsg
			},
			expectNil: false,
			validateContent: func(t *testing.T, dr *graphragpb.DiscoveryResult) {
				if len(dr.Hosts) != 1 {
					t.Errorf("expected 1 host, got %d", len(dr.Hosts))
				}
				if len(dr.Ports) != 1 {
					t.Errorf("expected 1 port, got %d", len(dr.Ports))
				}
				if len(dr.Services) != 1 {
					t.Errorf("expected 1 service, got %d", len(dr.Services))
				}
				if len(dr.Endpoints) != 1 {
					t.Errorf("expected 1 endpoint, got %d", len(dr.Endpoints))
				}
				if len(dr.Domains) != 1 {
					t.Errorf("expected 1 domain, got %d", len(dr.Domains))
				}
				if len(dr.Technologies) != 1 {
					t.Errorf("expected 1 technology, got %d", len(dr.Technologies))
				}
				if len(dr.Findings) != 1 {
					t.Errorf("expected 1 finding, got %d", len(dr.Findings))
				}
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			msg := tc.setupMsg()
			result := ExtractDiscovery(msg)

			if tc.expectNil {
				if result != nil {
					t.Errorf("expected nil result, got %+v", result)
				}
				return
			}

			if result == nil {
				t.Fatal("expected non-nil result, got nil")
			}

			if tc.validateContent != nil {
				tc.validateContent(t, result)
			}
		})
	}
}
