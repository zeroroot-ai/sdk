// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Zero Root AI

package sdk

import (
	"errors"
	"fmt"
	"io"
	"log/slog"
)

// Sentinel errors for common SDK error conditions.
// These errors can be used with errors.Is() for error checking.
var (
	// ErrAgentNotFound indicates the requested agent was not found in the registry.
	ErrAgentNotFound = errors.New("agent not found")

	// ErrToolNotFound indicates the requested tool was not found in the registry.
	ErrToolNotFound = errors.New("tool not found")

	// ErrPluginNotFound indicates the requested plugin was not found in the registry.
	ErrPluginNotFound = errors.New("plugin not found")

	// ErrInvalidConfig indicates the provided configuration is invalid or incomplete.
	ErrInvalidConfig = errors.New("invalid configuration")

	// ErrSlotNotSatisfied indicates that the LLM slot requirements cannot be met
	// by the available LLM providers or the requested capabilities are not available.
	ErrSlotNotSatisfied = errors.New("slot requirements not satisfied")

	// ErrExecutionFailed indicates that an agent, tool, or plugin execution failed.
	// The underlying error should be wrapped for additional context.
	ErrExecutionFailed = errors.New("execution failed")

	// ErrMissionNotFound indicates the requested mission was not found in the registry.
	ErrMissionNotFound = errors.New("mission not found")
)

// Error kinds categorize errors by their type.
const (
	// KindNotFound represents errors where a resource was not found.
	KindNotFound = "not_found"

	// KindValidation represents errors related to input validation.
	KindValidation = "validation"

	// KindExecution represents errors that occur during execution.
	KindExecution = "execution"

	// KindConfiguration represents errors related to configuration.
	KindConfiguration = "configuration"

	// KindNetwork represents errors related to network operations.
	KindNetwork = "network"

	// KindPermission represents errors related to permissions or authorization.
	KindPermission = "permission"

	// KindTimeout represents errors related to operation timeouts.
	KindTimeout = "timeout"

	// KindInternal represents internal SDK errors.
	KindInternal = "internal"
)

// SDKError is a structured error type that wraps underlying errors with
// additional context about the operation that failed and the category of error.
//
// SDKError implements the error interface and supports error unwrapping,
// making it compatible with errors.Is() and errors.As().
//
// Example usage:
//
//	err := &SDKError{
//		Op:   "Agent.Execute",
//		Kind: KindExecution,
//		Err:  ErrExecutionFailed,
//	}
type SDKError struct {
	// Op is the operation that failed (e.g., "Client.CreateAgent", "Tool.Execute").
	Op string

	// Kind categorizes the error (e.g., KindNotFound, KindValidation).
	Kind string

	// Err is the underlying error that caused this error.
	Err error

	// Context provides additional context about the error (optional).
	// This can include resource IDs, parameter values, or other debugging information.
	Context map[string]any
}

// Error implements the error interface, returning a formatted error message
// that includes the operation, kind, and underlying error.
func (e *SDKError) Error() string {
	if e.Err == nil {
		return fmt.Sprintf("sdk: %s: %s", e.Op, e.Kind)
	}

	if e.Context != nil && len(e.Context) > 0 {
		return fmt.Sprintf("sdk: %s (%s): %v [context: %+v]", e.Op, e.Kind, e.Err, e.Context)
	}

	return fmt.Sprintf("sdk: %s (%s): %v", e.Op, e.Kind, e.Err)
}

// Unwrap returns the underlying error, allowing errors.Is() and errors.As()
// to work correctly with wrapped errors.
func (e *SDKError) Unwrap() error {
	return e.Err
}

// Is implements error matching for SDKError, allowing comparison based on
// the underlying error or the SDKError itself.
func (e *SDKError) Is(target error) bool {
	if target == nil {
		return false
	}

	// Check if target is an SDKError with matching Kind
	if t, ok := target.(*SDKError); ok {
		// Match if both Op and Kind are the same, or if Kind matches and Op is empty in target
		if t.Kind != "" && e.Kind == t.Kind {
			if t.Op == "" || e.Op == t.Op {
				return true
			}
		}
	}

	// Delegate to underlying error
	return errors.Is(e.Err, target)
}

// WithContext returns a new SDKError with the provided context added.
// This is useful for adding debugging information to errors.
//
// Example:
//
//	err := &SDKError{
//		Op:   "Agent.Execute",
//		Kind: KindExecution,
//		Err:  ErrExecutionFailed,
//	}
//	err = err.WithContext(map[string]any{
//		"agent_id": "my-agent",
//		"input_length": 1024,
//	})
func (e *SDKError) WithContext(ctx map[string]any) *SDKError {
	newErr := *e
	if newErr.Context == nil {
		newErr.Context = make(map[string]any)
	}
	for k, v := range ctx {
		newErr.Context[k] = v
	}
	return &newErr
}

// NewNotFoundError creates a new SDKError with KindNotFound.
func NewNotFoundError(op string, err error) *SDKError {
	return &SDKError{
		Op:   op,
		Kind: KindNotFound,
		Err:  err,
	}
}

// NewValidationError creates a new SDKError with KindValidation.
func NewValidationError(op string, err error) *SDKError {
	return &SDKError{
		Op:   op,
		Kind: KindValidation,
		Err:  err,
	}
}

// NewExecutionError creates a new SDKError with KindExecution.
func NewExecutionError(op string, err error) *SDKError {
	return &SDKError{
		Op:   op,
		Kind: KindExecution,
		Err:  err,
	}
}

// NewConfigurationError creates a new SDKError with KindConfiguration.
func NewConfigurationError(op string, err error) *SDKError {
	return &SDKError{
		Op:   op,
		Kind: KindConfiguration,
		Err:  err,
	}
}

// NewNetworkError creates a new SDKError with KindNetwork.
func NewNetworkError(op string, err error) *SDKError {
	return &SDKError{
		Op:   op,
		Kind: KindNetwork,
		Err:  err,
	}
}

// NewPermissionError creates a new SDKError with KindPermission.
func NewPermissionError(op string, err error) *SDKError {
	return &SDKError{
		Op:   op,
		Kind: KindPermission,
		Err:  err,
	}
}

// NewTimeoutError creates a new SDKError with KindTimeout.
func NewTimeoutError(op string, err error) *SDKError {
	return &SDKError{
		Op:   op,
		Kind: KindTimeout,
		Err:  err,
	}
}

// NewInternalError creates a new SDKError with KindInternal.
func NewInternalError(op string, err error) *SDKError {
	return &SDKError{
		Op:   op,
		Kind: KindInternal,
		Err:  err,
	}
}

// CloseWithLog attempts to close the provided resource and logs any error
// at warning level. This is intended for use in defer statements to ensure
// cleanup errors are not silently ignored.
//
// The name parameter should describe the resource being closed (e.g., "file",
// "connection", "database"). If logger is nil, slog.Default() is used.
//
// Example usage:
//
//	defer sdk.CloseWithLog(file, logger, "config file")
//	defer sdk.CloseWithLog(conn, logger, "gRPC connection")
func CloseWithLog(closer io.Closer, logger *slog.Logger, name string) {
	if closer == nil {
		return
	}

	if logger == nil {
		logger = slog.Default()
	}

	if err := closer.Close(); err != nil {
		logger.Warn("failed to close resource",
			"resource", name,
			"error", err)
	}
}
