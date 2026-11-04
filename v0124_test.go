package dakera

// Server v0.12.4 wire additions: forget by description or by request and
// forget-request detection, standing instructions and degraded recalls, the
// recall defaults (importance_weighted, include_associated, rerank_head),
// session summary_pending, audit cursor pagination, strength projection,
// asynchronous import, the "stored as answered" admin changes and the new
// report fields. Response shapes follow the server's types.

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// v0124Server starts a server that answers every request with status and reply (JSON-encoded),
// and records the last request's method, path, query and decoded body.
type v0124Capture struct {
	Method string
	Path   string
	Query  map[string][]string
	Body   map[string]interface{}
	Raw    []byte
}

func v0124Server(t *testing.T, status int, reply interface{}) (*Client, *v0124Capture) {
	t.Helper()
	captured := &v0124Capture{}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		captured.Method = r.Method
		captured.Path = r.URL.Path
		captured.Query = r.URL.Query()
		captured.Raw, _ = io.ReadAll(r.Body)
		captured.Body = nil
		if len(captured.Raw) > 0 {
			_ = json.Unmarshal(captured.Raw, &captured.Body)
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(status)
		_ = json.NewEncoder(w).Encode(reply)
	}))
	t.Cleanup(server.Close)
	return NewClient(server.URL), captured
}

func TestV0124RecallDefaultsAreNotSent(t *testing.T) {
	client, got := v0124Server(t, 200, map[string]interface{}{"memories": []interface{}{}})
	_, err := client.Recall(context.Background(), "a1", RecallRequest{Query: "q"})
	require.NoError(t, err)
	assert.NotContains(t, got.Body, "include_associated")
	assert.NotContains(t, got.Body, "importance_weighted")
	assert.NotContains(t, got.Body, "rerank_head")
	assert.NotContains(t, got.Body, "top_k")

	off, head := false, 0
	_, err = client.Recall(context.Background(), "a1", RecallRequest{
		Query: "q", IncludeAssociated: &off, ImportanceWeighted: &off, RerankHead: &head,
	})
	require.NoError(t, err)
	assert.Equal(t, false, got.Body["include_associated"])
	assert.Equal(t, false, got.Body["importance_weighted"])
	assert.Equal(t, float64(0), got.Body["rerank_head"])
}

func TestV0124RecallInstructionsAndDegraded(t *testing.T) {
	client, _ := v0124Server(t, 200, map[string]interface{}{
		"memories": []interface{}{
			map[string]interface{}{"memory": map[string]interface{}{"id": "m1", "content": "fact", "memory_type": "episodic"}, "score": 0.9},
		},
		"associated_memories": []interface{}{
			map[string]interface{}{"memory": map[string]interface{}{"id": "m2", "content": "neighbour"}, "score": 0.0, "depth": 1},
		},
		"instructions": []interface{}{
			map[string]interface{}{
				"memory":    map[string]interface{}{"id": "i1", "content": "From now on, answer in French", "tags": []string{InstructionTag}},
				"score":     0.7,
				"retrieval": RetrievalInstruction,
			},
		},
		"degraded": []string{"graph: the edge store could not be read"},
	})
	resp, err := client.Recall(context.Background(), "a1", RecallRequest{Query: "q"})
	require.NoError(t, err)
	require.Len(t, resp.Instructions, 1)
	assert.Equal(t, "i1", resp.Instructions[0].ID)
	assert.Equal(t, RetrievalInstruction, resp.Instructions[0].Retrieval)
	assert.Equal(t, []string{"graph: the edge store could not be read"}, resp.Degraded)
	require.Len(t, resp.AssociatedMemories, 1)

	// An older server sends neither field.
	old, _ := v0124Server(t, 200, map[string]interface{}{"memories": []interface{}{}})
	resp, err = old.Recall(context.Background(), "a1", RecallRequest{Query: "q"})
	require.NoError(t, err)
	assert.Nil(t, resp.Instructions)
	assert.Nil(t, resp.Degraded)
}

func TestV0124StoreReportsForgetRequest(t *testing.T) {
	reply := storeAnswer()
	reply["forget_request"] = map[string]interface{}{
		"probability": 0.97, "threshold": 0.5, "is_request": true,
	}
	client, _ := v0124Server(t, 200, reply)
	resp, err := client.StoreMemory(context.Background(), "a1", StoreMemoryRequest{Content: "Please forget my address"})
	require.NoError(t, err)
	require.NotNil(t, resp.ForgetRequest)
	assert.True(t, resp.ForgetRequest.IsRequest)
	assert.InDelta(t, 0.97, resp.ForgetRequest.Probability, 1e-6)

	plain, _ := v0124Server(t, 200, storeAnswer())
	resp, err = plain.StoreMemory(context.Background(), "a1", StoreMemoryRequest{Content: "hello"})
	require.NoError(t, err)
	assert.Nil(t, resp.ForgetRequest)
}

