// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Zero Root AI

package security

import (
	"github.com/zeroroot-ai/sdk/finding"
)

// Well-known metadata keys for security domain
const (
	MetaKeyMitreAttack = finding.MetaKeyMitreAttack
	MetaKeyMitreAtlas  = finding.MetaKeyMitreAtlas
	MetaKeyCVSS        = finding.MetaKeyCVSS
	MetaKeyCWE         = finding.MetaKeyCWE
	MetaKeyRiskScore   = finding.MetaKeyRiskScore
)

// Re-exported security category constants for import convenience
const (
	CategoryJailbreak             = "jailbreak"
	CategoryPromptInjection       = "prompt_injection"
	CategoryDataExtraction        = "data_extraction"
	CategoryPrivilegeEscalation   = "privilege_escalation"
	CategoryDOS                   = "dos"
	CategoryModelManipulation     = "model_manipulation"
	CategoryInformationDisclosure = "information_disclosure"
)

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

// CVSSScore represents a CVSS scoring with version, vector, and score.
type CVSSScore struct {
	// Version is the CVSS version (e.g., "3.1", "4.0").
	Version string `json:"version"`

	// Vector is the CVSS vector string.
	Vector string `json:"vector"`

	// Score is the calculated CVSS score (0.0 to 10.0).
	Score float64 `json:"score"`
}

// SetMitreAttack sets the MITRE ATT&CK mapping in the finding metadata.
// This provides a backward-compatible way to store MITRE ATT&CK information
// that was previously stored as a direct field.
func SetMitreAttack(f *finding.Finding, mapping MitreMapping) {
	f.SetMetadata(MetaKeyMitreAttack, mapping)
}

// GetMitreAttack retrieves the MITRE ATT&CK mapping from the finding metadata.
// Returns the mapping and true if found, or an empty mapping and false if not present.
func GetMitreAttack(f *finding.Finding) (MitreMapping, bool) {
	return finding.GetTypedMetadata[MitreMapping](f, MetaKeyMitreAttack)
}

// SetMitreAtlas sets the MITRE ATLAS mapping in the finding metadata.
// This provides a backward-compatible way to store MITRE ATLAS information
// that was previously stored as a direct field.
func SetMitreAtlas(f *finding.Finding, mapping MitreMapping) {
	f.SetMetadata(MetaKeyMitreAtlas, mapping)
}

// GetMitreAtlas retrieves the MITRE ATLAS mapping from the finding metadata.
// Returns the mapping and true if found, or an empty mapping and false if not present.
func GetMitreAtlas(f *finding.Finding) (MitreMapping, bool) {
	return finding.GetTypedMetadata[MitreMapping](f, MetaKeyMitreAtlas)
}

// SetCVSS sets the CVSS score in the finding metadata.
// This provides a backward-compatible way to store CVSS information
// that was previously stored as a direct field.
func SetCVSS(f *finding.Finding, score CVSSScore) {
	f.SetMetadata(MetaKeyCVSS, score)
}

// GetCVSS retrieves the CVSS score from the finding metadata.
// Returns the score and true if found, or an empty score and false if not present.
func GetCVSS(f *finding.Finding) (CVSSScore, bool) {
	return finding.GetTypedMetadata[CVSSScore](f, MetaKeyCVSS)
}

// SetCWE sets the CWE (Common Weakness Enumeration) identifiers in the finding metadata.
// CWE IDs are typically in the format "CWE-79", "CWE-89", etc.
func SetCWE(f *finding.Finding, cweIDs []string) {
	f.SetMetadata(MetaKeyCWE, cweIDs)
}

// GetCWE retrieves the CWE identifiers from the finding metadata.
// Returns the CWE IDs and true if found, or nil and false if not present.
func GetCWE(f *finding.Finding) ([]string, bool) {
	return finding.GetTypedMetadata[[]string](f, MetaKeyCWE)
}

// NewSecurityFinding creates a new Finding pre-configured for security domain.
// This provides a convenient constructor that maintains the same API feel as
// the original NewFinding but explicitly for security use cases.
//
// The category parameter should be one of the security category constants
// (CategoryJailbreak, CategoryPromptInjection, etc.) but any string is accepted.
func NewSecurityFinding(missionID, agentName, title, description, category string, severity finding.Severity) *finding.Finding {
	return finding.NewFinding(missionID, agentName, title, description, category, severity)
}
