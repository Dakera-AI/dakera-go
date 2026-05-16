# ⚡ dakera-go

[![CI](https://github.com/Dakera-AI/dakera-go/actions/workflows/ci.yml/badge.svg)](https://github.com/Dakera-AI/dakera-go/actions/workflows/ci.yml) [![Go Reference](https://pkg.go.dev/badge/github.com/Dakera-AI/dakera-go.svg)](https://pkg.go.dev/github.com/Dakera-AI/dakera-go) [![License: MIT](https://img.shields.io/github/license/Dakera-AI/dakera-go)](LICENSE)
[![dakera.ai](https://img.shields.io/badge/dakera.ai-website-22c55e?style=flat-square)](https://dakera.ai) [![Docs](https://img.shields.io/badge/docs-dakera.ai%2Fdocs-3b82f6?style=flat-square)](https://dakera.ai/docs)

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
    client := dakera.NewClient(dakera.Config{
        BaseURL: "http://localhost:3300",
        APIKey:  "your-key",
    })

    ctx := context.Background()

    // Store a vector
    client.Vectors.Upsert(ctx, dakera.UpsertRequest{
        ID:     "vec-001",
        Values: []float32{0.1, 0.2, 0.3},
    })

    // Search memories
    results, err := client.Fulltext.Search(ctx, dakera.SearchRequest{
        Query: "completed task",
        TopK:  5,
    })
    if err != nil {
        panic(err)
    }
    for _, r := range results.Results {
        fmt.Println(r.ID, r.Score)
    }
}
```

## Connect to Dakera

```go
// Self-hosted
client := dakera.NewClient(dakera.Config{BaseURL: "http://your-server:3300", APIKey: "your-key"})

// Cloud (early access)
client := dakera.NewClient(dakera.Config{BaseURL: "https://api.dakera.ai", APIKey: "your-key"})
```

## Documentation

→ [Full docs](https://dakera.ai/docs)  
→ [API reference](https://dakera.ai/docs/api)  
→ [Go SDK reference](https://dakera.ai/docs/sdk/go)

## Related

| Repo | What it is |
|---|---|
| [dakera-py](https://github.com/dakera-ai/dakera-py) | Python SDK |
| [dakera-js](https://github.com/dakera-ai/dakera-js) | TypeScript SDK |
| [dakera-rs](https://github.com/dakera-ai/dakera-rs) | Rust client |
| [dakera-cli](https://github.com/dakera-ai/dakera-cli) | CLI |
| [dakera-mcp](https://github.com/dakera-ai/dakera-mcp) | MCP server · 83 tools |
| [dakera-deploy](https://github.com/dakera-ai/dakera-deploy) | Self-host Dakera |

---

**[dakera.ai](https://dakera.ai)** · [Documentation](https://dakera.ai/docs) · [Request Early Access](https://dakera.ai#cta)

<sub>Part of the Dakera AI open-source ecosystem. Built with Rust. Self-hosted. Zero dependencies.</sub>
