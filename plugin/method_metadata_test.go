// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Zero Root AI

package plugin

import (
	"testing"

	"github.com/zeroroot-ai/sdk/plugin/manifest"
)

// buildMethodMetadata must carry per-method descriptions (the thing the
// connector catalog / SearchTools surface) and keep names + descriptors aligned,
// declared methods first, then the handler-only methods in sorted order.
func TestBuildMethodMetadata(t *testing.T) {
	declared := []manifest.MethodDecl{
		{Name: "Echo", Description: "echoes the input"},
	}
	schemas := map[string]methodSchema{
		"Echo":         {input: `{"type":"object","properties":{"msg":{"type":"string"}}}`},
		"create_issue": {input: `{"type":"object"}`},
	}
	descriptions := map[string]string{
		"list_issues":  "list issues",
		"create_issue": "open an issue",
	}
	names, detailed := buildMethodMetadata(declared, schemas, descriptions)

	wantNames := []string{"Echo", "create_issue", "list_issues"}
	if len(names) != len(wantNames) || len(detailed) != len(wantNames) {
		t.Fatalf("names = %v, descriptors = %d, want %v", names, len(detailed), wantNames)
	}
	for i, n := range wantNames {
		if names[i] != n || detailed[i].GetName() != n {
			t.Fatalf("[%d] = %q/%q, want %q", i, names[i], detailed[i].GetName(), n)
		}
	}
	if detailed[0].GetDescription() != "echoes the input" {
		t.Fatalf("Echo description = %q", detailed[0].GetDescription())
	}
	if detailed[0].GetInputSchemaJson() == "" || detailed[1].GetInputSchemaJson() == "" {
		t.Fatal("a method with a schema must carry input_schema_json")
	}
	if got := detailed[2].GetInputSchemaJson(); got != "" {
		t.Fatalf("list_issues input_schema_json = %q, want empty", got)
	}
	if detailed[1].GetDescription() != "open an issue" {
		t.Fatalf("create_issue description = %q", detailed[1].GetDescription())
	}
}

func TestBuildMethodMetadataEmpty(t *testing.T) {
	names, detailed := buildMethodMetadata(nil, nil, nil)
	if len(names) != 0 || len(detailed) != 0 {
		t.Fatalf("empty inputs = (%v, %v), want empty", names, detailed)
	}
}
