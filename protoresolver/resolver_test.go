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

// TestTypeNotFoundError tests the TypeNotFoundError error type.
func TestTypeNotFoundError(t *testing.T) {
	tests := []struct {
		name        string
		err         *TypeNotFoundError
		expectedMsg string
	}{
		{
			name: "error with no available types",
			err: &TypeNotFoundError{
				TypeName:       "gibson.tools.mytool-c.HttpxRequest",
				AvailableTypes: nil,
			},
			expectedMsg: `type "gibson.tools.mytool-c.HttpxRequest" not found`,
		},
		{
			name: "error with empty available types",
			err: &TypeNotFoundError{
				TypeName:       "gibson.tools.mytool-c.HttpxRequest",
				AvailableTypes: []string{},
			},
			expectedMsg: `type "gibson.tools.mytool-c.HttpxRequest" not found`,
		},
		{
			name: "error with few available types",
			err: &TypeNotFoundError{
				TypeName: "gibson.tools.mytool-c.HttpxRequest",
				AvailableTypes: []string{
					"gibson.tools.mytool-c.HttpxResponse",
					"gibson.tools.mytool-c.HttpxConfig",
				},
			},
			expectedMsg: `type "gibson.tools.mytool-c.HttpxRequest" not found (available types: gibson.tools.mytool-c.HttpxResponse, gibson.tools.mytool-c.HttpxConfig)`,
		},
		{
			name: "error with exactly 10 available types",
			err: &TypeNotFoundError{
				TypeName: "gibson.tools.mytool-c.HttpxRequest",
				AvailableTypes: []string{
					"Type1", "Type2", "Type3", "Type4", "Type5",
					"Type6", "Type7", "Type8", "Type9", "Type10",
				},
			},
			expectedMsg: `type "gibson.tools.mytool-c.HttpxRequest" not found (available types: Type1, Type2, Type3, Type4, Type5, Type6, Type7, Type8, Type9, Type10)`,
		},
		{
			name: "error with more than 10 available types",
			err: &TypeNotFoundError{
				TypeName: "gibson.tools.mytool-c.HttpxRequest",
				AvailableTypes: []string{
					"Type1", "Type2", "Type3", "Type4", "Type5",
					"Type6", "Type7", "Type8", "Type9", "Type10",
					"Type11", "Type12", "Type13",
				},
			},
			expectedMsg: `type "gibson.tools.mytool-c.HttpxRequest" not found (available types: Type1, Type2, Type3, Type4, Type5, Type6, Type7, Type8, Type9, Type10) and 3 more`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.err.Error(); got != tt.expectedMsg {
				t.Errorf("Error() = %q, want %q", got, tt.expectedMsg)
			}
		})
	}
}

// TestSentinelErrors tests the package-level sentinel errors.
func TestSentinelErrors(t *testing.T) {
	tests := []struct {
		name        string
		err         error
		expectedMsg string
	}{
		{
			name:        "ErrNoSchemaAvailable",
			err:         ErrNoSchemaAvailable,
			expectedMsg: "no schema available for type resolution",
		},
		{
			name:        "ErrInvalidFileDescriptorSet",
			err:         ErrInvalidFileDescriptorSet,
			expectedMsg: "invalid file descriptor set",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.err.Error(); got != tt.expectedMsg {
				t.Errorf("Error() = %q, want %q", got, tt.expectedMsg)
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

// TestSchemaNotFoundError_ErrorsAs tests error wrapping with errors.As().
func TestSchemaNotFoundError_ErrorsAs(t *testing.T) {
	err := &SchemaNotFoundError{
		ToolName: "test-tool",
		TypeName: "test.Type",
		Cause:    ErrNoSchemaAvailable,
	}

	// Test that errors.As works
	var schemaErr *SchemaNotFoundError
	if !errors.As(err, &schemaErr) {
		t.Error("errors.As(err, &schemaErr) = false, want true")
	}

	if schemaErr.ToolName != "test-tool" {
		t.Errorf("schemaErr.ToolName = %q, want %q", schemaErr.ToolName, "test-tool")
	}

	// Test that errors.As doesn't match unrelated types
	var typeErr *TypeNotFoundError
	if errors.As(err, &typeErr) {
		t.Error("errors.As(err, &typeErr) = true, want false")
	}
}

// TestTypeNotFoundError_ErrorsAs tests error wrapping with errors.As().
func TestTypeNotFoundError_ErrorsAs(t *testing.T) {
	err := &TypeNotFoundError{
		TypeName:       "test.Type",
		AvailableTypes: []string{"other.Type"},
	}

	// Test that errors.As works
	var typeErr *TypeNotFoundError
	if !errors.As(err, &typeErr) {
		t.Error("errors.As(err, &typeErr) = false, want true")
	}

	if typeErr.TypeName != "test.Type" {
		t.Errorf("typeErr.TypeName = %q, want %q", typeErr.TypeName, "test.Type")
	}

	// Test that errors.As doesn't match unrelated types
	var schemaErr *SchemaNotFoundError
	if errors.As(err, &schemaErr) {
		t.Error("errors.As(err, &schemaErr) = true, want false")
	}
}

// TestProtoResolverConfig tests the config struct defaults.
func TestProtoResolverConfig(t *testing.T) {
	// Test that we can create config with custom values
	config := ProtoResolverConfig{
		CacheMaxEntries: 200,
		CacheTTL:        30,
		StrictMode:      false,
		LogFallbacks:    true,
	}

	if config.CacheMaxEntries != 200 {
		t.Errorf("CacheMaxEntries = %d, want 200", config.CacheMaxEntries)
	}
	if config.CacheTTL != 30 {
		t.Errorf("CacheTTL = %d, want 30", config.CacheTTL)
	}
	if config.StrictMode {
		t.Error("StrictMode = true, want false")
	}
	if !config.LogFallbacks {
		t.Error("LogFallbacks = false, want true")
	}
}

// TestResolutionResult tests the ResolutionResult struct.
func TestResolutionResult(t *testing.T) {
	result := ResolutionResult{
		Message:   nil,
		Strategy:  "global",
		IsDynamic: false,
		ToolName:  "test-tool",
		TypeName:  "test.Type",
	}

	if result.Strategy != "global" {
		t.Errorf("Strategy = %q, want %q", result.Strategy, "global")
	}
	if result.IsDynamic {
		t.Error("IsDynamic = true, want false")
	}
	if result.ToolName != "test-tool" {
		t.Errorf("ToolName = %q, want %q", result.ToolName, "test-tool")
	}
	if result.TypeName != "test.Type" {
		t.Errorf("TypeName = %q, want %q", result.TypeName, "test.Type")
	}
}
