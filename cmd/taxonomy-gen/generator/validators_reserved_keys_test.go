// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Zero Root AI

package generator

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/zeroroot-ai/sdk/cmd/taxonomy-gen/schema"
)

// makeNodeTypeWithReservedKeys is a test helper that builds a NodeType containing
// a single map-typed property with the given reserved keys.
func makeNodeTypeWithReservedKeys(nodeName, propName string, reservedKeys map[string]schema.ReservedKeyDef) schema.NodeType {
	return schema.NodeType{
		Name:     nodeName,
		Category: "meta",
		Properties: []schema.Property{
			{
				Name:         propName,
				Type:         "map<string,string>",
				ReservedKeys: reservedKeys,
			},
		},
	}
}

// TestReservedKeyRulesForNode_NoReservedKeys verifies that a node with no
// reserved-key annotations produces no rules.
func TestReservedKeyRulesForNode_NoReservedKeys(t *testing.T) {
	nt := schema.NodeType{
		Name:     "compliance_signal",
		Category: "meta",
		Properties: []schema.Property{
			{Name: "custom", Type: "map<string,string>"},
		},
	}
	rules := reservedKeyRulesForNode(nt)
	assert.Empty(t, rules, "node with no reserved keys should produce no rules")
}

// TestReservedKeyRulesForNode_NonMapProperty verifies that reserved_keys on a
// non-map property type (which would be a schema error) produces no rules.
func TestReservedKeyRulesForNode_NonMapProperty(t *testing.T) {
	nt := schema.NodeType{
		Name:     "test_node",
		Category: "meta",
		Properties: []schema.Property{
			{
				Name: "some_string",
				Type: "string",
				ReservedKeys: map[string]schema.ReservedKeyDef{
					"env": {ClosedVocabulary: []string{"prod", "dev"}},
				},
			},
		},
	}
	// reserved_keys on a non-map property type is ignored — only map types are processed.
	rules := reservedKeyRulesForNode(nt)
	assert.Empty(t, rules, "reserved keys on non-map property should produce no rules")
}

// TestReservedKeyRulesForNode_SingleKey verifies correct CEL rule generation for
// a single reserved key with a closed vocabulary.
func TestReservedKeyRulesForNode_SingleKey(t *testing.T) {
	nt := makeNodeTypeWithReservedKeys("compliance_signal", "resource_tags", map[string]schema.ReservedKeyDef{
		"env": {
			ClosedVocabulary: []string{"prod", "staging", "dev", "test"},
			Description:      "Deployment environment",
		},
	})
	rules := reservedKeyRulesForNode(nt)
	require.Len(t, rules, 1, "single reserved key should produce one rule")

	rule := rules[0]
	// The expression must test for absence OR presence in the vocabulary.
	assert.Contains(t, rule.Expr, `"env" in self.resource_tags`,
		"rule expression should check key membership in the map")
	assert.Contains(t, rule.Expr, `self.resource_tags["env"]`,
		"rule expression should index the map by the reserved key")
	assert.Contains(t, rule.Expr, `"prod"`, "vocabulary value 'prod' must appear in the rule")
	assert.Contains(t, rule.Expr, `"staging"`, "vocabulary value 'staging' must appear in the rule")
	assert.Contains(t, rule.Expr, `"dev"`, "vocabulary value 'dev' must appear in the rule")
	assert.Contains(t, rule.Expr, `"test"`, "vocabulary value 'test' must appear in the rule")
	// The rule must be a disjunction: absent OR in-vocab.
	assert.Contains(t, rule.Expr, "||",
		"rule must permit the key to be absent (disjunction with absence check)")

	// The message must name the field, the key, and the vocabulary.
	assert.Contains(t, rule.Message, "resource_tags", "message should name the map field")
	assert.Contains(t, rule.Message, "env", "message should name the reserved key")
}

// TestReservedKeyRulesForNode_MultipleKeys verifies that multiple reserved keys
// produce one rule each and that the output order is deterministic (sorted).
func TestReservedKeyRulesForNode_MultipleKeys(t *testing.T) {
	nt := makeNodeTypeWithReservedKeys("compliance_signal", "resource_tags", map[string]schema.ReservedKeyDef{
		"data_class": {ClosedVocabulary: []string{"public", "internal", "pii", "phi", "secret"}},
		"env":        {ClosedVocabulary: []string{"prod", "staging", "dev", "test"}},
		"legal_hold": {ClosedVocabulary: []string{"true", "false"}},
		"residency":  {ClosedVocabulary: []string{"US", "EU", "APAC", "LATAM", "AFR", "MEA"}},
	})
	rules := reservedKeyRulesForNode(nt)
	require.Len(t, rules, 4, "four reserved keys should produce four rules")

	// Verify deterministic ordering — must be sorted by key name.
	keys := make([]string, len(rules))
	for i, r := range rules {
		// Extract the key from the expression: the key name appears after the first `"` in the expr.
		// We look for the known key names in the expression to identify ordering.
		for _, k := range []string{"data_class", "env", "legal_hold", "residency"} {
			if strings.Contains(r.Expr, `"`+k+`"`) {
				keys[i] = k
				break
			}
		}
	}
	assert.Equal(t, []string{"data_class", "env", "legal_hold", "residency"}, keys,
		"rules must be emitted in sorted key order for deterministic output")
}

