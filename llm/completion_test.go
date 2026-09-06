// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Zero Root AI

package llm

import (
	"reflect"
	"testing"
)

func TestWithTemperature(t *testing.T) {
	req := &CompletionRequest{}
	opt := WithTemperature(0.7)
	opt(req)

	if req.Temperature == nil {
		t.Fatal("Temperature not set")
	}
	if *req.Temperature != 0.7 {
		t.Errorf("Temperature = %v, want 0.7", *req.Temperature)
	}
}

func TestWithMaxTokens(t *testing.T) {
	req := &CompletionRequest{}
	opt := WithMaxTokens(1000)
	opt(req)

	if req.MaxTokens == nil {
		t.Fatal("MaxTokens not set")
	}
	if *req.MaxTokens != 1000 {
		t.Errorf("MaxTokens = %v, want 1000", *req.MaxTokens)
	}
}

func TestWithTopP(t *testing.T) {
	req := &CompletionRequest{}
	opt := WithTopP(0.9)
	opt(req)

	if req.TopP == nil {
		t.Fatal("TopP not set")
	}
	if *req.TopP != 0.9 {
		t.Errorf("TopP = %v, want 0.9", *req.TopP)
	}
}

func TestWithStopSequences(t *testing.T) {
	req := &CompletionRequest{}
	opt := WithStopSequences("STOP", "END")
	opt(req)

	want := []string{"STOP", "END"}
	if !reflect.DeepEqual(req.Stop, want) {
		t.Errorf("Stop = %v, want %v", req.Stop, want)
	}
}

func TestWithTools(t *testing.T) {
	req := &CompletionRequest{}
	tools := []ToolDef{
		{Name: "tool1", Description: "Test tool 1"},
		{Name: "tool2", Description: "Test tool 2"},
	}
	opt := WithTools(tools...)
	opt(req)

	if !reflect.DeepEqual(req.Tools, tools) {
		t.Errorf("Tools = %v, want %v", req.Tools, tools)
	}
}

func TestNewCompletionRequest(t *testing.T) {
	messages := []Message{
		{Role: RoleUser, Content: "Hello"},
	}

	req := NewCompletionRequest(messages,
		WithTemperature(0.7),
		WithMaxTokens(1000),
	)

	if !reflect.DeepEqual(req.Messages, messages) {
		t.Errorf("Messages not set correctly")
	}
	if req.Temperature == nil || *req.Temperature != 0.7 {
		t.Errorf("Temperature not set correctly")
	}
	if req.MaxTokens == nil || *req.MaxTokens != 1000 {
		t.Errorf("MaxTokens not set correctly")
	}
}

func TestCompletionRequest_ApplyOptions(t *testing.T) {
	req := &CompletionRequest{}
	req.ApplyOptions(
		WithTemperature(0.8),
		WithMaxTokens(500),
		WithTopP(0.95),
	)

	if req.Temperature == nil || *req.Temperature != 0.8 {
		t.Error("Temperature not applied")
	}
	if req.MaxTokens == nil || *req.MaxTokens != 500 {
		t.Error("MaxTokens not applied")
	}
	if req.TopP == nil || *req.TopP != 0.95 {
		t.Error("TopP not applied")
	}
}

func TestCompletionResponse_HasContent(t *testing.T) {
	tests := []struct {
		name     string
		response CompletionResponse
		want     bool
	}{
		{
			name:     "has content",
			response: CompletionResponse{Content: "Hello"},
			want:     true,
		},
		{
			name:     "no content",
			response: CompletionResponse{},
			want:     false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.response.HasContent(); got != tt.want {
				t.Errorf("HasContent() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestCompletionResponse_HasToolCalls(t *testing.T) {
	tests := []struct {
		name     string
		response CompletionResponse
		want     bool
	}{
		{
			name: "has tool calls",
			response: CompletionResponse{
				ToolCalls: []ToolCall{{ID: "1", Name: "test"}},
			},
			want: true,
		},
		{
			name:     "no tool calls",
			response: CompletionResponse{},
			want:     false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.response.HasToolCalls(); got != tt.want {
				t.Errorf("HasToolCalls() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestCompletionResponse_IsComplete(t *testing.T) {
	tests := []struct {
		name     string
		response CompletionResponse
		want     bool
	}{
		{
			name:     "finished normally",
			response: CompletionResponse{FinishReason: "stop"},
			want:     true,
		},
		{
			name:     "finished with tool calls",
			response: CompletionResponse{FinishReason: "tool_calls"},
			want:     true,
		},
		{
			name:     "truncated by length",
			response: CompletionResponse{FinishReason: "length"},
			want:     false,
		},
		{
			name:     "content filter",
			response: CompletionResponse{FinishReason: "content_filter"},
			want:     false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.response.IsComplete(); got != tt.want {
				t.Errorf("IsComplete() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestTokenUsage_Add(t *testing.T) {
	u1 := TokenUsage{
		InputTokens:  100,
		OutputTokens: 50,
		TotalTokens:  150,
	}
	u2 := TokenUsage{
		InputTokens:  200,
		OutputTokens: 75,
		TotalTokens:  275,
	}

	result := u1.Add(u2)

	want := TokenUsage{
		InputTokens:  300,
		OutputTokens: 125,
		TotalTokens:  425,
	}

	if result != want {
		t.Errorf("Add() = %v, want %v", result, want)
	}
}

func TestTokenUsage_AddZero(t *testing.T) {
	u1 := TokenUsage{
		InputTokens:  100,
		OutputTokens: 50,
		TotalTokens:  150,
	}
	u2 := TokenUsage{}

	result := u1.Add(u2)

	if result != u1 {
		t.Errorf("Add(zero) = %v, want %v", result, u1)
	}
}
