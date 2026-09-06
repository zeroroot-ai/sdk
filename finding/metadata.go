// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Zero Root AI

package finding

import "encoding/json"

// Well-known metadata keys for various domains.
// These keys provide standardized naming for common metadata fields
// across security, compliance, infrastructure, and other domains.
const (
	// Security domain metadata keys
	MetaKeyMitreAttack = "mitre_attack"
	MetaKeyMitreAtlas  = "mitre_atlas"
	MetaKeyCVSS        = "cvss"
	MetaKeyCWE         = "cwe"
	MetaKeyRiskScore   = "risk_score"

	// Compliance domain metadata keys
	MetaKeyComplianceFramework = "compliance_framework"
	MetaKeyComplianceControl   = "compliance_control"

	// Infrastructure domain metadata keys
	MetaKeyCostImpact  = "cost_impact"
	MetaKeyResourceARN = "resource_arn"
)

// GetTypedMetadata retrieves metadata from a finding with type safety using Go generics.
// It returns the value cast to type T and a boolean indicating whether the key was found
// and the value could be successfully converted to type T.
//
// Example usage:
//
//	cvss, ok := GetTypedMetadata[CVSSScore](finding, MetaKeyCVSS)
//	if ok {
//	    fmt.Printf("CVSS Score: %f\n", cvss.Score)
//	}
//
// For complex types stored as map[string]any, this function will attempt to re-marshal
// and unmarshal the value to convert it to the desired type.
func GetTypedMetadata[T any](f *Finding, key string) (T, bool) {
	var zero T

	if f == nil || f.Metadata == nil {
		return zero, false
	}

	val, exists := f.Metadata[key]
	if !exists {
		return zero, false
	}

	// Try direct type assertion first (for simple types)
	if typed, ok := val.(T); ok {
		return typed, true
	}

	// For complex types, try JSON round-trip conversion
	// This handles cases where the value is stored as map[string]any
	// but needs to be converted to a specific struct type
	data, err := json.Marshal(val)
	if err != nil {
		return zero, false
	}

	var result T
	if err := json.Unmarshal(data, &result); err != nil {
		return zero, false
	}

	return result, true
}

// SetMetadata sets a metadata value for a given key.
// This is a convenience wrapper around direct map access.
func (f *Finding) SetMetadata(key string, value any) {
	if f.Metadata == nil {
		f.Metadata = make(map[string]any)
	}
	f.Metadata[key] = value
}

// GetMetadata retrieves a metadata value for a given key.
// Returns the value and a boolean indicating whether the key was found.
func (f *Finding) GetMetadata(key string) (any, bool) {
	if f.Metadata == nil {
		return nil, false
	}
	val, ok := f.Metadata[key]
	return val, ok
}

// HasMetadata checks if a metadata key exists.
func (f *Finding) HasMetadata(key string) bool {
	if f.Metadata == nil {
		return false
	}
	_, ok := f.Metadata[key]
	return ok
}

// DeleteMetadata removes a metadata key.
func (f *Finding) DeleteMetadata(key string) {
	if f.Metadata != nil {
		delete(f.Metadata, key)
	}
}

// GetMetadataKeys returns all metadata keys present in the finding.
func (f *Finding) GetMetadataKeys() []string {
	if f.Metadata == nil {
		return nil
	}

	keys := make([]string, 0, len(f.Metadata))
	for k := range f.Metadata {
		keys = append(keys, k)
	}
	return keys
}

// GetStringMetadata is a convenience function to retrieve string metadata.
// Returns the value and a boolean indicating success.
func (f *Finding) GetStringMetadata(key string) (string, bool) {
	return GetTypedMetadata[string](f, key)
}

// GetFloat64Metadata is a convenience function to retrieve float64 metadata.
// Returns the value and a boolean indicating success.
func (f *Finding) GetFloat64Metadata(key string) (float64, bool) {
	return GetTypedMetadata[float64](f, key)
}

// GetIntMetadata is a convenience function to retrieve int metadata.
// Returns the value and a boolean indicating success.
func (f *Finding) GetIntMetadata(key string) (int, bool) {
	return GetTypedMetadata[int](f, key)
}

// GetBoolMetadata is a convenience function to retrieve bool metadata.
// Returns the value and a boolean indicating success.
func (f *Finding) GetBoolMetadata(key string) (bool, bool) {
	return GetTypedMetadata[bool](f, key)
}

// GetStringSliceMetadata is a convenience function to retrieve []string metadata.
// Returns the value and a boolean indicating success.
func (f *Finding) GetStringSliceMetadata(key string) ([]string, bool) {
	return GetTypedMetadata[[]string](f, key)
}
