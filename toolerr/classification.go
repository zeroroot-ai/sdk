// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Zero Root AI

package toolerr

// RecoveryStrategy defines the type of recovery action that can be attempted
// to resolve or work around an error.
type RecoveryStrategy string

const (
	// StrategyRetry indicates the operation should be retried as-is
	StrategyRetry RecoveryStrategy = "retry"

	// StrategyRetryWithBackoff indicates retry with exponential backoff
	StrategyRetryWithBackoff RecoveryStrategy = "retry_with_backoff"

	// StrategyModifyParams indicates changing parameters may help
	StrategyModifyParams RecoveryStrategy = "modify_params"

	// StrategyUseAlternative indicates using a different tool may work
	StrategyUseAlternative RecoveryStrategy = "use_alternative_tool"

	// StrategySkip indicates the operation can be safely skipped
	StrategySkip RecoveryStrategy = "skip"
)

// RecoveryHint provides a concrete suggestion for recovering from an error.
// Multiple hints can be attached to an error, ordered by priority.
type RecoveryHint struct {
	// Strategy indicates the type of recovery action
	Strategy RecoveryStrategy `json:"strategy"`

	// Alternative specifies the tool or agent name when using StrategyUseAlternative or StrategySpawnAgent
	Alternative string `json:"alternative,omitempty"`

	// Params contains suggested parameter modifications when using StrategyModifyParams
	Params map[string]any `json:"params,omitempty"`

	// Reason explains why this recovery approach might succeed
	Reason string `json:"reason"`

	// Confidence indicates the likelihood of success (0.0 to 1.0)
	Confidence float64 `json:"confidence"`

	// Priority determines the order to try hints (lower = try first)
	Priority int `json:"priority"`
}
