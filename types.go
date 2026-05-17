// Package dakera provides a Go client for Dakera AI memory platform.
package dakera

import "time"

// Vector represents a stored vector with its metadata.
type Vector struct {
	ID       string                 `json:"id"`
	Values   []float32              `json:"values,omitempty"`
	Metadata map[string]interface{} `json:"metadata,omitempty"`
}

// VectorInput represents input for upserting a vector.
type VectorInput struct {
	ID       string                 `json:"id"`
	Values   []float32              `json:"values"`
	Metadata map[string]interface{} `json:"metadata,omitempty"`
}

// QueryResult represents a single query match.
type QueryResult struct {
	ID       string                 `json:"id"`
	Score    float32                `json:"score"`
	Values   []float32              `json:"values,omitempty"`
	Metadata map[string]interface{} `json:"metadata,omitempty"`
}

// SearchResult represents the result of a vector query.
type SearchResult struct {
	Results       []QueryResult `json:"results"`
	TotalSearched int           `json:"totalSearched,omitempty"`
}

// NamespaceInfo represents information about a namespace.
type NamespaceInfo struct {
	Name        string                 `json:"namespace"`
	VectorCount int64                  `json:"vector_count"`
	Dimension   int                    `json:"dimension,omitempty"`
	Distance    string                 `json:"distance,omitempty"`
	IndexType   string                 `json:"indexType,omitempty"`
	Metadata    map[string]interface{} `json:"metadata,omitempty"`
	CreatedAt   *time.Time             `json:"createdAt,omitempty"`
	UpdatedAt   *time.Time             `json:"updatedAt,omitempty"`
}

// IndexStats represents statistics about an index.
type IndexStats struct {
	Namespace    string  `json:"namespace"`
	VectorCount  int64   `json:"vectorCount"`
	IndexedCount int64   `json:"indexedCount"`
	Dimensions   int     `json:"dimensions"`
	IndexType    string  `json:"indexType"`
	SizeBytes    int64   `json:"sizeBytes,omitempty"`
	Utilization  float64 `json:"utilization,omitempty"`
}

// Document represents a document for full-text indexing.
type Document struct {
	ID       string                 `json:"id"`
	Text     string                 `json:"text"`
	Metadata map[string]interface{} `json:"metadata,omitempty"`
}

// DocumentInput represents input for indexing a document.
type DocumentInput struct {
	ID       string                 `json:"id"`
	Text     string                 `json:"text"`
	Metadata map[string]interface{} `json:"metadata,omitempty"`
}

// FullTextSearchResult represents a full-text search result.
type FullTextSearchResult struct {
	ID       string                 `json:"id"`
	Score    float32                `json:"score"`
	Text     string                 `json:"text,omitempty"`
	Metadata map[string]interface{} `json:"metadata,omitempty"`
}

// HybridSearchResult represents a hybrid search result.
type HybridSearchResult struct {
	ID          string                 `json:"id"`
	Score       float32                `json:"score"`
	VectorScore float32                `json:"vectorScore,omitempty"`
	TextScore   float32                `json:"textScore,omitempty"`
	Values      []float32              `json:"values,omitempty"`
	Content     string                 `json:"content,omitempty"`
	Metadata    map[string]interface{} `json:"metadata,omitempty"`
}

// HealthResponse represents the server health check response.
type HealthResponse struct {
	Status  string `json:"status"`
	Version string `json:"version,omitempty"`
}

// UpsertResponse represents the response from an upsert operation.
type UpsertResponse struct {
	UpsertedCount int `json:"upsertedCount"`
}

// DeleteResponse represents the response from a delete operation.
type DeleteResponse struct {
	DeletedCount int `json:"deletedCount"`
}

// IndexDocumentsResponse represents the response from indexing documents.
type IndexDocumentsResponse struct {
	IndexedCount int `json:"indexedCount"`
}

// StatusResponse represents a generic status response.
type StatusResponse struct {
	Status string `json:"status"`
}

// QueryOptions represents options for vector queries.
type QueryOptions struct {
	TopK            int                    `json:"top_k,omitempty"`
	Filter          map[string]interface{} `json:"filter,omitempty"`
	IncludeValues   bool                   `json:"include_values,omitempty"`
	IncludeMetadata bool                   `json:"include_metadata,omitempty"`
}

// DeleteOptions represents options for delete operations.
type DeleteOptions struct {
	IDs       []string               `json:"ids,omitempty"`
	Filter    map[string]interface{} `json:"filter,omitempty"`
	DeleteAll bool                   `json:"delete_all,omitempty"`
}

// FetchOptions represents options for fetch operations.
type FetchOptions struct {
	IncludeValues   bool `json:"include_values,omitempty"`
	IncludeMetadata bool `json:"include_metadata,omitempty"`
}

// BatchQuerySpec represents a single query in a batch query request.
type BatchQuerySpec struct {
	Vector          []float32              `json:"vector"`
	TopK            int                    `json:"top_k,omitempty"`
	Filter          map[string]interface{} `json:"filter,omitempty"`
	IncludeValues   bool                   `json:"include_values,omitempty"`
	IncludeMetadata bool                   `json:"include_metadata,omitempty"`
}

// FullTextSearchOptions represents options for full-text search.
type FullTextSearchOptions struct {
	TopK   int                    `json:"top_k,omitempty"`
	Filter map[string]interface{} `json:"filter,omitempty"`
}

// HybridSearchOptions represents options for hybrid search.
type HybridSearchOptions struct {
	TopK         int                    `json:"top_k,omitempty"`
	VectorWeight float32                `json:"vector_weight,omitempty"`
	Filter       map[string]interface{} `json:"filter,omitempty"`
}

// CreateNamespaceOptions represents options for creating a namespace.
type CreateNamespaceOptions struct {
	Dimensions int                    `json:"dimensions,omitempty"`
	IndexType  string                 `json:"index_type,omitempty"`
	Metadata   map[string]interface{} `json:"metadata,omitempty"`
}

// DistanceMetric represents the distance metric for similarity search.
// Valid values: "cosine", "euclidean", "dot_product".
type DistanceMetric string

const (
	DistanceMetricCosine     DistanceMetric = "cosine"
	DistanceMetricEuclidean  DistanceMetric = "euclidean"
	DistanceMetricDotProduct DistanceMetric = "dot_product"
)

// ConfigureNamespaceRequest is the request body for PUT /v1/namespaces/:namespace.
//
// Uses upsert semantics: creates the namespace if it does not exist, or
// updates its configuration if it already exists (v0.6.0).
type ConfigureNamespaceRequest struct {
	// Dimension is the vector dimension. Required on first creation;
	// must match the existing dimension on subsequent calls.
	Dimension int `json:"dimension"`
	// Distance is the distance metric. Defaults to cosine when omitted.
	Distance DistanceMetric `json:"distance,omitempty"`
}

// ConfigureNamespaceResponse is the response from PUT /v1/namespaces/:namespace.
type ConfigureNamespaceResponse struct {
	// Namespace is the namespace name.
	Namespace string `json:"namespace"`
	// Dimension is the vector dimension.
	Dimension int `json:"dimension"`
	// Distance is the distance metric in use.
	Distance DistanceMetric `json:"distance"`
	// Created is true if the namespace was newly created; false if it already existed.
	Created bool `json:"created"`
}

// RetryConfig holds exponential-backoff retry parameters.
type RetryConfig struct {
	// MaxRetries is the maximum number of attempts (including the initial one).
	// Defaults to 3.
	MaxRetries int

	// BaseDelay is the initial backoff duration. Defaults to 100ms.
	BaseDelay time.Duration

	// MaxDelay is the upper bound on backoff duration. Defaults to 60s.
	MaxDelay time.Duration

	// Jitter, when true, randomises the delay ±50%. Defaults to true.
	Jitter bool
}

// DefaultRetryConfig returns a RetryConfig with sensible defaults.
func DefaultRetryConfig() RetryConfig {
	return RetryConfig{
		MaxRetries: 3,
		BaseDelay:  100 * time.Millisecond,
		MaxDelay:   60 * time.Second,
		Jitter:     true,
	}
}

// ClientOptions represents options for the Dakera client.
type ClientOptions struct {
	// BaseURL is the Dakera server URL.
	BaseURL string

	// APIKey is the optional API key for authentication.
	APIKey string

	// Timeout is the request timeout duration. Defaults to 30s.
	Timeout time.Duration

	// ConnectTimeout is the TCP connection establishment timeout.
	// Defaults to Timeout when not set.
	ConnectTimeout time.Duration

	// MaxRetries is the maximum number of retries. Deprecated: use RetryBackoff.
	// When both are set, RetryBackoff takes precedence.
	MaxRetries int

	// RetryBackoff allows fine-grained retry configuration.
	// When set, MaxRetries is ignored.
	RetryBackoff *RetryConfig

	// Headers are additional HTTP headers to include in requests.
	Headers map[string]string

	// OdeURL is the base URL of the dakera-ode sidecar (e.g. "http://localhost:8080").
	// Required to call Client.ExtractEntities.
	OdeURL string
}

// Filter operators for metadata filtering.
const (
	OpEq  = "$eq"
	OpNe  = "$ne"
	OpGt  = "$gt"
	OpGte = "$gte"
	OpLt  = "$lt"
	OpLte = "$lte"
	OpIn  = "$in"
	OpNin = "$nin"
	OpAnd = "$and"
	OpOr  = "$or"
	// Existence check
	OpExists = "$exists"
	// String operators
	OpContains    = "$contains"
	OpIContains   = "$icontains"
	OpStartsWith  = "$startsWith"
	OpEndsWith    = "$endsWith"
	OpGlob        = "$glob"
	OpRegex       = "$regex"
	// Array operators (CE-79)
	OpArrayContains    = "$arrayContains"
	OpArrayContainsAll = "$arrayContainsAll"
	OpArrayContainsAny = "$arrayContainsAny"
)

// Eq creates an equality filter.
func Eq(value interface{}) map[string]interface{} {
	return map[string]interface{}{OpEq: value}
}

// Ne creates a not-equal filter.
func Ne(value interface{}) map[string]interface{} {
	return map[string]interface{}{OpNe: value}
}

// Gt creates a greater-than filter.
func Gt(value interface{}) map[string]interface{} {
	return map[string]interface{}{OpGt: value}
}

// Gte creates a greater-than-or-equal filter.
func Gte(value interface{}) map[string]interface{} {
	return map[string]interface{}{OpGte: value}
}

// Lt creates a less-than filter.
func Lt(value interface{}) map[string]interface{} {
	return map[string]interface{}{OpLt: value}
}

// Lte creates a less-than-or-equal filter.
func Lte(value interface{}) map[string]interface{} {
	return map[string]interface{}{OpLte: value}
}

// In creates an "in list" filter.
func In(values ...interface{}) map[string]interface{} {
	return map[string]interface{}{OpIn: values}
}

// Nin creates a "not in list" filter.
func Nin(values ...interface{}) map[string]interface{} {
	return map[string]interface{}{OpNin: values}
}

// And creates a logical AND filter.
func And(conditions ...map[string]interface{}) map[string]interface{} {
	return map[string]interface{}{OpAnd: conditions}
}

// Or creates a logical OR filter.
func Or(conditions ...map[string]interface{}) map[string]interface{} {
	return map[string]interface{}{OpOr: conditions}
}

// Exists creates a field-existence filter.
func Exists(present bool) map[string]interface{} {
	return map[string]interface{}{OpExists: present}
}

