// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Zero Root AI

package types

import (
	"encoding/json"
	"time"
)

// MissionContext contains the operational context for a security testing mission.
// It provides agents with mission parameters, constraints, and tracking information.
//
// MissionContext is a derived, agent-facing runtime view over the proto-canonical
// MissionDefinition. It MUST be constructed via FromDefinition when a
// MissionDefinition is available. Hand-populating ID or Name independently of
// a MissionDefinition is forbidden — those fields come from the proto.
//
// Runtime-only fields (Phase, CurrentAgent, Constraints) have no counterpart in
// MissionDefinition because they describe execution state rather than mission
// authorship. They are populated by the daemon's work-item envelope at dispatch
// time and are not part of the stored mission schema.
type MissionContext struct {
	// ID is a unique identifier for the mission.
	ID string `json:"id"`

	// Name is a human-readable name for the mission.
	Name string `json:"name"`

	// CurrentAgent identifies the agent currently executing the mission.
	CurrentAgent string `json:"current_agent,omitempty"`

	// Phase indicates the current mission phase (e.g., "reconnaissance", "exploitation", "reporting").
	Phase string `json:"phase,omitempty"`

	// Constraints defines operational limits and requirements for the mission.
	Constraints MissionConstraints `json:"constraints"`

	// Metadata stores additional mission-specific information.
	// This can include start time, objectives, priorities, team assignments, etc.
	Metadata map[string]any `json:"metadata,omitempty"`
}

// MissionConstraints defines operational limits for mission execution.
// These constraints ensure testing stays within acceptable boundaries.
type MissionConstraints struct {
	// MaxDuration is the maximum time allowed for mission execution.
	// Zero value means no time limit.
	MaxDuration time.Duration `json:"max_duration,omitempty"`

	// MaxFindings is the maximum number of findings to collect before stopping.
	// Zero value means no limit.
	MaxFindings int `json:"max_findings,omitempty"`

	// SeverityThreshold is the minimum severity level required to report findings.
	// Common values: "low", "medium", "high", "critical".
	SeverityThreshold string `json:"severity_threshold,omitempty"`

	// RequireEvidence indicates whether findings must include proof-of-concept evidence.
	RequireEvidence bool `json:"require_evidence"`
}

// UnmarshalJSON implements custom unmarshaling for MissionContext.
// This handles the case where the 'constraints' field can be either:
// - A MissionConstraints struct (SDK native format)
// - An array of strings (Gibson's harness.MissionContext format)
// When constraints is an array of strings, it is ignored and an empty
// MissionConstraints struct is used instead.
func (m *MissionContext) UnmarshalJSON(data []byte) error {
	// Use an alias to avoid infinite recursion
	type Alias MissionContext
	aux := &struct {
		Constraints json.RawMessage `json:"constraints"`
		*Alias
	}{
		Alias: (*Alias)(m),
	}

	if err := json.Unmarshal(data, aux); err != nil {
		return err
	}

	// If constraints is empty or null, use default empty struct
	if len(aux.Constraints) == 0 || string(aux.Constraints) == "null" {
		m.Constraints = MissionConstraints{}
		return nil
	}

	// Try to unmarshal as MissionConstraints struct first
	var constraints MissionConstraints
	if err := json.Unmarshal(aux.Constraints, &constraints); err != nil {
		// If that fails, check if it's an array (Gibson's format)
		var constraintsArray []string
		if arrErr := json.Unmarshal(aux.Constraints, &constraintsArray); arrErr == nil {
			// It's an array - use empty constraints struct
			// The array format is just string labels, not actual constraint values
			m.Constraints = MissionConstraints{}
			return nil
		}
		// Not a struct or array - return the original error
		return err
	}

	m.Constraints = constraints
	return nil
}

// MissionExecutionContext extends mission tracking with run history and execution state.
// It supports resumable missions, run continuity, and accumulated metrics across multiple executions.
type MissionExecutionContext struct {
	// Core identification
	MissionID   string `json:"mission_id"`
	MissionName string `json:"mission_name"`
	RunNumber   int    `json:"run_number"`

	// Execution context
	IsResumed       bool   `json:"is_resumed"`
	ResumedFromNode string `json:"resumed_from_node,omitempty"`

	// Run linkage
	PreviousRunID     string `json:"previous_run_id,omitempty"`
	PreviousRunStatus string `json:"previous_run_status,omitempty"`

	// Accumulated stats
	TotalFindingsAllRuns int `json:"total_findings_all_runs"`

	// Memory configuration
	MemoryContinuity string `json:"memory_continuity"`

	// Existing constraint fields
	Constraints MissionConstraints `json:"constraints"`
}

// MissionRunSummary provides a summary view of a mission execution run.
// Used for history queries and tracking execution patterns across multiple runs.
type MissionRunSummary struct {
	MissionID     string     `json:"mission_id"`
	RunNumber     int        `json:"run_number"`
	Status        string     `json:"status"`
	FindingsCount int        `json:"findings_count"`
	CreatedAt     time.Time  `json:"created_at"`
	CompletedAt   *time.Time `json:"completed_at,omitempty"`
}
