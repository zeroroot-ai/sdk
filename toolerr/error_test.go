// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Zero Root AI

package toolerr

import (
	"context"
	"errors"
	"fmt"
	"testing"
)

// TestNew verifies that New() creates a correct Error with all fields set.
func TestNew(t *testing.T) {
	tests := []struct {
		name      string
		tool      string
		operation string
		code      string
		message   string
	}{
		{
			name:      "complete error",
			tool:      "mytool",
			operation: "scan",
			code:      ErrCodeBinaryNotFound,
			message:   "mytool binary not found in PATH",
		},
		{
			name:      "empty message",
			tool:      "kubectl",
			operation: "apply",
			code:      ErrCodeExecutionFailed,
			message:   "",
		},
		{
			name:      "all fields populated",
			tool:      "terraform",
			operation: "plan",
			code:      ErrCodeTimeout,
			message:   "operation timed out after 30s",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := New(tt.tool, tt.operation, tt.code, tt.message)

			if err.Tool != tt.tool {
				t.Errorf("Tool = %q, want %q", err.Tool, tt.tool)
			}
			if err.Operation != tt.operation {
				t.Errorf("Operation = %q, want %q", err.Operation, tt.operation)
			}
			if err.Code != tt.code {
				t.Errorf("Code = %q, want %q", err.Code, tt.code)
			}
			if err.Message != tt.message {
				t.Errorf("Message = %q, want %q", err.Message, tt.message)
			}
			if err.Details != nil {
				t.Errorf("Details = %v, want nil", err.Details)
			}
			if err.Cause != nil {
				t.Errorf("Cause = %v, want nil", err.Cause)
			}
		})
	}
}

// TestWithCause verifies that WithCause() correctly sets the underlying error.
func TestWithCause(t *testing.T) {
	tests := []struct {
		name  string
		cause error
	}{
		{
			name:  "standard error",
			cause: errors.New("underlying error"),
		},
		{
			name:  "context deadline exceeded",
			cause: context.DeadlineExceeded,
		},
		{
			name:  "fmt error",
			cause: fmt.Errorf("wrapped: %w", errors.New("original")),
		},
		{
			name:  "nil cause",
			cause: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := New("test", "operation", ErrCodeExecutionFailed, "test message").
				WithCause(tt.cause)

			if !errors.Is(err.Cause, tt.cause) {
				t.Errorf("Cause = %v, want %v", err.Cause, tt.cause)
			}
		})
	}
}

// TestWithDetails verifies that WithDetails() correctly sets the Details map.
func TestWithDetails(t *testing.T) {
	tests := []struct {
		name    string
		details map[string]any
	}{
		{
			name: "string values",
			details: map[string]any{
				"target": "192.168.1.1",
				"port":   "80",
			},
		},
		{
			name: "mixed types",
			details: map[string]any{
				"timeout": "30s",
				"retries": 3,
				"success": false,
			},
		},
		{
			name:    "nil details",
			details: nil,
		},
		{
			name:    "empty map",
			details: map[string]any{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := New("test", "operation", ErrCodeNetworkError, "test message").
				WithDetails(tt.details)

			if len(err.Details) != len(tt.details) {
				t.Errorf("Details length = %d, want %d", len(err.Details), len(tt.details))
			}

			for k, v := range tt.details {
				if err.Details[k] != v {
					t.Errorf("Details[%q] = %v, want %v", k, err.Details[k], v)
				}
			}
		})
	}
}

// TestMethodChaining verifies that WithCause() and WithDetails() can be chained.
func TestMethodChaining(t *testing.T) {
	cause := errors.New("underlying error")
	details := map[string]any{
		"key1": "value1",
		"key2": 42,
	}

	// Test WithCause then WithDetails
	err1 := New("test", "op1", ErrCodeTimeout, "msg1").
		WithCause(cause).
		WithDetails(details)

	if !errors.Is(err1.Cause, cause) {
		t.Errorf("err1.Cause = %v, want %v", err1.Cause, cause)
	}
	if len(err1.Details) != len(details) {
		t.Errorf("err1.Details length = %d, want %d", len(err1.Details), len(details))
	}

	// Test WithDetails then WithCause (reverse order)
	err2 := New("test", "op2", ErrCodeParseError, "msg2").
		WithDetails(details).
		WithCause(cause)

	if !errors.Is(err2.Cause, cause) {
		t.Errorf("err2.Cause = %v, want %v", err2.Cause, cause)
	}
	if len(err2.Details) != len(details) {
		t.Errorf("err2.Details length = %d, want %d", len(err2.Details), len(details))
	}
}

