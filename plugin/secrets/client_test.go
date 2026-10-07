// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Zero Root AI

package secrets

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestClient_EmptyName_IsInvalid(t *testing.T) {
	calls := 0
	fn := func(_ context.Context, _ string) ([]byte, error) {
		calls++
		return []byte("val"), nil
	}
	cl := New(fn, CacheConfig{})

	_, err := cl.Resolve(context.Background(), "")
	require.ErrorIs(t, err, ErrInvalidArgument)
	assert.Equal(t, 0, calls, "RPC must not be called for an empty name")
}

// TestClient_AnyName_ReachesTheDaemon proves the SDK keeps no allow-list: a
// name reaches the daemon, which decides with the FGA relation can_resolve.
func TestClient_AnyName_ReachesTheDaemon(t *testing.T) {
	var asked []string
	fn := func(_ context.Context, name string) ([]byte, error) {
		asked = append(asked, name)
		return []byte("val"), nil
	}
	cl := New(fn, CacheConfig{})

	_, err := cl.Resolve(context.Background(), "cred:never_declared")
	require.NoError(t, err)
	assert.Equal(t, []string{"cred:never_declared"}, asked)
}

func TestClient_CacheMiss_CallsRPC(t *testing.T) {
	calls := 0
	fn := func(_ context.Context, _ string) ([]byte, error) {
		calls++
		return []byte("secret-value"), nil
	}
	cl := New(fn, CacheConfig{})

	v, err := cl.Resolve(context.Background(), "cred:api_key")
	require.NoError(t, err)
	assert.Equal(t, []byte("secret-value"), v)
	assert.Equal(t, 1, calls)
}

func TestClient_CacheHit_NoRPC(t *testing.T) {
	calls := 0
	fn := func(_ context.Context, _ string) ([]byte, error) {
		calls++
		return []byte("secret-value"), nil
	}
	cl := New(fn, CacheConfig{})

	_, err := cl.Resolve(context.Background(), "cred:api_key")
	require.NoError(t, err)
	_, err = cl.Resolve(context.Background(), "cred:api_key")
	require.NoError(t, err)

	assert.Equal(t, 1, calls, "second Resolve must use cached value")
}

func TestClient_TTLExpiry_RefetchesAfterExpiry(t *testing.T) {
	calls := 0
	fn := func(_ context.Context, _ string) ([]byte, error) {
		calls++
		return []byte("token"), nil
	}

	ttl := 50 * time.Millisecond
	cl := New(fn, CacheConfig{TTL: ttl})

	_, err := cl.Resolve(context.Background(), "cred:token")
	require.NoError(t, err)
	assert.Equal(t, 1, calls)

	time.Sleep(2 * ttl)

	_, err = cl.Resolve(context.Background(), "cred:token")
	require.NoError(t, err)
	assert.Equal(t, 2, calls, "RPC should be called again after TTL expiry")
}

func TestClient_SingleFlight_ConcurrentMiss(t *testing.T) {

	var callCount int64
	gate := make(chan struct{})
	fn := func(_ context.Context, _ string) ([]byte, error) {
		<-gate // block until all goroutines are in-flight
		atomic.AddInt64(&callCount, 1)
		return []byte("value"), nil
	}

	cl := New(fn, CacheConfig{})

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

	var callCount int64
	gate := make(chan struct{})
	fn := func(_ context.Context, _ string) ([]byte, error) {
		<-gate
		atomic.AddInt64(&callCount, 1)
		return []byte("value"), nil
	}
	cl := New(fn, CacheConfig{})

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
	var calls int
	fn := func(_ context.Context, _ string) ([]byte, error) {
		calls++
		return []byte("v"), nil
	}
	cl := New(fn, CacheConfig{})

	_, err := cl.Resolve(context.Background(), "cred:key")
	require.NoError(t, err)
	assert.Equal(t, 1, calls)

	cl.Invalidate("cred:key")

	_, err = cl.Resolve(context.Background(), "cred:key")
	require.NoError(t, err)
	assert.Equal(t, 2, calls, "Invalidate must force RPC on next Resolve")
}

func TestClient_MarkRevoked_ReturnsPermissionDenied(t *testing.T) {
	calls := 0
	fn := func(_ context.Context, _ string) ([]byte, error) {
		calls++
		return []byte("v"), nil
	}
	cl := New(fn, CacheConfig{})

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
	fn := func(_ context.Context, _ string) ([]byte, error) {
		return []byte("v"), nil
	}
	cl := New(fn, CacheConfig{})

	cl.MarkRevoked("cred:key")
	cl.Invalidate("cred:key") // even after Invalidate, revoked flag must persist

	_, err := cl.Resolve(context.Background(), "cred:key")
	require.Error(t, err)
	assert.ErrorIs(t, err, ErrPermissionDenied,
		"revoked flag must persist after Invalidate")
}

// errUnavailable stands in for any error the GetCredential RPC returns.
var errUnavailable = errors.New("credential unavailable")

func TestClient_RPCError_NotCached(t *testing.T) {
	callCount := 0
	fn := func(_ context.Context, _ string) ([]byte, error) {
		callCount++
		if callCount == 1 {
			return nil, errUnavailable
		}
		return []byte("found"), nil
	}
	cl := New(fn, CacheConfig{})

	_, err := cl.Resolve(context.Background(), "cred:key")
	require.ErrorIs(t, err, errUnavailable)

	// Second call must not get a cached negative; should call RPC again.
	v, err := cl.Resolve(context.Background(), "cred:key")
	require.NoError(t, err)
	assert.Equal(t, []byte("found"), v)
	assert.Equal(t, 2, callCount, "negative result must not be cached")
}

func TestClient_DefensiveCopyOnReturn(t *testing.T) {
	fn := func(_ context.Context, _ string) ([]byte, error) {
		return []byte("secret"), nil
	}
	cl := New(fn, CacheConfig{})

	v1, err := cl.Resolve(context.Background(), "cred:key")
	require.NoError(t, err)

	// Corrupt the returned slice.
	v1[0] = 'X'

	// Second resolve should return the original unchanged value.
	v2, err := cl.Resolve(context.Background(), "cred:key")
	require.NoError(t, err)
	assert.Equal(t, []byte("secret"), v2, "caller mutation must not affect cache")
}
