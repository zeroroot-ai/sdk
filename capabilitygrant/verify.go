// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Zero Root AI

package capabilitygrant

import (
	"context"
	"crypto/ed25519"
	"errors"
	"fmt"
	"strconv"
	"time"

	"github.com/golang-jwt/jwt/v5"

	"github.com/zeroroot-ai/sdk/auth"
)

// JWKSFetcher abstracts the source of the daemon's CG-JWT signing
// public keys. ext-authz supplies a concrete fetcher backed by an
// HTTP+cache implementation that polls /.well-known/jwks.json on the
// daemon (through Envoy). Tests inject a fetcher that returns
// in-memory keys.
//
// Implementations MUST be safe for concurrent use.
type JWKSFetcher interface {
	// Fetch returns the public key for the given JWS key id. The
	// returned key is a typed crypto.PublicKey; callers cast to the
	// expected algorithm (ed25519.PublicKey for Gibson CG-JWTs).
	Fetch(ctx context.Context, kid string) (any, error)
}

// VerifyOptions is the per-call configuration for Verify.
type VerifyOptions struct {
	// ExpectedIssuer is the iss value the CG-JWT must carry. Required.
	ExpectedIssuer string

	// ExpectedAudience is the aud value the CG-JWT must carry.
	// Required.
	ExpectedAudience string

	// Now is the reference time for expiry. Defaults to time.Now() when
	// zero.
	Now time.Time
}

// ErrSignature is returned when the CG-JWT signature does not verify.
// Wrapped with errors.Is.
var ErrSignature = errors.New("capabilitygrant: signature invalid")

// ErrUnknownKey is returned when the JWKS fetcher cannot supply the
// key identified by the JWT's kid header. Wrapped with errors.Is.
var ErrUnknownKey = errors.New("capabilitygrant: unknown key id")

// ErrMalformed is returned when the JWT cannot be parsed at all (bad
// base64, missing kid, etc.). Wrapped with errors.Is.
var ErrMalformed = errors.New("capabilitygrant: malformed token")

// Verify parses, signature-checks, and structurally validates a
// capability-grant JWT. It is the single entry point for ext-authz
// (and any future verifier) — the daemon never calls Verify, only
// mints (see internal/capabilitygrant.Mint in zeroroot-ai/gibson).
//
// On success, returns the typed Claims. On failure, returns one of:
//
//   - ErrMalformed if the token cannot be parsed
//   - ErrUnknownKey if the JWS kid is not in the JWKS
//   - ErrSignature if the signature does not verify
//   - ErrClaimsInvalid if any required claim is missing or invalid
//   - ErrExpired if exp is in the past
//
// Each error is wrapped with errors.Is for caller matching.
func Verify(ctx context.Context, fetcher JWKSFetcher, token string, opts VerifyOptions) (Claims, error) {
	if opts.Now.IsZero() {
		opts.Now = time.Now()
	}

	// Parse with EdDSA (Ed25519) — the only signing algorithm the
	// daemon emits. Reject any other alg up-front to prevent algorithm
	// confusion attacks (e.g. an attacker presenting an HMAC-signed
	// token under a public key).
	parsed, err := jwt.Parse(token, func(t *jwt.Token) (any, error) {
		if t.Method.Alg() != jwt.SigningMethodEdDSA.Alg() {
			return nil, fmt.Errorf("%w: unexpected alg %q", ErrSignature, t.Method.Alg())
		}
		kid, _ := t.Header["kid"].(string)
		if kid == "" {
			return nil, fmt.Errorf("%w: missing kid", ErrMalformed)
		}
		key, err := fetcher.Fetch(ctx, kid)
		if err != nil {
			return nil, fmt.Errorf("%w: %s: %w", ErrUnknownKey, kid, err)
		}
		pub, ok := key.(ed25519.PublicKey)
		if !ok {
			return nil, fmt.Errorf("%w: kid %s returned non-ed25519 key", ErrUnknownKey, kid)
		}
		return pub, nil
	}, jwt.WithValidMethods([]string{jwt.SigningMethodEdDSA.Alg()}))

	if err != nil {
		// jwt.Parse returns a layered error; preserve our wrapping if
		// the keyfunc returned one of our sentinel errors. We map
		// signature failures (jwt v5 exposes both ErrTokenSignatureInvalid
		// and ErrSignatureInvalid) to ErrSignature so callers can match.
		switch {
		case errors.Is(err, ErrUnknownKey), errors.Is(err, ErrMalformed):
			return Claims{}, err
		case errors.Is(err, jwt.ErrTokenSignatureInvalid),
			errors.Is(err, jwt.ErrSignatureInvalid):
			return Claims{}, fmt.Errorf("%w: %w", ErrSignature, err)
		default:
			return Claims{}, fmt.Errorf("%w: %w", ErrMalformed, err)
		}
	}

	if !parsed.Valid {
		return Claims{}, ErrMalformed
	}

	// Decode claims into our typed Claims. We use the bare MapClaims
	// path (rather than RegisteredClaims) so we can carry the Gibson-
	// specific fields without juggling embedded structs.
	mc, ok := parsed.Claims.(jwt.MapClaims)
	if !ok {
		return Claims{}, fmt.Errorf("%w: claims not a map", ErrMalformed)
	}

	claims, err := claimsFromMap(mc)
	if err != nil {
		return Claims{}, err
	}

	if err := claims.Validate(opts.Now, opts.ExpectedIssuer, opts.ExpectedAudience); err != nil {
		return Claims{}, err
	}

	return claims, nil
}

