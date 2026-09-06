// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Zero Root AI

package plugin

import (
	"testing"

	"github.com/zeroroot-ai/sdk/plugin/manifest"
)

// buildMethodMetadata must carry per-method descriptions (the thing the
// connector catalog / SearchTools surface) and keep names + descriptors aligned,
// declared methods first then discovered.
func TestBuildMethodMetadata(t *testing.T) {
	declared := []manifest.MethodDecl{
		{Name: "Echo", Description: "echoes the input"},
	}
	discovered := []DiscoveredMethod{
		{Name: "create_issue", Description: "open an issue"},
		{Name: "list_issues", Description: "list issues"},
	}

	schemas := map[string]methodSchema{
		"Echo": {input: `{"type":"object","properties":{"msg":{"type":"string"}}}`, output: `{"type":"string"}`},
	}
	names, detailed := buildMethodMetadata(declared, discovered, schemas)

	wantNames := []string{"Echo", "create_issue", "list_issues"}
	if len(names) != len(wantNames) {
		t.Fatalf("names = %v, want %v", names, wantNames)
	}
	for i, n := range wantNames {
		if names[i] != n {
			t.Fatalf("names[%d] = %q, want %q (order: declared then discovered)", i, names[i], n)
		}
	}

	if len(detailed) != len(wantNames) {
		t.Fatalf("detailed len = %d, want %d", len(detailed), len(wantNames))
	}
	wantDesc := map[string]string{
		"Echo":         "echoes the input",
		"create_issue": "open an issue",
		"list_issues":  "list issues",
	}
	for i, d := range detailed {
		if d.GetName() != names[i] {
			t.Fatalf("detailed[%d].name = %q, want aligned with names[%d]=%q", i, d.GetName(), i, names[i])
		}
		if got := d.GetDescription(); got != wantDesc[d.GetName()] {
			t.Fatalf("description for %q = %q, want %q", d.GetName(), got, wantDesc[d.GetName()])
		}
	}

	// The derived input schema for a declared method is forwarded on its
	// descriptor; discovered methods carry none.
	if got := detailed[0].GetInputSchemaJson(); got != schemas["Echo"].input {
		t.Fatalf("Echo input_schema_json = %q, want %q", got, schemas["Echo"].input)
	}
	if got := detailed[1].GetInputSchemaJson(); got != "" {
		t.Fatalf("discovered method input_schema_json = %q, want empty", got)
	}
}

func TestBuildMethodMetadataEmpty(t *testing.T) {
	names, detailed := buildMethodMetadata(nil, nil, nil)
	if len(names) != 0 || len(detailed) != 0 {
		t.Fatalf("empty inputs = (%v, %v), want empty", names, detailed)
	}
}
