// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Zero Root AI

package classifier

import "context"

// VectorStore provides an abstraction for storing and searching embedding vectors.
// Implementations can use in-memory storage, Qdrant, pgvector, or other vector
// databases. All methods must be thread-safe for concurrent access.
type VectorStore interface {
	// Upsert adds or updates a category embedding in the store.
	//
	// If an entry with the given ID already exists, it is updated with the new
	// embedding and metadata. Otherwise, a new entry is created.
	//
	// Parameters:
	//   - ctx: Context for cancellation and tracing
	//   - id: Unique identifier for the category (typically the category name)
	//   - embedding: The embedding vector (dimensionality must match store config)
	//   - metadata: Additional metadata to store (e.g., domain, description, timestamps)
	//
	// Returns an error if the upsert operation fails.
	Upsert(ctx context.Context, id string, embedding []float64, metadata map[string]any) error

	// Search finds the nearest neighbors to the given embedding vector.
	//
	// Results are returned sorted by descending similarity score (highest first).
	// The similarity metric depends on the implementation (typically cosine similarity).
	//
	// Parameters:
	//   - ctx: Context for cancellation and tracing
	//   - embedding: The query embedding vector
	//   - topK: Maximum number of results to return
	//
	// Returns a slice of SearchResult sorted by score (highest first) and an error
	// if the search fails. Returns an empty slice if the store is empty or no
	// matches are found.
	Search(ctx context.Context, embedding []float64, topK int) ([]SearchResult, error)

	// Delete removes a category from the store.
	//
	// If the specified ID does not exist, this is a no-op (idempotent).
	//
	// Parameters:
	//   - ctx: Context for cancellation and tracing
	//   - id: Unique identifier of the category to remove
	//
	// Returns an error if the delete operation fails.
	Delete(ctx context.Context, id string) error

	// Count returns the total number of categories stored.
	//
	// Parameters:
	//   - ctx: Context for cancellation and tracing
	//
	// Returns the count and an error if the operation fails.
	Count(ctx context.Context) (int, error)
}

// SearchResult represents a single result from a vector search operation.
type SearchResult struct {
	// ID is the unique identifier of the matched category
	ID string `json:"id"`

	// Score is the similarity score (0.0 to 1.0, higher is more similar)
	// The exact metric depends on the VectorStore implementation (typically cosine similarity)
	Score float64 `json:"score"`

	// Metadata contains additional information stored with the category
	// Common fields: "domain" (string), "description" (string), "created_at" (time)
	Metadata map[string]any `json:"metadata"`
}
