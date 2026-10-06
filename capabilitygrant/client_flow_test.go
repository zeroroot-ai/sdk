// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Zero Root AI

package capabilitygrant

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
)

// fakePlatform serves the discovery document and the register endpoint.
func fakePlatform(t *testing.T, registerStatus int, registerBody string) (srv *httptest.Server, gotAuth *string) {
	t.Helper()
	gotAuth = new(string)
	srv = httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case wellKnownPath:
			_ = json.NewEncoder(w).Encode(map[string]any{
				"protocol_version": "1.0",
				"endpoints":        map[string]string{"register": srv.URL + "/register"},
			})
		case "/register":
			*gotAuth = r.Header.Get("Authorization")
			body, _ := io.ReadAll(r.Body)
			var req registrationRequest
			if err := json.Unmarshal(body, &req); err != nil || req.AgentName == "" {
				w.WriteHeader(http.StatusBadRequest)
				return
			}
			w.WriteHeader(registerStatus)
			_, _ = io.WriteString(w, registerBody)
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(srv.Close)
	return srv, gotAuth
}

func newFlowClient(t *testing.T, platformURL string) *Client {
	t.Helper()
	t.Setenv("SPIFFE_ENDPOINT_SOCKET", "")
	t.Setenv("GIBSON_BOOTSTRAP_TOKEN", "")
	c, err := NewClient(ClientConfig{
		PlatformURL:    platformURL,
		BootstrapToken: "boot-1",
		HostKeyPath:    filepath.Join(t.TempDir(), "keys", "host_key.json"),
		AgentName:      "agent-a",
	})
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}
	return c
}

func TestClientDiscoverAndRegister(t *testing.T) {
	srv, gotAuth := fakePlatform(t, http.StatusCreated, `{"agent_id":"ag-1","component_scope":"component:agent/a"}`)
	c := newFlowClient(t, srv.URL)
	c.SetHTTPClient(nil) // ignored
	c.SetHTTPClient(srv.Client())
	ctx := context.Background()

	if err := c.Register(ctx); err == nil {
		t.Fatal("Register before Discover must fail")
	}
	if err := c.Discover(ctx); err != nil {
		t.Fatalf("Discover: %v", err)
	}
	if err := c.Register(ctx); err != nil {
		t.Fatalf("Register: %v", err)
	}
	if *gotAuth != "Bearer boot-1" {
		t.Fatalf("Authorization = %q, want the bootstrap token", *gotAuth)
	}
	if c.AgentID() != "ag-1" || c.HostID() == "" {
		t.Fatalf("AgentID/HostID = %q/%q", c.AgentID(), c.HostID())
	}
	if c.GRPCPerRPCCredentials() == nil {
		t.Fatal("GRPCPerRPCCredentials is nil")
	}
	if _, err := c.GRPCPerRPCCredentials().GetRequestMetadata(ctx); err == nil {
		t.Fatal("a context without gRPC RequestInfo must fail")
	}
	if err := c.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
}

func TestClientRegisterFailures(t *testing.T) {
	for name, tc := range map[string]struct {
		status int
		body   string
		want   string
	}{
		"http error":  {http.StatusForbidden, "denied", "HTTP 403"},
		"bad json":    {http.StatusOK, "{", "parse registration response"},
		"no agent id": {http.StatusOK, `{"component_scope":"s"}`, "missing agent_id"},
		"no scope":    {http.StatusOK, `{"agent_id":"a"}`, "missing component_scope"},
	} {
		t.Run(name, func(t *testing.T) {
			srv, _ := fakePlatform(t, tc.status, tc.body)
			c := newFlowClient(t, srv.URL)
			c.SetHTTPClient(srv.Client())
			if err := c.Discover(context.Background()); err != nil {
				t.Fatalf("Discover: %v", err)
			}
			err := c.Register(context.Background())
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("Register = %v, want %q", err, tc.want)
			}
		})
	}
}

func TestNewClientValidation(t *testing.T) {
	cases := map[string]ClientConfig{
		"no url":   {AgentName: "a"},
		"http url": {PlatformURL: "http://x.example", AgentName: "a"},
		"no name":  {PlatformURL: "https://x.example"},
	}
	for name, cfg := range cases {
		if _, err := NewClient(cfg); err == nil {
			t.Errorf("%s: want an error", name)
		}
	}
}

func TestRegistrationAuthReusesTheHostKey(t *testing.T) {
	srv, gotAuth := fakePlatform(t, http.StatusOK, `{"agent_id":"ag-1","component_scope":"s"}`)
	c := newFlowClient(t, srv.URL)
	c.config.BootstrapToken = ""
	t.Setenv("GIBSON_BOOTSTRAP_TOKEN", "")
	c.SetHTTPClient(srv.Client())
	if err := c.Discover(context.Background()); err != nil {
		t.Fatal(err)
	}
	if err := c.Register(context.Background()); err != nil {
		t.Fatalf("Register: %v", err)
	}
	if !strings.HasPrefix(*gotAuth, "Bearer ") || *gotAuth == "Bearer boot-1" {
		t.Fatalf("Authorization = %q, want a host+jwt", *gotAuth)
	}
}
