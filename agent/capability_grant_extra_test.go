// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Zero Root AI

// Package agent — supplemental capability grant tests to cover remaining
// branches missed by the main test suite.
package agent

import (
	"context"
	"crypto/ecdsa"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// -----------------------------------------------------------------------
// mintGrantCustom — extends grantOpts with fields for empty-claim injection
// -----------------------------------------------------------------------

// mintGrantNoTenant mints a grant with an empty tenant claim (bypassing the
// inputsHash check by matching expectedInputsHash = "aabbccdd").
func mintGrantNoTenant(t *testing.T, k testKeys) string {
	t.Helper()
	now := time.Now().UTC().Truncate(time.Second)
	exp := now.Add(30 * time.Second)

	claims := &rawGrantClaims{
		RegisteredClaims: jwt.RegisteredClaims{
			Issuer:    "ext-authz.test",
			Subject:   "agent-1",
			Audience:  jwt.ClaimStrings{"tool-http"},
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(exp),
			ID:        "jti-no-tenant",
		},
		InputsHash: "aabbccdd", // matches expectedInputsHash used below
		Tenant:     "",         // deliberately empty
	}
	token := jwt.NewWithClaims(jwt.SigningMethodES256, claims)
	token.Header["kid"] = k.kid
	signed, err := token.SignedString(k.priv)
	require.NoError(t, err)
	return signed
}

// mintGrantNoJTI mints a grant with an empty JTI.
func mintGrantNoJTI(t *testing.T, k testKeys) string {
	t.Helper()
	now := time.Now().UTC().Truncate(time.Second)
	exp := now.Add(30 * time.Second)

	claims := &rawGrantClaims{
		RegisteredClaims: jwt.RegisteredClaims{
			Issuer:    "ext-authz.test",
			Subject:   "agent-1",
			Audience:  jwt.ClaimStrings{"tool-http"},
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(exp),
			ID:        "", // deliberately empty
		},
		InputsHash: "aabbccdd",
		Tenant:     "tenant-123",
	}
	token := jwt.NewWithClaims(jwt.SigningMethodES256, claims)
	token.Header["kid"] = k.kid
	signed, err := token.SignedString(k.priv)
	require.NoError(t, err)
	return signed
}

// mintGrantEmptyInputsHashAndExpect mints a grant with empty inputs_hash
// when expectedInputsHash is also empty — the ConstantTimeCompare passes but
// the empty-check fires.
func mintGrantEmptyInputsHashAndExpect(t *testing.T, k testKeys) string {
	t.Helper()
	now := time.Now().UTC().Truncate(time.Second)
	exp := now.Add(30 * time.Second)

	claims := &rawGrantClaims{
		RegisteredClaims: jwt.RegisteredClaims{
			Issuer:    "ext-authz.test",
			Subject:   "agent-1",
			Audience:  jwt.ClaimStrings{"tool-http"},
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(exp),
			ID:        "jti-empty-hash",
		},
		InputsHash: "", // empty
		Tenant:     "tenant-123",
	}
	token := jwt.NewWithClaims(jwt.SigningMethodES256, claims)
	token.Header["kid"] = k.kid
	signed, err := token.SignedString(k.priv)
	require.NoError(t, err)
	return signed
}

// -----------------------------------------------------------------------
// Tests for empty claim branches in ValidateCapabilityGrant
// -----------------------------------------------------------------------

func TestCapabilityGrant_EmptyInputsHashAndExpected(t *testing.T) {
	k := generateTestKeys(t)
	srv, _ := jwksServer(t, k)

	v, err := NewCapabilityGrantValidator(srv.URL)
	require.NoError(t, err)

	raw := mintGrantEmptyInputsHashAndExpect(t, k)
	// ConstantTimeCompare("", "") == 1, but the subsequent empty-check must reject.
	_, err = v.ValidateCapabilityGrant(context.Background(), raw, "tool-http", "")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "inputs_hash")
}

func TestCapabilityGrant_EmptyTenant(t *testing.T) {
	k := generateTestKeys(t)
	srv, _ := jwksServer(t, k)

	v, err := NewCapabilityGrantValidator(srv.URL)
	require.NoError(t, err)

	raw := mintGrantNoTenant(t, k)
	_, err = v.ValidateCapabilityGrant(context.Background(), raw, "tool-http", "aabbccdd")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "tenant")
}

