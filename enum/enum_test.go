// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Zero Root AI

package enum

import (
	"encoding/json"
	"sync"
	"testing"
)

func TestRegister(t *testing.T) {
	Clear()

	mappings := map[string]string{
		"syn":     "SYN_SCAN",
		"ack":     "ACK_SCAN",
		"connect": "CONNECT_SCAN",
	}

	Register("mytool-a", "scan_type", mappings)

	// Verify mappings are stored correctly
	toolMappings := GetMappings("mytool-a")
	if toolMappings == nil {
		t.Fatal("Expected tool mappings to be registered, got nil")
	}

	fieldMappings, exists := toolMappings["scan_type"]
	if !exists {
		t.Fatal("Expected field mappings for 'scan_type', not found")
	}

	// Verify lowercase keys (case-insensitive storage)
	expectedMappings := map[string]string{
		"syn":     "SYN_SCAN",
		"ack":     "ACK_SCAN",
		"connect": "CONNECT_SCAN",
	}

	for shortValue, expectedProtoName := range expectedMappings {
		protoName, found := fieldMappings[shortValue]
		if !found {
			t.Errorf("Expected mapping for '%s', not found", shortValue)
			continue
		}
		if protoName != expectedProtoName {
			t.Errorf("For '%s': expected '%s', got '%s'", shortValue, expectedProtoName, protoName)
		}
	}

	if len(fieldMappings) != len(expectedMappings) {
		t.Errorf("Expected %d mappings, got %d", len(expectedMappings), len(fieldMappings))
	}
}

func TestRegisterBatch(t *testing.T) {
	Clear()

	fieldMappings := map[string]map[string]string{
		"scan_type": {
			"syn":     "SYN_SCAN",
			"ack":     "ACK_SCAN",
			"connect": "CONNECT_SCAN",
		},
		"timing": {
			"fast":   "TIMING_FAST",
			"slow":   "TIMING_SLOW",
			"normal": "TIMING_NORMAL",
		},
		"output_format": {
			"xml":  "OUTPUT_XML",
			"json": "OUTPUT_JSON",
		},
	}

	RegisterBatch("mytool-a", fieldMappings)

	// Verify all fields are registered
	toolMappings := GetMappings("mytool-a")
	if toolMappings == nil {
		t.Fatal("Expected tool mappings to be registered, got nil")
	}

	if len(toolMappings) != 3 {
		t.Errorf("Expected 3 fields registered, got %d", len(toolMappings))
	}

	// Verify scan_type field
	scanTypeMappings, exists := toolMappings["scan_type"]
	if !exists {
		t.Error("Expected 'scan_type' field to be registered")
	} else if len(scanTypeMappings) != 3 {
		t.Errorf("Expected 3 scan_type mappings, got %d", len(scanTypeMappings))
	}

	// Verify timing field
	timingMappings, exists := toolMappings["timing"]
	if !exists {
		t.Error("Expected 'timing' field to be registered")
	} else if len(timingMappings) != 3 {
		t.Errorf("Expected 3 timing mappings, got %d", len(timingMappings))
	}

	// Verify output_format field
	outputMappings, exists := toolMappings["output_format"]
	if !exists {
		t.Error("Expected 'output_format' field to be registered")
	} else if len(outputMappings) != 2 {
		t.Errorf("Expected 2 output_format mappings, got %d", len(outputMappings))
	}
}

func TestNormalize(t *testing.T) {
	Clear()

	Register("mytool-a", "scan_type", map[string]string{
		"syn":     "SYN_SCAN",
		"ack":     "ACK_SCAN",
		"connect": "CONNECT_SCAN",
	})

	Register("mytool-a", "timing", map[string]string{
		"fast": "TIMING_FAST",
		"slow": "TIMING_SLOW",
	})

	tests := []struct {
		name     string
		input    string
		expected map[string]interface{}
	}{
		{
			name:  "Single field normalization",
			input: `{"scan_type": "syn", "target": "example.com"}`,
			expected: map[string]interface{}{
				"scan_type": "SYN_SCAN",
				"target":    "example.com",
			},
		},
		{
			name:  "Multiple field normalization",
			input: `{"scan_type": "ack", "timing": "fast", "target": "example.com"}`,
			expected: map[string]interface{}{
				"scan_type": "ACK_SCAN",
				"timing":    "TIMING_FAST",
				"target":    "example.com",
			},
		},
		{
			name:  "Unmapped value passes through",
			input: `{"scan_type": "unknown", "target": "example.com"}`,
			expected: map[string]interface{}{
				"scan_type": "unknown",
				"target":    "example.com",
			},
		},
		{
			name:  "Field not in mappings passes through",
			input: `{"port": 80, "target": "example.com"}`,
			expected: map[string]interface{}{
				"port":   float64(80), // JSON unmarshals numbers as float64
				"target": "example.com",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := Normalize("mytool-a", tt.input)

			// Parse result to compare
			var resultData map[string]interface{}
			if err := json.Unmarshal([]byte(result), &resultData); err != nil {
				t.Fatalf("Failed to parse result JSON: %v", err)
			}

			// Compare each field
			for key, expectedValue := range tt.expected {
				resultValue, exists := resultData[key]
				if !exists {
					t.Errorf("Expected field '%s' in result, not found", key)
					continue
				}
				if resultValue != expectedValue {
					t.Errorf("Field '%s': expected '%v', got '%v'", key, expectedValue, resultValue)
				}
			}

			// Ensure no extra fields
			for key := range resultData {
				if _, exists := tt.expected[key]; !exists {
					t.Errorf("Unexpected field '%s' in result", key)
				}
			}
		})
	}
}

