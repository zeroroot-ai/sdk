// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Zero Root AI

package mission

import (
	"bytes"
	"encoding/json"
	"testing"
	"time"

	"github.com/zeroroot-ai/sdk/finding"
)

// TestCreateMissionOpts tests the CreateMissionOpts struct
func TestCreateMissionOpts(t *testing.T) {
	t.Run("empty opts", func(t *testing.T) {
		opts := CreateMissionOpts{}
		if opts.Name != "" {
			t.Errorf("expected empty name, got %s", opts.Name)
		}
		if opts.Constraints != nil {
			t.Error("expected nil constraints")
		}
		if opts.Metadata != nil {
			t.Error("expected nil metadata")
		}
		if opts.Tags != nil {
			t.Error("expected nil tags")
		}
	})

	t.Run("with all fields", func(t *testing.T) {
		constraints := &MissionConstraints{
			MaxDuration: 5 * time.Minute,
			MaxTokens:   1000,
			MaxCost:     10.50,
			MaxFindings: 100,
		}
		opts := CreateMissionOpts{
			Name:        "test-mission",
			Constraints: constraints,
			Metadata: map[string]any{
				"key1": "value1",
				"key2": 42,
			},
			Tags: []string{"tag1", "tag2"},
		}

		if opts.Name != "test-mission" {
			t.Errorf("expected name 'test-mission', got %s", opts.Name)
		}
		if opts.Constraints == nil {
			t.Fatal("expected constraints to be set")
		}
		if opts.Constraints.MaxDuration != 5*time.Minute {
			t.Errorf("expected MaxDuration 5m, got %v", opts.Constraints.MaxDuration)
		}
		if len(opts.Metadata) != 2 {
			t.Errorf("expected 2 metadata entries, got %d", len(opts.Metadata))
		}
		if len(opts.Tags) != 2 {
			t.Errorf("expected 2 tags, got %d", len(opts.Tags))
		}
	})
}

// TestCreateMissionOptsJSON tests JSON marshaling of CreateMissionOpts
func TestCreateMissionOptsJSON(t *testing.T) {
	t.Run("marshal and unmarshal", func(t *testing.T) {
		original := CreateMissionOpts{
			Name: "test-mission",
			Constraints: &MissionConstraints{
				MaxDuration: 10 * time.Minute,
				MaxTokens:   5000,
				MaxCost:     25.75,
				MaxFindings: 50,
			},
			Metadata: map[string]any{
				"environment": "production",
				"priority":    1,
			},
			Tags: []string{"security", "scan"},
		}

		data, err := json.Marshal(original)
		if err != nil {
			t.Fatalf("failed to marshal: %v", err)
		}

		var unmarshaled CreateMissionOpts
		if err := json.Unmarshal(data, &unmarshaled); err != nil {
			t.Fatalf("failed to unmarshal: %v", err)
		}

		if unmarshaled.Name != original.Name {
			t.Errorf("name mismatch: expected %s, got %s", original.Name, unmarshaled.Name)
		}
		if unmarshaled.Constraints.MaxDuration != original.Constraints.MaxDuration {
			t.Errorf("MaxDuration mismatch: expected %v, got %v", original.Constraints.MaxDuration, unmarshaled.Constraints.MaxDuration)
		}
		if len(unmarshaled.Tags) != len(original.Tags) {
			t.Errorf("tags length mismatch: expected %d, got %d", len(original.Tags), len(unmarshaled.Tags))
		}
	})

	t.Run("omitempty fields", func(t *testing.T) {
		opts := CreateMissionOpts{
			Name: "minimal",
		}

		data, err := json.Marshal(opts)
		if err != nil {
			t.Fatalf("failed to marshal: %v", err)
		}

		// Should only contain name field
		jsonStr := string(data)
		if jsonStr != `{"name":"minimal"}` {
			t.Errorf("expected minimal JSON, got %s", jsonStr)
		}
	})
}

