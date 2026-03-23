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
	Name        string                 `json:"name"`
	VectorCount int64                  `json:"vectorCount"`
	Dimensions  int                    `json:"dimensions,omitempty"`
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
	Content  string                 `json:"content"`
	Metadata map[string]interface{} `json:"metadata,omitempty"`
}

// DocumentInput represents input for indexing a document.
type DocumentInput struct {
	ID       string                 `json:"id"`
	Content  string                 `json:"content"`
	Metadata map[string]interface{} `json:"metadata,omitempty"`
}

// FullTextSearchResult represents a full-text search result.
type FullTextSearchResult struct {
	ID       string                 `json:"id"`
	Score    float32                `json:"score"`
	Content  string                 `json:"content,omitempty"`
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
	TopK   int                    `json:"top_k,omitempty"`
	Alpha  float32                `json:"alpha,omitempty"`
	Filter map[string]interface{} `json:"filter,omitempty"`
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

// ===========================================================================
// Text-Based Inference Types (Auto-Embedding)
// ===========================================================================

// EmbeddingModel represents supported embedding models for text-based operations.
type EmbeddingModel string

const (
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
type StoreMemoryResponse struct {
	MemoryID string `json:"memory_id"`
	Status   string `json:"status"`
}

// Memory represents a stored memory.
type Memory struct {
	ID          string                 `json:"id"`
	Content     string                 `json:"content"`
	MemoryType  string                 `json:"memory_type"`
	Importance  float32                `json:"importance"`
	Metadata    map[string]interface{} `json:"metadata,omitempty"`
	CreatedAt   string                 `json:"created_at,omitempty"`
	UpdatedAt   string                 `json:"updated_at,omitempty"`
	AccessCount *int                   `json:"access_count,omitempty"`
}

// RecalledMemory represents a recalled memory with similarity score.
type RecalledMemory struct {
	ID         string                 `json:"id"`
	Content    string                 `json:"content"`
	MemoryType string                 `json:"memory_type"`
	Importance float32                `json:"importance"`
	Score      float32                `json:"score"`
	Metadata   map[string]interface{} `json:"metadata,omitempty"`
	CreatedAt  string                 `json:"created_at,omitempty"`
}

// RecallRequest represents a request to recall memories.
type RecallRequest struct {
	Query         string   `json:"query"`
	TopK          int      `json:"top_k,omitempty"`
	MemoryType    string   `json:"memory_type,omitempty"`
	MinImportance *float32 `json:"min_importance,omitempty"`
}

// UpdateMemoryRequest represents a request to update a memory.
type UpdateMemoryRequest struct {
	Content    *string                `json:"content,omitempty"`
	Metadata   map[string]interface{} `json:"metadata,omitempty"`
	MemoryType *string                `json:"memory_type,omitempty"`
}

// SearchMemoriesRequest represents a request to search memories.
type SearchMemoriesRequest struct {
	Query         string   `json:"query"`
	TopK          int      `json:"top_k,omitempty"`
	MemoryType    string   `json:"memory_type,omitempty"`
	MinImportance *float32 `json:"min_importance,omitempty"`
}

// UpdateImportanceRequest represents a request to update memory importance.
type UpdateImportanceRequest struct {
	MemoryIDs  []string `json:"memory_ids"`
	Importance float32  `json:"importance"`
}

// ConsolidateRequest represents a request to consolidate memories.
type ConsolidateRequest struct {
	MemoryType string   `json:"memory_type,omitempty"`
	Threshold  *float32 `json:"threshold,omitempty"`
	DryRun     bool     `json:"dry_run,omitempty"`
}

// ConsolidateResponse represents the response from consolidation.
type ConsolidateResponse struct {
	ConsolidatedCount int      `json:"consolidated_count"`
	RemovedCount      int      `json:"removed_count"`
	NewMemories       []string `json:"new_memories"`
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
	SessionID string                 `json:"session_id"`
	AgentID   string                 `json:"agent_id"`
	StartedAt string                 `json:"started_at,omitempty"`
	EndedAt   string                 `json:"ended_at,omitempty"`
	Metadata  map[string]interface{} `json:"metadata,omitempty"`
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

// ClusterStatus represents the cluster status response.
type ClusterStatus struct {
	Status  string `json:"status"`
	Nodes   int    `json:"nodes"`
	Healthy bool   `json:"healthy"`
	Version string `json:"version,omitempty"`
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
