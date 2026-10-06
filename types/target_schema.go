// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Zero Root AI

package types

import (
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

// String returns a human-readable string representation of the TargetSchema.
// This is useful for logging and debugging.
func (ts *TargetSchema) String() string {
	return fmt.Sprintf("TargetSchema{Type: %s, Version: %s, Description: %s}",
		ts.Type, ts.Version, ts.Description)
}
