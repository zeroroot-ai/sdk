// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Zero Root AI

package llm

// StreamChunk represents a chunk of data received during streaming completion.
type StreamChunk struct {
	// Delta contains the incremental text content for this chunk.
	// This should be appended to previous chunks to build the full response.
	Delta string

	// ToolCalls contains incremental tool call information.
	// Tool calls may be split across multiple chunks and need to be accumulated.
	ToolCalls []ToolCall

	// FinishReason indicates why the generation stopped.
	// Only set on the final chunk. Common values: "stop", "length", "tool_calls", "content_filter"
	FinishReason string

	// Usage contains token usage statistics.
	// Typically only set on the final chunk.
	Usage *TokenUsage
}

// IsFinal returns true if this is the final chunk in the stream.
func (c *StreamChunk) IsFinal() bool {
	return c.FinishReason != ""
}

// HasContent returns true if this chunk contains text content.
func (c *StreamChunk) HasContent() bool {
	return c.Delta != ""
}

// HasToolCalls returns true if this chunk contains tool call information.
func (c *StreamChunk) HasToolCalls() bool {
	return len(c.ToolCalls) > 0
}

// HasUsage returns true if this chunk contains usage statistics.
func (c *StreamChunk) HasUsage() bool {
	return c.Usage != nil
}

// StreamAccumulator accumulates chunks from a streaming response.
type StreamAccumulator struct {
	// Content holds the accumulated text content.
	Content string

	// ToolCalls holds the accumulated tool calls.
	// Tool calls are indexed by ID to handle incremental updates.
	ToolCalls map[string]*ToolCall

	// FinishReason holds the final reason for completion.
	FinishReason string

	// Usage holds the final token usage statistics.
	Usage *TokenUsage
}

// NewStreamAccumulator creates a new accumulator for streaming responses.
func NewStreamAccumulator() *StreamAccumulator {
	return &StreamAccumulator{
		ToolCalls: make(map[string]*ToolCall),
	}
}

// Add processes a new chunk and updates the accumulator state.
func (a *StreamAccumulator) Add(chunk StreamChunk) {
	// Accumulate content
	if chunk.Delta != "" {
		a.Content += chunk.Delta
	}

	// Accumulate tool calls
	for _, tc := range chunk.ToolCalls {
		if tc.ID == "" {
			continue
		}

		existing, ok := a.ToolCalls[tc.ID]
		if !ok {
			// New tool call
			tcCopy := tc
			a.ToolCalls[tc.ID] = &tcCopy
		} else {
			// Update existing tool call
			if tc.Name != "" {
				existing.Name = tc.Name
			}
			existing.Arguments += tc.Arguments
		}
	}

	// Update finish reason and usage on final chunk
	if chunk.FinishReason != "" {
		a.FinishReason = chunk.FinishReason
	}
	if chunk.Usage != nil {
		a.Usage = chunk.Usage
	}
}

// ToResponse converts the accumulated state to a CompletionResponse.
func (a *StreamAccumulator) ToResponse() CompletionResponse {
	toolCalls := make([]ToolCall, 0, len(a.ToolCalls))
	for _, tc := range a.ToolCalls {
		toolCalls = append(toolCalls, *tc)
	}

	usage := TokenUsage{}
	if a.Usage != nil {
		usage = *a.Usage
	}

	return CompletionResponse{
		Content:      a.Content,
		ToolCalls:    toolCalls,
		FinishReason: a.FinishReason,
		Usage:        usage,
	}
}

// Reset clears the accumulator state for reuse.
func (a *StreamAccumulator) Reset() {
	a.Content = ""
	a.ToolCalls = make(map[string]*ToolCall)
	a.FinishReason = ""
	a.Usage = nil
}

// IsComplete returns true if the accumulator has received a finish reason.
func (a *StreamAccumulator) IsComplete() bool {
	return a.FinishReason != ""
}
