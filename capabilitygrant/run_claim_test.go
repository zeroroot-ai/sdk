// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Zero Root AI

package capabilitygrant

import (
	"encoding/base64"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
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

func TestClaimsOf(t *testing.T) {
	tok := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"iss": "i", "aud": "a", "sub": "agent-1", "tenant": "t1", "mission_id": "m", "task_id": "k",
		"exp": time.Now().Add(time.Hour).Unix(),
	})
	s, err := tok.SignedString([]byte("k"))
	if err != nil {
		t.Fatal(err)
	}
	c, err := ClaimsOf(s)
	if err != nil {
		t.Fatal(err)
	}
	if c.Subject != "agent-1" || c.MissionID != "m" || c.TaskID != "k" || c.ExpiresAt.IsZero() {
		t.Fatalf("claims = %+v", c)
	}
	if _, err := ClaimsOf("junk"); err == nil {
		t.Fatal("a token that does not parse must fail")
	}
	// A grant with no tenant claim is refused.
	bad, _ := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{"sub": "x"}).SignedString([]byte("k"))
	if _, err := ClaimsOf(bad); err == nil {
		t.Fatal("a grant with no tenant must fail")
	}
}
