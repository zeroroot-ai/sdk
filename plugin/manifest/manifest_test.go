// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Zero Root AI

package manifest_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/zeroroot-ai/sdk/plugin/manifest"
)

// exampleManifest is the canonical example from design.md, used for
// round-trip tests.
const exampleManifest = `
apiVersion: plugin.gibson.zeroroot.ai/v1
kind: Plugin
metadata:
  name: github
  version: 0.1.0
  description: Stateful proxy for GitHub REST API
  author: zeroroot-ai
spec:
  workload_class: plugin
  secrets:
    - name: cred:github_token
      scope: startup
      rotation: live
      required: true
  methods:
    - name: GetRepository
      description: Fetch a repository by owner/name
  runtime: process
  policy:
    setec_required: false
  health:
    startup_timeout: 30s
    liveness_interval: 10s
  egress:
    - host: api.github.com
      protocol: https
      port: 443
      purpose: GitHub REST API
`

func TestLoadBytes_RoundTrip(t *testing.T) {
	m, err := manifest.LoadBytes([]byte(exampleManifest))
	require.NoError(t, err)

	assert.Equal(t, manifest.APIVersionV1, m.APIVersion)
	assert.Equal(t, manifest.KindPlugin, m.Kind)
	assert.Equal(t, "github", m.Metadata.Name)
	assert.Equal(t, "0.1.0", m.Metadata.Version)
	assert.Equal(t, "Stateful proxy for GitHub REST API", m.Metadata.Description)
	assert.Equal(t, "zeroroot-ai", m.Metadata.Author)

	assert.Equal(t, manifest.WorkloadClassPlugin, m.Spec.WorkloadClass)
	assert.Equal(t, "process", m.Spec.Runtime)

	require.Len(t, m.Spec.Secrets, 1)
	assert.Equal(t, "cred:github_token", m.Spec.Secrets[0].Name)
	assert.Equal(t, "startup", m.Spec.Secrets[0].Scope)
	assert.Equal(t, "live", m.Spec.Secrets[0].Rotation)
	assert.True(t, m.Spec.Secrets[0].Required)

	require.Len(t, m.Spec.Methods, 1)
	assert.Equal(t, "GetRepository", m.Spec.Methods[0].Name)

	assert.False(t, m.Spec.Policy.SetecRequired)
	assert.Equal(t, 30*time.Second, m.Spec.Health.EffectiveStartupTimeout())
	assert.Equal(t, 10*time.Second, m.Spec.Health.EffectiveLivenessInterval())

	require.Len(t, m.Spec.Egress, 1)
	assert.Equal(t, "api.github.com", m.Spec.Egress[0].Host)
	assert.Equal(t, "https", m.Spec.Egress[0].Protocol)
	assert.Equal(t, 443, m.Spec.Egress[0].Port)
	assert.Equal(t, "GitHub REST API", m.Spec.Egress[0].Purpose)
}

func TestLoad_FromFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "plugin.yaml")
	require.NoError(t, os.WriteFile(path, []byte(exampleManifest), 0o644))

	m, err := manifest.Load(path)
	require.NoError(t, err)
	assert.Equal(t, "github", m.Metadata.Name)
}

func TestLoad_FileNotFound(t *testing.T) {
	_, err := manifest.Load("/no/such/file.yaml")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "open")
}

// TestDefaults verifies that omitted optional fields receive correct defaults.
func TestDefaults(t *testing.T) {
	minimal := `
apiVersion: plugin.gibson.zeroroot.ai/v1
kind: Plugin
metadata:
  name: my-plugin
  version: 1.0.0
spec:
  workload_class: plugin
  methods:
    - name: Ping
`
	m, err := manifest.LoadBytes([]byte(minimal))
	require.NoError(t, err)

	assert.Equal(t, "process", m.Spec.Runtime, "runtime should default to process")
	assert.Equal(t, manifest.DefaultStartupTimeout, m.Spec.Health.EffectiveStartupTimeout())
	assert.Equal(t, manifest.DefaultLivenessInterval, m.Spec.Health.EffectiveLivenessInterval())
}

// --- invalid manifest cases ---

