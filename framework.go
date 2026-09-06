// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Zero Root AI

package sdk

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"sync"
	"time"

	"github.com/google/uuid"
	"go.opentelemetry.io/otel/trace"

	"github.com/zeroroot-ai/sdk/agent"
	"github.com/zeroroot-ai/sdk/finding"
	"github.com/zeroroot-ai/sdk/plugin"
	"github.com/zeroroot-ai/sdk/tool"
)

// Framework provides the main SDK interface for interacting with the Gibson system.
// It manages missions, registries, findings, and lifecycle operations.
//
// The Framework acts as the central orchestrator, coordinating between:
//   - Agents: Autonomous security testing components
//   - Tools: Executable utilities used by agents
//   - Plugins: Extension points for custom functionality
//   - Missions: Orchestrated testing campaigns
//   - Findings: Security vulnerabilities discovered during testing
type Framework interface {
	// Mission management

	// CreateMission creates a new testing mission with the provided configuration.
	// Returns the created mission or an error if creation fails.
	CreateMission(ctx context.Context, opts ...MissionOption) (*Mission, error)

	// StartMission initiates execution of a mission.
	// The mission will begin executing tasks with configured agents.
	StartMission(ctx context.Context, missionID string) error

	// StopMission halts execution of a running mission.
	// In-flight tasks will be cancelled gracefully.
	StopMission(ctx context.Context, missionID string) error

	// GetMission retrieves mission details by ID.
	// Returns an error if the mission is not found.
	GetMission(ctx context.Context, missionID string) (*Mission, error)

	// ListMissions returns a list of missions matching the provided options.
	// Supports filtering, pagination, and sorting.
	ListMissions(ctx context.Context, opts ...ListOption) ([]*Mission, error)

	// Registry access

	// Agents returns the agent registry for registering and discovering agents.
	Agents() AgentRegistry

	// Tools returns the tool registry for registering and discovering tools.
	Tools() ToolRegistry

	// Plugins returns the plugin registry for registering and discovering plugins.
	Plugins() PluginRegistry

	// Findings

	// GetFindings retrieves findings matching the provided filter criteria.
	// Returns all findings if filter is nil.
	GetFindings(ctx context.Context, filter finding.Filter) ([]finding.Finding, error)

	// ExportFindings exports findings in the specified format to the writer.
	// Supported formats: JSON, SARIF, CSV, HTML.
	ExportFindings(ctx context.Context, format finding.ExportFormat, w io.Writer) error

	// Lifecycle

	// Start initializes the framework and prepares it for operation.
	// This should be called before using any framework functionality.
	Start(ctx context.Context) error

	// Shutdown gracefully stops the framework and releases resources.
	// This should be called when the framework is no longer needed.
	Shutdown(ctx context.Context) error
}

// Mission represents a testing campaign executed by the Gibson framework.
// A mission coordinates one or more agents to test a target system.
type Mission struct {
	// ID is the unique identifier for this mission.
	ID string `json:"id"`

	// Name is a human-readable name for the mission.
	Name string `json:"name"`

	// Description explains what this mission is testing.
	Description string `json:"description"`

	// Status indicates the current state of the mission.
	// Values: "pending", "running", "stopped", "completed", "failed"
	Status string `json:"status"`

	// TargetID identifies the system being tested.
	TargetID string `json:"target_id,omitempty"`

	// AgentNames lists the agents assigned to this mission.
	AgentNames []string `json:"agent_names,omitempty"`

	// CreatedAt is the timestamp when the mission was created.
	CreatedAt time.Time `json:"created_at"`

	// StartedAt is the timestamp when the mission execution began.
	StartedAt *time.Time `json:"started_at,omitempty"`

	// CompletedAt is the timestamp when the mission finished.
	CompletedAt *time.Time `json:"completed_at,omitempty"`

	// Metadata stores additional mission-specific information.
	Metadata map[string]interface{} `json:"metadata,omitempty"`
}

// AgentRegistry manages agent registration and discovery.
type AgentRegistry interface {
	// Register adds an agent to the registry.
	// Returns an error if an agent with the same name already exists.
	Register(a agent.Agent) error

	// Get retrieves an agent by name.
	// Returns an error if the agent is not found.
	Get(name string) (agent.Agent, error)

	// List returns descriptors for all registered agents.
	List() []agent.Descriptor

	// Unregister removes an agent from the registry.
	Unregister(name string) error
}

