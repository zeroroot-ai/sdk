// Schema evolution policy (mission-schema-canonicalization Requirement 7):
//
//   1. Enum values (MissionStatus, etc.) are append-only. No renumbers,
//      no deletions. Deprecated values use `[deprecated = true]` plus a
//      `// reserved` comment.
//   2. Message field numbers are append-only. No renumbers, no reuse.
//      Type-of-field changes ARE breaking and require a ship sequence
//      across SDK + every consumer (see ADR 0004 for the precedent).
//   3. Cross-file consistency: this file consumes
//      gibson.mission.v1.MissionDefinition AND
//      gibson.mission.v1.MissionConstraints (the canonical platform-wide
//      constraint shape per ADR 0004). The daemon-local MissionConstraints
//      message was removed in the same change. Any breaking change to the
//      canonical SDK type must be coordinated with this file under the
//      canonical ship sequence.

// ---------------------------------------------------------------------------
// Connection RPCs — request/response messages
// ---------------------------------------------------------------------------

// ---------------------------------------------------------------------------
// Event types — shared by Subscribe, RunMission, and ResumeMission
// ---------------------------------------------------------------------------

// ---------------------------------------------------------------------------
// Subscribe RPC — request/response messages
// ---------------------------------------------------------------------------

// ---------------------------------------------------------------------------
// Mission RPCs — request/response messages
// ---------------------------------------------------------------------------

// MissionConstraints message removed under ADR 0004
// (https://github.com/zeroroot-ai/docs/blob/main/adr/0004-canonical-mission-constraints.md).
// The canonical type is gibson.mission.v1.MissionConstraints; daemon-only
// fields (max_turns_per_agent, allowed_techniques, blocked_techniques,
// max_tokens_per_call) were promoted into the SDK type with normalized
// representation (google.protobuf.Duration over int32 seconds; repeated
// string for allow/block lists). Every consumer references the SDK type
// directly — no compat shim, no translator, one type end-to-end.

// The local MissionDefinition summary message (previously defined here) has been
// removed under spec mission-api-only-cleanup (Phase 3); the canonical definition
// now lives in gibson.mission.v1.MissionDefinition. Use MissionDefinitionInfo
// (above) for summary listings and gibson.mission.v1.MissionDefinition for
// definition registration via CreateMissionDefinition.

// InlineTargetConfig, InlineWorkflowConfig, InlineNodeConfig, InlineEdgeConfig, and the
// helper Seed message were removed under spec mission-api-only-cleanup (Phase 2).
// Missions reference registered targets and mission definitions by ID only —
// the daemon no longer accepts inline mission construction over the wire.

// ---------------------------------------------------------------------------
// Target RPCs — request/response messages
// ---------------------------------------------------------------------------

// ---------------------------------------------------------------------------
// Agent RPCs — request/response messages
// ---------------------------------------------------------------------------

// ---------------------------------------------------------------------------
// Tool RPCs — request/response messages
// ---------------------------------------------------------------------------

// ---------------------------------------------------------------------------
// Plugin RPCs — request/response messages
// ---------------------------------------------------------------------------

// ---------------------------------------------------------------------------
// Component lifecycle RPCs — request/response messages
// ---------------------------------------------------------------------------

// ---------------------------------------------------------------------------
// GetMyPermissions — permission summary for the current authenticated user
// ---------------------------------------------------------------------------

// ---------------------------------------------------------------------------
// ListMyMemberships — tenant-membership discovery for the current user
// ---------------------------------------------------------------------------

// ────────────────────────────────────────────────────────────────────────────
// GetMissionDefinition — mission-author-experience M5 (gibson#134)
// ────────────────────────────────────────────────────────────────────────────

// ────────────────────────────────────────────────────────────────────────────
// Mission flow-chart projection + layout store — MissionGraph epic (sdk#278)
//
// The daemon projects a mission definition (the pure work DAG) into a
// renderable graph and owns a SEPARATE layout store for hand-arranged
// positions. Presentation never lives in mission_definition.proto; the
// dashboard is a pure client of these RPCs.
// ────────────────────────────────────────────────────────────────────────────

// ────────────────────────────────────────────────────────────────────────────
// Checkpoint browser — mission-checkpointing R13/R14/R15
// ────────────────────────────────────────────────────────────────────────────

// ---------------------------------------------------------------------------
// CUE mission editor messages (mission-cue-editor epic)
//
// Collapsed onto DaemonService from gibson.daemon.admin.v1 (platform-sdk) so
// ADK / SDK users can drive the CUE language service with a single user token.
// Part of the one-customer-surface epic (zeroroot-ai/.github#143).
// ---------------------------------------------------------------------------
package daemonpb

import (
	"github.com/zeroroot-ai/sdk/api/proto/gibson/common/v1:commonpb"
	"github.com/zeroroot-ai/sdk/api/proto/gibson/mission/v1:missionpb"
	"github.com/zeroroot-ai/sdk/api/proto/gibson/target/v1:targetpb"
	"time"
	"github.com/zeroroot-ai/sdk/api/proto/gibson/manifest/v1:manifestpb"
)

// MissionStatus represents the execution status of a mission.
#MissionStatus:
	#MISSION_STATUS_UNSPECIFIED |
	#MISSION_STATUS_PENDING |
	#MISSION_STATUS_RUNNING |
	#MISSION_STATUS_PAUSED |
	#MISSION_STATUS_COMPLETED |
	#MISSION_STATUS_FAILED |
	#MISSION_STATUS_CANCELLED

#MISSION_STATUS_UNSPECIFIED: 0
#MISSION_STATUS_PENDING:     1
#MISSION_STATUS_RUNNING:     2
#MISSION_STATUS_PAUSED:      3
#MISSION_STATUS_COMPLETED:   4
#MISSION_STATUS_FAILED:      5
#MISSION_STATUS_CANCELLED:   6

#MissionStatus_value: {
	MISSION_STATUS_UNSPECIFIED: 0
	MISSION_STATUS_PENDING:     1
	MISSION_STATUS_RUNNING:     2
	MISSION_STATUS_PAUSED:      3
	MISSION_STATUS_COMPLETED:   4
	MISSION_STATUS_FAILED:      5
	MISSION_STATUS_CANCELLED:   6
}

// RenewCapabilityGrantRequest carries the identifiers needed to
// validate the renewal request. The currently-presented CG-JWT (in
// the X-Capability-Grant header) authorizes the call; this request
// body just identifies which task is being renewed.
//
// Spec: unified-identity-and-authorization Requirement 5.8.
#RenewCapabilityGrantRequest: {
	// agent_id is the agent's Zitadel service-account ID. Must match
	// the sub claim of the presented CG-JWT.
	agentId?: string @protobuf(1,string,name=agent_id)

	// mission_id names the mission. Must match the mission_id claim
	// of the presented CG-JWT.
	missionId?: string @protobuf(2,string,name=mission_id)

	// task_id names the specific task. Must match the task_id claim
	// of the presented CG-JWT.
	taskId?: string @protobuf(3,string,name=task_id)
}

// RenewCapabilityGrantResponse returns the freshly-minted CG-JWT
// plus its claimed expiry timestamp (Unix seconds, UTC) for client-
// side scheduling of the next renewal.
#RenewCapabilityGrantResponse: {
	// capability_grant is the compact-serialized JWT to attach as the
	// X-Capability-Grant header on subsequent harness callbacks.
	capabilityGrant?: string @protobuf(1,string,name=capability_grant)

	// expires_at_unix is the new exp claim, Unix seconds. Clients use
	// this to schedule renewal before expiry.
	expiresAtUnix?: int64 @protobuf(2,int64,name=expires_at_unix)
}

// ConnectRequest initiates a client connection to the daemon.
#ConnectRequest: {
	// client_version is the version of the Gibson CLI client
	clientVersion?: string @protobuf(1,string,name=client_version)

	// client_id is an optional unique identifier for this client
	clientId?: string @protobuf(2,string,name=client_id)
}

// ConnectResponse returns connection metadata.
#ConnectResponse: {
	// daemon_version is the version of the running daemon
	daemonVersion?: string @protobuf(1,string,name=daemon_version)

	// session_id is a unique identifier for this client session
	sessionId?: string @protobuf(2,string,name=session_id)

	// grpc_address is the address the daemon is listening on
	grpcAddress?: string @protobuf(3,string,name=grpc_address)
}

// PingRequest is an empty health check request.
#PingRequest: {}

// PingResponse confirms the daemon is responsive.
#PingResponse: {
	// timestamp is the server time when the ping was received
	timestamp?: int64 @protobuf(1,int64)
}

// StatusRequest queries daemon status.
#StatusRequest: {}

// StatusResponse returns complete daemon status information.
#StatusResponse: {
	// running indicates if the daemon is running (always true if responding)
	running?: bool @protobuf(1,bool)

	// pid is the process ID of the daemon
	pid?: int32 @protobuf(2,int32)

	// start_time is when the daemon started (Unix timestamp)
	startTime?: int64 @protobuf(3,int64,name=start_time)

	// uptime is the human-readable uptime string
	uptime?: string @protobuf(4,string)

	// grpc_address is the gRPC server address
	grpcAddress?: string @protobuf(5,string,name=grpc_address)

	// registry_type is the type of registry (embedded, external)
	registryType?: string @protobuf(6,string,name=registry_type)

	// registry_addr is the registry endpoint address
	registryAddr?: string @protobuf(7,string,name=registry_addr)

	// callback_addr is the callback server address
	callbackAddr?: string @protobuf(8,string,name=callback_addr)

	// agent_count is the number of registered agents
	agentCount?: int32 @protobuf(9,int32,name=agent_count)

	// mission_count is the total number of missions
	missionCount?: int32 @protobuf(10,int32,name=mission_count)

	// active_mission_count is the number of currently running missions
	activeMissionCount?: int32 @protobuf(11,int32,name=active_mission_count)
}