// TestMissionConstraints tests the MissionConstraints struct
func TestMissionConstraints(t *testing.T) {
	t.Run("zero values", func(t *testing.T) {
		c := MissionConstraints{}
		if c.MaxDuration != 0 {
			t.Errorf("expected zero MaxDuration, got %v", c.MaxDuration)
		}
		if c.MaxTokens != 0 {
			t.Errorf("expected zero MaxTokens, got %d", c.MaxTokens)
		}
		if c.MaxCost != 0 {
			t.Errorf("expected zero MaxCost, got %f", c.MaxCost)
		}
		if c.MaxFindings != 0 {
			t.Errorf("expected zero MaxFindings, got %d", c.MaxFindings)
		}
	})

	t.Run("all constraints set", func(t *testing.T) {
		c := MissionConstraints{
			MaxDuration: 30 * time.Minute,
			MaxTokens:   100000,
			MaxCost:     50.00,
			MaxFindings: 500,
		}

		if c.MaxDuration != 30*time.Minute {
			t.Errorf("expected MaxDuration 30m, got %v", c.MaxDuration)
		}
		if c.MaxTokens != 100000 {
			t.Errorf("expected MaxTokens 100000, got %d", c.MaxTokens)
		}
		if c.MaxCost != 50.00 {
			t.Errorf("expected MaxCost 50.00, got %f", c.MaxCost)
		}
		if c.MaxFindings != 500 {
			t.Errorf("expected MaxFindings 500, got %d", c.MaxFindings)
		}
	})
}

// TestMissionConstraintsJSON tests JSON marshaling of MissionConstraints
func TestMissionConstraintsJSON(t *testing.T) {
	t.Run("marshal and unmarshal", func(t *testing.T) {
		original := MissionConstraints{
			MaxDuration: 15 * time.Minute,
			MaxTokens:   50000,
			MaxCost:     100.50,
			MaxFindings: 200,
		}

		data, err := json.Marshal(original)
		if err != nil {
			t.Fatalf("failed to marshal: %v", err)
		}

		var unmarshaled MissionConstraints
		if err := json.Unmarshal(data, &unmarshaled); err != nil {
			t.Fatalf("failed to unmarshal: %v", err)
		}

		if unmarshaled.MaxDuration != original.MaxDuration {
			t.Errorf("MaxDuration mismatch: expected %v, got %v", original.MaxDuration, unmarshaled.MaxDuration)
		}
		if unmarshaled.MaxTokens != original.MaxTokens {
			t.Errorf("MaxTokens mismatch: expected %d, got %d", original.MaxTokens, unmarshaled.MaxTokens)
		}
		if unmarshaled.MaxCost != original.MaxCost {
			t.Errorf("MaxCost mismatch: expected %f, got %f", original.MaxCost, unmarshaled.MaxCost)
		}
		if unmarshaled.MaxFindings != original.MaxFindings {
			t.Errorf("MaxFindings mismatch: expected %d, got %d", original.MaxFindings, unmarshaled.MaxFindings)
		}
	})
}

// TestRunMissionOpts tests the RunMissionOpts struct
func TestRunMissionOpts(t *testing.T) {
	t.Run("default opts", func(t *testing.T) {
		opts := RunMissionOpts{}
		if opts.Wait {
			t.Error("expected Wait to be false by default")
		}
		if opts.Timeout != 0 {
			t.Errorf("expected zero Timeout, got %v", opts.Timeout)
		}
	})

	t.Run("with wait and timeout", func(t *testing.T) {
		opts := RunMissionOpts{
			Wait:    true,
			Timeout: 5 * time.Minute,
		}

		if !opts.Wait {
			t.Error("expected Wait to be true")
		}
		if opts.Timeout != 5*time.Minute {
			t.Errorf("expected Timeout 5m, got %v", opts.Timeout)
		}
	})
}

