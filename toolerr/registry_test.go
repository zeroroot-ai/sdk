// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Zero Root AI

package toolerr

import (
	"sync"
	"testing"
)

// TestRegisterAndGetHints verifies basic registration and lookup
func TestRegisterAndGetHints(t *testing.T) {
	// Use a custom registry to avoid polluting global state
	registry := &RecoveryRegistry{
		registry: make(map[string]map[string][]RecoveryHint),
	}

	// Save and restore global registry
	oldRegistry := globalRegistry
	globalRegistry = registry
	defer func() { globalRegistry = oldRegistry }()

	hint1 := RecoveryHint{
		Strategy:    StrategyUseAlternative,
		Alternative: "masscan",
		Reason:      "masscan can perform similar port scanning",
		Confidence:  0.8,
		Priority:    1,
	}

	hint2 := RecoveryHint{
		Strategy:    StrategyUseAlternative,
		Alternative: "netcat",
		Reason:      "nc can probe individual ports",
		Confidence:  0.5,
		Priority:    2,
	}

	// Register hints
	Register("mytool", ErrCodeBinaryNotFound, hint1, hint2)

	// Retrieve hints
	hints := GetHints("mytool", ErrCodeBinaryNotFound)

	// Verify
	if len(hints) != 2 {
		t.Fatalf("GetHints returned %d hints, want 2", len(hints))
	}

	if hints[0].Alternative != "masscan" {
		t.Errorf("hints[0].Alternative = %q, want %q", hints[0].Alternative, "masscan")
	}

	if hints[1].Alternative != "netcat" {
		t.Errorf("hints[1].Alternative = %q, want %q", hints[1].Alternative, "netcat")
	}
}

// TestGetHintsNotFound verifies nil is returned for unknown tool/code
func TestGetHintsNotFound(t *testing.T) {
	registry := &RecoveryRegistry{
		registry: make(map[string]map[string][]RecoveryHint),
	}

	oldRegistry := globalRegistry
	globalRegistry = registry
	defer func() { globalRegistry = oldRegistry }()

	tests := []struct {
		name      string
		tool      string
		errorCode string
	}{
		{
			name:      "unknown tool",
			tool:      "unknown_tool",
			errorCode: ErrCodeBinaryNotFound,
		},
		{
			name:      "known tool unknown code",
			tool:      "mytool",
			errorCode: "UNKNOWN_CODE",
		},
		{
			name:      "both unknown",
			tool:      "unknown",
			errorCode: "UNKNOWN",
		},
	}

	// Register one known entry
	Register("mytool", ErrCodeBinaryNotFound, RecoveryHint{
		Strategy:   StrategyRetry,
		Reason:     "test",
		Confidence: 0.5,
		Priority:   1,
	})

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			hints := GetHints(tt.tool, tt.errorCode)
			if hints != nil {
				t.Errorf("GetHints(%q, %q) = %v, want nil", tt.tool, tt.errorCode, hints)
			}
		})
	}
}

// TestRegisterMultipleTools verifies registry handles multiple tools
func TestRegisterMultipleTools(t *testing.T) {
	registry := &RecoveryRegistry{
		registry: make(map[string]map[string][]RecoveryHint),
	}

	oldRegistry := globalRegistry
	globalRegistry = registry
	defer func() { globalRegistry = oldRegistry }()

	mytoolHint := RecoveryHint{
		Strategy:    StrategyUseAlternative,
		Alternative: "masscan",
		Reason:      "mytool alternative",
		Confidence:  0.8,
		Priority:    1,
	}

	masscanHint := RecoveryHint{
		Strategy:    StrategyUseAlternative,
		Alternative: "mytool",
		Reason:      "masscan alternative",
		Confidence:  0.7,
		Priority:    1,
	}

	// Register hints for different tools
	Register("mytool", ErrCodeBinaryNotFound, mytoolHint)
	Register("masscan", ErrCodeBinaryNotFound, masscanHint)

	// Verify mytool hints
	mytoolHints := GetHints("mytool", ErrCodeBinaryNotFound)
	if len(mytoolHints) != 1 {
		t.Fatalf("mytool hints count = %d, want 1", len(mytoolHints))
	}
	if mytoolHints[0].Alternative != "masscan" {
		t.Errorf("mytool hint alternative = %q, want %q", mytoolHints[0].Alternative, "masscan")
	}

	// Verify masscan hints
	masscanHints := GetHints("masscan", ErrCodeBinaryNotFound)
	if len(masscanHints) != 1 {
		t.Fatalf("masscan hints count = %d, want 1", len(masscanHints))
	}
	if masscanHints[0].Alternative != "mytool" {
		t.Errorf("masscan hint alternative = %q, want %q", masscanHints[0].Alternative, "mytool")
	}
}

