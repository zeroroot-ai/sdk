// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Zero Root AI

// Package agent provides types and helpers for Gibson SDK agents.
//
// This file implements the CapabilityGrantValidator: a single-purpose
// validator for Capability Grant JWTs minted by ext_authz. Tool workers
// call ValidateCapabilityGrant on every inbound grant before executing
// the requested tool invocation.
//
// # JTI replay cache
//
// The replay cache is in-memory and per-process. It does NOT share state
// across replicas. A cluster of tool worker pods will therefore accept a
// replayed JTI on a different pod during the grant's TTL window. Given
// that grants expire within ≤60 s and tool worker pods are typically
// single-tenant, this is an acceptable trade-off. If stronger replay
// protection is required, replace replayLRU with a shared Redis/etcd store
// at the cost of network round-trips.
package agent

import (
	"container/list"
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/subtle"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"math/big"
	"net/http"
	"os"
	"sync"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

// -----------------------------------------------------------------------
// Public types
// -----------------------------------------------------------------------

// GrantClaims contains the decoded, validated claims from a Capability Grant JWT.
type GrantClaims struct {
	// Issuer is the JWT iss claim (e.g. "ext-authz.zeroroot.ai").
	Issuer string

	// AgentID is the JWT sub claim — the agent that requested the grant.
	AgentID string

	// ToolID is the JWT aud claim — the tool the agent is authorised to invoke.
	ToolID string

	// Tenant is the gibson:tenant claim — the owning tenant UUID.
	Tenant string

	// InputsHash is the gibson:inputs_hash claim — SHA-256 hex of the canonical
	// tool input proto provided by the dispatcher.
	InputsHash string

	// JTI is the unique token identifier used for replay detection.
	JTI string

	// IssuedAt is the time the grant was issued.
	IssuedAt time.Time

	// ExpiresAt is the time the grant expires.
	ExpiresAt time.Time
}

// JWKSStalenessCounter is the observability hook called when the JWKS cache
// becomes stale or unreachable. Implement this interface to record the event
// in your preferred metrics system. A nil counter is safe: it is treated as
// a no-op.
type JWKSStalenessCounter interface {
	Inc()
}

// CapabilityGrantValidator validates Capability Grant JWTs against the
// ext_authz JWKS endpoint. Construct once per process; safe for concurrent use.
type CapabilityGrantValidator struct {
	cfg    validatorConfig
	log    *slog.Logger
	client *http.Client

	mu          sync.RWMutex
	jwksKeys    map[string]*ecdsa.PublicKey // kid → public key
	jwksFetched time.Time                   // zero if never fetched
	jwksStaleAt time.Time                   // zero if never fetched; set to fetched+staleTTL

	replay       *replayLRU
	staleCounter JWKSStalenessCounter // nil → no-op
}

// ValidatorOption is a functional option for NewCapabilityGrantValidator.
type ValidatorOption func(*validatorConfig)

// WithJWKSCacheTTL sets how long a successful JWKS fetch is considered fresh.
// Default: 60 s.
func WithJWKSCacheTTL(d time.Duration) ValidatorOption {
	return func(c *validatorConfig) { c.cacheTTL = d }
}

// WithStaleJWKSWindow sets the maximum duration stale JWKS keys are served when
// the upstream is unreachable. After this window, ValidateCapabilityGrant returns
// an error instead of silently accepting tokens. Default: 5 m.
func WithStaleJWKSWindow(d time.Duration) ValidatorOption {
	return func(c *validatorConfig) { c.staleWindow = d }
}

// WithReplayCacheSize sets the maximum number of JTIs retained in the in-memory
// replay cache. When the cache is full the oldest entry is evicted. Default: 4096.
func WithReplayCacheSize(n int) ValidatorOption {
	return func(c *validatorConfig) { c.replayCacheSize = n }
}

// WithIssuer configures an expected issuer. When non-empty, ValidateCapabilityGrant
// rejects tokens whose iss claim does not match. Default: no issuer check.
func WithIssuer(iss string) ValidatorOption {
	return func(c *validatorConfig) { c.issuer = iss }
}

// WithJWKSStalenessCounter wires an observability hook that is called whenever
// the JWKS cache becomes stale or its upstream is unreachable. Use this to
// record the event in Prometheus, OpenTelemetry, or any other system. By
// default no counter is incremented.
func WithJWKSStalenessCounter(c JWKSStalenessCounter) ValidatorOption {
	return func(cfg *validatorConfig) { cfg.staleCounter = c }
}

// NewCapabilityGrantValidator creates a validator that fetches public keys from
// jwksURL. If jwksURL is empty the constructor reads the EXT_AUTHZ_JWKS_URL
// environment variable. Returns an error if no URL can be resolved.
//
// The constructor does NOT pre-fetch the JWKS; keys are fetched lazily on the
// first call to ValidateCapabilityGrant.
func NewCapabilityGrantValidator(jwksURL string, opts ...ValidatorOption) (*CapabilityGrantValidator, error) {
	if jwksURL == "" {
		jwksURL = os.Getenv("EXT_AUTHZ_JWKS_URL")
	}
	if jwksURL == "" {
		return nil, errors.New("capability grant validator: jwksURL is required (or set EXT_AUTHZ_JWKS_URL)")
	}

	cfg := validatorConfig{
		jwksURL:         jwksURL,
		cacheTTL:        60 * time.Second,
		staleWindow:     5 * time.Minute,
		replayCacheSize: 4096,
	}
	for _, o := range opts {
		o(&cfg)
	}

	v := &CapabilityGrantValidator{
		cfg:          cfg,
		log:          slog.Default(),
		client:       &http.Client{Timeout: 10 * time.Second},
		jwksKeys:     make(map[string]*ecdsa.PublicKey),
		replay:       newReplayLRU(cfg.replayCacheSize),
		staleCounter: cfg.staleCounter,
	}
	return v, nil
}

// ValidateCapabilityGrant verifies a Capability Grant JWT end-to-end:
//
//   - ES256 signature against the ext_authz JWKS
//   - exp / iat bounds (rejects future-dated iat > 5 s from now)
//   - audience equals expectedTool
//   - gibson:inputs_hash equals expectedInputsHash (constant-time comparison)
//   - JTI has not been seen before in this process
//   - Issuer matches the WithIssuer option if configured
//
// Returns typed GrantClaims on success, a descriptive error otherwise.
// Never logs the full JWT or the signature; sub and aud may appear at DEBUG level.
func (v *CapabilityGrantValidator) ValidateCapabilityGrant(
	ctx context.Context,
	rawJWT string,
	expectedTool string,
	expectedInputsHash string,
) (*GrantClaims, error) {
	keyFunc := func(token *jwt.Token) (any, error) {
		if token.Method.Alg() != jwt.SigningMethodES256.Alg() {
			return nil, fmt.Errorf("unexpected signing algorithm: %q", token.Method.Alg())
		}
		kid, _ := token.Header["kid"].(string)
		key, err := v.resolveKey(ctx, kid)
		if err != nil {
			return nil, err
		}
		return key, nil
	}

	var raw rawGrantClaims
	token, err := jwt.ParseWithClaims(rawJWT, &raw, keyFunc,
		jwt.WithExpirationRequired(),
		jwt.WithAudience(expectedTool),
		jwt.WithLeeway(0),
	)
	if err != nil {
		return nil, fmt.Errorf("capability grant: JWT parse/verify failed: %w", err)
	}
	if !token.Valid {
		return nil, errors.New("capability grant: token invalid")
	}

	// Reject future-dated iat (> 5 s clock skew). We perform this check manually
	// rather than using jwt.WithIssuedAt() so that we control the tolerance window.
	if raw.IssuedAt == nil {
		return nil, errors.New("capability grant: iat claim is missing")
	}
	if time.Until(raw.IssuedAt.Time) > 5*time.Second {
		return nil, errors.New("capability grant: iat is too far in the future")
	}

	// Issuer check (optional).
	if v.cfg.issuer != "" && raw.Issuer != v.cfg.issuer {
		return nil, fmt.Errorf("capability grant: issuer mismatch: got %q, want %q", raw.Issuer, v.cfg.issuer)
	}

	// Audience: jwt.WithAudience enforces the string is present, but the library
	// checks membership in a []string audience. We additionally verify exact match
	// with expectedTool after parsing.
	audOK := false
	for _, a := range raw.Audience {
		if a == expectedTool {
			audOK = true
			break
		}
	}
	if !audOK {
		return nil, fmt.Errorf("capability grant: audience mismatch: expected %q", expectedTool)
	}

	// Inputs hash — constant-time comparison.
	if subtle.ConstantTimeCompare([]byte(raw.InputsHash), []byte(expectedInputsHash)) != 1 {
		return nil, errors.New("capability grant: inputs_hash mismatch")
	}

	if raw.InputsHash == "" {
		return nil, errors.New("capability grant: gibson:inputs_hash claim is missing or empty")
	}
	if raw.Tenant == "" {
		return nil, errors.New("capability grant: gibson:tenant claim is missing or empty")
	}
	if raw.ID == "" {
		return nil, errors.New("capability grant: jti claim is missing or empty")
	}

	// JTI replay detection.
	if !v.replay.add(raw.ID) {
		return nil, fmt.Errorf("capability grant: jti %q has already been used (replay detected)", raw.ID)
	}

	sub := raw.Subject
	aud := expectedTool
	v.log.DebugContext(ctx, "capability grant validated", "sub", sub, "aud", aud, "tenant", raw.Tenant)

	var issuedAt, expiresAt time.Time
	if raw.IssuedAt != nil {
		issuedAt = raw.IssuedAt.Time
	}
	if raw.ExpiresAt != nil {
		expiresAt = raw.ExpiresAt.Time
	}

	return &GrantClaims{
		Issuer:     raw.Issuer,
		AgentID:    raw.Subject,
		ToolID:     expectedTool,
		Tenant:     raw.Tenant,
		InputsHash: raw.InputsHash,
		JTI:        raw.ID,
		IssuedAt:   issuedAt,
		ExpiresAt:  expiresAt,
	}, nil
}

// -----------------------------------------------------------------------
// Internal types
// -----------------------------------------------------------------------

// validatorConfig holds the resolved configuration for a CapabilityGrantValidator.
type validatorConfig struct {
	jwksURL         string
	cacheTTL        time.Duration
	staleWindow     time.Duration
	replayCacheSize int
	issuer          string
	staleCounter    JWKSStalenessCounter // nil → no-op
}

// rawGrantClaims is the jwt.Claims implementation used during parsing.
type rawGrantClaims struct {
	jwt.RegisteredClaims
	InputsHash string `json:"gibson:inputs_hash"`
	Tenant     string `json:"gibson:tenant"`
}

// -----------------------------------------------------------------------
// JWKS fetching and key resolution
// -----------------------------------------------------------------------

// jwkEC mirrors the JWKS JSON structure produced by ext_authz.
type jwkEC struct {
	KTY string `json:"kty"`
	CRV string `json:"crv"`
	KID string `json:"kid"`
	Alg string `json:"alg"`
	X   string `json:"x"`
	Y   string `json:"y"`
}

type jwkSet struct {
	Keys []jwkEC `json:"keys"`
}

// resolveKey returns the ECDSA public key for kid, refreshing the JWKS cache if needed.
// If kid is empty the first available key is returned (single-key deployments).
func (v *CapabilityGrantValidator) resolveKey(ctx context.Context, kid string) (*ecdsa.PublicKey, error) {
	// Fast path: read lock.
	v.mu.RLock()
	key, needsRefresh, staleExpired := v.lookupKeyLocked(kid)
	v.mu.RUnlock()

	if key != nil && !needsRefresh {
		return key, nil
	}

	if staleExpired {
		// Stale window exceeded — must not continue.
		v.incStaleCounter()
		v.log.Error("capability grant: JWKS stale window exceeded; rejecting validation",
			"jwks_url", v.cfg.jwksURL,
		)
		return nil, errors.New("capability grant: JWKS upstream unreachable and stale window exceeded")
	}

	// Refresh in background if we have a usable cached key; block if we don't.
	if key != nil {
		// Serve stale while refreshing in background.
		go v.refreshJWKS(context.Background()) //nolint:errcheck
		return key, nil
	}

	// No cached key at all — block on fetch.
	if err := v.refreshJWKS(ctx); err != nil {
		return nil, fmt.Errorf("capability grant: JWKS fetch failed: %w", err)
	}

	v.mu.RLock()
	key, _, _ = v.lookupKeyLocked(kid)
	v.mu.RUnlock()

	if key == nil {
		return nil, fmt.Errorf("capability grant: no key found for kid %q in JWKS", kid)
	}
	return key, nil
}

// lookupKeyLocked reads the in-memory JWKS cache under the read-lock.
// Returns (key, needsRefresh, staleExpired).
func (v *CapabilityGrantValidator) lookupKeyLocked(kid string) (key *ecdsa.PublicKey, needsRefresh bool, staleExpired bool) {
	never := v.jwksFetched.IsZero()
	if never {
		return nil, true, false
	}

	expired := time.Since(v.jwksFetched) > v.cfg.cacheTTL
	if !v.jwksStaleAt.IsZero() && time.Now().After(v.jwksStaleAt) {
		staleExpired = true
	}

	if kid == "" {
		// Return any key.
		for _, k := range v.jwksKeys {
			return k, expired, staleExpired
		}
		return nil, true, staleExpired
	}

	k, ok := v.jwksKeys[kid]
	if !ok {
		// Unknown kid — force refresh.
		return nil, true, staleExpired
	}
	return k, expired, staleExpired
}

// refreshJWKS fetches the JWKS endpoint and updates the in-memory key map.
func (v *CapabilityGrantValidator) refreshJWKS(ctx context.Context) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, v.cfg.jwksURL, http.NoBody)
	if err != nil {
		return err
	}
	req.Header.Set("Accept", "application/json")

	resp, err := v.client.Do(req)
	if err != nil {
		v.markFetchFailure()
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		v.markFetchFailure()
		return fmt.Errorf("JWKS endpoint returned HTTP %d", resp.StatusCode)
	}

	body, err := io.ReadAll(io.LimitReader(resp.Body, 64*1024))
	if err != nil {
		v.markFetchFailure()
		return err
	}

	var ks jwkSet
	if err := json.Unmarshal(body, &ks); err != nil {
		v.markFetchFailure()
		return fmt.Errorf("JWKS JSON unmarshal: %w", err)
	}

	keys := make(map[string]*ecdsa.PublicKey, len(ks.Keys))
	for _, jwk := range ks.Keys {
		if jwk.KTY != "EC" || jwk.CRV != "P-256" {
			continue
		}
		pub, err := jwkToECDSA(jwk)
		if err != nil {
			v.log.Warn("capability grant: skipping malformed JWK", "kid", jwk.KID, "err", err)
			continue
		}
		keys[jwk.KID] = pub
	}

	if len(keys) == 0 {
		v.markFetchFailure()
		return errors.New("JWKS contains no usable EC P-256 keys")
	}

	v.mu.Lock()
	v.jwksKeys = keys
	v.jwksFetched = time.Now()
	v.jwksStaleAt = v.jwksFetched.Add(v.cfg.staleWindow)
	v.mu.Unlock()

	v.log.Debug("capability grant: JWKS refreshed", "key_count", len(keys))
	return nil
}

