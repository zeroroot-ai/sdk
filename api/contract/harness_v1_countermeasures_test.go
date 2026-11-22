// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Zero Root AI

package contract_test

import (
	"testing"

	harnesspb "github.com/zeroroot-ai/sdk/api/gen/gibson/harness/v1"
)

// TestHarnessV1_Technique_Validation: the technique of a countermeasure read
// and of a finding is an ATT&CK technique id, or empty.
func TestHarnessV1_Technique_Validation(t *testing.T) {
	v := newValidator(t)

	for _, ok := range []string{"", "T1190", "T1059.001"} {
		if err := v.Validate(&harnesspb.GetCountermeasuresRequest{Technique: ok}); err != nil {
			t.Errorf("request technique %q: unexpected error: %v", ok, err)
		}
		if err := v.Validate(&harnesspb.ApplicationFinding{Technique: ok}); err != nil {
			t.Errorf("finding technique %q: unexpected error: %v", ok, err)
		}
	}
	for _, bad := range []string{"t1190", "T119", "T11900", "T1059.01", "T1059.0011", "D3-FA", "T1190 ", "attack"} {
		if err := v.Validate(&harnesspb.GetCountermeasuresRequest{Technique: bad}); err == nil {
			t.Errorf("request technique %q accepted", bad)
		}
		if err := v.Validate(&harnesspb.ApplicationFinding{Technique: bad}); err == nil {
			t.Errorf("finding technique %q accepted", bad)
		}
	}
}
