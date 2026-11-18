// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Zero Root AI

package serve

import (
	"context"
	"errors"
	"fmt"

	"github.com/zeroroot-ai/sdk/capabilitygrant"
	"github.com/zeroroot-ai/sdk/llm"
	"github.com/zeroroot-ai/sdk/openresponses"
)

// resolvedModel is the answer of the daemon to a ResolveModel call. Both
// harnesses use it.
type resolvedModel struct {
	model           string
	provider        string
	gatewayURL      string
	routeHeaders    map[string]string
	contextWindow   int
	maxOutputTokens int
	gatewayCAPEM    string
}

// newModelClient builds the one model client for a resolved slot. The client
// sends the grant that headers returns on each call, and it adds the token
// counts of each response to the tracker under the slot.
func newModelClient(
	slot string, m *resolvedModel, headers openresponses.HeaderFunc, tracker llm.TokenTracker,
) (*openresponses.Client, error) {
	if m.model == "" {
		return nil, fmt.Errorf("resolve model for slot %q: the daemon returned no model", slot)
	}
	if m.gatewayURL == "" {
		return nil, fmt.Errorf("resolve model for slot %q: the daemon returned no gateway URL", slot)
	}
	c := &openresponses.Client{
		BaseURL:         m.gatewayURL,
		Model:           m.model,
		RouteHeaders:    m.routeHeaders,
		Headers:         headers,
		CAPEM:           m.gatewayCAPEM,
		Provider:        m.provider,
		ContextWindow:   m.contextWindow,
		MaxOutputTokens: m.maxOutputTokens,
	}
	if tracker != nil {
		c.OnUsage = func(u openresponses.Usage) {
			tracker.Add(slot, llm.TokenUsage{
				InputTokens: u.InputTokens, OutputTokens: u.OutputTokens, TotalTokens: u.TotalTokens,
			})
		}
	}
	return c, nil
}

// grantHeaders returns a HeaderFunc that sends one grant in the header
// x-capability-grant. The source gives the task grant of the agent. There is
// no self-signed token for a gateway call: ext-authz needs the tenant, the
// mission and the task of the grant.
func grantHeaders(source func(ctx context.Context) (string, error)) openresponses.HeaderFunc {
	return func(ctx context.Context, _ string) (map[string]string, error) {
		g, err := source(ctx)
		if err != nil {
			return nil, err
		}
		return map[string]string{capabilitygrant.MetadataHeaderCapabilityGrant: g}, nil
	}
}

// staticGrant returns a source for a grant that never changes.
func staticGrant(token string) func(context.Context) (string, error) {
	return func(context.Context) (string, error) {
		if token == "" {
			return "", errNoTaskGrant
		}
		return token, nil
	}
}

// errNoTaskGrant is the error of a gateway call with no task grant.
var errNoTaskGrant = errors.New("no task grant: the agent has no task grant to send to the gateway")
