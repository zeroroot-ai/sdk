// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Zero Root AI

package serve

import (
	"context"
	"errors"
	"fmt"
	"log/slog"

	grpccodes "google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	harnessconst "github.com/zeroroot-ai/sdk/harness"
)

// authorizeError maps the error of an Authorize RPC to the sentinel errors of
// the harness package. Both transports use it, so Authorize means the same on
// each (D86). Every error is a refusal: an answer that is not an explicit allow
// never becomes nil.
//
//   - Unimplemented, Unavailable, DeadlineExceeded, a non-gRPC error:
//     ErrAuthzServiceUnavailable (the daemon cannot answer, or has no RPC)
//   - NotFound, FailedPrecondition, PermissionDenied, Unauthenticated:
//     ErrUnauthorized
//   - InvalidArgument: ErrInvalidAction
//   - anything else: a plain error
//
// scope names the run or the work item for the log.
func authorizeError(ctx context.Context, logger *slog.Logger, scope, action, resource string, err error) error {
	st, ok := status.FromError(err)
	if !ok {
		return denyUnavailable(ctx, logger, action, resource, err)
	}
	// Every code is listed, so a code added to gRPC later is a lint finding and
	// not a silent default. Every case returns an error, none returns nil.
	switch st.Code() {
	case grpccodes.Unimplemented, grpccodes.Unavailable, grpccodes.DeadlineExceeded:
		return denyUnavailable(ctx, logger, action, resource, err)

	case grpccodes.NotFound, grpccodes.FailedPrecondition:
		// An unknown run, a run that is no longer active, or a work item with
		// no mission run: there is nothing to authorize against, so deny.
		logger.Warn("Authorize: no active run for the caller",
			"scope", scope, "action", action, "resource", resource, "code", st.Code().String())
		return harnessconst.ErrUnauthorized

	case grpccodes.InvalidArgument:
		// The daemon refuses a malformed request, for example a resource with no kind.
		return fmt.Errorf("%w: %s", harnessconst.ErrInvalidAction, st.Message())

	case grpccodes.PermissionDenied, grpccodes.Unauthenticated:
		return fmt.Errorf("%w: %s", harnessconst.ErrUnauthorized, st.Message())

	case grpccodes.OK, grpccodes.Canceled, grpccodes.Unknown, grpccodes.AlreadyExists,
		grpccodes.ResourceExhausted, grpccodes.Aborted, grpccodes.OutOfRange,
		grpccodes.Internal, grpccodes.DataLoss:
		return fmt.Errorf("Authorize RPC error: %w", errors.New(st.Message()))

	default:
		// A code that gRPC adds later. It is still an error.
		return fmt.Errorf("Authorize RPC error (code %s): %w", st.Code(), errors.New(st.Message()))
	}
}

// denyUnavailable denies the action when the daemon cannot answer.
func denyUnavailable(ctx context.Context, logger *slog.Logger, action, resource string, cause error) error {
	logger.ErrorContext(ctx, "authorization service unavailable — denying (fail-closed)",
		"action", action, "resource", resource, "error", cause)
	return harnessconst.ErrAuthzServiceUnavailable
}
