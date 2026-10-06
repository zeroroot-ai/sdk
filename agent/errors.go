// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Zero Root AI

package agent

import (
	"encoding/json"
	"fmt"
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
