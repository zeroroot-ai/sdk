// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Zero Root AI

package agent

import (
	"context"
	"log/slog"
	"time"

	"github.com/zeroroot-ai/sdk/codegen/workspace"
	"github.com/zeroroot-ai/sdk/finding"
	"github.com/zeroroot-ai/sdk/llm"
	"github.com/zeroroot-ai/sdk/mission"
	"github.com/zeroroot-ai/sdk/planning"
	"github.com/zeroroot-ai/sdk/plugin"
	"github.com/zeroroot-ai/sdk/tool"
	"github.com/zeroroot-ai/sdk/types"
	"go.opentelemetry.io/otel/trace"
	"google.golang.org/protobuf/proto"
)

// ToolCall represents a single tool invocation request for parallel execution
type ToolCall struct {
	Name  string         // Tool name to invoke
	Input map[string]any // Tool input parameters
}

// ToolResult represents the result of a tool invocation
type ToolResult struct {
	Name   string         // Tool name that was invoked
	Output map[string]any // Tool output (nil if error)
	Error  error          // Error if tool failed (nil if success)
}

// QueuedToolResult represents the result of a single tool execution from QueueToolWork.
// Results arrive in completion order, not submission order.
// Use Index to correlate results with the original input slice position.
type QueuedToolResult struct {
	Index  int           // Position in original inputs slice (0-based)
	Output proto.Message // The tool's output proto (nil if error)
	Error  error         // Error if execution failed (nil if success)
}

// ToolStreamCallback receives streaming events during tool execution.
// Implementations should handle events asynchronously and not block,
// as callback methods are invoked from the stream receiver goroutine.
//
// All callback methods are optional - implementations can choose to ignore
// events they don't care about by providing no-op methods.
type ToolStreamCallback interface {
	// OnProgress is called when the tool reports progress.
	//
	// percent: Progress percentage from 0-100, or 0 if indeterminate.
	// phase: Current execution phase (e.g., "discovery", "scanning").
	// message: Human-readable status message.
	//
	// This is called frequently during long-running operations.
	// Implementations should avoid expensive operations in this callback.
	OnProgress(percent int, phase, message string)

	// OnPartial is called when the tool emits a partial result.
	//
	// output: A proto message of the tool's output type containing partial results.
	// incremental: If true, this result should be appended to previous partials.
	//              If false, this result replaces all previous partials.
	//
	// Partial results allow processing data as it becomes available rather than
	// waiting for complete execution.
	OnPartial(output proto.Message, incremental bool)

	// OnWarning is called when the tool emits a non-fatal warning.
	//
	// message: Human-readable warning message.
	// context: Additional context about where the warning occurred (may be empty).
	//
	// Warnings indicate recoverable errors or expected failures that don't
	// prevent the tool from continuing.
	OnWarning(message, context string)

	// OnError is called when the tool encounters an error.
	//
	// err: The error that occurred.
	// fatal: If true, the tool will terminate after this error.
	//        If false, the tool will continue execution.
	//
	// Fatal errors prevent any useful output. Non-fatal errors are logged
	// issues that don't stop execution.
	OnError(err error, fatal bool)
}