// ToolRegistry manages tool registration and discovery.
type ToolRegistry interface {
	// Register adds a tool to the registry.
	// Returns an error if a tool with the same name already exists.
	Register(t tool.Tool) error

	// Get retrieves a tool by name.
	// Returns an error if the tool is not found.
	Get(name string) (tool.Tool, error)

	// List returns descriptors for all registered tools.
	List() []tool.Descriptor

	// Unregister removes a tool from the registry.
	Unregister(name string) error
}

// PluginRegistry manages plugin registration and discovery.
//
// Plugins are external processes registered via plugin.Serve and
// RegisterComponent. This interface is used by the framework for
// in-process plugin discovery; for remote plugins use agent.Harness.ListPlugins.
type PluginRegistry interface {
	// List returns descriptors for all available plugins.
	List() []plugin.Descriptor
}

// defaultFramework is the concrete implementation of Framework.
type defaultFramework struct {
	logger        *slog.Logger
	tracer        trace.Tracer
	configPath    string
	agents        *agentRegistry
	tools         *toolRegistry
	plugins       *pluginRegistry
	missionStore  MissionStore
	findingsStore FindingsStore
	started       bool
}

// CreateMission creates a new mission with the provided options.
func (f *defaultFramework) CreateMission(ctx context.Context, opts ...MissionOption) (*Mission, error) {
	cfg := &missionConfig{}
	for _, opt := range opts {
		opt(cfg)
	}

	mission := &Mission{
		ID:          uuid.New().String(),
		Name:        cfg.name,
		Description: cfg.description,
		Status:      "pending",
		TargetID:    cfg.targetID,
		AgentNames:  cfg.agentNames,
		CreatedAt:   time.Now(),
		Metadata:    cfg.metadata,
	}

	f.missionStore.Set(mission.ID, mission)

	f.logger.Info("mission created",
		slog.String("mission_id", mission.ID),
		slog.String("name", mission.Name),
	)

	return mission, nil
}

// StartMission starts execution of a mission.
func (f *defaultFramework) StartMission(ctx context.Context, missionID string) error {
	mission, ok := f.missionStore.Get(missionID)
	if !ok {
		return fmt.Errorf("%w: %s", ErrMissionNotFound, missionID)
	}

	if mission.Status != "pending" {
		return fmt.Errorf("mission cannot be started from status: %s", mission.Status)
	}

	now := time.Now()
	mission.Status = "running"
	mission.StartedAt = &now
	f.missionStore.Set(missionID, mission)

	f.logger.Info("mission started",
		slog.String("mission_id", missionID),
	)

	return nil
}

// StopMission stops a running mission.
func (f *defaultFramework) StopMission(ctx context.Context, missionID string) error {
	mission, ok := f.missionStore.Get(missionID)
	if !ok {
		return fmt.Errorf("%w: %s", ErrMissionNotFound, missionID)
	}

	if mission.Status != "running" {
		return fmt.Errorf("mission is not running: %s", mission.Status)
	}

	now := time.Now()
	mission.Status = "stopped"
	mission.CompletedAt = &now
	f.missionStore.Set(missionID, mission)

	f.logger.Info("mission stopped",
		slog.String("mission_id", missionID),
	)

	return nil
}

// GetMission retrieves a mission by ID.
func (f *defaultFramework) GetMission(ctx context.Context, missionID string) (*Mission, error) {
	mission, ok := f.missionStore.Get(missionID)
	if !ok {
		return nil, fmt.Errorf("%w: %s", ErrMissionNotFound, missionID)
	}

	return mission, nil
}

// ListMissions returns a list of missions.
func (f *defaultFramework) ListMissions(ctx context.Context, opts ...ListOption) ([]*Mission, error) {
	cfg := &listConfig{}
	for _, opt := range opts {
		opt(cfg)
	}

	missions := f.missionStore.All()

	// Apply offset
	if cfg.offset > 0 {
		if cfg.offset >= len(missions) {
			return []*Mission{}, nil
		}
		missions = missions[cfg.offset:]
	}

	// Apply limit
	if cfg.limit > 0 && cfg.limit < len(missions) {
		missions = missions[:cfg.limit]
	}

	return missions, nil
}

