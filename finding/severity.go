// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Zero Root AI

package finding

import "fmt"

// Severity represents the severity level of a security finding.
type Severity string

const (
	// SeverityCritical indicates a critical security issue requiring immediate attention.
	// Examples: Remote code execution, complete system compromise
	SeverityCritical Severity = "critical"

	// SeverityHigh indicates a high-impact security issue.
	// Examples: Privilege escalation, significant data exposure
	SeverityHigh Severity = "high"

	// SeverityMedium indicates a moderate security issue.
	// Examples: Limited information disclosure, partial DoS
	SeverityMedium Severity = "medium"

	// SeverityLow indicates a minor security issue.
	// Examples: Minor information leaks, cosmetic security issues
	SeverityLow Severity = "low"

	// SeverityInfo indicates an informational finding without direct security impact.
	// Examples: Security recommendations, best practice violations
	SeverityInfo Severity = "info"
)

// severityWeights maps severity levels to numeric weights for risk calculation.
// Higher weights indicate more severe findings.
var severityWeights = map[Severity]float64{
	SeverityCritical: 10.0,
	SeverityHigh:     7.5,
	SeverityMedium:   5.0,
	SeverityLow:      2.5,
	SeverityInfo:     1.0,
}

// IsValid returns true if the severity level is valid.
func (s Severity) IsValid() bool {
	switch s {
	case SeverityCritical, SeverityHigh, SeverityMedium, SeverityLow, SeverityInfo:
		return true
	default:
		return false
	}
}

// Weight returns the numeric weight associated with the severity level.
// Returns 0.0 for invalid severity levels.
func (s Severity) Weight() float64 {
	if weight, ok := severityWeights[s]; ok {
		return weight
	}
	return 0.0
}

// String returns the string representation of the severity.
func (s Severity) String() string {
	return string(s)
}

// ParseSeverity parses a string into a Severity value.
// Returns an error if the string is not a valid severity level.
func ParseSeverity(s string) (Severity, error) {
	severity := Severity(s)
	if !severity.IsValid() {
		return "", fmt.Errorf("invalid severity: %s", s)
	}
	return severity, nil
}

// CompareSeverity compares two severity levels.
// Returns:
//   - negative if s1 < s2
//   - zero if s1 == s2
//   - positive if s1 > s2
func CompareSeverity(s1, s2 Severity) int {
	w1 := s1.Weight()
	w2 := s2.Weight()
	if w1 < w2 {
		return -1
	}
	if w1 > w2 {
		return 1
	}
	return 0
}

// AllSeverities returns all valid severity levels in order from critical to info.
func AllSeverities() []Severity {
	return []Severity{
		SeverityCritical,
		SeverityHigh,
		SeverityMedium,
		SeverityLow,
		SeverityInfo,
	}
}