func TestV0124BatchStoreForgetRequestsAndDuplicates(t *testing.T) {
	client, _ := v0124Server(t, 200, map[string]interface{}{
		"stored":                  []interface{}{},
		"stored_count":            1,
		"total_embedding_time_ms": 3,
		"forget_requests": []interface{}{
			map[string]interface{}{"id": "m3", "probability": 0.91, "threshold": 0.5, "is_request": true, "sentence": "forget my job"},
		},
		"duplicates": []interface{}{map[string]interface{}{"index": 1, "existing_id": "m0"}},
	})
	resp, err := client.StoreMemoriesBatch(context.Background(), BatchStoreMemoryRequest{
		AgentID:  "a1",
		Memories: []BatchStoreMemoryItem{{Content: "x"}, {Content: "x"}},
	})
	require.NoError(t, err)
	require.Len(t, resp.ForgetRequests, 1)
	assert.Equal(t, "m3", resp.ForgetRequests[0].ID)
	assert.True(t, resp.ForgetRequests[0].IsRequest)
	assert.Equal(t, "forget my job", resp.ForgetRequests[0].Sentence)
	assert.Equal(t, []BatchDuplicate{{Index: 1, ExistingID: "m0"}}, resp.Duplicates)
}

func TestV0124ForgetMemoriesByRequest(t *testing.T) {
	client, got := v0124Server(t, 200, map[string]interface{}{
		"deleted_count": 0,
		"forgotten_ids": []string{"m1", "m2"},
		"intent": map[string]interface{}{
			"probability": 0.88, "threshold": 0.5, "is_request": true, "sentence": "Oublie mon adresse.",
		},
	})
	until := int64(1_700_000_000)
	limit := 5
	resp, err := client.ForgetMemories(context.Background(), ForgetMemoriesRequest{
		AgentID: "a1", Request: "Bonjour. Oublie mon adresse.", Until: &until, Limit: &limit,
		Restorable: true, DryRun: true,
	})
	require.NoError(t, err)
	assert.Equal(t, "POST", got.Method)
	assert.Equal(t, "/v1/memory/forget", got.Path)
	assert.Equal(t, map[string]interface{}{
		"agent_id": "a1", "request": "Bonjour. Oublie mon adresse.", "until": float64(until),
		"limit": float64(5), "restorable": true, "dry_run": true,
	}, got.Body)
	assert.Equal(t, []string{"m1", "m2"}, resp.ForgottenIDs)
	require.NotNil(t, resp.Intent)
	assert.True(t, resp.Intent.IsRequest)
	assert.Equal(t, "Oublie mon adresse.", resp.Intent.Sentence)
}

func TestV0124ForgetMemoriesByDescriptionAndFilters(t *testing.T) {
	client, got := v0124Server(t, 200, map[string]interface{}{"deleted_count": 2, "forgotten_ids": []string{"m9", "m4"}})
	minScore := float32(0.6)
	resp, err := client.ForgetMemories(context.Background(), ForgetMemoriesRequest{
		AgentID: "a1", About: "I love sushi", MinScore: &minScore, MemoryIDs: []string{"m9", "m4", "m5"},
	})
	require.NoError(t, err)
	assert.Equal(t, "I love sushi", got.Body["about"])
	assert.InDelta(t, 0.6, got.Body["min_score"], 1e-6)
	assert.NotContains(t, got.Body, "request")
	assert.NotContains(t, got.Body, "dry_run")
	assert.NotContains(t, got.Body, "restorable")
	assert.Equal(t, 2, resp.DeletedCount)
	assert.Nil(t, resp.Intent)

	// The plain filter form sends only the filters (what older servers read).
	_, err = client.ForgetMemories(context.Background(), ForgetMemoriesRequest{AgentID: "a1", Tags: []string{"tmp"}})
	require.NoError(t, err)
	assert.Equal(t, map[string]interface{}{"agent_id": "a1", "tags": []interface{}{"tmp"}}, got.Body)
}

func TestV0124ForgetMemoriesRefusesAboutAndRequest(t *testing.T) {
	var calls int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&calls, 1)
	}))
	defer server.Close()
	_, err := NewClient(server.URL).ForgetMemories(context.Background(), ForgetMemoriesRequest{
		AgentID: "a1", About: "sushi", Request: "forget sushi",
	})
	var validation *ValidationError
	require.True(t, errors.As(err, &validation))
	assert.Equal(t, int32(0), atomic.LoadInt32(&calls))
}