// Contains creates a case-sensitive substring filter.
func Contains(substr string) map[string]interface{} {
	return map[string]interface{}{OpContains: substr}
}

// IContains creates a case-insensitive substring filter.
func IContains(substr string) map[string]interface{} {
	return map[string]interface{}{OpIContains: substr}
}

// StartsWith creates a prefix filter.
func StartsWith(prefix string) map[string]interface{} {
	return map[string]interface{}{OpStartsWith: prefix}
}

// EndsWith creates a suffix filter.
func EndsWith(suffix string) map[string]interface{} {
	return map[string]interface{}{OpEndsWith: suffix}
}

// Glob creates a glob-pattern filter (supports * and ? wildcards).
func Glob(pattern string) map[string]interface{} {
	return map[string]interface{}{OpGlob: pattern}
}

// Regex creates a regular-expression filter.
func Regex(pattern string) map[string]interface{} {
	return map[string]interface{}{OpRegex: pattern}
}

// ArrayContains creates a filter that matches when an array metadata field contains value.
func ArrayContains(value interface{}) map[string]interface{} {
	return map[string]interface{}{OpArrayContains: value}
}

// ArrayContainsAll creates a filter that matches when an array field contains all values.
func ArrayContainsAll(values ...interface{}) map[string]interface{} {
	return map[string]interface{}{OpArrayContainsAll: values}
}

// ArrayContainsAny creates a filter that matches when an array field contains any of values.
func ArrayContainsAny(values ...interface{}) map[string]interface{} {
	return map[string]interface{}{OpArrayContainsAny: values}
}

// ===========================================================================
// Text-Based Inference Types (Auto-Embedding)
// ===========================================================================

// EmbeddingModel represents supported embedding models for text-based operations.
type EmbeddingModel string

const (
	// EmbeddingModelBGELarge is the BGE-large model - Best quality, server default (1024 dimensions).
	EmbeddingModelBGELarge EmbeddingModel = "bge-large"
	// EmbeddingModelMiniLM is the MiniLM-L6 model - Fast, good quality (384 dimensions).
	EmbeddingModelMiniLM EmbeddingModel = "minilm"
	// EmbeddingModelBGESmall is the BGE-small model - Balanced performance (384 dimensions).
	EmbeddingModelBGESmall EmbeddingModel = "bge-small"
	// EmbeddingModelE5Small is the E5-small model - High quality (384 dimensions).
	EmbeddingModelE5Small EmbeddingModel = "e5-small"
)

// TextDocument represents input for upserting a text document with automatic embedding.
type TextDocument struct {
	ID         string                 `json:"id"`
	Text       string                 `json:"text"`
	Metadata   map[string]interface{} `json:"metadata,omitempty"`
	TTLSeconds *int                   `json:"ttl_seconds,omitempty"`
}

// TextSearchResult represents a single text search result.
type TextSearchResult struct {
	ID       string                 `json:"id"`
	Score    float32                `json:"score"`
	Text     string                 `json:"text,omitempty"`
	Metadata map[string]interface{} `json:"metadata,omitempty"`
	Vector   []float32              `json:"vector,omitempty"`
}

// TextUpsertResponse represents the response from a text upsert operation.
type TextUpsertResponse struct {
	UpsertedCount   int            `json:"upserted_count"`
	TokensProcessed int            `json:"tokens_processed"`
	Model           EmbeddingModel `json:"model"`
	EmbeddingTimeMs int64          `json:"embedding_time_ms"`
}

// TextQueryResponse represents the response from a text query operation.
type TextQueryResponse struct {
	Results         []TextSearchResult `json:"results"`
	Model           EmbeddingModel     `json:"model"`
	EmbeddingTimeMs int64              `json:"embedding_time_ms"`
	SearchTimeMs    int64              `json:"search_time_ms"`
}

// BatchTextQueryResponse represents the response from a batch text query operation.
type BatchTextQueryResponse struct {
	Results         [][]TextSearchResult `json:"results"`
	Model           EmbeddingModel       `json:"model"`
	EmbeddingTimeMs int64                `json:"embedding_time_ms"`
	SearchTimeMs    int64                `json:"search_time_ms"`
}

// TextUpsertOptions represents options for text upsert operations.
type TextUpsertOptions struct {
	Model EmbeddingModel `json:"model,omitempty"`
}

// TextQueryOptions represents options for text query operations.
type TextQueryOptions struct {
	TopK           int                    `json:"top_k,omitempty"`
	Filter         map[string]interface{} `json:"filter,omitempty"`
	IncludeText    bool                   `json:"include_text,omitempty"`
	IncludeVectors bool                   `json:"include_vectors,omitempty"`
	Model          EmbeddingModel         `json:"model,omitempty"`
}

// BatchTextQueryOptions represents options for batch text query operations.
type BatchTextQueryOptions struct {
	TopK           int                    `json:"top_k,omitempty"`
	Filter         map[string]interface{} `json:"filter,omitempty"`
	IncludeVectors bool                   `json:"include_vectors,omitempty"`
	Model          EmbeddingModel         `json:"model,omitempty"`
}

// ===========================================================================
// Memory Types
// ===========================================================================

// StoreMemoryRequest represents a request to store a memory.
type StoreMemoryRequest struct {
	AgentID    string                 `json:"agent_id,omitempty"`
	Content    string                 `json:"content"`
	MemoryType string                 `json:"memory_type,omitempty"`
	Importance *float32               `json:"importance,omitempty"`
	Metadata   map[string]interface{} `json:"metadata,omitempty"`
	// TTLSeconds is an optional TTL in seconds. The memory is hard-deleted after
	// this many seconds from creation.
	TTLSeconds *int `json:"ttl_seconds,omitempty"`
	// ExpiresAt is an optional explicit expiry Unix timestamp (seconds). Takes
	// precedence over TTLSeconds when both are set. The memory is hard-deleted
	// by the decay engine on expiry (DECAY-3).
	ExpiresAt *int64  `json:"expires_at,omitempty"`
	SessionID string  `json:"session_id,omitempty"`
	Embedding []float32 `json:"embedding,omitempty"`
}

// StoreMemoryResponse represents the response from storing a memory.
//
// The server wraps the created memory in a nested "memory" object:
//
//	{"memory": {"id": "...", "agent_id": "...", ...}, "embedding_time_ms": N}
type StoreMemoryResponse struct {
	Memory          *Memory `json:"memory"`
	EmbeddingTimeMs *int64  `json:"embedding_time_ms,omitempty"`
}

// Memory represents a stored memory.
type Memory struct {
	ID             string                 `json:"id"`
	Content        string                 `json:"content"`
	AgentID        string                 `json:"agent_id,omitempty"`
	MemoryType     string                 `json:"memory_type"`
	Importance     float32                `json:"importance"`
	Tags           []string               `json:"tags,omitempty"`
	Metadata       map[string]interface{} `json:"metadata,omitempty"`
	CreatedAt      int64                  `json:"created_at,omitempty"`
	LastAccessedAt int64                  `json:"last_accessed_at,omitempty"`
	AccessCount    *int                   `json:"access_count,omitempty"`
}

// RecalledMemory represents a recalled memory with similarity score.
type RecalledMemory struct {
	ID         string                 `json:"id"`
	Content    string                 `json:"content"`
	MemoryType string                 `json:"memory_type"`
	Importance float32                `json:"importance"`
	Score      float32                `json:"score"`
	Metadata   map[string]interface{} `json:"metadata,omitempty"`
	CreatedAt  int64                  `json:"created_at,omitempty"`
	// KG-3: hop depth at which this memory was found (only set on associated memories)
	Depth *int `json:"depth,omitempty"`
}

// RoutingMode controls which retrieval index the server uses for recall and
// search (CE-10). RoutingModeAuto (default) lets the server pick the best
// strategy based on the query.
type RoutingMode string

const (
	// RoutingModeAuto lets the server pick the best retrieval strategy (default).
	RoutingModeAuto RoutingMode = "auto"
	// RoutingModeVector forces ANN vector search (HNSW).
	RoutingModeVector RoutingMode = "vector"
	// RoutingModeBM25 forces BM25 full-text search.
	RoutingModeBM25 RoutingMode = "bm25"
	// RoutingModeHybrid fuses ANN and BM25 scores (RRF).
	RoutingModeHybrid RoutingMode = "hybrid"
)

// FusionStrategy controls how vector and BM25 scores are combined during
// hybrid recall (CE-14). FusionStrategyMinMax (server default since v0.11.2)
// uses weighted min-max normalization.
type FusionStrategy string

const (
	// FusionStrategyRRF uses Reciprocal Rank Fusion (Cormack et al., SIGIR 2009).
	FusionStrategyRRF FusionStrategy = "rrf"
	// FusionStrategyMinMax uses weighted min-max normalization — server default since v0.11.2.
	FusionStrategyMinMax FusionStrategy = "minmax"
)

// RecallRequest represents a request to recall memories.
type RecallRequest struct {
	AgentID       string   `json:"agent_id,omitempty"`
	Query         string   `json:"query"`
	TopK          int      `json:"top_k,omitempty"`
	MemoryType    string   `json:"memory_type,omitempty"`
	MinImportance *float32 `json:"min_importance,omitempty"`
	// COG-2: traverse KG from recalled memories and include
	// associatively linked memories in the response (default: false)
	IncludeAssociated bool `json:"include_associated,omitempty"`
	// COG-2: max associated memories to return (default: 10, max: 10)
	AssociatedMemoriesCap *int `json:"associated_memories_cap,omitempty"`
	// KG-3: traversal depth 1–3 (default: 1); requires IncludeAssociated
	AssociatedMemoriesDepth *int `json:"associated_memories_depth,omitempty"`
	// KG-3: minimum edge weight for KG traversal (default: 0.0)
	AssociatedMemoriesMinWeight *float32 `json:"associated_memories_min_weight,omitempty"`
	// CE-7: only recall memories created at or after this ISO-8601 timestamp
	Since *string `json:"since,omitempty"`
	// CE-7: only recall memories created at or before this ISO-8601 timestamp
	Until *string `json:"until,omitempty"`
	// CE-10: retrieval routing mode. nil uses the server default ("auto").
	Routing *RoutingMode `json:"routing,omitempty"`
	// CE-13: cross-encoder reranking. nil uses server default (true for recall).
	// Set to pointer-to-false to disable on latency-sensitive paths.
	Rerank *bool `json:"rerank,omitempty"`
	// CE-14: fusion strategy for hybrid recall. nil uses server default (FusionStrategyMinMax since v0.11.2).
	Fusion *FusionStrategy `json:"fusion,omitempty"`
	// CE-17: explicit vector/BM25 weight for Hybrid routing (0.0–1.0).
	// When set, overrides the adaptive heuristic from QueryClassifier.
	// Omit for adaptive defaults (recommended). Only effective when Routing = RoutingModeHybrid.
	VectorWeight *float32 `json:"vector_weight,omitempty"`
	// CE-23: pseudo-relevance feedback (PRF) passes for BM25 routing (1–3, default: 1).
	// Pass pointer-to-2 or pointer-to-3 for multi-hop or temporal queries.
	// Only effective when Routing = RoutingModeBm25.
	Iterations *uint8 `json:"iterations,omitempty"`
	// v0.11.0: session-adjacent memory enrichment (±5 min). nil uses server default (true).
	// Set to pointer-to-false to disable on latency-sensitive paths.
	Neighborhood *bool `json:"neighborhood,omitempty"`
}

