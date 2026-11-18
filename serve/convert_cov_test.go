// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Zero Root AI

package serve

import (
	"testing"

	"github.com/zeroroot-ai/sdk/agent"
	typespb "github.com/zeroroot-ai/sdk/api/gen/gibson/types/v1"
	"github.com/zeroroot-ai/sdk/finding"
	"github.com/zeroroot-ai/sdk/llm"
	"github.com/zeroroot-ai/sdk/mission"
)

func TestStatusAndScopeConversions(t *testing.T) {
	for _, s := range []mission.MissionStatus{
		mission.MissionStatusSubmitted, mission.MissionStatusWorking, mission.MissionStatusPaused,
		mission.MissionStatusCompleted, mission.MissionStatusFailed, mission.MissionStatusCanceled, mission.MissionStatusInputRequired, "other",
	} {
		_ = missionStatusToProto(s)
	}
	for _, s := range []agent.RunScope{agent.RunScopeAll, agent.RunScopePrevious, agent.RunScopeUnspecified} {
		_ = runScopeToProto(s)
	}
	pf := findingFilterToProto(finding.Filter{MissionID: "m", AgentName: "a", Severities: []finding.Severity{"high"}, Status: "open"})
	if pf == nil {
		t.Fatal("findingFilterToProto returned nil")
	}
	if fs := findingsFromProto([]*typespb.Finding{{Id: "f1", Title: "t"}, nil}); len(fs) == 0 {
		t.Fatal("findingsFromProto dropped the finding")
	}
	_ = runSummariesFromProto([]*typespb.MissionRunSummary{{}})
}

func TestCallbackTokenTracker(t *testing.T) {
	tr := NewCallbackTokenTracker()
	tr.Add("primary", llm.TokenUsage{InputTokens: 1, OutputTokens: 2, TotalTokens: 3})
	tr.Add("primary", llm.TokenUsage{InputTokens: 1, OutputTokens: 1, TotalTokens: 2})
	if got := tr.BySlot("primary").TotalTokens; got != 5 {
		t.Fatalf("BySlot total = %d, want 5", got)
	}
	if got := tr.Total().TotalTokens; got != 5 {
		t.Fatalf("Total = %d, want 5", got)
	}
	if len(tr.Slots()) != 1 {
		t.Fatalf("Slots = %v", tr.Slots())
	}
	tr.Reset()
	if tr.Total().TotalTokens != 0 {
		t.Fatal("Reset must clear the totals")
	}
}
