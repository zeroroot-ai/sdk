// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Zero Root AI

package types

import (
	"encoding/json"
	"errors"
	"testing"
	"time"

	"google.golang.org/protobuf/types/known/durationpb"

	missionpb "github.com/zeroroot-ai/sdk/api/gen/gibson/mission/v1"
)

func TestMissionContext_Validate(t *testing.T) {
	tests := []struct {
		name     string
		mission  MissionContext
		wantErr  bool
		errField string
	}{
		{
			name: "valid mission",
			mission: MissionContext{
				ID:   "mission-1",
				Name: "Test Mission",
			},
			wantErr: false,
		},
		{
			name: "missing ID",
			mission: MissionContext{
				Name: "Test Mission",
			},
			wantErr:  true,
			errField: "ID",
		},
		{
			name: "missing name",
			mission: MissionContext{
				ID: "mission-1",
			},
			wantErr:  true,
			errField: "Name",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.mission.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("Validate() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if tt.wantErr && err != nil {
				verr := &ValidationError{}
				if errors.As(err, &verr) {
					if verr.Field != tt.errField {
						t.Errorf("Validate() error field = %v, want %v", verr.Field, tt.errField)
					}
				}
			}
		})
	}
}

func TestMissionContext_GetMetadata(t *testing.T) {
	mission := &MissionContext{
		Metadata: map[string]any{
			"objective": "test objective",
			"priority":  1,
			"team":      "security-team",
		},
	}

	tests := []struct {
		name    string
		key     string
		wantVal any
		wantOk  bool
	}{
		{"existing string", "objective", "test objective", true},
		{"existing int", "priority", 1, true},
		{"existing string 2", "team", "security-team", true},
		{"non-existent", "unknown", nil, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotVal, gotOk := mission.GetMetadata(tt.key)
			if gotOk != tt.wantOk {
				t.Errorf("GetMetadata(%v) ok = %v, want %v", tt.key, gotOk, tt.wantOk)
			}
			if tt.wantOk && gotVal != tt.wantVal {
				t.Errorf("GetMetadata(%v) val = %v, want %v", tt.key, gotVal, tt.wantVal)
			}
		})
	}

	// Test with nil metadata
	emptyMission := &MissionContext{}
	if _, ok := emptyMission.GetMetadata("any-key"); ok {
		t.Error("GetMetadata on nil metadata should return false")
	}
}

func TestMissionContext_SetMetadata(t *testing.T) {
	mission := &MissionContext{}

	mission.SetMetadata("objective", "test objective")
	val, ok := mission.GetMetadata("objective")
	if !ok {
		t.Fatal("SetMetadata failed to set value")
	}
	if val != "test objective" {
		t.Errorf("After SetMetadata, GetMetadata() = %v, want %v", val, "test objective")
	}

	// Update existing metadata
	mission.SetMetadata("objective", "updated objective")
	val, ok = mission.GetMetadata("objective")
	if !ok {
		t.Fatal("GetMetadata failed to retrieve updated value")
	}
	if val != "updated objective" {
		t.Errorf("After updating metadata, GetMetadata() = %v, want %v", val, "updated objective")
	}
}

func TestMissionContext_IsExpired(t *testing.T) {
	tests := []struct {
		name        string
		maxDuration time.Duration
		startTime   time.Time
		want        bool
	}{
		{
			name:        "no max duration",
			maxDuration: 0,
			startTime:   time.Now().Add(-1 * time.Hour),
			want:        false,
		},
		{
			name:        "not expired",
			maxDuration: 2 * time.Hour,
			startTime:   time.Now().Add(-1 * time.Hour),
			want:        false,
		},
		{
			name:        "expired",
			maxDuration: 1 * time.Hour,
			startTime:   time.Now().Add(-2 * time.Hour),
			want:        true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mission := &MissionContext{
				Constraints: MissionConstraints{
					MaxDuration: tt.maxDuration,
				},
				Metadata: map[string]any{
					"start_time": tt.startTime,
				},
			}

			if got := mission.IsExpired(); got != tt.want {
				t.Errorf("IsExpired() = %v, want %v", got, tt.want)
			}
		})
	}

	// Test with no start time
	mission := &MissionContext{
		Constraints: MissionConstraints{
			MaxDuration: 1 * time.Hour,
		},
	}
	if mission.IsExpired() {
		t.Error("IsExpired should return false when no start_time is set")
	}
}

