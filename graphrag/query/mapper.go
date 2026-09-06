// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Zero Root AI

// Package query provides result mapping utilities for converting Neo4j query results
// to protocol buffer types using reflection.
package query

import (
	"errors"
	"fmt"
	"strings"

	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protoreflect"
)

// MapRowToProto maps a Neo4j result row (map[string]any) to a protocol buffer message.
// It uses protoreflect to set fields by name, handling type conversions for Neo4j quirks:
//   - Neo4j int64 -> proto int32 (with overflow check)
//   - Neo4j float64 -> proto float/double
//   - Handles nil values for optional fields (skips them)
//
// Field mapping is done by proto field name (not JSON name). Missing fields in the row
// are gracefully skipped. Unknown fields in the row are ignored.
//
// Example:
//
//	row := map[string]any{
//	    "ip": "192.168.1.1",
//	    "hostname": "server1.local",
//	    "state": "up",
//	}
//	host := &taxonomypb.Host{}
//	err := MapRowToProto(row, host)
func MapRowToProto(row map[string]any, target proto.Message) error {
	if target == nil {
		return errors.New("target message cannot be nil")
	}
	if row == nil {
		return errors.New("row cannot be nil")
	}

	msg := target.ProtoReflect()
	fields := msg.Descriptor().Fields()

	// Iterate through all fields in the proto message
	for i := 0; i < fields.Len(); i++ {
		field := fields.Get(i)
		fieldName := string(field.Name())

		// Check if this field exists in the row
		value, exists := row[fieldName]
		if !exists {
			// Try snake_case to camelCase conversion
			value, exists = row[snakeToCamel(fieldName)]
			if !exists {
				// Field not present in row - skip it (not an error for optional fields)
				continue
			}
		}

		// Skip nil values - leave proto field unset
		if value == nil {
			continue
		}

		// Set the field value with type conversion
		if err := setFieldValue(msg, field, value); err != nil {
			return fmt.Errorf("failed to set field %q: %w", fieldName, err)
		}
	}

	return nil
}

// MapRowsToProtos maps multiple Neo4j result rows to a slice of protocol buffer messages.
// The factory function is called for each row to create a new proto instance.
//
// Example:
//
//	rows := []map[string]any{
//	    {"ip": "192.168.1.1", "hostname": "server1"},
//	    {"ip": "192.168.1.2", "hostname": "server2"},
//	}
//	hosts, err := MapRowsToProtos(rows, func() *taxonomypb.Host {
//	    return &taxonomypb.Host{}
//	})
func MapRowsToProtos[T proto.Message](rows []map[string]any, factory func() T) ([]T, error) {
	if rows == nil {
		return nil, nil
	}

	results := make([]T, 0, len(rows))
	for i, row := range rows {
		target := factory()
		if err := MapRowToProto(row, target); err != nil {
			return nil, fmt.Errorf("failed to map row %d: %w", i, err)
		}
		results = append(results, target)
	}

	return results, nil
}

// setFieldValue sets a single field value on a proto message with type conversion.
// Handles Neo4j type quirks:
//   - int64 -> int32 conversion with overflow check
//   - float64 -> float conversion
//   - string, bool, bytes as-is
func setFieldValue(msg protoreflect.Message, field protoreflect.FieldDescriptor, value any) error {
	// Handle different field kinds
	switch field.Kind() {
	case protoreflect.BoolKind:
		b, ok := value.(bool)
		if !ok {
			return fmt.Errorf("expected bool, got %T", value)
		}
		msg.Set(field, protoreflect.ValueOfBool(b))

	case protoreflect.Int32Kind, protoreflect.Sint32Kind, protoreflect.Sfixed32Kind:
		// Neo4j returns int64 for all integers
		i64, err := toInt64(value)
		if err != nil {
			return err
		}
		// Check for overflow when converting to int32
		if i64 < -2147483648 || i64 > 2147483647 {
			return fmt.Errorf("int64 value %d overflows int32", i64)
		}
		msg.Set(field, protoreflect.ValueOfInt32(int32(i64)))

	case protoreflect.Int64Kind, protoreflect.Sint64Kind, protoreflect.Sfixed64Kind:
		i64, err := toInt64(value)
		if err != nil {
			return err
		}
		msg.Set(field, protoreflect.ValueOfInt64(i64))

	case protoreflect.Uint32Kind, protoreflect.Fixed32Kind:
		i64, err := toInt64(value)
		if err != nil {
			return err
		}
		if i64 < 0 || i64 > 4294967295 {
			return fmt.Errorf("int64 value %d overflows uint32", i64)
		}
		msg.Set(field, protoreflect.ValueOfUint32(uint32(i64)))

	case protoreflect.Uint64Kind, protoreflect.Fixed64Kind:
		i64, err := toInt64(value)
		if err != nil {
			return err
		}
		if i64 < 0 {
			return fmt.Errorf("negative value %d cannot be converted to uint64", i64)
		}
		msg.Set(field, protoreflect.ValueOfUint64(uint64(i64)))

	case protoreflect.FloatKind:
		f64, err := toFloat64(value)
		if err != nil {
			return err
		}
		msg.Set(field, protoreflect.ValueOfFloat32(float32(f64)))

	case protoreflect.DoubleKind:
		f64, err := toFloat64(value)
		if err != nil {
			return err
		}
		msg.Set(field, protoreflect.ValueOfFloat64(f64))

	case protoreflect.StringKind:
		s, ok := value.(string)
		if !ok {
			return fmt.Errorf("expected string, got %T", value)
		}
		msg.Set(field, protoreflect.ValueOfString(s))

	case protoreflect.BytesKind:
		// Handle both []byte and string
		switch v := value.(type) {
		case []byte:
			msg.Set(field, protoreflect.ValueOfBytes(v))
		case string:
			msg.Set(field, protoreflect.ValueOfBytes([]byte(v)))
		default:
			return fmt.Errorf("expected []byte or string, got %T", value)
		}

	case protoreflect.MessageKind:
		// For embedded messages, we'd need recursive handling
		// This is a complex case - for now, return an error
		return fmt.Errorf("message fields not yet supported (field %q)", field.Name())

	case protoreflect.EnumKind:
		// Try to handle enum by number or string
		enumValue, err := toEnumValue(field, value)
		if err != nil {
			return err
		}
		msg.Set(field, protoreflect.ValueOfEnum(enumValue))

	default:
		return fmt.Errorf("unsupported field kind: %s", field.Kind())
	}

	return nil
}

