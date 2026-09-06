// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Zero Root AI

package protoresolver

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"google.golang.org/protobuf/proto"
)

// ProtoResolver provides methods for resolving and unmarshaling protocol buffer types
// dynamically at runtime. This is essential for tools that need to handle proto messages
// without having the compiled Go types available, such as when using FileDescriptorSets
// for dynamic type resolution.
//
// Implementations should attempt to resolve types from the global proto registry first,
// and fall back to dynamic resolution using FileDescriptorSets when compiled types
// are unavailable.
type ProtoResolver interface {
	// ResolveInputType resolves and creates a new proto.Message instance for the specified
	// input type name. The metadata map may contain information such as "tool_name" or
	// "file_descriptor_set" to aid in resolution.
	//
	// Returns an error if the type cannot be found or if the FileDescriptorSet is invalid.
	// The returned message will be a zero-initialized instance ready for unmarshaling.
	ResolveInputType(ctx context.Context, typeName string, metadata map[string]string) (proto.Message, error)

	// ResolveOutputType resolves and creates a new proto.Message instance for the specified
	// output type name. The metadata map may contain information such as "tool_name" or
	// "file_descriptor_set" to aid in resolution.
	//
	// Returns an error if the type cannot be found or if the FileDescriptorSet is invalid.
	// The returned message will be a zero-initialized instance ready for unmarshaling.
	ResolveOutputType(ctx context.Context, typeName string, metadata map[string]string) (proto.Message, error)

	// UnmarshalProtoJSON unmarshals JSON data into a proto.Message of the specified type.
	// This method first resolves the type using the metadata, then unmarshals the JSON
	// data into the resolved message instance.
	//
	// The metadata map may contain information such as "tool_name" or "file_descriptor_set"
	// to aid in type resolution.
	//
	// Returns an error if type resolution fails, if the JSON is invalid, or if unmarshaling
	// fails due to schema mismatch.
	UnmarshalProtoJSON(ctx context.Context, typeName string, jsonData []byte, metadata map[string]string) (proto.Message, error)
}

// CachingProtoResolver extends ProtoResolver with caching capabilities for improved
// performance when repeatedly resolving the same types. Implementations should cache
// parsed FileDescriptorSets and resolved type descriptors.
//
// Cache invalidation should be used when tool schemas are updated or redeployed.
type CachingProtoResolver interface {
	ProtoResolver

	// InvalidateCache removes all cached FileDescriptorSets and type information
	// for the specified tool. Use "*" to invalidate the entire cache.
	//
	// This should be called when:
	// - A tool's proto schema has been updated
	// - A tool has been redeployed with schema changes
	// - Memory pressure requires cache cleanup
	InvalidateCache(toolName string)
}

// ProtoResolverConfig contains configuration options for creating a ProtoResolver.
// These settings control caching behavior, error handling, and logging.
type ProtoResolverConfig struct {
	// CacheMaxEntries specifies the maximum number of FileDescriptorSets to cache.
	// When this limit is reached, the least recently used entry is evicted.
	// Set to 0 to disable caching (not recommended for production).
	// Default: 100
	CacheMaxEntries int

	// CacheTTL specifies how long cached FileDescriptorSets remain valid.
	// After this duration, cached entries are considered expired and will be
	// re-parsed from metadata on next access.
	// Default: 1 hour
	CacheTTL time.Duration

	// StrictMode, when enabled, causes resolution to fail immediately if the type
	// is not found in the global registry and no FileDescriptorSet is available.
	// When disabled, the resolver may attempt fallback strategies.
	// Default: false
	StrictMode bool

	// LogFallbacks, when enabled, logs informational messages when falling back
	// from global registry to dynamic resolution using FileDescriptorSets.
	// Useful for debugging type resolution issues.
	// Default: false
	LogFallbacks bool
}

// ResolutionResult contains detailed information about how a proto type was resolved.
// This is useful for debugging, monitoring, and understanding the resolver's behavior.
type ResolutionResult struct {
	// Message is the resolved proto.Message instance.
	Message proto.Message

	// Strategy indicates how the message was resolved:
	// - "global": Resolved from protoregistry.GlobalTypes
	// - "dynamic": Created using dynamicpb from FileDescriptorSet
	// - "cached": Retrieved from cache (may be global or dynamic)
	Strategy string

	// IsDynamic is true if the message is a dynamicpb.Message rather than
	// a compiled proto type.
	IsDynamic bool

	// ToolName is the name of the tool that provided the schema, if applicable.
	ToolName string

	// TypeName is the fully-qualified proto type name that was resolved.
	TypeName string
}

// Sentinel errors for common resolution failures.
var (
	// ErrNoSchemaAvailable indicates that type resolution cannot proceed because
	// no schema (FileDescriptorSet) is available in metadata and the type is not
	// found in the global proto registry.
	ErrNoSchemaAvailable = errors.New("no schema available for type resolution")

	// ErrInvalidFileDescriptorSet indicates that the FileDescriptorSet provided
	// in metadata is malformed, cannot be decoded, or fails validation.
	ErrInvalidFileDescriptorSet = errors.New("invalid file descriptor set")
)

// SchemaNotFoundError indicates that a FileDescriptorSet was not found for the
// specified tool name. This typically occurs when:
// - The tool name is incorrect or misspelled
// - The tool has not registered its schema
// - The schema metadata is missing from the request
type SchemaNotFoundError struct {
	// ToolName is the name of the tool for which the schema was not found.
	ToolName string

	// TypeName is the proto type that was being resolved when the error occurred.
	TypeName string

	// Cause is the underlying error that led to this failure, if available.
	Cause error
}

// Error implements the error interface, providing a descriptive error message.
func (e *SchemaNotFoundError) Error() string {
	var parts []string
	parts = append(parts, fmt.Sprintf("schema not found for tool %q", e.ToolName))

	if e.TypeName != "" {
		parts = append(parts, fmt.Sprintf("while resolving type %q", e.TypeName))
	}

	if e.Cause != nil {
		parts = append(parts, fmt.Sprintf(": %v", e.Cause))
	}

	return strings.Join(parts, " ")
}

// Unwrap returns the underlying cause error, supporting error chain unwrapping.
func (e *SchemaNotFoundError) Unwrap() error {
	return e.Cause
}

// TypeNotFoundError indicates that a proto type was not found in the available
// schemas (both global registry and FileDescriptorSet). This error includes
// information about available types to help with debugging.
type TypeNotFoundError struct {
	// TypeName is the fully-qualified proto type name that was not found.
	TypeName string

	// AvailableTypes is a list of type names that are available in the schema.
	// This helps developers identify typos or find the correct type name.
	// May be empty if the schema is not available or too large to enumerate.
	AvailableTypes []string
}

// Error implements the error interface, providing a descriptive error message
// with suggestions for available types.
func (e *TypeNotFoundError) Error() string {
	msg := fmt.Sprintf("type %q not found", e.TypeName)

	if len(e.AvailableTypes) > 0 {
		// Show up to 10 available types as suggestions
		suggestions := e.AvailableTypes
		if len(suggestions) > 10 {
			suggestions = suggestions[:10]
		}
		msg += fmt.Sprintf(" (available types: %s)", strings.Join(suggestions, ", "))

		if len(e.AvailableTypes) > 10 {
			msg += fmt.Sprintf(" and %d more", len(e.AvailableTypes)-10)
		}
	}

	return msg
}
