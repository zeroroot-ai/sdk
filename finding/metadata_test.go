// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Zero Root AI

package finding

import (
	"testing"
)

func TestMetadataKeyConstants(t *testing.T) {
	// Test that all constants are defined with expected values
	tests := []struct {
		constant string
		expected string
	}{
		{MetaKeyMitreAttack, "mitre_attack"},
		{MetaKeyMitreAtlas, "mitre_atlas"},
		{MetaKeyCVSS, "cvss"},
		{MetaKeyCWE, "cwe"},
	}

	for _, tt := range tests {
		if tt.constant != tt.expected {
			t.Errorf("Constant mismatch: expected %s, got %s", tt.expected, tt.constant)
		}
	}
}