// RecallResponse is the response from the recall endpoint.
// Use associated_memories (COG-2 / KG-3) by setting IncludeAssociated on the request.
// Each associated memory includes a Depth field (KG-3).
type RecallResponse struct {
	Memories           []RecalledMemory `json:"memories"`
	AssociatedMemories []RecalledMemory `json:"associated_memories,omitempty"`
}

// UpdateMemoryRequest represents a request to update a memory.
type UpdateMemoryRequest struct {
	Content    *string                `json:"content,omitempty"`
	Metadata   map[string]interface{} `json:"metadata,omitempty"`
	MemoryType *string                `json:"memory_type,omitempty"`
}

// SearchMemoriesRequest represents a request to search memories.
type SearchMemoriesRequest struct {
	AgentID       string   `json:"agent_id,omitempty"`
	Query         string   `json:"query"`
	TopK          int      `json:"top_k,omitempty"`
	MemoryType    string   `json:"memory_type,omitempty"`
	MinImportance *float32 `json:"min_importance,omitempty"`
	// CE-10: retrieval routing mode. nil uses the server default ("auto").
	Routing *RoutingMode `json:"routing,omitempty"`
	// CE-13: cross-encoder reranking. nil uses server default (false for search).
	// Set to pointer-to-true to enable reranking on search queries.
	Rerank *bool `json:"rerank,omitempty"`
}

// UpdateImportanceRequest represents a request to update memory importance.
type UpdateImportanceRequest struct {
	AgentID    string   `json:"agent_id,omitempty"`
	MemoryIDs  []string `json:"memory_ids"`
	Importance float32  `json:"importance"`
}

// ConsolidationConfig is the optional algorithm config for DBSCAN adaptive
// consolidation (CE-6).
type ConsolidationConfig struct {
	// Algorithm selects the clustering algorithm: "dbscan" (default) or "greedy".
	Algorithm  string   `json:"algorithm,omitempty"`
	MinSamples *int     `json:"min_samples,omitempty"`
	Eps        *float32 `json:"eps,omitempty"`
}

// ConsolidationLogEntry is one step in the consolidation execution log (CE-6).
type ConsolidationLogEntry struct {
	Step           string  `json:"step"`
	MemoriesBefore int     `json:"memories_before"`
	MemoriesAfter  int     `json:"memories_after"`
	DurationMs     float64 `json:"duration_ms"`
}

// ConsolidateRequest represents a request to consolidate memories.
type ConsolidateRequest struct {
	AgentID    string               `json:"agent_id,omitempty"`
	MemoryType string               `json:"memory_type,omitempty"`
	Threshold  *float32             `json:"threshold,omitempty"`
	DryRun     bool                 `json:"dry_run,omitempty"`
	// Config selects the DBSCAN clustering algorithm and tunes its parameters (CE-6).
	Config     *ConsolidationConfig `json:"config,omitempty"`
}

// ConsolidateResponse represents the response from consolidation.
type ConsolidateResponse struct {
	MemoriesRemoved    int                     `json:"memories_removed"`
	SourceMemoryIDs    []string                `json:"source_memory_ids"`
	ConsolidatedMemory *Memory                 `json:"consolidated_memory,omitempty"`
	// Log is the step-by-step consolidation log (CE-6, may be nil).
	Log                []ConsolidationLogEntry `json:"log,omitempty"`
}

// MemoryFeedbackRequest represents a request for memory feedback.
type MemoryFeedbackRequest struct {
	MemoryID       string   `json:"memory_id"`
	Feedback       string   `json:"feedback"`
	RelevanceScore *float32 `json:"relevance_score,omitempty"`
}

// MemoryFeedbackResponse represents the response from feedback.
type MemoryFeedbackResponse struct {
	Status            string   `json:"status"`
	UpdatedImportance *float32 `json:"updated_importance,omitempty"`
}

// ===========================================================================
// Session Types
// ===========================================================================

// StartSessionRequest represents a request to start a session.
type StartSessionRequest struct {
	AgentID  string                 `json:"agent_id"`
	Metadata map[string]interface{} `json:"metadata,omitempty"`
}

// Session represents a session.
type Session struct {
	ID          string                 `json:"id"`
	AgentID     string                 `json:"agent_id"`
	StartedAt   int64                  `json:"started_at,omitempty"`
	EndedAt     *int64                 `json:"ended_at,omitempty"`
	Summary     string                 `json:"summary,omitempty"`
	Metadata    map[string]interface{} `json:"metadata,omitempty"`
	MemoryCount int                    `json:"memory_count"`
}

// SessionStartResponse is the response from POST /v1/sessions/start.
type SessionStartResponse struct {
	Session Session `json:"session"`
}

// SessionEndResponse is the response from POST /v1/sessions/{id}/end.
type SessionEndResponse struct {
	Session     Session `json:"session"`
	MemoryCount int     `json:"memory_count"`
}

// ListSessionsOptions represents options for listing sessions.
type ListSessionsOptions struct {
	AgentID    string `json:"agent_id,omitempty"`
	ActiveOnly *bool  `json:"active_only,omitempty"`
	Limit      *int   `json:"limit,omitempty"`
	Offset     *int   `json:"offset,omitempty"`
}

// ===========================================================================
// Agent Types
// ===========================================================================

// AgentSummary represents summary info for an agent.
type AgentSummary struct {
	AgentID        string `json:"agent_id"`
	MemoryCount    int64  `json:"memory_count"`
	SessionCount   int64  `json:"session_count"`
	ActiveSessions int64  `json:"active_sessions"`
}

// AgentStats represents detailed stats for an agent.
type AgentStats struct {
	AgentID        string           `json:"agent_id"`
	TotalMemories  int64            `json:"total_memories"`
	MemoriesByType map[string]int64 `json:"memories_by_type"`
	TotalSessions  int64            `json:"total_sessions"`
	ActiveSessions int64            `json:"active_sessions"`
	AvgImportance  *float32         `json:"avg_importance,omitempty"`
	OldestMemoryAt string           `json:"oldest_memory_at,omitempty"`
	NewestMemoryAt string           `json:"newest_memory_at,omitempty"`
}

// AgentMemoriesOptions represents options for listing agent memories.
type AgentMemoriesOptions struct {
	MemoryType string `json:"memory_type,omitempty"`
	Limit      *int   `json:"limit,omitempty"`
}

// AgentSessionsOptions represents options for listing agent sessions.
type AgentSessionsOptions struct {
	ActiveOnly *bool `json:"active_only,omitempty"`
	Limit      *int  `json:"limit,omitempty"`
}

// ===========================================================================
// Knowledge Graph Types
// ===========================================================================

// KnowledgeGraphRequest represents a request to build a knowledge graph.
type KnowledgeGraphRequest struct {
	AgentID       string   `json:"agent_id"`
	MemoryID      string   `json:"memory_id,omitempty"`
	Depth         *int     `json:"depth,omitempty"`
	MinSimilarity *float32 `json:"min_similarity,omitempty"`
}

// KnowledgeNode represents a node in the knowledge graph.
type KnowledgeNode struct {
	ID         string                 `json:"id"`
	Content    string                 `json:"content"`
	MemoryType string                 `json:"memory_type,omitempty"`
	Importance *float32               `json:"importance,omitempty"`
	Metadata   map[string]interface{} `json:"metadata,omitempty"`
}

// KnowledgeEdge represents an edge in the knowledge graph.
type KnowledgeEdge struct {
	Source       string  `json:"source"`
	Target       string  `json:"target"`
	Similarity   float32 `json:"similarity"`
	Relationship string  `json:"relationship,omitempty"`
}

// KnowledgeGraphResponse represents the response from knowledge graph operations.
type KnowledgeGraphResponse struct {
	Nodes    []KnowledgeNode `json:"nodes"`
	Edges    []KnowledgeEdge `json:"edges"`
	Clusters [][]string      `json:"clusters,omitempty"`
}

// FullKnowledgeGraphRequest represents a request to build a full knowledge graph.
type FullKnowledgeGraphRequest struct {
	AgentID          string   `json:"agent_id"`
	MaxNodes         *int     `json:"max_nodes,omitempty"`
	MinSimilarity    *float32 `json:"min_similarity,omitempty"`
	ClusterThreshold *float32 `json:"cluster_threshold,omitempty"`
	MaxEdgesPerNode  *int     `json:"max_edges_per_node,omitempty"`
}

// SummarizeRequest represents a request to summarize memories.
type SummarizeRequest struct {
	AgentID    string   `json:"agent_id"`
	MemoryIDs  []string `json:"memory_ids,omitempty"`
	TargetType string   `json:"target_type,omitempty"`
	DryRun     bool     `json:"dry_run,omitempty"`
}

// SummarizeResponse represents the response from summarization.
type SummarizeResponse struct {
	Summary     string `json:"summary"`
	SourceCount int    `json:"source_count"`
	NewMemoryID string `json:"new_memory_id,omitempty"`
}

// DeduplicateRequest represents a request to deduplicate memories.
type DeduplicateRequest struct {
	AgentID    string   `json:"agent_id"`
	Threshold  *float32 `json:"threshold,omitempty"`
	MemoryType string   `json:"memory_type,omitempty"`
	DryRun     bool     `json:"dry_run,omitempty"`
}

// DeduplicateResponse represents the response from deduplication.
type DeduplicateResponse struct {
	DuplicatesFound int        `json:"duplicates_found"`
	RemovedCount    int        `json:"removed_count"`
	Groups          [][]string `json:"groups"`
}

// ===========================================================================
// Analytics Types
// ===========================================================================

// AnalyticsOverview represents analytics overview response.
type AnalyticsOverview struct {
	TotalQueries     uint64  `json:"total_queries"`
	AvgLatencyMs     float64 `json:"avg_latency_ms"`
	P95LatencyMs     float64 `json:"p95_latency_ms"`
	P99LatencyMs     float64 `json:"p99_latency_ms"`
	QueriesPerSecond float64 `json:"queries_per_second"`
	ErrorRate        float64 `json:"error_rate"`
	CacheHitRate     float64 `json:"cache_hit_rate"`
	StorageUsedBytes uint64  `json:"storage_used_bytes"`
	TotalVectors     uint64  `json:"total_vectors"`
	TotalNamespaces  uint64  `json:"total_namespaces"`
	UptimeSeconds    uint64  `json:"uptime_seconds"`
}

// LatencyAnalytics represents latency analytics response.
type LatencyAnalytics struct {
	Period      string                      `json:"period"`
	AvgMs       float64                     `json:"avg_ms"`
	P50Ms       float64                     `json:"p50_ms"`
	P95Ms       float64                     `json:"p95_ms"`
	P99Ms       float64                     `json:"p99_ms"`
	MaxMs       float64                     `json:"max_ms"`
	ByOperation map[string]OperationLatency `json:"by_operation,omitempty"`
}

// OperationLatency represents latency stats for a specific operation.
type OperationLatency struct {
	AvgMs float64 `json:"avg_ms"`
	P95Ms float64 `json:"p95_ms"`
	Count uint64  `json:"count"`
}

// ThroughputAnalytics represents throughput analytics response.
type ThroughputAnalytics struct {
	Period              string            `json:"period"`
	TotalOperations     uint64            `json:"total_operations"`
	OperationsPerSecond float64           `json:"operations_per_second"`
	ByOperation         map[string]uint64 `json:"by_operation,omitempty"`
}