func TestV0124ForgetMemoriesModelUnavailable(t *testing.T) {
	client, _ := v0124Server(t, 503, map[string]interface{}{
		"error": "the re-ranker is not available", "code": "SERVICE_UNAVAILABLE",
	})
	client.retryConfig.MaxRetries = 1
	_, err := client.ForgetMemories(context.Background(), ForgetMemoriesRequest{AgentID: "a1", About: "sushi"})
	var serverErr *ServerError
	require.True(t, errors.As(err, &serverErr))
	assert.Equal(t, 503, serverErr.StatusCode)
}

func TestV0124DetectForgetRequests(t *testing.T) {
	client, got := v0124Server(t, 200, map[string]interface{}{
		"model": "multilingual-minilm", "threshold": 0.5, "sentence_threshold": 0.7,
		"results": []interface{}{
			map[string]interface{}{"probability": 0.02, "is_request": false},
			map[string]interface{}{"probability": 0.81, "is_request": true, "sentence": "私の住所のことは忘れて"},
		},
	})
	resp, err := client.DetectForgetRequests(context.Background(), []string{"I always forget my keys", "…私の住所のことは忘れて"})
	require.NoError(t, err)
	assert.Equal(t, "/v1/memory/forget/detect", got.Path)
	assert.Equal(t, []interface{}{"I always forget my keys", "…私の住所のことは忘れて"}, got.Body["texts"])
	assert.Equal(t, "multilingual-minilm", resp.Model)
	assert.InDelta(t, 0.7, resp.SentenceThreshold, 1e-6)
	require.Len(t, resp.Results, 2)
	assert.False(t, resp.Results[0].IsRequest)
	assert.True(t, resp.Results[1].IsRequest)
	assert.Equal(t, "私の住所のことは忘れて", resp.Results[1].Sentence)

	_, err = client.DetectForgetRequests(context.Background(), nil)
	var validation *ValidationError
	assert.True(t, errors.As(err, &validation))
}

func TestV0124EndSessionSummaryPending(t *testing.T) {
	client, got := v0124Server(t, 200, map[string]interface{}{
		"session":         map[string]interface{}{"id": "s1", "agent_id": "a1"},
		"memory_count":    5,
		"summary_pending": "the store refused the summary memory; it is stored once the store recovers",
	})
	resp, err := client.EndSession(context.Background(), "s1")
	require.NoError(t, err)
	assert.Equal(t, "/v1/sessions/s1/end", got.Path)
	assert.Equal(t, "{}", strings.TrimSpace(string(got.Raw)))
	assert.Contains(t, resp.SummaryPending, "store refused")

	never := false
	_, err = client.EndSessionWithOptions(context.Background(), "s1", EndSessionRequest{Summary: "done", AutoSummarize: &never})
	require.NoError(t, err)
	assert.Equal(t, map[string]interface{}{"summary": "done", "auto_summarize": false}, got.Body)
}

func TestV0124AuditCursorPagination(t *testing.T) {
	pages := map[string]map[string]interface{}{
		"": {
			"events":      []interface{}{map[string]interface{}{"id": 9, "event_type": "stored", "timestamp": 3}},
			"count":       1,
			"next_cursor": "c-9",
		},
		"c-9": {
			"events": []interface{}{map[string]interface{}{"id": 8, "event_type": "recalled", "timestamp": 2}},
			"count":  1,
		},
	}
	var cursors []string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		cursor := r.URL.Query().Get("cursor")
		cursors = append(cursors, cursor)
		assert.Equal(t, "a1", r.URL.Query().Get("agent_id"))
		_ = json.NewEncoder(w).Encode(pages[cursor])
	}))
	defer server.Close()
	client := NewClient(server.URL)

	var ids []string
	query := AuditQuery{AgentID: "a1", Limit: 1}
	for {
		page, err := client.ListAuditEvents(context.Background(), query)
		require.NoError(t, err)
		for _, e := range page.Events {
			ids = append(ids, e.ID)
		}
		assert.Equal(t, page.NextCursor, page.Cursor)
		if page.NextCursor == "" {
			break
		}
		query.Cursor = page.NextCursor
	}
	assert.Equal(t, []string{"9", "8"}, ids)
	assert.Equal(t, []string{"", "c-9"}, cursors)
}