// toInt64 converts various numeric types to int64.
// Handles Neo4j's tendency to return int64 for all integers.
func toInt64(value any) (int64, error) {
	switch v := value.(type) {
	case int:
		return int64(v), nil
	case int8:
		return int64(v), nil
	case int16:
		return int64(v), nil
	case int32:
		return int64(v), nil
	case int64:
		return v, nil
	case uint:
		return int64(v), nil
	case uint8:
		return int64(v), nil
	case uint16:
		return int64(v), nil
	case uint32:
		return int64(v), nil
	case uint64:
		if v > 9223372036854775807 {
			return 0, fmt.Errorf("uint64 value %d overflows int64", v)
		}
		return int64(v), nil
	case float32:
		return int64(v), nil
	case float64:
		return int64(v), nil
	default:
		return 0, fmt.Errorf("cannot convert %T to int64", value)
	}
}

// toFloat64 converts various numeric types to float64.
// Handles Neo4j's tendency to return float64 for all floats.
func toFloat64(value any) (float64, error) {
	switch v := value.(type) {
	case float32:
		return float64(v), nil
	case float64:
		return v, nil
	case int:
		return float64(v), nil
	case int8:
		return float64(v), nil
	case int16:
		return float64(v), nil
	case int32:
		return float64(v), nil
	case int64:
		return float64(v), nil
	case uint:
		return float64(v), nil
	case uint8:
		return float64(v), nil
	case uint16:
		return float64(v), nil
	case uint32:
		return float64(v), nil
	case uint64:
		return float64(v), nil
	default:
		return 0, fmt.Errorf("cannot convert %T to float64", value)
	}
}

// toEnumValue converts a value to an enum number.
// Accepts either int64 (enum number) or string (enum name).
func toEnumValue(field protoreflect.FieldDescriptor, value any) (protoreflect.EnumNumber, error) {
	enumDesc := field.Enum()
	if enumDesc == nil {
		return 0, errors.New("field is not an enum")
	}

	switch v := value.(type) {
	case string:
		// Look up enum value by name
		enumValue := enumDesc.Values().ByName(protoreflect.Name(v))
		if enumValue == nil {
			// Try uppercase variant
			enumValue = enumDesc.Values().ByName(protoreflect.Name(strings.ToUpper(v)))
		}
		if enumValue == nil {
			return 0, fmt.Errorf("unknown enum value %q for enum %s", v, enumDesc.FullName())
		}
		return enumValue.Number(), nil

	case int, int8, int16, int32, int64, uint, uint8, uint16, uint32, uint64:
		// Convert to int64 and use as enum number
		i64, err := toInt64(v)
		if err != nil {
			return 0, err
		}
		// Verify it's a valid enum value
		enumValue := enumDesc.Values().ByNumber(protoreflect.EnumNumber(i64))
		if enumValue == nil {
			return 0, fmt.Errorf("invalid enum number %d for enum %s", i64, enumDesc.FullName())
		}
		return protoreflect.EnumNumber(i64), nil

	default:
		return 0, fmt.Errorf("cannot convert %T to enum", value)
	}
}

// snakeToCamel converts snake_case to camelCase.
// Used for field name matching when Neo4j returns snake_case.
func snakeToCamel(s string) string {
	parts := strings.Split(s, "_")
	if len(parts) == 1 {
		return s
	}

	result := parts[0]
	for i := 1; i < len(parts); i++ {
		if len(parts[i]) > 0 {
			result += strings.ToUpper(parts[i][:1])
			if len(parts[i]) > 1 {
				result += parts[i][1:]
			}
		}
	}
	return result
}

