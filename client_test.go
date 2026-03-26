package dakera

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewClient(t *testing.T) {
	client := NewClient("http://localhost:3000")
	assert.NotNil(t, client)
}

func TestNewClientWithOptions(t *testing.T) {
	client := NewClientWithOptions(ClientOptions{
		BaseURL:    "http://localhost:3000/",
		APIKey:     "test-key",
		MaxRetries: 5,
	})
	assert.NotNil(t, client)
	assert.Equal(t, "http://localhost:3000", client.baseURL)
	assert.Equal(t, "test-key", client.apiKey)
	assert.Equal(t, 5, client.retryConfig.MaxRetries)
}

func TestUpsert(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "POST", r.Method)
		assert.Equal(t, "/v1/namespaces/test-ns/vectors", r.URL.Path)
		assert.Equal(t, "application/json", r.Header.Get("Content-Type"))

		var body map[string]interface{}
		json.NewDecoder(r.Body).Decode(&body)
		vectors := body["vectors"].([]interface{})
		assert.Len(t, vectors, 2)

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"upsertedCount": 2,
		})
	}))
	defer server.Close()

	client := NewClient(server.URL)
	resp, err := client.Upsert(context.Background(), "test-ns", []VectorInput{
		{ID: "vec1", Values: []float32{0.1, 0.2, 0.3}},
		{ID: "vec2", Values: []float32{0.4, 0.5, 0.6}},
	})

	require.NoError(t, err)
	assert.Equal(t, 2, resp.UpsertedCount)
}

func TestQuery(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "POST", r.Method)
		assert.Equal(t, "/v1/namespaces/test-ns/query", r.URL.Path)

		var body map[string]interface{}
		json.NewDecoder(r.Body).Decode(&body)
		assert.NotNil(t, body["vector"])

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"results": []map[string]interface{}{
				{"id": "vec1", "score": 0.95, "metadata": map[string]interface{}{"label": "a"}},
				{"id": "vec2", "score": 0.85},
			},
			"totalSearched": 100,
		})
	}))
	defer server.Close()

	client := NewClient(server.URL)
	resp, err := client.Query(context.Background(), "test-ns", []float32{0.1, 0.2, 0.3}, &QueryOptions{
		TopK: 10,
	})

	require.NoError(t, err)
	assert.Len(t, resp.Results, 2)
	assert.Equal(t, "vec1", resp.Results[0].ID)
	assert.Equal(t, float32(0.95), resp.Results[0].Score)
}

func TestQueryWithFilter(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body map[string]interface{}
		json.NewDecoder(r.Body).Decode(&body)
		assert.NotNil(t, body["filter"])

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"results": []map[string]interface{}{},
		})
	}))
	defer server.Close()

	client := NewClient(server.URL)
	_, err := client.Query(context.Background(), "test-ns", []float32{0.1, 0.2, 0.3}, &QueryOptions{
		Filter: map[string]interface{}{
			"category": Eq("test"),
		},
	})

	require.NoError(t, err)
}

func TestDelete(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "POST", r.Method)
		assert.Equal(t, "/v1/namespaces/test-ns/delete", r.URL.Path)

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"deletedCount": 2,
		})
	}))
	defer server.Close()

	client := NewClient(server.URL)
	resp, err := client.Delete(context.Background(), "test-ns", DeleteOptions{
		IDs: []string{"vec1", "vec2"},
	})

	require.NoError(t, err)
	assert.Equal(t, 2, resp.DeletedCount)
}

func TestDeleteByFilter(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body map[string]interface{}
		json.NewDecoder(r.Body).Decode(&body)
		assert.NotNil(t, body["filter"])

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"deletedCount": 5,
		})
	}))
	defer server.Close()

	client := NewClient(server.URL)
	resp, err := client.Delete(context.Background(), "test-ns", DeleteOptions{
		Filter: map[string]interface{}{
			"status": Eq("obsolete"),
		},
	})

	require.NoError(t, err)
	assert.Equal(t, 5, resp.DeletedCount)
}

func TestFetch(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "POST", r.Method)
		assert.Equal(t, "/v1/namespaces/test-ns/fetch", r.URL.Path)

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"vectors": []map[string]interface{}{
				{"id": "vec1", "values": []float32{0.1, 0.2, 0.3}},
				{"id": "vec2", "values": []float32{0.4, 0.5, 0.6}},
			},
		})
	}))
	defer server.Close()

	client := NewClient(server.URL)
	vectors, err := client.Fetch(context.Background(), "test-ns", []string{"vec1", "vec2"}, nil)

	require.NoError(t, err)
	assert.Len(t, vectors, 2)
	assert.Equal(t, "vec1", vectors[0].ID)
}

func TestBatchQuery(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "POST", r.Method)
		assert.Equal(t, "/v1/namespaces/test-ns/batch-query", r.URL.Path)

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"results": []map[string]interface{}{
				{"results": []map[string]interface{}{{"id": "vec1", "score": 0.9}}},
				{"results": []map[string]interface{}{{"id": "vec2", "score": 0.8}}},
			},
		})
	}))
	defer server.Close()

	client := NewClient(server.URL)
	results, err := client.BatchQuery(context.Background(), "test-ns", []BatchQuerySpec{
		{Vector: []float32{0.1, 0.2, 0.3}, TopK: 1},
		{Vector: []float32{0.4, 0.5, 0.6}, TopK: 1},
	})

	require.NoError(t, err)
	assert.Len(t, results, 2)
}

func TestIndexDocuments(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "POST", r.Method)
		assert.Equal(t, "/v1/namespaces/test-ns/fulltext/index", r.URL.Path)

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"indexedCount": 2,
		})
	}))
	defer server.Close()

	client := NewClient(server.URL)
	resp, err := client.IndexDocuments(context.Background(), "test-ns", []DocumentInput{
		{ID: "doc1", Content: "Hello world"},
		{ID: "doc2", Content: "Goodbye world"},
	})

	require.NoError(t, err)
	assert.Equal(t, 2, resp.IndexedCount)
}

func TestFulltextSearch(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "POST", r.Method)
		assert.Equal(t, "/v1/namespaces/test-ns/fulltext/search", r.URL.Path)

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"results": []map[string]interface{}{
				{"id": "doc1", "score": 2.5, "content": "Hello world"},
			},
		})
	}))
	defer server.Close()

	client := NewClient(server.URL)
	results, err := client.FulltextSearch(context.Background(), "test-ns", "hello", nil)

	require.NoError(t, err)
	assert.Len(t, results, 1)
	assert.Equal(t, "doc1", results[0].ID)
}

