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
// Memory Operations
// ===========================================================================

func TestStoreMemory(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "POST", r.Method)
		assert.Equal(t, "/v1/memory/store", r.URL.Path)
		var body map[string]interface{}
		json.NewDecoder(r.Body).Decode(&body)
		assert.Equal(t, "agent-1", body["agent_id"])
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"memory": map[string]interface{}{
				"id":          "mem-001",
				"content":     "user likes coffee",
				"agent_id":    "agent-1",
				"memory_type": "episodic",
				"importance":  0.8,
				"created_at":  1715900000,
			},
			"embedding_time_ms": 12,
		})
	}))
	defer server.Close()
	client := NewClient(server.URL)
	imp := float32(0.8)
	result, err := client.StoreMemory(context.Background(), "agent-1", StoreMemoryRequest{
		Content:    "user likes coffee",
		MemoryType: "episodic",
		Importance: &imp,
	})
	require.NoError(t, err)
	assert.Equal(t, "mem-001", result.Memory.ID)
	assert.Equal(t, "user likes coffee", result.Memory.Content)
}

func TestRecall(t *testing.T) {
	// Use the nested server wire format: {"memory": {...}, "score": N}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "POST", r.Method)
		assert.Equal(t, "/v1/memory/recall", r.URL.Path)
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"memories": []map[string]interface{}{
				{
					"memory": map[string]interface{}{
						"id": "mem-001", "content": "user likes coffee",
						"memory_type": "episodic", "importance": 0.8,
						"tags": []string{"coffee", "preference"},
					},
					"score": 0.95,
				},
				{
					"memory": map[string]interface{}{
						"id": "mem-002", "content": "user works at ACME",
						"memory_type": "semantic", "importance": 0.9,
					},
					"score": 0.82,
				},
			},
		})
	}))
	defer server.Close()
	client := NewClient(server.URL)
	result, err := client.Recall(context.Background(), "agent-1", RecallRequest{Query: "coffee preferences", TopK: 5})
	require.NoError(t, err)
	assert.Len(t, result.Memories, 2)
	assert.Equal(t, "mem-001", result.Memories[0].ID)
	assert.Equal(t, "user likes coffee", result.Memories[0].Content)
	assert.Equal(t, []string{"coffee", "preference"}, result.Memories[0].Tags)
	assert.InDelta(t, 0.95, float64(result.Memories[0].Score), 0.01)
	assert.Equal(t, "mem-002", result.Memories[1].ID)
	assert.Equal(t, "user works at ACME", result.Memories[1].Content)
	assert.InDelta(t, 0.82, float64(result.Memories[1].Score), 0.01)
}

func TestRecall_FlatFallback(t *testing.T) {
	// Flat format (used by /v1/agents/{id}/memories and legacy mocks) must also decode.
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"memories": []map[string]interface{}{
				{"id": "mem-003", "content": "flat format", "memory_type": "episodic", "importance": 0.7, "score": 0.75},
			},
		})
	}))
	defer server.Close()
	client := NewClient(server.URL)
	result, err := client.Recall(context.Background(), "agent-1", RecallRequest{Query: "flat"})
	require.NoError(t, err)
	assert.Len(t, result.Memories, 1)
	assert.Equal(t, "mem-003", result.Memories[0].ID)
	assert.Equal(t, "flat format", result.Memories[0].Content)
	assert.InDelta(t, 0.75, float64(result.Memories[0].Score), 0.01)
}

func TestGetMemory(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "GET", r.Method)
		assert.Contains(t, r.URL.Path, "/v1/memory/get/mem-001")
		assert.Equal(t, "agent-1", r.URL.Query().Get("agent_id"))
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"id":          "mem-001",
			"content":     "user likes coffee",
			"agent_id":    "agent-1",
			"memory_type": "episodic",
			"importance":  0.8,
		})
	}))
	defer server.Close()
	client := NewClient(server.URL)
	result, err := client.GetMemory(context.Background(), "agent-1", "mem-001")
	require.NoError(t, err)
	assert.Equal(t, "mem-001", result.ID)
	assert.Equal(t, "user likes coffee", result.Content)
}

