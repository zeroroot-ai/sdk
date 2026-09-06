// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Zero Root AI

package capabilitygrant

import (
	"context"
	"errors"
	"log/slog"
	"testing"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func testDetector(t *testing.T) (*revocationDetector, *int) {
	t.Helper()
	exitCalls := 0
	d := newRevocationDetector(slog.New(slog.DiscardHandler))
	d.onExit = func(_ int) { exitCalls++ }
	return d, &exitCalls
}

func TestRevocationDetector_SuccessResetsCounter(t *testing.T) {
	d, calls := testDetector(t)
	d.observe(status.Error(codes.Unauthenticated, "host revoked"))
	d.observe(status.Error(codes.Unauthenticated, "agent_revoked"))
	d.observe(nil) // success
	d.observe(status.Error(codes.Unauthenticated, "host revoked"))
	if *calls != 0 {
		t.Errorf("expected no exit, got %d calls", *calls)
	}
}

func TestRevocationDetector_ThresholdTriggers(t *testing.T) {
	d, calls := testDetector(t)
	for range RevocationThreshold {
		d.observe(status.Error(codes.Unauthenticated, "agent_revoked"))
	}
	if *calls != 1 {
		t.Errorf("expected exactly 1 exit call, got %d", *calls)
	}
}

func TestRevocationDetector_IgnoresNonAuthErrors(t *testing.T) {
	d, calls := testDetector(t)
	for range RevocationThreshold * 2 {
		d.observe(status.Error(codes.Unavailable, "connection failed"))
	}
	if *calls != 0 {
		t.Errorf("Unavailable errors should not count toward revocation; got %d exits", *calls)
	}
}

func TestRevocationDetector_IgnoresUnauthWithoutRevocationIndicator(t *testing.T) {
	d, calls := testDetector(t)
	for range RevocationThreshold * 2 {
		d.observe(status.Error(codes.Unauthenticated, "invalid jwt signature"))
	}
	if *calls != 0 {
		t.Errorf("Unauthenticated without revocation keyword should not count; got %d exits", *calls)
	}
}

func TestRevocationDetector_OnceGuard(t *testing.T) {
	d, calls := testDetector(t)
	for range RevocationThreshold * 5 {
		d.observe(status.Error(codes.Unauthenticated, "host revoked"))
	}
	if *calls != 1 {
		t.Errorf("exit must fire at most once; got %d", *calls)
	}
}

func TestRevocationUnaryInterceptor(t *testing.T) {
	c := &Client{logger: slog.New(slog.DiscardHandler)}
	exitCalls := 0
	interceptor := c.RevocationUnaryInterceptor()
	// Seed the detector with our test onExit.
	c.revocation().onExit = func(_ int) { exitCalls++ }

	revokedInvoker := func(
		ctx context.Context, method string, req, reply any,
		cc *grpc.ClientConn, opts ...grpc.CallOption,
	) error {
		return status.Error(codes.Unauthenticated, "agent_revoked: host is revoked")
	}

	for range RevocationThreshold {
		_ = interceptor(context.Background(), "/x/y", nil, nil, nil, revokedInvoker)
	}
	if exitCalls != 1 {
		t.Errorf("expected 1 exit after threshold; got %d", exitCalls)
	}
}

func TestIsRevocationMessage(t *testing.T) {
	cases := map[string]bool{
		"agent_revoked":              true,
		"Host revoked by admin":      true,
		"the agent has been revoked": true,
		"invalid signature":          false,
		"expired jwt":                false,
		"":                           false,
	}
	for in, want := range cases {
		if got := isRevocationMessage(in); got != want {
			t.Errorf("isRevocationMessage(%q) = %v, want %v", in, got, want)
		}
	}
}

// Ensure interceptor types satisfy gRPC's expected signatures.
var (
	_ grpc.UnaryClientInterceptor  = (&Client{}).RevocationUnaryInterceptor()
	_ grpc.StreamClientInterceptor = (&Client{}).RevocationStreamInterceptor()
	_                              = errors.New // keep errors import used
)
