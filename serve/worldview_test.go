// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Zero Root AI

package serve

import (
	"testing"

	"github.com/zeroroot-ai/sdk/agent"
	harnesspb "github.com/zeroroot-ai/sdk/api/gen/gibson/harness/v1"
)

func TestWorldViewFromProto(t *testing.T) {
	resp := &harnesspb.WorldViewResponse{
		Truncated: true,
		Entities: []*harnesspb.WorldEntity{
			{
				Handle:     "h-abc",
				Kind:       harnesspb.WorldEntityKind_WORLD_ENTITY_KIND_HOST,
				Label:      "10.0.0.1",
				Attributes: map[string]string{"open_ports": "22,443"},
			},
			{
				Handle: "h-def",
				Kind:   harnesspb.WorldEntityKind_WORLD_ENTITY_KIND_FINDING,
				Label:  "SSH weak ciphers",
			},
		},
	}

	got := worldViewFromProto(resp)

	if !got.Truncated {
		t.Error("Truncated was dropped in conversion")
	}
	if len(got.Entities) != 2 {
		t.Fatalf("got %d entities, want 2", len(got.Entities))
	}
	if got.Entities[0].Handle != agent.Handle("h-abc") {
		t.Errorf("handle = %q, want h-abc", got.Entities[0].Handle)
	}
	if got.Entities[0].Kind != agent.EntityKindHost {
		t.Errorf("kind = %q, want %q", got.Entities[0].Kind, agent.EntityKindHost)
	}
	if got.Entities[0].Label != "10.0.0.1" {
		t.Errorf("label = %q, want 10.0.0.1", got.Entities[0].Label)
	}
	if got.Entities[0].Attributes["open_ports"] != "22,443" {
		t.Errorf("attributes = %v, want open_ports=22,443", got.Entities[0].Attributes)
	}
	if got.Entities[1].Kind != agent.EntityKindFinding {
		t.Errorf("kind = %q, want %q", got.Entities[1].Kind, agent.EntityKindFinding)
	}
	if got.Entities[1].Attributes != nil {
		t.Errorf("absent attributes became %v, want nil", got.Entities[1].Attributes)
	}
}

// TestEntityKindUnknownIsNotSilentlyUnspecified pins the degrade-don't-drop rule:
// a kind a newer daemon knows and this SDK does not must arrive as something the
// agent can log, not as the zero value — which would be indistinguishable from a
// daemon that genuinely sent UNSPECIFIED.
func TestEntityKindUnknownIsNotSilentlyUnspecified(t *testing.T) {
	const fromTheFuture = harnesspb.WorldEntityKind(9999)

	if got := entityKind(fromTheFuture); got == agent.EntityKindUnspecified {
		t.Fatalf("unknown kind collapsed to unspecified; want a distinguishable value")
	}
	if got := entityKind(harnesspb.WorldEntityKind_WORLD_ENTITY_KIND_UNSPECIFIED); got != agent.EntityKindUnspecified {
		t.Errorf("explicit unspecified = %q, want empty", got)
	}
}

// TestWorldViewRequestCannotNameATenantOrScope is a structural guard over the
// generated descriptor: the read request must expose no field an agent could use
// to name another tenant's World or a wider scope. Adding one would make the
// cross-tenant check something a handler has to remember; there being no field is
// what makes it unforgettable (ADR-0012).
func TestWorldViewRequestCannotNameATenantOrScope(t *testing.T) {
	fields := (&harnesspb.WorldViewRequest{}).ProtoReflect().Descriptor().Fields()

	allowed := map[string]bool{"context": true, "focus": true}
	for i := 0; i < fields.Len(); i++ {
		name := string(fields.Get(i).Name())
		if !allowed[name] {
			t.Errorf("WorldViewRequest gained field %q: the slice is server-projected, "+
				"so any new selector on the request is a way to ask for a wider one", name)
		}
	}
}
