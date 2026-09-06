// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Zero Root AI

package finding

import (
	"errors"
	"fmt"
)

// ComplianceMapping links a Finding to a specific compliance framework
// control (e.g., SOC2 CC7.1, NIST AI RMF MEASURE.2.7, MITRE ATLAS
// AML.T0051). Multiple mappings per finding are allowed — one finding
// often evidences controls from multiple frameworks simultaneously.
//
// Framework and ControlID are required. Rationale (a human-readable
// explanation of why the finding evidences this control) and EvidenceRef
// (a pointer to supporting evidence within the finding or elsewhere)
// are optional.
//
// This type is the author-side mirror of the ComplianceMapping value
// object already defined in the foundation taxonomy and emitted on the
// Finding proto. The SDK type is the Go-struct form that agents and
// tools populate; the proto type is what crosses the wire.
type ComplianceMapping struct {
	// Framework is the compliance framework identifier.
	// Convention: SCREAMING_SNAKE_CASE matching the rule catalog
	// frameworks block (SOC2, NIST_AI_RMF, MITRE_ATLAS, MITRE_ATTACK,
	// PLATFORM, or a tenant-scoped custom framework).
	Framework string `json:"framework"`

	// ControlID is the control identifier within the framework.
	// Format varies by framework: SOC2 uses "CC7.1", NIST AI RMF uses
	// "MEASURE.2.7", MITRE ATLAS uses "AML.T0051", etc. The value is
	// copied verbatim into SARIF exports and Cypher queries.
	ControlID string `json:"control_id"`

	// Rationale is an optional human-readable explanation of why this
	// finding evidences the control. Auditors read this to understand
	// the mapping without needing Gibson domain knowledge.
	Rationale string `json:"rationale,omitempty"`

	// EvidenceRef is an optional pointer to supporting evidence — a URL,
	// a finding-internal evidence id, or a free-form reference string.
	EvidenceRef string `json:"evidence_ref,omitempty"`
}

// maxMappingCombinedLength caps the combined Framework + ControlID byte
// count to prevent pathological inputs from bloating the graph. 256
// bytes is well above any real framework/control identifier.
const maxMappingCombinedLength = 256

// Validate reports an error if the mapping is structurally invalid.
// Called by AddComplianceMapping before appending, and can be called
// directly by callers constructing mappings manually.
func (m *ComplianceMapping) Validate() error {
	if m == nil {
		return errors.New("compliance mapping is nil")
	}
	if m.Framework == "" {
		return errors.New("compliance mapping: framework is required")
	}
	if m.ControlID == "" {
		return errors.New("compliance mapping: control_id is required")
	}
	if len(m.Framework)+len(m.ControlID) > maxMappingCombinedLength {
		return fmt.Errorf("compliance mapping: framework + control_id exceeds %d bytes",
			maxMappingCombinedLength)
	}
	return nil
}

// AddComplianceMapping appends a mapping to the finding after validation.
// Duplicates (same Framework + ControlID) are no-ops — the existing
// mapping is preserved (first write wins for rationale / evidence_ref).
// Returns a validation error if the mapping is structurally invalid.
func (f *Finding) AddComplianceMapping(m ComplianceMapping) error {
	if err := m.Validate(); err != nil {
		return err
	}
	if f.HasMapping(m.Framework, m.ControlID) {
		return nil
	}
	f.ComplianceMappings = append(f.ComplianceMappings, m)
	return nil
}

// AddComplianceMappingWith is a convenience wrapper that builds a
// ComplianceMapping from positional arguments and appends it.
//
//	finding.AddComplianceMappingWith("NIST_AI_RMF", "MEASURE.2.7",
//	    "LLM output was validated against closed-vocabulary schema",
//	    "evidence-id-abc")
func (f *Finding) AddComplianceMappingWith(framework, controlID, rationale, evidenceRef string) error {
	return f.AddComplianceMapping(ComplianceMapping{
		Framework:   framework,
		ControlID:   controlID,
		Rationale:   rationale,
		EvidenceRef: evidenceRef,
	})
}

// HasMapping reports whether the finding already has a mapping for the
// given (framework, control) pair. Used by AddComplianceMapping for
// deduplication and by callers that want to check before adding.
func (f *Finding) HasMapping(framework, controlID string) bool {
	for _, m := range f.ComplianceMappings {
		if m.Framework == framework && m.ControlID == controlID {
			return true
		}
	}
	return false
}

// MappingsByFramework returns all mappings for the given framework.
// Returns an empty slice if the finding has no mappings for the
// framework — never nil.
func (f *Finding) MappingsByFramework(framework string) []ComplianceMapping {
	out := make([]ComplianceMapping, 0)
	for _, m := range f.ComplianceMappings {
		if m.Framework == framework {
			out = append(out, m)
		}
	}
	return out
}

// ControlIDs returns the set of control IDs mapped on the finding,
// across all frameworks. Useful for quick "does this evidence control X"
// lookups without caring about framework.
func (f *Finding) ControlIDs() []string {
	out := make([]string, 0, len(f.ComplianceMappings))
	for _, m := range f.ComplianceMappings {
		out = append(out, m.ControlID)
	}
	return out
}
