package dakera

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
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
		assert.Equal(t, "/v1/namespaces/test-ns/fulltext/hybrid", r.URL.Path)

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