func TestUpdateMemory(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "PUT", r.Method)
		assert.Equal(t, "/v1/memory/update/mem-001", r.URL.Path)
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"memory": map[string]interface{}{
				"id":          "mem-001",
				"content":     "user loves espresso",
				"agent_id":    "agent-1",
				"memory_type": "episodic",
				"importance":  0.85,
			},
		})
	}))
	defer server.Close()
	client := NewClient(server.URL)
	content := "user loves espresso"
	result, err := client.UpdateMemory(context.Background(), "agent-1", "mem-001", UpdateMemoryRequest{Content: &content})
	require.NoError(t, err)
	assert.Equal(t, "mem-001", result.Memory.ID)
}

func TestForget(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "POST", r.Method)
		assert.Equal(t, "/v1/memory/forget", r.URL.Path)
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{"status": "ok"})
	}))
	defer server.Close()
	client := NewClient(server.URL)
	err := client.Forget(context.Background(), "agent-1", "mem-001")
	require.NoError(t, err)
}

func TestSearchMemories(t *testing.T) {
	// Use the nested server wire format: {"memory": {...}, "score": N}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "POST", r.Method)
		assert.Equal(t, "/v1/memory/search", r.URL.Path)
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"memories": []map[string]interface{}{
				{
					"memory": map[string]interface{}{
						"id": "mem-001", "content": "coffee",
						"memory_type": "episodic", "importance": 0.8,
					},
					"score": 0.9,
				},
			},
		})
	}))
	defer server.Close()
	client := NewClient(server.URL)
	result, err := client.SearchMemories(context.Background(), "agent-1", SearchMemoriesRequest{Query: "coffee"})
	require.NoError(t, err)
	assert.Len(t, result, 1)
	assert.Equal(t, "mem-001", result[0].ID)
	assert.Equal(t, "coffee", result[0].Content)
	assert.InDelta(t, 0.9, float64(result[0].Score), 0.01)
}

func TestUpdateImportance(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "POST", r.Method)
		assert.Equal(t, "/v1/memory/importance", r.URL.Path)
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{"status": "ok"})
	}))
	defer server.Close()
	client := NewClient(server.URL)
	err := client.UpdateImportance(context.Background(), "agent-1", UpdateImportanceRequest{
		MemoryIDs:  []string{"mem-001"},
		Importance: 0.95,
	})
	require.NoError(t, err)
}

func TestConsolidate(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "POST", r.Method)
		assert.Equal(t, "/v1/memory/consolidate", r.URL.Path)
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"memories_removed":   3,
			"source_memory_ids":  []string{"mem-002", "mem-003", "mem-004"},
			"consolidated_memory": map[string]interface{}{
				"id": "mem-consolidated", "content": "merged content", "memory_type": "semantic", "importance": 0.9,
			},
		})
	}))
	defer server.Close()
	client := NewClient(server.URL)
	threshold := float32(0.9)
	result, err := client.Consolidate(context.Background(), "agent-1", ConsolidateRequest{Threshold: &threshold})
	require.NoError(t, err)
	assert.Equal(t, 3, result.MemoriesRemoved)
}

func TestConsolidateAgent(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "POST", r.Method)
		assert.Equal(t, "/v1/agents/agent-1/consolidate", r.URL.Path)
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"agent_id":            "agent-1",
			"memories_scanned":    100,
			"clusters_found":      3,
			"memories_deprecated": 5,
			"anchor_ids":          []string{"mem-1"},
			"deprecated_ids":      []string{"mem-2", "mem-3"},
		})
	}))
	defer server.Close()
	client := NewClient(server.URL)
	result, err := client.ConsolidateAgent(context.Background(), "agent-1")
	require.NoError(t, err)
	assert.Equal(t, "agent-1", result.AgentID)
	assert.Equal(t, 3, result.ClustersFound)
}

func TestGetConsolidationLog(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "GET", r.Method)
		assert.Equal(t, "/v1/agents/agent-1/consolidation/log", r.URL.Path)
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode([]map[string]interface{}{
			{"timestamp": 1715900000, "clusters_found": 2, "memories_deprecated": 3, "anchor_ids": []string{"a"}, "deprecated_ids": []string{"b"}},
		})
	}))
	defer server.Close()
	client := NewClient(server.URL)
	result, err := client.GetConsolidationLog(context.Background(), "agent-1")
	require.NoError(t, err)
	assert.Len(t, result, 1)
}