func TestHybridSearch(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "POST", r.Method)
		assert.Equal(t, "/v1/namespaces/test-ns/hybrid", r.URL.Path)

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"results": []map[string]interface{}{
				{"id": "doc1", "score": 0.85, "vectorScore": 0.9, "textScore": 0.8},
			},
		})
	}))
	defer server.Close()

	client := NewClient(server.URL)
	results, err := client.HybridSearch(context.Background(), "test-ns", []float32{0.1, 0.2, 0.3}, "hello", &HybridSearchOptions{
		Alpha: 0.5,
	})

	require.NoError(t, err)
	assert.Len(t, results, 1)
	assert.Equal(t, float32(0.9), results[0].VectorScore)
	assert.Equal(t, float32(0.8), results[0].TextScore)
}

func TestHybridSearchBM25Only(t *testing.T) {
	var capturedBody []byte
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "POST", r.Method)
		assert.Equal(t, "/v1/namespaces/test-ns/hybrid", r.URL.Path)
		capturedBody, _ = io.ReadAll(r.Body)
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"results": []map[string]interface{}{
				{"id": "doc2", "score": 0.75},
			},
		})
	}))
	defer server.Close()

	client := NewClient(server.URL)
	results, err := client.HybridSearch(context.Background(), "test-ns", nil, "hello", nil)

	require.NoError(t, err)
	assert.Len(t, results, 1)
	// vector must not be present in the request body
	var body map[string]interface{}
	require.NoError(t, json.Unmarshal(capturedBody, &body))
	_, hasVector := body["vector"]
	assert.False(t, hasVector, "vector field must be absent in BM25-only request")
}

func TestListNamespaces(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "GET", r.Method)
		assert.Equal(t, "/v1/namespaces", r.URL.Path)

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"namespaces": []map[string]interface{}{
				{"name": "ns1", "vectorCount": 100},
				{"name": "ns2", "vectorCount": 200},
			},
		})
	}))
	defer server.Close()

	client := NewClient(server.URL)
	namespaces, err := client.ListNamespaces(context.Background())

	require.NoError(t, err)
	assert.Len(t, namespaces, 2)
	assert.Equal(t, "ns1", namespaces[0].Name)
}

func TestGetNamespace(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "GET", r.Method)
		assert.Equal(t, "/v1/namespaces/test-ns", r.URL.Path)

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"name":        "test-ns",
			"vectorCount": 1000,
			"dimensions":  384,
		})
	}))
	defer server.Close()

	client := NewClient(server.URL)
	info, err := client.GetNamespace(context.Background(), "test-ns")

	require.NoError(t, err)
	assert.Equal(t, "test-ns", info.Name)
	assert.Equal(t, int64(1000), info.VectorCount)
}

func TestCreateNamespace(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "POST", r.Method)
		assert.Equal(t, "/v1/namespaces", r.URL.Path)

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"name":        "new-ns",
			"vectorCount": 0,
			"dimensions":  384,
		})
	}))
	defer server.Close()

	client := NewClient(server.URL)
	info, err := client.CreateNamespace(context.Background(), "new-ns", &CreateNamespaceOptions{
		Dimensions: 384,
	})

	require.NoError(t, err)
	assert.Equal(t, "new-ns", info.Name)
}

func TestDeleteNamespace(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "DELETE", r.Method)
		assert.Equal(t, "/v1/namespaces/test-ns", r.URL.Path)
		w.WriteHeader(http.StatusNoContent)
	}))
	defer server.Close()

	client := NewClient(server.URL)
	err := client.DeleteNamespace(context.Background(), "test-ns")

	require.NoError(t, err)
}

func TestHealth(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "GET", r.Method)
		assert.Equal(t, "/health", r.URL.Path)

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"status":  "healthy",
			"version": "0.1.0",
		})
	}))
	defer server.Close()

	client := NewClient(server.URL)
	health, err := client.Health(context.Background())

	require.NoError(t, err)
	assert.Equal(t, "healthy", health.Status)
}

func TestErrorHandling(t *testing.T) {
	t.Run("NotFoundError", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusNotFound)
			json.NewEncoder(w).Encode(map[string]interface{}{
				"error": "Namespace not found",
			})
		}))
		defer server.Close()

		client := NewClient(server.URL)
		_, err := client.GetNamespace(context.Background(), "nonexistent")

		require.Error(t, err)
		assert.True(t, IsNotFoundError(err))
	})

	t.Run("ValidationError", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusBadRequest)
			json.NewEncoder(w).Encode(map[string]interface{}{
				"error": "Invalid vector dimensions",
			})
		}))
		defer server.Close()

		client := NewClient(server.URL)
		_, err := client.Query(context.Background(), "test-ns", []float32{0.1}, nil)

		require.Error(t, err)
		assert.True(t, IsValidationError(err))
	})

	t.Run("ServerError", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusInternalServerError)
			json.NewEncoder(w).Encode(map[string]interface{}{
				"error": "Internal server error",
			})
		}))
		defer server.Close()

		client := NewClientWithOptions(ClientOptions{
			BaseURL:    server.URL,
			MaxRetries: 1,
		})
		_, err := client.Health(context.Background())

		require.Error(t, err)
		assert.True(t, IsServerError(err))
	})

	t.Run("RateLimitError", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			w.Header().Set("Retry-After", "60")
			w.WriteHeader(http.StatusTooManyRequests)
			json.NewEncoder(w).Encode(map[string]interface{}{
				"error": "Rate limit exceeded",
			})
		}))
		defer server.Close()

		// MaxRetries:1 → single attempt, returns error immediately without sleeping
		client := NewClientWithOptions(ClientOptions{
			BaseURL:    server.URL,
			MaxRetries: 1,
		})
		_, err := client.Query(context.Background(), "test-ns", []float32{0.1, 0.2, 0.3}, nil)

		require.Error(t, err)
		assert.True(t, IsRateLimitError(err))

		rateLimitErr := err.(*RateLimitError)
		assert.Equal(t, 60, rateLimitErr.RetryAfter)
	})
}

func TestFilterHelpers(t *testing.T) {
	t.Run("Eq", func(t *testing.T) {
		filter := Eq("value")
		assert.Equal(t, map[string]interface{}{OpEq: "value"}, filter)
	})

	t.Run("Gt", func(t *testing.T) {
		filter := Gt(100)
		assert.Equal(t, map[string]interface{}{OpGt: 100}, filter)
	})

	t.Run("In", func(t *testing.T) {
		filter := In("a", "b", "c")
		assert.Equal(t, map[string]interface{}{OpIn: []interface{}{"a", "b", "c"}}, filter)
	})

	t.Run("And", func(t *testing.T) {
		filter := And(
			map[string]interface{}{"status": Eq("active")},
			map[string]interface{}{"price": Lt(1000)},
		)
		expected := map[string]interface{}{
			OpAnd: []map[string]interface{}{
				{"status": map[string]interface{}{OpEq: "active"}},
				{"price": map[string]interface{}{OpLt: 1000}},
			},
		}
		assert.Equal(t, expected, filter)
	})
}

