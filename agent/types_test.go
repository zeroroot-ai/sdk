// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Zero Root AI

package agent

import (
	"errors"
	"testing"
)

func TestResultStatus(t *testing.T) {
	tests := []struct {
		name       string
		status     ResultStatus
		wantValid  bool
		wantString string
	}{
		{"success", StatusSuccess, true, "success"},
		{"failed", StatusFailed, true, "failed"},
		{"partial", StatusPartial, true, "partial"},
		{"cancelled", StatusCancelled, true, "cancelled"},
		{"timeout", StatusTimeout, true, "timeout"},
		{"invalid", ResultStatus("invalid"), false, "invalid"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.status.IsValid(); got != tt.wantValid {
				t.Errorf("IsValid() = %v, want %v", got, tt.wantValid)
			}
			if got := tt.status.String(); got != tt.wantString {
				t.Errorf("String() = %v, want %v", got, tt.wantString)
			}
		})
	}
}

func TestResultStatus_IsTerminal(t *testing.T) {
	statuses := []ResultStatus{
		StatusSuccess, StatusFailed, StatusPartial, StatusCancelled, StatusTimeout,
	}

	for _, status := range statuses {
		t.Run(status.String(), func(t *testing.T) {
			if !status.IsTerminal() {
				t.Errorf("expected %s to be terminal", status)
			}
		})
	}
}

