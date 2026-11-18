// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Zero Root AI

package llm

// TokenUsage tracks token consumption for a request.
type TokenUsage struct {
	// InputTokens is the number of tokens in the input/prompt.
	InputTokens int

	// OutputTokens is the number of tokens generated in the response.
	OutputTokens int

	// TotalTokens is the sum of input and output tokens.
	TotalTokens int
}

// Add combines two TokenUsage instances.
func (u TokenUsage) Add(other TokenUsage) TokenUsage {
	return TokenUsage{
		InputTokens:  u.InputTokens + other.InputTokens,
		OutputTokens: u.OutputTokens + other.OutputTokens,
		TotalTokens:  u.TotalTokens + other.TotalTokens,
	}
}
