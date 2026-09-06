// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Zero Root AI

// Package main implements the sdk-bump CLI tool.
package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// Consumer describes a downstream repository that pins the SDK.
type Consumer struct {
	// Name is the short name of the repo (matches the GitHub repo name after the org slash).
	Name string
	// GitRepo is the full "org/repo" slug, e.g. "zeroroot-ai/gibson".
	GitRepo string
	// GoModule indicates whether this is a Go module that needs `go get` + `go mod tidy`.
	GoModule bool
	// PostBump lists commands run inside the cloned repo after the SDK pin is updated.
	// Each element is a full shell-style command string; it is split on spaces and passed to
	// exec.Command — no shell interpolation, no quoting needed.
	PostBump []string
}

// CONSUMERS is the authoritative list of repos that pin github.com/zeroroot-ai/sdk.
// Verify paths under ~/Code/zeroroot.ai/ before adding or removing entries.
var CONSUMERS = []Consumer{
	{
		Name:     "gibson",
		GitRepo:  "zeroroot-ai/gibson",
		GoModule: true,
		PostBump: []string{
			"go mod tidy",
			"make proto",
			"go build ./...",
			"go test -short ./...",
		},
	},
	{
		Name:     "adk",
		GitRepo:  "zeroroot-ai/adk",
		GoModule: true,
		PostBump: []string{
			"go mod tidy",
			"go build ./...",
			"go test -short ./...",
		},
	},
	{
		Name:     "gibson-executor",
		GitRepo:  "zeroroot-ai/gibson-executor",
		GoModule: true,
		PostBump: []string{
			"go mod tidy",
			"go build ./...",
			"go test -short ./...",
		},
	},
	{
		Name:     "dashboard",
		GitRepo:  "zeroroot-ai/dashboard",
		GoModule: false,
		PostBump: []string{
			"pnpm install --no-frozen-lockfile",
			"npx buf generate",
			"pnpm typecheck",
		},
	},
}

// validateLocalPaths checks each consumer's local checkout (if present at workspaceRoot).
// Missing dirs produce warnings but are not fatal — the clone step will handle it.
func validateLocalPaths(workspaceRoot string, consumers []Consumer) {
	for _, c := range consumers {
		// Map from GitRepo "zeroroot-ai/gibson" → local path.
		// Known mappings based on the polyrepo workspace layout.
		localPath := localCheckoutPath(workspaceRoot, c)
		if localPath == "" {
			continue
		}
		if _, err := os.Stat(localPath); os.IsNotExist(err) {
			fmt.Fprintf(os.Stderr, "[sdk-bump] WARNING: consumer %q local checkout not found at %s — CONSUMERS list may be stale\n",
				c.Name, localPath)
		}
	}
}

// localCheckoutPath returns the expected local path for a consumer given the workspace root,
// or "" if the mapping is unknown.
func localCheckoutPath(workspaceRoot string, c Consumer) string {
	// Mapping derived from CLAUDE.md workspace layout.
	repoToLocal := map[string]string{
		"zeroroot-ai/gibson":          filepath.Join(workspaceRoot, "enterprise", "platform", "gibson"),
		"zeroroot-ai/adk":             filepath.Join(workspaceRoot, "opensource", "adk"),
		"zeroroot-ai/gibson-executor": filepath.Join(workspaceRoot, "opensource", "gibson-executor"),
		"zeroroot-ai/dashboard":       filepath.Join(workspaceRoot, "enterprise", "platform", "dashboard"),
	}
	return repoToLocal[c.GitRepo]
}

// filterConsumers returns the subset of CONSUMERS whose Name is in names.
// If names is empty, all consumers are returned.
func filterConsumers(all []Consumer, names []string) ([]Consumer, error) {
	if len(names) == 0 {
		return all, nil
	}
	nameSet := make(map[string]bool, len(names))
	for _, n := range names {
		nameSet[strings.TrimSpace(n)] = true
	}
	var out []Consumer
	for _, c := range all {
		if nameSet[c.Name] {
			out = append(out, c)
			delete(nameSet, c.Name)
		}
	}
	if len(nameSet) > 0 {
		unknown := make([]string, 0, len(nameSet))
		for n := range nameSet {
			unknown = append(unknown, n)
		}
		return nil, fmt.Errorf("unknown consumers: %s", strings.Join(unknown, ", "))
	}
	return out, nil
}
