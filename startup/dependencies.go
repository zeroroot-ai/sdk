// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Zero Root AI

// Package startup provides pre-registration validation for Gibson tool
// system dependencies. It parses component.yaml manifests and validates
// that required system binaries are present in PATH before the tool
// connects to the platform.
package startup

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"strings"
	"sync"
	"time"

	"golang.org/x/sync/errgroup"
	"gopkg.in/yaml.v3"

	"github.com/zeroroot-ai/sdk/health"
)

// SystemDependency represents a system binary dependency from component.yaml.
// It supports both short string format ("- mytool") and extended struct format
// ("- name: mytool\n  required: true\n  version_command: --version").
type SystemDependency struct {
	Name           string `yaml:"name"`
	Required       *bool  `yaml:"required,omitempty"`        // nil = true (default)
	VersionCommand string `yaml:"version_command,omitempty"` // default: "--version"
}

// IsRequired returns whether this dependency is required.
// Defaults to true when Required is nil (short string format or omitted).
func (d SystemDependency) IsRequired() bool {
	if d.Required == nil {
		return true
	}
	return *d.Required
}

// GetVersionCommand returns the version flag, defaulting to "--version".
func (d SystemDependency) GetVersionCommand() string {
	if d.VersionCommand == "" {
		return "--version"
	}
	return d.VersionCommand
}

// UnmarshalYAML handles both string and struct formats for system dependencies.
// String format: "- mytool" sets Name=mytool with defaults (required=true, version_command="--version").
// Struct format: "- name: mytool\n  required: true" decodes all fields.
func (d *SystemDependency) UnmarshalYAML(value *yaml.Node) error {
	// Short string format: "- mytool"
	if value.Kind == yaml.ScalarNode {
		d.Name = value.Value
		return nil
	}
	// Extended struct format: use type alias to avoid infinite recursion
	type raw SystemDependency
	return value.Decode((*raw)(d))
}

// ComponentManifest is a minimal parse of component.yaml for dependency extraction.
type ComponentManifest struct {
	Kind         string       `yaml:"kind"`
	Name         string       `yaml:"name"`
	Dependencies Dependencies `yaml:"dependencies"`
}

// Dependencies holds the dependency declarations from component.yaml.
type Dependencies struct {
	Gibson string             `yaml:"gibson"`
	System []SystemDependency `yaml:"system"`
}

// ValidationResult contains the results of system dependency validation.
type ValidationResult struct {
	Found    []BinaryInfo    // Binaries found with path and version
	Missing  []MissingBinary // Required binaries not found (causes error)
	Degraded []MissingBinary // Optional binaries not found (causes warning)
}

// BinaryInfo describes a system binary that was found and validated.
type BinaryInfo struct {
	Name    string `json:"binary"`
	Path    string `json:"path"`
	Version string `json:"version"`
}

// MissingBinary describes a system binary that was not found in PATH.
type MissingBinary struct {
	Name     string
	Required bool
	PATH     string
}

