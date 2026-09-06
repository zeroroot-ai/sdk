// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Zero Root AI

package toolerr_test

import (
	"errors"
	"fmt"

	"github.com/zeroroot-ai/sdk/toolerr"
)

// Example demonstrates basic usage of the toolerr package.
func Example() {
	// Create a simple error
	err1 := toolerr.New("mytool", "scan", toolerr.ErrCodeBinaryNotFound,
		"mytool binary not found in PATH")
	fmt.Println(err1)

	// Create an error with cause and details
	execErr := errors.New("exit status 1")
	err2 := toolerr.New("kubectl", "apply", toolerr.ErrCodeExecutionFailed,
		"command failed").
		WithCause(execErr).
		WithDetails(map[string]any{
			"namespace": "default",
			"resource":  "deployment",
		})
	fmt.Println(err2)

	// Check error type
	var toolErr *toolerr.Error
	if errors.As(err2, &toolErr) {
		fmt.Printf("Tool: %s, Code: %s\n", toolErr.Tool, toolErr.Code)
	}

	// Output:
	// mytool [scan/BINARY_NOT_FOUND]: mytool binary not found in PATH
	// kubectl [apply/EXECUTION_FAILED]: command failed: exit status 1
	// Tool: kubectl, Code: EXECUTION_FAILED
}

// Example_wrapping demonstrates error wrapping patterns.
func Example_wrapping() {
	// Original error
	baseErr := errors.New("connection refused")

	// Wrap with tool error
	err := toolerr.New("terraform", "plan", toolerr.ErrCodeNetworkError,
		"failed to connect to AWS").
		WithCause(baseErr)

	// Check if error chain contains specific error
	if errors.Is(err, baseErr) {
		fmt.Println("Error chain contains base error")
	}

	// Output:
	// Error chain contains base error
}

// Example_errorCodes demonstrates using standard error codes.
func Example_errorCodes() {
	codes := []string{
		toolerr.ErrCodeBinaryNotFound,
		toolerr.ErrCodeExecutionFailed,
		toolerr.ErrCodeTimeout,
		toolerr.ErrCodeParseError,
		toolerr.ErrCodeInvalidInput,
		toolerr.ErrCodeDependencyMissing,
		toolerr.ErrCodePermissionDenied,
		toolerr.ErrCodeNetworkError,
	}

	fmt.Printf("Available error codes: %d\n", len(codes))
	fmt.Printf("Example: %s\n", codes[0])

	// Output:
	// Available error codes: 8
	// Example: BINARY_NOT_FOUND
}
