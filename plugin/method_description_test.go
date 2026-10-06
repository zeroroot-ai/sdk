// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Zero Root AI

package plugin

import (
	"context"
	"strings"
	"testing"

	"github.com/zeroroot-ai/sdk/plugin/manifest"
)

// A method's description used to live only in the plugin manifest's `methods:`
// list. It is what an agent reads to choose between tools in the catalog
// (RegisterComponent.method_descriptors -> SearchTools), so it has to survive
// the manifest's deletion (sdk#127, ADR-0097).

func TestWithHandler_DescriptionIsRequired(t *testing.T) {
	for _, desc := range []string{"", "   ", "\t\n"} {
		c := &config{}
		WithHandler("Echo", desc, func(_ context.Context, req string) (string, error) {
			return req, nil
		})(c)

		if len(c.optionErrs) == 0 {
			t.Fatalf("description %q was accepted; an empty description degrades tool selection without failing anything", desc)
		}
		if got := c.optionErrs[0].Error(); !strings.Contains(got, "description is required") {
			t.Errorf("error does not say the description is required: %v", got)
		}
		if _, ok := c.handlers["Echo"]; ok {
			t.Error("the handler was registered despite the rejected description")
		}
	}
}

func TestWithHandler_DescriptionReachesTheDescriptors(t *testing.T) {
	c := &config{}
	WithHandler("Echo", "echoes the request back unchanged", func(_ context.Context, req string) (string, error) {
		return req, nil
	})(c)
	if len(c.optionErrs) != 0 {
		t.Fatalf("unexpected option errors: %v", c.optionErrs)
	}

	// No manifest declarations at all: the registered handler is the only source.
	names, detailed := buildMethodMetadata(nil, c.methodSchemas, c.methodDescriptions)
	if len(names) != 1 || names[0] != "Echo" {
		t.Fatalf("names = %v, want [Echo]", names)
	}
	if len(detailed) != 1 {
		t.Fatalf("descriptors = %d, want 1", len(detailed))
	}
	if got := detailed[0].GetDescription(); got != "echoes the request back unchanged" {
		t.Errorf("description = %q, want the handler's", got)
	}
	if detailed[0].GetInputSchemaJson() == "" {
		t.Error("the Go-derived input schema did not travel with the descriptor")
	}
}

// The handler wins where the two disagree, so a stale manifest cannot override
// the description sitting beside the code.
func TestBuildMethodMetadata_HandlerDescriptionWinsOverManifest(t *testing.T) {
	declared := []manifest.MethodDecl{{Name: "Echo", Description: "stale manifest text"}}
	descriptions := map[string]string{"Echo": "the handler's text"}

	_, detailed := buildMethodMetadata(declared, nil, descriptions)
	if len(detailed) != 1 {
		t.Fatalf("descriptors = %d, want 1", len(detailed))
	}
	if got := detailed[0].GetDescription(); got != "the handler's text" {
		t.Errorf("description = %q, want the handler's", got)
	}
}

// Registered-only methods are emitted in a stable order. Ranging the map would
// put Go's randomised iteration order into the RegisterComponent payload.
func TestBuildMethodMetadata_RegisteredOnlyOrderIsStable(t *testing.T) {
	descriptions := map[string]string{"Zeta": "z", "Alpha": "a", "Mid": "m"}

	var first []string
	for i := range 20 {
		names, _ := buildMethodMetadata(nil, nil, descriptions)
		if first == nil {
			first = names
			continue
		}
		if len(names) != len(first) {
			t.Fatalf("run %d: length changed: %v vs %v", i, names, first)
		}
		for j := range names {
			if names[j] != first[j] {
				t.Fatalf("run %d: order changed: %v vs %v", i, names, first)
			}
		}
	}
	if first[0] != "Alpha" || first[2] != "Zeta" {
		t.Errorf("names = %v, want sorted", first)
	}
}
