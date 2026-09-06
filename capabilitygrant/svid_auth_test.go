// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Zero Root AI

package capabilitygrant

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"errors"
	"testing"
	"time"

	josejwt "github.com/go-jose/go-jose/v4"
	"github.com/go-jose/go-jose/v4/jwt"
	"github.com/spiffe/go-spiffe/v2/svid/jwtsvid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// stubJWTSVIDSource is a test double for jwtSVIDSource. It stands in for a
// *workloadapi.JWTSource so buildRegistrationAuth's SVID-preferred path can
// be exercised without a live SPIRE Workload API socket, which the SDK's
// test environment does not have.
type stubJWTSVIDSource struct {
	svid *jwtsvid.SVID
	err  error

	// gotAudience records the audience buildRegistrationAuth asked for, so
	// tests can assert it was bound to the register URL.
	gotAudience string
}

func (s *stubJWTSVIDSource) FetchJWTSVID(_ context.Context, params jwtsvid.Params) (*jwtsvid.SVID, error) {
	s.gotAudience = params.Audience
	if s.err != nil {
		return nil, s.err
	}
	return s.svid, nil
}

func (s *stubJWTSVIDSource) Close() error { return nil }

// makeTestJWTSVID builds a real, structurally-valid *jwtsvid.SVID for
// audience via jwtsvid.ParseInsecure, so it exercises the same Marshal()
// codepath buildRegistrationAuth relies on. It is signed with an ephemeral
// EC key — ParseInsecure never verifies the signature (that's the daemon's
// job, against the SPIRE bundle), it only requires a structurally valid,
// algorithm-allowed JWT.
func makeTestJWTSVID(t *testing.T, audience string) *jwtsvid.SVID {
	t.Helper()

	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	require.NoError(t, err)

	signer, err := josejwt.NewSigner(josejwt.SigningKey{
		Algorithm: josejwt.ES256,
		Key:       key,
	}, nil)
	require.NoError(t, err)

	claims := jwt.Claims{
		Subject:  "spiffe://zeroroot.ai/ns/gibson/sa/scanner-01",
		Audience: jwt.Audience{audience},
		Expiry:   jwt.NewNumericDate(time.Now().Add(time.Hour)),
	}

	token, err := jwt.Signed(signer).Claims(claims).Serialize()
	require.NoError(t, err)

	svid, err := jwtsvid.ParseInsecure(token, []string{audience})
	require.NoError(t, err)

	return svid
}

func TestBuildRegistrationAuth_NoSVIDSource_FallsBackToBootstrap(t *testing.T) {
	srv, _ := buildMockPlatform(t, nil)
	client := newTestClient(t, srv)

	require.Nil(t, client.svidSource)

	header, err := client.buildRegistrationAuth(context.Background(), srv.URL+"/agent-auth/register")
	require.NoError(t, err)

	// newTestClient configures a bootstrap token and a fresh (never-persisted)
	// host key, so the bootstrap credential wins per buildRegistrationAuth's
	// documented precedence — unchanged by the SVID addition.
	assert.Equal(t, "Bearer test-bootstrap-token", header)
}

func TestBuildRegistrationAuth_SVIDSource_PreferredOverBootstrap(t *testing.T) {
	srv, _ := buildMockPlatform(t, nil)
	client := newTestClient(t, srv)

	registerURL := srv.URL + "/agent-auth/register"
	svid := makeTestJWTSVID(t, registerURL)
	stub := &stubJWTSVIDSource{svid: svid}
	client.svidSource = stub

	header, err := client.buildRegistrationAuth(context.Background(), registerURL)
	require.NoError(t, err)

	assert.Equal(t, "Bearer "+svid.Marshal(), header)
	assert.NotEqual(t, "Bearer test-bootstrap-token", header)
	assert.Equal(t, registerURL, stub.gotAudience, "SVID must be scoped to the register URL as its audience")
}

func TestBuildRegistrationAuth_SVIDSourceFails_FallsThroughToBootstrap(t *testing.T) {
	srv, _ := buildMockPlatform(t, nil)
	client := newTestClient(t, srv)

	registerURL := srv.URL + "/agent-auth/register"
	client.svidSource = &stubJWTSVIDSource{err: errors.New("workload api unreachable")}

	header, err := client.buildRegistrationAuth(context.Background(), registerURL)
	require.NoError(t, err)

	// A broken SVID source must never fail registration outright — it falls
	// through to the pre-existing bootstrap/host+jwt chain.
	assert.Equal(t, "Bearer test-bootstrap-token", header)
}

func TestClientClose_NoSVIDSource_NoOp(t *testing.T) {
	srv, _ := buildMockPlatform(t, nil)
	client := newTestClient(t, srv)

	require.NoError(t, client.Close())
}

func TestClientClose_ClosesSVIDSource(t *testing.T) {
	srv, _ := buildMockPlatform(t, nil)
	client := newTestClient(t, srv)

	closed := false
	client.svidSource = &closeTrackingStub{onClose: func() { closed = true }}

	require.NoError(t, client.Close())
	assert.True(t, closed)
}

// closeTrackingStub is a jwtSVIDSource whose only job is recording that
// Close was called.
type closeTrackingStub struct {
	onClose func()
}

func (c *closeTrackingStub) FetchJWTSVID(context.Context, jwtsvid.Params) (*jwtsvid.SVID, error) {
	return nil, errors.New("not implemented")
}

func (c *closeTrackingStub) Close() error {
	c.onClose()
	return nil
}