// markFetchFailure is called after a failed JWKS fetch. If the stale window
// has not been set yet (i.e., we have never fetched successfully) this is a no-op;
// the caller will report the error directly. If we have a prior successful fetch,
// the existing staleAt deadline is left unchanged so callers can continue serving
// the cached keys until the window expires.
func (v *CapabilityGrantValidator) markFetchFailure() {
	v.mu.RLock()
	hasKeys := len(v.jwksKeys) > 0
	v.mu.RUnlock()
	if hasKeys {
		v.incStaleCounter()
		v.log.Warn("capability grant: JWKS fetch failed; serving stale keys", "jwks_url", v.cfg.jwksURL)
	}
}

// jwkToECDSA converts a JWK EC object (P-256) to an *ecdsa.PublicKey.
func jwkToECDSA(j jwkEC) (*ecdsa.PublicKey, error) {
	xBytes, err := base64.RawURLEncoding.DecodeString(j.X)
	if err != nil {
		return nil, fmt.Errorf("decode x: %w", err)
	}
	yBytes, err := base64.RawURLEncoding.DecodeString(j.Y)
	if err != nil {
		return nil, fmt.Errorf("decode y: %w", err)
	}
	return &ecdsa.PublicKey{
		Curve: elliptic.P256(),
		X:     new(big.Int).SetBytes(xBytes),
		Y:     new(big.Int).SetBytes(yBytes),
	}, nil
}

