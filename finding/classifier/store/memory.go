// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Zero Root AI

package store

import (
	"context"
	"math"
	"sort"
	"sync"

	"github.com/zeroroot-ai/sdk/finding/classifier"
)

// vectorEntry represents a single stored embedding with its metadata.
type vectorEntry struct {
	id        string
	embedding []float64
	metadata  map[string]any
}

// MemoryStore is an in-memory implementation of VectorStore using slice-based
// storage with linear search. It uses cosine similarity for vector matching.
//
// This implementation is suitable for testing and simple deployments with
// fewer than 10,000 categories. For larger deployments, use a dedicated
// vector database like Qdrant.
//
// MemoryStore is thread-safe via sync.RWMutex.
type MemoryStore struct {
	mu      sync.RWMutex
	entries []vectorEntry
}

// NewMemoryStore creates a new in-memory vector store.
func NewMemoryStore() *MemoryStore {
	return &MemoryStore{
		entries: make([]vectorEntry, 0),
	}
}

// Upsert adds or updates a category embedding in the store.
//
// If an entry with the given ID already exists, it is updated with the new
// embedding and metadata. Otherwise, a new entry is created.
//
// The metadata map is shallow-copied to prevent external modifications.
func (m *MemoryStore) Upsert(ctx context.Context, id string, embedding []float64, metadata map[string]any) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Copy embedding to prevent external modifications
	embeddingCopy := make([]float64, len(embedding))
	copy(embeddingCopy, embedding)

	// Copy metadata to prevent external modifications
	metadataCopy := make(map[string]any, len(metadata))
	for k, v := range metadata {
		metadataCopy[k] = v
	}

	// Check if entry already exists
	for i := range m.entries {
		if m.entries[i].id == id {
			// Update existing entry
			m.entries[i].embedding = embeddingCopy
			m.entries[i].metadata = metadataCopy
			return nil
		}
	}

	// Add new entry
	m.entries = append(m.entries, vectorEntry{
		id:        id,
		embedding: embeddingCopy,
		metadata:  metadataCopy,
	})

	return nil
}

// Search finds the nearest neighbors to the given embedding vector using
// cosine similarity.
//
// Results are returned sorted by descending similarity score (highest first).
// Returns an empty slice if the store is empty.
func (m *MemoryStore) Search(ctx context.Context, embedding []float64, topK int) ([]classifier.SearchResult, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	// Return empty results if store is empty
	if len(m.entries) == 0 {
		return []classifier.SearchResult{}, nil
	}

	// Calculate similarity scores for all entries
	type scoredEntry struct {
		entry vectorEntry
		score float64
	}

	scored := make([]scoredEntry, 0, len(m.entries))
	for _, entry := range m.entries {
		score := cosineSimilarity(embedding, entry.embedding)
		scored = append(scored, scoredEntry{
			entry: entry,
			score: score,
		})
	}

	// Sort by descending score
	sort.Slice(scored, func(i, j int) bool {
		return scored[i].score > scored[j].score
	})

	// Limit to topK results
	if topK > len(scored) {
		topK = len(scored)
	}

	// Convert to SearchResult
	results := make([]classifier.SearchResult, topK)
	for i := range topK {
		// Copy metadata to prevent external modifications
		metadataCopy := make(map[string]any, len(scored[i].entry.metadata))
		for k, v := range scored[i].entry.metadata {
			metadataCopy[k] = v
		}

		results[i] = classifier.SearchResult{
			ID:       scored[i].entry.id,
			Score:    scored[i].score,
			Metadata: metadataCopy,
		}
	}

	return results, nil
}

// Delete removes a category from the store.
//
// If the specified ID does not exist, this is a no-op (idempotent).
func (m *MemoryStore) Delete(ctx context.Context, id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Find and remove the entry
	for i := range m.entries {
		if m.entries[i].id == id {
			// Remove by swapping with last element and truncating
			m.entries[i] = m.entries[len(m.entries)-1]
			m.entries = m.entries[:len(m.entries)-1]
			return nil
		}
	}

	// ID not found - no-op (idempotent)
	return nil
}

// Count returns the total number of categories stored.
func (m *MemoryStore) Count(ctx context.Context) (int, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return len(m.entries), nil
}

// cosineSimilarity calculates the cosine similarity between two vectors.
//
// Returns a value between -1 and 1, where:
//   - 1 indicates identical vectors
//   - 0 indicates orthogonal vectors
//   - -1 indicates opposite vectors
//
// Returns 0 if either vector is zero-length or if dimensions don't match.
func cosineSimilarity(a, b []float64) float64 {
	if len(a) != len(b) || len(a) == 0 {
		return 0.0
	}

	var dotProduct, magnitudeA, magnitudeB float64

	for i := range a {
		dotProduct += a[i] * b[i]
		magnitudeA += a[i] * a[i]
		magnitudeB += b[i] * b[i]
	}

	// Avoid division by zero
	if magnitudeA == 0 || magnitudeB == 0 {
		return 0.0
	}

	return dotProduct / (math.Sqrt(magnitudeA) * math.Sqrt(magnitudeB))
}
