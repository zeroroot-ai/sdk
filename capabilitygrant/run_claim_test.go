// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Zero Root AI

package capabilitygrant

import (
	"encoding/base64"
	"testing"
)

func tokenWithPayload(payload string) string {
	enc := base64.RawURLEncoding.EncodeToString
	return enc([]byte(`{"alg":"EdDSA"}`)) + "." + enc([]byte(payload)) + "." + enc([]byte("sig"))
}

func TestMissionRunOf(t *testing.T) {
	cases := map[string]struct {
		token string
		want  string
	}{
		"a grant with a run":          {tokenWithPayload(`{"tenant":"acme","mrid":"run-4"}`), "run-4"},
		"a grant with no run":         {tokenWithPayload(`{"tenant":"acme","task_id":"t"}`), ""},
		"a run that is no string":     {tokenWithPayload(`{"tenant":"acme","mrid":7}`), ""},
		"a payload that is no JSON":   {tokenWithPayload(`nope`), ""},
		"two parts":                   {"a.b", ""},
		"a grant with no tenant":      {tokenWithPayload(`{"mrid":"run-4"}`), ""},
		"a payload that is no base64": {"a.%%%.c", ""},
		"empty":                       {"", ""},
	}
	for name, tc := range cases {
		if got := MissionRunOf(tc.token); got != tc.want {
			t.Errorf("%s: MissionRunOf = %q, want %q", name, got, tc.want)
		}
	}
}
