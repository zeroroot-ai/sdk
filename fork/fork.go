// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Zero Root AI

// Package fork is the runtime fork contract of the sdk (D74, sdk#248).
//
// A fork from a snapshot continues the process of its parent. It starts with
// the grant, the callback identity and the task of the parent in memory. This
// package lets the process find out that it is a fork, and claim its own
// dispatch from the daemon before it does anything else.
//
// The contract has five parts:
//
//   - The sandbox identity travels with each callback. The callback client
//     gets a new identity token from setec on each call (setec#235) and sends
//     it in MetadataSandboxIdentity. setec signs the token with a key of the
//     sandbox that no process in the sandbox can read. Each snapshot raises
//     the identity generation, so a token of the parent does not verify in a
//     fork. The daemon takes the sandbox of the caller only from this token.
//   - The client also sends MetadataSandboxID, the hostname of the process.
//     It is a hint, not a proof. The daemon refuses a call whose hostname
//     names another sandbox than the token.
//   - The daemon refuses the grant of a forked source outside the source
//     sandbox. The refusal is FAILED_PRECONDITION with the reason
//     ReasonForkUnclaimed. IsForkUnclaimed recognizes it.
//   - A fork claims its dispatch with HarnessCallbackService.ClaimFork. The
//     claim holds the new grant, the ids, the node id, the model and the task.
//   - A source that may be forked (EnvForkable) parks after its result line
//     (Park). An agent that forks its current state gets ErrForked in the fork
//     (Point).
package fork

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	"google.golang.org/genproto/googleapis/rpc/errdetails"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"

	typespb "github.com/zeroroot-ai/sdk/api/gen/gibson/types/v1"
)

const (
	// EnvForkable is set to "1" by the daemon at the launch of a node that a
	// later node names in starts_from. Such a process parks after its result
	// line instead of exiting.
	EnvForkable = "GIBSON_FORKABLE"

	// EnvParkTimeout bounds the park of a source, as a Go duration ("10m").
	// Unset or empty means DefaultParkTimeout.
	EnvParkTimeout = "GIBSON_PARK_TIMEOUT"

	// MetadataSandboxID is the metadata key that carries the hostname of
	// the caller on each callback.
	MetadataSandboxID = "x-gibson-sandbox-id"

	// MetadataSandboxIdentity is the metadata key that carries the setec
	// identity token of the caller on each callback.
	MetadataSandboxIdentity = "x-gibson-sandbox-identity"

	// SandboxIdentityAudience is the audience of the identity token that a
	// callback sends. The daemon verifies the token for this audience.
	SandboxIdentityAudience = "gibson-harness-callback"

	// EnvIdentitySocket names the Unix socket of setec that gives the
	// identity tokens of the sandbox. setec sets it in the environment of
	// each process in a sandbox.
	EnvIdentitySocket = "SETEC_IDENTITY_SOCKET"

	// identityTokenPath is the HTTP path of a token on the identity socket.
	identityTokenPath = "/v1/token"

	// identityTimeout bounds one request for a token.
	identityTimeout = 5 * time.Second

	// ReasonForkUnclaimed is the ErrorInfo reason of the refusal of a grant
	// outside the sandbox it was given to.
	ReasonForkUnclaimed = "GIBSON_FORK_UNCLAIMED"

	// ErrorDomain is the ErrorInfo domain of ReasonForkUnclaimed.
	ErrorDomain = "gibson.harness.v1"

	// DefaultParkTimeout is the park bound when EnvParkTimeout is unset.
	DefaultParkTimeout = 10 * time.Minute

	// DefaultPollInterval is how often a parked process checks its hostname.
	DefaultPollInterval = 500 * time.Millisecond
)

// Claim is the dispatch of a fork, as ClaimFork returns it.
type Claim struct {
	// SandboxID is the id that the fork claimed with.
	SandboxID string
	// Grant is the capability grant of the fork. Use it for each later call.
	Grant string
	// MissionID, MissionRunID and AgentRunID scope the callbacks of the fork.
	MissionID    string
	MissionRunID string
	AgentRunID   string
	// NodeID is the mission node that the fork runs.
	NodeID string
	// Model is the model resolved for this dispatch.
	Model string
	// Task is the task that the fork runs.
	Task *typespb.Task
}

// Claimer asks the daemon for the dispatch of a fork. The callback client of
// package serve implements it.
type Claimer interface {
	ClaimFork(ctx context.Context, sandboxID string) (*Claim, error)
}

// ErrForked is the error that a call returns in a fork when the agent forked
// its current state (ORIGINATION_START_CALLER_STATE). The parent gets the
// normal result of the call. Read the claim with errors.As and run its task.
type ErrForked struct {
	Claim *Claim
}

