// gibson.target.v1 is the customer-facing target contract. A Target is a
// system to be assessed by a mission. The id (UUID) is the canonical identity;
// every other field is metadata. Nothing resolves targets by name — clients
// reference a target solely by its server-minted UUID.
//
// This package mirrors the daemon's types.Target / types.TargetFilter storage
// shape. The daemon, the `gibson` CLI (ADK), and the dashboard all consume the
// generated bindings of this file; hand-written parallel representations are
// forbidden.
//
// Schema evolution policy: message field numbers are append-only — no
// renumbers, no reuse. Type-of-field changes are breaking and require a ship
// sequence across the SDK and every consumer.
package targetpb

import "time"

// Target represents a system to be assessed by a mission.
//
// `id` is the canonical UUID identity, assigned by the daemon on CreateTarget;
// clients never invent it. `name` and all remaining fields are metadata. Two
// targets may share a name — only the UUID is unique.
#Target: {
	// id is the server-minted UUID. Empty on a CreateTarget request (ignored if
	// set); always populated on responses.
	id?: string @protobuf(1,string)

	// name is a human-readable label (metadata only, not an identifier).
	name?: string @protobuf(2,string)

	// type is the schema-based target type.
	type?: string @protobuf(3,string)

	// provider identifies the backing provider for model/provider targets.
	provider?: string @protobuf(4,string)

	// connection holds schema-based connection parameters (e.g. url, headers).
	connection?: {} @protobuf(5,google.protobuf.Struct)

	// model is the model identifier for model targets.
	model?: string @protobuf(6,string)

	// config holds free-form target configuration.
	config?: {} @protobuf(7,google.protobuf.Struct)

	// capabilities lists capability tags advertised by the target.
	capabilities?: [...string] @protobuf(8,string)

	// auth_type is the authentication scheme for the target.
	authType?: string @protobuf(9,string,name=auth_type)

	// credential_id references a stored credential. Empty when none.
	credentialId?: string @protobuf(10,string,name=credential_id)

	// status is the target lifecycle status.
	status?: string @protobuf(11,string)

	// description is free-text describing the target.
	description?: string @protobuf(12,string)

	// tags are user-assigned labels for filtering/organization.
	tags?: [...string] @protobuf(13,string)

	// timeout is the per-operation timeout in seconds.
	timeout?: int32 @protobuf(14,int32)

	// created_at is when the target was registered.
	createdAt?: time.Time @protobuf(15,google.protobuf.Timestamp,name=created_at)

	// updated_at is when the target was last modified.
	updatedAt?: time.Time @protobuf(16,google.protobuf.Timestamp,name=updated_at)

	// url is the target endpoint. Deprecated: prefer connection["url"].
	url?: string @protobuf(17,string)

	// headers are default HTTP headers. Deprecated: prefer connection["headers"].
	headers?: {
		[string]: string
	} @protobuf(18,map[string]string)
}

// TargetFilter narrows ListTargets results. Mirrors types.TargetFilter.
// All fields are optional; an empty filter returns the tenant's targets.
#TargetFilter: {
	// provider filters by backing provider.
	provider?: string @protobuf(1,string)

	// type filters by schema-based target type.
	type?: string @protobuf(2,string)

	// status filters by lifecycle status.
	status?: string @protobuf(3,string)

	// tags filters to targets carrying all of the given tags.
	tags?: [...string] @protobuf(4,string)

	// limit caps the number of results. Zero means the server default.
	limit?: int32 @protobuf(5,int32)

	// offset skips the first N results for pagination.
	offset?: int32 @protobuf(6,int32)
}
