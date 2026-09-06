// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Zero Root AI

package toolerr

import (
	"sync"
)

// RecoveryRegistry stores known failure modes and recovery strategies per tool.
// It provides thread-safe registration and lookup of recovery hints for specific
// tool error codes. The registry is used to enrich errors with actionable
// recovery suggestions that help orchestrators and LLMs make informed decisions.
//
// The registry uses a nested map structure:
//
//	tool -> errorCode -> []RecoveryHint
//
// This allows efficient O(1) lookups and supports multiple hints per error code,
// ordered by priority for sequential retry attempts.
type RecoveryRegistry struct {
	mu       sync.RWMutex
	registry map[string]map[string][]RecoveryHint
}

// globalRegistry is the package-level registry instance used by the
// Register, GetHints, and EnrichError functions. Tools and applications
// register their known failure modes at initialization time.
var globalRegistry = &RecoveryRegistry{
	registry: make(map[string]map[string][]RecoveryHint),
}

// Register adds recovery hints for a specific tool's error code.
// Multiple hints can be provided and will be stored in the order given.
// If hints are already registered for the same tool/errorCode combination,
// they will be replaced with the new hints.
//
// This function is thread-safe and can be called concurrently from multiple
// goroutines during initialization.
//
// Parameters:
//   - tool: the name of the tool (e.g., "mytool", "masscan", "mytool-b")
//   - errorCode: the error code constant (e.g., ErrCodeBinaryNotFound)
//   - hints: one or more recovery hints, typically ordered by priority
//
// Example:
//
//	Register("mytool", ErrCodeBinaryNotFound,
//	    RecoveryHint{
//	        Strategy:    StrategyUseAlternative,
//	        Alternative: "masscan",
//	        Reason:      "masscan can perform similar port scanning",
//	        Confidence:  0.8,
//	        Priority:    1,
//	    },
//	    RecoveryHint{
//	        Strategy:    StrategyUseAlternative,
//	        Alternative: "netcat",
//	        Reason:      "nc can probe individual ports",
//	        Confidence:  0.5,
//	        Priority:    2,
//	    },
//	)
func Register(tool, errorCode string, hints ...RecoveryHint) {
	globalRegistry.mu.Lock()
	defer globalRegistry.mu.Unlock()

	if globalRegistry.registry[tool] == nil {
		globalRegistry.registry[tool] = make(map[string][]RecoveryHint)
	}
	globalRegistry.registry[tool][errorCode] = hints
}

// GetHints retrieves recovery hints for a specific tool's error code.
// Returns nil if no hints are registered for the given tool/errorCode combination.
//
// This function is thread-safe and can be called concurrently with Register
// and EnrichError. It uses a read lock for efficient concurrent reads.
//
// Parameters:
//   - tool: the name of the tool
//   - errorCode: the error code to look up
//
// Returns:
//   - []RecoveryHint: slice of hints ordered by priority, or nil if not found
//
// Example:
//
//	hints := GetHints("mytool", ErrCodeBinaryNotFound)
//	if len(hints) > 0 {
//	    fmt.Printf("Found %d recovery options\n", len(hints))
//	    for _, hint := range hints {
//	        fmt.Printf("  - %s: %s\n", hint.Strategy, hint.Reason)
//	    }
//	}
func GetHints(tool, errorCode string) []RecoveryHint {
	globalRegistry.mu.RLock()
	defer globalRegistry.mu.RUnlock()

	if toolHints, ok := globalRegistry.registry[tool]; ok {
		if hints, ok := toolHints[errorCode]; ok {
			return hints
		}
	}
	return nil
}

// EnrichError looks up recovery hints for an error and attaches them.
// It also sets a default error class if one is not already set, based on
// the error code's semantic meaning.
//
// This function performs two enrichment steps:
//  1. Sets Error.Class using DefaultClassForCode if Class is empty
//  2. Looks up and appends registered recovery hints to Error.Hints
//
// If the error already has hints, the registered hints are appended
// rather than replacing existing ones. This allows errors to be enriched
// at multiple stages of processing.
//
// This function is thread-safe and can be called concurrently.
//
// Parameters:
//   - err: the error to enrich (can be nil)
//
// Returns:
//   - *Error: the same error instance with enriched fields, or nil if input is nil
//
// Example:
//
//	// Create error without classification
//	err := toolerr.New("mytool", "scan", toolerr.ErrCodeBinaryNotFound, "mytool not found")
//
//	// Enrich with registry data
//	enriched := toolerr.EnrichError(err)
//
//	// Now has class and hints
//	fmt.Println(enriched.Class) // "infrastructure"
//	fmt.Printf("Recovery options: %d\n", len(enriched.Hints))
func EnrichError(err *Error) *Error {
	if err == nil {
		return nil
	}

	// Set default class based on error code if not set
	if err.Class == "" {
		err.Class = DefaultClassForCode(err.Code)
	}

	// Look up registered hints and append them
	if hints := GetHints(err.Tool, err.Code); len(hints) > 0 {
		err.Hints = append(err.Hints, hints...)
	}

	return err
}
