# protoresolver

Package `protoresolver` provides utilities for dynamically resolving and unmarshaling Protocol Buffer message types at runtime. This is essential for systems that need to handle proto messages without having compiled Go types available, such as when using FileDescriptorSets for dynamic type resolution.

## Overview

The protoresolver package enables Gibson to interact with tools that use self-contained proto definitions without requiring those types to be compiled into Gibson Core. This is achieved through:

1. **Dynamic Message Factory**: Creates `dynamicpb.Message` instances from FileDescriptorSet definitions
2. **LRU Caching**: Caches parsed FileDescriptorSets to avoid repeated parsing overhead (~1-5ms per parse)
3. **Fallback Resolution**: Attempts to resolve types from the global proto registry first, then falls back to FileDescriptorSet-based dynamic resolution

## Key Interfaces

### ProtoResolver

The main interface for resolving proto message types:

```go
type ProtoResolver interface {
    // Resolve input message types
    ResolveInputType(ctx context.Context, typeName string, metadata map[string]string) (proto.Message, error)

    // Resolve output message types
    ResolveOutputType(ctx context.Context, typeName string, metadata map[string]string) (proto.Message, error)

    // Unmarshal JSON into a resolved message type
    UnmarshalJSON(ctx context.Context, typeName string, jsonData []byte, metadata map[string]string) (proto.Message, error)
}
```

### CachingProtoResolver

Extends `ProtoResolver` with cache management:

```go
type CachingProtoResolver interface {
    ProtoResolver

    // Invalidate cache for a specific tool or all tools ("*")
    InvalidateCache(toolName string)
}
```

## Usage

### Basic Setup

```go
import (
    "github.com/zeroroot-ai/sdk/protoresolver"
)

// Create resolver with default configuration
resolver := protoresolver.NewDefaultProtoResolver(protoresolver.DefaultConfig())

// Or customize configuration
config := protoresolver.ProtoResolverConfig{
    CacheMaxEntries: 200,              // Max FileDescriptorSets to cache
    CacheTTL:        2 * time.Hour,    // Cache entry lifetime
    StrictMode:      true,             // Fail if type not found
    LogFallbacks:    true,             // Log when falling back to FileDescriptorSet
}
resolver := protoresolver.NewDefaultProtoResolver(config)
```

### Resolving Types

The resolver requires metadata to locate the FileDescriptorSet for a tool:

```go
// Metadata typically includes:
// - "tool_name": Name of the tool providing the schema
// - "file_descriptor_set": Base64-encoded FileDescriptorSet
metadata := map[string]string{
    "tool_name":           "mytool-c",
    "file_descriptor_set": fdsBase64,
}

// Resolve input message type
msg, err := resolver.ResolveInputType(ctx, "gibson.tools.mytool-c.MytoolCRequest", metadata)
if err != nil {
    log.Fatal(err)
}

// msg is now a proto.Message instance (either from global registry or dynamicpb)
```

### Unmarshaling JSON

```go
jsonData := []byte(`{"target": "example.com", "ports": "80,443"}`)

msg, err := resolver.UnmarshalJSON(
    ctx,
    "gibson.tools.mytool-c.MytoolCRequest",
    jsonData,
    metadata,
)
if err != nil {
    log.Fatal(err)
}

// msg is now a populated proto.Message
```

### Cache Management

```go
// Invalidate cache for a specific tool (e.g., after schema update)
if cachingResolver, ok := resolver.(protoresolver.CachingProtoResolver); ok {
    cachingResolver.InvalidateCache("mytool-c")
}

// Invalidate all cached schemas
cachingResolver.InvalidateCache("*")
```

## Configuration Options

### ProtoResolverConfig

```go
type ProtoResolverConfig struct {
    // CacheMaxEntries: Maximum number of FileDescriptorSets to cache
    // When limit is reached, least recently used entries are evicted
    // Default: 100
    CacheMaxEntries int

    // CacheTTL: How long cached FileDescriptorSets remain valid
    // After this duration, entries are re-parsed from metadata
    // Default: 1 hour
    CacheTTL time.Duration

    // StrictMode: Fail immediately if type not found and no FileDescriptorSet available
    // When false, resolver may attempt fallback strategies
    // Default: true
    StrictMode bool

    // LogFallbacks: Log when falling back from global registry to FileDescriptorSet
    // Useful for debugging type resolution issues
    // Default: true
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
//     StrictMode:      true,
//     LogFallbacks:    true,
// }
```

## Error Handling

The package provides typed errors for common failure scenarios:

### SchemaNotFoundError

Indicates that a FileDescriptorSet was not found for the specified tool:

```go
msg, err := resolver.ResolveInputType(ctx, typeName, metadata)
if err != nil {
    var schemaErr *protoresolver.SchemaNotFoundError
    if errors.As(err, &schemaErr) {
        log.Printf("Schema not found for tool: %s", schemaErr.ToolName)
        log.Printf("While resolving type: %s", schemaErr.TypeName)
        // Handle missing schema...
    }
}
```

### TypeNotFoundError

Indicates that a proto type was not found in available schemas:

```go
msg, err := resolver.ResolveInputType(ctx, typeName, metadata)
if err != nil {
    var typeErr *protoresolver.TypeNotFoundError
    if errors.As(err, &typeErr) {
        log.Printf("Type not found: %s", typeErr.TypeName)
        log.Printf("Available types: %v", typeErr.AvailableTypes)
        // Suggest correct type name...
    }
}
```

### Sentinel Errors

```go
// No schema available (neither in global registry nor FileDescriptorSet)
if errors.Is(err, protoresolver.ErrNoSchemaAvailable) {
    // Handle missing schema...
}

// FileDescriptorSet is malformed or invalid
if errors.Is(err, protoresolver.ErrInvalidFileDescriptorSet) {
    // Handle invalid descriptor set...
}
```

## Resolution Strategy

The resolver follows this resolution strategy:

1. **Global Registry Lookup**: First attempts to resolve the type from `protoregistry.GlobalTypes`
   - Fast lookup for compiled proto types
   - No parsing overhead
   - Returns immediately if found

2. **Cache Lookup**: If not in global registry, checks the LRU cache for a previously parsed FileDescriptorSet
   - O(1) lookup by tool name
   - Returns cached `protoregistry.Files` if found and not expired

3. **FileDescriptorSet Parsing**: If cache miss, parses the FileDescriptorSet from metadata
   - Decodes base64-encoded descriptor set
   - Creates `protoregistry.Files` from descriptor
   - Caches the result for future use

4. **Dynamic Message Creation**: Creates a `dynamicpb.Message` instance from the resolved descriptor
   - Fully compatible with `proto.Message` interface
   - Can be marshaled/unmarshaled like any proto message

## Performance Characteristics

- **Global Registry Lookup**: ~100ns (in-memory map lookup)
- **Cache Hit**: ~200ns (LRU cache lookup + descriptor lookup)
- **Cache Miss**: ~1-5ms (base64 decode + proto unmarshal + descriptor creation)
- **Memory**: ~10-50KB per cached FileDescriptorSet (depends on schema complexity)

### Cache Statistics

Monitor cache performance using the `Stats()` method:

```go
if cache, ok := resolver.(*protoresolver.DefaultProtoResolver); ok {
    stats := cache.cache.Stats()
    hitRate := float64(stats.Hits) / float64(stats.Hits + stats.Misses)

    log.Printf("Cache hit rate: %.2f%%", hitRate*100)
    log.Printf("Current entries: %d", stats.Entries)
    log.Printf("Total evictions: %d", stats.Evictions)
}
```

## Metrics and Observability

The package provides metrics collection for monitoring proto resolution performance in production:

### ProtoMetricsCollector

```go
import "github.com/zeroroot-ai/sdk/protoresolver"

// Create metrics collector
collector := protoresolver.NewProtoMetricsCollector()

// Record resolution attempts
collector.RecordResolution(
    protoresolver.StrategyGlobalTypes,
    protoresolver.StatusSuccess,
    1.5, // duration in milliseconds
)

// Record cache operations
collector.RecordCacheHit()
collector.RecordCacheMiss()
collector.RecordCacheEviction()

// Get statistics
stats := collector.Stats()
fmt.Printf("Global types success: %d\n", stats.GlobalTypesSuccess)
fmt.Printf("FileDescriptorSet success: %d\n", stats.FDSSuccess)
fmt.Printf("Cache hit rate: %.2f%%\n",
    float64(stats.CacheHits) / float64(stats.CacheHits + stats.CacheMisses) * 100)
fmt.Printf("P95 resolution time: %.2fms\n", stats.P95DurationMs)
```

### Available Metrics

The collector tracks:

- **Resolution Strategy Counters**:
  - `global_types` success/error
  - `file_descriptor_set` success/error
  - `file_descriptor_cached` success/error

- **Cache Performance**:
  - Cache hits
  - Cache misses
  - Cache evictions

- **Duration Statistics**:
  - Average resolution time
  - Min/Max resolution time
  - P50, P95, P99 percentiles

### Metric Constants