func TestValidate_WrongAPIVersion(t *testing.T) {
	input := strings.ReplaceAll(exampleManifest, manifest.APIVersionV1, "plugin.gibson.zeroroot.ai/v0")
	_, err := manifest.LoadBytes([]byte(input))
	require.Error(t, err)
	assert.True(t, manifest.IsValidationError(err))
	assertViolation(t, err, "apiVersion")
}

func TestValidate_WrongKind(t *testing.T) {
	input := strings.ReplaceAll(exampleManifest, "kind: Plugin", "kind: Tool")
	_, err := manifest.LoadBytes([]byte(input))
	require.Error(t, err)
	assertViolation(t, err, "kind")
}

func TestValidate_WrongWorkloadClass(t *testing.T) {
	input := strings.ReplaceAll(exampleManifest, "workload_class: plugin", "workload_class: tool")
	_, err := manifest.LoadBytes([]byte(input))
	require.Error(t, err)
	assertViolation(t, err, "spec.workload_class")
}

func TestValidate_EmptyName(t *testing.T) {
	input := strings.ReplaceAll(exampleManifest, "name: github", "name: \"\"")
	_, err := manifest.LoadBytes([]byte(input))
	require.Error(t, err)
	assertViolation(t, err, "metadata.name")
}

func TestValidate_InvalidName_UpperCase(t *testing.T) {
	input := strings.ReplaceAll(exampleManifest, "name: github", "name: MyPlugin")
	_, err := manifest.LoadBytes([]byte(input))
	require.Error(t, err)
	assertViolation(t, err, "metadata.name")
}

func TestValidate_InvalidName_TooShort(t *testing.T) {
	// Single character doesn't satisfy ^[a-z][a-z0-9-]{0,61}[a-z0-9]$
	input := strings.ReplaceAll(exampleManifest, "name: github", "name: a")
	_, err := manifest.LoadBytes([]byte(input))
	require.Error(t, err)
	assertViolation(t, err, "metadata.name")
}

func TestValidate_MissingVersion(t *testing.T) {
	input := strings.ReplaceAll(exampleManifest, "version: 0.1.0", "version: \"\"")
	_, err := manifest.LoadBytes([]byte(input))
	require.Error(t, err)
	assertViolation(t, err, "metadata.version")
}

func TestValidate_InvalidSemver(t *testing.T) {
	input := strings.ReplaceAll(exampleManifest, "version: 0.1.0", "version: not-a-version")
	_, err := manifest.LoadBytes([]byte(input))
	require.Error(t, err)
	assertViolation(t, err, "metadata.version")
}

func TestValidate_ValidSemverVariants(t *testing.T) {
	versions := []string{"1.0.0", "0.1.0", "1.2.3-alpha.1", "1.2.3+build.42", "v1.2.3"}
	for _, ver := range versions {
		t.Run(ver, func(t *testing.T) {
			input := strings.ReplaceAll(exampleManifest, "version: 0.1.0", "version: "+ver)
			_, err := manifest.LoadBytes([]byte(input))
			require.NoError(t, err, "version %q should be valid semver", ver)
		})
	}
}

func TestValidate_NoMethods(t *testing.T) {
	// Remove the entire methods block and replace with empty.
	input := `
apiVersion: plugin.gibson.zeroroot.ai/v1
kind: Plugin
metadata:
  name: my-plugin
  version: 1.0.0
spec:
  workload_class: plugin
  methods: []
`
	_, err := manifest.LoadBytes([]byte(input))
	require.Error(t, err)
	assertViolation(t, err, "spec.methods")
}

func TestValidate_MethodMissingName(t *testing.T) {
	input := `
apiVersion: plugin.gibson.zeroroot.ai/v1
kind: Plugin
metadata:
  name: my-plugin
  version: 1.0.0
spec:
  workload_class: plugin
  methods:
    - name: ""
`
	_, err := manifest.LoadBytes([]byte(input))
	require.Error(t, err)
	assertViolation(t, err, "spec.methods[0].name")
}

func TestValidate_InvalidRuntime(t *testing.T) {
	input := strings.ReplaceAll(exampleManifest, "runtime: process", "runtime: docker")
	_, err := manifest.LoadBytes([]byte(input))
	require.Error(t, err)
	assertViolation(t, err, "spec.runtime")
}

