// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Zero Root AI

// Package protoresolver provides caching mechanisms for parsed Protocol Buffer FileDescriptorSets.
//
// The cache implementation uses an LRU (Least Recently Used) eviction policy to maintain
// a bounded memory footprint while providing fast O(1) lookups. This is particularly useful
// for avoiding repeated parsing overhead (~1-5ms per parse) when working with the same
// FileDescriptorSets across multiple tool invocations.
//
// Example usage:
//
//	// Create a cache with max 100 entries and 1 hour TTL
//	cache := NewFileDescriptorCache(100, 1*time.Hour)
//
//	// Store parsed FileDescriptorSet
//	cache.Put("my-tool", files)
//
//	// Retrieve from cache
//	if files, ok := cache.Get("my-tool"); ok {
//	    // Use cached files
//	}
//
//	// Check cache performance
//	stats := cache.Stats()
//	hitRate := float64(stats.Hits) / float64(stats.Hits + stats.Misses)
package protoresolver

import (
	"container/list"
	"sync"
	"sync/atomic"
	"time"

	"google.golang.org/protobuf/reflect/protoregistry"
)

// FileDescriptorCache defines the interface for caching parsed FileDescriptorSets.
// Implementations must be thread-safe for concurrent access.
type FileDescriptorCache interface {
	// Get retrieves a cached FileDescriptorSet by tool name.
	// Returns the files and true if found and not expired, nil and false otherwise.
	Get(toolName string) (*protoregistry.Files, bool)

	// Put stores a FileDescriptorSet in the cache.
	// If the cache is at capacity, the least recently used entry is evicted.
	Put(toolName string, files *protoregistry.Files)

	// Invalidate removes a specific entry from the cache.
	Invalidate(toolName string)

	// Stats returns current cache statistics.
	Stats() CacheStats
}

// CacheStats provides metrics about cache performance.
type CacheStats struct {
	Hits      int64 // Number of successful cache retrievals
	Misses    int64 // Number of cache misses
	Entries   int   // Current number of entries in cache
	Evictions int64 // Number of entries evicted due to capacity limits
}

// cacheEntry represents a single entry in the LRU cache.
type cacheEntry struct {
	toolName  string
	files     *protoregistry.Files
	timestamp time.Time
}

// lruCache implements FileDescriptorCache with LRU eviction policy.
type lruCache struct {
	mu         sync.RWMutex
	maxEntries int
	ttl        time.Duration
	entries    map[string]*list.Element // Map for O(1) lookups
	lruList    *list.List               // Doubly-linked list for LRU ordering
	hits       atomic.Int64
	misses     atomic.Int64
	evictions  atomic.Int64
	onEviction func() // Optional callback for eviction events
}

// NewFileDescriptorCache creates a new LRU cache for FileDescriptorSets.
// maxEntries specifies the maximum number of entries before eviction occurs.
// ttl specifies how long entries remain valid before expiring.
func NewFileDescriptorCache(maxEntries int, ttl time.Duration) FileDescriptorCache {
	if maxEntries <= 0 {
		maxEntries = 100 // Default capacity
	}
	if ttl <= 0 {
		ttl = 1 * time.Hour // Default TTL
	}

	return &lruCache{
		maxEntries: maxEntries,
		ttl:        ttl,
		entries:    make(map[string]*list.Element),
		lruList:    list.New(),
	}
}

// Get retrieves a cached FileDescriptorSet by tool name.
// Returns the files and true if found and not expired, nil and false otherwise.
// On cache hit, the entry is moved to the front of the LRU list.
func (c *lruCache) Get(toolName string) (*protoregistry.Files, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()

	element, exists := c.entries[toolName]
	if !exists {
		c.misses.Add(1)
		return nil, false
	}

	entry := element.Value.(*cacheEntry)

	// Check if entry has expired
	if time.Since(entry.timestamp) > c.ttl {
		// Remove expired entry
		c.lruList.Remove(element)
		delete(c.entries, toolName)
		c.misses.Add(1)
		return nil, false
	}

	// Move to front (most recently used)
	c.lruList.MoveToFront(element)
	c.hits.Add(1)

	return entry.files, true
}

// Put stores a FileDescriptorSet in the cache.
// If the entry already exists, it updates the entry and moves it to the front.
// If the cache is at capacity, the least recently used entry is evicted.
func (c *lruCache) Put(toolName string, files *protoregistry.Files) {
	c.mu.Lock()
	defer c.mu.Unlock()

	// Check if entry already exists
	if element, exists := c.entries[toolName]; exists {
		// Update existing entry and move to front
		entry := element.Value.(*cacheEntry)
		entry.files = files
		entry.timestamp = time.Now()
		c.lruList.MoveToFront(element)
		return
	}

	// Create new entry
	entry := &cacheEntry{
		toolName:  toolName,
		files:     files,
		timestamp: time.Now(),
	}

	// Add to front of LRU list
	element := c.lruList.PushFront(entry)
	c.entries[toolName] = element

	// Evict oldest entry if over capacity
	if c.lruList.Len() > c.maxEntries {
		c.evictOldest()
	}
}

// Invalidate removes a specific entry from the cache.
func (c *lruCache) Invalidate(toolName string) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if element, exists := c.entries[toolName]; exists {
		c.lruList.Remove(element)
		delete(c.entries, toolName)
	}
}

// Stats returns current cache statistics.
func (c *lruCache) Stats() CacheStats {
	c.mu.RLock()
	defer c.mu.RUnlock()

	return CacheStats{
		Hits:      c.hits.Load(),
		Misses:    c.misses.Load(),
		Entries:   c.lruList.Len(),
		Evictions: c.evictions.Load(),
	}
}

// evictOldest removes the least recently used entry from the cache.
// Must be called with mutex held.
func (c *lruCache) evictOldest() {
	oldest := c.lruList.Back()
	if oldest != nil {
		entry := oldest.Value.(*cacheEntry)
		c.lruList.Remove(oldest)
		delete(c.entries, entry.toolName)
		c.evictions.Add(1)

		// Notify eviction callback if set
		if c.onEviction != nil {
			c.onEviction()
		}
	}
}

// SetEvictionCallback sets a callback to be invoked when entries are evicted.
// This is used for metrics recording.
func (c *lruCache) SetEvictionCallback(callback func()) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.onEviction = callback
}
