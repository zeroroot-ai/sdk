// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Zero Root AI

package capabilitygrant

import (
	"path/filepath"
	"testing"
	"time"
)

// A caller that hands a bootstrap token enrolls with it, even in a pod with a
// SPIRE socket. Before, NewClient built a JWT-SVID source whenever
// SPIFFE_ENDPOINT_SOCKET was set, and Register presented the pod's own SVID,
// which the daemon refuses for a pod that is not a plugin (gibson
// e2e-cluster run 38246956280). The socket here does not exist: NewClient
// must not wait on it, and the client must hold no SVID source.
func TestNewClient_ExplicitBootstrapTokenSkipsTheSVIDSource(t *testing.T) {
	t.Setenv("SPIFFE_ENDPOINT_SOCKET", "unix://"+filepath.Join(t.TempDir(), "no-agent.sock"))
	t.Setenv("GIBSON_BOOTSTRAP_TOKEN", "")

	type result struct {
		c   *Client
		err error
	}
	done := make(chan result, 1)
	go func() {
		c, err := NewClient(ClientConfig{
			PlatformURL:    "https://platform.example.com",
			BootstrapToken: "boot-1",
			HostKeyPath:    filepath.Join(t.TempDir(), "keys", "host_key.json"),
			AgentName:      "agent-a",
		})
		done <- result{c, err}
	}()

	select {
	case r := <-done:
		if r.err != nil {
			t.Fatalf("NewClient: %v", r.err)
		}
		if r.c.svidSource != nil {
			t.Fatal("the client holds a JWT-SVID source although the caller gave a bootstrap token")
		}
	case <-time.After(5 * time.Second):
		t.Fatal("NewClient waited on the SPIRE socket although the caller gave a bootstrap token")
	}
}

// Both orders of the enrollment choice: a socket with no bootstrap token
// enrolls with the SVID, a bootstrap token wins over the socket, and no
// socket never enrolls with an SVID.
func TestEnrollWithSVID(t *testing.T) {
	for _, c := range []struct {
		name, socket, token string
		want                bool
	}{
		{"socket, no bootstrap token: the SVID", "unix:///run/spire/agent.sock", "", true},
		{"socket and a bootstrap token: the bootstrap token", "unix:///run/spire/agent.sock", "boot-1", false},
		{"no socket, a bootstrap token: the bootstrap token", "", "boot-1", false},
		{"no socket, no bootstrap token: no SVID", "", "", false},
	} {
		if got := enrollWithSVID(c.socket, c.token); got != c.want {
			t.Errorf("%s: enrollWithSVID = %v, want %v", c.name, got, c.want)
		}
	}
}
