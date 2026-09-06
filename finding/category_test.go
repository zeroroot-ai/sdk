// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Zero Root AI

package finding

import "testing"

func TestCategory_IsValid(t *testing.T) {
	tests := []struct {
		name     string
		category Category
		want     bool
	}{
		{"jailbreak is valid", CategoryJailbreak, true},
		{"prompt_injection is valid", CategoryPromptInjection, true},
		{"data_extraction is valid", CategoryDataExtraction, true},
		{"privilege_escalation is valid", CategoryPrivilegeEscalation, true},
		{"dos is valid", CategoryDOS, true},
		{"model_manipulation is valid", CategoryModelManipulation, true},
		{"information_disclosure is valid", CategoryInformationDisclosure, true},
		{"empty is invalid", Category(""), false},
		{"unknown is valid", Category("unknown"), true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.category.IsValid(); got != tt.want {
				t.Errorf("Category.IsValid() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestCategory_String(t *testing.T) {
	tests := []struct {
		name     string
		category Category
		want     string
	}{
		{"jailbreak", CategoryJailbreak, "jailbreak"},
		{"prompt_injection", CategoryPromptInjection, "prompt_injection"},
		{"data_extraction", CategoryDataExtraction, "data_extraction"},
		{"privilege_escalation", CategoryPrivilegeEscalation, "privilege_escalation"},
		{"dos", CategoryDOS, "dos"},
		{"model_manipulation", CategoryModelManipulation, "model_manipulation"},
		{"information_disclosure", CategoryInformationDisclosure, "information_disclosure"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.category.String(); got != tt.want {
				t.Errorf("Category.String() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestCategory_DisplayName(t *testing.T) {
	tests := []struct {
		name     string
		category Category
		want     string
	}{
		{"jailbreak", CategoryJailbreak, "Jailbreak"},
		{"prompt_injection", CategoryPromptInjection, "Prompt Injection"},
		{"data_extraction", CategoryDataExtraction, "Data Extraction"},
		{"privilege_escalation", CategoryPrivilegeEscalation, "Privilege Escalation"},
		{"dos", CategoryDOS, "Denial of Service"},
		{"model_manipulation", CategoryModelManipulation, "Model Manipulation"},
		{"information_disclosure", CategoryInformationDisclosure, "Information Disclosure"},
		{"unknown", Category("unknown"), "unknown"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.category.DisplayName(); got != tt.want {
				t.Errorf("Category.DisplayName() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestCategory_Description(t *testing.T) {
	tests := []struct {
		name     string
		category Category
		wantText string
	}{
		{"jailbreak has description", CategoryJailbreak, "bypass"},
		{"prompt_injection has description", CategoryPromptInjection, "injection"},
		{"data_extraction has description", CategoryDataExtraction, "exfiltration"},
		{"privilege_escalation has description", CategoryPrivilegeEscalation, "privileges"},
		{"dos has description", CategoryDOS, "denial"},
		{"model_manipulation has description", CategoryModelManipulation, "modify"},
		{"information_disclosure has description", CategoryInformationDisclosure, "exposure"},
		{"unknown returns category string", Category("unknown"), "unknown"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.category.Description()
			if tt.wantText != "" && got == "" {
				t.Errorf("Category.Description() = empty, want non-empty")
			} else if tt.wantText == "unknown" && got != "unknown" {
				t.Errorf("Category.Description() = %v, want %v", got, tt.wantText)
			}
		})
	}
}

func TestParseCategory(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    Category
		wantErr bool
	}{
		{"parse jailbreak", "jailbreak", CategoryJailbreak, false},
		{"parse prompt_injection", "prompt_injection", CategoryPromptInjection, false},
		{"parse data_extraction", "data_extraction", CategoryDataExtraction, false},
		{"parse privilege_escalation", "privilege_escalation", CategoryPrivilegeEscalation, false},
		{"parse dos", "dos", CategoryDOS, false},
		{"parse model_manipulation", "model_manipulation", CategoryModelManipulation, false},
		{"parse information_disclosure", "information_disclosure", CategoryInformationDisclosure, false},
		{"unknown category accepted", "unknown", Category("unknown"), false},
		{"empty string", "", "", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ParseCategory(tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("ParseCategory() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if got != tt.want {
				t.Errorf("ParseCategory() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestAllCategories(t *testing.T) {
	categories := AllCategories()
	if len(categories) != 7 {
		t.Errorf("AllCategories() returned %d categories, want 7", len(categories))
	}

	expected := []string{
		string(CategoryJailbreak),
		string(CategoryPromptInjection),
		string(CategoryDataExtraction),
		string(CategoryPrivilegeEscalation),
		string(CategoryDOS),
		string(CategoryModelManipulation),
		string(CategoryInformationDisclosure),
	}

	for i, cat := range expected {
		if categories[i] != cat {
			t.Errorf("AllCategories()[%d] = %v, want %v", i, categories[i], cat)
		}
	}
}