// StorageAnalytics represents storage analytics response.
type StorageAnalytics struct {
	TotalBytes  uint64                      `json:"total_bytes"`
	IndexBytes  uint64                      `json:"index_bytes"`
	DataBytes   uint64                      `json:"data_bytes"`
	ByNamespace map[string]NamespaceStorage `json:"by_namespace,omitempty"`
}

// NamespaceStorage represents storage info for a specific namespace.
type NamespaceStorage struct {
	Bytes       uint64 `json:"bytes"`
	VectorCount uint64 `json:"vector_count"`
}

// AnalyticsOptions represents options for analytics queries.
type AnalyticsOptions struct {
	Period    string `json:"period,omitempty"`
	Namespace string `json:"namespace,omitempty"`
}

// ===========================================================================
// Advanced Search Types
// ===========================================================================

// MultiVectorSearchRequest represents a multi-vector search request with positive/negative vectors.
type MultiVectorSearchRequest struct {
	Positive        [][]float32            `json:"positive"`
	Negative        [][]float32            `json:"negative,omitempty"`
	TopK            int                    `json:"top_k,omitempty"`
	Filter          map[string]interface{} `json:"filter,omitempty"`
	IncludeMetadata bool                   `json:"include_metadata,omitempty"`
	IncludeVectors  bool                   `json:"include_vectors,omitempty"`
	MmrLambda       *float32               `json:"mmr_lambda,omitempty"`
	MmrPrefetchK    *int                   `json:"mmr_prefetch_k,omitempty"`
}

// MultiVectorSearchResult represents a single result from multi-vector search.
type MultiVectorSearchResult struct {
	ID       string                 `json:"id"`
	Score    float32                `json:"score"`
	Values   []float32              `json:"values,omitempty"`
	Metadata map[string]interface{} `json:"metadata,omitempty"`
}

// MultiVectorSearchResponse represents the response from multi-vector search.
type MultiVectorSearchResponse struct {
	Results      []MultiVectorSearchResult `json:"results"`
	SearchTimeMs *int64                    `json:"search_time_ms,omitempty"`
	Strategy     string                    `json:"strategy,omitempty"`
}

// UnifiedQueryRequest represents a unified query combining vector and text search.
type UnifiedQueryRequest struct {
	Vector          []float32              `json:"vector,omitempty"`
	Text            string                 `json:"text,omitempty"`
	TopK            int                    `json:"top_k,omitempty"`
	Filter          map[string]interface{} `json:"filter,omitempty"`
	IncludeMetadata bool                   `json:"include_metadata,omitempty"`
	IncludeVectors  bool                   `json:"include_vectors,omitempty"`
	VectorWeight    *float32               `json:"vector_weight,omitempty"`
	TextWeight      *float32               `json:"text_weight,omitempty"`
	FusionMethod    string                 `json:"fusion_method,omitempty"`
	Rerank          bool                   `json:"rerank,omitempty"`
}

// UnifiedSearchResult represents a single result from unified query.
type UnifiedSearchResult struct {
	ID          string                 `json:"id"`
	Score       float32                `json:"score"`
	VectorScore *float32               `json:"vector_score,omitempty"`
	TextScore   *float32               `json:"text_score,omitempty"`
	Values      []float32              `json:"values,omitempty"`
	Content     string                 `json:"content,omitempty"`
	Metadata    map[string]interface{} `json:"metadata,omitempty"`
}

// UnifiedQueryResponse represents the response from unified query.
type UnifiedQueryResponse struct {
	Results      []UnifiedSearchResult `json:"results"`
	SearchTimeMs *int64                `json:"search_time_ms,omitempty"`
	FusionMethod string                `json:"fusion_method,omitempty"`
}

// AggregationRequest represents an aggregation request with grouping.
type AggregationRequest struct {
	Vector    []float32              `json:"vector,omitempty"`
	GroupBy   string                 `json:"group_by,omitempty"`
	Metrics   []string               `json:"metrics,omitempty"`
	TopK      *int                   `json:"top_k,omitempty"`
	Filter    map[string]interface{} `json:"filter,omitempty"`
	TopGroups *int                   `json:"top_groups,omitempty"`
}

// AggregationGroup represents a single aggregation group.
type AggregationGroup struct {
	Key     string                 `json:"key"`
	Count   int                    `json:"count"`
	Metrics map[string]interface{} `json:"metrics,omitempty"`
	TopHits []QueryResult          `json:"top_hits,omitempty"`
}

// AggregationResponse represents the response from aggregation.
type AggregationResponse struct {
	Groups       []AggregationGroup `json:"groups"`
	TotalGroups  int                `json:"total_groups"`
	SearchTimeMs *int64             `json:"search_time_ms,omitempty"`
}

// ExportRequest represents a request to export vectors.
type ExportRequest struct {
	Cursor         string                 `json:"cursor,omitempty"`
	Limit          *int                   `json:"limit,omitempty"`
	Filter         map[string]interface{} `json:"filter,omitempty"`
	IncludeVectors bool                   `json:"include_vectors,omitempty"`
}

// ExportedVector represents a single exported vector.
type ExportedVector struct {
	ID       string                 `json:"id"`
	Values   []float32              `json:"values,omitempty"`
	Metadata map[string]interface{} `json:"metadata,omitempty"`
}

// ExportResponse represents the response from vector export.
type ExportResponse struct {
	Vectors    []ExportedVector `json:"vectors"`
	NextCursor string           `json:"next_cursor,omitempty"`
	HasMore    bool             `json:"has_more"`
}

// QueryExplainRequest represents a request to explain a query execution plan.
type QueryExplainRequest struct {
	Vector          []float32              `json:"vector"`
	TopK            int                    `json:"top_k,omitempty"`
	Filter          map[string]interface{} `json:"filter,omitempty"`
	IncludeMetadata bool                   `json:"include_metadata,omitempty"`
}

// QueryExplainResponse represents the response from query explain.
type QueryExplainResponse struct {
	Plan         map[string]interface{}   `json:"plan"`
	Steps        []map[string]interface{} `json:"steps,omitempty"`
	TotalTimeMs  *float64                 `json:"total_time_ms,omitempty"`
	Results      []QueryResult            `json:"results,omitempty"`
	IndexType    string                   `json:"index_type,omitempty"`
	VectorsScanned *int64                 `json:"vectors_scanned,omitempty"`
}

// ColumnUpsertRequest represents a column-format upsert request for efficient bulk operations.
type ColumnUpsertRequest struct {
	IDs        []string                          `json:"ids"`
	Vectors    [][]float32                       `json:"vectors"`
	Attributes map[string][]interface{}           `json:"attributes,omitempty"`
	TTLSeconds *int                              `json:"ttl_seconds,omitempty"`
	Dimension  *int                              `json:"dimension,omitempty"`
}

// WarmCacheRequest represents a request to warm the cache.
type WarmCacheRequest struct {
	VectorIDs       []string `json:"vector_ids,omitempty"`
	Priority        string   `json:"priority,omitempty"`
	TargetTier      string   `json:"target_tier,omitempty"`
	Background      bool     `json:"background,omitempty"`
	TTLHintSeconds  *int     `json:"ttl_hint_seconds,omitempty"`
	AccessPattern   string   `json:"access_pattern,omitempty"`
	MaxVectors      *int     `json:"max_vectors,omitempty"`
}

// WarmCacheResponse represents the response from cache warming.
type WarmCacheResponse struct {
	Status       string `json:"status"`
	EntriesWarmed int   `json:"entries_warmed"`
	TimeTakenMs  *int64 `json:"time_taken_ms,omitempty"`
}

// ===========================================================================
// Admin Types
// ===========================================================================

// OpsStats represents the ops stats response — Read-scoped; works with read-only API keys.
type OpsStats struct {
	Version        string `json:"version"`
	TotalVectors   int64  `json:"total_vectors"`
	NamespaceCount int64  `json:"namespace_count"`
	UptimeSeconds  int64  `json:"uptime_seconds"`
	Timestamp      int64  `json:"timestamp"`
	State          string `json:"state"`
}

// ClusterStatus represents the cluster status response.
type ClusterStatus struct {
	Status       string `json:"status"`
	Nodes        int    `json:"nodes"`
	Healthy      bool   `json:"healthy"`
	Version      string `json:"version,omitempty"`
	// RedisHealthy indicates Redis connectivity (OPS-3).
	RedisHealthy *bool  `json:"redis_healthy,omitempty"`
}

// ClusterNode represents a cluster node.
type ClusterNode struct {
	ID      string `json:"id"`
	Address string `json:"address"`
	Status  string `json:"status"`
	Role    string `json:"role,omitempty"`
}

// CacheStats represents cache statistics.
type CacheStats struct {
	TotalEntries int     `json:"total_entries"`
	HitRate      float64 `json:"hit_rate"`
	MemoryBytes  int64   `json:"memory_bytes"`
}

// SlowQuery represents a slow query entry.
type SlowQuery struct {
	Query      string  `json:"query"`
	DurationMs float64 `json:"duration_ms"`
	Timestamp  string  `json:"timestamp"`
	Namespace  string  `json:"namespace,omitempty"`
}

// BackupInfo represents backup information.
type BackupInfo struct {
	ID          string `json:"id"`
	CreatedAt   string `json:"created_at"`
	SizeBytes   int64  `json:"size_bytes"`
	Status      string `json:"status"`
	IncludeData bool   `json:"include_data"`
}

// TtlConfig represents TTL configuration for a namespace.
type TtlConfig struct {
	Namespace  string `json:"namespace"`
	TtlSeconds int    `json:"ttl_seconds"`
	Strategy   string `json:"strategy,omitempty"`
}

// SlowQueryOptions represents options for querying slow queries.
type SlowQueryOptions struct {
	Limit         int `json:"limit,omitempty"`
	MinDurationMs int `json:"min_duration_ms,omitempty"`
}

// AutoPilotConfig represents the AutoPilot configuration.
type AutoPilotConfig struct {
	Enabled                      bool    `json:"enabled"`
	DedupThreshold               float32 `json:"dedup_threshold"`
	DedupIntervalHours           uint64  `json:"dedup_interval_hours"`
	ConsolidationIntervalHours   uint64  `json:"consolidation_interval_hours"`
}

// DedupResultSnapshot is the result from a deduplication cycle.
type DedupResultSnapshot struct {
	NamespacesProcessed int `json:"namespaces_processed"`
	MemoriesScanned     int `json:"memories_scanned"`
	DuplicatesRemoved   int `json:"duplicates_removed"`
}

// ConsolidationResultSnapshot is the result from a consolidation cycle.
type ConsolidationResultSnapshot struct {
	NamespacesProcessed  int `json:"namespaces_processed"`
	MemoriesScanned      int `json:"memories_scanned"`
	ClustersMerged       int `json:"clusters_merged"`
	MemoriesConsolidated int `json:"memories_consolidated"`
}

// AutoPilotStatusResponse is returned by GET /v1/admin/autopilot/status (PILOT-1).
type AutoPilotStatusResponse struct {
	Config                AutoPilotConfig              `json:"config"`
	LastDedupAt           *uint64                      `json:"last_dedup_at,omitempty"`
	LastConsolidationAt   *uint64                      `json:"last_consolidation_at,omitempty"`
	LastDedup             *DedupResultSnapshot          `json:"last_dedup,omitempty"`
	LastConsolidation     *ConsolidationResultSnapshot  `json:"last_consolidation,omitempty"`
	TotalDedupRemoved     uint64                       `json:"total_dedup_removed"`
	TotalConsolidated     uint64                       `json:"total_consolidated"`
}

