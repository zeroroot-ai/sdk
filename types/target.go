// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Zero Root AI

package types

func getString(m map[string]any, key, defaultVal string) string {
	if m == nil {
		return defaultVal
	}
	val, ok := m[key]
	if !ok || val == nil {
		return defaultVal
	}
	str, ok := val.(string)
	if !ok {
		return defaultVal
	}
	return str
}

// TargetInfo contains detailed information about a target system.
// It provides all necessary context for agents to interact with and test the target.
type TargetInfo struct {
	// ID is a unique identifier for the target.
	ID string `json:"id"`

	// Name is a human-readable name for the target.
	Name string `json:"name"`

	// Type categorizes the target system (e.g., "http_api", "kubernetes", "smart_contract").
	// Changed from TargetType enum to string for extensibility.
	Type string `json:"type"`

	// Provider identifies the vendor or service (e.g., "openai", "anthropic", "custom").
	Provider string `json:"provider,omitempty"`

	// Connection contains type-specific connection parameters.
	// For example, http_api targets use {"url": "...", "headers": {...}},
	// kubernetes targets use {"cluster": "...", "namespace": "..."},
	// smart_contract targets use {"chain": "...", "address": "..."}.
	Connection map[string]any `json:"connection,omitempty"`

	// Metadata stores additional target-specific information and context.
	// This can include model versions, capabilities, rate limits, etc.
	Metadata map[string]any `json:"metadata,omitempty"`
}

// URL returns the URL from the Connection map.
// This is a convenience method for accessing Connection["url"].
func (t *TargetInfo) URL() string {
	if t.Connection != nil {
		return getString(t.Connection, "url", "")
	}
	return ""
}
