// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Zero Root AI

package secrets

import (
	"context"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/zeroroot-ai/sdk/plugin/manifest"
)

// minimalManifest builds a valid Manifest containing the listed secret names.
func minimalManifest(secretNames ...string) *manifest.Manifest {
	decls := make([]manifest.SecretDecl, len(secretNames))
	for i, n := range secretNames {
		decls[i] = manifest.SecretDecl{
			Name:     n,
			Scope:    "startup",
			Rotation: "live",
			Required: true,
		}
	}
	return &manifest.Manifest{
		APIVersion: manifest.APIVersionV1,
		Kind:       manifest.KindPlugin,
		Metadata: manifest.ManifestMetadata{
			Name:    "test-plugin",
			Version: "0.1.0",
		},
		Spec: manifest.ManifestSpec{
			WorkloadClass: manifest.WorkloadClassPlugin,
			Secrets:       decls,
			Methods: []manifest.MethodDecl{
				{
					Name: "DoStuff",
				},
			},
			Runtime: "process",
		},
	}
}

func TestClient_ManifestValidation_NotDeclared(t *testing.T) {
	m := minimalManifest("cred:db_password")
	calls := 0
	fn := func(_ context.Context, _ string) ([]byte, error) {
		calls++
		return []byte("val"), nil
	}
	cl := New(m, fn, CacheConfig{})

	_, err := cl.Resolve(context.Background(), "cred:undeclared")
	require.Error(t, err)
	require.ErrorIs(t, err, ErrInvalidArgument,
		"expected ErrInvalidArgument, got %v", err)
	assert.Equal(t, 0, calls, "RPC must not be called for undeclared name")
}

func TestClient_CacheMiss_CallsRPC(t *testing.T) {
	m := minimalManifest("cred:api_key")
	calls := 0
	fn := func(_ context.Context, _ string) ([]byte, error) {
		calls++
		return []byte("secret-value"), nil
	}
	cl := New(m, fn, CacheConfig{})

	v, err := cl.Resolve(context.Background(), "cred:api_key")
	require.NoError(t, err)
	assert.Equal(t, []byte("secret-value"), v)
	assert.Equal(t, 1, calls)
}

func TestClient_CacheHit_NoRPC(t *testing.T) {
	m := minimalManifest("cred:api_key")
	calls := 0
	fn := func(_ context.Context, _ string) ([]byte, error) {
		calls++
		return []byte("secret-value"), nil
	}
	cl := New(m, fn, CacheConfig{})

	_, err := cl.Resolve(context.Background(), "cred:api_key")
	require.NoError(t, err)
	_, err = cl.Resolve(context.Background(), "cred:api_key")
	require.NoError(t, err)

	assert.Equal(t, 1, calls, "second Resolve must use cached value")
}

func TestClient_TTLExpiry_RefetchesAfterExpiry(t *testing.T) {
	m := minimalManifest("cred:token")
	calls := 0
	fn := func(_ context.Context, _ string) ([]byte, error) {
		calls++
		return []byte("token"), nil
	}

	ttl := 50 * time.Millisecond
	cl := New(m, fn, CacheConfig{TTL: ttl})

	_, err := cl.Resolve(context.Background(), "cred:token")
	require.NoError(t, err)
	assert.Equal(t, 1, calls)

	time.Sleep(2 * ttl)

	_, err = cl.Resolve(context.Background(), "cred:token")
	require.NoError(t, err)
	assert.Equal(t, 2, calls, "RPC should be called again after TTL expiry")
}

func TestClient_SingleFlight_ConcurrentMiss(t *testing.T) {
	m := minimalManifest("cred:api_key")

	var callCount int64
	gate := make(chan struct{})
	fn := func(_ context.Context, _ string) ([]byte, error) {
		<-gate // block until all goroutines are in-flight
		atomic.AddInt64(&callCount, 1)
		return []byte("value"), nil
	}

	cl := New(m, fn, CacheConfig{})

	const n = 1000
	var wg sync.WaitGroup
	wg.Add(n)
	for range n {
		go func() {
			defer wg.Done()
			_, _ = cl.Resolve(context.Background(), "cred:api_key")
		}()
	}

	// Let all goroutines accumulate, then release.
	time.Sleep(10 * time.Millisecond)
	close(gate)
	wg.Wait()

	assert.Equal(t, int64(1), atomic.LoadInt64(&callCount),
		"singleflight must collapse %d concurrent misses to 1 RPC", n)
}

