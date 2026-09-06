// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Zero Root AI

package sdk

import (
	"context"
	"log/slog"
	"time"

	"github.com/zeroroot-ai/sdk/agent"
	"github.com/zeroroot-ai/sdk/llm"
	"github.com/zeroroot-ai/sdk/tool"
	"go.opentelemetry.io/otel/trace"
	"google.golang.org/protobuf/proto"
)

// FrameworkOption configures the Framework.
type FrameworkOption func(*frameworkConfig)

// frameworkConfig holds configuration for the Framework instance.
type frameworkConfig struct {
	configPath    string
	logger        *slog.Logger
	tracer        trace.Tracer
	missionStore  MissionStore
	findingsStore FindingsStore
}

// WithConfig sets the configuration file path for the framework.
// The config file contains framework-level settings like connection strings,
// service endpoints, and default values.
func WithConfig(path string) FrameworkOption {
	return func(c *frameworkConfig) {
		c.configPath = path
	}
}

// WithLogger sets a custom logger for the framework.
// If not provided, a default logger will be created.
func WithLogger(logger *slog.Logger) FrameworkOption {
	return func(c *frameworkConfig) {
		c.logger = logger
	}
}

// WithTracer sets an OpenTelemetry tracer for distributed tracing.
// This enables observability and performance monitoring across the framework.
func WithTracer(tracer trace.Tracer) FrameworkOption {
	return func(c *frameworkConfig) {
		c.tracer = tracer
	}
}

// WithMissionStore replaces the default bounded in-memory mission store with a
// caller-supplied implementation.  The supplied store must be safe for
// concurrent use by multiple goroutines.
//
// The default store is a bounded LRU cache with a capacity of
// DefaultMissionStoreCapacity entries.  Pass a custom implementation to persist
// missions across process restarts or to apply a different eviction policy.
func WithMissionStore(store MissionStore) FrameworkOption {
	return func(c *frameworkConfig) {
		c.missionStore = store
	}
}

// WithFindingsStore replaces the default bounded in-memory findings store with
// a caller-supplied implementation.  The supplied store must be safe for
// concurrent use by multiple goroutines.
//
// The default store is a bounded ring buffer with a capacity of
// DefaultFindingsStoreCapacity entries.  Pass a custom implementation to
// persist findings externally or apply a different retention policy.
func WithFindingsStore(store FindingsStore) FrameworkOption {
	return func(c *frameworkConfig) {
		c.findingsStore = store
	}
}

// AgentOption configures an Agent.
type AgentOption func(*agent.Config)

// WithName sets the agent's unique identifier.
// The name should be a kebab-case string (e.g., "prompt-injector").
func WithName(name string) AgentOption {
	return func(c *agent.Config) {
		c.SetName(name)
	}
}

// WithVersion sets the agent's semantic version.
// Should follow semantic versioning format (e.g., "1.0.0").
func WithVersion(version string) AgentOption {
	return func(c *agent.Config) {
		c.SetVersion(version)
	}
}

// WithDescription sets the agent's human-readable description.
// This should explain what the agent does and its purpose.
func WithDescription(desc string) AgentOption {
	return func(c *agent.Config) {
		c.SetDescription(desc)
	}
}

// WithCapabilities sets the agent's security testing capabilities.
// These indicate what types of vulnerabilities the agent can discover.
func WithCapabilities(caps ...string) AgentOption {
	return func(c *agent.Config) {
		c.SetCapabilities(caps)
	}
}

// WithTargetTypes sets the types of target systems the agent can test.
// This helps the framework match agents to appropriate targets.
func WithTargetTypes(targetTypes ...string) AgentOption {
	return func(c *agent.Config) {
		c.SetTargetTypes(targetTypes)
	}
}

// WithTechniqueTypes sets the attack techniques the agent employs.
// This categorizes the agent's testing methodology.
func WithTechniqueTypes(techniqueTypes ...string) AgentOption {
	return func(c *agent.Config) {
		c.SetTechniqueTypes(techniqueTypes)
	}
}

