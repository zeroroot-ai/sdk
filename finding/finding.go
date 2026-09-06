// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Zero Root AI

package finding

import (
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
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

// NewFinding creates a new Finding with required fields and auto-generated values.
func NewFinding(missionID, agentName, title, description, category string, severity Severity) *Finding {
	now := time.Now()
	return &Finding{
		ID:          uuid.New().String(),
		MissionID:   missionID,
		AgentName:   agentName,
		Title:       title,
		Description: description,
		Category:    category,
		Severity:    severity,
		Confidence:  1.0,
		Status:      StatusOpen,
		CreatedAt:   now,
		UpdatedAt:   now,
		RiskScore:   calculateRiskScore(severity, 1.0),
	}
}

// NewFindingWithID creates a new Finding with a specific ID.
func NewFindingWithID(id, missionID, agentName, title, description, category string, severity Severity) *Finding {
	now := time.Now()
	return &Finding{
		ID:          id,
		MissionID:   missionID,
		AgentName:   agentName,
		Title:       title,
		Description: description,
		Category:    category,
		Severity:    severity,
		Confidence:  1.0,
		Status:      StatusOpen,
		CreatedAt:   now,
		UpdatedAt:   now,
		RiskScore:   calculateRiskScore(severity, 1.0),
	}
}

// Validate checks if the finding has all required fields and valid values.
func (f *Finding) Validate() error {
	if f.ID == "" {
		return errors.New("finding ID is required")
	}
	if f.MissionID == "" {
		return errors.New("mission ID is required")
	}
	if f.AgentName == "" {
		return errors.New("agent name is required")
	}
	if f.Title == "" {
		return errors.New("title is required")
	}
	if f.Description == "" {
		return errors.New("description is required")
	}
	if f.Category == "" {
		return errors.New("category is required")
	}
	if !f.Severity.IsValid() {
		return fmt.Errorf("invalid severity: %s", f.Severity)
	}
	if f.Confidence < 0.0 || f.Confidence > 1.0 {
		return fmt.Errorf("confidence must be between 0.0 and 1.0, got %f", f.Confidence)
	}
	if !f.Status.IsValid() {
		return fmt.Errorf("invalid status: %s", f.Status)
	}
	if f.CVSSScore != nil && (*f.CVSSScore < 0.0 || *f.CVSSScore > 10.0) {
		return fmt.Errorf("CVSS score must be between 0.0 and 10.0, got %f", *f.CVSSScore)
	}
	if f.CreatedAt.IsZero() {
		return errors.New("created_at timestamp is required")
	}
	if f.UpdatedAt.IsZero() {
		return errors.New("updated_at timestamp is required")
	}

	// Validate evidence
	for i, ev := range f.Evidence {
		if err := ev.Validate(); err != nil {
			return fmt.Errorf("invalid evidence at index %d: %w", i, err)
		}
	}

	// Validate reproduction steps
	for i, step := range f.Reproduction {
		if err := step.Validate(); err != nil {
			return fmt.Errorf("invalid reproduction step at index %d: %w", i, err)
		}
	}

	// Validate MITRE mappings
	if f.MitreAttack != nil {
		if err := f.MitreAttack.Validate(); err != nil {
			return fmt.Errorf("invalid MITRE ATT&CK mapping: %w", err)
		}
	}
	if f.MitreAtlas != nil {
		if err := f.MitreAtlas.Validate(); err != nil {
			return fmt.Errorf("invalid MITRE ATLAS mapping: %w", err)
		}
	}

	return nil
}

// AddEvidence adds a piece of evidence to the finding and updates the timestamp.
func (f *Finding) AddEvidence(evidence Evidence) {
	f.Evidence = append(f.Evidence, evidence)
	f.UpdatedAt = time.Now()
}

// AddReproductionStep adds a reproduction step to the finding and updates the timestamp.
func (f *Finding) AddReproductionStep(step ReproStep) {
	f.Reproduction = append(f.Reproduction, step)
	f.UpdatedAt = time.Now()
}

// AddTag adds a tag to the finding if it doesn't already exist.
func (f *Finding) AddTag(tag string) {
	for _, existingTag := range f.Tags {
		if existingTag == tag {
			return
		}
	}
	f.Tags = append(f.Tags, tag)
	f.UpdatedAt = time.Now()
}

// SetConfidence sets the confidence level and recalculates the risk score.
func (f *Finding) SetConfidence(confidence float64) error {
	if confidence < 0.0 || confidence > 1.0 {
		return fmt.Errorf("confidence must be between 0.0 and 1.0, got %f", confidence)
	}
	f.Confidence = confidence
	f.RiskScore = calculateRiskScore(f.Severity, confidence)
	f.UpdatedAt = time.Now()
	return nil
}

// SetStatus updates the finding status and timestamp.
func (f *Finding) SetStatus(status Status) error {
	if !status.IsValid() {
		return fmt.Errorf("invalid status: %s", status)
	}
	f.Status = status
	f.UpdatedAt = time.Now()
	return nil
}

// SetMitreAttack sets the MITRE ATT&CK mapping.
func (f *Finding) SetMitreAttack(mapping *MitreMapping) {
	f.MitreAttack = mapping
	f.UpdatedAt = time.Now()
}

// SetMitreAtlas sets the MITRE ATLAS mapping.
func (f *Finding) SetMitreAtlas(mapping *MitreMapping) {
	f.MitreAtlas = mapping
	f.UpdatedAt = time.Now()
}

// calculateRiskScore computes a risk score based on severity and confidence.
// Formula: severity_weight * confidence
func calculateRiskScore(severity Severity, confidence float64) float64 {
	return severity.Weight() * confidence
}

// Validate checks if the MITRE mapping is valid.
func (m *MitreMapping) Validate() error {
	if m.Matrix == "" {
		return errors.New("MITRE matrix is required")
	}
	if m.TacticID == "" {
		return errors.New("MITRE tactic ID is required")
	}
	if m.TacticName == "" {
		return errors.New("MITRE tactic name is required")
	}
	if m.TechniqueID == "" {
		return errors.New("MITRE technique ID is required")
	}
	if m.TechniqueName == "" {
		return errors.New("MITRE technique name is required")
	}
	return nil
}

// Validate checks if the reproduction step is valid.
func (r *ReproStep) Validate() error {
	if r.Order < 1 {
		return errors.New("reproduction step order must be >= 1")
	}
	if r.Description == "" {
		return errors.New("reproduction step description is required")
	}
	return nil
}

// NewMitreMapping creates a new MITRE framework mapping.
func NewMitreMapping(matrix, tacticID, tacticName, techniqueID, techniqueName string) *MitreMapping {
	return &MitreMapping{
		Matrix:        matrix,
		TacticID:      tacticID,
		TacticName:    tacticName,
		TechniqueID:   techniqueID,
		TechniqueName: techniqueName,
	}
}

// NewReproStep creates a new reproduction step.
func NewReproStep(order int, description, input, output string) ReproStep {
	return ReproStep{
		Order:       order,
		Description: description,
		Input:       input,
		Output:      output,
	}
}
