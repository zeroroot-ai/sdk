package manifestpb

import (
	"time"
	"github.com/zeroroot-ai/sdk/api/proto/gibson/identity/v1:identityv1"
)

// CapabilityManifest is the single, signed, versioned snapshot of every
// component, permission, cross-component rule, and runtime context that
// applies to the calling principal in their resolved tenant. Both SDKs
// (runtime) and the ADK (scaffold-time) consume this shape.
#CapabilityManifest: {
	manifestId?:      string         @protobuf(1,string,name=manifest_id)
	manifestVersion?: uint64         @protobuf(2,uint64,name=manifest_version)
	tenantId?:        string         @protobuf(3,string,name=tenant_id)
	subject?:         string         @protobuf(4,string)
	issuedAt?:        time.Time      @protobuf(5,google.protobuf.Timestamp,name=issued_at)
	expiresAt?:       time.Time      @protobuf(6,google.protobuf.Timestamp,name=expires_at)
	ttlSeconds?:      uint32         @protobuf(7,uint32,name=ttl_seconds)
	tenantContext?:   #TenantContext @protobuf(10,TenantContext,name=tenant_context)
	agents?: [...#ComponentCapability] @protobuf(11,ComponentCapability)
	tools?: [...#ComponentCapability] @protobuf(12,ComponentCapability)
	plugins?: [...#ComponentCapability] @protobuf(13,ComponentCapability)
	crossComponentRules?: [...#CrossComponentRule] @protobuf(20,CrossComponentRule,name=cross_component_rules)
	crossComponentRulesTruncated?: bool             @protobuf(21,bool,name=cross_component_rules_truncated)
	limits?:                       #LimitsAndQuotas @protobuf(30,LimitsAndQuotas)
	availableLlmSlots?: [...string] @protobuf(31,string,name=available_llm_slots)
	memory?: #MemoryPermissions @protobuf(32,MemoryPermissions)

	// Signature payload (Ed25519 over body with signature/kid cleared).
	signature?:    bytes  @protobuf(200,bytes)
	signingKeyId?: string @protobuf(201,string,name=signing_key_id)
}

// TenantContext surfaces the tenant identity and team memberships that
// gate the manifest's scope.
#TenantContext: {
	tenantId?:          string @protobuf(1,string,name=tenant_id)
	tenantDisplayName?: string @protobuf(2,string,name=tenant_display_name)
	teamMemberships?: [...string] @protobuf(3,string,name=team_memberships)
	isAdmin?: bool @protobuf(4,bool,name=is_admin)
}

// ComponentCapability is a single discoverable component (agent, tool, plugin)
// enriched with the permissions the subject holds against it, plus a typed
// contract describing how to invoke it.
//
// Kind discriminator (component-bootstrap-e2e R12): the typed
// `principal_kind` field is the canonical way to branch on
// agent/tool/plugin. The legacy `string kind = 2` is retained for
// backward compatibility during the one-minor-release deprecation
// window — readers SHOULD prefer `principal_kind` and SHOULD validate
// that the populated `contract` oneof matches the kind.
#ComponentCapability: {
	name?: string @protobuf(1,string)

	// DEPRECATED: prefer `principal_kind`. String-form kind for legacy
	// consumers; one-minor-release deprecation window.
	kind?:         string @protobuf(2,string)
	componentRef?: string @protobuf(3,string,name=component_ref)
	version?:      string @protobuf(4,string)
	description?:  string @protobuf(5,string)
	isSystem?:     bool   @protobuf(6,bool,name=is_system)
	ownerTenant?:  string @protobuf(7,string,name=owner_tenant)
	permissions?: [...string] @protobuf(10,string)
	{} | {
		agentContract: #AgentContract @protobuf(20,AgentContract,name=agent_contract)
	} | {
		toolContract: #ToolContract @protobuf(21,ToolContract,name=tool_contract)
	} | {
		pluginContract: #PluginContract @protobuf(22,PluginContract,name=plugin_contract)
	}
	liveness?: #ComponentLiveness @protobuf(30,ComponentLiveness)

	// principal_kind is the typed kind discriminator. Populated by the
	// daemon at manifest-resolution time; set by the SDK loader when
	// converting YAML manifests. Must agree with the populated
	// `contract` oneof (PRINCIPAL_KIND_AGENT ↔ agent_contract, etc.) —
	// mismatches are rejected by the daemon's RegisterPlugin /
	// CreateAgentIdentity validators.
	//
	// Spec: component-bootstrap-e2e Requirement 12.
	principalKind?: identityv1.#PrincipalKind @protobuf(40,gibson.identity.v1.PrincipalKind,name=principal_kind)
}

// AgentContract describes an agent's LLM slot surface and declared
// tool/plugin dependencies. The daemon composes this from the agent's
// descriptor RPC at registration time.
#AgentContract: {
	llmSlotNames?: [...string] @protobuf(1,string,name=llm_slot_names)
	declaredToolDependencies?: [...string] @protobuf(2,string,name=declared_tool_dependencies)
	declaredPluginDependencies?: [...string] @protobuf(3,string,name=declared_plugin_dependencies)
}