func TestV0124GetMemoryWithProjection(t *testing.T) {
	client, got := v0124Server(t, 200, map[string]interface{}{
		"id": "m1", "content": "c", "memory_type": "episodic", "importance": 0.8,
		"strength_projection": []interface{}{
			map[string]interface{}{"day": 0, "importance": 0.8},
			map[string]interface{}{"day": 1, "importance": 0.74},
		},
	})
	resp, err := client.GetMemoryWithProjection(context.Background(), "a1", "m1", 1)
	require.NoError(t, err)
	assert.Equal(t, "/v1/memory/get/m1", got.Path)
	assert.Equal(t, "1", got.Query["projection_days"][0])
	assert.Equal(t, "a1", got.Query["agent_id"][0])
	assert.Equal(t, "m1", resp.ID)
	require.Len(t, resp.StrengthProjection, 2)
	assert.Equal(t, 1, resp.StrengthProjection[1].Day)
	assert.InDelta(t, 0.74, resp.StrengthProjection[1].Importance, 1e-6)

	_, err = client.GetMemoryWithProjection(context.Background(), "", "m1", 0)
	var validation *ValidationError
	assert.True(t, errors.As(err, &validation))
	_, err = client.GetMemoryWithProjection(context.Background(), "", "m1", 366)
	assert.True(t, errors.As(err, &validation))
}

func TestV0124DeduplicatedContentAndDeletionKind(t *testing.T) {
	client, _ := v0124Server(t, 200, map[string]interface{}{
		"agent_id": "a1", "count": 1,
		"memories": []interface{}{map[string]interface{}{
			"id": "m1", "kind": "deletion", "survivor_id": "", "deleted_at": 1, "restorable_until": 2,
			"content": "I love sushi",
		}},
	})
	resp, err := client.ListDeduplicated(context.Background(), "a1")
	require.NoError(t, err)
	require.Len(t, resp.Memories, 1)
	assert.Equal(t, DeduplicatedKindDeletion, resp.Memories[0].Kind)
	assert.Equal(t, "I love sushi", resp.Memories[0].Content)
}

func TestV0124ImportMemoryFileAsync(t *testing.T) {
	var polls int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case r.Method == "POST" && r.URL.Path == "/v1/import":
			assert.Equal(t, "a1", r.URL.Query().Get("agent_id"))
			assert.Equal(t, "csv", r.URL.Query().Get("format"))
			assert.Equal(t, "true", r.URL.Query().Get("async"))
			assert.True(t, strings.HasPrefix(r.Header.Get("Content-Type"), "multipart/form-data"))
			file, header, err := r.FormFile("file")
			require.NoError(t, err)
			assert.Equal(t, "export.csv", header.Filename)
			body, _ := io.ReadAll(file)
			assert.Equal(t, "content\nhello\n", string(body))
			w.WriteHeader(http.StatusAccepted)
			_ = json.NewEncoder(w).Encode(map[string]interface{}{
				"job_id": "job-7", "status": "running", "format": "csv", "total": 1,
				"imported": 0, "skipped": 0, "errors": []string{}, "started_at": 10, "finished_at": nil,
			})
		case r.Method == "GET" && r.URL.Path == "/v1/import/job-7/status":
			status := "running"
			var finished interface{}
			if atomic.AddInt32(&polls, 1) >= 2 {
				status, finished = "done", 12
			}
			_ = json.NewEncoder(w).Encode(map[string]interface{}{
				"job_id": "job-7", "status": status, "format": "csv", "total": 1,
				"imported": 1, "skipped": 0, "errors": []string{}, "started_at": 10, "finished_at": finished,
			})
		default:
			t.Errorf("unexpected %s %s", r.Method, r.URL.Path)
		}
	}))
	defer server.Close()
	client := NewClient(server.URL)

	job, err := client.ImportMemoryFile(context.Background(), "a1", []byte("content\nhello\n"),
		&ImportMemoriesOptions{Format: "csv", FileName: "export.csv", Async: true})
	require.NoError(t, err)
	assert.Equal(t, ImportStatusRunning, job.Status)
	assert.Equal(t, "job-7", job.JobID)
	assert.Nil(t, job.FinishedAt)

	final, err := client.WaitForImportJob(context.Background(), job.JobID, JobWaitOptions{PollInterval: time.Millisecond, Timeout: 5 * time.Second})
	require.NoError(t, err)
	assert.Equal(t, ImportStatusDone, final.Status)
	require.NotNil(t, final.FinishedAt)
	assert.Equal(t, 1, final.Imported)

	_, err = client.ImportMemoryFile(context.Background(), "", nil, nil)
	var validation *ValidationError
	assert.True(t, errors.As(err, &validation))
}

func TestV0124ExportMemoryFile(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/v1/export", r.URL.Path)
		assert.Equal(t, "mem0", r.URL.Query().Get("format"))
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("X-Dakera-Export-Skipped", "0")
		_, _ = w.Write([]byte(`{"memories":[{"id":"m1","memory":"hello"}]}`))
	}))
	defer server.Close()
	client := NewClient(server.URL)
	file, err := client.ExportMemoryFile(context.Background(), "a1", "mem0")
	require.NoError(t, err)
	assert.Equal(t, "application/json", file.ContentType)
	assert.Equal(t, `{"memories":[{"id":"m1","memory":"hello"}]}`, string(file.Data))

	legacy, err := client.ExportMemories(context.Background(), "mem0", "a1", "", 0)
	require.NoError(t, err)
	assert.Equal(t, 1, legacy.Count)
	assert.Equal(t, "hello", legacy.Data[0]["memory"])
}

