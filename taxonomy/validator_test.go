// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Zero Root AI

package taxonomy

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDefaultTaxonomyValidator_ValidateFinding(t *testing.T) {
	schema := NewDefaultTaxonomySchema()
	validator := NewTaxonomyValidator(schema, ValidationModeStrict)

	tests := []struct {
		name        string
		finding     FindingFields
		expectValid bool
		expectErrs  int
		expectWarns int
	}{
		{
			name: "valid finding with all fields",
			finding: FindingFields{
				Category:   "injection",
				Severity:   "high",
				Confidence: 0.95,
				CVSSScore:  8.5,
				CWEIDs:     []string{"CWE-89"},
				CVEIDs:     []string{"CVE-2024-12345"},
			},
			expectValid: true,
			expectErrs:  0,
			expectWarns: 0,
		},
		{
			name: "valid finding with minimal fields",
			finding: FindingFields{
				Severity:   "medium",
				Confidence: 0.5,
			},
			expectValid: true,
			expectErrs:  0,
		},
		{
			name: "invalid category",
			finding: FindingFields{
				Category:   "invalid_category_xyz",
				Severity:   "high",
				Confidence: 0.8,
			},
			expectValid: false,
			expectErrs:  1,
		},
		{
			name: "invalid severity",
			finding: FindingFields{
				Category:   "injection",
				Severity:   "super_critical",
				Confidence: 0.8,
			},
			expectValid: false,
			expectErrs:  1,
		},
		{
			name: "confidence out of range (too high)",
			finding: FindingFields{
				Severity:   "high",
				Confidence: 1.5,
			},
			expectValid: false,
			expectErrs:  1,
		},
		{
			name: "confidence out of range (negative)",
			finding: FindingFields{
				Severity:   "high",
				Confidence: -0.1,
			},
			expectValid: false,
			expectErrs:  1,
		},
		{
			name: "CVSS score out of range",
			finding: FindingFields{
				Severity:  "high",
				CVSSScore: 11.0,
			},
			expectValid: false,
			expectErrs:  1,
		},
		{
			name: "invalid CWE format",
			finding: FindingFields{
				Severity: "high",
				CWEIDs:   []string{"CWE89", "invalid"},
			},
			expectValid: true, // CWE format issues are warnings
			expectWarns: 2,
		},
		{
			name: "invalid CVE format",
			finding: FindingFields{
				Severity: "high",
				CVEIDs:   []string{"CVE-2024", "invalid-cve"},
			},
			expectValid: true, // CVE format issues are warnings
			expectWarns: 2,
		},
		{
			name: "multiple errors",
			finding: FindingFields{
				Category:   "bad_category",
				Severity:   "bad_severity",
				Confidence: 2.0,
				CVSSScore:  -1.0,
			},
			expectValid: false,
			expectErrs:  4,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := validator.ValidateFinding(context.Background(), tt.finding)

			assert.Equal(t, tt.expectValid, result.IsValid(), "validity mismatch")
			assert.Len(t, result.Errors, tt.expectErrs, "error count mismatch")
			if tt.expectWarns > 0 {
				assert.Len(t, result.Warnings, tt.expectWarns, "warning count mismatch")
			}
		})
	}
}

func TestDefaultTaxonomyValidator_ValidateCategory(t *testing.T) {
	schema := NewDefaultTaxonomySchema()
	validator := NewTaxonomyValidator(schema, ValidationModeStrict)

	tests := []struct {
		name        string
		category    string
		expectValid bool
		expectSuggs int
	}{
		{
			name:        "valid category - injection",
			category:    "injection",
			expectValid: true,
		},
		{
			name:        "valid category - xss",
			category:    "xss",
			expectValid: true,
		},
		{
			name:        "valid category - case insensitive",
			category:    "INJECTION",
			expectValid: true,
		},
		{
			name:        "invalid category with suggestions",
			category:    "injectoin", // typo
			expectValid: false,
			expectSuggs: 1, // should suggest "injection"
		},
		{
			name:        "completely invalid category",
			category:    "foobar123",
			expectValid: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := validator.ValidateCategory(context.Background(), tt.category)

			assert.Equal(t, tt.expectValid, result.IsValid())
			if !tt.expectValid && tt.expectSuggs > 0 {
				require.Len(t, result.Errors, 1)
				assert.GreaterOrEqual(t, len(result.Errors[0].Suggestions), tt.expectSuggs)
			}
		})
	}
}

