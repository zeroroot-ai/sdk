// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Zero Root AI

// Package daemonclient provides a client library for connecting to the Gibson daemon.
//
// This package is the public SDK version of the daemon client, providing only
// operational RPC wrappers for DaemonService. Admin RPCs (Shutdown, tenant
// management, billing, etc.) are not included.
package daemonclient

import "time"

// DaemonStatus represents the current state and health information of the daemon.
//
// This struct is returned by the Status() method and includes both runtime state
// (PID, running status, uptime) and service information (registry, callback server,
// component counts).
type DaemonStatus struct {
	// Running indicates whether the daemon process is currently active
	Running bool `json:"running"`

	// PID is the process ID of the daemon
	PID int `json:"pid"`

	// StartTime is when the daemon was started
	StartTime time.Time `json:"start_time"`

	// Uptime is a human-readable duration since daemon start (e.g., "2h 15m")
	Uptime string `json:"uptime"`

	// SocketPath is the Unix socket path for daemon IPC (if using Unix sockets)
	SocketPath string `json:"socket_path,omitempty"`

	// GRPCAddress is the TCP address for daemon gRPC API (e.g., "localhost:50002")
	GRPCAddress string `json:"grpc_address"`

	// RegistryType is the registry mode: "embedded" or "etcd"
	RegistryType string `json:"registry_type"`

	// RegistryAddr is the registry endpoint address
	RegistryAddr string `json:"registry_address"`

	// CallbackAddr is the callback server endpoint address
	CallbackAddr string `json:"callback_address"`

	// AgentCount is the number of registered agents
	AgentCount int `json:"agent_count"`

	// MissionCount is the total number of missions (historical)
	MissionCount int `json:"mission_count"`

	// ActiveCount is the number of currently running missions
	ActiveCount int `json:"active_mission_count"`
}

// AgentInfo represents information about a registered agent.
type AgentInfo struct {
	Name        string
	Version     string
	Description string
	Address     string
	Status      string
}

// ToolInfo represents information about a registered tool.
type ToolInfo struct {
	Name         string
	Version      string
	Description  string
	Address      string
	Status       string
	Capabilities *Capabilities
}

// Capabilities represents the runtime privileges and features available to a tool.
type Capabilities struct {
	HasRoot         bool
	HasSudo         bool
	CanRawSocket    bool
	Features        map[string]bool
	BlockedArgs     []string
	ArgAlternatives map[string]string
}

// PluginInfo represents information about a registered plugin.
type PluginInfo struct {
	Name        string
	Version     string
	Description string
	Address     string
	Status      string
}

// PluginQueryResult represents the result of a plugin query.
type PluginQueryResult struct {
	// Result is the unmarshaled result from the plugin method
	Result any
	// DurationMs is how long the query took in milliseconds
	DurationMs int64
}

// MissionEvent represents an event from a running mission.
type MissionEvent struct {
	Type      string
	Timestamp time.Time
	Message   string
	Data      map[string]interface{}
}

// Event represents a generic daemon event for TUI subscription.
type Event struct {
	Type      string
	Source    string
	Timestamp time.Time
	Data      map[string]interface{}
}

// MissionInfo represents information about a mission.
type MissionInfo struct {
	ID                  string
	Name                string
	MissionDefinitionID string
	TargetID            string
	Status              string
	StartTime           time.Time
	EndTime             time.Time
	FindingCount        int
}

// CreateMissionOptions contains options for creating a new mission.
// Missions reference a registered target and a registered mission definition;
// inline construction was removed under spec mission-api-only-cleanup.
type CreateMissionOptions struct {
	Name                string
	Description         string
	TargetID            string
	MissionDefinitionID string
	Variables           map[string]string
	MemoryContinuity    string
	Metadata            map[string]string
}

// CreateMissionResult represents the result of creating a mission.
type CreateMissionResult struct {
	MissionID           string
	TargetID            string
	MissionDefinitionID string
	Name                string
	Description         string
	Status              string
}

// CreateMissionDefinitionResult represents the registration of a mission
// definition through the CreateMissionDefinition RPC.
type CreateMissionDefinitionResult struct {
	MissionDefinitionID string
	Info                MissionDefinitionInfo
}

// MissionDefinitionInfo is the summary record returned by the daemon for a
// registered mission definition.
type MissionDefinitionInfo struct {
	Name        string
	Version     string
	Description string
	Source      string
	InstalledAt time.Time
	UpdatedAt   time.Time
	NodeCount   int
}

// StartResult represents the result of starting a component.
type StartResult struct {
	PID     int
	Port    int
	LogPath string
}

// StopResult represents the result of stopping a component.
type StopResult struct {
	StoppedCount int
	TotalCount   int
}

// BuildResult represents the result of building a component.
type BuildResult struct {
	Success  bool          // Build success
	Stdout   string        // Build stdout
	Stderr   string        // Build stderr
	Duration time.Duration // Build time
}

// ComponentInfo represents detailed information about a component.
type ComponentInfo struct {
	Name      string
	Version   string
	Kind      string
	Status    string
	Source    string
	RepoPath  string
	BinPath   string
	Port      int
	PID       int
	CreatedAt time.Time
	UpdatedAt time.Time
	StartedAt *time.Time
	StoppedAt *time.Time
	Manifest  string // JSON-encoded manifest info
}

// LogsOptions contains options for retrieving component logs.
type LogsOptions struct {
	Follow bool // Stream logs continuously
	Lines  int  // Number of lines to return (default 50)
}

// LogEntry represents a single log entry from a component.
type LogEntry struct {
	Timestamp time.Time
	Level     string
	Message   string
	Fields    map[string]string
}

// MissionRun represents a single execution run of a mission with a given name.
type MissionRun struct {
	MissionID     string
	RunNumber     int
	Status        string
	CreatedAt     time.Time
	CompletedAt   *time.Time
	FindingsCount int
}

// MissionDefinition represents an installed mission definition.
type MissionDefinition struct {
	Name         string                  // Mission name
	Version      string                  // Mission version
	Description  string                  // Mission description
	Source       string                  // Git URL source
	InstalledAt  time.Time               // Installation timestamp
	Dependencies *MissionDependencyList  // Required dependencies
	Nodes        map[string]*MissionNode // Mission nodes
	Edges        []MissionEdge           // Mission edges
	EntryPoints  []string                // Entry point node IDs
	ExitPoints   []string                // Exit point node IDs
}

// MissionDependencyList contains lists of required dependencies.
type MissionDependencyList struct {
	Agents  []string
	Tools   []string
	Plugins []string
}

// MissionNode represents a node in a mission DAG.
type MissionNode struct {
	ID   string
	Type string
	Name string
}

// MissionEdge represents an edge in a mission DAG.
type MissionEdge struct {
	From string
	To   string
}

// OperationResult represents typed operation metrics.
type OperationResult struct {
	Status        string
	DurationMs    int64
	StartedAt     int64
	CompletedAt   int64
	TurnsUsed     int32
	TokensUsed    int64
	NodesExecuted int32
	NodesFailed   int32
	FindingsCount int32
	CriticalCount int32
	HighCount     int32
	MediumCount   int32
	LowCount      int32
	ErrorMessage  string
	ErrorCode     string
}
