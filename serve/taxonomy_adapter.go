// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Zero Root AI

package serve

import (
	harnesspb "github.com/zeroroot-ai/sdk/api/gen/gibson/harness/v1"
	"github.com/zeroroot-ai/sdk/graphrag"
)

// TaxonomyAdapter implements graphrag.TaxonomyIntrospector from proto data.
// It bridges the gRPC taxonomy response to the SDK taxonomy interface,
// enabling standalone agents to access taxonomy data fetched from Gibson.
type TaxonomyAdapter struct {
	version           string
	nodeTypes         map[string]*graphrag.NodeTypeInfo
	relationshipTypes map[string]*graphrag.RelationshipTypeInfo
	techniques        map[string]*graphrag.TechniqueInfo
	targetTypes       map[string]*targetTypeInfo
	techniqueTypes    map[string]*techniqueTypeInfo
	capabilities      map[string]*capabilityInfo

	// Slice caches for list methods
	nodeTypesList         []string
	relationshipTypesList []string
	techniqueIDsList      []string
}

// targetTypeInfo holds target type metadata (internal to adapter).
type targetTypeInfo struct {
}

// techniqueTypeInfo holds technique type metadata (internal to adapter).
type techniqueTypeInfo struct {
}

// capabilityInfo holds capability metadata (internal to adapter).
type capabilityInfo struct {
}

// NewTaxonomyAdapter creates a TaxonomyAdapter from a proto GetTaxonomySchemaResponse.
// It converts all proto types to SDK graphrag types and builds lookup maps.
func NewTaxonomyAdapter(resp *harnesspb.GetTaxonomySchemaResponse) *TaxonomyAdapter {
	if resp == nil {
		return &TaxonomyAdapter{
			version:               "",
			nodeTypes:             make(map[string]*graphrag.NodeTypeInfo),
			relationshipTypes:     make(map[string]*graphrag.RelationshipTypeInfo),
			techniques:            make(map[string]*graphrag.TechniqueInfo),
			targetTypes:           make(map[string]*targetTypeInfo),
			techniqueTypes:        make(map[string]*techniqueTypeInfo),
			capabilities:          make(map[string]*capabilityInfo),
			nodeTypesList:         []string{},
			relationshipTypesList: []string{},
			techniqueIDsList:      []string{},
		}
	}

	a := &TaxonomyAdapter{
		version:           resp.Version,
		nodeTypes:         make(map[string]*graphrag.NodeTypeInfo, len(resp.NodeTypes)),
		relationshipTypes: make(map[string]*graphrag.RelationshipTypeInfo, len(resp.RelationshipTypes)),
		techniques:        make(map[string]*graphrag.TechniqueInfo, len(resp.Techniques)),
		targetTypes:       make(map[string]*targetTypeInfo, len(resp.TargetTypes)),
		techniqueTypes:    make(map[string]*techniqueTypeInfo, len(resp.TechniqueTypes)),
		capabilities:      make(map[string]*capabilityInfo, len(resp.Capabilities)),
	}

	// Convert node types
	a.nodeTypesList = make([]string, 0, len(resp.NodeTypes))
	for _, nt := range resp.NodeTypes {
		info := &graphrag.NodeTypeInfo{
			Type:        nt.Type,
			Name:        nt.Name,
			Category:    nt.Category,
			Description: nt.Description,
			Properties:  convertProtoProperties(nt.Properties),
		}
		a.nodeTypes[nt.Type] = info
		a.nodeTypesList = append(a.nodeTypesList, nt.Type)
	}

	// Convert relationship types
	a.relationshipTypesList = make([]string, 0, len(resp.RelationshipTypes))
	for _, rt := range resp.RelationshipTypes {
		info := &graphrag.RelationshipTypeInfo{
			Type:          rt.Type,
			Name:          rt.Name,
			Category:      rt.Category,
			Description:   rt.Description,
			FromTypes:     rt.FromTypes,
			ToTypes:       rt.ToTypes,
			Bidirectional: rt.Bidirectional,
		}
		a.relationshipTypes[rt.Type] = info
		a.relationshipTypesList = append(a.relationshipTypesList, rt.Type)
	}

	// Convert techniques
	a.techniqueIDsList = make([]string, 0, len(resp.Techniques))
	for _, t := range resp.Techniques {
		info := &graphrag.TechniqueInfo{
			ID:          t.TechniqueId,
			Name:        t.Name,
			Taxonomy:    t.Taxonomy,
			Description: t.Description,
			Tactic:      t.Tactic,
		}
		a.techniques[t.TechniqueId] = info
		a.techniqueIDsList = append(a.techniqueIDsList, t.TechniqueId)
	}

	// Convert target types
	for _, tt := range resp.TargetTypes {
		a.targetTypes[tt.Type] = &targetTypeInfo{}
	}

	// Convert technique types
	for _, tt := range resp.TechniqueTypes {
		a.techniqueTypes[tt.Type] = &techniqueTypeInfo{}
	}

	// Convert capabilities
	for _, c := range resp.Capabilities {
		a.capabilities[c.Id] = &capabilityInfo{}
	}

	return a
}

