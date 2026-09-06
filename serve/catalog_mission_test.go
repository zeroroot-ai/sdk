// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Zero Root AI

package serve

import (
	"strings"
	"testing"

	"github.com/zeroroot-ai/sdk/mission"
)

// The seven parameters the checked-in scan mission declares (gibson#1688).
// None of them names a host: the runtime target binds from targetID alone.
func scanParams() map[string]string {
	return map[string]string{
		"application":   "customer-portal",
		"repositoryUrl": "https://gitlab.com/examplebank/customer-portal",
		"ref":           "refs/heads/main",
		"commit":        "0b26431",
		"pipelineId":    "4711",
		"pipelineUrl":   "https://gitlab.com/examplebank/customer-portal/-/pipelines/4711",
		"imageRef":      "registry.gitlab.com/examplebank/customer-portal@sha256:abc",
	}
}

// TestBuildCreateMissionRequest_CatalogMissionSendsNoGraph is the property that
// makes ADR-0018 true on the wire: naming a checked-in mission must send the
// NAME, not a copy of the graph. json.Marshal(nil) yields "null" — four
// non-empty bytes — so a builder that serialised unconditionally would populate
// mission_definition_json alongside catalog_mission and the daemon would refuse
// the call. The bug would look like "the catalog path does not work" rather
// than "we sent both".
func TestBuildCreateMissionRequest_CatalogMissionSendsNoGraph(t *testing.T) {
	req, err := buildCreateMissionRequest(nil, nil, "target-1", &mission.CreateMissionOpts{
		CatalogMission: "scan",
		CatalogParams:  scanParams(),
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(req.MissionDefinitionJson) != 0 {
		t.Errorf("mission_definition_json must be empty when a catalog mission is named, got %q",
			string(req.MissionDefinitionJson))
	}
	if req.CatalogMission != "scan" {
		t.Errorf("CatalogMission = %q, want %q", req.CatalogMission, "scan")
	}
	if req.TargetId != "target-1" {
		t.Errorf("TargetId = %q, want %q", req.TargetId, "target-1")
	}
}

// TestBuildCreateMissionRequest_CarriesEveryParameter checks the parameters by
// name rather than by map equality. The map is closed daemon-side — an
// unrecognised key is refused — so a key silently dropped here surfaces as
// "missing parameter" from a render, pointing at the definition rather than at
// the caller that did send it.
func TestBuildCreateMissionRequest_CarriesEveryParameter(t *testing.T) {
	want := scanParams()
	req, err := buildCreateMissionRequest(nil, nil, "target-1", &mission.CreateMissionOpts{
		CatalogMission: "scan",
		CatalogParams:  want,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(req.CatalogParams) != len(want) {
		t.Fatalf("CatalogParams has %d keys, want %d", len(req.CatalogParams), len(want))
	}
	for k, v := range want {
		got, ok := req.CatalogParams[k]
		if !ok {
			t.Errorf("CatalogParams is missing %q", k)
			continue
		}
		if got != v {
			t.Errorf("CatalogParams[%q] = %q, want %q", k, got, v)
		}
	}
}

// TestBuildCreateMissionRequest_NoParameterNamesTheHost guards the third
// acceptance criterion of gibson#1688. The target binds from targetID, never
// from a parameter, so a scan can only run against a target the tenant
// registered. This test fails the day someone adds a host-shaped parameter to
// the scan mission's declared set, which is the moment to argue about it rather
// than after it ships.
func TestBuildCreateMissionRequest_NoParameterNamesTheHost(t *testing.T) {
	req, err := buildCreateMissionRequest(nil, nil, "target-1", &mission.CreateMissionOpts{
		CatalogMission: "scan",
		CatalogParams:  scanParams(),
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	for _, banned := range []string{"host", "hostname", "target", "targetId", "url", "address"} {
		if _, ok := req.CatalogParams[banned]; ok {
			t.Errorf("catalog_params carries %q; the runtime target must bind from target_id alone", banned)
		}
	}
}

// TestBuildCreateMissionRequest_BothInputsRefusedLocally: the daemon also
// refuses this with InvalidArgument, but an error raised there names the wire.
// Refusing here names the argument to drop, and the assertion on the message is
// the point — a generic failure would send the caller to the daemon logs.
func TestBuildCreateMissionRequest_BothInputsRefusedLocally(t *testing.T) {
	_, err := buildCreateMissionRequest(nil, map[string]any{"nodes": nil}, "target-1",
		&mission.CreateMissionOpts{CatalogMission: "scan"})
	if err == nil {
		t.Fatal("expected an error when both a mission definition and CatalogMission are supplied")
	}
	if !strings.Contains(err.Error(), "pass nil for missionDef") {
		t.Errorf("error should tell the caller which argument to drop, got: %v", err)
	}
}

// TestBuildCreateMissionRequest_GraphPathUnchanged: the catalog fields are a
// second input to one route, not a new route, so the existing path must behave
// exactly as before — including opts == nil, which every current caller uses.
func TestBuildCreateMissionRequest_GraphPathUnchanged(t *testing.T) {
	req, err := buildCreateMissionRequest(nil, map[string]any{"name": "recon"}, "target-1", nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(req.MissionDefinitionJson) == 0 {
		t.Error("mission_definition_json must carry the serialized graph when no catalog mission is named")
	}
	if req.CatalogMission != "" {
		t.Errorf("CatalogMission = %q, want empty", req.CatalogMission)
	}
	if req.CatalogParams != nil {
		t.Errorf("CatalogParams = %v, want nil", req.CatalogParams)
	}
}

// TestBuildCreateMissionRequest_ParamsWithoutCatalogMissionSendNoName: params
// alone must not imply a catalog mission. The daemon refuses a request with
// neither input, and that refusal should stay reachable rather than being
// masked by the SDK inventing a name.
func TestBuildCreateMissionRequest_ParamsWithoutCatalogMissionSendNoName(t *testing.T) {
	req, err := buildCreateMissionRequest(nil, map[string]any{"name": "recon"}, "target-1",
		&mission.CreateMissionOpts{CatalogParams: scanParams()})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if req.CatalogMission != "" {
		t.Errorf("CatalogMission = %q, want empty when only params were supplied", req.CatalogMission)
	}
	if len(req.MissionDefinitionJson) == 0 {
		t.Error("the graph path must still be taken when no catalog mission is named")
	}
}
