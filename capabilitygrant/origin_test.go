// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Zero Root AI

package capabilitygrant

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"sync/atomic"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestNewClientRequiresHTTPS shows that a platform URL with any scheme other
// than https is refused before the client contacts anything.
func TestNewClientRequiresHTTPS(t *testing.T) {
	for _, raw := range []string{
		"http://platform.example.com",
		"HTTP://platform.example.com",
		"ftp://platform.example.com",
		"platform.example.com",
		"//platform.example.com",
	} {
		_, err := NewClient(ClientConfig{
			PlatformURL: raw,
			AgentName:   "test-agent",
			HostKeyPath: filepath.Join(t.TempDir(), "host_key.json"),
		})
		require.Error(t, err, raw)
		require.ErrorIs(t, err, ErrPlatformURLNotHTTPS, raw)
	}

	_, err := NewClient(ClientConfig{
		PlatformURL: "https://user:secret@platform.example.com",
		AgentName:   "test-agent",
		HostKeyPath: filepath.Join(t.TempDir(), "host_key.json"),
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "user information")

	_, err = NewClient(ClientConfig{
		PlatformURL: "HTTPS://platform.example.com",
		AgentName:   "test-agent",
		HostKeyPath: filepath.Join(t.TempDir(), "host_key.json"),
	})
	require.NoError(t, err)
}

// TestDiscoverRequiresHTTPS shows that the package-level Discover refuses a
// cleartext base URL without sending a request.
func TestDiscoverRequiresHTTPS(t *testing.T) {
	var hits atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		hits.Add(1)
		w.WriteHeader(http.StatusOK)
	}))
	t.Cleanup(srv.Close)

	_, err := Discover(context.Background(), srv.URL, srv.Client())
	require.ErrorIs(t, err, ErrPlatformURLNotHTTPS)
	assert.Equal(t, int32(0), hits.Load(), "no request may leave for a cleartext platform URL")
}

// discoveryServer serves a discovery document whose endpoints are built by
// endpoints from the server's own https origin.
func discoveryServer(t *testing.T, endpoints func(origin string) map[string]string) *httptest.Server {
	t.Helper()
	mux := http.NewServeMux()
	mux.HandleFunc("/.well-known/agent-configuration", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"protocol_version": "1.0",
			"provider_name":    "origin test",
			"issuer":           "https://" + r.Host,
			"supported_modes":  []string{"autonomous"},
			"endpoints":        endpoints("https://" + r.Host),
		})
	})
	srv := httptest.NewTLSServer(mux)
	t.Cleanup(srv.Close)
	return srv
}

// TestDiscoverRejectsForeignEndpoints shows that a discovery document is
// refused when any endpoint is not on the platform origin.
func TestDiscoverRejectsForeignEndpoints(t *testing.T) {
	tests := []struct {
		name      string
		endpoints func(origin string) map[string]string
		wantErr   bool
	}{
		{
			name: "all endpoints on the platform origin",
			endpoints: func(origin string) map[string]string {
				return map[string]string{
					"register":   origin + "/r",
					"execute":    origin + "/e",
					"list":       origin + "/l",
					"status":     origin + "/s",
					"revoke":     origin + "/v",
					"introspect": origin + "/i",
				}
			},
		},
		{
			name: "empty optional endpoints are allowed",
			endpoints: func(origin string) map[string]string {
				return map[string]string{"register": origin + "/r"}
			},
		},
		{
			name: "register on another host",
			endpoints: func(_ string) map[string]string {
				return map[string]string{"register": "https://evil.example/r"}
			},
			wantErr: true,
		},
		{
			name: "register on the same host over http",
			endpoints: func(origin string) map[string]string {
				return map[string]string{"register": "http" + origin[len("https"):] + "/r"}
			},
			wantErr: true,
		},
		{
			name: "register on the same host with another port",
			endpoints: func(origin string) map[string]string {
				return map[string]string{"register": origin + ":1/r"}
			},
			wantErr: true,
		},
		{
			name: "relative register path",
			endpoints: func(_ string) map[string]string {
				return map[string]string{"register": "/agent-auth/register"}
			},
			wantErr: true,
		},
		{
			name: "secondary endpoint on another host",
			endpoints: func(origin string) map[string]string {
				return map[string]string{
					"register":   origin + "/r",
					"introspect": "https://evil.example/i",
				}
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			srv := discoveryServer(t, tt.endpoints)
			doc, err := Discover(context.Background(), srv.URL, srv.Client())
			if tt.wantErr {
				require.ErrorIs(t, err, ErrEndpointOrigin)
				assert.Nil(t, doc)
				return
			}
			require.NoError(t, err)
			require.NotNil(t, doc)
		})
	}
}

// TestRegisterNeverReachesForeignEndpoint shows that a foreign register
// endpoint never receives the registration credential, through Discover or
// through PatchDiscoveryRegisterURL.
func TestRegisterNeverReachesForeignEndpoint(t *testing.T) {
	var foreignHits atomic.Int32
	foreign := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		foreignHits.Add(1)
		assert.Empty(t, r.Header.Get("Authorization"), "credential reached a foreign host")
		w.WriteHeader(http.StatusCreated)
	}))
	t.Cleanup(foreign.Close)

	// The platform's discovery document points registration at the foreign host.
	platform := discoveryServer(t, func(_ string) map[string]string {
		return map[string]string{"register": foreign.URL + "/register"}
	})

	client, err := NewClient(ClientConfig{
		PlatformURL:    platform.URL,
		BootstrapToken: "one-time-token",
		HostKeyPath:    filepath.Join(t.TempDir(), "host_key.json"),
		AgentName:      "test-agent",
	})
	require.NoError(t, err)
	client.SetHTTPClient(platform.Client())

	require.ErrorIs(t, client.Discover(context.Background()), ErrEndpointOrigin)
	require.Error(t, client.Register(context.Background()), "Register must fail without a discovery document")

	// The test seam cannot redirect registration off the platform origin either.
	require.ErrorIs(t, client.PatchDiscoveryRegisterURL(foreign.URL+"/register"), ErrEndpointOrigin)

	assert.Equal(t, int32(0), foreignHits.Load())
}