// OperationResult represents the unified result of a long-running operation (attack or mission).
// This provides typed metrics instead of JSON-encoded strings.
#OperationResult: {
	// status of the operation ("success", "failed", "timeout", "cancelled")
	status?: string @protobuf(1,string)

	// duration_ms is the total duration in milliseconds
	durationMs?: int64 @protobuf(2,int64,name=duration_ms)

	// started_at is the Unix timestamp (milliseconds) when the operation started
	startedAt?: int64 @protobuf(3,int64,name=started_at)

	// completed_at is the Unix timestamp (milliseconds) when the operation completed
	completedAt?: int64 @protobuf(4,int64,name=completed_at)

	// turns_used is the number of agent turns/iterations executed
	turnsUsed?: int32 @protobuf(5,int32,name=turns_used)

	// tokens_used is the total LLM tokens consumed
	tokensUsed?: int64 @protobuf(6,int64,name=tokens_used)

	// nodes_executed is the number of mission nodes that ran successfully
	nodesExecuted?: int32 @protobuf(7,int32,name=nodes_executed)

	// nodes_failed is the number of mission nodes that failed
	nodesFailed?: int32 @protobuf(8,int32,name=nodes_failed)

	// findings_count is the total number of findings discovered
	findingsCount?: int32 @protobuf(9,int32,name=findings_count)

	// critical_count is the number of critical severity findings
	criticalCount?: int32 @protobuf(10,int32,name=critical_count)

	// high_count is the number of high severity findings
	highCount?: int32 @protobuf(11,int32,name=high_count)

	// medium_count is the number of medium severity findings
	mediumCount?: int32 @protobuf(12,int32,name=medium_count)

	// low_count is the number of low severity findings
	lowCount?: int32 @protobuf(13,int32,name=low_count)

	// error_message contains the error message if status == "failed"
	errorMessage?: string @protobuf(14,string,name=error_message)

	// error_code contains a machine-readable error code if status == "failed"
	errorCode?: string @protobuf(15,string,name=error_code)
}

// FindingInfo describes a discovered vulnerability.
#FindingInfo: {
	// id is the unique finding identifier
	id?: string @protobuf(1,string)

	// title is the finding title
	title?: string @protobuf(2,string)

	// severity is the severity level (info, low, medium, high, critical)
	severity?: string @protobuf(3,string)

	// category is the finding category
	category?: string @protobuf(4,string)

	// description is the detailed finding description
	description?: string @protobuf(5,string)

	// technique is the MITRE ATT&CK or ATLAS technique ID
	technique?: string @protobuf(6,string)

	// evidence contains supporting evidence
	evidence?: string @protobuf(7,string)

	// timestamp is when the finding was discovered (Unix timestamp)
	timestamp?: int64 @protobuf(8,int64)
}

// MissionEvent represents a mission execution event.
#MissionEvent: {
	// event_type identifies the type of event
	eventType?: string @protobuf(1,string,name=event_type)

	// timestamp is when the event occurred (Unix timestamp)
	timestamp?: int64 @protobuf(2,int64)

	// mission_id is the unique mission identifier
	missionId?: string @protobuf(3,string,name=mission_id)

	// node_id is the mission node ID (if applicable)
	nodeId?: string @protobuf(4,string,name=node_id)

	// message is a human-readable event message
	message?: string @protobuf(5,string)

	// data contains event-specific data (typed map)
	data?: commonpb.#TypedMap @protobuf(6,gibson.common.v1.TypedMap)

	// error contains error information if the event represents an error
	error?: string @protobuf(7,string)

	// result contains typed operation metrics (for mission.completed events)
	result?: #OperationResult @protobuf(8,OperationResult)
}

// AgentEvent represents an agent lifecycle event.
#AgentEvent: {
	// event_type identifies the agent event type (registered, unregistered, health_change)
	eventType?: string @protobuf(1,string,name=event_type)

	// timestamp is when the event occurred (Unix timestamp)
	timestamp?: int64 @protobuf(2,int64)

	// agent_id is the agent identifier
	agentId?: string @protobuf(3,string,name=agent_id)

	// agent_name is the agent name
	agentName?: string @protobuf(4,string,name=agent_name)

	// message is a human-readable message
	message?: string @protobuf(5,string)

	// data contains event-specific data (typed map)
	data?: commonpb.#TypedMap @protobuf(6,gibson.common.v1.TypedMap)
}

// FindingEvent represents a finding discovery event.
#FindingEvent: {
	// event_type identifies the finding event type (discovered, updated)
	eventType?: string @protobuf(1,string,name=event_type)

	// timestamp is when the event occurred (Unix timestamp)
	timestamp?: int64 @protobuf(2,int64)

	// finding is the finding information
	finding?: #FindingInfo @protobuf(3,FindingInfo)

	// mission_id is the mission that discovered the finding
	missionId?: string @protobuf(4,string,name=mission_id)
}

// ToolEvent represents a tool execution event.
#ToolEvent: {
	// event_type identifies the tool event type (tool.started, tool.completed, tool.failed, tool.progress, tool.warning)
	eventType?: string @protobuf(1,string,name=event_type)

	// timestamp is when the event occurred (Unix timestamp)
	timestamp?: int64 @protobuf(2,int64)

	// tool_name is the name of the tool being executed
	toolName?: string @protobuf(3,string,name=tool_name)

	// agent_id is the agent identifier executing the tool
	agentId?: string @protobuf(4,string,name=agent_id)

	// agent_name is the agent name executing the tool
	agentName?: string @protobuf(5,string,name=agent_name)

	// mission_id is the mission context for this tool execution
	missionId?: string @protobuf(6,string,name=mission_id)

	// message is a human-readable event message
	message?: string @protobuf(7,string)

	// duration is the execution time in seconds (for completed/failed events)
	duration?: float64 @protobuf(8,double)

	// progress is the completion percentage (0-1 for progress events)
	progress?: float64 @protobuf(9,double)

	// error contains error information if the event represents an error
	error?: string @protobuf(10,string)

	// error_code contains a machine-readable error code
	errorCode?: string @protobuf(11,string,name=error_code)

	// warning contains warning information if the event represents a warning
	warning?: string @protobuf(12,string)

	// warning_severity is the severity level (low, medium, high)
	warningSeverity?: string @protobuf(13,string,name=warning_severity)

	// data contains event-specific data (typed map)
	data?: commonpb.#TypedMap @protobuf(14,gibson.common.v1.TypedMap)
}

// LLMEvent represents an LLM activity event.
#LLMEvent: {
	// event_type identifies the LLM event type (llm.request.started, llm.request.completed, llm.request.failed)
	eventType?: string @protobuf(1,string,name=event_type)

	// timestamp is when the event occurred (Unix timestamp)
	timestamp?: int64 @protobuf(2,int64)

	// agent_id is the agent identifier
	agentId?: string @protobuf(3,string,name=agent_id)

	// agent_name is the agent name
	agentName?: string @protobuf(4,string,name=agent_name)

	// model is the LLM model identifier (e.g., "claude-3-5-sonnet-20241022")
	model?: string @protobuf(5,string)

	// slot is the LLM slot (primary, fast, reasoning)
	slot?: string @protobuf(6,string)

	// message_count is the number of messages in the request
	messageCount?: int32 @protobuf(7,int32,name=message_count)

	// prompt_tokens is the number of input tokens
	promptTokens?: int32 @protobuf(8,int32,name=prompt_tokens)

	// completion_tokens is the number of output tokens
	completionTokens?: int32 @protobuf(9,int32,name=completion_tokens)

	// total_tokens is the sum of prompt and completion tokens
	totalTokens?: int32 @protobuf(10,int32,name=total_tokens)

	// duration_ms is the request duration in milliseconds
	durationMs?: float64 @protobuf(11,double,name=duration_ms)

	// cached indicates if the response was served from cache
	cached?: bool @protobuf(12,bool)

	// error contains error information if the event represents a failure
	error?: string @protobuf(13,string)

	// error_code identifies the error type (rate_limit, context_length, api_error, timeout)
	errorCode?: string @protobuf(14,string,name=error_code)

	// will_retry indicates if the failed request will be retried
	willRetry?: bool @protobuf(15,bool,name=will_retry)
}

// OrchestratorEvent represents an orchestrator decision event.
#OrchestratorEvent: {
	// event_type identifies the orchestrator event type (orchestrator.decision, orchestrator.approval_required)
	eventType?: string @protobuf(1,string,name=event_type)

	// timestamp is when the event occurred (Unix timestamp)
	timestamp?: int64 @protobuf(2,int64)

	// mission_id is the mission identifier
	missionId?: string @protobuf(3,string,name=mission_id)

	// iteration is the orchestrator iteration number
	iteration?: int32 @protobuf(4,int32)

	// action is the orchestrator action (execute_agent, skip_node, wait, complete, request_approval)
	action?: string @protobuf(5,string)

	// target_node_id is the mission node ID being targeted
	targetNodeId?: string @protobuf(6,string,name=target_node_id)

	// target_agent_name is the agent name being targeted
	targetAgentName?: string @protobuf(7,string,name=target_agent_name)

	// confidence is the decision confidence score (0-1)
	confidence?: float64 @protobuf(8,double)

	// reasoning is the orchestrator's reasoning (max 500 chars in practice)
	reasoning?: string @protobuf(9,string)

	// tokens_used is the number of tokens consumed for this decision
	tokensUsed?: int32 @protobuf(10,int32,name=tokens_used)

	// latency_ms is the decision latency in milliseconds
	latencyMs?: float64 @protobuf(11,double,name=latency_ms)

	// approval_id is set when approval is required
	approvalId?: string @protobuf(12,string,name=approval_id)

	// risk is the risk level (low, medium, high, critical)
	risk?: string @protobuf(13,string)

	// timeout_seconds is the timeout for approval requests
	timeoutSeconds?: int32 @protobuf(14,int32,name=timeout_seconds)
}

// Event represents a generic daemon event.
#Event: {
	// event_type identifies the type of event
	eventType?: string @protobuf(1,string,name=event_type)

	// timestamp is when the event occurred (Unix timestamp)
	timestamp?: int64 @protobuf(2,int64)

	// source is the event source (mission, agent, daemon, etc.)
	source?: string @protobuf(3,string)

	// data contains event-specific data (typed map)
	data?: commonpb.#TypedMap @protobuf(4,gibson.common.v1.TypedMap)
	// Specific event types (only one will be set)
	{} | {
		missionEvent: #MissionEvent @protobuf(5,MissionEvent,name=mission_event)
	} | {
		agentEvent: #AgentEvent @protobuf(7,AgentEvent,name=agent_event)
	} | {
		findingEvent: #FindingEvent @protobuf(8,FindingEvent,name=finding_event)
	} | {
		toolEvent: #ToolEvent @protobuf(9,ToolEvent,name=tool_event)
	} | {
		llmEvent: #LLMEvent @protobuf(10,LLMEvent,name=llm_event)
	} | {
		orchestratorEvent: #OrchestratorEvent @protobuf(11,OrchestratorEvent,name=orchestrator_event)
	}
}