func (e *ErrForked) Error() string {
	if e.Claim == nil {
		return "fork: this process is a fork"
	}
	return fmt.Sprintf("fork: this process is the fork %s and runs node %q", e.Claim.SandboxID, e.Claim.NodeID)
}

// hostname reads the sandbox id. It is a variable so tests can change it.
var hostname = os.Hostname

// SandboxID returns the sandbox id of the process: its hostname, read now.
func SandboxID() (string, error) {
	h, err := hostname()
	if err != nil {
		return "", fmt.Errorf("fork: read the hostname: %w", err)
	}
	return strings.TrimSpace(h), nil
}

// Watcher compares the sandbox id of the process with the id at the time the
// watcher was made. Make it before a fork can happen, at process start. A fork
// copies the memory of its parent, so its watcher holds the id of the parent.
type Watcher struct {
	origin string
	read   func() (string, error)
}

// NewWatcher records the sandbox id of the process.
func NewWatcher() (*Watcher, error) {
	return NewWatcherWith(SandboxID)
}

// NewWatcherWith records the sandbox id that read returns, and reads the id
// with it later. Use it for a runtime that reads the sandbox id in another
// way, and in tests.
func NewWatcherWith(read func() (string, error)) (*Watcher, error) {
	id, err := read()
	if err != nil {
		return nil, err
	}
	return &Watcher{origin: id, read: read}, nil
}

// Forked reports whether the process now runs in another sandbox than the one
// the watcher was made in. It returns the current sandbox id.
func (w *Watcher) Forked() (sandboxID string, forked bool, err error) {
	sandboxID, err = w.read()
	if err != nil {
		return "", false, fmt.Errorf("fork: read the sandbox id: %w", err)
	}
	return sandboxID, sandboxID != w.origin, nil
}

// Point checks whether the process is a fork. In the parent it returns a nil
// claim. In a fork it claims the dispatch and returns it.
func Point(ctx context.Context, w *Watcher, c Claimer) (*Claim, error) {
	id, forked, err := w.Forked()
	if err != nil || !forked {
		return nil, err
	}
	claim, err := c.ClaimFork(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("fork: claim the dispatch of fork %s: %w", id, err)
	}
	return claim, nil
}

// ParkOptions bounds a park. The zero value takes the defaults.
type ParkOptions struct {
	// Timeout ends the park. Zero means DefaultParkTimeout.
	Timeout time.Duration
	// PollInterval is how often the hostname is checked. Zero means
	// DefaultPollInterval.
	PollInterval time.Duration
}

// Park waits until the process is a fork, then claims and returns the
// dispatch. It returns a nil claim and a nil error when the timeout ends with
// no fork: the process is the parent, and it can exit with status 0. It
// returns the context error when ctx ends first.
func Park(ctx context.Context, w *Watcher, c Claimer, opts ParkOptions) (*Claim, error) {
	if opts.Timeout <= 0 {
		opts.Timeout = DefaultParkTimeout
	}
	if opts.PollInterval <= 0 {
		opts.PollInterval = DefaultPollInterval
	}
	deadline := time.NewTimer(opts.Timeout)
	defer deadline.Stop()
	tick := time.NewTicker(opts.PollInterval)
	defer tick.Stop()
	for {
		claim, err := Point(ctx, w, c)
		if err != nil || claim != nil {
			return claim, err
		}
		select {
		case <-ctx.Done():
			return nil, fmt.Errorf("fork: park: %w", ctx.Err())
		case <-deadline.C:
			return nil, nil
		case <-tick.C:
		}
	}
}

// Forkable reports whether the daemon launched this process as a source that
// may be forked.
func Forkable() bool {
	return os.Getenv(EnvForkable) == "1"
}

// ParkTimeout returns the park bound from EnvParkTimeout.
func ParkTimeout() (time.Duration, error) {
	v := strings.TrimSpace(os.Getenv(EnvParkTimeout))
	if v == "" {
		return DefaultParkTimeout, nil
	}
	d, err := time.ParseDuration(v)
	if err != nil {
		return 0, fmt.Errorf("fork: %s=%q: %w", EnvParkTimeout, v, err)
	}
	if d <= 0 {
		return 0, fmt.Errorf("fork: %s=%q: the timeout must be positive", EnvParkTimeout, v)
	}
	return d, nil
}

// IsForkUnclaimed reports whether err is the refusal of a grant outside the
// sandbox it was given to.
func IsForkUnclaimed(err error) bool {
	st, ok := status.FromError(err)
	if !ok {
		var se interface{ GRPCStatus() *status.Status }
		if !errors.As(err, &se) {
			return false
		}
		st = se.GRPCStatus()
	}
	if st.Code() != codes.FailedPrecondition {
		return false
	}
	for _, d := range st.Details() {
		if info, ok := d.(*errdetails.ErrorInfo); ok && info.GetReason() == ReasonForkUnclaimed && info.GetDomain() == ErrorDomain {
			return true
		}
	}
	return false
}

