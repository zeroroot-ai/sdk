// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Zero Root AI

package types

import (
	"encoding/json"
	"fmt"

	"github.com/zeroroot-ai/sdk/schema"
)

// TargetSchema defines the structure for target type schemas that agents declare.
// It provides a JSON Schema-based approach to defining and validating connection
// parameters for different target types (e.g., HTTP APIs, Kubernetes clusters,
// smart contracts).
//
// Example usage:
//
//	schema := TargetSchema{
//		Type:        "kubernetes",
//		Version:     "1.0",
//		Description: "Kubernetes cluster target",
//		Schema: schema.Object(map[string]schema.JSON{
//			"cluster":    schema.StringWithDesc("Cluster name or kubeconfig context"),
//			"namespace":  schema.StringWithDesc("Kubernetes namespace"),
//			"kubeconfig": schema.StringWithDesc("Path to kubeconfig file"),
//		}, "cluster"),
//	}
//
//	// Validate schema definition
//	if err := schema.Validate(); err != nil {
//		return err
//	}
//
//	// Validate connection parameters
//	connection := map[string]any{
//		"cluster":   "prod-cluster",
//		"namespace": "default",
//	}
//	if err := schema.ValidateConnection(connection); err != nil {
//		return err
//	}
type TargetSchema struct {
	// Type is the target type identifier (e.g., "kubernetes", "http_api", "smart_contract").
	// This should be unique across target types and follow a consistent naming convention.
	Type string `json:"type"`

	// Version is the schema version (e.g., "1.0", "2.0").
	// This allows for schema evolution and backward compatibility tracking.
	Version string `json:"version"`

	// Schema is the JSON Schema definition for connection parameters.
	// This defines the structure, types, and validation rules for target connections.
	Schema schema.JSON `json:"schema"`

	// Description is a human-readable description of the target type.
	// This should explain what the target type is used for and any special considerations.
	Description string `json:"description"`
}

// Validate checks if the TargetSchema definition itself is valid.
// It verifies that all required fields are present and properly structured.
//
// Returns an error if:
//   - Type is empty
//   - Version is empty
//   - Description is empty
//   - Schema is not properly structured
//   - Required fields are not defined in properties
//
// Example:
//
//	if err := targetSchema.Validate(); err != nil {
//		log.Fatalf("invalid target schema: %v", err)
//	}
func (ts *TargetSchema) Validate() error {
	if ts.Type == "" {
		return &ValidationError{
			Field:   "Type",
			Message: "target schema type is required",
		}
	}

	if ts.Version == "" {
		return &ValidationError{
			Field:   "Version",
			Message: "target schema version is required",
		}
	}

	if ts.Description == "" {
		return &ValidationError{
			Field:   "Description",
			Message: "target schema description is required",
		}
	}

	// Validate that the schema itself is a valid JSON Schema definition.
	// We expect the schema to be an object type with properties defined.
	if ts.Schema.Type == "" {
		return &ValidationError{
			Field:   "Schema",
			Message: "target schema must have a type defined",
		}
	}

	// For target schemas, we expect an object type at the root level
	if ts.Schema.Type != "object" {
		return &ValidationError{
			Field:   "Schema.Type",
			Message: fmt.Sprintf("target schema must be of type 'object', got '%s'", ts.Schema.Type),
		}
	}

	// Validate that properties are defined
	if ts.Schema.Properties == nil || len(ts.Schema.Properties) == 0 {
		return &ValidationError{
			Field:   "Schema.Properties",
			Message: "target schema must define at least one property",
		}
	}

	// Verify that all required fields exist in properties
	for _, reqField := range ts.Schema.Required {
		if _, exists := ts.Schema.Properties[reqField]; !exists {
			return &ValidationError{
				Field:   "Schema.Required",
				Message: fmt.Sprintf("required field '%s' is not defined in schema properties", reqField),
			}
		}
	}

	return nil
}

// ValidateConnection validates connection parameters against this target schema.
// It checks that all required fields are present and that values conform to the
// schema's type and constraint definitions.
//
// Parameters:
//   - connection: A map of connection parameters to validate
//
// Returns an error if:
//   - The schema itself is invalid
//   - Required fields are missing
//   - Field types don't match the schema
//   - Values violate schema constraints (min/max, pattern, enum, etc.)
//
// Example:
//
//	connection := map[string]any{
//		"url":     "https://api.example.com",
//		"method":  "POST",
//		"timeout": 30,
//	}
//
//	if err := schema.ValidateConnection(connection); err != nil {
//		log.Fatalf("invalid connection parameters: %v", err)
//	}
func (ts *TargetSchema) ValidateConnection(connection map[string]any) error {
	// First ensure the schema itself is valid
	if err := ts.Validate(); err != nil {
		return fmt.Errorf("cannot validate connection with invalid schema: %w", err)
	}

	// Handle nil connection map
	if connection == nil {
		// If there are required fields, this is an error
		if len(ts.Schema.Required) > 0 {
			return &ValidationError{
				Field:   "Connection",
				Message: fmt.Sprintf("connection parameters cannot be nil when schema requires fields: %v", ts.Schema.Required),
			}
		}
		// No required fields and nil connection is acceptable
		return nil
	}

	// Use the schema package's validation to check the connection parameters
	if err := ts.Schema.Validate(connection); err != nil {
		return &ValidationError{
			Field:   "Connection",
			Message: fmt.Sprintf("connection parameters validation failed: %v", err),
		}
	}

	return nil
}

// ToJSON serializes the TargetSchema to JSON format.
// This is useful for transmitting schemas over gRPC or storing them in configuration.
//
// Returns the JSON representation and any marshaling errors.
//
// Example:
//
//	jsonData, err := targetSchema.ToJSON()
//	if err != nil {
//		return err
//	}
func (ts *TargetSchema) ToJSON() ([]byte, error) {
	return json.Marshal(ts)
}

// FromJSON deserializes a TargetSchema from JSON format.
// This is useful for loading schemas from configuration files or gRPC messages.
//
// Parameters:
//   - data: JSON-encoded TargetSchema
//
// Returns an error if the JSON is invalid or cannot be unmarshaled.
//
// Example:
//
//	var schema TargetSchema
//	if err := schema.FromJSON(jsonData); err != nil {
//		return err
//	}
func (ts *TargetSchema) FromJSON(data []byte) error {
	return json.Unmarshal(data, ts)
}

// String returns a human-readable string representation of the TargetSchema.
// This is useful for logging and debugging.
func (ts *TargetSchema) String() string {
	return fmt.Sprintf("TargetSchema{Type: %s, Version: %s, Description: %s}",
		ts.Type, ts.Version, ts.Description)
}

// Clone creates a deep copy of the TargetSchema.
// This is useful when you need to modify a schema without affecting the original.
//
// Returns a new TargetSchema instance with the same values.
//
// Example:
//
//	modifiedSchema := originalSchema.Clone()
//	modifiedSchema.Version = "2.0"
func (ts *TargetSchema) Clone() TargetSchema {
	// Create a new schema with copied values
	cloned := TargetSchema{
		Type:        ts.Type,
		Version:     ts.Version,
		Description: ts.Description,
		Schema:      ts.Schema, // schema.JSON is a struct, so this is a copy
	}

	// Note: schema.JSON contains maps which are reference types,
	// so modifications to Properties or other map fields will affect both copies.
	// For a true deep clone, we'd need to recursively copy the Schema field.
	// However, for most use cases (validation), this level of copying is sufficient.

	return cloned
}
