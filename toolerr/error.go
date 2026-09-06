// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Zero Root AI

// Package toolerr provides structured error types for Gibson tools.
//
// This package defines standard error codes and a structured Error type
// that includes tool context, operation details, error codes, and cause chains.
// It integrates with Go's standard errors package for error wrapping and unwrapping.
package toolerr

import (
	"errors"
	"fmt"
	"strings"
)

// Standard error codes used across tools for consistent error reporting.
const (
	// ErrCodeBinaryNotFound indicates a required binary is not in PATH
	ErrCodeBinaryNotFound = "BINARY_NOT_FOUND"

	// ErrCodeExecutionFailed indicates command execution failed
	ErrCodeExecutionFailed = "EXECUTION_FAILED"

	// ErrCodeTimeout indicates an operation timed out
	ErrCodeTimeout = "TIMEOUT"

	// ErrCodeParseError indicates failure to parse output or data
	ErrCodeParseError = "PARSE_ERROR"

	// ErrCodeInvalidInput indicates invalid input parameters
	ErrCodeInvalidInput = "INVALID_INPUT"

	// ErrCodeDependencyMissing indicates a required dependency is missing
	ErrCodeDependencyMissing = "DEPENDENCY_MISSING"

	// ErrCodePermissionDenied indicates insufficient permissions
	ErrCodePermissionDenied = "PERMISSION_DENIED"

	// ErrCodeNetworkError indicates a network-related error
	ErrCodeNetworkError = "NETWORK_ERROR"
)

// Error is a structured error type for tool operations.
// It provides context about which tool and operation failed,
// includes a standard error code, and can wrap underlying errors.
type Error struct {
	// Tool is the name of the tool that generated the error
	Tool string

	// Operation is the specific operation that failed
	Operation string

	// Code is a standard error code constant
	Code string

	// Message is a human-readable error message
	Message string

	// Details contains additional context as key-value pairs
	Details map[string]any

	// Cause is the underlying error that caused this error
	Cause error

	// Class categorizes the error by its nature for semantic understanding
	Class ErrorClass `json:"class,omitempty"`

	// Hints provides recovery suggestions for this error
	Hints []RecoveryHint `json:"hints,omitempty"`
}

// New creates a new structured tool error.
//
// Parameters:
//   - tool: name of the tool (e.g., "mytool", "kubectl")
//   - operation: operation that failed (e.g., "scan", "execute")
//   - code: error code constant (e.g., ErrCodeBinaryNotFound)
//   - message: human-readable error description
//
// Example:
//
//	err := toolerr.New("mytool", "scan", toolerr.ErrCodeBinaryNotFound, "mytool binary not found in PATH")
func New(tool, operation, code, message string) *Error {
	return &Error{
		Tool:      tool,
		Operation: operation,
		Code:      code,
		Message:   message,
	}
}

// WithCause adds an underlying error to this error.
// This method returns the same error instance for method chaining.
//
// Example:
//
//	err := toolerr.New("mytool", "scan", toolerr.ErrCodeExecutionFailed, "scan failed").
//	    WithCause(execErr)
func (e *Error) WithCause(err error) *Error {
	e.Cause = err
	return e
}

// WithDetails adds additional context to this error.
// This method returns the same error instance for method chaining.
//
// Example:
//
//	err := toolerr.New("mytool", "scan", toolerr.ErrCodeTimeout, "scan timed out").
//	    WithDetails(map[string]any{"timeout": "30s", "target": "192.168.1.1"})
func (e *Error) WithDetails(details map[string]any) *Error {
	e.Details = details
	return e
}

// WithClass sets the error classification for semantic understanding.
// This method returns the same error instance for method chaining.
//
// Example:
//
//	err := toolerr.New("mytool", "scan", toolerr.ErrCodeBinaryNotFound, "mytool not found").
//	    WithClass(toolerr.ErrorClassInfrastructure)
func (e *Error) WithClass(class ErrorClass) *Error {
	e.Class = class
	return e
}

// WithHints adds recovery suggestions to this error.
// This method appends hints and returns the same error instance for method chaining.
//
// Example:
//
//	err := toolerr.New("mytool", "scan", toolerr.ErrCodeBinaryNotFound, "mytool not found").
//	    WithHints(toolerr.RecoveryHint{
//	        Strategy:    toolerr.StrategyUseAlternative,
//	        Alternative: "masscan",
//	        Reason:      "masscan can perform similar port scanning",
//	        Confidence:  0.8,
//	        Priority:    1,
//	    })
func (e *Error) WithHints(hints ...RecoveryHint) *Error {
	e.Hints = append(e.Hints, hints...)
	return e
}

// Error implements the error interface.
// It formats the error as: "tool [operation/code]: message: cause"
//
// Examples:
//   - "mytool [scan/BINARY_NOT_FOUND]: mytool binary not found in PATH"
//   - "kubectl [execute/EXECUTION_FAILED]: command failed: exit status 1"
func (e *Error) Error() string {
	var parts []string

	// Start with tool [operation/code]
	parts = append(parts, fmt.Sprintf("%s [%s/%s]", e.Tool, e.Operation, e.Code))

	// Add message
	if e.Message != "" {
		parts = append(parts, e.Message)
	}

	// Add cause if present
	if e.Cause != nil {
		parts = append(parts, e.Cause.Error())
	}

	return strings.Join(parts, ": ")
}

// Unwrap returns the underlying cause error.
// This enables errors.Is() and errors.As() to work with wrapped errors.
//
// Example:
//
//	err := toolerr.New("mytool", "scan", toolerr.ErrCodeExecutionFailed, "scan failed").
//	    WithCause(context.DeadlineExceeded)
//	if errors.Is(err, context.DeadlineExceeded) {
//	    // Handle timeout
//	}
func (e *Error) Unwrap() error {
	return e.Cause
}

// Is implements error equality checking for errors.Is().
// Two Error values are considered equal if they have the same Tool, Operation, and Code.
func (e *Error) Is(target error) bool {
	t, ok := target.(*Error)
	if !ok {
		return false
	}
	return e.Tool == t.Tool && e.Operation == t.Operation && e.Code == t.Code
}

// As implements error type assertion for errors.As().
// This allows errors.As() to extract the Error type from wrapped errors.
func (e *Error) As(target any) bool {
	t, ok := target.(**Error)
	if !ok {
		return false
	}
	*t = e
	return true
}

// Sentinel errors for common scenarios

var (
	// ErrBinaryNotFound is returned when a required binary is not in PATH
	ErrBinaryNotFound = errors.New("binary not found")

	// ErrTimeout is returned when an operation times out
	ErrTimeout = errors.New("operation timed out")

	// ErrInvalidInput is returned when input validation fails
	ErrInvalidInput = errors.New("invalid input")
)
