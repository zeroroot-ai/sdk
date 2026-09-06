// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Zero Root AI

package secrets

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/zeroroot-ai/sdk/plugin/manifest"
	"golang.org/x/sync/singleflight"
)

// Client is the plugin author's API surface for credential resolution.
//
// Internally it calls the daemon's HarnessCallbackService.GetCredential RPC
// (via a caller-supplied GetCredentialFn) and caches results in an in-process
// LRU cache with configurable TTL and single-flight deduplication.
//
// Spec: plugin-runtime Requirement 3.
type Client interface {
	// Resolve fetches a secret value by name. It validates that name is
	// declared in the plugin's manifest spec.secrets before any RPC.
	//
	// Error semantics:
	//   - ErrInvalidArgument: name is not in the manifest.
	//   - ErrPermissionDenied: the secret has been revoked via
	//     MarkRevoked; no RPC is attempted.
	//   - Any error from GetCredentialFn is returned unwrapped.
	Resolve(ctx context.Context, name string, opts ...Option) ([]byte, error)

	// Invalidate drops the cache entry for name. Called by the events
	// subscriber on a secret_rotated event so the next Resolve fetches a
	// fresh value.
	Invalidate(name string)

	// MarkRevoked sets a permanent denied flag for name. All subsequent
	// Resolve calls for this name return ErrPermissionDenied
	// without invoking the RPC.
	//
	// In v1 the revoked flag is never automatically cleared. The events
	// subscriber calls MarkRevoked on secret_access_revoked and would call
	// a future ClearRevoked on a re-grant event; that method is out of scope
	// for this version and is documented as a deliberate gap.
	MarkRevoked(name string)
}

// Option configures a single Resolve call.
type Option func(*resolveOpts)

type resolveOpts struct {
	useCache *bool // nil means "use the client default (true)"
}

// WithCache overrides the client-level caching behaviour for this single
// Resolve call. Pass false to force an RPC even when a cached value exists.
func WithCache(enabled bool) Option {
	return func(o *resolveOpts) {
		o.useCache = &enabled
	}
}

// GetCredentialFn is the function the secrets client calls to fetch a raw
// credential value when the cache is empty.
//
// In production this is wired to the daemon's
// HarnessCallbackService.GetCredential callback channel. In tests it is
// replaced by a fake.
type GetCredentialFn func(ctx context.Context, name string) ([]byte, error)

// CacheConfig holds optional constructor-level cache settings.
// The zero value applies all defaults.
type CacheConfig struct {
	// TTL is the per-entry cache lifetime. Must be in (0, MaxCacheTTL].
	// Defaults to DefaultCacheTTL (60s) when zero.
	TTL time.Duration

	// MaxSize is the LRU cache capacity in entries.
	// Defaults to DefaultCacheSize (1000) when zero.
	MaxSize int
}

// client is the production implementation of Client.
type client struct {
	manifest *manifest.Manifest
	callRPC  GetCredentialFn
	c        *cache
	sfg      singleflight.Group

	// revokedMu protects the revoked set.
	revokedMu sync.RWMutex
	revoked   map[string]struct{}

	// allowedNames is built once from the manifest for O(1) lookup.
	allowedNames map[string]struct{}
}

// New constructs a Client that validates names against the plugin's manifest
// and fetches values via callRPC when not cached.
//
// cacheConf configures the in-process LRU cache; the zero value applies all
// defaults.
func New(m *manifest.Manifest, callRPC GetCredentialFn, cacheConf CacheConfig) Client {
	ttl := cacheConf.TTL
	if ttl <= 0 {
		ttl = DefaultCacheTTL
	}
	if ttl > MaxCacheTTL {
		ttl = MaxCacheTTL
	}
	size := cacheConf.MaxSize
	if size <= 0 {
		size = DefaultCacheSize
	}

	allowed := make(map[string]struct{}, len(m.Spec.Secrets))
	for _, s := range m.Spec.Secrets {
		allowed[s.Name] = struct{}{}
	}

	return &client{
		manifest:     m,
		callRPC:      callRPC,
		c:            newCache(size, ttl),
		revoked:      make(map[string]struct{}),
		allowedNames: allowed,
	}
}

// Resolve implements Client.
func (cl *client) Resolve(ctx context.Context, name string, opts ...Option) ([]byte, error) {
	// Step 1: validate name against manifest.
	if _, ok := cl.allowedNames[name]; !ok {
		return nil, fmt.Errorf("secret %q is not declared in this plugin's manifest "+
			"spec.secrets — declare it before consuming: %w", name, ErrInvalidArgument)
	}

	// Step 2: check revoked flag.
	cl.revokedMu.RLock()
	_, isRevoked := cl.revoked[name]
	cl.revokedMu.RUnlock()
	if isRevoked {
		return nil, ErrPermissionDenied
	}

	// Resolve per-call options.
	ro := resolveOpts{}
	for _, opt := range opts {
		opt(&ro)
	}
	cacheEnabled := ro.useCache == nil || *ro.useCache

	// Step 3: cache lookup (skipped when WithCache(false)).
	if cacheEnabled {
		if v, ok := cl.c.get(name); ok {
			return v, nil
		}
	}

	// Step 4: cache miss — call RPC via singleflight.
	val, err, _ := cl.sfg.Do(name, func() (interface{}, error) {
		v, e := cl.callRPC(ctx, name)
		if e != nil {
			return nil, e
		}
		return v, nil
	})
	if err != nil {
		// Do not cache negative results for ErrNotFound / ErrPermissionDenied.
		return nil, err
	}

	value := val.([]byte)
	if cacheEnabled {
		cl.c.set(name, value)
	}

	// Defensive copy for the caller (cache stores its own copy already).
	out := make([]byte, len(value))
	copy(out, value)
	return out, nil
}

// Invalidate implements Client.
func (cl *client) Invalidate(name string) {
	cl.c.delete(name)
}

// MarkRevoked implements Client.
func (cl *client) MarkRevoked(name string) {
	cl.c.delete(name)

	cl.revokedMu.Lock()
	cl.revoked[name] = struct{}{}
	cl.revokedMu.Unlock()
}
