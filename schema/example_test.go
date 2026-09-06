// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Zero Root AI

package schema_test

import (
	"fmt"

	"github.com/zeroroot-ai/sdk/schema"
)

// Example demonstrates basic schema creation and validation.
func Example() {
	// Create a simple string schema
	nameSchema := schema.StringWithDesc("User's full name")

	// Validate a value
	if err := nameSchema.Validate("John Doe"); err != nil {
		fmt.Println("Invalid:", err)
	} else {
		fmt.Println("Valid name")
	}

	// Output: Valid name
}

// ExampleObject demonstrates object schema creation and validation.
func ExampleObject() {
	// Define a user schema
	userSchema := schema.Object(map[string]schema.JSON{
		"id":       schema.Int(),
		"username": schema.StringWithDesc("Unique username"),
		"email":    schema.String(),
		"age":      schema.Int(),
	}, "id", "username", "email") // id, username, and email are required

	// Valid user
	validUser := map[string]any{
		"id":       1,
		"username": "johndoe",
		"email":    "john@example.com",
		"age":      30,
	}

	if err := userSchema.Validate(validUser); err != nil {
		fmt.Println("Invalid:", err)
	} else {
		fmt.Println("Valid user")
	}

	// Output: Valid user
}

// ExampleArray demonstrates array schema creation and validation.
func ExampleArray() {
	// Create a schema for an array of numbers
	numbersSchema := schema.Array(schema.Number())

	// Valid array
	validNumbers := []float64{1.5, 2.7, 3.14}
	if err := numbersSchema.Validate(validNumbers); err != nil {
		fmt.Println("Invalid:", err)
	} else {
		fmt.Println("Valid numbers array")
	}

	// Output: Valid numbers array
}

// ExampleEnum demonstrates enum schema creation and validation.
func ExampleEnum() {
	// Create a status enum
	statusSchema := schema.Enum("pending", "active", "completed")

	// Valid status
	if err := statusSchema.Validate("active"); err != nil {
		fmt.Println("Invalid:", err)
	} else {
		fmt.Println("Valid status")
	}

	// Invalid status
	if err := statusSchema.Validate("cancelled"); err != nil {
		fmt.Println("Invalid status:", err)
	}

	// Output:
	// Valid status
	// Invalid status: value cancelled is not one of the allowed values: [pending active completed]
}

// ExampleJSON_Validate_constraints demonstrates validation with constraints.
func ExampleJSON_Validate_constraints() {
	// Create a schema with string constraints
	minLen := 3
	maxLen := 20
	usernameSchema := schema.JSON{
		Type:        "string",
		Description: "Username between 3 and 20 characters",
		MinLength:   &minLen,
		MaxLength:   &maxLen,
		Pattern:     "^[a-zA-Z0-9_]+$",
	}

	// Valid username
	if err := usernameSchema.Validate("john_doe"); err != nil {
		fmt.Println("Invalid:", err)
	} else {
		fmt.Println("Valid username")
	}

	// Too short
	if err := usernameSchema.Validate("ab"); err != nil {
		fmt.Println("Too short:", err)
	}

	// Output:
	// Valid username
	// Too short: string length 2 is less than minimum 3
}

// ExampleJSON_Validate_nested demonstrates validation of nested structures.
func ExampleJSON_Validate_nested() {
	// Create nested schemas
	addressSchema := schema.Object(map[string]schema.JSON{
		"street": schema.String(),
		"city":   schema.String(),
		"zip":    schema.String(),
	}, "street", "city")

	personSchema := schema.Object(map[string]schema.JSON{
		"name":    schema.String(),
		"age":     schema.Int(),
		"address": addressSchema,
	}, "name", "address")

	// Valid nested object
	person := map[string]any{
		"name": "Jane Doe",
		"age":  25,
		"address": map[string]any{
			"street": "123 Main St",
			"city":   "Springfield",
			"zip":    "12345",
		},
	}

	if err := personSchema.Validate(person); err != nil {
		fmt.Println("Invalid:", err)
	} else {
		fmt.Println("Valid person")
	}

	// Output: Valid person
}
