// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Zero Root AI

package agent

// worldview.go is the read half of the emit-only worker contract (sdk#341,
// gibson#1377). Observe is the agent's only write; WorldView is its only read.
//
// A WorldView is a slice of the tenant World the daemon projected FOR this
// caller — not a query the caller composed. There is no filter, no selector and
// no identifier on the way in beyond handles the caller was already given, so
// the slice boundary is a server-side property the agent cannot argue with.

// EntityKind names what a WorldEntity is.
type EntityKind string

// The entity kinds a slice can contain. These mirror the brain's entity types;
// an unrecognised kind from a newer daemon arrives as its wire string rather
// than being dropped, so an old agent degrades to "something I do not model"
// instead of losing the entity.
const (
	EntityKindUnspecified EntityKind = ""
	EntityKindHost        EntityKind = "host"
	EntityKindDomain      EntityKind = "domain"
	EntityKindSubdomain   EntityKind = "subdomain"
	EntityKindCredential  EntityKind = "credential"
	EntityKindAccount     EntityKind = "account"
	EntityKindFinding     EntityKind = "finding"
)

// Handle is an opaque reference to an entity the agent was shown.
//
// It is minted by the daemon per slice and is not constructible: there is no
// arithmetic, no ordering and no embedded brain id, so an agent cannot derive
// the handle of an entity it was not shown. Passing a handle the daemon did not
// issue to this caller is refused.
//
// A handle stays valid across re-projections of the same slice. It stops being
// valid when the entity it names is gone — which the agent should read as "that
// thing no longer exists", not as an expiry to retry around.
type Handle string

// WorldEntity is one entity in the agent's slice of the World.
type WorldEntity struct {
	// Handle is the only name the agent has for this entity. Use it for Focus.
	Handle Handle
	// Kind is what the entity is.
	Kind EntityKind
	// Label is the entity's coordinate as observed — an address, an FQDN, a
	// finding title. It is for reading, not for referring: two entities may
	// legitimately share a label across scopes, and only Handle is a reference.
	Label string
	// Attributes is the projected detail. The unfocused slice carries a
	// level-of-detail summary; a focused entity carries everything the daemon
	// has. Keys are chosen by the daemon and may grow over time.
	Attributes map[string]string
}

// WorldView is the projected slice returned to an agent.
type WorldView struct {
	// Entities are the entities in the slice, in a deterministic order.
	Entities []WorldEntity
	// Truncated reports that the slice hit the daemon's projection cap and
	// entities were dropped. The cap is a server-side budget; there is no way to
	// raise it from the agent side. A truncated slice is still a valid one.
	Truncated bool
}
