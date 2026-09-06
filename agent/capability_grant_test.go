// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Zero Root AI

package agent

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/stretchr/testify/assert"
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

// mintGrant produces a signed Capability Grant JWT using the test key.
func mintGrant(t *testing.T, k testKeys, opts grantOpts) string {
	t.Helper()
	now := opts.iat
	if now.IsZero() {
		now = time.Now().UTC().Truncate(time.Second)
	}
	exp := opts.exp
	if exp.IsZero() {
		exp = now.Add(30 * time.Second)
	}
	jti := opts.jti
	if jti == "" {
		jti = randomHex(t)
	}
	iss := opts.iss
	if iss == "" {
		iss = "ext-authz.test"
	}
	aud := opts.aud
	if aud == "" {
		aud = "tool-http"
	}
	inputsHash := opts.inputsHash
	if inputsHash == "" {
		inputsHash = "aabbccdd"
	}

	claims := &rawGrantClaims{
		RegisteredClaims: jwt.RegisteredClaims{
			Issuer:    iss,
			Subject:   "agent-1",
			Audience:  jwt.ClaimStrings{aud},
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(exp),
			ID:        jti,
		},
		InputsHash: inputsHash,
		Tenant:     "tenant-uuid-123",
	}

	token := jwt.NewWithClaims(jwt.SigningMethodES256, claims)
	token.Header["kid"] = k.kid
	signed, err := token.SignedString(k.priv)
	require.NoError(t, err)
	return signed
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

func TestCapabilityGrantValidate(t *testing.T) {
	k := generateTestKeys(t)
	srv, failNext := jwksServer(t, k)

	type tc struct {
		name           string
		rawJWT         func() string
		expectedTool   string
		expectedHash   string
		opts           []ValidatorOption
		wantErrContain string
	}

	validJWT := func() string {
		return mintGrant(t, k, grantOpts{})
	}

	tests := []tc{
		{
			name:         "success",
			rawJWT:       validJWT,
			expectedTool: "tool-http",
			expectedHash: "aabbccdd",
		},
		{
			name: "signature failure tampered payload",
			rawJWT: func() string {
				tok := mintGrant(t, k, grantOpts{})
				parts := strings.Split(tok, ".")
				require.Len(t, parts, 3)
				// flip one byte in the payload
				payload := parts[1]
				b, err := base64.RawURLEncoding.DecodeString(payload)
				require.NoError(t, err)
				b[0] ^= 0xFF
				parts[1] = base64.RawURLEncoding.EncodeToString(b)
				return strings.Join(parts, ".")
			},
			expectedTool:   "tool-http",
			expectedHash:   "aabbccdd",
			wantErrContain: "JWT parse/verify failed",
		},
		{
			name: "expired token",
			rawJWT: func() string {
				past := time.Now().Add(-120 * time.Second)
				return mintGrant(t, k, grantOpts{
					iat: past,
					exp: past.Add(10 * time.Second),
				})
			},
			expectedTool:   "tool-http",
			expectedHash:   "aabbccdd",
			wantErrContain: "JWT parse/verify failed",
		},
		{
			name: "future dated iat beyond 5s",
			rawJWT: func() string {
				future := time.Now().Add(30 * time.Second)
				return mintGrant(t, k, grantOpts{
					iat: future,
					exp: future.Add(30 * time.Second),
				})
			},
			expectedTool:   "tool-http",
			expectedHash:   "aabbccdd",
			wantErrContain: "iat is too far in the future",
		},
		{
			name: "audience mismatch",
			rawJWT: func() string {
				return mintGrant(t, k, grantOpts{aud: "tool-other"})
			},
			expectedTool:   "tool-http",
			expectedHash:   "aabbccdd",
			wantErrContain: "audience",
		},
		{
			name:           "inputs hash mismatch",
			rawJWT:         validJWT,
			expectedTool:   "tool-http",
			expectedHash:   "wrong-hash",
			wantErrContain: "inputs_hash mismatch",
		},
		{
			name: "wrong issuer with WithIssuer configured",
			rawJWT: func() string {
				return mintGrant(t, k, grantOpts{iss: "ext-authz.other"})
			},
			expectedTool:   "tool-http",
			expectedHash:   "aabbccdd",
			opts:           []ValidatorOption{WithIssuer("ext-authz.test")},
			wantErrContain: "issuer mismatch",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_ = failNext.Swap(0) // ensure server is healthy

			defaultOpts := []ValidatorOption{
				WithJWKSCacheTTL(5 * time.Minute),
				WithStaleJWKSWindow(10 * time.Minute),
			}
			opts := append(defaultOpts, tt.opts...)
			v, err := NewCapabilityGrantValidator(srv.URL, opts...)
			require.NoError(t, err)

			rawJWT := tt.rawJWT()
			claims, err := v.ValidateCapabilityGrant(context.Background(), rawJWT, tt.expectedTool, tt.expectedHash)

			if tt.wantErrContain != "" {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.wantErrContain)
				assert.Nil(t, claims)
			} else {
				require.NoError(t, err)
				require.NotNil(t, claims)
				assert.Equal(t, tt.expectedTool, claims.ToolID)
				assert.Equal(t, tt.expectedHash, claims.InputsHash)
				assert.Equal(t, "agent-1", claims.AgentID)
				assert.Equal(t, "tenant-uuid-123", claims.Tenant)
				assert.NotEmpty(t, claims.JTI)
				assert.False(t, claims.IssuedAt.IsZero())
				assert.False(t, claims.ExpiresAt.IsZero())
			}
		})
	}
}