func TestAuthorizationHeader(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "Bearer test-api-key", r.Header.Get("Authorization"))

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"status": "healthy",
		})
	}))
	defer server.Close()

	client := NewClientWithOptions(ClientOptions{
		BaseURL: server.URL,
		APIKey:  "test-api-key",
	})
	_, err := client.Health(context.Background())

	require.NoError(t, err)
}

func TestRetryConfig(t *testing.T) {
	t.Run("DefaultRetryConfig", func(t *testing.T) {
		rc := DefaultRetryConfig()
		assert.Equal(t, 3, rc.MaxRetries)
		assert.Equal(t, 100*time.Millisecond, rc.BaseDelay)
		assert.Equal(t, 60*time.Second, rc.MaxDelay)
		assert.True(t, rc.Jitter)
	})

	t.Run("RetryBackoffOverridesMaxRetries", func(t *testing.T) {
		client := NewClientWithOptions(ClientOptions{
			BaseURL:    "http://localhost:3000",
			MaxRetries: 1,
			RetryBackoff: &RetryConfig{
				MaxRetries: 7,
				BaseDelay:  200 * time.Millisecond,
			},
		})
		assert.Equal(t, 7, client.retryConfig.MaxRetries)
		assert.Equal(t, 200*time.Millisecond, client.retryConfig.BaseDelay)
	})

	t.Run("ConnectTimeoutDefaultsToTimeout", func(t *testing.T) {
		// When ConnectTimeout is not set, the transport dial timeout equals Timeout.
		client := NewClientWithOptions(ClientOptions{
			BaseURL: "http://localhost:3000",
			Timeout: 15 * time.Second,
		})
		assert.NotNil(t, client.httpClient)
	})

	t.Run("RetryOn5xxSucceedsOnRecovery", func(t *testing.T) {
		calls := 0
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			calls++
			if calls < 3 {
				w.WriteHeader(http.StatusInternalServerError)
				json.NewEncoder(w).Encode(map[string]interface{}{"error": "internal error"})
				return
			}
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(map[string]interface{}{"status": "healthy"})
		}))
		defer server.Close()

		client := NewClientWithOptions(ClientOptions{
			BaseURL: server.URL,
			RetryBackoff: &RetryConfig{
				MaxRetries: 3,
				BaseDelay:  0,
				MaxDelay:   0,
				Jitter:     false,
			},
		})
		_, err := client.Health(context.Background())
		require.NoError(t, err)
		assert.Equal(t, 3, calls)
	})

	t.Run("NoRetryOn4xx", func(t *testing.T) {
		calls := 0
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			calls++
			w.WriteHeader(http.StatusBadRequest)
			json.NewEncoder(w).Encode(map[string]interface{}{"error": "bad request"})
		}))
		defer server.Close()

		client := NewClientWithOptions(ClientOptions{
			BaseURL: server.URL,
			RetryBackoff: &RetryConfig{
				MaxRetries: 3,
				BaseDelay:  0,
				MaxDelay:   0,
				Jitter:     false,
			},
		})
		_, err := client.Query(context.Background(), "ns", []float32{0.1}, nil)
		require.Error(t, err)
		assert.Equal(t, 1, calls) // no retry on 4xx
	})

	t.Run("RateLimitRetryAfterZeroIsImmediate", func(t *testing.T) {
		calls := 0
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			calls++
			if calls == 1 {
				w.Header().Set("Retry-After", "0")
				w.WriteHeader(http.StatusTooManyRequests)
				json.NewEncoder(w).Encode(map[string]interface{}{"error": "rate limited"})
				return
			}
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(map[string]interface{}{"upsertedCount": 1})
		}))
		defer server.Close()

		client := NewClientWithOptions(ClientOptions{
			BaseURL: server.URL,
			RetryBackoff: &RetryConfig{
				MaxRetries: 2,
				BaseDelay:  60 * time.Second, // would be very slow if Retry-After ignored
				Jitter:     false,
			},
		})
		start := time.Now()
		resp, err := client.Upsert(context.Background(), "ns", []VectorInput{{ID: "v1", Values: []float32{0.1}}})
		elapsed := time.Since(start)

		require.NoError(t, err)
		assert.Equal(t, 1, resp.UpsertedCount)
		assert.Less(t, elapsed, 2*time.Second) // Retry-After:0 → near-instant
		assert.Equal(t, 2, calls)
	})
}

// ---------------------------------------------------------------------------
// CE-2: Batch Recall / Forget (v0.7.0)
// ---------------------------------------------------------------------------

func TestBatchRecall(t *testing.T) {
	t.Run("POSTsToCorrectEndpoint", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, "POST", r.Method)
			assert.Equal(t, "/v1/memories/recall/batch", r.URL.Path)

			var body map[string]interface{}
			require.NoError(t, json.NewDecoder(r.Body).Decode(&body))
			assert.Equal(t, "qa", body["agent_id"])

			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(map[string]interface{}{
				"memories": []map[string]interface{}{
					{
						"id":         "mem_1",
						"content":    "test memory",
						"memory_type": "episodic",
						"importance": 0.8,
						"score":      0.9,
					},
				},
				"total":    10,
				"filtered": 1,
			})
		}))
		defer server.Close()

		client := NewClient(server.URL)
		req := BatchRecallRequest{
			AgentID: "qa",
			Filter:  BatchMemoryFilter{Tags: []string{"test"}},
			Limit:   50,
		}
		resp, err := client.BatchRecall(context.Background(), req)

		require.NoError(t, err)
		assert.Equal(t, 10, resp.Total)
		assert.Equal(t, 1, resp.Filtered)
		assert.Len(t, resp.Memories, 1)
		assert.Equal(t, "mem_1", resp.Memories[0].ID)
	})

	t.Run("EmptyFilterReturnsAll", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(map[string]interface{}{
				"memories": []interface{}{},
				"total":    0,
				"filtered": 0,
			})
		}))
		defer server.Close()

		client := NewClient(server.URL)
		resp, err := client.BatchRecall(context.Background(), BatchRecallRequest{AgentID: "agent-x"})

		require.NoError(t, err)
		assert.Equal(t, 0, resp.Total)
		assert.Equal(t, 0, resp.Filtered)
		assert.Empty(t, resp.Memories)
	})
}

