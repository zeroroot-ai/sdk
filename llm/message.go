// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Zero Root AI

package llm

// Role represents the role of a message sender in a conversation.
type Role string

const (
	// RoleSystem represents system-level instructions or context.
	RoleSystem Role = "system"

	// RoleUser represents messages from the user.
	RoleUser Role = "user"

	// RoleAssistant represents messages from the AI assistant.
	RoleAssistant Role = "assistant"

	// RoleTool represents tool execution results.
	RoleTool Role = "tool"
)

// Message represents a single message in a conversation.
type Message struct {
	// Role indicates who sent the message (system, user, assistant, or tool).
	Role Role

	// Content is the text content of the message.
	Content string

	// ToolCalls contains tool invocations requested by the assistant.
	// Only valid when Role is RoleAssistant.
	ToolCalls []ToolCall

	// ToolResults contains the results of tool executions.
	// Only valid when Role is RoleTool.
	ToolResults []ToolResult

	// Name identifies the tool that produced this message.
	// Only valid when Role is RoleTool.
	Name string
}

// IsValid validates that the message has appropriate fields set for its role.
func (m Message) IsValid() bool {
	switch m.Role {
	case RoleSystem, RoleUser:
		return m.Content != "" && len(m.ToolCalls) == 0 && len(m.ToolResults) == 0 && m.Name == ""
	case RoleAssistant:
		// Assistant can have content, tool calls, or both
		return m.Content != "" || len(m.ToolCalls) > 0
	case RoleTool:
		return m.Name != "" && len(m.ToolResults) > 0
	default:
		return false
	}
}

// String returns a string representation of the role.
func (r Role) String() string {
	return string(r)
}

// IsValid checks if the role is one of the defined constants.
func (r Role) IsValid() bool {
	switch r {
	case RoleSystem, RoleUser, RoleAssistant, RoleTool:
		return true
	default:
		return false
	}
}