// -----------------------------------------------------------------------
// JTI replay detection
// -----------------------------------------------------------------------

func TestCapabilityGrantReplay(t *testing.T) {
	k := generateTestKeys(t)
	srv, _ := jwksServer(t, k)

	v, err := NewCapabilityGrantValidator(srv.URL,
		WithJWKSCacheTTL(5*time.Minute),
		WithStaleJWKSWindow(10*time.Minute),
	)
	require.NoError(t, err)

	// Use a fixed JTI so both calls share it.
	fixedJTI := "fixed-jti-replay-test"
	raw := mintGrant(t, k, grantOpts{jti: fixedJTI})

	// First use: should succeed.
	claims, err := v.ValidateCapabilityGrant(context.Background(), raw, "tool-http", "aabbccdd")
	require.NoError(t, err)
	require.NotNil(t, claims)

	// Second use (replay): must fail.
	// Re-mint with the same JTI so the token itself is still valid (not expired).
	raw2 := mintGrant(t, k, grantOpts{jti: fixedJTI})
	_, err = v.ValidateCapabilityGrant(context.Background(), raw2, "tool-http", "aabbccdd")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "replay")
}

// -----------------------------------------------------------------------
// JWKS stale / failure behaviour
// -----------------------------------------------------------------------

func TestCapabilityGrantJWKSUpstreamFailAfterFirstFetch(t *testing.T) {
	k := generateTestKeys(t)
	srv, failNext := jwksServer(t, k)

	// Short cache TTL so we try to refresh quickly, but long stale window.
	v, err := NewCapabilityGrantValidator(srv.URL,
		WithJWKSCacheTTL(1*time.Millisecond),
		WithStaleJWKSWindow(5*time.Minute),
	)
	require.NoError(t, err)

	// First call succeeds, populating the cache.
	raw := mintGrant(t, k, grantOpts{})
	_, err = v.ValidateCapabilityGrant(context.Background(), raw, "tool-http", "aabbccdd")
	require.NoError(t, err)

	// Fail the next JWKS fetch.
	failNext.Store(1)
	time.Sleep(2 * time.Millisecond) // ensure cacheTTL has elapsed

	// Second call should still succeed (serving stale keys within stale window).
	raw2 := mintGrant(t, k, grantOpts{})
	_, err = v.ValidateCapabilityGrant(context.Background(), raw2, "tool-http", "aabbccdd")
	require.NoError(t, err, "should still validate using stale keys within stale window")
}

