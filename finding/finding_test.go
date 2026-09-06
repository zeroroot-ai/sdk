// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Zero Root AI

package finding

import (
	"testing"
	"time"
)

func TestNewFinding(t *testing.T) {
	missionID := "mission-123"
	agentName := "agent-sql"
	title := "SQL Injection Found"
	description := "Discovered SQL injection in login form"
	category := CategoryDataExtraction
	severity := SeverityHigh

	before := time.Now()
	finding := NewFinding(missionID, agentName, title, description, string(category), severity)
	after := time.Now()

	if finding.ID == "" {
		t.Error("NewFinding() ID is empty, want auto-generated UUID")
	}
	if finding.MissionID != missionID {
		t.Errorf("NewFinding() MissionID = %v, want %v", finding.MissionID, missionID)
	}
	if finding.AgentName != agentName {
		t.Errorf("NewFinding() AgentName = %v, want %v", finding.AgentName, agentName)
	}
	if finding.Title != title {
		t.Errorf("NewFinding() Title = %v, want %v", finding.Title, title)
	}
	if finding.Description != description {
		t.Errorf("NewFinding() Description = %v, want %v", finding.Description, description)
	}
	if finding.Category != string(category) {
		t.Errorf("NewFinding() Category = %v, want %v", finding.Category, category)
	}
	if finding.Severity != severity {
		t.Errorf("NewFinding() Severity = %v, want %v", finding.Severity, severity)
	}
	if finding.Confidence != 1.0 {
		t.Errorf("NewFinding() Confidence = %v, want 1.0", finding.Confidence)
	}
	if finding.Status != StatusOpen {
		t.Errorf("NewFinding() Status = %v, want %v", finding.Status, StatusOpen)
	}
	if finding.CreatedAt.Before(before) || finding.CreatedAt.After(after) {
		t.Error("NewFinding() CreatedAt not in expected range")
	}
	if finding.UpdatedAt.Before(before) || finding.UpdatedAt.After(after) {
		t.Error("NewFinding() UpdatedAt not in expected range")
	}
	if finding.RiskScore != severity.Weight() {
		t.Errorf("NewFinding() RiskScore = %v, want %v", finding.RiskScore, severity.Weight())
	}
}

func TestNewFindingWithID(t *testing.T) {
	id := "custom-id-123"
	finding := NewFindingWithID(id, "mission-1", "agent-1", "Title", "Description", CategoryJailbreak, SeverityMedium)

	if finding.ID != id {
		t.Errorf("NewFindingWithID() ID = %v, want %v", finding.ID, id)
	}
}

