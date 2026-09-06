// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Zero Root AI

package toolrunner

import (
	"bytes"
	"context"
	"encoding/base64"
	"errors"
	"strings"
	"testing"

	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/wrapperspb"

	"github.com/zeroroot-ai/sdk/tool"
	"github.com/zeroroot-ai/sdk/types"
)

// fakeTool is a minimal tool.Tool implementation for table-driven tests.
type fakeTool struct {
	exec func(context.Context, proto.Message) (proto.Message, error)
}

func (f *fakeTool) Name() string                              { return "fake" }
func (f *fakeTool) Version() string                           { return "0.0.0" }
func (f *fakeTool) Description() string                       { return "fake" }
func (f *fakeTool) Tags() []string                            { return nil }
func (f *fakeTool) InputMessageType() string                  { return "google.protobuf.StringValue" }
func (f *fakeTool) OutputMessageType() string                 { return "google.protobuf.StringValue" }
func (f *fakeTool) Health(context.Context) types.HealthStatus { return types.NewHealthyStatus("ok") }
func (f *fakeTool) ExecuteProto(ctx context.Context, in proto.Message) (proto.Message, error) {
	return f.exec(ctx, in)
}

func encInput(t *testing.T, msg proto.Message) string {
	t.Helper()
	b, err := protojson.Marshal(msg)
	if err != nil {
		t.Fatalf("encInput marshal: %v", err)
	}
	return base64.StdEncoding.EncodeToString(b)
}

func TestRun_HappyPath_EmitsOutputMarker(t *testing.T) {
	tt := &fakeTool{
		exec: func(_ context.Context, in proto.Message) (proto.Message, error) {
			s := in.(*wrapperspb.StringValue)
			return wrapperspb.String("hello, " + s.GetValue()), nil
		},
	}
	env := []string{
		"GIBSON_TOOL_INPUT_B64=" + encInput(t, wrapperspb.String("world")),
	}
	var stdout bytes.Buffer
	exit, err := run(&stdout, nil, env, tt)
	if err != nil || exit != 0 {
		t.Fatalf("run() = (%d, %v); want (0, nil)", exit, err)
	}
	out := stdout.String()
	if !strings.Contains(out, markerOutput) {
		t.Fatalf("output missing marker: %q", out)
	}
	// Decode the payload and verify content.
	idx := strings.Index(out, markerOutput)
	payload := strings.TrimSuffix(out[idx+len(markerOutput):], "\n")
	raw, err := base64.StdEncoding.DecodeString(payload)
	if err != nil {
		t.Fatalf("decode payload: %v", err)
	}
	got := &wrapperspb.StringValue{}
	if err := protojson.Unmarshal(raw, got); err != nil {
		t.Fatalf("unmarshal payload: %v", err)
	}
	if got.GetValue() != "hello, world" {
		t.Fatalf("payload = %q; want %q", got.GetValue(), "hello, world")
	}
}

func TestRun_MissingEnv_ReturnsInputParseExit(t *testing.T) {
	tt := &fakeTool{exec: func(context.Context, proto.Message) (proto.Message, error) { return nil, nil }}
	exit, err := run(nil, nil, nil, tt)
	if exit != exitInputParse || err == nil {
		t.Fatalf("run() = (%d, %v); want (%d, non-nil)", exit, err, exitInputParse)
	}
	if !strings.Contains(err.Error(), "GIBSON_TOOL_INPUT_B64") {
		t.Fatalf("error %q does not mention env var", err)
	}
}

func TestRun_MalformedBase64_ReturnsInputParseExit(t *testing.T) {
	tt := &fakeTool{exec: func(context.Context, proto.Message) (proto.Message, error) { return nil, nil }}
	env := []string{"GIBSON_TOOL_INPUT_B64=!!!not-base64!!!"}
	exit, _ := run(nil, nil, env, tt)
	if exit != exitInputParse {
		t.Fatalf("exit = %d; want %d", exit, exitInputParse)
	}
}

func TestRun_ExecuteError_ReturnsExecuteExit(t *testing.T) {
	tt := &fakeTool{exec: func(context.Context, proto.Message) (proto.Message, error) {
		return nil, errors.New("boom")
	}}
	env := []string{"GIBSON_TOOL_INPUT_B64=" + encInput(t, wrapperspb.String("x"))}
	exit, err := run(nil, nil, env, tt)
	if exit != exitExecute || err == nil || !strings.Contains(err.Error(), "boom") {
		t.Fatalf("run() = (%d, %v); want (%d, contains 'boom')", exit, err, exitExecute)
	}
}

func TestRun_NilOutput_ReturnsExecuteExit(t *testing.T) {
	tt := &fakeTool{exec: func(context.Context, proto.Message) (proto.Message, error) {
		return nil, nil
	}}
	env := []string{"GIBSON_TOOL_INPUT_B64=" + encInput(t, wrapperspb.String("x"))}
	exit, err := run(nil, nil, env, tt)
	if exit != exitExecute || err == nil {
		t.Fatalf("run() = (%d, %v); want (%d, non-nil)", exit, err, exitExecute)
	}
}

func TestRun_UnknownInputType_ReturnsInputParseExit(t *testing.T) {
	type unknownTool struct{ *fakeTool }
	ut := &unknownTool{fakeTool: &fakeTool{exec: func(context.Context, proto.Message) (proto.Message, error) { return nil, nil }}}
	// override input message type to one that isn't registered
	tt := &fakeToolBadType{fakeTool: ut.fakeTool}
	env := []string{"GIBSON_TOOL_INPUT_B64=" + encInput(t, wrapperspb.String("x"))}
	exit, err := run(nil, nil, env, tt)
	if exit != exitInputParse || err == nil {
		t.Fatalf("run() = (%d, %v); want (%d, non-nil)", exit, err, exitInputParse)
	}
}

type fakeToolBadType struct{ *fakeTool }

func (f *fakeToolBadType) InputMessageType() string  { return "no.such.Type" }
func (f *fakeToolBadType) OutputMessageType() string { return "no.such.Type" }

// Ensure tool.Tool interface is satisfied (compile-time check).
var _ tool.Tool = (*fakeTool)(nil)
var _ tool.Tool = (*fakeToolBadType)(nil)
