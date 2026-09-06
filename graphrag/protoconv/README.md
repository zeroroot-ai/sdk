# protoconv - Protocol Buffer to Map Converter

The `protoconv` package provides utilities for converting protocol buffer messages to map representations for use in Gibson's GraphRAG knowledge graph system.

## Overview

This package is a core component of Gibson's proto-first taxonomy architecture. It enables direct use of proto types in the knowledge graph without requiring domain wrapper types, using Go's `protoreflect` for efficient, type-safe conversion.

## Key Features

- **Direct Proto Conversion**: Convert any proto message to `map[string]any`
- **Automatic Field Filtering**: Excludes framework-managed fields (IDs, timestamps, etc.)
- **Type Safety**: Uses protoreflect for compile-time type checking
- **Zero-Value Handling**: Only includes fields that are actually set
- **Identifying Properties**: Extract the minimal set of fields that uniquely identify a node

## Installation

```go
import "github.com/zeroroot-ai/sdk/graphrag/protoconv"
```

## Usage

### Converting All Properties

```go
import (
    "github.com/zeroroot-ai/sdk/api/gen/taxonomypb"
    "github.com/zeroroot-ai/sdk/graphrag/protoconv"
)

ip := "192.168.1.1"
hostname := "server.local"
host := &taxonomypb.Host{
    Id:       "host-123",
    Ip:       &ip,
    Hostname: &hostname,
}

// Convert to properties (excludes 'id' and other framework fields)
props, err := protoconv.ToProperties(host)
// props = {"ip": "192.168.1.1", "hostname": "server.local"}
```

### Extracting Identifying Properties

```go
// Ports are identified by number + protocol
port := &taxonomypb.Port{
    Id:           "port-123",
    Number:       443,
    Protocol:     "tcp",
    State:        strPtr("open"),
    ParentHostId: "host-123",
}

idProps, err := protoconv.IdentifyingProperties("port", port)
// idProps = {"number": 443, "protocol": "tcp"}
```

## Supported Field Types

The converter handles all standard proto field types:

- **Scalars**: `string`, `int32`, `int64`, `uint32`, `uint64`, `float32`, `float64`, `bool`, `bytes`
- **Enums**: Converted to string representation
- **Optional fields**: Only included if set (non-zero)

## Framework Field Filtering

The following fields are automatically excluded from property maps as they are managed by the Gibson framework:

- `id`, `parent_id`, `parent_type`, `parent_relationship`
- `parent_*_id` (e.g., `parent_host_id`, `parent_port_id`)
- `mission_id`, `mission_run_id`, `agent_run_id`
- `discovered_by`, `discovered_at`
- `created_at`, `updated_at`

## Identifying Properties by Node Type

Each node type has a defined set of identifying properties:

| Node Type   | Identifying Properties   |
|-------------|--------------------------|
| host        | ip                       |
| port        | number, protocol         |
| service     | name                     |
| endpoint    | url, method              |
| domain      | name                     |
| subdomain   | name                     |
| technology  | name, version            |
| certificate | fingerprint_sha256       |
| finding     | title                    |
| mission     | name, target             |

## API Reference

### ToProperties

```go
func ToProperties(msg proto.Message) (map[string]any, error)
```

Converts a proto message to a map containing all user-facing properties. Framework-managed fields are automatically excluded.

**Parameters:**
- `msg`: The proto message to convert

**Returns:**
- `map[string]any`: Properties as key-value pairs
- `error`: Error if conversion fails

### IdentifyingProperties

```go
func IdentifyingProperties(nodeType string, msg proto.Message) (map[string]any, error)
```

Extracts the subset of properties that uniquely identify a node of the given type.

**Parameters:**
- `nodeType`: The type of node (e.g., "host", "port", "service")
- `msg`: The proto message to extract from

**Returns:**
- `map[string]any`: Identifying properties as key-value pairs
- `error`: Error if node type is unknown or required properties are missing

## Design Principles

1. **Proto-First**: Proto messages are the source of truth, no domain wrappers needed
2. **Reflection-Based**: Uses protoreflect for runtime introspection
3. **Framework-Aware**: Automatically handles framework-managed fields
4. **Type-Safe**: Leverages Go's type system and proto definitions
5. **Zero-Copy**: Minimal allocations for efficient operation

## Testing

```bash
# Run all tests
go test ./graphrag/protoconv/

# Run with race detection
go test -race ./graphrag/protoconv/

# Run with coverage
go test -cover ./graphrag/protoconv/

# Run examples
go test -v -run Example ./graphrag/protoconv/
```

## License

Part of the Gibson framework - see repository root for license details.
