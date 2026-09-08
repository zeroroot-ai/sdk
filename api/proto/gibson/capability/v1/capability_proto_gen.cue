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

// IsolationMode is where the untrusted-execution isolation boundary lives for
// a capability grant (ADR-0010). It is consumed by the daemon's dispatch-policy
// gate together with the deployment shape (GIBSON_UNTRUSTED_EXEC): under the
// hosted SaaS shape (setec-only) only ISOLATION_MODE_HOSTED_SANDBOX is
// permitted; any other value is rejected fail-closed. Under a customer-operated
// shape (customer-isolation) the customer modes are permitted, and
// ISOLATION_MODE_ON_PREM_SANDBOX_ENDPOINT resolves the configured (customer-
// pointed) setec SandboxService endpoint.
//
// The enum lives here (public OSS) because callers parsing a
// CapabilityGrantInfo need to render the isolation posture without pulling the
// admin/mint service descriptor — same rationale as RecipientClass.
#IsolationMode:
	#ISOLATION_MODE_UNSPECIFIED |
	#ISOLATION_MODE_HOSTED_SANDBOX |
	#ISOLATION_MODE_CUSTOMER_CLUSTER_ATTESTED |
	#ISOLATION_MODE_CUSTOMER_SELF_SANDBOX |
	#ISOLATION_MODE_ON_PREM_SANDBOX_ENDPOINT

// ISOLATION_MODE_UNSPECIFIED is treated as ISOLATION_MODE_HOSTED_SANDBOX by
// the gate (back-compat for grants minted before this field shipped), which
// is the fail-closed-safe default in the hosted shape.
#ISOLATION_MODE_UNSPECIFIED: 0

// ISOLATION_MODE_HOSTED_SANDBOX: untrusted execution runs in the platform-
// operated setec sandbox fleet. The only mode permitted under setec-only.
#ISOLATION_MODE_HOSTED_SANDBOX: 1

// ISOLATION_MODE_CUSTOMER_CLUSTER_ATTESTED: untrusted execution runs in a
// customer-operated cluster whose isolation the daemon verifies by
// attestation. Attestation mechanics are a separate follow-up.
#ISOLATION_MODE_CUSTOMER_CLUSTER_ATTESTED: 2

// ISOLATION_MODE_CUSTOMER_SELF_SANDBOX: the customer owns and operates the
// isolation boundary entirely. Attestation mechanics are a separate
// follow-up.
#ISOLATION_MODE_CUSTOMER_SELF_SANDBOX: 3

// ISOLATION_MODE_ON_PREM_SANDBOX_ENDPOINT: untrusted execution dispatches to
// a customer-pointed setec SandboxService endpoint configured on the daemon.
#ISOLATION_MODE_ON_PREM_SANDBOX_ENDPOINT: 4

#IsolationMode_value: {
	ISOLATION_MODE_UNSPECIFIED:               0
	ISOLATION_MODE_HOSTED_SANDBOX:            1
	ISOLATION_MODE_CUSTOMER_CLUSTER_ATTESTED: 2
	ISOLATION_MODE_CUSTOMER_SELF_SANDBOX:     3
	ISOLATION_MODE_ON_PREM_SANDBOX_ENDPOINT:  4
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

	// isolation is where this grant's untrusted-execution boundary lives
	// (ADR-0010). UNSPECIFIED is treated as HOSTED_SANDBOX by the daemon's
	// dispatch-policy gate. Consumed together with the deployment shape:
	// setec-only permits only HOSTED_SANDBOX (fail-closed otherwise).
	isolation?: #IsolationMode @protobuf(11,IsolationMode)
}