// ToolIdempotency declares the at-most-once / at-least-once / exactly-once
// delivery semantics a tool guarantees. Read by the resume-from-checkpoint
// logic: tools at AT_LEAST_ONCE are safely retried on resume; tools at
// AT_MOST_ONCE are skipped if their pre-checkpoint invocation status is
// ambiguous; tools at EXACTLY_ONCE require the orchestrator to consult the
// idempotency journal before re-issuing. UNSPECIFIED is treated as
// AT_LEAST_ONCE for backward compatibility.
//
// Spec: mission-checkpointing R6.
#ToolIdempotency:
	#TOOL_IDEMPOTENCY_UNSPECIFIED |
	#TOOL_IDEMPOTENCY_AT_MOST_ONCE |
	#TOOL_IDEMPOTENCY_AT_LEAST_ONCE |
	#TOOL_IDEMPOTENCY_EXACTLY_ONCE

#TOOL_IDEMPOTENCY_UNSPECIFIED:   0
#TOOL_IDEMPOTENCY_AT_MOST_ONCE:  1
#TOOL_IDEMPOTENCY_AT_LEAST_ONCE: 2
#TOOL_IDEMPOTENCY_EXACTLY_ONCE:  3

#ToolIdempotency_value: {
	TOOL_IDEMPOTENCY_UNSPECIFIED:   0
	TOOL_IDEMPOTENCY_AT_MOST_ONCE:  1
	TOOL_IDEMPOTENCY_AT_LEAST_ONCE: 2
	TOOL_IDEMPOTENCY_EXACTLY_ONCE:  3
}

// ToolContract describes the proto envelope a tool accepts and emits.
// input_schema_json and output_schema_json are derived from the
// FileDescriptor and rendered to JSON Schema for SDK/ADK consumption.
#ToolContract: {
	inputProtoName?:   string @protobuf(1,string,name=input_proto_name)
	outputProtoName?:  string @protobuf(2,string,name=output_proto_name)
	inputSchemaJson?:  string @protobuf(3,string,name=input_schema_json)
	outputSchemaJson?: string @protobuf(4,string,name=output_schema_json)

	// see ToolIdempotency enum; UNSPECIFIED is treated as AT_LEAST_ONCE
	idempotency?: #ToolIdempotency @protobuf(5,ToolIdempotency)
}

// PluginContract enumerates a plugin's callable methods with per-method
// schemas and per-method FGA-derived invocation permission.
#PluginContract: {
	methods?: [...#PluginMethod] @protobuf(1,PluginMethod)
}

#PluginMethod: {
	name?:             string @protobuf(1,string)
	paramsSchemaJson?: string @protobuf(2,string,name=params_schema_json)
	resultSchemaJson?: string @protobuf(3,string,name=result_schema_json)
	canInvoke?:        bool   @protobuf(4,bool,name=can_invoke)
}

