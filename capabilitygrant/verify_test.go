// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Zero Root AI

package capabilitygrant

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"errors"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

// fakeFetcher is a JWKSFetcher backed by in-memory keys. Tests
// construct it with a kid → public key map.
type fakeFetcher map[string]any

func (f fakeFetcher) Fetch(_ context.Context, kid string) (any, error) {
	if k, ok := f[kid]; ok {
		return k, nil
	}
	return nil, errors.New("fake fetcher: not found")
}

func mustGenKey(t *testing.T) (ed25519.PublicKey, ed25519.PrivateKey) {
	t.Helper()
	pub, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	return pub, priv
}

func mintWithMap(t *testing.T, priv ed25519.PrivateKey, kid string, claims jwt.MapClaims) string {
	t.Helper()
	tok := jwt.NewWithClaims(jwt.SigningMethodEdDSA, claims)
	tok.Header["kid"] = kid
	signed, err := tok.SignedString(priv)
	if err != nil {
		t.Fatal(err)
	}
	return signed
}

func validClaimsMap(now time.Time) jwt.MapClaims {
	return jwt.MapClaims{
		"iss":          "https://daemon.example/gibson",
		"aud":          "gibson-daemon",
		"sub":          "agent-svc-acct-1",
		"tenant":       "acme",
		"mission_id":   "m-001",
		"task_id":      "t-001",
		"jti":          "jti-001",
		"iat":          now.Unix(),
		"exp":          now.Add(15 * time.Minute).Unix(),
		"allowed_rpcs": []any{"/gibson.harness.v1.HarnessCallbackService/LLMComplete"},
	}
}

func defaultOpts() VerifyOptions {
	return VerifyOptions{
		ExpectedIssuer:   "https://daemon.example/gibson",
		ExpectedAudience: "gibson-daemon",
	}
}

func TestVerify_HappyPath(t *testing.T) {
	pub, priv := mustGenKey(t)
	now := time.Now().UTC()
	tok := mintWithMap(t, priv, "k1", validClaimsMap(now))
	fetcher := fakeFetcher{"k1": pub}

	claims, err := Verify(context.Background(), fetcher, tok, defaultOpts())
	if err != nil {
		t.Fatalf("expected ok, got %v", err)
	}
	if claims.Subject != "agent-svc-acct-1" {
		t.Errorf("subject: %q", claims.Subject)
	}
	if claims.Tenant.String() != "acme" {
		t.Errorf("tenant: %q", claims.Tenant.String())
	}
	if !claims.AllowsMethod("/gibson.harness.v1.HarnessCallbackService/LLMComplete") {
		t.Errorf("expected method allowed")
	}
	if claims.AllowsMethod("/gibson.harness.v1.HarnessCallbackService/SubmitFinding") {
		t.Errorf("unexpected method allowed")
	}
}

func TestVerify_Expired(t *testing.T) {
	pub, priv := mustGenKey(t)
	now := time.Now().UTC()
	mc := validClaimsMap(now.Add(-1 * time.Hour))
	mc["exp"] = now.Add(-30 * time.Minute).Unix()
	tok := mintWithMap(t, priv, "k1", mc)
	_, err := Verify(context.Background(), fakeFetcher{"k1": pub}, tok, defaultOpts())
	// jwt v5 enforces exp during Parse, returning jwt.ErrTokenExpired,
	// which our wrapper folds into ErrSignature/ErrMalformed paths.
	// Either is acceptable as long as the token is rejected.
	if err == nil {
		t.Fatal("expected rejection")
	}
}

func TestVerify_LifetimeTooLong(t *testing.T) {
	pub, priv := mustGenKey(t)
	now := time.Now().UTC()
	mc := validClaimsMap(now)
	mc["exp"] = now.Add(MaxLifetime + time.Minute).Unix()
	tok := mintWithMap(t, priv, "k1", mc)
	_, err := Verify(context.Background(), fakeFetcher{"k1": pub}, tok, defaultOpts())
	if !errors.Is(err, ErrClaimsInvalid) {
		t.Fatalf("expected ErrClaimsInvalid, got %v", err)
	}
}

func TestVerify_BadSignature(t *testing.T) {
	pubGood, privBad := mustGenKey(t)
	_ = pubGood
	_, privAttacker := mustGenKey(t)
	now := time.Now().UTC()
	tok := mintWithMap(t, privAttacker, "k1", validClaimsMap(now))
	_, err := Verify(context.Background(), fakeFetcher{"k1": ed25519.PublicKey(privBad.Public().(ed25519.PublicKey))}, tok, defaultOpts())
	if err == nil || !errors.Is(err, ErrSignature) {
		t.Fatalf("expected ErrSignature, got %v", err)
	}
}

