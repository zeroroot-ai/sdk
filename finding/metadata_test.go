// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Zero Root AI

package finding

import (
	"testing"
)

func TestSetMetadata(t *testing.T) {
	f := &Finding{
		ID:        "test-1",
		MissionID: "mission-1",
		AgentName: "test-agent",
		Title:     "Test Finding",
	}

	// Test setting metadata
	f.SetMetadata(MetaKeyRiskScore, 7.5)

	if f.Metadata == nil {
		t.Fatal("Metadata map should be initialized")
	}

	val, exists := f.Metadata[MetaKeyRiskScore]
	if !exists {
		t.Error("Metadata key should exist")
	}

	if val != 7.5 {
		t.Errorf("Expected 7.5, got %v", val)
	}
}

func TestGetMetadata(t *testing.T) {
	f := &Finding{
		ID:        "test-1",
		MissionID: "mission-1",
		AgentName: "test-agent",
		Title:     "Test Finding",
		Metadata: map[string]any{
			MetaKeyCostImpact: 100.50,
		},
	}

	val, exists := f.GetMetadata(MetaKeyCostImpact)
	if !exists {
		t.Error("Metadata key should exist")
	}

	if val != 100.50 {
		t.Errorf("Expected 100.50, got %v", val)
	}

	// Test non-existent key
	_, exists = f.GetMetadata("nonexistent")
	if exists {
		t.Error("Non-existent key should not exist")
	}
}

func TestGetTypedMetadata(t *testing.T) {
	f := &Finding{
		ID:        "test-1",
		MissionID: "mission-1",
		AgentName: "test-agent",
		Title:     "Test Finding",
		Metadata: map[string]any{
			MetaKeyRiskScore:           8.5,
			MetaKeyResourceARN:         "arn:aws:ec2:us-east-1:123456789012:instance/i-1234567890abcdef0",
			MetaKeyComplianceFramework: "SOC2",
		},
	}

	// Test float64
	score, ok := GetTypedMetadata[float64](f, MetaKeyRiskScore)
	if !ok {
		t.Error("Should successfully get float64 metadata")
	}
	if score != 8.5 {
		t.Errorf("Expected 8.5, got %f", score)
	}

	// Test string
	arn, ok := GetTypedMetadata[string](f, MetaKeyResourceARN)
	if !ok {
		t.Error("Should successfully get string metadata")
	}
	expected := "arn:aws:ec2:us-east-1:123456789012:instance/i-1234567890abcdef0"
	if arn != expected {
		t.Errorf("Expected %s, got %s", expected, arn)
	}

	// Test non-existent key
	_, ok = GetTypedMetadata[string](f, "nonexistent")
	if ok {
		t.Error("Non-existent key should return false")
	}

	// Test wrong type
	_, ok = GetTypedMetadata[string](f, MetaKeyRiskScore)
	if ok {
		t.Error("Wrong type should return false")
	}
}

func TestGetTypedMetadata_StructTypes(t *testing.T) {
	type CVSSScore struct {
		Version string  `json:"version"`
		Vector  string  `json:"vector"`
		Score   float64 `json:"score"`
	}

	cvss := CVSSScore{
		Version: "3.1",
		Vector:  "CVSS:3.1/AV:N/AC:L/PR:N/UI:N/S:U/C:H/I:H/A:H",
		Score:   9.8,
	}

	f := &Finding{
		ID:        "test-1",
		MissionID: "mission-1",
		AgentName: "test-agent",
		Title:     "Test Finding",
	}

	// Set complex struct
	f.SetMetadata(MetaKeyCVSS, cvss)

	// Get it back
	retrieved, ok := GetTypedMetadata[CVSSScore](f, MetaKeyCVSS)
	if !ok {
		t.Error("Should successfully get struct metadata")
	}

	if retrieved.Version != "3.1" {
		t.Errorf("Expected version 3.1, got %s", retrieved.Version)
	}
	if retrieved.Score != 9.8 {
		t.Errorf("Expected score 9.8, got %f", retrieved.Score)
	}
}

func TestGetTypedMetadata_MapTypes(t *testing.T) {
	type MitreMapping struct {
		Matrix        string   `json:"matrix"`
		TacticID      string   `json:"tactic_id"`
		TacticName    string   `json:"tactic_name"`
		TechniqueID   string   `json:"technique_id"`
		TechniqueName string   `json:"technique_name"`
		SubTechniques []string `json:"sub_techniques,omitempty"`
	}

	f := &Finding{
		ID:        "test-1",
		MissionID: "mission-1",
		AgentName: "test-agent",
		Title:     "Test Finding",
	}

	// Store as map (simulating JSON unmarshaling)
	f.SetMetadata(MetaKeyMitreAttack, map[string]any{
		"matrix":         "enterprise",
		"tactic_id":      "TA0001",
		"tactic_name":    "Initial Access",
		"technique_id":   "T1190",
		"technique_name": "Exploit Public-Facing Application",
		"sub_techniques": []any{"T1190.001"},
	})

	// Retrieve as struct using JSON round-trip
	retrieved, ok := GetTypedMetadata[MitreMapping](f, MetaKeyMitreAttack)
	if !ok {
		t.Error("Should successfully convert map to struct")
	}

	if retrieved.Matrix != "enterprise" {
		t.Errorf("Expected matrix 'enterprise', got %s", retrieved.Matrix)
	}
	if retrieved.TechniqueID != "T1190" {
		t.Errorf("Expected technique T1190, got %s", retrieved.TechniqueID)
	}
	if len(retrieved.SubTechniques) != 1 || retrieved.SubTechniques[0] != "T1190.001" {
		t.Errorf("Expected sub-techniques [T1190.001], got %v", retrieved.SubTechniques)
	}
}

