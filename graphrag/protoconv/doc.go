// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Zero Root AI

// Package protoconv provides utilities for converting protocol buffer messages
// to map representations for use with Gibson's GraphRAG system.
//
// This package is a core component of the proto-first taxonomy architecture,
// enabling direct use of proto types in the knowledge graph without requiring
// domain wrapper types. It uses protoreflect for efficient, type-safe conversion.
//
// # Core Functions
//
// ToProperties converts a proto message to a map[string]any containing all
// user-facing properties. Framework-managed fields (id, timestamps, scoping)
// are automatically excluded.
//
// IdentifyingProperties extracts the subset of properties that uniquely identify
// a node of a given type. For example, hosts are identified by IP address, while
// ports are identified by number and protocol.
//
// # Field Handling
//
// The converter handles all standard proto field types:
//   - Scalars: string, int32, int64, uint32, uint64, float32, float64, bool, bytes
//   - Enums: converted to string representation
//   - Optional fields: only included if set (non-zero)
//
// # Framework Fields
//
// The following fields are automatically excluded from property maps:
//   - id, parent_id, parent_type, parent_relationship
//   - parent_*_id (e.g., parent_host_id, parent_port_id)
//   - mission_id, mission_run_id, agent_run_id
//   - discovered_by, discovered_at
//   - created_at, updated_at
//
// These fields are managed by the Gibson framework and should not be set by users.
//
// # Example Usage
//
//	import (
//		taxonomypb "github.com/zeroroot-ai/sdk/api/gen/taxonomy/v1"
//		"github.com/zeroroot-ai/sdk/graphrag/protoconv"
//	)
//
//	// Convert a host proto to properties
//	ip := "192.168.1.1"
//	hostname := "server.local"
//	host := &taxonomypb.Host{
//		Id:       "host-123",
//		Ip:       &ip,
//		Hostname: &hostname,
//	}
//
//	// Get all properties (excludes id)
//	props, err := protoconv.ToProperties(host)
//	// props = {"ip": "192.168.1.1", "hostname": "server.local"}
//
//	// Get identifying properties for host type
//	idProps, err := protoconv.IdentifyingProperties("host", host)
//	// idProps = {"ip": "192.168.1.1"}
//
// # Identifying Properties by Type
//
// Each node type has a defined set of identifying properties:
//
//	host:        ip
//	port:        number, protocol
//	service:     name
//	endpoint:    url, method
//	domain:      name
//	subdomain:   name
//	technology:  name, version
//	certificate: fingerprint_sha256
//	finding:     title
//	mission:     name, target
//
// These are enforced by the taxonomy system to ensure consistent node identity.
package protoconv
