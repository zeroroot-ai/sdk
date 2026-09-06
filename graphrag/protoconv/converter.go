// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Zero Root AI

// Package protoconv provides utilities for converting proto messages to map representations
// for GraphRAG operations. This package enables direct use of proto types in the taxonomy
// system without requiring domain wrapper types.
package protoconv

import (
	"errors"
	"fmt"

	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protoreflect"
)

// identifyingFieldsByType maps node types to their identifying property field names.
// These are the fields that uniquely identify a node of that type.
var identifyingFieldsByType = map[string][]string{
	"host":        {"ip"},
	"port":        {"number", "protocol"},
	"service":     {"name"},
	"endpoint":    {"url", "method"},
	"domain":      {"name"},
	"subdomain":   {"name"},
	"technology":  {"name", "version"},
	"certificate": {"fingerprint_sha256"},
	"finding":     {"title"},
	"mission":     {"name", "target"},
}

// ToProperties converts a proto message to a map[string]any representation.
// It uses protoreflect to iterate over all fields and extract their values.
// Only fields that are set (non-zero) are included in the result.
//
// Supported field types:
//   - string, int32, int64, float32, float64, bool, bytes
//   - enum (converted to string)
//   - optional fields (only included if set)
//
// Parent references (parent_id, parent_type, etc.) and metadata fields
// (id, mission_id, etc.) are excluded as they are handled separately by the framework.
func ToProperties(msg proto.Message) (map[string]any, error) {
	if msg == nil {
		return nil, errors.New("proto message is nil")
	}

	props := make(map[string]any)
	refl := msg.ProtoReflect()
	fields := refl.Descriptor().Fields()

	for i := 0; i < fields.Len(); i++ {
		field := fields.Get(i)
		fieldName := string(field.Name())

		// Skip metadata and framework-managed fields
		if isFrameworkField(fieldName) {
			continue
		}

		// Skip fields that aren't set
		if !refl.Has(field) {
			continue
		}

		value := refl.Get(field)

		// Convert field value to Go native type
		converted, err := convertFieldValue(field, value)
		if err != nil {
			return nil, fmt.Errorf("failed to convert field %s: %w", fieldName, err)
		}

		// Only include non-zero values
		if converted != nil && !isZeroValue(converted) {
			props[fieldName] = converted
		}
	}

	return props, nil
}

// IdentifyingProperties extracts the subset of properties that uniquely identify
// a node of the given type. Returns an error if the node type is not recognized
// or if required identifying properties are missing.
//
// For example:
//   - host nodes are identified by "ip"
//   - port nodes are identified by "number" and "protocol"
//   - service nodes are identified by "name"
func IdentifyingProperties(nodeType string, msg proto.Message) (map[string]any, error) {
	if msg == nil {
		return nil, errors.New("proto message is nil")
	}

	identFields, ok := identifyingFieldsByType[nodeType]
	if !ok {
		return nil, fmt.Errorf("unknown node type: %s", nodeType)
	}

	allProps, err := ToProperties(msg)
	if err != nil {
		return nil, fmt.Errorf("failed to extract properties: %w", err)
	}

	identProps := make(map[string]any)
	for _, fieldName := range identFields {
		value, ok := allProps[fieldName]
		if !ok {
			return nil, fmt.Errorf("missing identifying property %s for node type %s", fieldName, nodeType)
		}
		identProps[fieldName] = value
	}

	return identProps, nil
}

// convertFieldValue converts a protoreflect.Value to a Go native type.
func convertFieldValue(field protoreflect.FieldDescriptor, value protoreflect.Value) (any, error) {
	switch field.Kind() {
	case protoreflect.StringKind:
		return value.String(), nil

	case protoreflect.Int32Kind, protoreflect.Sint32Kind, protoreflect.Sfixed32Kind:
		return int32(value.Int()), nil

	case protoreflect.Int64Kind, protoreflect.Sint64Kind, protoreflect.Sfixed64Kind:
		return value.Int(), nil

	case protoreflect.Uint32Kind, protoreflect.Fixed32Kind:
		return uint32(value.Uint()), nil

	case protoreflect.Uint64Kind, protoreflect.Fixed64Kind:
		return value.Uint(), nil

	case protoreflect.FloatKind:
		return float32(value.Float()), nil

	case protoreflect.DoubleKind:
		return value.Float(), nil

	case protoreflect.BoolKind:
		return value.Bool(), nil

	case protoreflect.BytesKind:
		return value.Bytes(), nil

	case protoreflect.EnumKind:
		// Convert enum to string representation
		enumVal := value.Enum()
		enumDesc := field.Enum().Values().ByNumber(enumVal)
		if enumDesc == nil {
			return nil, fmt.Errorf("unknown enum value %d for field %s", enumVal, field.Name())
		}
		return string(enumDesc.Name()), nil

	case protoreflect.MessageKind:
		// Check if this is a map field (maps are represented as messages in protobuf)
		if field.IsMap() {
			return convertMapValue(field, value)
		}
		// Other message types are not supported in properties
		return nil, fmt.Errorf("message fields are not supported in properties: %s", field.Name())

	default:
		return nil, fmt.Errorf("unsupported field kind: %v", field.Kind())
	}
}

// convertMapValue converts a protobuf map field to a Go map.
// Only map<string, string> is currently supported for Neo4j compatibility.
func convertMapValue(field protoreflect.FieldDescriptor, value protoreflect.Value) (any, error) {
	keyKind := field.MapKey().Kind()
	valKind := field.MapValue().Kind()

	// Currently only support map<string, string> for Neo4j compatibility
	if keyKind != protoreflect.StringKind || valKind != protoreflect.StringKind {
		return nil, fmt.Errorf("only map<string, string> is supported, got map<%v, %v>", keyKind, valKind)
	}

	result := make(map[string]string)
	mapVal := value.Map()
	mapVal.Range(func(k protoreflect.MapKey, v protoreflect.Value) bool {
		result[k.String()] = v.String()
		return true
	})

	// Return as map[string]any for compatibility with Neo4j properties
	if len(result) == 0 {
		return nil, nil
	}
	return result, nil
}

// isFrameworkField returns true if the field is managed by the framework
// and should not be included in user-facing properties.
func isFrameworkField(fieldName string) bool {
	switch fieldName {
	case "id",
		"parent_id",
		"parent_type",
		"parent_relationship",
		"mission_id",
		"mission_run_id",
		"agent_run_id",
		"discovered_by",
		"discovered_at",
		"created_at",
		"updated_at":
		return true
	default:
		// Also exclude parent reference fields (e.g., parent_host_id, parent_port_id)
		// These follow the pattern "parent_*_id"
		if len(fieldName) > 10 && fieldName[:7] == "parent_" && fieldName[len(fieldName)-3:] == "_id" {
			return true
		}
		return false
	}
}

// isZeroValue checks if a value is a zero value for its type.
func isZeroValue(v any) bool {
	switch val := v.(type) {
	case string:
		return val == ""
	case int32:
		return val == 0
	case int64:
		return val == 0
	case uint32:
		return val == 0
	case uint64:
		return val == 0
	case float32:
		return val == 0.0
	case float64:
		return val == 0.0
	case bool:
		return !val
	case []byte:
		return len(val) == 0
	default:
		return false
	}
}
