// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Zero Root AI

package finding

// Status represents the current status of a finding.
type Status string

const (
	// StatusOpen indicates a newly discovered finding that hasn't been reviewed.
	StatusOpen Status = "open"

	// StatusConfirmed indicates a finding that has been verified as valid.
	StatusConfirmed Status = "confirmed"

	// StatusResolved indicates a finding that has been fixed or mitigated.
	StatusResolved Status = "resolved"

	// StatusFalsePositive indicates a finding that was determined to be invalid.
	StatusFalsePositive Status = "false_positive"
)

// String returns the string representation of the status.
func (s Status) String() string {
	return string(s)
}

// Filter represents criteria for filtering findings.
type Filter struct {
	// MissionID filters by mission identifier.
	MissionID string `json:"mission_id,omitempty"`

	// AgentName filters by agent name.
	AgentName string `json:"agent_name,omitempty"`

	// Severities filters by one or more severity levels.
	Severities []Severity `json:"severities,omitempty"`

	// Status filters by finding status.
	Status Status `json:"status,omitempty"`

	// Tags filters by tags (finding must have at least one matching tag).
	Tags []string `json:"tags,omitempty"`
}
