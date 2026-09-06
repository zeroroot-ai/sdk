// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Zero Root AI

// Package secrets provides the plugin-side secrets client for the Gibson plugin
// SDK. It wraps the daemon's HarnessCallbackService.GetCredential RPC with an
// in-process LRU cache, single-flight deduplication, and revocation tracking.
//
// This package (github.com/zeroroot-ai/sdk/plugin/secrets) is the plugin
// author's only path to credentials. The daemon-side broker interface
// (previously at github.com/zeroroot-ai/sdk/secrets) has moved to the
// private platform-clients module per ADR-0025 / ADR-0030; this package
// remains in the OSS SDK because customer plugin authors import it.
//
// Spec: plugin-runtime Requirement 3.
package secrets

import (
	"sync"
	"time"
)

// DefaultCacheTTL is the TTL applied to cached secret values when no TTL
// option is supplied. Matches Requirement 3.3.
const DefaultCacheTTL = 60 * time.Second

// MaxCacheTTL is the maximum TTL callers may configure. Requirement 3.3.
const MaxCacheTTL = 300 * time.Second

// DefaultCacheSize is the maximum number of entries the LRU cache holds
// before evicting the least-recently-used entry. Requirement 9.5.
const DefaultCacheSize = 1000

// cacheEntry is one slot in the LRU cache.
type cacheEntry struct {
	value     []byte
	expiresAt time.Time

	// LRU doubly-linked list pointers. next is newer, prev is older.
	next *cacheEntry
	prev *cacheEntry
	key  string
}

// cache is a concurrent-safe, bounded LRU secret cache.
// It is an internal type; the Client is the public surface.
type cache struct {
	mu      sync.RWMutex
	entries map[string]*cacheEntry
	maxSize int
	ttl     time.Duration

	// LRU eviction sentinels: head.next is the oldest entry,
	// tail.prev is the newest. Access always moves an entry to tail.prev.
	head *cacheEntry // oldest sentinel (dummy)
	tail *cacheEntry // newest sentinel (dummy)

	// nowFn is replaceable in tests to control time.
	nowFn func() time.Time
}

// newCache constructs a cache with the given size limit and TTL.
// size must be >= 1; ttl must be in (0, MaxCacheTTL].
func newCache(size int, ttl time.Duration) *cache {
	head := &cacheEntry{}
	tail := &cacheEntry{}
	head.next = tail
	tail.prev = head

	return &cache{
		entries: make(map[string]*cacheEntry, size),
		maxSize: size,
		ttl:     ttl,
		head:    head,
		tail:    tail,
		nowFn:   time.Now,
	}
}

// get returns a defensive copy of the cached value for key.
// It returns (nil, false) on cache miss or expiry.
func (c *cache) get(key string) ([]byte, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()

	e, ok := c.entries[key]
	if !ok {
		return nil, false
	}
	if c.nowFn().After(e.expiresAt) {
		c.removeEntry(e)
		return nil, false
	}
	// Move to newest position (LRU).
	c.detach(e)
	c.insertBefore(e, c.tail)

	// Defensive copy so caller mutations cannot corrupt cache state.
	out := make([]byte, len(e.value))
	copy(out, e.value)
	return out, true
}

// set stores a defensive copy of value under key, evicting the LRU entry when
// the cache is at capacity.
func (c *cache) set(key string, value []byte) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if e, ok := c.entries[key]; ok {
		// Overwrite in place and promote to newest.
		v := make([]byte, len(value))
		copy(v, value)
		e.value = v
		e.expiresAt = c.nowFn().Add(c.ttl)
		c.detach(e)
		c.insertBefore(e, c.tail)
		return
	}

	// Evict LRU if at capacity.
	if len(c.entries) >= c.maxSize {
		oldest := c.head.next
		if oldest != c.tail { // should always be true when maxSize >= 1
			c.removeEntry(oldest)
		}
	}

	v := make([]byte, len(value))
	copy(v, value)
	e := &cacheEntry{
		value:     v,
		expiresAt: c.nowFn().Add(c.ttl),
		key:       key,
	}
	c.entries[key] = e
	c.insertBefore(e, c.tail)
}

// delete removes key from the cache, if present. This is the path called by
// Client.Invalidate on a rotation event.
func (c *cache) delete(key string) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if e, ok := c.entries[key]; ok {
		c.removeEntry(e)
	}
}

// removeEntry unlinks e from the LRU list and deletes it from the map.
// Caller must hold c.mu.
func (c *cache) removeEntry(e *cacheEntry) {
	c.detach(e)
	delete(c.entries, e.key)
}

// detach removes e from the doubly-linked list without touching the map.
// Caller must hold c.mu.
func (c *cache) detach(e *cacheEntry) {
	e.prev.next = e.next
	e.next.prev = e.prev
	e.next = nil
	e.prev = nil
}

// insertBefore places e immediately before sentinel in the list.
// Caller must hold c.mu.
func (c *cache) insertBefore(e, sentinel *cacheEntry) {
	e.next = sentinel
	e.prev = sentinel.prev
	sentinel.prev.next = e
	sentinel.prev = e
}
