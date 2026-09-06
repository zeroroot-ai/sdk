// Package gibson.identity.v1 — caller-side identity inspection.
//
// IdentityService.WhoAmI is the canonical "what can I do?" RPC. Every
// authenticated principal may call it for themselves; tenant_admins
// may pass target_principal_id to inspect another agent in their
// tenant. The response carries the caller's effective FGA grants
// (component reads/writes/executes, plugin invocations) and any
// active capability grants.
//
// Spec: component-bootstrap-e2e Requirement 10.
package identityv1

import "github.com/zeroroot-ai/sdk/api/proto/gibson/capability/v1:capabilityv1"

// PrincipalKind identifies the runtime kind of a Gibson principal.
//
// Canonical home: this enum is the SDK-published source of truth. The
// daemon's local tenant_admin proto and the SDK's RecipientClass enum
// in gibson.admin.v1.grants are duplicates that pre-date the
// component-bootstrap-e2e consolidation; both should migrate to
// importing PrincipalKind from this package over time.
#PrincipalKind:
	#PRINCIPAL_KIND_UNSPECIFIED |
	#PRINCIPAL_KIND_AGENT |
	#PRINCIPAL_KIND_TOOL |
	#PRINCIPAL_KIND_PLUGIN

#PRINCIPAL_KIND_UNSPECIFIED: 0
#PRINCIPAL_KIND_AGENT:       1
#PRINCIPAL_KIND_TOOL:        2
#PRINCIPAL_KIND_PLUGIN:      3

#PrincipalKind_value: {
	PRINCIPAL_KIND_UNSPECIFIED: 0
	PRINCIPAL_KIND_AGENT:       1
	PRINCIPAL_KIND_TOOL:        2
	PRINCIPAL_KIND_PLUGIN:      3
}

// WhoAmIRequest carries an optional target_principal_id for admin
// inspection. When empty, the caller's own identity (from ext-authz
// headers) is used.
#WhoAmIRequest: {
	// target_principal_id is OPTIONAL. When set, the caller MUST be
	// tenant_admin on the target's tenant. Format matches the FGA
	// user form: "agent_principal:<uuid>" / "tool_principal:<uuid>" /
	// "plugin_principal:<uuid>".
	targetPrincipalId?: string @protobuf(1,string,name=target_principal_id)
}

// WhoAmIResponse carries the principal's effective grants.
#WhoAmIResponse: {
	// principal_id is the FGA principal identifier of the principal
	// this response describes.
	principalId?: string         @protobuf(1,string,name=principal_id)
	kind?:        #PrincipalKind @protobuf(2,PrincipalKind)

	// name is the human-readable name the principal was registered with
	// (e.g. "scanner-bot"). Used for display; not used for authz.
	name?: string @protobuf(3,string)

	// tenant_id is the tenant the principal belongs to.
	tenantId?: string @protobuf(4,string,name=tenant_id)

	// component_grants is one entry per component the principal can
	// touch in any way (read, configure, or execute). Components the
	// principal cannot touch are NOT included.
	componentGrants?: [...#ComponentGrantEffective] @protobuf(5,ComponentGrantEffective,name=component_grants)

	// plugin_grants is one entry per plugin the principal can invoke.
	// For agent_principals this is empty by FGA model design (agents
	// do not directly invoke plugins; tools do).
	pluginGrants?: [...#PluginGrantEffective] @protobuf(6,PluginGrantEffective,name=plugin_grants)

	// active_capability_grants is the principal's currently-issued
	// CG-JWTs (mission-scoped) at the time of the call. Reuses the
	// public CapabilityGrantInfo message from gibson.capability.v1
	// (extracted from gibson.admin.v1 in slice #108) rather than
	// minting a parallel type.
	activeCapabilityGrants?: [...capabilityv1.#CapabilityGrantInfo] @protobuf(7,gibson.capability.v1.CapabilityGrantInfo,name=active_capability_grants)

	// truncated is true when the response had to drop entries to fit
	// within the 1000-grant safety bound documented in
	// component-bootstrap-e2e Requirement 10.4. Callers seeing this
	// flag should treat the listing as incomplete and surface a UI
	// warning.
	truncated?: bool @protobuf(8,bool)
}

// ComponentGrantEffective describes the principal's per-action access
// to one component (FGA type "component"). The three booleans reflect
// the per-action FGA relations as composed by model.fga's deny-wins
// rules: each is true iff the principal can perform that action right
// now after all denies are subtracted.
#ComponentGrantEffective: {
	// component_ref is the FGA object identifier, e.g. "component:gitlab".
	componentRef?: string @protobuf(1,string,name=component_ref)
	canRead?:      bool   @protobuf(2,bool,name=can_read)      // can_read_as_component = can_read AND component_read_enabled
	canConfigure?: bool   @protobuf(3,bool,name=can_configure) // can_write_as_component = can_configure AND component_write_enabled
	canExecute?:   bool   @protobuf(4,bool,name=can_execute)   // can_execute_as_component = can_execute AND component_execute_enabled

	// sources enumerates how the principal got each granted action.
	// For UI legibility — multiple sources may stack (direct grant
	// plus tenant-member inheritance, etc.).
	sources?: [...#GrantSource] @protobuf(5,GrantSource)
}

// PluginGrantEffective describes the principal's invocation access to
// one plugin (FGA type "plugin", binary can_invoke).
#PluginGrantEffective: {
	pluginRef?: string @protobuf(1,string,name=plugin_ref) // e.g. "plugin:gitlab"
	sources?: [...#GrantSource] @protobuf(2,GrantSource)
}

// GrantSource attributes a single grant to its origin in the FGA model.
// Used by the dashboard's Permissions tab to render inheritance and by
// gibson-cli inspect to show the operator where a permission came from.
#GrantSource: {
	#Kind:
		#KIND_UNSPECIFIED |
		#KIND_DIRECT |
		#KIND_TENANT_MEMBER |
		#KIND_TEAM_MEMBER |
		#KIND_OWNER

	#KIND_UNSPECIFIED: 0

	// KIND_DIRECT — tuple writes the principal directly.
	#KIND_DIRECT: 1

	// KIND_TENANT_MEMBER — inherited via tenant#member.
	#KIND_TENANT_MEMBER: 2

	// KIND_TEAM_MEMBER — inherited via team#member.
	#KIND_TEAM_MEMBER: 3

	// KIND_OWNER — granted because the principal's tenant owns the
	// component (admin-from-owner FGA path).
	#KIND_OWNER: 4

	#Kind_value: {
		KIND_UNSPECIFIED:   0
		KIND_DIRECT:        1
		KIND_TENANT_MEMBER: 2
		KIND_TEAM_MEMBER:   3
		KIND_OWNER:         4
	}
	kind?: #Kind @protobuf(1,Kind)

	// source_object is the FGA object the inheritance flowed from
	// (e.g. "tenant:zeroroot-ai", "team:red"). Empty for KIND_DIRECT.
	sourceObject?: string @protobuf(2,string,name=source_object)
}
