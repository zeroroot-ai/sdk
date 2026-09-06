// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Zero Root AI

package store

import (
	"context"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/zeroroot-ai/sdk/finding/classifier"
)

func TestNewMemoryStore(t *testing.T) {
	store := NewMemoryStore()
	assert.NotNil(t, store)
	assert.Empty(t, store.entries)
}

func TestMemoryStore_Upsert(t *testing.T) {
	ctx := context.Background()
	store := NewMemoryStore()

	t.Run("adds new vector", func(t *testing.T) {
		embedding := []float64{1.0, 2.0, 3.0}
		metadata := map[string]any{
			"category":    "test",
			"description": "test description",
		}

		err := store.Upsert(ctx, "test-1", embedding, metadata)
		require.NoError(t, err)

		count, err := store.Count(ctx)
		require.NoError(t, err)
		assert.Equal(t, 1, count)
	})

	t.Run("updates existing vector", func(t *testing.T) {
		// Initial insert
		embedding1 := []float64{1.0, 2.0, 3.0}
		metadata1 := map[string]any{"version": 1}

		err := store.Upsert(ctx, "test-2", embedding1, metadata1)
		require.NoError(t, err)

		// Update with new embedding and metadata
		embedding2 := []float64{4.0, 5.0, 6.0}
		metadata2 := map[string]any{"version": 2}

		err = store.Upsert(ctx, "test-2", embedding2, metadata2)
		require.NoError(t, err)

		// Count should not increase
		count, err := store.Count(ctx)
		require.NoError(t, err)
		assert.Equal(t, 2, count) // test-1 and test-2

		// Search should return updated embedding
		results, err := store.Search(ctx, embedding2, 1)
		require.NoError(t, err)
		require.Len(t, results, 1)
		assert.Equal(t, "test-2", results[0].ID)
		assert.Equal(t, 2, results[0].Metadata["version"])
	})

	t.Run("does not modify original embedding", func(t *testing.T) {
		embedding := []float64{1.0, 2.0, 3.0}
		original := make([]float64, len(embedding))
		copy(original, embedding)

		err := store.Upsert(ctx, "test-3", embedding, map[string]any{})
		require.NoError(t, err)

		// Modify the original embedding
		embedding[0] = 999.0

		// Verify stored embedding is unchanged
		results, err := store.Search(ctx, original, 10)
		require.NoError(t, err)

		for _, result := range results {
			if result.ID == "test-3" {
				assert.Equal(t, 1.0, result.Score) // Perfect match with original
				return
			}
		}
		t.Fatal("test-3 not found in results")
	})

	t.Run("does not modify original metadata", func(t *testing.T) {
		metadata := map[string]any{"key": "value"}

		err := store.Upsert(ctx, "test-4", []float64{1.0, 2.0, 3.0}, metadata)
		require.NoError(t, err)

		// Modify original metadata
		metadata["key"] = "modified"

		// Verify stored metadata is unchanged
		results, err := store.Search(ctx, []float64{1.0, 2.0, 3.0}, 10)
		require.NoError(t, err)

		for _, result := range results {
			if result.ID == "test-4" {
				assert.Equal(t, "value", result.Metadata["key"])
				return
			}
		}
		t.Fatal("test-4 not found in results")
	})
}