func TestPatchConsolidationConfig(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "PATCH", r.Method)
		assert.Equal(t, "/v1/agents/agent-1/consolidation/config", r.URL.Path)
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"enabled":              true,
			"epsilon":              0.85,
			"min_samples":          2,
			"soft_deprecation_days": 7,
		})
	}))
	defer server.Close()
	client := NewClient(server.URL)
	enabled := true
	result, err := client.PatchConsolidationConfig(context.Background(), "agent-1", ConsolidationConfigPatch{Enabled: &enabled})
	require.NoError(t, err)
	assert.True(t, result.Enabled)
}

func TestMemoryFeedback(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "POST", r.Method)
		assert.Equal(t, "/v1/agents/agent-1/memories/feedback", r.URL.Path)
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"status":             "updated",
			"updated_importance": 0.9,
		})
	}))
	defer server.Close()
	client := NewClient(server.URL)
	score := float32(0.9)
	result, err := client.MemoryFeedback(context.Background(), "agent-1", MemoryFeedbackRequest{
		MemoryID:       "mem-001",
		Feedback:       "relevant",
		RelevanceScore: &score,
	})
	require.NoError(t, err)
	assert.Equal(t, "updated", result.Status)
}

// ===========================================================================
// Session Operations
// ===========================================================================

func TestStartSession(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "POST", r.Method)
		assert.Equal(t, "/v1/sessions/start", r.URL.Path)
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"session": map[string]interface{}{
				"id":           "sess-001",
				"agent_id":     "agent-1",
				"started_at":   1715900000,
				"memory_count": 0,
			},
		})
	}))
	defer server.Close()
	client := NewClient(server.URL)
	result, err := client.StartSession(context.Background(), StartSessionRequest{AgentID: "agent-1"})
	require.NoError(t, err)
	assert.Equal(t, "sess-001", result.ID)
	assert.Equal(t, "agent-1", result.AgentID)
}

func TestEndSession(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "POST", r.Method)
		assert.Equal(t, "/v1/sessions/sess-001/end", r.URL.Path)
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"session": map[string]interface{}{
				"id":           "sess-001",
				"agent_id":     "agent-1",
				"started_at":   1715900000,
				"ended_at":     1715903600,
				"memory_count": 5,
			},
			"memory_count": 5,
		})
	}))
	defer server.Close()
	client := NewClient(server.URL)
	result, err := client.EndSession(context.Background(), "sess-001")
	require.NoError(t, err)
	assert.Equal(t, 5, result.MemoryCount)
}

func TestGetSession(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "GET", r.Method)
		assert.Equal(t, "/v1/sessions/sess-001", r.URL.Path)
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"id":           "sess-001",
			"agent_id":     "agent-1",
			"started_at":   1715900000,
			"memory_count": 3,
		})
	}))
	defer server.Close()
	client := NewClient(server.URL)
	result, err := client.GetSession(context.Background(), "sess-001")
	require.NoError(t, err)
	assert.Equal(t, "sess-001", result.ID)
}

func TestListSessions(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "GET", r.Method)
		assert.Contains(t, r.URL.Path, "/v1/sessions")
		assert.Equal(t, "agent-1", r.URL.Query().Get("agent_id"))
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"sessions": []map[string]interface{}{
				{"id": "sess-001", "agent_id": "agent-1", "started_at": 1715900000, "memory_count": 3},
				{"id": "sess-002", "agent_id": "agent-1", "started_at": 1715910000, "memory_count": 7},
			},
		})
	}))
	defer server.Close()
	client := NewClient(server.URL)
	result, err := client.ListSessions(context.Background(), &ListSessionsOptions{AgentID: "agent-1"})
	require.NoError(t, err)
	assert.Len(t, result, 2)
}

func TestSessionMemories(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "GET", r.Method)
		assert.Equal(t, "/v1/sessions/sess-001/memories", r.URL.Path)
		w.Header().Set("Content-Type", "application/json")
		// Session memories use nested format: {"memory": {...}, "score": N}
		json.NewEncoder(w).Encode([]map[string]interface{}{
			{
				"memory": map[string]interface{}{
					"id": "mem-001", "content": "hello",
					"memory_type": "episodic", "importance": 0.7,
				},
				"score": 1.0,
			},
		})
	}))
	defer server.Close()
	client := NewClient(server.URL)
	result, err := client.SessionMemories(context.Background(), "sess-001")
	require.NoError(t, err)
	assert.Len(t, result, 1)
	assert.Equal(t, "mem-001", result[0].ID)
	assert.Equal(t, "hello", result[0].Content)
}