// SubscribeRequest establishes an event stream.
#SubscribeRequest: {
	// event_types filters which event types to receive (empty = all)
	eventTypes?: [...string] @protobuf(1,string,name=event_types)

	// mission_id filters to a specific mission (empty = all)
	missionId?: string @protobuf(2,string,name=mission_id)
}

// SubscribeResponse wraps an Event for the Subscribe streaming RPC.
#SubscribeResponse: {
	// event_type identifies the type of event
	eventType?: string @protobuf(1,string,name=event_type)

	// timestamp is when the event occurred (Unix timestamp)
	timestamp?: int64 @protobuf(2,int64)

	// source is the event source (mission, agent, daemon, etc.)
	source?: string @protobuf(3,string)

	// data contains event-specific data (typed map)
	data?: commonpb.#TypedMap @protobuf(4,gibson.common.v1.TypedMap)
	// Specific event types (only one will be set)
	{} | {
		missionEvent: #MissionEvent @protobuf(5,MissionEvent,name=mission_event)
	} | {
		agentEvent: #AgentEvent @protobuf(7,AgentEvent,name=agent_event)
	} | {
		findingEvent: #FindingEvent @protobuf(8,FindingEvent,name=finding_event)
	} | {
		toolEvent: #ToolEvent @protobuf(9,ToolEvent,name=tool_event)
	} | {
		llmEvent: #LLMEvent @protobuf(10,LLMEvent,name=llm_event)
	} | {
		orchestratorEvent: #OrchestratorEvent @protobuf(11,OrchestratorEvent,name=orchestrator_event)
	}
}

// RunMissionRequest starts a mission execution.
// API-only: missions are invoked by reference — no YAML, no file paths.
#RunMissionRequest: {
	// mission_definition_id is the ID of a registered mission definition to execute.
	missionDefinitionId?: string @protobuf(1,string,name=mission_definition_id)

	// target_id is the ID of a registered target the mission runs against.
	targetId?: string @protobuf(2,string,name=target_id)

	// variables contains mission variables to override
	variables?: {
		[string]: string
	} @protobuf(3,map[string]string)

	// memory_continuity defines how agent memory is shared across mission runs
	// Valid values: "isolated" (default), "inherit", "shared"
	memoryContinuity?: string @protobuf(4,string,name=memory_continuity)
}

// RunMissionResponse wraps a MissionEvent for the RunMission streaming RPC.
#RunMissionResponse: {
	// event_type identifies the type of event
	eventType?: string @protobuf(1,string,name=event_type)

	// timestamp is when the event occurred (Unix timestamp)
	timestamp?: int64 @protobuf(2,int64)

	// mission_id is the unique mission identifier
	missionId?: string @protobuf(3,string,name=mission_id)

	// node_id is the mission node ID (if applicable)
	nodeId?: string @protobuf(4,string,name=node_id)

	// message is a human-readable event message
	message?: string @protobuf(5,string)

	// data contains event-specific data (typed map)
	data?: commonpb.#TypedMap @protobuf(6,gibson.common.v1.TypedMap)

	// error contains error information if the event represents an error
	error?: string @protobuf(7,string)

	// result contains typed operation metrics (for mission.completed events)
	result?: #OperationResult @protobuf(8,OperationResult)
}

// ResumeMissionResponse wraps a MissionEvent for the ResumeMission streaming RPC.
#ResumeMissionResponse: {
	// event_type identifies the type of event
	eventType?: string @protobuf(1,string,name=event_type)

	// timestamp is when the event occurred (Unix timestamp)
	timestamp?: int64 @protobuf(2,int64)

	// mission_id is the unique mission identifier
	missionId?: string @protobuf(3,string,name=mission_id)

	// node_id is the mission node ID (if applicable)
	nodeId?: string @protobuf(4,string,name=node_id)

	// message is a human-readable event message
	message?: string @protobuf(5,string)

	// data contains event-specific data (typed map)
	data?: commonpb.#TypedMap @protobuf(6,gibson.common.v1.TypedMap)

	// error contains error information if the event represents an error
	error?: string @protobuf(7,string)

	// result contains typed operation metrics (for mission.completed events)
	result?: #OperationResult @protobuf(8,OperationResult)

	// checkpoint_metadata surfaces the source checkpoint metadata at the
	// start of a resumed stream so the dashboard can render the
	// "Resumed from checkpoint X" affordance. Populated on the first
	// event of a resume stream; nil/empty on subsequent events.
	// Spec: mission-checkpointing R9.
	checkpointMetadata?: #CheckpointMetadata @protobuf(9,CheckpointMetadata,name=checkpoint_metadata)
}

// CheckpointMetadata is the lightweight summary of the source checkpoint
// streamed back on a ResumeMission response so the dashboard can render
// "Resumed from checkpoint X". The full checkpoint payload is fetched
// separately via GetCheckpoint.
//
// Spec: mission-checkpointing R9.
#CheckpointMetadata: {
	// checkpoint_id is the unique identifier of the source checkpoint.
	checkpointId?: string @protobuf(1,string,name=checkpoint_id)

	// saved_at_unix_seconds is when the checkpoint was captured (Unix epoch seconds).
	savedAtUnixSeconds?: int64 @protobuf(2,int64,name=saved_at_unix_seconds)

	// super_step_number identifies which super-step boundary this checkpoint
	// captured (1-based; 0 if not super-step-aligned).
	superStepNumber?: int32 @protobuf(3,int32,name=super_step_number)

	// cadence_reason is a free-form classifier for why the checkpoint was
	// taken. Recognised values per R9.1: "super_step",
	// "parallel_group_complete", "approval_required", "graceful_shutdown".
	// Promoted to enum at v1.0.0.
	cadenceReason?: string @protobuf(4,string,name=cadence_reason)

	// size_bytes is the wire size of the checkpoint payload (advisory).
	sizeBytes?: int64 @protobuf(5,int64,name=size_bytes)
}

// StopMissionRequest requests mission termination.
#StopMissionRequest: {
	// mission_id is the identifier of the mission to stop
	missionId?: string @protobuf(1,string,name=mission_id)

	// force indicates whether to force-kill the mission (default: graceful)
	force?: bool @protobuf(2,bool)
}

// StopMissionResponse confirms mission stop request.
#StopMissionResponse: {
	// success indicates if the stop request was accepted
	success?: bool @protobuf(1,bool)

	// message provides additional context
	message?: string @protobuf(2,string)
}

// ListMissionsRequest queries mission list.
#ListMissionsRequest: {
	// active_only filters to only running missions
	activeOnly?: bool @protobuf(1,bool,name=active_only)

	// limit restricts the number of results
	limit?: int32 @protobuf(2,int32)

	// offset is the pagination offset
	offset?: int32 @protobuf(3,int32)

	// status_filter filters missions by status (running, completed, failed, cancelled)
	statusFilter?: string @protobuf(4,string,name=status_filter)

	// name_pattern filters missions by name using glob pattern matching
	namePattern?: string @protobuf(5,string,name=name_pattern)
}

// ListMissionsResponse returns mission list.
#ListMissionsResponse: {
	// missions is the list of missions
	missions?: [...#MissionInfo] @protobuf(1,MissionInfo)

	// total is the total count of missions (for pagination)
	total?: int32 @protobuf(2,int32)
}

// MissionInfo describes a mission.
#MissionInfo: {
	// id is the unique mission identifier
	id?: string @protobuf(1,string)

	// status is the mission status (running, completed, failed)
	status?: string @protobuf(3,string)

	// start_time is when the mission started (Unix timestamp)
	startTime?: int64 @protobuf(4,int64,name=start_time)

	// end_time is when the mission ended (Unix timestamp, 0 if running)
	endTime?: int64 @protobuf(5,int64,name=end_time)

	// finding_count is the number of findings discovered
	findingCount?: int32 @protobuf(6,int32,name=finding_count)

	// name is the human-readable mission name
	name?: string @protobuf(7,string)

	// description is the mission description
	description?: string @protobuf(9,string)

	// progress is the mission completion progress from 0.0 to 1.0
	progress?: float64 @protobuf(10,double)

	// mission_definition_id is the registered mission definition this run used.
	missionDefinitionId?: string @protobuf(11,string,name=mission_definition_id)

	// target_id is the registered target this mission ran against.
	targetId?: string @protobuf(12,string,name=target_id)
}

// PauseMissionRequest requests pausing a running mission.
#PauseMissionRequest: {
	// mission_id is the unique identifier of the mission to pause
	missionId?: string @protobuf(1,string,name=mission_id)

	// force indicates whether to pause immediately without waiting for a clean checkpoint boundary
	// If false (default), waits for the current node to complete before pausing
	force?: bool @protobuf(2,bool)
}

// PauseMissionResponse confirms the mission pause request.
#PauseMissionResponse: {
	// success indicates if the pause request was accepted
	success?: bool @protobuf(1,bool)

	// checkpoint_id is the ID of the checkpoint created during pause
	checkpointId?: string @protobuf(2,string,name=checkpoint_id)

	// message provides additional context about the pause operation
	message?: string @protobuf(3,string)
}

// ResumeMissionRequest requests resuming a paused mission.
#ResumeMissionRequest: {
	// mission_id is the unique identifier of the mission to resume
	missionId?: string @protobuf(1,string,name=mission_id)

	// checkpoint_id optionally specifies a specific checkpoint to resume from
	// If empty, resumes from the latest checkpoint
	checkpointId?: string @protobuf(2,string,name=checkpoint_id)

	// Empty string = legacy resume-from-latest behaviour (backward compatible).
	// When non-empty, the daemon rewinds the mission to the named checkpoint
	// and resumes execution from that point. The handler additionally enforces
	// the mission#admin FGA relation when this field is non-empty per
	// mission-checkpointing R16.3.
	targetCheckpointId?: string @protobuf(3,string,name=target_checkpoint_id)
}

// GetMissionHistoryRequest queries mission execution history by name.
#GetMissionHistoryRequest: {
	// name is the mission name to query history for
	name?: string @protobuf(1,string)

	// limit restricts the number of results (default: 100)
	limit?: int32 @protobuf(2,int32)

	// offset is the pagination offset (default: 0)
	offset?: int32 @protobuf(3,int32)
}

