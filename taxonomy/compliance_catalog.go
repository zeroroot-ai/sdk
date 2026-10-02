// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Zero Root AI

// DEPRECATED FILE. The compliance rule catalog in this file has no producer and
// no consumer anywhere in the estate, and the two it claimed to have never
// existed.
//
// The doc comment here used to say that "the evaluator in
// core/gibson/internal/harness/compliance_evaluator.go runs each rule's Matcher"
// and that "both the daemon and the CI validator consume it". Neither is true:
// that path does not exist anywhere in gibson at origin/main, and no CI
// validator reads the catalog. The only file that ever referenced
// taxonomy/compliance_rules.yaml is this package's own test. This is the defect
// class where a comment describes a consumer that does not exist, and here the
// comment was the only evidence the feature was wired.
//
// ADR-0013 moved compliance evidence to query time, and ADR-0027 forbids
// leaving the seam behind for someone to re-wire. sdk#137 retired the
// compliance_signal node type the catalog maps from: its CoreNodeType value 17
// is now reserved, so nothing can construct one.
//
// Everything exported here is therefore deprecated and scheduled for removal.
// It is NOT deleted in this change because this module is published under
// Apache-2.0 and an external consumer may compile against any of it. Removal
// lands in the first release after v0.190.0, tracked by sdk#111.
//
// Package taxonomy also holds the live ontology loader and validator, which are
// unaffected: see embed.go, schema.go, validator.go and ontology_schema.go.
package taxonomy

import (
	"errors"
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

// Catalog is the top-level container for a compliance rule bundle.
//
// Deprecated: the compliance catalog has no producer and no consumer. ADR-0013
// moved compliance evidence to query time and sdk#137 retired the
// compliance_signal node type. Removed in the first release after v0.190.0
// (sdk#111).
type Catalog struct {
	Version    string               `yaml:"version"`
	Frameworks map[string]Framework `yaml:"frameworks"`
	Rules      []Rule               `yaml:"rules"`
}

// Framework is metadata about a compliance framework (SOC2, NIST, etc).
//
// Deprecated: the compliance catalog has no producer and no consumer. ADR-0013
// moved compliance evidence to query time and sdk#137 retired the
// compliance_signal node type. Removed in the first release after v0.190.0
// (sdk#111).
type Framework struct {
	Name        string `yaml:"name"`
	Version     string `yaml:"version,omitempty"`
	Reference   string `yaml:"reference,omitempty"`
	Description string `yaml:"description,omitempty"`
}

// Rule is a single mapping from a signal Matcher to a control ID.
//
// Deprecated: the compliance catalog has no producer and no consumer. ADR-0013
// moved compliance evidence to query time and sdk#137 retired the
// compliance_signal node type. Removed in the first release after v0.190.0
// (sdk#111).
type Rule struct {
	ID         string   `yaml:"id"`
	Framework  string   `yaml:"framework"`
	ControlID  string   `yaml:"control_id"`
	Severity   string   `yaml:"severity,omitempty"`
	Notes      string   `yaml:"notes,omitempty"`
	Matcher    Matcher  `yaml:"matcher"`
	References []string `yaml:"references,omitempty"`
}

// Matcher is the tree-shaped predicate that evaluates against a signal.
// Exactly one of Equals / In / Dotted / Not / AnyOf / AllOf should be
// populated. Parse errors flag multiples as ambiguous.
//
// Deprecated: the compliance catalog has no producer and no consumer. ADR-0013
// moved compliance evidence to query time and sdk#137 retired the
// compliance_signal node type. Removed in the first release after v0.190.0
// (sdk#111).
type Matcher struct {
	// Equals: a flat key/value map where every entry must match the
	// signal's value for that property. Multiple entries in Equals are
	// AND'd together.
	Equals map[string]string `yaml:"equals,omitempty"`

	// In: key whose value must appear in the list.
	In map[string][]string `yaml:"in,omitempty"`

	// Not: nested matcher that must NOT match.
	Not *Matcher `yaml:"not,omitempty"`

	// AnyOf: at least one child matcher must match (OR).
	AnyOf []Matcher `yaml:"any_of,omitempty"`

	// AllOf: every child matcher must match (AND). This is the
	// default when multiple siblings are populated at the same level.
	AllOf []Matcher `yaml:"all_of,omitempty"`
}

// IsLeaf reports whether this matcher has no nested children.
//
// Deprecated: the compliance catalog has no producer and no consumer. ADR-0013
// moved compliance evidence to query time and sdk#137 retired the
// compliance_signal node type. Removed in the first release after v0.190.0
// (sdk#111).
func (m Matcher) IsLeaf() bool {
	return m.Not == nil && len(m.AnyOf) == 0 && len(m.AllOf) == 0
}

// LoadCatalog reads a catalog from a file path.
//
// Deprecated: the compliance catalog has no producer and no consumer. ADR-0013
// moved compliance evidence to query time and sdk#137 retired the
// compliance_signal node type. Removed in the first release after v0.190.0
// (sdk#111).
func LoadCatalog(path string) (*Catalog, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("load catalog %q: %w", path, err)
	}
	return LoadCatalogFromBytes(data)
}

