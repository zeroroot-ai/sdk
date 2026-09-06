// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Zero Root AI

package schema

import (
	"testing"
)

func TestString(t *testing.T) {
	schema := String()

	if schema.Type != "string" {
		t.Errorf("expected Type to be 'string', got %q", schema.Type)
	}

	// Test valid string
	if err := schema.Validate("hello"); err != nil {
		t.Errorf("expected valid string, got error: %v", err)
	}

	// Test invalid types
	if err := schema.Validate(123); err == nil {
		t.Error("expected error for integer, got nil")
	}
	if err := schema.Validate(true); err == nil {
		t.Error("expected error for boolean, got nil")
	}
}

func TestStringWithDesc(t *testing.T) {
	desc := "A test string"
	schema := StringWithDesc(desc)

	if schema.Type != "string" {
		t.Errorf("expected Type to be 'string', got %q", schema.Type)
	}
	if schema.Description != desc {
		t.Errorf("expected Description to be %q, got %q", desc, schema.Description)
	}

	// Test validation
	if err := schema.Validate("test"); err != nil {
		t.Errorf("expected valid string, got error: %v", err)
	}
}

func TestInt(t *testing.T) {
	schema := Int()

	if schema.Type != "integer" {
		t.Errorf("expected Type to be 'integer', got %q", schema.Type)
	}

	// Test valid integers
	validInts := []any{
		int(42),
		int8(42),
		int16(42),
		int32(42),
		int64(42),
		uint(42),
		uint8(42),
		uint16(42),
		uint32(42),
		uint64(42),
	}

	for _, val := range validInts {
		if err := schema.Validate(val); err != nil {
			t.Errorf("expected valid integer for %T(%v), got error: %v", val, val, err)
		}
	}

	// Test invalid types
	if err := schema.Validate("123"); err == nil {
		t.Error("expected error for string, got nil")
	}
	if err := schema.Validate(3.14); err == nil {
		t.Error("expected error for float with decimal, got nil")
	}

	// Test that whole number floats are accepted
	if err := schema.Validate(42.0); err != nil {
		t.Errorf("expected valid for whole number float, got error: %v", err)
	}
}

func TestNumber(t *testing.T) {
	schema := Number()

	if schema.Type != "number" {
		t.Errorf("expected Type to be 'number', got %q", schema.Type)
	}

	// Test valid numbers
	validNumbers := []any{
		int(42),
		int64(42),
		float32(3.14),
		float64(3.14),
		uint(42),
	}

	for _, val := range validNumbers {
		if err := schema.Validate(val); err != nil {
			t.Errorf("expected valid number for %T(%v), got error: %v", val, val, err)
		}
	}

	// Test invalid types
	if err := schema.Validate("123"); err == nil {
		t.Error("expected error for string, got nil")
	}
	if err := schema.Validate(true); err == nil {
		t.Error("expected error for boolean, got nil")
	}
}

func TestBool(t *testing.T) {
	schema := Bool()

	if schema.Type != "boolean" {
		t.Errorf("expected Type to be 'boolean', got %q", schema.Type)
	}

	// Test valid booleans
	if err := schema.Validate(true); err != nil {
		t.Errorf("expected valid boolean, got error: %v", err)
	}
	if err := schema.Validate(false); err != nil {
		t.Errorf("expected valid boolean, got error: %v", err)
	}

	// Test invalid types
	if err := schema.Validate(1); err == nil {
		t.Error("expected error for integer, got nil")
	}
	if err := schema.Validate("true"); err == nil {
		t.Error("expected error for string, got nil")
	}
}

func TestArray(t *testing.T) {
	itemSchema := String()
	schema := Array(itemSchema)

	if schema.Type != "array" {
		t.Errorf("expected Type to be 'array', got %q", schema.Type)
	}
	if schema.Items == nil {
		t.Error("expected Items to be set")
	}

	// Test valid arrays
	if err := schema.Validate([]string{"a", "b", "c"}); err != nil {
		t.Errorf("expected valid array, got error: %v", err)
	}
	if err := schema.Validate([]any{"x", "y", "z"}); err != nil {
		t.Errorf("expected valid array, got error: %v", err)
	}

	// Test invalid item types
	if err := schema.Validate([]int{1, 2, 3}); err == nil {
		t.Error("expected error for array with wrong item type, got nil")
	}

	// Test invalid type
	if err := schema.Validate("not an array"); err == nil {
		t.Error("expected error for non-array, got nil")
	}
}