// ===========================================================================
// Agent Operations
// ===========================================================================

func TestListAgents(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "GET", r.Method)
		assert.Equal(t, "/v1/agents", r.URL.Path)
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode([]map[string]interface{}{
			{"agent_id": "agent-1", "memory_count": 50, "session_count": 10, "active_sessions": 1},
			{"agent_id": "agent-2", "memory_count": 30, "session_count": 5, "active_sessions": 0},
		})
	}))
	defer server.Close()
	client := NewClient(server.URL)
	result, err := client.ListAgents(context.Background())
	require.NoError(t, err)
	assert.Len(t, result, 2)
	assert.Equal(t, "agent-1", result[0].AgentID)
}

func TestAgentMemories(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "GET", r.Method)
		assert.Contains(t, r.URL.Path, "/v1/agents/agent-1/memories")
		w.Header().Set("Content-Type", "application/json")
		// Agent memories use nested format: {"memory": {...}, "score": N}
		json.NewEncoder(w).Encode([]map[string]interface{}{
			{
				"memory": map[string]interface{}{
					"id": "mem-001", "content": "coffee",
					"memory_type": "episodic", "importance": 0.8,
				},
				"score": 0.0,
			},
		})
	}))
	defer server.Close()
	client := NewClient(server.URL)
	result, err := client.AgentMemories(context.Background(), "agent-1", nil)
	require.NoError(t, err)
	assert.Len(t, result, 1)
	assert.Equal(t, "mem-001", result[0].ID)
	assert.Equal(t, "coffee", result[0].Content)
}

func TestAgentStats(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "GET", r.Method)
		assert.Equal(t, "/v1/agents/agent-1/stats", r.URL.Path)
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"agent_id":         "agent-1",
			"total_memories":   50,
			"memories_by_type": map[string]int64{"episodic": 30, "semantic": 20},
			"total_sessions":   10,
			"active_sessions":  1,
		})
	}))
	defer server.Close()
	client := NewClient(server.URL)
	result, err := client.AgentStats(context.Background(), "agent-1")
	require.NoError(t, err)
	assert.Equal(t, "agent-1", result.AgentID)
	assert.Equal(t, int64(50), result.TotalMemories)
}

func TestAgentSessions(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "GET", r.Method)
		assert.Contains(t, r.URL.Path, "/v1/agents/agent-1/sessions")
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"sessions": []map[string]interface{}{
				{"id": "sess-001", "agent_id": "agent-1", "started_at": 1715900000, "memory_count": 3},
			},
		})
	}))
	defer server.Close()
	client := NewClient(server.URL)
	result, err := client.AgentSessions(context.Background(), "agent-1", nil)
	require.NoError(t, err)
	assert.Len(t, result, 1)
}

func TestGetWakeUpContext(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "GET", r.Method)
		assert.Contains(t, r.URL.Path, "/v1/agents/agent-1/wake-up")
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"agent_id": "agent-1",
			"memories": []map[string]interface{}{
				{"id": "mem-001", "content": "important context", "memory_type": "semantic", "importance": 0.95},
			},
			"total_available": 50,
		})
	}))
	defer server.Close()
	client := NewClient(server.URL)
	topN := 10
	result, err := client.GetWakeUpContext(context.Background(), "agent-1", &WakeUpOptions{TopN: &topN})
	require.NoError(t, err)
	assert.Equal(t, "agent-1", result.AgentID)
	assert.Len(t, result.Memories, 1)
	assert.Equal(t, int64(50), result.TotalAvailable)
}

func TestCompressAgent(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "POST", r.Method)
		assert.Equal(t, "/v1/agents/agent-1/compress", r.URL.Path)
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"agent_id":        "agent-1",
			"memories_before": 100,
			"memories_after":  80,
			"removed_count":   20,
			"duration_ms":     250.5,
		})
	}))
	defer server.Close()
	client := NewClient(server.URL)
	result, err := client.CompressAgent(context.Background(), "agent-1")
	require.NoError(t, err)
	assert.Equal(t, int64(20), result.RemovedCount)
	assert.Equal(t, int64(100), result.MemoriesBefore)
}

