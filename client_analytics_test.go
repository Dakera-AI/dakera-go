package dakera

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ===========================================================================
// Vector Bulk Operations
// ===========================================================================

func TestBulkUpdateVectors(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "POST", r.Method)
		assert.Equal(t, "/v1/namespaces/test-ns/vectors/bulk-update", r.URL.Path)
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"updated": 50,
			"failed":  0,
			"errors":  []string{},
		})
	}))
	defer server.Close()
	client := NewClient(server.URL)
	result, err := client.BulkUpdateVectors(context.Background(), "test-ns",
		map[string]interface{}{"category": map[string]interface{}{"$eq": "docs"}},
		map[string]interface{}{"status": "reviewed"},
	)
	require.NoError(t, err)
	assert.Equal(t, 50, result.Updated)
	assert.Equal(t, 0, result.Failed)
}

func TestBulkDeleteVectors(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "POST", r.Method)
		assert.Equal(t, "/v1/namespaces/test-ns/vectors/bulk-delete", r.URL.Path)
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"deleted": 25,
			"failed":  0,
			"errors":  []string{},
		})
	}))
	defer server.Close()
	client := NewClient(server.URL)
	result, err := client.BulkDeleteVectors(context.Background(), "test-ns",
		map[string]interface{}{"expired": map[string]interface{}{"$eq": true}},
	)
	require.NoError(t, err)
	assert.Equal(t, 25, result.Deleted)
}

func TestCountVectors(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "POST", r.Method)
		assert.Equal(t, "/v1/namespaces/test-ns/vectors/count", r.URL.Path)
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"count":     1500,
			"namespace": "test-ns",
		})
	}))
	defer server.Close()
	client := NewClient(server.URL)
	result, err := client.CountVectors(context.Background(), "test-ns", nil)
	require.NoError(t, err)
	assert.Equal(t, 1500, result.Count)
	assert.Equal(t, "test-ns", result.Namespace)
}

func TestCountVectors_WithFilter(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "POST", r.Method)
		var body map[string]interface{}
		json.NewDecoder(r.Body).Decode(&body)
		assert.NotNil(t, body["filter"])
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{"count": 300, "namespace": "test-ns"})
	}))
	defer server.Close()
	client := NewClient(server.URL)
	result, err := client.CountVectors(context.Background(), "test-ns", map[string]interface{}{"type": "document"})
	require.NoError(t, err)
	assert.Equal(t, 300, result.Count)
}

// ===========================================================================
// Advanced Search Operations
// ===========================================================================

func TestMultiVectorSearch(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "POST", r.Method)
		assert.Equal(t, "/v1/namespaces/test-ns/multi-vector", r.URL.Path)
		w.Header().Set("Content-Type", "application/json")
		searchTime := int64(15)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"results": []map[string]interface{}{
				{"id": "vec-1", "score": 0.95},
				{"id": "vec-2", "score": 0.88},
			},
			"search_time_ms": searchTime,
			"strategy":       "mmr",
		})
	}))
	defer server.Close()
	client := NewClient(server.URL)
	result, err := client.MultiVectorSearch(context.Background(), "test-ns", MultiVectorSearchRequest{
		Positive: [][]float32{{0.1, 0.2, 0.3}},
		Negative: [][]float32{{0.9, 0.8, 0.7}},
		TopK:     5,
	})
	require.NoError(t, err)
	assert.Len(t, result.Results, 2)
	assert.Equal(t, "mmr", result.Strategy)
}

func TestUnifiedQuery(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "POST", r.Method)
		assert.Equal(t, "/v1/namespaces/test-ns/unified-query", r.URL.Path)
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"results": []map[string]interface{}{
				{"id": "doc-1", "score": 0.92, "content": "hello world"},
			},
			"search_time_ms": 10,
			"fusion_method":  "rrf",
		})
	}))
	defer server.Close()
	client := NewClient(server.URL)
	result, err := client.UnifiedQuery(context.Background(), "test-ns", UnifiedQueryRequest{
		Text: "hello",
		TopK: 5,
	})
	require.NoError(t, err)
	assert.Len(t, result.Results, 1)
	assert.Equal(t, "rrf", result.FusionMethod)
}