// WithLLMSlot adds an LLM slot requirement to the agent.
// The framework will provision an LLM that meets these requirements.
//
// Example:
//
//	WithLLMSlot("primary", llm.SlotRequirements{
//	    MinContextWindow: 8000,
//	    RequiredFeatures: []string{"function_calling"},
//	})
func WithLLMSlot(name string, requirements llm.SlotRequirements) AgentOption {
	return func(c *agent.Config) {
		c.AddLLMSlot(name, requirements)
	}
}

// WithExecuteFunc sets the function that executes agent tasks.
// This is the core agent logic and is required.
func WithExecuteFunc(fn agent.ExecuteFunc) AgentOption {
	return func(c *agent.Config) {
		c.SetExecuteFunc(fn)
	}
}

// WithInitFunc sets the function that initializes the agent.
// This is called once when the agent is loaded.
// If not set, a default no-op implementation is used.
func WithInitFunc(fn agent.InitFunc) AgentOption {
	return func(c *agent.Config) {
		c.SetInitFunc(fn)
	}
}

// WithShutdownFunc sets the function that shuts down the agent.
// This is called when the agent is being unloaded.
// If not set, a default no-op implementation is used.
func WithShutdownFunc(fn agent.ShutdownFunc) AgentOption {
	return func(c *agent.Config) {
		c.SetShutdownFunc(fn)
	}
}

// WithHealthFunc sets the function that checks agent health.
// This is called to determine the operational status of the agent.
// If not set, a default implementation that returns healthy is used.
func WithHealthFunc(fn agent.HealthFunc) AgentOption {
	return func(c *agent.Config) {
		c.SetHealthFunc(fn)
	}
}

// ToolOption configures a Tool.
type ToolOption func(*tool.Config)

// WithToolName sets the tool's unique identifier.
// The name should be descriptive and unique within the system.
func WithToolName(name string) ToolOption {
	return func(c *tool.Config) {
		c.SetName(name)
	}
}

// WithToolVersion sets the tool's semantic version.
// Should follow semantic versioning format (e.g., "1.0.0").
func WithToolVersion(version string) ToolOption {
	return func(c *tool.Config) {
		c.SetVersion(version)
	}
}

// WithToolDescription sets the tool's human-readable description.
// This should explain what the tool does and how to use it.
func WithToolDescription(desc string) ToolOption {
	return func(c *tool.Config) {
		c.SetDescription(desc)
	}
}

// WithToolTags sets categorization tags for the tool.
// Tags help with discovery and filtering of tools.
func WithToolTags(tags ...string) ToolOption {
	return func(c *tool.Config) {
		c.SetTags(tags)
	}
}

// WithInputMessageType sets the proto message type for tool input.
// The messageType should be a fully-qualified proto message type name.
// Example: "zero_day.tools.http.HttpRequest"
func WithInputMessageType(messageType string) ToolOption {
	return func(c *tool.Config) {
		c.SetInputMessageType(messageType)
	}
}

// WithOutputMessageType sets the proto message type for tool output.
// The messageType should be a fully-qualified proto message type name.
// Example: "zero_day.tools.http.HttpResponse"
func WithOutputMessageType(messageType string) ToolOption {
	return func(c *tool.Config) {
		c.SetOutputMessageType(messageType)
	}
}

// WithExecuteProtoHandler sets the function that executes the tool with proto messages.
// This function implements the tool's core functionality and is required.
func WithExecuteProtoHandler(fn func(ctx context.Context, input proto.Message) (proto.Message, error)) ToolOption {
	return func(c *tool.Config) {
		c.SetExecuteProtoFunc(fn)
	}
}

// ServeOption configures serving behavior for agents, tools, and plugins.
// These options control how components are exposed as network services.
type ServeOption func(*serveConfig)

