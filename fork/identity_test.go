// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Zero Root AI

package fork

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
	"google.golang.org/grpc/test/bufconn"
	"google.golang.org/protobuf/types/known/emptypb"
)

// identitySocket is a fake setec identity socket. Each request gets the
// token of the current generation and a request number, as setec signs a new
// token with a new jti for each request. A snapshot raises the generation.
type identitySocket struct {
	mu         sync.Mutex
	generation int
	requests   int
	audiences  []string
	refuse     string
}

func (s *identitySocket) snapshot() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.generation++
}

func (s *identitySocket) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.requests++
	s.audiences = append(s.audiences, r.URL.Query().Get("audience"))
	w.Header().Set("Content-Type", "application/json")
	if r.Method != http.MethodGet || r.URL.Path != identityTokenPath {
		w.WriteHeader(http.StatusNotFound)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": "not found"})
		return
	}
	if s.refuse != "" {
		w.WriteHeader(http.StatusBadGateway)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": s.refuse})
		return
	}
	_ = json.NewEncoder(w).Encode(map[string]any{
		"token":   fmt.Sprintf("gen%d-req%d", s.generation, s.requests),
		"expires": 1,
	})
}

// serveIdentitySocket serves s on a new Unix socket and names it in
// EnvIdentitySocket for one test.
func serveIdentitySocket(t *testing.T, s http.Handler) string {
	t.Helper()
	// A Unix socket path has a short limit, so the directory is short.
	dir, err := os.MkdirTemp("", "id")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(dir) })
	path := filepath.Join(dir, "identity.sock")
	lis, err := (&net.ListenConfig{}).Listen(context.Background(), "unix", path)
	if err != nil {
		t.Fatal(err)
	}
	srv := &http.Server{Handler: s} //nolint:gosec // a test socket
	go func() { _ = srv.Serve(lis) }()
	t.Cleanup(func() { _ = srv.Close() })
	t.Setenv(EnvIdentitySocket, path)
	return path
}

