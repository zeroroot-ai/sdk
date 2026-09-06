// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Zero Root AI

package agent_test

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/zeroroot-ai/sdk/agent"
)

// Example_resultError demonstrates basic usage of ResultError for simple errors.
func Example_resultError() {
	// Create a simple error
	err := agent.NewResultError("TIMEOUT", "LLM request timed out after 30s")

	// Add context with details
	err.WithDetails(map[string]any{
		"slot":       "main",
		"timeout_ms": 30000,
		"model":      "gpt-4",
	})

	// Mark as retryable
	err.WithRetryable(true)

	// Set the component
	err.WithComponent("sql-injection-agent")

	fmt.Println(err.Error())
	fmt.Printf("Retryable: %v\n", err.Retryable)

	// Output:
	// sql-injection-agent [TIMEOUT]: LLM request timed out after 30s
	// Retryable: true
}

// Example_resultError_wrapping demonstrates error wrapping with ResultError.
func Example_resultError_wrapping() {
	// Simulate a tool execution error
	toolErr := errors.New("connection refused")

	// Wrap it with a ResultError
	err := agent.Wrap(toolErr, "TOOL_FAILED", "HTTP tool execution failed").
		WithComponent("recon-agent").
		WithDetails(map[string]any{
			"tool":   "http-request",
			"target": "https://example.com",
		})

	fmt.Println(err.Error())

	// Output:
	// recon-agent [TOOL_FAILED]: HTTP tool execution failed: [UNKNOWN]: connection refused
}

// Example_resultError_fromError demonstrates converting standard errors to ResultError.
func Example_resultError_fromError() {
	// Standard Go error
	stdErr := errors.New("file not found")

	// Convert to ResultError
	resultErr := agent.FromError(stdErr)

	fmt.Printf("Code: %s\n", resultErr.Code)
	fmt.Printf("Message: %s\n", resultErr.Message)

	// Nil error handling
	nilErr := agent.FromError(nil)
	fmt.Printf("Nil error: %v\n", nilErr)

	// Output:
	// Code: UNKNOWN
	// Message: file not found
	// Nil error: <nil>
}

// Example_resultError_json demonstrates JSON serialization of ResultError.
func Example_resultError_json() {
	// Create a complex error with nested causes
	err := agent.NewResultError("EXECUTION_FAILED", "Task execution failed").
		WithComponent("sql-injection-agent").
		WithRetryable(false).
		WithDetails(map[string]any{
			"endpoint": "/api/login",
			"attempts": 3,
		})

	// Add a cause
	cause := agent.NewResultError("TIMEOUT", "HTTP request timed out").
		WithComponent("http-tool")
	err.Cause = cause

	// Marshal to JSON
	data, _ := json.MarshalIndent(err, "", "  ")
	fmt.Println(string(data))

	// Unmarshal back
	var decoded agent.ResultError
	json.Unmarshal(data, &decoded)

	fmt.Printf("\nDecoded code: %s\n", decoded.Code)
	fmt.Printf("Decoded component: %s\n", decoded.Component)
	fmt.Printf("Has cause: %v\n", decoded.Cause != nil)

	// Output:
	// {
	//   "code": "EXECUTION_FAILED",
	//   "message": "Task execution failed",
	//   "details": {
	//     "attempts": 3,
	//     "endpoint": "/api/login"
	//   },
	//   "cause": {
	//     "code": "TIMEOUT",
	//     "message": "HTTP request timed out",
	//     "retryable": false,
	//     "component": "http-tool"
	//   },
	//   "retryable": false,
	//   "component": "sql-injection-agent"
	// }
	//
	// Decoded code: EXECUTION_FAILED
	// Decoded component: sql-injection-agent
	// Has cause: true
}

// Example_resultError_inResult demonstrates using ResultError in agent results.
func Example_resultError_inResult() {
	// Simulate an agent execution function
	execute := func(ctx context.Context, h agent.Harness, task agent.Task) (agent.Result, error) {
		// Simulate a failure
		err := agent.NewResultError("LLM_FAILED", "Failed to complete LLM request").
			WithComponent("my-agent").
			WithRetryable(true).
			WithDetails(map[string]any{
				"slot":  "main",
				"turns": 5,
			})

		// Return failed result with the structured error
		return agent.Result{
			Status: agent.StatusFailed,
			Error:  err,
		}, err
	}

	// Execute
	result, err := execute(context.Background(), nil, agent.Task{})

	if err != nil {
		// Check if it's a ResultError
		var resultErr *agent.ResultError
		if errors.As(err, &resultErr) {
			fmt.Printf("Error code: %s\n", resultErr.Code)
			fmt.Printf("Component: %s\n", resultErr.Component)
			fmt.Printf("Retryable: %v\n", resultErr.Retryable)
			fmt.Printf("Status: %s\n", result.Status)
		}
	}

	// Output:
	// Error code: LLM_FAILED
	// Component: my-agent
	// Retryable: true
	// Status: failed
}

// Example_resultError_errorChain demonstrates working with error chains.
func Example_resultError_errorChain() {
	// Build an error chain
	root := errors.New("connection refused")
	toolErr := agent.Wrap(root, "NETWORK_ERROR", "Failed to connect to target").
		WithComponent("http-tool").
		WithRetryable(true)
	agentErr := agent.Wrap(toolErr, "EXECUTION_FAILED", "Probe execution failed").
		WithComponent("sql-injection-agent")

	// Check error chain with errors.Is
	baseTimeout := agent.NewResultError("TIMEOUT", "timeout")
	fmt.Printf("Is timeout: %v\n", errors.Is(agentErr, baseTimeout))

	// Check if error is in chain with errors.As
	var resultErr *agent.ResultError
	if errors.As(agentErr, &resultErr) {
		fmt.Printf("Top-level code: %s\n", resultErr.Code)

		// Traverse the cause chain
		if resultErr.Cause != nil {
			fmt.Printf("Cause code: %s\n", resultErr.Cause.Code)
			if resultErr.Cause.Cause != nil {
				fmt.Printf("Root message: %s\n", resultErr.Cause.Cause.Message)
			}
		}
	}

	// Output:
	// Is timeout: false
	// Top-level code: EXECUTION_FAILED
	// Cause code: NETWORK_ERROR
	// Root message: connection refused
}

// Example_resultError_withStack demonstrates capturing stack traces.
func Example_resultError_withStack() {
	// Create an error with stack trace (useful for debugging)
	err := agent.NewResultError("INTERNAL_ERROR", "Unexpected panic in agent").
		WithComponent("test-agent").
		WithStack()

	fmt.Printf("Code: %s\n", err.Code)
	fmt.Printf("Has stack: %v\n", err.Stack != "")
	// Note: actual stack trace not printed as it's environment-specific

	// Output:
	// Code: INTERNAL_ERROR
	// Has stack: true
}

// Example_resultError_methodChaining demonstrates fluent API usage.
func Example_resultError_methodChaining() {
	// All With* methods support method chaining
	err := agent.NewResultError("EXECUTION_FAILED", "Task failed").
		WithComponent("sql-injection-agent").
		WithRetryable(true).
		WithDetails(map[string]any{
			"target":   "https://example.com",
			"endpoint": "/api/login",
			"method":   "POST",
		}).
		WithStack()

	fmt.Printf("Component: %s\n", err.Component)
	fmt.Printf("Retryable: %v\n", err.Retryable)
	fmt.Printf("Details count: %d\n", len(err.Details))

	// Output:
	// Component: sql-injection-agent
	// Retryable: true
	// Details count: 3
}