func TestCapabilityGrant_EmptyJTI(t *testing.T) {
	k := generateTestKeys(t)
	srv, _ := jwksServer(t, k)

	v, err := NewCapabilityGrantValidator(srv.URL)
	require.NoError(t, err)

	raw := mintGrantNoJTI(t, k)
	_, err = v.ValidateCapabilityGrant(context.Background(), raw, "tool-http", "aabbccdd")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "jti")
}

// -----------------------------------------------------------------------
// jwkToECDSA — bad Y coordinate base64
// -----------------------------------------------------------------------

func TestCapabilityGrant_JWKSBadYCoordinate(t *testing.T) {
	k := generateTestKeys(t)
	const coordLen = 32
	xBytes := leftPadBytes(k.pub.X.Bytes(), coordLen)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
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
				KTY: "EC",
				CRV: "P-256",
				KID: k.kid,
				Alg: "ES256",
				X:   base64.RawURLEncoding.EncodeToString(xBytes),
				Y:   "!!!invalid-base64!!!", // bad Y — causes jwkToECDSA decode error
			},
		}}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(ks)
	}))
	t.Cleanup(srv.Close)

	v, err := NewCapabilityGrantValidator(srv.URL)
	require.NoError(t, err)

	raw := mintGrant(t, k, grantOpts{})
	// The only valid key has a bad Y — JWKS contains no usable keys → error.
	_, err = v.ValidateCapabilityGrant(context.Background(), raw, "tool-http", "aabbccdd")
	require.Error(t, err)
}

// -----------------------------------------------------------------------
// refreshJWKS — non-EC / non-P256 key is silently skipped
// -----------------------------------------------------------------------

func TestCapabilityGrant_JWKSNonECKeySkipped(t *testing.T) {
	k := generateTestKeys(t)
	const coordLen = 32
	xBytes := leftPadBytes(k.pub.X.Bytes(), coordLen)
	yBytes := leftPadBytes(k.pub.Y.Bytes(), coordLen)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
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
			// RSA key — kty!=EC, skipped.
			{KTY: "RSA", KID: "rsa-key"},
			// EC but wrong curve — skipped.
			{KTY: "EC", CRV: "P-384", KID: "p384-key"},
			// Valid EC P-256 key.
			{
				KTY: "EC", CRV: "P-256", KID: k.kid, Alg: "ES256",
				X: base64.RawURLEncoding.EncodeToString(xBytes),
				Y: base64.RawURLEncoding.EncodeToString(yBytes),
			},
		}}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(ks)
	}))
	t.Cleanup(srv.Close)

	v, err := NewCapabilityGrantValidator(srv.URL)
	require.NoError(t, err)

	raw := mintGrant(t, k, grantOpts{})
	// Should succeed — non-EC/P-256 keys skipped, valid key still present.
	claims, err := v.ValidateCapabilityGrant(context.Background(), raw, "tool-http", "aabbccdd")
	require.NoError(t, err)
	require.NotNil(t, claims)
}

// -----------------------------------------------------------------------
// refreshJWKS — non-JSON body
// -----------------------------------------------------------------------

func TestCapabilityGrant_JWKSInvalidJSON(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte("not json at all {{{"))
	}))
	t.Cleanup(srv.Close)

	v, err := NewCapabilityGrantValidator(srv.URL)
	require.NoError(t, err)

	// Generate any key for minting (won't be validated — JWKS parse fails first).
	k := generateTestKeys(t)
	raw := mintGrant(t, k, grantOpts{})
	_, err = v.ValidateCapabilityGrant(context.Background(), raw, "tool-http", "aabbccdd")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "JWKS fetch failed")
}

// -----------------------------------------------------------------------
// lookupKeyLocked — staleExpired branch when keys exist
// This covers the path where jwksFetched is non-zero but staleAt has passed.
// The stale-expired case is already covered by TestCapabilityGrantJWKSStaleWindowExceeded
// but the "no key empty set" path isn't. Verify with empty-key server.
// -----------------------------------------------------------------------

func TestCapabilityGrant_JWKSEmptyKeySet(t *testing.T) {
	// Server returns valid JSON but with an empty keys array.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"keys":[]}`))
	}))
	t.Cleanup(srv.Close)

	v, err := NewCapabilityGrantValidator(srv.URL)
	require.NoError(t, err)

	k := generateTestKeys(t)
	raw := mintGrant(t, k, grantOpts{})
	_, err = v.ValidateCapabilityGrant(context.Background(), raw, "tool-http", "aabbccdd")
	require.Error(t, err)
}

// -----------------------------------------------------------------------
// lookupKeyLocked — empty kid lookup when cache has keys (returns any key)
// This covers the "kid=="" and loop-over-map" path in lookupKeyLocked.
// -----------------------------------------------------------------------

