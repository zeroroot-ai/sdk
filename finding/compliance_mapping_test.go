// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Zero Root AI

package finding

import (
	"strings"
	"testing"
)

func TestComplianceMapping_Validate(t *testing.T) {
	cases := []struct {
		name    string
		mapping ComplianceMapping
		wantErr string
	}{
		{
			name:    "valid minimal",
			mapping: ComplianceMapping{Framework: "SOC2", ControlID: "CC7.1"},
		},
		{
			name:    "valid full",
			mapping: ComplianceMapping{Framework: "NIST_AI_RMF", ControlID: "MEASURE.2.7", Rationale: "because", EvidenceRef: "e1"},
		},
		{
			name:    "empty framework",
			mapping: ComplianceMapping{ControlID: "CC7.1"},
			wantErr: "framework is required",
		},
		{
			name:    "empty control_id",
			mapping: ComplianceMapping{Framework: "SOC2"},
			wantErr: "control_id is required",
		},
		{
			name:    "combined length exceeds cap",
			mapping: ComplianceMapping{Framework: strings.Repeat("a", 200), ControlID: strings.Repeat("b", 100)},
			wantErr: "exceeds",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := tc.mapping.Validate()
			if tc.wantErr == "" {
				if err != nil {
					t.Errorf("unexpected error: %v", err)
				}
				return
			}
			if err == nil {
				t.Errorf("expected error containing %q, got nil", tc.wantErr)
				return
			}
			if !strings.Contains(err.Error(), tc.wantErr) {
				t.Errorf("error %q does not contain %q", err.Error(), tc.wantErr)
			}
		})
	}
}

func TestFinding_AddComplianceMapping(t *testing.T) {
	f := &Finding{}

	// First add.
	err := f.AddComplianceMapping(ComplianceMapping{Framework: "SOC2", ControlID: "CC7.1"})
	if err != nil {
		t.Fatalf("first add failed: %v", err)
	}
	if len(f.ComplianceMappings) != 1 {
		t.Errorf("len = %d; want 1", len(f.ComplianceMappings))
	}

	// Duplicate is a no-op.
	err = f.AddComplianceMapping(ComplianceMapping{Framework: "SOC2", ControlID: "CC7.1", Rationale: "different reason"})
	if err != nil {
		t.Fatalf("dup add returned error: %v", err)
	}
	if len(f.ComplianceMappings) != 1 {
		t.Errorf("duplicate should be no-op; len = %d", len(f.ComplianceMappings))
	}

	// Different control → added.
	err = f.AddComplianceMapping(ComplianceMapping{Framework: "SOC2", ControlID: "CC6.1"})
	if err != nil {
		t.Fatalf("second add failed: %v", err)
	}
	if len(f.ComplianceMappings) != 2 {
		t.Errorf("len = %d; want 2", len(f.ComplianceMappings))
	}

	// Different framework → added.
	err = f.AddComplianceMapping(ComplianceMapping{Framework: "NIST_AI_RMF", ControlID: "CC7.1"})
	if err != nil {
		t.Fatalf("third add failed: %v", err)
	}
	if len(f.ComplianceMappings) != 3 {
		t.Errorf("len = %d; want 3", len(f.ComplianceMappings))
	}

	// Invalid mapping rejected.
	err = f.AddComplianceMapping(ComplianceMapping{Framework: "SOC2"})
	if err == nil {
		t.Errorf("invalid mapping should be rejected")
	}
}

func TestFinding_HasMapping(t *testing.T) {
	f := &Finding{
		ComplianceMappings: []ComplianceMapping{
			{Framework: "SOC2", ControlID: "CC7.1"},
			{Framework: "MITRE_ATLAS", ControlID: "AML.T0051"},
		},
	}

	if !f.HasMapping("SOC2", "CC7.1") {
		t.Errorf("should have SOC2 CC7.1")
	}
	if !f.HasMapping("MITRE_ATLAS", "AML.T0051") {
		t.Errorf("should have MITRE_ATLAS AML.T0051")
	}
	if f.HasMapping("SOC2", "CC6.1") {
		t.Errorf("should not have SOC2 CC6.1")
	}
	if f.HasMapping("SOC2", "AML.T0051") {
		t.Errorf("framework mismatch should not match")
	}
}

func TestFinding_MappingsByFramework(t *testing.T) {
	f := &Finding{
		ComplianceMappings: []ComplianceMapping{
			{Framework: "SOC2", ControlID: "CC7.1"},
			{Framework: "SOC2", ControlID: "CC6.1"},
			{Framework: "NIST_AI_RMF", ControlID: "MEASURE.2.7"},
		},
	}

	soc2 := f.MappingsByFramework("SOC2")
	if len(soc2) != 2 {
		t.Errorf("SOC2 count = %d; want 2", len(soc2))
	}

	nist := f.MappingsByFramework("NIST_AI_RMF")
	if len(nist) != 1 {
		t.Errorf("NIST count = %d; want 1", len(nist))
	}

	missing := f.MappingsByFramework("PLATFORM")
	if len(missing) != 0 {
		t.Errorf("missing framework should return empty slice, got %d", len(missing))
	}
	if missing == nil {
		t.Errorf("should return empty slice, not nil")
	}
}

func TestFinding_ControlIDs(t *testing.T) {
	f := &Finding{
		ComplianceMappings: []ComplianceMapping{
			{Framework: "SOC2", ControlID: "CC7.1"},
			{Framework: "NIST_AI_RMF", ControlID: "MEASURE.2.7"},
		},
	}
	ids := f.ControlIDs()
	if len(ids) != 2 {
		t.Errorf("count = %d; want 2", len(ids))
	}
}

func TestFinding_AddComplianceMappingWith(t *testing.T) {
	f := &Finding{}
	err := f.AddComplianceMappingWith("SOC2", "CC7.1", "because", "e1")
	if err != nil {
		t.Fatal(err)
	}
	if len(f.ComplianceMappings) != 1 {
		t.Fatal("mapping not added")
	}
	m := f.ComplianceMappings[0]
	if m.Rationale != "because" || m.EvidenceRef != "e1" {
		t.Errorf("got %+v", m)
	}
}
