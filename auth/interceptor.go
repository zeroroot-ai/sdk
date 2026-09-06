// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Zero Root AI

package auth

import (
	"context"
	"errors"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

// UnaryServerInterceptor returns a grpc.UnaryServerInterceptor that
// reads the `x-gibson-identity-*` headers from incoming metadata,
// constructs a validated Identity, and places it on the request
// context. On any header-construction failure it rejects the request
// with codes.PermissionDenied — the daemon serves no traffic without a
// validated identity.
//
// This interceptor performs ZERO authentication work in the
// cryptographic sense:
//   - it does NOT validate JWT signatures (Envoy's jwt_authn does)
//   - it does NOT call OpenFGA (ext-authz does)
//   - it does NOT verify HMAC (the channel between Envoy and the
//     daemon is SPIFFE-pinned mTLS; HMAC is unnecessary)
//   - it does NOT consult any external service
//
// The interceptor's only job is the structural one: project the
// trusted-channel-delivered headers into a TenantID-typed context so
// handler code can write `auth.TenantFromContext(ctx)` and trust the
// answer.
//
// Spec: unified-identity-and-authorization Requirements 8.5, 8.6, 8.7.
func UnaryServerInterceptor() grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req any, _ *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (any, error) {
		newCtx, err := identityIntoContext(ctx)
		if err != nil {
			return nil, err
		}
		return handler(newCtx, req)
	}
}

// StreamServerInterceptor is the streaming equivalent of
// UnaryServerInterceptor. The Identity is established once at stream
// open and remains stable for the lifetime of the stream — clients do
// not re-authenticate per message.
func StreamServerInterceptor() grpc.StreamServerInterceptor {
	return func(srv any, ss grpc.ServerStream, _ *grpc.StreamServerInfo, handler grpc.StreamHandler) error {
		newCtx, err := identityIntoContext(ss.Context())
		if err != nil {
			return err
		}
		return handler(srv, &serverStreamWithContext{ServerStream: ss, ctx: newCtx})
	}
}

// identityIntoContext is the shared implementation. It returns a
// gRPC status error (codes.PermissionDenied) on failure so callers can
// return it directly.
func identityIntoContext(ctx context.Context) (context.Context, error) {
	md, ok := metadata.FromIncomingContext(ctx)
	if !ok {
		return ctx, status.Error(codes.PermissionDenied, "auth: no metadata on incoming context")
	}
	id, err := IdentityFromMetadata(md)
	if err != nil {
		switch {
		case errors.Is(err, ErrMissingIdentity):
			return ctx, status.Errorf(codes.PermissionDenied, "auth: %s", err)
		case errors.Is(err, ErrInvalidIdentity), errors.Is(err, ErrInvalidTenant):
			return ctx, status.Errorf(codes.PermissionDenied, "auth: %s", err)
		default:
			// Should not reach: IdentityFromMetadata returns only the
			// two sentinel errors. Map to PermissionDenied to fail-
			// closed in case a future error path is added.
			return ctx, status.Errorf(codes.PermissionDenied, "auth: %s", err)
		}
	}
	return WithIdentity(ctx, id), nil
}

// serverStreamWithContext wraps a grpc.ServerStream so the handler
// sees the identity-augmented context. This is the standard Go gRPC
// pattern for stream interceptors that mutate context.
type serverStreamWithContext struct {
	grpc.ServerStream
	ctx context.Context
}

func (s *serverStreamWithContext) Context() context.Context { return s.ctx }
