// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Zero Root AI

package auth

import (
	"context"
	"errors"
	"strconv"
	"testing"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

func incomingMD(md metadata.MD) context.Context {
	return metadata.NewIncomingContext(context.Background(), md)
}

func TestUnaryInterceptor_HappyPath(t *testing.T) {
	md := metadata.New(map[string]string{
		HeaderSubject:        "user-1",
		HeaderIssuer:         string(IssuerOIDC),
		HeaderCredentialType: string(CredentialOIDCUser),
		HeaderTenant:         "acme",
		HeaderIssuedAt:       strconv.FormatInt(time.Now().Unix(), 10),
	})
	ctx := incomingMD(md)

	var seenIdentity Identity
	handler := func(ctx context.Context, req any) (any, error) {
		id, err := IdentityFromContext(ctx)
		if err != nil {
			return nil, err
		}
		seenIdentity = id
		return "ok", nil
	}

	resp, err := UnaryServerInterceptor()(ctx, nil, &grpc.UnaryServerInfo{FullMethod: "/Test/Method"}, handler)
	if err != nil {
		t.Fatalf("expected ok, got %v", err)
	}
	if resp != "ok" {
		t.Fatalf("unexpected resp %v", resp)
	}
	if seenIdentity.Tenant.String() != "acme" {
		t.Fatalf("handler saw wrong tenant: %q", seenIdentity.Tenant.String())
	}
}

func TestUnaryInterceptor_NoMetadata(t *testing.T) {
	ctx := context.Background()
	handlerCalled := false
	handler := func(ctx context.Context, req any) (any, error) {
		handlerCalled = true
		return nil, nil
	}
	_, err := UnaryServerInterceptor()(ctx, nil, &grpc.UnaryServerInfo{}, handler)
	if handlerCalled {
		t.Fatal("handler should not be called on missing metadata")
	}
	st, ok := status.FromError(err)
	if !ok {
		t.Fatalf("expected gRPC status, got %v", err)
	}
	if st.Code() != codes.PermissionDenied {
		t.Fatalf("expected PermissionDenied, got %v", st.Code())
	}
}

func TestUnaryInterceptor_MissingTenant(t *testing.T) {
	md := metadata.New(map[string]string{
		HeaderSubject:        "user-1",
		HeaderIssuer:         string(IssuerOIDC),
		HeaderCredentialType: string(CredentialOIDCUser),
		HeaderIssuedAt:       strconv.FormatInt(time.Now().Unix(), 10),
		// Tenant intentionally omitted.
	})
	ctx := incomingMD(md)
	handlerCalled := false
	handler := func(ctx context.Context, req any) (any, error) {
		handlerCalled = true
		return nil, nil
	}
	_, err := UnaryServerInterceptor()(ctx, nil, &grpc.UnaryServerInfo{}, handler)
	if handlerCalled {
		t.Fatal("handler must not be invoked when tenant header missing")
	}
	st, _ := status.FromError(err)
	if st.Code() != codes.PermissionDenied {
		t.Fatalf("expected PermissionDenied, got %v", st.Code())
	}
}

func TestUnaryInterceptor_InvalidTenant(t *testing.T) {
	md := metadata.New(map[string]string{
		HeaderSubject:        "user-1",
		HeaderIssuer:         string(IssuerOIDC),
		HeaderCredentialType: string(CredentialOIDCUser),
		HeaderTenant:         "INVALID UPPER",
		HeaderIssuedAt:       strconv.FormatInt(time.Now().Unix(), 10),
	})
	ctx := incomingMD(md)
	_, err := UnaryServerInterceptor()(ctx, nil, &grpc.UnaryServerInfo{}, func(ctx context.Context, req any) (any, error) {
		return nil, errors.New("must not be called")
	})
	st, _ := status.FromError(err)
	if st.Code() != codes.PermissionDenied {
		t.Fatalf("expected PermissionDenied, got %v", st.Code())
	}
}

func TestStreamInterceptor_HappyPath(t *testing.T) {
	md := metadata.New(map[string]string{
		HeaderSubject:        "u",
		HeaderIssuer:         string(IssuerOIDC),
		HeaderCredentialType: string(CredentialOIDCUser),
		HeaderTenant:         "acme",
		HeaderIssuedAt:       strconv.FormatInt(time.Now().Unix(), 10),
	})
	ctx := incomingMD(md)
	ss := &fakeServerStream{ctx: ctx}
	var seenTenant TenantID
	handler := func(srv any, ss grpc.ServerStream) error {
		tid, ok := TenantFromContext(ss.Context())
		if !ok {
			return errors.New("expected tenant")
		}
		seenTenant = tid
		return nil
	}
	if err := StreamServerInterceptor()(nil, ss, &grpc.StreamServerInfo{}, handler); err != nil {
		t.Fatalf("expected ok, got %v", err)
	}
	if seenTenant.String() != "acme" {
		t.Fatalf("stream handler saw wrong tenant: %q", seenTenant.String())
	}
}

func TestStreamInterceptor_NoMetadata(t *testing.T) {
	ss := &fakeServerStream{ctx: context.Background()}
	called := false
	handler := func(srv any, ss grpc.ServerStream) error {
		called = true
		return nil
	}
	err := StreamServerInterceptor()(nil, ss, &grpc.StreamServerInfo{}, handler)
	if called {
		t.Fatal("handler should not be invoked")
	}
	st, _ := status.FromError(err)
	if st.Code() != codes.PermissionDenied {
		t.Fatalf("expected PermissionDenied, got %v", st.Code())
	}
}

// fakeServerStream is a minimal grpc.ServerStream used to test stream
// interceptors. It only implements the methods our interceptor uses.
type fakeServerStream struct {
	grpc.ServerStream
	ctx context.Context
}

func (f *fakeServerStream) Context() context.Context { return f.ctx }
