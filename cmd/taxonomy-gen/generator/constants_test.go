// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Zero Root AI

package generator

import (
	"bytes"
	"go/format"
	"os"
	"path/filepath"
	"testing"

	"github.com/zeroroot-ai/sdk/cmd/taxonomy-gen/parser"
)

// TestGenerateConstantsMatchesCommitted renders the constants from core.yaml
// and compares them with the committed graphrag/constants_generated.go.
func TestGenerateConstantsMatchesCommitted(t *testing.T) {
	tax, err := parser.ParseYAML("../../../taxonomy/core.yaml")
	if err != nil {
		t.Fatalf("parse core.yaml: %v", err)
	}
	live, _, _ := tax.Retire()

	out := filepath.Join(t.TempDir(), "constants_generated.go")
	if err := GenerateConstants(live, out, "graphrag"); err != nil {
		t.Fatalf("GenerateConstants: %v", err)
	}
	got, err := os.ReadFile(out) //nolint:gosec // out is a path in t.TempDir
	if err != nil {
		t.Fatal(err)
	}
	got, err = format.Source(got)
	if err != nil {
		t.Fatalf("generated code does not parse: %v", err)
	}
	want, err := os.ReadFile("../../../graphrag/constants_generated.go")
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, want) {
		t.Fatal("graphrag/constants_generated.go differs from the generator output; run make taxonomy-gen")
	}
}

// TestGenerateConstantsWriteError covers the write failure path.
func TestGenerateConstantsWriteError(t *testing.T) {
	tax, err := parser.ParseYAML("../../../taxonomy/core.yaml")
	if err != nil {
		t.Fatalf("parse core.yaml: %v", err)
	}
	if err := GenerateConstants(tax, filepath.Join(t.TempDir(), "missing", "x.go"), "graphrag"); err == nil {
		t.Fatal("want an error for a path in a missing directory")
	}
}
