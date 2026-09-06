// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Zero Root AI

package spiffe

import (
	"context"
	"errors"
	"os"
	"testing"
)

func TestDialOptions_NoSocket_FallsBackToPlainTLS(t *testing.T) {
	t.Setenv(SocketEnv, "")
	opts, cleanup, err := DialOptions(context.Background())
	defer cleanup()
	if err != nil {
		t.Fatalf("expected no error in fallback, got %v", err)
	}
	if len(opts) == 0 {
		t.Fatal("expected dial options")
	}
}

func TestServerOptions_NoSocket_Errors(t *testing.T) {
	t.Setenv(SocketEnv, "")
	_, cleanup, err := ServerOptions(context.Background())
	defer cleanup()
	if !errors.Is(err, ErrWorkloadAPIUnavailable) {
		t.Fatalf("expected ErrWorkloadAPIUnavailable, got %v", err)
	}
}

func TestExpectPeerSPIFFEID_NoSocket_Errors(t *testing.T) {
	t.Setenv(SocketEnv, "")
	_, cleanup, err := ExpectPeerSPIFFEID(context.Background(), "spiffe://example.org/x")
	defer cleanup()
	if !errors.Is(err, ErrWorkloadAPIUnavailable) {
		t.Fatalf("expected ErrWorkloadAPIUnavailable, got %v", err)
	}
}

func TestExpectPeerSPIFFEID_BadID(t *testing.T) {
	t.Setenv(SocketEnv, "unix:///tmp/nonexistent.sock")
	// Even though the socket doesn't exist, the env var is set so we
	// reach the parse step — but the file-stat check will fail first.
	// Use a directory we know exists for the env so the parse step is
	// reached. tmpdir is created in test setup.
	dir := t.TempDir()
	sock := dir + "/agent.sock"
	f, err := os.Create(sock)
	if err != nil {
		t.Fatal(err)
	}
	_ = f.Close()
	t.Setenv(SocketEnv, "unix://"+sock)

	_, cleanup, err := ExpectPeerSPIFFEID(context.Background(), "not-a-spiffe-id")
	defer cleanup()
	if err == nil {
		t.Fatal("expected error on bad spiffe id")
	}
}

func TestWorkloadAPIAvailable_UnixMissing(t *testing.T) {
	t.Setenv(SocketEnv, "unix:///nonexistent/agent.sock")
	if workloadAPIAvailable() {
		t.Fatal("expected unavailable when socket file missing")
	}
}

func TestWorkloadAPIAvailable_UnixPresent(t *testing.T) {
	dir := t.TempDir()
	sock := dir + "/agent.sock"
	f, err := os.Create(sock)
	if err != nil {
		t.Fatal(err)
	}
	_ = f.Close()
	t.Setenv(SocketEnv, "unix://"+sock)
	if !workloadAPIAvailable() {
		t.Fatal("expected available when socket file exists")
	}
}

func TestWorkloadAPIAvailable_NonUnixSchemeAccepted(t *testing.T) {
	t.Setenv(SocketEnv, "tcp://127.0.0.1:9999")
	if !workloadAPIAvailable() {
		t.Fatal("expected available for non-unix scheme (open will surface real reachability)")
	}
}
