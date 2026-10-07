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