func TestNormalizeCaseInsensitive(t *testing.T) {
	Clear()

	Register("mytool-a", "scan_type", map[string]string{
		"syn": "SYN_SCAN",
		"ack": "ACK_SCAN",
	})

	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "Lowercase",
			input:    `{"scan_type": "syn"}`,
			expected: "SYN_SCAN",
		},
		{
			name:     "Uppercase",
			input:    `{"scan_type": "SYN"}`,
			expected: "SYN_SCAN",
		},
		{
			name:     "Mixed case",
			input:    `{"scan_type": "SyN"}`,
			expected: "SYN_SCAN",
		},
		{
			name:     "Title case",
			input:    `{"scan_type": "Ack"}`,
			expected: "ACK_SCAN",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := Normalize("mytool-a", tt.input)

			var resultData map[string]interface{}
			if err := json.Unmarshal([]byte(result), &resultData); err != nil {
				t.Fatalf("Failed to parse result JSON: %v", err)
			}

			scanType, exists := resultData["scan_type"]
			if !exists {
				t.Fatal("Expected 'scan_type' field in result")
			}

			if scanType != tt.expected {
				t.Errorf("Expected scan_type '%s', got '%s'", tt.expected, scanType)
			}
		})
	}
}

func TestNormalizePassThrough(t *testing.T) {
	Clear()

	Register("mytool-a", "scan_type", map[string]string{
		"syn": "SYN_SCAN",
	})

	tests := []struct {
		name     string
		input    string
		expected map[string]interface{}
	}{
		{
			name:  "Unmapped enum value passes through",
			input: `{"scan_type": "custom_scan"}`,
			expected: map[string]interface{}{
				"scan_type": "custom_scan",
			},
		},
		{
			name:  "Non-string value passes through",
			input: `{"scan_type": 123}`,
			expected: map[string]interface{}{
				"scan_type": float64(123),
			},
		},
		{
			name:  "Boolean value passes through",
			input: `{"scan_type": true}`,
			expected: map[string]interface{}{
				"scan_type": true,
			},
		},
		{
			name:  "Null value passes through",
			input: `{"scan_type": null}`,
			expected: map[string]interface{}{
				"scan_type": nil,
			},
		},
		{
			name:  "Array value passes through",
			input: `{"scan_type": ["syn", "ack"]}`,
			expected: map[string]interface{}{
				"scan_type": []interface{}{"syn", "ack"},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := Normalize("mytool-a", tt.input)

			var resultData map[string]interface{}
			if err := json.Unmarshal([]byte(result), &resultData); err != nil {
				t.Fatalf("Failed to parse result JSON: %v", err)
			}

			scanType := resultData["scan_type"]
			expectedValue := tt.expected["scan_type"]

			// For arrays, use JSON comparison
			if expectedArray, ok := expectedValue.([]interface{}); ok {
				resultArray, ok := scanType.([]interface{})
				if !ok {
					t.Fatalf("Expected array, got %T", scanType)
				}
				if len(resultArray) != len(expectedArray) {
					t.Errorf("Expected array length %d, got %d", len(expectedArray), len(resultArray))
				}
				for i, expected := range expectedArray {
					if i >= len(resultArray) || resultArray[i] != expected {
						t.Errorf("Array element %d: expected '%v', got '%v'", i, expected, resultArray[i])
					}
				}
			} else {
				// Direct comparison for primitives
				if scanType != expectedValue {
					t.Errorf("Expected scan_type '%v' (%T), got '%v' (%T)", expectedValue, expectedValue, scanType, scanType)
				}
			}
		})
	}
}

