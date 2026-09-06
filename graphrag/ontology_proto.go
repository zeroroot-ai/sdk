// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Zero Root AI

package graphrag

import (
	graphragpb "github.com/zeroroot-ai/sdk/api/gen/gibson/graphrag/v1"
)

// OntologyExtensionToProto converts a Go OntologyExtension into the
// proto-canonical form used on the wire (RegisterComponentRequest.ontology_extension).
//
// A zero-value OntologyExtension returns a non-nil but empty proto message —
// the caller decides whether to send it. Use IsZero to skip sending when no
// ontology contribution is being made.
func OntologyExtensionToProto(ext OntologyExtension) *graphragpb.OntologyExtension {
	hierarchies := make([]*graphragpb.HierarchyDef, 0, len(ext.Hierarchies))
	for _, h := range ext.Hierarchies {
		hierarchies = append(hierarchies, &graphragpb.HierarchyDef{
			NodeType:   h.NodeType,
			Label:      h.Label,
			SubClassOf: h.SubClassOf,
		})
	}

	equivalences := make([]*graphragpb.SameAsPair, 0, len(ext.Equivalences))
	for _, e := range ext.Equivalences {
		equivalences = append(equivalences, &graphragpb.SameAsPair{
			IriA: e[0],
			IriB: e[1],
		})
	}

	ifps := make([]*graphragpb.IFPDef, 0, len(ext.IFPs))
	for _, i := range ext.IFPs {
		ifps = append(ifps, &graphragpb.IFPDef{
			NodeType: i.NodeType,
			Property: i.Property,
		})
	}

	return &graphragpb.OntologyExtension{
		Prefixes:     ext.Prefixes,
		Hierarchies:  hierarchies,
		Equivalences: equivalences,
		Ifps:         ifps,
		RawTriples:   ext.RawTriples,
	}
}

// OntologyExtensionFromProto converts the wire-form OntologyExtension into
// the Go struct the daemon's reasoner consumes. A nil input returns a
// zero-value OntologyExtension.
func OntologyExtensionFromProto(p *graphragpb.OntologyExtension) OntologyExtension {
	if p == nil {
		return OntologyExtension{}
	}

	hierarchies := make([]HierarchyDef, 0, len(p.GetHierarchies()))
	for _, h := range p.GetHierarchies() {
		hierarchies = append(hierarchies, HierarchyDef{
			NodeType:   h.GetNodeType(),
			Label:      h.GetLabel(),
			SubClassOf: h.GetSubClassOf(),
		})
	}

	equivalences := make([][2]string, 0, len(p.GetEquivalences()))
	for _, e := range p.GetEquivalences() {
		equivalences = append(equivalences, [2]string{e.GetIriA(), e.GetIriB()})
	}

	ifps := make([]IFPDef, 0, len(p.GetIfps()))
	for _, i := range p.GetIfps() {
		ifps = append(ifps, IFPDef{
			NodeType: i.GetNodeType(),
			Property: i.GetProperty(),
		})
	}

	return OntologyExtension{
		Prefixes:     p.GetPrefixes(),
		Hierarchies:  hierarchies,
		Equivalences: equivalences,
		IFPs:         ifps,
		RawTriples:   p.GetRawTriples(),
	}
}

// IsZero reports whether the OntologyExtension carries no ontology
// contribution. Callers should skip populating
// RegisterComponentRequest.ontology_extension when this is true to avoid
// sending an empty payload over the wire.
func (e OntologyExtension) IsZero() bool {
	return len(e.Prefixes) == 0 &&
		len(e.Hierarchies) == 0 &&
		len(e.Equivalences) == 0 &&
		len(e.IFPs) == 0 &&
		len(e.RawTriples) == 0
}
