// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Zero Root AI

package taxonomy

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// wellFormedYAML is a fully valid ontology used as the positive baseline.
const wellFormedYAML = `
version: "1.0"
prefixes:
  soc2: "https://trust.aicpa.org/soc2#"
  mitre: "https://attack.mitre.org/techniques/"
hierarchies:
  - node_type: control
    label: soc2:CC6
    sub_class_of: ""
  - node_type: control
    label: soc2:CC6.1
    sub_class_of: soc2:CC6
  - node_type: control
    label: soc2:CC6.2
    sub_class_of: soc2:CC6
  - node_type: technique
    label: mitre:T1190
    sub_class_of: ""
  - node_type: technique
    label: mitre:T1190.001
    sub_class_of: mitre:T1190
equivalences:
  - [soc2:CC6.1, mitre:T1190]
ifps:
  - node_type: control
    property: control_id
  - node_type: technique
    property: technique_id
`

func TestParse_WellFormed(t *testing.T) {
	o, err := Parse([]byte(wellFormedYAML))
	require.NoError(t, err)

	assert.Equal(t, "1.0", o.Version)
	assert.Len(t, o.Prefixes, 2)
	assert.Equal(t, "https://trust.aicpa.org/soc2#", o.Prefixes["soc2"])
	assert.Len(t, o.Hierarchies, 5)
	assert.Len(t, o.Equivalences, 1)
	assert.Equal(t, [2]string{"soc2:CC6.1", "mitre:T1190"}, o.Equivalences[0])
	assert.Len(t, o.IFPs, 2)
	assert.Equal(t, "control", o.IFPs[0].NodeType)
	assert.Equal(t, "control_id", o.IFPs[0].Property)
}

func TestParse_MissingVersion(t *testing.T) {
	yml := `
prefixes:
  soc2: "https://trust.aicpa.org/soc2#"
hierarchies: []
`
	_, err := Parse([]byte(yml))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "version")
}

func TestParse_EquivalenceBadLength(t *testing.T) {
	yml := `
version: "1.0"
prefixes:
  soc2: "https://trust.aicpa.org/soc2#"
hierarchies: []
equivalences:
  - [soc2:CC6.1, soc2:CC6.2, soc2:CC6.3]
`
	_, err := Parse([]byte(yml))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "2 IRIs")
}

func TestParse_EmptyHierarchies(t *testing.T) {
	yml := `
version: "1.0"
prefixes:
  soc2: "https://trust.aicpa.org/soc2#"
hierarchies: []
`
	o, err := Parse([]byte(yml))
	require.NoError(t, err)
	assert.Empty(t, o.Hierarchies)
	assert.Empty(t, o.Equivalences)
	assert.Empty(t, o.IFPs)
}

func TestParse_InvalidYAML(t *testing.T) {
	// gopkg.in/yaml.v3 treats many malformed inputs as valid YAML (mapping
	// scalars as keys). We verify that Parse still rejects obviously broken
	// input by checking that any resulting error is surfaced, OR that the
	// parsed Ontology fails the version-required check.
	_, err := Parse([]byte("\x00\x01\x02 not yaml at all \xff\xfe"))
	require.Error(t, err)
}

func TestValidate_WellFormed(t *testing.T) {
	o, err := Parse([]byte(wellFormedYAML))
	require.NoError(t, err)
	require.NoError(t, o.Validate())
}

func TestValidate_SubClassOfCycle(t *testing.T) {
	yml := `
version: "1.0"
prefixes:
  soc2: "https://trust.aicpa.org/soc2#"
hierarchies:
  - node_type: control
    label: soc2:A
    sub_class_of: soc2:C
  - node_type: control
    label: soc2:B
    sub_class_of: soc2:A
  - node_type: control
    label: soc2:C
    sub_class_of: soc2:B
`
	o, err := Parse([]byte(yml))
	require.NoError(t, err)
	err = o.Validate()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "cycle")
}

