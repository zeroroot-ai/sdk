// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Zero Root AI

package cueschemas_test

import (
	"io/fs"
	"strings"
	"testing"

	"github.com/zeroroot-ai/sdk/cueschemas"
)

// TestSchemasReadable verifies that the embedded CUE module tree contains the
// files that are required for the daemon's mission-definition validator to work
// without a subprocess or runtime download.
//
// Spec: sdk#147.
func TestSchemasReadable(t *testing.T) {
	required := []string{
		"cue.mod/module.cue",
		"api/proto/gibson/mission/v1/mission_definition_proto_gen.cue",
		"api/proto/gibson/types/v1/types_proto_gen.cue",
		"api/proto/gibson/job/v1/job_proto_gen.cue",
		"api/proto/gibson/common/v1/gibson_common_proto_gen.cue",
	}

	for _, path := range required {
		t.Run(path, func(t *testing.T) {
			f, err := cueschemas.Schemas.Open(path)
			if err != nil {
				t.Fatalf("Open(%q): %v", path, err)
			}
			defer f.Close()

			info, err := f.Stat()
			if err != nil {
				t.Fatalf("Stat(%q): %v", path, err)
			}
			if info.Size() == 0 {
				t.Fatalf("file %q is empty — embedded tree is incomplete", path)
			}
		})
	}
}

// TestSchemasModuleDeclaration verifies that the embedded cue.mod/module.cue
// declares the correct module path so that import resolution inside the CUE
// loader works correctly.
func TestSchemasModuleDeclaration(t *testing.T) {
	data, err := fs.ReadFile(cueschemas.Schemas, "cue.mod/module.cue")
	if err != nil {
		t.Fatalf("ReadFile(cue.mod/module.cue): %v", err)
	}

	const wantModule = `"github.com/zeroroot-ai/sdk@v1"`
	content := string(data)
	if !strings.Contains(content, wantModule) {
		t.Errorf("cue.mod/module.cue does not contain module declaration %q\ngot:\n%s", wantModule, content)
	}
}

// TestSchemasMissionDefinitionNonEmpty verifies that the mission definition CUE
// file is non-trivial (more than 100 bytes), catching accidental empty-file
// embed bugs at the byte level.
func TestSchemasMissionDefinitionNonEmpty(t *testing.T) {
	data, err := fs.ReadFile(cueschemas.Schemas, "api/proto/gibson/mission/v1/mission_definition_proto_gen.cue")
	if err != nil {
		t.Fatalf("ReadFile(mission_definition_proto_gen.cue): %v", err)
	}
	const minBytes = 100
	if len(data) < minBytes {
		t.Errorf("mission_definition_proto_gen.cue is suspiciously small (%d bytes, want >=%d)", len(data), minBytes)
	}
}

// TestSchemasNoStaleModulePaths verifies that none of the embedded CUE files
// reference the pre-rebrand module path (github.com/zero-day-ai/sdk). This
// catches regressions where cue-defs regeneration uses stale tooling.
func TestSchemasNoStaleModulePaths(t *testing.T) {
	const stale = "github.com/zero-day-ai/sdk"
	err := fs.WalkDir(cueschemas.Schemas, ".", func(p string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() || !strings.HasSuffix(p, ".cue") {
			return err
		}
		data, err := fs.ReadFile(cueschemas.Schemas, p)
		if err != nil {
			return err
		}
		if strings.Contains(string(data), stale) {
			t.Errorf("%s contains stale module path %q", p, stale)
		}
		return nil
	})
	if err != nil {
		t.Fatalf("WalkDir: %v", err)
	}
}

// TestSchemasCueMod verifies the cue.mod directory structure is intact by
// listing its entries.
func TestSchemasCueMod(t *testing.T) {
	entries, err := fs.ReadDir(cueschemas.Schemas, "cue.mod")
	if err != nil {
		t.Fatalf("ReadDir(cue.mod): %v", err)
	}
	if len(entries) == 0 {
		t.Fatal("cue.mod directory is empty — embedded tree is broken")
	}
	names := make(map[string]bool, len(entries))
	for _, e := range entries {
		names[e.Name()] = true
	}
	if !names["module.cue"] {
		t.Errorf("cue.mod/module.cue not found among %v", entries)
	}
}