func TestBatchForget(t *testing.T) {
	t.Run("DELETEsToCorrectEndpoint", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, "DELETE", r.Method)
			assert.Equal(t, "/v1/memories/forget/batch", r.URL.Path)

			var body map[string]interface{}
			require.NoError(t, json.NewDecoder(r.Body).Decode(&body))
			assert.Equal(t, "qa", body["agent_id"])

			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(map[string]interface{}{
				"deleted_count": 5,
			})
		}))
		defer server.Close()

		ts := int64(1700000000)
		client := NewClient(server.URL)
		req := BatchForgetRequest{
			AgentID: "qa",
			Filter:  BatchMemoryFilter{CreatedBefore: &ts},
		}
		resp, err := client.BatchForget(context.Background(), req)

		require.NoError(t, err)
		assert.Equal(t, 5, resp.DeletedCount)
	})
}

// ---------------------------------------------------------------------------
// OPS-1: Rate-Limit Headers (v0.7.0)
// ---------------------------------------------------------------------------

// ---------------------------------------------------------------------------
// AutoPilot Management (PILOT-1/2/3) — v0.7.2
// ---------------------------------------------------------------------------

func TestAutopilotStatus(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "GET", r.Method)
		assert.Equal(t, "/v1/admin/autopilot/status", r.URL.Path)

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"config": map[string]interface{}{
				"enabled":                        true,
				"dedup_threshold":                0.93,
				"dedup_interval_hours":           6,
				"consolidation_interval_hours":   12,
			},
			"last_dedup_at":        1700000000,
			"total_dedup_removed":  42,
			"total_consolidated":   10,
		})
	}))
	defer server.Close()

	client := NewClient(server.URL)
	resp, err := client.AutopilotStatus(context.Background())

	require.NoError(t, err)
	assert.True(t, resp.Config.Enabled)
	assert.Equal(t, float32(0.93), resp.Config.DedupThreshold)
	assert.Equal(t, uint64(42), resp.TotalDedupRemoved)
}

func TestAutopilotUpdateConfig(t *testing.T) {
	t.Run("UpdatesConfigAndReturnsResult", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, "PUT", r.Method)
			assert.Equal(t, "/v1/admin/autopilot/config", r.URL.Path)

			var body map[string]interface{}
			require.NoError(t, json.NewDecoder(r.Body).Decode(&body))
			assert.Equal(t, false, body["enabled"])
			assert.Equal(t, 0.90, body["dedup_threshold"])

			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(map[string]interface{}{
				"success": true,
				"config": map[string]interface{}{
					"enabled":                      false,
					"dedup_threshold":              0.90,
					"dedup_interval_hours":         8,
					"consolidation_interval_hours": 24,
				},
				"message": "AutoPilot config updated",
			})
		}))
		defer server.Close()

		enabled := false
		threshold := float32(0.90)
		client := NewClient(server.URL)
		resp, err := client.AutopilotUpdateConfig(context.Background(), AutoPilotConfigRequest{
			Enabled:        &enabled,
			DedupThreshold: &threshold,
		})

		require.NoError(t, err)
		assert.True(t, resp.Success)
		assert.False(t, resp.Config.Enabled)
	})

	t.Run("OmitsUnsetOptionalFields", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			var body map[string]interface{}
			require.NoError(t, json.NewDecoder(r.Body).Decode(&body))
			// Only dedup_interval_hours should be present
			assert.Contains(t, body, "dedup_interval_hours")
			assert.NotContains(t, body, "enabled")
			assert.NotContains(t, body, "dedup_threshold")

			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(map[string]interface{}{
				"success": true, "config": map[string]interface{}{}, "message": "ok",
			})
		}))
		defer server.Close()

		hours := uint64(4)
		client := NewClient(server.URL)
		_, err := client.AutopilotUpdateConfig(context.Background(), AutoPilotConfigRequest{
			DedupIntervalHours: &hours,
		})
		require.NoError(t, err)
	})
}

func TestAutopilotTrigger(t *testing.T) {
	t.Run("TriggerDedup", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, "POST", r.Method)
			assert.Equal(t, "/v1/admin/autopilot/trigger", r.URL.Path)

			var body map[string]interface{}
			require.NoError(t, json.NewDecoder(r.Body).Decode(&body))
			assert.Equal(t, "dedup", body["action"])

			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(map[string]interface{}{
				"success": true,
				"action":  "dedup",
				"dedup": map[string]interface{}{
					"namespaces_processed": 3,
					"memories_scanned":     500,
					"duplicates_removed":   12,
				},
				"message": "Dedup cycle completed",
			})
		}))
		defer server.Close()

		client := NewClient(server.URL)
		resp, err := client.AutopilotTrigger(context.Background(), "dedup")

		require.NoError(t, err)
		assert.True(t, resp.Success)
		assert.Equal(t, "dedup", resp.Action)
		require.NotNil(t, resp.Dedup)
		assert.Equal(t, 12, resp.Dedup.DuplicatesRemoved)
	})

	t.Run("TriggerAll", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(map[string]interface{}{
				"success": true,
				"action":  "all",
				"dedup": map[string]interface{}{
					"namespaces_processed": 2,
					"memories_scanned":     300,
					"duplicates_removed":   5,
				},
				"consolidation": map[string]interface{}{
					"namespaces_processed":  2,
					"memories_scanned":      300,
					"clusters_merged":       4,
					"memories_consolidated": 8,
				},
				"message": "Full AutoPilot cycle completed",
			})
		}))
		defer server.Close()

		client := NewClient(server.URL)
		resp, err := client.AutopilotTrigger(context.Background(), "all")

		require.NoError(t, err)
		assert.Equal(t, "all", resp.Action)
		require.NotNil(t, resp.Consolidation)
		assert.Equal(t, 4, resp.Consolidation.ClustersMerged)
	})
}

// ---------------------------------------------------------------------------
// Decay Engine Management (DECAY-1/2) — v0.7.3
// ---------------------------------------------------------------------------

func TestDecayConfig(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "GET", r.Method)
		assert.Equal(t, "/v1/admin/decay/config", r.URL.Path)

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"strategy":       "exponential",
			"half_life_hours": 168.0,
			"min_importance":  0.05,
		})
	}))
	defer server.Close()

	client := NewClient(server.URL)
	resp, err := client.DecayConfig(context.Background())

	require.NoError(t, err)
	assert.Equal(t, "exponential", resp.Strategy)
	assert.Equal(t, 168.0, resp.HalfLifeHours)
	assert.Equal(t, float32(0.05), resp.MinImportance)
}

