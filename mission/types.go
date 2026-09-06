// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Zero Root AI

package mission

import (
	"time"

	"github.com/zeroroot-ai/sdk/finding"
)

// CreateMissionOpts configures mission creation.
// These options allow agents to customize mission behavior and constraints.
type CreateMissionOpts struct {
	// Name is an optional human-readable name for the mission.
	// If empty, a name will be auto-generated based on the mission.
	Name string `json:"name,omitempty"`

	// Constraints defines execution limits for the mission.
	// If nil, default constraints will be applied.
	Constraints *MissionConstraints `json:"constraints,omitempty"`

	// Metadata contains arbitrary key-value pairs for storing
	// additional mission context or configuration.
	Metadata map[string]any `json:"metadata,omitempty"`

	// Tags are labels for categorizing and filtering missions.
	Tags []string `json:"tags,omitempty"`

	// CatalogMission names a mission definition checked into the platform's
	// mission catalog, to be originated instead of a caller-supplied graph.
	// The checked-in definition is the authoritative one (ADR-0018), so an
	// agent that wants a first-party mission names it here rather than
	// rebuilding its graph — two definitions of one mission is what ADR-0027
	// forbids.
	//
	// When set, the mission definition argument to CreateMission is not sent:
	// exactly one of the two may reach the daemon. Passing both is refused
	// locally rather than round-tripping to an InvalidArgument.
	CatalogMission string `json:"catalog_mission,omitempty"`

	// CatalogParams carries the parameters the named catalog mission declares,
	// keyed by the definition's own parameter names.
	//
	// The map is closed: the daemon refuses an unrecognised key rather than
	// ignoring it, so a caller can never believe a parameter bound when it did
	// not. The runtime target is deliberately not among these — it binds from
	// the targetID argument alone, so a mission can only run against a target
	// the tenant registered.
	//
	// Only meaningful alongside CatalogMission.
	CatalogParams map[string]string `json:"catalog_params,omitempty"`
}

// MissionConstraints limits mission execution to prevent resource exhaustion.
// All constraints are enforced by the mission orchestrator.
type MissionConstraints struct {
	// MaxDuration is the maximum time allowed for mission execution.
	// Zero value means no duration limit.
	MaxDuration time.Duration `json:"max_duration,omitempty"`

	// MaxTokens is the maximum number of LLM tokens the mission can consume.
	// Zero value means no token limit.
	MaxTokens int64 `json:"max_tokens,omitempty"`

	// MaxCost is the maximum dollar cost for LLM API calls.
	// Zero value means no cost limit.
	MaxCost float64 `json:"max_cost,omitempty"`

	// MaxFindings is the maximum number of findings the mission can generate.
	// Zero value means no finding limit.
	MaxFindings int `json:"max_findings,omitempty"`
}

// RunMissionOpts configures mission execution behavior.
type RunMissionOpts struct {
	// Wait indicates whether to block until the mission completes.
	// If false, RunMission returns immediately after queueing.
	Wait bool `json:"wait,omitempty"`

	// Timeout specifies how long to wait for mission completion.
	// Only applies when Wait is true. Zero value means wait indefinitely.
	Timeout time.Duration `json:"timeout,omitempty"`
}

// MissionInfo provides metadata about a mission.
// This is returned when creating or querying missions.
type MissionInfo struct {
	// ID is the unique identifier for the mission.
	ID string `json:"id"`

	// Name is the human-readable mission name.
	Name string `json:"name"`

	// Status is the current execution state.
	Status MissionStatus `json:"status"`

	// TargetID identifies the target being tested.
	TargetID string `json:"target_id"`

	// ParentMissionID is the ID of the parent mission, if this is a child mission.
	// Empty string means this is a root mission.
	ParentMissionID string `json:"parent_mission_id,omitempty"`

	// CreatedAt is the timestamp when the mission was created.
	CreatedAt time.Time `json:"created_at"`

	// Tags are labels for categorizing and filtering.
	Tags []string `json:"tags,omitempty"`
}

// MissionStatus represents the current state of a mission.
type MissionStatus string

