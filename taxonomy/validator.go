// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Zero Root AI

// Package taxonomy provides taxonomy validation for Gibson security findings.
// It validates finding categories, entity types, relationship types, and properties
// against the core taxonomy schema defined in core.yaml.
package taxonomy

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"github.com/agnivade/levenshtein"
)

// ValidationMode determines how validation errors are handled.
type ValidationMode string

const (
	// ValidationModeStrict returns errors for any validation failures.
	ValidationModeStrict ValidationMode = "strict"

	// ValidationModeWarn logs warnings but allows invalid values.
	ValidationModeWarn ValidationMode = "warn"

	// ValidationModeDisabled skips all validation.
	ValidationModeDisabled ValidationMode = "disabled"
)

// ValidationResult contains the outcome of a validation operation.
type ValidationResult struct {
	// Valid is true if all validations passed.
	Valid bool

	// Errors contains validation errors that should block the operation.
	Errors []ValidationError

	// Warnings contains validation warnings that don't block the operation.
	Warnings []ValidationWarning
}

// IsValid returns true if the validation passed without errors.
func (r *ValidationResult) IsValid() bool {
	return len(r.Errors) == 0
}

// AddError adds a validation error to the result.
func (r *ValidationResult) AddError(err ValidationError) {
	r.Errors = append(r.Errors, err)
	r.Valid = false
}

// AddWarning adds a validation warning to the result.
func (r *ValidationResult) AddWarning(warn ValidationWarning) {
	r.Warnings = append(r.Warnings, warn)
}

// Merge combines another ValidationResult into this one.
func (r *ValidationResult) Merge(other *ValidationResult) {
	if other == nil {
		return
	}
	r.Errors = append(r.Errors, other.Errors...)
	r.Warnings = append(r.Warnings, other.Warnings...)
	if len(r.Errors) > 0 {
		r.Valid = false
	}
}

// Error returns a combined error message if there are any errors.
func (r *ValidationResult) Error() error {
	if len(r.Errors) == 0 {
		return nil
	}
	var msgs []string
	for _, e := range r.Errors {
		msgs = append(msgs, e.Error())
	}
	return fmt.Errorf("validation failed: %s", strings.Join(msgs, "; "))
}

// ValidationError represents a single validation error.
type ValidationError struct {
	// Field is the name of the field that failed validation.
	Field string

	// Value is the invalid value.
	Value string

	// Message describes the validation failure.
	Message string

	// Suggestions contains possible valid values (for "did you mean?" hints).
	Suggestions []string
}

// Error implements the error interface.
func (e ValidationError) Error() string {
	msg := fmt.Sprintf("%s: %s", e.Field, e.Message)
	if len(e.Suggestions) > 0 {
		msg += fmt.Sprintf(" (did you mean: %s?)", strings.Join(e.Suggestions, ", "))
	}
	return msg
}

// ValidationWarning represents a non-blocking validation issue.
type ValidationWarning struct {
	// Field is the name of the field with the warning.
	Field string

	// Value is the value that triggered the warning.
	Value string

	// Message describes the warning.
	Message string

	// Suggestions contains recommended values.
	Suggestions []string
}

// String returns a formatted warning message.
func (w ValidationWarning) String() string {
	msg := fmt.Sprintf("%s: %s", w.Field, w.Message)
	if len(w.Suggestions) > 0 {
		msg += fmt.Sprintf(" (suggestions: %s)", strings.Join(w.Suggestions, ", "))
	}
	return msg
}

// TaxonomyValidator validates findings and entities against the taxonomy schema.
type TaxonomyValidator interface {
	// ValidateFinding validates a security finding's taxonomy fields.
	// Checks category, severity, confidence, and other taxonomy-defined fields.
	ValidateFinding(ctx context.Context, finding FindingFields) *ValidationResult

	// ValidateCategory checks if a category is valid in the taxonomy.
	ValidateCategory(ctx context.Context, category string) *ValidationResult

	// ValidateEntityType checks if an entity type is valid in the taxonomy.
	ValidateEntityType(ctx context.Context, entityType string) *ValidationResult

	// ValidateRelationshipType checks if a relationship type is valid in the taxonomy.
	ValidateRelationshipType(ctx context.Context, relType string) *ValidationResult

	// ValidateProperties validates properties against the schema for an entity type.
	ValidateProperties(ctx context.Context, entityType string, properties map[string]any) *ValidationResult

	// Mode returns the current validation mode.
	Mode() ValidationMode

	// SetMode changes the validation mode.
	SetMode(mode ValidationMode)
}

// FindingFields contains the taxonomy-related fields of a finding.
type FindingFields struct {
	Category   string
	Severity   string
	Confidence float64
	CVSSScore  float64
	CWEIDs     []string
	CVEIDs     []string
	Tags       []string
}

