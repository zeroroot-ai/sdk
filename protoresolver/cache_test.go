// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Zero Root AI

package protoresolver

import (
	"sync"
	"testing"
	"time"

	"google.golang.org/protobuf/reflect/protoregistry"
)

func TestNewFileDescriptorCache(t *testing.T) {
	tests := []struct {
		name        string
		maxEntries  int
		ttl         time.Duration
		expectedMax int
		expectedTTL time.Duration
	}{
		{
			name:        "valid parameters",
			maxEntries:  50,
			ttl:         30 * time.Minute,
			expectedMax: 50,
			expectedTTL: 30 * time.Minute,
		},
		{
			name:        "zero maxEntries uses default",
			maxEntries:  0,
			ttl:         30 * time.Minute,
			expectedMax: 100,
			expectedTTL: 30 * time.Minute,
		},
		{
			name:        "negative maxEntries uses default",
			maxEntries:  -1,
			ttl:         30 * time.Minute,
			expectedMax: 100,
			expectedTTL: 30 * time.Minute,
		},
		{
			name:        "zero ttl uses default",
			maxEntries:  50,
			ttl:         0,
			expectedMax: 50,
			expectedTTL: 1 * time.Hour,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cache := NewFileDescriptorCache(tt.maxEntries, tt.ttl)
			lru := cache.(*lruCache)

			if lru.maxEntries != tt.expectedMax {
				t.Errorf("maxEntries = %d, want %d", lru.maxEntries, tt.expectedMax)
			}
			if lru.ttl != tt.expectedTTL {
				t.Errorf("ttl = %v, want %v", lru.ttl, tt.expectedTTL)
			}
		})
	}
}

func TestCacheGetPut(t *testing.T) {
	cache := NewFileDescriptorCache(10, 1*time.Hour)
	files := &protoregistry.Files{}

	// Test miss on empty cache
	result, ok := cache.Get("tool1")
	if ok {
		t.Error("Expected cache miss on empty cache")
	}
	if result != nil {
		t.Error("Expected nil result on cache miss")
	}

	// Test put and get
	cache.Put("tool1", files)
	result, ok = cache.Get("tool1")
	if !ok {
		t.Error("Expected cache hit after put")
	}
	if result != files {
		t.Error("Expected same files instance")
	}

	// Verify stats
	stats := cache.Stats()
	if stats.Hits != 1 {
		t.Errorf("Hits = %d, want 1", stats.Hits)
	}
	if stats.Misses != 1 {
		t.Errorf("Misses = %d, want 1", stats.Misses)
	}
	if stats.Entries != 1 {
		t.Errorf("Entries = %d, want 1", stats.Entries)
	}
}

func TestCacheUpdate(t *testing.T) {
	cache := NewFileDescriptorCache(10, 1*time.Hour)
	files1 := &protoregistry.Files{}
	files2 := &protoregistry.Files{}

	// Put initial value
	cache.Put("tool1", files1)

	// Update with new value
	cache.Put("tool1", files2)

	// Verify updated value
	result, ok := cache.Get("tool1")
	if !ok {
		t.Error("Expected cache hit")
	}
	if result != files2 {
		t.Error("Expected updated files instance")
	}

	// Verify no eviction occurred (should still be 1 entry)
	stats := cache.Stats()
	if stats.Entries != 1 {
		t.Errorf("Entries = %d, want 1", stats.Entries)
	}
	if stats.Evictions != 0 {
		t.Errorf("Evictions = %d, want 0", stats.Evictions)
	}
}

func TestCacheLRUEviction(t *testing.T) {
	cache := NewFileDescriptorCache(3, 1*time.Hour)

	// Fill cache to capacity
	cache.Put("tool1", &protoregistry.Files{})
	cache.Put("tool2", &protoregistry.Files{})
	cache.Put("tool3", &protoregistry.Files{})

	stats := cache.Stats()
	if stats.Entries != 3 {
		t.Errorf("Entries = %d, want 3", stats.Entries)
	}

	// Add one more to trigger eviction of tool1 (oldest)
	cache.Put("tool4", &protoregistry.Files{})

	stats = cache.Stats()
	if stats.Entries != 3 {
		t.Errorf("Entries = %d, want 3", stats.Entries)
	}
	if stats.Evictions != 1 {
		t.Errorf("Evictions = %d, want 1", stats.Evictions)
	}

	// Verify tool1 was evicted
	if _, ok := cache.Get("tool1"); ok {
		t.Error("tool1 should have been evicted")
	}

	// Verify others still exist
	if _, ok := cache.Get("tool2"); !ok {
		t.Error("tool2 should still be in cache")
	}
	if _, ok := cache.Get("tool3"); !ok {
		t.Error("tool3 should still be in cache")
	}
	if _, ok := cache.Get("tool4"); !ok {
		t.Error("tool4 should be in cache")
	}
}

func TestCacheLRUOrdering(t *testing.T) {
	cache := NewFileDescriptorCache(3, 1*time.Hour)

	// Add three entries
	cache.Put("tool1", &protoregistry.Files{})
	cache.Put("tool2", &protoregistry.Files{})
	cache.Put("tool3", &protoregistry.Files{})

	// Access tool1 to make it most recently used
	cache.Get("tool1")

	// Add tool4, should evict tool2 (now oldest)
	cache.Put("tool4", &protoregistry.Files{})

	// Verify tool2 was evicted, not tool1
	if _, ok := cache.Get("tool2"); ok {
		t.Error("tool2 should have been evicted")
	}
	if _, ok := cache.Get("tool1"); !ok {
		t.Error("tool1 should still be in cache (was accessed)")
	}
}