func TestAggregate(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "POST", r.Method)
		assert.Equal(t, "/v1/namespaces/test-ns/aggregate", r.URL.Path)
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"groups": []map[string]interface{}{
				{"key": "category_a", "count": 10},
				{"key": "category_b", "count": 5},
			},
			"total_groups":   2,
			"search_time_ms": 8,
		})
	}))
	defer server.Close()
	client := NewClient(server.URL)
	result, err := client.Aggregate(context.Background(), "test-ns", AggregationRequest{
		GroupBy: "category",
		Metrics: []string{"count"},
	})
	require.NoError(t, err)
	assert.Equal(t, 2, result.TotalGroups)
	assert.Len(t, result.Groups, 2)
}

func TestExportVectors(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "POST", r.Method)
		assert.Equal(t, "/v1/namespaces/test-ns/export", r.URL.Path)
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"vectors": []map[string]interface{}{
				{"id": "vec-1", "metadata": map[string]interface{}{"type": "doc"}},
				{"id": "vec-2", "metadata": map[string]interface{}{"type": "doc"}},
			},
			"next_cursor": "cursor-abc",
			"has_more":    true,
		})
	}))
	defer server.Close()
	client := NewClient(server.URL)
	limit := 2
	result, err := client.ExportVectors(context.Background(), "test-ns", ExportRequest{Limit: &limit})
	require.NoError(t, err)
	assert.Len(t, result.Vectors, 2)
	assert.True(t, result.HasMore)
	assert.Equal(t, "cursor-abc", result.NextCursor)
}

func TestExplainQuery(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "POST", r.Method)
		assert.Equal(t, "/v1/namespaces/test-ns/explain", r.URL.Path)
		w.Header().Set("Content-Type", "application/json")
		totalTime := 5.2
		scanned := int64(1000)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"plan":            map[string]interface{}{"type": "hnsw_search"},
			"steps":           []map[string]interface{}{{"step": "index_scan", "duration_ms": 3.1}},
			"total_time_ms":   totalTime,
			"index_type":      "hnsw",
			"vectors_scanned": scanned,
		})
	}))
	defer server.Close()
	client := NewClient(server.URL)
	result, err := client.ExplainQuery(context.Background(), "test-ns", QueryExplainRequest{
		Vector: []float32{0.1, 0.2, 0.3},
		TopK:   10,
	})
	require.NoError(t, err)
	assert.Equal(t, "hnsw", result.IndexType)
	assert.NotNil(t, result.TotalTimeMs)
}

func TestUpsertColumns(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "POST", r.Method)
		assert.Equal(t, "/v1/namespaces/test-ns/upsert-columns", r.URL.Path)
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{"upsertedCount": 3})
	}))
	defer server.Close()
	client := NewClient(server.URL)
	result, err := client.UpsertColumns(context.Background(), "test-ns", ColumnUpsertRequest{
		IDs:     []string{"v1", "v2", "v3"},
		Vectors: [][]float32{{0.1, 0.2}, {0.3, 0.4}, {0.5, 0.6}},
	})
	require.NoError(t, err)
	assert.Equal(t, 3, result.UpsertedCount)
}

func TestWarmCache(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "POST", r.Method)
		assert.Equal(t, "/v1/namespaces/test-ns/cache/warm", r.URL.Path)
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"status":        "ok",
			"entries_warmed": 50,
			"time_taken_ms": 120,
		})
	}))
	defer server.Close()
	client := NewClient(server.URL)
	result, err := client.WarmCache(context.Background(), "test-ns", WarmCacheRequest{
		VectorIDs: []string{"v1", "v2"},
		Priority:  "high",
	})
	require.NoError(t, err)
	assert.Equal(t, "ok", result.Status)
	assert.Equal(t, 50, result.EntriesWarmed)
}

// ===========================================================================
// Text-Based Inference Operations
// ===========================================================================

func TestUpsertText(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "POST", r.Method)
		assert.Equal(t, "/v1/namespaces/test-ns/upsert-text", r.URL.Path)
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"upserted_count":   2,
			"tokens_processed": 150,
			"model":            "minilm",
			"embedding_time_ms": 25,
		})
	}))
	defer server.Close()
	client := NewClient(server.URL)
	result, err := client.UpsertText(context.Background(), "test-ns", []TextDocument{
		{ID: "doc-1", Text: "Hello world"},
		{ID: "doc-2", Text: "Goodbye world"},
	}, nil)
	require.NoError(t, err)
	assert.Equal(t, 2, result.UpsertedCount)
	assert.Equal(t, EmbeddingModel("minilm"), result.Model)
}