// TestRegisterMultipleErrorCodes verifies registry handles multiple error codes per tool
func TestRegisterMultipleErrorCodes(t *testing.T) {
	registry := &RecoveryRegistry{
		registry: make(map[string]map[string][]RecoveryHint),
	}

	oldRegistry := globalRegistry
	globalRegistry = registry
	defer func() { globalRegistry = oldRegistry }()

	binaryNotFoundHint := RecoveryHint{
		Strategy:    StrategyUseAlternative,
		Alternative: "masscan",
		Reason:      "alternative for binary not found",
		Confidence:  0.8,
		Priority:    1,
	}

	timeoutHint := RecoveryHint{
		Strategy:   StrategyModifyParams,
		Params:     map[string]any{"timing": 2},
		Reason:     "slower timing for timeout",
		Confidence: 0.7,
		Priority:   1,
	}

	// Register hints for different error codes
	Register("mytool", ErrCodeBinaryNotFound, binaryNotFoundHint)
	Register("mytool", ErrCodeTimeout, timeoutHint)

	// Verify binary not found hints
	hints1 := GetHints("mytool", ErrCodeBinaryNotFound)
	if len(hints1) != 1 {
		t.Fatalf("binary not found hints count = %d, want 1", len(hints1))
	}
	if hints1[0].Strategy != StrategyUseAlternative {
		t.Errorf("hint strategy = %q, want %q", hints1[0].Strategy, StrategyUseAlternative)
	}

	// Verify timeout hints
	hints2 := GetHints("mytool", ErrCodeTimeout)
	if len(hints2) != 1 {
		t.Fatalf("timeout hints count = %d, want 1", len(hints2))
	}
	if hints2[0].Strategy != StrategyModifyParams {
		t.Errorf("hint strategy = %q, want %q", hints2[0].Strategy, StrategyModifyParams)
	}
}

// TestRegisterReplacesHints verifies that registering same tool/code replaces hints
func TestRegisterReplacesHints(t *testing.T) {
	registry := &RecoveryRegistry{
		registry: make(map[string]map[string][]RecoveryHint),
	}

	oldRegistry := globalRegistry
	globalRegistry = registry
	defer func() { globalRegistry = oldRegistry }()

	hint1 := RecoveryHint{
		Strategy:   StrategyRetry,
		Reason:     "first hint",
		Confidence: 0.5,
		Priority:   1,
	}

	hint2 := RecoveryHint{
		Strategy:   StrategySkip,
		Reason:     "second hint",
		Confidence: 0.3,
		Priority:   1,
	}

	// Register first hint
	Register("mytool", ErrCodeTimeout, hint1)

	// Verify first hint
	hints := GetHints("mytool", ErrCodeTimeout)
	if len(hints) != 1 {
		t.Fatalf("after first register, hints count = %d, want 1", len(hints))
	}
	if hints[0].Strategy != StrategyRetry {
		t.Errorf("hint strategy = %q, want %q", hints[0].Strategy, StrategyRetry)
	}

	// Register second hint (should replace)
	Register("mytool", ErrCodeTimeout, hint2)

	// Verify second hint replaced first
	hints = GetHints("mytool", ErrCodeTimeout)
	if len(hints) != 1 {
		t.Fatalf("after second register, hints count = %d, want 1", len(hints))
	}
	if hints[0].Strategy != StrategySkip {
		t.Errorf("hint strategy = %q, want %q", hints[0].Strategy, StrategySkip)
	}
}

// TestEnrichErrorSetsClass verifies EnrichError sets default class
func TestEnrichErrorSetsClass(t *testing.T) {
	registry := &RecoveryRegistry{
		registry: make(map[string]map[string][]RecoveryHint),
	}

	oldRegistry := globalRegistry
	globalRegistry = registry
	defer func() { globalRegistry = oldRegistry }()

	tests := []struct {
		name          string
		code          string
		expectedClass ErrorClass
	}{
		{
			name:          "binary not found gets infrastructure",
			code:          ErrCodeBinaryNotFound,
			expectedClass: ErrorClassInfrastructure,
		},
		{
			name:          "timeout gets transient",
			code:          ErrCodeTimeout,
			expectedClass: ErrorClassTransient,
		},
		{
			name:          "invalid input gets semantic",
			code:          ErrCodeInvalidInput,
			expectedClass: ErrorClassSemantic,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := New("test", "operation", tt.code, "test message")

			// Verify class is empty before enrichment
			if err.Class != "" {
				t.Fatalf("Class should be empty before enrichment, got %q", err.Class)
			}

			// Enrich
			enriched := EnrichError(err)

			// Verify class is set
			if enriched.Class != tt.expectedClass {
				t.Errorf("Class = %q, want %q", enriched.Class, tt.expectedClass)
			}
		})
	}
}

