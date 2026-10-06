// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Zero Root AI

package llm

// TokenTracker tracks token usage across different LLM slots.
type TokenTracker interface {
	// Add records token usage for a specific slot.
	Add(slot string, usage TokenUsage)

	// Total returns the aggregate token usage across all slots.
	Total() TokenUsage

	// BySlot returns the token usage for a specific slot.
	BySlot(slot string) TokenUsage

	// Reset clears all tracked token usage.
	Reset()

	// Slots returns a list of all tracked slot names.
	Slots() []string
}