// claimsFromMap projects a jwt.MapClaims into our typed Claims
// struct. Each field is type-asserted defensively; missing or
// wrong-type fields produce ErrMalformed.
func claimsFromMap(mc jwt.MapClaims) (Claims, error) {
	c := Claims{}

	// Standard registered claims.
	c.Issuer = stringClaim(mc, "iss")
	c.Audience = audienceClaim(mc, "aud")
	c.Subject = stringClaim(mc, "sub")
	c.JTI = stringClaim(mc, "jti")

	if iat, ok := unixSecondsClaim(mc, "iat"); ok {
		c.IssuedAt = iat
	}
	if exp, ok := unixSecondsClaim(mc, "exp"); ok {
		c.ExpiresAt = exp
	}

	// Gibson-specific claims.
	tenantStr := stringClaim(mc, "tenant")
	if tenantStr == "" {
		return Claims{}, fmt.Errorf("%w: missing tenant claim", ErrClaimsInvalid)
	}
	tid, err := auth.NewTenantID(tenantStr)
	if err != nil {
		return Claims{}, fmt.Errorf("%w: tenant claim invalid: %w", ErrClaimsInvalid, err)
	}
	c.Tenant = tid

	c.MissionID = stringClaim(mc, "mission_id")
	c.TaskID = stringClaim(mc, "task_id")

	if rawAllowed, ok := mc["allowed_rpcs"].([]any); ok {
		c.AllowedRPCs = make([]string, 0, len(rawAllowed))
		for _, v := range rawAllowed {
			if s, ok := v.(string); ok {
				c.AllowedRPCs = append(c.AllowedRPCs, s)
			} else {
				return Claims{}, fmt.Errorf("%w: allowed_rpcs entry not a string", ErrClaimsInvalid)
			}
		}
	}

	return c, nil
}

func stringClaim(mc jwt.MapClaims, key string) string {
	v, _ := mc[key].(string)
	return v
}

// audienceClaim reads the aud claim. Per RFC 7519 it can be either a
// single string or a list; we accept both forms but only the first
// element of a list (the daemon mints single-aud CG-JWTs).
func audienceClaim(mc jwt.MapClaims, key string) string {
	v := mc[key]
	switch t := v.(type) {
	case string:
		return t
	case []any:
		if len(t) > 0 {
			if s, ok := t[0].(string); ok {
				return s
			}
		}
	}
	return ""
}

// unixSecondsClaim reads a numeric claim (iat, exp) and returns it as
// a UTC time.Time. The default golang-jwt decoder writes numeric
// claims as float64; we also accept int64 and string forms for
// resilience against alternative MapClaims producers.
func unixSecondsClaim(mc jwt.MapClaims, key string) (time.Time, bool) {
	v := mc[key]
	switch t := v.(type) {
	case float64:
		return time.Unix(int64(t), 0).UTC(), true
	case int64:
		return time.Unix(t, 0).UTC(), true
	case int:
		return time.Unix(int64(t), 0).UTC(), true
	case string:
		i, err := strconv.ParseInt(t, 10, 64)
		if err != nil {
			return time.Time{}, false
		}
		return time.Unix(i, 0).UTC(), true
	}
	return time.Time{}, false
}
