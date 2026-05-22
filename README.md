<p align="center">
  <img src="https://github.com/dakera-ai.png" alt="Dakera AI" width="80" />
</p>

<h1 align="center">dakera-go</h1>

<p align="center">
  Go client for <a href="https://dakera.ai">Dakera AI</a> — the memory engine for AI agents
</p>

<p align="center">
  <a href="https://github.com/Dakera-AI/dakera-go/actions/workflows/ci.yml"><img alt="CI" src="https://github.com/Dakera-AI/dakera-go/actions/workflows/ci.yml/badge.svg" /></a>
  <a href="https://pkg.go.dev/github.com/Dakera-AI/dakera-go"><img alt="Go Reference" src="https://pkg.go.dev/badge/github.com/Dakera-AI/dakera-go.svg" /></a>
  <a href="LICENSE"><img alt="License: MIT" src="https://img.shields.io/github/license/Dakera-AI/dakera-go" /></a>
  <a href="https://dakera.ai/docs"><img alt="Docs" src="https://img.shields.io/badge/docs-dakera.ai%2Fdocs-3b82f6?style=flat-square" /></a>
  <a href="https://dakera.ai/benchmark"><img alt="LoCoMo 87.8%" src="https://img.shields.io/badge/LoCoMo-87.8%25-22c55e?style=flat-square" /></a>
</p>

---

## Why Dakera?

| | Dakera | Others |
|---|---|---|
| **LoCoMo accuracy** | **87.8%** (1,540 Q standard eval) | 60–92% |
| **Deployment** | Single binary, Docker one-liner | External vector DB + embedding service required |
| **Embeddings** | Built-in — no OpenAI key needed | Requires external embedding API |
| **Search modes** | Vector · BM25 · Hybrid · Knowledge Graph | Usually one or two |
| **Dependencies** | stdlib `net/http` only | Often pulls in gRPC or third-party HTTP clients |