func TestMemoryStore_Search(t *testing.T) {
	ctx := context.Background()

	t.Run("returns empty results for empty store", func(t *testing.T) {
		store := NewMemoryStore()
		results, err := store.Search(ctx, []float64{1.0, 2.0, 3.0}, 5)
		require.NoError(t, err)
		assert.Empty(t, results)
	})

	t.Run("returns results sorted by similarity", func(t *testing.T) {
		store := NewMemoryStore()

		// Add vectors with varying similarity to query [1, 0, 0]
		vectors := map[string][]float64{
			"exact":      {1.0, 0.0, 0.0}, // Perfect match
			"close":      {0.9, 0.1, 0.0}, // High similarity
			"moderate":   {0.5, 0.5, 0.0}, // Moderate similarity
			"orthogonal": {0.0, 1.0, 0.0}, // Orthogonal
		}

		for id, vec := range vectors {
			err := store.Upsert(ctx, id, vec, map[string]any{"id": id})
			require.NoError(t, err)
		}

		// Search with query vector
		query := []float64{1.0, 0.0, 0.0}
		results, err := store.Search(ctx, query, 10)
		require.NoError(t, err)
		require.Len(t, results, 4)

		// Verify ordering (descending similarity)
		assert.Equal(t, "exact", results[0].ID)
		assert.InDelta(t, 1.0, results[0].Score, 0.001)

		assert.Equal(t, "close", results[1].ID)
		assert.Greater(t, results[1].Score, results[2].Score)

		assert.Equal(t, "moderate", results[2].ID)
		assert.Greater(t, results[2].Score, results[3].Score)

		assert.Equal(t, "orthogonal", results[3].ID)
		assert.InDelta(t, 0.0, results[3].Score, 0.001)
	})

	t.Run("respects topK limit", func(t *testing.T) {
		store := NewMemoryStore()

		// Add 10 vectors
		for i := range 10 {
			vec := []float64{float64(i), 0.0, 0.0}
			err := store.Upsert(ctx, string(rune('a'+i)), vec, map[string]any{})
			require.NoError(t, err)
		}

		// Request top 3
		results, err := store.Search(ctx, []float64{1.0, 0.0, 0.0}, 3)
		require.NoError(t, err)
		assert.Len(t, results, 3)
	})

	t.Run("handles topK larger than store size", func(t *testing.T) {
		store := NewMemoryStore()

		// Add 3 vectors
		for i := range 3 {
			vec := []float64{float64(i), 0.0, 0.0}
			err := store.Upsert(ctx, string(rune('a'+i)), vec, map[string]any{})
			require.NoError(t, err)
		}

		// Request top 10 (more than available)
		results, err := store.Search(ctx, []float64{1.0, 0.0, 0.0}, 10)
		require.NoError(t, err)
		assert.Len(t, results, 3) // Returns all available
	})

	t.Run("does not modify returned metadata", func(t *testing.T) {
		store := NewMemoryStore()

		err := store.Upsert(ctx, "test", []float64{1.0, 2.0, 3.0}, map[string]any{"key": "value"})
		require.NoError(t, err)

		results, err := store.Search(ctx, []float64{1.0, 2.0, 3.0}, 1)
		require.NoError(t, err)
		require.Len(t, results, 1)

		// Modify returned metadata
		results[0].Metadata["key"] = "modified"

		// Search again and verify original metadata is unchanged
		results2, err := store.Search(ctx, []float64{1.0, 2.0, 3.0}, 1)
		require.NoError(t, err)
		require.Len(t, results2, 1)
		assert.Equal(t, "value", results2[0].Metadata["key"])
	})
}

func TestMemoryStore_Delete(t *testing.T) {
	ctx := context.Background()

	t.Run("removes existing vector", func(t *testing.T) {
		store := NewMemoryStore()

		// Add vectors
		err := store.Upsert(ctx, "test-1", []float64{1.0, 2.0, 3.0}, map[string]any{})
		require.NoError(t, err)
		err = store.Upsert(ctx, "test-2", []float64{4.0, 5.0, 6.0}, map[string]any{})
		require.NoError(t, err)

		// Delete one
		err = store.Delete(ctx, "test-1")
		require.NoError(t, err)

		// Verify count decreased
		count, err := store.Count(ctx)
		require.NoError(t, err)
		assert.Equal(t, 1, count)

		// Verify deleted vector is not in search results
		results, err := store.Search(ctx, []float64{1.0, 2.0, 3.0}, 10)
		require.NoError(t, err)
		for _, result := range results {
			assert.NotEqual(t, "test-1", result.ID)
		}
	})

	t.Run("is idempotent for non-existent ID", func(t *testing.T) {
		store := NewMemoryStore()

		err := store.Upsert(ctx, "test-1", []float64{1.0, 2.0, 3.0}, map[string]any{})
		require.NoError(t, err)

		// Delete non-existent ID
		err = store.Delete(ctx, "does-not-exist")
		require.NoError(t, err)

		// Count should be unchanged
		count, err := store.Count(ctx)
		require.NoError(t, err)
		assert.Equal(t, 1, count)
	})

	t.Run("handles deleting from empty store", func(t *testing.T) {
		store := NewMemoryStore()

		err := store.Delete(ctx, "does-not-exist")
		require.NoError(t, err)

		count, err := store.Count(ctx)
		require.NoError(t, err)
		assert.Equal(t, 0, count)
	})
}