// GetMissionHistoryResponse returns mission execution history.
#GetMissionHistoryResponse: {
	// runs contains all mission runs for the requested name
	runs?: [...#MissionRun] @protobuf(1,MissionRun)

	// total is the total count of runs (for pagination)
	total?: int32 @protobuf(2,int32)
}

// MissionRun represents a single execution instance of a mission.
#MissionRun: {
	// mission_id is the unique identifier for this run
	missionId?: string @protobuf(1,string,name=mission_id)

	// run_number is the sequential run number for this mission name
	runNumber?: int32 @protobuf(2,int32,name=run_number)

	// status is the final status of this run (running, completed, failed, cancelled, paused)
	status?: string @protobuf(3,string)

	// created_at is when this run was created (Unix timestamp)
	createdAt?: int64 @protobuf(4,int64,name=created_at)

	// completed_at is when this run completed (Unix timestamp, 0 if not completed)
	completedAt?: int64 @protobuf(5,int64,name=completed_at)

	// findings_count is the number of findings discovered in this run
	findingsCount?: int32 @protobuf(6,int32,name=findings_count)

	// previous_run_id is the ID of the previous run (if any)
	previousRunId?: string @protobuf(7,string,name=previous_run_id)

	// trace_id is the OTel trace ID for Langfuse lookup
	traceId?: string @protobuf(8,string,name=trace_id)
}

// GetMissionCheckpointsRequest queries checkpoints for a mission.
#GetMissionCheckpointsRequest: {
	// mission_id is the unique identifier of the mission to query checkpoints for
	missionId?: string @protobuf(1,string,name=mission_id)
}

// GetMissionCheckpointsResponse returns all checkpoints for a mission.
#GetMissionCheckpointsResponse: {
	// checkpoints contains all checkpoints for the requested mission
	checkpoints?: [...#CheckpointInfo] @protobuf(1,CheckpointInfo)
}

// CheckpointInfo provides metadata about a mission checkpoint.
#CheckpointInfo: {
	// checkpoint_id is the unique identifier for this checkpoint
	checkpointId?: string @protobuf(1,string,name=checkpoint_id)

	// created_at is when this checkpoint was created (Unix timestamp)
	createdAt?: int64 @protobuf(2,int64,name=created_at)

	// completed_nodes is the number of nodes that had completed at checkpoint time
	completedNodes?: int32 @protobuf(3,int32,name=completed_nodes)

	// total_nodes is the total number of nodes in the mission
	totalNodes?: int32 @protobuf(4,int32,name=total_nodes)

	// findings_count is the number of findings at checkpoint time
	findingsCount?: int32 @protobuf(5,int32,name=findings_count)

	// version is the checkpoint format version
	version?: int32 @protobuf(6,int32)
}

// ListMissionDefinitionsRequest queries installed mission definitions.
#ListMissionDefinitionsRequest: {
	// limit restricts the number of results (0 = all)
	limit?: int32 @protobuf(1,int32)

	// offset is the pagination offset
	offset?: int32 @protobuf(2,int32)
}

// ListMissionDefinitionsResponse returns installed mission definitions.
#ListMissionDefinitionsResponse: {
	// missions is the list of installed mission definitions
	missions?: [...#MissionDefinitionInfo] @protobuf(1,MissionDefinitionInfo)

	// total is the total count of mission definitions (for pagination)
	total?: int32 @protobuf(2,int32)
}

// CreateMissionDefinitionRequest registers a new mission definition with the
// daemon. The definition is validated server-side via MissionDefinition.Validate
// and written to the definition store.
#CreateMissionDefinitionRequest: {
	// definition is the fully-formed mission definition to register.
	definition?: missionpb.#MissionDefinition @protobuf(1,gibson.mission.v1.MissionDefinition)

	// cue_source is the raw CUE source text that compiled to `definition`
	// (maximum 512 KB). Persisted alongside the definition so that
	// GetMissionDefinition can return the author's exact source rather than a
	// reconstruction. Optional for backward compatibility; when empty the
	// definition is stored without a recoverable source.
	cueSource?: string @protobuf(2,string,name=cue_source)
}

// CreateMissionDefinitionResponse returns the registered mission definition ID
// and its summary info record.
#CreateMissionDefinitionResponse: {
	// mission_definition_id is the server-assigned identifier for the definition.
	missionDefinitionId?: string @protobuf(1,string,name=mission_definition_id)

	// info is the summary record for the registered definition.
	info?: #MissionDefinitionInfo @protobuf(2,MissionDefinitionInfo)
}

// UpdateMissionDefinitionRequest carries the replacement definition.
// The name field of the embedded definition is the lookup key.
#UpdateMissionDefinitionRequest: {
	// definition is the replacement content. The name field is used as the
	// lookup key; all other fields replace the stored definition. The
	// server-assigned ID and original timestamps are preserved.
	definition?: missionpb.#MissionDefinition @protobuf(1,gibson.mission.v1.MissionDefinition)

	// cue_source is the raw CUE source text that compiled to `definition`
	// (maximum 512 KB). Overwrites the stored source in place under the stable
	// id. Optional for backward compatibility.
	cueSource?: string @protobuf(2,string,name=cue_source)
}

// UpdateMissionDefinitionResponse returns the stable server-assigned ID for
// the updated definition (unchanged across updates).
#UpdateMissionDefinitionResponse: {
	// mission_definition_id is the stable server-assigned identifier for this
	// definition (unchanged across updates).
	missionDefinitionId?: string @protobuf(1,string,name=mission_definition_id)
}

// MissionDefinitionInfo describes an installed mission definition.
#MissionDefinitionInfo: {
	// name is the mission name
	name?: string @protobuf(1,string)

	// version is the mission version
	version?: string @protobuf(2,string)

	// description is the mission description
	description?: string @protobuf(3,string)

	// source is the Git repository URL
	source?: string @protobuf(4,string)

	// installed_at is when the mission was installed (Unix timestamp)
	installedAt?: int64 @protobuf(5,int64,name=installed_at)

	// updated_at is when the mission was last updated (Unix timestamp)
	updatedAt?: int64 @protobuf(6,int64,name=updated_at)

	// node_count is the number of nodes in the mission
	nodeCount?: int32 @protobuf(7,int32,name=node_count)

	// mission_definition_id is the stable server-assigned identifier for this
	// definition (the GUID returned by CreateMissionDefinition, unchanged across
	// updates).
	missionDefinitionId?: string @protobuf(8,string,name=mission_definition_id)
}

// Mission represents a complete mission execution instance with full state.
#Mission: {
	// id is the unique mission identifier
	id?: string @protobuf(1,string)

	// name is the human-readable mission name
	name?: string @protobuf(2,string)

	// status is the current mission status
	status?: #MissionStatus @protobuf(3,MissionStatus)

	// target_id is the target identifier
	targetId?: string @protobuf(4,string,name=target_id)

	// mission_definition_id is the mission definition identifier
	missionDefinitionId?: string @protobuf(5,string,name=mission_definition_id)

	// constraints defines execution constraints. Canonical type per ADR 0004:
	// gibson.mission.v1.MissionConstraints is the single platform-wide shape.
	constraints?: missionpb.#MissionConstraints @protobuf(6,gibson.mission.v1.MissionConstraints)

	// metrics contains current execution metrics
	metrics?: #MissionMetrics @protobuf(7,MissionMetrics)

	// checkpoint is the latest checkpoint (if any)
	checkpoint?: #MissionCheckpoint @protobuf(8,MissionCheckpoint)

	// run_number is the sequential run number for this mission name
	runNumber?: int32 @protobuf(9,int32,name=run_number)

	// created_at is when the mission was created (Unix timestamp in milliseconds)
	createdAt?: int64 @protobuf(10,int64,name=created_at)

	// updated_at is when the mission was last updated (Unix timestamp in milliseconds)
	updatedAt?: int64 @protobuf(11,int64,name=updated_at)

	// started_at is when the mission execution started (Unix timestamp in milliseconds)
	startedAt?: int64 @protobuf(12,int64,name=started_at)

	// completed_at is when the mission execution completed (Unix timestamp in milliseconds, 0 if not completed)
	completedAt?: int64 @protobuf(13,int64,name=completed_at)
}

// MissionMetrics contains execution metrics for a mission.
#MissionMetrics: {
	// turns_used is the number of agent turns/iterations executed
	turnsUsed?: int32 @protobuf(1,int32,name=turns_used)

	// nodes_executed is the number of mission nodes that ran successfully
	nodesExecuted?: int32 @protobuf(2,int32,name=nodes_executed)

	// nodes_failed is the number of mission nodes that failed
	nodesFailed?: int32 @protobuf(3,int32,name=nodes_failed)

	// findings_count is the total number of findings discovered
	findingsCount?: int32 @protobuf(4,int32,name=findings_count)

	// critical_count is the number of critical severity findings
	criticalCount?: int32 @protobuf(5,int32,name=critical_count)

	// high_count is the number of high severity findings
	highCount?: int32 @protobuf(6,int32,name=high_count)

	// medium_count is the number of medium severity findings
	mediumCount?: int32 @protobuf(7,int32,name=medium_count)

	// low_count is the number of low severity findings
	lowCount?: int32 @protobuf(8,int32,name=low_count)

	// tokens_used is the total LLM tokens consumed
	tokensUsed?: int64 @protobuf(9,int64,name=tokens_used)
}

// MissionCheckpoint represents a saved checkpoint state for pause/resume.
#MissionCheckpoint: {
	// id is the unique checkpoint identifier
	id?: string @protobuf(1,string)

	// version is the checkpoint format version
	version?: int32 @protobuf(2,int32)

	// completed_nodes is the number of nodes that had completed at checkpoint time
	completedNodes?: int32 @protobuf(3,int32,name=completed_nodes)

	// total_nodes is the total number of nodes in the mission
	totalNodes?: int32 @protobuf(4,int32,name=total_nodes)

	// created_at is when this checkpoint was created (Unix timestamp in milliseconds)
	createdAt?: int64 @protobuf(5,int64,name=created_at)

	// state_data is the serialized checkpoint state (opaque blob)
	stateData?: bytes @protobuf(6,bytes,name=state_data)
}

