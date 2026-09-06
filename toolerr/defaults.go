// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Zero Root AI

package toolerr

// This file registers generic recovery hints that apply to all tools.
// Tool-specific defaults (e.g., nmap → masscan alternative, httpx → curl
// fallback) are NOT shipped by the SDK — per spec
// decouple-sdk-from-tool-protos, the SDK has no tool-specific knowledge.
// Consumers (e.g., the gibson daemon) register their own tool-specific
// hints at startup if they want them; see core/gibson/internal/harness/
// toolerr_defaults.go for the in-tree consumer's set.
//
// The init() function below registers only the wildcard ("*") hints.

func init() {
	registerGenericHints()
}

// registerGenericHints registers recovery hints that apply to all tools
// (registered under the wildcard tool name "*").
func registerGenericHints() {
	// Generic timeout handling
	Register("*", ErrCodeTimeout,
		RecoveryHint{
			Strategy:   StrategyRetry,
			Reason:     "timeouts may be transient; a single retry often succeeds",
			Confidence: 0.6,
			Priority:   1,
		},
	)

	// Generic network error handling
	Register("*", ErrCodeNetworkError,
		RecoveryHint{
			Strategy:   StrategyRetryWithBackoff,
			Reason:     "network issues are often temporary and resolve within seconds",
			Confidence: 0.7,
			Priority:   1,
		},
	)

	// Generic execution failure
	Register("*", ErrCodeExecutionFailed,
		RecoveryHint{
			Strategy:   StrategyRetry,
			Reason:     "execution failures may be transient resource issues",
			Confidence: 0.5,
			Priority:   1,
		},
	)
}
