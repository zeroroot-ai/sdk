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