// TestRunMissionOptsJSON tests JSON marshaling of RunMissionOpts
func TestRunMissionOptsJSON(t *testing.T) {
	t.Run("marshal and unmarshal", func(t *testing.T) {
		original := RunMissionOpts{
			Wait:    true,
			Timeout: 10 * time.Minute,
		}

		data, err := json.Marshal(original)
		if err != nil {
			t.Fatalf("failed to marshal: %v", err)
		}

		var unmarshaled RunMissionOpts
		if err := json.Unmarshal(data, &unmarshaled); err != nil {
			t.Fatalf("failed to unmarshal: %v", err)
		}

		if unmarshaled.Wait != original.Wait {
			t.Errorf("Wait mismatch: expected %v, got %v", original.Wait, unmarshaled.Wait)
		}
		if unmarshaled.Timeout != original.Timeout {
			t.Errorf("Timeout mismatch: expected %v, got %v", original.Timeout, unmarshaled.Timeout)
		}
	})
}

// TestMissionInfo tests the MissionInfo struct
func TestMissionInfo(t *testing.T) {
	t.Run("basic fields", func(t *testing.T) {
		now := time.Now()
		info := MissionInfo{
			ID:              "mission-123",
			Name:            "test-mission",
			Status:          MissionStatusPending,
			TargetID:        "target-456",
			ParentMissionID: "parent-789",
			CreatedAt:       now,
			Tags:            []string{"tag1", "tag2"},
		}

		if info.ID != "mission-123" {
			t.Errorf("expected ID 'mission-123', got %s", info.ID)
		}
		if info.Status != MissionStatusPending {
			t.Errorf("expected status pending, got %s", info.Status)
		}
		if info.ParentMissionID != "parent-789" {
			t.Errorf("expected parent ID 'parent-789', got %s", info.ParentMissionID)
		}
	})

	t.Run("root mission", func(t *testing.T) {
		info := MissionInfo{
			ID:              "mission-123",
			Name:            "root-mission",
			Status:          MissionStatusRunning,
			TargetID:        "target-456",
			ParentMissionID: "",
			CreatedAt:       time.Now(),
		}

		if info.ParentMissionID != "" {
			t.Error("root mission should have empty ParentMissionID")
		}
	})
}

// TestMissionInfoJSON tests JSON marshaling of MissionInfo
func TestMissionInfoJSON(t *testing.T) {
	t.Run("marshal and unmarshal", func(t *testing.T) {
		now := time.Now().Truncate(time.Second) // Truncate for JSON comparison
		original := MissionInfo{
			ID:              "mission-abc",
			Name:            "json-test",
			Status:          MissionStatusRunning,
			TargetID:        "target-xyz",
			ParentMissionID: "parent-123",
			CreatedAt:       now,
			Tags:            []string{"security", "test"},
		}

		data, err := json.Marshal(original)
		if err != nil {
			t.Fatalf("failed to marshal: %v", err)
		}

		var unmarshaled MissionInfo
		if err := json.Unmarshal(data, &unmarshaled); err != nil {
			t.Fatalf("failed to unmarshal: %v", err)
		}

		if unmarshaled.ID != original.ID {
			t.Errorf("ID mismatch: expected %s, got %s", original.ID, unmarshaled.ID)
		}
		if unmarshaled.Status != original.Status {
			t.Errorf("Status mismatch: expected %s, got %s", original.Status, unmarshaled.Status)
		}
		if !unmarshaled.CreatedAt.Equal(original.CreatedAt) {
			t.Errorf("CreatedAt mismatch: expected %v, got %v", original.CreatedAt, unmarshaled.CreatedAt)
		}
	})
}

// TestMissionStatus tests the MissionStatus type and its methods
func TestMissionStatus(t *testing.T) {
	tests := []struct {
		name      string
		status    MissionStatus
		wantValid bool
		wantTerm  bool
	}{
		{"pending", MissionStatusPending, true, false},
		{"running", MissionStatusRunning, true, false},
		{"paused", MissionStatusPaused, true, false},
		{"completed", MissionStatusCompleted, true, true},
		{"failed", MissionStatusFailed, true, true},
		{"cancelled", MissionStatusCancelled, true, true},
		{"invalid", MissionStatus("invalid"), false, false},
		{"empty", MissionStatus(""), false, false},
		{"unknown", MissionStatus("unknown-status"), false, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.status.IsValid(); got != tt.wantValid {
				t.Errorf("IsValid() = %v, want %v", got, tt.wantValid)
			}
			if got := tt.status.IsTerminal(); got != tt.wantTerm {
				t.Errorf("IsTerminal() = %v, want %v", got, tt.wantTerm)
			}
		})
	}
}

