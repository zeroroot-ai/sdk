// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Zero Root AI

package eval

import (
	"context"
	"errors"
	"fmt"

	"github.com/zeroroot-ai/sdk/llm"
)

// mockLLMProvider implements LLMProvider for testing.
// This is a shared test helper used across multiple test files.
type mockLLMProvider struct {
	responses     []*llm.CompletionResponse
	callCount     int
	shouldError   bool
	errorAfterN   int
	recordedCalls [][]llm.Message

	// Legacy fields for backward compatibility
	response *llm.CompletionResponse
	err      error
}

func (m *mockLLMProvider) Complete(ctx context.Context, messages []llm.Message, opts ...llm.CompletionOption) (*llm.CompletionResponse, error) {
	m.recordedCalls = append(m.recordedCalls, messages)

	// Support legacy single response/err pattern
	if m.err != nil {
		return nil, m.err
	}
	if m.response != nil {
		return m.response, nil
	}

	// Support new multi-response pattern
	if m.shouldError && (m.errorAfterN == 0 || m.callCount >= m.errorAfterN) {
		m.callCount++
		return nil, errors.New("mock LLM error")
	}

	if m.callCount >= len(m.responses) {
		m.callCount++
		return nil, fmt.Errorf("no more mock responses available (call %d)", m.callCount)
	}

	resp := m.responses[m.callCount]
	m.callCount++
	return resp, nil
}
