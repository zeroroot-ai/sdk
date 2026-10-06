// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Zero Root AI

package fork

import (
	"context"
	"errors"
	"fmt"
	"net"
	"sync"
	"testing"
	"time"

	"google.golang.org/genproto/googleapis/rpc/errdetails"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
	"google.golang.org/grpc/test/bufconn"
	"google.golang.org/protobuf/types/known/emptypb"
)

// host is a hostname that a test can change, as a fork changes it.
type host struct {
	mu   sync.Mutex
	name string
	err  error
}

func (h *host) set(name string) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.name = name
}

func (h *host) read() (string, error) {
	h.mu.Lock()
	defer h.mu.Unlock()
	return h.name, h.err
}

// useHost points the package hostname reader at h for one test.
func useHost(t *testing.T, h *host) {
	t.Helper()
	old := hostname
	hostname = h.read
	t.Cleanup(func() { hostname = old })
}

// claimer records each claim and answers with a fixed dispatch.
type claimer struct {
	mu    sync.Mutex
	calls []string
	err   error
}

func (c *claimer) ClaimFork(_ context.Context, sandboxID string) (*Claim, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.calls = append(c.calls, sandboxID)
	if c.err != nil {
		return nil, c.err
	}
	return &Claim{SandboxID: sandboxID, Grant: "fork-grant", NodeID: "node-b"}, nil
}

func (c *claimer) count() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	return len(c.calls)
}

func TestSandboxIDTrimsTheHostname(t *testing.T) {
	useHost(t, &host{name: "sbx-parent\n"})
	got, err := SandboxID()
	if err != nil || got != "sbx-parent" {
		t.Fatalf("SandboxID() = %q, %v; want sbx-parent", got, err)
	}
}

func TestPointInTheParentClaimsNothing(t *testing.T) {
	h := &host{name: "sbx-parent"}
	w, err := NewWatcherWith(h.read)
	if err != nil {
		t.Fatal(err)
	}
	c := &claimer{}
	claim, err := Point(context.Background(), w, c)
	if err != nil || claim != nil {
		t.Fatalf("Point in the parent = %v, %v; want nil, nil", claim, err)
	}
	if c.count() != 0 {
		t.Fatalf("the parent sent %d claims, want 0", c.count())
	}
}

func TestPointInAForkClaimsWithTheNewID(t *testing.T) {
	h := &host{name: "sbx-parent"}
	w, err := NewWatcherWith(h.read)
	if err != nil {
		t.Fatal(err)
	}
	h.set("sbx-fork-1")
	c := &claimer{}
	claim, err := Point(context.Background(), w, c)
	if err != nil {
		t.Fatalf("Point in a fork: %v", err)
	}
	if claim == nil || claim.SandboxID != "sbx-fork-1" || claim.Grant != "fork-grant" {
		t.Fatalf("Point in a fork = %+v; want the claim of sbx-fork-1", claim)
	}
}

func TestPointReportsAFailedClaim(t *testing.T) {
	h := &host{name: "sbx-parent"}
	w, _ := NewWatcherWith(h.read)
	h.set("sbx-fork-1")
	_, err := Point(context.Background(), w, &claimer{err: status.Error(codes.AlreadyExists, "claimed")})
	if status.Code(err) != codes.AlreadyExists {
		t.Fatalf("Point with a refused claim = %v; want the AlreadyExists of the daemon", err)
	}
}

func TestParkReturnsTheClaimAfterAFork(t *testing.T) {
	h := &host{name: "sbx-parent"}
	w, _ := NewWatcherWith(h.read)
	c := &claimer{}
	go func() {
		time.Sleep(30 * time.Millisecond)
		h.set("sbx-fork-2")
	}()
	claim, err := Park(context.Background(), w, c, ParkOptions{Timeout: 5 * time.Second, PollInterval: 5 * time.Millisecond})
	if err != nil || claim == nil || claim.SandboxID != "sbx-fork-2" {
		t.Fatalf("Park = %+v, %v; want the claim of sbx-fork-2", claim, err)
	}
}