// TestMissionStatusJSON tests JSON marshaling of MissionStatus
func TestMissionStatusJSON(t *testing.T) {
	t.Run("marshal valid status", func(t *testing.T) {
		status := MissionStatusRunning
		data, err := json.Marshal(status)
		if err != nil {
			t.Fatalf("failed to marshal: %v", err)
		}

		expected := `"running"`
		if string(data) != expected {
			t.Errorf("expected %s, got %s", expected, string(data))
		}

		var unmarshaled MissionStatus
		if err := json.Unmarshal(data, &unmarshaled); err != nil {
			t.Fatalf("failed to unmarshal: %v", err)
		}

		if unmarshaled != status {
			t.Errorf("status mismatch: expected %s, got %s", status, unmarshaled)
		}
	})
}

// TestMissionStatusInfo tests the MissionStatusInfo struct
func TestMissionStatusInfo(t *testing.T) {
	t.Run("in-progress mission", func(t *testing.T) {
		info := MissionStatusInfo{
			Status:   MissionStatusRunning,
			Progress: 0.45,
			Phase:    "reconnaissance",
			FindingCounts: map[string]int{
				"critical": 2,
				"high":     5,
				"medium":   10,
			},
			TokenUsage: 5000,
			Duration:   2 * time.Minute,
		}

		if info.Status != MissionStatusRunning {
			t.Errorf("expected running status, got %s", info.Status)
		}
		if info.Progress != 0.45 {
			t.Errorf("expected progress 0.45, got %f", info.Progress)
		}
		if info.FindingCounts["critical"] != 2 {
			t.Errorf("expected 2 critical findings, got %d", info.FindingCounts["critical"])
		}
	})

	t.Run("failed mission with error", func(t *testing.T) {
		info := MissionStatusInfo{
			Status:     MissionStatusFailed,
			Progress:   0.25,
			Phase:      "exploitation",
			TokenUsage: 1000,
			Duration:   30 * time.Second,
			Error:      "connection timeout",
		}

		if info.Status != MissionStatusFailed {
			t.Errorf("expected failed status, got %s", info.Status)
		}
		if info.Error != "connection timeout" {
			t.Errorf("expected error message, got %s", info.Error)
		}
	})
}

// TestMissionStatusInfoJSON tests JSON marshaling of MissionStatusInfo
func TestMissionStatusInfoJSON(t *testing.T) {
	t.Run("marshal and unmarshal", func(t *testing.T) {
		original := MissionStatusInfo{
			Status:   MissionStatusRunning,
			Progress: 0.67,
			Phase:    "scanning",
			FindingCounts: map[string]int{
				"critical": 1,
				"high":     3,
			},
			TokenUsage: 10000,
			Duration:   5 * time.Minute,
		}

		data, err := json.Marshal(original)
		if err != nil {
			t.Fatalf("failed to marshal: %v", err)
		}

		var unmarshaled MissionStatusInfo
		if err := json.Unmarshal(data, &unmarshaled); err != nil {
			t.Fatalf("failed to unmarshal: %v", err)
		}

		if unmarshaled.Status != original.Status {
			t.Errorf("Status mismatch: expected %s, got %s", original.Status, unmarshaled.Status)
		}
		if unmarshaled.Progress != original.Progress {
			t.Errorf("Progress mismatch: expected %f, got %f", original.Progress, unmarshaled.Progress)
		}
		if unmarshaled.TokenUsage != original.TokenUsage {
			t.Errorf("TokenUsage mismatch: expected %d, got %d", original.TokenUsage, unmarshaled.TokenUsage)
		}
	})
}

