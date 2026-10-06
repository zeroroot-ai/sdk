// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Zero Root AI

package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestRunWritesConstants(t *testing.T) {
	out := filepath.Join(t.TempDir(), "constants_generated.go")
	if err := run([]string{"--base", "../../taxonomy/core.yaml", "--output-constants", out}); err != nil {
		t.Fatalf("run: %v", err)
	}
	if fi, err := os.Stat(out); err != nil || fi.Size() == 0 {
		t.Fatalf("no constants written: %v", err)
	}
}

func TestRunErrors(t *testing.T) {
	dir := t.TempDir()
	cases := map[string][]string{
		"no flags":     nil,
		"bad flag":     {"--nope"},
		"missing yaml": {"--base", filepath.Join(dir, "none.yaml"), "--output-constants", filepath.Join(dir, "c.go")},
		"bad output":   {"--base", "../../taxonomy/core.yaml", "--output-constants", filepath.Join(dir, "missing", "c.go")},
	}
	for name, args := range cases {
		if err := run(args); err == nil {
			t.Errorf("%s: want an error", name)
		}
	}
}
