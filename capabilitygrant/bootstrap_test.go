// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Zero Root AI

package capabilitygrant

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestResolveBootstrap_ExplicitToken(t *testing.T) {
	cred, err := ResolveBootstrap("my-token")
	if err != nil {
		t.Fatal(err)
	}
	if cred.Type != "api_key" || cred.Token != "my-token" {
		t.Errorf("unexpected credential: %+v", cred)
	}
}

func TestResolveBootstrap_TrimsWhitespace(t *testing.T) {
	cred, err := ResolveBootstrap("  padded  \n")
	if err != nil {
		t.Fatal(err)
	}
	if cred.Token != "padded" {
		t.Errorf("got %q, want 'padded'", cred.Token)
	}
}

// The documented GIBSON_BOOTSTRAP_TOKEN fallback is honored when no explicit
// token is passed (the process-mode bridge-runner contract).
func TestResolveBootstrap_EnvFallback(t *testing.T) {
	t.Setenv("GIBSON_BOOTSTRAP_TOKEN", "  env-token  ")
	cred, err := ResolveBootstrap("")
	if err != nil {
		t.Fatal(err)
	}
	if cred.Type != "api_key" || cred.Token != "env-token" {
		t.Errorf("unexpected credential from env: %+v", cred)
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

func TestResolveBootstrapFromSecret_ReadsFile(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "token"), []byte("secret-value\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	cred, err := ResolveBootstrapFromSecret(dir, "token")
	if err != nil {
		t.Fatal(err)
	}
	if cred.Token != "secret-value" {
		t.Errorf("got %q, want 'secret-value'", cred.Token)
	}
}

func TestResolveBootstrapFromSecret_MissingFile(t *testing.T) {
	_, err := ResolveBootstrapFromSecret(t.TempDir(), "nonexistent")
	if err == nil {
		t.Fatal("expected error for missing file")
	}
	if !strings.Contains(err.Error(), "read bootstrap secret") {
		t.Errorf("error should mention read: %v", err)
	}
}

func TestResolveBootstrapFromSecret_EmptyFile(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "token"), []byte("   \n"), 0o600); err != nil {
		t.Fatal(err)
	}
	_, err := ResolveBootstrapFromSecret(dir, "token")
	if err == nil || !strings.Contains(err.Error(), "empty") {
		t.Errorf("expected empty-secret error, got %v", err)
	}
}

func TestMustResolveBootstrapFromSecret_Panics(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Error("expected panic on missing file")
		}
	}()
	_ = MustResolveBootstrapFromSecret(t.TempDir(), "nonexistent")
}

func TestMustResolveBootstrapFromSecret_Succeeds(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "key"), []byte("xyz"), 0o600); err != nil {
		t.Fatal(err)
	}
	cred := MustResolveBootstrapFromSecret(dir, "key")
	if cred.Token != "xyz" {
		t.Errorf("got %q, want 'xyz'", cred.Token)
	}
}
