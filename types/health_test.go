// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Zero Root AI

package types

import (
	"encoding/json"
	"testing"
)

func TestHealthStatus_IsHealthy(t *testing.T) {
	tests := []struct {
		name   string
		status HealthStatus
		want   bool
	}{
		{
			name:   "healthy status",
			status: HealthStatus{Status: StatusHealthy},
			want:   true,
		},
		{
			name:   "degraded status",
			status: HealthStatus{Status: StatusDegraded},
			want:   false,
		},
		{
			name:   "unhealthy status",
			status: HealthStatus{Status: StatusUnhealthy},
			want:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.status.IsHealthy(); got != tt.want {
				t.Errorf("IsHealthy() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestHealthStatus_IsDegraded(t *testing.T) {
	tests := []struct {
		name   string
		status HealthStatus
		want   bool
	}{
		{
			name:   "healthy status",
			status: HealthStatus{Status: StatusHealthy},
			want:   false,
		},
		{
			name:   "degraded status",
			status: HealthStatus{Status: StatusDegraded},
			want:   true,
		},
		{
			name:   "unhealthy status",
			status: HealthStatus{Status: StatusUnhealthy},
			want:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.status.IsDegraded(); got != tt.want {
				t.Errorf("IsDegraded() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestHealthStatus_IsUnhealthy(t *testing.T) {
	tests := []struct {
		name   string
		status HealthStatus
		want   bool
	}{
		{
			name:   "healthy status",
			status: HealthStatus{Status: StatusHealthy},
			want:   false,
		},
		{
			name:   "degraded status",
			status: HealthStatus{Status: StatusDegraded},
			want:   false,
		},
		{
			name:   "unhealthy status",
			status: HealthStatus{Status: StatusUnhealthy},
			want:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.status.IsUnhealthy(); got != tt.want {
				t.Errorf("IsUnhealthy() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestNewHealthyStatus(t *testing.T) {
	status := NewHealthyStatus("all systems operational")

	if status.Status != StatusHealthy {
		t.Errorf("Status = %v, want %v", status.Status, StatusHealthy)
	}

	if status.Message != "all systems operational" {
		t.Errorf("Message = %v, want %v", status.Message, "all systems operational")
	}

	if status.Details != nil {
		t.Errorf("Details should be nil, got %v", status.Details)
	}
}

func TestNewDegradedStatus(t *testing.T) {
	details := map[string]any{
		"latency": "high",
		"errors":  5,
	}

	status := NewDegradedStatus("high latency detected", details)

	if status.Status != StatusDegraded {
		t.Errorf("Status = %v, want %v", status.Status, StatusDegraded)
	}

	if status.Message != "high latency detected" {
		t.Errorf("Message = %v, want %v", status.Message, "high latency detected")
	}

	if status.Details == nil {
		t.Fatal("Details should not be nil")
	}

	if status.Details["latency"] != "high" {
		t.Errorf("Details[latency] = %v, want %v", status.Details["latency"], "high")
	}

	if status.Details["errors"] != 5 {
		t.Errorf("Details[errors] = %v, want %v", status.Details["errors"], 5)
	}
}

func TestNewUnhealthyStatus(t *testing.T) {
	details := map[string]any{
		"error": "connection refused",
		"code":  "ECONNREFUSED",
	}

	status := NewUnhealthyStatus("cannot connect to database", details)

	if status.Status != StatusUnhealthy {
		t.Errorf("Status = %v, want %v", status.Status, StatusUnhealthy)
	}

	if status.Message != "cannot connect to database" {
		t.Errorf("Message = %v, want %v", status.Message, "cannot connect to database")
	}

	if status.Details == nil {
		t.Fatal("Details should not be nil")
	}

	if status.Details["error"] != "connection refused" {
		t.Errorf("Details[error] = %v, want %v", status.Details["error"], "connection refused")
	}
}

func TestHealthStatus_JSONMarshaling(t *testing.T) {
	original := HealthStatus{
		Status:  StatusDegraded,
		Message: "test message",
		Details: map[string]any{
			"key1": "value1",
			"key2": 42,
		},
	}

	// Marshal to JSON
	data, err := json.Marshal(original)
	if err != nil {
		t.Fatalf("Failed to marshal: %v", err)
	}

	// Unmarshal back
	var unmarshaled HealthStatus
	if err := json.Unmarshal(data, &unmarshaled); err != nil {
		t.Fatalf("Failed to unmarshal: %v", err)
	}

	// Verify fields
	if unmarshaled.Status != original.Status {
		t.Errorf("Status = %v, want %v", unmarshaled.Status, original.Status)
	}

	if unmarshaled.Message != original.Message {
		t.Errorf("Message = %v, want %v", unmarshaled.Message, original.Message)
	}

	if unmarshaled.Details["key1"] != "value1" {
		t.Errorf("Details[key1] = %v, want %v", unmarshaled.Details["key1"], "value1")
	}

	// Note: JSON unmarshaling converts numbers to float64
	if unmarshaled.Details["key2"] != float64(42) {
		t.Errorf("Details[key2] = %v, want %v", unmarshaled.Details["key2"], 42)
	}
}

func TestHealthStatusConstants(t *testing.T) {
	// Verify constants have expected values
	if StatusHealthy != "healthy" {
		t.Errorf("StatusHealthy = %v, want %v", StatusHealthy, "healthy")
	}

	if StatusDegraded != "degraded" {
		t.Errorf("StatusDegraded = %v, want %v", StatusDegraded, "degraded")
	}

	if StatusUnhealthy != "unhealthy" {
		t.Errorf("StatusUnhealthy = %v, want %v", StatusUnhealthy, "unhealthy")
	}
}
