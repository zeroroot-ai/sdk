// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Zero Root AI

package security

import (
	"encoding/json"
	"testing"

	"github.com/zeroroot-ai/sdk/finding"
)

func TestSetGetMitreAttack(t *testing.T) {
	tests := []struct {
		name    string
		finding *finding.Finding
		mapping MitreMapping
		wantOk  bool
	}{
		{
			name:    "set and get mitre attack mapping",
			finding: finding.NewFinding("mission-1", "agent-1", "Test", "Desc", "test", finding.SeverityHigh),
			mapping: MitreMapping{
				Matrix:        "enterprise",
				TacticID:      "TA0001",
				TacticName:    "Initial Access",
				TechniqueID:   "T1059",
				TechniqueName: "Command and Scripting Interpreter",
				SubTechniques: []string{"T1059.001", "T1059.003"},
			},
			wantOk: true,
		},
		{
			name:    "set and get minimal mitre attack mapping",
			finding: finding.NewFinding("mission-1", "agent-1", "Test", "Desc", "test", finding.SeverityMedium),
			mapping: MitreMapping{
				Matrix:        "mobile",
				TacticID:      "TA0042",
				TacticName:    "Network Effects",
				TechniqueID:   "T1437",
				TechniqueName: "Application Layer Protocol",
			},
			wantOk: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			SetMitreAttack(tt.finding, tt.mapping)
			got, ok := GetMitreAttack(tt.finding)

			if ok != tt.wantOk {
				t.Errorf("GetMitreAttack() ok = %v, want %v", ok, tt.wantOk)
			}

			if !ok {
				return
			}

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

func TestGetMitreAttack_NotSet(t *testing.T) {
	f := finding.NewFinding("mission-1", "agent-1", "Test", "Desc", "test", finding.SeverityHigh)
	_, ok := GetMitreAttack(f)

	if ok {
		t.Error("GetMitreAttack() should return false when not set")
	}
}

func TestGetMitreAttack_NilFinding(t *testing.T) {
	_, ok := GetMitreAttack(nil)

	if ok {
		t.Error("GetMitreAttack() should return false for nil finding")
	}
}

func TestSetGetMitreAtlas(t *testing.T) {
	tests := []struct {
		name    string
		finding *finding.Finding
		mapping MitreMapping
		wantOk  bool
	}{
		{
			name:    "set and get mitre atlas mapping",
			finding: finding.NewFinding("mission-1", "agent-1", "Test", "Desc", "test", finding.SeverityHigh),
			mapping: MitreMapping{
				Matrix:        "atlas",
				TacticID:      "AML.TA0000",
				TacticName:    "ML Model Access",
				TechniqueID:   "AML.T0000",
				TechniqueName: "Craft Adversarial Input",
				SubTechniques: []string{"AML.T0000.000"},
			},
			wantOk: true,
		},
		{
			name:    "set and get minimal mitre atlas mapping",
			finding: finding.NewFinding("mission-1", "agent-1", "Test", "Desc", "test", finding.SeverityCritical),
			mapping: MitreMapping{
				Matrix:        "atlas",
				TacticID:      "AML.TA0001",
				TacticName:    "Reconnaissance",
				TechniqueID:   "AML.T0001",
				TechniqueName: "Discover ML Model Family",
			},
			wantOk: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			SetMitreAtlas(tt.finding, tt.mapping)
			got, ok := GetMitreAtlas(tt.finding)

			if ok != tt.wantOk {
				t.Errorf("GetMitreAtlas() ok = %v, want %v", ok, tt.wantOk)
			}

			if !ok {
				return
			}

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

func TestGetMitreAtlas_NotSet(t *testing.T) {
	f := finding.NewFinding("mission-1", "agent-1", "Test", "Desc", "test", finding.SeverityHigh)
	_, ok := GetMitreAtlas(f)

	if ok {
		t.Error("GetMitreAtlas() should return false when not set")
	}
}

func TestGetMitreAtlas_NilFinding(t *testing.T) {
	_, ok := GetMitreAtlas(nil)

	if ok {
		t.Error("GetMitreAtlas() should return false for nil finding")
	}
}

func TestSetGetCVSS(t *testing.T) {
	tests := []struct {
		name    string
		finding *finding.Finding
		score   CVSSScore
		wantOk  bool
	}{
		{
			name:    "set and get CVSS v3.1 score",
			finding: finding.NewFinding("mission-1", "agent-1", "Test", "Desc", "test", finding.SeverityHigh),
			score: CVSSScore{
				Version: "3.1",
				Vector:  "CVSS:3.1/AV:N/AC:L/PR:N/UI:N/S:U/C:H/I:H/A:H",
				Score:   9.8,
			},
			wantOk: true,
		},
		{
			name:    "set and get CVSS v4.0 score",
			finding: finding.NewFinding("mission-1", "agent-1", "Test", "Desc", "test", finding.SeverityMedium),
			score: CVSSScore{
				Version: "4.0",
				Vector:  "CVSS:4.0/AV:N/AC:L/AT:N/PR:N/UI:N/VC:H/VI:H/VA:H/SC:N/SI:N/SA:N",
				Score:   7.5,
			},
			wantOk: true,
		},
		{
			name:    "set and get CVSS with zero score",
			finding: finding.NewFinding("mission-1", "agent-1", "Test", "Desc", "test", finding.SeverityInfo),
			score: CVSSScore{
				Version: "3.1",
				Vector:  "CVSS:3.1/AV:L/AC:H/PR:H/UI:R/S:U/C:N/I:N/A:N",
				Score:   0.0,
			},
			wantOk: true,
		},
		{
			name:    "set and get CVSS with max score",
			finding: finding.NewFinding("mission-1", "agent-1", "Test", "Desc", "test", finding.SeverityCritical),
			score: CVSSScore{
				Version: "3.1",
				Vector:  "CVSS:3.1/AV:N/AC:L/PR:N/UI:N/S:C/C:H/I:H/A:H",
				Score:   10.0,
			},
			wantOk: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			SetCVSS(tt.finding, tt.score)
			got, ok := GetCVSS(tt.finding)

			if ok != tt.wantOk {
				t.Errorf("GetCVSS() ok = %v, want %v", ok, tt.wantOk)
			}

			if !ok {
				return
			}

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

func TestGetCVSS_NotSet(t *testing.T) {
	f := finding.NewFinding("mission-1", "agent-1", "Test", "Desc", "test", finding.SeverityHigh)
	_, ok := GetCVSS(f)

	if ok {
		t.Error("GetCVSS() should return false when not set")
	}
}

func TestGetCVSS_NilFinding(t *testing.T) {
	_, ok := GetCVSS(nil)

	if ok {
		t.Error("GetCVSS() should return false for nil finding")
	}
}

func TestSetGetCWE(t *testing.T) {
	tests := []struct {
		name    string
		finding *finding.Finding
		cweIDs  []string
		wantOk  bool
	}{
		{
			name:    "set and get single CWE",
			finding: finding.NewFinding("mission-1", "agent-1", "Test", "Desc", "test", finding.SeverityHigh),
			cweIDs:  []string{"CWE-79"},
			wantOk:  true,
		},
		{
			name:    "set and get multiple CWEs",
			finding: finding.NewFinding("mission-1", "agent-1", "Test", "Desc", "test", finding.SeverityMedium),
			cweIDs:  []string{"CWE-89", "CWE-79", "CWE-22"},
			wantOk:  true,
		},
		{
			name:    "set and get empty CWE list",
			finding: finding.NewFinding("mission-1", "agent-1", "Test", "Desc", "test", finding.SeverityLow),
			cweIDs:  []string{},
			wantOk:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			SetCWE(tt.finding, tt.cweIDs)
			got, ok := GetCWE(tt.finding)

			if ok != tt.wantOk {
				t.Errorf("GetCWE() ok = %v, want %v", ok, tt.wantOk)
			}

			if !ok {
				return
			}

			if len(got) != len(tt.cweIDs) {
				t.Errorf("CWE count = %v, want %v", len(got), len(tt.cweIDs))
			} else {
				for i, cwe := range got {
					if cwe != tt.cweIDs[i] {
						t.Errorf("CWE[%d] = %v, want %v", i, cwe, tt.cweIDs[i])
					}
				}
			}
		})
	}
}

func TestGetCWE_NotSet(t *testing.T) {
	f := finding.NewFinding("mission-1", "agent-1", "Test", "Desc", "test", finding.SeverityHigh)
	_, ok := GetCWE(f)

	if ok {
		t.Error("GetCWE() should return false when not set")
	}
}

func TestGetCWE_NilFinding(t *testing.T) {
	_, ok := GetCWE(nil)

	if ok {
		t.Error("GetCWE() should return false for nil finding")
	}
}

func TestSetCWE_NilSlice(t *testing.T) {
	f := finding.NewFinding("mission-1", "agent-1", "Test", "Desc", "test", finding.SeverityHigh)
	SetCWE(f, nil)
	got, ok := GetCWE(f)

	if !ok {
		t.Error("GetCWE() should return true even for nil slice")
		return
	}

	if got != nil {
		t.Errorf("GetCWE() = %v, want nil", got)
	}
}

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

func TestNewSecurityFinding(t *testing.T) {
	tests := []struct {
		name        string
		missionID   string
		agentName   string
		title       string
		description string
		category    string
		severity    finding.Severity
	}{
		{
			name:        "create jailbreak finding",
			missionID:   "mission-123",
			agentName:   "security-agent-1",
			title:       "System Prompt Jailbreak",
			description: "User bypassed system instructions",
			category:    CategoryJailbreak,
			severity:    finding.SeverityCritical,
		},
		{
			name:        "create prompt injection finding",
			missionID:   "mission-456",
			agentName:   "scanner-1",
			title:       "Indirect Prompt Injection",
			description: "Malicious instructions in retrieved context",
			category:    CategoryPromptInjection,
			severity:    finding.SeverityHigh,
		},
		{
			name:        "create data extraction finding",
			missionID:   "mission-789",
			agentName:   "tester-2",
			title:       "PII Data Extraction",
			description: "Model leaked personally identifiable information",
			category:    CategoryDataExtraction,
			severity:    finding.SeverityMedium,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f := NewSecurityFinding(tt.missionID, tt.agentName, tt.title, tt.description, tt.category, tt.severity)

			if f == nil {
				t.Fatal("NewSecurityFinding() returned nil")
			}

			if f.ID == "" {
				t.Error("ID should be auto-generated")
			}

			if f.MissionID != tt.missionID {
				t.Errorf("MissionID = %v, want %v", f.MissionID, tt.missionID)
			}

			if f.AgentName != tt.agentName {
				t.Errorf("AgentName = %v, want %v", f.AgentName, tt.agentName)
			}

			if f.Title != tt.title {
				t.Errorf("Title = %v, want %v", f.Title, tt.title)
			}

			if f.Description != tt.description {
				t.Errorf("Description = %v, want %v", f.Description, tt.description)
			}

			if f.Category != tt.category {
				t.Errorf("Category = %v, want %v", f.Category, tt.category)
			}

			if f.Severity != tt.severity {
				t.Errorf("Severity = %v, want %v", f.Severity, tt.severity)
			}

			if f.Status != finding.StatusOpen {
				t.Errorf("Status = %v, want %v", f.Status, finding.StatusOpen)
			}

			if f.Confidence != 1.0 {
				t.Errorf("Confidence = %v, want 1.0", f.Confidence)
			}

			if f.CreatedAt.IsZero() {
				t.Error("CreatedAt should be set")
			}

			if f.UpdatedAt.IsZero() {
				t.Error("UpdatedAt should be set")
			}

			// Verify risk score calculation
			expectedRiskScore := tt.severity.Weight() * 1.0
			if f.RiskScore != expectedRiskScore {
				t.Errorf("RiskScore = %v, want %v", f.RiskScore, expectedRiskScore)
			}
		})
	}
}

func TestSecurityCategoryConstants(t *testing.T) {
	// Verify that security category constants are exported and have expected values
	categories := []struct {
		name  string
		value string
	}{
		{"CategoryJailbreak", CategoryJailbreak},
		{"CategoryPromptInjection", CategoryPromptInjection},
		{"CategoryDataExtraction", CategoryDataExtraction},
		{"CategoryPrivilegeEscalation", CategoryPrivilegeEscalation},
		{"CategoryDOS", CategoryDOS},
		{"CategoryModelManipulation", CategoryModelManipulation},
		{"CategoryInformationDisclosure", CategoryInformationDisclosure},
	}

	for _, cat := range categories {
		t.Run(cat.name, func(t *testing.T) {
			if cat.value == "" {
				t.Errorf("%s should not be empty", cat.name)
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
		{"MetaKeyRiskScore", MetaKeyRiskScore},
	}

	for _, key := range keys {
		t.Run(key.name, func(t *testing.T) {
			if key.value == "" {
				t.Errorf("%s should not be empty", key.name)
			}
		})
	}
}

func TestHelpers_WithNilMetadata(t *testing.T) {
	// Create a finding without initializing metadata
	f := &finding.Finding{
		ID:        "test-1",
		MissionID: "mission-1",
		AgentName: "agent-1",
		// Metadata is nil
	}

	// Test that setters initialize metadata map
	SetMitreAttack(f, MitreMapping{
		Matrix:        "enterprise",
		TacticID:      "TA0001",
		TacticName:    "Initial Access",
		TechniqueID:   "T1059",
		TechniqueName: "Command and Scripting Interpreter",
	})

	if f.Metadata == nil {
		t.Error("SetMitreAttack should initialize metadata map")
	}

	got, ok := GetMitreAttack(f)
	if !ok {
		t.Error("GetMitreAttack should return true after set")
	}

	if got.Matrix != "enterprise" {
		t.Errorf("Matrix = %v, want enterprise", got.Matrix)
	}
}

func TestHelpers_RoundTripThroughJSON(t *testing.T) {
	// Create a finding with all security metadata
	f := NewSecurityFinding("mission-1", "agent-1", "Test", "Desc", CategoryJailbreak, finding.SeverityCritical)

	// Set all metadata
	SetMitreAttack(f, MitreMapping{
		Matrix:        "enterprise",
		TacticID:      "TA0001",
		TacticName:    "Initial Access",
		TechniqueID:   "T1059",
		TechniqueName: "Command and Scripting Interpreter",
		SubTechniques: []string{"T1059.001"},
	})

	SetMitreAtlas(f, MitreMapping{
		Matrix:        "atlas",
		TacticID:      "AML.TA0000",
		TacticName:    "ML Model Access",
		TechniqueID:   "AML.T0000",
		TechniqueName: "Craft Adversarial Input",
	})

	SetCVSS(f, CVSSScore{
		Version: "3.1",
		Vector:  "CVSS:3.1/AV:N/AC:L/PR:N/UI:N/S:U/C:H/I:H/A:H",
		Score:   9.8,
	})

	SetCWE(f, []string{"CWE-79", "CWE-89"})

	// Marshal to JSON
	data, err := json.Marshal(f)
	if err != nil {
		t.Fatalf("json.Marshal() error = %v", err)
	}

	// Unmarshal back
	var got finding.Finding
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatalf("json.Unmarshal() error = %v", err)
	}

	// Verify all metadata can be retrieved
	attack, ok := GetMitreAttack(&got)
	if !ok {
		t.Error("GetMitreAttack should return true after JSON round-trip")
	}
	if attack.TechniqueID != "T1059" {
		t.Errorf("MITRE ATT&CK TechniqueID = %v, want T1059", attack.TechniqueID)
	}

	atlas, ok := GetMitreAtlas(&got)
	if !ok {
		t.Error("GetMitreAtlas should return true after JSON round-trip")
	}
	if atlas.TechniqueID != "AML.T0000" {
		t.Errorf("MITRE ATLAS TechniqueID = %v, want AML.T0000", atlas.TechniqueID)
	}

	cvss, ok := GetCVSS(&got)
	if !ok {
		t.Error("GetCVSS should return true after JSON round-trip")
	}
	if cvss.Score != 9.8 {
		t.Errorf("CVSS Score = %v, want 9.8", cvss.Score)
	}

	cwe, ok := GetCWE(&got)
	if !ok {
		t.Error("GetCWE should return true after JSON round-trip")
	}
	if len(cwe) != 2 {
		t.Errorf("CWE count = %v, want 2", len(cwe))
	}
}

func TestHelpers_OverwriteExisting(t *testing.T) {
	f := NewSecurityFinding("mission-1", "agent-1", "Test", "Desc", CategoryJailbreak, finding.SeverityHigh)

	// Set initial value
	SetCVSS(f, CVSSScore{
		Version: "3.1",
		Vector:  "CVSS:3.1/AV:N/AC:L/PR:N/UI:N/S:U/C:H/I:H/A:H",
		Score:   9.8,
	})

	// Overwrite with new value
	SetCVSS(f, CVSSScore{
		Version: "4.0",
		Vector:  "CVSS:4.0/AV:N/AC:L/AT:N/PR:N/UI:N/VC:H/VI:H/VA:H/SC:N/SI:N/SA:N",
		Score:   7.5,
	})

	got, ok := GetCVSS(f)
	if !ok {
		t.Fatal("GetCVSS should return true")
	}

	if got.Version != "4.0" {
		t.Errorf("Version = %v, want 4.0 (should be overwritten)", got.Version)
	}
	if got.Score != 7.5 {
		t.Errorf("Score = %v, want 7.5 (should be overwritten)", got.Score)
	}
}
