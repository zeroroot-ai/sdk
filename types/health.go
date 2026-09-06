// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Zero Root AI

package types

// Health status constants represent the operational state of a component.
const (
	// StatusHealthy indicates the component is fully operational.
	StatusHealthy = "healthy"

	// StatusDegraded indicates the component is operational but experiencing issues.
	StatusDegraded = "degraded"

	// StatusUnhealthy indicates the component is not operational.
	StatusUnhealthy = "unhealthy"
)

// HealthStatus represents the health state of a component or service.
// It provides detailed information about operational status, issues, and context.
type HealthStatus struct {
	// Status is the current health state (healthy, degraded, or unhealthy).
	Status string `json:"status"`

	// Message provides a human-readable description of the health status.
	Message string `json:"message,omitempty"`

	// Details contains additional context and diagnostic information.
	// This can include error details, performance metrics, or dependency status.
	Details map[string]any `json:"details,omitempty"`
}

// IsHealthy returns true if the status is StatusHealthy.
func (h HealthStatus) IsHealthy() bool {
	return h.Status == StatusHealthy
}

// IsDegraded returns true if the status is StatusDegraded.
func (h HealthStatus) IsDegraded() bool {
	return h.Status == StatusDegraded
}

// IsUnhealthy returns true if the status is StatusUnhealthy.
func (h HealthStatus) IsUnhealthy() bool {
	return h.Status == StatusUnhealthy
}

// NewHealthyStatus creates a new healthy status with an optional message.
func NewHealthyStatus(message string) HealthStatus {
	return HealthStatus{
		Status:  StatusHealthy,
		Message: message,
	}
}

// NewDegradedStatus creates a new degraded status with a message and optional details.
func NewDegradedStatus(message string, details map[string]any) HealthStatus {
	return HealthStatus{
		Status:  StatusDegraded,
		Message: message,
		Details: details,
	}
}

// NewUnhealthyStatus creates a new unhealthy status with a message and optional details.
func NewUnhealthyStatus(message string, details map[string]any) HealthStatus {
	return HealthStatus{
		Status:  StatusUnhealthy,
		Message: message,
		Details: details,
	}
}
