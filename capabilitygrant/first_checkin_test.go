// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Zero Root AI

package capabilitygrant

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/stretchr/testify/require"
)

// authRecorder is a registration endpoint that records the Authorization
// header of each request.
type authRecorder struct {
	mu    sync.Mutex
	auths []string
}

func (a *authRecorder) seen() []string {
	a.mu.Lock()
	defer a.mu.Unlock()
	return append([]string(nil), a.auths...)
}

func serveRegistration(t *testing.T) (*httptest.Server, *authRecorder) {
	t.Helper()
	rec := &authRecorder{}
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		rec.mu.Lock()
		rec.auths = append(rec.auths, r.Header.Get("Authorization"))
		rec.mu.Unlock()
		w.WriteHeader(http.StatusCreated)
		_ = json.NewEncoder(w).Encode(registrationResponse{AgentID: "agent-1", ComponentScope: "component:agent-1"})
	}))
	t.Cleanup(srv.Close)
	return srv, rec
}

func registeringClient(t *testing.T, srv *httptest.Server, keyPath, bootstrap string) *Client {
	t.Helper()
	c, err := NewClient(ClientConfig{
		PlatformURL:    srv.URL,
		BootstrapToken: bootstrap,
		HostKeyPath:    keyPath,
		AgentName:      "agent",
		AgentMode:      "autonomous",
	})
	require.NoError(t, err)
	c.httpClient = srv.Client()
	c.discovery = &DiscoveryDocument{}
	c.discovery.Endpoints.Register = srv.URL + "/agent-auth/register"
	return c
}

// TestTheBootstrapTokenGoesOnlyWithTheFirstCheckIn proves that the one-time
// bootstrap token is sent once, with the check-in that generated the host key.
// A later registration in the same process, and a later process that loads
// the key from disk, sign a host JWT even when the token is still configured.
func TestTheBootstrapTokenGoesOnlyWithTheFirstCheckIn(t *testing.T) {
	t.Setenv("GIBSON_BOOTSTRAP_TOKEN", "")
	srv, rec := serveRegistration(t)
	keyPath := filepath.Join(t.TempDir(), "host_key.json")

	first := registeringClient(t, srv, keyPath, "one-time-token")
	require.True(t, first.hostKey.FirstCheckIn)
	require.NoError(t, first.Register(context.Background()))
	require.False(t, first.hostKey.FirstCheckIn, "a registered host is no longer new")
	require.NoError(t, first.Register(context.Background()))

	restarted := registeringClient(t, srv, keyPath, "one-time-token")
	require.False(t, restarted.hostKey.FirstCheckIn, "a key loaded from disk is not a first check-in")
	require.NoError(t, restarted.Register(context.Background()))

	auths := rec.seen()
	require.Len(t, auths, 3)
	require.Equal(t, "Bearer one-time-token", auths[0])
	for i, a := range auths[1:] {
		require.NotContains(t, a, "one-time-token", "registration %d sent the spent bootstrap token", i+2)
		require.Len(t, strings.Split(strings.TrimPrefix(a, "Bearer "), "."), 3, "registration %d sends a host JWT", i+2)
	}
}

// TestAFirstCheckInNeedsABootstrapCredential proves that a new host with no
// bootstrap credential is refused before any request. A host-signed first
// check-in would trust whoever calls first.
func TestAFirstCheckInNeedsABootstrapCredential(t *testing.T) {
	t.Setenv("GIBSON_BOOTSTRAP_TOKEN", "")
	if _, err := ResolveBootstrap(""); err == nil {
		t.Skip("this host has an in-cluster service account token")
	}
	srv, rec := serveRegistration(t)
	c := registeringClient(t, srv, filepath.Join(t.TempDir(), "host_key.json"), "")
	err := c.Register(context.Background())
	require.ErrorContains(t, err, "never checked in")
	require.Empty(t, rec.seen())
}

// TestAWideHostKeyFileIsRefused proves that a host key file that the group or
// other users can read is refused on reuse.
func TestAWideHostKeyFileIsRefused(t *testing.T) {
	path := filepath.Join(t.TempDir(), "host_key.json")
	_, err := LoadOrGenerateHostKey(path)
	require.NoError(t, err)
	for _, mode := range []os.FileMode{0o644, 0o640, 0o604, 0o660} {
		require.NoError(t, os.Chmod(path, mode))
		_, err := LoadOrGenerateHostKey(path)
		require.ErrorContains(t, err, "only its owner may read it", "mode %04o", mode)
	}
	require.NoError(t, os.Chmod(path, 0o600))
	key, err := LoadOrGenerateHostKey(path)
	require.NoError(t, err)
	require.False(t, key.FirstCheckIn)
}
