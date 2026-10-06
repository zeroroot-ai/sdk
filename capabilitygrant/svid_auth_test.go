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
