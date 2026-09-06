// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Zero Root AI

package serve

import (
	"context"
	"errors"
	"fmt"

	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"

	"github.com/zeroroot-ai/sdk/agent"
	harnesspb "github.com/zeroroot-ai/sdk/api/gen/gibson/harness/v1"
)

// worldEntityKinds maps the wire enum onto the agent-facing kind. An enum value a
// newer daemon knows and this SDK does not is NOT in the map; entityKind falls
// back to the wire name so the entity still reaches the agent, labelled with
// something it can log, rather than silently becoming "unspecified".
var worldEntityKinds = map[harnesspb.WorldEntityKind]agent.EntityKind{
	harnesspb.WorldEntityKind_WORLD_ENTITY_KIND_UNSPECIFIED: agent.EntityKindUnspecified,
	harnesspb.WorldEntityKind_WORLD_ENTITY_KIND_HOST:        agent.EntityKindHost,
	harnesspb.WorldEntityKind_WORLD_ENTITY_KIND_DOMAIN:      agent.EntityKindDomain,
	harnesspb.WorldEntityKind_WORLD_ENTITY_KIND_SUBDOMAIN:   agent.EntityKindSubdomain,
	harnesspb.WorldEntityKind_WORLD_ENTITY_KIND_CREDENTIAL:  agent.EntityKindCredential,
	harnesspb.WorldEntityKind_WORLD_ENTITY_KIND_ACCOUNT:     agent.EntityKindAccount,
	harnesspb.WorldEntityKind_WORLD_ENTITY_KIND_FINDING:     agent.EntityKindFinding,
}

func entityKind(k harnesspb.WorldEntityKind) agent.EntityKind {
	if kind, ok := worldEntityKinds[k]; ok {
		return kind
	}
	return agent.EntityKind(k.String())
}

// worldViewFromProto converts the wire slice into the agent-facing one.
func worldViewFromProto(resp *harnesspb.WorldViewResponse) agent.WorldView {
	view := agent.WorldView{Truncated: resp.GetTruncated()}
	for _, e := range resp.GetEntities() {
		var attrs map[string]string
		if len(e.GetAttributes()) > 0 {
			attrs = make(map[string]string, len(e.GetAttributes()))
			for k, v := range e.GetAttributes() {
				attrs[k] = v
			}
		}
		view.Entities = append(view.Entities, agent.WorldEntity{
			Handle:     agent.Handle(e.GetHandle()),
			Kind:       entityKind(e.GetKind()),
			Label:      e.GetLabel(),
			Attributes: attrs,
		})
	}
	return view
}

// WorldView fetches the caller's slice of the tenant World over the callback
// channel (ADR-0012). It sends no tenant and no scope — the request has no field
// for either — so what the slice contains is decided entirely by the daemon from
// the mission record it created.
func (h *CallbackHarness) WorldView(ctx context.Context, focus ...agent.Handle) (agent.WorldView, error) {
	ctx, span := h.tracer.Start(ctx, "gibson.brain.worldview", trace.WithSpanKind(trace.SpanKindClient))
	defer span.End()

	req := &harnesspb.WorldViewRequest{}
	for _, f := range focus {
		req.Focus = append(req.Focus, string(f))
	}

	resp, err := h.client.WorldView(ctx, req)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return agent.WorldView{}, fmt.Errorf("WorldView callback failed: %w", err)
	}
	if resp.Error != nil {
		return agent.WorldView{}, fmt.Errorf("WorldView rejected: %s", resp.Error.Message)
	}
	return worldViewFromProto(resp), nil
}

// WorldView is not wired in platform pull-mode: that transport reaches the daemon
// over the component service, which has no per-mission harness registration and so
// no mission record to project a slice from. Failing is the only honest answer —
// an empty slice would read as "the World is empty" (see BaseHarness.WorldView).
func (h *PlatformHarness) WorldView(_ context.Context, _ ...agent.Handle) (agent.WorldView, error) {
	return agent.WorldView{}, errors.New("WorldView is not supported in platform pull-mode yet (use the callback harness); tracked for the platform transport")
}