// TaxonomySchema provides access to taxonomy definitions for validation.
type TaxonomySchema interface {
	// Categories returns all valid finding categories.
	Categories() []string

	// HasCategory checks if a category exists.
	HasCategory(category string) bool

	// Severities returns all valid severity levels.
	Severities() []string

	// HasSeverity checks if a severity is valid.
	HasSeverity(severity string) bool

	// NodeTypes returns all valid entity/node types.
	NodeTypes() []string

	// HasNodeType checks if a node type exists.
	HasNodeType(nodeType string) bool

	// RelationshipTypes returns all valid relationship types.
	RelationshipTypes() []string

	// HasRelationshipType checks if a relationship type exists.
	HasRelationshipType(relType string) bool

	// NodeProperties returns property definitions for a node type.
	NodeProperties(nodeType string) []PropertyDef

	// RequiredProperties returns required properties for a node type.
	RequiredProperties(nodeType string) []string
}

// PropertyDef defines a property's schema.
type PropertyDef struct {
	Name        string
	Type        string // "string", "int", "float", "bool", "[]string"
	Required    bool
	Description string
	Format      string   // Optional format hint (e.g., "ip", "url", "cwe")
	Enum        []string // Valid values if enum type
}

// DefaultTaxonomyValidator is the standard implementation of TaxonomyValidator.
type DefaultTaxonomyValidator struct {
	schema TaxonomySchema
	mode   ValidationMode
}

// NewTaxonomyValidator creates a new validator with the given schema and mode.
func NewTaxonomyValidator(schema TaxonomySchema, mode ValidationMode) *DefaultTaxonomyValidator {
	if mode == "" {
		mode = ValidationModeWarn
	}
	return &DefaultTaxonomyValidator{
		schema: schema,
		mode:   mode,
	}
}

// Mode returns the current validation mode.
func (v *DefaultTaxonomyValidator) Mode() ValidationMode {
	return v.mode
}

// SetMode changes the validation mode.
func (v *DefaultTaxonomyValidator) SetMode(mode ValidationMode) {
	v.mode = mode
}

// ValidateFinding validates a security finding's taxonomy fields.
func (v *DefaultTaxonomyValidator) ValidateFinding(ctx context.Context, finding FindingFields) *ValidationResult {
	result := &ValidationResult{Valid: true}

	if v.mode == ValidationModeDisabled {
		return result
	}

	// Validate category
	if finding.Category != "" {
		catResult := v.ValidateCategory(ctx, finding.Category)
		result.Merge(catResult)
	}

	// Validate severity
	if finding.Severity != "" {
		if !v.schema.HasSeverity(finding.Severity) {
			suggestions := v.findSimilar(finding.Severity, v.schema.Severities(), 3)
			result.AddError(ValidationError{
				Field:       "severity",
				Value:       finding.Severity,
				Message:     fmt.Sprintf("invalid severity '%s'", finding.Severity),
				Suggestions: suggestions,
			})
		}
	}

	// Validate confidence range
	if finding.Confidence < 0.0 || finding.Confidence > 1.0 {
		result.AddError(ValidationError{
			Field:   "confidence",
			Value:   fmt.Sprintf("%f", finding.Confidence),
			Message: "confidence must be between 0.0 and 1.0",
		})
	}

	// Validate CVSS score range
	if finding.CVSSScore < 0.0 || finding.CVSSScore > 10.0 {
		result.AddError(ValidationError{
			Field:   "cvss_score",
			Value:   fmt.Sprintf("%f", finding.CVSSScore),
			Message: "CVSS score must be between 0.0 and 10.0",
		})
	}

	// Validate CWE ID format
	for _, cwe := range finding.CWEIDs {
		if !isValidCWEFormat(cwe) {
			result.AddWarning(ValidationWarning{
				Field:   "cwe_ids",
				Value:   cwe,
				Message: fmt.Sprintf("CWE ID '%s' does not match expected format (CWE-NNN)", cwe),
			})
		}
	}

	// Validate CVE ID format
	for _, cve := range finding.CVEIDs {
		if !isValidCVEFormat(cve) {
			result.AddWarning(ValidationWarning{
				Field:   "cve_ids",
				Value:   cve,
				Message: fmt.Sprintf("CVE ID '%s' does not match expected format (CVE-YYYY-NNNNN)", cve),
			})
		}
	}

	return result
}

// ValidateCategory checks if a category is valid in the taxonomy.
func (v *DefaultTaxonomyValidator) ValidateCategory(ctx context.Context, category string) *ValidationResult {
	result := &ValidationResult{Valid: true}

	if v.mode == ValidationModeDisabled {
		return result
	}

	if !v.schema.HasCategory(category) {
		suggestions := v.findSimilar(category, v.schema.Categories(), 3)
		result.AddError(ValidationError{
			Field:       "category",
			Value:       category,
			Message:     fmt.Sprintf("invalid category '%s'", category),
			Suggestions: suggestions,
		})
	}

	return result
}

