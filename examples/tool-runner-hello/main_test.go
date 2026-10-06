// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Zero Root AI

package main

import (
	"context"
	"testing"

	"google.golang.org/protobuf/types/known/wrapperspb"
)

func TestHelloTool(t *testing.T) {
	var h helloTool
	if h.Name() != "hello" || h.Version() == "" || h.Description() == "" || len(h.Tags()) == 0 {
		t.Fatal("the tool must describe itself")
	}
	if h.InputMessageType() != "google.protobuf.StringValue" || h.OutputMessageType() != "google.protobuf.StringValue" {
		t.Fatal("unexpected message types")
	}
	out, err := h.ExecuteProto(context.Background(), wrapperspb.String("world"))
	if err != nil {
		t.Fatalf("ExecuteProto: %v", err)
	}
	if got := out.(*wrapperspb.StringValue).GetValue(); got != "hello, world" {
		t.Fatalf("ExecuteProto = %q", got)
	}
}