func TestCapabilityGrantJWKSFirstFetchFails(t *testing.T) {
	k := generateTestKeys(t)
	srv, failNext := jwksServer(t, k)
	failNext.Store(1) // fail the first (and only) fetch

	v, err := NewCapabilityGrantValidator(srv.URL)
	require.NoError(t, err)

	raw := mintGrant(t, k, grantOpts{})
	_, err = v.ValidateCapabilityGrant(context.Background(), raw, "tool-http", "aabbccdd")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "JWKS fetch failed")
}

func TestCapabilityGrantJWKSStaleWindowExceeded(t *testing.T) {
	k := generateTestKeys(t)
	srv, failNext := jwksServer(t, k)

	// Very short stale window so we can exceed it in the test.
	v, err := NewCapabilityGrantValidator(srv.URL,
		WithJWKSCacheTTL(1*time.Millisecond),
		WithStaleJWKSWindow(2*time.Millisecond),
	)
	require.NoError(t, err)

	// Populate cache with a successful fetch.
	raw := mintGrant(t, k, grantOpts{})
	_, err = v.ValidateCapabilityGrant(context.Background(), raw, "tool-http", "aabbccdd")
	require.NoError(t, err)

	// Fail all subsequent JWKS fetches and wait past the stale window.
	failNext.Store(1)
	time.Sleep(10 * time.Millisecond)

	raw2 := mintGrant(t, k, grantOpts{})
	_, err = v.ValidateCapabilityGrant(context.Background(), raw2, "tool-http", "aabbccdd")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "stale window exceeded")
}

// -----------------------------------------------------------------------
// Constructor — missing URL
// -----------------------------------------------------------------------

func TestNewCapabilityGrantValidatorNoURL(t *testing.T) {
	t.Setenv("EXT_AUTHZ_JWKS_URL", "")
	_, err := NewCapabilityGrantValidator("")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "jwksURL is required")
}

func TestNewCapabilityGrantValidatorFallsBackToEnv(t *testing.T) {
	k := generateTestKeys(t)
	srv, _ := jwksServer(t, k)
	t.Setenv("EXT_AUTHZ_JWKS_URL", srv.URL)

	v, err := NewCapabilityGrantValidator("")
	require.NoError(t, err)
	require.NotNil(t, v)
}

// -----------------------------------------------------------------------
// Option accessors
// -----------------------------------------------------------------------

func TestCapabilityGrantValidatorOptions(t *testing.T) {
	k := generateTestKeys(t)
	srv, _ := jwksServer(t, k)

	v, err := NewCapabilityGrantValidator(srv.URL,
		WithJWKSCacheTTL(30*time.Second),
		WithStaleJWKSWindow(2*time.Minute),
		WithReplayCacheSize(128),
		WithIssuer("ext-authz.example.com"),
	)
	require.NoError(t, err)
	assert.Equal(t, 30*time.Second, v.cfg.cacheTTL)
	assert.Equal(t, 2*time.Minute, v.cfg.staleWindow)
	assert.Equal(t, 128, v.cfg.replayCacheSize)
	assert.Equal(t, "ext-authz.example.com", v.cfg.issuer)
}

// -----------------------------------------------------------------------
// replayLRU unit tests
// -----------------------------------------------------------------------

func TestReplayLRU(t *testing.T) {
	r := newReplayLRU(3)

	// Order after each add (front → back): a; b,a; c,b,a
	assert.True(t, r.add("a"), "first add should succeed")
	assert.True(t, r.add("b"))
	assert.True(t, r.add("c"))

	// Replay: 'a' is still in cache.
	assert.False(t, r.add("a"), "replay of 'a' should fail")

	// Adding 'd' evicts the back item which is 'a'.
	// After: d,c,b (front→back)
	assert.True(t, r.add("d"), "eviction should make room for 'd'")

	// 'a' was evicted, so re-adding it should succeed.
	assert.True(t, r.add("a"), "evicted 'a' should be re-addable")

	// 'c' is still in cache — should be a replay.
	assert.False(t, r.add("c"), "replay of 'c' should fail")
}

