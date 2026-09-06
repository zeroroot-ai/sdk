// Package gibson.capability.v1 — public, customer-visible wire shape
// for capability grants (CG-JWTs).
//
// CapabilityGrantInfo describes ONE active capability grant minted for
// an agent / tool / plugin install. It is the read-side projection
// returned by:
//   - gibson.identity.v1.IdentityService.WhoAmI
//     (a principal listing its own active grants)
//   - gibson.admin.v1.GrantsAdminService.ListActiveGrants
//     (a tenant_admin inspecting all grants in the tenant)
//
// Both surfaces share this type — the READ side lives in the public OSS
// SDK (this package); the WRITE/inspector SERVICE that mutates grants
// lives in the internal platform-sdk. Customer code that embeds the
// agent runtime parses CapabilityGrantInfo to render "what can I do
// right now" UI without needing the admin service descriptor.
//
// Spec: two-surface platform contract (ADR-0001, forthcoming);
//       component-bootstrap-e2e Requirement 10 (read side);
//       secrets-tenant-lifecycle Requirement 8.1 (write side, moved
//       to platform-sdk under slice #108).
package capabilityv1

// RecipientClass is the class of caller a capability grant is issued
// to. The runtime enum lives here (public OSS) because callers parsing
// a CapabilityGrantInfo need to discriminate without pulling the admin
// service descriptor.
#RecipientClass:
	#RECIPIENT_CLASS_UNSPECIFIED |
	#RECIPIENT_CLASS_AGENT |
	#RECIPIENT_CLASS_TOOL |
	#RECIPIENT_CLASS_PLUGIN

#RECIPIENT_CLASS_UNSPECIFIED: 0

// RECIPIENT_CLASS_AGENT: the grant authorizes an agent install to
// invoke a per-mission RPC set.
#RECIPIENT_CLASS_AGENT: 1

// RECIPIENT_CLASS_TOOL: the grant authorizes a tool install.
#RECIPIENT_CLASS_TOOL: 2

// RECIPIENT_CLASS_PLUGIN: the grant authorizes a plugin install.
#RECIPIENT_CLASS_PLUGIN: 3

#RecipientClass_value: {
	RECIPIENT_CLASS_UNSPECIFIED: 0
	RECIPIENT_CLASS_AGENT:       1
	RECIPIENT_CLASS_TOOL:        2
	RECIPIENT_CLASS_PLUGIN:      3
}

// CapabilityGrantInfo is the wire-shape for one active capability
// grant. It is derived from the daemon's grant store and is suitable
// for both the dashboard's grants table and the agent-side "what can
// I do right now" UI.
#CapabilityGrantInfo: {
	// jti is the JWT ID claim of the CG-JWT — the canonical identifier
	// the dashboard uses for filtering and per-row drill-down.
	jti?: string @protobuf(1,string)

	// recipient_install_id is the install ID this grant was minted for.
	recipientInstallId?: string @protobuf(2,string,name=recipient_install_id)

	// recipient_class is the class of the install (AGENT / TOOL / PLUGIN).
	recipientClass?: #RecipientClass @protobuf(3,RecipientClass,name=recipient_class)

	// recipient_name is the display name (component name) of the install.
	recipientName?: string @protobuf(4,string,name=recipient_name)

	// allowed_rpcs is the set of method strings the grant authorizes
	// (e.g. ["GetCredential", "RecordFinding"]).
	allowedRpcs?: [...string] @protobuf(5,string,name=allowed_rpcs)

	// mission_id is the mission this grant scopes the recipient to.
	// Empty for non-mission-scoped grants.
	missionId?: string @protobuf(6,string,name=mission_id)

	// task_id is the task within the mission. Empty when mission_id is
	// empty or the grant is mission-wide.
	taskId?: string @protobuf(7,string,name=task_id)

	// issued_at_unix is the iat claim, Unix seconds.
	issuedAtUnix?: int64 @protobuf(8,int64,name=issued_at_unix)

	// expires_at_unix is the exp claim, Unix seconds.
	expiresAtUnix?: int64 @protobuf(9,int64,name=expires_at_unix)

	// near_expiry is true when the grant expires within 5 minutes from
	// now. The dashboard renders these rows with a warning highlight.
	nearExpiry?: bool @protobuf(10,bool,name=near_expiry)
}
