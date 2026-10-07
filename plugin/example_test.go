// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Zero Root AI

package plugin_test

import (
	"context"
	"log"

	"github.com/zeroroot-ai/sdk/plugin"
)

// EchoRequest and EchoResponse are the plain typed Go structs the plugin author
// writes. The SDK derives the method's tool schema from them by reflection —
// there is no hand-written .proto and no codegen.
type EchoRequest struct {
	Message string `json:"message"`
}

type EchoResponse struct {
	Echoed string `json:"echoed"`
}

// echo is the typed handler. Its signature — func(ctx, Req) (Resp, error) — is
// the whole authoring contract (ADR-0065 R4).
func echo(_ context.Context, req EchoRequest) (EchoResponse, error) {
	return EchoResponse{Echoed: "echoed: " + req.Message}, nil
}

// ExampleServe demonstrates the minimal Go-first plugin main.go. The plugin
// declares its name, version and methods in code (ADR-0097).
//
// This example is compiled but not executed (no Output: comment) because
// plugin.Serve connects to a real daemon and blocks until shutdown.
func ExampleServe() {
	ctx := context.Background()

	err := plugin.Serve(ctx,
		plugin.WithName("echo"),
		plugin.WithVersion("0.1.0"),
		plugin.WithHandler("Echo", "echoes the message back", echo),
	)
	if err != nil {
		log.Fatal(err)
	}
}
