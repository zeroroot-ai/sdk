// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Zero Root AI

package agent

// Observation is a raw sighting an agent emits into the World (ECS brain, ADR-0107).
//
// Agents are emit-only workers: they report what they saw and the brain resolves
// identity and topology — agents never author graph nodes or relationships, and
// never read the graph back (the relevant world state is ambiently projected to
// them, ADR-0101). Scope is NOT carried on an observation; the daemon derives it
// from the mission context (the agent's vantage, ADR-0102).
//
// Observation is a closed sum type — the concrete types in this package
// (HostObservation, …) are the only valid observations. The brain folds each into
// a Timeline event; a graph projector materializes the World into the knowledge
// graph as a read-model.
type Observation interface {
	isObservation()
}

// HostObservation reports a host seen at Address, optionally with strong identity
// signals (SSH host key / cloud instance id) that identify it across addresses, and
// the ports observed open in this sighting. The brain resolves this to a host
// entity within the mission's scope (creating one or enriching an existing one) and
// reconciles its ports.
type HostObservation struct {
	// Address is the host's address within the mission scope (IP, hostname, …).
	Address string
	// SSHHostKey is a strong identity signal: stable across addresses (optional).
	SSHHostKey string
	// CloudID is a strong identity signal: cloud instance id (optional).
	CloudID string
	// Ports are the ports observed open in this sighting, with optional service
	// detail. Ports previously seen but absent here are treated as closed (their
	// record is kept, not deleted — ADR-0102).
	Ports []PortObservation
}

func (HostObservation) isObservation() {}

// PortObservation is an observed open port and its optional service detail. Service
// fields are enriched progressively across observations — a deeper scan refines
// them, a barer scan never erases detail already established.
type PortObservation struct {
	// Number is the port number.
	Number int
	// Protocol is the transport, e.g. "tcp" / "udp" (optional).
	Protocol string
	// Service is the service name, e.g. "ssh" / "http" (optional).
	Service string
	// Product is the service product, e.g. "OpenSSH" (optional).
	Product string
	// Version is the product version, e.g. "8.9p1" (optional).
	Version string
	// Endpoints are paths observed on this service, e.g. "/login" (optional).
	Endpoints []EndpointObservation
	// Technologies are technologies fingerprinted on this service (optional).
	Technologies []TechnologyObservation
	// Certificate is the TLS certificate served on this port, if any.
	Certificate *CertificateObservation
}

// EndpointObservation is a path observed on a service (e.g. an HTTP route).
type EndpointObservation struct {
	// Path is the endpoint path, e.g. "/api/login".
	Path string
	// Status is the observed HTTP status code (optional, 0 if unknown).
	Status int
}

// TechnologyObservation is a technology fingerprinted on a service.
type TechnologyObservation struct {
	// Name is the technology name, e.g. "nginx" / "WordPress".
	Name string
	// Version is the detected version (optional).
	Version string
}

// CertificateObservation is a TLS certificate served on a port. Identity is the
// fingerprint.
type CertificateObservation struct {
	Fingerprint string
	Subject     string
	Issuer      string
	// NotAfter is the expiry, RFC3339 (optional).
	NotAfter string
}

// DomainObservation reports a registrable domain seen in scope (e.g. "example.com").
// The brain resolves it to a domain entity by (scope, name).
type DomainObservation struct {
	// Name is the domain name, e.g. "example.com".
	Name string
}

func (DomainObservation) isObservation() {}

// SubdomainObservation reports a subdomain (FQDN) and, optionally, its parent
// domain and the addresses it resolves to. The brain resolves it by (scope, fqdn),
// links it under its parent domain (HAS_SUBDOMAIN), and to the hosts it resolves
// to (RESOLVES_TO).
type SubdomainObservation struct {
	// FQDN is the fully-qualified subdomain, e.g. "api.example.com".
	FQDN string
	// Domain is the parent registrable domain, e.g. "example.com" (optional).
	Domain string
	// Addresses are the addresses this subdomain resolves to (optional); each is
	// a host coordinate within the same scope.
	Addresses []string
}

func (SubdomainObservation) isObservation() {}

// CredentialObservation reports a discovered credential. Identity is the secret
// hash (never the raw secret) — the brain resolves it by (scope, hash).
type CredentialObservation struct {
	// SecretHash is a stable hash of the secret material — the identity. Agents
	// MUST NOT send raw secrets.
	SecretHash string
	// Username is the associated username/principal (optional).
	Username string
	// Kind classifies the credential, e.g. "password" / "api_key" / "ssh_key".
	Kind string
}

func (CredentialObservation) isObservation() {}

// AccountObservation reports a discovered account/principal. Identity is
// (scope, identifier).
type AccountObservation struct {
	// Identifier is the account identity within scope, e.g. "admin" / a UID.
	Identifier string
	// Kind classifies the account, e.g. "local" / "domain" / "service".
	Kind string
}

func (AccountObservation) isObservation() {}