// TestReservedKeyRulesForNode_AbsencePermitted verifies that the generated CEL
// rule accepts the case where the reserved key is absent from the map entirely.
// This validates the "absent OR in-vocab" semantics required by Req 12.4.
func TestReservedKeyRulesForNode_AbsencePermitted(t *testing.T) {
	nt := makeNodeTypeWithReservedKeys("compliance_signal", "resource_tags", map[string]schema.ReservedKeyDef{
		"env": {ClosedVocabulary: []string{"prod", "dev"}},
	})
	rules := reservedKeyRulesForNode(nt)
	require.Len(t, rules, 1)

	// The expression must start with the negated membership test so that an absent
	// key short-circuits the vocabulary check.
	expr := rules[0].Expr
	assert.True(t,
		strings.HasPrefix(expr, `!("env" in self.resource_tags)`),
		"absence check must come first: got %q", expr)
}

// TestReservedKeyRulesForNode_DeterministicAcrossRuns verifies that calling
// reservedKeyRulesForNode twice on the same input produces byte-identical output
// (required by make verify-idempotent — Req 12.4 / NFR generation determinism).
func TestReservedKeyRulesForNode_DeterministicAcrossRuns(t *testing.T) {
	nt := makeNodeTypeWithReservedKeys("compliance_signal", "resource_tags", map[string]schema.ReservedKeyDef{
		"data_class": {ClosedVocabulary: []string{"public", "internal", "pii", "phi", "secret"}},
		"env":        {ClosedVocabulary: []string{"prod", "staging", "dev", "test"}},
		"legal_hold": {ClosedVocabulary: []string{"true", "false"}},
		"residency":  {ClosedVocabulary: []string{"US", "EU", "APAC", "LATAM", "AFR", "MEA"}},
	})

	first := reservedKeyRulesForNode(nt)
	second := reservedKeyRulesForNode(nt)

	require.Len(t, second, len(first), "both runs must produce the same number of rules")
	for i := range first {
		assert.Equal(t, first[i].Expr, second[i].Expr,
			"rule[%d] expression must be identical across runs", i)
		assert.Equal(t, first[i].Message, second[i].Message,
			"rule[%d] message must be identical across runs", i)
	}
}

// TestHasAnyRules_WithReservedKeys verifies that hasAnyRules returns true for a
// node type that has no explicit validations but does have reserved-key properties.
func TestHasAnyRules_WithReservedKeys(t *testing.T) {
	nt := makeNodeTypeWithReservedKeys("compliance_signal", "resource_tags", map[string]schema.ReservedKeyDef{
		"env": {ClosedVocabulary: []string{"prod", "dev"}},
	})

	// Simulate the hasAnyRules template function inline to avoid template overhead.
	hasAnyRules := func(n schema.NodeType) bool {
		if len(n.Validations) > 0 {
			return true
		}
		return len(reservedKeyRulesForNode(n)) > 0
	}

	assert.Empty(t, nt.Validations, "test node should have no explicit validations")
	assert.True(t, hasAnyRules(nt),
		"hasAnyRules must return true when reserved-key rules exist, even without explicit validations")
}

// TestHasAnyRules_NoRulesAtAll verifies that hasAnyRules returns false for a
// node type with neither explicit validations nor reserved keys.
func TestHasAnyRules_NoRulesAtAll(t *testing.T) {
	nt := schema.NodeType{
		Name:     "plain_node",
		Category: "asset",
		Properties: []schema.Property{
			{Name: "name", Type: "string", Required: true},
			{Name: "custom", Type: "map<string,string>"},
		},
	}
	hasAnyRules := func(n schema.NodeType) bool {
		if len(n.Validations) > 0 {
			return true
		}
		return len(reservedKeyRulesForNode(n)) > 0
	}
	assert.False(t, hasAnyRules(nt), "hasAnyRules must return false when no rules exist")
}