// Agents returns the agent registry.
func (f *defaultFramework) Agents() AgentRegistry {
	return f.agents
}

// Tools returns the tool registry.
func (f *defaultFramework) Tools() ToolRegistry {
	return f.tools
}

// Plugins returns the plugin registry.
func (f *defaultFramework) Plugins() PluginRegistry {
	return f.plugins
}

// GetFindings retrieves findings matching the filter.
func (f *defaultFramework) GetFindings(ctx context.Context, filter finding.Filter) ([]finding.Finding, error) {
	var results []finding.Finding

	for _, record := range f.findingsStore.AllRecords() {
		if filter.Matches(record.Finding) {
			results = append(results, record.Finding)
		}
	}

	// Apply pagination
	if filter.Offset > 0 {
		if filter.Offset >= len(results) {
			return []finding.Finding{}, nil
		}
		results = results[filter.Offset:]
	}

	if filter.Limit > 0 && filter.Limit < len(results) {
		results = results[:filter.Limit]
	}

	return results, nil
}

// ExportFindings exports findings in the specified format.
func (f *defaultFramework) ExportFindings(ctx context.Context, format finding.ExportFormat, w io.Writer) error {
	if !format.IsValid() {
		return fmt.Errorf("invalid export format: %s", format)
	}

	// Get all findings
	allFindings, err := f.GetFindings(ctx, finding.Filter{})
	if err != nil {
		return fmt.Errorf("failed to get findings: %w", err)
	}

	// Ensure we have a non-nil slice for JSON encoding
	if allFindings == nil {
		allFindings = []finding.Finding{}
	}

	// Export based on format
	switch format {
	case finding.FormatJSON:
		encoder := json.NewEncoder(w)
		encoder.SetIndent("", "  ")
		return encoder.Encode(allFindings)

	case finding.FormatCSV:
		// Simple CSV export (headers + one line per finding)
		_, err := fmt.Fprintf(w, "ID,Title,Severity,Category,Status,CreatedAt\n")
		if err != nil {
			return err
		}
		for _, f := range allFindings {
			_, err := fmt.Fprintf(w, "%s,%s,%s,%s,%s,%s\n",
				f.ID, f.Title, f.Severity, f.Category, f.Status, f.CreatedAt.Format(time.RFC3339))
			if err != nil {
				return err
			}
		}
		return nil

	case finding.FormatHTML:
		// Simple HTML export
		_, err := fmt.Fprintf(w, `<!DOCTYPE html>
<html>
<head><title>Gibson Findings Report</title></head>
<body>
<h1>Security Findings Report</h1>
<p>Generated: %s</p>
<table border="1">
<tr><th>ID</th><th>Title</th><th>Severity</th><th>Category</th><th>Status</th></tr>
`, time.Now().Format(time.RFC3339))
		if err != nil {
			return err
		}
		for _, f := range allFindings {
			_, err := fmt.Fprintf(w, "<tr><td>%s</td><td>%s</td><td>%s</td><td>%s</td><td>%s</td></tr>\n",
				f.ID, f.Title, f.Severity, f.Category, f.Status)
			if err != nil {
				return err
			}
		}
		_, err = fmt.Fprintf(w, "</table>\n</body>\n</html>")
		return err

	case finding.FormatSARIF:
		encoder := json.NewEncoder(w)
		encoder.SetIndent("", "  ")
		return encoder.Encode(buildSARIF(allFindings, Version))

	default:
		return fmt.Errorf("unsupported export format: %s", format)
	}
}

// Start initializes the framework.
func (f *defaultFramework) Start(ctx context.Context) error {
	if f.started {
		return errors.New("framework already started")
	}

	f.logger.Info("starting Gibson framework")
	f.started = true
	return nil
}

// Shutdown gracefully stops the framework.
func (f *defaultFramework) Shutdown(ctx context.Context) error {
	if !f.started {
		return nil
	}

	f.logger.Info("shutting down Gibson framework")

	// Stop all running missions
	for _, mission := range f.missionStore.All() {
		if mission.Status == "running" {
			now := time.Now()
			mission.Status = "stopped"
			mission.CompletedAt = &now
			f.missionStore.Set(mission.ID, mission)
			f.logger.Info("stopped mission during shutdown", slog.String("mission_id", mission.ID))
		}
	}

	f.started = false
	return nil
}