// TestMissionResult tests the MissionResult struct
func TestMissionResult(t *testing.T) {
	t.Run("successful mission with findings", func(t *testing.T) {
		now := time.Now()
		result := MissionResult{
			MissionID: "mission-123",
			Status:    MissionStatusCompleted,
			Findings: []finding.Finding{
				{
					Title:    "SQL Injection",
					Severity: "critical",
				},
			},
			Output: map[string]any{
				"scanned_hosts": 10,
				"open_ports":    []int{80, 443, 8080},
			},
			Metrics: MissionMetrics{
				Duration:      10 * time.Minute,
				TokensUsed:    50000,
				ToolCalls:     25,
				AgentCalls:    3,
				FindingsCount: 1,
			},
			CompletedAt: now,
		}

		if result.MissionID != "mission-123" {
			t.Errorf("expected mission ID 'mission-123', got %s", result.MissionID)
		}
		if len(result.Findings) != 1 {
			t.Errorf("expected 1 finding, got %d", len(result.Findings))
		}
		if result.Metrics.FindingsCount != 1 {
			t.Errorf("expected 1 in metrics, got %d", result.Metrics.FindingsCount)
		}
	})

	t.Run("failed mission with error", func(t *testing.T) {
		result := MissionResult{
			MissionID: "mission-456",
			Status:    MissionStatusFailed,
			Metrics: MissionMetrics{
				Duration:   1 * time.Minute,
				TokensUsed: 100,
			},
			Error:       "target unreachable",
			CompletedAt: time.Now(),
		}

		if result.Status != MissionStatusFailed {
			t.Errorf("expected failed status, got %s", result.Status)
		}
		if result.Error != "target unreachable" {
			t.Errorf("expected error message, got %s", result.Error)
		}
	})
}

// TestMissionResultJSON tests JSON marshaling of MissionResult
func TestMissionResultJSON(t *testing.T) {
	t.Run("marshal and unmarshal", func(t *testing.T) {
		now := time.Now().Truncate(time.Second)
		original := MissionResult{
			MissionID: "mission-xyz",
			Status:    MissionStatusCompleted,
			Findings: []finding.Finding{
				{
					Title:       "XSS Vulnerability",
					Severity:    "high",
					Description: "Cross-site scripting found",
				},
			},
			Output: map[string]any{
				"test_result": "passed",
				"score":       95.5,
			},
			Metrics: MissionMetrics{
				Duration:      15 * time.Minute,
				TokensUsed:    75000,
				ToolCalls:     40,
				AgentCalls:    5,
				FindingsCount: 1,
			},
			CompletedAt: now,
		}

		data, err := json.Marshal(original)
		if err != nil {
			t.Fatalf("failed to marshal: %v", err)
		}

		var unmarshaled MissionResult
		if err := json.Unmarshal(data, &unmarshaled); err != nil {
			t.Fatalf("failed to unmarshal: %v", err)
		}

		if unmarshaled.MissionID != original.MissionID {
			t.Errorf("MissionID mismatch: expected %s, got %s", original.MissionID, unmarshaled.MissionID)
		}
		if len(unmarshaled.Findings) != len(original.Findings) {
			t.Errorf("Findings length mismatch: expected %d, got %d", len(original.Findings), len(unmarshaled.Findings))
		}
		if unmarshaled.Metrics.TokensUsed != original.Metrics.TokensUsed {
			t.Errorf("TokensUsed mismatch: expected %d, got %d", original.Metrics.TokensUsed, unmarshaled.Metrics.TokensUsed)
		}
	})
}

