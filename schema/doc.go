// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Zero Root AI

// Package schema provides JSON Schema types and validation utilities for the Gibson SDK.
//
// This package implements JSON Schema Draft 7 compatible types and validation logic,
// allowing developers to define and validate data structures against JSON Schema specifications.
//
// # Basic Usage
//
// Creating simple schemas:
//
//	// String schema
//	stringSchema := schema.String()
//
//	// Integer schema
//	intSchema := schema.Int()
//
//	// Boolean schema
//	boolSchema := schema.Bool()
//
// # Complex Schemas
//
// Creating object schemas with properties and required fields:
//
//	userSchema := schema.Object(map[string]schema.JSON{
//		"name":  schema.StringWithDesc("User's full name"),
//		"age":   schema.Int(),
//		"email": schema.String(),
//	}, "name", "email") // name and email are required
//
// Creating array schemas:
//
//	numbersSchema := schema.Array(schema.Number())
//	usersSchema := schema.Array(userSchema)
//
// # Validation
//
// Validating values against schemas:
//
//	err := stringSchema.Validate("hello") // nil (valid)
//	err = stringSchema.Validate(123)      // error: expected string, got int
//
//	user := map[string]any{
//		"name":  "John Doe",
//		"email": "john@example.com",
//		"age":   30,
//	}
//	err = userSchema.Validate(user) // nil (valid)
//
// # Constraints
//
// Adding constraints to schemas:
//
//	minLen := 3
//	maxLen := 50
//	constrainedString := schema.JSON{
//		Type:      "string",
//		MinLength: &minLen,
//		MaxLength: &maxLen,
//		Pattern:   "^[a-zA-Z]+$",
//	}
//
//	min := 0.0
//	max := 100.0
//	constrainedNumber := schema.JSON{
//		Type:    "number",
//		Minimum: &min,
//		Maximum: &max,
//	}
//
// # Enumerations
//
// Creating enum schemas:
//
//	statusSchema := schema.Enum("pending", "active", "completed")
//	err := statusSchema.Validate("active")  // nil (valid)
//	err = statusSchema.Validate("invalid")  // error: not in allowed values
//
// # Type Safety
//
// The JSON struct uses Go's type system to represent JSON Schema definitions,
// providing compile-time type safety for schema construction while maintaining
// flexibility for complex schema patterns.
package schema
