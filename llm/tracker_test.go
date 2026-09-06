// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Zero Root AI

package llm

import (
	"reflect"
	"sync"
	"testing"
)

func TestNewTokenTracker(t *testing.T) {
	tracker := NewTokenTracker()

	if tracker == nil {
		t.Fatal("NewTokenTracker() returned nil")
	}
	if tracker.slots == nil {
		t.Error("slots map not initialized")
	}

	total := tracker.Total()
	expected := TokenUsage{}
	if total != expected {
		t.Errorf("Initial total = %v, want %v", total, expected)
	}
}

func TestDefaultTokenTracker_Add(t *testing.T) {
	tracker := NewTokenTracker()

	usage1 := TokenUsage{
		InputTokens:  100,
		OutputTokens: 50,
		TotalTokens:  150,
	}

	tracker.Add("primary", usage1)

	total := tracker.Total()
	if total != usage1 {
		t.Errorf("Total() = %v, want %v", total, usage1)
	}

	slotUsage := tracker.BySlot("primary")
	if slotUsage != usage1 {
		t.Errorf("BySlot() = %v, want %v", slotUsage, usage1)
	}
}

func TestDefaultTokenTracker_AddMultipleSlots(t *testing.T) {
	tracker := NewTokenTracker()

	usage1 := TokenUsage{InputTokens: 100, OutputTokens: 50, TotalTokens: 150}
	usage2 := TokenUsage{InputTokens: 200, OutputTokens: 100, TotalTokens: 300}

	tracker.Add("primary", usage1)
	tracker.Add("vision", usage2)

	total := tracker.Total()
	expected := TokenUsage{
		InputTokens:  300,
		OutputTokens: 150,
		TotalTokens:  450,
	}

	if total != expected {
		t.Errorf("Total() = %v, want %v", total, expected)
	}

	if tracker.BySlot("primary") != usage1 {
		t.Error("primary slot usage incorrect")
	}
	if tracker.BySlot("vision") != usage2 {
		t.Error("vision slot usage incorrect")
	}
}

func TestDefaultTokenTracker_AddToSameSlot(t *testing.T) {
	tracker := NewTokenTracker()

	usage1 := TokenUsage{InputTokens: 100, OutputTokens: 50, TotalTokens: 150}
	usage2 := TokenUsage{InputTokens: 50, OutputTokens: 25, TotalTokens: 75}

	tracker.Add("primary", usage1)
	tracker.Add("primary", usage2)

	expected := TokenUsage{
		InputTokens:  150,
		OutputTokens: 75,
		TotalTokens:  225,
	}

	slotUsage := tracker.BySlot("primary")
	if slotUsage != expected {
		t.Errorf("BySlot() = %v, want %v", slotUsage, expected)
	}

	total := tracker.Total()
	if total != expected {
		t.Errorf("Total() = %v, want %v", total, expected)
	}
}

func TestDefaultTokenTracker_BySlot_NonExistent(t *testing.T) {
	tracker := NewTokenTracker()

	usage := tracker.BySlot("nonexistent")
	expected := TokenUsage{}

	if usage != expected {
		t.Errorf("BySlot(nonexistent) = %v, want %v", usage, expected)
	}
}

func TestDefaultTokenTracker_Reset(t *testing.T) {
	tracker := NewTokenTracker()

	usage := TokenUsage{InputTokens: 100, OutputTokens: 50, TotalTokens: 150}
	tracker.Add("primary", usage)
	tracker.Add("vision", usage)

	tracker.Reset()

	total := tracker.Total()
	if total != (TokenUsage{}) {
		t.Errorf("Total after Reset() = %v, want zero value", total)
	}

	slots := tracker.Slots()
	if len(slots) != 0 {
		t.Errorf("Slots after Reset() = %v, want empty", slots)
	}
}

func TestDefaultTokenTracker_Slots(t *testing.T) {
	tracker := NewTokenTracker()

	usage := TokenUsage{InputTokens: 100, OutputTokens: 50, TotalTokens: 150}
	tracker.Add("primary", usage)
	tracker.Add("vision", usage)
	tracker.Add("code", usage)

	slots := tracker.Slots()

	if len(slots) != 3 {
		t.Errorf("Slots() length = %d, want 3", len(slots))
	}

	// Verify all slots are present
	slotMap := make(map[string]bool)
	for _, slot := range slots {
		slotMap[slot] = true
	}

	expectedSlots := []string{"primary", "vision", "code"}
	for _, expected := range expectedSlots {
		if !slotMap[expected] {
			t.Errorf("Expected slot %q not found in result", expected)
		}
	}
}

func TestDefaultTokenTracker_HasSlot(t *testing.T) {
	tracker := NewTokenTracker()

	usage := TokenUsage{InputTokens: 100, OutputTokens: 50, TotalTokens: 150}
	tracker.Add("primary", usage)

	if !tracker.HasSlot("primary") {
		t.Error("HasSlot(primary) = false, want true")
	}

	if tracker.HasSlot("nonexistent") {
		t.Error("HasSlot(nonexistent) = true, want false")
	}
}

