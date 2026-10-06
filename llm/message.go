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

// String returns a string representation of the role.
func (r Role) String() string {
	return string(r)
}
