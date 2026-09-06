// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Zero Root AI

package llm

import "testing"

func TestRole_String(t *testing.T) {
	tests := []struct {
		name string
		role Role
		want string
	}{
		{"system", RoleSystem, "system"},
		{"user", RoleUser, "user"},
		{"assistant", RoleAssistant, "assistant"},
		{"tool", RoleTool, "tool"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.role.String(); got != tt.want {
				t.Errorf("Role.String() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestRole_IsValid(t *testing.T) {
	tests := []struct {
		name string
		role Role
		want bool
	}{
		{"system valid", RoleSystem, true},
		{"user valid", RoleUser, true},
		{"assistant valid", RoleAssistant, true},
		{"tool valid", RoleTool, true},
		{"empty invalid", Role(""), false},
		{"unknown invalid", Role("unknown"), false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.role.IsValid(); got != tt.want {
				t.Errorf("Role.IsValid() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestMessage_IsValid(t *testing.T) {
	tests := []struct {
		name    string
		message Message
		want    bool
	}{
		{
			name: "valid system message",
			message: Message{
				Role:    RoleSystem,
				Content: "You are a helpful assistant",
			},
			want: true,
		},
		{
			name: "valid user message",
			message: Message{
				Role:    RoleUser,
				Content: "Hello",
			},
			want: true,
		},
		{
			name: "valid assistant message with content",
			message: Message{
				Role:    RoleAssistant,
				Content: "Hello! How can I help?",
			},
			want: true,
		},
		{
			name: "valid assistant message with tool calls",
			message: Message{
				Role: RoleAssistant,
				ToolCalls: []ToolCall{
					{ID: "1", Name: "test", Arguments: "{}"},
				},
			},
			want: true,
		},
		{
			name: "valid assistant message with both",
			message: Message{
				Role:    RoleAssistant,
				Content: "Let me check that for you",
				ToolCalls: []ToolCall{
					{ID: "1", Name: "test", Arguments: "{}"},
				},
			},
			want: true,
		},
		{
			name: "valid tool message",
			message: Message{
				Role: RoleTool,
				Name: "test_tool",
				ToolResults: []ToolResult{
					{ToolCallID: "1", Content: "result"},
				},
			},
			want: true,
		},
		{
			name: "invalid system message - has tool calls",
			message: Message{
				Role:    RoleSystem,
				Content: "test",
				ToolCalls: []ToolCall{
					{ID: "1", Name: "test", Arguments: "{}"},
				},
			},
			want: false,
		},
		{
			name: "invalid user message - empty content",
			message: Message{
				Role:    RoleUser,
				Content: "",
			},
			want: false,
		},
		{
			name: "invalid assistant message - no content or tool calls",
			message: Message{
				Role: RoleAssistant,
			},
			want: false,
		},
		{
			name: "invalid tool message - no name",
			message: Message{
				Role: RoleTool,
				ToolResults: []ToolResult{
					{ToolCallID: "1", Content: "result"},
				},
			},
			want: false,
		},
		{
			name: "invalid tool message - no results",
			message: Message{
				Role: RoleTool,
				Name: "test_tool",
			},
			want: false,
		},
		{
			name: "invalid role",
			message: Message{
				Role:    Role("invalid"),
				Content: "test",
			},
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.message.IsValid(); got != tt.want {
				t.Errorf("Message.IsValid() = %v, want %v", got, tt.want)
			}
		})
	}
}
