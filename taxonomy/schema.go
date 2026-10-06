// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Zero Root AI

package taxonomy

import (
	"strings"
	"sync"
)

// CoreTaxonomySchema implements TaxonomySchema using the GraphRAG taxonomy introspector.
type CoreTaxonomySchema struct {

	// Cached lookups
	mu            sync.RWMutex
	categories    []string
	categorySet   map[string]bool
	severities    []string
	severitySet   map[string]bool
	nodeTypes     []string
	nodeTypeSet   map[string]bool
	relTypes      []string
	relTypeSet    map[string]bool
	nodeProps     map[string][]PropertyDef
	requiredProps map[string][]string
}

// Categories returns all valid finding categories.
func (s *CoreTaxonomySchema) Categories() []string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	result := make([]string, len(s.categories))
	copy(result, s.categories)
	return result
}

// HasCategory checks if a category exists.
func (s *CoreTaxonomySchema) HasCategory(category string) bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.categorySet[strings.ToLower(category)]
}

// Severities returns all valid severity levels.
func (s *CoreTaxonomySchema) Severities() []string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	result := make([]string, len(s.severities))
	copy(result, s.severities)
	return result
}

// HasSeverity checks if a severity is valid.
func (s *CoreTaxonomySchema) HasSeverity(severity string) bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.severitySet[strings.ToLower(severity)]
}

// NodeTypes returns all valid entity/node types.
func (s *CoreTaxonomySchema) NodeTypes() []string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	result := make([]string, len(s.nodeTypes))
	copy(result, s.nodeTypes)
	return result
}

// HasNodeType checks if a node type exists.
func (s *CoreTaxonomySchema) HasNodeType(nodeType string) bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.nodeTypeSet[strings.ToLower(nodeType)]
}

// RelationshipTypes returns all valid relationship types.
func (s *CoreTaxonomySchema) RelationshipTypes() []string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	result := make([]string, len(s.relTypes))
	copy(result, s.relTypes)
	return result
}

// HasRelationshipType checks if a relationship type exists.
func (s *CoreTaxonomySchema) HasRelationshipType(relType string) bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.relTypeSet[strings.ToUpper(relType)]
}

// NodeProperties returns property definitions for a node type.
func (s *CoreTaxonomySchema) NodeProperties(nodeType string) []PropertyDef {
	s.mu.RLock()
	defer s.mu.RUnlock()
	props := s.nodeProps[strings.ToLower(nodeType)]
	if props == nil {
		return nil
	}
	result := make([]PropertyDef, len(props))
	copy(result, props)
	return result
}

// RequiredProperties returns required properties for a node type.
func (s *CoreTaxonomySchema) RequiredProperties(nodeType string) []string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	required := s.requiredProps[strings.ToLower(nodeType)]
	if required == nil {
		return nil
	}
	result := make([]string, len(required))
	copy(result, required)
	return result
}

// Compile-time interface check
var _ TaxonomySchema = (*CoreTaxonomySchema)(nil)
