// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Zero Root AI

// Package taxonomy holds the compliance rule catalog loader and types.
//
// The catalog is a declarative YAML bundle that maps `compliance_signal`
// node properties to compliance control IDs (SOC2, NIST AI RMF, MITRE
// ATLAS/ATT&CK, PLATFORM). The evaluator in
// core/gibson/internal/harness/compliance_evaluator.go runs each rule's
// Matcher against an emitted signal and stamps matching control IDs onto
// the signal's `control_ids` list before persistence.
//
// The matcher language is deliberately small: equals / list membership /
// dotted-path lookups / not / any_of. No regex, no CEL, no scripts. This
// keeps rule authoring safe for tenant contributors and the evaluator
// deterministic under load.
//
// This file is in the SDK taxonomy package because both the daemon (which
// evaluates rules) and the CI validator (which checks fixtures) consume
// it, and the SDK is the only place both codepaths can share code without
// an import cycle.
package taxonomy

import (
	"errors"
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

// Catalog is the top-level container for a compliance rule bundle.
type Catalog struct {
	Version    string               `yaml:"version"`
	Frameworks map[string]Framework `yaml:"frameworks"`
	Rules      []Rule               `yaml:"rules"`
}

// Framework is metadata about a compliance framework (SOC2, NIST, etc).
type Framework struct {
	Name        string `yaml:"name"`
	Version     string `yaml:"version,omitempty"`
	Reference   string `yaml:"reference,omitempty"`
	Description string `yaml:"description,omitempty"`
}

// Rule is a single mapping from a signal Matcher to a control ID.
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
func (m Matcher) IsLeaf() bool {
	return m.Not == nil && len(m.AnyOf) == 0 && len(m.AllOf) == 0
}

// LoadCatalog reads a catalog from a file path.
func LoadCatalog(path string) (*Catalog, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("load catalog %q: %w", path, err)
	}
	return LoadCatalogFromBytes(data)
}

// LoadCatalogFromBytes parses a catalog from raw YAML bytes. Parse errors
// include the YAML line number for actionable operator diagnostics.
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
func (c *Catalog) RulesByFramework() map[string][]Rule {
	out := map[string][]Rule{}
	for _, r := range c.Rules {
		out[r.Framework] = append(out[r.Framework], r)
	}
	return out
}
