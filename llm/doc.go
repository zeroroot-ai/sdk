// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Zero Root AI

// Package llm provides types and interfaces for working with Large Language Models
// in the Gibson framework.
//
// This package defines the core abstractions for LLM interactions, including:
//   - Message types for conversations (system, user, assistant, tool)
//   - Completion requests and responses
//   - Streaming response handling
//   - Tool/function calling definitions
//   - LLM slot definitions and requirements
//   - Token usage tracking
//
// # Message Types
//
// The Message type represents a single message in a conversation with an LLM.
// Messages have different roles (system, user, assistant, tool) and may contain
// text content, tool calls, or tool results.
//
//	msg := llm.Message{
//	    Role:    llm.RoleUser,
//	    Content: "What is the weather in San Francisco?",
//	}
//
// # Completion Requests
//
// CompletionRequest represents a request to an LLM for text generation.
// Use functional options to configure the request:
//
//	req := llm.NewCompletionRequest(messages,
//	    llm.WithTemperature(0.7),
//	    llm.WithMaxTokens(1000),
//	    llm.WithTools(tools...),
//	)
//
// # Streaming Responses
//
// For streaming completions, use StreamChunk and StreamAccumulator to process
// incremental responses:
//
//	acc := llm.NewStreamAccumulator()
//	for chunk := range stream {
//	    acc.Add(chunk)
//	    if chunk.IsFinal() {
//	        response := acc.ToResponse()
//	        break
//	    }
//	}
//
// # Tool Calling
//
// Tools allow LLMs to invoke external functions. Define tools with ToolDef
// and handle tool calls with ToolCall and ToolResult:
//
//	tool := llm.ToolDef{
//	    Name:        "get_weather",
//	    Description: "Get current weather for a location",
//	    Parameters: map[string]any{
//	        "type": "object",
//	        "properties": map[string]any{
//	            "location": map[string]any{
//	                "type":        "string",
//	                "description": "City name",
//	            },
//	        },
//	        "required": []string{"location"},
//	    },
//	}
//
// # Slot Definitions
//
// Slots represent different LLM capabilities needed by a Gibson agent.
// SlotDefinition specifies requirements like context window size and required features:
//
//	slot := llm.SlotDefinition{
//	    Name:             "primary",
//	    Description:      "Main conversational LLM",
//	    Required:         true,
//	    MinContextWindow: 32000,
//	    RequiredFeatures: []string{"function_calling", "streaming"},
//	    PreferredModels:  []string{"gpt-4-turbo", "claude-3-opus"},
//	}
//
// # Token Tracking
//
// Track token usage across different LLM slots with TokenTracker:
//
//	tracker := llm.NewTokenTracker()
//	tracker.Add("primary", response.Usage)
//	total := tracker.Total()
//	fmt.Printf("Total tokens used: %d\n", total.TotalTokens)
package llm