func TestHasMetadata(t *testing.T) {
	f := &Finding{
		ID:        "test-1",
		MissionID: "mission-1",
		AgentName: "test-agent",
		Title:     "Test Finding",
		Metadata: map[string]any{
			MetaKeyCWE: []string{"CWE-79", "CWE-89"},
		},
	}

	if !f.HasMetadata(MetaKeyCWE) {
		t.Error("Should have CWE metadata")
	}

	if f.HasMetadata("nonexistent") {
		t.Error("Should not have nonexistent metadata")
	}

	// Test nil metadata
	f2 := &Finding{
		ID: "test-2",
	}
	if f2.HasMetadata(MetaKeyCWE) {
		t.Error("Should not have metadata when map is nil")
	}
}

func TestDeleteMetadata(t *testing.T) {
	f := &Finding{
		ID:        "test-1",
		MissionID: "mission-1",
		AgentName: "test-agent",
		Title:     "Test Finding",
		Metadata: map[string]any{
			MetaKeyRiskScore: 7.5,
			MetaKeyCWE:       []string{"CWE-79"},
		},
	}

	f.DeleteMetadata(MetaKeyRiskScore)

	if f.HasMetadata(MetaKeyRiskScore) {
		t.Error("Metadata should be deleted")
	}

	if !f.HasMetadata(MetaKeyCWE) {
		t.Error("Other metadata should still exist")
	}
}

func TestGetMetadataKeys(t *testing.T) {
	f := &Finding{
		ID:        "test-1",
		MissionID: "mission-1",
		AgentName: "test-agent",
		Title:     "Test Finding",
		Metadata: map[string]any{
			MetaKeyRiskScore:  7.5,
			MetaKeyCWE:        []string{"CWE-79"},
			MetaKeyCostImpact: 100.0,
		},
	}

	keys := f.GetMetadataKeys()
	if len(keys) != 3 {
		t.Errorf("Expected 3 keys, got %d", len(keys))
	}

	// Check all keys are present
	keyMap := make(map[string]bool)
	for _, k := range keys {
		keyMap[k] = true
	}

	if !keyMap[MetaKeyRiskScore] || !keyMap[MetaKeyCWE] || !keyMap[MetaKeyCostImpact] {
		t.Error("Not all keys were returned")
	}

	// Test nil metadata
	f2 := &Finding{ID: "test-2"}
	keys2 := f2.GetMetadataKeys()
	if keys2 != nil {
		t.Error("Should return nil for nil metadata map")
	}
}

func TestConvenienceHelpers(t *testing.T) {
	f := &Finding{
		ID:        "test-1",
		MissionID: "mission-1",
		AgentName: "test-agent",
		Title:     "Test Finding",
		Metadata: map[string]any{
			"string_val": "test",
			"float_val":  3.14,
			"int_val":    42,
			"bool_val":   true,
			"slice_val":  []string{"a", "b", "c"},
			"wrong_type": 3.14, // Will fail when retrieved as string
		},
	}

	// Test GetStringMetadata
	str, ok := f.GetStringMetadata("string_val")
	if !ok || str != "test" {
		t.Errorf("GetStringMetadata failed: got (%s, %v)", str, ok)
	}

	// Test GetFloat64Metadata
	fl, ok := f.GetFloat64Metadata("float_val")
	if !ok || fl != 3.14 {
		t.Errorf("GetFloat64Metadata failed: got (%f, %v)", fl, ok)
	}

	// Test GetIntMetadata
	i, ok := f.GetIntMetadata("int_val")
	if !ok || i != 42 {
		t.Errorf("GetIntMetadata failed: got (%d, %v)", i, ok)
	}

	// Test GetBoolMetadata
	b, ok := f.GetBoolMetadata("bool_val")
	if !ok || !b {
		t.Errorf("GetBoolMetadata failed: got (%v, %v)", b, ok)
	}

	// Test GetStringSliceMetadata
	slice, ok := f.GetStringSliceMetadata("slice_val")
	if !ok || len(slice) != 3 || slice[0] != "a" {
		t.Errorf("GetStringSliceMetadata failed: got (%v, %v)", slice, ok)
	}

	// Test type mismatch
	_, ok = f.GetStringMetadata("wrong_type")
	if ok {
		t.Error("Should fail on type mismatch")
	}
}

func TestMetadataKeyConstants(t *testing.T) {
	// Test that all constants are defined with expected values
	tests := []struct {
		constant string
		expected string
	}{
		{MetaKeyMitreAttack, "mitre_attack"},
		{MetaKeyMitreAtlas, "mitre_atlas"},
		{MetaKeyCVSS, "cvss"},
		{MetaKeyCWE, "cwe"},
		{MetaKeyRiskScore, "risk_score"},
		{MetaKeyComplianceFramework, "compliance_framework"},
		{MetaKeyComplianceControl, "compliance_control"},
		{MetaKeyCostImpact, "cost_impact"},
		{MetaKeyResourceARN, "resource_arn"},
	}

	for _, tt := range tests {
		if tt.constant != tt.expected {
			t.Errorf("Constant mismatch: expected %s, got %s", tt.expected, tt.constant)
		}
	}
}
