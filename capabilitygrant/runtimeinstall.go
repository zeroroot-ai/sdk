// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Zero Root AI

package capabilitygrant

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// Runtime *install* resolution (ADR-0045 lifecycle A). RuntimeCredential is the
// raw signing material; a RuntimeInstall additionally carries the platform dial
// target and is the on-disk record `gibson component register` writes. This is
// the canonical home for the layout so every consumer — `agent.Connect`,
// `gibson inspect`, the adk CLI — reads and writes the SAME files.
//
// Backends (decision A): a relocatable 0600 file by default, overridable by the
// GIBSON_AGENT_KEY env var (base64(JSON) RuntimeCredential) with the dial target
// in GIBSON_URL, for CI / k8s-Secret-as-env. A missing credential is a hard
// error — there is no auto re-register.

// EnvRuntimeCredential carries a base64(JSON) RuntimeCredential for headless
// contexts (CI, k8s Secret → env).
const EnvRuntimeCredential = "GIBSON_AGENT_KEY"

// EnvGibsonURL is the dial-target fallback used with EnvRuntimeCredential.
const EnvGibsonURL = "GIBSON_URL"

// EnvGibsonHome relocates the config dir (default ~/.gibson).
const EnvGibsonHome = "GIBSON_HOME"

// RuntimeInstall is the on-disk record produced by register: the runtime signing
// material plus the platform dial target. Its JSON shape is stable and shared
// across the SDK and the adk CLI.
type RuntimeInstall struct {
	GibsonURL  string            `json:"gibson_url"`
	Credential RuntimeCredential `json:"credential"`
}

// GibsonDir returns the base config dir: $GIBSON_HOME when set, else ~/.gibson.
func GibsonDir() (string, error) {
	if h := os.Getenv(EnvGibsonHome); h != "" {
		return h, nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("capabilitygrant: resolve home dir: %w", err)
	}
	return filepath.Join(home, ".gibson"), nil
}

func installFilename(name, suffix string) string {
	base := "default"
	if name != "" {
		base = name
	}
	return base + suffix
}

// RuntimeInstallPath returns <GibsonDir>/<kind>/<name>.runtime.json.
func RuntimeInstallPath(kind, name string) (string, error) {
	dir, err := GibsonDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, kind, installFilename(name, ".runtime.json")), nil
}

// SaveRuntimeInstall writes the install record atomically with 0600 permissions.
func SaveRuntimeInstall(kind, name string, install RuntimeInstall) (string, error) {
	if err := install.Credential.Valid(); err != nil {
		return "", err
	}
	path, err := RuntimeInstallPath(kind, name)
	if err != nil {
		return "", err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return "", fmt.Errorf("capabilitygrant: create runtime-install dir: %w", err)
	}
	data, err := json.MarshalIndent(install, "", "  ")
	if err != nil {
		return "", fmt.Errorf("capabilitygrant: marshal runtime install: %w", err)
	}
	tmp, err := os.CreateTemp(filepath.Dir(path), ".runtime-*.tmp")
	if err != nil {
		return "", fmt.Errorf("capabilitygrant: temp runtime install: %w", err)
	}
	tmpName := tmp.Name()
	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		os.Remove(tmpName)
		return "", fmt.Errorf("capabilitygrant: write runtime install: %w", err)
	}
	if err := tmp.Close(); err != nil {
		os.Remove(tmpName)
		return "", err
	}
	if err := os.Chmod(tmpName, 0o600); err != nil {
		os.Remove(tmpName)
		return "", err
	}
	if err := os.Rename(tmpName, path); err != nil {
		os.Remove(tmpName)
		return "", fmt.Errorf("capabilitygrant: rename runtime install: %w", err)
	}
	return path, nil
}

// InstallRef identifies a registered component install on this host.
type InstallRef struct {
	Kind string
	Name string
}

const runtimeSuffix = ".runtime.json"

// ListInstalls scans <GibsonDir>/{agent,tool,plugin}/*.runtime.json.
func ListInstalls() ([]InstallRef, error) {
	dir, err := GibsonDir()
	if err != nil {
		return nil, err
	}
	var out []InstallRef
	for _, kind := range []string{"agent", "tool", "plugin"} {
		entries, err := os.ReadDir(filepath.Join(dir, kind))
		if os.IsNotExist(err) {
			continue
		}
		if err != nil {
			continue
		}
		for _, e := range entries {
			if e.IsDir() || !strings.HasSuffix(e.Name(), runtimeSuffix) {
				continue
			}
			out = append(out, InstallRef{Kind: kind, Name: strings.TrimSuffix(e.Name(), runtimeSuffix)})
		}
	}
	return out, nil
}

// ResolveRuntimeInstall returns the runtime credential + dial URL for a
// registered component, honouring the env override before the file (decision
// A). A missing credential is a hard error (no auto re-register).
//
//   - GIBSON_AGENT_KEY set → base64(JSON) RuntimeCredential; URL from GIBSON_URL.
//   - else → <GibsonDir>/<kind>/<name>.runtime.json.
func ResolveRuntimeInstall(kind, name string) (RuntimeInstall, error) {
	if env := os.Getenv(EnvRuntimeCredential); env != "" {
		rc, err := DecodeRuntimeCredentialBase64(env)
		if err != nil {
			return RuntimeInstall{}, fmt.Errorf("capabilitygrant: %s: %w", EnvRuntimeCredential, err)
		}
		if err := rc.Valid(); err != nil {
			return RuntimeInstall{}, err
		}
		url := os.Getenv(EnvGibsonURL)
		if url == "" {
			return RuntimeInstall{}, fmt.Errorf("capabilitygrant: %s is set but %s is empty (need the dial target)", EnvRuntimeCredential, EnvGibsonURL)
		}
		return RuntimeInstall{GibsonURL: url, Credential: rc}, nil
	}

	path, err := RuntimeInstallPath(kind, name)
	if err != nil {
		return RuntimeInstall{}, err
	}
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return RuntimeInstall{}, fmt.Errorf("capabilitygrant: no runtime credential at %s — run `gibson component register` first (ADR-0045: register persists it, runtime reuses it)", path)
		}
		return RuntimeInstall{}, fmt.Errorf("capabilitygrant: read runtime install: %w", err)
	}
	var install RuntimeInstall
	if err := json.Unmarshal(data, &install); err != nil {
		return RuntimeInstall{}, fmt.Errorf("capabilitygrant: parse runtime install %s: %w", path, err)
	}
	if err := install.Credential.Valid(); err != nil {
		return RuntimeInstall{}, fmt.Errorf("capabilitygrant: runtime install %s incomplete: %w", path, err)
	}
	return install, nil
}