func TestMemoryStore_Count(t *testing.T) {
	ctx := context.Background()

	t.Run("returns zero for empty store", func(t *testing.T) {
		store := NewMemoryStore()
		count, err := store.Count(ctx)
		require.NoError(t, err)
		assert.Equal(t, 0, count)
	})

	t.Run("returns correct count after upserts", func(t *testing.T) {
		store := NewMemoryStore()

		for i := range 5 {
			err := store.Upsert(ctx, string(rune('a'+i)), []float64{float64(i)}, map[string]any{})
			require.NoError(t, err)
		}

		count, err := store.Count(ctx)
		require.NoError(t, err)
		assert.Equal(t, 5, count)
	})

	t.Run("returns correct count after deletes", func(t *testing.T) {
		store := NewMemoryStore()

		// Add 3
		for i := range 3 {
			err := store.Upsert(ctx, string(rune('a'+i)), []float64{float64(i)}, map[string]any{})
			require.NoError(t, err)
		}

		// Delete 1
		err := store.Delete(ctx, "a")
		require.NoError(t, err)

		count, err := store.Count(ctx)
		require.NoError(t, err)
		assert.Equal(t, 2, count)
	})
}

func TestMemoryStore_ConcurrentAccess(t *testing.T) {
	ctx := context.Background()
	store := NewMemoryStore()

	// Run concurrent operations
	const numGoroutines = 10
	const opsPerGoroutine = 100

	var wg sync.WaitGroup
	wg.Add(numGoroutines)

	for g := range numGoroutines {
		go func(id int) {
			defer wg.Done()

			for i := range opsPerGoroutine {
				// Upsert
				vecID := string(rune('a' + (id*opsPerGoroutine+i)%26))
				vec := []float64{float64(id), float64(i)}
				err := store.Upsert(ctx, vecID, vec, map[string]any{"g": id, "i": i})
				assert.NoError(t, err)

				// Search
				_, err = store.Search(ctx, vec, 5)
				assert.NoError(t, err)

				// Count
				_, err = store.Count(ctx)
				assert.NoError(t, err)

				// Occasionally delete
				if i%10 == 0 {
					err = store.Delete(ctx, vecID)
					assert.NoError(t, err)
				}
			}
		}(g)
	}

	wg.Wait()

	// Verify store is still functional
	count, err := store.Count(ctx)
	require.NoError(t, err)
	assert.Positive(t, count)
}

func TestCosineSimilarity(t *testing.T) {
	tests := []struct {
		name     string
		a        []float64
		b        []float64
		expected float64
	}{
		{
			name:     "identical vectors",
			a:        []float64{1.0, 2.0, 3.0},
			b:        []float64{1.0, 2.0, 3.0},
			expected: 1.0,
		},
		{
			name:     "orthogonal vectors",
			a:        []float64{1.0, 0.0, 0.0},
			b:        []float64{0.0, 1.0, 0.0},
			expected: 0.0,
		},
		{
			name:     "opposite vectors",
			a:        []float64{1.0, 0.0, 0.0},
			b:        []float64{-1.0, 0.0, 0.0},
			expected: -1.0,
		},
		{
			name:     "similar vectors",
			a:        []float64{1.0, 0.0, 0.0},
			b:        []float64{0.9, 0.1, 0.0},
			expected: 0.994, // Approximately
		},
		{
			name:     "different dimensions",
			a:        []float64{1.0, 2.0, 3.0},
			b:        []float64{1.0, 2.0},
			expected: 0.0,
		},
		{
			name:     "empty vectors",
			a:        []float64{},
			b:        []float64{},
			expected: 0.0,
		},
		{
			name:     "zero vector",
			a:        []float64{0.0, 0.0, 0.0},
			b:        []float64{1.0, 2.0, 3.0},
			expected: 0.0,
		},
		{
			name:     "both zero vectors",
			a:        []float64{0.0, 0.0, 0.0},
			b:        []float64{0.0, 0.0, 0.0},
			expected: 0.0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := cosineSimilarity(tt.a, tt.b)
			assert.InDelta(t, tt.expected, result, 0.01, "cosine similarity mismatch")
		})
	}
}

