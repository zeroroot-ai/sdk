// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Zero Root AI

package generator

import (
	"bytes"
	"fmt"
	"os"
	"strings"
	"text/template"

	"github.com/zeroroot-ai/sdk/cmd/taxonomy-gen/schema"
)

// reservedKeyRule holds the computed CEL rule and error message for a single
// reserved-key closed-vocabulary constraint.
type reservedKeyRule struct {
	// Expr is the CEL expression string, ready for embedding in a ruleSpec.
	Expr string
	// Message is the human-readable validation-failure message.
	Message string
}

// reservedKeyRulesForNode returns all reserved-key CEL rules for a given node
// type, ordered deterministically (property name then key name).
//
// Each map property with reserved_keys produces one CEL rule per key using the
// pattern:
//
//	!("key" in self.prop) || self.prop["key"] in ["val1", "val2", ...]
//
// This encodes "absent OR in-vocabulary" semantics: if the key is absent the
// rule short-circuits to true; if present the value must be one of the
// declared vocabulary members.
//
// NOTE: The CEL rules generated here assume the proto field is a real map type.
// In the current taxonomy, map-typed properties are stored as JSON-serialized
// optional strings in the proto layer, so these rules CANNOT be evaluated
// against a compiled taxonomypb message without additional proto map support.
// They are generated here for completeness and testing, but the template
// function used during code generation (reservedKeyRulesForTemplate) returns
// an empty slice to defer CEL emission until the metadata-rider spec ships
// proper proto map fields.
func reservedKeyRulesForNode(nt schema.NodeType) []reservedKeyRule {
	var rules []reservedKeyRule
	for _, p := range nt.Properties {
		if !p.IsMap() || !p.HasReservedKeys() {
			continue
		}
		// Iterate reserved keys in sorted order for deterministic output.
		for _, keyName := range p.SortedReservedKeys() {
			def := p.ReservedKeys[keyName]
			if len(def.ClosedVocabulary) == 0 {
				continue
			}
			expr := buildReservedKeyExpr(p.Name, keyName, def.ClosedVocabulary)
			msg := buildReservedKeyMessage(p.Name, keyName, def.ClosedVocabulary)
			rules = append(rules, reservedKeyRule{Expr: expr, Message: msg})
		}
	}
	return rules
}

// buildReservedKeyExpr returns a CEL expression for a single reserved-key constraint:
//
//	!("key" in self.prop) || self.prop["key"] in ["val1", "val2", ...]
func buildReservedKeyExpr(propName, keyName string, vocab []string) string {
	// Absence check: if the key is not present the whole expression is true.
	absence := fmt.Sprintf(`!(%q in self.%s)`, keyName, propName)
	// Presence check: the value must be one of the vocabulary members.
	quotedVals := make([]string, len(vocab))
	for i, v := range vocab {
		quotedVals[i] = fmt.Sprintf("%q", v)
	}
	inList := strings.Join(quotedVals, ", ")
	presence := fmt.Sprintf(`self.%s[%q] in [%s]`, propName, keyName, inList)
	return absence + " || " + presence
}

// buildReservedKeyMessage returns a human-readable error message for a
// reserved-key vocabulary violation.
func buildReservedKeyMessage(propName, keyName string, vocab []string) string {
	quotedVals := make([]string, len(vocab))
	for i, v := range vocab {
		quotedVals[i] = fmt.Sprintf("%q", v)
	}
	return fmt.Sprintf("%s.%s must be one of %s when present; got invalid value",
		propName, keyName, strings.Join(quotedVals, ", "))
}

// reservedKeyRulesForTemplate is the template-facing variant of reservedKeyRulesForNode.
// It returns an empty slice for all node types because map-typed properties are stored
// as JSON-serialized optional strings in the proto layer, and CEL cannot evaluate
// map-key access expressions against a string field. The rules will be emitted once
// the audit-metadata-riders spec ships proper proto map fields.
func reservedKeyRulesForTemplate(_ schema.NodeType) []reservedKeyRule {
	return nil
}

// GenerateValidators generates CEL validators from the taxonomy.
func GenerateValidators(taxonomy *schema.Taxonomy, outputPath, pkgName string) error {
	tmpl, err := template.New("validators").Funcs(validatorsFuncMap()).Parse(validatorsTemplate)
	if err != nil {
		return fmt.Errorf("failed to parse template: %w", err)
	}

	data := struct {
		*schema.Taxonomy
		Package    string
		SourcePath string
	}{
		Taxonomy:   taxonomy,
		Package:    "validation",
		SourcePath: "taxonomy/core.yaml",
	}

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, data); err != nil {
		return fmt.Errorf("failed to execute template: %w", err)
	}

	if err := os.WriteFile(outputPath, buf.Bytes(), 0644); err != nil {
		return fmt.Errorf("failed to write file: %w", err)
	}

	return nil
}

func validatorsFuncMap() template.FuncMap {
	return template.FuncMap{
		"toPascalCase":   toPascalCase,
		"toLowerSnake":   toLowerSnake,
		"hasValidations": func(nt schema.NodeType) bool { return len(nt.Validations) > 0 },
		"escapeCEL":      escapeCEL,
		// hasAnyRules returns true when a node type has either explicit validations
		// or reserved-key rules that need to be compiled into the CEL program list.
		// NOTE: reservedKeyRulesForTemplate returns nil until the metadata-rider spec
		// ships proto map support, so this only considers explicit validations for now.
		"hasAnyRules": func(nt schema.NodeType) bool {
			if len(nt.Validations) > 0 {
				return true
			}
			return len(reservedKeyRulesForTemplate(nt)) > 0
		},
		// reservedKeyRules returns the closed-vocabulary CEL rules for a node type.
		// Template uses the deferred variant that returns nil until proto map support
		// is available in the audit-metadata-riders spec.
		"reservedKeyRules": reservedKeyRulesForTemplate,
	}
}

