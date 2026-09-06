// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Zero Root AI

package main

import (
	"context"
	"fmt"
	"log"

	sdk "github.com/zeroroot-ai/sdk"
	"github.com/zeroroot-ai/sdk/agent"
	"github.com/zeroroot-ai/sdk/llm"
)

func main() {
	// Create agent using SDK options
	// The agent is configured with metadata, capabilities, and an execution function
	myAgent, err := sdk.NewAgent(
		// Basic agent metadata
		sdk.WithName("minimal-agent"),
		sdk.WithVersion("1.0.0"),
		sdk.WithDescription("A minimal example agent demonstrating basic SDK usage"),

		// Specify what types of AI systems this agent can test
		// Target types are now strings for extensibility
		sdk.WithTargetTypes("llm_chat"),

		// Configure LLM requirements for the agent
		// The agent requires an LLM with at least 8000 tokens of context window
		sdk.WithLLMSlot("main", llm.SlotRequirements{
			MinContextWindow: 8000,
		}),

		// Set the core execution function that runs when the agent receives a task
		sdk.WithExecuteFunc(executeAgent),
	)
	if err != nil {
		log.Fatalf("Failed to create agent: %v", err)
	}

	fmt.Printf("Agent created successfully!\n")
	fmt.Printf("  Name: %s\n", myAgent.Name())
	fmt.Printf("  Version: %s\n", myAgent.Version())
	fmt.Printf("  Description: %s\n", myAgent.Description())

	// Serve the agent as a gRPC service so the Gibson daemon can dial it
	// when its node in a mission DAG becomes active. The harness arg in
	// executeAgent is provisioned by the daemon at invocation time.
	fmt.Println("\nStarting agent server...")
	if err := sdk.ServeAgent(myAgent); err != nil {
		log.Fatalf("Failed to serve agent: %v", err)
	}
}

// executeAgent is the core agent logic that runs for each task.
// It receives:
//   - ctx: Context for cancellation and timeout control
//   - h: Harness providing access to LLMs, tools, and memory
//   - task: The task to execute, including goal and context
//
// It returns:
//   - Result: Contains status, output, and any findings discovered
//   - error: Any error encountered during execution
func executeAgent(ctx context.Context, h agent.Harness, task agent.Task) (agent.Result, error) {
	// Log the task being executed
	fmt.Printf("\nExecuting task: %s\n", task.ID)
	fmt.Printf("  Goal: %s\n", task.Goal)

	// Example: Use the LLM through the harness
	// The harness provides access to the "main" LLM slot we configured earlier
	// In a real agent, you would construct prompts based on the task goal
	// and analyze the responses for security vulnerabilities
	maxTokens := 100
	temperature := 0.7
	completion, err := h.Complete(ctx, "main", []llm.Message{
		{
			Role:    llm.RoleUser,
			Content: "Respond with 'Hello, Gibson!' to confirm operation.",
		},
	}, llm.WithMaxTokens(maxTokens), llm.WithTemperature(temperature))
	if err != nil {
		return agent.Result{
			Status: agent.StatusFailed,
			Output: fmt.Sprintf("LLM completion failed: %v", err),
		}, nil
	}

	fmt.Printf("  LLM Response: %s\n", completion.Content)
	fmt.Printf("  Tokens used: %d\n", completion.Usage.TotalTokens)

	// Example: List available tools
	// In a real agent, you might use tools to interact with the target system
	availableTools, err := h.ListTools(ctx)
	if err != nil {
		fmt.Printf("  Warning: Failed to list tools: %v\n", err)
	} else {
		fmt.Printf("  Available tools: %d\n", len(availableTools))
	}

	// Example: Call a tool (if available)
	// Replace mytoolpb with your tool's generated proto package — the SDK
	// has no compile-time knowledge of any specific tool's wire format and
	// resolves messages at runtime via FileDescriptorSet metadata.
	//
	// req := &mytoolpb.MyRequest{...}
	// resp := &mytoolpb.MyResponse{}
	// if err := h.CallToolProto(ctx, "my-tool", req, resp); err != nil {
	//     fmt.Printf("  Tool call failed: %v\n", err)
	// }

	// Example: Store data in agent memory
	// Memory is now organized into tiers: Working, Mission, and LongTerm
	// Working memory is ephemeral and cleared between executions
	if err := h.Memory().Working().Set(ctx, "last_task_id", task.ID); err != nil {
		fmt.Printf("  Warning: Failed to store in memory: %v\n", err)
	}

	// Example: Retrieve data from memory
	// lastTaskID, err := h.Memory().Working().Get(ctx, "last_task_id")
	// if err == nil {
	//     fmt.Printf("  Previous task ID: %v\n", lastTaskID)
	// }

	// Example: Access mission and target context
	mission := h.Mission()
	target := h.Target()
	fmt.Printf("  Mission: %s\n", mission.Name)
	fmt.Printf("  Target: %s\n", target.URL)

	// Example: Report a finding
	// In a real agent, you would discover actual vulnerabilities
	// Here we demonstrate the finding API
	//
	// Uncomment to report a sample finding:
	// import "github.com/zeroroot-ai/sdk/finding"
	//
	// err = h.SubmitFinding(ctx, &finding.Finding{
	//     Title:       "Example Vulnerability",
	//     Description: "This is a demonstration finding",
	//     Severity:    finding.SeverityLow,
	//     Category:    finding.CategoryPromptInjection,
	// })
	// if err != nil {
	//     fmt.Printf("  Warning: Failed to submit finding: %v\n", err)
	// }

	// Example: Use observability features
	logger := h.Logger()
	logger.Info("Task execution completed",
		"task_id", task.ID,
		"tokens", completion.Usage.TotalTokens)

	// Return a successful result
	return agent.Result{
		Status: agent.StatusSuccess,
		Output: "Task completed successfully. Agent is operational.",
		Metadata: map[string]any{
			"tokens_used": completion.Usage.TotalTokens,
			"response":    completion.Content,
			"mission":     mission.Name,
			"target":      target.URL,
		},
	}, nil
}
