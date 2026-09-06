// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Zero Root AI

package enum

import (
	"encoding/json"
	"strings"
	"sync"
)

// registry is the global enum mapping registry
var (
	registry = make(map[string]map[string]map[string]string)
	mu       sync.RWMutex
)

// Register registers enum mappings for a specific tool field.
// toolName: the name of the tool (e.g., "mytool-a")
// fieldName: the field name in the JSON (e.g., "scan_type")
// mappings: map of shorthand values to proto enum names (e.g., {"syn": "SYN_SCAN"})
func Register(toolName, fieldName string, mappings map[string]string) {
	mu.Lock()
	defer mu.Unlock()

	if registry[toolName] == nil {
		registry[toolName] = make(map[string]map[string]string)
	}

	if registry[toolName][fieldName] == nil {
		registry[toolName][fieldName] = make(map[string]string)
	}

	// Store mappings with lowercase keys for case-insensitive lookup
	for shortValue, protoName := range mappings {
		registry[toolName][fieldName][strings.ToLower(shortValue)] = protoName
	}
}

// RegisterBatch registers multiple field mappings for a tool at once.
// toolName: the name of the tool
// fieldMappings: map of field names to their enum mappings
func RegisterBatch(toolName string, fieldMappings map[string]map[string]string) {
	for fieldName, mappings := range fieldMappings {
		Register(toolName, fieldName, mappings)
	}
}

// Normalize applies enum mappings to JSON input for a specific tool.
// Returns the normalized JSON string with enum values replaced with proto names.
// If any error occurs, returns the original input unchanged.
//
// This function handles both flat JSON and TypedMap format:
// - Flat: {"verbosity": "high"} -> {"verbosity": "VERBOSITY_HIGH"}
// - TypedMap: {"entries": {"verbosity": {"stringValue": "high"}}} -> {"entries": {"verbosity": {"stringValue": "VERBOSITY_HIGH"}}}
func Normalize(toolName, inputJSON string) string {
	mu.RLock()
	toolMappings, exists := registry[toolName]
	mu.RUnlock()

	// No mappings for this tool, return unchanged
	if !exists || len(toolMappings) == 0 {
		return inputJSON
	}

	// Parse JSON into map
	var data map[string]interface{}
	if err := json.Unmarshal([]byte(inputJSON), &data); err != nil {
		// Return original if parsing fails
		return inputJSON
	}

	// Check if this is TypedMap format (has "entries" field)
	if entries, ok := data["entries"].(map[string]interface{}); ok {
		// TypedMap format: normalize values inside entries
		mu.RLock()
		for fieldName, fieldMappings := range toolMappings {
			if entry, hasEntry := entries[fieldName].(map[string]interface{}); hasEntry {
				// Look for stringValue field in the TypedValue
				if strValue, isString := entry["stringValue"].(string); isString {
					// Case-insensitive lookup
					if protoName, found := fieldMappings[strings.ToLower(strValue)]; found {
						entry["stringValue"] = protoName
					}
				}
			}
		}
		mu.RUnlock()
	} else {
		// Flat format: normalize top-level fields directly
		mu.RLock()
		for fieldName, fieldMappings := range toolMappings {
			if value, ok := data[fieldName]; ok {
				// Convert value to string for lookup
				var strValue string
				switch v := value.(type) {
				case string:
					strValue = v
				default:
					// Skip non-string values
					continue
				}

				// Case-insensitive lookup
				if protoName, found := fieldMappings[strings.ToLower(strValue)]; found {
					data[fieldName] = protoName
				}
			}
		}
		mu.RUnlock()
	}

	// Re-serialize to JSON
	normalized, err := json.Marshal(data)
	if err != nil {
		// Return original if serialization fails
		return inputJSON
	}

	return string(normalized)
}

// GetMappings returns all enum mappings for a specific tool.
// Returns nil if the tool has no registered mappings.
func GetMappings(toolName string) map[string]map[string]string {
	mu.RLock()
	defer mu.RUnlock()

	toolMappings, exists := registry[toolName]
	if !exists {
		return nil
	}

	// Return a deep copy to prevent external modifications
	result := make(map[string]map[string]string)
	for fieldName, fieldMappings := range toolMappings {
		result[fieldName] = make(map[string]string)
		for shortValue, protoName := range fieldMappings {
			result[fieldName][shortValue] = protoName
		}
	}

	return result
}

// Clear resets the entire enum registry.
// This is primarily useful for testing.
func Clear() {
	mu.Lock()
	defer mu.Unlock()

	registry = make(map[string]map[string]map[string]string)
}
