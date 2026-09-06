// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Zero Root AI

package graphrag

import (
	"errors"
	"fmt"
	"sort"
	"strings"
	"sync"
)

// Sentinel errors for registry operations.
var (
	// ErrNodeTypeNotRegistered indicates that the requested node type is not in the registry.
	// This error is returned when attempting to validate properties for an unknown node type.
	//
	// Example:
	//	props, err := registry.GetIdentifyingProperties("unknown_type")
	//	if errors.Is(err, graphrag.ErrNodeTypeNotRegistered) {
	//	    log.Errorf("Node type not found in registry: %v", err)
	//	}
	ErrNodeTypeNotRegistered = errors.New("node type not registered")

	// ErrMissingIdentifyingProperties indicates that one or more required identifying properties
	// are missing from the node's property map. This error is returned during property validation.
	//
	// Example:
	//	missing, err := registry.ValidateProperties("host", properties)
	//	if errors.Is(err, graphrag.ErrMissingIdentifyingProperties) {
	//	    log.Errorf("Missing properties: %v", missing)
	//	}
	ErrMissingIdentifyingProperties = errors.New("missing identifying properties")
)

// NodeTypeRegistry defines the interface for managing node type identifying properties.
// The registry maps each canonical node type to its identifying properties - the properties
// that uniquely identify a node within the knowledge graph and determine deterministic ID generation.
//
// Identifying properties are the minimum set of properties that:
//   - Uniquely identify a node of that type in the graph
//   - Must be present before creating the node
//   - Are used to generate deterministic node IDs (when ID generation is implemented)
//   - Create a natural key for deduplication
//
// For example:
//   - A "host" node is uniquely identified by its "ip" property
//   - A "port" node is uniquely identified by "host_id", "port_number", and "protocol"
//   - A "finding" node is uniquely identified by "mission_id" and "fingerprint"
//
// This interface is used by:
//   - ID generation logic to create deterministic IDs from identifying properties
//   - Node creation validation to ensure all required properties are present
//   - Deduplication logic to prevent duplicate nodes
//   - Documentation and code generation tools
type NodeTypeRegistry interface {
	// GetIdentifyingProperties returns the property names that uniquely identify a node of the given type.
	// These properties form the natural key for the node type and are used for deterministic ID generation.
	//
	// Returns ErrNodeTypeNotRegistered if the node type is not in the registry.
	//
	// Example:
	//	props, err := registry.GetIdentifyingProperties("port")
	//	// props = ["host_id", "port_number", "protocol"]
	GetIdentifyingProperties(nodeType string) ([]string, error)

	// IsRegistered checks if a node type exists in the registry.
	// Returns true if the node type is registered, false otherwise.
	//
	// Example:
	//	if registry.IsRegistered("host") {
	//	    // Node type is valid and registered
	//	}
	IsRegistered(nodeType string) bool

	// ValidateProperties checks if all identifying properties are present in the given property map.
	// Returns the list of missing property names if validation fails.
	// Returns ErrNodeTypeNotRegistered if the node type is not registered.
	// Returns ErrMissingIdentifyingProperties if any required properties are missing.
	//
	// Example:
	//	properties := map[string]any{"ip": "10.0.0.1"}
	//	missing, err := registry.ValidateProperties("host", properties)
	//	if err != nil {
	//	    if errors.Is(err, graphrag.ErrMissingIdentifyingProperties) {
	//	        log.Errorf("Missing required properties: %v", missing)
	//	    }
	//	}
	ValidateProperties(nodeType string, properties map[string]any) ([]string, error)

	// AllNodeTypes returns a sorted list of all registered node type names.
	// This is useful for documentation, validation, and debugging.
	//
	// Example:
	//	types := registry.AllNodeTypes()
	//	// types = ["agent_run", "api", "certificate", ...]
	AllNodeTypes() []string
}

// DefaultNodeTypeRegistry is the default implementation of NodeTypeRegistry.
// It uses an in-memory map to store the identifying properties for each canonical node type
// from the GraphRAG taxonomy (constants_generated.go).
//
// This implementation is thread-safe and can be used concurrently.
type DefaultNodeTypeRegistry struct {
	mu       sync.RWMutex
	registry map[string][]string
}