// ===========================================================================
// Import / Export
// ===========================================================================

func TestImportMemories(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "POST", r.Method)
		assert.Equal(t, "/v1/import", r.URL.Path)
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"imported_count": 10,
			"skipped_count":  2,
			"errors":         []string{},
		})
	}))
	defer server.Close()
	client := NewClient(server.URL)
	result, err := client.ImportMemories(context.Background(), []map[string]interface{}{
		{"content": "memory1"},
	}, "jsonl", "agent-1", "")
	require.NoError(t, err)
	assert.Equal(t, 10, result.ImportedCount)
}

func TestExportMemories(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "GET", r.Method)
		assert.Contains(t, r.URL.Path, "/v1/export")
		assert.Equal(t, "jsonl", r.URL.Query().Get("format"))
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"data":   []map[string]interface{}{{"id": "mem-1", "content": "hello"}},
			"format": "jsonl",
			"count":  1,
		})
	}))
	defer server.Close()
	client := NewClient(server.URL)
	result, err := client.ExportMemories(context.Background(), "jsonl", "agent-1", "", 100)
	require.NoError(t, err)
	assert.Equal(t, 1, result.Count)
}

// ===========================================================================
// Audit
// ===========================================================================

func TestListAuditEvents(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "GET", r.Method)
		assert.Contains(t, r.URL.Path, "/v1/audit")
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"events": []map[string]interface{}{
				{"id": "evt-1", "event_type": "memory.store", "agent_id": "agent-1", "timestamp": 1715900000},
			},
			"total": 1,
		})
	}))
	defer server.Close()
	client := NewClient(server.URL)
	result, err := client.ListAuditEvents(context.Background(), AuditQuery{AgentID: "agent-1", Limit: 10})
	require.NoError(t, err)
	assert.Len(t, result.Events, 1)
}

func TestExportAudit(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "POST", r.Method)
		assert.Equal(t, "/v1/audit/export", r.URL.Path)
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"data":   "{\"events\":[]}",
			"format": "json",
			"count":  0,
		})
	}))
	defer server.Close()
	client := NewClient(server.URL)
	result, err := client.ExportAudit(context.Background(), "json", "agent-1", "", 0, 0)
	require.NoError(t, err)
	assert.Equal(t, "json", result.Format)
}

// ===========================================================================
// Extract & Namespace Config
// ===========================================================================

func TestExtractText(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "POST", r.Method)
		assert.Equal(t, "/v1/extract", r.URL.Path)
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"entities":    []map[string]interface{}{{"entity_type": "person", "value": "John"}},
			"provider":    "gliner",
			"duration_ms": 15.2,
		})
	}))
	defer server.Close()
	client := NewClient(server.URL)
	result, err := client.ExtractText(context.Background(), "John went to the store", "", "gliner", "")
	require.NoError(t, err)
	assert.Equal(t, "gliner", result.Provider)
	assert.Len(t, result.Entities, 1)
}

func TestListExtractProviders(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "GET", r.Method)
		assert.Equal(t, "/v1/extract/providers", r.URL.Path)
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode([]map[string]interface{}{
			{"name": "gliner", "available": true, "models": []string{"base"}},
		})
	}))
	defer server.Close()
	client := NewClient(server.URL)
	result, err := client.ListExtractProviders(context.Background())
	require.NoError(t, err)
	assert.Len(t, result, 1)
	assert.Equal(t, "gliner", result[0].Name)
}

func TestConfigureNamespaceExtractor(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "PATCH", r.Method)
		assert.Equal(t, "/v1/namespaces/test-ns/extractor", r.URL.Path)
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]interface{}{"status": "ok"})
	}))
	defer server.Close()
	client := NewClient(server.URL)
	err := client.ConfigureNamespaceExtractor(context.Background(), "test-ns", "gliner", "base")
	require.NoError(t, err)
}

