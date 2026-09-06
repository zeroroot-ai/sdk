// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Zero Root AI

package sdk

import (
	"github.com/zeroroot-ai/sdk/finding"
)

// sarifLog is the top-level SARIF 2.1.0 document.
type sarifLog struct {
	Schema  string     `json:"$schema"`
	Version string     `json:"version"`
	Runs    []sarifRun `json:"runs"`
}

// sarifRun represents a single analysis run within a SARIF log.
type sarifRun struct {
	Tool    sarifTool     `json:"tool"`
	Rules   []sarifRule   `json:"rules,omitempty"`
	Results []sarifResult `json:"results"`
}

// sarifTool identifies the analysis tool that produced the run.
type sarifTool struct {
	Driver sarifDriver `json:"driver"`
}

// sarifDriver describes the primary analysis tool driver.
type sarifDriver struct {
	Name    string      `json:"name"`
	Version string      `json:"version"`
	Rules   []sarifRule `json:"rules,omitempty"`
}

// sarifRule represents a single rule reported by the tool driver.
type sarifRule struct {
	ID               string       `json:"id"`
	Name             string       `json:"name,omitempty"`
	ShortDescription sarifMessage `json:"shortDescription,omitempty"`
}

// sarifResult represents the outcome of a single rule evaluation.
type sarifResult struct {
	RuleID    string          `json:"ruleId"`
	Level     string          `json:"level"`
	Message   sarifMessage    `json:"message"`
	Locations []sarifLocation `json:"locations,omitempty"`
}

// sarifMessage is a plain-text or markdown message in SARIF.
type sarifMessage struct {
	Text string `json:"text"`
}

// sarifLocation identifies a location to which a result is relevant.
type sarifLocation struct {
	PhysicalLocation sarifPhysicalLocation `json:"physicalLocation"`
}

// sarifPhysicalLocation identifies an artifact and region within it.
type sarifPhysicalLocation struct {
	ArtifactLocation sarifArtifactLocation `json:"artifactLocation"`
	Region           *sarifRegion          `json:"region,omitempty"`
}

// sarifArtifactLocation specifies the location of an artifact.
type sarifArtifactLocation struct {
	URI string `json:"uri"`
}

// sarifRegion specifies a region within an artifact.
type sarifRegion struct {
	StartLine int `json:"startLine,omitempty"`
}

const (
	sarifSchema  = "https://raw.githubusercontent.com/oasis-tcs/sarif-spec/master/Schemata/sarif-schema-2.1.0.json"
	sarifVersion = "2.1.0"
)

// severityToSARIFLevel maps Gibson severity levels to SARIF result levels.
// SARIF levels: "error", "warning", "note", "none" (per SARIF 2.1.0 §3.27.10).
func severityToSARIFLevel(s finding.Severity) string {
	switch s {
	case finding.SeverityCritical, finding.SeverityHigh:
		return "error"
	case finding.SeverityMedium:
		return "warning"
	default:
		// low, info, unknown
		return "note"
	}
}

// buildSARIF constructs a valid SARIF 2.1.0 document from the provided findings.
//
// Rules are deduplicated by category — each unique category becomes one rule
// whose id is the category string. Results are emitted one per finding; the
// result's ruleId is the finding's Category, level is derived from Severity
// (critical/high → error, medium → warning, low/info → note), and message
// text is "<Title>: <Description>". When TargetID is non-empty it is attached
// as physicalLocation.artifactLocation.uri.
//
// The results slice is pre-allocated to cap(len(findings)) so the function
// performs a single allocation even for large finding sets.
func buildSARIF(findings []finding.Finding, toolVersion string) sarifLog {
	// Deduplicate rules by category. Insertion order is preserved via the
	// slice so the output is deterministic across runs.
	seen := make(map[string]struct{}, len(findings))
	rules := make([]sarifRule, 0, len(findings))

	for _, f := range findings {
		if f.Category == "" {
			continue
		}
		if _, exists := seen[f.Category]; !exists {
			seen[f.Category] = struct{}{}
			rules = append(rules, sarifRule{
				ID:               f.Category,
				Name:             f.Category,
				ShortDescription: sarifMessage{Text: f.Category},
			})
		}
	}

	// Build one result per finding.
	results := make([]sarifResult, 0, len(findings))
	for _, f := range findings {
		msg := f.Title
		if f.Description != "" {
			msg += ": " + f.Description
		}

		r := sarifResult{
			RuleID:  f.Category,
			Level:   severityToSARIFLevel(f.Severity),
			Message: sarifMessage{Text: msg},
		}

		// Attach a physical location when a target URI is known.
		if f.TargetID != "" {
			r.Locations = []sarifLocation{
				{
					PhysicalLocation: sarifPhysicalLocation{
						ArtifactLocation: sarifArtifactLocation{URI: f.TargetID},
					},
				},
			}
		}

		results = append(results, r)
	}

	return sarifLog{
		Schema:  sarifSchema,
		Version: sarifVersion,
		Runs: []sarifRun{
			{
				Tool: sarifTool{
					Driver: sarifDriver{
						Name:    "Gibson",
						Version: toolVersion,
						Rules:   rules,
					},
				},
				Results: results,
			},
		},
	}
}