// NewDefaultNodeTypeRegistry creates and initializes a new DefaultNodeTypeRegistry
// with all canonical node types from the GraphRAG taxonomy.
//
// The registry is pre-populated with identifying properties for each node type:
//
// Asset Node Types:
//   - host: [ip] - Identified by IP address
//   - port: [host_id, number, protocol] - Identified by host, port number, and protocol
//   - service: [port_id, name] - Identified by port and service name
//   - endpoint: [service_id, url, method] - Identified by service, URL path, and HTTP method
//   - domain: [name] - Identified by domain name
//   - subdomain: [parent_domain, name] - Identified by parent domain and subdomain name
//   - api: [base_url] - Identified by base URL
//   - technology: [name, version] - Identified by name and version
//   - certificate: [fingerprint] - Identified by certificate fingerprint
//   - cloud_asset: [provider, resource_id] - Identified by cloud provider and resource ID
//
// Finding Node Types:
//   - finding: [mission_id, fingerprint] - Identified by mission and unique fingerprint
//   - evidence: [finding_id, type, fingerprint] - Identified by finding, evidence type, and fingerprint
//   - mitigation: [finding_id, title] - Identified by finding and mitigation title
//
// Execution Node Types:
//   - mission: [name, timestamp] - Identified by mission name and start timestamp
//   - agent_run: [mission_id, agent_name, run_number] - Identified by mission, agent, and run sequence
//   - tool_execution: [agent_run_id, tool_name, sequence] - Identified by agent run, tool, and execution sequence
//   - llm_call: [agent_run_id, sequence] - Identified by agent run and call sequence
//
// Attack Node Types:
//   - technique: [id] - Identified by technique ID (e.g., T1003, ARC-E001)
//   - tactic: [id] - Identified by tactic ID
//
// Intelligence Node Types:
//   - intelligence: [mission_id, title, timestamp] - Identified by mission, title, and generation timestamp
//
// Example:
//
//	registry := graphrag.NewDefaultNodeTypeRegistry()
//	props, err := registry.GetIdentifyingProperties("port")
//	// props = ["host_id", "number", "protocol"]
func NewDefaultNodeTypeRegistry() *DefaultNodeTypeRegistry {
	r := &DefaultNodeTypeRegistry{
		registry: make(map[string][]string),
	}

	// Asset Node Types
	r.register(NodeTypeHost, []string{"ip"})
	r.register(NodeTypePort, []string{"host_id", "number", "protocol"})
	r.register(NodeTypeService, []string{"port_id", "name"})
	r.register(NodeTypeEndpoint, []string{"service_id", "url", "method"})
	r.register(NodeTypeDomain, []string{"name"})
	r.register(NodeTypeSubdomain, []string{"parent_domain", "name"})
	r.register(NodeTypeTechnology, []string{"name", "version"})
	r.register(NodeTypeCertificate, []string{"fingerprint"})

	// Finding Node Types
	r.register(NodeTypeFinding, []string{"mission_id", "fingerprint"})
	r.register(NodeTypeEvidence, []string{"finding_id", "type", "fingerprint"})

	// Execution Node Types
	r.register(NodeTypeMission, []string{"name", "timestamp"})
	r.register(NodeTypeMissionRun, []string{"mission_id", "run_number"})
	r.register(NodeTypeAgentRun, []string{"mission_run_id", "agent_name"})
	r.register(NodeTypeToolExecution, []string{"agent_run_id", "tool_name", "sequence"})
	r.register(NodeTypeLlmCall, []string{"agent_run_id", "sequence"})

	// Attack Node Types
	r.register(NodeTypeTechnique, []string{"technique_id"})

	return r
}

// register is an internal helper to add a node type to the registry.
// This method is not exported as the registry is intended to be immutable after initialization.
func (r *DefaultNodeTypeRegistry) register(nodeType string, properties []string) {
	r.registry[nodeType] = properties
}

// GetIdentifyingProperties returns the property names that uniquely identify a node of the given type.
// Thread-safe for concurrent access.
func (r *DefaultNodeTypeRegistry) GetIdentifyingProperties(nodeType string) ([]string, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	props, ok := r.registry[nodeType]
	if !ok {
		return nil, fmt.Errorf("%w: %s", ErrNodeTypeNotRegistered, nodeType)
	}

	// Return a copy to prevent external modification
	result := make([]string, len(props))
	copy(result, props)
	return result, nil
}

// IsRegistered checks if a node type exists in the registry.
// Thread-safe for concurrent access.
func (r *DefaultNodeTypeRegistry) IsRegistered(nodeType string) bool {
	r.mu.RLock()
	defer r.mu.RUnlock()

	_, ok := r.registry[nodeType]
	return ok
}

// ValidateProperties checks if all identifying properties are present in the given property map.
// Returns the list of missing property names and an error if validation fails.
// Thread-safe for concurrent access.
func (r *DefaultNodeTypeRegistry) ValidateProperties(nodeType string, properties map[string]any) ([]string, error) {
	identifyingProps, err := r.GetIdentifyingProperties(nodeType)
	if err != nil {
		return nil, err
	}

	var missing []string
	for _, prop := range identifyingProps {
		if val, ok := properties[prop]; !ok || val == nil || (isString(val) && strings.TrimSpace(val.(string)) == "") {
			missing = append(missing, prop)
		}
	}

	if len(missing) > 0 {
		return missing, fmt.Errorf("%w for node type '%s': %v", ErrMissingIdentifyingProperties, nodeType, missing)
	}

	return nil, nil
}

// AllNodeTypes returns a sorted list of all registered node type names.
// Thread-safe for concurrent access.
func (r *DefaultNodeTypeRegistry) AllNodeTypes() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()

	types := make([]string, 0, len(r.registry))
	for nodeType := range r.registry {
		types = append(types, nodeType)
	}

	sort.Strings(types)
	return types
}

// isString checks if a value is a string type.
func isString(val any) bool {
	_, ok := val.(string)
	return ok
}

// Global registry instance for package-level access.
// This is initialized with the default registry but can be replaced for testing.
var (
	globalRegistry     NodeTypeRegistry
	globalRegistryOnce sync.Once
	globalRegistryMu   sync.RWMutex
)

// Registry returns the global NodeTypeRegistry instance.
// The registry is lazily initialized on first access using the default implementation.
// This function is thread-safe.
//
// Example:
//
//	registry := graphrag.Registry()
//	props, err := registry.GetIdentifyingProperties("host")
func Registry() NodeTypeRegistry {
	globalRegistryOnce.Do(func() {
		globalRegistry = NewDefaultNodeTypeRegistry()
	})

	globalRegistryMu.RLock()
	defer globalRegistryMu.RUnlock()
	return globalRegistry
}

// SetRegistry sets the global NodeTypeRegistry instance.
// This should only be used for testing or when a custom registry implementation is needed.
// This function is thread-safe but should be called before any calls to Registry().
//
// Example (testing):
//
//	mockRegistry := &MockNodeTypeRegistry{}
//	graphrag.SetRegistry(mockRegistry)
//	defer graphrag.SetRegistry(graphrag.NewDefaultNodeTypeRegistry())
func SetRegistry(registry NodeTypeRegistry) {
	globalRegistryMu.Lock()
	defer globalRegistryMu.Unlock()
	globalRegistry = registry
}
