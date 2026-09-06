// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Zero Root AI

// Command tool-runner-hello is the simplest possible Gibson sandboxed tool:
// echoes its input string with a "hello, " prefix. Used as the integration
// smoke target for opensource/setec/development/k3s/.
package main

import (
	"context"

	// Import wrapperspb so google.protobuf.StringValue is registered with the
	// proto runtime (the runner looks it up by fully-qualified type name).
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/wrapperspb"

	toolrunner "github.com/zeroroot-ai/sdk/toolrunner"
	"github.com/zeroroot-ai/sdk/types"
)

type helloTool struct{}

func (helloTool) Name() string                              { return "hello" }
func (helloTool) Version() string                           { return "0.1.0" }
func (helloTool) Description() string                       { return "Echoes input with a 'hello, ' prefix." }
func (helloTool) Tags() []string                            { return []string{"example", "sandboxed"} }
func (helloTool) InputMessageType() string                  { return "google.protobuf.StringValue" }
func (helloTool) OutputMessageType() string                 { return "google.protobuf.StringValue" }
func (helloTool) Health(context.Context) types.HealthStatus { return types.NewHealthyStatus("ok") }

func (helloTool) ExecuteProto(_ context.Context, in proto.Message) (proto.Message, error) {
	s, _ := in.(*wrapperspb.StringValue)
	val := ""
	if s != nil {
		val = s.GetValue()
	}
	return wrapperspb.String("hello, " + val), nil
}

func main() {
	toolrunner.Run(helloTool{})
}
