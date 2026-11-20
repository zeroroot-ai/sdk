# protoresolver

Package `protoresolver` resolves Protocol Buffer message types at run time. A caller can use proto messages without compiled Go types. The package builds the types from a FileDescriptorSet that a tool sends in its metadata.

## Overview

The package has three parts:

1. **Dynamic Message Factory**: makes `dynamicpb.Message` instances from FileDescriptorSet definitions.
2. **LRU Cache**: keeps parsed FileDescriptorSets so that the resolver does not parse the same set again (about 1-5ms for each parse).
3. **Fallback Resolution**: looks for the type in the global proto registry first. If the type is not there, it uses the FileDescriptorSet from the metadata.

## Key Interfaces

### ProtoResolver

```go
type ProtoResolver interface {
    // Resolve an input message type
    ResolveInputType(ctx context.Context, typeName string, metadata map[string]string) (proto.Message, error)

    // Resolve an output message type
    ResolveOutputType(ctx context.Context, typeName string, metadata map[string]string) (proto.Message, error)

    // Resolve the type, then unmarshal JSON into it
    UnmarshalProtoJSON(ctx context.Context, typeName string, jsonData []byte, metadata map[string]string) (proto.Message, error)
}
```

`DefaultProtoResolver` is the implementation in this package.

### FileDescriptorCache

```go
type FileDescriptorCache interface {
    // Get returns the cached files for a tool, if present and not expired.
    Get(toolName string) (*protoregistry.Files, bool)

    // Put stores the files for a tool. At capacity, the least recently used entry goes.
    Put(toolName string, files *protoregistry.Files)
}
```

`NewFileDescriptorCache(maxEntries, ttl)` returns an LRU implementation. An entry leaves the cache when its TTL expires or when the cache evicts it.

## Usage

### Basic Setup

```go
import (
    "github.com/zeroroot-ai/sdk/protoresolver"
)

// Create a resolver with the default configuration
resolver := protoresolver.NewDefaultProtoResolver(protoresolver.DefaultConfig())

// Or set your own configuration
config := protoresolver.ProtoResolverConfig{
    CacheMaxEntries: 200,           // Max FileDescriptorSets to cache
    CacheTTL:        2 * time.Hour, // Cache entry lifetime
    LogFallbacks:    true,          // Log when the resolver uses the FileDescriptorSet
}
resolver := protoresolver.NewDefaultProtoResolver(config)
```

### Resolving Types

The resolver reads two metadata keys:

```go
// - "tool_name": the name of the tool that supplies the schema (the cache key)
// - "file_descriptor_set": the base64-encoded FileDescriptorSet
metadata := map[string]string{
    "tool_name":           "mytool-c",
    "file_descriptor_set": fdsBase64,
}

msg, err := resolver.ResolveInputType(ctx, "gibson.tools.mytool-c.MytoolCRequest", metadata)
if err != nil {
    log.Fatal(err)
}

// msg is a proto.Message from the global registry or a dynamicpb message
```

### Unmarshaling JSON

```go
jsonData := []byte(`{"target": "example.com", "ports": "80,443"}`)

msg, err := resolver.UnmarshalProtoJSON(
    ctx,
    "gibson.tools.mytool-c.MytoolCRequest",
    jsonData,
    metadata,
)
if err != nil {
    log.Fatal(err)
}
```

## Configuration Options

### ProtoResolverConfig

```go
type ProtoResolverConfig struct {
    // CacheMaxEntries: the maximum number of FileDescriptorSets to cache.
    // At this limit, the least recently used entry is evicted.
    // A value of 0 or less selects 100.
    CacheMaxEntries int

    // CacheTTL: how long a cached FileDescriptorSet stays valid.
    // After this time, the resolver parses the set from metadata again.
    // A value of 0 or less selects 1 hour.
    CacheTTL time.Duration

    // LogFallbacks: log when the resolver falls back from the global
    // registry to the FileDescriptorSet.
    LogFallbacks bool
}
```

### Default Configuration

