// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Zero Root AI

package protoresolver

import (
	"context"
	"encoding/base64"
	"testing"

	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/descriptorpb"
	"google.golang.org/protobuf/types/known/emptypb"
)

// createTestFileDescriptorSet creates a simple FileDescriptorSet for testing.
// It contains a message type named "test.TestMessage" with a single string field.
func createTestFileDescriptorSet() string {
	// Create a simple proto file descriptor
	fileDescriptor := &descriptorpb.FileDescriptorProto{
		Name:    proto.String("test.proto"),
		Package: proto.String("test"),
		MessageType: []*descriptorpb.DescriptorProto{
			{
				Name: proto.String("TestMessage"),
				Field: []*descriptorpb.FieldDescriptorProto{
					{
						Name:   proto.String("message"),
						Number: proto.Int32(1),
						Type:   descriptorpb.FieldDescriptorProto_TYPE_STRING.Enum(),
						Label:  descriptorpb.FieldDescriptorProto_LABEL_OPTIONAL.Enum(),
					},
				},
			},
			{
				Name: proto.String("AnotherMessage"),
				Field: []*descriptorpb.FieldDescriptorProto{
					{
						Name:   proto.String("value"),
						Number: proto.Int32(1),
						Type:   descriptorpb.FieldDescriptorProto_TYPE_INT32.Enum(),
						Label:  descriptorpb.FieldDescriptorProto_LABEL_OPTIONAL.Enum(),
					},
				},
			},
		},
	}

	// Create FileDescriptorSet with the file
	fds := &descriptorpb.FileDescriptorSet{
		File: []*descriptorpb.FileDescriptorProto{fileDescriptor},
	}

	// Marshal to bytes and encode as base64
	fdsBytes, err := proto.Marshal(fds)
	if err != nil {
		panic("failed to marshal test FileDescriptorSet: " + err.Error())
	}

	return base64.StdEncoding.EncodeToString(fdsBytes)
}

// TestNewDefaultProtoResolver tests resolver creation.
func TestNewDefaultProtoResolver(t *testing.T) {
	config := DefaultConfig()
	resolver := NewDefaultProtoResolver(config)

	if resolver == nil {
		t.Fatal("NewDefaultProtoResolver returned nil")
	}
	if resolver.cache == nil {
		t.Error("resolver.cache is nil")
	}
	if resolver.factory == nil {
		t.Error("resolver.factory is nil")
	}
	if resolver.logger == nil {
		t.Error("resolver.logger is nil")
	}
	if resolver.config.CacheMaxEntries != config.CacheMaxEntries {
		t.Error("resolver.config not properly set")
	}
}

// TestResolveInputType_GlobalTypesHit tests resolution from GlobalTypes.
func TestResolveInputType_GlobalTypesHit(t *testing.T) {
	resolver := NewDefaultProtoResolver(DefaultConfig())
	ctx := context.Background()

	// Use google.protobuf.Empty as it's always in GlobalTypes
	msg, err := resolver.ResolveInputType(ctx, "google.protobuf.Empty", nil)
	if err != nil {
		t.Fatalf("ResolveInputType failed: %v", err)
	}

	if msg == nil {
		t.Fatal("ResolveInputType returned nil message")
	}

	// Verify it's the correct type
	if _, ok := msg.(*emptypb.Empty); !ok {
		t.Errorf("Expected *emptypb.Empty, got %T", msg)
	}
}

// TestResolveOutputType_GlobalTypesHit tests resolution from GlobalTypes.
func TestResolveOutputType_GlobalTypesHit(t *testing.T) {
	resolver := NewDefaultProtoResolver(DefaultConfig())
	ctx := context.Background()

	// Use google.protobuf.Empty as it's always in GlobalTypes
	msg, err := resolver.ResolveOutputType(ctx, "google.protobuf.Empty", nil)
	if err != nil {
		t.Fatalf("ResolveOutputType failed: %v", err)
	}

	if msg == nil {
		t.Fatal("ResolveOutputType returned nil message")
	}

	// Verify it's the correct type
	if _, ok := msg.(*emptypb.Empty); !ok {
		t.Errorf("Expected *emptypb.Empty, got %T", msg)
	}
}