// AutoPilotConfigRequest is the request for PUT /v1/admin/autopilot/config (PILOT-2).
// All fields are optional — nil means "keep current value".
type AutoPilotConfigRequest struct {
	Enabled                    *bool    `json:"enabled,omitempty"`
	DedupThreshold             *float32 `json:"dedup_threshold,omitempty"`
	DedupIntervalHours         *uint64  `json:"dedup_interval_hours,omitempty"`
	ConsolidationIntervalHours *uint64  `json:"consolidation_interval_hours,omitempty"`
}

// AutoPilotConfigResponse is returned by PUT /v1/admin/autopilot/config (PILOT-2).
type AutoPilotConfigResponse struct {
	Success bool            `json:"success"`
	Config  AutoPilotConfig `json:"config"`
	Message string          `json:"message"`
}

// AutoPilotDedupResult is the dedup result from a manual trigger.
type AutoPilotDedupResult struct {
	NamespacesProcessed int `json:"namespaces_processed"`
	MemoriesScanned     int `json:"memories_scanned"`
	DuplicatesRemoved   int `json:"duplicates_removed"`
}

// AutoPilotConsolidationResult is the consolidation result from a manual trigger.
type AutoPilotConsolidationResult struct {
	NamespacesProcessed  int `json:"namespaces_processed"`
	MemoriesScanned      int `json:"memories_scanned"`
	ClustersMerged       int `json:"clusters_merged"`
	MemoriesConsolidated int `json:"memories_consolidated"`
}

// AutoPilotTriggerResponse is returned by POST /v1/admin/autopilot/trigger (PILOT-3).
type AutoPilotTriggerResponse struct {
	Success       bool                          `json:"success"`
	Action        string                        `json:"action"`
	Dedup         *AutoPilotDedupResult         `json:"dedup,omitempty"`
	Consolidation *AutoPilotConsolidationResult `json:"consolidation,omitempty"`
	Message       string                        `json:"message"`
}

// ===========================================================================
// Decay Engine Types (DECAY-1 / DECAY-2)
// ===========================================================================

// DecayConfigResponse is returned by GET /v1/admin/decay/config (DECAY-1).
type DecayConfigResponse struct {
	// Strategy is the decay strategy: "exponential", "linear", or "step".
	Strategy string `json:"strategy"`
	// HalfLifeHours is the half-life in hours.
	HalfLifeHours float64 `json:"half_life_hours"`
	// MinImportance is the minimum importance threshold; memories below are
	// hard-deleted on the next decay cycle.
	MinImportance float32 `json:"min_importance"`
}

// DecayConfigUpdateRequest is the request for PUT /v1/admin/decay/config (DECAY-1).
// All fields are optional — omit any to keep its current value.
type DecayConfigUpdateRequest struct {
	// Strategy is the decay strategy: "exponential", "linear", or "step".
	Strategy *string `json:"strategy,omitempty"`
	// HalfLifeHours must be > 0.
	HalfLifeHours *float64 `json:"half_life_hours,omitempty"`
	// MinImportance must be 0.0–1.0.
	MinImportance *float32 `json:"min_importance,omitempty"`
}

// DecayConfigUpdateResponse is returned by PUT /v1/admin/decay/config (DECAY-1).
type DecayConfigUpdateResponse struct {
	Success bool                `json:"success"`
	Config  DecayConfigResponse `json:"config"`
	Message string              `json:"message"`
}

// LastDecayCycleStats holds per-cycle statistics from a single decay run.
type LastDecayCycleStats struct {
	NamespacesProcessed int `json:"namespaces_processed"`
	MemoriesProcessed   int `json:"memories_processed"`
	MemoriesDecayed     int `json:"memories_decayed"`
	MemoriesDeleted     int `json:"memories_deleted"`
}

// DecayStatsResponse is returned by GET /v1/admin/decay/stats (DECAY-2).
type DecayStatsResponse struct {
	// TotalDecayed is the all-time count of memories whose importance was lowered.
	TotalDecayed uint64 `json:"total_decayed"`
	// TotalDeleted is the all-time count of memories hard-deleted by decay or TTL.
	TotalDeleted uint64 `json:"total_deleted"`
	// LastRunAt is the Unix timestamp of the last decay cycle (nil if never run).
	LastRunAt *uint64 `json:"last_run_at,omitempty"`
	// CyclesRun is the number of decay cycles completed since startup.
	CyclesRun uint64 `json:"cycles_run"`
	// LastCycle holds stats from the most recent decay cycle (nil if never run).
	LastCycle *LastDecayCycleStats `json:"last_cycle,omitempty"`
}

// ===========================================================================
// API Key Types
// ===========================================================================

// ApiKey represents an API key.
type ApiKey struct {
	ID          string   `json:"id"`
	Name        string   `json:"name"`
	Key         string   `json:"key,omitempty"`
	Permissions []string `json:"permissions,omitempty"`
	CreatedAt   string   `json:"created_at"`
	ExpiresAt   string   `json:"expires_at,omitempty"`
	Active      bool     `json:"active"`
}

// CreateKeyRequest represents a request to create an API key.
type CreateKeyRequest struct {
	Name        string   `json:"name"`
	Permissions []string `json:"permissions,omitempty"`
	ExpiresAt   string   `json:"expires_at,omitempty"`
}

// KeyUsage represents usage statistics for an API key.
type KeyUsage struct {
	KeyID              string           `json:"key_id"`
	TotalRequests      int64            `json:"total_requests"`
	LastUsed           string           `json:"last_used,omitempty"`
	RequestsByEndpoint map[string]int64 `json:"requests_by_endpoint,omitempty"`
}

// ===========================================================================
// Cross-Agent Network Types (DASH-A)
// ===========================================================================

// CrossAgentNetworkRequest configures the cross-agent similarity graph query.
// All fields are optional; zero values use server defaults.
type CrossAgentNetworkRequest struct {
	// Specific agent IDs to include. nil or empty means all agents.
	AgentIDs []string `json:"agent_ids,omitempty"`
	// Minimum cosine similarity for a cross-agent edge (default 0.3).
	MinSimilarity float32 `json:"min_similarity,omitempty"`
	// Maximum memories per agent, by descending importance (default 50).
	MaxNodesPerAgent int `json:"max_nodes_per_agent,omitempty"`
	// Minimum importance score for a memory to be included (default 0.0).
	MinImportance float32 `json:"min_importance,omitempty"`
	// Maximum cross-agent edges to return (default 200).
	MaxCrossEdges int `json:"max_cross_edges,omitempty"`
}

// AgentNetworkInfo is summary information for one agent.
type AgentNetworkInfo struct {
	AgentID       string  `json:"agent_id"`
	MemoryCount   int     `json:"memory_count"`
	AvgImportance float32 `json:"avg_importance"`
}

// AgentNetworkNode is a memory node in the cross-agent network graph.
type AgentNetworkNode struct {
	ID         string   `json:"id"`
	AgentID    string   `json:"agent_id"`
	Content    string   `json:"content"`
	Importance float32  `json:"importance"`
	Tags       []string `json:"tags"`
	MemoryType string   `json:"memory_type"`
	CreatedAt  int64    `json:"created_at"`
}

// AgentNetworkEdge is a similarity edge between memories from two different agents.
type AgentNetworkEdge struct {
	Source      string  `json:"source"`
	Target      string  `json:"target"`
	SourceAgent string  `json:"source_agent"`
	TargetAgent string  `json:"target_agent"`
	Similarity  float32 `json:"similarity"`
}

// AgentNetworkStats contains network-level statistics.
type AgentNetworkStats struct {
	TotalAgents     int     `json:"total_agents"`
	TotalNodes      int     `json:"total_nodes"`
	TotalCrossEdges int     `json:"total_cross_edges"`
	Density         float32 `json:"density"`
}

// CrossAgentNetworkResponse is returned by CrossAgentNetwork.
type CrossAgentNetworkResponse struct {
	Agents    []AgentNetworkInfo `json:"agents"`
	Nodes     []AgentNetworkNode `json:"nodes"`
	Edges     []AgentNetworkEdge `json:"edges"`
	Stats     AgentNetworkStats  `json:"stats"`
	NodeCount int                `json:"node_count"` // Total memory nodes in the network (server v0.6.2+).
}

// ===========================================================================
// SSE Streaming Event Types (CE-1)
// ===========================================================================

// OpStatus is the operation status for OperationProgress events.
type OpStatus string

const (
	OpStatusPending   OpStatus = "pending"
	OpStatusRunning   OpStatus = "running"
	OpStatusCompleted OpStatus = "completed"
	OpStatusFailed    OpStatus = "failed"
)

// VectorMutationOp is the mutation type for VectorsMutated events.
type VectorMutationOp string

const (
	VectorMutationUpserted VectorMutationOp = "upserted"
	VectorMutationDeleted  VectorMutationOp = "deleted"
)

// DakeraEvent is an event received from a Dakera SSE stream.
//
// The Type field identifies the event variant; only the fields relevant to
// that variant will be populated.
//
//   - "connected"            → Timestamp (connection confirmed, emitted on subscribe)
//   - "namespace_created"    → Namespace, Dimension
//   - "namespace_deleted"    → Namespace
//   - "operation_progress"   → OperationID, Namespace, OpType, Progress, Status, Message, UpdatedAt
//   - "job_progress"         → JobID, JobType, Namespace, Progress, Status
//   - "vectors_mutated"      → Namespace, Op, Count
//   - "stream_lagged"        → Dropped, Hint
type DakeraEvent struct {
	Type string `json:"type"`

	// connected
	Timestamp int64 `json:"timestamp,omitempty"`
	// namespace_created / namespace_deleted / vectors_mutated / operation_progress / job_progress
	Namespace string `json:"namespace,omitempty"`
	// namespace_created
	Dimension int `json:"dimension,omitempty"`
	// operation_progress
	OperationID string   `json:"operation_id,omitempty"`
	OpType      string   `json:"op_type,omitempty"`
	Progress    int      `json:"progress,omitempty"`
	Status      string   `json:"status,omitempty"`
	Message     string   `json:"message,omitempty"`
	UpdatedAt   int64    `json:"updated_at,omitempty"`
	// job_progress
	JobID   string `json:"job_id,omitempty"`
	JobType string `json:"job_type,omitempty"`
	// vectors_mutated
	Op    VectorMutationOp `json:"op,omitempty"`
	Count int              `json:"count,omitempty"`
	// stream_lagged
	Dropped int64  `json:"dropped,omitempty"`
	Hint    string `json:"hint,omitempty"`
}

// ---------------------------------------------------------------------------
// OPS-1: Rate-Limit Headers
// ---------------------------------------------------------------------------

// RateLimitHeaders holds rate-limit and quota headers from an API response.
//
// Fields are zero when the server does not include the header (e.g.
// non-namespaced endpoints where quota does not apply).
type RateLimitHeaders struct {
	// Limit is X-RateLimit-Limit — max requests allowed in the current window (0 = not present).
	Limit int64
	// Remaining is X-RateLimit-Remaining — requests left in the current window (0 = not present).
	Remaining int64
	// Reset is X-RateLimit-Reset — Unix timestamp (seconds) when the window resets (0 = not present).
	Reset int64
	// QuotaUsed is X-Quota-Used — namespace vectors / storage consumed (0 = not present).
	QuotaUsed int64
	// QuotaLimit is X-Quota-Limit — namespace quota ceiling (0 = not present).
	QuotaLimit int64
}