func TestQueryText(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "POST", r.Method)
		assert.Equal(t, "/v1/namespaces/test-ns/query-text", r.URL.Path)
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"results": []map[string]interface{}{
				{"id": "doc-1", "score": 0.92, "text": "Hello world"},
			},
			"model":            "minilm",
			"embedding_time_ms": 5,
			"search_time_ms":   3,
		})
	}))
	defer server.Close()
	client := NewClient(server.URL)
	result, err := client.QueryText(context.Background(), "test-ns", "greeting", &TextQueryOptions{TopK: 5, IncludeText: true})
	require.NoError(t, err)
	assert.Len(t, result.Results, 1)
	assert.Equal(t, "Hello world", result.Results[0].Text)
}

func TestBatchQueryText(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "POST", r.Method)
		assert.Equal(t, "/v1/namespaces/test-ns/batch-query-text", r.URL.Path)
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"results": [][]map[string]interface{}{
				{{"id": "doc-1", "score": 0.9}},
				{{"id": "doc-2", "score": 0.85}},
			},
			"model":            "minilm",
			"embedding_time_ms": 10,
			"search_time_ms":   6,
		})
	}))
	defer server.Close()
	client := NewClient(server.URL)
	result, err := client.BatchQueryText(context.Background(), "test-ns", []string{"hello", "goodbye"}, nil)
	require.NoError(t, err)
	assert.Len(t, result.Results, 2)
}

// ===========================================================================
// EmbeddingModel — ModernBERT wire-value tests (DAK-7098/7102)
// ===========================================================================

func TestEmbeddingModelModernBertEmbedBaseWireValue(t *testing.T) {
	assert.Equal(t, EmbeddingModel("modernbert-embed-base"), EmbeddingModelModernBertEmbedBase)
}

func TestEmbeddingModelGteModernBertBaseWireValue(t *testing.T) {
	assert.Equal(t, EmbeddingModel("gte-modernbert-base"), EmbeddingModelGteModernBertBase)
}

func TestUpsertTextWithModernBertModel(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body map[string]interface{}
		require.NoError(t, json.NewDecoder(r.Body).Decode(&body))
		assert.Equal(t, "modernbert-embed-base", body["model"])
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"upserted_count":    1,
			"tokens_processed":  42,
			"model":             "modernbert-embed-base",
			"embedding_time_ms": 12,
		})
	}))
	defer server.Close()
	client := NewClient(server.URL)
	result, err := client.UpsertText(context.Background(), "test-ns", []TextDocument{
		{ID: "d1", Text: "Hello ModernBERT"},
	}, &TextUpsertOptions{Model: EmbeddingModelModernBertEmbedBase})
	require.NoError(t, err)
	assert.Equal(t, EmbeddingModelModernBertEmbedBase, result.Model)
}

func TestQueryTextDeserializesGteModernBertModel(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"results":           []map[string]interface{}{{"id": "d1", "score": 0.95}},
			"model":             "gte-modernbert-base",
			"embedding_time_ms": 8,
			"search_time_ms":    2,
		})
	}))
	defer server.Close()
	client := NewClient(server.URL)
	result, err := client.QueryText(context.Background(), "test-ns", "hello", &TextQueryOptions{TopK: 1})
	require.NoError(t, err)
	assert.Equal(t, EmbeddingModelGteModernBertBase, result.Model)
}

// ===========================================================================
// Analytics Operations
// ===========================================================================

func TestAnalyticsOverview(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "GET", r.Method)
		assert.Contains(t, r.URL.Path, "/v1/analytics/overview")
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"total_queries":       10000,
			"avg_latency_ms":      5.2,
			"p95_latency_ms":      12.5,
			"p99_latency_ms":      25.0,
			"queries_per_second":  150.0,
			"error_rate":          0.001,
			"cache_hit_rate":      0.85,
			"storage_used_bytes":  1073741824,
			"total_vectors":       500000,
			"total_namespaces":    10,
			"uptime_seconds":      86400,
		})
	}))
	defer server.Close()
	client := NewClient(server.URL)
	result, err := client.AnalyticsOverview(context.Background(), &AnalyticsOptions{Period: "1h"})
	require.NoError(t, err)
	assert.Equal(t, uint64(10000), result.TotalQueries)
	assert.InDelta(t, 5.2, result.AvgLatencyMs, 0.1)
}