func TestCapabilityGrant_EmptyKidReturnsAnyKey(t *testing.T) {
	k := generateTestKeys(t)
	srv, _ := jwksServer(t, k)

	v, err := NewCapabilityGrantValidator(srv.URL,
		WithJWKSCacheTTL(10*time.Minute),
	)
	require.NoError(t, err)

	// Mint without a kid in the header — jwt lib uses empty kid.
	now := time.Now().UTC().Truncate(time.Second)
	claims := &rawGrantClaims{
		RegisteredClaims: jwt.RegisteredClaims{
			Issuer:    "ext-authz.test",
			Subject:   "agent-1",
			Audience:  jwt.ClaimStrings{"tool-http"},
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(30 * time.Second)),
			ID:        "jti-no-kid",
		},
		InputsHash: "aabbccdd",
		Tenant:     "tenant-uuid",
	}
	token := jwt.NewWithClaims(jwt.SigningMethodES256, claims)
	// Do NOT set token.Header["kid"] — leave it empty.
	signed, err := token.SignedString(k.priv)
	require.NoError(t, err)

	// First call: populates cache with the known kid.
	raw := mintGrant(t, k, grantOpts{})
	_, err = v.ValidateCapabilityGrant(context.Background(), raw, "tool-http", "aabbccdd")
	require.NoError(t, err)

	// Second call: empty kid → lookupKeyLocked returns any cached key.
	claims2, err := v.ValidateCapabilityGrant(context.Background(), signed, "tool-http", "aabbccdd")
	require.NoError(t, err)
	require.NotNil(t, claims2)
}

// -----------------------------------------------------------------------
// lookupKeyLocked: verify the "wrong algorithm" rejection path
// -----------------------------------------------------------------------

func TestCapabilityGrant_WrongSigningAlgorithm(t *testing.T) {
	k := generateTestKeys(t)
	srv, _ := jwksServer(t, k)

	v, err := NewCapabilityGrantValidator(srv.URL)
	require.NoError(t, err)

	// Use HS256 (symmetric) instead of ES256.
	now := time.Now().UTC().Truncate(time.Second)
	claims := &rawGrantClaims{
		RegisteredClaims: jwt.RegisteredClaims{
			Issuer:    "ext-authz.test",
			Subject:   "agent-1",
			Audience:  jwt.ClaimStrings{"tool-http"},
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(30 * time.Second)),
			ID:        "jti-hs256",
		},
		InputsHash: "aabbccdd",
		Tenant:     "tenant-uuid",
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	signed, err := token.SignedString([]byte("some-hmac-secret"))
	require.NoError(t, err)

	_, err = v.ValidateCapabilityGrant(context.Background(), signed, "tool-http", "aabbccdd")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "JWT parse/verify failed")
}

// -----------------------------------------------------------------------
// Issuer match when WithIssuer matches correctly
// -----------------------------------------------------------------------

func TestCapabilityGrant_IssuerMatchSucceeds(t *testing.T) {
	k := generateTestKeys(t)
	srv, _ := jwksServer(t, k)

	v, err := NewCapabilityGrantValidator(srv.URL, WithIssuer("ext-authz.test"))
	require.NoError(t, err)

	raw := mintGrant(t, k, grantOpts{iss: "ext-authz.test"})
	claims, err := v.ValidateCapabilityGrant(context.Background(), raw, "tool-http", "aabbccdd")
	require.NoError(t, err)
	assert.Equal(t, "ext-authz.test", claims.Issuer)
}

// -----------------------------------------------------------------------
// Background stale refresh (goroutine path in resolveKey)
// Serve stale while background refresh fires.
// -----------------------------------------------------------------------

func TestCapabilityGrant_ServeStaleWhileRefreshing(t *testing.T) {
	k := generateTestKeys(t)
	srv, failNext := jwksServer(t, k)

	v, err := NewCapabilityGrantValidator(srv.URL,
		WithJWKSCacheTTL(1*time.Millisecond),
		WithStaleJWKSWindow(10*time.Minute),
	)
	require.NoError(t, err)

	// Populate the cache.
	raw := mintGrant(t, k, grantOpts{})
	_, err = v.ValidateCapabilityGrant(context.Background(), raw, "tool-http", "aabbccdd")
	require.NoError(t, err)

	// Let cache TTL expire but stale window remains open.
	time.Sleep(5 * time.Millisecond)

	// Fail the next refresh — key is stale but within stale window.
	failNext.Store(1)

	// Should serve stale and trigger background refresh goroutine.
	raw2 := mintGrant(t, k, grantOpts{})
	_, err = v.ValidateCapabilityGrant(context.Background(), raw2, "tool-http", "aabbccdd")
	require.NoError(t, err, "stale key should still validate within stale window")
}

