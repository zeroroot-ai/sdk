// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Zero Root AI

package sdk

import (
	"fmt"

	"github.com/zeroroot-ai/sdk/agent"
	"github.com/zeroroot-ai/sdk/serve"
)

// NewAgent creates a new agent with the provided options.
// The agent must have at minimum a name, version, description, and execute function.
//
// Example:
//
//	agent, err := sdk.NewAgent(
//	    sdk.WithName("prompt-injector"),
//	    sdk.WithVersion("1.0.0"),
//	    sdk.WithDescription("Tests for prompt injection vulnerabilities"),
//	    sdk.WithCapabilities(agent.CapabilityPromptInjection),
//	    sdk.WithExecuteFunc(func(ctx context.Context, harness agent.Harness, task agent.Task) (agent.Result, error) {
//	        // Agent implementation
//	        return agent.NewSuccessResult("completed"), nil
//	    }),
//	)
func NewAgent(opts ...AgentOption) (agent.Agent, error) {
	cfg := agent.NewConfig()
	for _, opt := range opts {
		opt(cfg)
	}

	return agent.New(cfg)
}

// ServeAgent connects the agent to the Gibson platform with the default serve
// configuration and polls for work. Call serve.Agent to pass options, for
// example serve.WithCapabilityGrantFromEnv().
//
// Example:
//
//	err := sdk.ServeAgent(myAgent)
func ServeAgent(a agent.Agent) error {
	if err := serve.Agent(a); err != nil {
		return fmt.Errorf("serve agent: %w", err)
	}
	return nil
}