// TestEnrichErrorPreservesExistingClass verifies EnrichError doesn't override class
func TestEnrichErrorPreservesExistingClass(t *testing.T) {
	registry := &RecoveryRegistry{
		registry: make(map[string]map[string][]RecoveryHint),
	}

	oldRegistry := globalRegistry
	globalRegistry = registry
	defer func() { globalRegistry = oldRegistry }()

	err := New("test", "operation", ErrCodeTimeout, "test message").
		WithClass(ErrorClassPermanent)

	// Enrich
	enriched := EnrichError(err)

	// Verify class was NOT changed (timeout normally defaults to transient)
	if enriched.Class != ErrorClassPermanent {
		t.Errorf("Class = %q, want %q (should preserve existing class)", enriched.Class, ErrorClassPermanent)
	}
}

// TestEnrichErrorAppendsHints verifies EnrichError appends hints
func TestEnrichErrorAppendsHints(t *testing.T) {
	registry := &RecoveryRegistry{
		registry: make(map[string]map[string][]RecoveryHint),
	}

	oldRegistry := globalRegistry
	globalRegistry = registry
	defer func() { globalRegistry = oldRegistry }()

	registeredHint := RecoveryHint{
		Strategy:    StrategyUseAlternative,
		Alternative: "masscan",
		Reason:      "from registry",
		Confidence:  0.8,
		Priority:    1,
	}

	existingHint := RecoveryHint{
		Strategy:   StrategyRetry,
		Reason:     "manually added",
		Confidence: 0.5,
		Priority:   2,
	}

	// Register hint in registry
	Register("mytool", ErrCodeBinaryNotFound, registeredHint)

	// Create error with existing hint
	err := New("mytool", "scan", ErrCodeBinaryNotFound, "not found").
		WithHints(existingHint)

	// Verify one hint before enrichment
	if len(err.Hints) != 1 {
		t.Fatalf("Before enrichment, hints count = %d, want 1", len(err.Hints))
	}

	// Enrich
	enriched := EnrichError(err)

	// Verify hints were appended (not replaced)
	if len(enriched.Hints) != 2 {
		t.Fatalf("After enrichment, hints count = %d, want 2", len(enriched.Hints))
	}

	// Verify order: existing hint first, then registered hint
	if enriched.Hints[0].Reason != "manually added" {
		t.Errorf("hints[0].Reason = %q, want %q", enriched.Hints[0].Reason, "manually added")
	}
	if enriched.Hints[1].Reason != "from registry" {
		t.Errorf("hints[1].Reason = %q, want %q", enriched.Hints[1].Reason, "from registry")
	}
}

// TestEnrichErrorWithNoRegisteredHints verifies EnrichError works when no hints registered
func TestEnrichErrorWithNoRegisteredHints(t *testing.T) {
	registry := &RecoveryRegistry{
		registry: make(map[string]map[string][]RecoveryHint),
	}

	oldRegistry := globalRegistry
	globalRegistry = registry
	defer func() { globalRegistry = oldRegistry }()

	err := New("mytool", "scan", ErrCodeBinaryNotFound, "not found")

	// Enrich (no hints registered)
	enriched := EnrichError(err)

	// Should still set class
	if enriched.Class != ErrorClassInfrastructure {
		t.Errorf("Class = %q, want %q", enriched.Class, ErrorClassInfrastructure)
	}

	// Should have no hints
	if len(enriched.Hints) != 0 {
		t.Errorf("Hints count = %d, want 0", len(enriched.Hints))
	}
}

// TestEnrichErrorNil verifies EnrichError handles nil input
func TestEnrichErrorNil(t *testing.T) {
	result := EnrichError(nil)
	if result != nil {
		t.Errorf("EnrichError(nil) = %v, want nil", result)
	}
}

