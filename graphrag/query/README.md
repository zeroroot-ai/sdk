# Query Package

The `query` package provides utilities for building type-safe Neo4j Cypher queries and mapping results to protocol buffer types.

## Overview

This package consists of two main components:

1. **Query Builder** (`builder.go`, `types.go`) - Type-safe Cypher query construction
2. **Result Mapper** (`mapper.go`) - Neo4j result to proto message conversion

## Result Mapping

### Core Functions

#### `MapRowToProto`

Maps a single Neo4j result row (map[string]any) to a protocol buffer message using reflection.

```go
row := map[string]any{
    "id":       "host-1",
    "ip":       "192.168.1.1",
    "hostname": "server1.local",
    "os":       "Linux",
}

host := &taxonomypb.Host{}
err := query.MapRowToProto(row, host)
```

**Features:**
- Field mapping by proto field name (not JSON name)
- Automatic type conversion for Neo4j quirks:
  - `int64 -> int32` (with overflow checking)
  - `float64 -> float32/double`
  - Handles nil values for optional fields
- Unknown fields in the row are ignored
- Missing fields are gracefully skipped

#### `MapRowsToProtos`

Maps multiple Neo4j result rows to a slice of proto messages.

```go
rows := []map[string]any{
    {"id": "port-1", "number": int64(80), "protocol": "tcp"},
    {"id": "port-2", "number": int64(443), "protocol": "tcp"},
}

ports, err := query.MapRowsToProtos(rows, func() *taxonomypb.Port {
    return &taxonomypb.Port{}
})
```

### Helper Functions

#### `MapFieldsFromProto`

Extracts specific fields from a proto message into a map. Useful for building Neo4j query parameters.

```go
host := &taxonomypb.Host{
    Id: "host-1",
    Ip: proto.String("192.168.1.1"),
    Hostname: proto.String("server1"),
}

// Extract specific fields
params := query.MapFieldsFromProto(host, []string{"id", "ip"})
// params = map[string]any{"id": "host-1", "ip": "192.168.1.1"}

// Extract all fields (pass nil or empty slice)
allParams := query.MapFieldsFromProto(host, nil)
```

#### `ExtractIDFields`

Extracts only ID-related fields from a proto message. ID fields include:
- Field named "id"
- Fields ending in "_id"
- Fields starting with "parent_"

```go
service := &taxonomypb.Service{
    Id:           "service-1",
    Name:         "http",
    ParentPortId: "port-1",
}

ids := query.ExtractIDFields(service)
// ids = map[string]any{"id": "service-1", "parent_port_id": "port-1"}
```

#### `ValidateRequiredFields`

Checks that all required (non-optional) fields in a proto message are set.

```go
host := &taxonomypb.Host{} // Empty
err := query.ValidateRequiredFields(host)
// err: "missing required fields: id"
```

## Neo4j Type Handling

The mapper handles Neo4j's type quirks automatically:

### Integer Conversion

Neo4j returns all integers as `int64`. The mapper converts to the appropriate proto type:

```go
row := map[string]any{
    "number": int64(80), // Neo4j int64
}
port := &taxonomypb.Port{}
query.MapRowToProto(row, port)
// port.Number is int32(80)
```

Overflow is detected and returns an error:

```go
row := map[string]any{
    "number": int64(3000000000), // > int32 max
}
// Error: "int64 value 3000000000 overflows int32"
```

### Float Conversion

Neo4j returns all floats as `float64`. The mapper handles conversion to `float32` or `double` proto types:

```go
row := map[string]any{
    "confidence": float64(0.95), // Neo4j float64
}
finding := &taxonomypb.Finding{}
query.MapRowToProto(row, finding)
// finding.Confidence is correctly set
```

### Optional Fields

Nil values in the row result in unset optional fields:

```go
row := map[string]any{
    "id":       "host-1",
    "ip":       "192.168.1.1",
    "hostname": nil, // Nil value
}
host := &taxonomypb.Host{}
query.MapRowToProto(row, host)
// host.Id = "host-1"
// host.Ip = "192.168.1.1"
// host.Hostname = nil (unset)
```

## Complete Example

```go
package main

import (
    "context"
    "fmt"

    "github.com/zeroroot-ai/sdk/api/gen/taxonomypb"
    "github.com/zeroroot-ai/sdk/graphrag/query"
)

func main() {
    // Build a Cypher query
    cypher := query.BuildMatch("h", "host") + " " +
        query.BuildWhere("h", []query.Predicate{
            {Field: "ip", Op: query.Eq, Value: "192.168.1.1"},
        }) + " " +
        query.BuildReturn("h", nil)

    // Execute query (assuming you have a Neo4j client)
    rows, err := client.Query(ctx, cypher, nil)
    if err != nil {
        panic(err)
    }

    // Map results to proto messages
    hosts, err := query.MapRowsToProtos(rows, func() *taxonomypb.Host {
        return &taxonomypb.Host{}
    })
    if err != nil {
        panic(err)
    }

    // Use the results
    for _, host := range hosts {
        fmt.Printf("Host: %s (%s)\n", host.GetHostname(), host.GetIp())
    }
}
```

## Error Handling

The mapper provides clear error messages for common issues:

- **Type mismatch**: `"expected string, got int"`
- **Overflow**: `"int64 value 3000000000 overflows int32"`
- **Invalid enum**: `"unknown enum value 'invalid' for enum taxonomy.CoreNodeType"`
- **Nil input**: `"target message cannot be nil"`

## Testing

The package includes comprehensive tests covering:
- All proto types from taxonomy and graphrag
- Type conversions (int64->int32, float64->float32, etc.)
- Edge cases (nil values, missing fields, overflow)
- Batch operations
- Helper functions

Run tests:

```bash
go test ./graphrag/query/...
```

## Performance

The mapper uses protoreflect for flexibility while maintaining good performance:

- Field lookups are O(1) via proto descriptor
- Batch operations allocate slices upfront
- No unnecessary copying of data
- Type conversions are inlined

For high-performance scenarios, consider:
1. Reusing proto messages when possible
2. Using `MapRowsToProtos` for batch operations
3. Pre-validating data before mapping if validation is expensive
