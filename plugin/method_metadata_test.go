// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Zero Root AI

package plugin

import "testing"

// buildMethodMetadata must carry per-method descriptions (the thing the
// connector catalog / SearchTools surface), keep names and descriptors
// aligned, and forward the Go-derived input schema.
func TestBuildMethodMetadata(t *testing.T) {
	descriptions := map[string]string{
		"Echo":        "echoes the input",
		"CreateIssue": "open an issue",
	}
	schemas := map[string]methodSchema{
		"Echo": {input: `{"type":"object","properties":{"msg":{"type":"string"}}}`},
	}
	names, detailed := buildMethodMetadata(schemas, descriptions)

	wantNames := []string{"CreateIssue", "Echo"}
	if len(names) != len(wantNames) || len(detailed) != len(wantNames) {
		t.Fatalf("names = %v, descriptors = %d, want %v", names, len(detailed), wantNames)
	}
	for i, n := range wantNames {
		if names[i] != n {
			t.Fatalf("names[%d] = %q, want %q (sorted)", i, names[i], n)
		}
		if detailed[i].GetName() != n {
			t.Fatalf("detailed[%d].name = %q, want aligned with names", i, detailed[i].GetName())
		}
		if got := detailed[i].GetDescription(); got != descriptions[n] {
			t.Fatalf("description for %q = %q, want %q", n, got, descriptions[n])
		}
	}
	if got := detailed[1].GetInputSchemaJson(); got != schemas["Echo"].input {
		t.Fatalf("Echo input_schema_json = %q, want %q", got, schemas["Echo"].input)
	}
	if got := detailed[0].GetInputSchemaJson(); got != "" {
		t.Fatalf("a method with no schema carries input_schema_json %q, want empty", got)
	}
}

func TestBuildMethodMetadataEmpty(t *testing.T) {
	names, detailed := buildMethodMetadata(nil, nil)
	if len(names) != 0 || len(detailed) != 0 {
		t.Fatalf("empty inputs = (%v, %v), want empty", names, detailed)
	}
}