// TestErrorFormatting verifies the Error() method formats correctly.
func TestErrorFormatting(t *testing.T) {
	tests := []struct {
		name     string
		err      *Error
		expected string
	}{
		{
			name:     "simple error without cause",
			err:      New("mytool", "scan", ErrCodeBinaryNotFound, "binary not found"),
			expected: "mytool [scan/BINARY_NOT_FOUND]: binary not found",
		},
		{
			name: "error with cause",
			err: New("kubectl", "apply", ErrCodeExecutionFailed, "command failed").
				WithCause(errors.New("exit status 1")),
			expected: "kubectl [apply/EXECUTION_FAILED]: command failed: exit status 1",
		},
		{
			name:     "error without message",
			err:      New("tool", "op", ErrCodeTimeout, ""),
			expected: "tool [op/TIMEOUT]",
		},
		{
			name: "error with nested cause",
			err: New("terraform", "plan", ErrCodeNetworkError, "connection failed").
				WithCause(fmt.Errorf("dial: %w", errors.New("connection refused"))),
			expected: "terraform [plan/NETWORK_ERROR]: connection failed: dial: connection refused",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.err.Error()
			if got != tt.expected {
				t.Errorf("Error() = %q, want %q", got, tt.expected)
			}
		})
	}
}

// TestUnwrap verifies that Unwrap() returns the cause error.
func TestUnwrap(t *testing.T) {
	tests := []struct {
		name     string
		cause    error
		expected error
	}{
		{
			name:     "with cause",
			cause:    errors.New("underlying"),
			expected: errors.New("underlying"),
		},
		{
			name:     "without cause",
			cause:    nil,
			expected: nil,
		},
		{
			name:     "context deadline",
			cause:    context.DeadlineExceeded,
			expected: context.DeadlineExceeded,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := New("test", "op", ErrCodeTimeout, "msg")
			if tt.cause != nil {
				err = err.WithCause(tt.cause)
			}

			got := err.Unwrap()
			if !errors.Is(got, tt.cause) {
				t.Errorf("Unwrap() = %v, want %v", got, tt.cause)
			}
		})
	}
}

// TestErrorsIs verifies errors.Is() compatibility.
func TestErrorsIs(t *testing.T) {
	baseErr := errors.New("base error")
	toolErr := New("test", "op", ErrCodeTimeout, "timeout").WithCause(baseErr)

	// Test errors.Is with the cause
	if !errors.Is(toolErr, baseErr) {
		t.Error("errors.Is(toolErr, baseErr) = false, want true")
	}

	// Test errors.Is with context.DeadlineExceeded
	timeoutErr := New("test", "op", ErrCodeTimeout, "timeout").
		WithCause(context.DeadlineExceeded)
	if !errors.Is(timeoutErr, context.DeadlineExceeded) {
		t.Error("errors.Is(timeoutErr, context.DeadlineExceeded) = false, want true")
	}

	// Test errors.Is with unrelated error
	unrelatedErr := errors.New("unrelated")
	if errors.Is(toolErr, unrelatedErr) {
		t.Error("errors.Is(toolErr, unrelatedErr) = true, want false")
	}

	// Test errors.Is with same Tool/Operation/Code
	err1 := New("mytool", "scan", ErrCodeBinaryNotFound, "msg1")
	err2 := New("mytool", "scan", ErrCodeBinaryNotFound, "msg2")
	if !errors.Is(err1, err2) {
		t.Error("errors.Is(err1, err2) = false, want true (same tool/op/code)")
	}

	// Test errors.Is with different Code
	err3 := New("mytool", "scan", ErrCodeTimeout, "msg3")
	if errors.Is(err1, err3) {
		t.Error("errors.Is(err1, err3) = true, want false (different code)")
	}
}

// TestErrorsAs verifies errors.As() compatibility.
func TestErrorsAs(t *testing.T) {
	toolErr := New("test", "op", ErrCodeExecutionFailed, "msg").
		WithCause(errors.New("underlying"))

	// Test errors.As extraction
	var extracted *Error
	if !errors.As(toolErr, &extracted) {
		t.Fatal("errors.As(toolErr, &extracted) = false, want true")
	}

	if extracted.Tool != "test" {
		t.Errorf("extracted.Tool = %q, want %q", extracted.Tool, "test")
	}
	if extracted.Operation != "op" {
		t.Errorf("extracted.Operation = %q, want %q", extracted.Operation, "op")
	}
	if extracted.Code != ErrCodeExecutionFailed {
		t.Errorf("extracted.Code = %q, want %q", extracted.Code, ErrCodeExecutionFailed)
	}

	// Test errors.As with wrapped error
	wrappedErr := fmt.Errorf("wrapper: %w", toolErr)
	var extracted2 *Error
	if !errors.As(wrappedErr, &extracted2) {
		t.Fatal("errors.As(wrappedErr, &extracted2) = false, want true")
	}

	if extracted2.Tool != "test" {
		t.Errorf("extracted2.Tool = %q, want %q", extracted2.Tool, "test")
	}
}

