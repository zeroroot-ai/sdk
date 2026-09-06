// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Zero Root AI

// Package capabilitygrant implements the client side of the Gibson Capability Grant Protocol.
//
// # Overview
//
// External agents use this package to discover, register with, and authenticate
// to the Gibson platform. All communication is protocol-level only: the package
// contains no server-side secrets, no FGA knowledge, and no authorization logic.
//
// The package talks to the dashboard over HTTPS (port 443) for the Agent Auth
// HTTP endpoints and to the same host over gRPC for daemon harness callbacks.
//
// # Key Concepts
//
// Host Key — A persistent Ed25519 keypair that identifies a physical or logical
// host running one or more agents. Stored on disk (0600 permissions). Generated
// once and reused across restarts. The host ID is the RFC 7638 JWK thumbprint of
// the public key.
//
// Agent Key — An ephemeral Ed25519 keypair generated fresh on each agent process
// start. Used to sign short-lived agent+jwt tokens for per-call authentication.
//
// Bootstrap Credential — A one-time credential (API key or Kubernetes service
// account token) used to authenticate the first-time host registration. After
// registration succeeds the host key is used for all subsequent authentications.
//
// Discovery Document — A JSON document served at
// /.well-known/agent-configuration that describes the platform's protocol
// version, supported modes, and endpoint URLs.
//
// # Quick Start
//
//	client, err := capabilitygrant.NewClient(capabilitygrant.ClientConfig{
//	    PlatformURL: "https://platform.example.com",
//	    AgentName:   "my-scanner",
//	    AgentMode:   "autonomous",
//	})
//	if err != nil {
//	    log.Fatal(err)
//	}
//
//	ctx := context.Background()
//	if err := client.Discover(ctx); err != nil {
//	    log.Fatal(err)
//	}
//	if err := client.Register(ctx); err != nil {
//	    log.Fatal(err)
//	}
//
//	// Use client.AgentID() and client.GRPCPerRPCCredentials() when
//	// dialling the daemon for harness callbacks.
//
// # Security Notes
//
// - JWTs produced by this package expire in 55 seconds (under the 60-second
// server-enforced limit). Never cache them.
// - Host key files are written with 0600 permissions.
// - Bootstrap tokens are consumed once and never persisted.
// - The package uses stdlib crypto/ed25519 and net/http only; no third-party
// JWT libraries.
package capabilitygrant
