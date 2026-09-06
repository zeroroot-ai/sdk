// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Zero Root AI

package agent

// Observation is a raw sighting an agent emits into the World (ECS brain, ADR-0007).
//
// Agents are emit-only workers: they report what they saw and the brain resolves
// identity and topology — agents never author graph nodes or relationships, and
// never read the graph back (the relevant world state is ambiently projected to
// them, ADR-0001). Scope is NOT carried on an observation; the daemon derives it
// from the mission context (the agent's vantage, ADR-0002).
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
	// record is kept, not deleted — ADR-0002).
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
// lands as an Observation with its residue preserved (ADR-0012). So an agent can
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