// escapeCEL escapes a CEL expression for Go string literal.
func escapeCEL(s string) string {
	s = strings.ReplaceAll(s, `\`, `\\`)
	s = strings.ReplaceAll(s, `"`, `\"`)
	s = strings.ReplaceAll(s, "\n", `\n`)
	return s
}

const validatorsTemplate = `// Code generated by taxonomy-gen from taxonomy YAML. DO NOT EDIT.
// Source: {{.SourcePath}}
// Taxonomy Version: {{.Version}}

package {{.Package}}

import (
	"fmt"
	"sync"

	"github.com/google/cel-go/cel"
	taxonomypb "github.com/zeroroot-ai/sdk/api/gen/taxonomy/v1"
)

var (
	initOnce sync.Once
	initErr  error

	// Pre-compiled CEL programs for each type
{{- range .NodeTypes}}
{{- if hasAnyRules .}}
	{{.Name}}Validators []*compiledRule
{{- end}}
{{- end}}

	// Core types set
	coreTypes = map[string]bool{
{{- range .NodeTypes}}
		"{{.Name}}": true,
{{- end}}
	}

	// Parent requirements
	parentRequirements = map[string]ParentRequirement{
{{- range .NodeTypes}}
{{- if .Parent}}
		"{{.Name}}": {ParentType: "{{.Parent.Type}}", Relationship: "{{.Parent.Relationship}}", Required: {{.Parent.Required}}},
{{- end}}
{{- end}}
	}
)

type compiledRule struct {
	program cel.Program
	message string
}

// ParentRequirement defines the parent relationship for a node type.
type ParentRequirement struct {
	ParentType   string
	Relationship string
	Required     bool
}

func init() {
	initOnce.Do(func() {
		initErr = initValidators()
	})
}

func initValidators() error {
{{- range .NodeTypes}}
{{- if hasAnyRules .}}
	// Initialize {{.Name}} validators
	{
		env, err := cel.NewEnv(
			cel.Types(&taxonomypb.{{.Name | toPascalCase}}{}),
			cel.Variable("self", cel.ObjectType("taxonomy.v1.{{.Name | toPascalCase}}")),
		)
		if err != nil {
			return fmt.Errorf("failed to create CEL environment for {{.Name}}: %w", err)
		}
		{{.Name}}Validators = compileRules(env, []ruleSpec{
{{- range .Validations}}
			{expr: "{{.Rule | escapeCEL}}", message: "{{.Message}}"},
{{- end}}
{{- range (reservedKeyRules .)}}
			{expr: "{{.Expr | escapeCEL}}", message: "{{.Message | escapeCEL}}"},
{{- end}}
		})
	}
{{- end}}
{{- end}}
	return nil
}

type ruleSpec struct {
	expr    string
	message string
}

func compileRules(env *cel.Env, specs []ruleSpec) []*compiledRule {
	rules := make([]*compiledRule, 0, len(specs))
	for _, spec := range specs {
		ast, issues := env.Compile(spec.expr)
		if issues != nil && issues.Err() != nil {
			panic(fmt.Sprintf("failed to compile CEL rule '%s': %v", spec.expr, issues.Err()))
		}
		prg, err := env.Program(ast)
		if err != nil {
			panic(fmt.Sprintf("failed to create CEL program for '%s': %v", spec.expr, err))
		}
		rules = append(rules, &compiledRule{program: prg, message: spec.message})
	}
	return rules
}

// ==================== PUBLIC API ====================

// IsCoreType returns true if the node type is a core (validated) type.
func IsCoreType(nodeType string) bool {
	return coreTypes[nodeType]
}

// GetParentRequirement returns the parent requirement for a node type.
func GetParentRequirement(nodeType string) (ParentRequirement, bool) {
	req, ok := parentRequirements[nodeType]
	return req, ok
}

// ValidateNode validates any node. Custom types pass through without validation.
func ValidateNode(nodeType string, properties map[string]any, hasParent bool) error {
	// Custom type - no validation
	if !IsCoreType(nodeType) {
		return nil
	}

	// Check parent requirement
	if req, ok := parentRequirements[nodeType]; ok && req.Required && !hasParent {
		return fmt.Errorf("%s requires a parent of type %s", nodeType, req.ParentType)
	}

	return nil
}

// ==================== TYPE-SPECIFIC VALIDATORS ====================
{{range .NodeTypes}}
// Validate{{.Name | toPascalCase}} validates a {{.Name | toPascalCase}} proto.
func Validate{{.Name | toPascalCase}}(p *taxonomypb.{{.Name | toPascalCase}}) error {
	if initErr != nil {
		return fmt.Errorf("validator initialization failed: %w", initErr)
	}
{{- if hasAnyRules .}}
	for _, rule := range {{.Name}}Validators {
		result, _, err := rule.program.Eval(map[string]any{"self": p})
		if err != nil {
			return fmt.Errorf("validation error: %w", err)
		}
		if result.Value() != true {
			return fmt.Errorf("{{.Name}} validation failed: %s", rule.message)
		}
	}
{{- end}}
	return nil
}
{{end}}`