func TestMissionContext_ShouldStop(t *testing.T) {
	tests := []struct {
		name         string
		constraints  MissionConstraints
		startTime    time.Time
		findingCount int
		want         bool
	}{
		{
			name: "no constraints",
			constraints: MissionConstraints{
				MaxDuration: 0,
				MaxFindings: 0,
			},
			findingCount: 100,
			want:         false,
		},
		{
			name: "findings limit reached",
			constraints: MissionConstraints{
				MaxFindings: 10,
			},
			findingCount: 10,
			want:         true,
		},
		{
			name: "findings under limit",
			constraints: MissionConstraints{
				MaxFindings: 10,
			},
			findingCount: 5,
			want:         false,
		},
		{
			name: "time expired",
			constraints: MissionConstraints{
				MaxDuration: 1 * time.Hour,
			},
			startTime:    time.Now().Add(-2 * time.Hour),
			findingCount: 5,
			want:         true,
		},
		{
			name: "time not expired",
			constraints: MissionConstraints{
				MaxDuration: 2 * time.Hour,
			},
			startTime:    time.Now().Add(-1 * time.Hour),
			findingCount: 5,
			want:         false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mission := &MissionContext{
				Constraints: tt.constraints,
			}

			if !tt.startTime.IsZero() {
				mission.SetMetadata("start_time", tt.startTime)
			}

			if got := mission.ShouldStop(tt.findingCount); got != tt.want {
				t.Errorf("ShouldStop(%d) = %v, want %v", tt.findingCount, got, tt.want)
			}
		})
	}
}

func TestMissionConstraints_MeetsSeverityThreshold(t *testing.T) {
	tests := []struct {
		name      string
		threshold string
		severity  string
		want      bool
	}{
		{"no threshold", "", "low", true},
		{"low threshold, low severity", "low", "low", true},
		{"low threshold, medium severity", "low", "medium", true},
		{"low threshold, high severity", "low", "high", true},
		{"medium threshold, low severity", "medium", "low", false},
		{"medium threshold, medium severity", "medium", "medium", true},
		{"medium threshold, high severity", "medium", "high", true},
		{"high threshold, low severity", "high", "low", false},
		{"high threshold, medium severity", "high", "medium", false},
		{"high threshold, high severity", "high", "high", true},
		{"high threshold, critical severity", "high", "critical", true},
		{"critical threshold, high severity", "critical", "high", false},
		{"critical threshold, critical severity", "critical", "critical", true},
		{"unknown threshold", "unknown", "medium", true},
		{"unknown severity", "medium", "unknown", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			constraints := MissionConstraints{
				SeverityThreshold: tt.threshold,
			}

			if got := constraints.MeetsSeverityThreshold(tt.severity); got != tt.want {
				t.Errorf("MeetsSeverityThreshold(%v) with threshold %v = %v, want %v",
					tt.severity, tt.threshold, got, tt.want)
			}
		})
	}
}

func TestNewMissionContext(t *testing.T) {
	id := "mission-1"
	name := "Test Mission"

	mission := NewMissionContext(id, name)

	if mission.ID != id {
		t.Errorf("ID = %v, want %v", mission.ID, id)
	}

	if mission.Name != name {
		t.Errorf("Name = %v, want %v", mission.Name, name)
	}

	if mission.Metadata == nil {
		t.Error("Metadata should be initialized")
	}
}

func TestNewMissionConstraints(t *testing.T) {
	constraints := NewMissionConstraints()

	if constraints.RequireEvidence != true {
		t.Errorf("RequireEvidence = %v, want true", constraints.RequireEvidence)
	}

	if constraints.MaxDuration != 0 {
		t.Errorf("MaxDuration = %v, want 0", constraints.MaxDuration)
	}

	if constraints.MaxFindings != 0 {
		t.Errorf("MaxFindings = %v, want 0", constraints.MaxFindings)
	}
}