```go
const (
    // Metric names
    MetricProtoResolutionTotal    = "gibson.proto.resolution.total"
    MetricProtoResolutionDuration = "gibson.proto.resolution.duration_ms"
    MetricProtoCacheHits          = "gibson.proto.cache.hits"
    MetricProtoCacheMisses        = "gibson.proto.cache.misses"
    MetricProtoCacheEvictions     = "gibson.proto.cache.evictions"

    // Strategy labels
    StrategyGlobalTypes          = "global_types"
    StrategyFileDescriptorSet    = "file_descriptor_set"
    StrategyFileDescriptorCached = "file_descriptor_cached"

    // Status labels
    StatusSuccess = "success"
    StatusError   = "error"
)
```

### Custom Metrics Integration

Implement the `MetricsRecorder` interface to integrate with your metrics backend (Prometheus, OpenTelemetry, etc.):

```go
type MetricsRecorder interface {
    RecordCounter(name string, value int64, labels map[string]string)
    RecordGauge(name string, value float64, labels map[string]string)
    RecordHistogram(name string, value float64, labels map[string]string)
}

// Example Prometheus integration
type PrometheusRecorder struct {
    registry *prometheus.Registry
}

func (r *PrometheusRecorder) RecordCounter(name string, value int64, labels map[string]string) {
    // Forward to Prometheus counter
}
```

## Integration with Gibson

### Tool Execution

The protoresolver is used by `GRPCToolClient` to resolve tool input/output types:

```go
// In gibson/internal/registry/adapter.go
type RegistryAdapter struct {
    resolver protoresolver.ProtoResolver
}

// Tool execution uses resolver to handle dynamic types
func (a *RegistryAdapter) DiscoverTool(ctx context.Context, name string) (tool.Tool, error) {
    // ... discovery logic ...
    client := NewGRPCToolClient(conn, *selected, a.resolver)
    return client, nil
}
```

### SchemaProvider Interface

Tools that implement the `SchemaProvider` interface expose their proto schema:

```go
// Tool interface extended with schema methods
type Tool interface {
    Execute(ctx context.Context, input proto.Message) (proto.Message, error)

    // Schema provider methods
    InputMessageType() string    // e.g., "gibson.tools.mytool-c.MytoolCRequest"
    OutputMessageType() string   // e.g., "gibson.tools.mytool-c.MytoolCResponse"
}
```

The resolver uses these type names to locate and instantiate the correct message types.

## Troubleshooting

### Type Resolution Failures

**Problem**: `SchemaNotFoundError` when executing a tool

**Solution**:
- Verify the tool is properly registered with metadata containing `file_descriptor_set`
- Check that `tool_name` metadata matches the tool's registration name
- Enable `LogFallbacks` to see resolution attempts

**Problem**: `TypeNotFoundError` with available types list

**Solution**:
- Check for typos in the type name (case-sensitive)
- Verify the type name includes the full package: `gibson.tools.{tool}.{Message}`
- Review available types in the error message for the correct name

### Cache Issues

**Problem**: Stale schema after tool update

**Solution**:
- Invalidate the tool's cache entry: `resolver.InvalidateCache("tool-name")`
- Or restart Gibson to clear all caches

**Problem**: High cache miss rate

**Solution**:
- Increase `CacheMaxEntries` if many tools are in use
- Increase `CacheTTL` if schemas are stable
- Check if tool names are consistent across requests

### Performance Issues

**Problem**: Slow type resolution

**Solution**:
- Verify cache is enabled (`CacheMaxEntries > 0`)
- Check cache hit rate with `Stats()`
- Consider pre-warming cache by resolving common types at startup

## Testing

The package includes comprehensive tests:

```bash
cd /home/anthony/Code/zeroroot.ai/core/sdk/protoresolver
go test -v ./...
go test -bench=. ./...  # Run benchmarks
go test -cover ./...    # Check coverage
```

### Test Coverage

- Dynamic message creation
- Cache LRU eviction
- TTL expiration
- Concurrent access
- Error conditions

## Thread Safety

All public interfaces are thread-safe:

- `DefaultProtoResolver`: Safe for concurrent use
- `FileDescriptorCache`: Uses `sync.RWMutex` for concurrent reads
- `DynamicMessageFactory`: Stateless, safe for concurrent use

## Future Enhancements

Potential improvements:

1. **Preloading**: Support for preloading frequently used schemas at startup
2. **Schema Versioning**: Track schema versions and support migrations
3. **Compression**: LZ4 compression for cached FileDescriptorSets to reduce memory usage
4. **OpenTelemetry Integration**: Native OpenTelemetry exporter for automatic metrics export

## See Also

- [Gibson Tool Development Guide](/home/anthony/Code/zeroroot.ai/tools/CLAUDE.md)
- [Protocol Buffers Documentation](https://protobuf.dev/)
- [dynamicpb Package](https://pkg.go.dev/google.golang.org/protobuf/types/dynamicpb)
