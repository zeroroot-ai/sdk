// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Zero Root AI

package startup

import (
	"log/slog"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestIntegration_RealBinary(t *testing.T) {
	// Write a temp component.yaml referencing "ls" which is universally available
	dir := t.TempDir()
	manifestPath := filepath.Join(dir, "component.yaml")
	err := os.WriteFile(manifestPath, []byte(`
kind: tool
name: integration-test-tool
dependencies:
  system:
    - name: ls
      required: true
      version_command: "--version"
`), 0644)
	require.NoError(t, err)

	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelDebug}))

	result, err := ValidateSystemDependencies(logger, manifestPath)
	require.NoError(t, err, "ls should be found on all systems")
	require.NotNil(t, result)

	// Verify the binary was found
	require.Len(t, result.Found, 1)
	assert.Equal(t, "ls", result.Found[0].Name)
	assert.NotEmpty(t, result.Found[0].Path, "ls should have a path")

	// Version may or may not be populated depending on ls implementation,
	// but on GNU systems it typically outputs version info
	// We just verify the flow completed without error

	assert.Empty(t, result.Missing, "no binaries should be missing")
	assert.Empty(t, result.Degraded, "no binaries should be degraded")
}

func TestIntegration_MissingBinary(t *testing.T) {
	// Write a temp component.yaml referencing a nonexistent binary
	dir := t.TempDir()
	manifestPath := filepath.Join(dir, "component.yaml")
	err := os.WriteFile(manifestPath, []byte(`
kind: tool
name: missing-binary-test
dependencies:
  system:
    - name: nonexistent_binary_xyz
      required: true
`), 0644)
	require.NoError(t, err)

	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelDebug}))

	result, err := ValidateSystemDependencies(logger, manifestPath)
	require.Error(t, err, "should fail for missing binary")
	assert.Contains(t, err.Error(), "nonexistent_binary_xyz")
	assert.Contains(t, err.Error(), "PATH=")

	require.NotNil(t, result)
	require.Len(t, result.Missing, 1)
	assert.Equal(t, "nonexistent_binary_xyz", result.Missing[0].Name)
	assert.True(t, result.Missing[0].Required)
}

func TestIntegration_MixedRequiredAndOptional(t *testing.T) {
	// Test a component with both a real binary and a missing optional one
	dir := t.TempDir()
	manifestPath := filepath.Join(dir, "component.yaml")
	err := os.WriteFile(manifestPath, []byte(`
kind: tool
name: mixed-test
dependencies:
  system:
    - name: ls
      required: true
      version_command: "--version"
    - name: nonexistent_optional_binary
      required: false
`), 0644)
	require.NoError(t, err)

	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelDebug}))

	result, err := ValidateSystemDependencies(logger, manifestPath)
	require.NoError(t, err, "should succeed since only optional binary is missing")
	require.NotNil(t, result)

	// ls should be found
	require.Len(t, result.Found, 1)
	assert.Equal(t, "ls", result.Found[0].Name)

	// Optional binary should be degraded
	require.Len(t, result.Degraded, 1)
	assert.Equal(t, "nonexistent_optional_binary", result.Degraded[0].Name)
	assert.False(t, result.Degraded[0].Required)

	// No required binaries should be missing
	assert.Empty(t, result.Missing)
}

func TestIntegration_ComponentYAMLDiscovery(t *testing.T) {
	// Create a temp directory with a component.yaml
	dir := t.TempDir()
	componentPath := filepath.Join(dir, "component.yaml")
	err := os.WriteFile(componentPath, []byte(`
kind: tool
name: discovery-test
dependencies:
  system:
    - name: ls
      required: true
`), 0644)
	require.NoError(t, err)

	// Verify the file exists at the expected path
	_, err = os.Stat(componentPath)
	require.NoError(t, err, "component.yaml should exist in temp directory")

	// Verify we can parse it
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelDebug}))
	result, err := ValidateSystemDependencies(logger, componentPath)
	require.NoError(t, err)
	require.NotNil(t, result)
	require.Len(t, result.Found, 1)
	assert.Equal(t, "ls", result.Found[0].Name)
}

func TestIntegration_BackwardCompatibleStringFormat(t *testing.T) {
	// Verify the old string format still works end-to-end with real binaries
	dir := t.TempDir()
	manifestPath := filepath.Join(dir, "component.yaml")
	err := os.WriteFile(manifestPath, []byte(`
kind: tool
name: legacy-format-test
dependencies:
  gibson: ">=1.0.0"
  system:
    - ls
`), 0644)
	require.NoError(t, err)

	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelDebug}))

	result, err := ValidateSystemDependencies(logger, manifestPath)
	require.NoError(t, err, "legacy string format should work")
	require.NotNil(t, result)
	require.Len(t, result.Found, 1)
	assert.Equal(t, "ls", result.Found[0].Name)
	assert.NotEmpty(t, result.Found[0].Path)
}
