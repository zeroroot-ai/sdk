// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Zero Root AI

// Package toolerr provides structured error types for Gibson tools.
//
// This package defines standard error codes and a structured Error type
// that includes tool context, operation details, error codes, and cause chains.
// It integrates with Go's standard errors package for error wrapping and unwrapping.
package toolerr

// Standard error codes used across tools for consistent error reporting.
const (
	// ErrCodeBinaryNotFound indicates a required binary is not in PATH
	ErrCodeBinaryNotFound = "BINARY_NOT_FOUND"

	// ErrCodeExecutionFailed indicates command execution failed
	ErrCodeExecutionFailed = "EXECUTION_FAILED"

	// ErrCodeTimeout indicates an operation timed out
	ErrCodeTimeout = "TIMEOUT"

	// ErrCodeDependencyMissing indicates a required dependency is missing
	ErrCodeDependencyMissing = "DEPENDENCY_MISSING"

	// ErrCodePermissionDenied indicates insufficient permissions
	ErrCodePermissionDenied = "PERMISSION_DENIED"

	// ErrCodeNetworkError indicates a network-related error
	ErrCodeNetworkError = "NETWORK_ERROR"
)

// Sentinel errors for common scenarios
