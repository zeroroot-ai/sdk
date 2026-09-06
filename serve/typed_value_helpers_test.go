// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Zero Root AI

package serve

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSanitizeUTF8_ValidString(t *testing.T) {
	// Valid UTF-8 strings should pass through unchanged
	tests := []string{
		"",
		"hello world",
		"Hello, 世界",
		"emoji: 🚀",
		"mixed: abc123!@#$%",
	}

	for _, input := range tests {
		result := sanitizeUTF8(input)
		assert.Equal(t, input, result, "Valid UTF-8 should be unchanged")
	}
}

func TestSanitizeUTF8_InvalidBytes(t *testing.T) {
	// Invalid UTF-8 sequences should be replaced with U+FFFD
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "single invalid byte",
			input:    "hello\x80world",
			expected: "hello\uFFFDworld",
		},
		{
			name:     "multiple invalid bytes",
			input:    "\x80\x81\x82",
			expected: "\uFFFD\uFFFD\uFFFD",
		},
		{
			name:     "truncated multi-byte sequence",
			input:    "test\xc2", // incomplete 2-byte sequence
			expected: "test\uFFFD",
		},
		{
			name:     "invalid continuation byte",
			input:    "abc\xc0\x80def", // overlong encoding (invalid)
			expected: "abc\uFFFD\uFFFDdef",
		},
		{
			name:     "binary data mixed with text",
			input:    "Port: 443\x00\xff\xfe open",
			expected: "Port: 443\x00\uFFFD\uFFFD open",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result := sanitizeUTF8(tc.input)
			assert.Equal(t, tc.expected, result)
		})
	}
}

func TestToTypedValue_SanitizesStrings(t *testing.T) {
	// Test that ToTypedValue properly sanitizes invalid UTF-8 strings
	invalidUTF8 := "scan result: \x80\x81\x82 ports found"

	tv := ToTypedValue(invalidUTF8)
	require.NotNil(t, tv)

	// Extract the string value
	stringValue := tv.GetStringValue()
	require.NotEmpty(t, stringValue)

	// Verify it's been sanitized (invalid bytes replaced with replacement char)
	assert.Contains(t, stringValue, "\uFFFD", "Should contain replacement characters")
	assert.Contains(t, stringValue, "scan result:", "Should preserve valid parts")
	assert.Contains(t, stringValue, "ports found", "Should preserve valid parts")
}

func TestToTypedValue_MapWithInvalidStrings(t *testing.T) {
	// Test that maps containing invalid UTF-8 strings are properly sanitized
	input := map[string]any{
		"valid":   "hello world",
		"invalid": "data: \xff\xfe bytes",
		"nested": map[string]any{
			"deep": "nested \x80 value",
		},
	}

	tv := ToTypedValue(input)
	require.NotNil(t, tv)

	// Get the map value
	mapValue := tv.GetMapValue()
	require.NotNil(t, mapValue)

	// Check valid string is preserved
	validEntry := mapValue.Entries["valid"]
	require.NotNil(t, validEntry)
	assert.Equal(t, "hello world", validEntry.GetStringValue())

	// Check invalid string is sanitized
	invalidEntry := mapValue.Entries["invalid"]
	require.NotNil(t, invalidEntry)
	sanitized := invalidEntry.GetStringValue()
	assert.Contains(t, sanitized, "\uFFFD", "Should contain replacement characters")
	assert.Contains(t, sanitized, "data:", "Should preserve valid parts")

	// Check nested map is also sanitized
	nestedEntry := mapValue.Entries["nested"]
	require.NotNil(t, nestedEntry)
	nestedMap := nestedEntry.GetMapValue()
	require.NotNil(t, nestedMap)
	deepEntry := nestedMap.Entries["deep"]
	require.NotNil(t, deepEntry)
	deepSanitized := deepEntry.GetStringValue()
	assert.Contains(t, deepSanitized, "\uFFFD", "Nested strings should also be sanitized")
}

func TestToTypedValue_ArrayWithInvalidStrings(t *testing.T) {
	// Test that arrays containing invalid UTF-8 strings are properly sanitized
	input := []string{
		"valid string",
		"invalid \xff bytes",
		"another \x80\x81 one",
	}

	tv := ToTypedValue(input)
	require.NotNil(t, tv)

	arrayValue := tv.GetArrayValue()
	require.NotNil(t, arrayValue)
	require.Len(t, arrayValue.Items, 3)

	// First item should be unchanged
	assert.Equal(t, "valid string", arrayValue.Items[0].GetStringValue())

	// Second and third items should be sanitized
	assert.Contains(t, arrayValue.Items[1].GetStringValue(), "\uFFFD")
	assert.Contains(t, arrayValue.Items[2].GetStringValue(), "\uFFFD")
}