// identityConn returns a client connection with the interceptors of the
// contract to a server that records the identity metadata of each call.
func identityConn(t *testing.T) (*grpc.ClientConn, <-chan metadata.MD) {
	t.Helper()
	got := make(chan metadata.MD, 8)
	lis := bufconn.Listen(1 << 20)
	srv := grpc.NewServer(grpc.UnknownServiceHandler(func(_ any, ss grpc.ServerStream) error {
		md, _ := metadata.FromIncomingContext(ss.Context())
		got <- md
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
	return conn, got
}

func invoke(conn *grpc.ClientConn) error {
	return conn.Invoke(context.Background(), "/fixture.v1.Service/Call", &emptypb.Empty{}, &emptypb.Empty{}) //nolint:wrapcheck // the test reads the status
}

// TestTheInterceptorsSendTheIdentityToken proves that each call, unary or
// stream, carries a new token for the audience of the daemon, next to the
// hostname.
func TestTheInterceptorsSendTheIdentityToken(t *testing.T) {
	useHost(t, &host{name: "sbx-parent"})
	sock := &identitySocket{}
	serveIdentitySocket(t, sock)
	conn, got := identityConn(t)

	for i, want := range []string{"gen0-req1", "gen0-req2"} {
		if err := invoke(conn); err != nil {
			t.Fatalf("call %d: %v", i, err)
		}
		md := <-got
		if v := md.Get(MetadataSandboxIdentity); len(v) != 1 || v[0] != want {
			t.Fatalf("call %d sent the token %v, want [%s]", i, v, want)
		}
		if v := md.Get(MetadataSandboxID); len(v) != 1 || v[0] != "sbx-parent" {
			t.Fatalf("call %d sent the sandbox id %v, want [sbx-parent]", i, v)
		}
	}

	stream, err := conn.NewStream(context.Background(), &grpc.StreamDesc{ServerStreams: true}, "/fixture.v1.Service/Stream")
	if err != nil {
		t.Fatalf("stream: %v", err)
	}
	_ = stream.CloseSend()
	if v := (<-got).Get(MetadataSandboxIdentity); len(v) != 1 || v[0] != "gen0-req3" {
		t.Fatalf("the stream sent the token %v, want [gen0-req3]", v)
	}

	sock.mu.Lock()
	defer sock.mu.Unlock()
	for _, a := range sock.audiences {
		if a != SandboxIdentityAudience {
			t.Fatalf("a token was asked for the audience %q, want %q", a, SandboxIdentityAudience)
		}
	}
}

// TestAForkSendsItsOwnToken proves that the client keeps no token: after a
// snapshot the next call carries the token of the new generation, never a
// token of the parent.
func TestAForkSendsItsOwnToken(t *testing.T) {
	h := &host{name: "sbx-parent"}
	useHost(t, h)
	sock := &identitySocket{}
	serveIdentitySocket(t, sock)
	conn, got := identityConn(t)

	if err := invoke(conn); err != nil {
		t.Fatal(err)
	}
	if v := (<-got).Get(MetadataSandboxIdentity); len(v) != 1 || v[0] != "gen0-req1" {
		t.Fatalf("the parent sent %v, want [gen0-req1]", v)
	}

	sock.snapshot()
	h.set("sbx-fork-1")
	if err := invoke(conn); err != nil {
		t.Fatal(err)
	}
	md := <-got
	if v := md.Get(MetadataSandboxIdentity); len(v) != 1 || v[0] != "gen1-req2" {
		t.Fatalf("the fork sent %v, want [gen1-req2]", v)
	}
	if v := md.Get(MetadataSandboxID); len(v) != 1 || v[0] != "sbx-fork-1" {
		t.Fatalf("the fork sent the sandbox id %v, want [sbx-fork-1]", v)
	}
}

// TestAMissingTokenIsAClearError covers a process with no socket, a socket
// that refuses, and a socket that is gone.
func TestAMissingTokenIsAClearError(t *testing.T) {
	useHost(t, &host{name: "sbx-parent"})

	t.Run("no socket sends no token", func(t *testing.T) {
		t.Setenv(EnvIdentitySocket, "")
		if _, err := IdentityToken(context.Background()); !errors.Is(err, ErrNoSandboxIdentity) {
			t.Fatalf("IdentityToken() = %v, want ErrNoSandboxIdentity", err)
		}
		conn, got := identityConn(t)
		if err := invoke(conn); err != nil {
			t.Fatal(err)
		}
		md := <-got
		if v := md.Get(MetadataSandboxIdentity); len(v) != 0 {
			t.Fatalf("a process with no socket sent the token %v", v)
		}
		if v := md.Get(MetadataSandboxID); len(v) != 1 || v[0] != "sbx-parent" {
			t.Fatalf("a process with no socket sent the sandbox id %v", v)
		}
	})

	t.Run("a refusal stops the call", func(t *testing.T) {
		serveIdentitySocket(t, &identitySocket{refuse: "reach the launcher: no route"})
		conn, got := identityConn(t)
		err := invoke(conn)
		if status.Code(err) != codes.Unauthenticated {
			t.Fatalf("call = %v, want Unauthenticated", err)
		}
		if msg := status.Convert(err).Message(); !strings.Contains(msg, "reach the launcher: no route") {
			t.Fatalf("the error %q does not name the cause", msg)
		}
		_, serr := conn.NewStream(context.Background(), &grpc.StreamDesc{ServerStreams: true}, "/fixture.v1.Service/Stream")
		if status.Code(serr) != codes.Unauthenticated {
			t.Fatalf("stream = %v, want Unauthenticated", serr)
		}
		select {
		case md := <-got:
			t.Fatalf("a call with no token reached the server: %v", md)
		default:
		}
	})

	t.Run("a socket that is gone", func(t *testing.T) {
		t.Setenv(EnvIdentitySocket, filepath.Join(t.TempDir(), "gone.sock"))
		_, err := IdentityToken(context.Background())
		if err == nil || !strings.Contains(err.Error(), "gone.sock") {
			t.Fatalf("IdentityToken() = %v, want an error that names the socket", err)
		}
	})

	t.Run("an answer with no token", func(t *testing.T) {
		serveIdentitySocket(t, http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			_, _ = w.Write([]byte(`{}`))
		}))
		if _, err := IdentityToken(context.Background()); err == nil || !strings.Contains(err.Error(), "no token") {
			t.Fatalf("IdentityToken() = %v, want no token", err)
		}
	})

	t.Run("an answer with no JSON", func(t *testing.T) {
		serveIdentitySocket(t, http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			_, _ = w.Write([]byte(`not json`))
		}))
		if _, err := IdentityToken(context.Background()); err == nil || !strings.Contains(err.Error(), "no JSON") {
			t.Fatalf("IdentityToken() = %v, want no JSON", err)
		}
	})
}
