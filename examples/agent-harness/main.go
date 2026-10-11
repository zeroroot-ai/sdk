// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Zero Root AI

package main

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"errors"
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
	// envGrant is the capability grant that the daemon injects into the sandbox.
	envGrant = "GIBSON_CG_JWT" //nolint:gosec // the name of a variable, not a value
	// envCAPEM is the CA of the TLS edge of the platform.
	envCAPEM = "GIBSON_PLATFORM_CA_PEM"
	// envMissionRunID is the mission run that the daemon gave the sandbox.
	envMissionRunID = "GIBSON_MISSION_RUN_ID"
	envMissionID    = "GIBSON_MISSION_ID"
)

// tlsFromPEM builds the client TLS config that trusts the CA in pem. Empty pem
// means no TLS config, and the client dials without TLS.
func tlsFromPEM(pem string) (*tls.Config, error) {
	if pem == "" {
		return nil, nil
	}
	pool := x509.NewCertPool()
	if !pool.AppendCertsFromPEM([]byte(pem)) {
		return nil, errors.New(envCAPEM + " holds no certificate")
	}
	return &tls.Config{RootCAs: pool, MinVersion: tls.VersionTLS13}, nil
}

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
	tlsConf, err := tlsFromPEM(getenv(envCAPEM))
	if err != nil {
		return err
	}
	summary, err := RunFork(ctx, a, CallbackOptions{
		Endpoint:     endpoint,
		Token:        getenv(envGrant),
		TLS:          tlsConf,
		Mission:      types.MissionContext{ID: getenv(envMissionID)},
		MissionRunID: getenv(envMissionRunID),
	}, ForkOptions{})
	if err != nil {
		return err
	}
	log.Print(summary)
	return nil
}
