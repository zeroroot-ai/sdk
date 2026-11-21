// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Zero Root AI

package contract_test

import (
	"strings"
	"testing"

	"google.golang.org/protobuf/proto"

	graphpb "github.com/zeroroot-ai/sdk/api/gen/gibson/graph/v1"
)

func TestGraphV1_Finding_FixLoopFields(t *testing.T) {
	v := newValidator(t)
	confidence := 0.8

	t.Run("a finding with every fix loop field is accepted and survives the wire", func(t *testing.T) {
		in := &graphpb.Finding{
			Id:              "f-1",
			LifecycleStatus: graphpb.FindingLifecycleStatus_FINDING_LIFECYCLE_STATUS_FIXING,
			Assets:          []string{"host-1", "svc-api"},
			Priority:        "P2",
			PriorityRule:    "exposed-and-exploited",
			Confidence:      &confidence,
			MergeRequestUrl: "https://git.example.com/org/repo/-/merge_requests/7",
		}
		if err := v.Validate(in); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		wire, err := proto.Marshal(in)
		if err != nil {
			t.Fatalf("marshal: %v", err)
		}
		out := &graphpb.Finding{}
		if err := proto.Unmarshal(wire, out); err != nil {
			t.Fatalf("unmarshal: %v", err)
		}
		if !proto.Equal(in, out) {
			t.Errorf("round trip = %v, want %v", out, in)
		}
	})

	t.Run("a finding with no fix loop field is accepted", func(t *testing.T) {
		if err := v.Validate(&graphpb.Finding{Id: "f-1"}); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	t.Run("an unknown confidence differs from zero confidence", func(t *testing.T) {
		zero := 0.0
		if (&graphpb.Finding{}).Confidence != nil {
			t.Error("an unset confidence is not nil")
		}
		if err := v.Validate(&graphpb.Finding{Confidence: &zero}); err != nil {
			t.Errorf("a zero confidence is refused: %v", err)
		}
	})

	rejected := map[string]struct {
		f    *graphpb.Finding
		want string
	}{
		"priority P5":      {&graphpb.Finding{Priority: "P5"}, "priority"},
		"priority text":    {&graphpb.Finding{Priority: "urgent"}, "priority"},
		"confidence above": {&graphpb.Finding{Confidence: ptrFloat64(1.5)}, "confidence"},
		"confidence below": {&graphpb.Finding{Confidence: ptrFloat64(-0.1)}, "confidence"},
	}
	for name, tc := range rejected {
		t.Run("rejects "+name, func(t *testing.T) {
			err := v.Validate(tc.f)
			if err == nil {
				t.Fatal("expected a validation error, got nil")
			}
			if !strings.Contains(err.Error(), tc.want) {
				t.Errorf("error should name %q: %v", tc.want, err)
			}
		})
	}
}

func ptrFloat64(v float64) *float64 { return &v }
