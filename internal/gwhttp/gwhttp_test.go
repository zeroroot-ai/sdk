// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Zero Root AI

package gwhttp

import (
	"context"
	"encoding/pem"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCheckURL(t *testing.T) {
	require.NoError(t, CheckURL("http://x", ""))
	require.NoError(t, CheckURL("https://x", "PEM"))
	require.Error(t, CheckURL("http://x", "PEM"))
}

func TestTransportBadBundle(t *testing.T) {
	tr, err := Transport("")
	require.NoError(t, err)
	assert.Nil(t, tr)
	_, err = Transport("not pem")
	require.ErrorContains(t, err, "no certificate")
}

func TestClientAddsHeadersAndTrustsTheBundle(t *testing.T) {
	var got http.Header
	var path string
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		got, path = r.Header.Clone(), r.URL.Path
	}))
	defer srv.Close()
	ca := string(pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: srv.Certificate().Raw}))

	c, err := NewClient(srv.URL, ca, map[string]string{"x-fixed": "1"},
		func(_ context.Context, route string) (map[string]string, error) {
			return map[string]string{"x-capability-grant": "g:" + route}, nil
		})
	require.NoError(t, err)
	resp, err := c.Get(srv.URL + "/a2a/helper")
	require.NoError(t, err)
	_ = resp.Body.Close()
	assert.Equal(t, "1", got.Get("x-fixed"))
	assert.Equal(t, "g:/a2a/helper", got.Get("x-capability-grant"))
	assert.Equal(t, "/a2a/helper", path)

	// Without the bundle the system roots do not trust the test CA.
	plain, err := NewClient(srv.URL, "", nil, nil)
	require.NoError(t, err)
	_, err = plain.Get(srv.URL)
	require.Error(t, err)
}

func TestClientHeaderError(t *testing.T) {
	srv := httptest.NewServer(http.NotFoundHandler())
	defer srv.Close()
	c, err := NewClient(srv.URL, "", nil, func(context.Context, string) (map[string]string, error) {
		return nil, errors.New("no grant")
	})
	require.NoError(t, err)
	_, err = c.Get(srv.URL)
	require.ErrorContains(t, err, "no grant")
}

func TestNewClientRefusals(t *testing.T) {
	_, err := NewClient("http://plain", "PEM", nil, nil)
	require.ErrorContains(t, err, "https")
	_, err = NewClient("https://x", "not pem", nil, nil)
	require.ErrorContains(t, err, "no certificate")
}
