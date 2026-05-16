[![Docs](https://img.shields.io/badge/docs-dakera.ai-D4A843)](https://dakera.ai/docs)
# dakera-go



[![CI](https://github.com/Dakera-AI/dakera-go/actions/workflows/ci.yml/badge.svg)](https://github.com/Dakera-AI/dakera-go/actions/workflows/ci.yml) [![Go Reference](https://pkg.go.dev/badge/github.com/Dakera-AI/dakera-go.svg)](https://pkg.go.dev/github.com/Dakera-AI/dakera-go) [![License: MIT](https://img.shields.io/github/license/Dakera-AI/dakera-go)](LICENSE)
[![Docs](https://img.shields.io/badge/docs-dakera.ai-D4A843)](https://dakera.ai/docs)

Go client for Dakera AI — store, recall, and search agent memories against a Dakera instance.

Part of [Dakera AI](https://dakera.ai) — the memory engine for AI agents.

> The Dakera memory engine scores **87.6% on LoCoMo** (1,540 questions, standard eval) — [benchmark details](https://dakera.ai/benchmark)

---

## Run Dakera

You need a running Dakera server before using this SDK. The fastest way:

```bash
docker run -d \
  --name dakera \
  -p 3300:3300 \
  -e DAKERA_ROOT_API_KEY=dk-mykey \
  ghcr.io/dakera-ai/dakera:latest
```

For persistent storage (recommended for anything beyond a quick test):

```bash
curl -sSfL https://raw.githubusercontent.com/Dakera-AI/dakera-deploy/main/docker-compose.yml \
  -o docker-compose.yml
DAKERA_API_KEY=dk-mykey docker compose up -d

curl http://localhost:3300/health  # → {"status":"ok"}
```

Full deployment guide (Docker Compose, Kubernetes, Helm): [dakera-deploy](https://github.com/Dakera-AI/dakera-deploy)

---

## Install

```bash
go get github.com/dakera-ai/dakera-go@latest
```

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
        BaseURL: "http://localhost:3300",
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

    // Full-text search
    results, _ := client.FulltextSearch(ctx, "my-namespace", "completed task", nil)
    for _, r := range results {
        fmt.Println(r.ID, r.Score)
    }
}
```

## Features

- **Agent Memory** — store, recall, search, and forget memories with importance scoring
- **Sessions** — group memories by conversation with auto-consolidation on session end
- **Knowledge Graph** — traverse memory relationships, find paths, export graphs
- **Vector Search** — ANN queries with metadata filters and batch operations
- **Full-Text Search** — BM25 ranking with stemming and stop-word filtering
- **Hybrid Search** — combine vector similarity with keyword matching
- **Text Auto-Embedding** — server-side embedding generation (no local model needed)
- **Feedback Loop** — upvote/downvote/flag memories to improve recall quality
- **Entity Extraction** — GLiNER NER for automatic entity detection
- **Streaming** — SSE event subscriptions for real-time memory updates
- **Typed Filters** — `Eq()`, `Gt()`, `Contains()`, `ArrayContains()` and more
- **Retry & Rate Limiting** — built-in exponential backoff and rate-limit header tracking
- **Zero Dependencies** — standard library HTTP client, no external runtime deps

## Examples

See the [`examples/`](examples/) directory:

- [`basic/`](examples/basic/main.go) — vectors, namespaces, queries, filters
- [`memory/`](examples/memory/main.go) — store/recall memories, sessions, agent stats
- [`advanced/`](examples/advanced/main.go) — text embedding, full-text, hybrid search, analytics

## Connect to Dakera

```go
// Self-hosted
client := dakera.NewClientWithOptions(dakera.ClientOptions{
    BaseURL: "http://your-server:3300",
    APIKey:  "your-key",
})

// Cloud (early access)
client := dakera.NewClientWithOptions(dakera.ClientOptions{
    BaseURL: "https://api.dakera.ai",
    APIKey:  "your-key",
})

// With custom retry config
client := dakera.NewClientWithOptions(dakera.ClientOptions{
    BaseURL:     "http://localhost:3300",
    APIKey:      "your-key",
    RetryConfig: &dakera.RetryConfig{MaxRetries: 5, BaseDelayMs: 200},
})
```

## Documentation

-> [Full docs](https://dakera.ai/docs)  
-> [API reference](https://dakera.ai/docs/api)  
-> [Go SDK reference](https://dakera.ai/docs/sdk/go)

## Related

| Repo | What it is |
|---|---|
| [dakera-py](https://github.com/dakera-ai/dakera-py) | Python SDK |
| [dakera-js](https://github.com/dakera-ai/dakera-js) | TypeScript SDK |
| [dakera-rs](https://github.com/dakera-ai/dakera-rs) | Rust client |
| [dakera-cli](https://github.com/dakera-ai/dakera-cli) | CLI |
| [dakera-mcp](https://github.com/dakera-ai/dakera-mcp) | MCP server |
| [dakera-deploy](https://github.com/dakera-ai/dakera-deploy) | Self-host Dakera |

---

*Part of the Dakera AI open core. The engine is proprietary. The tools are yours.*
