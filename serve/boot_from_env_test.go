// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Zero Root AI

package serve

import (
	"strings"
	"testing"
)

// The boot contract of sdk#128: a developer sets two environment variables and
// starts the binary. No CLI step, no file they have to author, no option they
// have to remember.
//
// It did not hold for serve.Agent or serve.Tool. GIBSON_URL and
// GIBSON_BOOTSTRAP_TOKEN were read only by WithCapabilityGrantFromEnv, an
// opt-in option, while plugin.Serve read GIBSON_URL on its own. So the same two
// variables worked for one of the three entry points and not the other two, and
// the error a developer got was:
//
//	capability grant is required: set GIBSON_URL or call WithCapabilityGrant()
//
// telling them to set a variable they had already set. That is the state these
// tests pin shut.

// TestDefaultConfigReadsTheBootEnvironment is the whole change, at the layer
// every entry point shares.
func TestDefaultConfigReadsTheBootEnvironment(t *testing.T) {
	t.Setenv("GIBSON_URL", "https://platform.example.com")
	t.Setenv("GIBSON_BOOTSTRAP_TOKEN", "one-time-token")
	t.Setenv("GIBSON_HOST_KEY_PATH", "/tmp/does-not-need-to-exist/host_key.json")

	cfg := DefaultConfig()
	if cfg.PlatformURL != "https://platform.example.com" {
		t.Errorf("PlatformURL = %q; GIBSON_URL was set", cfg.PlatformURL)
	}
	if cfg.BootstrapToken != "one-time-token" {
		t.Errorf("BootstrapToken = %q; GIBSON_BOOTSTRAP_TOKEN was set", cfg.BootstrapToken)
	}
	if cfg.HostKeyPath != "/tmp/does-not-need-to-exist/host_key.json" {
		t.Errorf("HostKeyPath = %q; GIBSON_HOST_KEY_PATH was set", cfg.HostKeyPath)
	}
}

// TestTwoEnvVarsPassValidation is the error a developer actually hit. The
// message named the variable they had set, which made the real cause — an
// opt-in option they had not passed — unguessable.
func TestTwoEnvVarsPassValidation(t *testing.T) {
	t.Setenv("GIBSON_URL", "https://platform.example.com")
	t.Setenv("GIBSON_BOOTSTRAP_TOKEN", "one-time-token")

	if err := validateConfig(DefaultConfig()); err != nil {
		t.Fatalf("two environment variables did not satisfy validateConfig: %v", err)
	}
}

// TestNoEnvStillFailsLoudly keeps the other direction. A component with no
// platform configured must refuse to start, not silently serve nothing.
func TestNoEnvStillFailsLoudly(t *testing.T) {
	t.Setenv("GIBSON_URL", "")
	t.Setenv("GIBSON_BOOTSTRAP_TOKEN", "")

	err := validateConfig(DefaultConfig())
	if err == nil {
		t.Fatal("a component with no platform URL was allowed to start")
	}
	if !strings.Contains(err.Error(), "GIBSON_URL") {
		t.Errorf("the error does not name GIBSON_URL: %v", err)
	}
}

// TestTheEnvNamesAreTheOnesPluginServeReads. plugin.Serve reads GIBSON_URL
// directly, and capabilitygrant.ResolveBootstrap reads GIBSON_BOOTSTRAP_TOKEN.
// Three entry points reading three spellings of the same idea is how they came
// to disagree in the first place, so the names are asserted here rather than
// matched by eye across packages.
func TestTheEnvNamesAreTheOnesPluginServeReads(t *testing.T) {
	for _, name := range []string{"GIBSON_URL", "GIBSON_BOOTSTRAP_TOKEN"} {
		t.Setenv(name, "sentinel-"+name)
	}
	cfg := DefaultConfig()
	if cfg.PlatformURL != "sentinel-GIBSON_URL" {
		t.Errorf("the platform URL does not come from GIBSON_URL, the name plugin/serve.go reads")
	}
	if cfg.BootstrapToken != "sentinel-GIBSON_BOOTSTRAP_TOKEN" {
		t.Errorf("the bootstrap token does not come from GIBSON_BOOTSTRAP_TOKEN, the name ResolveBootstrap reads")
	}
	// A future rename landing in one place only is already covered, by
	// TestNoRetiredEnvVarNamesRemain in selfenrol_test.go, which fails on any
	// retired spelling appearing anywhere in this package.
}