// MissionManager provides mission lifecycle operations for agents.
// This interface enables agents to autonomously create, run, monitor, and manage missions,
// supporting hierarchical agent architectures and autonomous operation patterns.
type MissionManager interface {
	// CreateMission creates a new mission from a mission definition.
	// The missionDef parameter should be a Gibson mission.Mission instance.
	// Returns mission metadata including the assigned mission ID.
	//
	// If the creating agent is part of a mission, the parent mission ID
	// will be automatically tracked for lineage.
	//
	// Example:
	//   missionDef := BuildReconMission()
	//   opts := &mission.CreateMissionOpts{
	//       Name: "Subdomain Enumeration",
	//       Constraints: &mission.MissionConstraints{
	//           MaxDuration: 30 * time.Minute,
	//           MaxTokens:   100000,
	//       },
	//   }
	//   info, err := harness.CreateMission(ctx, missionDef, targetID, opts)
	CreateMission(ctx context.Context, missionDef any, targetID string, opts *mission.CreateMissionOpts) (*mission.MissionInfo, error)

	// RunMission queues a mission for execution.
	// This method is non-blocking by default and returns immediately after queuing.
	// Use WaitForMission to block until the mission completes.
	//
	// Returns an error if:
	//   - The mission does not exist
	//   - The mission is already running
	//   - The mission is in a terminal state (completed, failed, cancelled)
	//
	// Example:
	//   err := harness.RunMission(ctx, missionID, nil)
	//   if err != nil {
	//       return fmt.Errorf("failed to start mission: %w", err)
	//   }
	RunMission(ctx context.Context, missionID string, opts *mission.RunMissionOpts) error

	// GetMissionStatus returns the current state of a mission.
	// Returns detailed status information including progress, findings count,
	// token usage, and error messages if applicable.
	//
	// Returns an error if the mission does not exist.
	//
	// Example:
	//   status, err := harness.GetMissionStatus(ctx, missionID)
	//   if err != nil {
	//       return err
	//   }
	//   log.Printf("Mission %s: %s (%.1f%% complete)", missionID, status.Status, status.Progress*100)
	GetMissionStatus(ctx context.Context, missionID string) (*mission.MissionStatusInfo, error)

	// WaitForMission blocks until a mission completes or the timeout expires.
	// Returns the final mission result including findings and output.
	//
	// The timeout parameter specifies how long to wait. Use 0 for no timeout.
	// Returns context.DeadlineExceeded if the timeout is reached before completion.
	//
	// Example:
	//   result, err := harness.WaitForMission(ctx, missionID, 10*time.Minute)
	//   if err != nil {
	//       return fmt.Errorf("mission wait failed: %w", err)
	//   }
	//   log.Printf("Mission completed with %d findings", len(result.Findings))
	WaitForMission(ctx context.Context, missionID string, timeout time.Duration) (*mission.MissionResult, error)

	// ListMissions returns missions matching the provided filter criteria.
	// Returns an empty slice if no missions match the filter.
	//
	// The filter supports:
	//   - Status filtering (pending, running, completed, etc.)
	//   - Target ID filtering
	//   - Parent mission ID filtering (for finding child missions)
	//   - Time range filtering
	//   - Tag filtering
	//   - Pagination (limit/offset)
	//
	// Example:
	//   filter := &mission.MissionFilter{
	//       Status:   &statusRunning,
	//       TargetID: &currentTargetID,
	//       Limit:    10,
	//   }
	//   missions, err := harness.ListMissions(ctx, filter)
	ListMissions(ctx context.Context, filter *mission.MissionFilter) ([]*mission.MissionInfo, error)

	// CancelMission requests cancellation of a running mission.
	// The mission will be gracefully interrupted and its status will transition to "cancelled".
	//
	// This operation is idempotent - calling it on an already cancelled or
	// completed mission returns success.
	//
	// Example:
	//   err := harness.CancelMission(ctx, missionID)
	//   if err != nil {
	//       log.Printf("Failed to cancel mission: %v", err)
	//   }
	CancelMission(ctx context.Context, missionID string) error

	// GetMissionResults returns the final results of a completed mission.
	// Results include findings, output data, and execution metrics.
	//
	// Returns an error if:
	//   - The mission does not exist
	//   - The mission has not completed yet (use WaitForMission to wait)
	//
	// Example:
	//   result, err := harness.GetMissionResults(ctx, missionID)
	//   if err != nil {
	//       return err
	//   }
	//   for _, finding := range result.Findings {
	//       log.Printf("Found %s: %s", finding.Severity, finding.Title)
	//   }
	GetMissionResults(ctx context.Context, missionID string) (*mission.MissionResult, error)
}

// ── Harness capability groups ───────────────────────────────────────────────
//
// Harness is composed of these rather than carrying 27 methods flat. The
// grouping is not cosmetic. Before it, the groups existed only as section
// comments and they had drifted: DelegateToAgent, ListAgents and SubmitFinding
// sat under "Plugin Access Methods", Observe and WorldView under "Context
// Access", and the five LLM methods had no header at all. Comments do not
// compile. MissionManager was the one group already expressed as a type, and
// the one that had not rotted.
//
// Groups also let a caller ask for exactly the capability it needs, so a
// function that only reads cannot emit.
//
// See docs/adr/0002-harness-capability-groups.md.