func TestCacheInvalidate(t *testing.T) {
	cache := NewFileDescriptorCache(10, 1*time.Hour)

	cache.Put("tool1", &protoregistry.Files{})
	cache.Put("tool2", &protoregistry.Files{})

	// Verify both entries exist
	if _, ok := cache.Get("tool1"); !ok {
		t.Error("tool1 should be in cache")
	}

	// Invalidate tool1
	cache.Invalidate("tool1")

	// Verify tool1 removed, tool2 still exists
	if _, ok := cache.Get("tool1"); ok {
		t.Error("tool1 should have been invalidated")
	}
	if _, ok := cache.Get("tool2"); !ok {
		t.Error("tool2 should still be in cache")
	}

	stats := cache.Stats()
	if stats.Entries != 1 {
		t.Errorf("Entries = %d, want 1", stats.Entries)
	}

	// Invalidate non-existent entry (should not panic)
	cache.Invalidate("nonexistent")
}

func TestCacheTTLExpiration(t *testing.T) {
	cache := NewFileDescriptorCache(10, 50*time.Millisecond)

	cache.Put("tool1", &protoregistry.Files{})

	// Immediate get should succeed
	if _, ok := cache.Get("tool1"); !ok {
		t.Error("Expected cache hit immediately after put")
	}

	// Wait for TTL to expire
	time.Sleep(100 * time.Millisecond)

	// Get should fail due to expiration
	result, ok := cache.Get("tool1")
	if ok {
		t.Error("Expected cache miss after TTL expiration")
	}
	if result != nil {
		t.Error("Expected nil result after expiration")
	}

	// Verify entry was removed from cache
	stats := cache.Stats()
	if stats.Entries != 0 {
		t.Errorf("Entries = %d, want 0 (expired entry should be removed)", stats.Entries)
	}
}

func TestCacheStats(t *testing.T) {
	cache := NewFileDescriptorCache(5, 1*time.Hour)

	// Initial stats
	stats := cache.Stats()
	if stats.Hits != 0 || stats.Misses != 0 || stats.Entries != 0 || stats.Evictions != 0 {
		t.Error("Expected all stats to be zero initially")
	}

	// Add entries and track stats
	cache.Put("tool1", &protoregistry.Files{})
	cache.Put("tool2", &protoregistry.Files{})
	cache.Get("tool1")       // hit
	cache.Get("nonexistent") // miss

	stats = cache.Stats()
	if stats.Hits != 1 {
		t.Errorf("Hits = %d, want 1", stats.Hits)
	}
	if stats.Misses != 1 {
		t.Errorf("Misses = %d, want 1", stats.Misses)
	}
	if stats.Entries != 2 {
		t.Errorf("Entries = %d, want 2", stats.Entries)
	}
	if stats.Evictions != 0 {
		t.Errorf("Evictions = %d, want 0", stats.Evictions)
	}
}

func TestCacheConcurrentAccess(t *testing.T) {
	cache := NewFileDescriptorCache(100, 1*time.Hour)
	var wg sync.WaitGroup

	// Concurrent writes
	for i := range 50 {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			toolName := "tool" + string(rune('A'+id))
			cache.Put(toolName, &protoregistry.Files{})
		}(i)
	}

	// Concurrent reads
	for i := range 50 {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			toolName := "tool" + string(rune('A'+id))
			cache.Get(toolName)
		}(i)
	}

	// Concurrent invalidations
	for i := range 25 {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			toolName := "tool" + string(rune('A'+id))
			cache.Invalidate(toolName)
		}(i)
	}

	wg.Wait()

	// Just verify no panics occurred and stats are consistent
	stats := cache.Stats()
	if stats.Entries < 0 || stats.Entries > 100 {
		t.Errorf("Invalid entry count: %d", stats.Entries)
	}
}

func TestCacheConcurrentEviction(t *testing.T) {
	cache := NewFileDescriptorCache(10, 1*time.Hour)
	var wg sync.WaitGroup

	// Rapidly add entries to trigger evictions
	for i := range 100 {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			for j := range 10 {
				toolName := "tool" + string(rune('A'+id)) + string(rune('0'+j))
				cache.Put(toolName, &protoregistry.Files{})
			}
		}(i)
	}

	wg.Wait()

	stats := cache.Stats()
	if stats.Entries > 10 {
		t.Errorf("Entries = %d, should not exceed maxEntries (10)", stats.Entries)
	}
	if stats.Evictions == 0 {
		t.Error("Expected evictions to occur")
	}
}

func BenchmarkCacheGet(b *testing.B) {
	cache := NewFileDescriptorCache(1000, 1*time.Hour)
	files := &protoregistry.Files{}

	// Populate cache
	for i := range 100 {
		toolName := "tool" + string(rune('A'+i))
		cache.Put(toolName, files)
	}

	b.ResetTimer()
	for range b.N {
		cache.Get("toolA")
	}
}

func BenchmarkCachePut(b *testing.B) {
	cache := NewFileDescriptorCache(1000, 1*time.Hour)
	files := &protoregistry.Files{}

	b.ResetTimer()
	for i := range b.N {
		toolName := "tool" + string(rune('A'+(i%100)))
		cache.Put(toolName, files)
	}
}

func BenchmarkCacheConcurrent(b *testing.B) {
	cache := NewFileDescriptorCache(1000, 1*time.Hour)
	files := &protoregistry.Files{}

	// Populate cache
	for i := range 100 {
		toolName := "tool" + string(rune('A'+i))
		cache.Put(toolName, files)
	}

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		i := 0
		for pb.Next() {
			toolName := "tool" + string(rune('A'+(i%100)))
			if i%2 == 0 {
				cache.Get(toolName)
			} else {
				cache.Put(toolName, files)
			}
			i++
		}
	})
}
