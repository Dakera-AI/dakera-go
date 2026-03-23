# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [0.8.0] - 2026-03-23

### Changed
- `Client.HybridSearch()` — `vector []float32` parameter now accepts `nil`. When `nil` the server
  performs BM25-only full-text search. Existing callers that pass a non-nil slice are unaffected.
  (core v0.8.0 / dakera-mcp PR#20)

## [0.7.3] - 2026-03-23

### Added
- `StoreMemoryRequest.ExpiresAt *int64` — optional explicit expiry Unix timestamp (seconds);
  takes precedence over `TTLSeconds`; memory is hard-deleted by the decay engine on expiry (DECAY-3)
- `DecayConfigResponse`, `DecayConfigUpdateRequest`, `DecayConfigUpdateResponse` structs (DECAY-1)
- `LastDecayCycleStats`, `DecayStatsResponse` structs (DECAY-2)
- `Client.DecayConfig()` — `GET /v1/admin/decay/config` — current strategy, half-life,
  and min-importance threshold (DECAY-1). Requires Admin scope.
- `Client.DecayUpdateConfig()` — `PUT /v1/admin/decay/config` — live config update with
  no restart required (DECAY-1). All fields optional.
- `Client.DecayStats()` — `GET /v1/admin/decay/stats` — cumulative counters and
  last-cycle snapshot (DECAY-2). Requires Admin scope.

## [0.7.2] - 2026-03-23

### Added
- `AutoPilotConfig`, `AutoPilotStatusResponse`, `DedupResultSnapshot`, `ConsolidationResultSnapshot`
  structs for AutoPilot status data
- `AutoPilotConfigRequest`, `AutoPilotConfigResponse` structs for runtime configuration updates
- `AutoPilotDedupResult`, `AutoPilotConsolidationResult`, `AutoPilotTriggerResponse` structs
- `Client.AutopilotStatus()` — `GET /v1/admin/autopilot/status` (PILOT-1)
- `Client.AutopilotUpdateConfig()` — `PUT /v1/admin/autopilot/config` (PILOT-2)
- `Client.AutopilotTrigger()` — `POST /v1/admin/autopilot/trigger` (PILOT-3)

## [0.7.1] - 2026-03-22

### Added
- `BatchMemoryFilter` / `BatchRecallRequest` / `BatchRecallResponse` / `BatchForgetRequest` /
  `BatchForgetResponse` — typed structs for batch memory operations
- `Client.BatchRecall()` — `POST /v1/memories/recall/batch` — recall memories for
  multiple agents in a single request
- `Client.BatchForget()` — `DELETE /v1/memories/forget/batch` — forget memories for
  multiple agents in a single request
- `RateLimitHeaders` struct + `Client.LastRateLimitHeaders()` method — exposes
  `X-RateLimit-Limit`, `X-RateLimit-Remaining`, `X-RateLimit-Reset` from the last response

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