// -----------------------------------------------------------------------
// JTI replay cache (in-memory LRU, per-process)
// -----------------------------------------------------------------------

// replayLRU is a fixed-size LRU set of JTI strings used to detect token replay.
// It is safe for concurrent use.
type replayLRU struct {
	mu      sync.Mutex
	maxSize int
	items   map[string]*list.Element
	order   *list.List
}

func newReplayLRU(size int) *replayLRU {
	if size <= 0 {
		size = 4096
	}
	return &replayLRU{
		maxSize: size,
		items:   make(map[string]*list.Element, size),
		order:   list.New(),
	}
}

// add records jti and returns true if it is new, false if it was already seen (replay).
func (r *replayLRU) add(jti string) bool {
	r.mu.Lock()
	defer r.mu.Unlock()

	if _, exists := r.items[jti]; exists {
		return false // replay
	}

	elem := r.order.PushFront(jti)
	r.items[jti] = elem

	if r.order.Len() > r.maxSize {
		oldest := r.order.Back()
		if oldest != nil {
			r.order.Remove(oldest)
			delete(r.items, oldest.Value.(string))
		}
	}
	return true
}

// incStaleCounter increments the optional staleness counter if one was wired.
func (v *CapabilityGrantValidator) incStaleCounter() {
	if v.staleCounter != nil {
		v.staleCounter.Inc()
	}
}