func TestReplayLRUDefaultSize(t *testing.T) {
	// Ensure size <=0 falls back to default.
	r := newReplayLRU(0)
	assert.Equal(t, 4096, r.maxSize)
}

// -----------------------------------------------------------------------
// JWKS with malformed JWK (bad base64 in x/y) — triggers skip-warning path
// -----------------------------------------------------------------------

func TestCapabilityGrantJWKSMalformedKeySkipped(t *testing.T) {
	k := generateTestKeys(t)
	// Build a server that returns one valid key + one key with bad X coordinate.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		const coordLen = 32
		xBytes := leftPadBytes(k.pub.X.Bytes(), coordLen)
		yBytes := leftPadBytes(k.pub.Y.Bytes(), coordLen)
		type jwk struct {
			KTY string `json:"kty"`
			CRV string `json:"crv"`
			KID string `json:"kid"`
			Alg string `json:"alg"`
			X   string `json:"x"`
			Y   string `json:"y"`
		}
		type jwkSetLocal struct {
			Keys []jwk `json:"keys"`
		}
		ks := jwkSetLocal{Keys: []jwk{
			{
				KTY: "EC", CRV: "P-256", KID: k.kid, Alg: "ES256",
				X: base64.RawURLEncoding.EncodeToString(xBytes),
				Y: base64.RawURLEncoding.EncodeToString(yBytes),
			},
			// bad key: X is not valid base64.
			{KTY: "EC", CRV: "P-256", KID: "bad-key", Alg: "ES256", X: "!!!notbase64", Y: "aaaa"},
		}}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(ks)
	}))
	t.Cleanup(srv.Close)

	v, err := NewCapabilityGrantValidator(srv.URL)
	require.NoError(t, err)

	// Should still validate using the good key.
	raw := mintGrant(t, k, grantOpts{})
	claims, err := v.ValidateCapabilityGrant(context.Background(), raw, "tool-http", "aabbccdd")
	require.NoError(t, err)
	require.NotNil(t, claims)
}

// -----------------------------------------------------------------------
// Unknown kid forces a JWKS refresh
// -----------------------------------------------------------------------

func TestCapabilityGrantUnknownKidTriggersRefresh(t *testing.T) {
	k := generateTestKeys(t)
	srv, _ := jwksServer(t, k)

	// Long cache TTL — refresh only happens on unknown kid.
	v, err := NewCapabilityGrantValidator(srv.URL,
		WithJWKSCacheTTL(10*time.Minute),
		WithStaleJWKSWindow(30*time.Minute),
	)
	require.NoError(t, err)

	// Populate cache with known kid.
	raw := mintGrant(t, k, grantOpts{})
	_, err = v.ValidateCapabilityGrant(context.Background(), raw, "tool-http", "aabbccdd")
	require.NoError(t, err)

	// Attempt with a different key whose kid is not in cache.
	k2 := generateTestKeys(t)
	k2.kid = "unknown-kid"
	raw2 := mintGrant(t, k2, grantOpts{})
	_, err = v.ValidateCapabilityGrant(context.Background(), raw2, "tool-http", "aabbccdd")
	// The kid is not in the JWKS served by srv — should fail (key not found or sig fail).
	require.Error(t, err)
}

// -----------------------------------------------------------------------
// ValidateCapabilityGrant: empty jti / tenant / inputs_hash
// -----------------------------------------------------------------------

func TestCapabilityGrantMissingClaims(t *testing.T) {
	k := generateTestKeys(t)
	srv, _ := jwksServer(t, k)

	v, err := NewCapabilityGrantValidator(srv.URL)
	require.NoError(t, err)

	t.Run("empty inputs_hash", func(t *testing.T) {
		// Mint a valid token but with empty inputs_hash.
		// The mismatch check fires first (ConstantTimeCompare("", "expected") != 1).
		raw := mintGrant(t, k, grantOpts{inputsHash: ""})
		_, err := v.ValidateCapabilityGrant(context.Background(), raw, "tool-http", "expected-hash")
		require.Error(t, err)
	})
}