// TestResolveInputType_FileDescriptorSetFallback tests dynamic resolution.
func TestResolveInputType_FileDescriptorSetFallback(t *testing.T) {
	resolver := NewDefaultProtoResolver(DefaultConfig())
	ctx := context.Background()

	// Create metadata with test FileDescriptorSet
	fdsBase64 := createTestFileDescriptorSet()
	metadata := map[string]string{
		"file_descriptor_set": fdsBase64,
		"tool_name":           "test-tool",
	}

	// Resolve a type from the test FDS
	msg, err := resolver.ResolveInputType(ctx, "test.TestMessage", metadata)
	if err != nil {
		t.Fatalf("ResolveInputType failed: %v", err)
	}

	if msg == nil {
		t.Fatal("ResolveInputType returned nil message")
	}

	// Verify it's a proto.Message (will be dynamicpb.Message)
	if !msg.ProtoReflect().IsValid() {
		t.Error("Message is not valid")
	}

	// Verify the message descriptor has the expected type name
	if msg.ProtoReflect().Descriptor().FullName() != "test.TestMessage" {
		t.Errorf("Type name = %s, want test.TestMessage", msg.ProtoReflect().Descriptor().FullName())
	}
}

// TestResolveInputType_InvalidBase64 tests error handling for invalid base64.
func TestResolveInputType_InvalidBase64(t *testing.T) {
	resolver := NewDefaultProtoResolver(DefaultConfig())
	ctx := context.Background()

	metadata := map[string]string{
		"file_descriptor_set": "invalid-base64!!!",
		"tool_name":           "test-tool",
	}

	_, err := resolver.ResolveInputType(ctx, "test.Type", metadata)
	if err == nil {
		t.Fatal("Expected error for invalid base64, got nil")
	}
}

// TestResolveInputType_InvalidProto tests error handling for invalid proto bytes.
func TestResolveInputType_InvalidProto(t *testing.T) {
	resolver := NewDefaultProtoResolver(DefaultConfig())
	ctx := context.Background()

	// Valid base64 but invalid proto bytes
	invalidProto := base64.StdEncoding.EncodeToString([]byte("not a valid proto"))
	metadata := map[string]string{
		"file_descriptor_set": invalidProto,
		"tool_name":           "test-tool",
	}

	_, err := resolver.ResolveInputType(ctx, "test.Type", metadata)
	if err == nil {
		t.Fatal("Expected error for invalid proto, got nil")
	}
}

// TestResolveInputType_TypeNotFound tests error when type is not in FDS.
func TestResolveInputType_TypeNotFound(t *testing.T) {
	resolver := NewDefaultProtoResolver(DefaultConfig())
	ctx := context.Background()

	fdsBase64 := createTestFileDescriptorSet()
	metadata := map[string]string{
		"file_descriptor_set": fdsBase64,
		"tool_name":           "test-tool",
	}

	// Try to resolve a type that doesn't exist in the FDS
	_, err := resolver.ResolveInputType(ctx, "test.NonexistentType", metadata)
	if err == nil {
		t.Fatal("Expected error for type not found, got nil")
	}
}

// TestUnmarshalJSON_ValidJSON tests unmarshaling valid JSON.
func TestUnmarshalJSON_ValidJSON(t *testing.T) {
	resolver := NewDefaultProtoResolver(DefaultConfig())
	ctx := context.Background()

	fdsBase64 := createTestFileDescriptorSet()
	metadata := map[string]string{
		"file_descriptor_set": fdsBase64,
		"tool_name":           "test-tool",
	}

	// Valid JSON for test.TestMessage
	jsonData := []byte(`{"message": "hello world"}`)

	msg, err := resolver.UnmarshalProtoJSON(ctx, "test.TestMessage", jsonData, metadata)
	if err != nil {
		t.Fatalf("UnmarshalJSON failed: %v", err)
	}

	if msg == nil {
		t.Fatal("UnmarshalJSON returned nil message")
	}

	// Verify the message was unmarshaled correctly
	msgReflect := msg.ProtoReflect()
	fields := msgReflect.Descriptor().Fields()
	msgField := fields.ByName("message")
	if msgField == nil {
		t.Fatal("Field 'message' not found in descriptor")
	}

	value := msgReflect.Get(msgField).String()
	if value != "hello world" {
		t.Errorf("message = %q, want %q", value, "hello world")
	}
}

