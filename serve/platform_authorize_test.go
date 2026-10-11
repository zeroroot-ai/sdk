// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Zero Root AI

package serve

import (
	"context"
	"errors"
	"log/slog"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	componentpb "github.com/zeroroot-ai/sdk/api/gen/gibson/component/v1"
	harnessconst "github.com/zeroroot-ai/sdk/harness"
)

// authorizeServer answers ComponentService.Authorize as the test says.
type authorizeServer struct {
	componentpb.UnimplementedComponentServiceServer
	answer func(*componentpb.AuthorizeRequest) (*componentpb.AuthorizeResponse, error)
	got    []*componentpb.AuthorizeRequest
}

func (s *authorizeServer) Authorize(_ context.Context, req *componentpb.AuthorizeRequest) (*componentpb.AuthorizeResponse, error) {
	s.got = append(s.got, req)
	return s.answer(req)
}

// newAuthorizeHarness builds a PlatformHarness on an in-process server that
// serves only Authorize.
func newAuthorizeHarness(t *testing.T, srv *authorizeServer, workID string) *PlatformHarness {
	t.Helper()
	h, cleanup := setupAuthorizeTest(t, srv, workID)
	t.Cleanup(cleanup)
	return h
}

func setupAuthorizeTest(t *testing.T, srv *authorizeServer, workID string) (h *PlatformHarness, cleanup func()) {
	t.Helper()
	// Reuse the dial logic of the parity tests by registering our server in
	// place of theirs.
	h, cleanup = setupParityTestWith(t, func(s *grpc.Server) { componentpb.RegisterComponentServiceServer(s, srv) })
	h.workID = workID
	return h, cleanup
}

func TestPlatformHarness_Authorize(t *testing.T) {
	denyWith := func(c codes.Code) func(*componentpb.AuthorizeRequest) (*componentpb.AuthorizeResponse, error) {
		return func(*componentpb.AuthorizeRequest) (*componentpb.AuthorizeResponse, error) {
			return nil, status.Error(c, "refused")
		}
	}
	cases := []struct {
		name   string
		answer func(*componentpb.AuthorizeRequest) (*componentpb.AuthorizeResponse, error)
		want   error // nil means allowed
		other  bool  // want a plain error that is none of the sentinels
	}{
		{"an explicit allow", func(*componentpb.AuthorizeRequest) (*componentpb.AuthorizeResponse, error) {
			return &componentpb.AuthorizeResponse{Allowed: true, Reason: "fga_allow"}, nil
		}, nil, false},
		{"a denial", func(*componentpb.AuthorizeRequest) (*componentpb.AuthorizeResponse, error) {
			return &componentpb.AuthorizeResponse{Allowed: false, Reason: "not_authorized"}, nil
		}, harnessconst.ErrUnauthorized, false},
		{"an empty answer", func(*componentpb.AuthorizeRequest) (*componentpb.AuthorizeResponse, error) {
			return &componentpb.AuthorizeResponse{}, nil
		}, harnessconst.ErrUnauthorized, false},
		{"NotFound", denyWith(codes.NotFound), harnessconst.ErrUnauthorized, false},
		{"FailedPrecondition", denyWith(codes.FailedPrecondition), harnessconst.ErrUnauthorized, false},
		{"PermissionDenied", denyWith(codes.PermissionDenied), harnessconst.ErrUnauthorized, false},
		{"Unauthenticated", denyWith(codes.Unauthenticated), harnessconst.ErrUnauthorized, false},
		{"InvalidArgument", denyWith(codes.InvalidArgument), harnessconst.ErrInvalidAction, false},
		{"Unimplemented", denyWith(codes.Unimplemented), harnessconst.ErrAuthzServiceUnavailable, false},
		{"Unavailable", denyWith(codes.Unavailable), harnessconst.ErrAuthzServiceUnavailable, false},
		{"DeadlineExceeded", denyWith(codes.DeadlineExceeded), harnessconst.ErrAuthzServiceUnavailable, false},
		{"Internal", denyWith(codes.Internal), nil, true},
		{"Unknown", denyWith(codes.Unknown), nil, true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			srv := &authorizeServer{answer: tc.answer}
			h := newAuthorizeHarness(t, srv, "work-9")

			err := h.Authorize(context.Background(), "execute", "tool:nmap")

			require.Len(t, srv.got, 1)
			assert.Equal(t, "work-9", srv.got[0].GetWorkId())
			assert.Equal(t, "execute", srv.got[0].GetAction())
			assert.Equal(t, "tool:nmap", srv.got[0].GetResource())
			switch {
			case tc.want != nil:
				require.ErrorIs(t, err, tc.want)
			case tc.other:
				require.Error(t, err)
				for _, sentinel := range []error{harnessconst.ErrUnauthorized, harnessconst.ErrInvalidAction, harnessconst.ErrAuthzServiceUnavailable} {
					require.NotErrorIs(t, err, sentinel, "must not read as a sentinel")
				}
			default:
				require.NoError(t, err)
			}
		})
	}

	t.Run("an empty action sends nothing", func(t *testing.T) {
		srv := &authorizeServer{answer: denyWith(codes.OK)}
		h := newAuthorizeHarness(t, srv, "work-9")
		require.ErrorIs(t, h.Authorize(context.Background(), "", "tool:nmap"), harnessconst.ErrInvalidAction)
		require.ErrorIs(t, h.Authorize(context.Background(), "execute", ""), harnessconst.ErrInvalidAction)
		assert.Empty(t, srv.got)
	})
	t.Run("a harness with no work item sends nothing", func(t *testing.T) {
		srv := &authorizeServer{answer: denyWith(codes.OK)}
		h := newAuthorizeHarness(t, srv, "")
		require.ErrorIs(t, h.Authorize(context.Background(), "execute", "tool:nmap"), harnessconst.ErrNoRunContext)
		assert.Empty(t, srv.got)
	})
}

func TestPlatformClient_Authorize_NotConnected(t *testing.T) {
	pc, err := NewPlatformClientWithOptions("http://127.0.0.1:1", "")
	require.NoError(t, err)
	allowed, _, err := pc.Authorize(context.Background(), "w", "execute", "tool:nmap")
	require.Error(t, err)
	assert.False(t, allowed)
}

// TestAuthorizeError_NoCodeAllows proves that no gRPC code maps to nil.
func TestAuthorizeError_NoCodeAllows(t *testing.T) {
	for c := codes.OK; c <= codes.Unauthenticated+3; c++ {
		err := authorizeError(context.Background(), slog.New(slog.DiscardHandler), "w", "execute", "tool:nmap", status.Error(c, "x"))
		require.Error(t, err, "code %s must not allow", c)
	}
	require.Error(t, authorizeError(context.Background(), slog.New(slog.DiscardHandler), "w", "execute", "tool:nmap", errors.New("not a status")))
}
