// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Zero Root AI

package serve

import (
	"testing"
	"time"

	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/durationpb"

	harnesspb "github.com/zeroroot-ai/sdk/api/gen/gibson/harness/v1"
	missionpb "github.com/zeroroot-ai/sdk/api/gen/gibson/mission/v1"
	"github.com/zeroroot-ai/sdk/mission"
)

// everyConstraint sets each field of gibson.mission.v1.MissionConstraints to a
// value that is not the zero value.
func everyConstraint() *missionpb.MissionConstraints {
	return &missionpb.MissionConstraints{
		MaxDuration:       durationpb.New(90 * time.Minute),
		MaxTokens:         250000,
		MaxCost:           12.5,
		MaxFindings:       40,
		SeverityThreshold: "high",
		RequireEvidence:   true,
		BlockedTools:      []string{"shell"},
		BlockedDomains:    []string{"prod.example.com"},
		MaxTurnsPerAgent:  30,
		AllowedTechniques: []string{"T1595"},
		BlockedTechniques: []string{"T1485"},
		MaxTokensPerCall:  4096,
	}
}

// TestEveryConstraintSetsEachField keeps the fixture complete. A new field of
// MissionConstraints fails this test until the fixture sets it, so the test
// below always covers each field.
func TestEveryConstraintSetsEachField(t *testing.T) {
	msg := everyConstraint().ProtoReflect()
	fields := msg.Descriptor().Fields()
	for i := range fields.Len() {
		fd := fields.Get(i)
		if !msg.Has(fd) {
			t.Errorf("everyConstraint does not set field %s", fd.Name())
		}
	}
}

// TestBuildCreateMissionRequest_CarriesEveryConstraint proves sdk#180: a
// mission that an agent creates through the callback carries each field of
// the platform constraint type on the wire, in canonical_constraints.
func TestBuildCreateMissionRequest_CarriesEveryConstraint(t *testing.T) {
	want := everyConstraint()
	req, err := buildCreateMissionRequest(nil, map[string]any{"name": "m"}, "target-1", &mission.CreateMissionOpts{
		Constraints: want,
	})
	if err != nil {
		t.Fatalf("buildCreateMissionRequest: %v", err)
	}

	wire, err := proto.Marshal(req)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var got harnesspb.CreateMissionRequest
	if err := proto.Unmarshal(wire, &got); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if !proto.Equal(got.GetCanonicalConstraints(), want) {
		t.Errorf("canonical_constraints on the wire = %v, want %v", got.GetCanonicalConstraints(), want)
	}
}

// TestBuildCreateMissionRequest_NoConstraintsSendsNone proves that options
// with no constraints leave the constraint field empty.
func TestBuildCreateMissionRequest_NoConstraintsSendsNone(t *testing.T) {
	req, err := buildCreateMissionRequest(nil, map[string]any{"name": "m"}, "target-1", &mission.CreateMissionOpts{Name: "n"})
	if err != nil {
		t.Fatalf("buildCreateMissionRequest: %v", err)
	}
	if req.GetCanonicalConstraints() != nil {
		t.Errorf("canonical_constraints is set: %v", req.GetCanonicalConstraints())
	}
}
