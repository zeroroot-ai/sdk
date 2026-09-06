// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Zero Root AI

// Package auth is the SDK-side identity and authorization surface.
//
// # Architecture
//
// Gibson's authentication and authorization model is layered:
//
//	Caller ──Bearer JWT──▶ Envoy
//	                        │ jwt_authn validates OIDC JWT
//	                        ▼
//	                       ext-authz
//	                        │ FGA check + identity-header inject
//	                        ▼
//	                       Daemon (this package's interceptor)
//	                        │ read x-gibson-identity-* headers
//	                        │ build Identity, place on ctx
//	                        ▼
//	                       Handler (uses TenantFromContext, pool.For)
//
// The daemon performs NO authentication or authorization work. Envoy
// validates JWTs against the configured OIDC provider's JWKS. ext-authz
// consults OpenFGA. The daemon's only job is to project the
// trusted-channel-delivered identity headers into a TenantID-typed context.
//
// # Wire-channel security
//
// Identity headers between Envoy and the daemon are NOT HMAC-signed.
// Channel security is provided by SPIFFE-pinned mTLS: the daemon's
// gRPC listener accepts only Envoy's specific SPIFFE peer SVID, and
// the listener is reachable only by Envoy via NetworkPolicy. A request
// reaching the daemon's interceptor has, by construction, transited
// the auth chain.
//
// # Public API surface
//
//   - TenantID: sealed validated tenant identifier; the connection-pool
//     selector for the data-plane spec
//   - Identity: request-scoped identity carrier
//   - WithIdentity / IdentityFromContext / TenantFromContext: context
//     plumbing
//   - UnaryServerInterceptor / StreamServerInterceptor: the gRPC
//     interceptors every Gibson Go service applies
//   - IdentityFromMetadata / IdentityToMetadata: header parse / emit
//     for tests and in-process dispatch
//
// # Spec reference
//
// This package implements Phase 1 of the
// `unified-identity-and-authorization` spec.
package auth