// convertProtoProperties converts proto TaxonomyProperty to SDK PropertyInfo.
func convertProtoProperties(props []*harnesspb.TaxonomyProperty) []graphrag.PropertyInfo {
	if props == nil {
		return nil
	}

	result := make([]graphrag.PropertyInfo, len(props))
	for i := range props {
		result[i] = graphrag.PropertyInfo{}
	}
	return result
}

// ============================================================================
// TaxonomyReader Interface Implementation
// ============================================================================

// Version returns the taxonomy version string.
func (a *TaxonomyAdapter) Version() string {
	return a.version
}

// ============================================================================
// TaxonomyIntrospector Interface Implementation
// ============================================================================

// NodeTypes returns all canonical node type names.
func (a *TaxonomyAdapter) NodeTypes() []string {
	// Return a copy to prevent external modification
	result := make([]string, len(a.nodeTypesList))
	copy(result, a.nodeTypesList)
	return result
}

// RelationshipTypes returns all canonical relationship type names.
func (a *TaxonomyAdapter) RelationshipTypes() []string {
	// Return a copy to prevent external modification
	result := make([]string, len(a.relationshipTypesList))
	copy(result, a.relationshipTypesList)
	return result
}

// TechniqueIDs returns all technique IDs, optionally filtered by source.
// Pass "" for all techniques, "mitre" for MITRE ATT&CK, or "arcanum" for Arcanum.
func (a *TaxonomyAdapter) TechniqueIDs(source string) []string {
	if source == "" {
		// Return all techniques
		result := make([]string, len(a.techniqueIDsList))
		copy(result, a.techniqueIDsList)
		return result
	}

	// Filter by source
	var result []string
	for _, id := range a.techniqueIDsList {
		if info, ok := a.techniques[id]; ok && info.Taxonomy == source {
			result = append(result, id)
		}
	}
	return result
}

// NodeTypeInfo returns detailed info about a node type.
// Returns nil if the node type is not found.
func (a *TaxonomyAdapter) NodeTypeInfo(typeName string) *graphrag.NodeTypeInfo {
	info, ok := a.nodeTypes[typeName]
	if !ok {
		return nil
	}

	// Return a copy to prevent external modification
	result := &graphrag.NodeTypeInfo{
		Type:        info.Type,
		Name:        info.Name,
		Category:    info.Category,
		Description: info.Description,
	}

	if len(info.Properties) > 0 {
		result.Properties = make([]graphrag.PropertyInfo, len(info.Properties))
		copy(result.Properties, info.Properties)
	}

	return result
}

// RelationshipTypeInfo returns detailed info about a relationship type.
// Returns nil if the relationship type is not found.
func (a *TaxonomyAdapter) RelationshipTypeInfo(typeName string) *graphrag.RelationshipTypeInfo {
	info, ok := a.relationshipTypes[typeName]
	if !ok {
		return nil
	}

	// Return a copy to prevent external modification
	result := &graphrag.RelationshipTypeInfo{
		Type:          info.Type,
		Name:          info.Name,
		Category:      info.Category,
		Description:   info.Description,
		Bidirectional: info.Bidirectional,
	}

	if len(info.FromTypes) > 0 {
		result.FromTypes = make([]string, len(info.FromTypes))
		copy(result.FromTypes, info.FromTypes)
	}

	if len(info.ToTypes) > 0 {
		result.ToTypes = make([]string, len(info.ToTypes))
		copy(result.ToTypes, info.ToTypes)
	}

	return result
}

// TechniqueInfo returns detailed info about a technique.
// Returns nil if the technique is not found.
func (a *TaxonomyAdapter) TechniqueInfo(techniqueID string) *graphrag.TechniqueInfo {
	info, ok := a.techniques[techniqueID]
	if !ok {
		return nil
	}

	// Return a copy to prevent external modification
	return &graphrag.TechniqueInfo{
		ID:          info.ID,
		Name:        info.Name,
		Taxonomy:    info.Taxonomy,
		Description: info.Description,
		Tactic:      info.Tactic,
	}
}

// ============================================================================
// Additional Methods (not part of interface)
// ============================================================================

// ExtensionNames returns names of all registered extensions.
// TaxonomyAdapter doesn't track extensions, so this returns an empty slice.
func (a *TaxonomyAdapter) ExtensionNames() []string {
	return []string{}
}

// ExtensionInfo returns full extension definition.
// TaxonomyAdapter doesn't track extensions, so this always returns nil.
func (a *TaxonomyAdapter) ExtensionInfo(name string) *graphrag.TaxonomyExtension {
	return nil
}

// NodeTypeSource returns the source of a node type.
// TaxonomyAdapter only has core types, so it returns "core" for recognized types
// or "unknown" for unrecognized types.
func (a *TaxonomyAdapter) NodeTypeSource(nodeType string) string {
	if _, exists := a.nodeTypes[nodeType]; exists {
		return "core"
	}
	return "unknown"
}

// Verify TaxonomyAdapter implements graphrag.TaxonomyIntrospector
var _ graphrag.TaxonomyIntrospector = (*TaxonomyAdapter)(nil)