func TestV0124NamespaceConfigSourceAndPolicyStored(t *testing.T) {
	client, _ := v0124Server(t, 200, map[string]interface{}{
		"namespace": "ns", "extract_entities": true,
		"entity_types": []string{"person", "organization", "location"}, "source": "server_default",
	})
	cfg, err := client.GetNamespaceEntityConfig(context.Background(), "ns")
	require.NoError(t, err)
	assert.Equal(t, EntityConfigSourceServerDefault, cfg.Source)

	client, got := v0124Server(t, 200, map[string]interface{}{
		"working_ttl_seconds": nil, "episodic_ttl_seconds": nil, "semantic_ttl_seconds": nil,
		"procedural_ttl_seconds": nil, "working_decay": "exponential", "stored": false,
	})
	policy, err := client.GetMemoryPolicy(context.Background(), "ns")
	require.NoError(t, err)
	require.NotNil(t, policy.Stored)
	assert.False(t, *policy.Stored)
	assert.Nil(t, policy.WorkingTTLSeconds)

	// SetMemoryPolicy never sends "stored" (the server refuses unknown fields).
	_, err = client.SetMemoryPolicy(context.Background(), "ns", *policy)
	require.NoError(t, err)
	assert.Equal(t, "PUT", got.Method)
	assert.NotContains(t, got.Body, "stored")
	assert.NotContains(t, got.Body, "working_ttl_seconds")
	require.NotNil(t, policy.Stored, "the caller's policy is not changed")
}

func TestV0124LinkLabel(t *testing.T) {
	client, got := v0124Server(t, 200, map[string]interface{}{
		"from_id": "m1", "to_id": "m2", "edge_type": "linked_by", "label": "caused",
	})
	resp, err := client.MemoryLink(context.Background(), "a1", "m1", "m2", "caused")
	require.NoError(t, err)
	assert.Equal(t, "caused", got.Body["label"])
	assert.Equal(t, "caused", resp.Label)
	assert.Equal(t, "caused", resp.Edge.Label)

	var edge GraphEdge
	require.NoError(t, json.Unmarshal([]byte(`{"from_id":"m1","to_id":"m2","edge_type":"linked_by","weight":1,"created_at":1,"label":"caused"}`), &edge))
	assert.Equal(t, "caused", edge.Label)
	assert.Equal(t, "m1", edge.SourceID)
}

func TestV0124ExtractEntitiesExtractorAndWarnings(t *testing.T) {
	client, _ := v0124Server(t, 200, map[string]interface{}{
		"entities":  []interface{}{},
		"count":     0,
		"extractor": "rule_based",
		"warnings":  []string{"the neural pass failed: model not loaded"},
	})
	resp, err := client.ExtractEntities(context.Background(), "Alice moved to Paris", nil)
	require.NoError(t, err)
	assert.Equal(t, ExtractorRuleBased, resp.Extractor)
	assert.Len(t, resp.Warnings, 1)
}

func TestV0124FailureReports(t *testing.T) {
	client, _ := v0124Server(t, 200, map[string]interface{}{
		"groups": []interface{}{}, "duplicates_found": 2, "duplicates_merged": 0,
		"duplicates_skipped_changed": 0,
		"groups_failed": []interface{}{map[string]interface{}{
			"canonical_id": "m1", "duplicate_ids": []string{"m2", "m3"}, "reason": "write refused",
		}},
	})
	dedup, err := client.Deduplicate(context.Background(), DeduplicateRequest{AgentID: "a1"})
	require.NoError(t, err)
	assert.Equal(t, []DuplicateGroupFailure{{CanonicalID: "m1", DuplicateIDs: []string{"m2", "m3"}, Reason: "write refused"}}, dedup.GroupsFailed)

	client, _ = v0124Server(t, 200, map[string]interface{}{
		"node_count": 0, "agents": []interface{}{}, "nodes": []interface{}{}, "edges": []interface{}{},
		"stats":       map[string]interface{}{},
		"unavailable": []interface{}{map[string]interface{}{"agent_id": "a2", "reason": "read failed"}},
	})
	network, err := client.CrossAgentNetwork(context.Background(), CrossAgentNetworkRequest{})
	require.NoError(t, err)
	assert.Equal(t, []AgentUnavailable{{AgentID: "a2", Reason: "read failed"}}, network.Unavailable)

	client, _ = v0124Server(t, 200, map[string]interface{}{
		"agent_id": "a1", "memories_scanned": 10, "clusters_found": 1, "summaries_created": 1,
		"originals_deprecated": 1, "summary_ids": []string{"s1"}, "deprecated_ids": []string{"m1"},
		"deprecation_skipped": []string{"m2"}, "deprecation_error": nil, "summaries_skipped": []interface{}{},
	})
	compressed, err := client.CompressAgent(context.Background(), "a1")
	require.NoError(t, err)
	assert.Equal(t, []string{"m1"}, compressed.DeprecatedIDs)
	assert.Equal(t, []string{"m2"}, compressed.DeprecationSkipped)
	assert.Empty(t, compressed.DeprecationError)
}

