// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Zero Root AI

package finding

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

// String returns the string representation of the severity.
func (s Severity) String() string {
	return string(s)
}
