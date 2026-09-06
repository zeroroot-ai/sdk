// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Zero Root AI

// Package toolerr provides structured error types for Gibson tools.
//
// # Overview
//
// This package defines standard error codes and a structured Error type
// for consistent error reporting across all Gibson tools. It integrates
// seamlessly with Go's standard errors package for error wrapping and unwrapping.
//
// # Error Codes
//
// Standard error codes are defined as constants:
//
//   - ErrCodeBinaryNotFound: Required binary not in PATH
//   - ErrCodeExecutionFailed: Command execution failed
//   - ErrCodeTimeout: Operation timed out
//   - ErrCodeParseError: Failed to parse output or data
//   - ErrCodeInvalidInput: Invalid input parameters
//   - ErrCodeDependencyMissing: Required dependency missing
//   - ErrCodePermissionDenied: Insufficient permissions
//   - ErrCodeNetworkError: Network-related error
//
// # Usage
//
// Create a basic error:
//
//	err := toolerr.New("mytool", "scan", toolerr.ErrCodeBinaryNotFound,
//	    "mytool binary not found in PATH")
//
// Add context with method chaining:
//
//	err := toolerr.New("kubectl", "apply", toolerr.ErrCodeExecutionFailed,
//	    "command failed").
//	    WithCause(execErr).
//	    WithDetails(map[string]any{
//	        "namespace": "default",
//	        "resource": "deployment",
//	    })
//
// Check for specific errors:
//
//	if errors.Is(err, toolerr.ErrTimeout) {
//	    // Handle timeout
//	}
//
// Extract error details:
//
//	var toolErr *toolerr.Error
//	if errors.As(err, &toolErr) {
//	    fmt.Printf("Tool: %s, Operation: %s, Code: %s\n",
//	        toolErr.Tool, toolErr.Operation, toolErr.Code)
//	}
//
// # Integration with errors package
//
// The Error type implements:
//   - error interface via Error() method
//   - errors.Unwrap via Unwrap() method
//   - errors.Is via Is() method
//   - errors.As via As() method
//
// This ensures full compatibility with Go's error handling patterns.
package toolerr
