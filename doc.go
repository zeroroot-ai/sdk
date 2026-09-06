// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Zero Root AI

// Package sdk provides the official Software Development Kit for the Gibson Framework.
//
// The Gibson SDK enables developers to build, deploy, and manage AI agents, tools,
// and plugins within the Gibson Framework ecosystem. It provides a comprehensive set
// of APIs for interacting with the Gibson runtime, creating custom agents, integrating
// tools, and extending framework functionality through plugins.
//
// # Core Concepts
//
// The SDK is organized around several key concepts:
//
//   - Agents: AI-powered entities that perform tasks and interact with users
//   - Tools: Reusable capabilities that agents can invoke to accomplish tasks
//   - Plugins: Extensions that add new functionality to the framework
//   - Slots: Requirements that define LLM capabilities needed by agents
//   - Runtime: The execution environment that manages agent lifecycle and tool execution
//
// # Architecture
//
// The SDK follows a layered architecture:
//
//   - Client Layer: High-level APIs for common operations
//   - Protocol Layer: gRPC-based communication with Gibson runtime
//   - Plugin Layer: Plugin development and integration APIs
//   - Observability Layer: OpenTelemetry-based monitoring and tracing
//
// # Getting Started
//
// To use the SDK, create a framework instance:
//
//	import "github.com/zeroroot-ai/sdk"
//
//	framework, err := sdk.NewFramework(
//		sdk.WithLogger(logger),
//		sdk.WithConfig("/path/to/config.yaml"),
//	)
//	if err != nil {
//		log.Fatal(err)
//	}
//	defer framework.Shutdown(context.Background())
//
// # Agent Development
//
// Create custom agents using the builder pattern:
//
//	agent, err := sdk.NewAgent(
//		sdk.WithName("my-agent"),
//		sdk.WithVersion("1.0.0"),
//		sdk.WithDescription("My custom security agent"),
//		sdk.WithCapabilities("prompt-injection", "sqli"),
//		sdk.WithExecuteFunc(func(ctx context.Context, harness agent.Harness, task agent.Task) (agent.Result, error) {
//			// Agent logic here
//			return agent.NewSuccessResult("completed"), nil
//		}),
//	)
//	if err != nil {
//		log.Fatal(err)
//	}
//
// # Tool Development
//
// Create custom tools using the builder pattern:
//
//	tool, err := sdk.NewTool(
//		sdk.WithToolName("http-request"),
//		sdk.WithToolDescription("Makes HTTP requests"),
//		sdk.WithToolTags("http", "network"),
//		sdk.WithInputMessageType("zero_day.tools.http.HttpRequest"),
//		sdk.WithOutputMessageType("zero_day.tools.http.HttpResponse"),
//		sdk.WithExecuteProtoHandler(func(ctx context.Context, input proto.Message) (proto.Message, error) {
//			// Tool logic here
//			resp := &HttpResponse{Status: 200}
//			return resp, nil
//		}),
//	)
//	if err != nil {
//		log.Fatal(err)
//	}
//
// # Plugin Development
//
// Build plugins Go-first: write typed request/response structs and a
// func(ctx, Req) (Resp, error) handler; the SDK derives the tool schema from
// the Go types (ADR-0065 R4):
//
//	func main() {
//	    if err := plugin.Serve(ctx,
//	        plugin.WithManifest("plugin.yaml"),
//	        plugin.WithHandler("Echo", echoHandler),
//	    ); err != nil {
//	        log.Fatal(err)
//	    }
//	}
//
// See the plugin package documentation and the examples/ directory for
// complete examples.
//
// # Error Handling
//
// The SDK uses sentinel errors and structured error types for robust error handling:
//
//	if err != nil {
//		if errors.Is(err, sdk.ErrAgentNotFound) {
//			// Handle agent not found
//		}
//		// Handle other errors
//	}
//
// # Observability
//
// The SDK integrates OpenTelemetry for distributed tracing and metrics:
//
//	import "go.opentelemetry.io/otel"
//
//	tracer := otel.Tracer("my-agent")
//	ctx, span := tracer.Start(ctx, "agent-execution")
//	defer span.End()
//
// # Thread Safety
//
// All SDK client methods are safe for concurrent use. Agent and tool implementations
// should ensure thread safety when managing shared state.
//
// # Best Practices
//
//   - Always use context for cancellation and timeouts
//   - Implement proper error handling and error wrapping
//   - Use structured logging for debugging and monitoring
//   - Implement graceful shutdown for long-running operations
//   - Validate input parameters before processing
//   - Use dependency injection for testability
//
// # Examples
//
// See the examples directory for complete working examples of:
//
//   - Creating and deploying custom agents
//   - Building reusable tools
//   - Developing framework plugins
//   - Integrating with existing systems
//   - Testing agents and tools
//
// # Support
//
// For more information, visit:
//
//	Documentation: https://docs.gibson.ai
//	GitHub: https://github.com/zeroroot-ai/gibson
//	Community: https://community.gibson.ai
package sdk