// CreateMissionRequest requests creation of a new mission.
// API-only: missions reference a registered target and mission definition by ID.
// Inline target / inline mission / YAML paths are no longer accepted.
#CreateMissionRequest: {
	// name is the mission name
	name?: string @protobuf(1,string)

	// description is the mission description
	description?: string @protobuf(2,string)

	// target_id is the ID of a pre-registered target.
	targetId?: string @protobuf(3,string,name=target_id)

	// mission_definition_id is the ID of a pre-registered mission definition.
	missionDefinitionId?: string @protobuf(4,string,name=mission_definition_id)

	// constraints defines dispatch-time execution constraints. Canonical type
	// per ADR 0004: gibson.mission.v1.MissionConstraints is the single
	// platform-wide shape.
	//
	// Precedence: dispatch-time constraints (this field) take full precedence
	// over any constraints baked into the referenced MissionDefinition. There
	// is NO per-field merge — if this field is set, the entire dispatch
	// constraint set wins; any field absent from this message reverts to 0
	// (unlimited), not to the definition's value.
	//
	// To inherit the definition's constraints, leave this field unset (the
	// zero-value message is NOT the same as "absent"). Callers that want
	// partial overrides must read the definition's constraints first and
	// re-supply all fields they wish to preserve.
	//
	// Token budget precedence for per-call caps:
	//   dispatch constraints.max_tokens_per_call
	//     > definition constraints.max_tokens_per_call
	//     > per-node *NodeConfig.max_tokens_per_call (lowest; wins if set)
	//
	// Spec: ADR 0004, mission-schema-canonicalization; gibson#133 (M4).
	constraints?: missionpb.#MissionConstraints @protobuf(5,gibson.mission.v1.MissionConstraints)

	// metadata provides additional mission metadata
	metadata?: {
		[string]: string
	} @protobuf(6,map[string]string)

	// variables contains mission variables to override at creation time
	variables?: {
		[string]: string
	} @protobuf(7,map[string]string)

	// memory_continuity defines how agent memory is shared across mission runs
	// Valid values: "isolated" (default), "inherit", "shared"
	memoryContinuity?: string @protobuf(8,string,name=memory_continuity)

	// source_yaml is the original YAML the dashboard used to construct this
	// mission. Optional. When non-empty, the daemon stores it alongside the
	// structured mission state. Empty for programmatic callers that never had
	// a YAML source.
	// Spec: dashboard-neo4j-crud-removal Req 3.5.
	sourceYaml?: string @protobuf(9,string,name=source_yaml)
}

// CreateMissionResponse returns the result of creating a mission.
#CreateMissionResponse: {
	// success indicates if the mission was created successfully
	success?: bool @protobuf(1,bool)

	// mission is the created mission
	mission?: #Mission @protobuf(2,Mission)

	// message provides additional context or error information
	message?: string @protobuf(3,string)
}

// CreateTargetRequest carries the metadata for a new target.
#CreateTargetRequest: {
	// target carries the new target's metadata. Its id field is ignored; the
	// daemon mints the canonical UUID.
	target?: targetpb.#Target @protobuf(1,gibson.target.v1.Target)
}

// CreateTargetResponse returns the minted target.
#CreateTargetResponse: {
	// target_id is the server-minted UUID — the canonical identity clients use
	// to reference this target thereafter.
	targetId?: string @protobuf(1,string,name=target_id)

	// target is the full stored target, including the minted id and timestamps.
	target?: targetpb.#Target @protobuf(2,gibson.target.v1.Target)
}

// GetTargetRequest looks up a target by UUID.
#GetTargetRequest: {
	// target_id is the target UUID.
	targetId?: string @protobuf(1,string,name=target_id)
}

// GetTargetResponse returns the requested target.
#GetTargetResponse: {
	target?: targetpb.#Target @protobuf(1,gibson.target.v1.Target)
}

// ListTargetsRequest narrows the tenant's targets.
#ListTargetsRequest: {
	// filter narrows the result set. Omit for the tenant's full target list.
	filter?: targetpb.#TargetFilter @protobuf(1,gibson.target.v1.TargetFilter)
}

// ListTargetsResponse returns the matching targets.
#ListTargetsResponse: {
	targets?: [...targetpb.#Target] @protobuf(1,gibson.target.v1.Target)
}

// UpdateTargetRequest replaces a target's metadata.
#UpdateTargetRequest: {
	// target is the replacement content. Its id field is the lookup key and is
	// preserved; all other fields replace the stored target.
	target?: targetpb.#Target @protobuf(1,gibson.target.v1.Target)
}

// UpdateTargetResponse returns the updated target.
#UpdateTargetResponse: {
	target?: targetpb.#Target @protobuf(1,gibson.target.v1.Target)
}

// DeleteTargetRequest removes a target by UUID.
#DeleteTargetRequest: {
	// target_id is the UUID of the target to delete.
	targetId?: string @protobuf(1,string,name=target_id)
}

// DeleteTargetResponse reports the outcome of a delete.
#DeleteTargetResponse: {
	success?: bool @protobuf(1,bool)
}

// ListAgentsRequest queries agent registry.
#ListAgentsRequest: {
	// kind filters by component kind (empty = all agents)
	kind?: string @protobuf(1,string)
}

// ListAgentsResponse returns registered agents.
#ListAgentsResponse: {
	// agents is the list of registered agents
	agents?: [...#AgentInfo] @protobuf(1,AgentInfo)
}

// AgentInfo describes a registered agent.
#AgentInfo: {
	// id is the unique agent identifier
	id?: string @protobuf(1,string)

	// name is the agent name
	name?: string @protobuf(2,string)

	// kind is the component kind (always "agent")
	kind?: string @protobuf(3,string)

	// version is the agent version
	version?: string @protobuf(4,string)

	// endpoint is the gRPC endpoint for the agent
	endpoint?: string @protobuf(5,string)

	// capabilities lists agent capabilities
	capabilities?: [...string] @protobuf(6,string)

	// health is the agent health status (healthy, unhealthy)
	health?: string @protobuf(7,string)

	// last_seen is when the agent was last seen (Unix timestamp)
	lastSeen?: int64 @protobuf(8,int64,name=last_seen)
}

// GetAgentStatusRequest queries a specific agent.
#GetAgentStatusRequest: {
	// agent_id is the unique agent identifier
	agentId?: string @protobuf(1,string,name=agent_id)
}

// GetAgentStatusResponse returns agent status.
#GetAgentStatusResponse: {
	// agent is the agent information
	agent?: #AgentInfo @protobuf(1,AgentInfo)

	// active indicates if the agent is currently executing a task
	active?: bool @protobuf(2,bool)

	// current_task describes the active task (if any)
	currentTask?: string @protobuf(3,string,name=current_task)

	// task_start_time is when the current task started (Unix timestamp)
	taskStartTime?: int64 @protobuf(4,int64,name=task_start_time)
}

// ListToolsRequest queries tool registry.
#ListToolsRequest: {}

// ListToolsResponse returns registered tools.
#ListToolsResponse: {
	// tools is the list of registered tools
	tools?: [...#ToolInfo] @protobuf(1,ToolInfo)
}

// Capabilities describes runtime privileges and features available to a tool.
#Capabilities: {
	// has_root indicates the tool is running as uid 0 (root user)
	hasRoot?: bool @protobuf(1,bool,name=has_root)

	// has_sudo indicates passwordless sudo access is available
	hasSudo?: bool @protobuf(2,bool,name=has_sudo)

	// can_raw_socket indicates the ability to create raw network sockets
	canRawSocket?: bool @protobuf(3,bool,name=can_raw_socket)

	// features contains tool-specific feature availability flags
	features?: {
		[string]: bool
	} @protobuf(4,map[string]bool)

	// blocked_args lists command-line arguments that cannot be used
	blockedArgs?: [...string] @protobuf(5,string,name=blocked_args)

	// arg_alternatives maps blocked arguments to their safer alternatives
	argAlternatives?: {
		[string]: string
	} @protobuf(6,map[string]string,arg_alternatives)
}

// ToolInfo describes a registered tool.
#ToolInfo: {
	// id is the unique tool identifier
	id?: string @protobuf(1,string)

	// name is the tool name
	name?: string @protobuf(2,string)

	// version is the tool version
	version?: string @protobuf(3,string)

	// endpoint is the gRPC endpoint for the tool
	endpoint?: string @protobuf(4,string)

	// description is the tool description
	description?: string @protobuf(5,string)

	// health is the tool health status (healthy, unhealthy)
	health?: string @protobuf(6,string)

	// last_seen is when the tool was last seen (Unix timestamp)
	lastSeen?: int64 @protobuf(7,int64,name=last_seen)

	// capabilities describes runtime privileges and features (optional)
	capabilities?: #Capabilities @protobuf(8,Capabilities)
}

// ListPluginsRequest queries plugin registry.
#ListPluginsRequest: {}

// ListPluginsResponse returns registered plugins.
#ListPluginsResponse: {
	// plugins is the list of registered plugins
	plugins?: [...#PluginInfo] @protobuf(1,PluginInfo)
}

// PluginInfo describes a registered plugin.
#PluginInfo: {
	// id is the unique plugin identifier
	id?: string @protobuf(1,string)

	// name is the plugin name
	name?: string @protobuf(2,string)

	// version is the plugin version
	version?: string @protobuf(3,string)

	// endpoint is the gRPC endpoint for the plugin
	endpoint?: string @protobuf(4,string)

	// description is the plugin description
	description?: string @protobuf(5,string)

	// health is the plugin health status (healthy, unhealthy)
	health?: string @protobuf(6,string)

	// last_seen is when the plugin was last seen (Unix timestamp)
	lastSeen?: int64 @protobuf(7,int64,name=last_seen)
}

// QueryPluginRequest executes a method on a plugin.
#QueryPluginRequest: {
	// name is the plugin name to query
	name?: string @protobuf(1,string)

	// method is the method name to execute
	method?: string @protobuf(2,string)

	// params is the typed parameters for the method
	params?: commonpb.#TypedMap @protobuf(3,gibson.common.v1.TypedMap)

	// timeout_ms is the optional timeout in milliseconds (0 = default)
	timeoutMs?: int64 @protobuf(4,int64,name=timeout_ms)
}

