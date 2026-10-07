package manifestpb

import "time"

// CapabilityManifest is the single, signed, versioned snapshot of every
// component, permission, cross-component rule, and runtime context that
// applies to the calling principal in their resolved tenant. Both SDKs
// (runtime) and the ADK (scaffold-time) consume this shape.
#CapabilityManifest: {
	manifestId?:      string    @protobuf(1,string,name=manifest_id)
	manifestVersion?: uint64    @protobuf(2,uint64,name=manifest_version)
	tenantId?:        string    @protobuf(3,string,name=tenant_id)
	subject?:         string    @protobuf(4,string)
	expiresAt?:       time.Time @protobuf(6,google.protobuf.Timestamp,name=expires_at)
	ttlSeconds?:      uint32    @protobuf(7,uint32,name=ttl_seconds)
	agents?: [...#ComponentCapability] @protobuf(11,ComponentCapability)
	tools?: [...#ComponentCapability] @protobuf(12,ComponentCapability)
	plugins?: [...#ComponentCapability] @protobuf(13,ComponentCapability)
	limits?: #LimitsAndQuotas   @protobuf(30,LimitsAndQuotas)
	memory?: #MemoryPermissions @protobuf(32,MemoryPermissions)

	// Signature payload (Ed25519 over body with signature/kid cleared).
	signature?: bytes @protobuf(200,bytes)
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
	ownerTenant?:  string @protobuf(7,string,name=owner_tenant)
	permissions?: [...string] @protobuf(10,string)
	liveness?: #ComponentLiveness @protobuf(30,ComponentLiveness)
}

// LimitsAndQuotas carries the tier-derived resource ceilings applied to
// the subject's session. max_spend_usd is a decimal string to avoid
// floating-point rounding at quota boundaries.
#LimitsAndQuotas: {
	maxTokensPerCall?: uint64 @protobuf(1,uint64,name=max_tokens_per_call)
}

// MemoryPermissions expresses per-tier memory access (e.g. "ro", "rw", "").
#MemoryPermissions: {
	working?: string @protobuf(1,string)
	mission?: string @protobuf(2,string)
}

// ComponentLiveness is a snapshot of the component's runtime health at
// manifest issuance time. It is advisory only; the daemon remains the
// source of truth for live execution decisions.
#ComponentLiveness: {
	status?:        string    @protobuf(1,string)
	lastHeartbeat?: time.Time @protobuf(2,google.protobuf.Timestamp,name=last_heartbeat)
}

// GetCapabilityManifestRequest identifies the subject whose manifest is
// requested. agent_principal_id is only honored for tenant admins and
// enables scaffold-time impersonation previews.
#GetCapabilityManifestRequest: {
	agentPrincipalId?: string @protobuf(1,string,name=agent_principal_id,"(buf.validate.field).string=")
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
	eventType?: #EventType @protobuf(1,EventType,name=event_type)
	tenantId?:  string     @protobuf(2,string,name=tenant_id)
	reason?:    string     @protobuf(3,string)

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
