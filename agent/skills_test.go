// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Zero Root AI

package agent

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	protolib "google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/structpb"

	"github.com/zeroroot-ai/sdk/openresponses"
)

type fakeInvoker struct {
	ToolCaller
	calls []string
	fail  string
}

func (f *fakeInvoker) CallToolProto(
	_ context.Context, name string, req protolib.Message, resp protolib.Message, _ ...ToolCallOption,
) error {
	if name != InvokeToolName {
		return errors.New("wrong tool " + name)
	}
	id := req.(*structpb.Struct).GetFields()["id"].GetStringValue()
	f.calls = append(f.calls, id)
	if id == f.fail {
		return errors.New("skill not found")
	}
	body, _ := structpb.NewStruct(map[string]any{"body": "instructions of " + id})
	protolib.Merge(resp, body)
	return nil
}

func TestLoadPinnedSkills(t *testing.T) {
	f := &fakeInvoker{}
	items, err := LoadPinnedSkills(context.Background(), f, []string{"skill:a", "skill:b"})
	require.NoError(t, err)
	assert.Equal(t, []string{"skill:a", "skill:b"}, f.calls)
	require.Len(t, items, 4)

	call, ok := items[0].(openresponses.FunctionCall)
	require.True(t, ok)
	assert.Equal(t, "invoke_tool", call.Name)
	assert.JSONEq(t, `{"id":"skill:a"}`, call.Arguments)
	out, ok := items[1].(openresponses.FunctionCallOutput)
	require.True(t, ok)
	assert.Equal(t, call.CallID, out.CallID)
	var body map[string]string
	require.NoError(t, json.Unmarshal([]byte(out.Output), &body))
	assert.Equal(t, "instructions of skill:a", body["body"])
	assert.NotEqual(t, call.CallID, items[2].(openresponses.FunctionCall).CallID)
}

func TestLoadPinnedSkillsFailureStopsTheRun(t *testing.T) {
	f := &fakeInvoker{fail: "skill:b"}
	items, err := LoadPinnedSkills(context.Background(), f, []string{"skill:a", "skill:b", "skill:c"})
	require.Error(t, err)
	assert.Nil(t, items)
	assert.Contains(t, err.Error(), `"skill:b"`)
	assert.Equal(t, []string{"skill:a", "skill:b"}, f.calls)
}

func TestLoadPinnedSkillsNone(t *testing.T) {
	items, err := LoadPinnedSkills(context.Background(), &fakeInvoker{}, nil)
	require.NoError(t, err)
	assert.Empty(t, items)
}
