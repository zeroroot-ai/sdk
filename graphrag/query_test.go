// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Zero Root AI

package graphrag

import (
	"testing"

	"google.golang.org/protobuf/proto"

	graphragpb "github.com/zeroroot-ai/sdk/api/gen/gibson/graphrag/v1"
)

// TestGraphQuery_RelationshipTraversal_WireRoundTrip: the SDK search primitive
// (sdk#72) queries the shared graph by type, by relationship, and by
// text/relevance. Type and text/relevance already round-trip (node_types,
// text, embedding); this covers the relationship-traversal fields added for
// sdk#72 — from_node_id anchors the traversal, relationship_type restricts
// it — so a query built with them survives an actual proto
// marshal/unmarshal, not just struct construction.
func TestGraphQuery_RelationshipTraversal_WireRoundTrip(t *testing.T) {
	q := &graphragpb.GraphQuery{
		Text:             "unauthenticated access",
		NodeTypes:        []string{"Finding", "Hypothesis"},
		TopK:             10,
		FromNodeId:       "host-10.0.0.5",
		RelationshipType: "RESOLVES_TO",
	}

	wire, err := proto.Marshal(q)
	if err != nil {
		t.Fatalf("marshal failed: %v", err)
	}

	got := &graphragpb.GraphQuery{}
	if err := proto.Unmarshal(wire, got); err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}

	if got.GetFromNodeId() != "host-10.0.0.5" {
		t.Fatalf("from_node_id lost in round trip: %+v", got)
	}
	if got.GetRelationshipType() != "RESOLVES_TO" {
		t.Fatalf("relationship_type lost in round trip: %+v", got)
	}
	if got.GetText() != "unauthenticated access" {
		t.Fatalf("text lost in round trip: %+v", got)
	}
	if len(got.GetNodeTypes()) != 2 {
		t.Fatalf("node_types lost in round trip: %+v", got.GetNodeTypes())
	}
}

// TestGraphQuery_RelationshipTraversal_OptionalFieldsStayEmpty: a query with
// no relationship traversal (the existing type/text/relevance search) must
// not acquire phantom values for the new fields.
func TestGraphQuery_RelationshipTraversal_OptionalFieldsStayEmpty(t *testing.T) {
	q := &graphragpb.GraphQuery{Text: "plain search", TopK: 5}
	if q.GetFromNodeId() != "" || q.GetRelationshipType() != "" {
		t.Fatalf("expected empty relationship fields by default, got %+v", q)
	}
}