// TestMissionMetrics tests the MissionMetrics struct
func TestMissionMetrics(t *testing.T) {
	t.Run("typical metrics", func(t *testing.T) {
		metrics := MissionMetrics{
			Duration:      30 * time.Minute,
			TokensUsed:    100000,
			ToolCalls:     50,
			AgentCalls:    10,
			FindingsCount: 25,
		}

		if metrics.Duration != 30*time.Minute {
			t.Errorf("expected Duration 30m, got %v", metrics.Duration)
		}
		if metrics.TokensUsed != 100000 {
			t.Errorf("expected TokensUsed 100000, got %d", metrics.TokensUsed)
		}
		if metrics.ToolCalls != 50 {
			t.Errorf("expected ToolCalls 50, got %d", metrics.ToolCalls)
		}
		if metrics.AgentCalls != 10 {
			t.Errorf("expected AgentCalls 10, got %d", metrics.AgentCalls)
		}
		if metrics.FindingsCount != 25 {
			t.Errorf("expected FindingsCount 25, got %d", metrics.FindingsCount)
		}
	})

	t.Run("zero metrics", func(t *testing.T) {
		metrics := MissionMetrics{}
		if metrics.TokensUsed != 0 {
			t.Errorf("expected zero TokensUsed, got %d", metrics.TokensUsed)
		}
		if metrics.FindingsCount != 0 {
			t.Errorf("expected zero FindingsCount, got %d", metrics.FindingsCount)
		}
	})
}

// TestMissionMetricsJSON tests JSON marshaling of MissionMetrics
func TestMissionMetricsJSON(t *testing.T) {
	t.Run("marshal and unmarshal", func(t *testing.T) {
		original := MissionMetrics{
			Duration:      45 * time.Minute,
			TokensUsed:    200000,
			ToolCalls:     100,
			AgentCalls:    20,
			FindingsCount: 50,
		}

		data, err := json.Marshal(original)
		if err != nil {
			t.Fatalf("failed to marshal: %v", err)
		}

		var unmarshaled MissionMetrics
		if err := json.Unmarshal(data, &unmarshaled); err != nil {
			t.Fatalf("failed to unmarshal: %v", err)
		}

		if unmarshaled.Duration != original.Duration {
			t.Errorf("Duration mismatch: expected %v, got %v", original.Duration, unmarshaled.Duration)
		}
		if unmarshaled.TokensUsed != original.TokensUsed {
			t.Errorf("TokensUsed mismatch: expected %d, got %d", original.TokensUsed, unmarshaled.TokensUsed)
		}
		if unmarshaled.ToolCalls != original.ToolCalls {
			t.Errorf("ToolCalls mismatch: expected %d, got %d", original.ToolCalls, unmarshaled.ToolCalls)
		}
		if unmarshaled.AgentCalls != original.AgentCalls {
			t.Errorf("AgentCalls mismatch: expected %d, got %d", original.AgentCalls, unmarshaled.AgentCalls)
		}
		if unmarshaled.FindingsCount != original.FindingsCount {
			t.Errorf("FindingsCount mismatch: expected %d, got %d", original.FindingsCount, unmarshaled.FindingsCount)
		}
	})
}

// TestMissionFilter tests the MissionFilter struct
func TestMissionFilter(t *testing.T) {
	t.Run("empty filter", func(t *testing.T) {
		filter := MissionFilter{}
		if filter.Status != nil {
			t.Error("expected nil Status")
		}
		if filter.TargetID != nil {
			t.Error("expected nil TargetID")
		}
		if filter.Limit != 0 {
			t.Error("expected zero Limit")
		}
	})

	t.Run("filter with all fields", func(t *testing.T) {
		status := MissionStatusCompleted
		targetID := "target-123"
		parentID := "parent-456"
		createdAfter := time.Now().Add(-24 * time.Hour)
		createdBefore := time.Now()

		filter := MissionFilter{
			Status:          &status,
			TargetID:        &targetID,
			ParentMissionID: &parentID,
			CreatedAfter:    &createdAfter,
			CreatedBefore:   &createdBefore,
			Tags:            []string{"security", "compliance"},
			Limit:           100,
			Offset:          10,
		}

		if filter.Status == nil || *filter.Status != MissionStatusCompleted {
			t.Error("Status not set correctly")
		}
		if filter.TargetID == nil || *filter.TargetID != "target-123" {
			t.Error("TargetID not set correctly")
		}
		if filter.ParentMissionID == nil || *filter.ParentMissionID != "parent-456" {
			t.Error("ParentMissionID not set correctly")
		}
		if len(filter.Tags) != 2 {
			t.Errorf("expected 2 tags, got %d", len(filter.Tags))
		}
		if filter.Limit != 100 {
			t.Errorf("expected Limit 100, got %d", filter.Limit)
		}
		if filter.Offset != 10 {
			t.Errorf("expected Offset 10, got %d", filter.Offset)
		}
	})

	t.Run("filter for child missions", func(t *testing.T) {
		parentID := "parent-abc"
		filter := MissionFilter{
			ParentMissionID: &parentID,
		}

		if filter.ParentMissionID == nil || *filter.ParentMissionID != "parent-abc" {
			t.Error("ParentMissionID filter not set correctly")
		}
	})
}