func TestGetNamespaceEntityConfig(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "GET", r.Method)
		assert.Equal(t, "/v1/namespaces/test-ns/config", r.URL.Path)
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"namespace":        "test-ns",
			"extract_entities": true,
			"entity_types":     []string{"person", "organization"},
		})
	}))
	defer server.Close()
	client := NewClient(server.URL)
	result, err := client.GetNamespaceEntityConfig(context.Background(), "test-ns")
	require.NoError(t, err)
	assert.True(t, result.ExtractEntities)
}

func TestGetNamespaceExtractor(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "GET", r.Method)
		assert.Equal(t, "/v1/namespaces/test-ns/extractor", r.URL.Path)
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"provider": "gliner",
			"model":    "base",
		})
	}))
	defer server.Close()
	client := NewClient(server.URL)
	result, err := client.GetNamespaceExtractor(context.Background(), "test-ns")
	require.NoError(t, err)
	assert.Equal(t, "gliner", result.Provider)
}

func TestConfigureNamespaceMethod(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "PUT", r.Method)
		assert.Equal(t, "/v1/namespaces/test-ns", r.URL.Path)
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"namespace": "test-ns",
			"dimension": 384,
			"distance":  "cosine",
			"created":   true,
		})
	}))
	defer server.Close()
	client := NewClient(server.URL)
	result, err := client.ConfigureNamespace(context.Background(), "test-ns", ConfigureNamespaceRequest{
		Dimension: 384,
		Distance:  DistanceMetricCosine,
	})
	require.NoError(t, err)
	assert.True(t, result.Created)
	assert.Equal(t, 384, result.Dimension)
}

// ===========================================================================
// Policy
// ===========================================================================

func TestGetMemoryPolicy(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "GET", r.Method)
		assert.Equal(t, "/v1/namespaces/test-ns/memory_policy", r.URL.Path)
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"working_ttl_seconds":  14400,
			"episodic_ttl_seconds": 2592000,
			"working_decay":        "exponential",
		})
	}))
	defer server.Close()
	client := NewClient(server.URL)
	result, err := client.GetMemoryPolicy(context.Background(), "test-ns")
	require.NoError(t, err)
	assert.NotNil(t, result.WorkingTTLSeconds)
	assert.Equal(t, int64(14400), *result.WorkingTTLSeconds)
}

func TestSetMemoryPolicy(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "PUT", r.Method)
		assert.Equal(t, "/v1/namespaces/test-ns/memory_policy", r.URL.Path)
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"working_ttl_seconds": 7200,
			"working_decay":       "linear",
		})
	}))
	defer server.Close()
	client := NewClient(server.URL)
	ttl := int64(7200)
	decay := "linear"
	result, err := client.SetMemoryPolicy(context.Background(), "test-ns", MemoryPolicy{
		WorkingTTLSeconds: &ttl,
		WorkingDecay:      &decay,
	})
	require.NoError(t, err)
	assert.Equal(t, int64(7200), *result.WorkingTTLSeconds)
}

// ===========================================================================
// Keys
// ===========================================================================

func TestCreateKey(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "POST", r.Method)
		assert.Equal(t, "/admin/keys", r.URL.Path)
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"id":          "key-001",
			"name":        "test-key",
			"key":         "dk_live_xxxx",
			"permissions": []string{"read", "write"},
			"created_at":  "2026-05-17T00:00:00Z",
			"active":      true,
		})
	}))
	defer server.Close()
	client := NewClient(server.URL)
	result, err := client.CreateKey(context.Background(), CreateKeyRequest{Name: "test-key", Permissions: []string{"read", "write"}})
	require.NoError(t, err)
	assert.Equal(t, "key-001", result.ID)
	assert.True(t, result.Active)
}

func TestListKeys(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "GET", r.Method)
		assert.Equal(t, "/admin/keys", r.URL.Path)
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode([]map[string]interface{}{
			{"id": "key-001", "name": "test-key", "created_at": "2026-05-17T00:00:00Z", "active": true},
		})
	}))
	defer server.Close()
	client := NewClient(server.URL)
	result, err := client.ListKeys(context.Background())
	require.NoError(t, err)
	assert.Len(t, result, 1)
}

func TestGetKey(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "GET", r.Method)
		assert.Equal(t, "/admin/keys/key-001", r.URL.Path)
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"id": "key-001", "name": "test-key", "created_at": "2026-05-17T00:00:00Z", "active": true,
		})
	}))
	defer server.Close()
	client := NewClient(server.URL)
	result, err := client.GetKey(context.Background(), "key-001")
	require.NoError(t, err)
	assert.Equal(t, "key-001", result.ID)
}