// LLMCaller is the model-facing surface: one completion call per shape an agent needs.
type LLMCaller interface {
	// LLM Access Methods
	//
	// These methods provide access to LLM completions through named slots.
	// Slots are configured based on the agent's LLMSlots() requirements.

	// Complete performs a single LLM completion request.
	// The slot parameter identifies which LLM to use (e.g., "primary", "vision").
	// Options can be provided to customize temperature, max tokens, etc.
	Complete(ctx context.Context, slot string, messages []llm.Message, opts ...llm.CompletionOption) (*llm.CompletionResponse, error)

	// CompleteWithTools performs a completion with tool calling enabled.
	// The LLM can request to invoke tools and will receive tool results in subsequent turns.
	CompleteWithTools(ctx context.Context, slot string, messages []llm.Message, tools []llm.ToolDef) (*llm.CompletionResponse, error)

	// Stream performs a streaming completion request.
	// Returns a channel that yields incremental chunks as they arrive.
	// The channel will be closed when the stream completes or an error occurs.
	Stream(ctx context.Context, slot string, messages []llm.Message) (<-chan llm.StreamChunk, error)

	// CompleteStructured performs a completion with provider-native structured output.
	// The response schema is derived from the provided struct type.
	// For Anthropic: uses tool_use pattern (schema becomes a tool definition)
	// For OpenAI: uses response_format with json_schema
	// The prompt should be natural language - no JSON instructions needed.
	// Returns a pointer to the populated struct or an error.
	// The schema parameter should be an instance of the struct type (e.g., MyStruct{}).
	CompleteStructured(ctx context.Context, slot string, messages []llm.Message, schema any) (any, error)

	// CompleteStructuredAny is an alias for CompleteStructured for compatibility.
	CompleteStructuredAny(ctx context.Context, slot string, messages []llm.Message, schema any) (any, error)
}

// ToolCaller is dispatching tools and reading their results.
type ToolCaller interface {

	// Tool Access Methods
	//
	// These methods provide access to external tools (e.g., HTTP client, shell, browser).

	// CallToolProto invokes a tool by name with proto message input/output.
	// The request and response parameters should be pointers to proto message types.
	CallToolProto(ctx context.Context, name string, request proto.Message, response proto.Message) error

	// CallToolProtoStream invokes a tool with streaming event callbacks.
	// This enables real-time progress updates, partial results, and warnings during tool execution.
	//
	// The callback parameter receives events as they arrive:
	//   - OnProgress: Progress percentage and status updates
	//   - OnPartial: Incremental or replacement partial results
	//   - OnWarning: Non-fatal warnings during execution
	//   - OnError: Fatal or non-fatal errors
	//
	// The method blocks until the tool completes or fails, returning the final output
	// or an error. The final output is also delivered via callback.OnPartial() before returning.
	//
	// Example:
	//   callback := &MyToolCallback{}
	//   output := &pb.NmapOutput{}
	//   err := h.CallToolProtoStream(ctx, "mytool", &pb.NmapInput{Target: "192.168.1.0/24"}, output, callback)
	//   if err != nil {
	//       return fmt.Errorf("mytool failed: %w", err)
	//   }
	//   // output now contains the final result
	CallToolProtoStream(ctx context.Context, toolName string, input proto.Message, output proto.Message, callback ToolStreamCallback) error

	// ListTools returns descriptors for all available tools.
	// This can be used to discover available functionality.
	ListTools(ctx context.Context) ([]tool.Descriptor, error)

	// QueueToolWork submits multiple tool executions to a Redis queue for parallel processing.
	// Each input in the slice will be queued separately and processed by available tool workers.
	// Returns a job ID that can be used to retrieve results via ToolResults.
	//
	// This method enables high-throughput parallel tool execution by distributing work
	// across multiple workers. Results arrive in completion order (not input order),
	// so use QueuedToolResult.Index to correlate results with inputs.
	//
	// The method returns immediately after queueing - it does not wait for execution.
	// Use ToolResults to receive results as they complete.
	//
	// Example:
	//   // Queue 100 port scans in parallel
	//   inputs := make([]proto.Message, 100)
	//   for i, target := range targets {
	//       inputs[i] = &pb.NmapInput{Target: target}
	//   }
	//   jobID, err := h.QueueToolWork(ctx, "mytool", inputs)
	//   if err != nil {
	//       return fmt.Errorf("failed to queue scans: %w", err)
	//   }
	//
	//   // Process results as they arrive
	//   for result := range h.ToolResults(ctx, jobID) {
	//       if result.Error != nil {
	//           log.Printf("Scan %d failed: %v", result.Index, result.Error)
	//           continue
	//       }
	//       output := result.Output.(*pb.NmapOutput)
	//       log.Printf("Scan %d complete: %d hosts found", result.Index, len(output.Hosts))
	//   }
	QueueToolWork(ctx context.Context, toolName string, inputs []proto.Message) (string, error)

	// ToolResults returns a channel that receives results as tool executions complete.
	// The channel is closed when all results have been received or an error occurs.
	//
	// Results may arrive in any order depending on execution time. Use QueuedToolResult.Index
	// to correlate results with the original input slice positions from QueueToolWork.
	//
	// The channel will be closed automatically when:
	//   - All results have been received (one per input)
	//   - The context is cancelled
	//   - A fatal error occurs retrieving results
	//
	// Example:
	//   for result := range h.ToolResults(ctx, jobID) {
	//       if result.Error != nil {
	//           log.Printf("Tool execution %d failed: %v", result.Index, result.Error)
	//           continue
	//       }
	//       // Process successful result
	//       processOutput(result.Index, result.Output)
	//   }
	ToolResults(ctx context.Context, jobID string) <-chan QueuedToolResult
}

