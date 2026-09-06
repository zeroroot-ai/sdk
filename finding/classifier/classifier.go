// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Zero Root AI

package classifier

import (
	"context"

	"github.com/zeroroot-ai/sdk/finding/registry"
)

// CategoryClassifier provides semantic category classification and normalization
// for findings. It uses vector embeddings to match proposed categories against
// existing ones, either returning a similar category or registering a new one.
//
// Implementations must be thread-safe for concurrent use.
type CategoryClassifier interface {
	// Classify normalizes a category via semantic matching.
	//
	// It embeds the proposed category and description, searches for similar
	// existing categories, and either returns a matching category (if similarity
	// exceeds the threshold) or registers the proposed category as new.
	//
	// Parameters:
	//   - ctx: Context for cancellation and tracing
	//   - proposed: The proposed category name (e.g., "jailbreaking")
	//   - description: Additional context about the category
	//
	// Returns the normalized category name (either matched or newly registered)
	// and an error if classification fails.
	Classify(ctx context.Context, proposed, description string) (string, error)

	// Register explicitly adds a category to the classifier's index.
	//
	// This method embeds the category information and stores it in the vector
	// store for future matching. It is idempotent - registering an existing
	// category is a no-op.
	//
	// Parameters:
	//   - ctx: Context for cancellation and tracing
	//   - info: Category metadata including name, domain, and description
	//
	// Returns an error if registration fails.
	Register(ctx context.Context, info registry.CategoryInfo) error

	// Search finds similar categories using semantic similarity.
	//
	// It embeds the query text and returns the top-K most similar categories
	// from the vector store, sorted by descending similarity score.
	//
	// Parameters:
	//   - ctx: Context for cancellation and tracing
	//   - query: The search query text
	//   - topK: Maximum number of results to return
	//
	// Returns a slice of CategoryMatch results sorted by score (highest first)
	// and an error if search fails. Returns an empty slice if no matches found.
	Search(ctx context.Context, query string, topK int) ([]CategoryMatch, error)

	// Bootstrap loads categories from a registry into the classifier's index.
	//
	// This method efficiently embeds and stores all categories from the provided
	// registry using batch embedding. It is idempotent - categories already in
	// the store are skipped.
	//
	// Parameters:
	//   - ctx: Context for cancellation and tracing
	//   - registry: CategoryRegistry containing categories to index
	//
	// Returns an error if bootstrap fails. Partial failures return an error
	// indicating which categories failed to index.
	Bootstrap(ctx context.Context, reg *registry.CategoryRegistry) error
}
