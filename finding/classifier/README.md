# Category Classifier

The `classifier` package provides semantic category classification and normalization for security findings using vector embeddings. It allows LLMs to generate arbitrary category strings while ensuring they are automatically mapped to existing categories via semantic similarity.

## Overview

When agents submit findings, they often generate category names that are semantically similar but syntactically different (e.g., "SQL injection", "sql-injection", "database injection"). The classifier normalizes these variations by:

1. Embedding the proposed category name and description
2. Searching for semantically similar existing categories
3. Returning a matching category if similarity exceeds the threshold (default: 0.85)
4. Registering the proposed category as new if no match is found

## Interfaces

### CategoryClassifier

The main interface for category classification:

```go
type CategoryClassifier interface {
    // Classify normalizes a category via semantic matching
    Classify(ctx context.Context, proposed, description string) (string, error)

    // Register explicitly adds a category to the classifier's index
    Register(ctx context.Context, info registry.CategoryInfo) error

    // Search finds similar categories using semantic similarity
    Search(ctx context.Context, query string, topK int) ([]CategoryMatch, error)

    // Bootstrap loads categories from a registry into the classifier's index
    Bootstrap(ctx context.Context, reg *registry.CategoryRegistry) error
}
```

All implementations must be thread-safe for concurrent use.

### VectorStore

Abstraction for storing and searching embedding vectors:

```go
type VectorStore interface {
    // Upsert adds or updates a category embedding
    Upsert(ctx context.Context, id string, embedding []float64, metadata map[string]any) error

    // Search finds the nearest neighbors to the given embedding
    Search(ctx context.Context, embedding []float64, topK int) ([]SearchResult, error)

    // Delete removes a category from the store
    Delete(ctx context.Context, id string) error

    // Count returns the total number of categories stored
    Count(ctx context.Context) (int, error)
}
```

All methods must be thread-safe for concurrent access.

## Implementations

### MemoryStore

In-memory vector store implementation using cosine similarity:

```go
type MemoryStore struct {
    // Thread-safe via sync.RWMutex
}

func NewMemoryStore() *MemoryStore
```

**Characteristics:**
- Uses slice-based storage with linear search
- Employs cosine similarity for vector matching
- Thread-safe via `sync.RWMutex`
- Suitable for testing and deployments with < 10,000 categories
- For larger deployments, use Qdrant or another vector database

**Similarity Metric:**
- Returns scores between -1 and 1
- 1 = identical vectors
- 0 = orthogonal vectors
- -1 = opposite vectors

## Configuration

```go
type Config struct {
    // Threshold is the minimum similarity score for matching (default: 0.85)
    Threshold float64 `json:"threshold" yaml:"threshold"`

    // AutoRegister controls whether new categories are automatically registered
    // when no similar category is found (default: true)
    AutoRegister bool `json:"auto_register" yaml:"auto_register"`

    // StoreType specifies the vector store backend
    // Valid values: "memory", "qdrant" (default: "memory")
    StoreType string `json:"store_type" yaml:"store_type"`
}

// Get default configuration
cfg := classifier.DefaultConfig()
```

**Default Values:**
- `Threshold`: 0.85
- `AutoRegister`: true
- `StoreType`: "memory"

## Types

### CategoryMatch

Represents a category search result with similarity score:

```go
type CategoryMatch struct {
    Category    string  `json:"category"`     // Matched category name
    Domain      string  `json:"domain"`       // Category domain (e.g., "security")
    Description string  `json:"description"`  // Category description
    Score       float64 `json:"score"`        // Similarity score (0.0 to 1.0)
}
```

### SearchResult

Represents a vector search result:

```go
type SearchResult struct {
    ID       string         `json:"id"`       // Unique category identifier
    Score    float64        `json:"score"`    // Similarity score (0.0 to 1.0)
    Metadata map[string]any `json:"metadata"` // Additional metadata
}
```

Common metadata fields:
- `domain` (string): Category domain
- `description` (string): Category description
- `created_at` (time.Time): Creation timestamp

## Usage Examples

### Creating a Classifier

```go
import (
    "context"

    "github.com/zeroroot-ai/sdk/finding/classifier"
    "github.com/zeroroot-ai/sdk/finding/classifier/store"
    "github.com/zeroroot-ai/sdk/finding/registry"
    "github.com/zeroroot-ai/sdk/llm/embedder"
)

// Create embedder (required for generating vector embeddings)
emb := embedder.NewOpenAI("text-embedding-3-small", apiKey)

// Create vector store
vectorStore := store.NewMemoryStore()

// Create classifier with default config
cfg := classifier.DefaultConfig()
clf := classifier.New(emb, vectorStore, cfg)

// Or create with custom config
customCfg := classifier.Config{
    Threshold:    0.90,  // Stricter matching
    AutoRegister: false, // Manual registration only
    StoreType:    "memory",
}
clf := classifier.New(emb, vectorStore, customCfg)
```

