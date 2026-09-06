// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Zero Root AI

package toolerr

import (
	"encoding/json"
	"testing"
)

// TestErrorClassConstants verifies all ErrorClass constants are defined
func TestErrorClassConstants(t *testing.T) {
	classes := []ErrorClass{
		ErrorClassInfrastructure,
		ErrorClassSemantic,
		ErrorClassTransient,
		ErrorClassPermanent,
	}

	expected := []string{
		"infrastructure",
		"semantic",
		"transient",
		"permanent",
	}

	for i, class := range classes {
		if string(class) != expected[i] {
			t.Errorf("ErrorClass[%d] = %q, want %q", i, class, expected[i])
		}
		if class == "" {
			t.Errorf("ErrorClass[%d] is empty", i)
		}
	}
}

// TestRecoveryStrategyConstants verifies all RecoveryStrategy constants are defined
func TestRecoveryStrategyConstants(t *testing.T) {
	strategies := []RecoveryStrategy{
		StrategyRetry,
		StrategyRetryWithBackoff,
		StrategyModifyParams,
		StrategyUseAlternative,
		StrategySpawnAgent,
		StrategySkip,
	}

	expected := []string{
		"retry",
		"retry_with_backoff",
		"modify_params",
		"use_alternative_tool",
		"spawn_agent",
		"skip",
	}

	for i, strategy := range strategies {
		if string(strategy) != expected[i] {
			t.Errorf("RecoveryStrategy[%d] = %q, want %q", i, strategy, expected[i])
		}
		if strategy == "" {
			t.Errorf("RecoveryStrategy[%d] is empty", i)
		}
	}
}

// TestDefaultClassForCode verifies error code to class mapping
func TestDefaultClassForCode(t *testing.T) {
	tests := []struct {
		name     string
		code     string
		expected ErrorClass
	}{
		{
			name:     "binary not found is infrastructure",
			code:     ErrCodeBinaryNotFound,
			expected: ErrorClassInfrastructure,
		},
		{
			name:     "permission denied is infrastructure",
			code:     ErrCodePermissionDenied,
			expected: ErrorClassInfrastructure,
		},
		{
			name:     "dependency missing is infrastructure",
			code:     ErrCodeDependencyMissing,
			expected: ErrorClassInfrastructure,
		},
		{
			name:     "invalid input is semantic",
			code:     ErrCodeInvalidInput,
			expected: ErrorClassSemantic,
		},
		{
			name:     "parse error is semantic",
			code:     ErrCodeParseError,
			expected: ErrorClassSemantic,
		},
		{
			name:     "timeout is transient",
			code:     ErrCodeTimeout,
			expected: ErrorClassTransient,
		},
		{
			name:     "network error is transient",
			code:     ErrCodeNetworkError,
			expected: ErrorClassTransient,
		},
		{
			name:     "execution failed defaults to transient",
			code:     ErrCodeExecutionFailed,
			expected: ErrorClassTransient,
		},
		{
			name:     "unknown code defaults to transient",
			code:     "UNKNOWN_ERROR",
			expected: ErrorClassTransient,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := DefaultClassForCode(tt.code)
			if got != tt.expected {
				t.Errorf("DefaultClassForCode(%q) = %q, want %q", tt.code, got, tt.expected)
			}
		})
	}
}

// TestWithClass verifies WithClass method works correctly
func TestWithClass(t *testing.T) {
	tests := []struct {
		name  string
		class ErrorClass
	}{
		{
			name:  "infrastructure class",
			class: ErrorClassInfrastructure,
		},
		{
			name:  "semantic class",
			class: ErrorClassSemantic,
		},
		{
			name:  "transient class",
			class: ErrorClassTransient,
		},
		{
			name:  "permanent class",
			class: ErrorClassPermanent,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := New("test", "operation", ErrCodeTimeout, "test message").
				WithClass(tt.class)

			if err.Class != tt.class {
				t.Errorf("Class = %q, want %q", err.Class, tt.class)
			}
		})
	}
}

// TestWithHints verifies WithHints method works correctly
func TestWithHints(t *testing.T) {
	hint1 := RecoveryHint{
		Strategy:    StrategyUseAlternative,
		Alternative: "masscan",
		Reason:      "masscan can perform similar scanning",
		Confidence:  0.8,
		Priority:    1,
	}

	hint2 := RecoveryHint{
		Strategy:   StrategyModifyParams,
		Params:     map[string]any{"timeout": "60s"},
		Reason:     "longer timeout may help",
		Confidence: 0.6,
		Priority:   2,
	}

	tests := []struct {
		name          string
		hints         []RecoveryHint
		expectedCount int
	}{
		{
			name:          "single hint",
			hints:         []RecoveryHint{hint1},
			expectedCount: 1,
		},
		{
			name:          "multiple hints",
			hints:         []RecoveryHint{hint1, hint2},
			expectedCount: 2,
		},
		{
			name:          "no hints",
			hints:         []RecoveryHint{},
			expectedCount: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := New("test", "operation", ErrCodeTimeout, "test message").
				WithHints(tt.hints...)

			if len(err.Hints) != tt.expectedCount {
				t.Errorf("len(Hints) = %d, want %d", len(err.Hints), tt.expectedCount)
			}

			for i, hint := range tt.hints {
				if err.Hints[i].Strategy != hint.Strategy {
					t.Errorf("Hints[%d].Strategy = %q, want %q", i, err.Hints[i].Strategy, hint.Strategy)
				}
			}
		})
	}
}