func TestObject(t *testing.T) {
	properties := map[string]JSON{
		"name":  String(),
		"age":   Int(),
		"email": String(),
	}
	schema := Object(properties, "name", "email")

	if schema.Type != "object" {
		t.Errorf("expected Type to be 'object', got %q", schema.Type)
	}
	if len(schema.Properties) != 3 {
		t.Errorf("expected 3 properties, got %d", len(schema.Properties))
	}
	if len(schema.Required) != 2 {
		t.Errorf("expected 2 required fields, got %d", len(schema.Required))
	}

	// Test valid object
	validObj := map[string]any{
		"name":  "John Doe",
		"age":   30,
		"email": "john@example.com",
	}
	if err := schema.Validate(validObj); err != nil {
		t.Errorf("expected valid object, got error: %v", err)
	}

	// Test missing required field
	invalidObj := map[string]any{
		"name": "John Doe",
		"age":  30,
		// missing email
	}
	if err := schema.Validate(invalidObj); err == nil {
		t.Error("expected error for missing required field, got nil")
	}

	// Test invalid property type
	invalidTypeObj := map[string]any{
		"name":  "John Doe",
		"age":   "thirty", // should be integer
		"email": "john@example.com",
	}
	if err := schema.Validate(invalidTypeObj); err == nil {
		t.Error("expected error for invalid property type, got nil")
	}
}

func TestEnum(t *testing.T) {
	schema := Enum("red", "green", "blue")

	if len(schema.Enum) != 3 {
		t.Errorf("expected 3 enum values, got %d", len(schema.Enum))
	}

	// Test valid enum values
	validValues := []string{"red", "green", "blue"}
	for _, val := range validValues {
		if err := schema.Validate(val); err != nil {
			t.Errorf("expected valid enum value %q, got error: %v", val, err)
		}
	}

	// Test invalid enum value
	if err := schema.Validate("yellow"); err == nil {
		t.Error("expected error for invalid enum value, got nil")
	}
}

func TestValidateStringConstraints(t *testing.T) {
	tests := []struct {
		name      string
		schema    JSON
		value     any
		wantError bool
	}{
		{
			name: "valid string with min/max length",
			schema: JSON{
				Type:      "string",
				MinLength: intPtr(3),
				MaxLength: intPtr(10),
			},
			value:     "hello",
			wantError: false,
		},
		{
			name: "string too short",
			schema: JSON{
				Type:      "string",
				MinLength: intPtr(5),
			},
			value:     "hi",
			wantError: true,
		},
		{
			name: "string too long",
			schema: JSON{
				Type:      "string",
				MaxLength: intPtr(5),
			},
			value:     "hello world",
			wantError: true,
		},
		{
			name: "valid pattern match",
			schema: JSON{
				Type:    "string",
				Pattern: "^[a-z]+$",
			},
			value:     "hello",
			wantError: false,
		},
		{
			name: "invalid pattern match",
			schema: JSON{
				Type:    "string",
				Pattern: "^[a-z]+$",
			},
			value:     "Hello123",
			wantError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.schema.Validate(tt.value)
			if (err != nil) != tt.wantError {
				t.Errorf("Validate() error = %v, wantError %v", err, tt.wantError)
			}
		})
	}
}

func TestValidateNumericConstraints(t *testing.T) {
	tests := []struct {
		name      string
		schema    JSON
		value     any
		wantError bool
	}{
		{
			name: "valid integer with min/max",
			schema: JSON{
				Type:    "integer",
				Minimum: floatPtr(0),
				Maximum: floatPtr(100),
			},
			value:     50,
			wantError: false,
		},
		{
			name: "integer below minimum",
			schema: JSON{
				Type:    "integer",
				Minimum: floatPtr(0),
			},
			value:     -10,
			wantError: true,
		},
		{
			name: "integer above maximum",
			schema: JSON{
				Type:    "integer",
				Maximum: floatPtr(100),
			},
			value:     150,
			wantError: true,
		},
		{
			name: "valid number with min/max",
			schema: JSON{
				Type:    "number",
				Minimum: floatPtr(0.0),
				Maximum: floatPtr(1.0),
			},
			value:     0.5,
			wantError: false,
		},
		{
			name: "number below minimum",
			schema: JSON{
				Type:    "number",
				Minimum: floatPtr(0.0),
			},
			value:     -0.1,
			wantError: true,
		},
		{
			name: "number above maximum",
			schema: JSON{
				Type:    "number",
				Maximum: floatPtr(1.0),
			},
			value:     1.5,
			wantError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.schema.Validate(tt.value)
			if (err != nil) != tt.wantError {
				t.Errorf("Validate() error = %v, wantError %v", err, tt.wantError)
			}
		})
	}
}

func TestValidateNestedObjects(t *testing.T) {
	addressSchema := Object(map[string]JSON{
		"street": String(),
		"city":   String(),
		"zip":    String(),
	}, "street", "city")

	personSchema := Object(map[string]JSON{
		"name":    String(),
		"age":     Int(),
		"address": addressSchema,
	}, "name", "address")

	// Valid nested object
	validPerson := map[string]any{
		"name": "Jane Doe",
		"age":  25,
		"address": map[string]any{
			"street": "123 Main St",
			"city":   "Springfield",
			"zip":    "12345",
		},
	}
	if err := personSchema.Validate(validPerson); err != nil {
		t.Errorf("expected valid nested object, got error: %v", err)
	}

	// Invalid nested object (missing required field in nested object)
	invalidPerson := map[string]any{
		"name": "Jane Doe",
		"address": map[string]any{
			"street": "123 Main St",
			// missing city
		},
	}
	if err := personSchema.Validate(invalidPerson); err == nil {
		t.Error("expected error for invalid nested object, got nil")
	}
}

