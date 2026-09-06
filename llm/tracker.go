// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Zero Root AI

package llm

import "sync"

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

// DefaultTokenTracker is a thread-safe implementation of TokenTracker.
type DefaultTokenTracker struct {
	mu    sync.RWMutex
	slots map[string]TokenUsage
	total TokenUsage
}

// NewTokenTracker creates a new DefaultTokenTracker.
func NewTokenTracker() *DefaultTokenTracker {
	return &DefaultTokenTracker{
		slots: make(map[string]TokenUsage),
	}
}

// Add records token usage for a specific slot.
func (t *DefaultTokenTracker) Add(slot string, usage TokenUsage) {
	t.mu.Lock()
	defer t.mu.Unlock()

	// Update slot-specific usage
	current := t.slots[slot]
	t.slots[slot] = current.Add(usage)

	// Update total usage
	t.total = t.total.Add(usage)
}

// Total returns the aggregate token usage across all slots.
func (t *DefaultTokenTracker) Total() TokenUsage {
	t.mu.RLock()
	defer t.mu.RUnlock()
	return t.total
}

// BySlot returns the token usage for a specific slot.
// Returns an empty TokenUsage if the slot has not been used.
func (t *DefaultTokenTracker) BySlot(slot string) TokenUsage {
	t.mu.RLock()
	defer t.mu.RUnlock()
	return t.slots[slot]
}

// Reset clears all tracked token usage.
func (t *DefaultTokenTracker) Reset() {
	t.mu.Lock()
	defer t.mu.Unlock()

	t.slots = make(map[string]TokenUsage)
	t.total = TokenUsage{}
}

// Slots returns a list of all tracked slot names.
func (t *DefaultTokenTracker) Slots() []string {
	t.mu.RLock()
	defer t.mu.RUnlock()

	slots := make([]string, 0, len(t.slots))
	for slot := range t.slots {
		slots = append(slots, slot)
	}
	return slots
}

// HasSlot returns true if the tracker has recorded usage for the given slot.
func (t *DefaultTokenTracker) HasSlot(slot string) bool {
	t.mu.RLock()
	defer t.mu.RUnlock()
	_, exists := t.slots[slot]
	return exists
}

// Clone creates a deep copy of the tracker.
func (t *DefaultTokenTracker) Clone() *DefaultTokenTracker {
	t.mu.RLock()
	defer t.mu.RUnlock()

	clone := &DefaultTokenTracker{
		slots: make(map[string]TokenUsage, len(t.slots)),
		total: t.total,
	}

	for slot, usage := range t.slots {
		clone.slots[slot] = usage
	}

	return clone
}

// Snapshot returns a read-only copy of the current token usage state.
type Snapshot struct {
	// Slots contains token usage by slot name.
	Slots map[string]TokenUsage

	// Total contains aggregate token usage.
	Total TokenUsage
}

// Snapshot returns a snapshot of the current token usage state.
func (t *DefaultTokenTracker) Snapshot() Snapshot {
	t.mu.RLock()
	defer t.mu.RUnlock()

	slots := make(map[string]TokenUsage, len(t.slots))
	for slot, usage := range t.slots {
		slots[slot] = usage
	}

	return Snapshot{
		Slots: slots,
		Total: t.total,
	}
}
