// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Zero Root AI

package agent

import (
	"encoding/json"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewResultError(t *testing.T) {
	tests := []struct {
		name    string
		code    string
		message string
		want    *ResultError
	}{
		{
			name:    "basic error",
			code:    "TIMEOUT",
			message: "operation timed out",
			want: &ResultError{
				Code:      "TIMEOUT",
				Message:   "operation timed out",
				Retryable: false,
			},
		},
		{
			name:    "empty message",
			code:    "UNKNOWN",
			message: "",
			want: &ResultError{
				Code:      "UNKNOWN",
				Message:   "",
				Retryable: false,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := NewResultError(tt.code, tt.message)
			assert.Equal(t, tt.want.Code, got.Code)
			assert.Equal(t, tt.want.Message, got.Message)
			assert.Equal(t, tt.want.Retryable, got.Retryable)
			assert.Nil(t, got.Details)
			assert.Nil(t, got.Cause)
			assert.Empty(t, got.Component)
			assert.Empty(t, got.Stack)
		})
	}
}

func TestResultError_Error(t *testing.T) {
	tests := []struct {
		name string
		err  *ResultError
		want string
	}{
		{
			name: "with component",
			err: &ResultError{
				Code:      "TIMEOUT",
				Message:   "operation timed out",
				Component: "sql-injector",
			},
			want: "sql-injector [TIMEOUT]: operation timed out",
		},
		{
			name: "without component",
			err: &ResultError{
				Code:    "NETWORK_ERROR",
				Message: "connection refused",
			},
			want: "[NETWORK_ERROR]: connection refused",
		},
		{
			name: "with cause",
			err: &ResultError{
				Code:    "TASK_FAILED",
				Message: "task execution failed",
				Cause: &ResultError{
					Code:    "TIMEOUT",
					Message: "operation timed out",
				},
			},
			want: "[TASK_FAILED]: task execution failed: [TIMEOUT]: operation timed out",
		},
		{
			name: "empty message",
			err: &ResultError{
				Code:      "UNKNOWN",
				Component: "test-agent",
			},
			want: "test-agent [UNKNOWN]",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.err.Error()
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestWrap(t *testing.T) {
	tests := []struct {
		name    string
		err     error
		code    string
		message string
		verify  func(t *testing.T, got *ResultError)
	}{
		{
			name:    "wrap nil error",
			err:     nil,
			code:    "TASK_FAILED",
			message: "task failed",
			verify: func(t *testing.T, got *ResultError) {
				assert.Equal(t, "TASK_FAILED", got.Code)
				assert.Equal(t, "task failed", got.Message)
				assert.Nil(t, got.Cause)
			},
		},
		{
			name:    "wrap standard error",
			err:     errors.New("connection refused"),
			code:    "NETWORK_ERROR",
			message: "failed to connect",
			verify: func(t *testing.T, got *ResultError) {
				assert.Equal(t, "NETWORK_ERROR", got.Code)
				assert.Equal(t, "failed to connect", got.Message)
				require.NotNil(t, got.Cause)
				assert.Equal(t, "UNKNOWN", got.Cause.Code)
				assert.Equal(t, "connection refused", got.Cause.Message)
			},
		},
		{
			name:    "wrap ResultError",
			err:     NewResultError("TIMEOUT", "operation timed out"),
			code:    "TASK_FAILED",
			message: "task failed due to timeout",
			verify: func(t *testing.T, got *ResultError) {
				assert.Equal(t, "TASK_FAILED", got.Code)
				assert.Equal(t, "task failed due to timeout", got.Message)
				require.NotNil(t, got.Cause)
				assert.Equal(t, "TIMEOUT", got.Cause.Code)
				assert.Equal(t, "operation timed out", got.Cause.Message)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := Wrap(tt.err, tt.code, tt.message)
			tt.verify(t, got)
		})
	}
}

func TestFromError(t *testing.T) {
	tests := []struct {
		name   string
		err    error
		verify func(t *testing.T, got *ResultError)
	}{
		{
			name: "nil error",
			err:  nil,
			verify: func(t *testing.T, got *ResultError) {
				assert.Nil(t, got)
			},
		},
		{
			name: "standard error",
			err:  errors.New("something went wrong"),
			verify: func(t *testing.T, got *ResultError) {
				require.NotNil(t, got)
				assert.Equal(t, "UNKNOWN", got.Code)
				assert.Equal(t, "something went wrong", got.Message)
			},
		},
		{
			name: "ResultError",
			err:  NewResultError("TIMEOUT", "operation timed out"),
			verify: func(t *testing.T, got *ResultError) {
				require.NotNil(t, got)
				assert.Equal(t, "TIMEOUT", got.Code)
				assert.Equal(t, "operation timed out", got.Message)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := FromError(tt.err)
			tt.verify(t, got)
		})
	}
}

func TestResultError_Unwrap(t *testing.T) {
	tests := []struct {
		name string
		err  *ResultError
		want error
	}{
		{
			name: "no cause",
			err:  NewResultError("TIMEOUT", "operation timed out"),
			want: nil,
		},
		{
			name: "with cause",
			err: &ResultError{
				Code:    "TASK_FAILED",
				Message: "task failed",
				Cause: &ResultError{
					Code:    "TIMEOUT",
					Message: "operation timed out",
				},
			},
			want: &ResultError{
				Code:    "TIMEOUT",
				Message: "operation timed out",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.err.Unwrap()
			if tt.want == nil {
				assert.NoError(t, got)
			} else {
				assert.Equal(t, tt.want, got)
			}
		})
	}
}

func TestResultError_WithDetails(t *testing.T) {
	err := NewResultError("EXECUTION_FAILED", "probe failed")

	result := err.WithDetails(map[string]any{
		"target":      "https://example.com",
		"status_code": 500,
	})

	// Verify method chaining
	assert.Equal(t, err, result)

	// Verify details were added
	assert.Equal(t, "https://example.com", err.Details["target"])
	assert.Equal(t, 500, err.Details["status_code"])

	// Add more details
	result.WithDetails(map[string]any{
		"retry_count": 3,
	})

	assert.Equal(t, 3, err.Details["retry_count"])
	assert.Equal(t, "https://example.com", err.Details["target"]) // original still present
}

func TestResultError_WithRetryable(t *testing.T) {
	err := NewResultError("NETWORK_ERROR", "connection refused")

	result := err.WithRetryable(true)

	// Verify method chaining
	assert.Equal(t, err, result)

	// Verify retryable was set
	assert.True(t, err.Retryable)

	// Can be changed
	result.WithRetryable(false)
	assert.False(t, err.Retryable)
}

func TestResultError_WithComponent(t *testing.T) {
	err := NewResultError("TIMEOUT", "operation timed out")

	result := err.WithComponent("sql-injection-agent")

	// Verify method chaining
	assert.Equal(t, err, result)

	// Verify component was set
	assert.Equal(t, "sql-injection-agent", err.Component)
}

func TestResultError_WithStack(t *testing.T) {
	err := NewResultError("INTERNAL_ERROR", "unexpected panic")

	result := err.WithStack()

	// Verify method chaining
	assert.Equal(t, err, result)

	// Verify stack was captured
	assert.NotEmpty(t, err.Stack)

	// Stack should contain this test function
	assert.Contains(t, err.Stack, "TestResultError_WithStack")
}

func TestResultError_JSON(t *testing.T) {
	tests := []struct {
		name string
		err  *ResultError
	}{
		{
			name: "simple error",
			err:  NewResultError("TIMEOUT", "operation timed out"),
		},
		{
			name: "full error",
			err: NewResultError("EXECUTION_FAILED", "task failed").
				WithComponent("sql-injector").
				WithRetryable(true).
				WithDetails(map[string]any{
					"target": "https://example.com",
					"count":  42,
				}),
		},
		{
			name: "nested error",
			err: Wrap(
				NewResultError("TIMEOUT", "operation timed out").WithComponent("http-tool"),
				"TASK_FAILED",
				"task failed due to timeout",
			).WithComponent("test-agent"),
		},
		{
			name: "with stack",
			err:  NewResultError("INTERNAL_ERROR", "panic").WithStack(),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Marshal to JSON
			data, err := json.Marshal(tt.err)
			require.NoError(t, err)

			// Unmarshal back
			var got ResultError
			err = json.Unmarshal(data, &got)
			require.NoError(t, err)

			// Verify round-trip
			assert.Equal(t, tt.err.Code, got.Code)
			assert.Equal(t, tt.err.Message, got.Message)
			assert.Equal(t, tt.err.Retryable, got.Retryable)
			assert.Equal(t, tt.err.Component, got.Component)

			// For details, we need to compare carefully because JSON numbers become float64
			if tt.err.Details != nil {
				require.NotNil(t, got.Details)
				assert.Len(t, got.Details, len(tt.err.Details))
				for k := range tt.err.Details {
					assert.Contains(t, got.Details, k)
					// Don't do exact type comparison for numbers due to JSON marshaling
				}
			} else {
				assert.Nil(t, got.Details)
			}

			// Verify cause chain
			if tt.err.Cause != nil {
				require.NotNil(t, got.Cause)
				assert.Equal(t, tt.err.Cause.Code, got.Cause.Code)
				assert.Equal(t, tt.err.Cause.Message, got.Cause.Message)
			} else {
				assert.Nil(t, got.Cause)
			}

			// Stack might differ slightly but should be present or absent
			assert.Equal(t, tt.err.Stack != "", got.Stack != "")
		})
	}
}

func TestResultError_ErrorsIs(t *testing.T) {
	baseErr := NewResultError("TIMEOUT", "operation timed out")
	wrappedErr := Wrap(baseErr, "TASK_FAILED", "task failed")

	// errors.Is should work with wrapped ResultErrors
	require.ErrorIs(t, wrappedErr, baseErr)

	// Different error should not match
	otherErr := NewResultError("NETWORK_ERROR", "connection refused")
	assert.NotErrorIs(t, wrappedErr, otherErr)
}

func TestResultError_ErrorsAs(t *testing.T) {
	baseErr := NewResultError("TIMEOUT", "operation timed out")
	wrappedErr := Wrap(baseErr, "TASK_FAILED", "task failed")

	// errors.As should extract the ResultError
	var resultErr *ResultError
	require.ErrorAs(t, wrappedErr, &resultErr)
	assert.Equal(t, "TASK_FAILED", resultErr.Code)

	// Should be able to traverse the cause chain
	require.ErrorAs(t, resultErr.Cause, &resultErr)
	assert.Equal(t, "TIMEOUT", resultErr.Code)
}

func TestResultError_MethodChaining(t *testing.T) {
	// Verify that all With* methods support fluent chaining
	err := NewResultError("EXECUTION_FAILED", "task failed").
		WithComponent("test-agent").
		WithRetryable(true).
		WithDetails(map[string]any{
			"target": "https://example.com",
		}).
		WithStack()

	assert.Equal(t, "EXECUTION_FAILED", err.Code)
	assert.Equal(t, "task failed", err.Message)
	assert.Equal(t, "test-agent", err.Component)
	assert.True(t, err.Retryable)
	assert.Equal(t, "https://example.com", err.Details["target"])
	assert.NotEmpty(t, err.Stack)
}

func TestResultError_ComplexErrorChain(t *testing.T) {
	// Create a complex error chain
	rootErr := errors.New("connection refused")
	toolErr := Wrap(rootErr, "NETWORK_ERROR", "HTTP request failed").
		WithComponent("http-tool").
		WithRetryable(true)
	agentErr := Wrap(toolErr, "EXECUTION_FAILED", "probe execution failed").
		WithComponent("sql-injection-agent").
		WithDetails(map[string]any{
			"target":   "https://example.com",
			"endpoint": "/api/login",
		})

	// Verify error message contains full chain
	errMsg := agentErr.Error()
	assert.Contains(t, errMsg, "sql-injection-agent")
	assert.Contains(t, errMsg, "EXECUTION_FAILED")
	assert.Contains(t, errMsg, "probe execution failed")
	assert.Contains(t, errMsg, "http-tool")
	assert.Contains(t, errMsg, "NETWORK_ERROR")
	assert.Contains(t, errMsg, "connection refused")

	// Verify JSON serialization preserves chain
	data, err := json.Marshal(agentErr)
	require.NoError(t, err)

	var decoded ResultError
	err = json.Unmarshal(data, &decoded)
	require.NoError(t, err)

	assert.Equal(t, "EXECUTION_FAILED", decoded.Code)
	assert.Equal(t, "sql-injection-agent", decoded.Component)
	require.NotNil(t, decoded.Cause)
	assert.Equal(t, "NETWORK_ERROR", decoded.Cause.Code)
	assert.Equal(t, "http-tool", decoded.Cause.Component)
	require.NotNil(t, decoded.Cause.Cause)
	assert.Equal(t, "UNKNOWN", decoded.Cause.Cause.Code)
	assert.Contains(t, decoded.Cause.Cause.Message, "connection refused")
}
