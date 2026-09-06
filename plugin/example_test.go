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

// ExampleServe demonstrates the minimal Go-first plugin main.go. The manifest
// path and method name match the plugin's plugin.yaml.
//
// This example is compiled but not executed (no Output: comment) because
// plugin.Serve connects to a real daemon and blocks until shutdown.
func ExampleServe() {
	ctx := context.Background()

	err := plugin.Serve(ctx,
		plugin.WithManifest("./plugin.yaml"),
		plugin.WithHandler("Echo", echo),
	)
	if err != nil {
		log.Fatal(err)
	}
}
