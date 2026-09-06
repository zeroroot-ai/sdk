// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Zero Root AI

package schema

import (
	"encoding/json"
	"fmt"
	"reflect"
	"regexp"
	"strings"
)

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

// String creates a JSON schema for a string type.
func String() JSON {
	return JSON{Type: "string"}
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

// Number creates a JSON schema for a number type.
func Number() JSON {
	return JSON{Type: "number"}
}

// Bool creates a JSON schema for a boolean type.
func Bool() JSON {
	return JSON{Type: "boolean"}
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

// Enum creates a JSON schema with enumerated values.
func Enum(values ...any) JSON {
	return JSON{Enum: values}
}

// Validate validates the given value against this JSON schema.
// It returns an error if the value does not conform to the schema.
func (s JSON) Validate(value any) error {
	return s.validateWithRegistry(value, nil, make(map[string]bool))
}

// validateWithRegistry validates the given value against this JSON schema with $ref support.
// It tracks visited refs to detect circular references.
func (s JSON) validateWithRegistry(value any, registry map[string]JSON, visited map[string]bool) error {
	// Handle nil values
	if value == nil {
		if s.Type != "" {
			return fmt.Errorf("expected type %s, got nil", s.Type)
		}
		return nil
	}

	// Handle $ref
	if s.Ref != "" {
		// Parse the ref - we only support local refs (#/definitions/X)
		if !strings.HasPrefix(s.Ref, "#/definitions/") {
			return fmt.Errorf("unsupported $ref format: %s (only #/definitions/X is supported)", s.Ref)
		}

		// Extract definition name
		defName := strings.TrimPrefix(s.Ref, "#/definitions/")

		// Check for circular reference
		if visited[s.Ref] {
			return fmt.Errorf("circular $ref detected: %s", s.Ref)
		}

		// Look up definition in registry
		if registry == nil {
			return fmt.Errorf("$ref %s cannot be resolved: no schema registry provided", s.Ref)
		}

		refSchema, exists := registry[defName]
		if !exists {
			return fmt.Errorf("$ref %s cannot be resolved: definition not found", s.Ref)
		}

		// Mark as visited and validate against referenced schema
		visited[s.Ref] = true
		defer delete(visited, s.Ref)

		return refSchema.validateWithRegistry(value, registry, visited)
	}

	// Validate enum
	if len(s.Enum) > 0 {
		return s.validateEnum(value)
	}

	// Validate type
	if s.Type != "" {
		if err := s.validateType(value); err != nil {
			return err
		}
	}

	// Type-specific validation
	switch s.Type {
	case "string":
		return s.validateString(value)
	case "integer":
		return s.validateInteger(value)
	case "number":
		return s.validateNumber(value)
	case "boolean":
		return s.validateBoolean(value)
	case "array":
		return s.validateArrayWithRegistry(value, registry, visited)
	case "object":
		return s.validateObjectWithRegistry(value, registry, visited)
	}

	return nil
}

// validateType checks if the value matches the expected type.
func (s JSON) validateType(value any) error {
	v := reflect.ValueOf(value)

	// Dereference pointers to get the underlying value
	for v.Kind() == reflect.Ptr {
		if v.IsNil() {
			return fmt.Errorf("expected %s, got nil pointer", s.Type)
		}
		v = v.Elem()
	}

	switch s.Type {
	case "string":
		if v.Kind() != reflect.String {
			return fmt.Errorf("expected string, got %T", value)
		}
	case "integer":
		switch v.Kind() {
		case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64,
			reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
			// Valid integer types
		case reflect.Float32, reflect.Float64:
			// Check if float is actually an integer
			f := v.Float()
			if f != float64(int64(f)) {
				return fmt.Errorf("expected integer, got float with decimal: %v", value)
			}
		default:
			return fmt.Errorf("expected integer, got %T", value)
		}
	case "number":
		switch v.Kind() {
		case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64,
			reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64,
			reflect.Float32, reflect.Float64:
			// Valid number types
		default:
			return fmt.Errorf("expected number, got %T", value)
		}
	case "boolean":
		if v.Kind() != reflect.Bool {
			return fmt.Errorf("expected boolean, got %T", value)
		}
	case "array":
		if v.Kind() != reflect.Slice && v.Kind() != reflect.Array {
			return fmt.Errorf("expected array, got %T", value)
		}
	case "object":
		if v.Kind() != reflect.Map && v.Kind() != reflect.Struct {
			return fmt.Errorf("expected object, got %T", value)
		}
	}

	return nil
}

// validateString validates string-specific constraints.
func (s JSON) validateString(value any) error {
	str, ok := value.(string)
	if !ok {
		return fmt.Errorf("expected string, got %T", value)
	}

	// Validate length constraints
	if s.MinLength != nil && len(str) < *s.MinLength {
		return fmt.Errorf("string length %d is less than minimum %d", len(str), *s.MinLength)
	}
	if s.MaxLength != nil && len(str) > *s.MaxLength {
		return fmt.Errorf("string length %d is greater than maximum %d", len(str), *s.MaxLength)
	}

	// Validate pattern
	if s.Pattern != "" {
		matched, err := regexp.MatchString(s.Pattern, str)
		if err != nil {
			return fmt.Errorf("invalid pattern: %w", err)
		}
		if !matched {
			return fmt.Errorf("string does not match pattern %s", s.Pattern)
		}
	}

	return nil
}

// validateInteger validates integer-specific constraints.
func (s JSON) validateInteger(value any) error {
	var num float64

	v := reflect.ValueOf(value)
	switch v.Kind() {
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		num = float64(v.Int())
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		num = float64(v.Uint())
	case reflect.Float32, reflect.Float64:
		num = v.Float()
		if num != float64(int64(num)) {
			return fmt.Errorf("expected integer, got float with decimal: %v", value)
		}
	default:
		return fmt.Errorf("expected integer, got %T", value)
	}

	return s.validateNumericConstraints(num)
}

// validateNumber validates number-specific constraints.
func (s JSON) validateNumber(value any) error {
	var num float64

	v := reflect.ValueOf(value)
	switch v.Kind() {
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		num = float64(v.Int())
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		num = float64(v.Uint())
	case reflect.Float32, reflect.Float64:
		num = v.Float()
	default:
		return fmt.Errorf("expected number, got %T", value)
	}

	return s.validateNumericConstraints(num)
}

// validateNumericConstraints validates minimum and maximum constraints.
func (s JSON) validateNumericConstraints(num float64) error {
	if s.Minimum != nil && num < *s.Minimum {
		return fmt.Errorf("value %v is less than minimum %v", num, *s.Minimum)
	}
	if s.Maximum != nil && num > *s.Maximum {
		return fmt.Errorf("value %v is greater than maximum %v", num, *s.Maximum)
	}
	return nil
}

// validateBoolean validates boolean type.
func (s JSON) validateBoolean(value any) error {
	_, ok := value.(bool)
	if !ok {
		return fmt.Errorf("expected boolean, got %T", value)
	}
	return nil
}

// validateArrayWithRegistry validates array-specific constraints with $ref support.
func (s JSON) validateArrayWithRegistry(value any, registry map[string]JSON, visited map[string]bool) error {
	v := reflect.ValueOf(value)
	if v.Kind() != reflect.Slice && v.Kind() != reflect.Array {
		return fmt.Errorf("expected array, got %T", value)
	}

	// Validate items if schema is provided
	if s.Items != nil {
		for i := 0; i < v.Len(); i++ {
			item := v.Index(i).Interface()
			if err := s.Items.validateWithRegistry(item, registry, visited); err != nil {
				return fmt.Errorf("item %d: %w", i, err)
			}
		}
	}

	return nil
}

// validateObjectWithRegistry validates object-specific constraints with $ref support.
func (s JSON) validateObjectWithRegistry(value any, registry map[string]JSON, visited map[string]bool) error {
	// Convert value to map for validation
	var objMap map[string]any

	switch v := value.(type) {
	case map[string]any:
		objMap = v
	default:
		// Try to marshal and unmarshal to get a map
		data, err := json.Marshal(value)
		if err != nil {
			return fmt.Errorf("failed to marshal object: %w", err)
		}
		if err := json.Unmarshal(data, &objMap); err != nil {
			return fmt.Errorf("failed to unmarshal object: %w", err)
		}
	}

	// Validate required fields
	for _, req := range s.Required {
		if _, exists := objMap[req]; !exists {
			return fmt.Errorf("required field %s is missing", req)
		}
	}

	// Validate properties
	for key, val := range objMap {
		if propSchema, exists := s.Properties[key]; exists {
			if err := propSchema.validateWithRegistry(val, registry, visited); err != nil {
				return fmt.Errorf("property %s: %w", key, err)
			}
		}
	}

	return nil
}

// validateEnum validates that the value is one of the allowed enum values.
func (s JSON) validateEnum(value any) error {
	for _, enumVal := range s.Enum {
		if reflect.DeepEqual(value, enumVal) {
			return nil
		}
	}
	return fmt.Errorf("value %v is not one of the allowed values: %v", value, s.Enum)
}