// TestMissionFilterJSON tests JSON marshaling of MissionFilter
func TestMissionFilterJSON(t *testing.T) {
	t.Run("marshal and unmarshal", func(t *testing.T) {
		status := MissionStatusRunning
		targetID := "target-xyz"
		parentID := "parent-abc"
		now := time.Now().Truncate(time.Second)
		after := now.Add(-1 * time.Hour)

		original := MissionFilter{
			Status:          &status,
			TargetID:        &targetID,
			ParentMissionID: &parentID,
			CreatedAfter:    &after,
			CreatedBefore:   &now,
			Tags:            []string{"test"},
			Limit:           50,
			Offset:          5,
		}

		data, err := json.Marshal(original)
		if err != nil {
			t.Fatalf("failed to marshal: %v", err)
		}

		var unmarshaled MissionFilter
		if err := json.Unmarshal(data, &unmarshaled); err != nil {
			t.Fatalf("failed to unmarshal: %v", err)
		}

		if unmarshaled.Status == nil || *unmarshaled.Status != status {
			t.Error("Status mismatch")
		}
		if unmarshaled.TargetID == nil || *unmarshaled.TargetID != targetID {
			t.Error("TargetID mismatch")
		}
		if unmarshaled.Limit != original.Limit {
			t.Errorf("Limit mismatch: expected %d, got %d", original.Limit, unmarshaled.Limit)
		}
	})

	t.Run("empty filter omits fields", func(t *testing.T) {
		filter := MissionFilter{
			Limit: 10,
		}

		data, err := json.Marshal(filter)
		if err != nil {
			t.Fatalf("failed to marshal: %v", err)
		}

		// Should only contain limit
		jsonStr := string(data)
		if jsonStr != `{"limit":10}` {
			t.Errorf("expected minimal JSON, got %s", jsonStr)
		}
	})
}

// TestJSONRoundTrip tests complete JSON round-trip for all types
func TestJSONRoundTrip(t *testing.T) {
	t.Run("all types round-trip", func(t *testing.T) {
		// Create instances of all types
		types := []any{
			CreateMissionOpts{
				Name: "test",
				Constraints: &MissionConstraints{
					MaxDuration: time.Hour,
				},
			},
			MissionConstraints{
				MaxTokens: 1000,
			},
			RunMissionOpts{
				Wait:    true,
				Timeout: time.Minute,
			},
			MissionInfo{
				ID:        "id",
				Status:    MissionStatusRunning,
				CreatedAt: time.Now().Truncate(time.Second),
			},
			MissionStatusInfo{
				Status:   MissionStatusCompleted,
				Progress: 1.0,
			},
			MissionResult{
				MissionID:   "id",
				Status:      MissionStatusCompleted,
				CompletedAt: time.Now().Truncate(time.Second),
			},
			MissionMetrics{
				Duration:   time.Hour,
				TokensUsed: 5000,
			},
			MissionFilter{
				Limit: 100,
			},
		}

		for i, original := range types {
			data, err := json.Marshal(original)
			if err != nil {
				t.Errorf("test %d: failed to marshal: %v", i, err)
				continue
			}

			// Create new instance of same type
			var unmarshaled any
			switch original.(type) {
			case CreateMissionOpts:
				unmarshaled = &CreateMissionOpts{}
			case MissionConstraints:
				unmarshaled = &MissionConstraints{}
			case RunMissionOpts:
				unmarshaled = &RunMissionOpts{}
			case MissionInfo:
				unmarshaled = &MissionInfo{}
			case MissionStatusInfo:
				unmarshaled = &MissionStatusInfo{}
			case MissionResult:
				unmarshaled = &MissionResult{}
			case MissionMetrics:
				unmarshaled = &MissionMetrics{}
			case MissionFilter:
				unmarshaled = &MissionFilter{}
			}

			if err := json.Unmarshal(data, unmarshaled); err != nil {
				t.Errorf("test %d: failed to unmarshal: %v", i, err)
			}
		}
	})
}