// CrossComponentRule expresses an explicit override on the default
// can_execute evaluation for a (source_component, target_component) pair.
// Only rules that override the default are emitted, keeping payload bounded.
#CrossComponentRule: {
	sourceComponentRef?: string  @protobuf(1,string,name=source_component_ref)
	targetComponentRef?: string  @protobuf(2,string,name=target_component_ref)
	effect?:             #Effect @protobuf(3,Effect)
	reason?:             string  @protobuf(4,string)

	#Effect:
		#EFFECT_UNSPECIFIED |
		#EFFECT_ALLOW |
		#EFFECT_DENY

	#EFFECT_UNSPECIFIED: 0
	#EFFECT_ALLOW:       1
	#EFFECT_DENY:        2

	#Effect_value: {
		EFFECT_UNSPECIFIED: 0
		EFFECT_ALLOW:       1
		EFFECT_DENY:        2
	}
}

// LimitsAndQuotas carries the tier-derived resource ceilings applied to
// the subject's session. max_spend_usd is a decimal string to avoid
// floating-point rounding at quota boundaries.
#LimitsAndQuotas: {
	maxTokensPerCall?:    uint64 @protobuf(1,uint64,name=max_tokens_per_call)
	maxTokensPerSession?: uint64 @protobuf(2,uint64,name=max_tokens_per_session)
	rateLimitPerMinute?:  uint32 @protobuf(3,uint32,name=rate_limit_per_minute)
	maxSpendUsd?:         string @protobuf(4,string,name=max_spend_usd)
}

// MemoryPermissions expresses per-tier memory access (e.g. "ro", "rw", "").
#MemoryPermissions: {
	working?:  string @protobuf(1,string)
	mission?:  string @protobuf(2,string)
	longterm?: string @protobuf(3,string)
}

// ComponentLiveness is a snapshot of the component's runtime health at
// manifest issuance time. It is advisory only; the daemon remains the
// source of truth for live execution decisions.
#ComponentLiveness: {
	status?:        string    @protobuf(1,string)
	lastHeartbeat?: time.Time @protobuf(2,google.protobuf.Timestamp,name=last_heartbeat)
	instanceCount?: uint32    @protobuf(3,uint32,name=instance_count)
}

// GetCapabilityManifestRequest identifies the subject whose manifest is
// requested. agent_principal_id is only honored for tenant admins and
// enables scaffold-time impersonation previews.
#GetCapabilityManifestRequest: {
	agentPrincipalId?: string @protobuf(1,string,name=agent_principal_id)
}

// GetCapabilityManifestResponse wraps the signed manifest. Wrapping keeps
// the RPC Buf-STANDARD compliant and leaves room for envelope metadata
// (e.g. server-side issuance metrics) to be added without breaking the
// wire format of the manifest body itself.
#GetCapabilityManifestResponse: {
	manifest?: #CapabilityManifest @protobuf(1,CapabilityManifest)
}

// WatchManifestInvalidationsRequest opens a server-streaming channel
// emitting ManifestInvalidationEvent for the caller's resolved tenant.
#WatchManifestInvalidationsRequest: {}

// WatchManifestInvalidationsResponse wraps a single invalidation event
// so the RPC's streaming response type satisfies Buf STANDARD naming.
#WatchManifestInvalidationsResponse: {
	event?: #ManifestInvalidationEvent @protobuf(1,ManifestInvalidationEvent)
}

// ManifestInvalidationEvent is delivered when the subject's manifest
// should be considered stale. HEARTBEAT events indicate the stream is
// alive; INVALIDATED events indicate a refresh is warranted.
#ManifestInvalidationEvent: {
	eventType?:          #EventType @protobuf(1,EventType,name=event_type)
	tenantId?:           string     @protobuf(2,string,name=tenant_id)
	reason?:             string     @protobuf(3,string)
	newManifestVersion?: uint64     @protobuf(4,uint64,name=new_manifest_version)
	emittedAt?:          time.Time  @protobuf(5,google.protobuf.Timestamp,name=emitted_at)

	#EventType:
		#EVENT_TYPE_UNSPECIFIED |
		#EVENT_TYPE_HEARTBEAT |
		#EVENT_TYPE_INVALIDATED

	#EVENT_TYPE_UNSPECIFIED: 0
	#EVENT_TYPE_HEARTBEAT:   1
	#EVENT_TYPE_INVALIDATED: 2

	#EventType_value: {
		EVENT_TYPE_UNSPECIFIED: 0
		EVENT_TYPE_HEARTBEAT:   1
		EVENT_TYPE_INVALIDATED: 2
	}
}
