// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Zero Root AI

package toolerr_test

import (
	"encoding/json"
	"fmt"

	"github.com/zeroroot-ai/sdk/toolerr"
)

// ExampleErrorClass demonstrates error classification for semantic understanding
func ExampleErrorClass() {
	// Infrastructure error - missing binary
	err := toolerr.New("mytool", "scan", toolerr.ErrCodeBinaryNotFound, "mytool binary not found in PATH").
		WithClass(toolerr.ErrorClassInfrastructure)

	fmt.Printf("Class: %s\n", err.Class)
	// Output: Class: infrastructure
}

// ExampleDefaultClassForCode demonstrates automatic error classification
func ExampleDefaultClassForCode() {
	// Get default classification for different error codes
	fmt.Printf("BINARY_NOT_FOUND: %s\n", toolerr.DefaultClassForCode(toolerr.ErrCodeBinaryNotFound))
	fmt.Printf("TIMEOUT: %s\n", toolerr.DefaultClassForCode(toolerr.ErrCodeTimeout))
	fmt.Printf("INVALID_INPUT: %s\n", toolerr.DefaultClassForCode(toolerr.ErrCodeInvalidInput))
	// Output:
	// BINARY_NOT_FOUND: infrastructure
	// TIMEOUT: transient
	// INVALID_INPUT: semantic
}

// ExampleRecoveryHint demonstrates recovery suggestions
func ExampleRecoveryHint() {
	hint := toolerr.RecoveryHint{
		Strategy:    toolerr.StrategyUseAlternative,
		Alternative: "masscan",
		Reason:      "masscan can perform similar port scanning",
		Confidence:  0.8,
		Priority:    1,
	}

	err := toolerr.New("mytool", "scan", toolerr.ErrCodeBinaryNotFound, "mytool not found").
		WithClass(toolerr.ErrorClassInfrastructure).
		WithHints(hint)

	fmt.Printf("Error has %d recovery hint(s)\n", len(err.Hints))
	fmt.Printf("Suggestion: Try %s (%s)\n", err.Hints[0].Alternative, err.Hints[0].Reason)
	// Output:
	// Error has 1 recovery hint(s)
	// Suggestion: Try masscan (masscan can perform similar port scanning)
}

// ExampleError_WithClass demonstrates fluent API for error classification
func ExampleError_WithClass() {
	err := toolerr.New("kubectl", "apply", toolerr.ErrCodePermissionDenied, "insufficient permissions").
		WithClass(toolerr.ErrorClassInfrastructure).
		WithDetails(map[string]any{
			"namespace": "default",
			"resource":  "deployment",
		})

	fmt.Println(err)
	// Output: kubectl [apply/PERMISSION_DENIED]: insufficient permissions
}

// ExampleError_WithHints demonstrates adding multiple recovery hints
func ExampleError_WithHints() {
	err := toolerr.New("mytool", "scan", toolerr.ErrCodeTimeout, "scan timed out").
		WithClass(toolerr.ErrorClassTransient).
		WithHints(
			toolerr.RecoveryHint{
				Strategy:   toolerr.StrategyModifyParams,
				Params:     map[string]any{"timing": 2},
				Reason:     "slower timing (T2) may succeed on congested networks",
				Confidence: 0.7,
				Priority:   1,
			},
			toolerr.RecoveryHint{
				Strategy:   toolerr.StrategyRetryWithBackoff,
				Reason:     "network congestion may be temporary",
				Confidence: 0.6,
				Priority:   2,
			},
		)

	fmt.Printf("Error: %s\n", err)
	fmt.Printf("Recovery options: %d\n", len(err.Hints))
	// Output:
	// Error: mytool [scan/TIMEOUT]: scan timed out
	// Recovery options: 2
}

// ExampleError_WithHints_chaining demonstrates incremental hint addition
func ExampleError_WithHints_chaining() {
	err := toolerr.New("masscan", "scan", toolerr.ErrCodeBinaryNotFound, "masscan not found")

	// Add first hint
	err.WithHints(toolerr.RecoveryHint{
		Strategy:    toolerr.StrategyUseAlternative,
		Alternative: "mytool",
		Reason:      "mytool provides similar scanning capabilities",
		Confidence:  0.9,
		Priority:    1,
	})

	// Add second hint (appends to existing hints)
	err.WithHints(toolerr.RecoveryHint{
		Strategy:    toolerr.StrategySpawnAgent,
		Alternative: "network_scanner",
		Reason:      "specialized agent can handle scanning",
		Confidence:  0.7,
		Priority:    2,
	})

	fmt.Printf("Total hints: %d\n", len(err.Hints))
	// Output: Total hints: 2
}

