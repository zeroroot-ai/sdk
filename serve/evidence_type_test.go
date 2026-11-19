// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Zero Root AI

package serve

import (
	"testing"

	typespb "github.com/zeroroot-ai/sdk/api/gen/gibson/types/v1"
	"github.com/zeroroot-ai/sdk/finding"
)

// TestEvidenceType_RoundTrip proves that each SDK evidence type survives the
// trip to the proto type and back.
func TestEvidenceType_RoundTrip(t *testing.T) {
	all := []finding.EvidenceType{
		finding.EvidenceHTTPRequest, finding.EvidenceHTTPResponse, finding.EvidenceScreenshot,
		finding.EvidenceLog, finding.EvidencePayload, finding.EvidenceConversation,
		finding.EvidenceCodeSnippet, finding.EvidenceNetworkTrace, finding.EvidenceOther,
	}
	for _, et := range all {
		t.Run(string(et), func(t *testing.T) {
			p := evidenceTypeToProto(et)
			if p == typespb.EvidenceType_EVIDENCE_TYPE_UNSPECIFIED {
				t.Fatalf("%q maps to UNSPECIFIED", et)
			}
			if got := evidenceTypeFromProto(p); got != et {
				t.Errorf("round trip = %q, want %q", got, et)
			}
		})
	}
}
