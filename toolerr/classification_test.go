// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Zero Root AI

package toolerr

import (
	"encoding/json"
	"testing"
)

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