// ValidateEntityType checks if an entity type is valid in the taxonomy.
func (v *DefaultTaxonomyValidator) ValidateEntityType(ctx context.Context, entityType string) *ValidationResult {
	result := &ValidationResult{Valid: true}

	if v.mode == ValidationModeDisabled {
		return result
	}

	if !v.schema.HasNodeType(entityType) {
		suggestions := v.findSimilar(entityType, v.schema.NodeTypes(), 3)
		result.AddError(ValidationError{
			Field:       "entity_type",
			Value:       entityType,
			Message:     fmt.Sprintf("invalid entity type '%s'", entityType),
			Suggestions: suggestions,
		})
	}

	return result
}

// ValidateRelationshipType checks if a relationship type is valid in the taxonomy.
func (v *DefaultTaxonomyValidator) ValidateRelationshipType(ctx context.Context, relType string) *ValidationResult {
	result := &ValidationResult{Valid: true}

	if v.mode == ValidationModeDisabled {
		return result
	}

	if !v.schema.HasRelationshipType(relType) {
		suggestions := v.findSimilar(relType, v.schema.RelationshipTypes(), 3)
		result.AddError(ValidationError{
			Field:       "relationship_type",
			Value:       relType,
			Message:     fmt.Sprintf("invalid relationship type '%s'", relType),
			Suggestions: suggestions,
		})
	}

	return result
}

// ValidateProperties validates properties against the schema for an entity type.
func (v *DefaultTaxonomyValidator) ValidateProperties(ctx context.Context, entityType string, properties map[string]any) *ValidationResult {
	result := &ValidationResult{Valid: true}

	if v.mode == ValidationModeDisabled {
		return result
	}

	// First validate the entity type exists
	if !v.schema.HasNodeType(entityType) {
		result.AddError(ValidationError{
			Field:   "entity_type",
			Value:   entityType,
			Message: fmt.Sprintf("unknown entity type '%s'", entityType),
		})
		return result
	}

	// Check required properties
	requiredProps := v.schema.RequiredProperties(entityType)
	for _, required := range requiredProps {
		if _, exists := properties[required]; !exists {
			result.AddError(ValidationError{
				Field:   required,
				Message: fmt.Sprintf("required property '%s' is missing for entity type '%s'", required, entityType),
			})
		}
	}

	// Validate property types
	propDefs := v.schema.NodeProperties(entityType)
	propDefMap := make(map[string]PropertyDef)
	for _, pd := range propDefs {
		propDefMap[pd.Name] = pd
	}

	for propName, propValue := range properties {
		if def, exists := propDefMap[propName]; exists {
			if err := validatePropertyType(propName, propValue, def); err != nil {
				result.AddError(*err)
			}
		}
		// Unknown properties are allowed (extensions)
	}

	return result
}

// findSimilar finds the most similar strings using Levenshtein distance.
func (v *DefaultTaxonomyValidator) findSimilar(input string, candidates []string, maxResults int) []string {
	if len(candidates) == 0 {
		return nil
	}

	type scored struct {
		value    string
		distance int
	}

	inputLower := strings.ToLower(input)
	var scores []scored

	for _, candidate := range candidates {
		candidateLower := strings.ToLower(candidate)
		dist := levenshtein.ComputeDistance(inputLower, candidateLower)
		// Only include if somewhat similar (distance < half the input length + 3)
		threshold := len(input)/2 + 3
		if dist <= threshold {
			scores = append(scores, scored{value: candidate, distance: dist})
		}
	}

	// Sort by distance
	sort.Slice(scores, func(i, j int) bool {
		return scores[i].distance < scores[j].distance
	})

	// Return top results
	var results []string
	for i := 0; i < len(scores) && i < maxResults; i++ {
		results = append(results, scores[i].value)
	}

	return results
}

