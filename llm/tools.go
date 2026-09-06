// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Zero Root AI

package llm

import (
	"encoding/json"
	"errors"
	"fmt"
)

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

// Validate checks if the tool definition is valid.
func (t *ToolDef) Validate() error {
	if t.Name == "" {
		return errors.New("tool name cannot be empty")
	}
	if t.Description == "" {
		return errors.New("tool description cannot be empty")
	}
	if t.Parameters == nil {
		return errors.New("tool parameters cannot be nil")
	}
	return nil
}

// ParseArguments parses the tool call arguments into the provided value.
// The value parameter should be a pointer to the struct that will receive the arguments.
func (c *ToolCall) ParseArguments(v any) error {
	if c.Arguments == "" {
		return errors.New("no arguments to parse")
	}
	return json.Unmarshal([]byte(c.Arguments), v)
}

// Validate checks if the tool call is valid.
func (c *ToolCall) Validate() error {
	if c.ID == "" {
		return errors.New("tool call ID cannot be empty")
	}
	if c.Name == "" {
		return errors.New("tool call name cannot be empty")
	}
	if c.Arguments == "" {
		return errors.New("tool call arguments cannot be empty")
	}

	// Verify that arguments is valid JSON
	var temp any
	if err := json.Unmarshal([]byte(c.Arguments), &temp); err != nil {
		return fmt.Errorf("invalid JSON in arguments: %w", err)
	}

	return nil
}

// NewToolResult creates a successful tool result.
func NewToolResult(toolCallID, content string) ToolResult {
	return ToolResult{
		ToolCallID: toolCallID,
		Content:    content,
		IsError:    false,
	}
}

// NewToolError creates an error tool result.
func NewToolError(toolCallID, errorMsg string) ToolResult {
	return ToolResult{
		ToolCallID: toolCallID,
		Content:    errorMsg,
		IsError:    true,
	}
}

// Validate checks if the tool result is valid.
func (r *ToolResult) Validate() error {
	if r.ToolCallID == "" {
		return errors.New("tool call ID cannot be empty")
	}
	if r.Content == "" {
		return errors.New("tool result content cannot be empty")
	}
	return nil
}

// SetJSONContent sets the result content from a JSON-encodable value.
func (r *ToolResult) SetJSONContent(v any) error {
	data, err := json.Marshal(v)
	if err != nil {
		return fmt.Errorf("failed to marshal content: %w", err)
	}
	r.Content = string(data)
	return nil
}

// ParseContent parses the result content into the provided value.
// The value parameter should be a pointer to the struct that will receive the content.
func (r *ToolResult) ParseContent(v any) error {
	if r.Content == "" {
		return errors.New("no content to parse")
	}
	return json.Unmarshal([]byte(r.Content), v)
}

// ToolChoice represents how the LLM should use tools.
type ToolChoice string

const (
	// ToolChoiceNone means the LLM will not use any tools.
	ToolChoiceNone ToolChoice = "none"

	// ToolChoiceAuto means the LLM decides whether to use tools.
	ToolChoiceAuto ToolChoice = "auto"

	// ToolChoiceRequired means the LLM must use a tool.
	ToolChoiceRequired ToolChoice = "required"
)

// String returns the string representation of the tool choice.
func (tc ToolChoice) String() string {
	return string(tc)
}

// IsValid checks if the tool choice is valid.
func (tc ToolChoice) IsValid() bool {
	switch tc {
	case ToolChoiceNone, ToolChoiceAuto, ToolChoiceRequired:
		return true
	default:
		return false
	}
}