// TestUnmarshalJSON_InvalidJSON tests error handling for invalid JSON.
func TestUnmarshalJSON_InvalidJSON(t *testing.T) {
	resolver := NewDefaultProtoResolver(DefaultConfig())
	ctx := context.Background()

	fdsBase64 := createTestFileDescriptorSet()
	metadata := map[string]string{
		"file_descriptor_set": fdsBase64,
		"tool_name":           "test-tool",
	}

	// Invalid JSON
	jsonData := []byte(`{invalid json}`)

	_, err := resolver.UnmarshalProtoJSON(ctx, "test.TestMessage", jsonData, metadata)
	if err == nil {
		t.Fatal("Expected error for invalid JSON, got nil")
	}
}

// TestUnmarshalJSON_TypeResolutionFails tests error when type can't be resolved.
func TestUnmarshalJSON_TypeResolutionFails(t *testing.T) {
	resolver := NewDefaultProtoResolver(DefaultConfig())
	ctx := context.Background()

	// Metadata without file_descriptor_set
	metadata := map[string]string{
		"tool_name": "test-tool",
	}

	jsonData := []byte(`{"message": "test"}`)

	_, err := resolver.UnmarshalProtoJSON(ctx, "nonexistent.Type", jsonData, metadata)
	if err == nil {
		t.Fatal("Expected error for type resolution failure, got nil")
	}
}

// TestUnmarshalJSON_GlobalType tests unmarshaling with global type.
func TestUnmarshalJSON_GlobalType(t *testing.T) {
	resolver := NewDefaultProtoResolver(DefaultConfig())
	ctx := context.Background()

	// Use google.protobuf.Empty (always in GlobalTypes)
	jsonData := []byte(`{}`)

	msg, err := resolver.UnmarshalProtoJSON(ctx, "google.protobuf.Empty", jsonData, nil)
	if err != nil {
		t.Fatalf("UnmarshalJSON failed: %v", err)
	}

	if msg == nil {
		t.Fatal("UnmarshalJSON returned nil message")
	}

	if _, ok := msg.(*emptypb.Empty); !ok {
		t.Errorf("Expected *emptypb.Empty, got %T", msg)
	}
}

// TestResolveInputType_DefaultToolName tests behavior when tool_name is missing.
func TestResolveInputType_DefaultToolName(t *testing.T) {
	resolver := NewDefaultProtoResolver(DefaultConfig())
	ctx := context.Background()

	fdsBase64 := createTestFileDescriptorSet()
	metadata := map[string]string{
		"file_descriptor_set": fdsBase64,
		// tool_name is intentionally missing
	}

	// Should still work, using "default" as tool name
	msg, err := resolver.ResolveInputType(ctx, "test.TestMessage", metadata)
	if err != nil {
		t.Fatalf("ResolveInputType failed: %v", err)
	}

	if msg == nil {
		t.Fatal("ResolveInputType returned nil message")
	}
}

// TestResolveInputType_EmptyToolName tests behavior when tool_name is empty string.
func TestResolveInputType_EmptyToolName(t *testing.T) {
	resolver := NewDefaultProtoResolver(DefaultConfig())
	ctx := context.Background()

	fdsBase64 := createTestFileDescriptorSet()
	metadata := map[string]string{
		"file_descriptor_set": fdsBase64,
		"tool_name":           "",
	}

	// Should still work, using "default" as tool name
	msg, err := resolver.ResolveInputType(ctx, "test.TestMessage", metadata)
	if err != nil {
		t.Fatalf("ResolveInputType failed: %v", err)
	}

	if msg == nil {
		t.Fatal("ResolveInputType returned nil message")
	}
}