// validatePropertyType checks if a property value matches its expected type.
func validatePropertyType(name string, value any, def PropertyDef) *ValidationError {
	if value == nil {
		if def.Required {
			return &ValidationError{
				Field:   name,
				Message: fmt.Sprintf("required property '%s' is nil", name),
			}
		}
		return nil
	}

	switch def.Type {
	case "string":
		if _, ok := value.(string); !ok {
			return &ValidationError{
				Field:   name,
				Value:   fmt.Sprintf("%v", value),
				Message: fmt.Sprintf("property '%s' must be a string, got %T", name, value),
			}
		}
	case "int", "int32", "int64":
		switch value.(type) {
		case int, int32, int64, float64: // float64 for JSON unmarshaling
			// OK
		default:
			return &ValidationError{
				Field:   name,
				Value:   fmt.Sprintf("%v", value),
				Message: fmt.Sprintf("property '%s' must be an integer, got %T", name, value),
			}
		}
	case "float", "float32", "float64":
		switch value.(type) {
		case float32, float64, int, int32, int64:
			// OK
		default:
			return &ValidationError{
				Field:   name,
				Value:   fmt.Sprintf("%v", value),
				Message: fmt.Sprintf("property '%s' must be a float, got %T", name, value),
			}
		}
	case "bool":
		if _, ok := value.(bool); !ok {
			return &ValidationError{
				Field:   name,
				Value:   fmt.Sprintf("%v", value),
				Message: fmt.Sprintf("property '%s' must be a boolean, got %T", name, value),
			}
		}
	case "[]string":
		switch v := value.(type) {
		case []string:
			// OK
		case []any:
			for i, item := range v {
				if _, ok := item.(string); !ok {
					return &ValidationError{
						Field:   name,
						Value:   fmt.Sprintf("%v", value),
						Message: fmt.Sprintf("property '%s[%d]' must be a string, got %T", name, i, item),
					}
				}
			}
		default:
			return &ValidationError{
				Field:   name,
				Value:   fmt.Sprintf("%v", value),
				Message: fmt.Sprintf("property '%s' must be a string array, got %T", name, value),
			}
		}
	}

	// Validate enum values
	if len(def.Enum) > 0 {
		strVal, ok := value.(string)
		if ok {
			found := false
			for _, allowed := range def.Enum {
				if strVal == allowed {
					found = true
					break
				}
			}
			if !found {
				return &ValidationError{
					Field:       name,
					Value:       strVal,
					Message:     fmt.Sprintf("property '%s' must be one of: %s", name, strings.Join(def.Enum, ", ")),
					Suggestions: def.Enum,
				}
			}
		}
	}

	return nil
}

// isValidCWEFormat checks if a string matches CWE-NNN format.
func isValidCWEFormat(cwe string) bool {
	if !strings.HasPrefix(strings.ToUpper(cwe), "CWE-") {
		return false
	}
	numPart := cwe[4:]
	if len(numPart) == 0 {
		return false
	}
	for _, c := range numPart {
		if c < '0' || c > '9' {
			return false
		}
	}
	return true
}

// isValidCVEFormat checks if a string matches CVE-YYYY-NNNNN format.
func isValidCVEFormat(cve string) bool {
	if !strings.HasPrefix(strings.ToUpper(cve), "CVE-") {
		return false
	}
	parts := strings.Split(cve[4:], "-")
	if len(parts) != 2 {
		return false
	}
	// Year part (4 digits)
	if len(parts[0]) != 4 {
		return false
	}
	for _, c := range parts[0] {
		if c < '0' || c > '9' {
			return false
		}
	}
	// ID part (at least 4 digits)
	if len(parts[1]) < 4 {
		return false
	}
	for _, c := range parts[1] {
		if c < '0' || c > '9' {
			return false
		}
	}
	return true
}

// NoOpTaxonomyValidator is a validator that allows all values.
// Use this for testing or when validation is disabled.
type NoOpTaxonomyValidator struct{}

// NewNoOpTaxonomyValidator creates a no-op validator.
func NewNoOpTaxonomyValidator() *NoOpTaxonomyValidator {
	return &NoOpTaxonomyValidator{}
}

// ValidateFinding always returns valid.
func (v *NoOpTaxonomyValidator) ValidateFinding(ctx context.Context, finding FindingFields) *ValidationResult {
	return &ValidationResult{Valid: true}
}

// ValidateCategory always returns valid.
func (v *NoOpTaxonomyValidator) ValidateCategory(ctx context.Context, category string) *ValidationResult {
	return &ValidationResult{Valid: true}
}

// ValidateEntityType always returns valid.
func (v *NoOpTaxonomyValidator) ValidateEntityType(ctx context.Context, entityType string) *ValidationResult {
	return &ValidationResult{Valid: true}
}

// ValidateRelationshipType always returns valid.
func (v *NoOpTaxonomyValidator) ValidateRelationshipType(ctx context.Context, relType string) *ValidationResult {
	return &ValidationResult{Valid: true}
}

// ValidateProperties always returns valid.
func (v *NoOpTaxonomyValidator) ValidateProperties(ctx context.Context, entityType string, properties map[string]any) *ValidationResult {
	return &ValidationResult{Valid: true}
}

// Mode returns disabled mode.
func (v *NoOpTaxonomyValidator) Mode() ValidationMode {
	return ValidationModeDisabled
}

// SetMode is a no-op.
func (v *NoOpTaxonomyValidator) SetMode(mode ValidationMode) {}

// Compile-time interface checks
var (
	_ TaxonomyValidator = (*DefaultTaxonomyValidator)(nil)
	_ TaxonomyValidator = (*NoOpTaxonomyValidator)(nil)
)
