// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Zero Root AI

package contract_test

import (
	"bytes"
	"testing"

	"google.golang.org/protobuf/proto"

	componentv1 "github.com/zeroroot-ai/sdk/api/gen/gibson/component/v1"
)

// TestLLMMessage_ToolHistoryRoundTrip pins the two message shapes a
// multi-turn tool conversation replays: the assistant turn that carries the
// requested calls, and the tool turn that answers one of them.
func TestLLMMessage_ToolHistoryRoundTrip(t *testing.T) {
	assistant := &componentv1.LLMMessage{
		Role:    "assistant",
		Content: "",
		ToolCalls: []*componentv1.ToolCallResult{
			{Id: "call_1", Name: "http_get", ArgumentsJson: `{"url":"https://example.test"}`},
			{Id: "call_2", Name: "dns_lookup", ArgumentsJson: `{"host":"example.test"}`},
		},
	}

	wire, err := proto.Marshal(assistant)
	if err != nil {
		t.Fatalf("marshal assistant turn: %v", err)
	}
	var gotAssistant componentv1.LLMMessage
	if err := proto.Unmarshal(wire, &gotAssistant); err != nil {
		t.Fatalf("unmarshal assistant turn: %v", err)
	}
	if !proto.Equal(assistant, &gotAssistant) {
		t.Fatalf("assistant turn did not round-trip:\n want %v\n  got %v", assistant, &gotAssistant)
	}
	if len(gotAssistant.GetToolCalls()) != 2 {
		t.Fatalf("want 2 tool calls, got %d", len(gotAssistant.GetToolCalls()))
	}
	if gotAssistant.GetToolCalls()[0].GetId() != "call_1" {
		t.Fatalf("tool call id lost: %q", gotAssistant.GetToolCalls()[0].GetId())
	}

	toolResult := &componentv1.LLMMessage{
		Role:       "tool",
		Content:    `{"status":200}`,
		ToolCallId: "call_1",
	}
	wire, err = proto.Marshal(toolResult)
	if err != nil {
		t.Fatalf("marshal tool turn: %v", err)
	}
	var gotTool componentv1.LLMMessage
	if err := proto.Unmarshal(wire, &gotTool); err != nil {
		t.Fatalf("unmarshal tool turn: %v", err)
	}
	if gotTool.GetToolCallId() != "call_1" {
		t.Fatalf("tool_call_id lost: %q", gotTool.GetToolCallId())
	}
	if gotTool.GetContent() != `{"status":200}` {
		t.Fatalf("content lost: %q", gotTool.GetContent())
	}
}

// TestLLMMessage_PlainMessageWireUnchanged proves the additive fields cost a
// pre-existing role+content message nothing on the wire: an unset repeated
// field and an empty string emit no bytes, so an old client and a new client
// still produce byte-identical encodings for the same plain message.
func TestLLMMessage_PlainMessageWireUnchanged(t *testing.T) {
	plain := &componentv1.LLMMessage{Role: "user", Content: "hello"}

	got, err := proto.Marshal(plain)
	if err != nil {
		t.Fatalf("marshal plain message: %v", err)
	}

	// Hand-encoded pre-change bytes: field 1 (role) and field 2 (content),
	// both length-delimited.
	want := []byte{
		0x0a, 0x04, 'u', 's', 'e', 'r',
		0x12, 0x05, 'h', 'e', 'l', 'l', 'o',
	}
	if !bytes.Equal(got, want) {
		t.Fatalf("plain message wire format changed:\n want % x\n  got % x", want, got)
	}
}
