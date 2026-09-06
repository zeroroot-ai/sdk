// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Zero Root AI

// Package graphrag provides Graph-based Retrieval-Augmented Generation (GraphRAG) capabilities
// for the Gibson AI Security Testing Framework.
//
// This package enables agents to emit structured knowledge — nodes and the
// relationships between them — into the World. Under the ECS-brain model
// (ADR-0001) agents are emit-only: they construct and submit nodes/relationships
// and never query or traverse the graph back; the brain is the sole reader and
// the relevant world state is ambiently projected to agents. The recall/query/
// traversal surface (Query, Result, TraversalOptions, TraversalResult,
// AttackPattern, FindingNode, AttackChain, IntelligenceQueries) was removed in
// the ECS-brain cutover (sdk#341).
//
// # Core Capabilities
//
// GraphRAG provides several key emit capabilities:
//   - Construction of typed, taxonomy-validated graph nodes
//   - Relationship modeling between nodes, including bidirectional links
//   - Batch operations for efficient bulk storage
//   - Mission-scoped knowledge isolation
//
// # Core Types
//
// The package provides the following types:
//
//   - GraphNode: Represents a node in the knowledge graph with properties and content
//   - Relationship: Represents connections between nodes with optional properties
//   - Batch: Collection of nodes and relationships for bulk operations
//
// # Creating and Storing Nodes
//
// Create nodes using the fluent GraphNode builder:
//
//	// Create a simple node
//	node := graphrag.NewGraphNode("finding").
//	    WithID("finding-123").
//	    WithContent("SQL injection vulnerability in login endpoint").
//	    WithProperty("severity", "high").
//	    WithProperty("confidence", 0.95)
//
//	// Validate before use
//	if err := node.Validate(); err != nil {
//	    log.Fatal(err)
//	}
//
//	// Create a technique node
//	technique := graphrag.NewGraphNode("technique").
//	    WithID("T1190").
//	    WithContent("Exploit Public-Facing Application").
//	    WithProperty("tactic", "Initial Access").
//	    WithProperty("platform", "Web")
//
// The MissionID and AgentName fields are auto-populated by the Gibson harness.
//
// # Relationship Management
//
// Create and manage graph relationships:
//
//	// Simple unidirectional relationship
//	rel := graphrag.NewRelationship(
//	    "finding-123",
//	    "technique-T1190",
//	    "USES_TECHNIQUE",
//	).WithProperty("confidence", 0.95)
//
//	// Bidirectional relationship
//	rel := graphrag.NewRelationship(
//	    "finding-123",
//	    "finding-456",
//	    "SIMILAR_TO",
//	).WithProperty("similarity", 0.87).
//	  WithBidirectional(true)
//
//	// Validate before use
//	if err := rel.Validate(); err != nil {
//	    log.Fatal(err)
//	}
//
// ## Relationship Types
//
// Canonical relationship types defined in the taxonomy:
//
// Asset Hierarchy:
//   - HAS_SUBDOMAIN: Domain has subdomain
//   - RESOLVES_TO: Domain/subdomain resolves to host
//   - HAS_PORT: Host has open port
//   - RUNS_SERVICE: Port runs service
//   - HAS_ENDPOINT: Service has endpoint
//   - USES_TECHNOLOGY: Component uses technology
//   - SERVES_CERTIFICATE: Service serves certificate
//   - HOSTS: Provider hosts resource
//
// Finding Links:
//   - AFFECTS: Finding affects asset
//   - USES_TECHNIQUE: Finding uses attack technique
//   - HAS_EVIDENCE: Finding has evidence
//   - LEADS_TO: Finding leads to another finding (attack chains)
//   - MITIGATES: Mitigation addresses finding
//   - SIMILAR_TO: Findings are similar (bidirectional)
//   - EXPLOITS: Finding exploits vulnerability
//
// Execution Context:
//   - PART_OF: Component is part of parent
//   - EXECUTED_BY: Entity executed by agent/tool
//   - DISCOVERED: Asset discovered by execution
//   - PRODUCED: Execution produced artifact
//
// Use generated constants for type safety:
//
//	rel := graphrag.NewRelationshipWithValidation(
//	    fromID,
//	    toID,
//	    graphrag.RelTypeHasSubdomain,  // type-safe constant
//	)
//
// See constants_generated.go for the complete list with documentation.
//
// # Batch Operations
//
// Use Batch for efficient bulk storage of nodes and relationships:
//
//	batch := graphrag.NewBatch().
//	    AddNode(*finding1).
//	    AddNode(*finding2).
//	    AddNode(*technique).
//	    AddRelationship(*rel1).
//	    AddRelationship(*rel2)
//
//	// Submit batch to GraphRAG storage
//	// (actual storage operations are handled by the Gibson framework)
//
// Batch operations are more efficient than individual operations when
// creating multiple nodes and relationships simultaneously.
//
// # Taxonomy System
//
// GraphRAG uses a YAML-driven taxonomy system for node and relationship types.
// The taxonomy provides type safety, validation, and comprehensive documentation
// for all graph types used across the Gibson platform.
//
// ## Using Generated Constants
//
// Instead of string literals, use the generated constants from constants_generated.go:
//
//	// Instead of this:
//	node := graphrag.NewGraphNode("domain")  // string literal
//
//	// Use this:
//	node := graphrag.NewGraphNode(graphrag.NodeTypeDomain)  // type-safe constant
//
// Generated constants are available for:
//   - All canonical node types (NodeType* constants)
//   - All canonical relationship types (RelType* constants)
//   - MITRE ATT&CK techniques (Technique* constants)
//   - Arcanum prompt injection techniques (ArcanumTechnique* constants)
//
// ## Taxonomy Validation
//
// Nodes and relationships are automatically validated against the taxonomy:
//
//	// Create node with canonical type - no warning
//	node := graphrag.NewNodeWithValidation("domain")
//
//	// Create node with custom type - logs warning but still works
//	node := graphrag.NewNodeWithValidation("my_custom_type")
//	// WARNING: Node type 'my_custom_type' is not in the canonical taxonomy
//
//	// Check if a type is canonical
//	if graphrag.ValidateAndWarnNodeType("domain") {
//	    // Type is canonical
//	}
//
// The validation helpers encourage use of canonical types while allowing
// flexibility for agent-specific custom types.
//
// ## Node Types
//
// Canonical node types defined in the taxonomy:
//
// Asset Discovery:
//   - domain: Internet domain names
//   - subdomain: Subdomains of parent domains
//   - host: IP addresses and hostnames
//   - port: TCP/UDP ports
//   - service: Running services
//   - endpoint: HTTP endpoints and APIs
//   - api: API specifications
//   - technology: Detected technologies
//   - cloud_asset: Cloud resources
//   - certificate: TLS/SSL certificates
//
// Security Findings:
//   - finding: Security vulnerabilities and findings
//   - evidence: Supporting evidence for findings
//   - mitigation: Recommended security controls
//
// Execution Context:
//   - mission: Testing missions
//   - agent_run: Agent execution instances
//   - tool_execution: Tool invocations
//   - llm_call: LLM API calls
//
// Attack Techniques:
//   - technique: MITRE ATT&CK or Arcanum techniques
//   - tactic: MITRE ATT&CK tactics
//
// See constants_generated.go for the complete list with documentation.
//
// # Hybrid Scoring
//
// GraphRAG combines two scoring dimensions:
//
//  1. Vector Score: Semantic similarity via embeddings
//     - Measures conceptual relevance
//     - Uses cosine similarity on embedding vectors
//
//  2. Graph Score: Structural relevance via relationships
//     - Measures connectedness and relationship strength
//     - Incorporates relationship types and properties
//     - Uses multi-hop propagation with decay
//
// Control the balance with WithWeights(vector, graph):
//
//	// Emphasize semantic similarity
//	query.WithWeights(0.8, 0.2)
//
//	// Emphasize graph structure
//	query.WithWeights(0.3, 0.7)
//
//	// Balanced (default)
//	query.WithWeights(0.6, 0.4)
//
// # Error Handling
//
// GraphRAG operations return specific sentinel errors:
//
//	result, err := client.Query(ctx, query)
//	if errors.Is(err, graphrag.ErrGraphRAGNotEnabled) {
//	    // GraphRAG is not configured in the framework
//	}
//	if errors.Is(err, graphrag.ErrNodeNotFound) {
//	    // Referenced node does not exist
//	}
//	if errors.Is(err, graphrag.ErrInvalidQuery) {
//	    // Query validation failed
//	}
//	if errors.Is(err, graphrag.ErrEmbeddingFailed) {
//	    // Embedding generation failed
//	}
//	if errors.Is(err, graphrag.ErrQueryTimeout) {
//	    // Query exceeded timeout
//	}
//	if errors.Is(err, graphrag.ErrStorageFailed) {
//	    // Storage operation failed
//	}
//	if errors.Is(err, graphrag.ErrRelationshipFailed) {
//	    // Relationship operation failed
//	}
//
// Always use errors.Is() for error checking to properly handle wrapped errors.
//
// # Mission Isolation
//
// GraphRAG supports mission-scoped knowledge graphs:
//
//	// Store finding in mission context
//	node := graphrag.NewGraphNode("finding").
//	    WithID("finding-123").
//	    WithContent("SQL injection vulnerability found")
//	// MissionID is auto-populated by the Gibson harness
//
// This ensures knowledge isolation between different testing missions.
//
// # Common Patterns for Security Agents
//
// ## Storing Attack Data
//
// Store attack attempts and findings:
//
//	// Create finding node
//	finding := graphrag.NewGraphNode("finding").
//	    WithContent("SQL injection in /api/login parameter 'username'").
//	    WithProperty("severity", "critical").
//	    WithProperty("cvss_score", 9.8).
//	    WithProperty("endpoint", "/api/login").
//	    WithProperty("parameter", "username")
//
//	// Create technique node
//	technique := graphrag.NewGraphNode("technique").
//	    WithID("T1190").
//	    WithContent("Exploit Public-Facing Application").
//	    WithProperty("tactic", "Initial Access")
//
//	// Link finding to technique
//	rel := graphrag.NewRelationship(
//	    finding.ID,
//	    "T1190",
//	    "USES_TECHNIQUE",
//	).WithProperty("confidence", 0.95)
//
// ## Building Attack Chains
//
// Create attack chain from multiple findings:
//
//	// Create findings
//	recon := graphrag.NewGraphNode("finding").
//	    WithContent("Port scan detected open SSH port")
//
//	bruteforce := graphrag.NewGraphNode("finding").
//	    WithContent("SSH bruteforce successful with weak credentials")
//
//	privesc := graphrag.NewGraphNode("finding").
//	    WithContent("Privilege escalation via sudo misconfiguration")
//
//	// Link findings in sequence
//	rel1 := graphrag.NewRelationship(recon.ID, bruteforce.ID, "LEADS_TO").
//	    WithProperty("sequence", 1)
//
//	rel2 := graphrag.NewRelationship(bruteforce.ID, privesc.ID, "LEADS_TO").
//	    WithProperty("sequence", 2)
//
// # Integration with Gibson
//
// GraphRAG integrates seamlessly with Gibson agents:
//
//	func (a *MyAgent) Execute(ctx context.Context, req sdk.ExecuteRequest) error {
//	    // Store finding in graph
//	    finding := graphrag.NewGraphNode("finding").
//	        WithContent("XSS vulnerability in search parameter").
//	        WithProperty("severity", "medium").
//	        WithProperty("url", req.Target.URL)
//
//	    if err := finding.Validate(); err != nil {
//	        return fmt.Errorf("invalid node: %w", err)
//	    }
//
//	    // Emit the node; the brain ingests it and projects relevant
//	    // world state back to the agent (no querying from the agent side).
//
//	    return nil
//	}
//
// # Performance Considerations
//
// GraphRAG operations are optimized for different query patterns:
//
//   - Vector search: O(n) with approximate nearest neighbor index
//   - Graph traversal: O(d * b^k) where d=degree, b=branching, k=MaxHops
//   - Hybrid scoring: Computed in parallel with result streaming
//
// Best practices:
//   - Use appropriate TopK values (typically 5-20)
//   - Limit MaxHops for large graphs (typically 2-3)
//   - Set MinScore to filter low-quality matches
//   - Use NodeTypes filtering to reduce search space
//   - Cache frequently accessed embeddings
//   - Use Batch for bulk operations
//   - Validate nodes, queries, and relationships before use
//   - Handle errors with errors.Is() for sentinel errors
//
// # Concurrency Safety
//
// All GraphRAG operations are safe for concurrent use by multiple goroutines.
// The underlying storage and indexing layers handle concurrent reads and writes
// with appropriate locking and isolation guarantees.
package graphrag
