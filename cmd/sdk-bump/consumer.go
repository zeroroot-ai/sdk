// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Zero Root AI

// Package main implements the sdk-bump CLI tool.
package main

import (
	"fmt"
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