// QueryPluginResponse returns the result of a plugin query.
#QueryPluginResponse: {
	// result is the typed result from the plugin method
	result?: commonpb.#TypedValue @protobuf(1,gibson.common.v1.TypedValue)

	// error is set if the query failed
	error?: string @protobuf(2,string)

	// duration_ms is how long the query took in milliseconds
	durationMs?: int64 @protobuf(3,int64,name=duration_ms)
}

// StartComponentRequest requests starting a component.
#StartComponentRequest: {
	// kind is the component kind ("agent", "tool", "plugin")
	kind?: string @protobuf(1,string)

	// name is the component name
	name?: string @protobuf(2,string)
}

// StartComponentResponse returns the result of starting a component.
#StartComponentResponse: {
	// success indicates if the component was started successfully
	success?: bool @protobuf(1,bool)

	// pid is the process ID of the started component
	pid?: int32 @protobuf(2,int32)

	// port is the port the component is listening on
	port?: int32 @protobuf(3,int32)

	// message provides additional context or error information
	message?: string @protobuf(4,string)

	// log_path is the path to the component's log file
	logPath?: string @protobuf(5,string,name=log_path)
}

// StopComponentRequest requests stopping a component.
#StopComponentRequest: {
	// kind is the component kind ("agent", "tool", "plugin")
	kind?: string @protobuf(1,string)

	// name is the component name
	name?: string @protobuf(2,string)

	// force indicates whether to skip graceful shutdown (SIGKILL instead of SIGTERM)
	force?: bool @protobuf(3,bool)
}

// StopComponentResponse returns the result of stopping a component.
#StopComponentResponse: {
	// success indicates if the component was stopped successfully
	success?: bool @protobuf(1,bool)

	// stopped_count is the number of instances successfully stopped
	stoppedCount?: int32 @protobuf(2,int32,name=stopped_count)

	// total_count is the total number of instances that were running
	totalCount?: int32 @protobuf(3,int32,name=total_count)

	// message provides additional context or error information
	message?: string @protobuf(4,string)
}

// BuildComponentRequest requests rebuilding a component from source.
#BuildComponentRequest: {
	// kind is the component kind ("agent", "tool", "plugin")
	kind?: string @protobuf(1,string)

	// name is the component name to build
	name?: string @protobuf(2,string)
}

// BuildComponentResponse returns the result of building a component.
#BuildComponentResponse: {
	// success indicates if the build was successful
	success?: bool @protobuf(1,bool)

	// stdout contains the build standard output
	stdout?: string @protobuf(2,string)

	// stderr contains the build standard error
	stderr?: string @protobuf(3,string)

	// duration_ms is the build time in milliseconds
	durationMs?: int64 @protobuf(4,int64,name=duration_ms)

	// message provides additional context or error information
	message?: string @protobuf(5,string)
}

// ShowComponentRequest requests detailed information about a component.
#ShowComponentRequest: {
	// kind is the component kind ("agent", "tool", "plugin")
	kind?: string @protobuf(1,string)

	// name is the component name to show
	name?: string @protobuf(2,string)
}

// ShowComponentResponse returns detailed component information.
#ShowComponentResponse: {
	// success indicates if the component was found
	success?: bool @protobuf(1,bool)

	// name is the component name
	name?: string @protobuf(2,string)

	// version is the component version
	version?: string @protobuf(3,string)

	// kind is the component kind
	kind?: string @protobuf(4,string)

	// status is the component status (installed, running, stopped)
	status?: string @protobuf(5,string)

	// source is the Git repository URL
	source?: string @protobuf(6,string)

	// repo_path is the local repository path
	repoPath?: string @protobuf(7,string,name=repo_path)

	// bin_path is the path to the binary
	binPath?: string @protobuf(8,string,name=bin_path)

	// port is the listening port (if running)
	port?: int32 @protobuf(9,int32)

	// pid is the process ID (if running)
	pid?: int32 @protobuf(10,int32)

	// created_at is when the component was installed (Unix timestamp)
	createdAt?: int64 @protobuf(11,int64,name=created_at)

	// updated_at is when the component was last updated (Unix timestamp)
	updatedAt?: int64 @protobuf(12,int64,name=updated_at)

	// started_at is when the component was started (Unix timestamp, 0 if not running)
	startedAt?: int64 @protobuf(13,int64,name=started_at)

	// stopped_at is when the component was stopped (Unix timestamp, 0 if never stopped)
	stoppedAt?: int64 @protobuf(14,int64,name=stopped_at)

	// manifest_info contains manifest details (JSON-encoded)
	manifestInfo?: string @protobuf(15,string,name=manifest_info)

	// message provides additional context or error information
	message?: string @protobuf(16,string)
}

// GetComponentLogsRequest requests log entries for a component.
#GetComponentLogsRequest: {
	// kind is the component kind ("agent", "tool", "plugin")
	kind?: string @protobuf(1,string)

	// name is the component name to get logs for
	name?: string @protobuf(2,string)

	// follow indicates whether to stream logs continuously
	follow?: bool @protobuf(3,bool)

	// lines is the number of lines to return (0 = all, default 50)
	lines?: int32 @protobuf(4,int32)
}

// LogEntry represents a single log entry from a component.
#LogEntry: {
	// timestamp is when the log entry was created (Unix timestamp)
	timestamp?: int64 @protobuf(1,int64)

	// level is the log level (debug, info, warn, error)
	level?: string @protobuf(2,string)

	// message is the log message
	message?: string @protobuf(3,string)

	// fields contains additional structured log fields (typed map)
	fields?: commonpb.#TypedMap @protobuf(4,gibson.common.v1.TypedMap)
}

// GetComponentLogsResponse wraps a LogEntry for the GetComponentLogs streaming RPC.
#GetComponentLogsResponse: {
	// timestamp is when the log entry was created (Unix timestamp)
	timestamp?: int64 @protobuf(1,int64)

	// level is the log level (debug, info, warn, error)
	level?: string @protobuf(2,string)

	// message is the log message
	message?: string @protobuf(3,string)

	// fields contains additional structured log fields (typed map)
	fields?: commonpb.#TypedMap @protobuf(4,gibson.common.v1.TypedMap)
}

// GetMyPermissionsRequest queries the caller's permissions within a tenant.
#GetMyPermissionsRequest: {
	// tenant_id is the tenant to scope the query to.
	// If empty, the tenant is inferred from the caller's auth context.
	tenantId?: string @protobuf(1,string,name=tenant_id)
}

// PermissionComponentGrant is a compact component grant for use in permissions summaries.
#PermissionComponentGrant: {
	// component_ref is the component identifier, e.g. "tool:mytool"
	componentRef?: string @protobuf(1,string,name=component_ref)

	// actions lists the FGA relations the caller holds (execute, configure, read)
	actions?: [...string] @protobuf(2,string)
}

// PermissionTeamMembership describes the caller's membership in a team.
#PermissionTeamMembership: {
	// team_id is the unique team identifier
	teamId?: string @protobuf(1,string,name=team_id)

	// team_name is the human-readable team name
	teamName?: string @protobuf(2,string,name=team_name)

	// is_admin is true when the caller is an admin of the team
	isAdmin?: bool @protobuf(3,bool,name=is_admin)
}

// GetMyPermissionsResponse returns a compact summary of the caller's permissions.
#GetMyPermissionsResponse: {
	// tenant_id is the tenant this summary is scoped to
	tenantId?: string @protobuf(1,string,name=tenant_id)

	// role is the caller's FGA-backed role ("owner", "admin", "operator", "viewer")
	role?: string @protobuf(2,string)

	// is_admin is true when the caller holds the admin or owner relation on the tenant
	isAdmin?: bool @protobuf(3,bool,name=is_admin)

	// component_grants lists the component access grants held by the caller
	componentGrants?: [...#PermissionComponentGrant] @protobuf(4,PermissionComponentGrant,name=component_grants)

	// team_memberships lists the teams the caller belongs to within this tenant
	teamMemberships?: [...#PermissionTeamMembership] @protobuf(5,PermissionTeamMembership,name=team_memberships)
}

// ListMyMembershipsRequest has no fields. The caller is identified via the
// call context (HMAC-signed identity headers set by ext-authz). This RPC
// answers "which tenants am I a member of?" before any tenant-scoped RPC
// can be made.
#ListMyMembershipsRequest: {}

// Membership describes one tenant the caller is a member of, plus the
// caller's role in that tenant.
#Membership: {
	// tenant_id is the FGA object id for this tenant (UUID or slug). It is
	// the value the dashboard sets as the `x-gibson-tenant` header on
	// tenant-scoped RPCs.
	tenantId?: string @protobuf(1,string,name=tenant_id)

	// tenant_name is the human-friendly display name. Best-effort: when the
	// daemon's tenant-name cache misses, this falls back to tenant_id.
	tenantName?: string @protobuf(2,string,name=tenant_name)

	// role is the caller's FGA-backed role within this tenant ("admin" or
	// "member"). Set to "admin" when the caller holds the admin relation on
	// the tenant; otherwise "member".
	role?: string @protobuf(3,string)
}

// ListMyMembershipsResponse returns the caller's tenant memberships.
// Sorted by tenant_name ASC for stable rendering.
#ListMyMembershipsResponse: {
	// memberships is the (possibly empty) list of tenants the caller belongs
	// to. An empty list means the caller has no tenant — the dashboard
	// should route to onboarding rather than the picker in that case.
	memberships?: [...#Membership] @protobuf(1,Membership)
}

// GetMissionDefinitionRequest fetches a single mission definition by name.
#GetMissionDefinitionRequest: {
	// name is the mission definition name to look up. Case-sensitive;
	// must match the name field used in CreateMissionDefinition.
	name?: string @protobuf(1,string)
}

// GetMissionDefinitionResponse returns the full structured proto for the
// requested mission definition. Every author-facing field is present:
// workspace, constraints, per-node retry/data/reuse policies, etc.
#GetMissionDefinitionResponse: {
	// definition is the full mission definition proto. Never nil on success.
	definition?: missionpb.#MissionDefinition @protobuf(1,gibson.mission.v1.MissionDefinition)

	// mission_definition_id is the stable server-assigned identifier for this
	// definition (the GUID returned by CreateMissionDefinition, unchanged across
	// updates). Populated on every successful response.
	missionDefinitionId?: string @protobuf(2,string,name=mission_definition_id)

	// cue_source is the raw CUE source the author submitted when the definition
	// was created or last updated. Empty for definitions registered before
	// source persistence landed, or registered without a source.
	cueSource?: string @protobuf(3,string,name=cue_source)
}