func TestFinding_Validate(t *testing.T) {
	validFinding := NewFinding(
		"mission-1",
		"agent-1",
		"Test Finding",
		"Test Description",
		CategoryJailbreak,
		SeverityHigh,
	)

	tests := []struct {
		name     string
		finding  *Finding
		wantErr  bool
		errField string
	}{
		{
			name:    "valid finding",
			finding: validFinding,
			wantErr: false,
		},
		{
			name: "missing ID",
			finding: &Finding{
				MissionID:   "mission-1",
				AgentName:   "agent-1",
				Title:       "Title",
				Description: "Description",
				Category:    CategoryJailbreak,
				Severity:    SeverityHigh,
				Confidence:  1.0,
				Status:      StatusOpen,
				CreatedAt:   time.Now(),
				UpdatedAt:   time.Now(),
			},
			wantErr:  true,
			errField: "ID",
		},
		{
			name: "missing mission ID",
			finding: &Finding{
				ID:          "id-1",
				AgentName:   "agent-1",
				Title:       "Title",
				Description: "Description",
				Category:    CategoryJailbreak,
				Severity:    SeverityHigh,
				Confidence:  1.0,
				Status:      StatusOpen,
				CreatedAt:   time.Now(),
				UpdatedAt:   time.Now(),
			},
			wantErr:  true,
			errField: "mission",
		},
		{
			name: "missing agent name",
			finding: &Finding{
				ID:          "id-1",
				MissionID:   "mission-1",
				Title:       "Title",
				Description: "Description",
				Category:    CategoryJailbreak,
				Severity:    SeverityHigh,
				Confidence:  1.0,
				Status:      StatusOpen,
				CreatedAt:   time.Now(),
				UpdatedAt:   time.Now(),
			},
			wantErr:  true,
			errField: "agent",
		},
		{
			name: "missing title",
			finding: &Finding{
				ID:          "id-1",
				MissionID:   "mission-1",
				AgentName:   "agent-1",
				Description: "Description",
				Category:    CategoryJailbreak,
				Severity:    SeverityHigh,
				Confidence:  1.0,
				Status:      StatusOpen,
				CreatedAt:   time.Now(),
				UpdatedAt:   time.Now(),
			},
			wantErr:  true,
			errField: "title",
		},
		{
			name: "missing description",
			finding: &Finding{
				ID:         "id-1",
				MissionID:  "mission-1",
				AgentName:  "agent-1",
				Title:      "Title",
				Category:   CategoryJailbreak,
				Severity:   SeverityHigh,
				Confidence: 1.0,
				Status:     StatusOpen,
				CreatedAt:  time.Now(),
				UpdatedAt:  time.Now(),
			},
			wantErr:  true,
			errField: "description",
		},
		{
			name: "empty category",
			finding: &Finding{
				ID:          "id-1",
				MissionID:   "mission-1",
				AgentName:   "agent-1",
				Title:       "Title",
				Description: "Description",
				Category:    "",
				Severity:    SeverityHigh,
				Confidence:  1.0,
				Status:      StatusOpen,
				CreatedAt:   time.Now(),
				UpdatedAt:   time.Now(),
			},
			wantErr:  true,
			errField: "category",
		},
		{
			name: "invalid severity",
			finding: &Finding{
				ID:          "id-1",
				MissionID:   "mission-1",
				AgentName:   "agent-1",
				Title:       "Title",
				Description: "Description",
				Category:    CategoryJailbreak,
				Severity:    Severity("invalid"),
				Confidence:  1.0,
				Status:      StatusOpen,
				CreatedAt:   time.Now(),
				UpdatedAt:   time.Now(),
			},
			wantErr:  true,
			errField: "severity",
		},
		{
			name: "confidence too low",
			finding: &Finding{
				ID:          "id-1",
				MissionID:   "mission-1",
				AgentName:   "agent-1",
				Title:       "Title",
				Description: "Description",
				Category:    CategoryJailbreak,
				Severity:    SeverityHigh,
				Confidence:  -0.1,
				Status:      StatusOpen,
				CreatedAt:   time.Now(),
				UpdatedAt:   time.Now(),
			},
			wantErr:  true,
			errField: "confidence",
		},
		{
			name: "confidence too high",
			finding: &Finding{
				ID:          "id-1",
				MissionID:   "mission-1",
				AgentName:   "agent-1",
				Title:       "Title",
				Description: "Description",
				Category:    CategoryJailbreak,
				Severity:    SeverityHigh,
				Confidence:  1.1,
				Status:      StatusOpen,
				CreatedAt:   time.Now(),
				UpdatedAt:   time.Now(),
			},
			wantErr:  true,
			errField: "confidence",
		},
		{
			name: "invalid status",
			finding: &Finding{
				ID:          "id-1",
				MissionID:   "mission-1",
				AgentName:   "agent-1",
				Title:       "Title",
				Description: "Description",
				Category:    CategoryJailbreak,
				Severity:    SeverityHigh,
				Confidence:  1.0,
				Status:      Status("invalid"),
				CreatedAt:   time.Now(),
				UpdatedAt:   time.Now(),
			},
			wantErr:  true,
			errField: "status",
		},
		{
			name: "CVSS score too low",
			finding: &Finding{
				ID:          "id-1",
				MissionID:   "mission-1",
				AgentName:   "agent-1",
				Title:       "Title",
				Description: "Description",
				Category:    CategoryJailbreak,
				Severity:    SeverityHigh,
				Confidence:  1.0,
				Status:      StatusOpen,
				CVSSScore:   ptrFloat64(-0.1),
				CreatedAt:   time.Now(),
				UpdatedAt:   time.Now(),
			},
			wantErr:  true,
			errField: "CVSS",
		},
		{
			name: "CVSS score too high",
			finding: &Finding{
				ID:          "id-1",
				MissionID:   "mission-1",
				AgentName:   "agent-1",
				Title:       "Title",
				Description: "Description",
				Category:    CategoryJailbreak,
				Severity:    SeverityHigh,
				Confidence:  1.0,
				Status:      StatusOpen,
				CVSSScore:   ptrFloat64(10.1),
				CreatedAt:   time.Now(),
				UpdatedAt:   time.Now(),
			},
			wantErr:  true,
			errField: "CVSS",
		},
		{
			name: "missing created_at",
			finding: &Finding{
				ID:          "id-1",
				MissionID:   "mission-1",
				AgentName:   "agent-1",
				Title:       "Title",
				Description: "Description",
				Category:    CategoryJailbreak,
				Severity:    SeverityHigh,
				Confidence:  1.0,
				Status:      StatusOpen,
				UpdatedAt:   time.Now(),
			},
			wantErr:  true,
			errField: "created_at",
		},
		{
			name: "missing updated_at",
			finding: &Finding{
				ID:          "id-1",
				MissionID:   "mission-1",
				AgentName:   "agent-1",
				Title:       "Title",
				Description: "Description",
				Category:    CategoryJailbreak,
				Severity:    SeverityHigh,
				Confidence:  1.0,
				Status:      StatusOpen,
				CreatedAt:   time.Now(),
			},
			wantErr:  true,
			errField: "updated_at",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.finding.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("Finding.Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestFinding_AddEvidence(t *testing.T) {
	finding := NewFinding("mission-1", "agent-1", "Title", "Description", CategoryJailbreak, SeverityHigh)
	initialUpdateTime := finding.UpdatedAt

	time.Sleep(10 * time.Millisecond) // Ensure timestamp difference

	evidence := *NewEvidence(EvidenceHTTPRequest, "Test Request", "POST /api/test")
	finding.AddEvidence(evidence)

	if len(finding.Evidence) != 1 {
		t.Errorf("AddEvidence() resulted in %d evidence, want 1", len(finding.Evidence))
	}
	if finding.Evidence[0].Title != "Test Request" {
		t.Errorf("AddEvidence() evidence title = %v, want Test Request", finding.Evidence[0].Title)
	}
	if !finding.UpdatedAt.After(initialUpdateTime) {
		t.Error("AddEvidence() should update UpdatedAt timestamp")
	}
}

func TestFinding_AddReproductionStep(t *testing.T) {
	finding := NewFinding("mission-1", "agent-1", "Title", "Description", CategoryJailbreak, SeverityHigh)
	initialUpdateTime := finding.UpdatedAt

	time.Sleep(10 * time.Millisecond) // Ensure timestamp difference

	step := NewReproStep(1, "First step", "input data", "expected output")
	finding.AddReproductionStep(step)

	if len(finding.Reproduction) != 1 {
		t.Errorf("AddReproductionStep() resulted in %d steps, want 1", len(finding.Reproduction))
	}
	if finding.Reproduction[0].Description != "First step" {
		t.Errorf("AddReproductionStep() step description = %v, want First step", finding.Reproduction[0].Description)
	}
	if !finding.UpdatedAt.After(initialUpdateTime) {
		t.Error("AddReproductionStep() should update UpdatedAt timestamp")
	}
}

func TestFinding_AddTag(t *testing.T) {
	finding := NewFinding("mission-1", "agent-1", "Title", "Description", CategoryJailbreak, SeverityHigh)

	finding.AddTag("tag1")
	finding.AddTag("tag2")
	finding.AddTag("tag1") // Duplicate

	if len(finding.Tags) != 2 {
		t.Errorf("AddTag() resulted in %d tags, want 2", len(finding.Tags))
	}
	if finding.Tags[0] != "tag1" || finding.Tags[1] != "tag2" {
		t.Errorf("AddTag() tags = %v, want [tag1 tag2]", finding.Tags)
	}
}

func TestFinding_SetConfidence(t *testing.T) {
	finding := NewFinding("mission-1", "agent-1", "Title", "Description", CategoryJailbreak, SeverityHigh)
	initialRiskScore := finding.RiskScore

	err := finding.SetConfidence(0.8)
	if err != nil {
		t.Errorf("SetConfidence() error = %v, want nil", err)
	}
	if finding.Confidence != 0.8 {
		t.Errorf("SetConfidence() confidence = %v, want 0.8", finding.Confidence)
	}
	if finding.RiskScore == initialRiskScore {
		t.Error("SetConfidence() should recalculate RiskScore")
	}

	expectedRiskScore := SeverityHigh.Weight() * 0.8
	if finding.RiskScore != expectedRiskScore {
		t.Errorf("SetConfidence() RiskScore = %v, want %v", finding.RiskScore, expectedRiskScore)
	}

	// Test invalid confidence
	err = finding.SetConfidence(-0.1)
	if err == nil {
		t.Error("SetConfidence() with negative value should return error")
	}

	err = finding.SetConfidence(1.1)
	if err == nil {
		t.Error("SetConfidence() with value > 1.0 should return error")
	}
}

func TestFinding_SetStatus(t *testing.T) {
	finding := NewFinding("mission-1", "agent-1", "Title", "Description", CategoryJailbreak, SeverityHigh)

	err := finding.SetStatus(StatusConfirmed)
	if err != nil {
		t.Errorf("SetStatus() error = %v, want nil", err)
	}
	if finding.Status != StatusConfirmed {
		t.Errorf("SetStatus() status = %v, want %v", finding.Status, StatusConfirmed)
	}

	// Test invalid status
	err = finding.SetStatus(Status("invalid"))
	if err == nil {
		t.Error("SetStatus() with invalid status should return error")
	}
}

func TestFinding_SetMitreAttack(t *testing.T) {
	finding := NewFinding("mission-1", "agent-1", "Title", "Description", CategoryJailbreak, SeverityHigh)

	mapping := NewMitreMapping("enterprise", "TA0001", "Initial Access", "T1059", "Command and Scripting Interpreter")
	finding.SetMitreAttack(mapping)

	if finding.MitreAttack == nil {
		t.Fatal("SetMitreAttack() MitreAttack is nil")
	}
	if finding.MitreAttack.TechniqueID != "T1059" {
		t.Errorf("SetMitreAttack() TechniqueID = %v, want T1059", finding.MitreAttack.TechniqueID)
	}
}

func TestFinding_SetMitreAtlas(t *testing.T) {
	finding := NewFinding("mission-1", "agent-1", "Title", "Description", CategoryJailbreak, SeverityHigh)

	mapping := NewMitreMapping("atlas", "AML.TA0000", "ML Model Access", "AML.T0000", "Infer Training Data Membership")
	finding.SetMitreAtlas(mapping)

	if finding.MitreAtlas == nil {
		t.Fatal("SetMitreAtlas() MitreAtlas is nil")
	}
	if finding.MitreAtlas.TechniqueID != "AML.T0000" {
		t.Errorf("SetMitreAtlas() TechniqueID = %v, want AML.T0000", finding.MitreAtlas.TechniqueID)
	}
}

func TestMitreMapping_Validate(t *testing.T) {
	tests := []struct {
		name    string
		mapping *MitreMapping
		wantErr bool
	}{
		{
			name: "valid mapping",
			mapping: &MitreMapping{
				Matrix:        "enterprise",
				TacticID:      "TA0001",
				TacticName:    "Initial Access",
				TechniqueID:   "T1059",
				TechniqueName: "Command and Scripting Interpreter",
			},
			wantErr: false,
		},
		{
			name: "missing matrix",
			mapping: &MitreMapping{
				TacticID:      "TA0001",
				TacticName:    "Initial Access",
				TechniqueID:   "T1059",
				TechniqueName: "Command and Scripting Interpreter",
			},
			wantErr: true,
		},
		{
			name: "missing tactic ID",
			mapping: &MitreMapping{
				Matrix:        "enterprise",
				TacticName:    "Initial Access",
				TechniqueID:   "T1059",
				TechniqueName: "Command and Scripting Interpreter",
			},
			wantErr: true,
		},
		{
			name: "missing tactic name",
			mapping: &MitreMapping{
				Matrix:        "enterprise",
				TacticID:      "TA0001",
				TechniqueID:   "T1059",
				TechniqueName: "Command and Scripting Interpreter",
			},
			wantErr: true,
		},
		{
			name: "missing technique ID",
			mapping: &MitreMapping{
				Matrix:        "enterprise",
				TacticID:      "TA0001",
				TacticName:    "Initial Access",
				TechniqueName: "Command and Scripting Interpreter",
			},
			wantErr: true,
		},
		{
			name: "missing technique name",
			mapping: &MitreMapping{
				Matrix:      "enterprise",
				TacticID:    "TA0001",
				TacticName:  "Initial Access",
				TechniqueID: "T1059",
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.mapping.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("MitreMapping.Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestReproStep_Validate(t *testing.T) {
	tests := []struct {
		name    string
		step    *ReproStep
		wantErr bool
	}{
		{
			name: "valid step",
			step: &ReproStep{
				Order:       1,
				Description: "First step",
				Input:       "input",
				Output:      "output",
			},
			wantErr: false,
		},
		{
			name: "order zero",
			step: &ReproStep{
				Order:       0,
				Description: "Step",
			},
			wantErr: true,
		},
		{
			name: "negative order",
			step: &ReproStep{
				Order:       -1,
				Description: "Step",
			},
			wantErr: true,
		},
		{
			name: "missing description",
			step: &ReproStep{
				Order: 1,
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.step.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("ReproStep.Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestNewMitreMapping(t *testing.T) {
	mapping := NewMitreMapping("enterprise", "TA0001", "Initial Access", "T1059", "Command and Scripting Interpreter")

	if mapping.Matrix != "enterprise" {
		t.Errorf("NewMitreMapping() Matrix = %v, want enterprise", mapping.Matrix)
	}
	if mapping.TacticID != "TA0001" {
		t.Errorf("NewMitreMapping() TacticID = %v, want TA0001", mapping.TacticID)
	}
	if mapping.TacticName != "Initial Access" {
		t.Errorf("NewMitreMapping() TacticName = %v, want Initial Access", mapping.TacticName)
	}
	if mapping.TechniqueID != "T1059" {
		t.Errorf("NewMitreMapping() TechniqueID = %v, want T1059", mapping.TechniqueID)
	}
	if mapping.TechniqueName != "Command and Scripting Interpreter" {
		t.Errorf("NewMitreMapping() TechniqueName = %v, want Command and Scripting Interpreter", mapping.TechniqueName)
	}
}

func TestNewReproStep(t *testing.T) {
	step := NewReproStep(1, "First step", "input data", "expected output")

	if step.Order != 1 {
		t.Errorf("NewReproStep() Order = %v, want 1", step.Order)
	}
	if step.Description != "First step" {
		t.Errorf("NewReproStep() Description = %v, want First step", step.Description)
	}
	if step.Input != "input data" {
		t.Errorf("NewReproStep() Input = %v, want input data", step.Input)
	}
	if step.Output != "expected output" {
		t.Errorf("NewReproStep() Output = %v, want expected output", step.Output)
	}
}

func TestCalculateRiskScore(t *testing.T) {
	tests := []struct {
		name       string
		severity   Severity
		confidence float64
		want       float64
	}{
		{"critical max confidence", SeverityCritical, 1.0, 10.0},
		{"high max confidence", SeverityHigh, 1.0, 7.5},
		{"medium max confidence", SeverityMedium, 1.0, 5.0},
		{"low max confidence", SeverityLow, 1.0, 2.5},
		{"info max confidence", SeverityInfo, 1.0, 1.0},
		{"critical half confidence", SeverityCritical, 0.5, 5.0},
		{"high zero confidence", SeverityHigh, 0.0, 0.0},
		{"medium 0.8 confidence", SeverityMedium, 0.8, 4.0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := calculateRiskScore(tt.severity, tt.confidence)
			if got != tt.want {
				t.Errorf("calculateRiskScore() = %v, want %v", got, tt.want)
			}
		})
	}
}

// Helper function for tests
func ptrFloat64(f float64) *float64 {
	return &f
}

// TestStringCategory tests that Category accepts string values including custom domain categories.
func TestStringCategory(t *testing.T) {
	tests := []struct {
		name        string
		category    string
		shouldValid bool
	}{
		{
			name:        "security category - jailbreak",
			category:    CategoryJailbreak,
			shouldValid: true,
		},
		{
			name:        "security category - data extraction",
			category:    CategoryDataExtraction,
			shouldValid: true,
		},
		{
			name:        "custom compliance category",
			category:    "compliance_drift",
			shouldValid: true,
		},
		{
			name:        "custom infrastructure category",
			category:    "cost_spike",
			shouldValid: true,
		},
		{
			name:        "custom ML category",
			category:    "model_bias",
			shouldValid: true,
		},
		{
			name:        "custom operational category",
			category:    "sla_violation",
			shouldValid: true,
		},
		{
			name:        "empty category",
			category:    "",
			shouldValid: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			finding := NewFinding(
				"mission-1",
				"agent-1",
				"Test Finding",
				"Test Description",
				tt.category,
				SeverityMedium,
			)

			if finding.Category != tt.category {
				t.Errorf("Category = %v, want %v", finding.Category, tt.category)
			}

			err := finding.Validate()
			if tt.shouldValid && err != nil {
				t.Errorf("Expected valid finding, got error: %v", err)
			}
			if !tt.shouldValid && err == nil {
				t.Error("Expected validation error for empty category, got nil")
			}
		})
	}
}

// TestCustomDomainCategories tests that custom domain-specific categories work correctly.
func TestCustomDomainCategories(t *testing.T) {
	customCategories := []string{
		"compliance_drift",
		"cost_spike",
		"model_bias",
		"sla_violation",
		"data_quality_issue",
		"performance_degradation",
		"security_misconfiguration",
	}

	for _, category := range customCategories {
		t.Run(category, func(t *testing.T) {
			finding := NewFinding(
				"mission-test",
				"agent-test",
				"Test Finding",
				"Test Description",
				category,
				SeverityHigh,
			)

			if finding.Category != category {
				t.Errorf("Category = %v, want %v", finding.Category, category)
			}

			err := finding.Validate()
			if err != nil {
				t.Errorf("Custom category %s should be valid, got error: %v", category, err)
			}
		})
	}
}

// TestBackwardCompatibilityCategoryConstants tests that existing security category constants work.
func TestBackwardCompatibilityCategoryConstants(t *testing.T) {
	tests := []struct {
		name     string
		category string
	}{
		{"jailbreak", CategoryJailbreak},
		{"prompt_injection", CategoryPromptInjection},
		{"data_extraction", CategoryDataExtraction},
		{"privilege_escalation", CategoryPrivilegeEscalation},
		{"dos", CategoryDOS},
		{"model_manipulation", CategoryModelManipulation},
		{"information_disclosure", CategoryInformationDisclosure},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Test that constants work as before
			finding := NewFinding(
				"mission-1",
				"agent-1",
				"Test Finding",
				"Test Description",
				tt.category,
				SeverityHigh,
			)

			if finding.Category != tt.category {
				t.Errorf("Category constant = %v, want %v", finding.Category, tt.category)
			}

			err := finding.Validate()
			if err != nil {
				t.Errorf("Backward compatible category %s should be valid, got error: %v", tt.category, err)
			}

			// Test that Category type methods still work
			cat := Category(tt.category)
			if !cat.IsValid() {
				t.Errorf("Category %s IsValid() = false, want true", tt.category)
			}
			if cat.DisplayName() == "" {
				t.Errorf("Category %s DisplayName() is empty", tt.category)
			}
			if cat.Description() == "" {
				t.Errorf("Category %s Description() is empty", tt.category)
			}
		})
	}
}

// TestMetadataSetAndGet tests basic metadata set and get operations.
func TestMetadataSetAndGet(t *testing.T) {
	finding := NewFinding(
		"mission-1",
		"agent-1",
		"Test Finding",
		"Test Description",
		CategoryJailbreak,
		SeverityHigh,
	)

	// Test SetMetadata and GetMetadata
	finding.SetMetadata("key1", "value1")
	finding.SetMetadata("key2", 42)
	finding.SetMetadata("key3", true)
	finding.SetMetadata("key4", 3.14)

	// Test GetMetadata
	val1, ok1 := finding.GetMetadata("key1")
	if !ok1 || val1 != "value1" {
		t.Errorf("GetMetadata(key1) = %v, %v, want value1, true", val1, ok1)
	}

	val2, ok2 := finding.GetMetadata("key2")
	if !ok2 || val2 != 42 {
		t.Errorf("GetMetadata(key2) = %v, %v, want 42, true", val2, ok2)
	}

	val3, ok3 := finding.GetMetadata("key3")
	if !ok3 || val3 != true {
		t.Errorf("GetMetadata(key3) = %v, %v, want true, true", val3, ok3)
	}

	val4, ok4 := finding.GetMetadata("key4")
	if !ok4 || val4 != 3.14 {
		t.Errorf("GetMetadata(key4) = %v, %v, want 3.14, true", val4, ok4)
	}

	// Test missing key
	_, ok := finding.GetMetadata("nonexistent")
	if ok {
		t.Error("GetMetadata(nonexistent) ok = true, want false")
	}
}

// TestMetadataTypedAccess tests typed metadata access using GetTypedMetadata.
func TestMetadataTypedAccess(t *testing.T) {
	finding := NewFinding(
		"mission-1",
		"agent-1",
		"Test Finding",
		"Test Description",
		CategoryJailbreak,
		SeverityHigh,
	)

	// Set various typed values
	finding.SetMetadata("string_key", "test_value")
	finding.SetMetadata("int_key", 123)
	finding.SetMetadata("float_key", 45.67)
	finding.SetMetadata("bool_key", true)
	finding.SetMetadata("slice_key", []string{"a", "b", "c"})

	// Test string retrieval
	strVal, ok := GetTypedMetadata[string](finding, "string_key")
	if !ok || strVal != "test_value" {
		t.Errorf("GetTypedMetadata[string] = %v, %v, want test_value, true", strVal, ok)
	}

	// Test int retrieval
	intVal, ok := GetTypedMetadata[int](finding, "int_key")
	if !ok || intVal != 123 {
		t.Errorf("GetTypedMetadata[int] = %v, %v, want 123, true", intVal, ok)
	}

	// Test float retrieval
	floatVal, ok := GetTypedMetadata[float64](finding, "float_key")
	if !ok || floatVal != 45.67 {
		t.Errorf("GetTypedMetadata[float64] = %v, %v, want 45.67, true", floatVal, ok)
	}

	// Test bool retrieval
	boolVal, ok := GetTypedMetadata[bool](finding, "bool_key")
	if !ok || boolVal != true {
		t.Errorf("GetTypedMetadata[bool] = %v, %v, want true, true", boolVal, ok)
	}

	// Test slice retrieval
	sliceVal, ok := GetTypedMetadata[[]string](finding, "slice_key")
	if !ok || len(sliceVal) != 3 {
		t.Errorf("GetTypedMetadata[[]string] = %v, %v, want [a b c], true", sliceVal, ok)
	}

	// Test wrong type
	_, ok = GetTypedMetadata[int](finding, "string_key")
	if ok {
		t.Error("GetTypedMetadata with wrong type should return false")
	}

	// Test missing key
	_, ok = GetTypedMetadata[string](finding, "missing_key")
	if ok {
		t.Error("GetTypedMetadata with missing key should return false")
	}
}

// TestMetadataHelperMethods tests the convenience helper methods.
func TestMetadataHelperMethods(t *testing.T) {
	finding := NewFinding(
		"mission-1",
		"agent-1",
		"Test Finding",
		"Test Description",
		CategoryJailbreak,
		SeverityHigh,
	)

	// Test string helper
	finding.SetMetadata("name", "test-name")
	strVal, ok := finding.GetStringMetadata("name")
	if !ok || strVal != "test-name" {
		t.Errorf("GetStringMetadata = %v, %v, want test-name, true", strVal, ok)
	}

	// Test float64 helper
	finding.SetMetadata("score", 8.5)
	floatVal, ok := finding.GetFloat64Metadata("score")
	if !ok || floatVal != 8.5 {
		t.Errorf("GetFloat64Metadata = %v, %v, want 8.5, true", floatVal, ok)
	}

	// Test int helper
	finding.SetMetadata("count", 100)
	intVal, ok := finding.GetIntMetadata("count")
	if !ok || intVal != 100 {
		t.Errorf("GetIntMetadata = %v, %v, want 100, true", intVal, ok)
	}

	// Test bool helper
	finding.SetMetadata("enabled", true)
	boolVal, ok := finding.GetBoolMetadata("enabled")
	if !ok || boolVal != true {
		t.Errorf("GetBoolMetadata = %v, %v, want true, true", boolVal, ok)
	}

	// Test string slice helper
	finding.SetMetadata("tags", []string{"tag1", "tag2"})
	sliceVal, ok := finding.GetStringSliceMetadata("tags")
	if !ok || len(sliceVal) != 2 {
		t.Errorf("GetStringSliceMetadata = %v, %v, want [tag1 tag2], true", sliceVal, ok)
	}
}

// TestMetadataHasAndDelete tests HasMetadata and DeleteMetadata.
func TestMetadataHasAndDelete(t *testing.T) {
	finding := NewFinding(
		"mission-1",
		"agent-1",
		"Test Finding",
		"Test Description",
		CategoryJailbreak,
		SeverityHigh,
	)

	// Initially no metadata
	if finding.HasMetadata("key1") {
		t.Error("HasMetadata should return false for empty metadata")
	}

	// Add metadata
	finding.SetMetadata("key1", "value1")
	if !finding.HasMetadata("key1") {
		t.Error("HasMetadata should return true after setting metadata")
	}

	// Delete metadata
	finding.DeleteMetadata("key1")
	if finding.HasMetadata("key1") {
		t.Error("HasMetadata should return false after deleting metadata")
	}

	// Delete non-existent key (should not panic)
	finding.DeleteMetadata("nonexistent")
}

// TestMetadataGetKeys tests GetMetadataKeys.
func TestMetadataGetKeys(t *testing.T) {
	finding := NewFinding(
		"mission-1",
		"agent-1",
		"Test Finding",
		"Test Description",
		CategoryJailbreak,
		SeverityHigh,
	)

	// Initially no keys
	keys := finding.GetMetadataKeys()
	if keys != nil && len(keys) != 0 {
		t.Errorf("GetMetadataKeys should return nil or empty slice, got %v", keys)
	}

	// Add metadata
	finding.SetMetadata("key1", "value1")
	finding.SetMetadata("key2", "value2")
	finding.SetMetadata("key3", "value3")

	keys = finding.GetMetadataKeys()
	if len(keys) != 3 {
		t.Errorf("GetMetadataKeys length = %d, want 3", len(keys))
	}

	// Check all keys are present (order doesn't matter)
	keyMap := make(map[string]bool)
	for _, k := range keys {
		keyMap[k] = true
	}
	if !keyMap["key1"] || !keyMap["key2"] || !keyMap["key3"] {
		t.Errorf("GetMetadataKeys = %v, want all keys [key1, key2, key3]", keys)
	}
}

// TestMetadataComplexTypes tests storing and retrieving complex types in metadata.
func TestMetadataComplexTypes(t *testing.T) {
	finding := NewFinding(
		"mission-1",
		"agent-1",
		"Test Finding",
		"Test Description",
		"compliance_drift",
		SeverityMedium,
	)

	// Test map storage
	complianceData := map[string]any{
		"framework": "SOC2",
		"control":   "CC6.1",
		"status":    "non-compliant",
	}
	finding.SetMetadata("compliance_data", complianceData)

	retrieved, ok := finding.GetMetadata("compliance_data")
	if !ok {
		t.Fatal("Failed to retrieve complex metadata")
	}

	retrievedMap, ok := retrieved.(map[string]any)
	if !ok {
		t.Fatal("Retrieved metadata is not a map")
	}

	if retrievedMap["framework"] != "SOC2" {
		t.Errorf("Nested field framework = %v, want SOC2", retrievedMap["framework"])
	}

	// Test struct-like data through JSON round-trip
	type CostData struct {
		Amount   float64 `json:"amount"`
		Currency string  `json:"currency"`
		Region   string  `json:"region"`
	}

	costData := CostData{
		Amount:   1500.50,
		Currency: "USD",
		Region:   "us-east-1",
	}
	finding.SetMetadata("cost_data", costData)

	retrievedCost, ok := GetTypedMetadata[CostData](finding, "cost_data")
	if !ok {
		t.Fatal("Failed to retrieve typed struct metadata")
	}

	if retrievedCost.Amount != 1500.50 {
		t.Errorf("Cost amount = %v, want 1500.50", retrievedCost.Amount)
	}
	if retrievedCost.Currency != "USD" {
		t.Errorf("Cost currency = %v, want USD", retrievedCost.Currency)
	}
}

// TestMetadataWithNilMap tests metadata operations when Metadata map is nil.
func TestMetadataWithNilMap(t *testing.T) {
	// Create a finding but don't initialize Metadata
	finding := &Finding{
		ID:          "test-id",
		MissionID:   "mission-1",
		AgentName:   "agent-1",
		Title:       "Test",
		Description: "Test",
		Category:    CategoryJailbreak,
		Severity:    SeverityHigh,
		Confidence:  1.0,
		Status:      StatusOpen,
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}

	// GetMetadata should work with nil metadata map
	val, ok := finding.GetMetadata("key")
	if ok || val != nil {
		t.Error("GetMetadata on nil metadata map should return nil, false")
	}

	// GetTypedMetadata should work with nil metadata map
	_, ok = GetTypedMetadata[string](finding, "key")
	if ok {
		t.Error("GetTypedMetadata on nil metadata map should return false")
	}

	// HasMetadata should work with nil metadata map
	if finding.HasMetadata("key") {
		t.Error("HasMetadata on nil metadata map should return false")
	}

	// GetMetadataKeys should work with nil metadata map
	keys := finding.GetMetadataKeys()
	if keys != nil {
		t.Errorf("GetMetadataKeys on nil metadata map should return nil, got %v", keys)
	}

	// SetMetadata should initialize the map
	finding.SetMetadata("key", "value")
	if finding.Metadata == nil {
		t.Error("SetMetadata should initialize the metadata map")
	}
	val, ok = finding.GetMetadata("key")
	if !ok || val != "value" {
		t.Error("After SetMetadata, should be able to retrieve the value")
	}
}

// TestMetadataWellKnownKeys tests using well-known metadata keys.
func TestMetadataWellKnownKeys(t *testing.T) {
	finding := NewFinding(
		"mission-1",
		"agent-1",
		"Test Finding",
		"Test Description",
		CategoryJailbreak,
		SeverityHigh,
	)

	// Test well-known keys from metadata.go
	finding.SetMetadata(MetaKeyCVSS, 7.5)
	finding.SetMetadata(MetaKeyCWE, "CWE-79")
	finding.SetMetadata(MetaKeyRiskScore, 8.0)
	finding.SetMetadata(MetaKeyComplianceFramework, "PCI-DSS")
	finding.SetMetadata(MetaKeyCostImpact, 1000.00)
	finding.SetMetadata(MetaKeyResourceARN, "arn:aws:s3:::bucket-name")

	// Verify retrieval
	cvss, ok := finding.GetFloat64Metadata(MetaKeyCVSS)
	if !ok || cvss != 7.5 {
		t.Errorf("CVSS metadata = %v, %v, want 7.5, true", cvss, ok)
	}

	cwe, ok := finding.GetStringMetadata(MetaKeyCWE)
	if !ok || cwe != "CWE-79" {
		t.Errorf("CWE metadata = %v, %v, want CWE-79, true", cwe, ok)
	}

	framework, ok := finding.GetStringMetadata(MetaKeyComplianceFramework)
	if !ok || framework != "PCI-DSS" {
		t.Errorf("Compliance framework = %v, %v, want PCI-DSS, true", framework, ok)
	}
}

// TestFindingWithMetadataValidation tests that findings with metadata pass validation.
func TestFindingWithMetadataValidation(t *testing.T) {
	finding := NewFinding(
		"mission-1",
		"agent-1",
		"Compliance Finding",
		"SOC2 control violation detected",
		"compliance_drift",
		SeverityHigh,
	)

	// Add domain-specific metadata
	finding.SetMetadata(MetaKeyComplianceFramework, "SOC2")
	finding.SetMetadata(MetaKeyComplianceControl, "CC6.1")
	finding.SetMetadata("violation_type", "encryption_missing")

	// Validation should pass with custom category and metadata
	err := finding.Validate()
	if err != nil {
		t.Errorf("Validation failed for finding with custom category and metadata: %v", err)
	}

	// Verify metadata survived validation
	framework, ok := finding.GetStringMetadata(MetaKeyComplianceFramework)
	if !ok || framework != "SOC2" {
		t.Error("Metadata should be preserved after validation")
	}
}
