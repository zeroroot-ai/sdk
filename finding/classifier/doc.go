// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Zero Root AI

// Package classifier provides semantic category classification for findings
// using vector embeddings and similarity search.
//
// The classifier enables LLM-powered agents to generate findings with arbitrary
// category names, which are then normalized against existing categories via
// semantic similarity, or registered as new categories when no match exists.
//
// # Overview
//
// Instead of requiring agents to know predefined category lists, the classifier
// allows natural language category names like "jailbreaking attempt" to be
// automatically matched to existing categories like "jailbreak" through vector
// similarity. This enables:
//
//   - Domain-agnostic category management (security, compliance, infrastructure, etc.)
//   - LLM-driven category generation without predefined taxonomies
//   - Self-improving category databases that grow with usage
//   - Semantic matching that handles synonyms and variations
//
// # Architecture
//
// The package defines three core interfaces:
//
//   - CategoryClassifier: Main interface for classification operations
//   - VectorStore: Pluggable backend for storing and searching embeddings
//   - Embedder: Interface for text-to-vector embedding generation
//
// Implementations are provided in the daemon (core/gibson/internal/finding/classifier.go)
// using ONNX Runtime for local embedding and various vector store backends.
//
// # Basic Usage
//
// Agents typically interact with the classifier through the AgentHarness, which
// automatically normalizes categories during finding submission:
//
//	finding := &finding.Finding{
//		Title:       "Jailbreak via system prompt override",
//		Category:    "prompt override attack",  // LLM-generated category
//		Description: "Attacker bypassed safety controls by...",
//		Severity:    finding.SeverityHigh,
//	}
//
//	// If classifier is enabled, category is normalized to "jailbreak"
//	err := harness.SubmitFinding(ctx, finding)
//
// # Direct Usage
//
// For advanced use cases, the classifier can be used directly:
//
//	// Classify a category
//	normalized, err := classifier.Classify(ctx, "sql_injections", "Database injection attacks")
//	// Returns "sql_injection" if it exists in the store
//
//	// Search for similar categories
//	matches, err := classifier.Search(ctx, "privilege escalation", 5)
//	for _, match := range matches {
//		fmt.Printf("%s: %.2f\n", match.Category, match.Score)
//	}
//
//	// Register a new category
//	err = classifier.Register(ctx, registry.CategoryInfo{
//		Name:        "cost_anomaly",
//		Domain:      "infrastructure",
//		DisplayName: "Cost Anomaly",
//		Description: "Unexpected increase in cloud spending",
//	})
//
//	// Bootstrap from a registry
//	reg := registry.DefaultRegistry()
//	err = classifier.Bootstrap(ctx, reg)
//
// # Configuration
//
// The classifier behavior is controlled by Config:
//
//	config := classifier.Config{
//		Threshold:    0.85,    // Similarity threshold for matching
//		AutoRegister: true,    // Auto-register new categories
//		StoreType:    "memory", // Vector store backend
//	}
//
// Threshold determines the similarity cutoff:
//   - Score >= threshold: Return existing category
//   - Score < threshold: Register proposed category as new
//
// # Vector Stores
//
// The package supports multiple vector store backends through the VectorStore
// interface:
//
//   - MemoryStore: In-memory storage for testing and simple deployments
//   - QdrantStore: Persistent storage using Qdrant vector database
//   - Custom implementations: Implement VectorStore for other backends
//
// # Embeddings
//
// The classifier requires an Embedder implementation to generate vector embeddings.
// The daemon uses ONNX Runtime with the all-MiniLM-L6-v2 model (384 dimensions)
// for fast, local, offline embedding generation.
//
// # Thread Safety
//
// All interfaces are designed for safe concurrent access. Implementations must
// protect shared state with appropriate synchronization primitives.
//
// # Error Handling
//
// The classifier uses standard error codes (see errors.go) and follows graceful
// degradation principles:
//
//   - If embedding fails: Log warning, use proposed category as-is
//   - If search fails: Log error, register proposed category
//   - If registration fails: Return error but don't block finding submission
//
// # Integration Points
//
// The classifier integrates with:
//
//   - registry.CategoryRegistry: Bootstrap existing categories
//   - finding.Finding: Category normalization during submission
//   - harness.AgentHarness: Transparent classification in finding mission
//   - memory/embedder: ONNX-based embedding generation
//
// # Future Extensions
//
// Planned enhancements include:
//
//   - Qdrant vector store for persistent, scalable storage
//   - Neo4j integration for cross-mission category discovery
//   - Model checksum verification for security
//   - Batch classification for efficiency
//   - Category aliasing and merging
package classifier
