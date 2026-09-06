// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Zero Root AI

package llm

// CompletionRequest represents a request for LLM completion.
type CompletionRequest struct {
	// Messages contains the conversation history.
	Messages []Message

	// Temperature controls randomness in the output (0.0 to 2.0).
	// Lower values make output more focused and deterministic.
	// Higher values make output more creative and random.
	Temperature *float64

	// MaxTokens limits the maximum number of tokens to generate.
	MaxTokens *int

	// TopP controls nucleus sampling (0.0 to 1.0).
	// Only tokens with cumulative probability up to TopP are considered.
	TopP *float64

	// Stop contains sequences that will stop generation when encountered.
	Stop []string

	// Tools contains tool definitions available for the model to use.
	Tools []ToolDef
}

// CompletionResponse represents a response from an LLM completion.
type CompletionResponse struct {
	// Content is the generated text content.
	Content string

	// ToolCalls contains tool invocations requested by the model.
	ToolCalls []ToolCall

	// FinishReason indicates why the generation stopped.
	// Common values: "stop", "length", "tool_calls", "content_filter"
	FinishReason string

	// Usage contains token usage statistics.
	Usage TokenUsage
}

// TokenUsage tracks token consumption for a request.
type TokenUsage struct {
	// InputTokens is the number of tokens in the input/prompt.
	InputTokens int

	// OutputTokens is the number of tokens generated in the response.
	OutputTokens int

	// TotalTokens is the sum of input and output tokens.
	TotalTokens int
}

// CompletionOption is a functional option for configuring CompletionRequest.
type CompletionOption func(*CompletionRequest)

// WithTemperature sets the temperature for the completion request.
// Temperature controls randomness (0.0 to 2.0).
func WithTemperature(t float64) CompletionOption {
	return func(r *CompletionRequest) {
		r.Temperature = &t
	}
}

// WithMaxTokens sets the maximum number of tokens to generate.
func WithMaxTokens(n int) CompletionOption {
	return func(r *CompletionRequest) {
		r.MaxTokens = &n
	}
}

// WithTopP sets the nucleus sampling parameter.
// TopP controls diversity via nucleus sampling (0.0 to 1.0).
func WithTopP(p float64) CompletionOption {
	return func(r *CompletionRequest) {
		r.TopP = &p
	}
}

// WithStopSequences sets sequences that will stop generation.
func WithStopSequences(stops ...string) CompletionOption {
	return func(r *CompletionRequest) {
		r.Stop = stops
	}
}

// WithTools sets the available tools for the completion request.
func WithTools(tools ...ToolDef) CompletionOption {
	return func(r *CompletionRequest) {
		r.Tools = tools
	}
}

// ApplyOptions applies a set of options to the completion request.
func (r *CompletionRequest) ApplyOptions(opts ...CompletionOption) {
	for _, opt := range opts {
		if opt != nil {
			opt(r)
		}
	}
}

// NewCompletionRequest creates a new CompletionRequest with the given messages and options.
func NewCompletionRequest(messages []Message, opts ...CompletionOption) *CompletionRequest {
	req := &CompletionRequest{
		Messages: messages,
	}
	req.ApplyOptions(opts...)
	return req
}

// HasContent returns true if the response contains text content.
func (r *CompletionResponse) HasContent() bool {
	return r.Content != ""
}

// HasToolCalls returns true if the response contains tool calls.
func (r *CompletionResponse) HasToolCalls() bool {
	return len(r.ToolCalls) > 0
}

// IsComplete returns true if generation finished normally (not truncated).
func (r *CompletionResponse) IsComplete() bool {
	return r.FinishReason == "stop" || r.FinishReason == "tool_calls"
}

// Add combines two TokenUsage instances.
func (u TokenUsage) Add(other TokenUsage) TokenUsage {
	return TokenUsage{
		InputTokens:  u.InputTokens + other.InputTokens,
		OutputTokens: u.OutputTokens + other.OutputTokens,
		TotalTokens:  u.TotalTokens + other.TotalTokens,
	}
}