func TestParkInTheParentEndsAtTheTimeout(t *testing.T) {
	h := &host{name: "sbx-parent"}
	w, _ := NewWatcherWith(h.read)
	c := &claimer{}
	start := time.Now()
	claim, err := Park(context.Background(), w, c, ParkOptions{Timeout: 40 * time.Millisecond, PollInterval: 5 * time.Millisecond})
	if err != nil || claim != nil {
		t.Fatalf("Park in the parent = %v, %v; want nil, nil", claim, err)
	}
	if time.Since(start) < 40*time.Millisecond {
		t.Fatalf("Park returned before the timeout")
	}
	if c.count() != 0 {
		t.Fatalf("the parent sent %d claims, want 0", c.count())
	}
}

func TestParkEndsWithTheContext(t *testing.T) {
	h := &host{name: "sbx-parent"}
	w, _ := NewWatcherWith(h.read)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := Park(ctx, w, &claimer{}, ParkOptions{Timeout: time.Minute, PollInterval: time.Millisecond}); !errors.Is(err, context.Canceled) {
		t.Fatalf("Park with a cancelled context = %v; want context.Canceled", err)
	}
}

func TestForkableAndParkTimeout(t *testing.T) {
	t.Setenv(EnvForkable, "")
	if Forkable() {
		t.Error("Forkable() is true with no env")
	}
	t.Setenv(EnvForkable, "1")
	if !Forkable() {
		t.Error("Forkable() is false with GIBSON_FORKABLE=1")
	}

	t.Setenv(EnvParkTimeout, "")
	if d, err := ParkTimeout(); err != nil || d != DefaultParkTimeout {
		t.Errorf("ParkTimeout() unset = %v, %v; want the default", d, err)
	}
	t.Setenv(EnvParkTimeout, "90s")
	if d, err := ParkTimeout(); err != nil || d != 90*time.Second {
		t.Errorf("ParkTimeout() = %v, %v; want 90s", d, err)
	}
	for _, bad := range []string{"soon", "0s", "-1m"} {
		t.Setenv(EnvParkTimeout, bad)
		if _, err := ParkTimeout(); err == nil {
			t.Errorf("ParkTimeout() accepted %q", bad)
		}
	}
}

func unclaimed(t *testing.T, code codes.Code, reason, domain string) error {
	t.Helper()
	st, err := status.New(code, "the grant belongs to another sandbox").WithDetails(&errdetails.ErrorInfo{Reason: reason, Domain: domain})
	if err != nil {
		t.Fatal(err)
	}
	return st.Err() //nolint:wrapcheck // the test builds the status error that the daemon sends
}

func TestIsForkUnclaimed(t *testing.T) {
	cases := []struct {
		name string
		err  error
		want bool
	}{
		{"the refusal", unclaimed(t, codes.FailedPrecondition, ReasonForkUnclaimed, ErrorDomain), true},
		{"the refusal, wrapped", fmt.Errorf("LLMComplete: %w", unclaimed(t, codes.FailedPrecondition, ReasonForkUnclaimed, ErrorDomain)), true},
		{"another reason", unclaimed(t, codes.FailedPrecondition, "OTHER", ErrorDomain), false},
		{"another domain", unclaimed(t, codes.FailedPrecondition, ReasonForkUnclaimed, "other.v1"), false},
		{"another code", unclaimed(t, codes.PermissionDenied, ReasonForkUnclaimed, ErrorDomain), false},
		{"no details", status.Error(codes.FailedPrecondition, "x"), false},
		{"not a status", errors.New("x"), false},
		{"nil", nil, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := IsForkUnclaimed(tc.err); got != tc.want {
				t.Fatalf("IsForkUnclaimed() = %v, want %v", got, tc.want)
			}
		})
	}
}

func TestErrForkedNamesTheFork(t *testing.T) {
	var err error = &ErrForked{Claim: &Claim{SandboxID: "sbx-fork-1", NodeID: "node-b"}}
	var ef *ErrForked
	if !errors.As(fmt.Errorf("CreateMission: %w", err), &ef) || ef.Claim.NodeID != "node-b" {
		t.Fatalf("errors.As did not find the claim in %v", err)
	}
	if (&ErrForked{}).Error() == "" || err.Error() == "" {
		t.Fatal("ErrForked has no message")
	}
}

