// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Zero Root AI

package taxonomy

import (
	"context"
	"io/fs"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestValidationResult_Error(t *testing.T) {
	t.Run("nil when no errors", func(t *testing.T) {
		r := &ValidationResult{Valid: true}
		assert.NoError(t, r.Error())
	})

	t.Run("combines all error messages", func(t *testing.T) {
		r := &ValidationResult{Valid: true}
		r.AddError(ValidationError{Field: "a", Message: "first problem"})
		r.AddError(ValidationError{Field: "b", Message: "second problem"})

		err := r.Error()
		require.Error(t, err)
		assert.Contains(t, err.Error(), "first problem")
		assert.Contains(t, err.Error(), "second problem")
	})
}

func TestValidationWarning_String(t *testing.T) {
	t.Run("without suggestions", func(t *testing.T) {
		w := ValidationWarning{Field: "severity", Message: "unusual value"}
		assert.Equal(t, "severity: unusual value", w.String())
	})

	t.Run("with suggestions", func(t *testing.T) {
		w := ValidationWarning{
			Field:       "severity",
			Message:     "unknown value",
			Suggestions: []string{"high", "medium"},
		}
		got := w.String()
		assert.Contains(t, got, "severity: unknown value")
		assert.Contains(t, got, "suggestions: high, medium")
	})
}

func TestDefaultTaxonomyValidator_ValidateProperties(t *testing.T) {
	schema := NewDefaultTaxonomySchema()
	v := NewTaxonomyValidator(schema, ValidationModeStrict)
	ctx := context.Background()

	t.Run("valid properties pass", func(t *testing.T) {
		res := v.ValidateProperties(ctx, "finding", map[string]any{
			"title":    "SQL injection",
			"severity": "high",
		})
		assert.True(t, res.IsValid())
		assert.Empty(t, res.Errors)
	})

	t.Run("unknown entity type errors", func(t *testing.T) {
		res := v.ValidateProperties(ctx, "not_a_real_type", map[string]any{})
		assert.False(t, res.IsValid())
		require.NotEmpty(t, res.Errors)
		assert.Equal(t, "entity_type", res.Errors[0].Field)
	})

	t.Run("missing required property errors", func(t *testing.T) {
		res := v.ValidateProperties(ctx, "finding", map[string]any{
			"title": "missing severity",
		})
		assert.False(t, res.IsValid())
		require.NotEmpty(t, res.Errors)
	})

	t.Run("wrong property type errors", func(t *testing.T) {
		res := v.ValidateProperties(ctx, "finding", map[string]any{
			"title":    123, // should be a string
			"severity": "high",
		})
		assert.False(t, res.IsValid())
		require.NotEmpty(t, res.Errors)
	})

	t.Run("disabled mode skips validation", func(t *testing.T) {
		v.SetMode(ValidationModeDisabled)
		assert.Equal(t, ValidationModeDisabled, v.Mode())
		res := v.ValidateProperties(ctx, "not_a_real_type", map[string]any{})
		assert.True(t, res.IsValid())
		v.SetMode(ValidationModeStrict)
	})
}

func TestNoOpTaxonomyValidator_RemainingMethods(t *testing.T) {
	v := NewNoOpTaxonomyValidator()
	ctx := context.Background()

	assert.True(t, v.ValidateRelationshipType(ctx, "anything").IsValid())
	assert.True(t, v.ValidateProperties(ctx, "anything", map[string]any{"x": 1}).IsValid())

	// SetMode is a documented no-op: mode stays disabled.
	v.SetMode(ValidationModeStrict)
	assert.Equal(t, ValidationModeDisabled, v.Mode())
}

func TestEmbeddedOntology(t *testing.T) {
	fsys := EmbeddedOntology()
	require.NotNil(t, fsys)

	// The embedded FS is rooted such that the "ontology" directory is readable.
	entries, err := fs.ReadDir(fsys, "ontology")
	require.NoError(t, err)
	assert.NotEmpty(t, entries)
}