// serveConfig holds configuration for serving components.
type serveConfig struct {
	healthEndpoint    string
	gracefulTimeout   time.Duration
	enableReflection  bool
	maxRecvMsgSize    int
	maxSendMsgSize    int
	connectionTimeout time.Duration
}

// WithHealthEndpoint sets the path for the health check endpoint.
func WithHealthEndpoint(path string) ServeOption {
	return func(c *serveConfig) {
		c.healthEndpoint = path
	}
}

// WithGracefulShutdown sets the timeout for graceful shutdown.
// The server will wait up to this duration for in-flight requests to complete.
func WithGracefulShutdown(timeout time.Duration) ServeOption {
	return func(c *serveConfig) {
		c.gracefulTimeout = timeout
	}
}

// WithReflection enables gRPC server reflection.
// This allows clients to discover available services and methods at runtime.
func WithReflection(enable bool) ServeOption {
	return func(c *serveConfig) {
		c.enableReflection = enable
	}
}

// WithMaxMessageSize sets the maximum message size for gRPC requests and responses.
// This applies to both incoming (recv) and outgoing (send) messages.
func WithMaxMessageSize(maxBytes int) ServeOption {
	return func(c *serveConfig) {
		c.maxRecvMsgSize = maxBytes
		c.maxSendMsgSize = maxBytes
	}
}

// WithConnectionTimeout sets the timeout for establishing connections.
// This applies to both client and server connection setup.
func WithConnectionTimeout(timeout time.Duration) ServeOption {
	return func(c *serveConfig) {
		c.connectionTimeout = timeout
	}
}

// MissionOption configures mission creation.
type MissionOption func(*missionConfig)

// missionConfig holds configuration for creating a mission.
type missionConfig struct {
	name        string
	description string
	targetID    string
	agentNames  []string
	timeout     time.Duration
	metadata    map[string]interface{}
}

// WithMissionName sets the mission's human-readable name.
func WithMissionName(name string) MissionOption {
	return func(c *missionConfig) {
		c.name = name
	}
}

// WithMissionDescription sets the mission's description.
func WithMissionDescription(desc string) MissionOption {
	return func(c *missionConfig) {
		c.description = desc
	}
}

// WithMissionTarget sets the target system for the mission.
func WithMissionTarget(targetID string) MissionOption {
	return func(c *missionConfig) {
		c.targetID = targetID
	}
}

// WithMissionAgents sets the agents to use for the mission.
func WithMissionAgents(agentNames ...string) MissionOption {
	return func(c *missionConfig) {
		c.agentNames = agentNames
	}
}

// WithMissionTimeout sets the overall timeout for the mission.
func WithMissionTimeout(timeout time.Duration) MissionOption {
	return func(c *missionConfig) {
		c.timeout = timeout
	}
}

// WithMissionMetadata adds arbitrary metadata to the mission.
func WithMissionMetadata(key string, value interface{}) MissionOption {
	return func(c *missionConfig) {
		if c.metadata == nil {
			c.metadata = make(map[string]interface{})
		}
		c.metadata[key] = value
	}
}

// ListOption configures listing operations.
type ListOption func(*listConfig)

// listConfig holds configuration for listing operations.
type listConfig struct {
	limit  int
	offset int
	filter map[string]interface{}
}

// WithLimit sets the maximum number of items to return.
func WithLimit(limit int) ListOption {
	return func(c *listConfig) {
		c.limit = limit
	}
}

// WithOffset sets the number of items to skip (for pagination).
func WithOffset(offset int) ListOption {
	return func(c *listConfig) {
		c.offset = offset
	}
}

// WithFilter adds a filter criterion for listing.
func WithFilter(key string, value interface{}) ListOption {
	return func(c *listConfig) {
		if c.filter == nil {
			c.filter = make(map[string]interface{})
		}
		c.filter[key] = value
	}
}
