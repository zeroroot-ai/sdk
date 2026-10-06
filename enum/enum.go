// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Zero Root AI

package enum

import (
	"sync"
)

// registry is the global enum mapping registry
var (
	registry = make(map[string]map[string]map[string]string)
	mu       sync.RWMutex
)

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
