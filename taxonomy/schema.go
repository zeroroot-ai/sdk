// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Zero Root AI

package taxonomy

import (
	"strings"
	"sync"

	"github.com/zeroroot-ai/sdk/graphrag"
)

// CoreTaxonomySchema implements TaxonomySchema using the GraphRAG taxonomy introspector.
type CoreTaxonomySchema struct {
	introspector graphrag.TaxonomyIntrospector

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
	initialized   bool
}

// Standard severity levels
var standardSeverities = []string{
	"critical",
	"high",
	"medium",
	"low",
	"info",
	"informational",
}

// Standard finding categories
var standardCategories = []string{
	"injection",
	"authentication",
	"authorization",
	"cryptography",
	"configuration",
	"disclosure",
	"dos",
	"input_validation",
	"xss",
	"csrf",
	"ssrf",
	"xxe",
	"deserialization",
	"path_traversal",
	"command_injection",
	"sql_injection",
	"ldap_injection",
	"file_upload",
	"open_redirect",
	"cors",
	"session",
	"sensitive_data",
	"misconfiguration",
	"outdated_software",
	"default_credentials",
	"weak_password",
	"missing_security_headers",
	"tls_ssl",
	"certificate",
	"network",
	"api",
	"business_logic",
	"race_condition",
	"other",
}

// NewCoreTaxonomySchema creates a schema from the GraphRAG taxonomy introspector.
func NewCoreTaxonomySchema(introspector graphrag.TaxonomyIntrospector) *CoreTaxonomySchema {
	schema := &CoreTaxonomySchema{
		introspector:  introspector,
		categorySet:   make(map[string]bool),
		severitySet:   make(map[string]bool),
		nodeTypeSet:   make(map[string]bool),
		relTypeSet:    make(map[string]bool),
		nodeProps:     make(map[string][]PropertyDef),
		requiredProps: make(map[string][]string),
	}
	schema.initialize()
	return schema
}

// NewDefaultTaxonomySchema creates a schema with default taxonomy values.
func NewDefaultTaxonomySchema() *CoreTaxonomySchema {
	// Use SimpleTaxonomy from graphrag package
	introspector := graphrag.NewSimpleTaxonomy()
	return NewCoreTaxonomySchema(introspector)
}

// initialize loads all taxonomy data into cached maps.
func (s *CoreTaxonomySchema) initialize() {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.initialized {
		return
	}

	// Initialize categories
	s.categories = make([]string, len(standardCategories))
	copy(s.categories, standardCategories)
	for _, cat := range s.categories {
		s.categorySet[strings.ToLower(cat)] = true
	}

	// Initialize severities
	s.severities = make([]string, len(standardSeverities))
	copy(s.severities, standardSeverities)
	for _, sev := range s.severities {
		s.severitySet[strings.ToLower(sev)] = true
	}

	// Initialize node types from introspector
	if s.introspector != nil {
		s.nodeTypes = s.introspector.NodeTypes()
		for _, nt := range s.nodeTypes {
			s.nodeTypeSet[strings.ToLower(nt)] = true
		}

		// Initialize relationship types
		s.relTypes = s.introspector.RelationshipTypes()
		for _, rt := range s.relTypes {
			s.relTypeSet[strings.ToUpper(rt)] = true
		}

		// Initialize property definitions from node type info
		for _, nt := range s.nodeTypes {
			info := s.introspector.NodeTypeInfo(nt)
			if info != nil {
				var props []PropertyDef
				var required []string
				for _, p := range info.Properties {
					props = append(props, PropertyDef{
						Name:        p.Name,
						Type:        p.Type,
						Required:    p.Required,
						Description: p.Description,
						Format:      p.Format,
						Enum:        p.Enum,
					})
					if p.Required {
						required = append(required, p.Name)
					}
				}
				s.nodeProps[nt] = props
				s.requiredProps[nt] = required
			}
		}
	}

	// Add default property definitions for core types if not present
	s.ensureDefaultProperties()

	s.initialized = true
}

// ensureDefaultProperties adds standard property definitions for core types.
func (s *CoreTaxonomySchema) ensureDefaultProperties() {
	// Host properties - always set defaults if empty
	if props := s.nodeProps["host"]; len(props) == 0 {
		s.nodeProps["host"] = []PropertyDef{
			{Name: "ip", Type: "string", Description: "IP address"},
			{Name: "hostname", Type: "string", Description: "Hostname"},
			{Name: "os", Type: "string", Description: "Operating system"},
		}
	}

	// Port properties
	if props := s.nodeProps["port"]; len(props) == 0 {
		s.nodeProps["port"] = []PropertyDef{
			{Name: "number", Type: "int", Required: true, Description: "Port number"},
			{Name: "protocol", Type: "string", Description: "Protocol (tcp/udp)"},
			{Name: "state", Type: "string", Description: "Port state (open/closed/filtered)"},
		}
		s.requiredProps["port"] = []string{"number"}
	}

	// Service properties
	if props := s.nodeProps["service"]; len(props) == 0 {
		s.nodeProps["service"] = []PropertyDef{
			{Name: "name", Type: "string", Required: true, Description: "Service name"},
			{Name: "version", Type: "string", Description: "Service version"},
			{Name: "product", Type: "string", Description: "Product name"},
		}
		s.requiredProps["service"] = []string{"name"}
	}

	// Finding properties
	if props := s.nodeProps["finding"]; len(props) == 0 {
		s.nodeProps["finding"] = []PropertyDef{
			{Name: "title", Type: "string", Required: true, Description: "Finding title"},
			{Name: "description", Type: "string", Description: "Finding description"},
			{Name: "severity", Type: "string", Required: true, Description: "Severity level", Enum: standardSeverities},
			{Name: "category", Type: "string", Description: "Finding category"},
			{Name: "confidence", Type: "float", Description: "Confidence score (0-1)"},
			{Name: "cvss_score", Type: "float", Description: "CVSS score (0-10)"},
			{Name: "cwe_ids", Type: "[]string", Description: "CWE identifiers"},
			{Name: "cve_ids", Type: "[]string", Description: "CVE identifiers"},
		}
		s.requiredProps["finding"] = []string{"title", "severity"}
	}

	// Endpoint properties
	if props := s.nodeProps["endpoint"]; len(props) == 0 {
		s.nodeProps["endpoint"] = []PropertyDef{
			{Name: "url", Type: "string", Required: true, Description: "Endpoint URL"},
			{Name: "method", Type: "string", Description: "HTTP method"},
			{Name: "status_code", Type: "int", Description: "HTTP status code"},
		}
		s.requiredProps["endpoint"] = []string{"url"}
	}

	// Domain properties
	if props := s.nodeProps["domain"]; len(props) == 0 {
		s.nodeProps["domain"] = []PropertyDef{
			{Name: "name", Type: "string", Required: true, Description: "Domain name"},
			{Name: "registrar", Type: "string", Description: "Domain registrar"},
		}
		s.requiredProps["domain"] = []string{"name"}
	}
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