// LoadCatalogFromBytes parses a catalog from raw YAML bytes. Parse errors
// include the YAML line number for actionable operator diagnostics.
//
// Deprecated: the compliance catalog has no producer and no consumer. ADR-0013
// moved compliance evidence to query time and sdk#137 retired the
// compliance_signal node type. Removed in the first release after v0.190.0
// (sdk#111).
func LoadCatalogFromBytes(data []byte) (*Catalog, error) {
	var c Catalog
	if err := yaml.Unmarshal(data, &c); err != nil {
		return nil, fmt.Errorf("parse catalog yaml: %w", err)
	}
	if err := c.Validate(); err != nil {
		return nil, err
	}
	return &c, nil
}

// Validate runs structural checks on the loaded catalog. Called by
// LoadCatalogFromBytes so every loaded catalog is valid at point of load.
//
// Deprecated: the compliance catalog has no producer and no consumer. ADR-0013
// moved compliance evidence to query time and sdk#137 retired the
// compliance_signal node type. Removed in the first release after v0.190.0
// (sdk#111).
func (c *Catalog) Validate() error {
	if c.Version == "" {
		return errors.New("catalog: version field is required")
	}
	if len(c.Rules) == 0 {
		return errors.New("catalog: at least one rule is required")
	}
	seen := map[string]bool{}
	for i, r := range c.Rules {
		if r.ID == "" {
			return fmt.Errorf("catalog: rule #%d is missing id", i+1)
		}
		if seen[r.ID] {
			return fmt.Errorf("catalog: duplicate rule id %q", r.ID)
		}
		seen[r.ID] = true
		if r.Framework == "" {
			return fmt.Errorf("catalog: rule %q is missing framework", r.ID)
		}
		if _, ok := c.Frameworks[r.Framework]; !ok {
			return fmt.Errorf("catalog: rule %q references unknown framework %q", r.ID, r.Framework)
		}
		if r.ControlID == "" {
			return fmt.Errorf("catalog: rule %q is missing control_id", r.ID)
		}
		if err := validateMatcher(r.Matcher, r.ID); err != nil {
			return err
		}
	}
	return nil
}

// validateMatcher walks a matcher tree and rejects structurally invalid
// matchers (e.g. a Not with nothing under it, or an empty matcher).
func validateMatcher(m Matcher, ruleID string) error {
	populated := 0
	if len(m.Equals) > 0 {
		populated++
	}
	if len(m.In) > 0 {
		populated++
	}
	if m.Not != nil {
		populated++
		if err := validateMatcher(*m.Not, ruleID); err != nil {
			return err
		}
	}
	if len(m.AnyOf) > 0 {
		populated++
		for _, child := range m.AnyOf {
			if err := validateMatcher(child, ruleID); err != nil {
				return err
			}
		}
	}
	if len(m.AllOf) > 0 {
		populated++
		for _, child := range m.AllOf {
			if err := validateMatcher(child, ruleID); err != nil {
				return err
			}
		}
	}
	if populated == 0 {
		return fmt.Errorf("catalog: rule %q has empty matcher", ruleID)
	}
	return nil
}

// RulesByFramework returns rules grouped by framework name.
//
// Deprecated: the compliance catalog has no producer and no consumer. ADR-0013
// moved compliance evidence to query time and sdk#137 retired the
// compliance_signal node type. Removed in the first release after v0.190.0
// (sdk#111).
func (c *Catalog) RulesByFramework() map[string][]Rule {
	out := map[string][]Rule{}
	for _, r := range c.Rules {
		out[r.Framework] = append(out[r.Framework], r)
	}
	return out
}
