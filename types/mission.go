// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Zero Root AI

package types

import (
	"encoding/json"
	"time"

	missionpb "github.com/zeroroot-ai/sdk/api/gen/gibson/mission/v1"
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

// Validate checks if the MissionContext has all required fields.
func (m *MissionContext) Validate() error {
	if m.ID == "" {
		return &ValidationError{Field: "ID", Message: "mission ID is required"}
	}

	if m.Name == "" {
		return &ValidationError{Field: "Name", Message: "mission name is required"}
	}

	return nil
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

// GetMetadata retrieves a metadata value by key.
func (m *MissionContext) GetMetadata(key string) (any, bool) {
	if m.Metadata == nil {
		return nil, false
	}
	val, ok := m.Metadata[key]
	return val, ok
}

// SetMetadata sets a metadata value.
func (m *MissionContext) SetMetadata(key string, value any) {
	if m.Metadata == nil {
		m.Metadata = make(map[string]any)
	}
	m.Metadata[key] = value
}

// IsExpired checks if the mission has exceeded its maximum duration.
// Returns false if no max duration is set or no start time is available.
func (m *MissionContext) IsExpired() bool {
	if m.Constraints.MaxDuration == 0 {
		return false
	}

	startTime, ok := m.GetMetadata("start_time")
	if !ok {
		return false
	}

	start, ok := startTime.(time.Time)
	if !ok {
		return false
	}

	return time.Since(start) > m.Constraints.MaxDuration
}

// ShouldStop checks if the mission should stop based on constraints.
// This checks both time limits and finding count limits.
func (m *MissionContext) ShouldStop(findingCount int) bool {
	// Check time limit
	if m.IsExpired() {
		return true
	}

	// Check finding count limit
	if m.Constraints.MaxFindings > 0 && findingCount >= m.Constraints.MaxFindings {
		return true
	}

	return false
}

// MeetsSeverityThreshold checks if a severity level meets the mission threshold.
func (m *MissionConstraints) MeetsSeverityThreshold(severity string) bool {
	if m.SeverityThreshold == "" {
		return true // No threshold set, accept all
	}

	severityLevels := map[string]int{
		"low":      1,
		"medium":   2,
		"high":     3,
		"critical": 4,
	}

	threshold, ok := severityLevels[m.SeverityThreshold]
	if !ok {
		return true // Unknown threshold, accept
	}

	level, ok := severityLevels[severity]
	if !ok {
		return true // Unknown severity, accept
	}

	return level >= threshold
}

// NewMissionContext creates a new mission context with default values.
//
// Prefer FromDefinition when a *missionpb.MissionDefinition is available.
// NewMissionContext is retained for test doubles and for call sites that
// receive ID/Name from a daemon work-item envelope without a full definition.
func NewMissionContext(id, name string) *MissionContext {
	return &MissionContext{
		ID:       id,
		Name:     name,
		Metadata: make(map[string]any),
	}
}

// FromDefinition constructs a MissionContext from the proto-canonical
// MissionDefinition. This is the single authorised constructor for agent code
// that has access to a full mission definition.
//
// Fields that exist on MissionDefinition (ID, Name, Metadata, Constraints) are
// projected from the proto. Runtime-only fields (Phase, CurrentAgent) are left
// at their zero values and should be populated separately from the daemon
// work-item envelope.
//
// If MissionDefinition.constraints is nil the projection's Constraints field
// is left at its zero value (no panic).
//
// Passing nil returns a zero-value MissionContext.
func FromDefinition(def *missionpb.MissionDefinition) MissionContext {
	if def == nil {
		return MissionContext{}
	}
	mc := MissionContext{
		ID:   def.GetId(),
		Name: def.GetName(),
	}
	// Convert proto map[string]string to map[string]any for the runtime view.
	if m := def.GetMetadata(); len(m) > 0 {
		mc.Metadata = make(map[string]any, len(m))
		for k, v := range m {
			mc.Metadata[k] = v
		}
	}
	// Project MissionConstraints from the proto field (sdk#47).
	// GetConstraints() returns nil when the field is absent; guard is explicit
	// so future readers understand the zero-value intent.
	if pc := def.GetConstraints(); pc != nil {
		var maxDur time.Duration
		if d := pc.GetMaxDuration(); d != nil {
			maxDur = d.AsDuration()
		}
		mc.Constraints = MissionConstraints{
			MaxDuration:       maxDur,
			MaxFindings:       int(pc.GetMaxFindings()),
			SeverityThreshold: pc.GetSeverityThreshold(),
			RequireEvidence:   pc.GetRequireEvidence(),
		}
	}
	return mc
}

// NewMissionConstraints creates mission constraints with default values.
func NewMissionConstraints() MissionConstraints {
	return MissionConstraints{
		RequireEvidence: true,
	}
}

// WithMaxDuration sets the maximum mission duration.
func (c MissionConstraints) WithMaxDuration(d time.Duration) MissionConstraints {
	c.MaxDuration = d
	return c
}

// WithMaxFindings sets the maximum number of findings.
func (c MissionConstraints) WithMaxFindings(count int) MissionConstraints {
	c.MaxFindings = count
	return c
}

// WithSeverityThreshold sets the minimum severity threshold.
func (c MissionConstraints) WithSeverityThreshold(threshold string) MissionConstraints {
	c.SeverityThreshold = threshold
	return c
}

// WithRequireEvidence sets whether evidence is required.
func (c MissionConstraints) WithRequireEvidence(require bool) MissionConstraints {
	c.RequireEvidence = require
	return c
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

// HasPreviousRun returns true if this execution has a previous run to reference.
func (m *MissionExecutionContext) HasPreviousRun() bool {
	return m.PreviousRunID != ""
}

// IsFirstRun returns true if this is the first execution of the mission.
func (m *MissionExecutionContext) IsFirstRun() bool {
	return m.RunNumber == 1
}
