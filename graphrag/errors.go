// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Zero Root AI

package graphrag

import "errors"

// Sentinel errors for GraphRAG operations.
// These errors can be used with errors.Is() for error checking.
var (
	// ErrGraphRAGNotEnabled indicates that GraphRAG functionality is not configured
	// or enabled in the Gibson framework. This typically means the GraphRAG storage
	// backend is not initialized or the feature is disabled in configuration.
	//
	// Example:
	//	result, err := client.Query(ctx, query)
	//	if errors.Is(err, graphrag.ErrGraphRAGNotEnabled) {
	//	    log.Warn("GraphRAG is not enabled, falling back to basic search")
	//	}
	ErrGraphRAGNotEnabled = errors.New("graphrag not enabled")

	// ErrNodeNotFound indicates that the requested node does not exist in the graph.
	// This can occur during relationship creation, node updates, or graph traversal.
	//
	// Example:
	//	err := client.CreateRelationship(ctx, relationship)
	//	if errors.Is(err, graphrag.ErrNodeNotFound) {
	//	    log.Errorf("Cannot create relationship: node not found")
	//	}
	ErrNodeNotFound = errors.New("node not found")

	// ErrInvalidQuery indicates that the query validation failed. This can occur when:
	//   - Both Text and Embedding are provided (must provide exactly one)
	//   - Neither Text nor Embedding is provided
	//   - TopK is less than or equal to 0
	//   - MaxHops is less than or equal to 0
	//   - MinScore is not between 0.0 and 1.0
	//   - VectorWeight or GraphWeight is negative
	//   - VectorWeight + GraphWeight does not equal 1.0
	//
	// Always call query.Validate() before executing to catch validation errors early.
	//
	// Example:
	//	query := graphrag.NewQuery("").WithTopK(-1)  // Invalid
	//	if err := query.Validate(); errors.Is(err, graphrag.ErrInvalidQuery) {
	//	    log.Errorf("Query validation failed: %v", err)
	//	}
	ErrInvalidQuery = errors.New("invalid query")

	// ErrEmbeddingFailed indicates that the embedding generation process failed.
	// This can occur when:
	//   - The embedding model is not available
	//   - The input text is too long for the model
	//   - The embedding service returns an error
	//   - Network connectivity issues with remote embedding APIs
	//
	// Example:
	//	result, err := client.Query(ctx, query)
	//	if errors.Is(err, graphrag.ErrEmbeddingFailed) {
	//	    log.Errorf("Failed to generate embeddings: %v", err)
	//	}
	ErrEmbeddingFailed = errors.New("embedding generation failed")

	// ErrStorageFailed indicates that a storage operation failed. This can occur during:
	//   - Node creation or updates
	//   - Relationship creation
	//   - Index updates
	//   - Transaction commits
	//   - Database connectivity issues
	//
	// The underlying error should be wrapped for additional context about the
	// specific storage failure.
	//
	// Example:
	//	err := harness.Observe(ctx, obs)
	//	if errors.Is(err, graphrag.ErrStorageFailed) {
	//	    log.Errorf("Storage operation failed: %v", err)
	//	}
	ErrStorageFailed = errors.New("storage operation failed")

	// ErrQueryTimeout indicates that the query execution exceeded the configured timeout.
	// This can occur for complex queries with:
	//   - Large MaxHops values requiring deep graph traversal
	//   - High TopK values requiring extensive result processing
	//   - Large datasets requiring full index scans
	//   - Slow embedding generation
	//
	// Consider reducing MaxHops, TopK, or increasing the timeout for complex queries.
	//
	// Example:
	//	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	//	defer cancel()
	//	result, err := client.Query(ctx, query)
	//	if errors.Is(err, graphrag.ErrQueryTimeout) {
	//	    log.Warn("Query timed out, consider reducing MaxHops or TopK")
	//	}
	ErrQueryTimeout = errors.New("query timeout")

	// ErrRelationshipFailed indicates that a relationship operation failed. This can occur when:
	//   - Creating a relationship between non-existent nodes
	//   - Invalid relationship type
	//   - Relationship validation fails
	//   - Duplicate relationships (if uniqueness is enforced)
	//   - Storage backend errors
	//
	// Always validate relationships before creating them to catch errors early.
	//
	// Example:
	//	rel := graphrag.NewRelationship("node1", "node2", "RELATES_TO")
	//	err := client.CreateRelationship(ctx, rel)
	//	if errors.Is(err, graphrag.ErrRelationshipFailed) {
	//	    log.Errorf("Failed to create relationship: %v", err)
	//	}
	ErrRelationshipFailed = errors.New("relationship operation failed")
)
