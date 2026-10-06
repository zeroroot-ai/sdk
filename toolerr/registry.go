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
