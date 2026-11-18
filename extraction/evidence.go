// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Zero Root AI

package extraction

import (
	"errors"
	"fmt"

	graphragpb "github.com/zeroroot-ai/sdk/api/gen/gibson/graphrag/v1"
	typespb "github.com/zeroroot-ai/sdk/api/gen/gibson/types/v1"
)

// Evidence is one evidence item that a parser read from tool output. A parser
// fills these four fields and nothing else. It never sets the tier and it never
// sets the tool call ID: the daemon sets both, and it ignores a value that a
// parser sends. This is why the type has no field for them.
type Evidence struct {
	Type     typespb.EvidenceType
	Title    string
	Content  string
	Metadata map[string]string
}

// AddEvidence adds an evidence item to a DiscoveryResult for a finding of the
// same result. The finding is named by the id of an entry of r.Findings.
//
// It returns an error when r is nil, when no finding of r has the id, or when
// the item has no title and no content. The item is stored as
// DiscoveryResult.finding_evidence, in the one evidence shape
// (gibson.types.v1.Evidence).
func AddEvidence(r *graphragpb.DiscoveryResult, findingID string, e Evidence) error {
	if r == nil {
		return errors.New("add evidence: the discovery result is nil")
	}
	if findingID == "" {
		return errors.New("add evidence: the finding id is empty")
	}
	if e.Title == "" && e.Content == "" {
		return errors.New("add evidence: the item has no title and no content")
	}
	found := false
	for _, f := range r.GetFindings() {
		if f.GetId() == findingID {
			found = true
			break
		}
	}
	if !found {
		return fmt.Errorf("add evidence: the result has no finding with id %q", findingID)
	}
	r.FindingEvidence = append(r.FindingEvidence, &graphragpb.FindingEvidence{
		FindingId: findingID,
		Evidence: &typespb.Evidence{
			Type:     e.Type,
			Title:    e.Title,
			Content:  e.Content,
			Metadata: copyMetadata(e.Metadata),
		},
	})
	return nil
}

func copyMetadata(m map[string]string) map[string]string {
	if len(m) == 0 {
		return nil
	}
	out := make(map[string]string, len(m))
	for k, v := range m {
		out[k] = v
	}
	return out
}