### Bootstrapping Categories

Load predefined categories from a registry:

```go
import "github.com/zeroroot-ai/sdk/finding/registry"

ctx := context.Background()

// Create registry and add categories
reg := registry.NewCategoryRegistry()
reg.Add(registry.CategoryInfo{
    Name:        "sql_injection",
    Domain:      "security",
    Description: "SQL injection vulnerabilities allowing database manipulation",
})
reg.Add(registry.CategoryInfo{
    Name:        "xss",
    Domain:      "security",
    Description: "Cross-site scripting vulnerabilities allowing script injection",
})
reg.Add(registry.CategoryInfo{
    Name:        "authentication_bypass",
    Domain:      "security",
    Description: "Authentication mechanism bypass allowing unauthorized access",
})

// Bootstrap the classifier
if err := clf.Bootstrap(ctx, reg); err != nil {
    log.Fatalf("Bootstrap failed: %v", err)
}

// Verify categories were loaded
count, _ := vectorStore.Count(ctx)
log.Printf("Loaded %d categories", count)
```

### Classifying a Finding Category

```go
ctx := context.Background()

// LLM generates category string
proposed := "database injection"
description := "Unsanitized user input allows arbitrary SQL execution"

// Classify the category
normalized, err := clf.Classify(ctx, proposed, description)
if err != nil {
    log.Fatalf("Classification failed: %v", err)
}

log.Printf("Proposed: %s", proposed)
log.Printf("Normalized: %s", normalized)
// Output: Normalized: sql_injection

// Another example
proposed = "jailbreaking"
description = "Bypassing LLM safety constraints to generate harmful content"

normalized, err = clf.Classify(ctx, proposed, description)
if err != nil {
    log.Fatalf("Classification failed: %v", err)
}

log.Printf("Normalized: %s", normalized)
// Output: Normalized: jailbreaking (new category registered)
```

### Searching for Similar Categories

```go
ctx := context.Background()

// Search for categories matching a query
matches, err := clf.Search(ctx, "injection attack", 3)
if err != nil {
    log.Fatalf("Search failed: %v", err)
}

log.Printf("Found %d similar categories:", len(matches))
for _, match := range matches {
    log.Printf("- %s (domain: %s, score: %.3f)",
        match.Category, match.Domain, match.Score)
}

// Output:
// Found 3 similar categories:
// - sql_injection (domain: security, score: 0.923)
// - xss (domain: security, score: 0.781)
// - authentication_bypass (domain: security, score: 0.654)
```

### Manual Category Registration

```go
ctx := context.Background()

// Register a category explicitly
info := registry.CategoryInfo{
    Name:        "prompt_injection",
    Domain:      "ai_security",
    Description: "Prompt injection attacks against LLM applications",
}

if err := clf.Register(ctx, info); err != nil {
    log.Fatalf("Registration failed: %v", err)
}

// Registration is idempotent - registering again is a no-op
if err := clf.Register(ctx, info); err != nil {
    log.Fatalf("Second registration failed: %v", err)
}
```

### Complete Agent Integration

```go
package main

import (
    "context"
    "log"

    "github.com/zeroroot-ai/sdk/agent"
    "github.com/zeroroot-ai/sdk/finding"
    "github.com/zeroroot-ai/sdk/finding/classifier"
    "github.com/zeroroot-ai/sdk/finding/classifier/store"
    "github.com/zeroroot-ai/sdk/finding/registry"
    "github.com/zeroroot-ai/sdk/llm/embedder"
)

type SecurityAgent struct {
    classifier classifier.CategoryClassifier
}

func (a *SecurityAgent) Initialize(ctx context.Context, cfg agent.AgentConfig) error {
    // Create embedder
    emb := embedder.NewOpenAI("text-embedding-3-small", cfg.OpenAIKey)

    // Create classifier
    vectorStore := store.NewMemoryStore()
    a.classifier = classifier.New(emb, vectorStore, classifier.DefaultConfig())

    // Bootstrap with predefined categories
    reg := registry.NewCategoryRegistry()
    reg.Add(registry.CategoryInfo{
        Name:        "sql_injection",
        Domain:      "security",
        Description: "SQL injection vulnerabilities",
    })
    // ... add more categories

    if err := a.classifier.Bootstrap(ctx, reg); err != nil {
        return err
    }

    return nil
}

func (a *SecurityAgent) Execute(ctx context.Context, task agent.Task, h agent.Harness) (agent.Result, error) {
    // LLM generates finding with arbitrary category
    llmCategory := "database command injection"
    llmDescription := "SQL injection in login form allows data extraction"

    // Normalize the category
    normalizedCategory, err := a.classifier.Classify(ctx, llmCategory, llmDescription)
    if err != nil {
        log.Printf("Classification failed, using original: %v", err)
        normalizedCategory = llmCategory
    }

    // Submit finding with normalized category
    f := &finding.Finding{
        Title:       "SQL Injection in Login Form",
        Description: llmDescription,
        Category:    normalizedCategory,  // "sql_injection"
        Severity:    finding.SeverityHigh,
        Confidence:  0.95,
    }

    if err := h.SubmitFinding(ctx, f); err != nil {
        return agent.Result{}, err
    }

    return agent.NewResult(task.ID).Complete(nil), nil
}
```