// TestResolveType_MultipleCalls tests resolving multiple types from same FDS.
func TestResolveType_MultipleCalls(t *testing.T) {
	resolver := NewDefaultProtoResolver(DefaultConfig())
	ctx := context.Background()

	fdsBase64 := createTestFileDescriptorSet()
	metadata := map[string]string{
		"file_descriptor_set": fdsBase64,
		"tool_name":           "test-tool",
	}

	// Resolve first type
	msg1, err := resolver.ResolveInputType(ctx, "test.TestMessage", metadata)
	if err != nil {
		t.Fatalf("ResolveInputType for TestMessage failed: %v", err)
	}

	// Resolve second type from same FDS
	msg2, err := resolver.ResolveInputType(ctx, "test.AnotherMessage", metadata)
	if err != nil {
		t.Fatalf("ResolveInputType for AnotherMessage failed: %v", err)
	}

	if msg1 == nil || msg2 == nil {
		t.Fatal("One or both messages are nil")
	}

	// Verify types are different
	if msg1.ProtoReflect().Descriptor().FullName() == msg2.ProtoReflect().Descriptor().FullName() {
		t.Error("Expected different types, got same type")
	}
}

// TestResolveOutputType_AllScenarios tests ResolveOutputType with various scenarios.
func TestResolveOutputType_AllScenarios(t *testing.T) {
	tests := []struct {
		name      string
		typeName  string
		metadata  map[string]string
		wantError bool
	}{
		{
			name:      "global type",
			typeName:  "google.protobuf.Empty",
			metadata:  nil,
			wantError: false,
		},
		{
			name:     "dynamic type",
			typeName: "test.TestMessage",
			metadata: map[string]string{
				"file_descriptor_set": createTestFileDescriptorSet(),
				"tool_name":           "test-tool",
			},
			wantError: false,
		},
		{
			name:     "missing schema",
			typeName: "nonexistent.Type",
			metadata: map[string]string{
				"tool_name": "test-tool",
			},
			wantError: true,
		},
		{
			name:     "type not in FDS",
			typeName: "test.NonexistentType",
			metadata: map[string]string{
				"file_descriptor_set": createTestFileDescriptorSet(),
				"tool_name":           "test-tool",
			},
			wantError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resolver := NewDefaultProtoResolver(DefaultConfig())
			ctx := context.Background()

			msg, err := resolver.ResolveOutputType(ctx, tt.typeName, tt.metadata)

			if tt.wantError {
				if err == nil {
					t.Error("Expected error, got nil")
				}
			} else {
				if err != nil {
					t.Errorf("Expected no error, got: %v", err)
				}
				if msg == nil {
					t.Error("Expected message, got nil")
				}
			}
		})
	}
}

// BenchmarkResolveInputType_GlobalTypes benchmarks resolution from GlobalTypes.
func BenchmarkResolveInputType_GlobalTypes(b *testing.B) {
	resolver := NewDefaultProtoResolver(DefaultConfig())
	ctx := context.Background()

	b.ResetTimer()
	for range b.N {
		_, err := resolver.ResolveInputType(ctx, "google.protobuf.Empty", nil)
		if err != nil {
			b.Fatal(err)
		}
	}
}

// BenchmarkResolveInputType_DynamicCached benchmarks cached dynamic resolution.
func BenchmarkResolveInputType_DynamicCached(b *testing.B) {
	resolver := NewDefaultProtoResolver(DefaultConfig())
	ctx := context.Background()

	fdsBase64 := createTestFileDescriptorSet()
	metadata := map[string]string{
		"file_descriptor_set": fdsBase64,
		"tool_name":           "test-tool",
	}

	// Prime the cache
	_, err := resolver.ResolveInputType(ctx, "test.TestMessage", metadata)
	if err != nil {
		b.Fatal(err)
	}

	b.ResetTimer()
	for range b.N {
		_, err := resolver.ResolveInputType(ctx, "test.TestMessage", metadata)
		if err != nil {
			b.Fatal(err)
		}
	}
}

// BenchmarkUnmarshalJSON benchmarks JSON unmarshaling.
func BenchmarkUnmarshalJSON(b *testing.B) {
	resolver := NewDefaultProtoResolver(DefaultConfig())
	ctx := context.Background()

	fdsBase64 := createTestFileDescriptorSet()
	metadata := map[string]string{
		"file_descriptor_set": fdsBase64,
		"tool_name":           "test-tool",
	}

	jsonData := []byte(`{"message": "hello world"}`)

	b.ResetTimer()
	for range b.N {
		_, err := resolver.UnmarshalProtoJSON(ctx, "test.TestMessage", jsonData, metadata)
		if err != nil {
			b.Fatal(err)
		}
	}
}
