// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Zero Root AI

package contract_test

import (
	"testing"

	graphragv1 "github.com/zeroroot-ai/sdk/api/gen/gibson/graphrag/v1"
)

func TestGraphragV1_AllMessagesRoundTrip(t *testing.T) {
	roundTripPackage(t, "gibson.graphrag.v1")
}

// TestDiscoveryResult_FindingEvidence pins the field of the parser evidence.
// Field 10 held graphrag.v1.Evidence and stays reserved. The new field has its
// own number, and its items use the one evidence shape, types.v1.Evidence.
func TestDiscoveryResult_FindingEvidence(t *testing.T) {
	md := (&graphragv1.DiscoveryResult{}).ProtoReflect().Descriptor()
	if md.Fields().ByNumber(10) != nil || md.Fields().ByName("evidence") != nil {
		t.Fatal("DiscoveryResult field 10 or the name evidence exists again")
	}
	if !md.ReservedRanges().Has(10) || !md.ReservedNames().Has("evidence") {
		t.Fatal("DiscoveryResult field 10 and the name evidence must stay reserved")
	}
	fd := md.Fields().ByName("finding_evidence")
	if fd == nil || fd.Number() != 23 || !fd.IsList() {
		t.Fatalf("finding_evidence is not the repeated field 23: %v", fd)
	}
	item := fd.Message()
	if got := string(item.Fields().ByName("evidence").Message().FullName()); got != "gibson.types.v1.Evidence" {
		t.Fatalf("FindingEvidence.evidence is %s, want gibson.types.v1.Evidence", got)
	}
}
