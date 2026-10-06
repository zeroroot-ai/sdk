// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Zero Root AI

// Package fork is the runtime fork contract of the sdk (D74, sdk#248).
//
// A fork from a snapshot continues the process of its parent. It starts with
// the grant, the callback identity and the task of the parent in memory. This
// package lets the process find out that it is a fork, and claim its own
// dispatch from the daemon before it does anything else.
//
// The contract has four parts:
//
//   - The sandbox id travels with each callback. The callback client sends
//     MetadataSandboxID on each call, read at the time of the call. The value
//     is the hostname of the process: setec gives each sandbox a hostname from
//     its name, so a fork reads its own id.
//   - The daemon refuses the grant of a forked source outside the source
//     sandbox. The refusal is FAILED_PRECONDITION with the reason
//     ReasonForkUnclaimed. IsForkUnclaimed recognizes it.
//   - A fork claims its dispatch with HarnessCallbackService.ClaimFork. The
//     claim holds the new grant, the ids, the node id, the model and the task.
//   - A source that may be forked (EnvForkable) parks after its result line
//     (Park). An agent that forks its current state gets ErrForked in the fork
//     (Point).
//
// The daemon trusts the hostname that the process sends. A fork that sends the
// id of its parent acts as the parent. The contract stops a fork from using the
// parent credential by accident. It does not stop agent code that lies on
// purpose; a per-sandbox identity from setec closes that gap.
package fork

import (
	"context"
	"errors"
	"fmt"
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

	// MetadataSandboxID is the metadata key that carries the sandbox id of
	// the caller on each callback.
	MetadataSandboxID = "x-gibson-sandbox-id"

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

// Origin is the sandbox id at the time the watcher was made.
func (w *Watcher) Origin() string { return w.origin }

// Forked reports whether the process now runs in another sandbox than the one
// the watcher was made in. It returns the current sandbox id.
func (w *Watcher) Forked() (string, bool, error) {
	id, err := w.read()
	if err != nil {
		return "", false, err
	}
	return id, id != w.origin, nil
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
			return nil, ctx.Err()
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

// withSandboxID adds the sandbox id of the process to the outgoing metadata.
// A process with no readable hostname sends no id, and the daemon decides.
func withSandboxID(ctx context.Context) context.Context {
	id, err := SandboxID()
	if err != nil || id == "" {
		return ctx
	}
	return metadata.AppendToOutgoingContext(ctx, MetadataSandboxID, id)
}

// UnaryClientInterceptor sends MetadataSandboxID on each unary call.
func UnaryClientInterceptor() grpc.UnaryClientInterceptor {
	return func(ctx context.Context, method string, req, reply any, cc *grpc.ClientConn, invoker grpc.UnaryInvoker, opts ...grpc.CallOption) error {
		return invoker(withSandboxID(ctx), method, req, reply, cc, opts...)
	}
}

// StreamClientInterceptor sends MetadataSandboxID on each stream.
func StreamClientInterceptor() grpc.StreamClientInterceptor {
	return func(ctx context.Context, desc *grpc.StreamDesc, cc *grpc.ClientConn, method string, streamer grpc.Streamer, opts ...grpc.CallOption) (grpc.ClientStream, error) {
		return streamer(withSandboxID(ctx), desc, cc, method, opts...)
	}
}
