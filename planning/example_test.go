// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Zero Root AI

package planning_test

import (
	"fmt"

	"github.com/zeroroot-ai/sdk/planning"
)

func ExampleStepHints() {
	// Create hints with the fluent builder pattern
	hints := planning.NewStepHints().
		WithConfidence(0.85).
		WithKeyFinding("Admin panel discovered at /admin").
		WithKeyFinding("Default credentials may be in use").
		WithSuggestion("auth_bypass_agent").
		WithSuggestion("credential_stuffing_agent")

	// Access the hints
	fmt.Printf("Confidence: %.2f\n", hints.Confidence())
	fmt.Printf("Key findings: %d\n", len(hints.KeyFindings()))
	fmt.Printf("Suggestions: %d\n", len(hints.SuggestedNext()))
	fmt.Printf("Needs replan: %v\n", hints.HasReplanRecommendation())

	// Output:
	// Confidence: 0.85
	// Key findings: 2
	// Suggestions: 2
	// Needs replan: false
}

func ExampleStepHints_RecommendReplan() {
	// Create hints that recommend replanning
	hints := planning.NewStepHints().
		WithConfidence(0.3).
		RecommendReplan("Target uses custom auth - standard attacks ineffective")

	fmt.Printf("Needs replan: %v\n", hints.HasReplanRecommendation())
	fmt.Printf("Reason: %s\n", hints.ReplanReason())

	// Output:
	// Needs replan: true
	// Reason: Target uses custom auth - standard attacks ineffective
}

func ExampleStepHints_WithConfidence() {
	// Confidence values are clamped to [0.0, 1.0]
	hints1 := planning.NewStepHints().WithConfidence(1.5)
	hints2 := planning.NewStepHints().WithConfidence(-0.5)
	hints3 := planning.NewStepHints().WithConfidence(0.75)

	fmt.Printf("1.5 clamped to: %.1f\n", hints1.Confidence())
	fmt.Printf("-0.5 clamped to: %.1f\n", hints2.Confidence())
	fmt.Printf("0.75 stays: %.2f\n", hints3.Confidence())

	// Output:
	// 1.5 clamped to: 1.0
	// -0.5 clamped to: 0.0
	// 0.75 stays: 0.75
}
