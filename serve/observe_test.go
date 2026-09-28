// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Zero Root AI

package serve

import (
	"testing"

	"google.golang.org/protobuf/proto"

	"github.com/zeroroot-ai/sdk/agent"
	harnesspb "github.com/zeroroot-ai/sdk/api/gen/gibson/harness/v1"
)

func TestObservationToProto_Host(t *testing.T) {
	req, err := observationToProto(agent.HostObservation{
		Address:    "10.0.0.5",
		SSHHostKey: "AAAAkey",
		CloudID:    "i-abc123",
		Ports: []agent.PortObservation{
			{Number: 22, Protocol: "tcp", Service: "ssh", Product: "OpenSSH", Version: "8.9p1"},
			{Number: 80, Protocol: "tcp", Service: "http"},
		},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	h := req.GetHost()
	if h == nil {
		t.Fatal("expected host observation in request")
	}
	if h.Address != "10.0.0.5" || h.SshHostKey != "AAAAkey" || h.CloudId != "i-abc123" {
		t.Fatalf("host identity not mapped: %+v", h)
	}
	if len(h.Ports) != 2 || h.Ports[0].Number != 22 || h.Ports[0].Product != "OpenSSH" || h.Ports[1].Service != "http" {
		t.Fatalf("ports not mapped: %+v", h.Ports)
	}
	// Scope must NOT be carried on the observation (daemon derives it).
	if req.Context != nil {
		t.Fatalf("observation should not carry context/scope, got %+v", req.Context)
	}
}

func TestObservationToProto_Domain(t *testing.T) {
	req, err := observationToProto(agent.DomainObservation{Name: "example.com"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if d := req.GetDomain(); d == nil || d.Name != "example.com" {
		t.Fatalf("domain not mapped: %+v", req.GetDomain())
	}
}

func TestObservationToProto_Subdomain(t *testing.T) {
	req, err := observationToProto(agent.SubdomainObservation{
		FQDN: "api.example.com", Domain: "example.com", Addresses: []string{"10.0.0.5"},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	s := req.GetSubdomain()
	if s == nil || s.Fqdn != "api.example.com" || s.Domain != "example.com" || len(s.Addresses) != 1 {
		t.Fatalf("subdomain not mapped: %+v", s)
	}
}

func TestObservationToProto_Memory(t *testing.T) {
	req, err := observationToProto(agent.MemoryObservation{
		Text:      "The dashboard never opens a direct daemon gRPC channel.",
		Kind:      "convention",
		Tags:      []string{"dashboard", "envoy"},
		SourceRef: "CLAUDE.md",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	m := req.GetMemory()
	if m == nil {
		t.Fatal("expected memory observation in request")
	}
	if m.Text != "The dashboard never opens a direct daemon gRPC channel." || m.Kind != "convention" || m.SourceRef != "CLAUDE.md" {
		t.Fatalf("memory fields not mapped: %+v", m)
	}
	if len(m.Tags) != 2 || m.Tags[0] != "dashboard" || m.Tags[1] != "envoy" {
		t.Fatalf("tags not mapped: %+v", m.Tags)
	}
	// Scope and tenant are server-side. The request must not carry context.
	if req.Context != nil {
		t.Fatalf("observation should not carry context/scope, got %+v", req.Context)
	}
}

// TestObservationToProto_LifecycleEntity: the lifecycle sighting survives the
// wire with its label, both property maps and its edges intact (sdk#537).
//
// It is the shape a triage or scan agent uses to say "this Image contains this
// Package", so a field lost in mapping would land a node with no route to the
// Application and read back as unreachable — the silent false negative the
// reachability read exists to prevent.
func TestObservationToProto_LifecycleEntity(t *testing.T) {
	req, err := observationToProto(agent.LifecycleEntityObservation{
		Label:        "Package",
		IDProperties: map[string]string{"purl": "pkg:npm/lodash@4.17.20"},
		Properties:   map[string]string{"name": "lodash", "version": "4.17.20"},
		Edges: []agent.LifecycleEntityEdge{{
			Type:               "CONTAINS",
			TargetLabel:        "Image",
			TargetIDProperties: map[string]string{"digest": "sha256:abc"},
		}},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	e := req.GetLifecycleEntity()
	if e == nil {
		t.Fatal("expected lifecycle entity observation in request")
	}
	if e.Label != "Package" {
		t.Fatalf("label not mapped: %q", e.Label)
	}
	if got := e.IdProperties["purl"]; got != "pkg:npm/lodash@4.17.20" {
		t.Fatalf("id properties not mapped: %+v", e.IdProperties)
	}
	if e.Properties["name"] != "lodash" || e.Properties["version"] != "4.17.20" {
		t.Fatalf("properties not mapped: %+v", e.Properties)
	}
	if len(e.Edges) != 1 {
		t.Fatalf("expected 1 edge, got %d", len(e.Edges))
	}
	edge := e.Edges[0]
	if edge.Type != "CONTAINS" || edge.TargetLabel != "Image" {
		t.Fatalf("edge not mapped: %+v", edge)
	}
	if edge.TargetIdProperties["digest"] != "sha256:abc" {
		t.Fatalf("edge target identity not mapped: %+v", edge.TargetIdProperties)
	}
	// Scope and tenant are server-side. The request must not carry context.
	if req.Context != nil {
		t.Fatalf("observation should not carry context/scope, got %+v", req.Context)
	}
}

// TestObservationToProto_LifecycleEntity_EmptyEdgesStayEmpty: an entity with no
// edges is a legitimate sighting — "this Package exists" — and must not acquire
// a phantom edge from a nil slice.
func TestObservationToProto_LifecycleEntity_EmptyEdgesStayEmpty(t *testing.T) {
	req, err := observationToProto(agent.LifecycleEntityObservation{
		Label:        "Application",
		IDProperties: map[string]string{"key": "customer-portal"},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	e := req.GetLifecycleEntity()
	if e == nil {
		t.Fatal("expected lifecycle entity observation in request")
	}
	if len(e.Edges) != 0 {
		t.Fatalf("expected no edges, got %+v", e.Edges)
	}
}

// TestObservationToProto_Hypothesis: a hypothesis carries the proposer,
// confidence, claim and referenced entities onto the wire (ADR-0021, sdk#70).
// It is the agent's own reasoning, not a sighting, but it still rides the
// normal emit-only ObserveRequest oneof — no raw graph write.
func TestObservationToProto_Hypothesis(t *testing.T) {
	req, err := observationToProto(agent.HypothesisObservation{
		Proposer:   "triage-agent",
		Confidence: 0.65,
		Claim:      "port 6443 on 10.0.0.5 is unauthenticated",
		Technique:  "unauthenticated-service-probe",
		References: []agent.ReferencedEntity{
			{Label: "Host", IDProperties: map[string]string{"address": "10.0.0.5"}},
		},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	h := req.GetHypothesis()
	if h == nil {
		t.Fatal("expected hypothesis observation in request")
	}
	if h.Proposer != "triage-agent" {
		t.Fatalf("proposer not mapped: %+v", h)
	}
	if h.Confidence != 0.65 {
		t.Fatalf("confidence not mapped: %+v", h)
	}
	if h.Claim != "port 6443 on 10.0.0.5 is unauthenticated" {
		t.Fatalf("claim not mapped: %+v", h)
	}
	if h.Technique != "unauthenticated-service-probe" {
		t.Fatalf("technique not mapped: %+v", h)
	}
	if len(h.References) != 1 || h.References[0].Label != "Host" {
		t.Fatalf("references not mapped: %+v", h.References)
	}
	if h.References[0].IdProperties["address"] != "10.0.0.5" {
		t.Fatalf("reference id properties not mapped: %+v", h.References[0].IdProperties)
	}
	// Scope and tenant are server-side. The request must not carry context.
	if req.Context != nil {
		t.Fatalf("observation should not carry context/scope, got %+v", req.Context)
	}
}

// TestObservationToProto_Hypothesis_EmptyReferencesStayEmpty: a hypothesis
// need not name entities up front (some claims are about the environment in
// general), and a nil slice must not become a phantom reference.
func TestObservationToProto_Hypothesis_EmptyReferencesStayEmpty(t *testing.T) {
	req, err := observationToProto(agent.HypothesisObservation{
		Proposer:   "triage-agent",
		Confidence: 0.2,
		Claim:      "the cluster admission controller is misconfigured",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	h := req.GetHypothesis()
	if h == nil {
		t.Fatal("expected hypothesis observation in request")
	}
	if len(h.References) != 0 {
		t.Fatalf("expected no references, got %+v", h.References)
	}
}

// TestObservationToProto_Hypothesis_WireRoundTrip: the hypothesis must survive
// an actual proto marshal/unmarshal, not just struct construction — this is
// the "round-trips through the wire" acceptance criterion (sdk#70).
func TestObservationToProto_Hypothesis_WireRoundTrip(t *testing.T) {
	req, err := observationToProto(agent.HypothesisObservation{
		Proposer:   "triage-agent",
		Confidence: 0.9,
		Claim:      "the lodash dependency is exploitable via prototype pollution",
		Technique:  "dependency-cve-match",
		References: []agent.ReferencedEntity{
			{Label: "Package", IDProperties: map[string]string{"purl": "pkg:npm/lodash@4.17.20"}},
			{Label: "Application", IDProperties: map[string]string{"key": "customer-portal"}},
		},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	wire, err := proto.Marshal(req)
	if err != nil {
		t.Fatalf("marshal failed: %v", err)
	}

	got := &harnesspb.ObserveRequest{}
	if err := proto.Unmarshal(wire, got); err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}

	h := got.GetHypothesis()
	if h == nil {
		t.Fatal("expected hypothesis observation after round trip")
	}
	if h.Proposer != "triage-agent" || h.Confidence != 0.9 {
		t.Fatalf("proposer/confidence lost in round trip: %+v", h)
	}
	if h.Claim != "the lodash dependency is exploitable via prototype pollution" {
		t.Fatalf("claim lost in round trip: %+v", h)
	}
	if h.Technique != "dependency-cve-match" {
		t.Fatalf("technique lost in round trip: %+v", h)
	}
	if len(h.References) != 2 {
		t.Fatalf("references lost in round trip: %+v", h.References)
	}
	if h.References[0].Label != "Package" || h.References[0].IdProperties["purl"] != "pkg:npm/lodash@4.17.20" {
		t.Fatalf("first reference lost in round trip: %+v", h.References[0])
	}
	if h.References[1].Label != "Application" || h.References[1].IdProperties["key"] != "customer-portal" {
		t.Fatalf("second reference lost in round trip: %+v", h.References[1])
	}
}
