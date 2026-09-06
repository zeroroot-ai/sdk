// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Zero Root AI

package llm

import (
	"reflect"
	"testing"
)

func TestStreamChunk_IsFinal(t *testing.T) {
	tests := []struct {
		name  string
		chunk StreamChunk
		want  bool
	}{
		{
			name:  "final chunk with stop",
			chunk: StreamChunk{FinishReason: "stop"},
			want:  true,
		},
		{
			name:  "final chunk with length",
			chunk: StreamChunk{FinishReason: "length"},
			want:  true,
		},
		{
			name:  "not final",
			chunk: StreamChunk{Delta: "hello"},
			want:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.chunk.IsFinal(); got != tt.want {
				t.Errorf("IsFinal() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestStreamChunk_HasContent(t *testing.T) {
	tests := []struct {
		name  string
		chunk StreamChunk
		want  bool
	}{
		{
			name:  "has content",
			chunk: StreamChunk{Delta: "hello"},
			want:  true,
		},
		{
			name:  "no content",
			chunk: StreamChunk{},
			want:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.chunk.HasContent(); got != tt.want {
				t.Errorf("HasContent() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestStreamChunk_HasToolCalls(t *testing.T) {
	tests := []struct {
		name  string
		chunk StreamChunk
		want  bool
	}{
		{
			name: "has tool calls",
			chunk: StreamChunk{
				ToolCalls: []ToolCall{{ID: "1"}},
			},
			want: true,
		},
		{
			name:  "no tool calls",
			chunk: StreamChunk{},
			want:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.chunk.HasToolCalls(); got != tt.want {
				t.Errorf("HasToolCalls() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestStreamChunk_HasUsage(t *testing.T) {
	tests := []struct {
		name  string
		chunk StreamChunk
		want  bool
	}{
		{
			name: "has usage",
			chunk: StreamChunk{
				Usage: &TokenUsage{TotalTokens: 100},
			},
			want: true,
		},
		{
			name:  "no usage",
			chunk: StreamChunk{},
			want:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.chunk.HasUsage(); got != tt.want {
				t.Errorf("HasUsage() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestStreamAccumulator_Add(t *testing.T) {
	acc := NewStreamAccumulator()

	// Add first chunk with content
	acc.Add(StreamChunk{
		Delta: "Hello",
	})

	if acc.Content != "Hello" {
		t.Errorf("Content = %q, want %q", acc.Content, "Hello")
	}

	// Add second chunk with more content
	acc.Add(StreamChunk{
		Delta: " world",
	})

	if acc.Content != "Hello world" {
		t.Errorf("Content = %q, want %q", acc.Content, "Hello world")
	}

	// Add final chunk with finish reason
	usage := &TokenUsage{TotalTokens: 100}
	acc.Add(StreamChunk{
		FinishReason: "stop",
		Usage:        usage,
	})

	if acc.FinishReason != "stop" {
		t.Errorf("FinishReason = %q, want %q", acc.FinishReason, "stop")
	}
	if acc.Usage == nil || acc.Usage.TotalTokens != 100 {
		t.Errorf("Usage not set correctly")
	}
}

func TestStreamAccumulator_AddToolCalls(t *testing.T) {
	acc := NewStreamAccumulator()

	// Add first chunk with tool call start
	acc.Add(StreamChunk{
		ToolCalls: []ToolCall{
			{ID: "call_1", Name: "get_weather"},
		},
	})

	if len(acc.ToolCalls) != 1 {
		t.Fatalf("Expected 1 tool call, got %d", len(acc.ToolCalls))
	}

	// Add chunk with arguments
	acc.Add(StreamChunk{
		ToolCalls: []ToolCall{
			{ID: "call_1", Arguments: `{"location":`},
		},
	})

	// Add more arguments
	acc.Add(StreamChunk{
		ToolCalls: []ToolCall{
			{ID: "call_1", Arguments: `"SF"}`},
		},
	})

	tc := acc.ToolCalls["call_1"]
	if tc == nil {
		t.Fatal("Tool call not found")
	}
	if tc.Name != "get_weather" {
		t.Errorf("Name = %q, want %q", tc.Name, "get_weather")
	}
	if tc.Arguments != `{"location":"SF"}` {
		t.Errorf("Arguments = %q, want %q", tc.Arguments, `{"location":"SF"}`)
	}
}

func TestStreamAccumulator_ToResponse(t *testing.T) {
	acc := NewStreamAccumulator()
	acc.Add(StreamChunk{Delta: "Hello"})
	acc.Add(StreamChunk{
		ToolCalls: []ToolCall{
			{ID: "call_1", Name: "test", Arguments: "{}"},
		},
	})
	acc.Add(StreamChunk{
		FinishReason: "stop",
		Usage:        &TokenUsage{TotalTokens: 100},
	})

	response := acc.ToResponse()

	if response.Content != "Hello" {
		t.Errorf("Content = %q, want %q", response.Content, "Hello")
	}
	if len(response.ToolCalls) != 1 {
		t.Errorf("Expected 1 tool call, got %d", len(response.ToolCalls))
	}
	if response.FinishReason != "stop" {
		t.Errorf("FinishReason = %q, want %q", response.FinishReason, "stop")
	}
	if response.Usage.TotalTokens != 100 {
		t.Errorf("Usage.TotalTokens = %d, want 100", response.Usage.TotalTokens)
	}
}

func TestStreamAccumulator_Reset(t *testing.T) {
	acc := NewStreamAccumulator()
	acc.Add(StreamChunk{Delta: "Hello"})
	acc.Add(StreamChunk{
		FinishReason: "stop",
		Usage:        &TokenUsage{TotalTokens: 100},
	})

	acc.Reset()

	if acc.Content != "" {
		t.Errorf("Content not reset")
	}
	if len(acc.ToolCalls) != 0 {
		t.Errorf("ToolCalls not reset")
	}
	if acc.FinishReason != "" {
		t.Errorf("FinishReason not reset")
	}
	if acc.Usage != nil {
		t.Errorf("Usage not reset")
	}
}

func TestStreamAccumulator_IsComplete(t *testing.T) {
	acc := NewStreamAccumulator()

	if acc.IsComplete() {
		t.Error("Should not be complete initially")
	}

	acc.Add(StreamChunk{Delta: "Hello"})

	if acc.IsComplete() {
		t.Error("Should not be complete without finish reason")
	}

	acc.Add(StreamChunk{FinishReason: "stop"})

	if !acc.IsComplete() {
		t.Error("Should be complete after finish reason")
	}
}

func TestStreamAccumulator_IgnoreEmptyToolCallID(t *testing.T) {
	acc := NewStreamAccumulator()

	// Tool calls without ID should be ignored
	acc.Add(StreamChunk{
		ToolCalls: []ToolCall{
			{ID: "", Name: "test"},
		},
	})

	if len(acc.ToolCalls) != 0 {
		t.Error("Tool call without ID should be ignored")
	}
}

func TestStreamAccumulator_MultipleToolCalls(t *testing.T) {
	acc := NewStreamAccumulator()

	acc.Add(StreamChunk{
		ToolCalls: []ToolCall{
			{ID: "call_1", Name: "tool1", Arguments: "{}"},
			{ID: "call_2", Name: "tool2", Arguments: "{}"},
		},
	})

	if len(acc.ToolCalls) != 2 {
		t.Errorf("Expected 2 tool calls, got %d", len(acc.ToolCalls))
	}

	response := acc.ToResponse()
	if len(response.ToolCalls) != 2 {
		t.Errorf("Expected 2 tool calls in response, got %d", len(response.ToolCalls))
	}
}

func TestNewStreamAccumulator(t *testing.T) {
	acc := NewStreamAccumulator()

	if acc.Content != "" {
		t.Error("Content should be empty")
	}
	if acc.ToolCalls == nil {
		t.Error("ToolCalls map should be initialized")
	}
	if acc.FinishReason != "" {
		t.Error("FinishReason should be empty")
	}
	if acc.Usage != nil {
		t.Error("Usage should be nil")
	}
}

func TestStreamAccumulator_ToResponse_EmptyUsage(t *testing.T) {
	acc := NewStreamAccumulator()
	acc.Add(StreamChunk{Delta: "Hello"})

	response := acc.ToResponse()

	// Should have zero-valued usage, not nil
	expected := TokenUsage{}
	if !reflect.DeepEqual(response.Usage, expected) {
		t.Errorf("Usage = %v, want %v", response.Usage, expected)
	}
}
