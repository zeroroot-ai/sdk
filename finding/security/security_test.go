// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Zero Root AI

package security

import (
	"encoding/json"
	"testing"
)

func TestMitreMapping_JSONSerialization(t *testing.T) {
	tests := []struct {
		name    string
		mapping MitreMapping
	}{
		{
			name: "full mitre mapping",
			mapping: MitreMapping{
				Matrix:        "enterprise",
				TacticID:      "TA0001",
				TacticName:    "Initial Access",
				TechniqueID:   "T1059",
				TechniqueName: "Command and Scripting Interpreter",
				SubTechniques: []string{"T1059.001", "T1059.003", "T1059.006"},
			},
		},
		{
			name: "minimal mitre mapping",
			mapping: MitreMapping{
				Matrix:        "mobile",
				TacticID:      "TA0042",
				TacticName:    "Network Effects",
				TechniqueID:   "T1437",
				TechniqueName: "Application Layer Protocol",
			},
		},
		{
			name: "empty sub-techniques",
			mapping: MitreMapping{
				Matrix:        "atlas",
				TacticID:      "AML.TA0000",
				TacticName:    "ML Model Access",
				TechniqueID:   "AML.T0000",
				TechniqueName: "Craft Adversarial Input",
				SubTechniques: []string{},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Marshal to JSON
			data, err := json.Marshal(tt.mapping)
			if err != nil {
				t.Fatalf("json.Marshal() error = %v", err)
			}

			// Unmarshal back
			var got MitreMapping
			if err := json.Unmarshal(data, &got); err != nil {
				t.Fatalf("json.Unmarshal() error = %v", err)
			}

			// Verify fields
			if got.Matrix != tt.mapping.Matrix {
				t.Errorf("Matrix = %v, want %v", got.Matrix, tt.mapping.Matrix)
			}
			if got.TacticID != tt.mapping.TacticID {
				t.Errorf("TacticID = %v, want %v", got.TacticID, tt.mapping.TacticID)
			}
			if got.TacticName != tt.mapping.TacticName {
				t.Errorf("TacticName = %v, want %v", got.TacticName, tt.mapping.TacticName)
			}
			if got.TechniqueID != tt.mapping.TechniqueID {
				t.Errorf("TechniqueID = %v, want %v", got.TechniqueID, tt.mapping.TechniqueID)
			}
			if got.TechniqueName != tt.mapping.TechniqueName {
				t.Errorf("TechniqueName = %v, want %v", got.TechniqueName, tt.mapping.TechniqueName)
			}

			// Compare sub-techniques
			if len(got.SubTechniques) != len(tt.mapping.SubTechniques) {
				t.Errorf("SubTechniques length = %v, want %v", len(got.SubTechniques), len(tt.mapping.SubTechniques))
			} else {
				for i, st := range got.SubTechniques {
					if st != tt.mapping.SubTechniques[i] {
						t.Errorf("SubTechniques[%d] = %v, want %v", i, st, tt.mapping.SubTechniques[i])
					}
				}
			}
		})
	}
}

func TestMitreMapping_JSONOmitsEmptySubTechniques(t *testing.T) {
	mapping := MitreMapping{
		Matrix:        "enterprise",
		TacticID:      "TA0001",
		TacticName:    "Initial Access",
		TechniqueID:   "T1059",
		TechniqueName: "Command and Scripting Interpreter",
		// SubTechniques not set (nil)
	}

	data, err := json.Marshal(mapping)
	if err != nil {
		t.Fatalf("json.Marshal() error = %v", err)
	}

	// Verify that sub_techniques is not present in JSON
	var jsonMap map[string]any
	if err := json.Unmarshal(data, &jsonMap); err != nil {
		t.Fatalf("json.Unmarshal() error = %v", err)
	}

	if _, exists := jsonMap["sub_techniques"]; exists {
		t.Error("sub_techniques should be omitted when empty")
	}
}

func TestCVSSScore_JSONSerialization(t *testing.T) {
	tests := []struct {
		name  string
		score CVSSScore
	}{
		{
			name: "CVSS v3.1 high score",
			score: CVSSScore{
				Version: "3.1",
				Vector:  "CVSS:3.1/AV:N/AC:L/PR:N/UI:N/S:U/C:H/I:H/A:H",
				Score:   9.8,
			},
		},
		{
			name: "CVSS v4.0 medium score",
			score: CVSSScore{
				Version: "4.0",
				Vector:  "CVSS:4.0/AV:N/AC:L/AT:N/PR:N/UI:N/VC:H/VI:H/VA:H/SC:N/SI:N/SA:N",
				Score:   7.5,
			},
		},
		{
			name: "CVSS with zero score",
			score: CVSSScore{
				Version: "3.1",
				Vector:  "CVSS:3.1/AV:L/AC:H/PR:H/UI:R/S:U/C:N/I:N/A:N",
				Score:   0.0,
			},
		},
		{
			name: "CVSS with max score",
			score: CVSSScore{
				Version: "3.1",
				Vector:  "CVSS:3.1/AV:N/AC:L/PR:N/UI:N/S:C/C:H/I:H/A:H",
				Score:   10.0,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Marshal to JSON
			data, err := json.Marshal(tt.score)
			if err != nil {
				t.Fatalf("json.Marshal() error = %v", err)
			}

			// Unmarshal back
			var got CVSSScore
			if err := json.Unmarshal(data, &got); err != nil {
				t.Fatalf("json.Unmarshal() error = %v", err)
			}

			// Verify fields
			if got.Version != tt.score.Version {
				t.Errorf("Version = %v, want %v", got.Version, tt.score.Version)
			}
			if got.Vector != tt.score.Vector {
				t.Errorf("Vector = %v, want %v", got.Vector, tt.score.Vector)
			}
			if got.Score != tt.score.Score {
				t.Errorf("Score = %v, want %v", got.Score, tt.score.Score)
			}
		})
	}
}

func TestMetadataKeyConstants(t *testing.T) {
	// Verify that metadata key constants are exported and have expected values
	keys := []struct {
		name  string
		value string
	}{
		{"MetaKeyMitreAttack", MetaKeyMitreAttack},
		{"MetaKeyMitreAtlas", MetaKeyMitreAtlas},
		{"MetaKeyCVSS", MetaKeyCVSS},
		{"MetaKeyCWE", MetaKeyCWE},
	}

	for _, key := range keys {
		t.Run(key.name, func(t *testing.T) {
			if key.value == "" {
				t.Errorf("%s should not be empty", key.name)
			}
		})
	}
}