func TestNormalizeInvalidJSON(t *testing.T) {
	Clear()

	Register("mytool-a", "scan_type", map[string]string{
		"syn": "SYN_SCAN",
	})

	tests := []struct {
		name  string
		input string
	}{
		{
			name:  "Invalid JSON syntax",
			input: `{"scan_type": "syn"`,
		},
		{
			name:  "Not an object",
			input: `["syn", "ack"]`,
		},
		{
			name:  "Empty string",
			input: ``,
		},
		{
			name:  "Malformed JSON",
			input: `{scan_type: syn}`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := Normalize("mytool-a", tt.input)

			// Result should equal original input (graceful degradation)
			if result != tt.input {
				t.Errorf("Expected original input to be returned unchanged\nExpected: %s\nGot: %s", tt.input, result)
			}
		})
	}
}

func TestNormalizeNoMappings(t *testing.T) {
	Clear()

	tests := []struct {
		name     string
		toolName string
		input    string
	}{
		{
			name:     "Tool not registered",
			toolName: "unregistered-tool",
			input:    `{"scan_type": "syn", "target": "example.com"}`,
		},
		{
			name:     "Tool with no mappings",
			toolName: "empty-tool",
			input:    `{"field": "value"}`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := Normalize(tt.toolName, tt.input)

			// Result should equal original input (no transformation)
			if result != tt.input {
				t.Errorf("Expected input to be unchanged\nExpected: %s\nGot: %s", tt.input, result)
			}
		})
	}
}

func TestGetMappings(t *testing.T) {
	Clear()

	// Register multiple tools
	Register("mytool-a", "scan_type", map[string]string{
		"syn": "SYN_SCAN",
		"ack": "ACK_SCAN",
	})

	Register("masscan", "rate", map[string]string{
		"fast": "RATE_FAST",
		"slow": "RATE_SLOW",
	})

	tests := []struct {
		name          string
		toolName      string
		expectNil     bool
		expectedCount int
	}{
		{
			name:          "Get mytool-a mappings",
			toolName:      "mytool-a",
			expectNil:     false,
			expectedCount: 1,
		},
		{
			name:          "Get masscan mappings",
			toolName:      "masscan",
			expectNil:     false,
			expectedCount: 1,
		},
		{
			name:      "Get unregistered tool mappings",
			toolName:  "unknown",
			expectNil: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := GetMappings(tt.toolName)

			if tt.expectNil {
				if result != nil {
					t.Errorf("Expected nil, got %v", result)
				}
				return
			}

			if result == nil {
				t.Fatal("Expected mappings, got nil")
			}

			if len(result) != tt.expectedCount {
				t.Errorf("Expected %d field mappings, got %d", tt.expectedCount, len(result))
			}
		})
	}

	// Verify mytool-a specific mappings
	nmapMappings := GetMappings("mytool-a")
	if nmapMappings == nil {
		t.Fatal("Expected mytool-a mappings, got nil")
	}

	scanTypeMappings, exists := nmapMappings["scan_type"]
	if !exists {
		t.Fatal("Expected 'scan_type' field in mytool-a mappings")
	}

	if scanTypeMappings["syn"] != "SYN_SCAN" {
		t.Errorf("Expected 'syn' -> 'SYN_SCAN', got '%s'", scanTypeMappings["syn"])
	}

	if scanTypeMappings["ack"] != "ACK_SCAN" {
		t.Errorf("Expected 'ack' -> 'ACK_SCAN', got '%s'", scanTypeMappings["ack"])
	}
}

func TestGetMappingsReturnsDeepCopy(t *testing.T) {
	Clear()

	Register("mytool-a", "scan_type", map[string]string{
		"syn": "SYN_SCAN",
		"ack": "ACK_SCAN",
	})

	// Get mappings
	mappings := GetMappings("mytool-a")
	if mappings == nil {
		t.Fatal("Expected mappings, got nil")
	}

	// Modify the returned mappings
	mappings["scan_type"]["syn"] = "MODIFIED"
	mappings["scan_type"]["new_key"] = "NEW_VALUE"
	mappings["new_field"] = map[string]string{
		"test": "TEST_VALUE",
	}

	// Get mappings again and verify they weren't affected
	freshMappings := GetMappings("mytool-a")
	if freshMappings == nil {
		t.Fatal("Expected mappings, got nil")
	}

	scanTypeMappings := freshMappings["scan_type"]
	if scanTypeMappings["syn"] != "SYN_SCAN" {
		t.Errorf("Expected 'syn' -> 'SYN_SCAN', got '%s' (modifications leaked!)", scanTypeMappings["syn"])
	}

	if _, exists := scanTypeMappings["new_key"]; exists {
		t.Error("New key leaked into registry (not a deep copy!)")
	}

	if _, exists := freshMappings["new_field"]; exists {
		t.Error("New field leaked into registry (not a deep copy!)")
	}

	if len(scanTypeMappings) != 2 {
		t.Errorf("Expected 2 scan_type mappings, got %d (modifications leaked!)", len(scanTypeMappings))
	}
}

