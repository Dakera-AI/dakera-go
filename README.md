# Dakera Go SDK

[![CI](https://github.com/dakera-ai/dakera-go/actions/workflows/ci.yml/badge.svg)](https://github.com/dakera-ai/dakera-go/actions/workflows/ci.yml)
[![Go Reference](https://pkg.go.dev/badge/github.com/dakera-ai/dakera-go.svg)](https://pkg.go.dev/github.com/dakera-ai/dakera-go)
[![License: MIT](https://img.shields.io/badge/License-MIT-blue.svg)](LICENSE)
[![Go](https://img.shields.io/badge/Go-1.21+-00ADD8)](https://go.dev/)

Official Go client for [Dakera](https://dakera.ai) — a high-performance vector database for AI agent memory.

## Installation

```bash
go get github.com/dakera-ai/dakera-go
```

## Quick Start

```go
package main

import (
    "context"
    "fmt"
    "log"

    dakera "github.com/dakera-ai/dakera-go"
)

func main() {
    // Connect to Dakera
    client := dakera.NewClient("http://localhost:3000")
    ctx := context.Background()

    // Upsert vectors
    _, err := client.Upsert(ctx, "my-namespace", []dakera.VectorInput{
        {ID: "vec1", Values: []float32{0.1, 0.2, 0.3}, Metadata: map[string]interface{}{"label": "a"}},
        {ID: "vec2", Values: []float32{0.4, 0.5, 0.6}, Metadata: map[string]interface{}{"label": "b"}},
    })
    if err != nil {
        log.Fatal(err)
    }

    // Query similar vectors
    results, err := client.Query(ctx, "my-namespace", []float32{0.1, 0.2, 0.3}, &dakera.QueryOptions{
        TopK: 10,
    })
    if err != nil {
        log.Fatal(err)
    }

    for _, result := range results.Results {
        fmt.Printf("%s: %f\n", result.ID, result.Score)
    }
}
```

## Features

- **Idiomatic Go**: Proper error handling, context support, and Go conventions
- **Vector Operations**: Upsert, query, delete, fetch vectors
- **Full-Text Search**: Index documents and perform BM25 search
- **Hybrid Search**: Combine vector and text search with configurable weights
- **Namespace Management**: Create, list, delete namespaces
- **Agent Memory**: Store, recall, and manage memories for AI agents
- **Metadata Filtering**: Filter queries by metadata fields with helper functions
- **Automatic Retries**: Built-in retry logic with exponential backoff
- **Error Handling**: Typed errors for different error scenarios

## Usage Examples

### Vector Operations

```go
package main

import (
    "context"
    "log"

    dakera "github.com/dakera-ai/dakera-go"
)

func main() {
    client := dakera.NewClientWithOptions(dakera.ClientOptions{
        BaseURL:    "http://localhost:3000",
        APIKey:     "your-api-key", // optional
        MaxRetries: 5,
    })
    ctx := context.Background()

    // Upsert vectors
    _, err := client.Upsert(ctx, "my-namespace", []dakera.VectorInput{
        {ID: "vec1", Values: []float32{0.1, 0.2, 0.3}, Metadata: map[string]interface{}{"category": "A"}},
        {ID: "vec2", Values: []float32{0.4, 0.5, 0.6}, Metadata: map[string]interface{}{"category": "B"}},
    })
    if err != nil {
        log.Fatal(err)
    }

    // Query with metadata filter
    results, err := client.Query(ctx, "my-namespace", []float32{0.1, 0.2, 0.3}, &dakera.QueryOptions{
        TopK: 5,
        Filter: map[string]interface{}{
            "category": dakera.Eq("A"),
        },
        IncludeMetadata: true,
    })
    if err != nil {
        log.Fatal(err)
    }

    // Batch query
    batchResults, err := client.BatchQuery(ctx, "my-namespace", []dakera.BatchQuerySpec{
        {Vector: []float32{0.1, 0.2, 0.3}, TopK: 5},
        {Vector: []float32{0.4, 0.5, 0.6}, TopK: 3},
    })
    if err != nil {
        log.Fatal(err)
    }

    // Fetch vectors by ID
    vectors, err := client.Fetch(ctx, "my-namespace", []string{"vec1", "vec2"}, nil)
    if err != nil {
        log.Fatal(err)
    }

    // Delete vectors
    _, err = client.Delete(ctx, "my-namespace", dakera.DeleteOptions{
        IDs: []string{"vec1", "vec2"},
    })
    if err != nil {
        log.Fatal(err)
    }

    // Delete by filter
    _, err = client.Delete(ctx, "my-namespace", dakera.DeleteOptions{
        Filter: map[string]interface{}{
            "category": dakera.Eq("obsolete"),
        },
    })
    if err != nil {
        log.Fatal(err)
    }
}
```

### Full-Text Search

```go
// Index documents
_, err := client.IndexDocuments(ctx, "my-namespace", []dakera.DocumentInput{
    {ID: "doc1", Content: "Machine learning is transforming industries"},
    {ID: "doc2", Content: "Vector databases enable semantic search"},
})
if err != nil {
    log.Fatal(err)
}

// Search
results, err := client.FulltextSearch(ctx, "my-namespace", "machine learning", nil)
if err != nil {
    log.Fatal(err)
}

for _, result := range results {
    fmt.Printf("%s: %f\n", result.ID, result.Score)
}
```

### Hybrid Search

```go
// Combine vector and text search
results, err := client.HybridSearch(
    ctx,
    "my-namespace",
    []float32{0.1, 0.2, 0.3}, // Query vector
    "machine learning",       // Text query
    &dakera.HybridSearchOptions{
        TopK:  10,
        Alpha: 0.7, // 0 = pure vector, 1 = pure text
    },
)
if err != nil {
    log.Fatal(err)
}

for _, result := range results {
    fmt.Printf("%s: score=%f, vector=%f, text=%f\n",
        result.ID, result.Score, result.VectorScore, result.TextScore)
}
```

### Namespace Management

```go
// Create namespace
info, err := client.CreateNamespace(ctx, "embeddings", &dakera.CreateNamespaceOptions{
    Dimensions: 384,
    IndexType:  "hnsw",
})
if err != nil {
    log.Fatal(err)
}

// List namespaces
namespaces, err := client.ListNamespaces(ctx)
if err != nil {
    log.Fatal(err)
}
for _, ns := range namespaces {
    fmt.Printf("%s: %d vectors\n", ns.Name, ns.VectorCount)
}

// Get namespace info
info, err = client.GetNamespace(ctx, "embeddings")
if err != nil {
    log.Fatal(err)
}
fmt.Printf("Dimensions: %d, Index: %s\n", info.Dimensions, info.IndexType)

// Delete namespace
err = client.DeleteNamespace(ctx, "old-namespace")
if err != nil {
    log.Fatal(err)
}
```

### Metadata Filtering

Dakera supports rich metadata filtering with helper functions:

```go
// Equality
filter1 := map[string]interface{}{
    "status": dakera.Eq("active"),
}

// Comparison
filter2 := map[string]interface{}{
    "price": dakera.Gt(100),
}

// In list
filter3 := map[string]interface{}{
    "category": dakera.In("electronics", "books"),
}

// Logical operators
filter4 := dakera.And(
    map[string]interface{}{"status": dakera.Eq("active")},
    map[string]interface{}{"price": dakera.Lt(1000)},
)

results, err := client.Query(ctx, "products", queryVector, &dakera.QueryOptions{
    Filter: filter4,
    TopK:   20,
})
```

### Error Handling

```go
import (
    dakera "github.com/dakera-ai/dakera-go"
)

results, err := client.Query(ctx, "nonexistent", []float32{0.1, 0.2}, nil)
if err != nil {
    if dakera.IsNotFoundError(err) {
        fmt.Printf("Namespace not found: %v\n", err)
    } else if dakera.IsValidationError(err) {
        fmt.Printf("Invalid request: %v\n", err)
    } else if dakera.IsRateLimitError(err) {
        rateLimitErr := err.(*dakera.RateLimitError)
        fmt.Printf("Rate limited, retry after %d seconds\n", rateLimitErr.RetryAfter)
    } else if dakera.IsServerError(err) {
        fmt.Printf("Server error: %v\n", err)
    } else {
        fmt.Printf("Error: %v\n", err)
    }
}
```

## Configuration

| Option | Type | Default | Description |
|--------|------|---------|-------------|
| `BaseURL` | string | required | Dakera server URL |
| `APIKey` | string | "" | API key for authentication |
| `Timeout` | time.Duration | 30s | Request timeout |
| `MaxRetries` | int | 3 | Max retries for failed requests |
| `Headers` | map[string]string | nil | Additional HTTP headers |

## API Reference

### Client

#### Vector Operations
- `Upsert(ctx, namespace, vectors)` - Insert or update vectors
- `Query(ctx, namespace, vector, options)` - Query similar vectors
- `Delete(ctx, namespace, options)` - Delete vectors
- `Fetch(ctx, namespace, ids, options)` - Fetch vectors by ID
- `BatchQuery(ctx, namespace, queries)` - Execute multiple queries

#### Full-Text Operations
- `IndexDocuments(ctx, namespace, documents)` - Index documents
- `FulltextSearch(ctx, namespace, query, options)` - Text search
- `HybridSearch(ctx, namespace, vector, query, options)` - Hybrid search

#### Namespace Operations
- `ListNamespaces(ctx)` - List all namespaces
- `GetNamespace(ctx, namespace)` - Get namespace info
- `CreateNamespace(ctx, namespace, options)` - Create namespace
- `DeleteNamespace(ctx, namespace)` - Delete namespace

#### Admin Operations
- `Health(ctx)` - Check server health
- `GetIndexStats(ctx, namespace)` - Get index statistics
- `Compact(ctx, namespace)` - Trigger compaction
- `Flush(ctx, namespace)` - Flush pending writes

### Filter Helpers

- `Eq(value)` - Equality filter
- `Ne(value)` - Not equal filter
- `Gt(value)` - Greater than filter
- `Gte(value)` - Greater than or equal filter
- `Lt(value)` - Less than filter
- `Lte(value)` - Less than or equal filter
- `In(values...)` - In list filter
- `Nin(values...)` - Not in list filter
- `And(conditions...)` - Logical AND
- `Or(conditions...)` - Logical OR

### Error Types

- `DakeraError` - Base error type
- `ConnectionError` - Connection failures
- `NotFoundError` - Resource not found (404)
- `ValidationError` - Invalid request (400)
- `RateLimitError` - Rate limit exceeded (429)
- `ServerError` - Server errors (5xx)
- `AuthenticationError` - Auth failures (401)
- `TimeoutError` - Request timeout

### Error Checkers

- `IsNotFoundError(err)` - Check if NotFoundError
- `IsValidationError(err)` - Check if ValidationError
- `IsRateLimitError(err)` - Check if RateLimitError
- `IsServerError(err)` - Check if ServerError
- `IsAuthenticationError(err)` - Check if AuthenticationError
- `IsTimeoutError(err)` - Check if TimeoutError
- `IsConnectionError(err)` - Check if ConnectionError

## Requirements

- Go 1.21 or later

## Development

```bash
# Run tests
go test -v ./...

# Run tests with coverage
go test -cover ./...

# Format code
go fmt ./...

# Lint
golangci-lint run
```

## Related Repositories

| Repository | Description |
|------------|-------------|
| [dakera](https://github.com/dakera-ai/dakera) | Core vector database engine (Rust) |
| [dakera-py](https://github.com/dakera-ai/dakera-py) | Python SDK |
| [dakera-js](https://github.com/dakera-ai/dakera-js) | TypeScript/JavaScript SDK |
| [dakera-rs](https://github.com/dakera-ai/dakera-rs) | Rust SDK |
| [dakera-cli](https://github.com/dakera-ai/dakera-cli) | Command-line interface |
| [dakera-mcp](https://github.com/dakera-ai/dakera-mcp) | MCP Server for AI agent memory |
| [dakera-dashboard](https://github.com/dakera-ai/dakera-dashboard) | Admin dashboard (Leptos/WASM) |
| [dakera-docs](https://github.com/dakera-ai/dakera-docs) | Documentation |
| [dakera-deploy](https://github.com/dakera-ai/dakera-deploy) | Deployment configs and Docker Compose |

## License

MIT License - see [LICENSE](LICENSE) for details.
