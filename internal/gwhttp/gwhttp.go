// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Zero Root AI

// Package gwhttp holds the HTTP client rules that the model client and the
// A2A delegation client share for a call to a Gibson gateway.
//
// The gateway serves TLS with a private CA, and a sandbox has no bundle for
// it. The daemon sends the bundle as PEM. The client trusts exactly that
// bundle. When the daemon sends none, the client uses the system roots. There
// is no option to skip verification.
package gwhttp

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"errors"
	"fmt"
	"net/http"
	"strings"
)

// HeaderFunc returns the headers to add to a call of a route. The route is the
// path of the request URL. The agent uses it to send its grant in the header
// x-capability-grant.
type HeaderFunc func(ctx context.Context, route string) (map[string]string, error)

// CheckURL refuses a plain http URL when a CA bundle is set. The bundle only
// makes sense for TLS, and sending a grant in the clear is not allowed.
func CheckURL(baseURL, caPEM string) error {
	if caPEM != "" && !strings.HasPrefix(baseURL, "https://") {
		return errors.New("a gateway client with a CA bundle needs an https URL")
	}
	return nil
}

// Transport returns a transport that trusts exactly the bundle in caPEM. It
// returns nil for an empty bundle: the caller then uses the system roots.
func Transport(caPEM string) (*http.Transport, error) {
	if caPEM == "" {
		return nil, nil
	}
	pool := x509.NewCertPool()
	if !pool.AppendCertsFromPEM([]byte(caPEM)) {
		return nil, errors.New("the CA bundle of the gateway holds no certificate")
	}
	return &http.Transport{
		Proxy:           http.ProxyFromEnvironment,
		TLSClientConfig: &tls.Config{RootCAs: pool, MinVersion: tls.VersionTLS12},
	}, nil
}

// roundTripper adds fixed headers and the headers of a HeaderFunc to each
// request.
type roundTripper struct {
	next    http.RoundTripper
	fixed   map[string]string
	headers HeaderFunc
}

// NewClient returns an HTTP client for a gateway. It trusts the bundle in
// caPEM, or the system roots when caPEM is empty. It adds the fixed headers
// and the headers of the HeaderFunc to each request.
func NewClient(baseURL, caPEM string, fixed map[string]string, headers HeaderFunc) (*http.Client, error) {
	if err := CheckURL(baseURL, caPEM); err != nil {
		return nil, err
	}
	tr, err := Transport(caPEM)
	if err != nil {
		return nil, err
	}
	var next http.RoundTripper = http.DefaultTransport
	if tr != nil {
		next = tr
	}
	return &http.Client{Transport: &roundTripper{next: next, fixed: fixed, headers: headers}}, nil
}

// RoundTrip adds the headers and sends the request.
func (r *roundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	req = req.Clone(req.Context())
	for k, v := range r.fixed {
		req.Header.Set(k, v)
	}
	if r.headers != nil {
		extra, err := r.headers(req.Context(), req.URL.Path)
		if err != nil {
			return nil, fmt.Errorf("get call headers: %w", err)
		}
		for k, v := range extra {
			req.Header.Set(k, v)
		}
	}
	return r.next.RoundTrip(req)
}
