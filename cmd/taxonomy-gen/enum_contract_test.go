// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Zero Root AI

package main

import (
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

// The committed enum IS the wire contract. These tests compare it to
// taxonomy/core.yaml by (name, number) pair, never by line or by order, so
// reordering core.yaml is free and changing a number is not.
//
// The repo already has a drift gate that runs `make generate` and diffs the
// output. That gate is necessary but not sufficient: when numbering was derived
// from position, deleting one node type moved three live values, and the gate
// reported it as generated files drifting with a message telling you to commit
// the regenerated output. It could not tell "you added a type" from "three
// types changed meaning". These tests can.

const (
	coreYAMLPath = "../../taxonomy/core.yaml"
	protoPath    = "../../api/proto/taxonomy/v1/taxonomy.proto"
)

type yamlEntry struct {
	Name    string `yaml:"name"`
	Number  int    `yaml:"number"`
	Retired bool   `yaml:"retired"`
}

type yamlTaxonomy struct {
	NodeTypes         []yamlEntry `yaml:"node_types"`
	RelationshipTypes []yamlEntry `yaml:"relationship_types"`
}

func loadCoreYAML(t *testing.T) yamlTaxonomy {
	t.Helper()
	// #nosec G304 -- a fixed repo-relative path to the checked-in taxonomy.
	raw, err := os.ReadFile(filepath.Clean(coreYAMLPath))
	if err != nil {
		t.Fatalf("read %s: %v", coreYAMLPath, err)
	}
	var tax yamlTaxonomy
	if err := yaml.Unmarshal(raw, &tax); err != nil {
		t.Fatalf("parse %s: %v", coreYAMLPath, err)
	}
	if len(tax.NodeTypes) == 0 || len(tax.RelationshipTypes) == 0 {
		t.Fatal("core.yaml parsed with no node types or no relationship types; this test would measure nothing")
	}
	return tax
}

func loadProto(t *testing.T) string {
	t.Helper()
	// #nosec G304 -- a fixed repo-relative path to the generated proto.
	raw, err := os.ReadFile(filepath.Clean(protoPath))
	if err != nil {
		t.Fatalf("read %s: %v", protoPath, err)
	}
	return string(raw)
}

// enumBody returns the text between `enum <name> {` and its closing brace.
func enumBody(t *testing.T, proto, name string) string {
	t.Helper()
	start := strings.Index(proto, "enum "+name+" {")
	if start < 0 {
		t.Fatalf("proto has no enum %s", name)
	}
	rest := proto[start:]
	end := strings.Index(rest, "\n}")
	if end < 0 {
		t.Fatalf("enum %s is not closed", name)
	}
	return rest[:end]
}

var (
	memberRe   = regexp.MustCompile(`(?m)^\s+([A-Z][A-Z0-9_]*)\s*=\s*(\d+);`)
	reservedNo = regexp.MustCompile(`(?m)^\s+reserved\s+(\d+);`)
	reservedNm = regexp.MustCompile(`(?m)^\s+reserved\s+"([A-Z][A-Z0-9_]*)";`)
)

func members(t *testing.T, body string) map[string]int {
	t.Helper()
	out := map[string]int{}
	for _, m := range memberRe.FindAllStringSubmatch(body, -1) {
		n, err := strconv.Atoi(m[2])
		if err != nil {
			t.Fatalf("enum member %s has a non-numeric value %q", m[1], m[2])
		}
		out[m[1]] = n
	}
	if len(out) == 0 {
		t.Fatal("parsed no enum members; this test would measure nothing")
	}
	return out
}

func set(matches [][]string) map[string]bool {
	out := map[string]bool{}
	for _, m := range matches {
		out[m[1]] = true
	}
	return out
}

func upperSnake(name string) string { return strings.ToUpper(name) }

// checkEnum is the whole contract for one enum: every live entry is a member at
// its declared number, every retired entry is reserved by number and by name and
// is NOT a member, and the enum holds nothing core.yaml does not declare.
func checkEnum(t *testing.T, proto, enumName, prefix string, entries []yamlEntry) {
	t.Helper()
	body := enumBody(t, proto, enumName)
	got := members(t, body)
	reservedNumbers := map[int]bool{}
	for _, m := range reservedNo.FindAllStringSubmatch(body, -1) {
		n, _ := strconv.Atoi(m[1])
		reservedNumbers[n] = true
	}
	reservedNames := set(reservedNm.FindAllStringSubmatch(body, -1))

	declared := map[string]bool{prefix + "UNSPECIFIED": true}
	for _, e := range entries {
		member := prefix + upperSnake(e.Name)
		declared[member] = true

		if e.Retired {
			if n, isMember := got[member]; isMember {
				t.Errorf("%s is retired in core.yaml but %s is still an enum member at %d. "+
					"Something can still construct one", e.Name, member, n)
			}
			if !reservedNumbers[e.Number] {
				t.Errorf("%s is retired holding number %d, but %s does not reserve %d. "+
					"The next type added would take it and change the meaning of data already on the wire",
					e.Name, e.Number, enumName, e.Number)
			}
			if !reservedNames[member] {
				t.Errorf("%s is retired but %s does not reserve the name %q", e.Name, enumName, member)
			}
			continue
		}

		n, isMember := got[member]
		if !isMember {
			t.Errorf("core.yaml declares %s but %s has no member %s", e.Name, enumName, member)
			continue
		}
		if n != e.Number {
			t.Errorf("%s: core.yaml says %d, %s says %d. One of the two was hand-edited",
				e.Name, e.Number, enumName, n)
		}
		if reservedNumbers[e.Number] {
			t.Errorf("%s is live at %d but %s also reserves %d", e.Name, e.Number, enumName, e.Number)
		}
	}

	for member := range got {
		if !declared[member] {
			t.Errorf("%s has member %s, which core.yaml does not declare. "+
				"A generated enum cannot gain a value on its own, so this was hand-edited", enumName, member)
		}
	}
}

func TestCoreNodeTypeEnumMatchesCoreYAML(t *testing.T) {
	tax := loadCoreYAML(t)
	checkEnum(t, loadProto(t), "CoreNodeType", "CORE_NODE_TYPE_", tax.NodeTypes)
}

func TestCoreRelationTypeEnumMatchesCoreYAML(t *testing.T) {
	tax := loadCoreYAML(t)
	checkEnum(t, loadProto(t), "CoreRelationType", "CORE_RELATION_TYPE_", tax.RelationshipTypes)
}

// TestEnumNumbersAreUnique: two names sharing a value is the corruption that
// reusing a retired number causes, and it is worth asserting against the
// committed artifact and not only against the parser.
func TestEnumNumbersAreUnique(t *testing.T) {
	proto := loadProto(t)
	for _, name := range []string{"CoreNodeType", "CoreRelationType"} {
		t.Run(name, func(t *testing.T) {
			seen := map[int]string{}
			for member, n := range members(t, enumBody(t, proto, name)) {
				if other, dup := seen[n]; dup {
					t.Errorf("%s and %s both have value %d", other, member, n)
				}
				seen[n] = member
			}
		})
	}
}

// TestRetiredTypesAreGeneratedNowhere is the second acceptance criterion of
// sdk#132: a retired type must produce no message, no helper, no validator, no
// constant and no query binding, so that nothing can construct one.
//
// It reads the committed generated files rather than trusting the templates,
// because the templates are what would be wrong.
func TestRetiredTypesAreGeneratedNowhere(t *testing.T) {
	tax := loadCoreYAML(t)

	var retired []string
	for _, e := range tax.NodeTypes {
		if e.Retired {
			retired = append(retired, e.Name)
		}
	}
	for _, e := range tax.RelationshipTypes {
		if e.Retired {
			retired = append(retired, e.Name)
		}
	}
	if len(retired) == 0 {
		t.Skip("no type is retired, so there is nothing to check")
	}

	// Every file a generator writes, from the taxonomy-gen recipe in the
	// Makefile. The proto is excluded: it is the one file that MUST name a
	// retired type, in its reserved lines.
	generated := []string{
		"../../graphrag/domain/domain_generated.go",
		"../../graphrag/validation/validators_generated.go",
		"../../graphrag/constants_generated.go",
		"../../graphrag/query/query_generated.go",
		"../../graphrag/helpers_generated.go",
		"../../graphrag/taxonomy/relationships_generated.go",
	}

	checked := 0
	for _, path := range generated {
		// #nosec G304 -- a fixed list of repo-relative generated files.
		raw, err := os.ReadFile(filepath.Clean(path))
		if err != nil {
			t.Fatalf("read %s: %v. If a generated file moved, this list is stale and the test is measuring less than it claims", path, err)
		}
		checked++
		body := string(raw)
		for _, name := range retired {
			for _, form := range []string{name, upperSnake(name), camel(name)} {
				if strings.Contains(body, form) {
					t.Errorf("%s names the retired type %q (as %q). "+
						"A retired type must be generated nowhere, or something can still build one",
						path, name, form)
				}
			}
		}
	}
	if checked != len(generated) {
		t.Fatalf("checked %d of %d generated files", checked, len(generated))
	}
	t.Logf("checked %d generated files for %d retired types", checked, len(retired))
}

// camel turns compliance_signal into ComplianceSignal, the Go identifier a
// generator would emit.
func camel(snake string) string {
	var b strings.Builder
	for _, part := range strings.Split(snake, "_") {
		if part == "" {
			continue
		}
		b.WriteString(strings.ToUpper(part[:1]))
		b.WriteString(part[1:])
	}
	return b.String()
}
