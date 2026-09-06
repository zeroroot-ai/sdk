// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Zero Root AI

package schema

import (
	"reflect"
	"strings"
	"time"
)

// FromType generates a JSON schema from a Go type using reflection.
// The schema describes the structure that the type represents, not the values.
//
// This is used when agents call CompleteStructured with a Go struct type -
// the schema is sent to the daemon which forwards it to the LLM provider.
//
// Supported types:
//   - struct: generates an object schema with properties from exported fields
//   - slice/array: generates an array schema
//   - map: generates an object schema with additionalProperties
//   - string, int*, uint*, float*, bool: generates primitive schemas
//   - time.Time: generates string schema with date-time format
//   - interface{}/any: generates empty schema (allows any)
//
// Struct tags:
//   - `json:"name"`: uses the JSON tag name for the property
//   - `json:"-"`: skips the field
//   - `json:"name,omitempty"`: field is optional (not in required list)
func FromType(t any) JSON {
	if t == nil {
		return JSON{}
	}

	rt := reflect.TypeOf(t)
	return fromReflectType(rt)
}

// fromReflectType generates a JSON schema from a reflect.Type
func fromReflectType(t reflect.Type) JSON {
	// Handle pointer types
	if t.Kind() == reflect.Ptr {
		return fromReflectType(t.Elem())
	}

	// Special handling for time.Time
	if t == reflect.TypeOf(time.Time{}) {
		return JSON{
			Type:   "string",
			Format: "date-time",
		}
	}

	switch t.Kind() {
	case reflect.Struct:
		return fromStruct(t)
	case reflect.Slice, reflect.Array:
		itemSchema := fromReflectType(t.Elem())
		return JSON{
			Type:  "array",
			Items: &itemSchema,
		}
	case reflect.Map:
		// Maps are objects - we can't enforce key types in JSON schema
		return JSON{
			Type: "object",
		}
	case reflect.String:
		return JSON{Type: "string"}
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64,
		reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		return JSON{Type: "integer"}
	case reflect.Float32, reflect.Float64:
		return JSON{Type: "number"}
	case reflect.Bool:
		return JSON{Type: "boolean"}
	case reflect.Interface:
		// interface{} or any - allows any type
		return JSON{}
	default:
		// Unknown types - allow any
		return JSON{}
	}
}

// fromStruct generates a JSON schema from a struct type
func fromStruct(t reflect.Type) JSON {
	properties := make(map[string]JSON)
	var required []string

	for i := 0; i < t.NumField(); i++ {
		field := t.Field(i)

		// Skip unexported fields
		if !field.IsExported() {
			continue
		}

		// Parse json tag
		jsonTag := field.Tag.Get("json")
		if jsonTag == "-" {
			continue // Skip fields with json:"-"
		}

		// Get field name from json tag or field name
		fieldName := field.Name
		isOmitempty := false
		if jsonTag != "" {
			parts := strings.Split(jsonTag, ",")
			if parts[0] != "" {
				fieldName = parts[0]
			}
			for _, part := range parts[1:] {
				if part == "omitempty" {
					isOmitempty = true
					break
				}
			}
		}

		// Generate schema for the field type
		fieldSchema := fromReflectType(field.Type)

		// Add description from doc tag if present
		if desc := field.Tag.Get("description"); desc != "" {
			fieldSchema.Description = desc
		}

		properties[fieldName] = fieldSchema

		// Non-omitempty fields are required
		if !isOmitempty {
			required = append(required, fieldName)
		}
	}

	return JSON{
		Type:       "object",
		Properties: properties,
		Required:   required,
	}
}