// TestConcurrentRegister verifies concurrent registration is thread-safe
func TestConcurrentRegister(t *testing.T) {
	registry := &RecoveryRegistry{
		registry: make(map[string]map[string][]RecoveryHint),
	}

	oldRegistry := globalRegistry
	globalRegistry = registry
	defer func() { globalRegistry = oldRegistry }()

	var wg sync.WaitGroup
	numGoroutines := 100

	// Concurrently register hints for different tools
	for i := range numGoroutines {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()

			tool := "tool" + string(rune('0'+id%10))
			hint := RecoveryHint{
				Strategy:   StrategyRetry,
				Reason:     "concurrent test",
				Confidence: 0.5,
				Priority:   id,
			}

			Register(tool, ErrCodeTimeout, hint)
		}(i)
	}

	wg.Wait()

	// Verify all tools were registered
	for i := range 10 {
		tool := "tool" + string(rune('0'+i))
		hints := GetHints(tool, ErrCodeTimeout)
		if hints == nil {
			t.Errorf("Tool %q was not registered", tool)
		}
	}
}

// TestConcurrentGetHints verifies concurrent reads are thread-safe
func TestConcurrentGetHints(t *testing.T) {
	registry := &RecoveryRegistry{
		registry: make(map[string]map[string][]RecoveryHint),
	}

	oldRegistry := globalRegistry
	globalRegistry = registry
	defer func() { globalRegistry = oldRegistry }()

	// Register hint
	hint := RecoveryHint{
		Strategy:   StrategyRetry,
		Reason:     "test",
		Confidence: 0.5,
		Priority:   1,
	}
	Register("mytool", ErrCodeBinaryNotFound, hint)

	var wg sync.WaitGroup
	numGoroutines := 100

	// Concurrently read hints
	for range numGoroutines {
		wg.Add(1)
		go func() {
			defer wg.Done()

			hints := GetHints("mytool", ErrCodeBinaryNotFound)
			if len(hints) != 1 {
				t.Errorf("GetHints returned %d hints, want 1", len(hints))
			}
		}()
	}

	wg.Wait()
}

// TestConcurrentRegisterAndGetHints verifies concurrent read/write is thread-safe
func TestConcurrentRegisterAndGetHints(t *testing.T) {
	registry := &RecoveryRegistry{
		registry: make(map[string]map[string][]RecoveryHint),
	}

	oldRegistry := globalRegistry
	globalRegistry = registry
	defer func() { globalRegistry = oldRegistry }()

	var wg sync.WaitGroup
	numGoroutines := 100

	// Mix of concurrent writes and reads
	for i := range numGoroutines {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()

			if id%2 == 0 {
				// Write
				hint := RecoveryHint{
					Strategy:   StrategyRetry,
					Reason:     "test",
					Confidence: 0.5,
					Priority:   id,
				}
				Register("mytool", ErrCodeTimeout, hint)
			} else {
				// Read
				_ = GetHints("mytool", ErrCodeTimeout)
			}
		}(i)
	}

	wg.Wait()

	// Verify registry is consistent
	hints := GetHints("mytool", ErrCodeTimeout)
	if hints == nil {
		t.Error("Expected hints to be registered")
	}
}

// TestConcurrentEnrichError verifies concurrent enrichment is thread-safe
func TestConcurrentEnrichError(t *testing.T) {
	registry := &RecoveryRegistry{
		registry: make(map[string]map[string][]RecoveryHint),
	}

	oldRegistry := globalRegistry
	globalRegistry = registry
	defer func() { globalRegistry = oldRegistry }()

	// Register hint
	hint := RecoveryHint{
		Strategy:   StrategyRetry,
		Reason:     "test",
		Confidence: 0.5,
		Priority:   1,
	}
	Register("mytool", ErrCodeBinaryNotFound, hint)

	var wg sync.WaitGroup
	numGoroutines := 100

	// Concurrently enrich errors
	for range numGoroutines {
		wg.Add(1)
		go func() {
			defer wg.Done()

			err := New("mytool", "scan", ErrCodeBinaryNotFound, "not found")
			enriched := EnrichError(err)

			if enriched.Class != ErrorClassInfrastructure {
				t.Errorf("Class = %q, want %q", enriched.Class, ErrorClassInfrastructure)
			}
			if len(enriched.Hints) != 1 {
				t.Errorf("Hints count = %d, want 1", len(enriched.Hints))
			}
		}()
	}

	wg.Wait()
}