func TestValidate_UnknownPrefix(t *testing.T) {
	yml := `
version: "1.0"
prefixes:
  soc2: "https://trust.aicpa.org/soc2#"
hierarchies:
  - node_type: control
    label: unknown:CC6
    sub_class_of: ""
`
	o, err := Parse([]byte(yml))
	require.NoError(t, err)
	err = o.Validate()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "prefix")
	assert.Contains(t, err.Error(), "unknown")
}

func TestValidate_UnknownPrefixInSubClassOf(t *testing.T) {
	yml := `
version: "1.0"
prefixes:
  soc2: "https://trust.aicpa.org/soc2#"
hierarchies:
  - node_type: control
    label: soc2:CC6.1
    sub_class_of: ghost:CC6
`
	o, err := Parse([]byte(yml))
	require.NoError(t, err)
	err = o.Validate()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "prefix")
}

func TestValidate_MalformedIRI(t *testing.T) {
	tests := []struct {
		name string
		yml  string
	}{
		{
			name: "no colon in label",
			yml: `
version: "1.0"
prefixes:
  soc2: "https://trust.aicpa.org/soc2#"
hierarchies:
  - node_type: control
    label: noColon
`,
		},
		{
			name: "empty localname in label",
			yml: `
version: "1.0"
prefixes:
  soc2: "https://trust.aicpa.org/soc2#"
hierarchies:
  - node_type: control
    label: soc2:
`,
		},
		{
			name: "empty string label",
			yml: `
version: "1.0"
prefixes:
  soc2: "https://trust.aicpa.org/soc2#"
hierarchies:
  - node_type: control
    label: ""
`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			o, err := Parse([]byte(tt.yml))
			// Parse itself may succeed for some forms; Validate must catch the issue.
			if err != nil {
				// Parse already caught it — acceptable.
				return
			}
			err = o.Validate()
			require.Error(t, err, "expected validation error for malformed IRI")
		})
	}
}

func TestValidate_MissingIFPProperty(t *testing.T) {
	// An IFP that references a node_type not present in any hierarchy entry.
	yml := `
version: "1.0"
prefixes:
  soc2: "https://trust.aicpa.org/soc2#"
hierarchies:
  - node_type: control
    label: soc2:CC6
    sub_class_of: ""
ifps:
  - node_type: ghost_type
    property: id
`
	o, err := Parse([]byte(yml))
	require.NoError(t, err)
	err = o.Validate()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "ghost_type")
}

func TestValidate_EquivalenceChainTransitivity(t *testing.T) {
	// Equivalence pairs that form a transitive chain are parser-valid; richer
	// closure reasoning lives in the daemon Reasoner. This test verifies that
	// the parser correctly stores the pairs as [2]string values, preserving the
	// data shape that the Reasoner will consume.
	yml := `
version: "1.0"
prefixes:
  soc2: "https://trust.aicpa.org/soc2#"
  nist: "https://csrc.nist.gov/glossary/term/"
  mitre: "https://attack.mitre.org/techniques/"
hierarchies: []
equivalences:
  - [soc2:CC6.1, nist:access-control]
  - [nist:access-control, mitre:T1190]
`
	o, err := Parse([]byte(yml))
	require.NoError(t, err)
	// Validate must pass — transitivity closure is not checked at parse time.
	require.NoError(t, o.Validate())

	// Verify the data shape is preserved correctly.
	require.Len(t, o.Equivalences, 2)
	assert.Equal(t, [2]string{"soc2:CC6.1", "nist:access-control"}, o.Equivalences[0])
	assert.Equal(t, [2]string{"nist:access-control", "mitre:T1190"}, o.Equivalences[1])
}

func TestValidate_MissingNodeTypeInHierarchy(t *testing.T) {
	yml := `
version: "1.0"
prefixes:
  soc2: "https://trust.aicpa.org/soc2#"
hierarchies:
  - node_type: ""
    label: soc2:CC6
`
	o, err := Parse([]byte(yml))
	require.NoError(t, err)
	err = o.Validate()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "node_type is required")
}

