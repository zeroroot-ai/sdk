// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Zero Root AI

package secretenv

import "testing"

// The daemon writes the variable and the component reads it. These assert the
// exact bytes both sides agree on, so a change to the fold has to change this
// file and is visible in review.
func TestKey(t *testing.T) {
	for _, tc := range []struct {
		name string
		want string
	}{
		{"goat-kubeconfig", "GIBSON_SECRET_GOAT_KUBECONFIG"},
		{"GOAT_KUBECONFIG", "GIBSON_SECRET_GOAT_KUBECONFIG"},
		{"goat.kubeconfig", "GIBSON_SECRET_GOAT_KUBECONFIG"},
		{"goat kubeconfig", "GIBSON_SECRET_GOAT_KUBECONFIG"},
		{"prod/db", "GIBSON_SECRET_PROD_DB"},
		{"api-key-2", "GIBSON_SECRET_API_KEY_2"},
		{"", "GIBSON_SECRET_"},
	} {
		if got := Key(tc.name); got != tc.want {
			t.Errorf("Key(%q) = %q, want %q", tc.name, got, tc.want)
		}
	}
}

// The fold is not injective, and both sides must see the same collision so the
// daemon's refusal and the component's read agree on what collided.
func TestKey_FoldsDistinctNamesOntoOneVariable(t *testing.T) {
	if Key("a-b") != Key("a.b") {
		t.Fatal(`Key("a-b") and Key("a.b") must agree; the daemon refuses this collision and can only do so if the rule is shared`)
	}
}

func TestLookup_ReadsTheDeclaredSecret(t *testing.T) {
	t.Setenv("GIBSON_SECRET_GOAT_KUBECONFIG", "apiVersion: v1")

	got, ok := Lookup("goat-kubeconfig")
	if !ok {
		t.Fatal("Lookup reported the secret missing when the daemon had set it")
	}
	if got != "apiVersion: v1" {
		t.Errorf("Lookup = %q, want the value the daemon set", got)
	}
}

// A secret the mission did not declare for this component is absent, not empty.
// A component that treated absence as a usable credential would run against
// whatever ambient configuration it found.
func TestLookup_UndeclaredSecretIsMissing(t *testing.T) {
	if _, ok := Lookup("never-declared"); ok {
		t.Error("Lookup reported a secret nobody declared as present")
	}
}

// An empty variable is missing, not blank. The daemon refuses to dispatch with
// a blank credential, so an empty value did not come from the daemon.
func TestLookup_EmptyIsMissingNotBlank(t *testing.T) {
	t.Setenv("GIBSON_SECRET_GOAT_KUBECONFIG", "")

	if v, ok := Lookup("goat-kubeconfig"); ok {
		t.Errorf("Lookup returned %q and ok for an empty variable; an empty credential must read as missing", v)
	}
}
