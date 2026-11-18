// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Zero Root AI

package llm

import "testing"

func TestTokenUsageAdd(t *testing.T) {
	a := TokenUsage{InputTokens: 10, OutputTokens: 3, TotalTokens: 13}
	b := TokenUsage{InputTokens: 1, OutputTokens: 2, TotalTokens: 3}
	got := a.Add(b)
	want := TokenUsage{InputTokens: 11, OutputTokens: 5, TotalTokens: 16}
	if got != want {
		t.Fatalf("Add = %+v, want %+v", got, want)
	}
	if a != (TokenUsage{InputTokens: 10, OutputTokens: 3, TotalTokens: 13}) {
		t.Fatal("Add changed its receiver")
	}
	if (TokenUsage{}).Add(TokenUsage{}) != (TokenUsage{}) {
		t.Fatal("zero plus zero is not zero")
	}
}