func TestDefaultTaxonomyValidator_ValidateEntityType(t *testing.T) {
	schema := NewDefaultTaxonomySchema()
	validator := NewTaxonomyValidator(schema, ValidationModeStrict)

	tests := []struct {
		name        string
		entityType  string
		expectValid bool
	}{
		{
			name:        "valid entity type - host",
			entityType:  "host",
			expectValid: true,
		},
		{
			name:        "valid entity type - port",
			entityType:  "port",
			expectValid: true,
		},
		{
			name:        "valid entity type - service",
			entityType:  "service",
			expectValid: true,
		},
		{
			name:        "valid entity type - finding",
			entityType:  "finding",
			expectValid: true,
		},
		{
			name:        "valid entity type - endpoint",
			entityType:  "endpoint",
			expectValid: true,
		},
		{
			name:        "invalid entity type",
			entityType:  "invalid_entity",
			expectValid: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := validator.ValidateEntityType(context.Background(), tt.entityType)
			assert.Equal(t, tt.expectValid, result.IsValid())
		})
	}
}

func TestDefaultTaxonomyValidator_ValidateRelationshipType(t *testing.T) {
	schema := NewDefaultTaxonomySchema()
	validator := NewTaxonomyValidator(schema, ValidationModeStrict)

	tests := []struct {
		name        string
		relType     string
		expectValid bool
	}{
		{
			name:        "valid relationship - HAS_PORT",
			relType:     "HAS_PORT",
			expectValid: true,
		},
		{
			name:        "valid relationship - RUNS_SERVICE",
			relType:     "RUNS_SERVICE",
			expectValid: true,
		},
		{
			name:        "valid relationship - lowercase",
			relType:     "has_port",
			expectValid: true,
		},
		{
			name:        "invalid relationship",
			relType:     "INVALID_REL",
			expectValid: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := validator.ValidateRelationshipType(context.Background(), tt.relType)
			assert.Equal(t, tt.expectValid, result.IsValid())
		})
	}
}

func TestDefaultTaxonomyValidator_ValidationModes(t *testing.T) {
	schema := NewDefaultTaxonomySchema()

	invalidFinding := FindingFields{
		Category: "invalid_category",
		Severity: "invalid_severity",
	}

	t.Run("strict mode returns errors", func(t *testing.T) {
		validator := NewTaxonomyValidator(schema, ValidationModeStrict)
		result := validator.ValidateFinding(context.Background(), invalidFinding)

		assert.False(t, result.IsValid())
		assert.NotEmpty(t, result.Errors)
	})

	t.Run("warn mode returns errors but logs warnings", func(t *testing.T) {
		validator := NewTaxonomyValidator(schema, ValidationModeWarn)
		result := validator.ValidateFinding(context.Background(), invalidFinding)

		// Warn mode still populates errors for inspection
		assert.False(t, result.IsValid())
		assert.NotEmpty(t, result.Errors)
	})

	t.Run("disabled mode skips validation", func(t *testing.T) {
		validator := NewTaxonomyValidator(schema, ValidationModeDisabled)
		result := validator.ValidateFinding(context.Background(), invalidFinding)

		assert.True(t, result.IsValid())
		assert.Empty(t, result.Errors)
	})

	t.Run("mode can be changed", func(t *testing.T) {
		validator := NewTaxonomyValidator(schema, ValidationModeStrict)
		assert.Equal(t, ValidationModeStrict, validator.Mode())

		validator.SetMode(ValidationModeDisabled)
		assert.Equal(t, ValidationModeDisabled, validator.Mode())

		result := validator.ValidateFinding(context.Background(), invalidFinding)
		assert.True(t, result.IsValid())
	})
}

func TestNoOpTaxonomyValidator(t *testing.T) {
	validator := NewNoOpTaxonomyValidator()

	t.Run("always returns valid for findings", func(t *testing.T) {
		result := validator.ValidateFinding(context.Background(), FindingFields{
			Category: "completely_invalid",
			Severity: "not_a_severity",
		})
		assert.True(t, result.IsValid())
	})

	t.Run("always returns valid for categories", func(t *testing.T) {
		result := validator.ValidateCategory(context.Background(), "invalid")
		assert.True(t, result.IsValid())
	})

	t.Run("always returns valid for entity types", func(t *testing.T) {
		result := validator.ValidateEntityType(context.Background(), "invalid")
		assert.True(t, result.IsValid())
	})

	t.Run("mode is always disabled", func(t *testing.T) {
		assert.Equal(t, ValidationModeDisabled, validator.Mode())
	})
}

