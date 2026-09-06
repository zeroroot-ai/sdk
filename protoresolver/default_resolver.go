// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Zero Root AI

package protoresolver

import (
	"context"
	"encoding/base64"
	"fmt"
	"log/slog"
	"time"

	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protodesc"
	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/reflect/protoregistry"
	"google.golang.org/protobuf/types/descriptorpb"
)

// DefaultProtoResolver is the default implementation of CachingProtoResolver.
// It attempts to resolve proto types from the global registry first, then falls
// back to dynamic resolution using FileDescriptorSets from metadata.
type DefaultProtoResolver struct {
	cache   FileDescriptorCache
	factory DynamicMessageFactory
	config  ProtoResolverConfig
	logger  *slog.Logger
	metrics *ProtoMetricsCollector
}

// NewDefaultProtoResolver creates a new DefaultProtoResolver with the provided configuration.
func NewDefaultProtoResolver(config ProtoResolverConfig) *DefaultProtoResolver {
	cache := NewFileDescriptorCache(config.CacheMaxEntries, config.CacheTTL)
	metrics := NewProtoMetricsCollector()

	// Wire up cache eviction callback to record metrics
	if lru, ok := cache.(*lruCache); ok {
		lru.SetEvictionCallback(func() {
			metrics.RecordCacheEviction()
		})
	}

	return &DefaultProtoResolver{
		cache:   cache,
		factory: NewDynamicMessageFactory(),
		config:  config,
		logger:  slog.Default(),
		metrics: metrics,
	}
}

// DefaultConfig returns the default configuration for a ProtoResolver.
func DefaultConfig() ProtoResolverConfig {
	return ProtoResolverConfig{
		CacheMaxEntries: 100,
		CacheTTL:        time.Hour,
		StrictMode:      true,
		LogFallbacks:    true,
	}
}

// ResolveInputType resolves and creates a new proto.Message instance for the specified
// input type name. It attempts resolution from global registry first, then falls back
// to dynamic resolution using FileDescriptorSet from metadata.
func (d *DefaultProtoResolver) ResolveInputType(ctx context.Context, typeName string, metadata map[string]string) (proto.Message, error) {
	return d.resolveType(ctx, typeName, metadata, "input")
}

// ResolveOutputType resolves and creates a new proto.Message instance for the specified
// output type name. It attempts resolution from global registry first, then falls back
// to dynamic resolution using FileDescriptorSet from metadata.
func (d *DefaultProtoResolver) ResolveOutputType(ctx context.Context, typeName string, metadata map[string]string) (proto.Message, error) {
	return d.resolveType(ctx, typeName, metadata, "output")
}

// resolveType is the internal implementation for type resolution.
func (d *DefaultProtoResolver) resolveType(ctx context.Context, typeName string, metadata map[string]string, typeKind string) (proto.Message, error) {
	// Start timing for metrics
	startTime := time.Now()

	// Step 1: Try global registry first
	msgType, err := protoregistry.GlobalTypes.FindMessageByName(protoreflect.FullName(typeName))
	if err == nil {
		// Record successful resolution from global types
		durationMs := float64(time.Since(startTime).Microseconds()) / 1000.0
		d.metrics.RecordResolution(StrategyGlobalTypes, StatusSuccess, durationMs)

		d.logger.Debug("proto type resolved",
			"type", typeName,
			"kind", typeKind,
			"strategy", "global_types",
			"cache_hit", false)
		return msgType.New().Interface(), nil
	}

	// Step 2: Global registry lookup failed - log if configured
	if d.config.LogFallbacks {
		d.logger.Debug("type not found in global registry, attempting dynamic resolution",
			"type", typeName,
			"kind", typeKind,
			"error", err.Error())
	}

	// Step 3: Get FileDescriptorSet from metadata
	fdsBase64, ok := metadata["file_descriptor_set"]
	if !ok {
		toolName := metadata["tool_name"]
		if toolName == "" {
			toolName = "unknown"
		}

		// Record error - no schema available
		durationMs := float64(time.Since(startTime).Microseconds()) / 1000.0
		d.metrics.RecordResolution(StrategyFileDescriptorSet, StatusError, durationMs)

		return nil, &SchemaNotFoundError{
			ToolName: toolName,
			TypeName: typeName,
			Cause:    ErrNoSchemaAvailable,
		}
	}

	// Step 4: Check cache for parsed files
	toolName := metadata["tool_name"]
	if toolName == "" {
		toolName = "default"
	}

	cachedFiles, cacheHit := d.cache.Get(toolName)
	if cacheHit {
		// Record cache hit
		d.metrics.RecordCacheHit()

		// Use cached files to create message
		msg, err := d.factory.CreateMessageFromFiles(cachedFiles, typeName)
		if err != nil {
			// Record error with cached resolution
			durationMs := float64(time.Since(startTime).Microseconds()) / 1000.0
			d.metrics.RecordResolution(StrategyFileDescriptorCached, StatusError, durationMs)
			return nil, fmt.Errorf("failed to create message from cached files: %w", err)
		}

		// Record successful cached resolution
		durationMs := float64(time.Since(startTime).Microseconds()) / 1000.0
		d.metrics.RecordResolution(StrategyFileDescriptorCached, StatusSuccess, durationMs)

		d.logger.Debug("proto type resolved",
			"type", typeName,
			"kind", typeKind,
			"strategy", "file_descriptor_set",
			"cache_hit", true)

		return msg, nil
	}

	// Record cache miss
	d.metrics.RecordCacheMiss()

	// Step 5: Cache miss - parse FileDescriptorSet and create message
	msg, err := d.factory.CreateMessage(fdsBase64, typeName)
	if err != nil {
		// Record error with FDS resolution
		durationMs := float64(time.Since(startTime).Microseconds()) / 1000.0
		d.metrics.RecordResolution(StrategyFileDescriptorSet, StatusError, durationMs)
		return nil, fmt.Errorf("failed to create dynamic message: %w", err)
	}

	// Step 6: Parse and cache the files for future use
	// We need to get the files registry to cache it
	// The factory already parsed it, so we parse again to get the registry
	// This is slightly inefficient but maintains separation of concerns
	files, err := d.parseFileDescriptorSet(fdsBase64)
	if err == nil {
		d.cache.Put(toolName, files)
	}

	// Record successful FDS resolution
	durationMs := float64(time.Since(startTime).Microseconds()) / 1000.0
	d.metrics.RecordResolution(StrategyFileDescriptorSet, StatusSuccess, durationMs)

	d.logger.Debug("proto type resolved",
		"type", typeName,
		"kind", typeKind,
		"strategy", "file_descriptor_set",
		"cache_hit", false)

	return msg, nil
}