## How It Works

### Classification Flow

1. **Embed Input**: The proposed category and description are combined and embedded using the configured embedder
2. **Search Store**: The embedding is used to search the vector store for the top-3 most similar categories
3. **Compare Threshold**: If the best match has a score >= threshold (default 0.85), return the matched category
4. **Auto-Register**: If no match exceeds threshold and `AutoRegister` is true, register the proposed category as new
5. **Return Result**: Return the normalized category name (either matched or newly registered)

### Similarity Threshold

The threshold determines how strictly categories must match:

| Threshold | Behavior |
|-----------|----------|
| 0.95-1.0  | Very strict - only near-identical matches |
| 0.85-0.94 | Balanced - semantically similar matches (default: 0.85) |
| 0.70-0.84 | Lenient - broader semantic matches |
| < 0.70    | Very lenient - may produce false positives |

### Thread Safety

All implementations are thread-safe:
- `MemoryStore` uses `sync.RWMutex` for concurrent access
- Multiple goroutines can call `Classify`, `Search`, and `Register` simultaneously
- Embedding operations may have their own concurrency controls

## Performance Considerations

### MemoryStore

- **Linear search**: O(n) where n = number of categories
- **Suitable for**: < 10,000 categories
- **Memory usage**: ~4KB per category (for typical embeddings)
- **Concurrent reads**: No blocking between readers
- **Concurrent writes**: Blocked during writes

### Optimization Tips

1. **Batch bootstrap**: Use `Bootstrap()` instead of multiple `Register()` calls
2. **Cache embeddings**: Store frequently used category embeddings
3. **Use Qdrant for scale**: For > 10,000 categories, use a dedicated vector database
4. **Monitor latency**: Track embedding + search time for performance regression

## Error Handling

Common errors and their causes:

```go
// Embedding failure (network, API key, etc.)
_, err := clf.Classify(ctx, "category", "description")
if err != nil {
    // Check if embedder error
    log.Printf("Embedding failed: %v", err)
}

// Vector store error
_, err = clf.Search(ctx, "query", 10)
if err != nil {
    // Check vector store health
    log.Printf("Search failed: %v", err)
}

// Bootstrap partial failure
err = clf.Bootstrap(ctx, reg)
if err != nil {
    // Some categories may have been indexed
    log.Printf("Bootstrap incomplete: %v", err)
}
```

## Testing

```go
import (
    "testing"

    "github.com/stretchr/testify/assert"
    "github.com/zeroroot-ai/sdk/finding/classifier"
    "github.com/zeroroot-ai/sdk/finding/classifier/store"
)

func TestClassifier(t *testing.T) {
    // Create mock embedder that returns fixed embeddings
    mockEmbedder := &MockEmbedder{
        embeddings: map[string][]float64{
            "sql_injection": {0.1, 0.2, 0.3, ...},
            "xss":          {0.4, 0.5, 0.6, ...},
        },
    }

    vectorStore := store.NewMemoryStore()
    clf := classifier.New(mockEmbedder, vectorStore, classifier.DefaultConfig())

    // Test classification
    ctx := context.Background()
    normalized, err := clf.Classify(ctx, "database injection", "SQL injection")

    assert.NoError(t, err)
    assert.Equal(t, "sql_injection", normalized)
}
```

## See Also

- [Finding Registry](../registry/README.md) - Category registry management
- [Finding Submission](../README.md) - Creating and submitting findings
- [Agent Harness](../../../agent/README.md) - Agent integration guide