// ExampleRecoveryStrategy demonstrates all recovery strategies
func ExampleRecoveryStrategy() {
	strategies := []toolerr.RecoveryStrategy{
		toolerr.StrategyRetry,
		toolerr.StrategyRetryWithBackoff,
		toolerr.StrategyModifyParams,
		toolerr.StrategyUseAlternative,
		toolerr.StrategySpawnAgent,
		toolerr.StrategySkip,
	}

	fmt.Println("Available recovery strategies:")
	for _, s := range strategies {
		fmt.Printf("  - %s\n", s)
	}
	// Output:
	// Available recovery strategies:
	//   - retry
	//   - retry_with_backoff
	//   - modify_params
	//   - use_alternative_tool
	//   - spawn_agent
	//   - skip
}

// Example_fullErrorWithRecovery demonstrates a complete error with classification and hints
func Example_fullErrorWithRecovery() {
	err := toolerr.New("mytool", "scan", toolerr.ErrCodeBinaryNotFound, "mytool binary not found in PATH").
		WithClass(toolerr.ErrorClassInfrastructure).
		WithDetails(map[string]any{
			"target": "192.168.1.0/24",
			"ports":  "1-1000",
		}).
		WithHints(
			toolerr.RecoveryHint{
				Strategy:    toolerr.StrategyUseAlternative,
				Alternative: "masscan",
				Reason:      "masscan can perform similar port scanning",
				Confidence:  0.8,
				Priority:    1,
			},
			toolerr.RecoveryHint{
				Strategy:    toolerr.StrategySpawnAgent,
				Alternative: "port_scanner",
				Reason:      "specialized port scanning agent available",
				Confidence:  0.7,
				Priority:    2,
			},
		)

	fmt.Printf("Error: %s\n", err)
	fmt.Printf("Class: %s\n", err.Class)
	fmt.Printf("Recovery hints: %d\n", len(err.Hints))
	fmt.Printf("Primary suggestion: Use %s\n", err.Hints[0].Alternative)
	// Output:
	// Error: mytool [scan/BINARY_NOT_FOUND]: mytool binary not found in PATH
	// Class: infrastructure
	// Recovery hints: 2
	// Primary suggestion: Use masscan
}

// Example_jsonSerialization demonstrates JSON serialization of errors with classification
func Example_jsonSerialization() {
	err := toolerr.New("kubectl", "apply", toolerr.ErrCodeTimeout, "operation timed out").
		WithClass(toolerr.ErrorClassTransient).
		WithHints(toolerr.RecoveryHint{
			Strategy:   toolerr.StrategyRetryWithBackoff,
			Reason:     "API server may be temporarily overloaded",
			Confidence: 0.8,
			Priority:   1,
		})

	// Serialize to JSON
	data, _ := json.MarshalIndent(err, "", "  ")
	fmt.Println(string(data))
	// Output:
	// {
	//   "Tool": "kubectl",
	//   "Operation": "apply",
	//   "Code": "TIMEOUT",
	//   "Message": "operation timed out",
	//   "Details": null,
	//   "Cause": null,
	//   "class": "transient",
	//   "hints": [
	//     {
	//       "strategy": "retry_with_backoff",
	//       "reason": "API server may be temporarily overloaded",
	//       "confidence": 0.8,
	//       "priority": 1
	//     }
	//   ]
	// }
}

// Example_semanticErrorClassification demonstrates semantic error handling
func Example_semanticErrorClassification() {
	// Semantic error - invalid input that needs correction
	err := toolerr.New("mytool", "scan", toolerr.ErrCodeInvalidInput, "invalid port range").
		WithClass(toolerr.ErrorClassSemantic).
		WithDetails(map[string]any{
			"input": "ports: 0-99999",
		}).
		WithHints(toolerr.RecoveryHint{
			Strategy:   toolerr.StrategyModifyParams,
			Params:     map[string]any{"ports": "1-65535"},
			Reason:     "port numbers must be between 1 and 65535",
			Confidence: 0.95,
			Priority:   1,
		})

	fmt.Printf("Error type: %s\n", err.Class)
	fmt.Printf("Suggested fix: %v\n", err.Hints[0].Params["ports"])
	// Output:
	// Error type: semantic
	// Suggested fix: 1-65535
}

// Example_permanentErrorClassification demonstrates permanent error handling
func Example_permanentErrorClassification() {
	// Permanent error - cannot be retried
	err := toolerr.New("mytool-b", "scan", "TARGET_NOT_FOUND", "target does not exist").
		WithClass(toolerr.ErrorClassPermanent).
		WithDetails(map[string]any{
			"target": "nonexistent.example.com",
		}).
		WithHints(toolerr.RecoveryHint{
			Strategy:   toolerr.StrategySkip,
			Reason:     "target does not exist and cannot be scanned",
			Confidence: 1.0,
			Priority:   1,
		})

	fmt.Printf("Error class: %s\n", err.Class)
	fmt.Printf("Recommendation: %s\n", err.Hints[0].Strategy)
	// Output:
	// Error class: permanent
	// Recommendation: skip
}
