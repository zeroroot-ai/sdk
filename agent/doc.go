// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Zero Root AI

// Package agent provides types and interfaces for building AI security testing agents
// in the Gibson Framework SDK.
//
// # Overview
//
// This package defines the core Agent interface and supporting types that enable
// developers to create autonomous security testing agents. Agents are intelligent
// components that can discover vulnerabilities in AI systems using LLMs, tools, and plugins.
//
// # Core Components
//
// Agent Interface - The main interface all agents must implement:
//   - Name, Version, Description: Agent metadata
//   - Capabilities, TargetTypes, TechniqueTypes: Testing capabilities
//   - LLMSlots: LLM requirements
//   - Execute: Core task execution logic
//   - Initialize, Shutdown, Health: Lifecycle management
//
// Harness Interface - The runtime environment provided to agents:
//   - LLM access (Complete, CompleteWithTools, Stream)
//   - Tool access (CallToolProto, ListTools)
//   - Plugin access (QueryPlugin, ListPlugins)
//   - Agent delegation (DelegateToAgent, ListAgents)
//   - Finding management (SubmitFinding, GetFindings)
//   - Memory storage (Memory)
//   - Context access (Mission, Target)
//   - Observability (Tracer, Logger, TokenUsage)
//
// Credential access: agents do not access credentials directly. To call a
// credentialed external service from an agent, dispatch a tool that invokes
// a plugin (see plugin-runtime spec). The plugin holds the credential; the
// agent never sees it.
//
// Observations and the Taxonomy: an agent writes to the World by emitting a
// typed Observation, never by writing graph nodes or relationships. Which
// shapes become typed nodes is the global Taxonomy's decision, made server-side
// and identically for every tenant — not the agent's, and not this package's.
//
// A shape the Taxonomy admits materializes as a typed node. A shape it does not
// admit is neither rejected nor lost: it lands as an Observation with its
// residue preserved and stays immediately queryable (ADR-0012). So an agent can
// always write, and can never invent schema. Promoting a recurring Observation
// shape into the Taxonomy is a reviewed code change, not something an agent or a
// tenant can do at runtime.
//
// LifecycleEntityObservation is the open end of this for application-lifecycle
// work — Application, Repository, Image, Package, Deployment, Vulnerability,
// MergeRequest, Pipeline, Control — carrying the label, the identity, the
// properties and the edges seen. The same rule governs it: an unadmitted label
// or edge type lands as an Observation rather than failing the write.
//
// Builder Pattern - Simplified agent creation via Config:
//   - Fluent API for configuration
//   - Function-based implementation
//   - Automatic validation
//
// # Agent Capabilities
//
// Agents declare their testing capabilities:
//   - CapabilityPromptInjection: Test for prompt injection attacks
//   - CapabilityJailbreak: Test for jailbreak attempts
//   - CapabilityDataExtraction: Test for data extraction
//   - CapabilityModelManipulation: Test for model manipulation
//   - CapabilityDOS: Test for denial-of-service
//
// # Task Execution
//
// Agents receive Task objects containing:
//   - ID: Unique task identifier
//   - Context: Additional information (including objective)
//   - Constraints: Operational limits (max turns, tokens, allowed tools)
//   - Metadata: Task-specific data
//
// Agents return Result objects containing:
//   - Status: Success, Failed, Partial, Cancelled, or Timeout
//   - Output: Task results
//   - Findings: Security findings discovered
//   - Metadata: Result-specific data
//   - Error: Error information if failed
//
// # Building Agents
//
// Using the builder pattern:
//
//	cfg := agent.NewConfig().
//		SetName("prompt-injector").
//		SetVersion("1.0.0").
//		SetDescription("Tests for prompt injection vulnerabilities").
//		AddCapability(agent.CapabilityPromptInjection).
//		AddTargetType(types.TargetTypeLLMChat).
//		AddTechniqueType(types.TechniquePromptInjection).
//		AddLLMSlot("primary", llm.SlotRequirements{
//			MinContextWindow: 8000,
//			RequiredFeatures: []string{"function_calling"},
//		}).
//		SetExecuteFunc(func(ctx context.Context, harness agent.Harness, task agent.Task) (agent.Result, error) {
//			// Agent implementation here
//			logger := harness.Logger()
//			logger.Info("executing task", "task_id", task.ID)
//
//			// Use LLM
//			resp, err := harness.Complete(ctx, "primary", messages)
//			if err != nil {
//				return agent.NewFailedResult(err), err
//			}
//
//			// Submit findings
//			err = harness.SubmitFinding(ctx, finding)
//			if err != nil {
//				logger.Error("failed to submit finding", "error", err)
//			}
//
//			result := agent.NewSuccessResult(resp.Content)
//			result.AddFinding(finding.ID())
//			return result, nil
//		})
//
//	agent, err := agent.New(cfg)
//	if err != nil {
//		log.Fatal(err)
//	}
//
// # Implementing Full Interface
//
// For more complex agents, implement the Agent interface directly:
//
//	type MyAgent struct {
//		config map[string]any
//	}
//
//	func (a *MyAgent) Name() string { return "my-agent" }
//	func (a *MyAgent) Version() string { return "1.0.0" }
//	// ... implement other methods
//
//	func (a *MyAgent) Execute(ctx context.Context, harness agent.Harness, task agent.Task) (agent.Result, error) {
//		// Complex implementation with state management
//	}
//
// # Harness Usage
//
// The harness provides all runtime capabilities:
//
//	// LLM access
//	messages := []llm.Message{
//		{Role: llm.RoleUser, Content: "Test prompt"},
//	}
//	resp, err := harness.Complete(ctx, "primary", messages,
//		llm.WithTemperature(0.7),
//		llm.WithMaxTokens(1000),
//	)
//
//	// Tool calling (proto-based)
//	// Replace mytoolpb with your tool's generated proto package; the SDK
//	// resolves the message types at runtime via FileDescriptorSet metadata
//	// flowing through protoresolver, so the SDK has no compile-time
//	// knowledge of any specific tool's wire format.
//	req := &mytoolpb.MyRequest{...}
//	resp := &mytoolpb.MyResponse{}
//	err := harness.CallToolProto(ctx, "my-tool", req, resp)
//
//	// Finding submission
//	finding := createFinding()
//	err := harness.SubmitFinding(ctx, finding)
//
//	// Memory access
//	mem := harness.Memory()
//	err := mem.Set(ctx, "last-attempt", attemptData)
//
//	// Observability
//	logger := harness.Logger()
//	logger.Info("processing task", "id", task.ID)
//
//	tracer := harness.Tracer()
//	ctx, span := tracer.Start(ctx, "vulnerability-scan")
//	defer span.End()
//
// # Agent Lifecycle
//
//  1. Creation: Agent is instantiated via New() or custom constructor
//  2. Initialize: Initialize() is called with configuration
//  3. Execution: Execute() is called for each task
//  4. Health Checks: Health() is called periodically for monitoring
//  5. Shutdown: Shutdown() is called when agent is being unloaded
//
// # Best Practices
//
//   - Use structured logging via harness.Logger()
//   - Create spans for major operations via harness.Tracer()
//   - Respect task constraints (max turns, max tokens, allowed tools)
//   - Submit findings as they're discovered
//   - Handle context cancellation gracefully
//   - Return appropriate result status (Success, Partial, Failed, etc.)
//   - Include detailed metadata in results for debugging
//   - Use memory store for state that needs to persist across tasks
//   - Do NOT accept raw API keys or secrets as agent parameters. Agents have
//     no credential API by design. To call a credentialed external service,
//     dispatch a tool that uses a plugin; the plugin resolves the credential
//     through the broker. The agent never sees the secret value.
//
// # Error Handling
//
// Agents should use ResultError for structured, JSON-serializable errors:
//
//	// Create a structured error
//	err := agent.NewResultError("TIMEOUT", "LLM request timed out").
//		WithComponent("my-agent").
//		WithRetryable(true).
//		WithDetails(map[string]any{"slot": "main", "timeout_ms": 30000})
//
//	// Wrap existing errors
//	toolErr := agent.Wrap(err, "EXECUTION_FAILED", "Task failed").
//		WithComponent("my-agent")
//
//	// Convert standard errors
//	resultErr := agent.FromError(standardErr)
//
//	// Return in results
//	return agent.Result{Status: agent.StatusFailed, Error: err}, err
//
// ResultError provides:
//   - Code: Standard error code from taxonomy
//   - Message: Human-readable description
//   - Details: Additional context (JSON-serializable)
//   - Cause: Wrapped error chain
//   - Retryable: Whether operation can be retried
//   - Component: Source component identifier
//   - Stack: Optional stack trace for debugging
//
// Standard result statuses:
//   - Return Result with StatusFailed and error for unrecoverable errors
//   - Return Result with StatusPartial for partially completed tasks
//   - Return Result with StatusTimeout when hitting time limits
//   - Return Result with StatusCancelled when context is cancelled
//   - Log errors with sufficient context for debugging
//
// # Thread Safety
//
// Agents should be thread-safe if they maintain internal state:
//   - Use mutexes to protect shared state
//   - Avoid race conditions in concurrent operations
//   - The harness provides thread-safe access to all resources
//
// # Testing
//
// Use mock implementations for testing:
//   - Create mock Harness for unit tests
//   - Test Execute() with various Task configurations
//   - Verify Initialize() and Shutdown() are called correctly
//   - Test Health() returns appropriate status
//   - Validate error handling and edge cases
package agent
