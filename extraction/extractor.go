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

	"google.golang.org/protobuf/proto"

	graphragpb "github.com/zeroroot-ai/sdk/api/gen/gibson/graphrag/v1"
)

// EntityExtractor converts a tool-specific proto response into a standardized
// DiscoveryResult for GraphRAG storage. Each tool implements this interface
// for its own response type.
type EntityExtractor interface {

	// Extract converts a tool response into a DiscoveryResult containing
	// graph entities (hosts, ports, services, findings, etc.).
	// Returns nil DiscoveryResult (not error) for empty results.
	Extract(ctx context.Context, msg proto.Message) (*graphragpb.DiscoveryResult, error)
}