func TestDecayUpdateConfig(t *testing.T) {
	t.Run("UpdatesConfigAndReturnsResult", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, "PUT", r.Method)
			assert.Equal(t, "/v1/admin/decay/config", r.URL.Path)

			var body map[string]interface{}
			require.NoError(t, json.NewDecoder(r.Body).Decode(&body))
			assert.Equal(t, "linear", body["strategy"])
			assert.Equal(t, 72.0, body["half_life_hours"])

			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(map[string]interface{}{
				"success": true,
				"config": map[string]interface{}{
					"strategy":        "linear",
					"half_life_hours": 72.0,
					"min_importance":  0.1,
				},
				"message": "Decay config updated",
			})
		}))
		defer server.Close()

		strategy := "linear"
		halfLife := 72.0
		client := NewClient(server.URL)
		resp, err := client.DecayUpdateConfig(context.Background(), DecayConfigUpdateRequest{
			Strategy:     &strategy,
			HalfLifeHours: &halfLife,
		})

		require.NoError(t, err)
		assert.True(t, resp.Success)
		assert.Equal(t, "linear", resp.Config.Strategy)
		assert.Equal(t, 72.0, resp.Config.HalfLifeHours)
	})

	t.Run("OmitsUnsetOptionalFields", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			var body map[string]interface{}
			require.NoError(t, json.NewDecoder(r.Body).Decode(&body))
			assert.Contains(t, body, "min_importance")
			assert.NotContains(t, body, "strategy")
			assert.NotContains(t, body, "half_life_hours")

			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(map[string]interface{}{
				"success": true, "config": map[string]interface{}{}, "message": "ok",
			})
		}))
		defer server.Close()

		minImportance := float32(0.02)
		client := NewClient(server.URL)
		_, err := client.DecayUpdateConfig(context.Background(), DecayConfigUpdateRequest{
			MinImportance: &minImportance,
		})
		require.NoError(t, err)
	})
}

func TestDecayStats(t *testing.T) {
	t.Run("ReturnsCountersAndLastCycleSnapshot", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, "GET", r.Method)
			assert.Equal(t, "/v1/admin/decay/stats", r.URL.Path)

			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(map[string]interface{}{
				"total_decayed": 1024,
				"total_deleted": 128,
				"last_run_at":   1700000000,
				"cycles_run":    42,
				"last_cycle": map[string]interface{}{
					"namespaces_processed": 5,
					"memories_processed":   200,
					"memories_decayed":     30,
					"memories_deleted":     5,
				},
			})
		}))
		defer server.Close()

		client := NewClient(server.URL)
		resp, err := client.DecayStats(context.Background())

		require.NoError(t, err)
		assert.Equal(t, uint64(1024), resp.TotalDecayed)
		assert.Equal(t, uint64(128), resp.TotalDeleted)
		assert.Equal(t, uint64(42), resp.CyclesRun)
		require.NotNil(t, resp.LastCycle)
		assert.Equal(t, 30, resp.LastCycle.MemoriesDecayed)
	})

	t.Run("HandlesNeverRunState", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(map[string]interface{}{
				"total_decayed": 0,
				"total_deleted": 0,
				"cycles_run":    0,
			})
		}))
		defer server.Close()

		client := NewClient(server.URL)
		resp, err := client.DecayStats(context.Background())

		require.NoError(t, err)
		assert.Equal(t, uint64(0), resp.CyclesRun)
		assert.Nil(t, resp.LastCycle)
	})
}

// ---------------------------------------------------------------------------
// store_memory expires_at (DECAY-3) — v0.7.3
// ---------------------------------------------------------------------------

func TestStoreMemoryWithExpiresAt(t *testing.T) {
	t.Run("IncludesExpiresAtInBody", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, "POST", r.Method)
			assert.Equal(t, "/v1/agents/agent-1/memories", r.URL.Path)

			var body map[string]interface{}
			require.NoError(t, json.NewDecoder(r.Body).Decode(&body))
			assert.Equal(t, float64(1800000000), body["expires_at"])

			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(map[string]interface{}{"id": "mem_1", "content": "test"})
		}))
		defer server.Close()

		expiresAt := int64(1800000000)
		client := NewClient(server.URL)
		_, err := client.StoreMemory(context.Background(), "agent-1", StoreMemoryRequest{
			Content:    "test",
			MemoryType: "episodic",
			ExpiresAt:  &expiresAt,
		})
		require.NoError(t, err)
	})

	t.Run("OmitsExpiresAtWhenNotSet", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			var body map[string]interface{}
			require.NoError(t, json.NewDecoder(r.Body).Decode(&body))
			assert.NotContains(t, body, "expires_at")

			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(map[string]interface{}{"id": "mem_1", "content": "test"})
		}))
		defer server.Close()

		client := NewClient(server.URL)
		_, err := client.StoreMemory(context.Background(), "agent-1", StoreMemoryRequest{
			Content:    "test",
			MemoryType: "episodic",
		})
		require.NoError(t, err)
	})
}

func TestLastRateLimitHeaders(t *testing.T) {
	t.Run("NilBeforeAnyRequest", func(t *testing.T) {
		client := NewClient("http://localhost:3000")
		assert.Nil(t, client.LastRateLimitHeaders())
	})

	t.Run("PopulatedAfterRequestWithHeaders", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			w.Header().Set("X-RateLimit-Limit", "500")
			w.Header().Set("X-RateLimit-Remaining", "499")
			w.Header().Set("X-RateLimit-Reset", "1700000120")
			w.Header().Set("X-Quota-Used", "100")
			w.Header().Set("X-Quota-Limit", "10000")
			json.NewEncoder(w).Encode(map[string]interface{}{"status": "healthy", "version": "0.7.0"})
		}))
		defer server.Close()

		client := NewClient(server.URL)
		_, err := client.Health(context.Background())
		require.NoError(t, err)

		rl := client.LastRateLimitHeaders()
		require.NotNil(t, rl)
		assert.Equal(t, int64(500), rl.Limit)
		assert.Equal(t, int64(499), rl.Remaining)
		assert.Equal(t, int64(1700000120), rl.Reset)
		assert.Equal(t, int64(100), rl.QuotaUsed)
		assert.Equal(t, int64(10000), rl.QuotaLimit)
	})

	t.Run("ZeroValuesWhenHeadersAbsent", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(map[string]interface{}{"status": "healthy"})
		}))
		defer server.Close()

		client := NewClient(server.URL)
		_, err := client.Health(context.Background())
		require.NoError(t, err)

		rl := client.LastRateLimitHeaders()
		require.NotNil(t, rl)
		assert.Equal(t, int64(0), rl.Limit)
		assert.Equal(t, int64(0), rl.Remaining)
		assert.Equal(t, int64(0), rl.Reset)
	})
}

// =============================================================================
// SSE Connected Event (DAK-720) — v0.8.3
// =============================================================================

func TestDakeraEventConnectedJSONDeserialization(t *testing.T) {
	payload := `{"type":"connected","timestamp":1700000000000}`
	var event DakeraEvent
	err := json.Unmarshal([]byte(payload), &event)
	require.NoError(t, err)
	assert.Equal(t, "connected", event.Type)
	assert.Equal(t, int64(1700000000000), event.Timestamp)
	// All other fields should be zero/empty for a connected event.
	assert.Empty(t, event.Namespace)
	assert.Equal(t, 0, event.Dimension)
}