func TestValidate_ValidRuntimes(t *testing.T) {
	for _, rt := range []string{"process", "pod", "setec"} {
		t.Run(rt, func(t *testing.T) {
			input := strings.ReplaceAll(exampleManifest, "runtime: process", "runtime: "+rt)
			_, err := manifest.LoadBytes([]byte(input))
			require.NoError(t, err)
		})
	}
}

func TestValidate_InvalidSecretName(t *testing.T) {
	input := strings.ReplaceAll(exampleManifest, "name: cred:github_token", "name: github_token")
	_, err := manifest.LoadBytes([]byte(input))
	require.Error(t, err)
	assertViolation(t, err, "spec.secrets[0].name")
}

func TestValidate_InvalidSecretScope(t *testing.T) {
	input := strings.ReplaceAll(exampleManifest, "scope: startup", "scope: always")
	_, err := manifest.LoadBytes([]byte(input))
	require.Error(t, err)
	assertViolation(t, err, "spec.secrets[0].scope")
}

func TestValidate_InvalidSecretRotation(t *testing.T) {
	input := strings.ReplaceAll(exampleManifest, "rotation: live", "rotation: manual")
	_, err := manifest.LoadBytes([]byte(input))
	require.Error(t, err)
	assertViolation(t, err, "spec.secrets[0].rotation")
}

func TestValidate_SecretNameVariants(t *testing.T) {
	valid := []string{
		"cred:db_password",
		"cred:some/nested/key",
		"provider_config:anthropic:default",
		"cred:a.b.c",
		"cred:x-y-z",
	}
	for _, name := range valid {
		t.Run(name, func(t *testing.T) {
			input := strings.ReplaceAll(exampleManifest, "name: cred:github_token", "name: "+name)
			_, err := manifest.LoadBytes([]byte(input))
			require.NoError(t, err, "secret name %q should be valid", name)
		})
	}
}

func TestValidate_InvalidEgressHost_IP(t *testing.T) {
	input := strings.ReplaceAll(exampleManifest, "host: api.github.com", "host: 1.2.3.4")
	_, err := manifest.LoadBytes([]byte(input))
	require.Error(t, err)
	assertViolation(t, err, "spec.egress[0].host")
}

func TestValidate_InvalidEgressProtocol(t *testing.T) {
	input := strings.ReplaceAll(exampleManifest, "protocol: https", "protocol: ftp")
	_, err := manifest.LoadBytes([]byte(input))
	require.Error(t, err)
	assertViolation(t, err, "spec.egress[0].protocol")
}

func TestValidate_InvalidEgressPort_Zero(t *testing.T) {
	input := strings.ReplaceAll(exampleManifest, "port: 443", "port: 0")
	_, err := manifest.LoadBytes([]byte(input))
	require.Error(t, err)
	assertViolation(t, err, "spec.egress[0].port")
}

func TestValidate_InvalidEgressPort_TooHigh(t *testing.T) {
	input := strings.ReplaceAll(exampleManifest, "port: 443", "port: 70000")
	_, err := manifest.LoadBytes([]byte(input))
	require.Error(t, err)
	assertViolation(t, err, "spec.egress[0].port")
}

func TestValidate_EgressWildcardHost(t *testing.T) {
	input := strings.ReplaceAll(exampleManifest, "host: api.github.com", "host: '*.github.com'")
	_, err := manifest.LoadBytes([]byte(input))
	require.NoError(t, err)
}

func TestValidate_ValidEgressProtocols(t *testing.T) {
	for _, proto := range []string{"https", "http", "grpc", "tcp", "udp"} {
		t.Run(proto, func(t *testing.T) {
			input := strings.ReplaceAll(exampleManifest, "protocol: https", "protocol: "+proto)
			_, err := manifest.LoadBytes([]byte(input))
			require.NoError(t, err)
		})
	}
}

func TestValidate_InvalidDuration(t *testing.T) {
	input := strings.ReplaceAll(exampleManifest, "startup_timeout: 30s", "startup_timeout: not-a-duration")
	_, err := manifest.LoadBytes([]byte(input))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "duration")
}