func TestAnalyticsLatency(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "GET", r.Method)
		assert.Contains(t, r.URL.Path, "/v1/analytics/latency")
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"period":  "1h",
			"avg_ms":  4.5,
			"p50_ms":  3.0,
			"p95_ms":  10.0,
			"p99_ms":  20.0,
			"max_ms":  150.0,
		})
	}))
	defer server.Close()
	client := NewClient(server.URL)
	result, err := client.AnalyticsLatency(context.Background(), &AnalyticsOptions{Period: "1h"})
	require.NoError(t, err)
	assert.Equal(t, "1h", result.Period)
	assert.InDelta(t, 4.5, result.AvgMs, 0.1)
}

func TestAnalyticsThroughput(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "GET", r.Method)
		assert.Contains(t, r.URL.Path, "/v1/analytics/throughput")
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"period":                "1h",
			"total_operations":      50000,
			"operations_per_second": 13.9,
		})
	}))
	defer server.Close()
	client := NewClient(server.URL)
	result, err := client.AnalyticsThroughput(context.Background(), nil)
	require.NoError(t, err)
	assert.Equal(t, uint64(50000), result.TotalOperations)
}

func TestAnalyticsStorage(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "GET", r.Method)
		assert.Contains(t, r.URL.Path, "/v1/analytics/storage")
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"total_bytes": 2147483648,
			"index_bytes": 536870912,
			"data_bytes":  1610612736,
		})
	}))
	defer server.Close()
	client := NewClient(server.URL)
	result, err := client.AnalyticsStorage(context.Background(), "test-ns")
	require.NoError(t, err)
	assert.Equal(t, uint64(2147483648), result.TotalBytes)
}

func TestGetKpis(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "GET", r.Method)
		assert.Equal(t, "/v1/kpis", r.URL.Path)
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"recall_latency_p50_ms":            2.5,
			"recall_latency_p99_ms":            15.0,
			"store_latency_p50_ms":             3.0,
			"api_error_rate_5xx_pct":           0.01,
			"active_agents_count":              25,
			"session_count_week":               150,
			"cross_agent_network_node_count":   500,
			"memory_retention_7d_pct":          92.5,
		})
	}))
	defer server.Close()
	client := NewClient(server.URL)
	result, err := client.GetKpis(context.Background())
	require.NoError(t, err)
	assert.InDelta(t, 2.5, result.RecallLatencyP50Ms, 0.1)
	assert.Equal(t, uint64(25), result.ActiveAgentsCount)
}

// ===========================================================================
// Knowledge Graph Operations
// ===========================================================================

func TestKnowledgeGraph(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "POST", r.Method)
		assert.Equal(t, "/v1/knowledge/graph", r.URL.Path)
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"nodes": []map[string]interface{}{
				{"id": "mem-1", "content": "node1", "memory_type": "episodic"},
				{"id": "mem-2", "content": "node2", "memory_type": "semantic"},
			},
			"edges": []map[string]interface{}{
				{"source": "mem-1", "target": "mem-2", "similarity": 0.9},
			},
		})
	}))
	defer server.Close()
	client := NewClient(server.URL)
	result, err := client.KnowledgeGraph(context.Background(), KnowledgeGraphRequest{AgentID: "agent-1"})
	require.NoError(t, err)
	assert.Len(t, result.Nodes, 2)
	assert.Len(t, result.Edges, 1)
}

func TestFullKnowledgeGraph(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "POST", r.Method)
		assert.Equal(t, "/v1/knowledge/graph/full", r.URL.Path)
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"nodes":    []map[string]interface{}{{"id": "n1", "content": "x"}},
			"edges":    []map[string]interface{}{},
			"clusters": [][]string{{"n1"}},
		})
	}))
	defer server.Close()
	client := NewClient(server.URL)
	result, err := client.FullKnowledgeGraph(context.Background(), FullKnowledgeGraphRequest{AgentID: "agent-1"})
	require.NoError(t, err)
	assert.Len(t, result.Nodes, 1)
}

func TestSummarize(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "POST", r.Method)
		assert.Equal(t, "/v1/knowledge/summarize", r.URL.Path)
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"summary":       "User prefers coffee and works at ACME",
			"source_count":  5,
			"new_memory_id": "mem-summary-1",
		})
	}))
	defer server.Close()
	client := NewClient(server.URL)
	result, err := client.Summarize(context.Background(), SummarizeRequest{AgentID: "agent-1"})
	require.NoError(t, err)
	assert.Contains(t, result.Summary, "coffee")
	assert.Equal(t, 5, result.SourceCount)
}