func TestClear(t *testing.T) {
	// Register some mappings
	Register("mytool-a", "scan_type", map[string]string{
		"syn": "SYN_SCAN",
	})

	Register("masscan", "rate", map[string]string{
		"fast": "RATE_FAST",
	})

	// Verify registrations exist
	if GetMappings("mytool-a") == nil {
		t.Fatal("Expected mytool-a mappings before clear")
	}
	if GetMappings("masscan") == nil {
		t.Fatal("Expected masscan mappings before clear")
	}

	// Clear registry
	Clear()

	// Verify all mappings are gone
	if GetMappings("mytool-a") != nil {
		t.Error("Expected nil after Clear(), mytool-a mappings still exist")
	}

	if GetMappings("masscan") != nil {
		t.Error("Expected nil after Clear(), masscan mappings still exist")
	}

	// Verify normalization returns original input (no mappings)
	input := `{"scan_type": "syn"}`
	result := Normalize("mytool-a", input)
	if result != input {
		t.Error("Expected normalization to return unchanged input after Clear()")
	}
}

func TestConcurrentAccess(t *testing.T) {
	Clear()

	const (
		numGoroutines = 50
		numOperations = 100
	)

	var wg sync.WaitGroup
	wg.Add(numGoroutines * 4) // 4 operation types per goroutine

	// Concurrent Register operations
	for i := range numGoroutines {
		go func(id int) {
			defer wg.Done()
			for range numOperations {
				Register("mytool-a", "scan_type", map[string]string{
					"syn": "SYN_SCAN",
					"ack": "ACK_SCAN",
				})
			}
		}(i)
	}

	// Concurrent RegisterBatch operations
	for i := range numGoroutines {
		go func(id int) {
			defer wg.Done()
			for range numOperations {
				RegisterBatch("masscan", map[string]map[string]string{
					"rate": {
						"fast": "RATE_FAST",
						"slow": "RATE_SLOW",
					},
				})
			}
		}(i)
	}

	// Concurrent Normalize operations
	for i := range numGoroutines {
		go func(id int) {
			defer wg.Done()
			for range numOperations {
				input := `{"scan_type": "syn", "target": "example.com"}`
				result := Normalize("mytool-a", input)
				// Just verify it doesn't panic, result may vary during concurrent writes
				if result == "" {
					t.Error("Normalize returned empty string")
				}
			}
		}(i)
	}

	// Concurrent GetMappings operations
	for i := range numGoroutines {
		go func(id int) {
			defer wg.Done()
			for range numOperations {
				mappings := GetMappings("mytool-a")
				// Just verify it doesn't panic
				_ = mappings
			}
		}(i)
	}

	// Wait for all goroutines to complete
	wg.Wait()

	// Verify registry is in a valid state (no corruption)
	nmapMappings := GetMappings("mytool-a")
	if nmapMappings == nil {
		t.Fatal("Expected mytool-a mappings after concurrent access")
	}

	masscanMappings := GetMappings("masscan")
	if masscanMappings == nil {
		t.Fatal("Expected masscan mappings after concurrent access")
	}

	// Verify normalization still works
	input := `{"scan_type": "syn"}`
	result := Normalize("mytool-a", input)
	if result == "" {
		t.Error("Normalize returned empty string after concurrent access")
	}

	var resultData map[string]interface{}
	if err := json.Unmarshal([]byte(result), &resultData); err != nil {
		t.Errorf("Failed to parse result JSON after concurrent access: %v", err)
	}
}