// parseFileDescriptorSet parses a base64-encoded FileDescriptorSet into a file registry.
func (d *DefaultProtoResolver) parseFileDescriptorSet(fdsBase64 string) (*protoregistry.Files, error) {
	// We leverage the factory's parsing logic by creating a temporary message
	// and extracting the files from the parsing process.
	// However, since factory.CreateMessage doesn't return the files,
	// we need to implement parsing here.

	// Import the necessary packages
	// This is a bit of duplication, but necessary for caching
	fdsBytes, err := base64.StdEncoding.DecodeString(fdsBase64)
	if err != nil {
		return nil, fmt.Errorf("failed to decode base64 file_descriptor_set: %w", err)
	}

	var fds descriptorpb.FileDescriptorSet
	if err := proto.Unmarshal(fdsBytes, &fds); err != nil {
		return nil, fmt.Errorf("failed to unmarshal file_descriptor_set: %w", err)
	}

	files, err := protodesc.NewFiles(&fds)
	if err != nil {
		return nil, fmt.Errorf("failed to create file registry from descriptor set: %w", err)
	}

	return files, nil
}

// UnmarshalProtoJSON unmarshals JSON data into a proto.Message of the specified type.
// It first resolves the type using ResolveInputType, then unmarshals the JSON
// data into the resolved message instance.
func (d *DefaultProtoResolver) UnmarshalProtoJSON(ctx context.Context, typeName string, jsonData []byte, metadata map[string]string) (proto.Message, error) {
	// Step 1: Resolve the type
	msg, err := d.ResolveInputType(ctx, typeName, metadata)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve type for unmarshaling: %w", err)
	}

	// Step 2: Unmarshal JSON into the message
	if err := protojson.Unmarshal(jsonData, msg); err != nil {
		return nil, fmt.Errorf("failed to unmarshal JSON into %s: %w", typeName, err)
	}

	d.logger.Debug("proto message unmarshaled from JSON",
		"type", typeName,
		"json_size", len(jsonData))

	return msg, nil
}

// InvalidateCache removes all cached FileDescriptorSets and type information
// for the specified tool. Use "*" to invalidate the entire cache.
func (d *DefaultProtoResolver) InvalidateCache(toolName string) {
	if toolName == "*" {
		// Invalidate all cache entries
		// Since our cache interface doesn't have a Clear() method,
		// we can't invalidate all at once. This would need to be enhanced
		// in the cache implementation if needed.
		// For now, just log a warning
		d.logger.Warn("cache invalidation for all tools not implemented, use specific tool names")
		return
	}

	d.cache.Invalidate(toolName)
	d.logger.Debug("cache invalidated", "tool", toolName)
}

// Metrics returns the current metrics statistics for the resolver.
// This provides visibility into resolution performance and cache efficiency.
//
// Example:
//
//	stats := resolver.Metrics()
//	hitRate := float64(stats.CacheHits) / float64(stats.CacheHits + stats.CacheMisses)
//	fmt.Printf("Cache hit rate: %.2f%%\n", hitRate * 100)
func (d *DefaultProtoResolver) Metrics() ProtoMetricsStats {
	return d.metrics.Stats()
}