func TestMissionConstraints_FluentAPI(t *testing.T) {
	constraints := NewMissionConstraints().
		WithMaxDuration(2 * time.Hour).
		WithMaxFindings(50).
		WithSeverityThreshold("medium").
		WithRequireEvidence(false)

	if constraints.MaxDuration != 2*time.Hour {
		t.Errorf("MaxDuration = %v, want %v", constraints.MaxDuration, 2*time.Hour)
	}

	if constraints.MaxFindings != 50 {
		t.Errorf("MaxFindings = %v, want 50", constraints.MaxFindings)
	}

	if constraints.SeverityThreshold != "medium" {
		t.Errorf("SeverityThreshold = %v, want medium", constraints.SeverityThreshold)
	}

	if constraints.RequireEvidence != false {
		t.Errorf("RequireEvidence = %v, want false", constraints.RequireEvidence)
	}
}

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

// TestFromDefinition verifies that FromDefinition correctly projects
// a proto MissionDefinition into a MissionContext.
func TestFromDefinition(t *testing.T) {
	t.Run("nil definition returns zero value", func(t *testing.T) {
		mc := FromDefinition(nil)
		if mc.ID != "" || mc.Name != "" || mc.Metadata != nil {
			t.Errorf("FromDefinition(nil) = %+v, want zero MissionContext", mc)
		}
	})

	t.Run("projects id and name from proto", func(t *testing.T) {
		def := &missionpb.MissionDefinition{
			Id:   "def-42",
			Name: "SQL Injection Scan",
		}
		mc := FromDefinition(def)
		if mc.ID != "def-42" {
			t.Errorf("ID = %q, want %q", mc.ID, "def-42")
		}
		if mc.Name != "SQL Injection Scan" {
			t.Errorf("Name = %q, want %q", mc.Name, "SQL Injection Scan")
		}
	})

	t.Run("converts proto metadata map[string]string to map[string]any", func(t *testing.T) {
		def := &missionpb.MissionDefinition{
			Id:   "def-99",
			Name: "Recon",
			Metadata: map[string]string{
				"team":     "red",
				"priority": "high",
			},
		}
		mc := FromDefinition(def)
		if mc.Metadata["team"] != "red" {
			t.Errorf("Metadata[team] = %v, want %q", mc.Metadata["team"], "red")
		}
		if mc.Metadata["priority"] != "high" {
			t.Errorf("Metadata[priority] = %v, want %q", mc.Metadata["priority"], "high")
		}
	})

	t.Run("runtime-only fields are zero after projection", func(t *testing.T) {
		def := &missionpb.MissionDefinition{
			Id:   "def-1",
			Name: "Mission",
		}
		mc := FromDefinition(def)
		// Phase and CurrentAgent are runtime fields populated by the daemon envelope,
		// not derived from the definition schema.
		if mc.Phase != "" {
			t.Errorf("Phase = %q, want empty — runtime field must not be set by FromDefinition", mc.Phase)
		}
		if mc.CurrentAgent != "" {
			t.Errorf("CurrentAgent = %q, want empty — runtime field must not be set by FromDefinition", mc.CurrentAgent)
		}
		// Constraints is now projected from the proto field (sdk#47). When the
		// proto field is absent (nil), the projection must produce zero-value
		// Constraints without panicking.
		if mc.Constraints != (MissionConstraints{}) {
			t.Errorf("Constraints = %+v, want zero when proto field is absent", mc.Constraints)
		}
	})

	t.Run("empty metadata on proto leaves Metadata nil", func(t *testing.T) {
		def := &missionpb.MissionDefinition{
			Id:   "def-2",
			Name: "Empty Meta",
		}
		mc := FromDefinition(def)
		if mc.Metadata != nil {
			t.Errorf("Metadata = %v, want nil for proto with no metadata", mc.Metadata)
		}
	})

	t.Run("constraints absent produces zero MissionConstraints no panic", func(t *testing.T) {
		// sdk#47: backward-compat — nil proto constraints must not panic.
		def := &missionpb.MissionDefinition{
			Id:   "def-noconstraints",
			Name: "No Constraints",
		}
		mc := FromDefinition(def)
		if mc.Constraints != (MissionConstraints{}) {
			t.Errorf("Constraints = %+v, want zero when proto constraints absent", mc.Constraints)
		}
	})

	t.Run("constraints present projects all fields", func(t *testing.T) {
		// sdk#47: full round-trip from MissionConstraints proto to MissionContext.Constraints.
		def := &missionpb.MissionDefinition{
			Id:   "def-full",
			Name: "Full Constraints",
			Constraints: &missionpb.MissionConstraints{
				MaxDuration:       durationpb.New(2 * time.Hour),
				MaxTokens:         50000,
				MaxCost:           1.25,
				MaxFindings:       100,
				SeverityThreshold: "high",
				RequireEvidence:   true,
				BlockedTools:      []string{"nmap", "sqlmap"},
				BlockedDomains:    []string{"prod.example.com"},
			},
		}
		mc := FromDefinition(def)
		c := mc.Constraints
		if c.MaxDuration != 2*time.Hour {
			t.Errorf("MaxDuration = %v, want 2h", c.MaxDuration)
		}
		if c.MaxFindings != 100 {
			t.Errorf("MaxFindings = %d, want 100", c.MaxFindings)
		}
		if c.SeverityThreshold != "high" {
			t.Errorf("SeverityThreshold = %q, want %q", c.SeverityThreshold, "high")
		}
		if !c.RequireEvidence {
			t.Errorf("RequireEvidence = false, want true")
		}
		// MaxTokens, MaxCost, BlockedTools, BlockedDomains are proto-level fields
		// not yet surfaced on MissionConstraints runtime struct; verify proto
		// accessors work on the definition itself.
		if def.GetConstraints().GetMaxTokens() != 50000 {
			t.Errorf("proto MaxTokens = %d, want 50000", def.GetConstraints().GetMaxTokens())
		}
		if def.GetConstraints().GetMaxCost() != 1.25 {
			t.Errorf("proto MaxCost = %v, want 1.25", def.GetConstraints().GetMaxCost())
		}
		if len(def.GetConstraints().GetBlockedTools()) != 2 {
			t.Errorf("proto BlockedTools len = %d, want 2", len(def.GetConstraints().GetBlockedTools()))
		}
	})

	t.Run("constraints max_duration zero means no limit", func(t *testing.T) {
		// sdk#47: zero Duration in proto (nil duration field) → zero time.Duration = no limit.
		def := &missionpb.MissionDefinition{
			Id:   "def-nodur",
			Name: "No Duration",
			Constraints: &missionpb.MissionConstraints{
				MaxFindings: 10,
				// max_duration intentionally absent (zero-value Duration)
			},
		}
		mc := FromDefinition(def)
		if mc.Constraints.MaxDuration != 0 {
			t.Errorf("MaxDuration = %v, want 0 (no limit)", mc.Constraints.MaxDuration)
		}
		if mc.Constraints.MaxFindings != 10 {
			t.Errorf("MaxFindings = %d, want 10", mc.Constraints.MaxFindings)
		}
	})

	t.Run("constraints severity threshold projected correctly", func(t *testing.T) {
		for _, threshold := range []string{"low", "medium", "high", "critical"} {
			def := &missionpb.MissionDefinition{
				Id:   "def-sev",
				Name: "Severity Test",
				Constraints: &missionpb.MissionConstraints{
					SeverityThreshold: threshold,
				},
			}
			mc := FromDefinition(def)
			if mc.Constraints.SeverityThreshold != threshold {
				t.Errorf("SeverityThreshold = %q, want %q", mc.Constraints.SeverityThreshold, threshold)
			}
		}
	})
}