func TestDakeraEventConnectedDistinctFromOtherTypes(t *testing.T) {
	// namespace_created must NOT be mistaken for connected.
	payload := `{"type":"namespace_created","namespace":"my-ns","dimension":384}`
	var event DakeraEvent
	err := json.Unmarshal([]byte(payload), &event)
	require.NoError(t, err)
	assert.Equal(t, "namespace_created", event.Type)
	assert.NotEqual(t, "connected", event.Type)
	assert.Equal(t, "my-ns", event.Namespace)
	assert.Equal(t, 384, event.Dimension)
}

func TestMemoryEventConnectedNormalization(t *testing.T) {
	// connected handshake uses "event_type": "connected" (or via SSE event: field).
	payload := `{"event_type":"connected","agent_id":"","timestamp":1700000000000}`
	var event MemoryEvent
	err := json.Unmarshal([]byte(payload), &event)
	require.NoError(t, err)
	assert.Equal(t, "connected", event.EventType)
	assert.Equal(t, "", event.AgentID)
	assert.Equal(t, int64(1700000000000), event.Timestamp)
	assert.Nil(t, event.MemoryID)
	assert.Nil(t, event.Content)
}

func TestStreamNamespaceEventsConnectedEvent(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/v1/namespaces/test-ns/events", r.URL.Path)
		assert.Equal(t, "text/event-stream", r.Header.Get("Accept"))

		w.Header().Set("Content-Type", "text/event-stream")
		w.Header().Set("Cache-Control", "no-cache")
		w.WriteHeader(http.StatusOK)
		if f, ok := w.(http.Flusher); ok {
			fmt.Fprintf(w, "data: {\"type\":\"connected\",\"timestamp\":1700000000000}\n\n")
			f.Flush()
		}
		// Block until the client cancels.
		<-r.Context().Done()
	}))
	defer server.Close()

	client := NewClient(server.URL)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	ch, err := client.StreamNamespaceEvents(ctx, "test-ns")
	require.NoError(t, err)

	result := <-ch
	cancel()

	require.NoError(t, result.Err)
	require.NotNil(t, result.Event)
	assert.Equal(t, "connected", result.Event.Type)
	assert.Equal(t, int64(1700000000000), result.Event.Timestamp)
}

func TestStreamMemoryEventsConnectedEvent(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/v1/events/stream", r.URL.Path)

		w.Header().Set("Content-Type", "text/event-stream")
		w.Header().Set("Cache-Control", "no-cache")
		w.WriteHeader(http.StatusOK)
		if f, ok := w.(http.Flusher); ok {
			// connected handshake — uses event: field + JSON body
			fmt.Fprintf(w, "event: connected\ndata: {\"type\":\"connected\",\"timestamp\":1700000000000}\n\n")
			f.Flush()
		}
		<-r.Context().Done()
	}))
	defer server.Close()

	client := NewClient(server.URL)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	ch, err := client.StreamMemoryEvents(ctx)
	require.NoError(t, err)

	result := <-ch
	cancel()

	require.NoError(t, result.Err)
	require.NotNil(t, result.Event)
	assert.Equal(t, "connected", result.Event.EventType)
	assert.Equal(t, "", result.Event.AgentID)
	assert.Equal(t, int64(1700000000000), result.Event.Timestamp)
}

// ===========================================================================
// Memory Knowledge Graph Tests (CE-5 / SDK-9)
// ===========================================================================

var graphResponse = MemoryGraph{
	RootID: "mem-abc",
	Depth:  2,
	Nodes: []GraphNode{
		{MemoryID: "mem-abc", ContentPreview: "Root memory", Importance: 0.9, Depth: 0},
		{MemoryID: "mem-def", ContentPreview: "Related memory", Importance: 0.7, Depth: 1},
	},
	Edges: []GraphEdge{
		{
			ID:        "edge-1",
			SourceID:  "mem-abc",
			TargetID:  "mem-def",
			EdgeType:  EdgeTypeRelatedTo,
			Weight:    0.92,
			CreatedAt: 1774000000,
		},
	},
}

var pathResponse = GraphPath{
	SourceID: "mem-abc",
	TargetID: "mem-ghi",
	Path:     []string{"mem-abc", "mem-def", "mem-ghi"},
	Hops:     2,
	Edges:    []GraphEdge{},
}

var linkResponse = GraphLinkResponse{
	Edge: GraphEdge{
		ID:        "edge-new",
		SourceID:  "mem-abc",
		TargetID:  "mem-xyz",
		EdgeType:  EdgeTypeLinkedBy,
		Weight:    1.0,
		CreatedAt: 1774002000,
	},
}

var exportResponse = GraphExport{
	AgentID:   "test-agent",
	Format:    "json",
	Data:      `{"nodes":[],"edges":[]}`,
	NodeCount: 10,
	EdgeCount: 7,
}

func TestMemoryGraph_DefaultDepth(t *testing.T) {
	var capturedURL string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedURL = r.URL.String()
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(graphResponse)
	}))
	defer server.Close()

	client := NewClient(server.URL)
	result, err := client.MemoryGraph(context.Background(), "mem-abc", nil)

	require.NoError(t, err)
	assert.Equal(t, "mem-abc", result.RootID)
	assert.Len(t, result.Nodes, 2)
	assert.Len(t, result.Edges, 1)
	assert.Contains(t, capturedURL, "depth=1")
}

func TestMemoryGraph_CustomDepthAndTypes(t *testing.T) {
	var capturedURL string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedURL = r.URL.String()
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(graphResponse)
	}))
	defer server.Close()

	client := NewClient(server.URL)
	opts := &GraphOptions{Depth: 3, Types: []EdgeType{EdgeTypeRelatedTo, EdgeTypeLinkedBy}}
	_, err := client.MemoryGraph(context.Background(), "mem-abc", opts)

	require.NoError(t, err)
	assert.Contains(t, capturedURL, "depth=3")
	assert.Contains(t, capturedURL, "related_to")
	assert.Contains(t, capturedURL, "linked_by")
}

func TestMemoryGraph_NoTypesParam(t *testing.T) {
	var capturedURL string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedURL = r.URL.String()
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(graphResponse)
	}))
	defer server.Close()

	client := NewClient(server.URL)
	_, err := client.MemoryGraph(context.Background(), "mem-abc", &GraphOptions{Depth: 1})

	require.NoError(t, err)
	assert.NotContains(t, capturedURL, "types=")
}

