// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Zero Root AI

package serve

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// A developer sets two environment variables and starts their component. No
// CLI step, no manifest, no option call. These tests pin that contract at the
// level it can be checked hermetically: the names, and the fact that every
// entrypoint resolves them the same way (sdk#128, ADR-0097).
//
// The handshake itself already existed in capabilitygrant — Discover →
// Bootstrap → Register, with ResolveBootstrap falling back to the environment.
// What did NOT hold was the naming: `serve` read GIBSON_AGENT_BOOTSTRAP_TOKEN
// and GIBSON_PLATFORM_URL while capabilitygrant and the plugin path read
// GIBSON_BOOTSTRAP_TOKEN and GIBSON_URL, so the documented variables and the
// option that claims to read them disagreed.

func TestWithCapabilityGrantFromEnv_ReadsTheCanonicalNames(t *testing.T) {
	t.Setenv("GIBSON_URL", "https://api.example.test")
	t.Setenv("GIBSON_BOOTSTRAP_TOKEN", "tok-canonical")
	t.Setenv("GIBSON_HOST_KEY_PATH", "/tmp/hk")

	cfg := &Config{}
	WithCapabilityGrantFromEnv()(cfg)

	if cfg.PlatformURL != "https://api.example.test" {
		t.Errorf("PlatformURL = %q, want the value of GIBSON_URL", cfg.PlatformURL)
	}
	if cfg.BootstrapToken != "tok-canonical" {
		t.Errorf("BootstrapToken = %q, want the value of GIBSON_BOOTSTRAP_TOKEN", cfg.BootstrapToken)
	}
	if cfg.HostKeyPath != "/tmp/hk" {
		t.Errorf("HostKeyPath = %q", cfg.HostKeyPath)
	}
}

// The retired names must not work. Leaving them live would be the parallel path
// ADR-0027 forbids, and a developer setting one would get silence.
func TestWithCapabilityGrantFromEnv_RetiredNamesAreNotRead(t *testing.T) {
	for _, retired := range []string{"GIBSON_AGENT_BOOTSTRAP_TOKEN", "GIBSON_PLATFORM_URL"} {
		t.Setenv("GIBSON_URL", "")
		t.Setenv("GIBSON_BOOTSTRAP_TOKEN", "")
		t.Setenv(retired, "should-be-ignored")

		cfg := &Config{}
		WithCapabilityGrantFromEnv()(cfg)

		if cfg.PlatformURL == "should-be-ignored" || cfg.BootstrapToken == "should-be-ignored" {
			t.Errorf("%s is still read; it was retired in favour of the canonical name", retired)
		}
	}
}

// No source file may name a retired variable, including in a doc comment: a
// comment telling a developer to set GIBSON_PLATFORM_URL is worse than no
// comment, because it reads as current.
func TestNoRetiredEnvVarNamesRemain(t *testing.T) {
	retired := []string{"GIBSON_AGENT_BOOTSTRAP_TOKEN", "GIBSON_PLATFORM_URL"}

	roots := []string{".", "../capabilitygrant", "../plugin"}
	for _, root := range roots {
		entries, err := os.ReadDir(root)
		if err != nil {
			t.Fatalf("read %s: %v", root, err)
		}
		for _, e := range entries {
			if e.IsDir() || !strings.HasSuffix(e.Name(), ".go") {
				continue
			}
			// This file names them on purpose, to assert they are gone.
			if e.Name() == "selfenrol_test.go" {
				continue
			}
			// #nosec G304 -- the path comes from reading this repository's own
			// source directories, listed above; there is no external input.
			b, err := os.ReadFile(filepath.Join(root, e.Name()))
			if err != nil {
				t.Fatalf("read %s/%s: %v", root, e.Name(), err)
			}
			for _, name := range retired {
				if strings.Contains(string(b), name) {
					t.Errorf("%s/%s still names the retired variable %s", root, e.Name(), name)
				}
			}
		}
	}
}

// WithPlatformFromEnv reads the same two canonical names. It is a separate
// option covering SPIFFE and daemon address as well, and it carried its own
// copy of the retired names — which is how the two halves of `serve` drifted
// apart from capabilitygrant without anything failing.
func TestWithPlatformFromEnv_ReadsTheCanonicalNames(t *testing.T) {
	t.Setenv("GIBSON_URL", "https://api.platform.test")
	t.Setenv("GIBSON_BOOTSTRAP_TOKEN", "tok-platform")
	t.Setenv("GIBSON_HOST_KEY_PATH", "/tmp/hk-platform")

	cfg := &Config{}
	WithPlatformFromEnv()(cfg)

	if cfg.PlatformURL != "https://api.platform.test" {
		t.Errorf("PlatformURL = %q, want the value of GIBSON_URL", cfg.PlatformURL)
	}
	if cfg.BootstrapToken != "tok-platform" {
		t.Errorf("BootstrapToken = %q, want the value of GIBSON_BOOTSTRAP_TOKEN", cfg.BootstrapToken)
	}
	if cfg.HostKeyPath != "/tmp/hk-platform" {
		t.Errorf("HostKeyPath = %q", cfg.HostKeyPath)
	}
}

func TestWithPlatformFromEnv_RetiredNamesAreNotRead(t *testing.T) {
	for _, retired := range []string{"GIBSON_AGENT_BOOTSTRAP_TOKEN", "GIBSON_PLATFORM_URL"} {
		t.Setenv("GIBSON_URL", "")
		t.Setenv("GIBSON_BOOTSTRAP_TOKEN", "")
		t.Setenv(retired, "should-be-ignored")

		cfg := &Config{}
		WithPlatformFromEnv()(cfg)

		if cfg.PlatformURL == "should-be-ignored" || cfg.BootstrapToken == "should-be-ignored" {
			t.Errorf("%s is still read by WithPlatformFromEnv", retired)
		}
	}
}
