// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Zero Root AI

package compliance

import (
	"context"
	"testing"
)

func TestWithCustom_PopulatesAndImmutable(t *testing.T) {
	ctx := context.Background()
	ctx1 := WithCustom(ctx, map[string]string{"a": "1", "b": "2"})
	s1 := CallSettingsFromContext(ctx1)
	if s1.Custom["a"] != "1" || s1.Custom["b"] != "2" {
		t.Errorf("s1.Custom = %v", s1.Custom)
	}

	ctx2 := WithCustom(ctx1, map[string]string{"a": "overridden"})
	s2 := CallSettingsFromContext(ctx2)
	if s2.Custom["a"] != "overridden" {
		t.Errorf("s2.Custom[a] = %q; want overridden", s2.Custom["a"])
	}
	if s2.Custom["b"] != "2" {
		t.Errorf("s2.Custom[b] = %q; want 2", s2.Custom["b"])
	}

	// Parent context must not be mutated by child.
	if s1.Custom["a"] != "1" {
		t.Errorf("parent s1 was mutated: %v", s1.Custom)
	}
}

func TestWithResourceTags(t *testing.T) {
	ctx := WithResourceTags(context.Background(), map[string]string{"env": "prod"})
	s := CallSettingsFromContext(ctx)
	if s.ResourceTags["env"] != "prod" {
		t.Errorf("ResourceTags[env] = %q", s.ResourceTags["env"])
	}
}

func TestCombinedWithCalls(t *testing.T) {
	ctx := context.Background()
	ctx = WithCustom(ctx, map[string]string{"k1": "v1"})
	ctx = WithResourceTags(ctx, map[string]string{"env": "prod"})
	ctx = WithCustom(ctx, map[string]string{"k2": "v2"})

	s := CallSettingsFromContext(ctx)
	if s.Custom["k1"] != "v1" || s.Custom["k2"] != "v2" {
		t.Errorf("Custom = %v", s.Custom)
	}
	if s.ResourceTags["env"] != "prod" {
		t.Errorf("ResourceTags = %v", s.ResourceTags)
	}
}

func TestFromContext_None(t *testing.T) {
	s := CallSettingsFromContext(context.Background())
	if s != nil {
		t.Errorf("expected nil settings on bare context, got %v", s)
	}
}