func TestVerify_UnknownKid(t *testing.T) {
	_, priv := mustGenKey(t)
	now := time.Now().UTC()
	tok := mintWithMap(t, priv, "unknown-kid", validClaimsMap(now))
	_, err := Verify(context.Background(), fakeFetcher{"k1": ed25519.PublicKey([]byte{})}, tok, defaultOpts())
	if !errors.Is(err, ErrUnknownKey) {
		t.Fatalf("expected ErrUnknownKey, got %v", err)
	}
}

func TestVerify_MissingKid(t *testing.T) {
	_, priv := mustGenKey(t)
	now := time.Now().UTC()
	tok := jwt.NewWithClaims(jwt.SigningMethodEdDSA, validClaimsMap(now))
	// Intentionally omit kid header.
	signed, err := tok.SignedString(priv)
	if err != nil {
		t.Fatal(err)
	}
	_, err = Verify(context.Background(), fakeFetcher{}, signed, defaultOpts())
	if !errors.Is(err, ErrMalformed) {
		t.Fatalf("expected ErrMalformed, got %v", err)
	}
}

func TestVerify_AlgConfusion_HMAC(t *testing.T) {
	// An attacker presents a token signed with HMAC under a public key
	// they fetched from JWKS; the verifier must reject before fetching
	// the key.
	now := time.Now().UTC()
	tok := jwt.NewWithClaims(jwt.SigningMethodHS256, validClaimsMap(now))
	tok.Header["kid"] = "k1"
	signed, err := tok.SignedString([]byte("secret"))
	if err != nil {
		t.Fatal(err)
	}
	pub, _ := mustGenKey(t)
	_, err = Verify(context.Background(), fakeFetcher{"k1": pub}, signed, defaultOpts())
	if err == nil {
		t.Fatal("expected rejection of HMAC-signed token")
	}
}

func TestVerify_WrongIssuer(t *testing.T) {
	pub, priv := mustGenKey(t)
	now := time.Now().UTC()
	mc := validClaimsMap(now)
	mc["iss"] = "https://attacker.example"
	tok := mintWithMap(t, priv, "k1", mc)
	_, err := Verify(context.Background(), fakeFetcher{"k1": pub}, tok, defaultOpts())
	if !errors.Is(err, ErrClaimsInvalid) {
		t.Fatalf("expected ErrClaimsInvalid, got %v", err)
	}
}

func TestVerify_WrongAudience(t *testing.T) {
	pub, priv := mustGenKey(t)
	now := time.Now().UTC()
	mc := validClaimsMap(now)
	mc["aud"] = "some-other-daemon"
	tok := mintWithMap(t, priv, "k1", mc)
	_, err := Verify(context.Background(), fakeFetcher{"k1": pub}, tok, defaultOpts())
	if !errors.Is(err, ErrClaimsInvalid) {
		t.Fatalf("expected ErrClaimsInvalid, got %v", err)
	}
}

func TestVerify_MissingTenant(t *testing.T) {
	pub, priv := mustGenKey(t)
	now := time.Now().UTC()
	mc := validClaimsMap(now)
	delete(mc, "tenant")
	tok := mintWithMap(t, priv, "k1", mc)
	_, err := Verify(context.Background(), fakeFetcher{"k1": pub}, tok, defaultOpts())
	if !errors.Is(err, ErrClaimsInvalid) {
		t.Fatalf("expected ErrClaimsInvalid, got %v", err)
	}
}

func TestVerify_AllowedRPCsNonStringEntry(t *testing.T) {
	pub, priv := mustGenKey(t)
	now := time.Now().UTC()
	mc := validClaimsMap(now)
	mc["allowed_rpcs"] = []any{"/ok", 42}
	tok := mintWithMap(t, priv, "k1", mc)
	_, err := Verify(context.Background(), fakeFetcher{"k1": pub}, tok, defaultOpts())
	if !errors.Is(err, ErrClaimsInvalid) {
		t.Fatalf("expected ErrClaimsInvalid, got %v", err)
	}
}

func TestClaims_Validate_Independent(t *testing.T) {
	now := time.Now().UTC()
	c := Claims{
		Issuer:      "iss",
		Audience:    "aud",
		Subject:     "sub",
		MissionID:   "m",
		TaskID:      "t",
		JTI:         "j",
		IssuedAt:    now,
		ExpiresAt:   now.Add(time.Minute),
		AllowedRPCs: nil,
	}
	if err := c.Validate(now, "iss", "aud"); err == nil {
		t.Fatal("expected error: missing tenant")
	}
}