// ValidateSystemDependencies parses component.yaml and validates all system
// binary dependencies. Returns an error if any required binary is missing.
//
// This function:
//  1. Parses component.yaml from the given path
//  2. Validates each binary exists using health.BinaryCheck (no process spawn)
//  3. Runs version commands in parallel (5s per-binary timeout, 10s total)
//  4. Logs structured version info for each found binary
//  5. Returns an error listing ALL missing required binaries (not just the first)
func ValidateSystemDependencies(logger *slog.Logger, manifestPath string) (*ValidationResult, error) {
	// Load and parse component.yaml
	data, err := os.ReadFile(manifestPath)
	if err != nil {
		return nil, fmt.Errorf("fatal: component.yaml not found at %s: %w", manifestPath, err)
	}

	var manifest ComponentManifest
	if err := yaml.Unmarshal(data, &manifest); err != nil {
		return nil, fmt.Errorf("fatal: failed to parse component.yaml: %w", err)
	}

	if len(manifest.Dependencies.System) == 0 {
		logger.Warn("no system dependencies declared in component.yaml -- skipping binary checks")
		return &ValidationResult{}, nil
	}

	// Log the PATH being searched
	logger.Info("validating system dependencies",
		"component", manifest.Name,
		"path", os.Getenv("PATH"),
		"dependency_count", len(manifest.Dependencies.System),
	)

	result := &ValidationResult{}
	var missingRequired []MissingBinary

	// Phase 1: Check binary existence (no subprocess, fast)
	for _, dep := range manifest.Dependencies.System {
		status := health.BinaryCheck(dep.Name)
		if status.IsUnhealthy() {
			mb := MissingBinary{
				Name:     dep.Name,
				Required: dep.IsRequired(),
				PATH:     os.Getenv("PATH"),
			}
			if dep.IsRequired() {
				missingRequired = append(missingRequired, mb)
			} else {
				result.Degraded = append(result.Degraded, mb)
				logger.Warn("optional binary not found in PATH -- some capabilities may be unavailable",
					"binary", dep.Name,
				)
			}
		} else {
			path, _ := exec.LookPath(dep.Name)
			result.Found = append(result.Found, BinaryInfo{
				Name: dep.Name,
				Path: path,
			})
		}
	}

	// If any required binaries are missing, report ALL of them
	if len(missingRequired) > 0 {
		result.Missing = missingRequired
		var names []string
		for _, m := range missingRequired {
			names = append(names, m.Name)
		}
		errMsg := fmt.Sprintf(
			"fatal: required binary(ies) not found in PATH: [%s]. "+
				"Ensure the binaries are installed in the container image. "+
				"Check the Dockerfile for the tool's base image. PATH=%s",
			strings.Join(names, ", "),
			os.Getenv("PATH"),
		)
		// Also write to stderr for kubectl logs visibility
		fmt.Fprintln(os.Stderr, errMsg)
		return result, fmt.Errorf("%s", errMsg)
	}

	// Phase 2: Get versions for found binaries (parallel, informational)
	getVersions(logger, manifest.Dependencies.System, result)

	return result, nil
}

// getVersions runs version commands in parallel with a combined 10s timeout.
// Version check failures are non-fatal -- they are logged as warnings.
// errgroup.WithContext is used so that cancellation of the 10s deadline propagates
// into per-binary version commands. The mutex protects concurrent writes to
// result.Found, which is orthogonal to the cancellation coordination.
func getVersions(logger *slog.Logger, deps []SystemDependency, result *ValidationResult) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Build an index of found binaries → slice index before spawning goroutines.
	// This avoids concurrent reads/writes to result.Found (goroutines only write
	// their own pre-assigned index; the outer loop reads result.Found before
	// any goroutine is started).
	type foundEntry struct {
		dep   SystemDependency
		index int
	}
	var toVersion []foundEntry
	for _, dep := range deps {
		for j, f := range result.Found {
			if f.Name == dep.Name {
				toVersion = append(toVersion, foundEntry{dep: dep, index: j})
				break
			}
		}
	}

	var mu sync.Mutex
	eg, gctx := errgroup.WithContext(ctx)

	for _, entry := range toVersion {
		// capture loop variable
		eg.Go(func() error {
			versionFlag := entry.dep.GetVersionCommand()
			path, _ := exec.LookPath(entry.dep.Name)

			vCtx, vCancel := context.WithTimeout(gctx, 5*time.Second)
			defer vCancel()

			// Execute directly -- no shell expansion (security)
			cmd := exec.CommandContext(vCtx, path, versionFlag)
			output, err := cmd.CombinedOutput()

			if err != nil {
				logger.Warn("unable to determine version",
					"binary", entry.dep.Name,
					"version_command", versionFlag,
					"error", err,
				)
				// Version failures are non-fatal; return nil so errgroup
				// continues processing remaining binaries.
				return nil
			}

			// Parse first line of version output
			version := strings.TrimSpace(strings.SplitN(string(output), "\n", 2)[0])

			// Update the pre-resolved Found entry with version info.
			// Mutex protects concurrent index writes.
			mu.Lock()
			result.Found[entry.index].Version = version
			mu.Unlock()

			logger.Info("system dependency verified",
				"binary", entry.dep.Name,
				"version", version,
				"path", path,
			)
			return nil
		})
	}

	// Ignore error: goroutines always return nil; errgroup is for cancellation propagation.
	_ = eg.Wait()
}