// ---------------------------------------------------------------------------
// CE-2: Batch Recall / Forget
// ---------------------------------------------------------------------------

// BatchMemoryFilter holds filter predicates for batch memory operations (CE-2).
//
// All fields are optional.  For BatchForget at least one must be set
// (server-side safety guard).
type BatchMemoryFilter struct {
	// Tags restricts to memories that carry all listed tags.
	Tags []string `json:"tags,omitempty"`
	// MinImportance is the minimum importance (inclusive).
	MinImportance *float32 `json:"min_importance,omitempty"`
	// MaxImportance is the maximum importance (inclusive).
	MaxImportance *float32 `json:"max_importance,omitempty"`
	// CreatedAfter restricts to memories created at or after this Unix timestamp (seconds).
	CreatedAfter *int64 `json:"created_after,omitempty"`
	// CreatedBefore restricts to memories created before or at this Unix timestamp (seconds).
	CreatedBefore *int64 `json:"created_before,omitempty"`
	// MemoryType restricts to a specific memory type (e.g. "episodic").
	MemoryType string `json:"memory_type,omitempty"`
	// SessionID restricts to memories from a specific session.
	SessionID string `json:"session_id,omitempty"`
}

// BatchRecallRequest is the request body for POST /v1/memories/recall/batch.
type BatchRecallRequest struct {
	// AgentID is the agent whose memory namespace to search.
	AgentID string `json:"agent_id"`
	// Filter contains the filter predicates to apply.
	Filter BatchMemoryFilter `json:"filter"`
	// Limit is the maximum number of results to return (default: 100).
	Limit int `json:"limit,omitempty"`
}

// BatchRecallResponse is the response from POST /v1/memories/recall/batch.
type BatchRecallResponse struct {
	Memories []RecalledMemory `json:"memories"`
	// Total is the total memories in the agent namespace.
	Total int `json:"total"`
	// Filtered is the number of memories that passed the filter.
	Filtered int `json:"filtered"`
}

// BatchForgetRequest is the request body for DELETE /v1/memories/forget/batch.
type BatchForgetRequest struct {
	// AgentID is the agent whose memory namespace to purge from.
	AgentID string `json:"agent_id"`
	// Filter contains the filter predicates — at least one must be set (server safety guard).
	Filter BatchMemoryFilter `json:"filter"`
}

// BatchForgetResponse is the response from DELETE /v1/memories/forget/batch.
type BatchForgetResponse struct {
	DeletedCount int `json:"deleted_count"`
}

// ---------------------------------------------------------------------------
// CE-5 / SDK-9: Memory Knowledge Graph
// ---------------------------------------------------------------------------

// EdgeType classifies a relationship edge in the memory knowledge graph.
//
//   - EdgeTypeRelatedTo: cosine similarity ≥ 0.85 — semantically similar memories.
//   - EdgeTypeSharesEntity: both memories share a named entity (CE-4 tags).
//   - EdgeTypePrecedes: temporal ordering — source was created before target.
//   - EdgeTypeLinkedBy: explicit user/agent-created link.
type EdgeType string

const (
	// EdgeTypeRelatedTo indicates two memories are semantically similar (cosine ≥ 0.85).
	EdgeTypeRelatedTo EdgeType = "related_to"
	// EdgeTypeSharesEntity indicates both memories reference the same named entity.
	EdgeTypeSharesEntity EdgeType = "shares_entity"
	// EdgeTypePrecedes indicates source was created before target (temporal ordering).
	EdgeTypePrecedes EdgeType = "precedes"
	// EdgeTypeLinkedBy indicates an explicit user/agent-created link.
	EdgeTypeLinkedBy EdgeType = "linked_by"
)

// GraphEdge is a directed edge in the memory knowledge graph.
type GraphEdge struct {
	// ID is the unique edge identifier.
	ID string `json:"id"`
	// SourceID is the source memory ID.
	SourceID string `json:"source_id"`
	// TargetID is the target memory ID.
	TargetID string `json:"target_id"`
	// EdgeType is the relationship type between the two memories.
	EdgeType EdgeType `json:"edge_type"`
	// Weight is the edge weight (0.0–1.0). For RelatedTo this is the cosine similarity score.
	Weight float64 `json:"weight"`
	// CreatedAt is the Unix timestamp of edge creation.
	CreatedAt int64 `json:"created_at"`
}

// GraphNode is a memory node in the knowledge graph traversal result.
type GraphNode struct {
	// MemoryID is the memory identifier.
	MemoryID string `json:"memory_id"`
	// ContentPreview is the first 200 characters of memory content.
	ContentPreview string `json:"content_preview"`
	// Importance is the memory importance score.
	Importance float64 `json:"importance"`
	// Depth is the traversal depth from the root node (root = 0).
	Depth int `json:"depth"`
}

// MemoryGraph is the graph traversal result from GET /v1/memories/{id}/graph.
type MemoryGraph struct {
	// RootID is the root memory ID from which traversal started.
	RootID string `json:"root_id"`
	// Depth is the maximum traversal depth used.
	Depth int `json:"depth"`
	// Nodes contains all memory nodes reachable within the requested depth.
	Nodes []GraphNode `json:"nodes"`
	// Edges contains all edges connecting the returned nodes.
	Edges []GraphEdge `json:"edges"`
}

// GraphPath is the shortest path between two memories from GET /v1/memories/{id}/path.
type GraphPath struct {
	// SourceID is the starting memory ID.
	SourceID string `json:"source_id"`
	// TargetID is the destination memory ID.
	TargetID string `json:"target_id"`
	// Path is the ordered list of memory IDs from source to target (inclusive).
	Path []string `json:"path"`
	// Hops is the number of edges traversed (len(Path) - 1). -1 if no path exists.
	Hops int `json:"hops"`
	// Edges are the edges along the path, in traversal order.
	Edges []GraphEdge `json:"edges"`
}

// GraphLinkRequest is the request body for POST /v1/memories/{id}/links.
type GraphLinkRequest struct {
	// TargetID is the target memory ID to link to.
	TargetID string `json:"target_id"`
	// EdgeType is the edge type — must be EdgeTypeLinkedBy for explicit links.
	EdgeType EdgeType `json:"edge_type"`
}

// GraphLinkResponse is the response from POST /v1/memories/{id}/links.
type GraphLinkResponse struct {
	// Edge is the newly created edge.
	Edge GraphEdge `json:"edge"`
}

// GraphExport is the agent graph export from GET /v1/agents/{id}/graph/export.
type GraphExport struct {
	// AgentID is the agent whose graph was exported.
	AgentID string `json:"agent_id"`
	// Format is the export format: "json", "graphml", or "csv".
	Format string `json:"format"`
	// Data is the serialised graph in the requested format.
	Data string `json:"data"`
	// NodeCount is the total number of memory nodes in the export.
	NodeCount int64 `json:"node_count"`
	// EdgeCount is the total number of edges in the export.
	EdgeCount int64 `json:"edge_count"`
}

// GraphOptions holds options for the MemoryGraph method.
type GraphOptions struct {
	// Depth is the maximum traversal depth (default: 1, max: 3).
	Depth int
	// Types filters by edge types. nil or empty returns all types.
	Types []EdgeType
}

// ===========================================================================
// CE-4 Entity Extraction (GLiNER)
// ===========================================================================

// NamespaceNerConfig holds entity extraction configuration for a namespace (CE-4).
type NamespaceNerConfig struct {
	ExtractEntities bool     `json:"extract_entities"`
	EntityTypes     []string `json:"entity_types,omitempty"`
}

// ExtractedEntity is a single entity extracted by GLiNER or the rule-based pipeline.
type ExtractedEntity struct {
	EntityType string  `json:"entity_type"`
	Value      string  `json:"value"`
	Score      float64 `json:"score"`
}

// EntityExtractionResponse is returned by ExtractEntities.
type EntityExtractionResponse struct {
	Entities []ExtractedEntity `json:"entities"`
}

// MemoryEntitiesResponse is returned by MemoryEntities.
type MemoryEntitiesResponse struct {
	MemoryID string            `json:"memory_id"`
	Entities []ExtractedEntity `json:"entities"`
}

// ===========================================================================
// Memory Feedback Loop (INT-1)
// ===========================================================================

// FeedbackSignal is the signal type for memory active learning (INT-1).
//
//   - "upvote": Boost importance ×1.15, capped at 1.0.
//   - "downvote": Penalise importance ×0.85, floor 0.0.
//   - "flag": Mark as irrelevant — sets decay_flag=true, no immediate importance change.
//   - "positive": Backward-compatible alias for "upvote".
//   - "negative": Backward-compatible alias for "downvote".
type FeedbackSignal string

const (
	FeedbackSignalUpvote   FeedbackSignal = "upvote"
	FeedbackSignalDownvote FeedbackSignal = "downvote"
	FeedbackSignalFlag     FeedbackSignal = "flag"
	FeedbackSignalPositive FeedbackSignal = "positive"
	FeedbackSignalNegative FeedbackSignal = "negative"
)

// FeedbackHistoryEntry is a single recorded feedback event stored in memory metadata (INT-1).
type FeedbackHistoryEntry struct {
	Signal        FeedbackSignal `json:"signal"`
	Timestamp     uint64         `json:"timestamp"`
	OldImportance float32        `json:"old_importance"`
	NewImportance float32        `json:"new_importance"`
}

// MemoryFeedbackBodyRequest is the request body for POST /v1/memories/:id/feedback (INT-1).
type MemoryFeedbackBodyRequest struct {
	AgentID string         `json:"agent_id"`
	Signal  FeedbackSignal `json:"signal"`
}

// MemoryImportancePatchRequest is the request body for PATCH /v1/memories/:id/importance (INT-1).
type MemoryImportancePatchRequest struct {
	AgentID    string  `json:"agent_id"`
	Importance float32 `json:"importance"`
}

// FeedbackResponse is returned by FeedbackMemory and PatchMemoryImportance (INT-1).
type FeedbackResponse struct {
	MemoryID      string         `json:"memory_id"`
	NewImportance float32        `json:"new_importance"`
	Signal        FeedbackSignal `json:"signal"`
}

// FeedbackHistoryResponse is returned by GetMemoryFeedbackHistory (INT-1).
type FeedbackHistoryResponse struct {
	MemoryID string                 `json:"memory_id"`
	Entries  []FeedbackHistoryEntry `json:"entries"`
}

// AgentFeedbackSummary is returned by GetAgentFeedbackSummary (INT-1).
type AgentFeedbackSummary struct {
	AgentID       string  `json:"agent_id"`
	Upvotes       uint64  `json:"upvotes"`
	Downvotes     uint64  `json:"downvotes"`
	Flags         uint64  `json:"flags"`
	TotalFeedback uint64  `json:"total_feedback"`
	HealthScore   float32 `json:"health_score"`
}

// FeedbackHealthResponse is returned by GetFeedbackHealth (INT-1).
type FeedbackHealthResponse struct {
	AgentID       string  `json:"agent_id"`
	HealthScore   float32 `json:"health_score"`
	MemoryCount   int     `json:"memory_count"`
	AvgImportance float32 `json:"avg_importance"`
}

// =============================================================================
// Namespace API Keys (SEC-1)
// =============================================================================

// KeySuccessResponse is returned by key deletion and deactivation endpoints.
type KeySuccessResponse struct {
	Success bool   `json:"success"`
	Message string `json:"message"`
}

