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
  <a href="https://dakera.ai/benchmark"><img alt="LoCoMo 88.2%" src="https://img.shields.io/badge/LoCoMo-88.2%25-22c55e?style=flat-square" /></a>
  <a href="https://dakera.ai/playground"><img alt="Playground" src="https://img.shields.io/badge/playground-try_it-ff6b35?style=flat-square" /></a>
</p>

---

## Why Dakera?

| | Dakera | Others |
|---|---|---|
| **LoCoMo Recall@20** | **88.2%** (1,536 Q, LLM-judged retrieval recall) | not directly comparable |
| **Deployment** | Single binary, Docker one-liner | External vector DB + embedding service required |
| **Embeddings** | Built-in — no OpenAI key needed | Requires external embedding API |
| **Search modes** | Vector · BM25 · Hybrid · Knowledge Graph | Usually one or two |
| **Dependencies** | stdlib `net/http` only | Often pulls in gRPC or third-party HTTP clients |

→ [Try the playground](https://dakera.ai/playground) · [Full benchmark results](https://dakera.ai/benchmark) · [dakera.ai](https://dakera.ai)

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
curl -sSfL https://raw.githubusercontent.com/Dakera-AI/dakera-deploy/main/docker/docker-compose.yml \
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
client := dakera.NewClientWithOptions(dakera.ClientOptions{BaseURL: "http://localhost:3000", APIKey: "dk-mykey"})
imp := float32(0.9)
client.StoreMemory(context.Background(), "my-agent", dakera.StoreMemoryRequest{Content: "User prefers brevity", Importance: &imp})
```

Full example — store, recall, upsert, and hybrid search:

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

## What's new in v0.12.3

SDK v0.12.3 targets **Dakera server v0.12.3** (Dakera-AI/dakera#917, #920, #921) and stays
**compatible with v0.12.0 – v0.12.2 servers**: new request fields are omitted unless you set them and new
response fields are optional (zero on an older server). The neighbourhood modes, `IncludeSummaries`,
`MatchedText` / `MatchedIDs`, `Retrieval`, `ContextOf` and `ContextMemories` need **server >= 0.12.3**
(an older server ignores the new request fields and answers inline). The `super_admin` requirement of
`OpsShutdown` and the `422` on unknown admin-request fields are server 0.12.3 behaviour.

### Recall: at most `TopK` results, context apart

A recall also returns *neighbourhood context*: memories written in the same session within a few
minutes of the top results. By default (server `DAKERA_NEIGHBORHOOD_MODE=inline`) they follow the
ranked results in `Memories`, so `len(resp.Memories)` can exceed `TopK`; each one has
`IsNeighborhood()` true and a `Score` that is not comparable with the ranked results'. Ask for
`NeighborhoodModeSeparate` to keep `Memories` to at most `TopK` ranked results and get the context
in `ContextMemories` (each with `ContextOf`, the id of the ranked memory it belongs to):

```go
resp, err := client.Recall(ctx, "my-agent", dakera.RecallRequest{
    Query:            "where does Alice live?",
    TopK:             3,
    NeighborhoodMode: dakera.NeighborhoodModeSeparate, // or NeighborhoodModeInline / NeighborhoodModeOff
})
for _, m := range resp.Memories { // at most 3
    fmt.Println(m.ID, m.Score, m.MatchedText) // MatchedText / MatchedIDs: see below
}
for _, c := range resp.ContextMemories {
    fmt.Println("context of", c.ContextOf, ":", c.Content)
}
```

`IncludeSummaries` (`*bool`) says whether session summaries may be recalled (unset: the server's
choice). When the server collapses derived sentence sub-memories into their source
(`DAKERA_RECALL_COLLAPSE_DERIVED`), a result is the source memory with `MatchedIDs` (the sentences
that matched, best first) and `MatchedText` (the best sentence, when it outranked the source's own
text); search results carry the same fields (`SearchMemoriesDetailed` also returns `TotalCount`,
`RerankReport` and `EffectiveTopK`). Every new field is omitted from requests unless set and is
optional in answers, so older servers keep working (they ignore `NeighborhoodMode` and answer
inline).

### Document import (server v0.12.3+)

A PDF, DOCX, HTML, Markdown or text document becomes a set of *chunk memories* of an agent:
ordinary memories, each sized for the embedding model, carrying its heading breadcrumb and its
provenance (`_dakera_document`: document id, title, pages, heading path). Recall, search and
filters treat them as memories; a result can cite the document, page and section.

```go
// Text, Markdown or HTML given inline
acc, err := client.ImportDocument(ctx, dakera.ImportDocumentRequest{
    AgentID:   "docs-bot",
    Content:   markdown,
    MediaType: dakera.MediaTypeMarkdown,
    DocumentImportOptions: dakera.DocumentImportOptions{
        Tags:     []string{"manual"},
        Chunking: &dakera.ChunkingOptions{Strategy: dakera.ChunkStrategyStructure},
    },
})
// A PDF / DOCX: upload it as an attachment, then import it
up, _ := client.UploadAttachment(ctx, "library", pdfBytes, dakera.MediaTypePDF)
acc, err = client.ImportAttachmentDocument(ctx, "library", up.AttachmentRef,
    dakera.ImportAttachmentDocumentRequest{AgentID: "docs-bot", DocumentID: "manual-v3"})

// The import is a job
job, err := client.WaitForDocumentJob(ctx, "docs-bot", acc.JobID, dakera.JobWaitOptions{Timeout: 5 * time.Minute})

// Cite where a recalled chunk comes from
for _, m := range resp.Memories {
    if doc := m.DocumentChunk(); doc != nil {
        fmt.Println(doc.Title, doc.HeadingPath, doc.PageStart)
    }
}
```

`ListDocuments` (cursor paging), `GetDocument` (a document and its chunks) and `DeleteDocument`
complete the surface. `Capabilities(ctx)` reports `Documents` (`SupportsDocuments()`, limits,
media types). A server with `DAKERA_DOCUMENTS` off answers `501` (`*FeatureDisabledError`); a
server older than v0.12.3 answers `404` (`*NotFoundError`, whose message says so). A failed job
(`*JobFailedError`) carries the status and code: `413` (too big / too many chunks), `422`
(`UNPROCESSABLE_CONTENT`), `504` (`EXTRACTION_TIMEOUT`).

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
- **T-I-F Reliability** — `TifScore` struct and `EvaluateTif()` for Truth-Indeterminacy-Falsity scoring of memory reliability
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
    BaseURL: "http://<your-server-ip>:3000",
    APIKey:  "your-key",
})

// With custom retry config
client := dakera.NewClientWithOptions(dakera.ClientOptions{
    BaseURL:     "http://localhost:3000",
    APIKey:      "your-key",
    RetryBackoff: &dakera.RetryConfig{MaxRetries: 5, BaseDelay: 200 * time.Millisecond, MaxDelay: 30 * time.Second, Jitter: true},
})
```

Defaults: a 30 s request timeout (`Timeout`, also the connect and response-header timeout unless
`ConnectTimeout` is set) and Go's TLS defaults (TLS 1.2+, system roots, verification on). Event
streams have no overall timeout (cancel their `ctx`).

**Proxies (since v0.12.3)**: the default transport is a clone of `http.DefaultTransport`, so the
client honours the `HTTPS_PROXY`, `HTTP_PROXY` and `NO_PROXY` environment variables like any Go HTTP
client (before v0.12.3 it ignored them). To opt out, set `NO_PROXY` for the Dakera host, or pass your
own transport with `Proxy: nil`:

```go
t := http.DefaultTransport.(*http.Transport).Clone()
t.Proxy = nil // never use a proxy
client := dakera.NewClientWithOptions(dakera.ClientOptions{BaseURL: "http://localhost:3000", Transport: t})
```

Retries: `MaxRetries` is the number of attempts. `429` and `503` (honouring
`Retry-After`) and connection failures before the request was sent are retried for every request;
other `5xx` answers and dropped connections are retried only for requests that cannot apply a write
twice (`GET`, `PUT`, `DELETE`, recall, search and the query routes), never for a write `POST` such
as `StoreMemory`. The API key is never printed: `Client` and `ClientOptions` redact it in `%v`,
`%+v` and `%#v`. `OdeExtractEntities` requests to the ODE sidecar (`OdeURL`) currently carry the
Dakera API key; tracked in [Dakera-AI/dakera#924](https://github.com/Dakera-AI/dakera/issues/924).

`OpsShutdown` (`POST /ops/shutdown`) needs a global **`super_admin`** key; an `admin` key gets a
`403` (`*AuthorizationError`) from servers v0.12.3 and later.

---

## What's new in v0.12.2

SDK v0.12.2 targets **Dakera server v0.12.2** and stays **compatible with v0.12.0 and v0.12.1
servers**: new request fields are omitted unless you set them, new response fields are optional
(`nil` / empty on an older server), and the new routes answer `404` there. Check
`caps.Auth`, `caps.Sessions` (or `SupportsKeyUpdate()`, `SupportsSessionTouch()`) to see what the
connected server has.

| Feature | API |
|---|---|
| Create an agent before its first memory | `CreateAgent(ctx, "mlx-dev")` (`Created` is false for an existing agent) |
| Edit a key's name or namespaces | `UpdateKey`, `UpdateNamespaceKey` with `UpdateKeyRequest{Namespaces: dakera.GrantNamespaces("team-*")}` (`nil` = unchanged, `GrantAllNamespaces()` = `null`, `GrantNamespaces()` = `[]`) |
| Rotate with a grace period | `RotateKeyWithOptions(ctx, id, &dakera.RotateKeyOptions{GraceSecs: &grace})` → `OldKeyID`, `OldKeyExpiresAt` |
| Who am I | `Whoami(ctx)` — scope, grants, `Unrestricted`, `GrantsVersion`, `InertNamespaces` |
| Namespace kinds | `ListNamespacesWithKinds`, `NamespaceInfo.Kind` |
| Session idle lifecycle | `StartSessionRequest.IdleTimeoutSecs`, `TouchSession`, `Session.EndedReason` / `LastActivityAt` / `IdleSince`, `StoreMemoryResponse.SessionState`, `BatchStoreMemoryResponse.EndedSessions`, `SetSessionIdleTimeout` |
| Derived records | `IncludeDerived` on `AgentMemoriesOptions` and `WakeUpOptions`; `AdminDerivationStatus`, `AdminDrainDerivations` |
| Content previews | `ContentPreviewChars` on `AgentMemoriesOptions`, `SessionMemoriesOptions`, `FullKnowledgeGraphRequest`, `CrossAgentNetworkRequest` → `ContentLen`, `ContentTruncated` |

```go
// A developer key that may create and use its own agents (prefix grant).
key, _ := client.CreateKey(ctx, dakera.CreateKeyRequest{
    Name: "dev", Scope: dakera.KeyScopeWrite,
    Namespaces: dakera.GrantNamespaces("_dakera_agent_mlx-*"),
})
_ = key // key.Key is shown once: store it in your secret manager, never in code

// A session the server never ends for inactivity.
zero := 0
sess, _ := client.StartSession(ctx, dakera.StartSessionRequest{AgentID: "mlx-dev", IdleTimeoutSecs: &zero})

// A list view: 200-character previews, full text only when needed.
preview := 200
mems, _ := client.AgentMemories(ctx, "mlx-dev", &dakera.AgentMemoriesOptions{ContentPreviewChars: &preview})
for _, m := range mems {
    if m.ContentTruncated != nil && *m.ContentTruncated {
        full, _ := client.GetMemory(ctx, "mlx-dev", m.ID)
        _ = full
    }
}
_ = sess
```

Behaviour changes in server v0.12.2 that clients can hit:

- **Sessions end after 4 hours without activity** by default (`DAKERA_SESSION_IDLE_TIMEOUT_SECS`).
  Activity is a memory stored, batch-stored, imported or updated with the session, a
  session-scoped recall or search, or `TouchSession`. Storing into an ended session still
  succeeds: check `StoreMemoryResponse.SessionState` / `BatchStoreMemoryResponse.EndedSessions`
  and start a new session when it says `ended`.
- **Sessions are authorized by their agent**: keys no longer need a `_dakera_sessions` grant (it
  is accepted and inert), a key without grants lists no sessions, and `EndSession` with a Read key
  gets `403`.
- **Stricter validation (400, the field named in the message)**: invalid key grants (`a**b`,
  blank entries, internal namespaces), a client-set `dakera-curated` tag, `_dakera_*` metadata
  keys other than `_dakera_content_date` / `_dakera_lang`, ids of the form `mem_s` + 24 hex, agent
  ids over 241 bytes, TTLs over 100 years.
- **The content limit is in bytes** (UTF-8, `DAKERA_MAX_MEMORY_CONTENT_BYTES`, 100000 by default)
  and `UpdateMemory` enforces it too.
- **Listings exclude derived records**: `AgentMemories` and `GetWakeUpContext` leave CE-31
  sentence sub-memories out unless `IncludeDerived` is set.
- **Legacy `foo*` grants stay inert** on keys created before v0.12.2 (`GrantsVersion` 0,
  `InertNamespaces`) until the key's namespaces are saved again with `UpdateKey`.

---

## What's new in v0.12.0

SDK v0.12.0 targets **Dakera server v0.12.0** and is **compatible with both v0.11.108 and
v0.12.0 servers**: every new request field is omitted from the wire unless you set it, and the
new routes only exist on a v0.12 server (see the table below). Operator guide for the server
side: `docs/v0.12/UPGRADE.md` in the server release, and the release notes in the
[Dakera changelog](https://dakera.ai/docs/changelog).

| Feature | API | Needs |
|---|---|---|
| What the server supports | `client.Capabilities(ctx)` — models, index kinds, `rabitq`, records, attachments, vision, scoring, `QueryLanguages` | v0.12 (404 on v0.11) |
| Readiness | `HealthReady`, `IsReady`, `WaitUntilReady` (a `503` is "not ready", never healthy) | any |
| `Retry-After` | honoured on `429` and `503` (capped at `RetryConfig.MaxDelay`); `ServerError.RetryAfter` | any |
| Errors | `PayloadTooLargeError` (`IsQuota()` tells a full namespace from an oversize request), `FeatureDisabledError` / `NotImplementedError` (`501`), `ConflictError` (`409`); `DakeraError.Details` / `.Resource` | any |
| Attachments | `UploadAttachment`, `ListAttachments`, `DownloadAttachment`, `DeleteAttachment`, `StoreMemoryRequest.AttachmentRef` | `DAKERA_ATTACHMENTS` |
| Transcription / image index jobs | `TranscribeAttachment`, `IndexAttachmentImage`, `Get*Job`, `WaitFor*` | `DAKERA_ATTACHMENTS` (+ `DAKERA_VISION`) |
| Records | `UpsertRecords`, `GetRecord` — one vector plus named representations (`dense`, `token_multivector`, `patch_multivector`; `f32`/`f16`/`i8`) | `DAKERA_RECORDS` |
| Per-request language | `Lang` on store, batch store, update, recall, search and `ExtractMemoryEntities` | v0.12 |
| New values | models `bge-m3`, `colbert-small`; index kind `ivfpq`; search mode `rabitq` (server-wide, `DAKERA_SEARCH_MODE`) | v0.12 |
| Clearing entity types | `PutNamespaceEntityConfig` (PUT) and `PatchNamespaceEntityConfig` (merge); `ConfigureNamespaceNer` clears with PUT | v0.12 for PUT |

```go
// Wait for a starting server, then check what it can do.
if _, err := client.WaitUntilReady(ctx, dakera.ReadyWaitOptions{Timeout: 2 * time.Minute}); err != nil {
    log.Fatal(err)
}
caps, _ := client.Capabilities(ctx)
if caps.SupportsAttachments() {
    up, _ := client.UploadAttachment(ctx, dakera.AgentMemoryNamespace("my-agent"), wavBytes, "audio/wav")
    job, _ := client.TranscribeAttachment(ctx, dakera.AgentMemoryNamespace("my-agent"), up.AttachmentRef,
        dakera.TranscribeRequest{AgentID: "my-agent", Lang: "en"})
    _, err = client.WaitForTranscription(ctx, dakera.AgentMemoryNamespace("my-agent"), up.AttachmentRef, job.JobID,
        dakera.JobWaitOptions{Timeout: 5 * time.Minute})
}
```

Things to know when upgrading a server:

- **Health checks**: v0.12 binds its port while models load. `/health` answers `503` with
  `Retry-After` until then; use `/health/ready` (`HealthReady` / `WaitUntilReady`).
- **Clearing `entity_types`**: on a v0.12 server `PATCH /v1/namespaces/{ns}/config` merges and an
  empty list is omitted from JSON, so a PATCH cannot clear it. Use `PutNamespaceEntityConfig`
  (or `ConfigureNamespaceNer` with no entity types, which uses PUT and falls back to PATCH on a
  v0.11 server).
- **Errors are JSON everywhere**; every `503` carries `Retry-After`; over-size bodies are `413`;
  configuration errors are `501`.
- `UpsertResponse` / `DeleteResponse` now decode the server's real `upserted_count` /
  `deleted_count` fields (earlier versions read camelCase names the server never sent and so
  reported `0`).
- The Go SDK does not use gRPC; gRPC clients written against v0.12 must send the API key in call
  metadata (`x-api-key` or `authorization: Bearer`).

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