func TestMemoryPath(t *testing.T) {
	var capturedURL string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedURL = r.URL.String()
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(pathResponse)
	}))
	defer server.Close()

	client := NewClient(server.URL)
	result, err := client.MemoryPath(context.Background(), "mem-abc", "mem-ghi")

	require.NoError(t, err)
	assert.Equal(t, []string{"mem-abc", "mem-def", "mem-ghi"}, result.Path)
	assert.Equal(t, 2, result.Hops)
	assert.Contains(t, capturedURL, "/v1/memories/mem-abc/path")
	assert.Contains(t, capturedURL, "target=mem-ghi")
}

func TestMemoryLink_DefaultEdgeType(t *testing.T) {
	var capturedBody []byte
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedBody, _ = io.ReadAll(r.Body)
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(linkResponse)
	}))
	defer server.Close()

	client := NewClient(server.URL)
	result, err := client.MemoryLink(context.Background(), "mem-abc", "mem-xyz", EdgeTypeLinkedBy)

	require.NoError(t, err)
	assert.Equal(t, "edge-new", result.Edge.ID)
	assert.Equal(t, EdgeTypeLinkedBy, result.Edge.EdgeType)

	var body GraphLinkRequest
	require.NoError(t, json.Unmarshal(capturedBody, &body))
	assert.Equal(t, "mem-xyz", body.TargetID)
	assert.Equal(t, EdgeTypeLinkedBy, body.EdgeType)
}

func TestMemoryLink_CustomEdgeType(t *testing.T) {
	var capturedBody []byte
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedBody, _ = io.ReadAll(r.Body)
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(linkResponse)
	}))
	defer server.Close()

	client := NewClient(server.URL)
	_, err := client.MemoryLink(context.Background(), "mem-abc", "mem-xyz", EdgeTypePrecedes)

	require.NoError(t, err)
	var body GraphLinkRequest
	require.NoError(t, json.Unmarshal(capturedBody, &body))
	assert.Equal(t, EdgeTypePrecedes, body.EdgeType)
}

func TestAgentGraphExport_DefaultJSON(t *testing.T) {
	var capturedURL string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedURL = r.URL.String()
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(exportResponse)
	}))
	defer server.Close()

	client := NewClient(server.URL)
	result, err := client.AgentGraphExport(context.Background(), "test-agent", "")

	require.NoError(t, err)
	assert.Equal(t, "test-agent", result.AgentID)
	assert.Equal(t, "json", result.Format)
	assert.Equal(t, int64(10), result.NodeCount)
	assert.Contains(t, capturedURL, "/v1/agents/test-agent/graph/export")
	assert.Contains(t, capturedURL, "format=json")
}

func TestAgentGraphExport_Graphml(t *testing.T) {
	var capturedURL string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedURL = r.URL.String()
		w.Header().Set("Content-Type", "application/json")
		resp := exportResponse
		resp.Format = "graphml"
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	client := NewClient(server.URL)
	result, err := client.AgentGraphExport(context.Background(), "test-agent", "graphml")

	require.NoError(t, err)
	assert.Equal(t, "graphml", result.Format)
	assert.Contains(t, capturedURL, "format=graphml")
}

func TestEdgeTypeConstants(t *testing.T) {
	assert.Equal(t, EdgeType("related_to"), EdgeTypeRelatedTo)
	assert.Equal(t, EdgeType("shares_entity"), EdgeTypeSharesEntity)
	assert.Equal(t, EdgeType("precedes"), EdgeTypePrecedes)
	assert.Equal(t, EdgeType("linked_by"), EdgeTypeLinkedBy)
}

func TestGraphEdge_JSONRoundtrip(t *testing.T) {
	edge := GraphEdge{
		ID:        "e1",
		SourceID:  "mem-a",
		TargetID:  "mem-b",
		EdgeType:  EdgeTypeRelatedTo,
		Weight:    0.88,
		CreatedAt: 1774000000,
	}
	data, err := json.Marshal(edge)
	require.NoError(t, err)
	var decoded GraphEdge
	require.NoError(t, json.Unmarshal(data, &decoded))
	assert.Equal(t, edge, decoded)
}

// ---------------------------------------------------------------------------
// SubscribeAgentMemories (SDK-10)
// ---------------------------------------------------------------------------

func makeMemorySSEBody(events []MemoryEvent) string {
	var sb strings.Builder
	for _, e := range events {
		data, _ := json.Marshal(e)
		sb.WriteString("event: " + e.EventType + "\n")
		sb.WriteString("data: " + string(data) + "\n\n")
	}
	return sb.String()
}

// makeSseServer creates an httptest.Server that streams events once and then
// blocks until the client disconnects (ctx cancelled), preventing reconnect loops.
func makeSseServer(t *testing.T, body string) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)
		fmt.Fprint(w, body)
		if f, ok := w.(http.Flusher); ok {
			f.Flush()
		}
		// Block until the client disconnects so the goroutine does not reconnect.
		<-r.Context().Done()
	}))
}

func TestSubscribeAgentMemories_FiltersByAgentID(t *testing.T) {
	events := []MemoryEvent{
		{EventType: "stored", AgentID: "agent-a", Timestamp: 1774533000, MemoryID: strPtr("m1")},
		{EventType: "stored", AgentID: "agent-b", Timestamp: 1774533000, MemoryID: strPtr("m2")},
		{EventType: "recalled", AgentID: "agent-a", Timestamp: 1774533000, MemoryID: strPtr("m3")},
	}
	srv := makeSseServer(t, makeMemorySSEBody(events))
	defer srv.Close()

	client := NewClientWithOptions(ClientOptions{BaseURL: srv.URL, APIKey: "test"})
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	ch, err := client.SubscribeAgentMemories(ctx, "agent-a", nil)
	require.NoError(t, err)

	var collected []MemoryEvent
	for r := range ch {
		if r.Err != nil {
			break // ignore context-cancel teardown errors
		}
		collected = append(collected, *r.Event)
		if len(collected) == 2 {
			cancel()
		}
	}
	assert.Len(t, collected, 2)
	for _, e := range collected {
		assert.Equal(t, "agent-a", e.AgentID)
	}
}

func TestSubscribeAgentMemories_SkipsConnectedHandshake(t *testing.T) {
	body := "event: connected\ndata: {\"event_type\":\"connected\",\"agent_id\":\"\",\"timestamp\":1774533000}\n\n" +
		"event: stored\ndata: {\"event_type\":\"stored\",\"agent_id\":\"bot\",\"timestamp\":1774533000,\"memory_id\":\"m1\"}\n\n"
	srv := makeSseServer(t, body)
	defer srv.Close()

	client := NewClientWithOptions(ClientOptions{BaseURL: srv.URL, APIKey: "test"})
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	ch, err := client.SubscribeAgentMemories(ctx, "bot", nil)
	require.NoError(t, err)

	var collected []MemoryEvent
	for r := range ch {
		if r.Err != nil {
			break
		}
		collected = append(collected, *r.Event)
		cancel()
	}
	assert.Len(t, collected, 1)
	assert.Equal(t, "m1", *collected[0].MemoryID)
}