// MissionGraphViewport is the diagram pan/zoom framing.
#MissionGraphViewport: {
	x?:    float64 @protobuf(1,double) // horizontal pan offset
	y?:    float64 @protobuf(2,double) // vertical pan offset
	zoom?: float64 @protobuf(3,double) // zoom factor (1.0 = 100%); 0 = renderer default (fit)
}

// MissionGraphNode is a renderable box in the mission flow-chart.
#MissionGraphNode: {
	// id is the mission node id this box represents.
	id?: string @protobuf(1,string)

	// kind is the renderer classification of the node:
	// "agent" | "tool" | "plugin" | "condition" | "parallel" | "join" |
	// "unknown" (string, not enum, so renderers degrade gracefully on future
	// node kinds).
	kind?: string @protobuf(2,string)

	// name is the human-readable label (falls back to id when unset).
	name?: string @protobuf(3,string)

	// summary is a kind-specific one-line descriptor (agent name, tool name,
	// plugin name+method, condition expression, etc.). May be empty.
	summary?: string @protobuf(4,string)

	// is_entry / is_exit mark mission entry and exit nodes.
	isEntry?: bool @protobuf(5,bool,name=is_entry)
	isExit?:  bool @protobuf(6,bool,name=is_exit)

	// rank is the 0-based topological layer (left-to-right depth).
	rank?: int32 @protobuf(7,int32)

	// x / y are the box position in the renderer's abstract canvas space: the
	// saved layout when present, else the deterministic auto-layout position.
	x?: float64 @protobuf(8,double)
	y?: float64 @protobuf(9,double)

	// layout_source is "saved" when x/y came from the layout store, "auto" when
	// computed by the deterministic auto-layout.
	layoutSource?: string @protobuf(10,string,name=layout_source)
}

// MissionGraphEdge is a renderable data-flow line between two boxes.
#MissionGraphEdge: {
	from?: string @protobuf(1,string)
	to?:   string @protobuf(2,string)

	// condition is the optional CEL guard carried on an explicit mission edge.
	condition?: string @protobuf(3,string)

	// role is the branch semantics for edges leaving a condition node:
	// "" (default) | "true" | "false".
	role?: string @protobuf(4,string)
}

// MissionGraph is the daemon-computed renderable projection of a mission
// definition. Node and edge ordering is deterministic.
#MissionGraph: {
	nodes?: [...#MissionGraphNode] @protobuf(1,MissionGraphNode)
	edges?: [...#MissionGraphEdge] @protobuf(2,MissionGraphEdge)
	entryPoints?: [...string] @protobuf(3,string,name=entry_points)
	exitPoints?: [...string] @protobuf(4,string,name=exit_points)
	viewport?: #MissionGraphViewport @protobuf(5,MissionGraphViewport)
}

// GetMissionGraphRequest selects the mission definition to project.
#GetMissionGraphRequest: {
	// mission_definition_id is the stable id of the registered mission
	// definition (as returned by CreateMissionDefinition / carried on a run).
	missionDefinitionId?: string @protobuf(1,string,name=mission_definition_id)
}

// GetMissionGraphResponse carries the projected, layout-merged graph.
#GetMissionGraphResponse: {
	graph?: #MissionGraph @protobuf(1,MissionGraph)
}

// NodePosition is a single node's saved diagram coordinate.
#NodePosition: {
	nodeId?: string  @protobuf(1,string,name=node_id)
	x?:      float64 @protobuf(2,double)
	y?:      float64 @protobuf(3,double)
}

// MissionLayout is the saved diagram layout for a mission definition. It lives
// in a store separate from the mission definition; the work-schema carries no
// presentation state.
#MissionLayout: {
	// mission_definition_id is the definition this layout belongs to.
	missionDefinitionId?: string @protobuf(1,string,name=mission_definition_id)

	// nodes are the saved per-node positions. Nodes without an entry fall back
	// to the daemon's deterministic auto-layout in GetMissionGraph.
	nodes?: [...#NodePosition] @protobuf(2,NodePosition)

	// viewport is the saved pan/zoom framing (optional).
	viewport?: #MissionGraphViewport @protobuf(3,MissionGraphViewport)

	// version is an opaque revision token for optimistic concurrency. Empty when
	// no layout has been saved yet. Returned by GetMissionLayout and
	// SaveMissionLayout; pass it back as expected_version on the next save.
	version?: string @protobuf(4,string)
}

// GetMissionLayoutRequest selects the layout to read.
#GetMissionLayoutRequest: {
	missionDefinitionId?: string @protobuf(1,string,name=mission_definition_id)
}

// GetMissionLayoutResponse returns the saved layout, or an empty layout (no
// node positions, empty version) when none has been saved.
#GetMissionLayoutResponse: {
	layout?: #MissionLayout @protobuf(1,MissionLayout)
}

// SaveMissionLayoutRequest persists a hand-arranged layout.
#SaveMissionLayoutRequest: {
	// layout is the layout to persist. Its mission_definition_id is the key.
	layout?: #MissionLayout @protobuf(1,MissionLayout)

	// expected_version, when set, must match the currently-stored layout's
	// version or the save is rejected (codes.Aborted) as a stale write. Empty
	// means "create if absent" / last-write-wins for the first save.
	expectedVersion?: string @protobuf(2,string,name=expected_version)
}

// SaveMissionLayoutResponse returns the new revision token after a successful
// save. Pass it as expected_version on the subsequent save.
#SaveMissionLayoutResponse: {
	version?: string @protobuf(1,string)
}

// CheckpointSource enumerates the cadence under which a checkpoint was
// captured. Maps to the cadence_reason string carried on
// CheckpointMetadata; both shapes coexist until v1.0.0.
//
// Spec: mission-checkpointing R13.1.
#CheckpointSource:
	#CHECKPOINT_SOURCE_UNSPECIFIED |
	#CHECKPOINT_SOURCE_SUPER_STEP |
	#CHECKPOINT_SOURCE_APPROVAL_GATE |
	#CHECKPOINT_SOURCE_GRACEFUL_SHUTDOWN |
	#CHECKPOINT_SOURCE_PARALLEL_GROUP |
	#CHECKPOINT_SOURCE_MANUAL

#CHECKPOINT_SOURCE_UNSPECIFIED:       0
#CHECKPOINT_SOURCE_SUPER_STEP:        1
#CHECKPOINT_SOURCE_APPROVAL_GATE:     2
#CHECKPOINT_SOURCE_GRACEFUL_SHUTDOWN: 3
#CHECKPOINT_SOURCE_PARALLEL_GROUP:    4
#CHECKPOINT_SOURCE_MANUAL:            5

#CheckpointSource_value: {
	CHECKPOINT_SOURCE_UNSPECIFIED:       0
	CHECKPOINT_SOURCE_SUPER_STEP:        1
	CHECKPOINT_SOURCE_APPROVAL_GATE:     2
	CHECKPOINT_SOURCE_GRACEFUL_SHUTDOWN: 3
	CHECKPOINT_SOURCE_PARALLEL_GROUP:    4
	CHECKPOINT_SOURCE_MANUAL:            5
}

// CheckpointSummary is the lightweight per-checkpoint listing row used by
// ListCheckpoints and embedded in the full Checkpoint payload. Spec R13.1.
#CheckpointSummary: {
	checkpointId?: string            @protobuf(1,string,name=checkpoint_id)
	missionId?:    string            @protobuf(2,string,name=mission_id)
	superStep?:    int64             @protobuf(3,int64,name=super_step)
	capturedAt?:   time.Time         @protobuf(4,google.protobuf.Timestamp,name=captured_at)
	sizeBytes?:    int64             @protobuf(5,int64,name=size_bytes)
	source?:       #CheckpointSource @protobuf(6,CheckpointSource)

	// in_flight_idempotency surfaces the mode of any tool whose call was
	// mid-flight at checkpoint time (UNSPECIFIED if none in flight).
	inFlightIdempotency?: manifestpb.#ToolIdempotency @protobuf(7,gibson.manifest.v1.ToolIdempotency,name=in_flight_idempotency)

	// parallel_group_id, when non-empty, ties this checkpoint to a specific
	// parallel-group barrier.
	parallelGroupId?: string @protobuf(8,string,name=parallel_group_id)

	// expires_at, when set, is when the checkpoint will be GC'd by the
	// retention policy (advisory).
	expiresAt?: time.Time @protobuf(9,google.protobuf.Timestamp,name=expires_at)
}

// ListCheckpointsRequest is paginated and ordered. Pagination cursor is
// opaque (string) — do not parse it client-side.
#ListCheckpointsRequest: {
	missionId?: string @protobuf(1,string,name=mission_id)

	// page_size default 50, server ceiling 200.
	pageSize?:  int32  @protobuf(2,int32,name=page_size)
	pageToken?: string @protobuf(3,string,name=page_token)

	#Order:
		#ORDER_UNSPECIFIED |
		#ORDER_NEWEST_FIRST |
		#ORDER_OLDEST_FIRST

	#ORDER_UNSPECIFIED:  0
	#ORDER_NEWEST_FIRST: 1
	#ORDER_OLDEST_FIRST: 2

	#Order_value: {
		ORDER_UNSPECIFIED:  0
		ORDER_NEWEST_FIRST: 1
		ORDER_OLDEST_FIRST: 2
	}
	order?: #Order @protobuf(4,Order)
}

// ListCheckpointsResponse returns one page of checkpoint summaries.
#ListCheckpointsResponse: {
	checkpoints?: [...#CheckpointSummary] @protobuf(1,CheckpointSummary)
	nextPageToken?: string @protobuf(2,string,name=next_page_token)
	totalCount?:    int32  @protobuf(3,int32,name=total_count)
}

// GetCheckpointRequest carries the (mission, checkpoint) pair plus a flag
// to opt into large blob inclusion (R14.4: the daemon may substitute
// BlobReference for payloads ≥1 MiB).
#GetCheckpointRequest: {
	missionId?:    string @protobuf(1,string,name=mission_id)
	checkpointId?: string @protobuf(2,string,name=checkpoint_id)
	includeBlobs?: bool   @protobuf(3,bool,name=include_blobs)
}

// GetCheckpointResponse wraps the full Checkpoint payload to satisfy
// Buf STANDARD's RPC_RESPONSE_STANDARD_NAME rule.
#GetCheckpointResponse: {
	checkpoint?: #Checkpoint @protobuf(1,Checkpoint)
}

