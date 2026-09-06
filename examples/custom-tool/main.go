// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Zero Root AI

// Package main is a minimal Gibson tool example.
//
// IMPORTANT: For a real tool, run `gibson component init <name> --kind tool`
// from the ADK CLI (github.com/zeroroot-ai/adk). That scaffold ships
// the full proto pipeline (api/proto/<name>/v1/<name>.proto with
// field 100 = gibson.graphrag.v1.DiscoveryResult, buf.yaml, buf.gen.yaml,
// vendored SDK protos) plus an AGENTS.md teaching the contract.
//
// This example uses google.protobuf.StringValue / BytesValue as stand-
// ins for what would normally be your tool's own generated Request /
// Response types. It exists only to show the modern tool.Tool
// interface shape; do not pattern-match on it for real tools.
package main

import (
	"context"
	"fmt"
	"log"

	wrapperspb "google.golang.org/protobuf/types/known/wrapperspb"
	"google.golang.org/protobuf/proto"

	sdk "github.com/zeroroot-ai/sdk"
	"github.com/zeroroot-ai/sdk/serve"
	"github.com/zeroroot-ai/sdk/tool"
	"github.com/zeroroot-ai/sdk/types"
)

// EchoTool is a minimal Tool that echoes its input.
//
// In a real tool you would:
//
//  1. Define api/proto/<name>/v1/<name>.proto with EchoRequest /
//     EchoResponse messages, where EchoResponse declares field 100 as
//     `gibson.graphrag.v1.DiscoveryResult discovery = 100;` so the
//     daemon's DiscoveryProcessor can write any entities you discover
//     into the GraphRAG knowledge graph automatically.
//  2. Run `make proto` to generate Go bindings.
//  3. Replace wrapperspb.StringValue / BytesValue with the generated
//     types and assert/return the real types in ExecuteProto.
//
// `gibson component init --kind tool` does all of step 1 for you.
type EchoTool struct{}

func (t *EchoTool) Name() string        { return "echo-tool" }
func (t *EchoTool) Version() string     { return "0.1.0" }
func (t *EchoTool) Description() string { return "minimal example tool — see gibson component init --kind tool for the canonical scaffold" }
func (t *EchoTool) Tags() []string      { return []string{"example", "echo"} }

// InputMessageType / OutputMessageType return the fully-qualified proto
// message names. Gibson uses these to dispatch agent calls to the
// right tool. Real tools return their own
// "gibson.tools.<name>.v1.<Name>{Request,Response}".
func (t *EchoTool) InputMessageType() string  { return "google.protobuf.StringValue" }
func (t *EchoTool) OutputMessageType() string { return "google.protobuf.BytesValue" }

// ExecuteProto is the tool's entrypoint. The daemon serialises the
// agent's request into the input proto.Message and unwraps the
// response. In a real tool, after building your domain output, you
// would populate the response's Discovery field (proto field 100)
// with the entities + relationships you found — the daemon writes
// them to Neo4j automatically with no Cypher from you.
func (t *EchoTool) ExecuteProto(ctx context.Context, in proto.Message) (proto.Message, error) {
	req, ok := in.(*wrapperspb.StringValue)
	if !ok {
		return nil, fmt.Errorf("echo-tool: expected *wrapperspb.StringValue, got %T", in)
	}
	return wrapperspb.Bytes([]byte("echo: " + req.GetValue())), nil
}

func (t *EchoTool) Health(ctx context.Context) types.HealthStatus {
	return types.HealthStatus{Status: types.StatusHealthy}
}

func main() {
	t := &EchoTool{}

	// You can construct via the SDK builder…
	if _, err := sdk.NewTool(
		sdk.WithToolName(t.Name()),
		sdk.WithToolVersion(t.Version()),
		sdk.WithToolDescription(t.Description()),
		sdk.WithToolTags(t.Tags()...),
		sdk.WithInputMessageType(t.InputMessageType()),
		sdk.WithOutputMessageType(t.OutputMessageType()),
		sdk.WithExecuteProtoHandler(t.ExecuteProto),
	); err != nil {
		log.Fatalf("build tool: %v", err)
	}
	_ = (tool.Tool)(t) // compile-time assertion that EchoTool satisfies tool.Tool

	// …or implement the interface directly (as EchoTool does above) and
	// hand the value to serve.Tool. Both work.
	fmt.Println("Starting echo tool on :50051 …")
	if err := serve.Tool(t); err != nil {
		log.Fatalf("serve tool: %v", err)
	}
}