// A caller that missed the cache while a flight was running, and reaches the
// flight group only after that flight ended, must not fetch again: the value
// is in the cache by then. Before the flight filled the cache itself, this
// order produced a second fetch. It is the order behind the one failure of
// TestClient_SingleFlight_ConcurrentMiss in the merge queue, which that test
// reaches only when the scheduler parks a goroutine at exactly this point.
func TestClient_SingleFlight_LateCallerAfterTheFlightEnded(t *testing.T) {
	m := minimalManifest("cred:api_key")

	var callCount int64
	gate := make(chan struct{})
	fn := func(_ context.Context, _ string) ([]byte, error) {
		<-gate
		atomic.AddInt64(&callCount, 1)
		return []byte("value"), nil
	}
	cl := New(m, fn, CacheConfig{})

	// The first caller passes the hook. The second one is held in it.
	var hookCalls int64
	lateArrived := make(chan struct{})
	releaseLate := make(chan struct{})
	testHookAfterCacheMiss = func() {
		if atomic.AddInt64(&hookCalls, 1) == 2 {
			close(lateArrived)
			<-releaseLate
		}
	}
	t.Cleanup(func() { testHookAfterCacheMiss = nil })

	firstDone := make(chan struct{})
	go func() {
		defer close(firstDone)
		_, _ = cl.Resolve(context.Background(), "cred:api_key")
	}()
	// Wait until the first caller's flight is running, then start the late one.
	require.Eventually(t, func() bool { return atomic.LoadInt64(&hookCalls) == 1 }, time.Second, time.Millisecond)

	lateDone := make(chan []byte, 1)
	go func() {
		v, _ := cl.Resolve(context.Background(), "cred:api_key")
		lateDone <- v
	}()
	<-lateArrived // the late caller has missed the cache

	close(gate) // the first flight ends and its caller returns
	<-firstDone

	close(releaseLate) // only now does the late caller reach the flight group
	assert.Equal(t, []byte("value"), <-lateDone)
	assert.Equal(t, int64(1), atomic.LoadInt64(&callCount),
		"a caller that arrives after the flight ended must read the cache, not fetch again")
}

func TestClient_Invalidate_DropsCache(t *testing.T) {
	m := minimalManifest("cred:key")
	var calls int
	fn := func(_ context.Context, _ string) ([]byte, error) {
		calls++
		return []byte("v"), nil
	}
	cl := New(m, fn, CacheConfig{})

	_, err := cl.Resolve(context.Background(), "cred:key")
	require.NoError(t, err)
	assert.Equal(t, 1, calls)

	cl.Invalidate("cred:key")

	_, err = cl.Resolve(context.Background(), "cred:key")
	require.NoError(t, err)
	assert.Equal(t, 2, calls, "Invalidate must force RPC on next Resolve")
}

func TestClient_MarkRevoked_ReturnsPermissionDenied(t *testing.T) {
	m := minimalManifest("cred:key")
	calls := 0
	fn := func(_ context.Context, _ string) ([]byte, error) {
		calls++
		return []byte("v"), nil
	}
	cl := New(m, fn, CacheConfig{})

	// Prime the cache.
	_, err := cl.Resolve(context.Background(), "cred:key")
	require.NoError(t, err)

	cl.MarkRevoked("cred:key")

	_, err = cl.Resolve(context.Background(), "cred:key")
	require.Error(t, err)
	require.ErrorIs(t, err, ErrPermissionDenied)
	assert.Equal(t, 1, calls, "RPC must not be called after MarkRevoked")
}

func TestClient_MarkRevoked_PersistsAcrossInvalidate(t *testing.T) {
	m := minimalManifest("cred:key")
	fn := func(_ context.Context, _ string) ([]byte, error) {
		return []byte("v"), nil
	}
	cl := New(m, fn, CacheConfig{})

	cl.MarkRevoked("cred:key")
	cl.Invalidate("cred:key") // even after Invalidate, revoked flag must persist

	_, err := cl.Resolve(context.Background(), "cred:key")
	require.Error(t, err)
	assert.ErrorIs(t, err, ErrPermissionDenied,
		"revoked flag must persist after Invalidate")
}

func TestClient_DefensiveCopyOnReturn(t *testing.T) {
	m := minimalManifest("cred:key")
	fn := func(_ context.Context, _ string) ([]byte, error) {
		return []byte("secret"), nil
	}
	cl := New(m, fn, CacheConfig{})

	v1, err := cl.Resolve(context.Background(), "cred:key")
	require.NoError(t, err)

	// Corrupt the returned slice.
	v1[0] = 'X'

	// Second resolve should return the original unchanged value.
	v2, err := cl.Resolve(context.Background(), "cred:key")
	require.NoError(t, err)
	assert.Equal(t, []byte("secret"), v2, "caller mutation must not affect cache")
}
