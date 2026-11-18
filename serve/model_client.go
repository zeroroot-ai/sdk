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
// x-capability-grant. A static token is the task grant of the agent. A signer
// makes a new grant for each route.
func grantHeaders(token string, signer capabilitygrant.MethodSigner) openresponses.HeaderFunc {
	return func(_ context.Context, route string) (map[string]string, error) {
		switch {
		case signer != nil:
			g, err := signer.SignForMethod(route)
			if err != nil {
				return nil, fmt.Errorf("sign grant for %s: %w", route, err)
			}
			return map[string]string{capabilitygrant.MetadataHeaderCapabilityGrant: g}, nil
		case token != "":
			return map[string]string{capabilitygrant.MetadataHeaderCapabilityGrant: token}, nil
		default:
			return nil, errors.New("the agent has no task grant to send to the model gateway")
		}
	}
}