// TestEnrichErrorReturnsOriginal verifies EnrichError returns the same instance
func TestEnrichErrorReturnsOriginal(t *testing.T) {
	registry := &RecoveryRegistry{
		registry: make(map[string]map[string][]RecoveryHint),
	}

	oldRegistry := globalRegistry
	globalRegistry = registry
	defer func() { globalRegistry = oldRegistry }()

	err := New("test", "operation", ErrCodeTimeout, "test message")
	enriched := EnrichError(err)

	// Verify it's the same instance
	if enriched != err {
		t.Error("EnrichError should return the same error instance")
	}
}

// TestEnrichErrorChaining verifies EnrichError can be chained
func TestEnrichErrorChaining(t *testing.T) {
	registry := &RecoveryRegistry{
		registry: make(map[string]map[string][]RecoveryHint),
	}

	oldRegistry := globalRegistry
	globalRegistry = registry
	defer func() { globalRegistry = oldRegistry }()

	hint := RecoveryHint{
		Strategy:   StrategyRetry,
		Reason:     "test",
		Confidence: 0.5,
		Priority:   1,
	}
	Register("mytool", ErrCodeBinaryNotFound, hint)

	// Chain EnrichError with other methods
	err := EnrichError(
		New("mytool", "scan", ErrCodeBinaryNotFound, "not found").
			WithDetails(map[string]any{"path": "/usr/bin"}),
	)

	// Verify all fields are set
	if err.Class != ErrorClassInfrastructure {
		t.Errorf("Class = %q, want %q", err.Class, ErrorClassInfrastructure)
	}
	if len(err.Hints) != 1 {
		t.Errorf("Hints count = %d, want 1", len(err.Hints))
	}
	if err.Details["path"] != "/usr/bin" {
		t.Errorf("Details[path] = %v, want %q", err.Details["path"], "/usr/bin")
	}
}

// BenchmarkRegister benchmarks the Register function
func BenchmarkRegister(b *testing.B) {
	registry := &RecoveryRegistry{
		registry: make(map[string]map[string][]RecoveryHint),
	}

	oldRegistry := globalRegistry
	globalRegistry = registry
	defer func() { globalRegistry = oldRegistry }()

	hint := RecoveryHint{
		Strategy:   StrategyRetry,
		Reason:     "test",
		Confidence: 0.5,
		Priority:   1,
	}

	b.ResetTimer()
	for range b.N {
		Register("mytool", ErrCodeBinaryNotFound, hint)
	}
}

// BenchmarkGetHints benchmarks the GetHints function
func BenchmarkGetHints(b *testing.B) {
	registry := &RecoveryRegistry{
		registry: make(map[string]map[string][]RecoveryHint),
	}

	oldRegistry := globalRegistry
	globalRegistry = registry
	defer func() { globalRegistry = oldRegistry }()

	hint := RecoveryHint{
		Strategy:   StrategyRetry,
		Reason:     "test",
		Confidence: 0.5,
		Priority:   1,
	}
	Register("mytool", ErrCodeBinaryNotFound, hint)

	b.ResetTimer()
	for range b.N {
		_ = GetHints("mytool", ErrCodeBinaryNotFound)
	}
}

// BenchmarkEnrichError benchmarks the EnrichError function
func BenchmarkEnrichError(b *testing.B) {
	registry := &RecoveryRegistry{
		registry: make(map[string]map[string][]RecoveryHint),
	}

	oldRegistry := globalRegistry
	globalRegistry = registry
	defer func() { globalRegistry = oldRegistry }()

	hint := RecoveryHint{
		Strategy:   StrategyRetry,
		Reason:     "test",
		Confidence: 0.5,
		Priority:   1,
	}
	Register("mytool", ErrCodeBinaryNotFound, hint)

	b.ResetTimer()
	for range b.N {
		err := New("mytool", "scan", ErrCodeBinaryNotFound, "not found")
		_ = EnrichError(err)
	}
}

// BenchmarkConcurrentGetHints benchmarks concurrent GetHints calls
func BenchmarkConcurrentGetHints(b *testing.B) {
	registry := &RecoveryRegistry{
		registry: make(map[string]map[string][]RecoveryHint),
	}

	oldRegistry := globalRegistry
	globalRegistry = registry
	defer func() { globalRegistry = oldRegistry }()

	hint := RecoveryHint{
		Strategy:   StrategyRetry,
		Reason:     "test",
		Confidence: 0.5,
		Priority:   1,
	}
	Register("mytool", ErrCodeBinaryNotFound, hint)

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			_ = GetHints("mytool", ErrCodeBinaryNotFound)
		}
	})
}