// PluginCaller is invoking platform plugins and discovering what is installed.
type PluginCaller interface {

	// Plugin Access Methods
	//
	// These methods provide access to plugins (modular extensions to the framework).

	// QueryPlugin sends a query to a plugin and returns the result.
	// The method parameter identifies the plugin operation to invoke.
	// The params provide input data for the operation.
	QueryPlugin(ctx context.Context, name string, method string, params map[string]any) (any, error)

	// ListPlugins returns descriptors for all available plugins.
	ListPlugins(ctx context.Context) ([]plugin.Descriptor, error)
}

// Delegator is handing sub-tasks to other enrolled agents.
type Delegator interface {

	// Agent Delegation Methods
	//
	// These methods allow agents to delegate tasks to other agents.

	// DelegateToAgent assigns a task to another agent for execution.
	// This enables hierarchical agent architectures and specialization.
	DelegateToAgent(ctx context.Context, name string, task Task) (Result, error)

	// ListAgents returns descriptors for all available agents.
	ListAgents(ctx context.Context) ([]Descriptor, error)
}

// WorldEmitter is emitting into the tenant World.
//
// Deliberately separate from WorldReader. ADR-0012 makes the projector the sole
// graph writer, and splitting emit from read gives that constraint a type: a
// function taking a WorldReader cannot emit, and the compiler enforces it at
// every call site rather than a reviewer catching it sometimes.
//
// "Emitter" rather than "Writer" because these are World emissions the projector
// later consumes, not graph writes.
type WorldEmitter interface {

	// Finding Management Methods
	//
	// These methods manage security findings discovered during testing.

	// SubmitFinding records a new security finding (an emit). The finding flows
	// into the brain as an observation; agents do not query findings back — the
	// relevant world state is ambiently projected to them (ADR-0001).
	SubmitFinding(ctx context.Context, f *finding.Finding) error

	// Observation Emit
	//
	// Observe emits a typed observation into the World (ADR-0007). This is the
	// agent's write path: agents report what they saw (a host, its ports/services,
	// …) and the brain resolves identity and topology — agents never author graph
	// nodes or relationships, and never query the graph back (the relevant world
	// state is ambiently projected to them, ADR-0001). Scope is derived server-side
	// from mission context, not carried on the observation.
	Observe(ctx context.Context, obs Observation) error
}

// WorldReader is reading the tenant World. See WorldEmitter for why the halves are split.
type WorldReader interface {

	// World Read
	//
	// WorldView returns the agent's slice of the tenant World (ADR-0012) — the
	// read half of the emit-only worker contract, and the counterpart to Observe.
	//
	// The slice is projected by the daemon from the mission record it created:
	// the tenant that owns the World and the scope that bounds the slice are both
	// read there. Nothing the agent passes selects either, so an agent cannot ask
	// for another tenant's World or for a wider slice — those are unrepresentable
	// rather than refused.
	//
	// focus narrows the result to entities the agent was already shown, named by
	// the opaque handles it received, and returns those at full detail. A handle
	// that was not issued to this agent is an error; focus can only zoom into a
	// slice, never widen it.
	WorldView(ctx context.Context, focus ...Handle) (WorldView, error)
}

// Planner is planning context and the hints an agent reports back.
type Planner interface {

	// Planning Context Methods
	//
	// These methods provide access to planning context and allow agents to
	// report feedback to the planning system.

	// PlanContext returns the planning context for the current execution.
	// Returns nil if no planning context is available (non-planned execution).
	// Agents can use this to access mission goals, step budgets, and position
	// in the overall plan.
	PlanContext() planning.PlanningContext

	// ReportStepHints allows agents to provide feedback to the planning system.
	// Agents can report confidence levels, suggest next steps, recommend replanning,
	// and share key findings that should influence future planning decisions.
	// This method is a no-op if planning is not enabled.
	ReportStepHints(ctx context.Context, hints *planning.StepHints) error
}

