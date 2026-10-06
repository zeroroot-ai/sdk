// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Zero Root AI

package agent

import (
	"fmt"
	"reflect"
	"sort"
	"testing"
)

// Harness must satisfy every capability group it is composed of. These are
// compile-time assertions: dropping a group from Harness, or renaming one,
// fails the build here rather than silently shrinking the interface.
//
// See ADR-0162.
var (
	_ LLMCaller       = Harness(nil)
	_ ToolCaller      = Harness(nil)
	_ PluginCaller    = Harness(nil)
	_ Delegator       = Harness(nil)
	_ WorldEmitter    = Harness(nil)
	_ WorldReader     = Harness(nil)
	_ Planner         = Harness(nil)
	_ WorkspaceAccess = Harness(nil)
	_ KnowledgeReader = Harness(nil)
	_ MissionManager  = Harness(nil)
)

// TestHarnessMethodSetUnchanged pins the full method set.
//
// The grouping refactor is meant to be purely structural: same methods, arranged
// into named clusters. This list is the contract that says so. A method added
// here without a matching group is the drift ADR-0162 exists to prevent — if you
// are updating this list, check the new method joined a group rather than being
// bolted on flat.
func TestHarnessMethodSetUnchanged(t *testing.T) {
	want := []string{
		"Authorize",
		"CallToolProto", "CallToolProtoStream", "ListTools", "QueueToolWork", "ToolResults",
		"CancelMission", "CreateMission", "GetMissionResults", "GetMissionStatus",
		"ListMissions", "RunMission", "WaitForMission",
		"Complete", "CompleteStructured", "CompleteStructuredAny", "CompleteWithTools", "Stream",
		"DelegateToAgent", "ListAgents",
		"ListPlugins", "QueryPlugin",
		"Logger", "Mission", "Target", "TokenUsage", "Tracer",
		"Observe", "PlaceBet", "SubmitFinding",
		"PlanContext", "ReportStepHints",
		"Workspace", "Workspaces",
		"WorldView",
		// KnowledgeReader, added deliberately — the nine reads a dispatched
		// agent needs to see what earlier work established (sdk#489), plus the
		// one lifecycle traversal the search-shaped reads cannot answer
		// (ApplicationFindings, sdk#537).
		"QueryNodes", "FindSimilarAttacks", "GetAttackChains",
		"FindSimilarFindings", "GetRelatedFindings",
		"GetFindings", "GetRunFindings", "GetMissionRunHistory",
		"ApplicationFindings",
	}
	sort.Strings(want)

	tp := reflect.TypeOf((*Harness)(nil)).Elem()
	got := make([]string, 0, tp.NumMethod())
	for i := range tp.NumMethod() {
		got = append(got, tp.Method(i).Name)
	}
	sort.Strings(got)

	if len(got) != len(want) {
		t.Fatalf("Harness has %d methods, expected %d\ngot:  %v\nwant: %v", len(got), len(want), got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("method set differs at %d: got %q, want %q", i, got[i], want[i])
		}
	}
}

// TestGroupsAreDisjoint guards the one way this arrangement can rot quietly: the
// same method appearing in two groups still compiles, still satisfies Harness,
// and leaves nobody able to say which group owns it.
func TestGroupsAreDisjoint(t *testing.T) {
	groups := map[string]reflect.Type{
		"LLMCaller":       reflect.TypeOf((*LLMCaller)(nil)).Elem(),
		"ToolCaller":      reflect.TypeOf((*ToolCaller)(nil)).Elem(),
		"PluginCaller":    reflect.TypeOf((*PluginCaller)(nil)).Elem(),
		"Delegator":       reflect.TypeOf((*Delegator)(nil)).Elem(),
		"WorldEmitter":    reflect.TypeOf((*WorldEmitter)(nil)).Elem(),
		"WorldReader":     reflect.TypeOf((*WorldReader)(nil)).Elem(),
		"Planner":         reflect.TypeOf((*Planner)(nil)).Elem(),
		"WorkspaceAccess": reflect.TypeOf((*WorkspaceAccess)(nil)).Elem(),
		"KnowledgeReader": reflect.TypeOf((*KnowledgeReader)(nil)).Elem(),
		"MissionManager":  reflect.TypeOf((*MissionManager)(nil)).Elem(),
	}
	owner := map[string]string{}
	for name, tp := range groups {
		for i := range tp.NumMethod() {
			m := tp.Method(i).Name
			if prev, dup := owner[m]; dup {
				t.Errorf("method %q is in both %s and %s; a method belongs to exactly one group", m, prev, name)
				continue
			}
			owner[m] = name
		}
	}
}

// TestKnowledgeReadsAreUnavailableNotEmpty pins the contract that makes
// ErrKnowledgeUnavailable worth having.
//
// A harness with no platform behind it must report that it CANNOT read, never
// that the tenant knows nothing. Returning (nil, nil) here would compile, pass
// any test that only checks err == nil, and leave an agent concluding "no prior
// findings for this target" when nothing was ever looked up. That is a silent
// false negative in a security product, and it is the exact failure the same
// principle already fixed on the daemon's WorldView seam.
// wrapRead keeps wrapcheck satisfied without changing what the test asserts:
// nil stays nil, and a wrapped error still matches errors.Is.
func wrapRead(err error) error {
	if err == nil {
		return nil
	}
	return fmt.Errorf("knowledge read: %w", err)
}
