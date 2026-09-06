// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Zero Root AI

// Package extraction provides a framework for converting tool-specific proto
// responses into standardized GraphRAG DiscoveryResult messages.
//
// Tool developers implement the EntityExtractor interface for their tool's
// response type, then pass it to serve.WithExtractor() so the SDK serve loop
// auto-populates proto field 100 before publishing results.
//
// Example:
//
//	type MyExtractor struct{}
//
//	func (e *MyExtractor) ToolName() string              { return "mytool" }
//	func (e *MyExtractor) CanExtract(msg proto.Message) bool { _, ok := msg.(*mypb.Response); return ok }
//	func (e *MyExtractor) Extract(ctx context.Context, msg proto.Message) (*graphragpb.DiscoveryResult, error) {
//	    resp := msg.(*mypb.Response)
//	    return &graphragpb.DiscoveryResult{Hosts: convertHosts(resp)}, nil
//	}
//
//	// In main.go:
//	serve.Tool(myTool, serve.WithExtractor(&MyExtractor{}))
package extraction

import (
	"context"
	"errors"
	"fmt"
	"sync"

	"google.golang.org/protobuf/proto"

	graphragpb "github.com/zeroroot-ai/sdk/api/gen/gibson/graphrag/v1"
)

// EntityExtractor converts a tool-specific proto response into a standardized
// DiscoveryResult for GraphRAG storage. Each tool implements this interface
// for its own response type.
type EntityExtractor interface {
	// ToolName returns the name of the tool this extractor handles.
	// Must match the tool's Name() return value (e.g., "mytool").
	ToolName() string

	// CanExtract returns true if this extractor can process the given message.
	CanExtract(msg proto.Message) bool

	// Extract converts a tool response into a DiscoveryResult containing
	// graph entities (hosts, ports, services, findings, etc.).
	// Returns nil DiscoveryResult (not error) for empty results.
	Extract(ctx context.Context, msg proto.Message) (*graphragpb.DiscoveryResult, error)
}

// ExtractorRegistry manages EntityExtractor implementations by tool name.
// It is safe for concurrent use.
type ExtractorRegistry interface {
	// Register adds an extractor for its tool. Returns error if already registered.
	Register(extractor EntityExtractor) error

	// Unregister removes an extractor by tool name.
	Unregister(toolName string) error

	// Get retrieves an extractor by tool name.
	Get(toolName string) (EntityExtractor, error)

	// Has checks if an extractor is registered for the tool.
	Has(toolName string) bool

	// ListTools returns names of all tools with registered extractors.
	ListTools() []string

	// ExtractFromResponse extracts entities using the appropriate extractor.
	ExtractFromResponse(ctx context.Context, toolName string, msg proto.Message) (*graphragpb.DiscoveryResult, error)
}

type registry struct {
	mu         sync.RWMutex
	extractors map[string]EntityExtractor
}

// NewExtractorRegistry creates an empty ExtractorRegistry.
func NewExtractorRegistry() ExtractorRegistry {
	return &registry{
		extractors: make(map[string]EntityExtractor),
	}
}

func (r *registry) Register(extractor EntityExtractor) error {
	if extractor == nil {
		return errors.New("extractor cannot be nil")
	}
	toolName := extractor.ToolName()
	if toolName == "" {
		return errors.New("extractor tool name cannot be empty")
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	if _, exists := r.extractors[toolName]; exists {
		return fmt.Errorf("extractor for tool %q already registered", toolName)
	}
	r.extractors[toolName] = extractor
	return nil
}

func (r *registry) Unregister(toolName string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if _, exists := r.extractors[toolName]; !exists {
		return fmt.Errorf("no extractor registered for tool %q", toolName)
	}
	delete(r.extractors, toolName)
	return nil
}

func (r *registry) Get(toolName string) (EntityExtractor, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	extractor, exists := r.extractors[toolName]
	if !exists {
		return nil, fmt.Errorf("no extractor registered for tool %q", toolName)
	}
	return extractor, nil
}

func (r *registry) Has(toolName string) bool {
	r.mu.RLock()
	defer r.mu.RUnlock()
	_, exists := r.extractors[toolName]
	return exists
}

func (r *registry) ListTools() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()

	tools := make([]string, 0, len(r.extractors))
	for name := range r.extractors {
		tools = append(tools, name)
	}
	return tools
}

func (r *registry) ExtractFromResponse(ctx context.Context, toolName string, msg proto.Message) (*graphragpb.DiscoveryResult, error) {
	extractor, err := r.Get(toolName)
	if err != nil {
		return nil, err
	}
	if !extractor.CanExtract(msg) {
		return nil, fmt.Errorf("extractor for tool %q cannot process message type %T", toolName, msg)
	}
	result, err := extractor.Extract(ctx, msg)
	if err != nil {
		return nil, fmt.Errorf("extraction failed for tool %q: %w", toolName, err)
	}
	return result, nil
}