// TestWithHintsAppends verifies that WithHints appends rather than replaces
func TestWithHintsAppends(t *testing.T) {
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
		Priority:   2,
	}

	err := New("test", "operation", ErrCodeTimeout, "test message")

	// Add first hint
	err.WithHints(hint1)
	if len(err.Hints) != 1 {
		t.Fatalf("After first WithHints, len(Hints) = %d, want 1", len(err.Hints))
	}

	// Add second hint - should append
	err.WithHints(hint2)
	if len(err.Hints) != 2 {
		t.Fatalf("After second WithHints, len(Hints) = %d, want 2", len(err.Hints))
	}

	// Verify both hints are present
	if err.Hints[0].Strategy != StrategyRetry {
		t.Errorf("Hints[0].Strategy = %q, want %q", err.Hints[0].Strategy, StrategyRetry)
	}
	if err.Hints[1].Strategy != StrategySkip {
		t.Errorf("Hints[1].Strategy = %q, want %q", err.Hints[1].Strategy, StrategySkip)
	}
}

// TestMethodChainingWithClassAndHints verifies fluent API works with new methods
func TestMethodChainingWithClassAndHints(t *testing.T) {
	hint := RecoveryHint{
		Strategy:    StrategyUseAlternative,
		Alternative: "masscan",
		Reason:      "alternative tool",
		Confidence:  0.8,
		Priority:    1,
	}

	// Test all combinations of chaining
	err1 := New("test", "op", ErrCodeBinaryNotFound, "msg").
		WithClass(ErrorClassInfrastructure).
		WithHints(hint).
		WithDetails(map[string]any{"key": "value"})

	if err1.Class != ErrorClassInfrastructure {
		t.Errorf("err1.Class = %q, want %q", err1.Class, ErrorClassInfrastructure)
	}
	if len(err1.Hints) != 1 {
		t.Errorf("len(err1.Hints) = %d, want 1", len(err1.Hints))
	}
	if err1.Details["key"] != "value" {
		t.Errorf("err1.Details[key] = %v, want %q", err1.Details["key"], "value")
	}

	// Test reverse order
	err2 := New("test", "op", ErrCodeBinaryNotFound, "msg").
		WithDetails(map[string]any{"key": "value"}).
		WithHints(hint).
		WithClass(ErrorClassInfrastructure)

	if err2.Class != ErrorClassInfrastructure {
		t.Errorf("err2.Class = %q, want %q", err2.Class, ErrorClassInfrastructure)
	}
	if len(err2.Hints) != 1 {
		t.Errorf("len(err2.Hints) = %d, want 1", len(err2.Hints))
	}
}

// TestRecoveryHintJSON verifies RecoveryHint serializes correctly
func TestRecoveryHintJSON(t *testing.T) {
	hint := RecoveryHint{
		Strategy:    StrategyModifyParams,
		Alternative: "alternative_tool",
		Params: map[string]any{
			"timeout": "60s",
			"retries": 3,
		},
		Reason:     "test reason",
		Confidence: 0.75,
		Priority:   2,
	}

	// Marshal to JSON
	data, err := json.Marshal(hint)
	if err != nil {
		t.Fatalf("json.Marshal failed: %v", err)
	}

	// Unmarshal back
	var decoded RecoveryHint
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("json.Unmarshal failed: %v", err)
	}

	// Verify fields
	if decoded.Strategy != hint.Strategy {
		t.Errorf("Strategy = %q, want %q", decoded.Strategy, hint.Strategy)
	}
	if decoded.Alternative != hint.Alternative {
		t.Errorf("Alternative = %q, want %q", decoded.Alternative, hint.Alternative)
	}
	if decoded.Reason != hint.Reason {
		t.Errorf("Reason = %q, want %q", decoded.Reason, hint.Reason)
	}
	if decoded.Confidence != hint.Confidence {
		t.Errorf("Confidence = %f, want %f", decoded.Confidence, hint.Confidence)
	}
	if decoded.Priority != hint.Priority {
		t.Errorf("Priority = %d, want %d", decoded.Priority, hint.Priority)
	}
}