// MemoryObservation reports a fact the agent wants to keep across runs, in the
// agent's own words. It is the memory write for coding agents: a memory is a
// World observation, not a store of its own (gibson#1593, decision 10). The
// brain records one Observation per sighting and the projector materializes it
// as an Observation node with shape "Memory".
type MemoryObservation struct {
	// Text is the fact.
	Text string
	// Kind is a short category, e.g. "convention" / "decision" / "layout".
	Kind string
	// Tags help recall. Free-form, lower-case (optional).
	Tags []string
	// SourceRef names where the fact came from: a path, a URL, or a work id
	// (optional).
	SourceRef string
}

func (MemoryObservation) isObservation() {}

// LifecycleEntityObservation reports a sighting of a typed application-lifecycle
// entity — an Application, Repository, Image, Package, Deployment,
// Vulnerability, MergeRequest, Pipeline or Control — and the edges it was seen
// to have (gibson#1656).
//
// Admission is the Taxonomy's decision, not the agent's. The daemon puts Label
// and every edge type to the global Taxonomy: an admitted shape becomes a typed
// node, and one the Taxonomy does not admit is neither rejected nor lost — it
// lands as an Observation with its residue preserved (ADR-0112). So an agent can
// always write, and can never invent schema.
//
// This stays a sighting, not a graph write: the agent reports what it saw, the
// brain resolves identity, and the projector materializes it. Naming an existing
// node's identity enriches that node rather than creating a second one.
type LifecycleEntityObservation struct {
	// Label is the Taxonomy label, e.g. "Package".
	Label string
	// IDProperties identify the entity. One property is the identity; several
	// are folded into a composite key in sorted order, so the same entity
	// observed twice resolves to the same node whatever order the map came in.
	// An entity with no identity property records nothing — there would be no
	// stable node to project.
	IDProperties map[string]string
	// Properties are the entity's non-identifying facts. A later sighting's
	// properties take precedence; a sighting that omits a property never erases
	// what an earlier one established.
	Properties map[string]string
	// Edges are the outgoing relationships seen in this sighting. Both ends and
	// the relationship type must be admitted by the Taxonomy, or the edge is
	// skipped while the entity itself still lands.
	Edges []LifecycleEntityEdge
}

func (LifecycleEntityObservation) isObservation() {}

// HypothesisObservation reports an agent's own reasoning: a proposed, unproven
// claim it wants the fleet to test, not a sighting (ADR-0121). It is attributed
// to the proposing agent and carries a confidence, and it stays unverified
// until it settles (see the betting / settlement slices, ADR-0122 / ADR-0123).
//
// This keeps the agent write surface emit-only: an agent still only emits
// observations, never a raw graph node or edge (ADR-0107). A HypothesisObservation
// differs from every other Observation variant in kind, not in surface — it
// carries the agent's inference rather than something it sensed.
type HypothesisObservation struct {
	// HypothesisID is the agent-chosen identifier for this claim — the SAME
	// value the agent later names as Bet.HypothesisID when it stakes on this
	// claim, and that a settlement path (Engine.SettleBetByHITL, gibson#280)
	// records against. This is the one join key across Hypothesis, Bet and
	// BetSettlement (gibson#339). Optional: empty preserves a Hypothesis
	// with no bettable identity — it still folds as a claim (ADR-0121), it
	// just cannot be staked on or settled by id.
	HypothesisID string
	// Proposer identifies the agent making the claim.
	Proposer string
	// Confidence is the proposer's calibrated confidence in the claim, in [0,1].
	Confidence float64
	// Claim states the hypothesis in a form the brain can later settle (see the
	// settlement slice, ADR-0123), e.g. "port 6443 on 10.0.0.5 is unauthenticated".
	Claim string
	// References names the entities the claim is about, by label and identity —
	// the agent does not know node ids, and must not be able to guess them.
	References []ReferencedEntity
	// Technique names the technique this hypothesis exercises. Reputation
	// keys on technique x environment (ADR-0122, gibson#333/#284), so this
	// must ride on the Hypothesis the same way it already rides on Bet.
	// Optional and additive: empty means no technique signal (resolves to
	// the neutral prior), the same behavior as before this field existed.
	Technique string
}

func (HypothesisObservation) isObservation() {}

// ReferencedEntity names an entity a Hypothesis is about, by its Taxonomy label
// and identity properties rather than a node id — the same identity scheme
// LifecycleEntityEdge uses to name its target.
type ReferencedEntity struct {
	// Label is the Taxonomy label, e.g. "Host" / "Package".
	Label string
	// IDProperties identify the entity, folded the same way as
	// LifecycleEntityObservation.IDProperties.
	IDProperties map[string]string
}

// LifecycleEntityEdge is one outgoing relationship to another typed entity,
// named by its label and identity rather than by a node id — the emitter does
// not know node ids, and must not be able to guess them.
type LifecycleEntityEdge struct {
	// Type is a Taxonomy relationship type, e.g. "CONTAINS".
	Type string
	// TargetLabel is the Taxonomy label of the other end.
	TargetLabel string
	// TargetIDProperties identify the other end, folded the same way as
	// IDProperties.
	TargetIDProperties map[string]string
}