// CreateNamespaceKeyRequest is the request body for POST /v1/namespaces/:ns/keys (SEC-1).
type CreateNamespaceKeyRequest struct {
	Name         string `json:"name"`
	ExpiresInDays *int  `json:"expires_in_days,omitempty"`
}

// CreateNamespaceKeyResponse is returned by POST /v1/namespaces/:ns/keys (SEC-1).
// The Key field is shown only once — store it securely.
type CreateNamespaceKeyResponse struct {
	KeyID     string  `json:"key_id"`
	Key       string  `json:"key"`
	Name      string  `json:"name"`
	Namespace string  `json:"namespace"`
	CreatedAt int64   `json:"created_at"`
	ExpiresAt *int64  `json:"expires_at,omitempty"`
	Warning   string  `json:"warning"`
}

// NamespaceKeyInfo holds namespace-scoped API key metadata (no secret) — SEC-1.
type NamespaceKeyInfo struct {
	KeyID     string  `json:"key_id"`
	Name      string  `json:"name"`
	Namespace string  `json:"namespace"`
	CreatedAt int64   `json:"created_at"`
	Active    bool    `json:"active"`
	ExpiresAt *int64  `json:"expires_at,omitempty"`
}

// ListNamespaceKeysResponse is returned by GET /v1/namespaces/:ns/keys (SEC-1).
type ListNamespaceKeysResponse struct {
	Namespace string             `json:"namespace"`
	Keys      []NamespaceKeyInfo `json:"keys"`
	Total     int                `json:"total"`
}

// NamespaceKeyUsageResponse is returned by GET /v1/namespaces/:ns/keys/:key_id/usage (SEC-1).
type NamespaceKeyUsageResponse struct {
	KeyID                string  `json:"key_id"`
	Namespace            string  `json:"namespace"`
	TotalRequests        uint64  `json:"total_requests"`
	SuccessfulRequests   uint64  `json:"successful_requests"`
	FailedRequests       uint64  `json:"failed_requests"`
	BytesTransferred     uint64  `json:"bytes_transferred"`
	AvgLatencyMs         float64 `json:"avg_latency_ms"`
}

// ===========================================================================
// DX-1: Memory Import / Export
// ===========================================================================

// MemoryImportResponse is returned by POST /v1/import (DX-1).
type MemoryImportResponse struct {
	ImportedCount int      `json:"imported_count"`
	SkippedCount  int      `json:"skipped_count"`
	Errors        []string `json:"errors,omitempty"`
}

// MemoryExportResponse is returned by GET /v1/export (DX-1).
type MemoryExportResponse struct {
	Data   []map[string]interface{} `json:"data"`
	Format string                   `json:"format"`
	Count  int                      `json:"count"`
}

// ===========================================================================
// OBS-1: Business-Event Audit Log
// ===========================================================================

// AuditEvent is a single business-event entry from the audit log (OBS-1).
type AuditEvent struct {
	ID        string                 `json:"id"`
	EventType string                 `json:"event_type"`
	AgentID   string                 `json:"agent_id,omitempty"`
	Namespace string                 `json:"namespace,omitempty"`
	Timestamp int64                  `json:"timestamp"`
	Details   map[string]interface{} `json:"details,omitempty"`
}

// AuditListResponse is returned by GET /v1/audit (OBS-1).
type AuditListResponse struct {
	Events []AuditEvent `json:"events"`
	Total  int          `json:"total"`
	Cursor string       `json:"cursor,omitempty"`
}

// AuditExportResponse is returned by POST /v1/audit/export (OBS-1).
type AuditExportResponse struct {
	Data   string `json:"data"`
	Format string `json:"format"`
	Count  int    `json:"count"`
}

// AuditQuery holds optional filters for audit log queries (OBS-1).
type AuditQuery struct {
	AgentID   string
	EventType string
	FromTs    int64
	ToTs      int64
	Limit     int
	Cursor    string
}

// ===========================================================================
// EXT-1: External Extraction Providers
// ===========================================================================

// ExtractionResult is returned by POST /v1/extract (EXT-1).
type ExtractionResult struct {
	Entities   []map[string]interface{} `json:"entities"`
	Provider   string                   `json:"provider"`
	Model      string                   `json:"model,omitempty"`
	DurationMs float64                  `json:"duration_ms"`
}

// ExtractionProviderInfo describes an available extraction provider (EXT-1).
type ExtractionProviderInfo struct {
	Name      string   `json:"name"`
	Available bool     `json:"available"`
	Models    []string `json:"models,omitempty"`
}

// extractProvidersResponse handles both array and object response shapes.
type extractProvidersResponse struct {
	Providers []ExtractionProviderInfo `json:"providers"`
}

// ===========================================================================
// SEC-3: AES-256-GCM Encryption Key Rotation
// ===========================================================================

// RotateEncryptionKeyRequest is the body for POST /v1/admin/encryption/rotate-key (SEC-3).
type RotateEncryptionKeyRequest struct {
	// NewKey is the new passphrase or 64-char hex key to rotate to.
	NewKey string `json:"new_key"`
	// Namespace, if set, restricts rotation to memories in that namespace.
	// Omit (empty string) to rotate all namespaces.
	Namespace string `json:"namespace,omitempty"`
}

// RotateEncryptionKeyResponse is returned by POST /v1/admin/encryption/rotate-key (SEC-3).
type RotateEncryptionKeyResponse struct {
	Rotated    int      `json:"rotated"`
	Skipped    int      `json:"skipped"`
	Namespaces []string `json:"namespaces"`
}

// ===========================================================================
// ODE-2: GLiNER Entity Extraction (dakera-ode sidecar)
// ===========================================================================

// OdeEntity is a single entity extracted by the GLiNER model (ODE-2).
type OdeEntity struct {
	// Text is the span text as it appears in the input.
	Text string `json:"text"`
	// Label is the entity type label (e.g. "person", "organization").
	Label string `json:"label"`
	// Start is the start character offset (inclusive) within the input text.
	Start int `json:"start"`
	// End is the end character offset (exclusive) within the input text.
	End int `json:"end"`
	// Score is the confidence score in the range [0, 1].
	Score float64 `json:"score"`
}

// ExtractEntitiesRequest is the body for POST /ode/extract (ODE-2).
type ExtractEntitiesRequest struct {
	// Content is the text to extract entities from.
	Content string `json:"content"`
	// AgentID is the agent context for the extraction.
	AgentID string `json:"agent_id"`
	// MemoryID is an optional memory ID to associate with the extraction.
	MemoryID string `json:"memory_id,omitempty"`
	// EntityTypes is an optional list of entity type labels to extract.
	// When omitted the ODE sidecar uses its default set.
	EntityTypes []string `json:"entity_types,omitempty"`
}

// ExtractEntitiesResponse is returned by POST /ode/extract on the ODE sidecar (ODE-2).
type ExtractEntitiesResponse struct {
	// Entities are extracted entities ordered by their start offset.
	Entities []OdeEntity `json:"entities"`
	// Model is the GLiNER model variant used for extraction.
	Model string `json:"model"`
	// ProcessingTimeMs is the wall-clock time taken by the ODE sidecar in milliseconds.
	ProcessingTimeMs int `json:"processing_time_ms"`
}

// ============================================================================
// KG-2: Graph Query & Export Types
// ============================================================================

// KgQueryResponse is returned by GET /v1/knowledge/query (KG-2).
type KgQueryResponse struct {
	// AgentID is the agent whose graph was queried.
	AgentID string `json:"agent_id"`
	// NodeCount is the number of unique memory node IDs referenced by the returned edges.
	NodeCount int `json:"node_count"`
	// EdgeCount is the number of edges returned.
	EdgeCount int `json:"edge_count"`
	// Edges contains the matching edges, up to the requested limit.
	Edges []GraphEdge `json:"edges"`
}

// KgPathResponse is returned by GET /v1/knowledge/path (KG-2).
type KgPathResponse struct {
	// AgentID is the agent whose graph was traversed.
	AgentID string `json:"agent_id"`
	// FromID is the source memory ID.
	FromID string `json:"from_id"`
	// ToID is the target memory ID.
	ToID string `json:"to_id"`
	// HopCount is the number of edges in the shortest path (0 if source == target).
	HopCount int `json:"hop_count"`
	// Path is the ordered list of memory IDs from source to target (inclusive).
	Path []string `json:"path"`
}

// KgExportResponse is returned by GET /v1/knowledge/export with format=json (KG-2).
type KgExportResponse struct {
	// AgentID is the agent whose graph was exported.
	AgentID string `json:"agent_id"`
	// Format is the export format used ("json" when this struct is returned).
	Format string `json:"format"`
	// NodeCount is the total number of unique memory node IDs in the export.
	NodeCount int `json:"node_count"`
	// EdgeCount is the total number of edges in the export.
	EdgeCount int `json:"edge_count"`
	// Edges contains all graph edges for the agent.
	Edges []GraphEdge `json:"edges"`
}

// ===========================================================================
// COG-1: Cognitive Memory Lifecycle — per-namespace memory policy
// ===========================================================================

// MemoryPolicy is the per-namespace memory lifecycle policy (COG-1).
//
// Controls type-specific TTLs, decay curves, and spaced repetition behaviour.
// All fields are optional (pointer types); nil values use the server-side
// COG-1 defaults.  Only set the fields you want to override.
//
// Used by Client.GetMemoryPolicy and Client.SetMemoryPolicy.
type MemoryPolicy struct {
	// Differential TTLs -------------------------------------------------------

	// WorkingTTLSeconds is the default TTL for working memories in seconds
	// (server default: 14 400 = 4 h).
	WorkingTTLSeconds *int64 `json:"working_ttl_seconds,omitempty"`
	// EpisodicTTLSeconds is the default TTL for episodic memories in seconds
	// (server default: 2 592 000 = 30 d).
	EpisodicTTLSeconds *int64 `json:"episodic_ttl_seconds,omitempty"`
	// SemanticTTLSeconds is the default TTL for semantic memories in seconds
	// (server default: 31 536 000 = 365 d).
	SemanticTTLSeconds *int64 `json:"semantic_ttl_seconds,omitempty"`
	// ProceduralTTLSeconds is the default TTL for procedural memories in
	// seconds (server default: 63 072 000 = 730 d).
	ProceduralTTLSeconds *int64 `json:"procedural_ttl_seconds,omitempty"`

	// Decay curves ------------------------------------------------------------

	// WorkingDecay is the decay strategy for working memories
	// (server default: "exponential").
	WorkingDecay *string `json:"working_decay,omitempty"`
	// EpisodicDecay is the decay strategy for episodic memories
	// (server default: "power_law").
	EpisodicDecay *string `json:"episodic_decay,omitempty"`
	// SemanticDecay is the decay strategy for semantic memories
	// (server default: "logarithmic").
	SemanticDecay *string `json:"semantic_decay,omitempty"`
	// ProceduralDecay is the decay strategy for procedural memories
	// (server default: "flat" — no decay).
	ProceduralDecay *string `json:"procedural_decay,omitempty"`

	// Spaced repetition -------------------------------------------------------

	// SpacedRepetitionFactor is the TTL extension multiplier per recall hit.
	// Extension = access_count × factor × base_interval_seconds.
	// Set to 0 to disable. (server default: 1.0)
	SpacedRepetitionFactor *float64 `json:"spaced_repetition_factor,omitempty"`
	// SpacedRepetitionBaseIntervalSeconds is the base interval in seconds for
	// spaced repetition TTL extension (server default: 86 400 = 1 d).
	SpacedRepetitionBaseIntervalSeconds *int64 `json:"spaced_repetition_base_interval_seconds,omitempty"`

	// Proactive consolidation (COG-3) -----------------------------------------

	// ConsolidationEnabled enables background DBSCAN deduplication for this
	// namespace. When true the server merges semantically near-duplicate
	// memories every ConsolidationIntervalHours hours. (server default: false)
	ConsolidationEnabled *bool `json:"consolidation_enabled,omitempty"`
	// ConsolidationThreshold is the DBSCAN epsilon — cosine-similarity
	// threshold to consider two memories duplicates. Higher values only merge
	// very close neighbours. (server default: 0.92)
	ConsolidationThreshold *float64 `json:"consolidation_threshold,omitempty"`
	// ConsolidationIntervalHours is how often (in hours) the background
	// consolidation job runs for this namespace. (server default: 24)
	ConsolidationIntervalHours *uint32 `json:"consolidation_interval_hours,omitempty"`
	// ConsolidatedCount is the lifetime count of memories merged by the
	// consolidation engine. Read-only — the server manages this field; any
	// value sent via SetMemoryPolicy is silently ignored.
	ConsolidatedCount *uint64 `json:"consolidated_count,omitempty"`

	// Per-namespace rate limiting (SEC-5) -------------------------------------

	// RateLimitEnabled enables per-namespace store/recall rate limiting.
	// (server default: false)
	RateLimitEnabled *bool `json:"rate_limit_enabled,omitempty"`
	// RateLimitStoresPerMinute sets the max store operations per minute for
	// this namespace. nil = unlimited (server default).
	RateLimitStoresPerMinute *uint32 `json:"rate_limit_stores_per_minute,omitempty"`
	// RateLimitRecallsPerMinute sets the max recall operations per minute for
	// this namespace. nil = unlimited (server default).
	RateLimitRecallsPerMinute *uint32 `json:"rate_limit_recalls_per_minute,omitempty"`

	// Store-time deduplication (CE-10) -----------------------------------------

	// DedupOnStore enables similarity deduplication at store time (CE-10).
	// When true the server computes a similarity check before persisting a new
	// memory and drops it if a near-duplicate already exists (threshold
	// controlled by DedupThreshold). (server default: false)
	DedupOnStore *bool `json:"dedup_on_store,omitempty"`
	// DedupThreshold is the cosine-similarity threshold for store-time
	// deduplication (server default: 0.92). Memories with similarity ≥ this
	// value are considered duplicates and the incoming memory is dropped. Only
	// active when DedupOnStore is true.
	DedupThreshold *float32 `json:"dedup_threshold,omitempty"`
}