// ErrNoSandboxIdentity is the error of a process that has no identity socket:
// EnvIdentitySocket is not set. Such a process does not run in a setec
// sandbox, and it cannot prove which sandbox it is.
var ErrNoSandboxIdentity = errors.New("fork: this process has no sandbox identity: " + EnvIdentitySocket + " is not set")

// IdentitySocket returns the path of the setec identity socket from
// EnvIdentitySocket, or "" when the process has none.
func IdentitySocket() string {
	return strings.TrimSpace(os.Getenv(EnvIdentitySocket))
}

// IdentityToken gets a new identity token of the sandbox of the process from
// the setec identity socket, for SandboxIdentityAudience. It reads
// EnvIdentitySocket and asks the socket on each call, and it keeps no copy.
// So a fork, which gets a new identity, never sends the token of its parent.
// It returns ErrNoSandboxIdentity when EnvIdentitySocket is not set.
func IdentityToken(ctx context.Context) (string, error) {
	socket := IdentitySocket()
	if socket == "" {
		return "", ErrNoSandboxIdentity
	}
	ctx, cancel := context.WithTimeout(ctx, identityTimeout)
	defer cancel()
	client := &http.Client{Transport: &http.Transport{
		DialContext: func(ctx context.Context, _, _ string) (net.Conn, error) {
			return (&net.Dialer{}).DialContext(ctx, "unix", socket)
		},
	}}
	defer client.CloseIdleConnections()
	u := "http://setec-identity" + identityTokenPath + "?" + url.Values{"audience": {SandboxIdentityAudience}}.Encode()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, http.NoBody)
	if err != nil {
		return "", fmt.Errorf("fork: make the identity token request: %w", err)
	}
	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("fork: get the sandbox identity token from %s: %w", socket, err)
	}
	defer func() { _ = resp.Body.Close() }()
	var body struct {
		Token string `json:"token"`
		Error string `json:"error"`
	}
	if err := json.NewDecoder(io.LimitReader(resp.Body, 16<<10)).Decode(&body); err != nil {
		return "", fmt.Errorf("fork: the identity socket %s answered %s with no JSON: %w", socket, resp.Status, err)
	}
	switch {
	case resp.StatusCode != http.StatusOK:
		return "", fmt.Errorf("fork: the identity socket %s refused the token: %s: %s", socket, resp.Status, body.Error)
	case body.Token == "":
		return "", fmt.Errorf("fork: the identity socket %s answered no token", socket)
	}
	return body.Token, nil
}

// withSandboxIdentity adds the identity token and the hostname of the
// process to the outgoing metadata. A process with no identity socket sends
// no token, and the daemon decides. A process with a socket that gives no
// token sends no call: the daemon would refuse it.
func withSandboxIdentity(ctx context.Context) (context.Context, error) {
	token, err := IdentityToken(ctx)
	switch {
	case errors.Is(err, ErrNoSandboxIdentity):
	case err != nil:
		return ctx, status.Error(codes.Unauthenticated, err.Error())
	default:
		ctx = metadata.AppendToOutgoingContext(ctx, MetadataSandboxIdentity, token)
	}
	if id, err := SandboxID(); err == nil && id != "" {
		ctx = metadata.AppendToOutgoingContext(ctx, MetadataSandboxID, id)
	}
	return ctx, nil
}

// UnaryClientInterceptor sends MetadataSandboxIdentity and MetadataSandboxID
// on each unary call.
func UnaryClientInterceptor() grpc.UnaryClientInterceptor {
	return func(ctx context.Context, method string, req, reply any, cc *grpc.ClientConn, invoker grpc.UnaryInvoker, opts ...grpc.CallOption) error {
		ctx, err := withSandboxIdentity(ctx)
		if err != nil {
			return err
		}
		return invoker(ctx, method, req, reply, cc, opts...)
	}
}

// StreamClientInterceptor sends MetadataSandboxIdentity and MetadataSandboxID
// on each stream.
func StreamClientInterceptor() grpc.StreamClientInterceptor {
	return func(ctx context.Context, desc *grpc.StreamDesc, cc *grpc.ClientConn, method string, streamer grpc.Streamer, opts ...grpc.CallOption) (grpc.ClientStream, error) {
		ctx, err := withSandboxIdentity(ctx)
		if err != nil {
			return nil, err
		}
		return streamer(ctx, desc, cc, method, opts...)
	}
}