→ [Full benchmark results](https://dakera.ai/benchmark) · [dakera.ai](https://dakera.ai)

---

## Run Dakera

```bash
docker run -d \
  --name dakera \
  -p 3000:3000 \
  -e DAKERA_ROOT_API_KEY=dk-mykey \
  ghcr.io/dakera-ai/dakera:latest

curl http://localhost:3000/health  # → {"status":"ok"}
```

For persistent storage with Docker Compose:

```bash
curl -sSfL https://raw.githubusercontent.com/Dakera-AI/dakera-deploy/main/docker-compose.yml \
  -o docker-compose.yml
DAKERA_API_KEY=dk-mykey docker compose up -d
```

Full deployment guide (Docker Compose, Kubernetes, Helm): [dakera-deploy](https://github.com/Dakera-AI/dakera-deploy)

---

## Install

```bash
go get github.com/dakera-ai/dakera-go@latest
```

Requires Go 1.21+. Uses only the standard library — no external runtime dependencies.

---

## Quick Start

```go
package main

import (
    "context"
    "fmt"
    dakera "github.com/dakera-ai/dakera-go"
)

func main() {
    client := dakera.NewClientWithOptions(dakera.ClientOptions{
        BaseURL: "http://localhost:3000",
        APIKey:  "dk-mykey",
    })
    ctx := context.Background()

    // Store an agent memory
    imp := float32(0.9)
    mem, _ := client.StoreMemory(ctx, "my-agent", dakera.StoreMemoryRequest{
        Content:    "User prefers concise responses with code examples",
        Importance: &imp,
    })
    fmt.Println("Stored:", mem.Memory.ID)

    // Recall memories (semantic search)
    resp, _ := client.Recall(ctx, "my-agent", dakera.RecallRequest{
        Query: "what does the user prefer?",
        TopK:  5,
    })
    for _, m := range resp.Memories {
        fmt.Printf("[%.2f] %s\n", m.Score, m.Content)
    }

    // Upsert vectors
    client.Upsert(ctx, "my-namespace", []dakera.VectorInput{
        {ID: "vec1", Values: []float32{0.1, 0.2, 0.3}},
    })

    // Hybrid search (vector + BM25)
    results, _ := client.HybridSearch(ctx, "my-namespace", nil, "completed task", &dakera.HybridSearchOptions{
        TopK:         5,
        VectorWeight: 0.7,
    })
    for _, r := range results {
        fmt.Println(r.ID, r.Score)
    }
}
```

### Context-based cancellation

All methods accept a `context.Context` — use it for timeouts, deadlines, or cancellation:

```go
ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
defer cancel()

resp, err := client.Recall(ctx, "my-agent", dakera.RecallRequest{Query: "preferences", TopK: 3})
```

### Text auto-embedding

Send raw text without pre-computing embeddings — Dakera embeds server-side:

```go
client.UpsertText(ctx, "my-namespace", []dakera.TextInput{
    {ID: "doc1", Text: "Agent completed onboarding flow successfully"},
})
```

---

## Features

- **Agent Memory** — store, recall, search, and forget memories with importance scoring
- **Sessions** — group memories by conversation with auto-consolidation on session end
- **Knowledge Graph** — traverse memory relationships, find paths, export graphs
- **Vector Search** — ANN queries with metadata filters and batch operations
- **Full-Text Search** — BM25 ranking with stemming and stop-word filtering
- **Hybrid Search** — combine vector similarity with keyword matching
- **Text Auto-Embedding** — server-side embedding generation (no local model needed)
- **Namespaces** — isolated vector stores per project, tenant, or use case
- **Feedback Loop** — upvote/downvote/flag memories to improve recall quality
- **Entity Extraction** — GLiNER NER for automatic entity detection
- **SSE Streaming** — Server-sent event subscriptions for real-time memory updates
- **Typed Filters** — `Eq()`, `Gt()`, `Contains()`, `ArrayContains()` and more
- **Context-Based** — all methods accept `context.Context` for cancellation and timeouts
- **Retry & Rate Limiting** — built-in exponential backoff and rate-limit header tracking
- **Zero Dependencies** — standard library `net/http` client, no external runtime deps

---

## Connect to Dakera

```go
// Self-hosted
client := dakera.NewClientWithOptions(dakera.ClientOptions{
    BaseURL: "http://your-server:3000",
    APIKey:  "your-key",
})

// Cloud (early access)
client := dakera.NewClientWithOptions(dakera.ClientOptions{
    BaseURL: "http://localhost:3000",
    APIKey:  "your-key",
})

// With custom retry config
client := dakera.NewClientWithOptions(dakera.ClientOptions{
    BaseURL:     "http://localhost:3000",
    APIKey:      "your-key",
    RetryConfig: &dakera.RetryConfig{MaxRetries: 5, BaseDelayMs: 200},
})
```

---

## Examples

See the [`examples/`](examples/) directory:

- [`basic/`](examples/basic/main.go) — vectors, namespaces, queries, filters
- [`memory/`](examples/memory/main.go) — store/recall memories, sessions, agent stats
- [`advanced/`](examples/advanced/main.go) — text embedding, full-text, hybrid search, analytics

---

## Resources

| | |
|---|---|
| [Documentation](https://dakera.ai/docs) | Full API reference and guides |
| [Go SDK docs](https://pkg.go.dev/github.com/Dakera-AI/dakera-go) | pkg.go.dev reference |
| [Benchmark](https://dakera.ai/benchmark) | LoCoMo evaluation results |
| [dakera.ai](https://dakera.ai) | Website and early access |
| [GitHub Org](https://github.com/dakera-ai) | All public repos |
| [dakera-deploy](https://github.com/Dakera-AI/dakera-deploy) | Self-hosting guide |

### Other SDKs

| SDK | Package |
|---|---|
| [dakera-py](https://github.com/dakera-ai/dakera-py) | `dakera` (PyPI) |
| [dakera-js](https://github.com/dakera-ai/dakera-js) | `@dakera-ai/dakera` (npm) |
| [dakera-rs](https://github.com/dakera-ai/dakera-rs) | `dakera-client` (crates.io) |
| [dakera-cli](https://github.com/dakera-ai/dakera-cli) | CLI tool |
| [dakera-mcp](https://github.com/dakera-ai/dakera-mcp) | MCP server for Claude/Cursor |

---

<p align="center">
  <a href="https://dakera.ai">dakera.ai</a> ·
  <a href="https://dakera.ai/docs">Docs</a> ·
  <a href="https://dakera.ai/benchmark">Benchmark</a> ·
  <a href="https://dakera.ai#cta">Request Early Access</a>
</p>

<p align="center"><sub>Built with Rust. Single binary. Zero external dependencies.</sub></p>