func TestValidationResult_Merge(t *testing.T) {
	r1 := &ValidationResult{Valid: true}
	r1.AddError(ValidationError{Field: "field1", Message: "error1"})

	r2 := &ValidationResult{Valid: true}
	r2.AddError(ValidationError{Field: "field2", Message: "error2"})
	r2.AddWarning(ValidationWarning{Field: "field3", Message: "warn1"})

	r1.Merge(r2)

	assert.False(t, r1.IsValid())
	assert.Len(t, r1.Errors, 2)
	assert.Len(t, r1.Warnings, 1)
}

func TestValidationError_Suggestions(t *testing.T) {
	err := ValidationError{
		Field:       "category",
		Value:       "injectoin",
		Message:     "invalid category",
		Suggestions: []string{"injection", "input_validation"},
	}

	errStr := err.Error()
	assert.Contains(t, errStr, "category")
	assert.Contains(t, errStr, "invalid category")
	assert.Contains(t, errStr, "did you mean")
	assert.Contains(t, errStr, "injection")
}

func TestCWEFormat(t *testing.T) {
	tests := []struct {
		cwe   string
		valid bool
	}{
		{"CWE-89", true},
		{"CWE-1234", true},
		{"CWE-1", true},
		{"cwe-89", true},
		{"CWE89", false},
		{"CWE-", false},
		{"89", false},
		{"", false},
		{"CWE-abc", false},
	}

	for _, tt := range tests {
		t.Run(tt.cwe, func(t *testing.T) {
			assert.Equal(t, tt.valid, isValidCWEFormat(tt.cwe))
		})
	}
}

func TestCVEFormat(t *testing.T) {
	tests := []struct {
		cve   string
		valid bool
	}{
		{"CVE-2024-12345", true},
		{"CVE-2024-1234", true},
		{"CVE-2024-123456", true},
		{"cve-2024-12345", true},
		{"CVE-2024-123", false}, // too short
		{"CVE-24-12345", false}, // year too short
		{"CVE-2024", false},     // missing ID
		{"CVE2024-12345", false},
		{"", false},
	}

	for _, tt := range tests {
		t.Run(tt.cve, func(t *testing.T) {
			assert.Equal(t, tt.valid, isValidCVEFormat(tt.cve))
		})
	}
}

func TestCoreTaxonomySchema(t *testing.T) {
	schema := NewDefaultTaxonomySchema()

	t.Run("has standard categories", func(t *testing.T) {
		categories := schema.Categories()
		assert.NotEmpty(t, categories)
		assert.True(t, schema.HasCategory("injection"))
		assert.True(t, schema.HasCategory("xss"))
		assert.True(t, schema.HasCategory("INJECTION")) // case insensitive
		assert.False(t, schema.HasCategory("not_a_category"))
	})

	t.Run("has standard severities", func(t *testing.T) {
		severities := schema.Severities()
		assert.NotEmpty(t, severities)
		assert.True(t, schema.HasSeverity("critical"))
		assert.True(t, schema.HasSeverity("high"))
		assert.True(t, schema.HasSeverity("MEDIUM")) // case insensitive
		assert.False(t, schema.HasSeverity("super_critical"))
	})

	t.Run("has core node types", func(t *testing.T) {
		nodeTypes := schema.NodeTypes()
		assert.NotEmpty(t, nodeTypes)
		assert.True(t, schema.HasNodeType("host"))
		assert.True(t, schema.HasNodeType("port"))
		assert.True(t, schema.HasNodeType("service"))
		assert.True(t, schema.HasNodeType("finding"))
	})

	t.Run("has relationship types", func(t *testing.T) {
		relTypes := schema.RelationshipTypes()
		assert.NotEmpty(t, relTypes)
		assert.True(t, schema.HasRelationshipType("HAS_PORT"))
		assert.True(t, schema.HasRelationshipType("RUNS_SERVICE"))
	})

	t.Run("has property definitions", func(t *testing.T) {
		hostProps := schema.NodeProperties("host")
		assert.NotEmpty(t, hostProps)

		portProps := schema.NodeProperties("port")
		assert.NotEmpty(t, portProps)

		findingProps := schema.NodeProperties("finding")
		assert.NotEmpty(t, findingProps)
	})

	t.Run("has required properties", func(t *testing.T) {
		portRequired := schema.RequiredProperties("port")
		assert.Contains(t, portRequired, "number")

		findingRequired := schema.RequiredProperties("finding")
		assert.Contains(t, findingRequired, "title")
		assert.Contains(t, findingRequired, "severity")
	})
}