func TestV0124AnalyticsOverviewEchoesPeriodAndNamespace(t *testing.T) {
	client, got := v0124Server(t, 200, map[string]interface{}{
		"total_queries": 4, "period": "1h", "namespace": "ns",
	})
	resp, err := client.AnalyticsOverview(context.Background(), &AnalyticsOptions{Period: "1h", Namespace: "ns"})
	require.NoError(t, err)
	assert.Equal(t, "1h", got.Query["period"][0])
	assert.Equal(t, "ns", got.Query["namespace"][0])
	assert.Equal(t, "1h", resp.Period)
	assert.Equal(t, "ns", resp.Namespace)
}

func TestV0124AutopilotConsolidationModeAndFailures(t *testing.T) {
	client, _ := v0124Server(t, 200, map[string]interface{}{
		"config": map[string]interface{}{
			"enabled": true, "dedup_mode": "report", "consolidation_mode": "report",
		},
		"last_dedup": map[string]interface{}{
			"namespaces_processed": 2, "failures": []string{"_dakera_agent_a: content unreadable"},
		},
		"last_consolidation": map[string]interface{}{
			"namespaces_processed": 2, "clusters_found": 3, "memories_in_clusters": 9,
		},
		"total_dedup_removed": 0, "total_consolidated": 0,
	})
	status, err := client.AutopilotStatus(context.Background())
	require.NoError(t, err)
	assert.Equal(t, ConsolidationModeReport, status.Config.ConsolidationMode)
	assert.Equal(t, DedupModeReport, status.Config.DedupMode)
	assert.Len(t, status.LastDedup.Failures, 1)
	assert.Equal(t, 3, status.LastConsolidation.ClustersFound)
	assert.Equal(t, 9, status.LastConsolidation.MemoriesInClusters)

	client, got := v0124Server(t, 200, map[string]interface{}{"success": true, "config": map[string]interface{}{}, "message": "ok"})
	_, err = client.AutopilotUpdateConfig(context.Background(), AutoPilotConfigRequest{ConsolidationMode: ConsolidationModeMerge})
	require.NoError(t, err)
	assert.Equal(t, map[string]interface{}{"consolidation_mode": "merge"}, got.Body)

	client, _ = v0124Server(t, 200, map[string]interface{}{
		"success": false, "action": "all", "message": "partial",
		"dedup": map[string]interface{}{"mode": "report", "failures": []string{"x"}},
		"consolidation": map[string]interface{}{
			"mode": "report", "clusters_found": 1, "memories_in_clusters": 3, "failures": []string{"y"},
		},
	})
	trigger, err := client.AutopilotTrigger(context.Background(), "all")
	require.NoError(t, err)
	assert.False(t, trigger.Success)
	assert.Equal(t, []string{"x"}, trigger.Dedup.Failures)
	assert.Equal(t, ConsolidationModeReport, trigger.Consolidation.Mode)
	assert.Equal(t, []string{"y"}, trigger.Consolidation.Failures)
}

