// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Zero Root AI

package serve

import (
	"bytes"
	"encoding/json"
	"io"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/zeroroot-ai/sdk/enum"
)

func TestOutputSchema_WithoutEnumMappings(t *testing.T) {
	// Clear any existing mappings
	enum.Clear()

	tool := &mockTool{
		name:        "test-tool",
		version:     "1.0.0",
		description: "A test tool",
		tags:        []string{"test"},
	}

	// Capture stdout
	old := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	err := OutputSchema(tool)
	require.NoError(t, err)

	w.Close()
	os.Stdout = old

	var buf bytes.Buffer
	io.Copy(&buf, r)

	// Parse the output
	var schema map[string]interface{}
	err = json.Unmarshal(buf.Bytes(), &schema)
	require.NoError(t, err)

	// Verify basic fields
	assert.Equal(t, "test-tool", schema["name"])
	assert.Equal(t, "1.0.0", schema["version"])
	assert.Equal(t, "A test tool", schema["description"])
	assert.Equal(t, "gibson.common.v1.TypedMap", schema["input_message_type"])
	assert.Equal(t, "gibson.common.v1.TypedMap", schema["output_message_type"])

	// Verify enum_mappings is NOT present (no mappings registered)
	_, exists := schema["enum_mappings"]
	assert.False(t, exists, "enum_mappings should not be present when no mappings are registered")
}

func TestOutputSchema_WithEnumMappings(t *testing.T) {
	// Clear and register test mappings
	enum.Clear()
	enum.RegisterBatch("test-tool", map[string]map[string]string{
		"scanType": {
			"ping": "SCAN_TYPE_PING",
			"syn":  "SCAN_TYPE_SYN",
		},
		"timing": {
			"normal": "TIMING_TEMPLATE_NORMAL",
			"fast":   "TIMING_TEMPLATE_FAST",
		},
	})

	tool := &mockTool{
		name:        "test-tool",
		version:     "1.0.0",
		description: "A test tool with enums",
		tags:        []string{"test"},
	}

	// Capture stdout
	old := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	err := OutputSchema(tool)
	require.NoError(t, err)

	w.Close()
	os.Stdout = old

	var buf bytes.Buffer
	io.Copy(&buf, r)

	// Parse the output
	var schema map[string]interface{}
	err = json.Unmarshal(buf.Bytes(), &schema)
	require.NoError(t, err)

	// Verify basic fields
	assert.Equal(t, "test-tool", schema["name"])
	assert.Equal(t, "1.0.0", schema["version"])

	// Verify enum_mappings IS present
	enumMappings, exists := schema["enum_mappings"]
	require.True(t, exists, "enum_mappings should be present when mappings are registered")

	// Verify the structure of enum_mappings
	mappings, ok := enumMappings.(map[string]interface{})
	require.True(t, ok, "enum_mappings should be a map")

	// Check scanType mappings
	scanType, exists := mappings["scanType"]
	require.True(t, exists)
	scanTypeMap, ok := scanType.(map[string]interface{})
	require.True(t, ok)
	assert.Equal(t, "SCAN_TYPE_PING", scanTypeMap["ping"])
	assert.Equal(t, "SCAN_TYPE_SYN", scanTypeMap["syn"])

	// Check timing mappings
	timing, exists := mappings["timing"]
	require.True(t, exists)
	timingMap, ok := timing.(map[string]interface{})
	require.True(t, ok)
	assert.Equal(t, "TIMING_TEMPLATE_NORMAL", timingMap["normal"])
	assert.Equal(t, "TIMING_TEMPLATE_FAST", timingMap["fast"])
}

func TestOutputSchema_EmptyMappings(t *testing.T) {
	// Clear mappings
	enum.Clear()

	// Register a tool but with empty mappings
	enum.RegisterBatch("test-tool", map[string]map[string]string{})

	tool := &mockTool{
		name:        "test-tool",
		version:     "1.0.0",
		description: "A test tool",
		tags:        []string{"test"},
	}

	// Capture stdout
	old := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	err := OutputSchema(tool)
	require.NoError(t, err)

	w.Close()
	os.Stdout = old

	var buf bytes.Buffer
	io.Copy(&buf, r)

	// Parse the output
	var schema map[string]interface{}
	err = json.Unmarshal(buf.Bytes(), &schema)
	require.NoError(t, err)

	// Verify enum_mappings is NOT present (empty mappings should not be included)
	_, exists := schema["enum_mappings"]
	assert.False(t, exists, "enum_mappings should not be present when mappings are empty")
}

func TestOutputSchema_DifferentTool(t *testing.T) {
	// Clear and register mappings for a different tool
	enum.Clear()
	enum.RegisterBatch("other-tool", map[string]map[string]string{
		"field": {
			"value": "VALUE",
		},
	})

	tool := &mockTool{
		name:        "test-tool",
		version:     "1.0.0",
		description: "A test tool",
		tags:        []string{"test"},
	}

	// Capture stdout
	old := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	err := OutputSchema(tool)
	require.NoError(t, err)

	w.Close()
	os.Stdout = old

	var buf bytes.Buffer
	io.Copy(&buf, r)

	// Parse the output
	var schema map[string]interface{}
	err = json.Unmarshal(buf.Bytes(), &schema)
	require.NoError(t, err)

	// Verify enum_mappings is NOT present (mappings are for a different tool)
	_, exists := schema["enum_mappings"]
	assert.False(t, exists, "enum_mappings should not be present when mappings are for a different tool")
}