// TestErrorCodeConstants verifies that all error code constants are defined.
func TestErrorCodeConstants(t *testing.T) {
	codes := []string{
		ErrCodeBinaryNotFound,
		ErrCodeExecutionFailed,
		ErrCodeTimeout,
		ErrCodeParseError,
		ErrCodeInvalidInput,
		ErrCodeDependencyMissing,
		ErrCodePermissionDenied,
		ErrCodeNetworkError,
	}

	for _, code := range codes {
		if code == "" {
			t.Errorf("error code is empty")
		}
		// Verify code is uppercase (allowing underscores)
		for _, r := range code {
			if r != '_' && (r < 'A' || r > 'Z') {
				t.Errorf("error code %q contains non-uppercase character %q", code, r)
			}
		}
	}
}

// TestSentinelErrors verifies that sentinel errors are defined.
func TestSentinelErrors(t *testing.T) {
	sentinels := []error{
		ErrBinaryNotFound,
		ErrTimeout,
		ErrInvalidInput,
	}

	for i, sentinel := range sentinels {
		if sentinel == nil {
			t.Errorf("sentinel error %d is nil", i)
		}
		if sentinel.Error() == "" {
			t.Errorf("sentinel error %d has empty message", i)
		}
	}
}

// BenchmarkNew benchmarks the New() function.
func BenchmarkNew(b *testing.B) {
	for range b.N {
		_ = New("test", "operation", ErrCodeTimeout, "message")
	}
}

// BenchmarkWithCause benchmarks the WithCause() method.
func BenchmarkWithCause(b *testing.B) {
	cause := errors.New("underlying")
	b.ResetTimer()
	for range b.N {
		_ = New("test", "op", ErrCodeTimeout, "msg").WithCause(cause)
	}
}

// BenchmarkWithDetails benchmarks the WithDetails() method.
func BenchmarkWithDetails(b *testing.B) {
	details := map[string]any{"key": "value"}
	b.ResetTimer()
	for range b.N {
		_ = New("test", "op", ErrCodeTimeout, "msg").WithDetails(details)
	}
}

// BenchmarkErrorFormatting benchmarks the Error() method.
func BenchmarkErrorFormatting(b *testing.B) {
	err := New("test", "operation", ErrCodeTimeout, "message").
		WithCause(errors.New("underlying"))
	b.ResetTimer()
	for range b.N {
		_ = err.Error()
	}
}

// ExampleNew demonstrates basic error creation.
func ExampleNew() {
	err := New("mytool", "scan", ErrCodeBinaryNotFound, "mytool binary not found in PATH")
	fmt.Println(err)
	// Output: mytool [scan/BINARY_NOT_FOUND]: mytool binary not found in PATH
}

// ExampleError_WithCause demonstrates adding a cause to an error.
func ExampleError_WithCause() {
	baseErr := errors.New("exit status 1")
	err := New("kubectl", "apply", ErrCodeExecutionFailed, "command failed").
		WithCause(baseErr)
	fmt.Println(err)
	// Output: kubectl [apply/EXECUTION_FAILED]: command failed: exit status 1
}

// ExampleError_WithDetails demonstrates adding context details.
func ExampleError_WithDetails() {
	err := New("terraform", "plan", ErrCodeTimeout, "operation timed out").
		WithDetails(map[string]any{
			"timeout": "30s",
			"target":  "vpc-12345",
		})
	fmt.Println(err)
	// Output: terraform [plan/TIMEOUT]: operation timed out
}

// ExampleError_WithCause_chaining demonstrates method chaining.
func ExampleError_WithCause_chaining() {
	err := New("mytool", "scan", ErrCodeNetworkError, "connection failed").
		WithCause(errors.New("connection refused")).
		WithDetails(map[string]any{"host": "192.168.1.1", "port": 80})
	fmt.Println(err)
	// Output: mytool [scan/NETWORK_ERROR]: connection failed: connection refused
}
