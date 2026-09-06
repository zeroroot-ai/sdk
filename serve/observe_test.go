// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Zero Root AI

package serve

import (
	"testing"

	"github.com/zeroroot-ai/sdk/agent"
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
