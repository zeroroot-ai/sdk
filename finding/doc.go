// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Zero Root AI

// Package finding provides types and utilities for representing security findings
// discovered during AI/LLM security testing missions.
//
// The package includes comprehensive types for documenting vulnerabilities,
// evidence collection, MITRE ATT&CK/ATLAS mappings, and risk scoring.
//
// # Core Types
//
// Finding represents a discovered security vulnerability with full context:
//   - Severity and risk scoring
//   - MITRE ATT&CK and ATLAS mappings
//   - Evidence collection and reproduction steps
//   - Status tracking and remediation guidance
//
// # Categories
//
// Findings are categorized into standard security classes:
//   - Jailbreak attempts
//   - Prompt injection
//   - Data extraction
//   - Privilege escalation
//   - Denial of service
//   - Model manipulation
//   - Information disclosure
//
// # Severity Levels
//
// Severity is ranked from Critical to Info with associated weights
// for risk calculation and prioritization.
//
// # Evidence Collection
//
// Evidence can include:
//   - HTTP requests/responses
//   - Screenshots
//   - Log entries
//   - Payloads
//   - Conversation transcripts
//
// # Export and Filtering
//
// Findings can be exported in multiple formats (JSON, SARIF, CSV, HTML)
// and filtered by various criteria for analysis and reporting.
//
// Example usage:
//
//	finding := finding.NewFinding(
//		"mission-123",
//		"agent-sql-inject",
//		"SQL Injection in Login",
//		"Discovered SQL injection vulnerability...",
//		finding.CategoryDataExtraction,
//		finding.SeverityHigh,
//	)
//
//	finding.AddEvidence(finding.Evidence{
//		Type:    finding.EvidenceHTTPRequest,
//		Title:   "Malicious Request",
//		Content: "POST /login ...",
//	})
//
//	if err := finding.Validate(); err != nil {
//		log.Fatal(err)
//	}
package finding
