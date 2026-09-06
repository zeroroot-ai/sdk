// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Zero Root AI

package serve

import (
	"sync"

	"github.com/zeroroot-ai/sdk/llm"
)

// CallbackTokenTracker implements llm.TokenTracker with thread-safe tracking
// of token usage across different LLM slots during callback-based execution.
type CallbackTokenTracker struct {
	mu    sync.RWMutex
	slots map[string]llm.TokenUsage
	total llm.TokenUsage
}

// NewCallbackTokenTracker creates a new thread-safe token tracker.
func NewCallbackTokenTracker() *CallbackTokenTracker {
	return &CallbackTokenTracker{
		slots: make(map[string]llm.TokenUsage),
	}
}

// Add records token usage for a specific slot.
// This method is thread-safe and can be called concurrently.
func (t *CallbackTokenTracker) Add(slot string, usage llm.TokenUsage) {
	t.mu.Lock()
	defer t.mu.Unlock()

	// Update slot-specific usage
	current := t.slots[slot]
	t.slots[slot] = current.Add(usage)

	// Update total usage
	t.total = t.total.Add(usage)
}

// Total returns the aggregate token usage across all slots.
// This method is thread-safe.
func (t *CallbackTokenTracker) Total() llm.TokenUsage {
	t.mu.RLock()
	defer t.mu.RUnlock()
	return t.total
}

// BySlot returns the token usage for a specific slot.
// Returns an empty TokenUsage if the slot has not been used.
// This method is thread-safe.
func (t *CallbackTokenTracker) BySlot(slot string) llm.TokenUsage {
	t.mu.RLock()
	defer t.mu.RUnlock()
	return t.slots[slot]
}

// Reset clears all tracked token usage.
// This method is thread-safe.
func (t *CallbackTokenTracker) Reset() {
	t.mu.Lock()
	defer t.mu.Unlock()

	t.slots = make(map[string]llm.TokenUsage)
	t.total = llm.TokenUsage{}
}

// Slots returns a list of all tracked slot names.
// This method is thread-safe.
func (t *CallbackTokenTracker) Slots() []string {
	t.mu.RLock()
	defer t.mu.RUnlock()

	slots := make([]string, 0, len(t.slots))
	for slot := range t.slots {
		slots = append(slots, slot)
	}
	return slots
}

// HasSlot returns true if the tracker has recorded usage for the given slot.
// This method is thread-safe.
func (t *CallbackTokenTracker) HasSlot(slot string) bool {
	t.mu.RLock()
	defer t.mu.RUnlock()
	_, exists := t.slots[slot]
	return exists
}

// Clone creates a deep copy of the tracker.
// This method is thread-safe.
func (t *CallbackTokenTracker) Clone() *CallbackTokenTracker {
	t.mu.RLock()
	defer t.mu.RUnlock()

	clone := &CallbackTokenTracker{
		slots: make(map[string]llm.TokenUsage, len(t.slots)),
		total: t.total,
	}

	for slot, usage := range t.slots {
		clone.slots[slot] = usage
	}

	return clone
}
