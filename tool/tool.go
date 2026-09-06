// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Zero Root AI

package tool

import (
	"context"

	"github.com/zeroroot-ai/sdk/types"
	"google.golang.org/protobuf/proto"
)

// Tool is the interface for SDK tools.
// Tools are executable components that perform specific operations with protocol buffer messages.
// All tools must implement proto-based execution using InputMessageType, OutputMessageType, and ExecuteProto.
type Tool interface {
	// Name returns the unique identifier for this tool.
	Name() string

	// Version returns the semantic version of this tool.
	Version() string

	// Description returns a human-readable description of what this tool does.
	Description() string

	// Tags returns a list of tags for categorizing and discovering this tool.
	Tags() []string

	// InputMessageType returns the fully-qualified proto message type name for input.
	// Example: "zero_day.tools.http.HttpRequest"
	// This type name is used to dynamically create and unmarshal proto messages.
	InputMessageType() string

	// OutputMessageType returns the fully-qualified proto message type name for output.
	// Example: "zero_day.tools.http.HttpResponse"
	// This type name is used to dynamically create and marshal proto messages.
	OutputMessageType() string

	// ExecuteProto runs the tool with proto message input and returns proto message output.
	// The input parameter must be a pointer to the proto message type specified by InputMessageType.
	// Returns a pointer to the proto message type specified by OutputMessageType.
	// Context is used for cancellation, deadlines, and request-scoped values.
	ExecuteProto(ctx context.Context, input proto.Message) (proto.Message, error)

	// Health checks the operational status of the tool.
	// This can be used to verify dependencies, resources, and readiness.
	Health(ctx context.Context) types.HealthStatus
}
