// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Zero Root AI

// Package registry provides thread-safe category registration and validation
// for the Finding system.
//
// The CategoryRegistry enables domain-specific category management while
// maintaining backward compatibility with string-based categories. This allows
// Gibson to support multiple domains (security, compliance, infrastructure, etc.)
// with consistent category naming and metadata.
//
// # Basic Usage
//
//	// Create a new registry
//	registry := registry.NewCategoryRegistry()
//
//	// Register a custom category
//	err := registry.RegisterCategory(registry.CategoryInfo{
//		Name:        "cost_spike",
//		Domain:      "infrastructure",
//		DisplayName: "Cost Spike",
//		Description: "Unexpected increase in cloud spending",
//		Color:       "#FFA500",
//		Icon:        "dollar-sign",
//	})
//
//	// Validate a category (soft validation - warns but doesn't fail)
//	err = registry.Validate("cost_spike")
//
//	// Get category metadata
//	info, exists := registry.GetInfo("cost_spike")
//	if exists {
//		fmt.Println(info.DisplayName)
//	}
//
//	// List categories by domain
//	infraCategories := registry.ListByDomain("infrastructure")
//
// # Default Registry
//
// The DefaultRegistry() function returns a registry pre-populated with security
// categories, providing backward compatibility with existing Gibson agents:
//
//	registry := registry.DefaultRegistry()
//	// Contains: jailbreak, prompt_injection, data_extraction, etc.
//
// # Thread Safety
//
// All registry operations are thread-safe using sync.RWMutex. Concurrent
// registration, validation, and lookup operations are supported.
//
// # Validation
//
// The registry performs soft validation - if a category is not registered,
// Validate() logs a warning but returns nil (no error). This allows for
// extensibility while providing helpful feedback about unregistered categories.
package registry