func TestResultStatus_IsSuccessful(t *testing.T) {
	tests := []struct {
		status ResultStatus
		want   bool
	}{
		{StatusSuccess, true},
		{StatusPartial, true},
		{StatusFailed, false},
		{StatusCancelled, false},
		{StatusTimeout, false},
	}

	for _, tt := range tests {
		t.Run(tt.status.String(), func(t *testing.T) {
			if got := tt.status.IsSuccessful(); got != tt.want {
				t.Errorf("IsSuccessful() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestNewTask(t *testing.T) {
	task := NewTask("task-1")

	if task.ID != "task-1" {
		t.Errorf("ID = %v, want task-1", task.ID)
	}
	if task.Context == nil {
		t.Error("Context should be initialized")
	}
	if task.Metadata == nil {
		t.Error("Metadata should be initialized")
	}
}

func TestTask_Context(t *testing.T) {
	task := NewTask("task-1")

	// Test SetContext and GetContext
	task.SetContext("key1", "value1")
	task.SetContext("key2", 42)

	val, ok := task.GetContext("key1")
	if !ok || val != "value1" {
		t.Errorf("GetContext(key1) = %v, %v, want 'value1', true", val, ok)
	}

	val, ok = task.GetContext("key2")
	if !ok || val != 42 {
		t.Errorf("GetContext(key2) = %v, %v, want 42, true", val, ok)
	}

	val, ok = task.GetContext("nonexistent")
	if ok {
		t.Errorf("GetContext(nonexistent) = %v, %v, want nil, false", val, ok)
	}
}

func TestTask_Metadata(t *testing.T) {
	task := NewTask("task-1")

	// Test SetMetadata and GetMetadata
	task.SetMetadata("priority", "high")
	task.SetMetadata("timeout", 300)

	val, ok := task.GetMetadata("priority")
	if !ok || val != "high" {
		t.Errorf("GetMetadata(priority) = %v, %v, want 'high', true", val, ok)
	}

	val, ok = task.GetMetadata("timeout")
	if !ok || val != 300 {
		t.Errorf("GetMetadata(timeout) = %v, %v, want 300, true", val, ok)
	}

	val, ok = task.GetMetadata("nonexistent")
	if ok {
		t.Errorf("GetMetadata(nonexistent) = %v, %v, want nil, false", val, ok)
	}
}

func TestTaskConstraints_IsToolAllowed(t *testing.T) {
	tests := []struct {
		name        string
		constraints TaskConstraints
		toolName    string
		want        bool
	}{
		{
			name:        "no restrictions",
			constraints: TaskConstraints{},
			toolName:    "any-tool",
			want:        true,
		},
		{
			name: "allowed tool",
			constraints: TaskConstraints{
				AllowedTools: []string{"tool1", "tool2"},
			},
			toolName: "tool1",
			want:     true,
		},
		{
			name: "not in allowed list",
			constraints: TaskConstraints{
				AllowedTools: []string{"tool1", "tool2"},
			},
			toolName: "tool3",
			want:     false,
		},
		{
			name: "blocked tool",
			constraints: TaskConstraints{
				BlockedTools: []string{"blocked"},
			},
			toolName: "blocked",
			want:     false,
		},
		{
			name: "blocked takes precedence",
			constraints: TaskConstraints{
				AllowedTools: []string{"tool1", "tool2"},
				BlockedTools: []string{"tool1"},
			},
			toolName: "tool1",
			want:     false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.constraints.IsToolAllowed(tt.toolName); got != tt.want {
				t.Errorf("IsToolAllowed(%s) = %v, want %v", tt.toolName, got, tt.want)
			}
		})
	}
}

func TestTaskConstraints_HasLimits(t *testing.T) {
	tests := []struct {
		name           string
		constraints    TaskConstraints
		wantTurnLimit  bool
		wantTokenLimit bool
	}{
		{
			name:           "no limits",
			constraints:    TaskConstraints{},
			wantTurnLimit:  false,
			wantTokenLimit: false,
		},
		{
			name: "turn limit only",
			constraints: TaskConstraints{
				MaxTurns: 10,
			},
			wantTurnLimit:  true,
			wantTokenLimit: false,
		},
		{
			name: "token limit only",
			constraints: TaskConstraints{
				MaxTokens: 1000,
			},
			wantTurnLimit:  false,
			wantTokenLimit: true,
		},
		{
			name: "both limits",
			constraints: TaskConstraints{
				MaxTurns:  10,
				MaxTokens: 1000,
			},
			wantTurnLimit:  true,
			wantTokenLimit: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.constraints.HasTurnLimit(); got != tt.wantTurnLimit {
				t.Errorf("HasTurnLimit() = %v, want %v", got, tt.wantTurnLimit)
			}
			if got := tt.constraints.HasTokenLimit(); got != tt.wantTokenLimit {
				t.Errorf("HasTokenLimit() = %v, want %v", got, tt.wantTokenLimit)
			}
		})
	}
}

func TestNewSuccessResult(t *testing.T) {
	output := map[string]any{"result": "success"}
	result := NewSuccessResult(output)

	if result.Status != StatusSuccess {
		t.Errorf("Status = %v, want %v", result.Status, StatusSuccess)
	}
	if result.Output == nil {
		t.Error("Output should not be nil")
	}
	if result.Findings == nil {
		t.Error("Findings should be initialized")
	}
	if result.Metadata == nil {
		t.Error("Metadata should be initialized")
	}
	if result.Error != nil {
		t.Errorf("Error should be nil, got %v", result.Error)
	}
}

func TestNewFailedResult(t *testing.T) {
	err := errors.New("test error")
	result := NewFailedResult(err)

	if result.Status != StatusFailed {
		t.Errorf("Status = %v, want %v", result.Status, StatusFailed)
	}
	if !errors.Is(result.Error, err) {
		t.Errorf("Error = %v, want %v", result.Error, err)
	}
	if result.Findings == nil {
		t.Error("Findings should be initialized")
	}
	if result.Metadata == nil {
		t.Error("Metadata should be initialized")
	}
}

func TestNewPartialResult(t *testing.T) {
	output := "partial output"
	err := errors.New("partial error")
	result := NewPartialResult(output, err)

	if result.Status != StatusPartial {
		t.Errorf("Status = %v, want %v", result.Status, StatusPartial)
	}
	if result.Output != output {
		t.Errorf("Output = %v, want %v", result.Output, output)
	}
	if !errors.Is(result.Error, err) {
		t.Errorf("Error = %v, want %v", result.Error, err)
	}
}

func TestNewCancelledResult(t *testing.T) {
	result := NewCancelledResult()

	if result.Status != StatusCancelled {
		t.Errorf("Status = %v, want %v", result.Status, StatusCancelled)
	}
	if result.Findings == nil {
		t.Error("Findings should be initialized")
	}
	if result.Metadata == nil {
		t.Error("Metadata should be initialized")
	}
}

func TestNewTimeoutResult(t *testing.T) {
	result := NewTimeoutResult()

	if result.Status != StatusTimeout {
		t.Errorf("Status = %v, want %v", result.Status, StatusTimeout)
	}
	if result.Findings == nil {
		t.Error("Findings should be initialized")
	}
	if result.Metadata == nil {
		t.Error("Metadata should be initialized")
	}
}

func TestResult_AddFinding(t *testing.T) {
	result := NewSuccessResult(nil)

	result.AddFinding("finding-1")
	result.AddFinding("finding-2")

	if len(result.Findings) != 2 {
		t.Errorf("len(Findings) = %d, want 2", len(result.Findings))
	}
	if result.Findings[0] != "finding-1" {
		t.Errorf("Findings[0] = %s, want finding-1", result.Findings[0])
	}
	if result.Findings[1] != "finding-2" {
		t.Errorf("Findings[1] = %s, want finding-2", result.Findings[1])
	}
}

func TestResult_Metadata(t *testing.T) {
	result := NewSuccessResult(nil)

	// Test SetMetadata and GetMetadata
	result.SetMetadata("execution_time", 1.5)
	result.SetMetadata("turns", 5)

	val, ok := result.GetMetadata("execution_time")
	if !ok || val != 1.5 {
		t.Errorf("GetMetadata(execution_time) = %v, %v, want 1.5, true", val, ok)
	}

	val, ok = result.GetMetadata("turns")
	if !ok || val != 5 {
		t.Errorf("GetMetadata(turns) = %v, %v, want 5, true", val, ok)
	}

	val, ok = result.GetMetadata("nonexistent")
	if ok {
		t.Errorf("GetMetadata(nonexistent) = %v, %v, want nil, false", val, ok)
	}
}

func TestResult_NilFindings(t *testing.T) {
	// Test AddFinding when Findings is nil
	result := Result{Status: StatusSuccess}

	result.AddFinding("finding-1")

	if result.Findings == nil {
		t.Error("Findings should be initialized by AddFinding")
	}
	if len(result.Findings) != 1 {
		t.Errorf("len(Findings) = %d, want 1", len(result.Findings))
	}
}
