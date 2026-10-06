// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Zero Root AI

package llm

// ToolDef defines a tool that an LLM can invoke.
type ToolDef struct {
	// Name is the unique identifier for this tool.
	Name string

	// Description explains what the tool does and when to use it.
	// This helps the LLM decide when to invoke the tool.
	Description string

	// Parameters is a JSON Schema describing the tool's input parameters.
	// The schema should define the structure, types, and validation rules.
	Parameters map[string]any
}

// ToolCall represents an LLM's request to invoke a tool.
type ToolCall struct {
	// ID is a unique identifier for this tool call.
	// Used to match tool results back to the original call.
	ID string

	// Name is the name of the tool to invoke.
	Name string

	// Arguments contains the tool parameters as a JSON string.
	// This should be parsed according to the tool's parameter schema.
	Arguments string
}

// ToolResult represents the result of executing a tool.
type ToolResult struct {
	// ToolCallID matches the ID from the corresponding ToolCall.
	ToolCallID string

	// Content contains the result data as a string.
	// For structured data, this should be JSON-encoded.
	Content string

	// IsError indicates whether the tool execution failed.
	// If true, Content contains an error message.
	IsError bool
}
