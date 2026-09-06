// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Zero Root AI

package llm

import (
	"reflect"
	"testing"
)

func TestSlotDefinition_Validate(t *testing.T) {
	tests := []struct {
		name    string
		slot    SlotDefinition
		wantErr bool
	}{
		{
			name: "valid slot",
			slot: SlotDefinition{
				Name:             "primary",
				Description:      "Main LLM",
				MinContextWindow: 8000,
			},
			wantErr: false,
		},
		{
			name: "empty name",
			slot: SlotDefinition{
				Description:      "Test",
				MinContextWindow: 8000,
			},
			wantErr: true,
		},
		{
			name: "negative context window",
			slot: SlotDefinition{
				Name:             "primary",
				Description:      "Test",
				MinContextWindow: -100,
			},
			wantErr: true,
		},
		{
			name: "zero context window is valid",
			slot: SlotDefinition{
				Name:             "primary",
				Description:      "Test",
				MinContextWindow: 0,
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.slot.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestSlotDefinition_ToRequirements(t *testing.T) {
	slot := SlotDefinition{
		Name:             "primary",
		Description:      "Main LLM",
		Required:         true,
		MinContextWindow: 32000,
		RequiredFeatures: []string{"function_calling", "streaming"},
		PreferredModels:  []string{"gpt-4-turbo", "claude-3-opus"},
	}

	req := slot.ToRequirements()

	if req.MinContextWindow != 32000 {
		t.Errorf("MinContextWindow = %d, want 32000", req.MinContextWindow)
	}
	if !reflect.DeepEqual(req.RequiredFeatures, slot.RequiredFeatures) {
		t.Errorf("RequiredFeatures not copied correctly")
	}
	if !reflect.DeepEqual(req.PreferredModels, slot.PreferredModels) {
		t.Errorf("PreferredModels not copied correctly")
	}

	// Verify that slices are copied, not shared
	req.RequiredFeatures[0] = "modified"
	if slot.RequiredFeatures[0] == "modified" {
		t.Error("RequiredFeatures should be a copy, not shared")
	}
}

func TestSlotDefinition_HasFeature(t *testing.T) {
	slot := SlotDefinition{
		RequiredFeatures: []string{"function_calling", "streaming", "vision"},
	}

	tests := []struct {
		feature string
		want    bool
	}{
		{"function_calling", true},
		{"streaming", true},
		{"vision", true},
		{"json_mode", false},
		{"", false},
	}

	for _, tt := range tests {
		t.Run(tt.feature, func(t *testing.T) {
			if got := slot.HasFeature(tt.feature); got != tt.want {
				t.Errorf("HasFeature(%q) = %v, want %v", tt.feature, got, tt.want)
			}
		})
	}
}

func TestSlotDefinition_PrefersModel(t *testing.T) {
	slot := SlotDefinition{
		PreferredModels: []string{"gpt-4-turbo", "claude-3-opus"},
	}

	tests := []struct {
		model string
		want  bool
	}{
		{"gpt-4-turbo", true},
		{"claude-3-opus", true},
		{"gpt-3.5-turbo", false},
		{"", false},
	}

	for _, tt := range tests {
		t.Run(tt.model, func(t *testing.T) {
			if got := slot.PrefersModel(tt.model); got != tt.want {
				t.Errorf("PrefersModel(%q) = %v, want %v", tt.model, got, tt.want)
			}
		})
	}
}

func TestSlotRequirements_Satisfies(t *testing.T) {
	tests := []struct {
		name          string
		req           SlotRequirements
		features      []string
		contextWindow int
		want          bool
	}{
		{
			name: "satisfies all requirements",
			req: SlotRequirements{
				MinContextWindow: 8000,
				RequiredFeatures: []string{"function_calling", "streaming"},
			},
			features:      []string{"function_calling", "streaming", "vision"},
			contextWindow: 32000,
			want:          true,
		},
		{
			name: "context window too small",
			req: SlotRequirements{
				MinContextWindow: 32000,
				RequiredFeatures: []string{"function_calling"},
			},
			features:      []string{"function_calling"},
			contextWindow: 8000,
			want:          false,
		},
		{
			name: "missing required feature",
			req: SlotRequirements{
				MinContextWindow: 8000,
				RequiredFeatures: []string{"function_calling", "vision"},
			},
			features:      []string{"function_calling"},
			contextWindow: 32000,
			want:          false,
		},
		{
			name: "exact match",
			req: SlotRequirements{
				MinContextWindow: 8000,
				RequiredFeatures: []string{"function_calling"},
			},
			features:      []string{"function_calling"},
			contextWindow: 8000,
			want:          true,
		},
		{
			name: "no requirements",
			req: SlotRequirements{
				MinContextWindow: 0,
				RequiredFeatures: []string{},
			},
			features:      []string{},
			contextWindow: 1000,
			want:          true,
		},
		{
			name: "no features available but required",
			req: SlotRequirements{
				MinContextWindow: 0,
				RequiredFeatures: []string{"function_calling"},
			},
			features:      []string{},
			contextWindow: 32000,
			want:          false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.req.Satisfies(tt.features, tt.contextWindow); got != tt.want {
				t.Errorf("Satisfies() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestSlotRequirements_HasFeature(t *testing.T) {
	req := SlotRequirements{
		RequiredFeatures: []string{"function_calling", "streaming"},
	}

	tests := []struct {
		feature string
		want    bool
	}{
		{"function_calling", true},
		{"streaming", true},
		{"vision", false},
		{"", false},
	}

	for _, tt := range tests {
		t.Run(tt.feature, func(t *testing.T) {
			if got := req.HasFeature(tt.feature); got != tt.want {
				t.Errorf("HasFeature(%q) = %v, want %v", tt.feature, got, tt.want)
			}
		})
	}
}

func TestValidationError_Error(t *testing.T) {
	err := &ValidationError{
		Field:   "Name",
		Message: "cannot be empty",
	}

	want := "Name: cannot be empty"
	if got := err.Error(); got != want {
		t.Errorf("Error() = %q, want %q", got, want)
	}
}

func TestSlotDefinition_EmptyFeatures(t *testing.T) {
	slot := SlotDefinition{
		Name:             "primary",
		Description:      "Test",
		RequiredFeatures: []string{},
	}

	if err := slot.Validate(); err != nil {
		t.Errorf("Validate() should succeed with empty features: %v", err)
	}

	if slot.HasFeature("anything") {
		t.Error("HasFeature should return false for empty features list")
	}
}

func TestSlotDefinition_EmptyModels(t *testing.T) {
	slot := SlotDefinition{
		Name:            "primary",
		Description:     "Test",
		PreferredModels: []string{},
	}

	if err := slot.Validate(); err != nil {
		t.Errorf("Validate() should succeed with empty models: %v", err)
	}

	if slot.PrefersModel("anything") {
		t.Error("PrefersModel should return false for empty models list")
	}
}