```go
config := protoresolver.DefaultConfig()
// Returns:
// ProtoResolverConfig{
//     CacheMaxEntries: 100,
//     CacheTTL:        time.Hour,
//     LogFallbacks:    true,
// }
```

## Error Handling

### SchemaNotFoundError

The resolver returns this error when the type is not in the global registry and the metadata has no FileDescriptorSet:

```go
msg, err := resolver.ResolveInputType(ctx, typeName, metadata)
if err != nil {
    var schemaErr *protoresolver.SchemaNotFoundError
    if errors.As(err, &schemaErr) {
        log.Printf("Schema not found for tool: %s", schemaErr.ToolName)
        log.Printf("While resolving type: %s", schemaErr.TypeName)
    }
}
```

### Sentinel Errors

`SchemaNotFoundError` wraps `ErrNoSchemaAvailable`:

```go
if errors.Is(err, protoresolver.ErrNoSchemaAvailable) {
    // Neither the global registry nor the metadata has the schema
}
```

Other failures (a bad FileDescriptorSet, an unknown type name, invalid JSON) come back as wrapped errors.

## Resolution Strategy

1. **Global Registry Lookup**: the resolver looks for the type in `protoregistry.GlobalTypes`. If it finds the type, it returns at once.
2. **Cache Lookup**: the resolver looks up the parsed files by `tool_name` in the LRU cache. An expired entry counts as a miss.
3. **FileDescriptorSet Parsing**: on a cache miss, the resolver decodes the base64 set, builds `protoregistry.Files`, and caches the result.
4. **Dynamic Message Creation**: the resolver makes a `dynamicpb.Message` from the descriptor. The message is a normal `proto.Message`.

## Performance Characteristics

- **Global Registry Lookup**: about 100ns
- **Cache Hit**: about 200ns
- **Cache Miss**: about 1-5ms (base64 decode, proto unmarshal, descriptor creation)
- **Memory**: about 10-50KB for each cached FileDescriptorSet

## Metrics

`DefaultProtoResolver` records its work in an internal `ProtoMetricsCollector`. You can also use a collector directly:

```go
collector := protoresolver.NewProtoMetricsCollector()

collector.RecordResolution(
    protoresolver.StrategyGlobalTypes,
    protoresolver.StatusSuccess,
    1.5, // duration in milliseconds
)
collector.RecordCacheHit()
collector.RecordCacheMiss()
collector.RecordCacheEviction()
```

The strategy labels are `StrategyGlobalTypes`, `StrategyFileDescriptorSet` and `StrategyFileDescriptorCached`. The status labels are `StatusSuccess` and `StatusError`.

## Troubleshooting

### Type Resolution Failures

**Problem**: `SchemaNotFoundError` when a tool runs

**Solution**:
- Make sure that the tool sends `file_descriptor_set` in its metadata.
- Make sure that the `tool_name` metadata matches the name of the tool.
- Set `LogFallbacks` to see the resolution attempts.

**Problem**: the resolver cannot find the type in the FileDescriptorSet

**Solution**:
- Check the type name for typos. The name is case-sensitive.
- Use the full package name: `gibson.tools.{tool}.{Message}`.

### Cache Issues

**Problem**: a stale schema after a tool update

**Solution**:
- Lower `CacheTTL`. The resolver parses the set again after the entry expires.
- Restart the process to clear the cache.

**Problem**: a high cache miss rate

**Solution**:
- Increase `CacheMaxEntries` if many tools are in use.
- Increase `CacheTTL` if the schemas do not change often.
- Make sure that each tool sends the same `tool_name` on every request.

## Testing

```bash
go test ./protoresolver/
go test -bench=. ./protoresolver/
go test -cover ./protoresolver/
```

## Thread Safety

- `DefaultProtoResolver` is safe for concurrent use.
- The LRU cache uses a `sync.RWMutex`.
- `DynamicMessageFactory` has no state and is safe for concurrent use.

## See Also

- [Gibson SDK README](../README.md)
- [Protocol Buffers Documentation](https://protobuf.dev/)
- [dynamicpb Package](https://pkg.go.dev/google.golang.org/protobuf/types/dynamicpb)
