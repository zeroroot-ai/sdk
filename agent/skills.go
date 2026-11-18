// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Zero Root AI

package agent

import (
	"context"
	"encoding/json"
	"fmt"

	"google.golang.org/protobuf/encoding/protojson"
	protolib "google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/structpb"

	"github.com/zeroroot-ai/sdk/openresponses"
)

// InvokeToolName is the name of the meta-tool that runs a catalog tool by its
// canonical id. A skill:<name> id passes through it like any other id.
const InvokeToolName = "invoke_tool"

// LoadPinnedSkills loads each pinned Agent Skill with invoke_tool and returns
// the items that belong at the start of the conversation. Each skill gives
// one function_call item and one function_call_output item, in the order of
// ids.
//
// A skill that fails to load stops the call. The error names the skill. The
// run must not start with a part of its pinned skills.
func LoadPinnedSkills(ctx context.Context, tools ToolCaller, ids []string) ([]openresponses.Item, error) {
	items := make([]openresponses.Item, 0, 2*len(ids))
	for i, id := range ids {
		args, err := json.Marshal(map[string]string{"id": id})
		if err != nil {
			return nil, fmt.Errorf("load pinned skill %q: encode arguments: %w", id, err)
		}
		in := &structpb.Struct{}
		if err := protojson.Unmarshal(args, in); err != nil {
			return nil, fmt.Errorf("load pinned skill %q: build request: %w", id, err)
		}
		out := &structpb.Struct{}
		if err := tools.CallToolProto(ctx, InvokeToolName, protolib.Message(in), protolib.Message(out)); err != nil {
			return nil, fmt.Errorf("load pinned skill %q: %w", id, err)
		}
		body, err := protojson.Marshal(out)
		if err != nil {
			return nil, fmt.Errorf("load pinned skill %q: encode result: %w", id, err)
		}
		callID := fmt.Sprintf("pinned_skill_%d", i)
		items = append(items,
			openresponses.FunctionCall{
				CallID: callID, Name: InvokeToolName, Arguments: string(args),
				Status: openresponses.StatusCompleted,
			},
			openresponses.FunctionCallOutput{CallID: callID, Output: string(body)},
		)
	}
	return items, nil
}