func TestDeduplicate(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "POST", r.Method)
		assert.Equal(t, "/v1/knowledge/deduplicate", r.URL.Path)
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"duplicates_found": 3,
			"removed_count":    2,
			"groups":           [][]string{{"mem-1", "mem-2"}, {"mem-3", "mem-4", "mem-5"}},
		})
	}))
	defer server.Close()
	client := NewClient(server.URL)
	result, err := client.Deduplicate(context.Background(), DeduplicateRequest{AgentID: "agent-1"})
	require.NoError(t, err)
	assert.Equal(t, 3, result.DuplicatesFound)
	assert.Equal(t, 2, result.RemovedCount)
}

func TestCrossAgentNetwork(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "POST", r.Method)
		assert.Equal(t, "/v1/knowledge/network/cross-agent", r.URL.Path)
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"agents":     []map[string]interface{}{{"agent_id": "a1", "memory_count": 10, "avg_importance": 0.7}},
			"nodes":      []map[string]interface{}{},
			"edges":      []map[string]interface{}{},
			"stats":      map[string]interface{}{"total_agents": 2, "total_nodes": 20, "total_cross_edges": 5, "density": 0.1},
			"node_count": 20,
		})
	}))
	defer server.Close()
	client := NewClient(server.URL)
	result, err := client.CrossAgentNetwork(context.Background(), CrossAgentNetworkRequest{MinSimilarity: 0.5})
	require.NoError(t, err)
	assert.Equal(t, 20, result.NodeCount)
}

func TestKnowledgeQuery(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "GET", r.Method)
		assert.Contains(t, r.URL.Path, "/v1/knowledge/query")
		assert.Equal(t, "agent-1", r.URL.Query().Get("agent_id"))
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"agent_id":   "agent-1",
			"node_count": 5,
			"edge_count": 3,
			"edges":      []map[string]interface{}{},
		})
	}))
	defer server.Close()
	client := NewClient(server.URL)
	result, err := client.KnowledgeQuery(context.Background(), "agent-1", "", "related_to", 0.5, 2, 50)
	require.NoError(t, err)
	assert.Equal(t, "agent-1", result.AgentID)
	assert.Equal(t, 5, result.NodeCount)
}

func TestKnowledgePath(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "GET", r.Method)
		assert.Contains(t, r.URL.Path, "/v1/knowledge/path")
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"agent_id":  "agent-1",
			"from_id":   "mem-1",
			"to_id":     "mem-5",
			"hop_count": 2,
			"path":      []string{"mem-1", "mem-3", "mem-5"},
		})
	}))
	defer server.Close()
	client := NewClient(server.URL)
	result, err := client.KnowledgePath(context.Background(), "agent-1", "mem-1", "mem-5")
	require.NoError(t, err)
	assert.Equal(t, 2, result.HopCount)
	assert.Len(t, result.Path, 3)
}

func TestKnowledgeExport(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "GET", r.Method)
		assert.Contains(t, r.URL.Path, "/v1/knowledge/export")
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"agent_id":   "agent-1",
			"format":     "json",
			"node_count": 50,
			"edge_count": 120,
			"edges":      []map[string]interface{}{},
		})
	}))
	defer server.Close()
	client := NewClient(server.URL)
	result, err := client.KnowledgeExport(context.Background(), "agent-1", "json")
	require.NoError(t, err)
	assert.Equal(t, "json", result.Format)
	assert.Equal(t, 50, result.NodeCount)
}

// ===========================================================================
// Error Tests
// ===========================================================================

func TestBulkUpdateVectors_Error(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]interface{}{"error": "server error"})
	}))
	defer server.Close()
	client := NewClientWithOptions(ClientOptions{BaseURL: server.URL, MaxRetries: 1})
	_, err := client.BulkUpdateVectors(context.Background(), "ns", map[string]interface{}{}, map[string]interface{}{})
	require.Error(t, err)
}

func TestMultiVectorSearch_Error(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]interface{}{"error": "missing positive vectors"})
	}))
	defer server.Close()
	client := NewClientWithOptions(ClientOptions{BaseURL: server.URL, MaxRetries: 1})
	_, err := client.MultiVectorSearch(context.Background(), "ns", MultiVectorSearchRequest{})
	require.Error(t, err)
}

func TestAnalyticsOverview_Error(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		json.NewEncoder(w).Encode(map[string]interface{}{"error": "unauthorized"})
	}))
	defer server.Close()
	client := NewClientWithOptions(ClientOptions{BaseURL: server.URL, MaxRetries: 1})
	_, err := client.AnalyticsOverview(context.Background(), nil)
	require.Error(t, err)
}
