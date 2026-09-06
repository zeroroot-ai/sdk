// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Zero Root AI

// Package compliance provides the agent-side API for contributing metadata
// to compliance signals emitted by the daemon harness middleware.
//
// Agents attach metadata via context rather than via variadic options on
// the harness interface — adding a variadic parameter to every AgentHarness
// method would be a breaking change for every existing implementation. The
// context-based approach achieves the same intent (last-write-wins merging
// at the call site) without an interface churn.
//
// Example usage from agent code:
//
//	func (a *MyAgent) Execute(ctx context.Context, harness agent.Harness) error {
//	    ctx = compliance.WithCustom(ctx, map[string]string{
//	        "gitlab_project_id": "1234",
//	        "gitlab_branch":     "main",
//	        "change_ticket":     "CHG-0042",
//	    })
//	    ctx = compliance.WithResourceTags(ctx, map[string]string{
//	        "env": "prod",
//	    })
//	    return harness.CallToolProto(ctx, "gitlab", req, resp)
//	}
//
// The daemon-side ComplianceMiddleware reads these context values via the
// CallSettingsFromContext helper and merges them into the emitted signal's
// custom and resource_tags bags at precedence level 3 (agent).
//
// Precedence rules:
//   - Target node tags (1) win over all
//   - Mission YAML (2) wins over agent
//   - Agent (3) — this package
//   - Tool/plugin (4) — tool proto field 99 via compliance_tool_provider
//   - Daemon defaults (5) — lowest
package compliance

import (
	"context"
)

// CallSettings is the bag of metadata an agent is contributing to the
// compliance signal for a single harness call. Both maps are always
// non-nil — use the With* helpers to populate.
type CallSettings struct {
	// Custom holds free-form key/value tags that describe the ACTION
	// being performed (not the resource). These populate the signal's
	// `custom` bag.
	Custom map[string]string

	// ResourceTags holds key/value tags that describe the RESOURCE
	// being touched. These populate the signal's `resource_tags` bag.
	ResourceTags map[string]string
}

// NewCallSettings returns an empty, non-nil CallSettings.
func NewCallSettings() *CallSettings {
	return &CallSettings{
		Custom:       map[string]string{},
		ResourceTags: map[string]string{},
	}
}

type callSettingsCtxKey struct{}

// WithCustom returns a new context carrying the given custom tags, merged
// with any existing agent-stamped tags. Last-write-wins on collisions.
func WithCustom(ctx context.Context, kv map[string]string) context.Context {
	s := settingsFromCtx(ctx)
	for k, v := range kv {
		s.Custom[k] = v
	}
	return context.WithValue(ctx, callSettingsCtxKey{}, s)
}

// WithResourceTags returns a new context carrying the given resource tags,
// merged with any existing agent-stamped tags. Last-write-wins.
func WithResourceTags(ctx context.Context, kv map[string]string) context.Context {
	s := settingsFromCtx(ctx)
	for k, v := range kv {
		s.ResourceTags[k] = v
	}
	return context.WithValue(ctx, callSettingsCtxKey{}, s)
}

// CallSettingsFromContext returns the agent-stamped CallSettings from the
// context, or nil if none has been set. The returned pointer is shared —
// callers must not mutate it.
func CallSettingsFromContext(ctx context.Context) *CallSettings {
	v, ok := ctx.Value(callSettingsCtxKey{}).(*CallSettings)
	if !ok {
		return nil
	}
	return v
}

// settingsFromCtx returns the existing CallSettings or a new empty one.
// Used by With* helpers so the returned context always has a non-nil value.
func settingsFromCtx(ctx context.Context) *CallSettings {
	if s, ok := ctx.Value(callSettingsCtxKey{}).(*CallSettings); ok && s != nil {
		// Return a shallow copy so the caller's With* chain does not
		// retroactively mutate the parent context's settings.
		out := &CallSettings{
			Custom:       make(map[string]string, len(s.Custom)),
			ResourceTags: make(map[string]string, len(s.ResourceTags)),
		}
		for k, v := range s.Custom {
			out.Custom[k] = v
		}
		for k, v := range s.ResourceTags {
			out.ResourceTags[k] = v
		}
		return out
	}
	return NewCallSettings()
}
