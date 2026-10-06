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

	// LogFallbacks, when enabled, logs informational messages when falling back
	// from global registry to dynamic resolution using FileDescriptorSets.
	// Useful for debugging type resolution issues.
	// Default: false
	LogFallbacks bool
}

// Sentinel errors for common resolution failures.
var (
	// ErrNoSchemaAvailable indicates that type resolution cannot proceed because
	// no schema (FileDescriptorSet) is available in metadata and the type is not
	// found in the global proto registry.
	ErrNoSchemaAvailable = errors.New("no schema available for type resolution")
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
