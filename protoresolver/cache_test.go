// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Zero Root AI

package protoresolver

import (
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
