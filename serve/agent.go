// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Zero Root AI

package serve

import (
	"log/slog"

	"github.com/zeroroot-ai/sdk/agent"
)

// Agent connects an agent to the Gibson platform in pull mode.
// It performs the Capability Grant bootstrap (discover → register), dials the platform,
// and polls for work until a shutdown signal is received or an error occurs.
//
// WithCapabilityGrant or WithCapabilityGrantFromEnv must be provided; Agent returns an error
// if PlatformURL is not configured.
//
// Example:
//
//	err := serve.Agent(myAgent, serve.WithCapabilityGrantFromEnv())
//	if err != nil {
//	    log.Fatal(err)
//	}
func Agent(a agent.Agent, opts ...Option) error {
	cfg := DefaultConfig()
	for _, opt := range opts {
		opt(cfg)
	}

	if err := validateConfig(cfg); err != nil {
		return err
	}
	slog.Info("starting agent", "name", a.Name(), "platform_url", cfg.PlatformURL)
	return servePlatformAgent(a, cfg)
}
