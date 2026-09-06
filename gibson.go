// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Zero Root AI

package sdk

import (
	"log/slog"
	"os"

	"github.com/zeroroot-ai/sdk/agent"
	"github.com/zeroroot-ai/sdk/serve"
	"github.com/zeroroot-ai/sdk/tool"
)

// NewFramework creates a new Gibson framework instance.
// The framework provides the main SDK interface for mission management,
// registry access, and finding operations.
//
// Example:
//
//	framework, err := sdk.NewFramework(
//	    sdk.WithLogger(logger),
//	    sdk.WithConfig("/path/to/config.yaml"),
//	)
//	if err != nil {
//	    log.Fatal(err)
//	}
//	defer framework.Shutdown(context.Background())
func NewFramework(opts ...FrameworkOption) (Framework, error) {
	cfg := &frameworkConfig{}
	for _, opt := range opts {
		opt(cfg)
	}

	// Create default logger if not provided
	if cfg.logger == nil {
		cfg.logger = slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
			Level: slog.LevelInfo,
		}))
	}

	// Use caller-supplied stores, or fall back to bounded defaults.
	missionStore := cfg.missionStore
	if missionStore == nil {
		missionStore = NewBoundedMissionStore(DefaultMissionStoreCapacity)
	}
	findingsStore := cfg.findingsStore
	if findingsStore == nil {
		findingsStore = NewBoundedFindingsStore(DefaultFindingsStoreCapacity)
	}

	// Create the framework instance
	f := &defaultFramework{
		logger:        cfg.logger,
		tracer:        cfg.tracer,
		configPath:    cfg.configPath,
		agents:        newAgentRegistry(cfg.logger),
		tools:         newToolRegistry(cfg.logger),
		plugins:       newPluginRegistry(cfg.logger),
		missionStore:  missionStore,
		findingsStore: findingsStore,
	}

	return f, nil
}

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

// NewTool creates a new tool with the provided options.
// The tool must have at minimum a name and execute handler.
//
// Example:
//
//	tool, err := sdk.NewTool(
//	    sdk.WithToolName("http-request"),
//	    sdk.WithToolDescription("Makes HTTP requests"),
//	    sdk.WithToolTags("http", "network"),
//	    sdk.WithInputSchema(schema.Object(map[string]schema.JSON{
//	        "url": schema.String(),
//	    })),
//	    sdk.WithExecuteHandler(func(ctx context.Context, input map[string]any) (map[string]any, error) {
//	        // Tool implementation
//	        return map[string]any{"status": 200}, nil
//	    }),
//	)
func NewTool(opts ...ToolOption) (tool.Tool, error) {
	cfg := tool.NewConfig()
	for _, opt := range opts {
		opt(cfg)
	}

	return tool.New(cfg)
}

// convertServeOption converts a public ServeOption to an internal serve.Option.
// This bridges the public SDK API with the internal serve package implementation.
func convertServeOption(opt ServeOption) serve.Option {
	return func(c *serve.Config) {
		// Create a temporary serveConfig to capture the option's values
		tempCfg := &serveConfig{}
		opt(tempCfg)

		// Map serveConfig fields to serve.Config fields
		if tempCfg.healthEndpoint != "" {
			c.HealthEndpoint = tempCfg.healthEndpoint
		}
		if tempCfg.gracefulTimeout != 0 {
			c.GracefulTimeout = tempCfg.gracefulTimeout
		}
	}
}

// ServeAgent connects the agent to the Gibson platform and polls for work.
// Capability Grant authentication is required; provide WithCapabilityGrantFromEnv()
// or WithCapabilityGrant(url). SPIFFE transport upgrade is optional via
// WithSPIFFEFromEnv() for in-cluster components.
//
// Example:
//
//	err := sdk.ServeAgent(myAgent, sdk.WithCapabilityGrantFromEnv())
func ServeAgent(a agent.Agent, opts ...ServeOption) error {
	// Convert public ServeOptions to internal serve.Options
	serveOpts := make([]serve.Option, len(opts))
	for i, opt := range opts {
		serveOpts[i] = convertServeOption(opt)
	}

	// Delegate to the serve package implementation
	return serve.Agent(a, serveOpts...)
}

// ServeTool connects the tool to the Gibson platform and polls for work.
// Capability Grant authentication is required; provide WithCapabilityGrantFromEnv()
// or WithCapabilityGrant(url). SPIFFE transport upgrade is optional via
// WithSPIFFEFromEnv() for in-cluster components.
//
// Example:
//
//	err := sdk.ServeTool(myTool, sdk.WithCapabilityGrantFromEnv())
func ServeTool(t tool.Tool, opts ...ServeOption) error {
	// Convert public ServeOptions to internal serve.Options
	serveOpts := make([]serve.Option, len(opts))
	for i, opt := range opts {
		serveOpts[i] = convertServeOption(opt)
	}

	// Delegate to the serve package implementation
	return serve.Tool(t, serveOpts...)
}
