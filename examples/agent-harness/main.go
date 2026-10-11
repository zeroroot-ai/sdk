// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Zero Root AI

package main

import (
	"context"
	"fmt"
	"log"
	"os"

	sdk "github.com/zeroroot-ai/sdk"
	"github.com/zeroroot-ai/sdk/types"
)

// Environment variables of the agent.
const (
	// envCallbackEndpoint names the daemon address. When set, the agent runs as
	// a oneshot fork on the callback transport. When unset, it serves the pull
	// transport of the platform.
	envCallbackEndpoint = "GIBSON_CALLBACK_ENDPOINT"
	// envCallbackToken is the grant of the callback client.
	envCallbackToken = "GIBSON_CALLBACK_TOKEN" //nolint:gosec // the name of a variable, not a value
)

func main() {
	if err := run(context.Background(), os.Getenv); err != nil {
		log.Fatal(err)
	}
}

func run(ctx context.Context, getenv func(string) string) error {
	a, err := New()
	if err != nil {
		return err
	}
	log.Printf("slots: %v", SlotSummary(a))

	endpoint := getenv(envCallbackEndpoint)
	if endpoint == "" {
		if err := sdk.ServeAgent(a); err != nil {
			return fmt.Errorf("serve the agent: %w", err)
		}
		return nil
	}
	summary, err := RunFork(ctx, a, CallbackOptions{
		Endpoint: endpoint,
		Token:    getenv(envCallbackToken),
		Mission:  types.MissionContext{ID: getenv("GIBSON_MISSION_ID")},
	}, ForkOptions{})
	if err != nil {
		return err
	}
	log.Print(summary)
	return nil
}
