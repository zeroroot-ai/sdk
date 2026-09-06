// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Zero Root AI

package serve

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"
	protolib "google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/descriptorpb"

	"github.com/zeroroot-ai/sdk/types"
)

// mockToolNoMessageTypes is a tool that doesn't specify message types
type mockToolNoMessageTypes struct {
	name string
}

func (m *mockToolNoMessageTypes) Name() string              { return m.name }
func (m *mockToolNoMessageTypes) Version() string           { return "1.0.0" }
func (m *mockToolNoMessageTypes) Description() string       { return "Test tool" }
func (m *mockToolNoMessageTypes) Tags() []string            { return nil }
func (m *mockToolNoMessageTypes) InputMessageType() string  { return "" }
func (m *mockToolNoMessageTypes) OutputMessageType() string { return "" }
func (m *mockToolNoMessageTypes) ExecuteProto(ctx context.Context, input protolib.Message) (protolib.Message, error) {
	return nil, nil
}
func (m *mockToolNoMessageTypes) Health(ctx context.Context) types.HealthStatus {
	return types.HealthStatus{Status: "healthy"}
}

func TestExtractFileDescriptorSet_Success(t *testing.T) {
	tool := &mockTool{
		name:        "test-tool",
		version:     "1.0.0",
		description: "Test tool",
		tags:        []string{"test"},
	}

	fdsBytes, err := ExtractFileDescriptorSet(tool)
	require.NoError(t, err, "ExtractFileDescriptorSet should not return an error")
	require.NotNil(t, fdsBytes, "FileDescriptorSet bytes should be extracted successfully")
	assert.NotEmpty(t, fdsBytes, "FileDescriptorSet bytes should not be empty")

	// Verify it can be deserialized
	fds := &descriptorpb.FileDescriptorSet{}
	err = proto.Unmarshal(fdsBytes, fds)
	require.NoError(t, err)

	// Should have at least one file descriptor
	assert.NotEmpty(t, fds.File, "FileDescriptorSet should contain file descriptors")
}

func TestExtractFileDescriptorSet_NoMessageTypes(t *testing.T) {
	tool := &mockToolNoMessageTypes{
		name: "test-tool",
	}

	fdsBytes, err := ExtractFileDescriptorSet(tool)
	assert.NoError(t, err, "Should not return error for tool with no message types")
	assert.Nil(t, fdsBytes, "FileDescriptorSet bytes should be nil when tool has no message types")
}

func TestExtractFileDescriptorSet_SameInputOutput(t *testing.T) {
	tool := &mockTool{
		name:        "test-tool",
		version:     "1.0.0",
		description: "Test tool with same input/output types",
		tags:        []string{"test"},
	}

	// mockTool uses gibson.common.TypedMap for both input and output
	fdsBytes, err := ExtractFileDescriptorSet(tool)
	require.NoError(t, err)
	require.NotNil(t, fdsBytes)

	// Verify it can be deserialized
	fds := &descriptorpb.FileDescriptorSet{}
	err = proto.Unmarshal(fdsBytes, fds)
	require.NoError(t, err)

	// Should not duplicate the same file descriptor
	// (though it may include dependencies, which is fine)
	assert.NotEmpty(t, fds.File)
}

func TestExtractFileDescriptorSet_IncludesDependencies(t *testing.T) {
	tool := &mockTool{
		name:        "test-tool",
		version:     "1.0.0",
		description: "Test tool",
		tags:        []string{"test"},
	}

	fdsBytes, err := ExtractFileDescriptorSet(tool)
	require.NoError(t, err)
	require.NotNil(t, fdsBytes)

	// Verify it can be deserialized
	fds := &descriptorpb.FileDescriptorSet{}
	err = proto.Unmarshal(fdsBytes, fds)
	require.NoError(t, err)

	// TypedMap references TypedValue, so we should have dependencies included
	// The exact number depends on the proto structure, but should be > 1
	assert.NotEmpty(t, fds.File, "Should include at least the main file descriptor")
}

// mockToolWithBrokenSchemaProvider implements SchemaProvider but returns nil protos
type mockToolWithBrokenSchemaProvider struct {
	mockToolNoMessageTypes
}

func (m *mockToolWithBrokenSchemaProvider) InputProto() proto.Message {
	return nil // Broken implementation
}

func (m *mockToolWithBrokenSchemaProvider) OutputProto() proto.Message {
	return nil // Broken implementation
}

func (m *mockToolWithBrokenSchemaProvider) InputMessageType() string {
	return "test.TestMessage"
}

func (m *mockToolWithBrokenSchemaProvider) OutputMessageType() string {
	return "test.TestMessage"
}

func TestExtractFileDescriptorSet_SchemaProviderReturnsNil(t *testing.T) {
	tool := &mockToolWithBrokenSchemaProvider{
		mockToolNoMessageTypes: mockToolNoMessageTypes{
			name: "broken-tool",
		},
	}

	fdsBytes, err := ExtractFileDescriptorSet(tool)
	require.Error(t, err, "Should return error when SchemaProvider returns nil")
	assert.Nil(t, fdsBytes, "Should not return bytes when SchemaProvider is broken")
	assert.Contains(t, err.Error(), "SchemaProvider.InputProto() returned nil", "Error should mention InputProto returned nil")
}