// ===========================================================================
// Product KPIs (OBS-2)
// ===========================================================================

// KpiSnapshot is a point-in-time product KPI snapshot returned by GET /v1/kpis (OBS-2).
//
// All latency values are in milliseconds. Rate/percentage values are in the
// range 0.0–100.0. Integer counts are unsigned.
//
// Requires Admin scope.
type KpiSnapshot struct {
	// RecallLatencyP50Ms is the median recall latency across all namespaces
	// over the last minute (ms).
	RecallLatencyP50Ms float64 `json:"recall_latency_p50_ms"`
	// RecallLatencyP99Ms is the 99th-percentile recall latency across all
	// namespaces over the last minute (ms).
	RecallLatencyP99Ms float64 `json:"recall_latency_p99_ms"`
	// StoreLatencyP50Ms is the median store latency across all namespaces
	// over the last minute (ms).
	StoreLatencyP50Ms float64 `json:"store_latency_p50_ms"`
	// ApiErrorRate5xxPct is the 5xx error rate as a percentage of total API
	// requests over the last minute.
	ApiErrorRate5xxPct float64 `json:"api_error_rate_5xx_pct"`
	// ActiveAgentsCount is the number of distinct agent identifiers that stored
	// or recalled a memory in the last 24 hours.
	ActiveAgentsCount uint64 `json:"active_agents_count"`
	// SessionCountWeek is the total sessions created in the rolling 7-day window.
	SessionCountWeek uint64 `json:"session_count_week"`
	// CrossAgentNetworkNodeCount is the current number of nodes in the
	// cross-agent knowledge graph.
	CrossAgentNetworkNodeCount uint64 `json:"cross_agent_network_node_count"`
	// MemoryRetention7dPct is the percentage of memories created 7 days ago
	// that are still active (not decayed or deleted).
	MemoryRetention7dPct float64 `json:"memory_retention_7d_pct"`
}

// ===========================================================================
// Wake-Up Types (DAK-1690)
// ===========================================================================

// WakeUpOptions contains optional parameters for GetWakeUpContext.
type WakeUpOptions struct {
	// TopN is the maximum number of memories to return (default 20, max 100).
	TopN *int
	// MinImportance filters out memories below this importance threshold (default 0.0).
	MinImportance *float32
}

// WakeUpResponse is returned by GET /v1/agents/{agent_id}/wake-up (DAK-1690).
//
// Contains top-N memories ranked by importance × exp(-ln2 × age / 14d) for
// fast agent start-up context loading. No embedding inference — served from
// the metadata index for sub-millisecond latency.
//
// Requires Read scope on the agent namespace.
type WakeUpResponse struct {
	// AgentID is the agent whose memories are returned.
	AgentID string `json:"agent_id"`
	// Memories are the top-N memories ranked by recency-weighted importance.
	Memories []Memory `json:"memories"`
	// TotalAvailable is the total number of memories available before the
	// top_n cap was applied.
	TotalAvailable int64 `json:"total_available"`
}

// CompressResponse is returned by POST /v1/agents/{agent_id}/compress (CE-12).
//
// Contains compression statistics for the agent's memory namespace after the
// server runs the compression pass.
type CompressResponse struct {
	// AgentID is the agent whose namespace was compressed.
	AgentID string `json:"agent_id"`
	// MemoriesBefore is the number of memories before compression.
	MemoriesBefore int64 `json:"memories_before"`
	// MemoriesAfter is the number of memories after compression.
	MemoriesAfter int64 `json:"memories_after"`
	// RemovedCount is the number of memories removed during compression.
	RemovedCount int64 `json:"removed_count"`
	// DurationMs is the wall-clock duration of the compression pass in
	// milliseconds. May be zero if the server does not report it.
	DurationMs float64 `json:"duration_ms,omitempty"`
}

// FulltextReindexNamespaceResult is the per-namespace breakdown from
// POST /admin/fulltext/reindex (CE-54).
type FulltextReindexNamespaceResult struct {
	// Namespace that was scanned.
	Namespace string `json:"namespace"`
	// VectorsScanned is the total number of vectors examined.
	VectorsScanned int `json:"vectors_scanned"`
	// NewlyIndexed is the number of memories added to the BM25 index.
	NewlyIndexed int `json:"newly_indexed"`
	// AlreadyIndexed is the number of memories already in the BM25 index.
	AlreadyIndexed int `json:"already_indexed"`
	// ParseFailures is the number of memories that could not be parsed.
	ParseFailures int `json:"parse_failures"`
}

// FulltextReindexResponse is returned by POST /admin/fulltext/reindex (CE-54).
//
// Returned by [Client.AdminFulltextReindex].
type FulltextReindexResponse struct {
	// NamespacesProcessed is the number of namespaces scanned.
	NamespacesProcessed int `json:"namespaces_processed"`
	// TotalIndexed is the total memories newly added to BM25 across all namespaces.
	TotalIndexed int `json:"total_indexed"`
	// TotalSkipped is the total memories already in the BM25 index (skipped).
	TotalSkipped int `json:"total_skipped"`
	// Details is the per-namespace breakdown.
	Details []FulltextReindexNamespaceResult `json:"details"`
}

// ===========================================================================
// Engine Parity — Health Probes, Vector Bulk Ops, Agent Consolidation
// ===========================================================================

// ReadinessResponse is returned by GET /health/ready.
type ReadinessResponse struct {
	Ready   bool                               `json:"ready"`
	Version string                             `json:"version"`
	Checks  map[string]ReadinessCheckComponent `json:"checks"`
}

// ReadinessCheckComponent is one component in a readiness check.
type ReadinessCheckComponent struct {
	Status  string `json:"status"`
	Message string `json:"message,omitempty"`
}

// LivenessResponse is returned by GET /health/live.
type LivenessResponse struct {
	Alive         bool   `json:"alive"`
	Version       string `json:"version"`
	UptimeSeconds int64  `json:"uptime_seconds"`
}

// BulkUpdateResponse is returned by POST /v1/namespaces/{ns}/vectors/bulk-update.
type BulkUpdateResponse struct {
	Updated int      `json:"updated"`
	Failed  int      `json:"failed"`
	Errors  []string `json:"errors"`
}

// BulkDeleteResponse is returned by POST /v1/namespaces/{ns}/vectors/bulk-delete.
type BulkDeleteResponse struct {
	Deleted int      `json:"deleted"`
	Failed  int      `json:"failed"`
	Errors  []string `json:"errors"`
}

// CountVectorsResponse is returned by POST /v1/namespaces/{ns}/vectors/count.
type CountVectorsResponse struct {
	Count     int    `json:"count"`
	Namespace string `json:"namespace"`
}

// AgentConsolidateResponse is returned by POST /v1/agents/{agent_id}/consolidate.
type AgentConsolidateResponse struct {
	AgentID            string   `json:"agent_id"`
	MemoriesScanned    int      `json:"memories_scanned"`
	ClustersFound      int      `json:"clusters_found"`
	MemoriesDeprecated int      `json:"memories_deprecated"`
	AnchorIDs          []string `json:"anchor_ids"`
	DeprecatedIDs      []string `json:"deprecated_ids"`
	Skipped            *bool    `json:"skipped,omitempty"`
	Reason             string   `json:"reason,omitempty"`
}

// AgentConsolidationLogEntry is one entry in the agent consolidation log.
type AgentConsolidationLogEntry struct {
	Timestamp          int64    `json:"timestamp"`
	ClustersFound      int      `json:"clusters_found"`
	MemoriesDeprecated int      `json:"memories_deprecated"`
	AnchorIDs          []string `json:"anchor_ids"`
	DeprecatedIDs      []string `json:"deprecated_ids"`
}

// ConsolidationConfigPatch is the request for PATCH /v1/agents/{agent_id}/consolidation/config.
type ConsolidationConfigPatch struct {
	Enabled             *bool    `json:"enabled,omitempty"`
	Epsilon             *float64 `json:"epsilon,omitempty"`
	MinSamples          *int     `json:"min_samples,omitempty"`
	SoftDeprecationDays *int     `json:"soft_deprecation_days,omitempty"`
}

// AgentConsolidationConfig is the response from consolidation config endpoints.
type AgentConsolidationConfig struct {
	Enabled             bool    `json:"enabled"`
	Epsilon             float64 `json:"epsilon"`
	MinSamples          int     `json:"min_samples"`
	SoftDeprecationDays int     `json:"soft_deprecation_days"`
}

// NamespaceEntityConfig is returned by GET /v1/namespaces/{ns}/config.
type NamespaceEntityConfig struct {
	Namespace       string   `json:"namespace"`
	ExtractEntities bool     `json:"extract_entities"`
	EntityTypes     []string `json:"entity_types"`
}

// ExtractorConfigResponse is returned by GET /v1/namespaces/{ns}/extractor.
type ExtractorConfigResponse struct {
	Provider string `json:"provider"`
	Model    string `json:"model,omitempty"`
	BaseURL  string `json:"base_url,omitempty"`
}
