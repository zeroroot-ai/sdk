// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Zero Root AI

package contract_test

import (
	"testing"

	"google.golang.org/protobuf/proto"

	componentv1 "github.com/zeroroot-ai/sdk/api/gen/gibson/component/v1"
	harnessv1 "github.com/zeroroot-ai/sdk/api/gen/gibson/harness/v1"
	typesv1 "github.com/zeroroot-ai/sdk/api/gen/gibson/types/v1"
)

// TestResolveModelResponse_RoundTrip pins the fields of the answer that the
// daemon and the SDK share. The field numbers are in model-client-contract.md.
func TestResolveModelResponse_RoundTrip(t *testing.T) {
	headers := map[string]string{"x-gibson-tenant": "t1", "x-gibson-provider": "p1"}
	for name, msg := range map[string]proto.Message{
		"component": &componentv1.ResolveModelResponse{
			Model: "m", Provider: "p", GatewayUrl: "https://gw", RouteHeaders: headers,
			ContextWindow: 200000, MaxOutputTokens: 8192, GatewayCaPem: "PEM",
		},
		"harness": &harnessv1.ResolveModelResponse{
			Model: "m", Provider: "p", GatewayUrl: "https://gw", RouteHeaders: headers,
			ContextWindow: 200000, MaxOutputTokens: 8192, GatewayCaPem: "PEM",
		},
	} {
		wire, err := proto.Marshal(msg)
		if err != nil {
			t.Fatalf("%s: marshal: %v", name, err)
		}
		got := msg.ProtoReflect().New().Interface()
		if err := proto.Unmarshal(wire, got); err != nil {
			t.Fatalf("%s: unmarshal: %v", name, err)
		}
		if !proto.Equal(msg, got) {
			t.Fatalf("%s: did not round-trip:\n want %v\n  got %v", name, msg, got)
		}
	}
}

func TestTaskPinnedSkillsIsGoneAndItsNumberStaysReserved(t *testing.T) {
	// The pinned Agent Skill ids ride in the Gibson run envelope of the A2A
	// message. Field 7 of Task must not come back with another meaning.
	task := typesv1.File_gibson_types_v1_types_proto.Messages().ByName("Task")
	if task.Fields().ByName("pinned_skills") != nil || task.Fields().ByNumber(7) != nil {
		t.Fatal("Task field 7 or pinned_skills exists again")
	}
	if !task.ReservedRanges().Has(7) || !task.ReservedNames().Has("pinned_skills") {
		t.Fatal("Task field 7 and the name pinned_skills must stay reserved")
	}
}