func TestV0124DecayTtlWarmHealthDerivations(t *testing.T) {
	client, _ := v0124Server(t, 200, map[string]interface{}{
		"total_decayed": 1, "total_deleted": 0, "cycles_run": 4, "cycles_failed": 1,
		"last_cycle": map[string]interface{}{"namespaces_processed": 2, "failures": []string{"listing failed"}},
	})
	decay, err := client.DecayStats(context.Background())
	require.NoError(t, err)
	assert.Equal(t, uint64(1), decay.CyclesFailed)
	assert.Equal(t, []string{"listing failed"}, decay.LastCycle.Failures)

	client, _ = v0124Server(t, 200, map[string]interface{}{
		"success": false, "vectors_removed": 3, "namespaces_cleaned": []string{"a"}, "message": "m",
		"failed": []interface{}{map[string]interface{}{"namespace": "b", "reason": "read failed"}},
	})
	ttl, err := client.AdminTtlCleanup(context.Background(), "")
	require.NoError(t, err)
	assert.False(t, ttl.Success)
	assert.Equal(t, []UnavailableNamespace{{Namespace: "b", Reason: "read failed"}}, ttl.Failed)

	client, got := v0124Server(t, 200, map[string]interface{}{
		"success": true, "entries_warmed": 0, "entries_skipped": 0, "job_id": "job-1",
		"message": "warming in the background", "target_tier": "l1", "priority": "normal",
	})
	threshold := float32(0.8)
	warm, err := client.WarmCache(context.Background(), "ns", WarmCacheRequest{
		QueryVector: []float32{0.1, 0.2}, MinScoreThreshold: &threshold, Background: true,
	})
	require.NoError(t, err)
	vector := got.Body["query_vector"].([]interface{})
	require.Len(t, vector, 2)
	assert.InDelta(t, 0.1, vector[0], 1e-6)
	assert.InDelta(t, 0.2, vector[1], 1e-6)
	assert.InDelta(t, 0.8, got.Body["min_score_threshold"], 1e-6)
	assert.True(t, warm.Success)
	assert.Equal(t, "job-1", warm.JobID)
	assert.Equal(t, "l1", warm.TargetTier)

	client, _ = v0124Server(t, 200, map[string]interface{}{
		"status": "degraded", "version": "0.12.4",
		"degraded":        []interface{}{map[string]interface{}{"component": "pending_writes", "reason": "1 change waits"}},
		"config_warnings": []interface{}{map[string]interface{}{"component": "sizing", "reason": "2 CPUs"}},
	})
	health, err := client.Health(context.Background())
	require.NoError(t, err)
	assert.Equal(t, []HealthComponent{{Component: "pending_writes", Reason: "1 change waits"}}, health.Degraded)
	assert.Equal(t, "sizing", health.ConfigWarnings[0].Component)

	client, _ = v0124Server(t, 200, map[string]interface{}{
		"settled": false,
		"entity_tagging": map[string]interface{}{
			"mode": "background", "pending": 3, "sweep_namespaces": []string{}, "tagged": 10,
			"extractor_gave_up": 1,
		},
	})
	derivations, err := client.AdminDerivationStatus(context.Background())
	require.NoError(t, err)
	require.NotNil(t, derivations.EntityTagging)
	assert.Equal(t, int64(3), derivations.EntityTagging.Pending)
	assert.Equal(t, int64(1), derivations.EntityTagging.ExtractorGaveUp)
}

func TestV0124StoredChangesAnswerPending(t *testing.T) {
	client, _ := v0124Server(t, http.StatusAccepted, map[string]interface{}{
		"success": true, "namespace": "ns", "config": map[string]interface{}{}, "message": "logged",
	})
	quota, err := client.AdminSetQuota(context.Background(), "ns", SetQuotaRequest{})
	require.NoError(t, err)
	assert.True(t, quota.Pending)

	client, _ = v0124Server(t, http.StatusOK, map[string]interface{}{
		"success": true, "namespace": "ns", "config": map[string]interface{}{}, "message": "stored",
	})
	quota, err = client.AdminSetQuota(context.Background(), "ns", SetQuotaRequest{})
	require.NoError(t, err)
	assert.False(t, quota.Pending)

	client, _ = v0124Server(t, http.StatusAccepted, map[string]interface{}{"success": true, "message": "logged"})
	revoked, err := client.DeleteNamespaceKey(context.Background(), "ns", "k1")
	require.NoError(t, err)
	assert.True(t, revoked.Pending)

	client, _ = v0124Server(t, http.StatusConflict, map[string]interface{}{
		"error": "a later change of the key took over", "code": "CONFLICT",
	})
	err = client.DeleteKey(context.Background(), "k1")
	var conflict *ConflictError
	assert.True(t, errors.As(err, &conflict))
}

func TestV0124FencedNamespaceRetriesAfterRetryAfter(t *testing.T) {
	var calls int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if atomic.AddInt32(&calls, 1) == 1 {
			w.Header().Set("Retry-After", "0")
			w.WriteHeader(http.StatusServiceUnavailable)
			_ = json.NewEncoder(w).Encode(map[string]interface{}{
				"error": "namespace ns is being torn down", "code": "SERVICE_UNAVAILABLE",
			})
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]interface{}{"name": "ns"})
	}))
	defer server.Close()
	_, err := NewClient(server.URL).CreateNamespace(context.Background(), "ns", nil)
	require.NoError(t, err)
	assert.Equal(t, int32(2), atomic.LoadInt32(&calls))

	// Without retries the 503 carries Retry-After.
	fenced := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Retry-After", "5")
		w.WriteHeader(http.StatusServiceUnavailable)
		_ = json.NewEncoder(w).Encode(map[string]interface{}{"error": "fenced", "code": "SERVICE_UNAVAILABLE"})
	}))
	defer fenced.Close()
	client := NewClient(fenced.URL)
	client.retryConfig.MaxRetries = 1
	_, err = client.CreateNamespace(context.Background(), "ns", nil)
	var serverErr *ServerError
	require.True(t, errors.As(err, &serverErr))
	assert.Equal(t, 5, serverErr.RetryAfter)
}