func TestValidateNestedArrays(t *testing.T) {
	numberArraySchema := Array(Number())
	matrixSchema := Array(numberArraySchema)

	// Valid nested array
	validMatrix := []any{
		[]any{1.0, 2.0, 3.0},
		[]any{4.0, 5.0, 6.0},
	}
	if err := matrixSchema.Validate(validMatrix); err != nil {
		t.Errorf("expected valid nested array, got error: %v", err)
	}

	// Invalid nested array (wrong item type in inner array)
	invalidMatrix := []any{
		[]any{1.0, 2.0, 3.0},
		[]any{4.0, "five", 6.0},
	}
	if err := matrixSchema.Validate(invalidMatrix); err == nil {
		t.Error("expected error for invalid nested array, got nil")
	}
}

func TestValidateNil(t *testing.T) {
	schema := String()

	// Nil should fail for typed schema
	if err := schema.Validate(nil); err == nil {
		t.Error("expected error for nil value with typed schema, got nil")
	}

	// Nil should pass for empty schema
	emptySchema := JSON{}
	if err := emptySchema.Validate(nil); err != nil {
		t.Errorf("expected nil to be valid for empty schema, got error: %v", err)
	}
}

func TestValidateComplexSchema(t *testing.T) {
	// Create a complex schema representing a user with posts
	postSchema := Object(map[string]JSON{
		"id":      Int(),
		"title":   StringWithDesc("Post title"),
		"content": String(),
		"tags":    Array(String()),
	}, "id", "title")

	userSchema := Object(map[string]JSON{
		"id":       Int(),
		"username": String(),
		"email":    String(),
		"age":      Int(),
		"posts":    Array(postSchema),
		"status":   Enum("active", "inactive", "suspended"),
	}, "id", "username", "email")

	// Valid complex object
	validUser := map[string]any{
		"id":       1,
		"username": "johndoe",
		"email":    "john@example.com",
		"age":      30,
		"status":   "active",
		"posts": []any{
			map[string]any{
				"id":      101,
				"title":   "First Post",
				"content": "Hello world",
				"tags":    []any{"intro", "welcome"},
			},
			map[string]any{
				"id":      102,
				"title":   "Second Post",
				"content": "More content",
				"tags":    []any{"update"},
			},
		},
	}

	if err := userSchema.Validate(validUser); err != nil {
		t.Errorf("expected valid complex object, got error: %v", err)
	}

	// Invalid status enum
	invalidUser := map[string]any{
		"id":       1,
		"username": "johndoe",
		"email":    "john@example.com",
		"status":   "banned", // invalid enum value
	}

	if err := userSchema.Validate(invalidUser); err == nil {
		t.Error("expected error for invalid enum value, got nil")
	}
}

func TestValidateArrayOfPrimitives(t *testing.T) {
	// Array of integers
	intArraySchema := Array(Int())
	if err := intArraySchema.Validate([]int{1, 2, 3, 4, 5}); err != nil {
		t.Errorf("expected valid int array, got error: %v", err)
	}

	// Array of strings
	stringArraySchema := Array(String())
	if err := stringArraySchema.Validate([]string{"a", "b", "c"}); err != nil {
		t.Errorf("expected valid string array, got error: %v", err)
	}

	// Array of booleans
	boolArraySchema := Array(Bool())
	if err := boolArraySchema.Validate([]bool{true, false, true}); err != nil {
		t.Errorf("expected valid bool array, got error: %v", err)
	}
}

func TestValidateMixedTypes(t *testing.T) {
	// Test that arrays with mixed types fail validation
	intArraySchema := Array(Int())
	mixedArray := []any{1, "two", 3}

	if err := intArraySchema.Validate(mixedArray); err == nil {
		t.Error("expected error for mixed type array, got nil")
	}
}

func TestEdgeCases(t *testing.T) {
	t.Run("empty object validates against object schema", func(t *testing.T) {
		schema := Object(map[string]JSON{
			"name": String(),
		})
		emptyObj := map[string]any{}
		if err := schema.Validate(emptyObj); err != nil {
			t.Errorf("expected empty object to validate, got error: %v", err)
		}
	})

	t.Run("empty array validates against array schema", func(t *testing.T) {
		schema := Array(String())
		emptyArray := []string{}
		if err := schema.Validate(emptyArray); err != nil {
			t.Errorf("expected empty array to validate, got error: %v", err)
		}
	})

	t.Run("object with extra properties validates", func(t *testing.T) {
		schema := Object(map[string]JSON{
			"name": String(),
		}, "name")
		objWithExtra := map[string]any{
			"name":  "John",
			"extra": "field",
		}
		if err := schema.Validate(objWithExtra); err != nil {
			t.Errorf("expected object with extra properties to validate, got error: %v", err)
		}
	})
}

// Helper functions for test cases
func intPtr(i int) *int {
	return &i
}

func floatPtr(f float64) *float64 {
	return &f
}
