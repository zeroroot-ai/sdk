// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Zero Root AI

package capabilitygrant

import (
	"testing"
)

func TestResolveBootstrap_TrimsWhitespace(t *testing.T) {
	cred, err := ResolveBootstrap("  padded  \n")
	if err != nil {
		t.Fatal(err)
	}
	if cred.Token != "padded" {
		t.Errorf("got %q, want 'padded'", cred.Token)
	}
}

// An explicit token takes precedence over the environment variable.
func TestResolveBootstrap_ExplicitOverridesEnv(t *testing.T) {
	t.Setenv("GIBSON_BOOTSTRAP_TOKEN", "env-token")
	cred, err := ResolveBootstrap("explicit-token")
	if err != nil {
		t.Fatal(err)
	}
	if cred.Token != "explicit-token" {
		t.Errorf("got %q, want explicit-token", cred.Token)
	}
}
