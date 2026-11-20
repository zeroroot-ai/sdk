// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Zero Root AI

package security

import (
	"github.com/zeroroot-ai/sdk/finding"
)

// Well-known metadata keys for security domain
const (
	MetaKeyMitreAttack = finding.MetaKeyMitreAttack
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