func TestNormalizeTypedMapFormat(t *testing.T) {
	Clear()

	Register("test-tool", "scan_type", map[string]string{
		"syn":     "SYN_SCAN",
		"connect": "CONNECT_SCAN",
	})

	Register("test-tool", "verbosity", map[string]string{
		"low":    "VERBOSITY_LOW",
		"medium": "VERBOSITY_MEDIUM",
		"high":   "VERBOSITY_HIGH",
	})

	tests := []struct {
		name     string
		input    string
		expected map[string]interface{}
	}{
		{
			name: "Single enum field in TypedMap",
			input: `{
				"entries": {
					"scan_type": {
						"stringValue": "syn"
					},
					"target": {
						"stringValue": "localhost"
					}
				}
			}`,
			expected: map[string]interface{}{
				"entries": map[string]interface{}{
					"scan_type": map[string]interface{}{
						"stringValue": "SYN_SCAN",
					},
					"target": map[string]interface{}{
						"stringValue": "localhost",
					},
				},
			},
		},
		{
			name: "Multiple enum fields in TypedMap",
			input: `{
				"entries": {
					"scan_type": {
						"stringValue": "connect"
					},
					"verbosity": {
						"stringValue": "high"
					},
					"target": {
						"stringValue": "example.com"
					}
				}
			}`,
			expected: map[string]interface{}{
				"entries": map[string]interface{}{
					"scan_type": map[string]interface{}{
						"stringValue": "CONNECT_SCAN",
					},
					"verbosity": map[string]interface{}{
						"stringValue": "VERBOSITY_HIGH",
					},
					"target": map[string]interface{}{
						"stringValue": "example.com",
					},
				},
			},
		},
		{
			name: "Unmapped value in TypedMap passes through",
			input: `{
				"entries": {
					"scan_type": {
						"stringValue": "unknown_scan"
					}
				}
			}`,
			expected: map[string]interface{}{
				"entries": map[string]interface{}{
					"scan_type": map[string]interface{}{
						"stringValue": "unknown_scan",
					},
				},
			},
		},
		{
			name: "Non-string TypedValue passes through",
			input: `{
				"entries": {
					"port": {
						"intValue": 80
					},
					"enabled": {
						"boolValue": true
					}
				}
			}`,
			expected: map[string]interface{}{
				"entries": map[string]interface{}{
					"port": map[string]interface{}{
						"intValue": float64(80),
					},
					"enabled": map[string]interface{}{
						"boolValue": true,
					},
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := Normalize("test-tool", tt.input)

			var resultData map[string]interface{}
			if err := json.Unmarshal([]byte(result), &resultData); err != nil {
				t.Fatalf("Failed to parse result JSON: %v", err)
			}

			// Verify entries field exists
			entries, ok := resultData["entries"].(map[string]interface{})
			if !ok {
				t.Fatalf("Expected 'entries' field to be a map, got %T", resultData["entries"])
			}

			expectedEntries := tt.expected["entries"].(map[string]interface{})

			// Compare each entry
			for key, expectedEntry := range expectedEntries {
				resultEntry, exists := entries[key].(map[string]interface{})
				if !exists {
					t.Errorf("Expected entry '%s' in result, not found", key)
					continue
				}

				expectedEntryMap := expectedEntry.(map[string]interface{})

				// Compare all fields in the entry
				for fieldName, expectedValue := range expectedEntryMap {
					resultValue, hasField := resultEntry[fieldName]
					if !hasField {
						t.Errorf("Entry '%s': expected field '%s', not found", key, fieldName)
						continue
					}
					if resultValue != expectedValue {
						t.Errorf("Entry '%s', field '%s': expected '%v', got '%v'", key, fieldName, expectedValue, resultValue)
					}
				}
			}
		})
	}
}

func TestNormalizeTypedMapCaseInsensitive(t *testing.T) {
	Clear()

	Register("test-tool", "verbosity", map[string]string{
		"high": "VERBOSITY_HIGH",
	})

	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name: "Lowercase",
			input: `{
				"entries": {
					"verbosity": {
						"stringValue": "high"
					}
				}
			}`,
			expected: "VERBOSITY_HIGH",
		},
		{
			name: "Uppercase",
			input: `{
				"entries": {
					"verbosity": {
						"stringValue": "HIGH"
					}
				}
			}`,
			expected: "VERBOSITY_HIGH",
		},
		{
			name: "Mixed case",
			input: `{
				"entries": {
					"verbosity": {
						"stringValue": "HiGh"
					}
				}
			}`,
			expected: "VERBOSITY_HIGH",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := Normalize("test-tool", tt.input)

			var resultData map[string]interface{}
			if err := json.Unmarshal([]byte(result), &resultData); err != nil {
				t.Fatalf("Failed to parse result JSON: %v", err)
			}

			entries := resultData["entries"].(map[string]interface{})
			verbosityEntry := entries["verbosity"].(map[string]interface{})
			verbosityValue := verbosityEntry["stringValue"].(string)

			if verbosityValue != tt.expected {
				t.Errorf("Expected verbosity '%s', got '%s'", tt.expected, verbosityValue)
			}
		})
	}
}