func TestDeleteKey(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "DELETE", r.Method)
		assert.Equal(t, "/admin/keys/key-001", r.URL.Path)
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]interface{}{"status": "deleted"})
	}))
	defer server.Close()
	client := NewClient(server.URL)
	err := client.DeleteKey(context.Background(), "key-001")
	require.NoError(t, err)
}

func TestDeactivateKey(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "POST", r.Method)
		assert.Equal(t, "/admin/keys/key-001/deactivate", r.URL.Path)
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"id": "key-001", "name": "test-key", "created_at": "2026-05-17T00:00:00Z", "active": false,
		})
	}))
	defer server.Close()
	client := NewClient(server.URL)
	result, err := client.DeactivateKey(context.Background(), "key-001")
	require.NoError(t, err)
	assert.False(t, result.Active)
}

func TestRotateKey(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "POST", r.Method)
		assert.Equal(t, "/admin/keys/key-001/rotate", r.URL.Path)
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"id": "key-001", "name": "test-key", "key": "dk_live_new_xxxx", "created_at": "2026-05-17T00:00:00Z", "active": true,
		})
	}))
	defer server.Close()
	client := NewClient(server.URL)
	result, err := client.RotateKey(context.Background(), "key-001")
	require.NoError(t, err)
	assert.True(t, result.Active)
}

func TestKeyUsage(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "GET", r.Method)
		assert.Equal(t, "/admin/keys/key-001/usage", r.URL.Path)
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"key_id":         "key-001",
			"total_requests": 500,
			"last_used":      "2026-05-17T00:00:00Z",
		})
	}))
	defer server.Close()
	client := NewClient(server.URL)
	result, err := client.KeyUsage(context.Background(), "key-001")
	require.NoError(t, err)
	assert.Equal(t, "key-001", result.KeyID)
	assert.Equal(t, int64(500), result.TotalRequests)
}

// ===========================================================================
// ValidFrom bi-temporal field tests (DAK-7424)
// ===========================================================================

func TestStoreMemory_ValidFrom(t *testing.T) {
	var capturedBody map[string]interface{}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewDecoder(r.Body).Decode(&capturedBody)
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"memory": map[string]interface{}{
				"id":          "mem-vf1",
				"content":     "past event",
				"agent_id":    "agent-1",
				"memory_type": "episodic",
			},
		})
	}))
	defer server.Close()
	client := NewClient(server.URL)
	ts := int64(1700000000)
	_, err := client.StoreMemory(context.Background(), "agent-1", StoreMemoryRequest{
		Content:   "past event",
		ValidFrom: &ts,
	})
	require.NoError(t, err)
	assert.Equal(t, float64(ts), capturedBody["valid_from"], "valid_from must be forwarded in request body")
}

func TestStoreMemory_ValidFrom_Omitted(t *testing.T) {
	var capturedBody map[string]interface{}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewDecoder(r.Body).Decode(&capturedBody)
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"memory": map[string]interface{}{
				"id": "mem-vf2", "content": "now", "agent_id": "agent-1", "memory_type": "episodic",
			},
		})
	}))
	defer server.Close()
	client := NewClient(server.URL)
	_, err := client.StoreMemory(context.Background(), "agent-1", StoreMemoryRequest{
		Content: "now",
	})
	require.NoError(t, err)
	_, present := capturedBody["valid_from"]
	assert.False(t, present, "valid_from must be omitted from request body when nil")
}

// Error Tests
// ===========================================================================

func TestStoreMemory_Error(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]interface{}{"error": "server error"})
	}))
	defer server.Close()
	client := NewClientWithOptions(ClientOptions{BaseURL: server.URL, MaxRetries: 1})
	_, err := client.StoreMemory(context.Background(), "agent-1", StoreMemoryRequest{Content: "test"})
	require.Error(t, err)
}

func TestRecall_Error(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]interface{}{"error": "server error"})
	}))
	defer server.Close()
	client := NewClientWithOptions(ClientOptions{BaseURL: server.URL, MaxRetries: 1})
	_, err := client.Recall(context.Background(), "agent-1", RecallRequest{Query: "test"})
	require.Error(t, err)
}