// Checkpoint is the full-decrypted-payload return type of GetCheckpoint.
// working_memory and mission_memory are opaque msgpack bytes per R14
// design. Large blobs may be substituted server-side via BlobReference
// when include_blobs=false.
#Checkpoint: {
	summary?:       #CheckpointSummary @protobuf(1,CheckpointSummary)
	workingMemory?: bytes              @protobuf(2,bytes,name=working_memory)
	missionMemory?: bytes              @protobuf(3,bytes,name=mission_memory)
	steps?: [...#DagStep] @protobuf(4,DagStep)
	findings?: [...#FindingSnapshot] @protobuf(5,FindingSnapshot)
	parallelGroups?: {
		[string]: #ParallelGroupState
	} @protobuf(6,map[string]ParallelGroupState,parallel_groups)
}

// DagStep is one node's snapshot at checkpoint time. inputs/outputs are
// opaque per-step bytes.
#DagStep: {
	nodeId?:     string    @protobuf(1,string,name=node_id)
	state?:      string    @protobuf(2,string)
	startedAt?:  time.Time @protobuf(3,google.protobuf.Timestamp,name=started_at)
	finishedAt?: time.Time @protobuf(4,google.protobuf.Timestamp,name=finished_at)
	inputs?:     bytes     @protobuf(5,bytes)
	outputs?:    bytes     @protobuf(6,bytes)
}

// FindingSnapshot is the per-finding slice of a checkpoint. payload is
// opaque (taxonomy-canonical Finding bytes).
#FindingSnapshot: {
	findingId?: string @protobuf(1,string,name=finding_id)
	severity?:  string @protobuf(2,string)
	payload?:   bytes  @protobuf(3,bytes)
}

// ParallelGroupState captures the state of a parallel group's barrier.
#ParallelGroupState: {
	groupId?:   string @protobuf(1,string,name=group_id)
	expected?:  int32  @protobuf(2,int32)
	completed?: int32  @protobuf(3,int32)
	completedNodeIds?: [...string] @protobuf(4,string,name=completed_node_ids)
}

// BlobReference is the substitution payload returned in lieu of large
// (>=1 MiB) memory/finding blobs when include_blobs=false. The blob_key
// is opaque to clients.
#BlobReference: {
	checkpointId?: string @protobuf(1,string,name=checkpoint_id)
	blobKey?:      string @protobuf(2,string,name=blob_key)
	sizeBytes?:    int64  @protobuf(3,int64,name=size_bytes)
}

// DiffCheckpointsRequest names the two checkpoints to diff. Both must
// belong to mission_id.
#DiffCheckpointsRequest: {
	missionId?:     string @protobuf(1,string,name=mission_id)
	checkpointAId?: string @protobuf(2,string,name=checkpoint_a_id)
	checkpointBId?: string @protobuf(3,string,name=checkpoint_b_id)
}

// DiffCheckpointsResponse wraps the CheckpointDiff payload to satisfy
// Buf STANDARD's RPC_RESPONSE_STANDARD_NAME rule.
#DiffCheckpointsResponse: {
	diff?: #CheckpointDiff @protobuf(1,CheckpointDiff)
}

// CheckpointDiff returns structured, per-domain deltas between two
// checkpoints. Secret redaction (R15.6) is enforced server-side.
#CheckpointDiff: {
	workingMemoryDeltas?: [...#MemoryKeyDelta] @protobuf(1,MemoryKeyDelta,name=working_memory_deltas)
	missionMemoryDeltas?: [...#MemoryKeyDelta] @protobuf(2,MemoryKeyDelta,name=mission_memory_deltas)
	dagStepDeltas?: [...#DagStepDelta] @protobuf(3,DagStepDelta,name=dag_step_deltas)
	findingDeltas?: [...#FindingDelta] @protobuf(4,FindingDelta,name=finding_deltas)
	parallelGroupDeltas?: [...#ParallelGroupDelta] @protobuf(5,ParallelGroupDelta,name=parallel_group_deltas)
}

// MemoryKeyDelta is a single (key, op, before, after) record describing a
// change in a memory tier between checkpoints.
#MemoryKeyDelta: {
	key?: string @protobuf(1,string)

	#Op:
		#OP_UNSPECIFIED |
		#OP_ADDED |
		#OP_REMOVED |
		#OP_CHANGED

	#OP_UNSPECIFIED: 0
	#OP_ADDED:       1
	#OP_REMOVED:     2
	#OP_CHANGED:     3

	#Op_value: {
		OP_UNSPECIFIED: 0
		OP_ADDED:       1
		OP_REMOVED:     2
		OP_CHANGED:     3
	}
	op?:     #Op   @protobuf(2,Op)
	before?: bytes @protobuf(3,bytes)
	after?:  bytes @protobuf(4,bytes)
}

// DagStepDelta describes a change to a single DAG step's snapshot.
#DagStepDelta: {
	nodeId?: string @protobuf(1,string,name=node_id)

	#Op:
		#OP_UNSPECIFIED |
		#OP_ADDED |
		#OP_REMOVED |
		#OP_CHANGED

	#OP_UNSPECIFIED: 0
	#OP_ADDED:       1
	#OP_REMOVED:     2
	#OP_CHANGED:     3

	#Op_value: {
		OP_UNSPECIFIED: 0
		OP_ADDED:       1
		OP_REMOVED:     2
		OP_CHANGED:     3
	}
	op?:     #Op   @protobuf(2,Op)
	before?: bytes @protobuf(3,bytes)
	after?:  bytes @protobuf(4,bytes)
}

// FindingDelta describes a change to a single finding between checkpoints.
#FindingDelta: {
	findingId?: string @protobuf(1,string,name=finding_id)

	#Op:
		#OP_UNSPECIFIED |
		#OP_ADDED |
		#OP_REMOVED |
		#OP_CHANGED

	#OP_UNSPECIFIED: 0
	#OP_ADDED:       1
	#OP_REMOVED:     2
	#OP_CHANGED:     3

	#Op_value: {
		OP_UNSPECIFIED: 0
		OP_ADDED:       1
		OP_REMOVED:     2
		OP_CHANGED:     3
	}
	op?:     #Op   @protobuf(2,Op)
	before?: bytes @protobuf(3,bytes)
	after?:  bytes @protobuf(4,bytes)
}

// ParallelGroupDelta describes a change to a parallel-group barrier
// between checkpoints.
#ParallelGroupDelta: {
	groupId?: string @protobuf(1,string,name=group_id)

	#Op:
		#OP_UNSPECIFIED |
		#OP_ADDED |
		#OP_REMOVED |
		#OP_CHANGED

	#OP_UNSPECIFIED: 0
	#OP_ADDED:       1
	#OP_REMOVED:     2
	#OP_CHANGED:     3

	#Op_value: {
		OP_UNSPECIFIED: 0
		OP_ADDED:       1
		OP_REMOVED:     2
		OP_CHANGED:     3
	}
	op?:     #Op   @protobuf(2,Op)
	before?: bytes @protobuf(3,bytes)
	after?:  bytes @protobuf(4,bytes)
}

// CUEDiagnostic is a single error or warning produced by CUE compilation or
// schema validation. Line and col are 1-based.
#CUEDiagnostic: {
	// line is the 1-based source line number.
	line?: int32 @protobuf(1,int32)

	// col is the 1-based column offset.
	col?: int32 @protobuf(2,int32)

	// message is the human-readable diagnostic text.
	message?: string @protobuf(3,string)

	// severity is "error" or "warning".
	severity?: string @protobuf(4,string)
}

// CUECompletionItem is a single completion suggestion from CompleteMissionCUE.
#CUECompletionItem: {
	// label is the text to insert (the completion token).
	label?: string @protobuf(1,string)

	// detail is a short type annotation or signature hint.
	detail?: string @protobuf(2,string)

	// documentation is the Markdown documentation for this item.
	documentation?: string @protobuf(3,string)

	// kind classifies the item: "field" | "value" | "keyword".
	kind?: string @protobuf(4,string)
}

// ValidateMissionCUERequest carries raw CUE source text to validate.
#ValidateMissionCUERequest: {
	// cue_source is the raw CUE source text.
	cueSource?: string @protobuf(1,string,name=cue_source)
}

// ValidateMissionCUEResponse returns the diagnostics produced by compiling
// the submitted CUE source against the mission schema. An empty list means
// the source is valid.
#ValidateMissionCUEResponse: {
	// diagnostics is the list of errors and warnings. Empty on success.
	diagnostics?: [...#CUEDiagnostic] @protobuf(1,CUEDiagnostic)

	// compiled_definition is the MissionDefinition proto produced by compiling
	// the CUE source. Only populated when diagnostics is empty (i.e. the source
	// is valid). Callers can pass this directly to CreateMissionDefinition
	// without a separate compile round-trip.
	compiledDefinition?: missionpb.#MissionDefinition @protobuf(2,gibson.mission.v1.MissionDefinition,name=compiled_definition)
}

// CompleteMissionCUERequest requests completion items at a cursor position.
#CompleteMissionCUERequest: {
	// cue_source is the raw CUE source text at the time of the request.
	cueSource?: string @protobuf(1,string,name=cue_source)

	// line is the 1-based cursor line number.
	line?: int32 @protobuf(2,int32)

	// col is the 1-based cursor column offset.
	col?: int32 @protobuf(3,int32)
}

// CompleteMissionCUEResponse returns the completion items for the cursor
// position.
#CompleteMissionCUEResponse: {
	// items is the list of completion candidates.
	items?: [...#CUECompletionItem] @protobuf(1,CUECompletionItem)
}

// HoverMissionCUERequest requests hover documentation for a cursor position.
#HoverMissionCUERequest: {
	// cue_source is the raw CUE source text at the time of the request.
	cueSource?: string @protobuf(1,string,name=cue_source)

	// line is the 1-based cursor line number.
	line?: int32 @protobuf(2,int32)

	// col is the 1-based cursor column offset.
	col?: int32 @protobuf(3,int32)
}

// HoverMissionCUEResponse returns Markdown hover documentation for the
// symbol under the cursor, or an empty string if there is no hover info.
#HoverMissionCUEResponse: {
	// markdown is the hover documentation rendered as Markdown.
	markdown?: string @protobuf(1,string)
}
