// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Zero Root AI

package types

import (
	"encoding/json"
	"testing"
	"time"
)

func TestMissionContext_UnmarshalJSON_ConstraintsFormats(t *testing.T) {
	tests := []struct {
		name    string
		json    string
		wantErr bool
	}{
		{
			name: "constraints as struct",
			json: `{
				"id": "mission-1",
				"name": "Test Mission",
				"constraints": {
					"max_duration": 7200000000000,
					"max_findings": 50,
					"severity_threshold": "high",
					"require_evidence": true
				}
			}`,
			wantErr: false,
		},
		{
			name: "constraints as empty array (Gibson harness format)",
			json: `{
				"id": "mission-1",
				"name": "Test Mission",
				"constraints": []
			}`,
			wantErr: false,
		},
		{
			name: "constraints as string array (Gibson harness format)",
			json: `{
				"id": "mission-1",
				"name": "Test Mission",
				"constraints": ["no-prod-access", "max-10-findings"]
			}`,
			wantErr: false,
		},
		{
			name: "constraints as null",
			json: `{
				"id": "mission-1",
				"name": "Test Mission",
				"constraints": null
			}`,
			wantErr: false,
		},
		{
			name: "constraints missing",
			json: `{
				"id": "mission-1",
				"name": "Test Mission"
			}`,
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var mission MissionContext
			err := json.Unmarshal([]byte(tt.json), &mission)

			if (err != nil) != tt.wantErr {
				t.Errorf("UnmarshalJSON() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if !tt.wantErr {
				if mission.ID != "mission-1" {
					t.Errorf("ID = %v, want mission-1", mission.ID)
				}
				if mission.Name != "Test Mission" {
					t.Errorf("Name = %v, want Test Mission", mission.Name)
				}
			}
		})
	}
}

func TestMissionContext_JSONMarshaling(t *testing.T) {
	original := MissionContext{
		ID:           "mission-1",
		Name:         "Test Mission",
		CurrentAgent: "agent-1",
		Phase:        "reconnaissance",
		Constraints: MissionConstraints{
			MaxDuration:       2 * time.Hour,
			MaxFindings:       50,
			SeverityThreshold: "medium",
			RequireEvidence:   true,
		},
		Metadata: map[string]any{
			"objective": "test objective",
			"priority":  1,
		},
	}

	// Marshal to JSON
	data, err := json.Marshal(original)
	if err != nil {
		t.Fatalf("Failed to marshal: %v", err)
	}

	// Unmarshal back
	var unmarshaled MissionContext
	if err := json.Unmarshal(data, &unmarshaled); err != nil {
		t.Fatalf("Failed to unmarshal: %v", err)
	}

	// Verify basic fields
	if unmarshaled.ID != original.ID {
		t.Errorf("ID = %v, want %v", unmarshaled.ID, original.ID)
	}

	if unmarshaled.Name != original.Name {
		t.Errorf("Name = %v, want %v", unmarshaled.Name, original.Name)
	}

	if unmarshaled.CurrentAgent != original.CurrentAgent {
		t.Errorf("CurrentAgent = %v, want %v", unmarshaled.CurrentAgent, original.CurrentAgent)
	}

	if unmarshaled.Phase != original.Phase {
		t.Errorf("Phase = %v, want %v", unmarshaled.Phase, original.Phase)
	}

	// Verify constraints
	if unmarshaled.Constraints.MaxDuration != original.Constraints.MaxDuration {
		t.Errorf("MaxDuration = %v, want %v", unmarshaled.Constraints.MaxDuration, original.Constraints.MaxDuration)
	}

	if unmarshaled.Constraints.MaxFindings != original.Constraints.MaxFindings {
		t.Errorf("MaxFindings = %v, want %v", unmarshaled.Constraints.MaxFindings, original.Constraints.MaxFindings)
	}

	if unmarshaled.Constraints.SeverityThreshold != original.Constraints.SeverityThreshold {
		t.Errorf("SeverityThreshold = %v, want %v", unmarshaled.Constraints.SeverityThreshold, original.Constraints.SeverityThreshold)
	}

	if unmarshaled.Constraints.RequireEvidence != original.Constraints.RequireEvidence {
		t.Errorf("RequireEvidence = %v, want %v", unmarshaled.Constraints.RequireEvidence, original.Constraints.RequireEvidence)
	}

	// Verify metadata
	if unmarshaled.Metadata["objective"] != "test objective" {
		t.Errorf("Metadata[objective] = %v, want %v", unmarshaled.Metadata["objective"], "test objective")
	}
}
