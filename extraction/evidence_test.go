// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Zero Root AI

package extraction

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"

	graphragpb "github.com/zeroroot-ai/sdk/api/gen/gibson/graphrag/v1"
	typespb "github.com/zeroroot-ai/sdk/api/gen/gibson/types/v1"
)

func resultWithFinding(id string) *graphragpb.DiscoveryResult {
	return &graphragpb.DiscoveryResult{Findings: []*graphragpb.Finding{{Id: proto.String(id), Title: "Open admin port", Severity: "high"}}}
}

func TestAddEvidenceFillsOnlyWhatAParserKnows(t *testing.T) {
	r := resultWithFinding("f-1")
	meta := map[string]string{"template": "exposed-panel"}
	require.NoError(t, AddEvidence(r, "f-1", Evidence{
		Type: typespb.EvidenceType_EVIDENCE_TYPE_REQUEST, Title: "matched request", Content: "GET /admin", Metadata: meta,
	}))
	require.NoError(t, AddEvidence(r, "f-1", Evidence{Type: typespb.EvidenceType_EVIDENCE_TYPE_LOG, Content: "line"}))

	require.Len(t, r.GetFindingEvidence(), 2)
	got := r.GetFindingEvidence()[0]
	assert.Equal(t, "f-1", got.GetFindingId())
	assert.Equal(t, "matched request", got.GetEvidence().GetTitle())
	assert.Equal(t, "GET /admin", got.GetEvidence().GetContent())
	assert.Equal(t, typespb.EvidenceType_EVIDENCE_TYPE_REQUEST, got.GetEvidence().GetType())
	assert.Equal(t, meta, got.GetEvidence().GetMetadata())
	assert.Equal(t, typespb.EvidenceTier_EVIDENCE_TIER_UNSPECIFIED, got.GetEvidence().GetTier(), "the parser never sets the tier")
	assert.Empty(t, got.GetEvidence().GetToolCallId(), "the parser never sets the tool call id")

	// The item holds a copy of the metadata.
	meta["template"] = "changed"
	assert.Equal(t, "exposed-panel", got.GetEvidence().GetMetadata()["template"])
	assert.Nil(t, r.GetFindingEvidence()[1].GetEvidence().GetMetadata())
}

func TestAddEvidenceRefusals(t *testing.T) {
	require.Error(t, AddEvidence(nil, "f-1", Evidence{Content: "x"}))
	r := resultWithFinding("f-1")
	require.ErrorContains(t, AddEvidence(r, "", Evidence{Content: "x"}), "finding id")
	require.ErrorContains(t, AddEvidence(r, "f-1", Evidence{}), "no title and no content")
	require.ErrorContains(t, AddEvidence(r, "f-2", Evidence{Content: "x"}), "no finding")
	assert.Empty(t, r.GetFindingEvidence(), "a refused item is not added")
}

func TestFindingEvidenceSurvivesTheWire(t *testing.T) {
	r := resultWithFinding("f-1")
	require.NoError(t, AddEvidence(r, "f-1", Evidence{Title: "t", Content: "c"}))
	raw, err := proto.Marshal(r)
	require.NoError(t, err)
	var back graphragpb.DiscoveryResult
	require.NoError(t, proto.Unmarshal(raw, &back))
	assert.True(t, proto.Equal(r, &back))
}