// MapFieldsFromProto extracts specific fields from a proto message into a map.
// Useful for building Neo4j query parameters from proto messages.
// If fields is nil or empty, all fields are extracted.
//
// Example:
//
//	host := &taxonomypb.Host{Ip: "192.168.1.1", Hostname: "server1"}
//	params := MapFieldsFromProto(host, []string{"ip", "hostname"})
//	// params = map[string]any{"ip": "192.168.1.1", "hostname": "server1"}
func MapFieldsFromProto(msg proto.Message, fields []string) map[string]any {
	if msg == nil {
		return nil
	}

	result := make(map[string]any)
	m := msg.ProtoReflect()
	desc := m.Descriptor()

	// If no fields specified, extract all set fields
	if len(fields) == 0 {
		for i := 0; i < desc.Fields().Len(); i++ {
			field := desc.Fields().Get(i)
			if m.Has(field) {
				result[string(field.Name())] = protoValueToGo(m.Get(field), field)
			}
		}
		return result
	}

	// Extract only specified fields
	for _, fieldName := range fields {
		field := desc.Fields().ByName(protoreflect.Name(fieldName))
		if field == nil {
			// Field not found - skip
			continue
		}
		if m.Has(field) {
			result[fieldName] = protoValueToGo(m.Get(field), field)
		}
	}

	return result
}

// protoValueToGo converts a protoreflect.Value to a Go native type.
func protoValueToGo(val protoreflect.Value, field protoreflect.FieldDescriptor) any {
	switch field.Kind() {
	case protoreflect.BoolKind:
		return val.Bool()
	case protoreflect.Int32Kind, protoreflect.Sint32Kind, protoreflect.Sfixed32Kind:
		return int32(val.Int())
	case protoreflect.Int64Kind, protoreflect.Sint64Kind, protoreflect.Sfixed64Kind:
		return val.Int()
	case protoreflect.Uint32Kind, protoreflect.Fixed32Kind:
		return uint32(val.Uint())
	case protoreflect.Uint64Kind, protoreflect.Fixed64Kind:
		return val.Uint()
	case protoreflect.FloatKind:
		return float32(val.Float())
	case protoreflect.DoubleKind:
		return val.Float()
	case protoreflect.StringKind:
		return val.String()
	case protoreflect.BytesKind:
		return val.Bytes()
	case protoreflect.EnumKind:
		return int32(val.Enum())
	case protoreflect.MessageKind:
		// Return the message itself for complex types
		return val.Message().Interface()
	default:
		// Fallback - return interface value
		return val.Interface()
	}
}

// ExtractIDFields extracts fields that are used as ID components from a proto message.
// Returns a map of field names to values for all non-empty ID-related fields.
// This is useful for building composite IDs from proto messages.
//
// ID fields are identified as:
//   - "id" field
//   - Fields ending in "_id"
//   - Fields that are required and string type
//
// Example:
//
//	port := &taxonomypb.Port{
//	    ParentHostId: "192.168.1.1",
//	    Number: 80,
//	    Protocol: "tcp",
//	}
//	ids := ExtractIDFields(port)
//	// ids = map[string]any{"parent_host_id": "192.168.1.1"}
func ExtractIDFields(msg proto.Message) map[string]any {
	if msg == nil {
		return nil
	}

	result := make(map[string]any)
	m := msg.ProtoReflect()
	desc := m.Descriptor()

	for i := 0; i < desc.Fields().Len(); i++ {
		field := desc.Fields().Get(i)
		fieldName := string(field.Name())

		// Check if field is ID-related
		isIDField := fieldName == "id" ||
			strings.HasSuffix(fieldName, "_id") ||
			strings.HasPrefix(fieldName, "parent_")

		if !isIDField {
			continue
		}

		// Only include if field is set
		if !m.Has(field) {
			continue
		}

		result[fieldName] = protoValueToGo(m.Get(field), field)
	}

	return result
}

// ValidateRequiredFields checks that all required (non-optional) fields in a proto message are set.
// Returns an error listing all missing required fields.
//
// In proto3, all fields are technically optional, but we consider a field "required" if:
//   - It's not marked as optional in the proto definition
//   - It's a primitive type (not a message)
//
// Example:
//
//	host := &taxonomypb.Host{} // Empty host
//	err := ValidateRequiredFields(host)
//	// err might contain: "missing required fields: id"
func ValidateRequiredFields(msg proto.Message) error {
	if msg == nil {
		return errors.New("message cannot be nil")
	}

	m := msg.ProtoReflect()
	desc := m.Descriptor()

	var missing []string
	for i := 0; i < desc.Fields().Len(); i++ {
		field := desc.Fields().Get(i)

		// Skip optional fields (explicitly marked optional in proto3)
		if field.HasOptionalKeyword() {
			continue
		}

		// Skip message types (they're always optional in proto3)
		if field.Kind() == protoreflect.MessageKind {
			continue
		}

		// Check if required field is set
		if !m.Has(field) {
			missing = append(missing, string(field.Name()))
		}
	}

	if len(missing) > 0 {
		return fmt.Errorf("missing required fields: %s", strings.Join(missing, ", "))
	}

	return nil
}