func TestMemoryStore_EdgeCases(t *testing.T) {
	ctx := context.Background()

	t.Run("duplicate IDs are updated not duplicated", func(t *testing.T) {
		store := NewMemoryStore()

		// Insert same ID multiple times
		for i := range 5 {
			err := store.Upsert(ctx, "same-id", []float64{float64(i)}, map[string]any{"iteration": i})
			require.NoError(t, err)
		}

		// Should only have 1 entry
		count, err := store.Count(ctx)
		require.NoError(t, err)
		assert.Equal(t, 1, count)

		// Should have latest metadata
		results, err := store.Search(ctx, []float64{4.0}, 1)
		require.NoError(t, err)
		require.Len(t, results, 1)
		assert.Equal(t, "same-id", results[0].ID)
		assert.Equal(t, 4, results[0].Metadata["iteration"])
	})

	t.Run("handles zero-length embeddings", func(t *testing.T) {
		store := NewMemoryStore()

		err := store.Upsert(ctx, "zero-vec", []float64{0.0, 0.0, 0.0}, map[string]any{})
		require.NoError(t, err)

		// Search with zero vector
		results, err := store.Search(ctx, []float64{0.0, 0.0, 0.0}, 1)
		require.NoError(t, err)
		assert.Len(t, results, 1)
		assert.Equal(t, 0.0, results[0].Score) // Zero vectors have 0 similarity
	})

	t.Run("handles high-dimensional vectors", func(t *testing.T) {
		store := NewMemoryStore()

		// 384 dimensions (typical for all-MiniLM-L6-v2)
		dim := 384
		vec1 := make([]float64, dim)
		vec2 := make([]float64, dim)

		for i := range dim {
			vec1[i] = 1.0
			vec2[i] = 1.0
		}

		err := store.Upsert(ctx, "high-dim", vec1, map[string]any{})
		require.NoError(t, err)

		results, err := store.Search(ctx, vec2, 1)
		require.NoError(t, err)
		require.Len(t, results, 1)
		assert.InDelta(t, 1.0, results[0].Score, 0.001)
	})

	t.Run("handles negative values in embeddings", func(t *testing.T) {
		store := NewMemoryStore()

		err := store.Upsert(ctx, "negative", []float64{-1.0, -2.0, -3.0}, map[string]any{})
		require.NoError(t, err)

		// Same vector should have perfect similarity
		results, err := store.Search(ctx, []float64{-1.0, -2.0, -3.0}, 1)
		require.NoError(t, err)
		require.Len(t, results, 1)
		assert.InDelta(t, 1.0, results[0].Score, 0.001)
	})

	t.Run("handles empty metadata", func(t *testing.T) {
		store := NewMemoryStore()

		err := store.Upsert(ctx, "no-metadata", []float64{1.0, 2.0, 3.0}, nil)
		require.NoError(t, err)

		results, err := store.Search(ctx, []float64{1.0, 2.0, 3.0}, 1)
		require.NoError(t, err)
		require.Len(t, results, 1)
		assert.NotNil(t, results[0].Metadata)
		assert.Empty(t, results[0].Metadata)
	})
}

// Benchmark tests
func BenchmarkMemoryStore_Upsert(b *testing.B) {
	ctx := context.Background()
	store := NewMemoryStore()
	vec := make([]float64, 384)
	for i := range vec {
		vec[i] = float64(i) * 0.1
	}

	b.ResetTimer()
	for i := range b.N {
		_ = store.Upsert(ctx, string(rune(i%1000)), vec, map[string]any{"index": i})
	}
}

func BenchmarkMemoryStore_Search(b *testing.B) {
	ctx := context.Background()
	store := NewMemoryStore()

	// Populate with 1000 vectors
	vec := make([]float64, 384)
	for i := range 1000 {
		for j := range vec {
			vec[j] = float64(i*j) * 0.001
		}
		_ = store.Upsert(ctx, string(rune(i)), vec, map[string]any{})
	}

	// Benchmark search
	query := make([]float64, 384)
	for i := range query {
		query[i] = float64(i) * 0.1
	}

	b.ResetTimer()
	for range b.N {
		_, _ = store.Search(ctx, query, 10)
	}
}

func BenchmarkCosineSimilarity(b *testing.B) {
	a := make([]float64, 384)
	b_vec := make([]float64, 384)
	for i := range 384 {
		a[i] = float64(i) * 0.1
		b_vec[i] = float64(i) * 0.1
	}

	b.ResetTimer()
	for range b.N {
		_ = cosineSimilarity(a, b_vec)
	}
}

// Test that MemoryStore implements classifier.VectorStore interface
var _ classifier.VectorStore = (*MemoryStore)(nil)
