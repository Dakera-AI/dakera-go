# ⚡ dakera-go

Go client for Dakera AI — store, recall, and search agent memories against a Dakera instance.

Part of [Dakera AI](https://dakera.ai) — the memory engine for AI agents.

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

*Part of the Dakera AI open core. The engine is proprietary. The tools are yours.*
