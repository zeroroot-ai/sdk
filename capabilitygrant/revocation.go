// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Zero Root AI

package capabilitygrant

import (
	"context"
	"log/slog"
	"os"
	"strings"
	"sync"
	"sync/atomic"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// RevocationExitCode is the process exit code used when the SDK detects
// that the agent has been revoked. A dedicated exit code lets process
// managers (systemd, Docker, Kubernetes) distinguish this fatal state
// from transient errors and avoid restart loops.
const RevocationExitCode = 42

// RevocationThreshold is the number of consecutive 401 Unauthenticated
// responses with a revocation indicator required before the SDK
// self-terminates. Single 401s may be transient (token rotation races,
// clock skew); a run of three is unambiguous.
const RevocationThreshold = 3

// revocationIndicators are substrings in the error message that identify
// an explicit revocation response from the daemon. The daemon's
// CapabilityGrant verifier returns Unauthenticated with one of these strings
// when the host record is in status=revoked.
var revocationIndicators = []string{
	"agent_revoked",
	"host revoked",
	"revoked",
}

// revocationDetector counts consecutive revocation signals and triggers
// self-termination once RevocationThreshold is hit. Safe for concurrent
// use; the counter resets on any successful call.
type revocationDetector struct {
	logger    *slog.Logger
	consec    atomic.Int32
	once      sync.Once
	onExit    func(code int) // override for tests; defaults to os.Exit.
	threshold int32
}

func newRevocationDetector(logger *slog.Logger) *revocationDetector {
	if logger == nil {
		logger = slog.Default()
	}
	return &revocationDetector{
		logger:    logger,
		threshold: RevocationThreshold,
		onExit:    os.Exit,
	}
}

// observe records the outcome of one RPC. Must be called from the gRPC
// unary/stream interceptor chain.
func (d *revocationDetector) observe(err error) {
	if err == nil {
		d.consec.Store(0)
		return
	}
	st, ok := status.FromError(err)
	if !ok || st.Code() != codes.Unauthenticated {
		// Only Unauthenticated errors count toward the threshold. Other
		// transient errors (Unavailable, DeadlineExceeded) do not imply
		// revocation.
		return
	}
	if !isRevocationMessage(st.Message()) {
		return
	}
	n := d.consec.Add(1)
	d.logger.Warn("capabilitygrant: revocation signal observed",
		slog.Int("consecutive", int(n)),
		slog.Int("threshold", int(d.threshold)),
		slog.String("message", st.Message()),
	)
	if n >= d.threshold {
		d.trigger()
	}
}

func (d *revocationDetector) trigger() {
	d.once.Do(func() {
		d.logger.Error("capabilitygrant: agent revoked — terminating process",
			slog.Int("exit_code", RevocationExitCode),
		)
		d.onExit(RevocationExitCode)
	})
}

func isRevocationMessage(msg string) bool {
	m := strings.ToLower(msg)
	for _, ind := range revocationIndicators {
		if strings.Contains(m, ind) {
			return true
		}
	}
	return false
}

// RevocationUnaryInterceptor returns a gRPC UnaryClientInterceptor that
// observes every RPC outcome for revocation signals. After
// RevocationThreshold consecutive revoked responses, it calls os.Exit
// with RevocationExitCode.
//
// Wire this into your gRPC dial options alongside GRPCPerRPCCredentials:
//
//	conn, err := grpc.NewClient(platformURL,
//	    grpc.WithPerRPCCredentials(client.GRPCPerRPCCredentials()),
//	    grpc.WithChainUnaryInterceptor(client.RevocationUnaryInterceptor()),
//	    grpc.WithChainStreamInterceptor(client.RevocationStreamInterceptor()),
//	)
func (c *Client) RevocationUnaryInterceptor() grpc.UnaryClientInterceptor {
	d := c.revocation()
	return func(
		ctx context.Context, method string, req, reply any,
		cc *grpc.ClientConn, invoker grpc.UnaryInvoker, opts ...grpc.CallOption,
	) error {
		err := invoker(ctx, method, req, reply, cc, opts...)
		d.observe(err)
		return err
	}
}

// RevocationStreamInterceptor is the streaming counterpart. It observes
// the initial stream open error; subsequent stream errors should be
// surfaced by the application, which may re-dial and hit this path again.
func (c *Client) RevocationStreamInterceptor() grpc.StreamClientInterceptor {
	d := c.revocation()
	return func(
		ctx context.Context, desc *grpc.StreamDesc, cc *grpc.ClientConn,
		method string, streamer grpc.Streamer, opts ...grpc.CallOption,
	) (grpc.ClientStream, error) {
		cs, err := streamer(ctx, desc, cc, method, opts...)
		d.observe(err)
		return cs, err
	}
}

// revocation lazily initializes the per-client detector.
func (c *Client) revocation() *revocationDetector {
	c.revocationInit.Do(func() {
		c.revocationDet = newRevocationDetector(c.logger)
	})
	return c.revocationDet
}
