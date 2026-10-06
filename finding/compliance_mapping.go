// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Zero Root AI

package finding

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
