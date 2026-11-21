// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Zero Root AI

package contract_test

import (
	"testing"

	"google.golang.org/protobuf/proto"

	harnesspb "github.com/zeroroot-ai/sdk/api/gen/gibson/harness/v1"
)

// TestHarnessV1_SearchToolsCandidate_ToolClass: an agent picks a tool by its
// tool class, so the class must survive the wire.
func TestHarnessV1_SearchToolsCandidate_ToolClass(t *testing.T) {
	in := &harnesspb.SearchToolsResponse{Candidates: []*harnesspb.SearchToolsCandidate{{
		Id: "mcp:gitlab:create_merge_request", Source: "mcp", Connector: "gitlab",
		Tool: "create_merge_request", ToolClass: "git-host",
	}}}
	wire, err := proto.Marshal(in)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	out := &harnesspb.SearchToolsResponse{}
	if err := proto.Unmarshal(wire, out); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if got := out.GetCandidates()[0].GetToolClass(); got != "git-host" {
		t.Errorf("tool_class = %q, want git-host", got)
	}
}