// WorkspaceAccess is access to cloned repositories with integrated editing and Git operations.
type WorkspaceAccess interface {

	// Workspace Access Methods
	//
	// These methods provide access to Git repository workspaces for code generation missions.
	// Workspaces provide isolated access to cloned repositories with integrated editing and Git operations.

	// Workspace returns the primary workspace for single-repository missions.
	// This is a convenience method that returns the first workspace defined in the mission configuration.
	// Returns nil if no workspaces are configured for this mission.
	//
	// Example:
	//   ws := harness.Workspace()
	//   if ws == nil {
	//       return errors.New("no workspace configured")
	//   }
	//   content, err := ws.ReadFile(ctx, "main.go")
	Workspace() workspace.Workspace

	// Workspaces returns all workspaces keyed by repository name.
	// For multi-repository missions, use this to access specific workspaces by name.
	// Returns an empty map if no workspaces are configured.
	//
	// Example:
	//   workspaces := harness.Workspaces()
	//   if ws, ok := workspaces["backend"]; ok {
	//       editor := ws.Editor()
	//       // Perform editing operations
	//   }
	Workspaces() map[string]workspace.Workspace
}

// Harness provides the runtime environment for agent execution.
// It provides access to LLMs, tools, plugins, findings, memory, and observability.
//
// Defense-in-depth authorization: Harness includes an Authorize method that
// tools, plugins, and agents must call before any sensitive operation (executing
// a scan, calling an external API, writing to a filesystem, etc.). This is the
// second layer of enforcement complementing the daemon's pre-dispatch FGA check.
// Each Authorize call contacts the daemon in real time, enabling mid-mission
// revocation to take effect at the next sensitive operation rather than at
// mission-dispatch time.
type Harness interface {
	// Capability groups — defined above.
	LLMCaller
	ToolCaller
	PluginCaller
	Delegator
	WorldEmitter
	WorldReader
	Planner
	WorkspaceAccess
	KnowledgeReader

	// Mission Management Methods
	//
	// These methods provide mission lifecycle management for autonomous operation.
	// Agents can create, run, monitor, and manage missions programmatically,
	// enabling hierarchical agent architectures and autonomous campaigns.
	MissionManager

	// Ungrouped: single-purpose accessors belonging to no cluster.
	// Authorization Methods
	//
	// Authorize checks whether the current work execution is permitted to
	// perform the specified action on the specified resource. Returns nil
	// if the action is allowed, or a typed error on denial.
	//
	// Tools, plugins, and agents SHOULD call Authorize before every sensitive
	// operation — network scans, external API calls, filesystem writes, etc. —
	// as a defense-in-depth measure that complements the daemon's pre-dispatch
	// authorization check from the FGA layer.
	//
	// action must be one of the constants defined in the harness package:
	//   harness.ActionExecute, harness.ActionConfigure,
	//   harness.ActionRead,    harness.ActionWrite.
	//
	// resource must be in the format "<type>:<name>", e.g.:
	//   "tool:mytool", "system:github.com", "component:plugin-gitlab".
	//
	// Error handling:
	//   - nil                          — allowed, proceed
	//   - ErrUnauthorized              — FGA denied; log and return PERMISSION_DENIED
	//   - ErrAuthzServiceUnavailable   — daemon/FGA unreachable; fail-closed by default
	//   - ErrInvalidAction             — malformed action or resource; treat as deny
	//   - ErrWorkExpired               — work envelope TTL exceeded; reject work item
	//
	// When the daemon does not support the Authorize RPC (rolling upgrade),
	// the harness client degrades gracefully to allow.
	Authorize(ctx context.Context, action, resource string) error

	// Context Access
	//
	// These methods provide access to mission and target context.

	// Mission returns the current mission context.
	// This includes mission parameters, constraints, and metadata.
	Mission() types.MissionContext

	// Target returns information about the target being tested.
	// This includes target URL, type, authentication, and metadata.
	Target() types.TargetInfo

	// Observability
	//
	// These methods provide access to logging, tracing, and metrics.

	// Tracer returns an OpenTelemetry tracer for distributed tracing.
	// Agents should create spans for major operations to enable observability.
	Tracer() trace.Tracer

	// Logger returns a structured logger for the agent.
	// All log output should use this logger for consistent formatting.
	Logger() *slog.Logger

	// TokenUsage returns the token usage tracker for this execution.
	// This tracks token consumption across all LLM slots.
	TokenUsage() llm.TokenTracker
}

// Descriptor provides metadata about an agent.
// This is used for agent discovery and selection.
type Descriptor struct {
	// Name is the unique identifier for the agent.
	Name string

	// Version is the semantic version of the agent.
	Version string

	// Description explains what the agent does.
	Description string

	// Capabilities lists the security testing capabilities the agent provides.
	Capabilities []string

	// TargetTypes lists the types of targets the agent can test.
	TargetTypes []string

	// TechniqueTypes lists the attack techniques the agent employs.
	TechniqueTypes []string
}
