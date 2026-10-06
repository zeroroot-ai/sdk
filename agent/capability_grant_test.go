// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Zero Root AI

package agent

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// -----------------------------------------------------------------------
// Test helpers
// -----------------------------------------------------------------------

// testKeys holds a generated EC P-256 key pair used across tests.
type testKeys struct {
	priv *ecdsa.PrivateKey
	pub  *ecdsa.PublicKey
	kid  string
}

func generateTestKeys(t *testing.T) testKeys {
	t.Helper()
	priv, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	require.NoError(t, err)
	return testKeys{priv: priv, pub: &priv.PublicKey, kid: "test-key-1"}
}

// jwksServer creates an httptest.Server that serves a JWKS for the given keys.
// It returns the server and an atomic failure flag — set failNext to 1 to make
// the next request return HTTP 500.
func jwksServer(t *testing.T, keys ...testKeys) (srv *httptest.Server, failNext *atomic.Int32) {
	t.Helper()
	failNext = new(atomic.Int32)

	srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if failNext.Swap(0) != 0 {
			http.Error(w, "upstream error", http.StatusInternalServerError)
			return
		}
		type jwk struct {
			KTY string `json:"kty"`
			CRV string `json:"crv"`
			KID string `json:"kid"`
			Alg string `json:"alg"`
			X   string `json:"x"`
			Y   string `json:"y"`
		}
		type jwkSet struct {
			Keys []jwk `json:"keys"`
		}
		const coordLen = 32
		var ks jwkSet
		for _, k := range keys {
			xBytes := leftPadBytes(k.pub.X.Bytes(), coordLen)
			yBytes := leftPadBytes(k.pub.Y.Bytes(), coordLen)
			ks.Keys = append(ks.Keys, jwk{
				KTY: "EC",
				CRV: "P-256",
				KID: k.kid,
				Alg: "ES256",
				X:   base64.RawURLEncoding.EncodeToString(xBytes),
				Y:   base64.RawURLEncoding.EncodeToString(yBytes),
			})
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(ks)
	}))
	t.Cleanup(srv.Close)
	return srv, failNext
}

func leftPadBytes(b []byte, n int) []byte {
	if len(b) >= n {
		return b
	}
	out := make([]byte, n)
	copy(out[n-len(b):], b)
	return out
}

type grantOpts struct {
	iss        string
	aud        string
	iat        time.Time
	exp        time.Time
	jti        string
	inputsHash string
}

func randomHex(t *testing.T) string {
	t.Helper()
	b := make([]byte, 16)
	_, err := rand.Read(b)
	require.NoError(t, err)
	return base64.RawURLEncoding.EncodeToString(b)
}

// -----------------------------------------------------------------------
// Table-driven happy path + error cases
// -----------------------------------------------------------------------

// -----------------------------------------------------------------------
// JTI replay detection
// -----------------------------------------------------------------------

// -----------------------------------------------------------------------
// JWKS stale / failure behaviour
// -----------------------------------------------------------------------

// -----------------------------------------------------------------------
// Constructor — missing URL
// -----------------------------------------------------------------------

// -----------------------------------------------------------------------
// Option accessors
// -----------------------------------------------------------------------

// -----------------------------------------------------------------------
// replayLRU unit tests
// -----------------------------------------------------------------------

// -----------------------------------------------------------------------
// JWKS with malformed JWK (bad base64 in x/y) — triggers skip-warning path
// -----------------------------------------------------------------------

// -----------------------------------------------------------------------
// Unknown kid forces a JWKS refresh
// -----------------------------------------------------------------------

// -----------------------------------------------------------------------
// ValidateCapabilityGrant: empty jti / tenant / inputs_hash
// -----------------------------------------------------------------------
