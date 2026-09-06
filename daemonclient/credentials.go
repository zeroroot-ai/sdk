// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Zero Root AI

// Package daemonclient: credential auto-detection for connecting to the Gibson daemon.
//
// Detection: SPIRE Workload API socket at /run/spire/sockets/agent.sock → mTLS
// with SVID. (The former OIDC client_credentials fallback was removed per
// ADR-0045 — no component presents a raw Zitadel token to the daemon; agents
// and tools authenticate with the Capability-Grant JWT via sdk/agent.Connect,
// plugins via sdk/plugin.Serve. This package serves internal scripts and
// in-cluster SPIFFE callers only.)
//
// TLS is mandatory. The system trust store is used by default. Set GIBSON_DAEMON_CA
// to a PEM file path to additionally trust a self-signed CA (Kind/dev clusters).
package daemonclient

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"os"

	"github.com/spiffe/go-spiffe/v2/spiffetls/tlsconfig"
	"github.com/spiffe/go-spiffe/v2/workloadapi"
	"google.golang.org/grpc/credentials"
)

// Env var names used by credential auto-detection.
const (
	// EnvDaemonCA is an optional path to a PEM-encoded CA certificate file.
	// When set, the CA is added to the TLS trust pool alongside system roots.
	// Intended for Kind/dev clusters that use a self-signed CA.
	EnvDaemonCA = "GIBSON_DAEMON_CA"

	// spireSocketPath is the canonical SPIRE Workload API socket path.
	spireSocketPath = "/run/spire/sockets/agent.sock"
)

// Option is a functional option for New and NewWithCredentials.
type Option func(*clientConfig)

type clientConfig struct {
	// reserved for future options (e.g., custom dialer, keepalive)
}

// detectCredentials runs the two-step auto-detection sequence and returns
// the first credential found plus the TLS transport credentials to use.
//
// Returns an error only when no credential is detected or TLS cannot be built.
func detectCredentials(ctx context.Context) (credentials.PerRPCCredentials, credentials.TransportCredentials, error) {
	// SPIRE Workload API socket → mTLS with SVID (the only auto-detected
	// credential; ADR-0045 removed the OIDC client_credentials fallback).
	if _, err := os.Stat(spireSocketPath); err == nil {
		tc, perRPC, err := buildSPIRECredentials(ctx)
		if err != nil {
			return nil, nil, fmt.Errorf("SPIRE credential setup failed: %w", err)
		}
		return perRPC, tc, nil
	}

	return nil, nil, fmt.Errorf(
		"no credential detected: ensure the SPIRE Workload API socket is present at %s",
		spireSocketPath,
	)
}

// buildTLSTransport constructs gRPC transport credentials from the system trust store
// plus an optional GIBSON_DAEMON_CA file. Returns an error if no trust roots are found.
func buildTLSTransport() (credentials.TransportCredentials, error) {
	hasSystemRoots := true
	pool, err := x509.SystemCertPool()
	if err != nil {
		// Some minimal environments (scratch containers) may not have system roots.
		// We'll check below whether GIBSON_DAEMON_CA can rescue us.
		pool = x509.NewCertPool()
		hasSystemRoots = false
	}

	hasExtraCA := false
	caPath := os.Getenv(EnvDaemonCA)
	if caPath != "" {
		pem, err := os.ReadFile(caPath)
		if err != nil {
			return nil, fmt.Errorf("reading %s=%q: %w", EnvDaemonCA, caPath, err)
		}
		if !pool.AppendCertsFromPEM(pem) {
			return nil, fmt.Errorf("%s=%q: no valid PEM certificates found in file", EnvDaemonCA, caPath)
		}
		hasExtraCA = true
	}

	// Require at least one root: system roots or an explicit CA.
	if !hasSystemRoots && !hasExtraCA {
		return nil, fmt.Errorf(
			"no TLS trust roots available: system certificate pool is empty and %s is not set",
			EnvDaemonCA,
		)
	}

	return credentials.NewTLS(&tls.Config{
		RootCAs:    pool,
		MinVersion: tls.VersionTLS12,
	}), nil
}

// buildSPIRECredentials fetches a SVID from the SPIRE Workload API and returns
// mTLS transport credentials. The perRPC return value is nil because SPIFFE mTLS
// is transport-level — no per-RPC header is needed.
func buildSPIRECredentials(ctx context.Context) (credentials.TransportCredentials, credentials.PerRPCCredentials, error) {
	source, err := workloadapi.NewX509Source(
		ctx,
		workloadapi.WithClientOptions(workloadapi.WithAddr("unix://"+spireSocketPath)),
	)
	if err != nil {
		return nil, nil, fmt.Errorf("workload API source: %w", err)
	}
	// Authorise any SPIFFE ID in the same trust domain as the caller.
	tc := credentials.NewTLS(tlsconfig.MTLSClientConfig(source, source, tlsconfig.AuthorizeAny()))
	return tc, nil, nil
}
