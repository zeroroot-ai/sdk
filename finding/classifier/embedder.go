// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Zero Root AI

package classifier

import "context"

// Embedder generates embedding vectors from text content.
//
// This interface mirrors the daemon's embedder interface (core/gibson/internal/memory/embedder)
// but is defined in the SDK to allow the daemon to inject its implementation without
// creating a dependency from SDK to daemon packages.
//
// Implementations must be thread-safe for concurrent access.
type Embedder interface {
	// Embed generates an embedding vector for a single text.
	//
	// The text is processed by the embedding model to produce a dense vector
	// representation. The dimensionality of the vector is model-specific.
	//
	// Parameters:
	//   - ctx: Context for cancellation and tracing
	//   - text: The text to embed
	//
	// Returns the embedding vector and an error if embedding fails.
	Embed(ctx context.Context, text string) ([]float64, error)

	// EmbedBatch generates embeddings for multiple texts efficiently.
	//
	// This method should be used when embedding multiple texts to take advantage
	// of batching optimizations in the underlying model. The order of embeddings
	// in the returned slice matches the order of texts in the input slice.
	//
	// Parameters:
	//   - ctx: Context for cancellation and tracing
	//   - texts: Slice of texts to embed
	//
	// Returns a slice of embedding vectors (one per input text) and an error if
	// embedding fails for any text.
	EmbedBatch(ctx context.Context, texts []string) ([][]float64, error)

	// Dimensions returns the dimensionality of embedding vectors.
	//
	// All embeddings produced by this embedder will have this dimensionality.
	// For example, all-MiniLM-L6-v2 produces 384-dimensional vectors.
	//
	// Returns the embedding vector dimensionality.
	Dimensions() int
}