const (
	// MissionStatusPending indicates the mission is queued but not yet running.
	MissionStatusPending MissionStatus = "pending"

	// MissionStatusRunning indicates the mission is currently executing.
	MissionStatusRunning MissionStatus = "running"

	// MissionStatusPaused indicates the mission is temporarily suspended.
	MissionStatusPaused MissionStatus = "paused"

	// MissionStatusCompleted indicates the mission finished successfully.
	MissionStatusCompleted MissionStatus = "completed"

	// MissionStatusFailed indicates the mission encountered an error.
	MissionStatusFailed MissionStatus = "failed"

	// MissionStatusCancelled indicates the mission was cancelled by a user or agent.
	MissionStatusCancelled MissionStatus = "cancelled"
)

// IsValid checks if the status is a recognized value.
func (s MissionStatus) IsValid() bool {
	switch s {
	case MissionStatusPending, MissionStatusRunning, MissionStatusPaused,
		MissionStatusCompleted, MissionStatusFailed, MissionStatusCancelled:
		return true
	default:
		return false
	}
}

// IsTerminal returns true if the status represents a final state.
func (s MissionStatus) IsTerminal() bool {
	switch s {
	case MissionStatusCompleted, MissionStatusFailed, MissionStatusCancelled:
		return true
	default:
		return false
	}
}

// MissionStatusInfo provides detailed status information about a running mission.
type MissionStatusInfo struct {
	// Status is the current execution state.
	Status MissionStatus `json:"status"`

	// Progress is the completion percentage (0.0 to 1.0).
	Progress float64 `json:"progress"`

	// Phase is the current mission phase or step name.
	Phase string `json:"phase,omitempty"`

	// FindingCounts maps finding severity levels to counts.
	// Keys are severity names (e.g., "critical", "high", "medium").
	FindingCounts map[string]int `json:"finding_counts,omitempty"`

	// TokenUsage is the cumulative number of LLM tokens consumed.
	TokenUsage int64 `json:"token_usage"`

	// Duration is the elapsed execution time.
	Duration time.Duration `json:"duration"`

	// Error contains error details if the mission failed.
	Error string `json:"error,omitempty"`
}

// MissionResult contains the final results of a completed mission.
type MissionResult struct {
	// MissionID is the unique identifier of the completed mission.
	MissionID string `json:"mission_id"`

	// Status is the final execution state.
	Status MissionStatus `json:"status"`

	// Findings are the security vulnerabilities discovered during the mission.
	Findings []finding.Finding `json:"findings,omitempty"`

	// Output contains arbitrary mission output data.
	// This can include scan results, extracted data, or other artifacts.
	Output map[string]any `json:"output,omitempty"`

	// Metrics provides execution statistics.
	Metrics MissionMetrics `json:"metrics"`

	// Error contains error details if the mission failed.
	Error string `json:"error,omitempty"`

	// CompletedAt is the timestamp when the mission finished.
	CompletedAt time.Time `json:"completed_at"`
}

// MissionMetrics aggregates execution statistics for a mission.
type MissionMetrics struct {
	// Duration is the total execution time.
	Duration time.Duration `json:"duration"`

	// TokensUsed is the total number of LLM tokens consumed.
	TokensUsed int64 `json:"tokens_used"`

	// ToolCalls is the number of tool invocations during execution.
	ToolCalls int `json:"tool_calls"`

	// AgentCalls is the number of sub-agent invocations.
	AgentCalls int `json:"agent_calls"`

	// FindingsCount is the total number of findings generated.
	FindingsCount int `json:"findings_count"`
}

// MissionFilter specifies criteria for listing missions.
// All fields are optional; nil/zero values are ignored.
type MissionFilter struct {
	// Status filters by mission status.
	Status *MissionStatus `json:"status,omitempty"`

	// TargetID filters by target identifier.
	TargetID *string `json:"target_id,omitempty"`

	// ParentMissionID filters by parent mission.
	// Use to find all child missions of a specific parent.
	ParentMissionID *string `json:"parent_mission_id,omitempty"`

	// CreatedAfter filters to missions created after this timestamp.
	CreatedAfter *time.Time `json:"created_after,omitempty"`

	// CreatedBefore filters to missions created before this timestamp.
	CreatedBefore *time.Time `json:"created_before,omitempty"`

	// Tags filters to missions having all specified tags.
	Tags []string `json:"tags,omitempty"`

	// Limit is the maximum number of results to return.
	// Zero value means no limit.
	Limit int `json:"limit,omitempty"`

	// Offset is the number of results to skip (for pagination).
	Offset int `json:"offset,omitempty"`
}
