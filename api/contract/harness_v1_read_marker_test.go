// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Zero Root AI

package contract_test

import (
	"strings"
	"testing"

	harnesspb "github.com/zeroroot-ai/sdk/api/gen/gibson/harness/v1"
)

// TestHarnessV1_ReadMarker_Validation: a marker with no Connector names no
// loop of an agent, so the daemon could not find it again after a restart.
func TestHarnessV1_ReadMarker_Validation(t *testing.T) {
	v := newValidator(t)

	t.Run("accepts a marker with a connector", func(t *testing.T) {
		req := &harnesspb.WriteReadMarkerRequest{Connector: "splunk", Cursor: "c-1", ResultIds: []string{"r-1"}}
		if err := v.Validate(req); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if err := v.Validate(&harnesspb.LastReadMarkerRequest{Connector: "splunk"}); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	tooMany := make([]string, 1001)
	for i := range tooMany {
		tooMany[i] = "r"
	}
	cases := []struct {
		name string
		err  error
		want string
	}{
		{"a write with no connector", v.Validate(&harnesspb.WriteReadMarkerRequest{Cursor: "c-1"}), "connector"},
		{"a read with no connector", v.Validate(&harnesspb.LastReadMarkerRequest{}), "connector"},
		{
			"a write with too many result ids",
			v.Validate(&harnesspb.WriteReadMarkerRequest{Connector: "splunk", ResultIds: tooMany}),
			"result_ids",
		},
	}
	for _, tc := range cases {
		t.Run("rejects "+tc.name, func(t *testing.T) {
			if tc.err == nil {
				t.Fatal("expected a validation error, got nil")
			}
			if !strings.Contains(tc.err.Error(), tc.want) {
				t.Errorf("error should name %q: %v", tc.want, tc.err)
			}
		})
	}
}
