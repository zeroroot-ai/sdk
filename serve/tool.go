// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Zero Root AI

package serve

import (
	"log/slog"
	"os"

	"github.com/zeroroot-ai/sdk/tool"
)

// SchemaFlag is the command-line flag used to request schema output.
const SchemaFlag = "--schema"

// Tool serves a tool implementation.
//
// If --schema flag is passed, outputs tool schema to stdout and exits.
// Otherwise, performs the Capability Grant bootstrap (discover → register), connects
// to the Gibson platform in pull mode, and polls for work.
//
// WithCapabilityGrant or WithCapabilityGrantFromEnv must be provided; Tool returns an error
// if PlatformURL is not configured.
//
// Example:
//
//	err := serve.Tool(myTool, serve.WithCapabilityGrantFromEnv())
//	if err != nil {
//	    log.Fatal(err)
//	}
func Tool(t tool.Tool, opts ...Option) error {
	// Check for --schema flag
	if hasSchemaFlag() {
		return OutputSchema(t)
	}

	cfg := DefaultConfig()
	for _, opt := range opts {
		opt(cfg)
	}

	if err := validateConfig(cfg); err != nil {
		return err
	}
	slog.Info("connecting to platform", "name", t.Name(), "platform_url", cfg.PlatformURL)
	return servePlatformTool(t, cfg)
}

// hasSchemaFlag checks if --schema was passed as a command-line argument.
func hasSchemaFlag() bool {
	for _, arg := range os.Args[1:] {
		if arg == SchemaFlag {
			return true
		}
	}
	return false
}
