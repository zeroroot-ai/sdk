// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Zero Root AI

package agent

import (
	"encoding/json"
	"errors"
	"fmt"
	"runtime"
	"strings"
)

// ResultError is a JSON-serializable error type for agent results.
// It provides structured error information that can be transmitted
// across agent boundaries, stored in databases, and presented to users.
//
// ResultError supports error wrapping and integrates with Go's standard
// errors package for error chain traversal.
type ResultError struct {
	// Code is a standard error code from the error taxonomy
	Code string `json:"code"`

	// Message is a human-readable error description
	Message string `json:"message"`

	// Details contains additional context as key-value pairs
	Details map[string]any `json:"details,omitempty"`

	// Cause is the wrapped underlying error
	Cause *ResultError `json:"cause,omitempty"`

	// Retryable indicates whether the operation can be retried
	Retryable bool `json:"retryable"`

	// Component identifies the source component (agent, tool, or system)
	Component string `json:"component,omitempty"`

	// Stack contains an optional stack trace for debugging
	Stack string `json:"stack,omitempty"`
}

// Error implements the error interface.
// It formats the error as: "component [code]: message"
//
// Examples:
//   - "sql-injector [EXECUTION_FAILED]: failed to execute probe"
//   - "[TIMEOUT]: operation timed out"
func (e *ResultError) Error() string {
	var parts []string

	// Start with component and code
	if e.Component != "" {
		parts = append(parts, fmt.Sprintf("%s [%s]", e.Component, e.Code))
	} else {
		parts = append(parts, fmt.Sprintf("[%s]", e.Code))
	}

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

// NewResultError creates a new ResultError with the given code and message.
//
// Parameters:
//   - code: error code from the taxonomy (e.g., "EXECUTION_FAILED")
//   - message: human-readable error description
//
// Example:
//
//	err := agent.NewResultError("TIMEOUT", "LLM request timed out after 30s")
func NewResultError(code, message string) *ResultError {
	return &ResultError{
		Code:      code,
		Message:   message,
		Details:   nil,
		Retryable: false,
	}
}

// Wrap creates a new ResultError that wraps an existing error.
// If the existing error is already a ResultError, it is used as the cause.
// Otherwise, it is converted to a ResultError first using FromError.
//
// Parameters:
//   - err: the error to wrap
//   - code: error code for the new error
//   - message: human-readable message for the new error
//
// Example:
//
//	err := tool.Execute(ctx, input)
//	if err != nil {
//	    return agent.Wrap(err, "TOOL_FAILED", "Failed to execute reconnaissance tool")
//	}
func Wrap(err error, code, message string) *ResultError {
	if err == nil {
		return NewResultError(code, message)
	}

	wrapped := &ResultError{
		Code:    code,
		Message: message,
	}

	// If the error is already a ResultError, use it directly as cause
	re := &ResultError{}
	if errors.As(err, &re) {
		wrapped.Cause = re
		return wrapped
	}

	// Otherwise, convert it to a ResultError
	wrapped.Cause = FromError(err)
	return wrapped
}

// FromError converts any error to a ResultError.
// If the error is already a ResultError, it is returned as-is.
// If the error is nil, nil is returned.
// Otherwise, a new ResultError is created with code "UNKNOWN" and the error's message.
//
// Example:
//
//	err := someOperation()
//	resultErr := agent.FromError(err)
//	// Now resultErr can be serialized to JSON
func FromError(err error) *ResultError {
	if err == nil {
		return nil
	}

	// If already a ResultError, return as-is
	re := &ResultError{}
	if errors.As(err, &re) {
		return re
	}

	// Convert standard error to ResultError
	return &ResultError{
		Code:    "UNKNOWN",
		Message: err.Error(),
	}
}

// Unwrap returns the underlying cause error.
// This enables errors.Is() and errors.As() to work with wrapped ResultErrors.
//
// Example:
//
//	baseErr := agent.NewResultError("TIMEOUT", "operation timed out")
//	wrappedErr := agent.Wrap(baseErr, "TASK_FAILED", "task failed due to timeout")
//	if errors.Is(wrappedErr, baseErr) {
//	    // Handle timeout
//	}
func (e *ResultError) Unwrap() error {
	if e.Cause == nil {
		return nil
	}
	return e.Cause
}

// WithDetails adds additional context to this error.
// This method returns the same error instance for method chaining.
//
// Example:
//
//	err := agent.NewResultError("EXECUTION_FAILED", "probe failed").
//	    WithDetails(map[string]any{
//	        "target": "https://example.com",
//	        "status_code": 500,
//	    })
func (e *ResultError) WithDetails(details map[string]any) *ResultError {
	if e.Details == nil {
		e.Details = make(map[string]any)
	}
	for k, v := range details {
		e.Details[k] = v
	}
	return e
}

// WithRetryable sets the retryable flag.
// This method returns the same error instance for method chaining.
//
// Example:
//
//	err := agent.NewResultError("NETWORK_ERROR", "connection refused").
//	    WithRetryable(true)
func (e *ResultError) WithRetryable(retryable bool) *ResultError {
	e.Retryable = retryable
	return e
}

// WithComponent sets the component that generated this error.
// This method returns the same error instance for method chaining.
//
// Example:
//
//	err := agent.NewResultError("TIMEOUT", "operation timed out").
//	    WithComponent("sql-injection-agent")
func (e *ResultError) WithComponent(component string) *ResultError {
	e.Component = component
	return e
}

// WithStack captures a stack trace at the current location.
// This method returns the same error instance for method chaining.
//
// Stack traces are useful for debugging but should be used sparingly
// in production as they can be expensive to capture and store.
//
// Example:
//
//	err := agent.NewResultError("INTERNAL_ERROR", "unexpected panic").
//	    WithStack()
func (e *ResultError) WithStack() *ResultError {
	e.Stack = captureStack(3) // skip 3 frames: captureStack, WithStack, caller
	return e
}

// MarshalJSON implements json.Marshaler to ensure ResultError can be serialized.
func (e *ResultError) MarshalJSON() ([]byte, error) {
	type Alias ResultError
	return json.Marshal((*Alias)(e))
}

// UnmarshalJSON implements json.Unmarshaler to ensure ResultError can be deserialized.
func (e *ResultError) UnmarshalJSON(data []byte) error {
	type Alias ResultError
	return json.Unmarshal(data, (*Alias)(e))
}

// captureStack captures a stack trace starting from skip frames above the caller.
func captureStack(skip int) string {
	var buf strings.Builder
	pcs := make([]uintptr, 32)
	n := runtime.Callers(skip, pcs)
	frames := runtime.CallersFrames(pcs[:n])

	for {
		frame, more := frames.Next()
		fmt.Fprintf(&buf, "%s\n\t%s:%d\n", frame.Function, frame.File, frame.Line)
		if !more {
			break
		}
	}

	return buf.String()
}
