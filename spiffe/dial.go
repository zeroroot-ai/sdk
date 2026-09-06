// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Zero Root AI

// Package spiffe wraps the SPIFFE Workload API for Gibson services.
//
// The SDK exposes two thin helpers:
//
//  1. DialOptions(ctx) returns gRPC dial options that present an
//     X.509 SVID for mTLS when the SPIFFE Workload API socket is
//     reachable, and falls back to plain TLS (server-side
//     verification only) when it is not. This dual mode is required
//     by Requirement 10.1: agent binaries running on customer
//     networks (not k8s, no SPIRE) authenticate with Zitadel JWTs
//     over server-side TLS, while in-cluster pods get full mTLS via
//     SPIFFE.
//
//  2. ExpectPeerSPIFFEID(spiffeID) returns a gRPC server option that
//     accepts only connections whose peer presents an SVID matching
//     the supplied SPIFFE ID. The Gibson daemon uses this to
//     enforce the "every byte from Envoy" invariant: the listener
//     rejects any connection whose peer is not Envoy at the TLS
//     handshake.
//
// Spec: unified-identity-and-authorization Requirements 2.7, 10.1.
package spiffe

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/spiffe/go-spiffe/v2/spiffeid"
	"github.com/spiffe/go-spiffe/v2/spiffetls/tlsconfig"
	"github.com/spiffe/go-spiffe/v2/workloadapi"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
)

// SocketEnv is the standard environment variable that points to the
// SPIFFE Workload API socket. SPIRE Agents inject this into every
// pod-attested workload.
const SocketEnv = "SPIFFE_ENDPOINT_SOCKET"

// DialOptions returns gRPC dial options suitable for connecting to a
// Gibson service through Envoy.
//
//   - When the SPIFFE Workload API is available (SocketEnv set), the
//     options request mTLS using the X.509 SVID delivered by the
//     Workload API; the server's SVID is also validated against the
//     SPIFFE trust bundle.
//   - When the Workload API is not available, the options use plain
//     TLS with the system root CA pool (or whatever ServerName-aware
//     defaults the runtime supplies). This is the path taken by
//     external (customer-network, non-k8s) agents — they authenticate
//     via the Zitadel JWT in the Authorization header rather than at
//     the TLS layer.
//
// DialOptions never panics. It does NOT contact the Workload API
// synchronously beyond the existence check; the X509Source is started
// lazily and updates rotate transparently.
//
// Caller must call the returned cleanup function to release the
// X509Source (when one was created). Calling cleanup is safe even
// when no source was created (it is a no-op in the plain-TLS path).
func DialOptions(ctx context.Context) ([]grpc.DialOption, func(), error) {
	if !workloadAPIAvailable() {
		// Plain TLS only. The system root CA pool covers Let's Encrypt
		// and other public CAs used at the SaaS Envoy edge.
		return []grpc.DialOption{
			grpc.WithTransportCredentials(credentials.NewTLS(&tls.Config{MinVersion: tls.VersionTLS13})),
		}, func() {}, nil
	}

	src, err := workloadapi.NewX509Source(ctx)
	if err != nil {
		return nil, func() {}, fmt.Errorf("spiffe: open X509Source: %w", err)
	}

	// AuthorizeAny accepts any SPIFFE peer in the trust domain. Most
	// callers want this — they trust Envoy because Envoy is the only
	// reachable peer (NetworkPolicy enforces it). Callers that want
	// stricter peer matching pass their own dial options after
	// composing with these.
	creds := credentials.NewTLS(tlsconfig.MTLSClientConfig(src, src, tlsconfig.AuthorizeAny()))
	cleanup := func() { _ = src.Close() }

	return []grpc.DialOption{grpc.WithTransportCredentials(creds)}, cleanup, nil
}

// ServerOptions returns gRPC server options that present the
// workload's own X.509 SVID and require mTLS from peers. Use
// ExpectPeerSPIFFEID to additionally restrict the accepted peer
// SPIFFE ID — without that restriction the listener accepts any
// SVID-presenting peer in the trust bundle.
//
// Returns an error if the Workload API socket is not reachable.
// In-cluster Gibson services REQUIRE SPIFFE; calling code should
// fail-closed if ServerOptions errors at startup.
func ServerOptions(ctx context.Context) ([]grpc.ServerOption, func(), error) {
	if !workloadAPIAvailable() {
		return nil, func() {}, ErrWorkloadAPIUnavailable
	}
	src, err := workloadapi.NewX509Source(ctx)
	if err != nil {
		return nil, func() {}, fmt.Errorf("spiffe: open X509Source: %w", err)
	}
	creds := credentials.NewTLS(tlsconfig.MTLSServerConfig(src, src, tlsconfig.AuthorizeAny()))
	cleanup := func() { _ = src.Close() }
	return []grpc.ServerOption{grpc.Creds(creds)}, cleanup, nil
}

// ExpectPeerSPIFFEID returns a gRPC server option that accepts only
// connections whose peer's X.509 SVID matches the supplied SPIFFE ID.
// The daemon uses this to lock its listener to Envoy's specific SVID
// — any other peer is rejected at the TLS handshake.
//
// Returns an error if the Workload API socket is not reachable, or
// if id cannot be parsed as a SPIFFE ID.
func ExpectPeerSPIFFEID(ctx context.Context, id string) ([]grpc.ServerOption, func(), error) {
	if !workloadAPIAvailable() {
		return nil, func() {}, ErrWorkloadAPIUnavailable
	}
	parsed, err := spiffeid.FromString(id)
	if err != nil {
		return nil, func() {}, fmt.Errorf("spiffe: parse %q: %w", id, err)
	}
	src, err := workloadapi.NewX509Source(ctx)
	if err != nil {
		return nil, func() {}, fmt.Errorf("spiffe: open X509Source: %w", err)
	}
	creds := credentials.NewTLS(tlsconfig.MTLSServerConfig(src, src, tlsconfig.AuthorizeID(parsed)))
	cleanup := func() { _ = src.Close() }
	return []grpc.ServerOption{grpc.Creds(creds)}, cleanup, nil
}

// workloadAPIAvailable reports whether the SPIFFE Workload API socket
// is reachable. Two checks: the env var is set, and (when it points to
// a unix path) the socket file exists. Lightweight and does not open
// a connection.
func workloadAPIAvailable() bool {
	addr := os.Getenv(SocketEnv)
	if addr == "" {
		return false
	}
	if u := strings.TrimPrefix(addr, "unix://"); u != addr {
		// unix socket — sanity check the file exists
		_, err := os.Stat(u)
		return err == nil
	}
	// Other schemes (tcp://, etc.) are accepted on the env var alone;
	// the X509Source open will surface any reachability error.
	return true
}

// ErrWorkloadAPIUnavailable is returned when SPIFFE-required options
// (ServerOptions, ExpectPeerSPIFFEID) are requested but the Workload
// API socket is not reachable. Callers should treat this as a
// fail-closed configuration error.
var ErrWorkloadAPIUnavailable = errors.New("spiffe: Workload API socket not reachable; cannot fetch SVID")
