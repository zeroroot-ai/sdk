// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Zero Root AI

package generator

import "testing"

// TestToUpperSnake covers the defect this found on main. A single ALL-CAPS word
// has no underscore, so it used to fall through to the camelCase branch, which
// puts a separator before every capital:
//
//	TRIGGERED -> T_R_I_G_G_E_R_E_D
//	AFFECTS   -> A_F_F_E_C_T_S
//
// Both shipped in CoreRelationType. The enum NUMBERS were right, so no wire
// value was ever wrong and nothing failed. Only the member names were mangled,
// and nothing hand-written referenced them.
func TestToUpperSnake(t *testing.T) {
	cases := []struct{ in, want string }{
		// The regression: already all-caps, no underscore.
		{"TRIGGERED", "TRIGGERED"},
		{"AFFECTS", "AFFECTS"},
		// Already all-caps with underscores: unchanged.
		{"USED_TOOL", "USED_TOOL"},
		{"HAS_SUBDOMAIN", "HAS_SUBDOMAIN"},
		{"SERVES_CERTIFICATE", "SERVES_CERTIFICATE"},
		// lower_snake, the node-type convention.
		{"mission_run", "MISSION_RUN"},
		{"compliance_signal", "COMPLIANCE_SIGNAL"},
		{"host", "HOST"},
		// camelCase still splits, which is the branch's reason to exist.
		{"missionRun", "MISSION_RUN"},
		{"toolExecution", "TOOL_EXECUTION"},
		// Mixed case with an underscore.
		{"mission_Run", "MISSION_RUN"},
		// Digits must not introduce a separator.
		{"HTTP2", "HTTP2"},
	}

	for _, c := range cases {
		t.Run(c.in, func(t *testing.T) {
			if got := toUpperSnake(c.in); got != c.want {
				t.Errorf("toUpperSnake(%q) = %q, want %q", c.in, got, c.want)
			}
		})
	}
}