func TestSubscribeAgentMemories_TagFilter(t *testing.T) {
	events := []MemoryEvent{
		{EventType: "stored", AgentID: "bot", Timestamp: 1774533000, MemoryID: strPtr("m1"), Tags: []string{"important", "work"}},
		{EventType: "stored", AgentID: "bot", Timestamp: 1774533000, MemoryID: strPtr("m2"), Tags: []string{"trivial"}},
		{EventType: "stored", AgentID: "bot", Timestamp: 1774533000, MemoryID: strPtr("m3"), Tags: []string{"important"}},
	}
	srv := makeSseServer(t, makeMemorySSEBody(events))
	defer srv.Close()

	client := NewClientWithOptions(ClientOptions{BaseURL: srv.URL, APIKey: "test"})
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	ch, err := client.SubscribeAgentMemories(ctx, "bot", []string{"important"})
	require.NoError(t, err)

	ids := map[string]bool{}
	for r := range ch {
		if r.Err != nil {
			break
		}
		ids[*r.Event.MemoryID] = true
		if len(ids) == 2 {
			cancel()
		}
	}
	assert.True(t, ids["m1"])
	assert.True(t, ids["m3"])
	assert.False(t, ids["m2"])
}

func TestHasTagOverlap(t *testing.T) {
	assert.True(t, hasTagOverlap([]string{"a", "b"}, []string{"b", "c"}))
	assert.False(t, hasTagOverlap([]string{"a"}, []string{"b", "c"}))
	assert.False(t, hasTagOverlap(nil, []string{"b"}))
	assert.False(t, hasTagOverlap([]string{"a"}, nil))
}

// ---------------------------------------------------------------------------
// CE-4 Entity Extraction (GLiNER)
// ---------------------------------------------------------------------------

func TestConfigureNamespaceNer(t *testing.T) {
	var capturedMethod, capturedPath string
	var capturedBody []byte
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedMethod = r.Method
		capturedPath = r.URL.Path
		capturedBody, _ = io.ReadAll(r.Body)
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"ok": true,
		})
	}))
	defer server.Close()

	client := NewClient(server.URL)
	cfg := NamespaceNerConfig{
		ExtractEntities: true,
		EntityTypes:     []string{"person", "location"},
	}
	result, err := client.ConfigureNamespaceNer(context.Background(), "my-ns", cfg)

	require.NoError(t, err)
	assert.Equal(t, "PATCH", capturedMethod)
	assert.Equal(t, "/v1/namespaces/my-ns/config", capturedPath)
	assert.NotNil(t, result)
	assert.Equal(t, true, result["ok"])

	var sentBody map[string]interface{}
	require.NoError(t, json.Unmarshal(capturedBody, &sentBody))
	assert.Equal(t, true, sentBody["extract_entities"])
	entityTypes := sentBody["entity_types"].([]interface{})
	assert.Len(t, entityTypes, 2)
}

func TestConfigureNamespaceNer_PathEscaping(t *testing.T) {
	var capturedPath string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedPath = r.URL.EscapedPath()
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{})
	}))
	defer server.Close()

	client := NewClient(server.URL)
	_, err := client.ConfigureNamespaceNer(context.Background(), "ns/with/slashes", NamespaceNerConfig{ExtractEntities: false})

	require.NoError(t, err)
	assert.Equal(t, "/v1/namespaces/ns%2Fwith%2Fslashes/config", capturedPath)
}

func TestExtractEntities(t *testing.T) {
	var capturedMethod, capturedPath string
	var capturedBody []byte
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedMethod = r.Method
		capturedPath = r.URL.Path
		capturedBody, _ = io.ReadAll(r.Body)
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"entities": []map[string]interface{}{
				{"entity_type": "person", "value": "Alice", "score": 0.97},
				{"entity_type": "location", "value": "Paris", "score": 0.91},
			},
		})
	}))
	defer server.Close()

	client := NewClient(server.URL)
	result, err := client.ExtractEntities(context.Background(), "Alice lives in Paris.", []string{"person", "location"})

	require.NoError(t, err)
	assert.Equal(t, "POST", capturedMethod)
	assert.Equal(t, "/v1/memories/extract", capturedPath)
	assert.Len(t, result.Entities, 2)
	assert.Equal(t, "person", result.Entities[0].EntityType)
	assert.Equal(t, "Alice", result.Entities[0].Value)
	assert.InDelta(t, 0.97, result.Entities[0].Score, 0.001)
	assert.Equal(t, "location", result.Entities[1].EntityType)

	var sentBody map[string]interface{}
	require.NoError(t, json.Unmarshal(capturedBody, &sentBody))
	assert.Equal(t, "Alice lives in Paris.", sentBody["text"])
	assert.NotNil(t, sentBody["entity_types"])
}

func TestExtractEntities_NilEntityTypes(t *testing.T) {
	var capturedBody []byte
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedBody, _ = io.ReadAll(r.Body)
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"entities": []map[string]interface{}{},
		})
	}))
	defer server.Close()

	client := NewClient(server.URL)
	result, err := client.ExtractEntities(context.Background(), "Some text.", nil)

	require.NoError(t, err)
	assert.NotNil(t, result)
	assert.Empty(t, result.Entities)

	var sentBody map[string]interface{}
	require.NoError(t, json.Unmarshal(capturedBody, &sentBody))
	_, hasEntityTypes := sentBody["entity_types"]
	assert.False(t, hasEntityTypes, "entity_types should be omitted when nil")
}

func TestMemoryEntities(t *testing.T) {
	var capturedMethod, capturedPath string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedMethod = r.Method
		capturedPath = r.URL.Path
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"memory_id": "mem-42",
			"entities": []map[string]interface{}{
				{"entity_type": "org", "value": "Dakera", "score": 0.99},
			},
		})
	}))
	defer server.Close()

	client := NewClient(server.URL)
	result, err := client.MemoryEntities(context.Background(), "mem-42")

	require.NoError(t, err)
	assert.Equal(t, "GET", capturedMethod)
	assert.Equal(t, "/v1/memory/entities/mem-42", capturedPath)
	assert.Equal(t, "mem-42", result.MemoryID)
	assert.Len(t, result.Entities, 1)
	assert.Equal(t, "org", result.Entities[0].EntityType)
	assert.Equal(t, "Dakera", result.Entities[0].Value)
	assert.InDelta(t, 0.99, result.Entities[0].Score, 0.001)
}

func strPtr(s string) *string { return &s }