// TestTheInterceptorsSendTheCurrentSandboxID proves that each call carries the
// hostname at the time of the call, so the first call of a fork carries the id
// of the fork.
func TestTheInterceptorsSendTheCurrentSandboxID(t *testing.T) {
	h := &host{name: "sbx-parent"}
	useHost(t, h)

	got := make(chan string, 4)
	record := func(ctx context.Context) {
		md, _ := metadata.FromIncomingContext(ctx)
		got <- fmt.Sprint(md.Get(MetadataSandboxID))
	}
	lis := bufconn.Listen(1 << 20)
	// Each method of the fixture is unknown to the server, so each call, unary
	// or stream, reaches this handler. It records the header and sends one
	// reply.
	srv := grpc.NewServer(grpc.UnknownServiceHandler(func(_ any, ss grpc.ServerStream) error {
		record(ss.Context())
		return ss.SendMsg(&emptypb.Empty{})
	}))
	go func() { _ = srv.Serve(lis) }()
	t.Cleanup(srv.Stop)

	conn, err := grpc.NewClient("passthrough:///bufnet",
		grpc.WithContextDialer(func(context.Context, string) (net.Conn, error) { return lis.Dial() }),
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithChainUnaryInterceptor(UnaryClientInterceptor()),
		grpc.WithChainStreamInterceptor(StreamClientInterceptor()),
	)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = conn.Close() })

	ctx := context.Background()
	call := func() {
		t.Helper()
		if err := conn.Invoke(ctx, "/fixture.v1.Service/Call", &emptypb.Empty{}, &emptypb.Empty{}); err != nil {
			t.Fatalf("unary call: %v", err)
		}
	}

	call()
	if v := <-got; v != "[sbx-parent]" {
		t.Fatalf("the parent sent %s, want [sbx-parent]", v)
	}

	h.set("sbx-fork-1")
	call()
	if v := <-got; v != "[sbx-fork-1]" {
		t.Fatalf("the fork sent %s, want [sbx-fork-1]", v)
	}

	stream, err := conn.NewStream(ctx, &grpc.StreamDesc{ServerStreams: true}, "/fixture.v1.Service/Stream")
	if err != nil {
		t.Fatalf("stream: %v", err)
	}
	_ = stream.CloseSend()
	if v := <-got; v != "[sbx-fork-1]" {
		t.Fatalf("the stream of the fork sent %s, want [sbx-fork-1]", v)
	}

	h.set("")
	call()
	if v := <-got; v != "[]" {
		t.Fatalf("a process with no hostname sent %s, want no id", v)
	}
}

// TestAnUnreadableHostname covers each path that reads the sandbox id when the
// hostname cannot be read.
func TestAnUnreadableHostname(t *testing.T) {
	broken := &host{err: errors.New("no hostname")}
	useHost(t, broken)

	if _, err := SandboxID(); err == nil {
		t.Error("SandboxID() with no hostname returned no error")
	}
	if _, err := NewWatcher(); err == nil {
		t.Error("NewWatcher() with no hostname returned no error")
	}

	h := &host{name: "sbx-parent"}
	w, err := NewWatcherWith(h.read)
	if err != nil {
		t.Fatal(err)
	}
	h.mu.Lock()
	h.err = errors.New("no hostname")
	h.mu.Unlock()
	if _, _, err := w.Forked(); err == nil {
		t.Error("Forked() with no hostname returned no error")
	}
	if _, err := Point(context.Background(), w, &claimer{}); err == nil {
		t.Error("Point() with no hostname returned no error")
	}
}

// TestParkTakesTheDefaults proves that the zero ParkOptions take the default
// timeout and poll interval: a fork found on the first check returns at once.
func TestParkTakesTheDefaults(t *testing.T) {
	h := &host{name: "sbx-parent"}
	w, _ := NewWatcherWith(h.read)
	h.set("sbx-fork-3")
	claim, err := Park(context.Background(), w, &claimer{}, ParkOptions{})
	if err != nil || claim == nil || claim.SandboxID != "sbx-fork-3" {
		t.Fatalf("Park with the defaults = %+v, %v; want the claim of sbx-fork-3", claim, err)
	}
}

func TestNewWatcherReadsTheHostname(t *testing.T) {
	useHost(t, &host{name: "sbx-parent"})
	w, err := NewWatcher()
	if err != nil || w.origin != "sbx-parent" {
		t.Fatalf("NewWatcher() = %v, %v; want the origin sbx-parent", w, err)
	}
}