// -----------------------------------------------------------------------
// generateTestKeys with a different key produces signature failure
// -----------------------------------------------------------------------

func TestCapabilityGrant_WrongKey(t *testing.T) {
	k1 := generateTestKeys(t)
	k2 := generateTestKeys(t)
	k2.kid = k1.kid // same kid so we look it up, but different private key

	// Serve k1's public key.
	srv, _ := jwksServer(t, k1)
	v, err := NewCapabilityGrantValidator(srv.URL)
	require.NoError(t, err)

	// Mint with k2's private key (wrong key).
	raw := mintGrant(t, k2, grantOpts{})
	_, err = v.ValidateCapabilityGrant(context.Background(), raw, "tool-http", "aabbccdd")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "JWT parse/verify failed")
}

// -----------------------------------------------------------------------
// Inline JWKS HTTP status != 200
// -----------------------------------------------------------------------

func TestCapabilityGrant_JWKSHTTPError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "service unavailable", http.StatusServiceUnavailable)
	}))
	t.Cleanup(srv.Close)

	v, err := NewCapabilityGrantValidator(srv.URL)
	require.NoError(t, err)

	k := generateTestKeys(t)
	raw := mintGrant(t, k, grantOpts{})
	_, err = v.ValidateCapabilityGrant(context.Background(), raw, "tool-http", "aabbccdd")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "JWKS fetch failed")
}

// -----------------------------------------------------------------------
// Prometheus counter is initialized (smoke test for init())
// -----------------------------------------------------------------------

func TestCapGrantJWKSStaleness_WithCounter(t *testing.T) {
	// Verify that a wired JWKSStalenessCounter receives Inc() calls when
	// the JWKS cache becomes stale.
	var count int
	counter := &countingCounter{inc: func() { count++ }}

	v, err := NewCapabilityGrantValidator(
		"http://127.0.0.1:1", // unreachable
		WithJWKSStalenessCounter(counter),
		WithJWKSCacheTTL(0),
	)
	require.NoError(t, err)

	// Inject stale state: mark as fetched with an expired TTL so
	// markFetchFailure sees hasKeys > 0.
	v.mu.Lock()
	v.jwksKeys = map[string]*ecdsa.PublicKey{"k": generateTestKeys(t).pub}
	v.jwksFetched = time.Now().Add(-time.Hour)
	v.jwksStaleAt = time.Now().Add(time.Hour)
	v.mu.Unlock()

	v.markFetchFailure()
	assert.Equal(t, 1, count, "staleness counter should have been incremented")
}

type countingCounter struct{ inc func() }

func (c *countingCounter) Inc() { c.inc() }

// -----------------------------------------------------------------------
// Validate iat missing path
// -----------------------------------------------------------------------

func TestCapabilityGrant_MissingIAT(t *testing.T) {
	k := generateTestKeys(t)
	srv, _ := jwksServer(t, k)

	v, err := NewCapabilityGrantValidator(srv.URL)
	require.NoError(t, err)

	// Mint without IssuedAt — use RS256 to avoid ES256 validation complicating things.
	// Actually, mint using our helper but then strip IssuedAt from the JWT's claims.
	// We can do this by constructing a jwt.Token directly.
	type claimsWithoutIAT struct {
		jwt.RegisteredClaims
		InputsHash string `json:"gibson:inputs_hash"`
		Tenant     string `json:"gibson:tenant"`
	}

	now := time.Now().UTC()
	claims := &claimsWithoutIAT{
		RegisteredClaims: jwt.RegisteredClaims{
			Issuer:    "ext-authz.test",
			Subject:   "agent-1",
			Audience:  jwt.ClaimStrings{"tool-http"},
			ExpiresAt: jwt.NewNumericDate(now.Add(30 * time.Second)),
			ID:        "jti-no-iat",
			// IssuedAt intentionally nil
		},
		InputsHash: "aabbccdd",
		Tenant:     "tenant-uuid",
	}
	token := jwt.NewWithClaims(jwt.SigningMethodES256, claims)
	token.Header["kid"] = k.kid
	signed, err := token.SignedString(k.priv)
	require.NoError(t, err)

	_, err = v.ValidateCapabilityGrant(context.Background(), signed, "tool-http", "aabbccdd")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "iat")
}