func TestValidate_EmptyHierarchiesNoIFPCheck(t *testing.T) {
	// When hierarchies is empty the IFP node_type cross-reference is skipped
	// (there are no node types to check against). An IFP with an arbitrary
	// node_type should be accepted.
	yml := `
version: "1.0"
hierarchies: []
ifps:
  - node_type: any_type
    property: id
`
	o, err := Parse([]byte(yml))
	require.NoError(t, err)
	require.NoError(t, o.Validate())
}

func TestValidate_NoPrefixesNoPrefixCheck(t *testing.T) {
	// When no prefixes are declared the prefix-resolution check is skipped.
	// This supports minimal ontologies that embed raw IRIs as localnames.
	// NOTE: this is a deliberate design choice — the check is opt-in when
	// prefixes are declared.
	yml := `
version: "1.0"
hierarchies: []
`
	o, err := Parse([]byte(yml))
	require.NoError(t, err)
	require.NoError(t, o.Validate())
}

func TestIsValidIRI(t *testing.T) {
	tests := []struct {
		iri   string
		valid bool
	}{
		{"soc2:CC6.1", true},
		{"mitre:T1190.001", true},
		{"cwe:CWE-79", true},
		{"a:b", true},
		{"", false},
		{"nocodon", false},
		{":localonly", false},
		{"prefix:", false},
		{"has space:local", false},
		{"prefix:has space", false},
	}

	for _, tt := range tests {
		t.Run(tt.iri, func(t *testing.T) {
			assert.Equal(t, tt.valid, isValidIRI(tt.iri))
		})
	}
}

func TestValidate_SelfCycle(t *testing.T) {
	// A node that is its own parent.
	yml := `
version: "1.0"
prefixes:
  soc2: "https://trust.aicpa.org/soc2#"
hierarchies:
  - node_type: control
    label: soc2:CC6
    sub_class_of: soc2:CC6
`
	o, err := Parse([]byte(yml))
	require.NoError(t, err)
	err = o.Validate()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "cycle")
}

func TestValidate_CrossTypeHierarchiesAreIndependent(t *testing.T) {
	// A label in node_type "technique" that shares an IRI with a "control"
	// label should not cause a false cycle — cycles are per-node-type.
	yml := `
version: "1.0"
prefixes:
  soc2: "https://trust.aicpa.org/soc2#"
hierarchies:
  - node_type: control
    label: soc2:CC6
    sub_class_of: ""
  - node_type: technique
    label: soc2:CC6
    sub_class_of: ""
`
	o, err := Parse([]byte(yml))
	require.NoError(t, err)
	require.NoError(t, o.Validate())
}

func TestValidate_IFPMissingNodeType(t *testing.T) {
	yml := `
version: "1.0"
hierarchies: []
ifps:
  - node_type: ""
    property: id
`
	o, err := Parse([]byte(yml))
	require.NoError(t, err)
	err = o.Validate()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "node_type is required")
}

func TestValidate_IFPMissingProperty(t *testing.T) {
	yml := `
version: "1.0"
prefixes:
  soc2: "https://trust.aicpa.org/soc2#"
hierarchies:
  - node_type: control
    label: soc2:CC6
ifps:
  - node_type: control
    property: ""
`
	o, err := Parse([]byte(yml))
	require.NoError(t, err)
	err = o.Validate()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "property is required")
}

func TestValidate_MalformedEquivalenceIRI(t *testing.T) {
	yml := `
version: "1.0"
prefixes:
  soc2: "https://trust.aicpa.org/soc2#"
hierarchies: []
equivalences:
  - [soc2:CC6.1, notaniri]
`
	o, err := Parse([]byte(yml))
	require.NoError(t, err)
	err = o.Validate()
	require.Error(t, err)
	assert.Contains(t, strings.ToLower(err.Error()), "iri")
}

func TestValidate_UnknownPrefixInEquivalence(t *testing.T) {
	yml := `
version: "1.0"
prefixes:
  soc2: "https://trust.aicpa.org/soc2#"
hierarchies: []
equivalences:
  - [soc2:CC6.1, ghost:CC6]
`
	o, err := Parse([]byte(yml))
	require.NoError(t, err)
	err = o.Validate()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "prefix")
}
