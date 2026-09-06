// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Zero Root AI

package finding

import (
	"errors"
	"fmt"
	"time"
)

// ExportFormat represents the format for exporting findings.
type ExportFormat string

const (
	// FormatJSON exports findings as JSON.
	FormatJSON ExportFormat = "json"

	// FormatSARIF exports findings in SARIF (Static Analysis Results Interchange Format).
	FormatSARIF ExportFormat = "sarif"

	// FormatCSV exports findings as comma-separated values.
	FormatCSV ExportFormat = "csv"

	// FormatHTML exports findings as an HTML report.
	FormatHTML ExportFormat = "html"
)

// IsValid returns true if the export format is valid.
func (f ExportFormat) IsValid() bool {
	switch f {
	case FormatJSON, FormatSARIF, FormatCSV, FormatHTML:
		return true
	default:
		return false
	}
}

// String returns the string representation of the export format.
func (f ExportFormat) String() string {
	return string(f)
}

// FileExtension returns the file extension for the export format.
func (f ExportFormat) FileExtension() string {
	switch f {
	case FormatJSON:
		return ".json"
	case FormatSARIF:
		return ".sarif"
	case FormatCSV:
		return ".csv"
	case FormatHTML:
		return ".html"
	default:
		return ""
	}
}

// MimeType returns the MIME type for the export format.
func (f ExportFormat) MimeType() string {
	switch f {
	case FormatJSON:
		return "application/json"
	case FormatSARIF:
		return "application/sarif+json"
	case FormatCSV:
		return "text/csv"
	case FormatHTML:
		return "text/html"
	default:
		return "application/octet-stream"
	}
}

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

// IsValid returns true if the status is valid.
func (s Status) IsValid() bool {
	switch s {
	case StatusOpen, StatusConfirmed, StatusResolved, StatusFalsePositive:
		return true
	default:
		return false
	}
}

// String returns the string representation of the status.
func (s Status) String() string {
	return string(s)
}

// DisplayName returns a human-readable display name for the status.
func (s Status) DisplayName() string {
	switch s {
	case StatusOpen:
		return "Open"
	case StatusConfirmed:
		return "Confirmed"
	case StatusResolved:
		return "Resolved"
	case StatusFalsePositive:
		return "False Positive"
	default:
		return string(s)
	}
}

// Filter represents criteria for filtering findings.
type Filter struct {
	// MissionID filters by mission identifier.
	MissionID string `json:"mission_id,omitempty"`

	// AgentName filters by agent name.
	AgentName string `json:"agent_name,omitempty"`

	// Categories filters by one or more categories.
	Categories []string `json:"categories,omitempty"`

	// Severities filters by one or more severity levels.
	Severities []Severity `json:"severities,omitempty"`

	// Status filters by finding status.
	Status Status `json:"status,omitempty"`

	// Tags filters by tags (finding must have at least one matching tag).
	Tags []string `json:"tags,omitempty"`

	// MinScore filters findings with risk score >= this value.
	MinScore float64 `json:"min_score,omitempty"`

	// CreatedAfter filters findings created after this time.
	CreatedAfter time.Time `json:"created_after,omitempty"`

	// CreatedBefore filters findings created before this time.
	CreatedBefore time.Time `json:"created_before,omitempty"`

	// Limit limits the number of results returned.
	Limit int `json:"limit,omitempty"`

	// Offset skips the first N results (for pagination).
	Offset int `json:"offset,omitempty"`
}

// Matches returns true if the given finding matches all filter criteria.
func (f *Filter) Matches(finding Finding) bool {
	// MissionID filter
	if f.MissionID != "" && finding.MissionID != f.MissionID {
		return false
	}

	// AgentName filter
	if f.AgentName != "" && finding.AgentName != f.AgentName {
		return false
	}

	// Categories filter
	if len(f.Categories) > 0 {
		matched := false
		for _, cat := range f.Categories {
			if finding.Category == cat {
				matched = true
				break
			}
		}
		if !matched {
			return false
		}
	}

	// Severities filter
	if len(f.Severities) > 0 {
		matched := false
		for _, sev := range f.Severities {
			if finding.Severity == sev {
				matched = true
				break
			}
		}
		if !matched {
			return false
		}
	}

	// Status filter
	if f.Status != "" && finding.Status != f.Status {
		return false
	}

	// Tags filter (at least one tag must match)
	if len(f.Tags) > 0 {
		matched := false
		for _, filterTag := range f.Tags {
			for _, findingTag := range finding.Tags {
				if findingTag == filterTag {
					matched = true
					break
				}
			}
			if matched {
				break
			}
		}
		if !matched {
			return false
		}
	}

	// MinScore filter
	if f.MinScore > 0 && finding.RiskScore < f.MinScore {
		return false
	}

	// CreatedAfter filter
	if !f.CreatedAfter.IsZero() && finding.CreatedAt.Before(f.CreatedAfter) {
		return false
	}

	// CreatedBefore filter
	if !f.CreatedBefore.IsZero() && finding.CreatedAt.After(f.CreatedBefore) {
		return false
	}

	return true
}

// Validate checks if the filter configuration is valid.
func (f *Filter) Validate() error {
	// Validate categories - now accepting any non-empty string
	for _, cat := range f.Categories {
		if cat == "" {
			return errors.New("empty category in filter")
		}
	}

	// Validate severities
	for _, sev := range f.Severities {
		if !sev.IsValid() {
			return fmt.Errorf("invalid severity in filter: %s", sev)
		}
	}

	// Validate status
	if f.Status != "" && !f.Status.IsValid() {
		return fmt.Errorf("invalid status in filter: %s", f.Status)
	}

	// Validate MinScore
	if f.MinScore < 0 {
		return errors.New("min_score cannot be negative")
	}

	// Validate Limit
	if f.Limit < 0 {
		return errors.New("limit cannot be negative")
	}

	// Validate Offset
	if f.Offset < 0 {
		return errors.New("offset cannot be negative")
	}

	// Validate time range
	if !f.CreatedAfter.IsZero() && !f.CreatedBefore.IsZero() && f.CreatedAfter.After(f.CreatedBefore) {
		return errors.New("created_after must be before created_before")
	}

	return nil
}

// ParseExportFormat parses a string into an ExportFormat value.
// Returns an error if the string is not a valid export format.
func ParseExportFormat(s string) (ExportFormat, error) {
	format := ExportFormat(s)
	if !format.IsValid() {
		return "", fmt.Errorf("invalid export format: %s", s)
	}
	return format, nil
}

// ParseStatus parses a string into a Status value.
// Returns an error if the string is not a valid status.
func ParseStatus(s string) (Status, error) {
	status := Status(s)
	if !status.IsValid() {
		return "", fmt.Errorf("invalid status: %s", s)
	}
	return status, nil
}

// AllExportFormats returns all valid export formats.
func AllExportFormats() []ExportFormat {
	return []ExportFormat{
		FormatJSON,
		FormatSARIF,
		FormatCSV,
		FormatHTML,
	}
}

// AllStatuses returns all valid statuses.
func AllStatuses() []Status {
	return []Status{
		StatusOpen,
		StatusConfirmed,
		StatusResolved,
		StatusFalsePositive,
	}
}
