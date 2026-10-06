// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Zero Root AI

package finding

import (
	"time"
)

// Finding represents a security vulnerability or issue discovered during testing.
type Finding struct {
	// ID is a unique identifier for the finding.
	ID string `json:"id"`

	// MissionID identifies the mission that discovered this finding.
	MissionID string `json:"mission_id"`

	// AgentName identifies the agent that discovered this finding.
	AgentName string `json:"agent_name"`

	// DelegatedFrom indicates if this finding was delegated from another agent.
	DelegatedFrom string `json:"delegated_from,omitempty"`

	// Title is a brief summary of the finding.
	Title string `json:"title"`

	// Description provides detailed information about the finding.
	Description string `json:"description"`

	// Category classifies the type of security issue.
	Category string `json:"category"`

	// Subcategory provides additional classification detail.
	Subcategory string `json:"subcategory,omitempty"`

	// Severity indicates the severity level of the finding.
	Severity Severity `json:"severity"`

	// Confidence represents the confidence level (0.0 to 1.0) in the finding.
	Confidence float64 `json:"confidence"`

	// MitreAttack maps the finding to MITRE ATT&CK framework.
	MitreAttack *MitreMapping `json:"mitre_attack,omitempty"`

	// MitreAtlas maps the finding to MITRE ATLAS framework.
	MitreAtlas *MitreMapping `json:"mitre_atlas,omitempty"`

	// Evidence contains supporting evidence for the finding.
	Evidence []Evidence `json:"evidence,omitempty"`

	// Reproduction contains steps to reproduce the finding.
	Reproduction []ReproStep `json:"reproduction,omitempty"`

	// CVSSScore is the Common Vulnerability Scoring System score (0.0 to 10.0).
	CVSSScore *float64 `json:"cvss_score,omitempty"`

	// RiskScore is a calculated risk score based on severity, confidence, and other factors.
	RiskScore float64 `json:"risk_score"`

	// Remediation provides guidance on fixing or mitigating the issue.
	Remediation string `json:"remediation,omitempty"`

	// References contains links to relevant documentation or resources.
	References []string `json:"references,omitempty"`

	// TargetID identifies the specific target or component affected.
	TargetID string `json:"target_id,omitempty"`

	// Technique describes the technique used to discover the finding.
	Technique string `json:"technique,omitempty"`

	// Tags are arbitrary labels for categorization and filtering.
	Tags []string `json:"tags,omitempty"`

	// Metadata contains extensible domain-specific data.
	Metadata map[string]any `json:"metadata,omitempty"`

	// Status indicates the current state of the finding.
	Status Status `json:"status"`

	// CreatedAt is the timestamp when the finding was created.
	CreatedAt time.Time `json:"created_at"`

	// UpdatedAt is the timestamp when the finding was last updated.
	UpdatedAt time.Time `json:"updated_at"`

	// ComplianceMappings links this finding to compliance framework controls.
	// Multiple mappings per finding are supported — one finding often
	// evidences controls from multiple frameworks (SOC2, NIST AI RMF,
	// MITRE ATLAS). See compliance_mapping.go for the helpers.
	// Added by audit-finding-compliance-mappings.
	ComplianceMappings []ComplianceMapping `json:"compliance_mappings,omitempty"`
}

// MitreMapping represents a mapping to a MITRE framework (ATT&CK or ATLAS).
type MitreMapping struct {
	// Matrix identifies the MITRE matrix (e.g., "enterprise", "mobile", "atlas").
	Matrix string `json:"matrix"`

	// TacticID is the MITRE tactic identifier (e.g., "TA0001").
	TacticID string `json:"tactic_id"`

	// TacticName is the human-readable tactic name.
	TacticName string `json:"tactic_name"`

	// TechniqueID is the MITRE technique identifier (e.g., "T1059").
	TechniqueID string `json:"technique_id"`

	// TechniqueName is the human-readable technique name.
	TechniqueName string `json:"technique_name"`

	// SubTechniques lists any sub-technique identifiers (e.g., "T1059.001").
	SubTechniques []string `json:"sub_techniques,omitempty"`
}

// ReproStep represents a single step in reproducing a finding.
type ReproStep struct {
	// Order indicates the sequence number of this step.
	Order int `json:"order"`

	// Description explains what to do in this step.
	Description string `json:"description"`

	// Input contains the input data or command for this step.
	Input string `json:"input,omitempty"`

	// Output contains the expected output or result from this step.
	Output string `json:"output,omitempty"`
}
