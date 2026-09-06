// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Zero Root AI

package classifier

// CategoryMatch represents a category search result with similarity score.
type CategoryMatch struct {
	// Category is the matched category name
	Category string `json:"category"`

	// Domain is the category's domain (e.g., "security", "compliance")
	Domain string `json:"domain"`

	// Description provides context about the category
	Description string `json:"description"`

	// Score is the similarity score (0.0 to 1.0, higher is more similar)
	Score float64 `json:"score"`
}

// Config holds configuration for the category classifier.
type Config struct {
	// Threshold is the minimum similarity score for matching an existing category.
	// If the best match score is >= threshold, the existing category is returned.
	// If the best match score is < threshold, a new category is registered.
	// Default: 0.85
	Threshold float64 `json:"threshold" yaml:"threshold"`

	// AutoRegister controls whether new categories are automatically registered
	// when no similar category is found. If false, Classify returns the proposed
	// category without registering it.
	// Default: true
	AutoRegister bool `json:"auto_register" yaml:"auto_register"`

	// StoreType specifies the vector store backend to use.
	// Valid values: "memory", "qdrant"
	// Default: "memory"
	StoreType string `json:"store_type" yaml:"store_type"`
}

// DefaultConfig returns a Config with sensible defaults.
func DefaultConfig() Config {
	return Config{
		Threshold:    0.85,
		AutoRegister: true,
		StoreType:    "memory",
	}
}
