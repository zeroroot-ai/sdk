// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Zero Root AI

// Package id provides deterministic ID generation for GraphRAG nodes.
//
// This package implements content-addressable IDs based on node type and identifying properties.
// IDs are stable across agent runs and missions, enabling reliable deduplication and relationship
// tracking in the knowledge graph.
//
// # Core Concepts
//
// Deterministic IDs are generated using:
//   - Node type from the GraphRAG taxonomy
//   - Identifying properties (defined in NodeTypeRegistry)
//   - SHA-256 hashing of canonical property representation
//   - Base64url encoding for compact, URL-safe IDs
//
// # ID Format
//
// IDs follow the format: {nodeType}:{base64url(sha256(canonical)[:12])}
//
// Example:
//
//	host:YlVwLX3qR0SyC7uV
//	port:8K7J6H5G4F3D2S1A
//	domain:X9Y8Z7W6V5U4T3S2
//
// The node type prefix makes IDs human-readable and self-documenting.
// The hash ensures uniqueness while maintaining determinism.
//
// # Canonical Representation
//
// Properties are normalized before hashing to ensure consistency:
//   - Strings: lowercase and trimmed
//   - Numbers: formatted with fixed precision
//   - Booleans: "true" or "false"
//   - Complex types: JSON serialization
//   - Property order: alphabetically sorted
//
// This guarantees that the same logical node always produces the same ID,
// regardless of how the properties are passed or stored.
//
// # Usage
//
// Basic usage with the default registry:
//
//	registry := graphrag.NewDefaultNodeTypeRegistry()
//	gen := id.NewGenerator(registry)
//
//	// Generate ID for a host node
//	hostID, err := gen.Generate("host", map[string]any{
//	    "ip": "10.0.0.1",
//	})
//
//	// Generate ID for a port node
//	portID, err := gen.Generate("port", map[string]any{
//	    "host_id":  hostID,
//	    "number":   443,
//	    "protocol": "tcp",
//	})
//
// The generator validates that all identifying properties are present
// and returns clear errors if validation fails.
//
// # Determinism Guarantees
//
// The generator guarantees:
//   - Same input always produces same output
//   - Different inputs produce different outputs (collision-resistant)
//   - Property order independence
//   - Case and whitespace normalization
//   - Type coercion consistency (int vs int64, etc.)
//
// These guarantees enable:
//   - Reliable deduplication across agent runs
//   - Stable relationship IDs across missions
//   - Consistent graph structure
//   - Efficient caching and lookup
//
// # Integration with GraphRAG
//
// This package integrates with the GraphRAG system:
//   - NodeTypeRegistry defines identifying properties per node type
//   - Generator creates IDs for nodes before storage
//   - Relationships reference nodes by deterministic IDs
//   - Storage layer uses IDs for deduplication
//
// See the graphrag package documentation for more details on the overall system.
package id
