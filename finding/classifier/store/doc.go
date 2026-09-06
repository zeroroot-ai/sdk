// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Zero Root AI

// Package store provides implementations of the VectorStore interface for
// storing and searching category embeddings.
//
// The VectorStore abstraction allows pluggable backends for vector storage,
// enabling different implementations based on deployment requirements:
//
//   - MemoryStore: In-memory storage with linear search, suitable for testing
//     and deployments with < 10,000 categories
//   - QdrantStore: Production-ready distributed vector database (future)
//   - PgvectorStore: PostgreSQL with pgvector extension (future)
//
// All implementations must be thread-safe and implement cosine similarity
// for vector search operations.
package store
