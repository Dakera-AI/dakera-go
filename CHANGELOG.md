# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [0.7.0] - 2026-03-22

### Added
- `RetryConfig` struct with `MaxRetries`, `BaseDelay`, `MaxDelay`, and `Jitter` fields for
  fine-grained exponential-backoff control.
- `ClientOptions.RetryBackoff` (`*RetryConfig`) — overrides `MaxRetries` when set.
- `ClientOptions.ConnectTimeout` — sets the TCP dial timeout independently of the overall
  request timeout via `net/http.Transport` + `net.Dialer`.
- HTTP 429 responses now respect the `Retry-After` header: if `Retry-After: N` is present the
  client waits exactly N seconds before retrying (including `Retry-After: 0` for instant retry).
  Falls back to exponential backoff when the header is absent.
- 5xx responses are retried up to `MaxRetries` times; 4xx responses (except 429) are never
  retried.

## [0.6.2] - 2026-03-21

### Added
- `CrossAgentNetworkResponse.NodeCount` (`int`, `json:"node_count"`) — reflects the `node_count`
  field added in dakera server v0.6.2 (PR #26). Defaults to `0` when absent so responses from
  older server versions remain valid.
- SSE endpoints now support `?api_key=<key>` query-parameter authentication in addition to
  the `Authorization: Bearer` header. Useful when constructing streaming URLs for clients that
  cannot send custom headers (e.g. browser-native `EventSource`).

## [0.5.0] - 2026-03-20

### Added
- `CrossAgentNetwork(ctx, req)` — POST /v1/knowledge/network/cross-agent, builds the cross-agent memory similarity graph (DASH-A / Admin scope)
- `CrossAgentNetworkRequest`, `CrossAgentNetworkResponse`, `AgentNetworkInfo`, `AgentNetworkNode`, `AgentNetworkEdge`, `AgentNetworkStats` types in `types.go`
- `StreamMemoryEvents(ctx)` — GET /v1/events/stream SSE subscription for memory lifecycle events (DASH-B / Read scope)
- `MemoryEvent`, `MemoryEventResult` types and `streamMemorySSE` helper in `events.go`

## [0.4.0] - 2026-03-19

### Added
- `StreamNamespaceEvents(ctx, namespace)` — SSE streaming for namespace-scoped events (CE-1 / SDK-4)
- `StreamGlobalEvents(ctx)` — SSE streaming for admin-scoped global events (CE-1 / SDK-4)
- `DakeraEvent`, `OpStatus`, `VectorMutationOp`, `EventResult` types in `types.go`
- `events.go` — SSE client implementation using `bufio.Scanner` over HTTP streaming

## [0.3.0] - 2026-03-19

### Added
- `examples/memory/main.go` — demonstrates memory storage, recall, session management, and Summarize
- `examples/advanced/main.go` — demonstrates UpsertText, QueryText, HybridSearch, WarmCache, AdminIndexStats, RebuildIndexes, and AnalyticsOverview
- All examples use context-first patterns throughout

### Changed
- README quickstart refresh — updated tagline, added agent memory feature section, removed cortex references (DAK-102)

## [0.2.0] - 2026-03-19

### Changed
- CI: add explicit `GITHUB_TOKEN` permissions (`contents: read`) to workflow for hardened security

## [0.1.0] - 2025-03-15

### Added
- Initial release of Dakera Go SDK
- Client with full API coverage
- Vector operations: upsert, query, fetch, delete
- Namespace management
- Full-text search support
- Agent memory operations
- Session management
- Knowledge graph operations
- Inference (auto-embedding) support
- Typed structs for all API requests and responses
- Custom error types
- Example application
- Race condition safe (tested with -race)