// TestEdgeCases tests edge cases and boundary conditions
func TestEdgeCases(t *testing.T) {
	t.Run("negative values in constraints", func(t *testing.T) {
		// Negative values are technically allowed, but may have special meaning
		c := MissionConstraints{
			MaxDuration: -1 * time.Hour,
			MaxTokens:   -1000,
			MaxCost:     -50.0,
			MaxFindings: -10,
		}

		data, err := json.Marshal(c)
		if err != nil {
			t.Fatalf("failed to marshal: %v", err)
		}

		var unmarshaled MissionConstraints
		if err := json.Unmarshal(data, &unmarshaled); err != nil {
			t.Fatalf("failed to unmarshal: %v", err)
		}

		if unmarshaled.MaxTokens != c.MaxTokens {
			t.Errorf("MaxTokens mismatch: expected %d, got %d", c.MaxTokens, unmarshaled.MaxTokens)
		}
	})

	t.Run("very large values", func(t *testing.T) {
		c := MissionConstraints{
			MaxDuration: 1000000 * time.Hour,
			MaxTokens:   9223372036854775807, // Max int64
			MaxCost:     999999999.99,
			MaxFindings: 2147483647, // Max int32
		}

		data, err := json.Marshal(c)
		if err != nil {
			t.Fatalf("failed to marshal: %v", err)
		}

		var unmarshaled MissionConstraints
		if err := json.Unmarshal(data, &unmarshaled); err != nil {
			t.Fatalf("failed to unmarshal: %v", err)
		}

		if unmarshaled.MaxTokens != c.MaxTokens {
			t.Errorf("MaxTokens mismatch: expected %d, got %d", c.MaxTokens, unmarshaled.MaxTokens)
		}
	})

	t.Run("empty strings in filter", func(t *testing.T) {
		emptyStr := ""
		filter := MissionFilter{
			TargetID:        &emptyStr,
			ParentMissionID: &emptyStr,
		}

		if filter.TargetID == nil || *filter.TargetID != "" {
			t.Error("empty TargetID should be preserved")
		}
	})

	t.Run("nil slices vs empty slices", func(t *testing.T) {
		opts1 := CreateMissionOpts{
			Tags: nil,
		}
		opts2 := CreateMissionOpts{
			Tags: []string{},
		}

		// Both should be treated as omitempty
		data1, _ := json.Marshal(opts1)
		data2, _ := json.Marshal(opts2)

		if !bytes.Equal(data1, data2) {
			t.Errorf("nil and empty slices should marshal identically: %s vs %s", data1, data2)
		}
	})

	t.Run("progress edge cases", func(t *testing.T) {
		tests := []struct {
			name     string
			progress float64
		}{
			{"zero", 0.0},
			{"complete", 1.0},
			{"over", 1.5},
			{"negative", -0.5},
			{"very small", 0.0001},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				info := MissionStatusInfo{
					Status:   MissionStatusRunning,
					Progress: tt.progress,
				}

				data, err := json.Marshal(info)
				if err != nil {
					t.Fatalf("failed to marshal: %v", err)
				}

				var unmarshaled MissionStatusInfo
				if err := json.Unmarshal(data, &unmarshaled); err != nil {
					t.Fatalf("failed to unmarshal: %v", err)
				}

				if unmarshaled.Progress != info.Progress {
					t.Errorf("Progress mismatch: expected %f, got %f", info.Progress, unmarshaled.Progress)
				}
			})
		}
	})
}