// TestErrorWithClassJSON verifies Error serializes Class correctly with omitempty
func TestErrorWithClassJSON(t *testing.T) {
	// Error with class
	err1 := New("test", "op", ErrCodeTimeout, "msg").
		WithClass(ErrorClassTransient)

	data1, err := json.Marshal(err1)
	if err != nil {
		t.Fatalf("json.Marshal failed: %v", err)
	}

	var decoded1 map[string]any
	if err := json.Unmarshal(data1, &decoded1); err != nil {
		t.Fatalf("json.Unmarshal failed: %v", err)
	}

	if decoded1["class"] != "transient" {
		t.Errorf("class field = %v, want %q", decoded1["class"], "transient")
	}

	// Error without class should omit field
	err2 := New("test", "op", ErrCodeTimeout, "msg")

	data2, err := json.Marshal(err2)
	if err != nil {
		t.Fatalf("json.Marshal failed: %v", err)
	}

	var decoded2 map[string]any
	if err := json.Unmarshal(data2, &decoded2); err != nil {
		t.Fatalf("json.Unmarshal failed: %v", err)
	}

	if _, exists := decoded2["class"]; exists {
		t.Error("class field should be omitted when empty")
	}
}

// TestErrorWithHintsJSON verifies Error serializes Hints correctly with omitempty
func TestErrorWithHintsJSON(t *testing.T) {
	hint := RecoveryHint{
		Strategy:   StrategyRetry,
		Reason:     "test",
		Confidence: 0.5,
		Priority:   1,
	}

	// Error with hints
	err1 := New("test", "op", ErrCodeTimeout, "msg").
		WithHints(hint)

	data1, err := json.Marshal(err1)
	if err != nil {
		t.Fatalf("json.Marshal failed: %v", err)
	}

	var decoded1 map[string]any
	if err := json.Unmarshal(data1, &decoded1); err != nil {
		t.Fatalf("json.Unmarshal failed: %v", err)
	}

	hints, ok := decoded1["hints"].([]any)
	if !ok || len(hints) != 1 {
		t.Errorf("hints field = %v, want array with 1 element", decoded1["hints"])
	}

	// Error without hints should omit field
	err2 := New("test", "op", ErrCodeTimeout, "msg")

	data2, err := json.Marshal(err2)
	if err != nil {
		t.Fatalf("json.Marshal failed: %v", err)
	}

	var decoded2 map[string]any
	if err := json.Unmarshal(data2, &decoded2); err != nil {
		t.Fatalf("json.Unmarshal failed: %v", err)
	}

	if _, exists := decoded2["hints"]; exists {
		t.Error("hints field should be omitted when empty")
	}
}

// TestBackwardCompatibility verifies existing code still works without new fields
func TestBackwardCompatibility(t *testing.T) {
	// Create error without using new fields
	err := New("mytool", "scan", ErrCodeBinaryNotFound, "binary not found").
		WithCause(ErrBinaryNotFound).
		WithDetails(map[string]any{"path": "/usr/bin"})

	// Should work as before
	if err.Tool != "mytool" {
		t.Errorf("Tool = %q, want %q", err.Tool, "mytool")
	}
	if err.Code != ErrCodeBinaryNotFound {
		t.Errorf("Code = %q, want %q", err.Code, ErrCodeBinaryNotFound)
	}

	// New fields should be zero values
	if err.Class != "" {
		t.Errorf("Class should be empty, got %q", err.Class)
	}
	if len(err.Hints) != 0 {
		t.Errorf("Hints should be empty, got %d hints", len(err.Hints))
	}

	// Error formatting should work
	errStr := err.Error()
	if errStr == "" {
		t.Error("Error() should return non-empty string")
	}
}

// BenchmarkWithClass benchmarks the WithClass method
func BenchmarkWithClass(b *testing.B) {
	for range b.N {
		_ = New("test", "op", ErrCodeTimeout, "msg").
			WithClass(ErrorClassTransient)
	}
}

// BenchmarkWithHints benchmarks the WithHints method
func BenchmarkWithHints(b *testing.B) {
	hint := RecoveryHint{
		Strategy:   StrategyRetry,
		Reason:     "test",
		Confidence: 0.5,
		Priority:   1,
	}
	b.ResetTimer()
	for range b.N {
		_ = New("test", "op", ErrCodeTimeout, "msg").
			WithHints(hint)
	}
}

// BenchmarkDefaultClassForCode benchmarks the DefaultClassForCode function
func BenchmarkDefaultClassForCode(b *testing.B) {
	codes := []string{
		ErrCodeBinaryNotFound,
		ErrCodeTimeout,
		ErrCodeInvalidInput,
	}
	b.ResetTimer()
	for range b.N {
		for _, code := range codes {
			_ = DefaultClassForCode(code)
		}
	}
}