func TestValidate_MultipleViolations(t *testing.T) {
	// A manifest with several simultaneous errors; all should be reported.
	input := `
apiVersion: plugin.gibson.zeroroot.ai/v0
kind: Tool
metadata:
  name: ""
  version: bad-ver
spec:
  workload_class: agent
  methods: []
`
	_, err := manifest.LoadBytes([]byte(input))
	require.Error(t, err)

	var ve *manifest.ValidationError
	require.ErrorAs(t, err, &ve)
	assert.GreaterOrEqual(t, len(ve.Violations), 4,
		"expected at least 4 violations, got: %v", ve.Violations)
}

// TestUnknownField_LineNumber verifies that an unknown YAML field produces an
// error whose message includes a line number (from yaml.v3's strict decoder).
func TestUnknownField_LineNumber(t *testing.T) {
	input := `
apiVersion: plugin.gibson.zeroroot.ai/v1
kind: Plugin
metadata:
  name: my-plugin
  version: 1.0.0
  unknown_field: oops
spec:
  workload_class: plugin
  methods:
    - name: Ping
`
	_, err := manifest.LoadBytes([]byte(input))
	require.Error(t, err)
	// yaml.v3 includes "line N:" in the error for unknown fields.
	assert.Regexp(t, `line \d+`, err.Error(), "error should include YAML line number")
}

// TestIsValidationError distinguishes validation errors from other errors.
func TestIsValidationError(t *testing.T) {
	t.Run("validation_error", func(t *testing.T) {
		input := strings.ReplaceAll(exampleManifest, "kind: Plugin", "kind: Foo")
		_, err := manifest.LoadBytes([]byte(input))
		require.Error(t, err)
		assert.True(t, manifest.IsValidationError(err))
	})

	t.Run("non_validation_error", func(t *testing.T) {
		_, err := manifest.Load("/does/not/exist")
		require.Error(t, err)
		assert.False(t, manifest.IsValidationError(err))
	})
}

// TestValidationError_Error checks the error string contains all violations.
func TestValidationError_Error(t *testing.T) {
	ve := &manifest.ValidationError{
		Violations: []string{"violation one", "violation two"},
	}
	s := ve.Error()
	assert.Contains(t, s, "violation one")
	assert.Contains(t, s, "violation two")
}

// assertViolation is a helper that checks the error contains the given
// substring in at least one violation message.
func assertViolation(t *testing.T, err error, contains string) {
	t.Helper()
	var ve *manifest.ValidationError
	if !strings.Contains(err.Error(), contains) {
		t.Errorf("expected error containing %q, got: %s", contains, err)
		return
	}
	_ = ve
}

func TestDefaults_ContentTrust(t *testing.T) {
	minimal := `
apiVersion: plugin.gibson.zeroroot.ai/v1
kind: Plugin
metadata:
  name: my-plugin
  version: 1.0.0
spec:
  workload_class: plugin
  methods:
    - name: Ping
`
	m, err := manifest.LoadBytes([]byte(minimal))
	require.NoError(t, err)
	assert.Equal(t, manifest.ContentTrustTrusted, m.Spec.Policy.ContentTrust,
		"content_trust should default to trusted")
}

func TestContentTrust_Untrusted_Parses(t *testing.T) {
	input := `
apiVersion: plugin.gibson.zeroroot.ai/v1
kind: Plugin
metadata:
  name: my-plugin
  version: 1.0.0
spec:
  workload_class: plugin
  policy:
    content_trust: untrusted
  methods:
    - name: Ping
`
	m, err := manifest.LoadBytes([]byte(input))
	require.NoError(t, err)
	assert.Equal(t, manifest.ContentTrustUntrusted, m.Spec.Policy.ContentTrust)
}

func TestValidate_InvalidContentTrust(t *testing.T) {
	input := `
apiVersion: plugin.gibson.zeroroot.ai/v1
kind: Plugin
metadata:
  name: my-plugin
  version: 1.0.0
spec:
  workload_class: plugin
  policy:
    content_trust: bogus
  methods:
    - name: Ping
`
	_, err := manifest.LoadBytes([]byte(input))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "content_trust")
}
