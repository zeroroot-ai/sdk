// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Zero Root AI

package llm

import (
	"encoding/json"
	"testing"
)

func TestToolDef_Validate(t *testing.T) {
	tests := []struct {
		name    string
		tool    ToolDef
		wantErr bool
	}{
		{
			name: "valid tool",
			tool: ToolDef{
				Name:        "get_weather",
				Description: "Get current weather",
				Parameters:  map[string]any{"type": "object"},
			},
			wantErr: false,
		},
		{
			name: "empty name",
			tool: ToolDef{
				Description: "Test",
				Parameters:  map[string]any{"type": "object"},
			},
			wantErr: true,
		},
		{
			name: "empty description",
			tool: ToolDef{
				Name:       "test",
				Parameters: map[string]any{"type": "object"},
			},
			wantErr: true,
		},
		{
			name: "nil parameters",
			tool: ToolDef{
				Name:        "test",
				Description: "Test tool",
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.tool.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestToolCall_ParseArguments(t *testing.T) {
	type Args struct {
		Location string `json:"location"`
		Unit     string `json:"unit"`
	}

	tests := []struct {
		name     string
		call     ToolCall
		wantArgs Args
		wantErr  bool
	}{
		{
			name: "valid arguments",
			call: ToolCall{
				ID:        "1",
				Name:      "get_weather",
				Arguments: `{"location":"San Francisco","unit":"celsius"}`,
			},
			wantArgs: Args{Location: "San Francisco", Unit: "celsius"},
			wantErr:  false,
		},
		{
			name: "empty arguments",
			call: ToolCall{
				ID:        "1",
				Name:      "test",
				Arguments: "",
			},
			wantErr: true,
		},
		{
			name: "invalid json",
			call: ToolCall{
				ID:        "1",
				Name:      "test",
				Arguments: `{invalid}`,
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var args Args
			err := tt.call.ParseArguments(&args)
			if (err != nil) != tt.wantErr {
				t.Errorf("ParseArguments() error = %v, wantErr %v", err, tt.wantErr)
			}
			if !tt.wantErr && args != tt.wantArgs {
				t.Errorf("ParseArguments() args = %v, want %v", args, tt.wantArgs)
			}
		})
	}
}

func TestToolCall_Validate(t *testing.T) {
	tests := []struct {
		name    string
		call    ToolCall
		wantErr bool
	}{
		{
			name: "valid tool call",
			call: ToolCall{
				ID:        "call_1",
				Name:      "get_weather",
				Arguments: `{"location":"SF"}`,
			},
			wantErr: false,
		},
		{
			name: "empty ID",
			call: ToolCall{
				Name:      "test",
				Arguments: `{}`,
			},
			wantErr: true,
		},
		{
			name: "empty name",
			call: ToolCall{
				ID:        "call_1",
				Arguments: `{}`,
			},
			wantErr: true,
		},
		{
			name: "empty arguments",
			call: ToolCall{
				ID:   "call_1",
				Name: "test",
			},
			wantErr: true,
		},
		{
			name: "invalid JSON arguments",
			call: ToolCall{
				ID:        "call_1",
				Name:      "test",
				Arguments: `{invalid}`,
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.call.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestNewToolResult(t *testing.T) {
	result := NewToolResult("call_1", "success")

	if result.ToolCallID != "call_1" {
		t.Errorf("ToolCallID = %q, want %q", result.ToolCallID, "call_1")
	}
	if result.Content != "success" {
		t.Errorf("Content = %q, want %q", result.Content, "success")
	}
	if result.IsError {
		t.Error("IsError should be false")
	}
}

func TestNewToolError(t *testing.T) {
	result := NewToolError("call_1", "error occurred")

	if result.ToolCallID != "call_1" {
		t.Errorf("ToolCallID = %q, want %q", result.ToolCallID, "call_1")
	}
	if result.Content != "error occurred" {
		t.Errorf("Content = %q, want %q", result.Content, "error occurred")
	}
	if !result.IsError {
		t.Error("IsError should be true")
	}
}

func TestToolResult_Validate(t *testing.T) {
	tests := []struct {
		name    string
		result  ToolResult
		wantErr bool
	}{
		{
			name: "valid result",
			result: ToolResult{
				ToolCallID: "call_1",
				Content:    "success",
			},
			wantErr: false,
		},
		{
			name: "empty tool call ID",
			result: ToolResult{
				Content: "success",
			},
			wantErr: true,
		},
		{
			name: "empty content",
			result: ToolResult{
				ToolCallID: "call_1",
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.result.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestToolResult_SetJSONContent(t *testing.T) {
	type WeatherData struct {
		Temperature int    `json:"temperature"`
		Condition   string `json:"condition"`
	}

	result := &ToolResult{ToolCallID: "call_1"}
	data := WeatherData{Temperature: 72, Condition: "sunny"}

	err := result.SetJSONContent(data)
	if err != nil {
		t.Fatalf("SetJSONContent() error = %v", err)
	}

	// Verify content is valid JSON
	var decoded WeatherData
	if err := json.Unmarshal([]byte(result.Content), &decoded); err != nil {
		t.Fatalf("Content is not valid JSON: %v", err)
	}

	if decoded != data {
		t.Errorf("Decoded data = %v, want %v", decoded, data)
	}
}

func TestToolResult_ParseContent(t *testing.T) {
	type WeatherData struct {
		Temperature int    `json:"temperature"`
		Condition   string `json:"condition"`
	}

	tests := []struct {
		name    string
		result  ToolResult
		want    WeatherData
		wantErr bool
	}{
		{
			name: "valid JSON content",
			result: ToolResult{
				ToolCallID: "call_1",
				Content:    `{"temperature":72,"condition":"sunny"}`,
			},
			want:    WeatherData{Temperature: 72, Condition: "sunny"},
			wantErr: false,
		},
		{
			name: "empty content",
			result: ToolResult{
				ToolCallID: "call_1",
				Content:    "",
			},
			wantErr: true,
		},
		{
			name: "invalid JSON",
			result: ToolResult{
				ToolCallID: "call_1",
				Content:    `{invalid}`,
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var data WeatherData
			err := tt.result.ParseContent(&data)
			if (err != nil) != tt.wantErr {
				t.Errorf("ParseContent() error = %v, wantErr %v", err, tt.wantErr)
			}
			if !tt.wantErr && data != tt.want {
				t.Errorf("ParseContent() data = %v, want %v", data, tt.want)
			}
		})
	}
}

func TestToolChoice_String(t *testing.T) {
	tests := []struct {
		choice ToolChoice
		want   string
	}{
		{ToolChoiceNone, "none"},
		{ToolChoiceAuto, "auto"},
		{ToolChoiceRequired, "required"},
	}

	for _, tt := range tests {
		t.Run(string(tt.choice), func(t *testing.T) {
			if got := tt.choice.String(); got != tt.want {
				t.Errorf("String() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestToolChoice_IsValid(t *testing.T) {
	tests := []struct {
		choice ToolChoice
		want   bool
	}{
		{ToolChoiceNone, true},
		{ToolChoiceAuto, true},
		{ToolChoiceRequired, true},
		{ToolChoice("invalid"), false},
		{ToolChoice(""), false},
	}

	for _, tt := range tests {
		t.Run(string(tt.choice), func(t *testing.T) {
			if got := tt.choice.IsValid(); got != tt.want {
				t.Errorf("IsValid() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestToolResult_SetJSONContent_Error(t *testing.T) {
	result := &ToolResult{ToolCallID: "call_1"}

	// Channels cannot be marshaled to JSON
	err := result.SetJSONContent(make(chan int))
	if err == nil {
		t.Error("SetJSONContent() should return error for unmarshallable types")
	}
}
