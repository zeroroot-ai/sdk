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

func getMap(m map[string]any, key string) map[string]any {
	if m == nil {
		return nil
	}
	val, ok := m[key]
	if !ok || val == nil {
		return nil
	}
	nested, ok := val.(map[string]any)
	if !ok {
		return nil
	}
	return nested
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

// Validate checks if the TargetInfo has all required fields.
func (t *TargetInfo) Validate() error {
	if t.ID == "" {
		return &ValidationError{Field: "ID", Message: "target ID is required"}
	}

	if t.Name == "" {
		return &ValidationError{Field: "Name", Message: "target name is required"}
	}

	if t.Type == "" {
		return &ValidationError{Field: "Type", Message: "target type is required"}
	}

	if len(t.Connection) == 0 {
		return &ValidationError{Field: "Connection", Message: "target must have Connection parameters"}
	}

	return nil
}

// URL returns the URL from the Connection map.
// This is a convenience method for accessing Connection["url"].
func (t *TargetInfo) URL() string {
	if t.Connection != nil {
		return getString(t.Connection, "url", "")
	}
	return ""
}

// GetConnection retrieves a connection parameter value by key.
// Returns the value and true if the key exists, nil and false otherwise.
func (t *TargetInfo) GetConnection(key string) (any, bool) {
	if t.Connection == nil {
		return nil, false
	}
	val, ok := t.Connection[key]
	return val, ok
}

// GetConnectionString retrieves a string connection parameter by key.
// Returns the string value or empty string if the key doesn't exist or is not a string.
// This is a convenience wrapper around GetConnection for common string parameters.
func (t *TargetInfo) GetConnectionString(key string) string {
	return getString(t.Connection, key, "")
}

// GetHeader retrieves a header value by key from Connection["headers"].
// Returns empty string if not found.
func (t *TargetInfo) GetHeader(key string) string {
	if t.Connection != nil {
		if headers := getMap(t.Connection, "headers"); headers != nil {
			if val, ok := headers[key].(string); ok {
				return val
			}
		}
	}
	return ""
}

// SetHeader sets a header value in Connection["headers"].
func (t *TargetInfo) SetHeader(key, value string) {
	// Ensure Connection map exists
	if t.Connection == nil {
		t.Connection = make(map[string]any)
	}
	// Ensure headers map exists in Connection
	headers, _ := t.Connection["headers"].(map[string]any)
	if headers == nil {
		headers = make(map[string]any)
		t.Connection["headers"] = headers
	}
	headers[key] = value
}

// GetMetadata retrieves a metadata value by key.
func (t *TargetInfo) GetMetadata(key string) (any, bool) {
	if t.Metadata == nil {
		return nil, false
	}
	val, ok := t.Metadata[key]
	return val, ok
}

// SetMetadata sets a metadata value.
func (t *TargetInfo) SetMetadata(key string, value any) {
	if t.Metadata == nil {
		t.Metadata = make(map[string]any)
	}
	t.Metadata[key] = value
}

// ValidationError represents a validation error for target info.
type ValidationError struct {
	Field   string
	Message string
}

func (e *ValidationError) Error() string {
	return e.Field + ": " + e.Message
}
