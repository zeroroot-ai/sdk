// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Zero Root AI

package serve

import (
	"context"
	"errors"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	protolib "google.golang.org/protobuf/proto"

	commonpb "github.com/zeroroot-ai/sdk/api/gen/gibson/common/v1"
	"github.com/zeroroot-ai/sdk/types"
)

// mockTool is a mock implementation of tool.Tool for testing.
type mockTool struct {
	name             string
	version          string
	description      string
	tags             []string
	health           types.HealthStatus
	executeProtoFunc func(ctx context.Context, input protolib.Message) (protolib.Message, error)
}

func (m *mockTool) Name() string        { return m.name }
func (m *mockTool) Version() string     { return m.version }
func (m *mockTool) Description() string { return m.description }
func (m *mockTool) Tags() []string      { return m.tags }

func (m *mockTool) InputMessageType() string {
	return "gibson.common.v1.TypedMap"
}

func (m *mockTool) OutputMessageType() string {
	return "gibson.common.v1.TypedMap"
}

func (m *mockTool) ExecuteProto(ctx context.Context, input protolib.Message) (protolib.Message, error) {
	if m.executeProtoFunc != nil {
		return m.executeProtoFunc(ctx, input)
	}
	// Default implementation: echo back the input with a status field added
	inputMap, ok := input.(*commonpb.TypedMap)
	if !ok {
		return nil, errors.New("invalid input type")
	}
	// Create output map with input entries plus a result field
	outputMap := &commonpb.TypedMap{
		Entries: make(map[string]*commonpb.TypedValue),
	}
	// Copy input entries
	for k, v := range inputMap.Entries {
		outputMap.Entries[k] = v
	}
	// Add result field
	outputMap.Entries["result"] = &commonpb.TypedValue{
		Kind: &commonpb.TypedValue_StringValue{StringValue: "success"},
	}
	return outputMap, nil
}

func (m *mockTool) Health(ctx context.Context) types.HealthStatus {
	if m.health.Status == "" {
		return types.NewHealthyStatus("Tool is healthy")
	}
	return m.health
}

// Tests for subprocess mode detection

func TestHasSchemaFlag(t *testing.T) {
	tests := []struct {
		name     string
		args     []string
		expected bool
	}{
		{
			name:     "with --schema flag",
			args:     []string{"tool", "--schema"},
			expected: true,
		},
		{
			name:     "with --schema flag among other args",
			args:     []string{"tool", "--verbose", "--schema", "--debug"},
			expected: true,
		},
		{
			name:     "without --schema flag",
			args:     []string{"tool"},
			expected: false,
		},
		{
			name:     "with similar but different flag",
			args:     []string{"tool", "--schemas", "--schema-file=foo"},
			expected: false,
		},
		{
			name:     "empty args",
			args:     []string{"tool"},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Save original os.Args
			originalArgs := os.Args
			defer func() { os.Args = originalArgs }()

			os.Args = tt.args
			result := hasSchemaFlag()
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestToolConstants(t *testing.T) {
	// Verify constants match expected values
	assert.Equal(t, "--schema", SchemaFlag)
}
