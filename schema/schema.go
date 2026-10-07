// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Zero Root AI

package schema

// JSON represents a JSON Schema definition.
// It provides a structured way to define and validate JSON data structures.
type JSON struct {
	Type        string          `json:"type,omitempty"`
	Description string          `json:"description,omitempty"`
	Properties  map[string]JSON `json:"properties,omitempty"`
	Required    []string        `json:"required,omitempty"`
	Items       *JSON           `json:"items,omitempty"`
	Enum        []any           `json:"enum,omitempty"`
	Default     any             `json:"default,omitempty"`
	Minimum     *float64        `json:"minimum,omitempty"`
	Maximum     *float64        `json:"maximum,omitempty"`
	MinLength   *int            `json:"minLength,omitempty"`
	MaxLength   *int            `json:"maxLength,omitempty"`
	Pattern     string          `json:"pattern,omitempty"`
	Format      string          `json:"format,omitempty"`
	Ref         string          `json:"$ref,omitempty"`
}

// Any creates a JSON schema that accepts any type.
// This is useful for dynamic or unstructured data.
func Any() JSON {
	return JSON{}
}

// StringWithDesc creates a JSON schema for a string type with a description.
func StringWithDesc(desc string) JSON {
	return JSON{
		Type:        "string",
		Description: desc,
	}
}

// Int creates a JSON schema for an integer type.
func Int() JSON {
	return JSON{Type: "integer"}
}

// Array creates a JSON schema for an array type with the specified item schema.
func Array(items JSON) JSON {
	return JSON{
		Type:  "array",
		Items: &items,
	}
}

// Object creates a JSON schema for an object type with the specified properties and required fields.
func Object(properties map[string]JSON, required ...string) JSON {
	return JSON{
		Type:       "object",
		Properties: properties,
		Required:   required,
	}
}
