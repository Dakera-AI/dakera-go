# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

## [0.11.3] - 2026-04-18

### Added
- **CE-17 — Explicit `VectorWeight` for Hybrid recall**: `RecallRequest` gains an optional
  `VectorWeight *float32` field (JSON: `vector_weight`, 0.0–1.0). When set, overrides the
  server's adaptive vector/BM25 heuristic for `Routing = RoutingModeHybrid` calls. Omitting
  the field (`omitempty`) preserves existing adaptive behaviour — zero breaking changes.
  (server: [#173](https://github.com/Dakera-AI/dakera/pull/173))

## [0.11.2] - 2026-04-16

### Changed
- **v0.11.2:** Server default fusion strategy changed from `FusionStrategyRRF` to
  `FusionStrategyMinMax` (CEO architecture decision, DAK-1948). MinMax +6.3pp overall
  Recall@10, +13.5pp temporal. Callers that pass `Fusion: nil` will now use MinMax on the
  server. Pass `&FusionStrategyRRF` explicitly to keep RRF behaviour. Updated godoc comments
  to reflect the new server default.

## [0.11.1] - 2026-04-16

### Fixed
- No code changes in this release. Version bump for parity with `dakera-rs` v0.11.1, which
  fixed a serialization bug where `FusionStrategy::MinMax` was sent as `"min_max"` instead of
  `"minmax"`. Go serialized `FusionStrategyMinMax` correctly as `"minmax"` in v0.11.0 (typed
  string constant) — no action required if you are using this SDK.

## [0.11.0] - 2026-04-15

### Added
- **CE-14:** `FusionStrategy` type with `FusionStrategyRRF` and `FusionStrategyMinMax` constants — controls hybrid score fusion.
- **CE-14:** `Fusion *FusionStrategy` field on `RecallRequest`. `nil` uses server default (`FusionStrategyRRF`).
- **v0.11.0:** `Neighborhood *bool` field on `RecallRequest`. Session-adjacent memory enrichment (±5 min). `nil` uses server default (`true`). Set pointer-to-false to disable.


## [0.10.2] - 2026-04-13

### Added
- **CE-13:** `Rerank *bool` field on `RecallRequest` (server default: `true`) and `SearchMemoriesRequest` (server default: `false`). Enables cross-encoder reranking via `Xenova/bge-reranker-base`. Pass pointer-to-false on `RecallRequest` to disable on latency-sensitive paths.
- **CE-13:** `EmbeddingModelBGELarge` constant (`"bge-large"`, 1024 dimensions) — new server-default embedding model.

## [0.10.1] - 2026-04-13

### Fixed
- **`StoreMemoryResponse`:** Corrected struct to match actual server response shape — server returns `{"memory": {...}, "embedding_time_ms": N}` (nested). Field is now `Memory *Memory` (was `MemoryID string`). Access via `resp.Memory.ID`.
- **`ConsolidateResponse`:** Corrected field names — `MemoriesRemoved` (was `ConsolidatedCount`), `SourceMemoryIDs` (was `NewMemories`), `ConsolidatedMemory *Memory` (was `RemovedCount int`).

## [0.10.0] - 2026-04-12

### Added
- **CE-10:** `RoutingMode` string type with constants `RoutingModeAuto`, `RoutingModeVector`, `RoutingModeBM25`, `RoutingModeHybrid` — controls which retrieval index to use for recall and search.
- **CE-10:** `Routing *RoutingMode` field on `RecallRequest` and `SearchMemoriesRequest`. nil uses the server default (`"auto"`).
- **CE-12:** `CompressAgent(ctx, agentID)` method on `Client` — calls `POST /v1/agents/{id}/compress` and returns `*CompressResponse`.
- **CE-12:** `CompressResponse` struct with `AgentID`, `MemoriesBefore`, `MemoriesAfter`, `RemovedCount`, `DurationMs`.
- **CE-10:** `MemoryPolicy.DedupOnStore *bool` — enable similarity deduplication at store time.
- **CE-10:** `MemoryPolicy.DedupThreshold *float32` — cosine-similarity threshold for store-time deduplication.

## [0.9.15] - 2026-04-08

### Notes
- Version bump to match server v0.9.15. No SDK API changes.
- Server changes (transparent to SDK callers):
  - **DAK-1691:** Session-end auto-consolidation — `EndSession` now triggers server-side DBSCAN clustering of near-duplicate session memories, soft-expiring them with a 30-day TTL. High-importance memories (>0.8) are protected. No request/response signature change.
  - **DAK-1689:** HNSW post-filter ANN fix — filtered vector queries are now O(N·ANN) instead of O(N·linear). No SDK change.

## [0.9.14] - 2026-04-07

### Added
- **DAK-1690: Agent wake-up context endpoint:**
  - `Client.GetWakeUpContext(ctx, agentID, opts *WakeUpOptions)` — `GET /v1/agents/{agentID}/wake-up` — returns `*WakeUpResponse` with top-N memories ranked by importance × recency decay. Sub-millisecond; no embedding inference. Requires Read scope.
  - `WakeUpResponse` struct (`AgentID`, `Memories []Memory`, `TotalAvailable int`) and `WakeUpOptions` struct exported from package.

## [0.9.13] - 2026-04-07

### Fixed
- **Session type fix (DAK-1548):** `Session.ID` is now correctly mapped (was `SessionID`). `StartSession()` and `EndSession()` now correctly deserialize wrapped server responses. Added `SessionStartResponse` and `SessionEndResponse` types — `EndSession()` now returns `*SessionEndResponse` exposing `MemoryCount int`.

## [0.9.12] - 2026-04-06

### Added
- **OBS-2: Product KPI Snapshot endpoint:**
  - `Client.GetKpis(ctx)` — `GET /v1/kpis` — returns `*KpiSnapshot` with 8 real-time
    operational metrics. Sub-millisecond; served from in-memory counters. Requires Admin scope.
  - `KpiSnapshot` struct in `types.go`:
    - `RecallLatencyP50Ms` / `RecallLatencyP99Ms` (`float64`) — median/p99 recall latency (ms)
    - `StoreLatencyP50Ms` (`float64`) — median store latency (ms)
    - `ApiErrorRate5xxPct` (`float64`) — 5xx error rate as a percentage of total requests
    - `ActiveAgentsCount` (`uint64`) — distinct agents active in the last 24 hours
    - `SessionCountWeek` (`uint64`) — sessions created in the rolling 7-day window
    - `CrossAgentNetworkNodeCount` (`uint64`) — nodes in the cross-agent knowledge graph
    - `MemoryRetention7dPct` (`float64`) — percentage of memories from 7 days ago still active

### Server-side only (no SDK changes required)
- **v0.9.12 performance fixes:** session-agent index lookup reduced to O(1); memory counters
  now updated via atomic increments; S3 flushes are async (non-blocking).

## [0.9.11] - 2026-04-01

### Added
- **KG-3: Deep Associative Recall bindings:**
  - `RecalledMemory` gains `Depth *int` (`json:"depth,omitempty"`) — the KG hop at which an associated memory was found.
  - `RecallRequest` gains two new pointer fields:
    - `AssociatedMemoriesDepth *int` (`json:"associated_memories_depth,omitempty"`) — KG traversal depth 1–3 (default: `1`).
    - `AssociatedMemoriesMinWeight *float32` (`json:"associated_memories_min_weight,omitempty"`) — minimum KG edge weight (default: `0.0`).
  - Fully backward-compatible: nil fields are omitted from JSON; existing callers retain depth-1 (COG-2) behaviour.
- **COG-3: Proactive Memory Consolidation bindings:**
  - `MemoryPolicy` struct gains four new pointer fields (all `omitempty`):
    - `ConsolidationEnabled *bool` — opt-in background DBSCAN deduplication (server default: `false`).
    - `ConsolidationThreshold *float64` — cosine-similarity epsilon (server default: `0.92`).
    - `ConsolidationIntervalHours *uint32` — background job interval in hours (server default: `24`).
    - `ConsolidatedCount *uint64` — **read-only** lifetime merge count (server-managed).
- **SEC-5: Per-namespace rate limiting bindings:**
  - `MemoryPolicy` struct gains three new pointer fields (all `omitempty`):
    - `RateLimitEnabled *bool` (`json:"rate_limit_enabled,omitempty"`) — opt-in per-namespace rate limiting (server default: `false`).
    - `RateLimitStoresPerMinute *uint32` (`json:"rate_limit_stores_per_minute,omitempty"`) — max store ops/min; `nil` = unlimited (server default).
    - `RateLimitRecallsPerMinute *uint32` (`json:"rate_limit_recalls_per_minute,omitempty"`) — max recall ops/min; `nil` = unlimited (server default).
  - When a limit is exceeded the server returns HTTP 429; the existing `RateLimitError` is returned with `RetryAfter: 60`.

## [0.9.9] - 2026-03-31

### Added
- **CE-7: Time-Window Recall bindings:**
  - `RecallRequest` gains `Since *string` and `Until *string` fields
    (ISO-8601 timestamps, `json:"since,omitempty"` / `"until,omitempty"`).
  - Filters are applied server-side before semantic ranking — only memories
    created within the specified window are considered.
  - Invalid ISO-8601 values return a `400` error from the server.

## [0.9.8] - 2026-03-31

### Added
- **COG-2: Associative Recall bindings:**
  - `RecallRequest` gains `IncludeAssociated bool` and
    `AssociatedMemoriesCap *int` fields.
  - `Recall()` now returns `*RecallResponse` instead of `[]RecalledMemory`.
    `RecallResponse` has `Memories []RecalledMemory` and
    `AssociatedMemories []RecalledMemory` (populated when `IncludeAssociated`
    is set).
- **COG-1: Cognitive Memory Lifecycle bindings:**
  - `GetMemoryPolicy(ctx, namespace)` — retrieve the memory lifecycle policy
    (`GET /v1/namespaces/{namespace}/memory_policy`). Returns `*MemoryPolicy`.
  - `SetMemoryPolicy(ctx, namespace, policy)` — set the lifecycle policy
    (`PUT /v1/namespaces/{namespace}/memory_policy`).
  - New type: `MemoryPolicy` — pointer-type fields for type-specific TTLs
    (`WorkingTTLSeconds`, `EpisodicTTLSeconds`, `SemanticTTLSeconds`,
    `ProceduralTTLSeconds`), per-type decay curves (`WorkingDecay`,
    `EpisodicDecay`, `SemanticDecay`, `ProceduralDecay` — one of
    `"exponential"`, `"linear"`, `"step"`, `"power_law"`, `"logarithmic"`,
    `"flat"`), and spaced repetition (`SpacedRepetitionFactor`,
    `SpacedRepetitionBaseIntervalSeconds`).

## [0.9.7] - 2026-03-31

### Added
- **KG-2: Graph Query & Export bindings:**
  - `Client.KnowledgeQuery(ctx, agentID, rootID, edgeType, minWeight, maxDepth, limit)`
    — filter-based DSL query over the memory knowledge graph
    (`GET /v1/knowledge/query`). Returns `*KgQueryResponse, error`.
  - `Client.KnowledgePath(ctx, agentID, fromID, toID)` — BFS shortest path between
    two memory IDs (`GET /v1/knowledge/path`). Returns `*KgPathResponse, error`.
  - `Client.KnowledgeExport(ctx, agentID, format)` — export the full graph as JSON
    or GraphML (`GET /v1/knowledge/export`). Returns `*KgExportResponse, error`.
  - New types: `KgQueryResponse`, `KgPathResponse`, `KgExportResponse`.

## [0.9.6] - 2026-03-30

### Added
- **GLiNER Entity Extraction via ODE sidecar (ODE-2):**
  - `Client.OdeExtractEntities(ctx, ExtractEntitiesRequest)` — extract named
    entities from text using the dakera-ode GLiNER sidecar (`POST /ode/extract`).
    Returns `*ExtractEntitiesResponse` with per-entity character offsets,
    confidence scores, model variant, and processing time in ms.
  - New `OdeURL` field in `ClientOptions`.
  - New types: `OdeEntity`, `ExtractEntitiesRequest`, `ExtractEntitiesResponse`.

## [0.9.5] - 2026-03-30

### Added
- **AES-256-GCM Encryption Key Rotation (SEC-3):**
  - `Client.RotateEncryptionKey(ctx, newKey, namespace)` — re-encrypt all
    memory content blobs with a new key (`POST /v1/admin/encryption/rotate-key`).
    Pass `namespace=""` to rotate all namespaces. Returns
    `*RotateEncryptionKeyResponse`. Requires Admin scope.
  - New types: `RotateEncryptionKeyRequest`, `RotateEncryptionKeyResponse`.

## [0.9.4] - 2026-03-30

### Added
- **Memory Import/Export (DX-1):**
  - `Client.ImportMemories(ctx, data, format, agentID, namespace)` — import
    memories from Mem0, Zep, JSONL, or CSV (`POST /v1/import`). Returns
    `*MemoryImportResponse`.
  - `Client.ExportMemories(ctx, format, agentID, namespace, limit)` — export
    memories in a portable format (`GET /v1/export`). Returns `*MemoryExportResponse`.
  - New types: `MemoryImportResponse`, `MemoryExportResponse`.
- **Business-Event Audit Log (OBS-1):**
  - `Client.ListAuditEvents(ctx, query)` — paginated audit log query
    (`GET /v1/audit`). Returns `*AuditListResponse`.
  - `Client.StreamAuditEvents(ctx, agentID, eventType)` — live SSE stream of
    audit events (`GET /v1/audit/stream`). Returns `<-chan EventResult`.
  - `Client.ExportAudit(ctx, format, agentID, eventType, fromTs, toTs)` — bulk
    export audit entries (`POST /v1/audit/export`). Returns `*AuditExportResponse`.
  - New types: `AuditEvent`, `AuditListResponse`, `AuditExportResponse`, `AuditQuery`.
- **DBSCAN Adaptive Consolidation (CE-6):** `ConsolidateRequest` now has an
  optional `Config *ConsolidationConfig` field for algorithm selection
  (`"dbscan"` or `"greedy"`) and DBSCAN parameter tuning. `ConsolidateResponse`
  includes an optional `Log []ConsolidationLogEntry`.
  New types: `ConsolidationConfig`, `ConsolidationLogEntry`.
- **External Extraction Providers (EXT-1):**
  - `Client.ExtractText(ctx, text, namespace, provider, model)` — extract
    entities from text (`POST /v1/extract`). Providers: `gliner` (bundled),
    `openai`, `anthropic`, `openrouter`, `ollama`. Returns `*ExtractionResult`.
  - `Client.ListExtractProviders(ctx)` — list available providers
    (`GET /v1/extract/providers`). Returns `[]ExtractionProviderInfo`.
  - `Client.ConfigureNamespaceExtractor(ctx, namespace, provider, model)` — set
    namespace default extractor (`PATCH /v1/namespaces/{ns}/extractor`).
  - New types: `ExtractionResult`, `ExtractionProviderInfo`.
- **Redis Health (OPS-3):** `ClusterStatus` gains `RedisHealthy *bool`
  (`json:"redis_healthy,omitempty"`).
- **Cluster Env Aliases (DIST-1):** Documented `DAKERA_CLUSTER_NODE_ID`,
  `SEED_NODES`, `BIND_ADDR` server environment variables.
- **Memory Encryption (SEC-3):** Server supports AES-256-GCM at-rest encryption
  via `DAKERA_ENCRYPTION_KEY` — transparent to SDK clients.

## [0.9.3] - 2026-03-29

### Added
- **Prometheus Metrics (INFRA-3):** `Client.OpsMetrics(ctx)` — returns the raw
  Prometheus text exposition format string from `GET /v1/ops/metrics` (Admin scope).

## [0.9.2] - 2026-03-27

### Added
- **Namespace-scoped API Keys (SEC-1):**
  - `Client.CreateNamespaceKey(ctx, namespace, name, expiresInDays)` — create a
    scoped API key (`POST /v1/namespaces/{ns}/keys`). Returns
    `*CreateNamespaceKeyResponse`. The raw key is shown **only once**.
  - `Client.ListNamespaceKeys(ctx, namespace)` — list all API keys for a namespace
    (`GET /v1/namespaces/{ns}/keys`). Returns `*ListNamespaceKeysResponse`.
  - `Client.DeleteNamespaceKey(ctx, namespace, keyID)` — revoke a namespace API
    key (`DELETE /v1/namespaces/{ns}/keys/{keyID}`). Returns
    `*KeySuccessResponse`.
  - `Client.GetNamespaceKeyUsage(ctx, namespace, keyID)` — usage stats for a key
    (`GET /v1/namespaces/{ns}/keys/{keyID}/usage`). Returns
    `*NamespaceKeyUsageResponse`.
  - New types: `CreateNamespaceKeyRequest`, `CreateNamespaceKeyResponse`,
    `NamespaceKeyInfo`, `ListNamespaceKeysResponse`, `NamespaceKeyUsageResponse`,
    `KeySuccessResponse` in `types.go`.

## [0.9.1] - 2026-03-26

### Added
- **Memory Feedback Loop (INT-1):**
  - `Client.FeedbackMemory(ctx, memoryID, agentID, signal, note)` — submit feedback
    (upvote/downvote/flag) for a memory (`POST /v1/memories/{id}/feedback`). Returns
    `*FeedbackResponse`.
  - `Client.PatchMemoryImportance(ctx, memoryID, agentID, importance)` — directly set a
    memory's importance score (`PATCH /v1/memories/{id}/importance`). Returns `*FeedbackResponse`.
  - `Client.GetMemoryFeedbackHistory(ctx, memoryID)` — retrieve all feedback events for a
    memory (`GET /v1/memories/{id}/feedback/history`). Returns `*FeedbackHistoryResponse`.
  - `Client.GetAgentFeedbackSummary(ctx, agentID)` — aggregate feedback counts and health score
    for an agent (`GET /v1/agents/{id}/feedback/summary`). Returns `*AgentFeedbackSummary`.
  - `Client.GetFeedbackHealth(ctx, agentID)` — health score (mean importance of non-expired
    memories) for an agent (`GET /v1/feedback/health`). Returns `*FeedbackHealthResponse`.
  - New types: `FeedbackSignal` (string enum: `"upvote"` / `"downvote"` / `"flag"`),
    `FeedbackResponse`, `FeedbackHistoryEntry`, `FeedbackHistoryResponse`, `MemoryFeedbackBody`,
    `MemoryImportancePatch`, `AgentFeedbackSummary`, `FeedbackHealthResponse` in `types.go`.

## [0.9.0] - 2026-03-26

### Added
- **Memory Knowledge Graph API (SDK-9 / CE-5 pre-impl):**
  - `Client.MemoryGraph(ctx, memoryID, depth, types)` — returns the graph of memories
    connected to `memoryID` (`GET /v1/memories/{id}/graph`). Depth and edge-type filters
    are optional.
  - `Client.MemoryPath(ctx, sourceID, targetID)` — shortest path between two memory nodes
    (`GET /v1/memories/{id}/path`).
  - `Client.MemoryLink(ctx, sourceID, targetID, edgeType)` — create a directed edge between
    two memories (`POST /v1/memories/{id}/links`).
  - `Client.AgentGraphExport(ctx, agentID, format)` — export the full memory graph for an
    agent as JSON or CSV (`GET /v1/agents/{id}/graph/export`).
  - New types: `EdgeType`, `GraphEdge`, `GraphNode`, `MemoryGraph`, `GraphPath`,
    `GraphLinkResponse`, `GraphExport` in `types.go`.
  - **Note:** requires server CE-5 for end-to-end functionality; unit tests use mocked
    responses and pass fully against the current server (server CE-5 / DAK-1002).
- **Real-time memory event streaming (SDK-10):**
  - `Client.SubscribeAgentMemories(ctx, agentID, tagFilter, reconnect)` — returns a channel
    of `MemoryEvent` from `GET /v1/events/stream`. Supports tag-based filtering and optional
    auto-reconnect. Skips the `connected` handshake event automatically.

## [0.8.6] - 2026-03-25

### Changed
- `OpsStats` struct — added `State string` field (json: `"state"`, values `"healthy"` or
  `"degraded"`) reflecting storage health. Syncs with core DAK-918 (`/v1/ops/stats` fix).

## [0.8.5] - 2026-03-25

### Added
- `Client.OpsStats(ctx)` — new Read-scoped endpoint `GET /v1/ops/stats` returns `*OpsStats`
  (`Version`, `TotalVectors`, `NamespaceCount`, `UptimeSeconds`, `Timestamp`). Works with
  read-only API keys; use instead of `ClusterStatus()` when Admin scope is unavailable
  (core DAK-852).
- `OpsStats` struct in `types.go`.

> **Note:** v0.8.4 was a Python-only security patch (urllib3 CVE) and was not released for
> this module. This release jumps from v0.8.3 to v0.8.5 to realign all SDKs at the same version.

## [0.8.2] - 2026-03-23

### Added
- `DakeraEvent.Timestamp` — new field populated by the `connected` handshake event
  (`Type == "connected"`). Clients can use this to distinguish connected-and-idle
  from not-yet-connected (core DAK-720).
- `MemoryEvent`: `streamMemorySSE` now parses the SSE `event:` field. For the
  `connected` handshake (`event: connected`), callers receive a `MemoryEvent`
  with `EventType = "connected"` and `AgentID = ""` (core DAK-720).
- `StoreMemoryRequest.ExpiresAt` — optional explicit expiry Unix timestamp (seconds).
  Takes precedence over `TTLSeconds` when both are set (core DECAY-3 / DAK-740).

## [0.8.1] - 2026-03-23

### Fixed
- `Client.HybridSearch()` — corrected endpoint URL from
  `/v1/namespaces/{ns}/fulltext/hybrid` to `/v1/namespaces/{ns}/hybrid` (DAK-679).
  Hybrid search was returning HTTP 404 in production since v0.8.0.

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
