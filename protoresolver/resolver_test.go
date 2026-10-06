// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Zero Root AI

package protoresolver

import (
	"errors"
	"testing"
)

// TestSchemaNotFoundError tests the SchemaNotFoundError error type.
func TestSchemaNotFoundError(t *testing.T) {
	tests := []struct {
		name         string
		err          *SchemaNotFoundError
		expectedMsg  string
		expectUnwrap bool
	}{
		{
			name: "basic error with tool name only",
			err: &SchemaNotFoundError{
				ToolName: "mytool-c",
			},
			expectedMsg:  `schema not found for tool "mytool-c"`,
			expectUnwrap: false,
		},
		{
			name: "error with tool name and type name",
			err: &SchemaNotFoundError{
				ToolName: "mytool-c",
				TypeName: "gibson.tools.mytool-c.HttpxRequest",
			},
			expectedMsg:  `schema not found for tool "mytool-c" while resolving type "gibson.tools.mytool-c.HttpxRequest"`,
			expectUnwrap: false,
		},
		{
			name: "error with all fields",
			err: &SchemaNotFoundError{
				ToolName: "mytool-c",
				TypeName: "gibson.tools.mytool-c.HttpxRequest",
				Cause:    ErrNoSchemaAvailable,
			},
			expectedMsg:  `schema not found for tool "mytool-c" while resolving type "gibson.tools.mytool-c.HttpxRequest" : no schema available for type resolution`,
			expectUnwrap: true,
		},
		{
			name: "error with tool name and cause only",
			err: &SchemaNotFoundError{
				ToolName: "mytool-c",
				Cause:    errors.New("metadata missing"),
			},
			expectedMsg:  `schema not found for tool "mytool-c" : metadata missing`,
			expectUnwrap: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Test Error() method
			if got := tt.err.Error(); got != tt.expectedMsg {
				t.Errorf("Error() = %q, want %q", got, tt.expectedMsg)
			}

			// Test Unwrap() method
			unwrapped := tt.err.Unwrap()
			if tt.expectUnwrap && unwrapped == nil {
				t.Error("Unwrap() = nil, want non-nil cause")
			}
			if !tt.expectUnwrap && unwrapped != nil {
				t.Errorf("Unwrap() = %v, want nil", unwrapped)
			}

			// Test errors.Is() with wrapped errors
			if tt.err.Cause != nil && !errors.Is(tt.err, tt.err.Cause) {
				t.Error("errors.Is() should work with wrapped cause")
			}
		})
	}
}

// TestSchemaNotFoundError_ErrorsIs tests error wrapping with errors.Is().
func TestSchemaNotFoundError_ErrorsIs(t *testing.T) {
	cause := errors.New("underlying error")
	err := &SchemaNotFoundError{
		ToolName: "test-tool",
		TypeName: "test.Type",
		Cause:    cause,
	}

	// Test that errors.Is works with the cause
	if !errors.Is(err, cause) {
		t.Error("errors.Is(err, cause) = false, want true")
	}

	// Test that errors.Is doesn't match unrelated errors
	otherErr := errors.New("other error")
	if errors.Is(err, otherErr) {
		t.Error("errors.Is(err, otherErr) = true, want false")
	}

	// Test with ErrNoSchemaAvailable as cause
	err2 := &SchemaNotFoundError{
		ToolName: "test-tool",
		Cause:    ErrNoSchemaAvailable,
	}
	if !errors.Is(err2, ErrNoSchemaAvailable) {
		t.Error("errors.Is(err2, ErrNoSchemaAvailable) = false, want true")
	}
}
