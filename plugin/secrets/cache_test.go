// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Zero Root AI

package secrets

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCache_GetMiss(t *testing.T) {
	c := newCache(10, DefaultCacheTTL)
	v, ok := c.get("missing")
	assert.False(t, ok)
	assert.Nil(t, v)
}

func TestCache_SetAndGet(t *testing.T) {
	c := newCache(10, DefaultCacheTTL)
	c.set("k", []byte("hello"))

	v, ok := c.get("k")
	require.True(t, ok)
	assert.Equal(t, []byte("hello"), v)
}

func TestCache_DefensiveCopy(t *testing.T) {
	c := newCache(10, DefaultCacheTTL)
	original := []byte("secret")
	c.set("k", original)

	v, ok := c.get("k")
	require.True(t, ok)

	// Mutate the returned slice; cache must remain unchanged.
	v[0] = 'X'
	v2, ok2 := c.get("k")
	require.True(t, ok2)
	assert.Equal(t, []byte("secret"), v2, "cache entry was corrupted by caller mutation")
}

func TestCache_TTLExpiry(t *testing.T) {
	now := time.Now()
	c := newCache(10, 2*time.Second)
	c.nowFn = func() time.Time { return now }
	c.set("k", []byte("val"))

	// Within TTL.
	_, ok := c.get("k")
	assert.True(t, ok)

	// Advance time past TTL.
	c.nowFn = func() time.Time { return now.Add(3 * time.Second) }
	_, ok = c.get("k")
	assert.False(t, ok, "expired entry should not be returned")
}

func TestCache_Delete(t *testing.T) {
	c := newCache(10, DefaultCacheTTL)
	c.set("k", []byte("val"))
	c.delete("k")

	_, ok := c.get("k")
	assert.False(t, ok)
}

func TestCache_DeleteMissing(t *testing.T) {
	c := newCache(10, DefaultCacheTTL)
	// Should not panic on a missing key.
	c.delete("nonexistent")
}

func TestCache_LRUEviction(t *testing.T) {
	c := newCache(3, DefaultCacheTTL)
	c.set("a", []byte("1"))
	c.set("b", []byte("2"))
	c.set("c", []byte("3"))

	// Access "a" to make it the most-recently-used.
	_, ok := c.get("a")
	require.True(t, ok)

	// Adding "d" should evict "b" (now the LRU).
	c.set("d", []byte("4"))

	_, okB := c.get("b")
	_, okA := c.get("a")
	_, okC := c.get("c")
	_, okD := c.get("d")

	assert.False(t, okB, "b should have been evicted (LRU)")
	assert.True(t, okA, "a should be present (accessed recently)")
	assert.True(t, okC, "c should be present")
	assert.True(t, okD, "d should be present (just added)")
}

func TestCache_Overwrite(t *testing.T) {
	c := newCache(10, DefaultCacheTTL)
	c.set("k", []byte("old"))
	c.set("k", []byte("new"))

	v, ok := c.get("k")
	require.True(t, ok)
	assert.Equal(t, []byte("new"), v)
}

func TestCache_Size(t *testing.T) {
	c := newCache(5, DefaultCacheTTL)
	for i := range 10 {
		c.set(string(rune('a'+i)), []byte{byte(i)})
	}
	c.mu.RLock()
	n := len(c.entries)
	c.mu.RUnlock()
	assert.Equal(t, 5, n, "cache must not exceed maxSize")
}
