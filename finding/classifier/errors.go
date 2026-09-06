// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Zero Root AI

package classifier

// Error codes for category classifier operations.
// These follow the pattern established in the SDK and daemon packages.
const (
	// ErrCodeEmbedderUnavailable indicates that the embedder service is not available
	// or failed to initialize. This is a critical error that prevents classifier operation.
	ErrCodeEmbedderUnavailable = "CLASSIFIER_EMBEDDER_UNAVAILABLE"

	// ErrCodeEmbeddingFailed indicates that text embedding failed due to a transient
	// or permanent error. The classifier may fall back to passthrough behavior.
	ErrCodeEmbeddingFailed = "CLASSIFIER_EMBEDDING_FAILED"

	// ErrCodeSearchFailed indicates that vector store search failed. The classifier
	// may fall back to registering the proposed category without matching.
	ErrCodeSearchFailed = "CLASSIFIER_SEARCH_FAILED"

	// ErrCodeBootstrapFailed indicates that category bootstrap failed partially or
	// completely. This may occur if some categories fail to embed or store.
	ErrCodeBootstrapFailed = "CLASSIFIER_BOOTSTRAP_FAILED"

	// ErrCodeInvalidConfig indicates that the classifier configuration is invalid.
	// Common causes: negative threshold, invalid store type, missing required fields.
	ErrCodeInvalidConfig = "CLASSIFIER_INVALID_CONFIG"
)