func TestV0124RestoreBackupPointInTime(t *testing.T) {
	client, got := v0124Server(t, 202, map[string]interface{}{"restore_id": "r1", "status": "in_progress", "backup_id": "b1"})
	at := uint64(1_700_000_000)
	resp, err := client.AdminRestoreBackup(context.Background(), RestoreBackupRequest{PointInTime: &at, TargetNamespaces: []string{"ns"}})
	require.NoError(t, err)
	assert.Equal(t, map[string]interface{}{"point_in_time": float64(at), "target_namespaces": []interface{}{"ns"}}, got.Body)
	assert.Equal(t, "b1", resp.BackupID)
}

func TestV0124QueryConsistencyAndVectors(t *testing.T) {
	client, got := v0124Server(t, 200, map[string]interface{}{"results": []interface{}{}})
	_, err := client.Query(context.Background(), "ns", []float32{1, 0}, &QueryOptions{
		TopK: 3, IncludeValues: true, Consistency: ReadConsistencyBoundedStaleness,
		StalenessConfig: &StalenessConfig{MaxStalenessMs: 2000},
	})
	require.NoError(t, err)
	assert.Equal(t, true, got.Body["include_vectors"])
	assert.NotContains(t, got.Body, "include_values")
	assert.Equal(t, "bounded_staleness", got.Body["consistency"])
	assert.Equal(t, map[string]interface{}{"max_staleness_ms": float64(2000)}, got.Body["staleness_config"])

	_, err = client.Query(context.Background(), "ns", []float32{1, 0}, &QueryOptions{TopK: 3})
	require.NoError(t, err)
	assert.NotContains(t, got.Body, "consistency")

	_, err = client.BatchQuery(context.Background(), "ns", []BatchQuerySpec{{Vector: []float32{1}, Consistency: ReadConsistencyStrong}})
	require.NoError(t, err)
	queries := got.Body["queries"].([]interface{})
	assert.Equal(t, "strong", queries[0].(map[string]interface{})["consistency"])
}

func TestV0124ConsolidateAndSummarizeMatchTheServer(t *testing.T) {
	var calls int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&calls, 1)
	}))
	defer server.Close()
	idle := NewClient(server.URL)
	_, err := idle.Consolidate(context.Background(), "a1", ConsolidateRequest{DryRun: true})
	var validation *ValidationError
	require.True(t, errors.As(err, &validation))
	_, err = idle.Summarize(context.Background(), SummarizeRequest{AgentID: "a1", MemoryIDs: []string{"m1"}, DryRun: true})
	require.True(t, errors.As(err, &validation))
	assert.Equal(t, int32(0), atomic.LoadInt32(&calls), "nothing is sent for a dry run")

	client, got := v0124Server(t, 200, map[string]interface{}{
		"consolidated_memory": map[string]interface{}{"id": "c1", "content": "merged"},
		"source_memory_ids":   []string{"m1", "m2"}, "memories_removed": 2,
	})
	resp, err := client.Consolidate(context.Background(), "a1", ConsolidateRequest{MemoryIDs: []string{"m1", "m2"}, TargetType: "semantic"})
	require.NoError(t, err)
	assert.Equal(t, []interface{}{"m1", "m2"}, got.Body["memory_ids"])
	assert.Equal(t, "semantic", got.Body["target_type"])
	assert.Equal(t, "c1", resp.ConsolidatedMemory.ID)

	client, _ = v0124Server(t, 200, map[string]interface{}{
		"summary_memory": map[string]interface{}{"id": "s1", "content": "the summary"},
		"source_count":   2,
	})
	summary, err := client.Summarize(context.Background(), SummarizeRequest{AgentID: "a1", MemoryIDs: []string{"m1", "m2"}})
	require.NoError(t, err)
	require.NotNil(t, summary.SummaryMemory)
	assert.Equal(t, "the summary", summary.Summary)
	assert.Equal(t, "s1", summary.NewMemoryID)
	assert.Equal(t, 2, summary.SourceCount)
}

func TestV0124FeedbackHistorySendsAgentID(t *testing.T) {
	client, got := v0124Server(t, 200, map[string]interface{}{
		"memory_id": "m1", "entries": []interface{}{},
	})
	_, err := client.GetMemoryFeedbackHistoryForAgent(context.Background(), "a1", "m1")
	require.NoError(t, err)
	assert.Equal(t, "/v1/memories/m1/feedback", got.Path)
	assert.Equal(t, "a1", got.Query["agent_id"][0])

	_, err = client.EvaluateTifForAgent(context.Background(), "a1", "m1")
	require.NoError(t, err)
	assert.Equal(t, "a1", got.Query["agent_id"][0])
}