// agentRegistry is the concrete implementation of AgentRegistry.
type agentRegistry struct {
	logger *slog.Logger
	agents map[string]agent.Agent
	mu     sync.RWMutex
}

func newAgentRegistry(logger *slog.Logger) *agentRegistry {
	return &agentRegistry{
		logger: logger,
		agents: make(map[string]agent.Agent),
	}
}

func (r *agentRegistry) Register(a agent.Agent) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if _, exists := r.agents[a.Name()]; exists {
		return fmt.Errorf("agent already registered: %s", a.Name())
	}

	r.agents[a.Name()] = a
	r.logger.Info("agent registered",
		slog.String("name", a.Name()),
		slog.String("version", a.Version()),
	)
	return nil
}

func (r *agentRegistry) Get(name string) (agent.Agent, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	a, ok := r.agents[name]
	if !ok {
		return nil, fmt.Errorf("%w: %s", ErrAgentNotFound, name)
	}
	return a, nil
}

func (r *agentRegistry) List() []agent.Descriptor {
	r.mu.RLock()
	defer r.mu.RUnlock()

	descriptors := make([]agent.Descriptor, 0, len(r.agents))
	for _, a := range r.agents {
		descriptors = append(descriptors, agent.Descriptor{
			Name:        a.Name(),
			Version:     a.Version(),
			Description: a.Description(),
		})
	}
	return descriptors
}

func (r *agentRegistry) Unregister(name string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if _, exists := r.agents[name]; !exists {
		return fmt.Errorf("%w: %s", ErrAgentNotFound, name)
	}

	delete(r.agents, name)
	r.logger.Info("agent unregistered", slog.String("name", name))
	return nil
}

// toolRegistry is the concrete implementation of ToolRegistry.
type toolRegistry struct {
	logger *slog.Logger
	tools  map[string]tool.Tool
	mu     sync.RWMutex
}

func newToolRegistry(logger *slog.Logger) *toolRegistry {
	return &toolRegistry{
		logger: logger,
		tools:  make(map[string]tool.Tool),
	}
}

func (r *toolRegistry) Register(t tool.Tool) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if _, exists := r.tools[t.Name()]; exists {
		return fmt.Errorf("tool already registered: %s", t.Name())
	}

	r.tools[t.Name()] = t
	r.logger.Info("tool registered",
		slog.String("name", t.Name()),
		slog.String("version", t.Version()),
	)
	return nil
}

func (r *toolRegistry) Get(name string) (tool.Tool, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	t, ok := r.tools[name]
	if !ok {
		return nil, fmt.Errorf("%w: %s", ErrToolNotFound, name)
	}
	return t, nil
}

func (r *toolRegistry) List() []tool.Descriptor {
	r.mu.RLock()
	defer r.mu.RUnlock()

	descriptors := make([]tool.Descriptor, 0, len(r.tools))
	for _, t := range r.tools {
		descriptors = append(descriptors, tool.Descriptor{
			Name:        t.Name(),
			Version:     t.Version(),
			Description: t.Description(),
		})
	}
	return descriptors
}

func (r *toolRegistry) Unregister(name string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if _, exists := r.tools[name]; !exists {
		return fmt.Errorf("%w: %s", ErrToolNotFound, name)
	}

	delete(r.tools, name)
	r.logger.Info("tool unregistered", slog.String("name", name))
	return nil
}

// pluginRegistry is the concrete implementation of PluginRegistry.
//
// pluginRegistry is the in-process plugin registry used by the SDK framework.
// For agent-side usage, plugin descriptors are discovered via
// agent.Harness.ListPlugins which queries the daemon's component registry.
// This in-process registry returns an empty list; it is retained to satisfy
// the PluginRegistry interface for framework consumers that do not use a harness.
type pluginRegistry struct {
	logger *slog.Logger
}

func newPluginRegistry(logger *slog.Logger) *pluginRegistry {
	return &pluginRegistry{logger: logger}
}

func (r *pluginRegistry) List() []plugin.Descriptor {
	return []plugin.Descriptor{}
}