func TestDefaultTokenTracker_Clone(t *testing.T) {
	tracker := NewTokenTracker()

	usage1 := TokenUsage{InputTokens: 100, OutputTokens: 50, TotalTokens: 150}
	usage2 := TokenUsage{InputTokens: 200, OutputTokens: 100, TotalTokens: 300}

	tracker.Add("primary", usage1)
	tracker.Add("vision", usage2)

	clone := tracker.Clone()

	// Verify clone has same data
	if clone.Total() != tracker.Total() {
		t.Error("Clone has different total")
	}
	if clone.BySlot("primary") != tracker.BySlot("primary") {
		t.Error("Clone has different primary slot usage")
	}
	if clone.BySlot("vision") != tracker.BySlot("vision") {
		t.Error("Clone has different vision slot usage")
	}

	// Verify clone is independent
	newUsage := TokenUsage{InputTokens: 50, OutputTokens: 25, TotalTokens: 75}
	clone.Add("primary", newUsage)

	if clone.BySlot("primary") == tracker.BySlot("primary") {
		t.Error("Clone is not independent from original")
	}
}

func TestDefaultTokenTracker_Snapshot(t *testing.T) {
	tracker := NewTokenTracker()

	usage1 := TokenUsage{InputTokens: 100, OutputTokens: 50, TotalTokens: 150}
	usage2 := TokenUsage{InputTokens: 200, OutputTokens: 100, TotalTokens: 300}

	tracker.Add("primary", usage1)
	tracker.Add("vision", usage2)

	snapshot := tracker.Snapshot()

	// Verify snapshot has correct data
	expectedTotal := TokenUsage{InputTokens: 300, OutputTokens: 150, TotalTokens: 450}
	if snapshot.Total != expectedTotal {
		t.Errorf("Snapshot.Total = %v, want %v", snapshot.Total, expectedTotal)
	}

	if snapshot.Slots["primary"] != usage1 {
		t.Error("Snapshot has incorrect primary slot usage")
	}
	if snapshot.Slots["vision"] != usage2 {
		t.Error("Snapshot has incorrect vision slot usage")
	}

	// Verify snapshot is independent
	tracker.Add("primary", usage1)
	if snapshot.Slots["primary"] == tracker.BySlot("primary") {
		t.Error("Snapshot is not independent from tracker")
	}
}

func TestDefaultTokenTracker_Concurrency(t *testing.T) {
	tracker := NewTokenTracker()
	var wg sync.WaitGroup

	// Run multiple goroutines adding to different slots
	numGoroutines := 100
	numAddsPerGoroutine := 100

	for i := range numGoroutines {
		wg.Add(1)
		go func(slotID int) {
			defer wg.Done()
			slot := "slot" + string(rune('0'+slotID%10))
			usage := TokenUsage{InputTokens: 1, OutputTokens: 1, TotalTokens: 2}
			for range numAddsPerGoroutine {
				tracker.Add(slot, usage)
			}
		}(i)
	}

	wg.Wait()

	// Verify total is correct
	total := tracker.Total()
	expectedTotal := numGoroutines * numAddsPerGoroutine * 2
	if total.TotalTokens != expectedTotal {
		t.Errorf("Total tokens = %d, want %d", total.TotalTokens, expectedTotal)
	}
}

func TestDefaultTokenTracker_ConcurrentReadWrite(t *testing.T) {
	tracker := NewTokenTracker()
	var wg sync.WaitGroup

	// Writer goroutines
	for range 50 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			usage := TokenUsage{InputTokens: 10, OutputTokens: 5, TotalTokens: 15}
			tracker.Add("primary", usage)
		}()
	}

	// Reader goroutines
	for range 50 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_ = tracker.Total()
			_ = tracker.BySlot("primary")
			_ = tracker.Slots()
		}()
	}

	wg.Wait()
	// If we get here without a race condition, the test passes
}

func TestDefaultTokenTracker_EmptySlots(t *testing.T) {
	tracker := NewTokenTracker()

	slots := tracker.Slots()
	if len(slots) != 0 {
		t.Errorf("Empty tracker Slots() = %v, want empty slice", slots)
	}
}

func TestSnapshot_Immutability(t *testing.T) {
	tracker := NewTokenTracker()
	usage := TokenUsage{InputTokens: 100, OutputTokens: 50, TotalTokens: 150}
	tracker.Add("primary", usage)

	snapshot := tracker.Snapshot()

	// Modify the snapshot's map
	snapshot.Slots["primary"] = TokenUsage{InputTokens: 999}

	// Verify tracker is not affected
	if tracker.BySlot("primary").InputTokens == 999 {
		t.Error("Modifying snapshot affected the tracker")
	}
}

func TestTokenTracker_Interface(t *testing.T) {
	// Verify DefaultTokenTracker implements TokenTracker interface
	var _ TokenTracker = (*DefaultTokenTracker)(nil)
}

func TestDefaultTokenTracker_ZeroUsage(t *testing.T) {
	tracker := NewTokenTracker()

	// Adding zero usage should work but not change totals
	tracker.Add("primary", TokenUsage{})

	total := tracker.Total()
	if !reflect.DeepEqual(total, TokenUsage{}) {
		t.Errorf("Total after adding zero usage = %v, want zero", total)
	}

	// But the slot should still exist
	if !tracker.HasSlot("primary") {
		t.Error("Slot should exist even with zero usage")
	}
}
