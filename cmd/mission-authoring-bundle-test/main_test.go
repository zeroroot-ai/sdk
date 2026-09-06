// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Zero Root AI

// Package main_test is the bundle round-trip integration test for
// mission-authoring-cue. It runs `make mission-authoring-bundle`
// from the SDK root, extracts the produced tarball into a temp
// directory, and asserts each artifact family parses cleanly.
//
// Spec: mission-authoring-cue Requirement 2 + 18.
package main_test

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"encoding/json"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// sdkRoot resolves the SDK module root by walking up from the test
// file's directory until go.mod is found.
func sdkRoot(t *testing.T) string {
	t.Helper()
	dir, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatalf("could not find SDK root from %s", dir)
		}
		dir = parent
	}
}

func TestBundleRoundTrip(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping bundle round-trip in -short mode")
	}
	root := sdkRoot(t)

	// Build the bundle — invokes the same make target the publish
	// workflow uses on tag.
	cmd := exec.Command("make", "mission-authoring-bundle")
	cmd.Dir = root
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("make mission-authoring-bundle failed: %v\n%s", err, out)
	}

	tarPath := filepath.Join(root, "gen", "mission-authoring-bundle.tar.gz")
	stat, err := os.Stat(tarPath)
	if err != nil {
		t.Fatalf("expected %s to exist: %v", tarPath, err)
	}
	if stat.Size() < 1024 {
		t.Errorf("bundle suspiciously small: %d bytes", stat.Size())
	}

	// Extract and inventory the artifacts.
	extractDir := t.TempDir()
	if err := extractTarGz(tarPath, extractDir); err != nil {
		t.Fatalf("extract: %v", err)
	}

	got, err := inventory(extractDir)
	if err != nil {
		t.Fatalf("inventory: %v", err)
	}

	want := []string{
		"mission-definition.schema.json",
		"glossary.json",
		"docs/verbs.mdx",
		"docs/nouns.mdx",
		"docs/schema-ref.mdx",
		"docs/templates.mdx",
	}
	for _, p := range want {
		if !contains(got, p) {
			t.Errorf("bundle missing %q (have: %v)", p, got)
		}
	}

	// Validate JSON Schema parses as JSON.
	schema, err := os.ReadFile(filepath.Join(extractDir, "mission-definition.schema.json"))
	if err != nil {
		t.Fatalf("read schema: %v", err)
	}
	var schemaDoc map[string]any
	if err := json.Unmarshal(schema, &schemaDoc); err != nil {
		t.Errorf("schema JSON parse failed: %v", err)
	}
	if schemaDoc["$schema"] != "https://json-schema.org/draft/2020-12/schema" {
		t.Errorf("schema $schema=%v want draft 2020-12", schemaDoc["$schema"])
	}
	if _, ok := schemaDoc["$defs"].(map[string]any); !ok {
		t.Errorf("schema missing $defs")
	}

	// Validate glossary parses.
	gloss, err := os.ReadFile(filepath.Join(extractDir, "glossary.json"))
	if err != nil {
		t.Fatalf("read glossary: %v", err)
	}
	var glossDoc map[string]string
	if err := json.Unmarshal(gloss, &glossDoc); err != nil {
		t.Errorf("glossary JSON parse failed: %v", err)
	}
	if len(glossDoc) < 5 {
		t.Errorf("glossary has only %d entries; expected dozens", len(glossDoc))
	}

	// Validate MDX files have non-empty content with the expected
	// frontmatter shape.
	for _, mdx := range []string{"verbs.mdx", "nouns.mdx", "schema-ref.mdx", "templates.mdx"} {
		body, err := os.ReadFile(filepath.Join(extractDir, "docs", mdx))
		if err != nil {
			t.Errorf("read %s: %v", mdx, err)
			continue
		}
		if !strings.HasPrefix(string(body), "---\n") {
			t.Errorf("%s missing MDX frontmatter", mdx)
		}
		if len(body) < 100 {
			t.Errorf("%s suspiciously small (%d bytes)", mdx, len(body))
		}
	}
}

func extractTarGz(path, dest string) error {
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer func() { _ = f.Close() }()
	gz, err := gzip.NewReader(f)
	if err != nil {
		return err
	}
	defer func() { _ = gz.Close() }()
	tr := tar.NewReader(gz)
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			return nil
		}
		if err != nil {
			return err
		}
		full := filepath.Join(dest, hdr.Name)
		switch hdr.Typeflag {
		case tar.TypeDir:
			if err := os.MkdirAll(full, 0o755); err != nil {
				return err
			}
		case tar.TypeReg:
			if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
				return err
			}
			out, err := os.Create(full)
			if err != nil {
				return err
			}
			if _, err := io.Copy(out, tr); err != nil {
				_ = out.Close()
				return err
			}
			if err := out.Close(); err != nil {
				return err
			}
		}
	}
}

func inventory(root string) ([]string, error) {
	var paths []string
	err := filepath.Walk(root, func(p string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			return nil
		}
		rel, err := filepath.Rel(root, p)
		if err != nil {
			return err
		}
		paths = append(paths, filepath.ToSlash(rel))
		return nil
	})
	return paths, err
}

func contains(xs []string, want string) bool {
	for _, x := range xs {
		if x == want {
			return true
		}
	}
	return false
}

// Compile-time assertion that bytes/json are imported (silences
// linters when the test body changes).
var (
	_ = bytes.NewReader
	_ = json.Marshal
)
