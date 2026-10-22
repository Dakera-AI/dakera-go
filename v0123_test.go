package dakera

// Server v0.12.3 wire additions: event_time (#932), the truncation report
// (#933), ignored-field warnings (#938), deduplicated memories and the
// AutoPilot dedup modes (#930), get_memory without agent_id (#938), the
// session id naming rule, audit / memory event reasons and the new error
// codes. Response shapes are copied from a v0.12.3 server.

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

func i64Ptr(v int64) *int64 { return &v }

func f32Ptr(v float32) *float32 { return &v }

func storeAnswer() map[string]interface{} {
	return map[string]interface{}{
		"memory": map[string]interface{}{
			"id": "m1", "memory_type": "episodic", "content": "c", "agent_id": "a1",
			"importance": 0.5, "tags": []string{}, "created_at": 1, "last_accessed_at": 1, "access_count": 0,
		},
		"embedding_time_ms": 3,
	}
}

// ---------------------------------------------------------------------------
// event_time
// ---------------------------------------------------------------------------

func TestV0123_EventTime_JSON(t *testing.T) {
	cases := []struct {
		in   *EventTime
		want string
	}{
		{EventTimeUnix(1684549260), `1684549260`},
		{EventTimeRFC3339("2023-05-20T02:21:00Z"), `"2023-05-20T02:21:00Z"`},
		{EventTimeAt(time.Date(2023, 5, 20, 2, 21, 0, 500, time.UTC)), `"2023-05-20T02:21:00Z"`},
		{EventTimeAt(time.Date(2023, 5, 20, 4, 21, 0, 0, time.FixedZone("CEST", 2*3600))), `"2023-05-20T04:21:00+02:00"`},
	}
	for _, tc := range cases {
		out, err := json.Marshal(tc.in)
		require.NoError(t, err)
		assert.JSONEq(t, tc.want, string(out))
	}
	assert.Equal(t, "1684549260", EventTimeUnix(1684549260).String())
	assert.Equal(t, "2023-05-20T02:21:00Z", EventTimeRFC3339("2023-05-20T02:21:00Z").String())

	var fromNumber, fromString EventTime
	require.NoError(t, json.Unmarshal([]byte(`1684549260.9`), &fromNumber))
	assert.Equal(t, "1684549260", fromNumber.String())
	require.NoError(t, json.Unmarshal([]byte(`"2023-05-20T02:21:00Z"`), &fromString))
	assert.Equal(t, "2023-05-20T02:21:00Z", fromString.String())
	assert.Error(t, json.Unmarshal([]byte(`true`), &fromString))
}

func TestV0123_StoreMemory_SendsEventTime(t *testing.T) {
	for _, tc := range []struct {
		eventTime *EventTime
		want      interface{}
	}{
		{EventTimeUnix(1684549260), float64(1684549260)},
		{EventTimeRFC3339("2023-05-20T02:21:00Z"), "2023-05-20T02:21:00Z"},
	} {
		var got captured
		server := v0122Server(t, &got, 200, storeAnswer())
		_, err := v12Client(server.URL).StoreMemory(context.Background(), "a1", StoreMemoryRequest{
			Content: "Alice moved to Paris.", EventTime: tc.eventTime,
		})
		require.NoError(t, err)
		assert.Equal(t, tc.want, got.jsonBody(t)["event_time"])
	}

	var got captured
	server := v0122Server(t, &got, 200, storeAnswer())
	_, err := v12Client(server.URL).StoreMemory(context.Background(), "a1", StoreMemoryRequest{Content: "c"})
	require.NoError(t, err)
	assert.ElementsMatch(t, []string{"agent_id", "content"}, jsonKeys(got.jsonBody(t)))
}

// The body StoreMemory sends with every optional field set holds only fields
// the server's StoreMemoryRequest knows (a v0.12.3 server answers it with no
// warnings). The deprecated Embedding is not sent; ValidFrom goes into
// metadata, where the server reads it.
func TestV0123_StoreMemory_AllFieldsAreServerFields(t *testing.T) {
	var got captured
	server := v0122Server(t, &got, 200, storeAnswer())
	_, err := v12Client(server.URL).StoreMemory(context.Background(), "go-wire", StoreMemoryRequest{
		Content:       "Alice moved to Paris in May 2023.",
		MemoryType:    "episodic",
		Importance:    f32Ptr(0.8),
		Tags:          []string{"move"},
		Metadata:      map[string]interface{}{"source": "import"},
		TTLSeconds:    intPtr(86400),
		ExpiresAt:     i64Ptr(4102444800),
		SessionID:     "chat:2023-05-20",
		Embedding:     []float32{0.1, 0.2},
		ValidFrom:     i64Ptr(1684549260),
		ID:            "go-wire-1",
		Lang:          "en",
		AttachmentRef: "sha256:ab",
		EventTime:     EventTimeUnix(1684549260),
	})
	require.NoError(t, err)
	body := got.jsonBody(t)
	assert.ElementsMatch(t, []string{
		"agent_id", "content", "memory_type", "importance", "tags", "metadata", "ttl_seconds",
		"expires_at", "session_id", "id", "lang", "attachment_ref", "event_time",
	}, jsonKeys(body))
	assert.Equal(t, map[string]interface{}{"source": "import", "valid_from": float64(1684549260)}, body["metadata"])
	assert.Equal(t, "chat:2023-05-20", body["session_id"], "session ids may contain ':'")
}

func TestV0123_StoreMemoriesBatch_SendsItemEventTime(t *testing.T) {
	var got captured
	server := v0122Server(t, &got, 200, map[string]interface{}{"stored": []interface{}{}, "stored_count": 0, "total_embedding_time_ms": 0})
	_, err := v12Client(server.URL).StoreMemoriesBatch(context.Background(), BatchStoreMemoryRequest{
		AgentID: "go-wire",
		Lang:    "en",
		Memories: []BatchStoreMemoryItem{
			{
				Content: "Dana visited Rome.", MemoryType: "episodic", Importance: f32Ptr(0.6),
				Tags: []string{"trip"}, SessionID: "chat:2023-06-01", Metadata: map[string]any{"k": "v"},
				TTLSeconds: i64Ptr(86400), ExpiresAt: i64Ptr(4102444800), ID: "go-wire-b1",
				AttachmentRef: "sha256:ab", EventTime: EventTimeUnix(1685577600),
			},
			{Content: "Eve visited Oslo.", EventTime: EventTimeRFC3339("2023-06-02T10:00:00Z")},
			{Content: "no date"},
		},
	})
	require.NoError(t, err)
	body := got.jsonBody(t)
	assert.ElementsMatch(t, []string{"agent_id", "lang", "memories"}, jsonKeys(body))
	items := body["memories"].([]interface{})
	first := items[0].(map[string]interface{})
	assert.ElementsMatch(t, []string{
		"content", "memory_type", "importance", "tags", "session_id", "metadata", "ttl_seconds",
		"expires_at", "id", "attachment_ref", "event_time",
	}, jsonKeys(first))
	assert.Equal(t, float64(1685577600), first["event_time"])
	assert.Equal(t, "2023-06-02T10:00:00Z", items[1].(map[string]interface{})["event_time"])
	assert.NotContains(t, items[2].(map[string]interface{}), "event_time")
}

// ---------------------------------------------------------------------------
// Truncation report and warnings
// ---------------------------------------------------------------------------

func TestV0123_StoreMemory_TruncationAndWarnings(t *testing.T) {
	answer := storeAnswer()
	answer["truncated"] = true
	answer["content_tokens"] = 4718
	answer["embedded_tokens"] = 512
	answer["max_tokens"] = 512
	answer["windows"] = 0
	answer["tokens_estimated"] = true
	answer["warnings"] = []string{"unknown field 'created_at' ignored"}
	var got captured
	server := v0122Server(t, &got, 200, answer)
	resp, err := v12Client(server.URL).StoreMemory(context.Background(), "a1", StoreMemoryRequest{Content: "long"})
	require.NoError(t, err)
	require.NotNil(t, resp.Memory)
	assert.Equal(t, "m1", resp.Memory.ID)
	require.NotNil(t, resp.Truncation)
	assert.Equal(t, EmbedTruncation{
		Truncated: true, ContentTokens: 4718, EmbeddedTokens: 512, MaxTokens: 512, TokensEstimated: true,
	}, *resp.Truncation)
	assert.Equal(t, []string{"unknown field 'created_at' ignored"}, resp.Warnings)

	// An older server (or content the model reads whole): no report.
	server = v0122Server(t, &got, 200, storeAnswer())
	resp, err = v12Client(server.URL).StoreMemory(context.Background(), "a1", StoreMemoryRequest{Content: "short"})
	require.NoError(t, err)
	assert.Nil(t, resp.Truncation)
	assert.Nil(t, resp.Warnings)
	require.NotNil(t, resp.EmbeddingTimeMs)
	assert.Equal(t, int64(3), *resp.EmbeddingTimeMs)
}

func TestV0123_UpdateMemory_Truncation(t *testing.T) {
	var got captured
	server := v0122Server(t, &got, 200, map[string]interface{}{
		"id": "go-wire-1", "memory_type": "episodic", "content": "long", "agent_id": "go-wire",
		"importance": 0.8, "tags": []string{"move"}, "created_at": 1, "last_accessed_at": 1, "access_count": 0,
		"truncated": true, "content_tokens": 4718, "embedded_tokens": 512, "max_tokens": 512, "windows": 3,
	})
	content := "long"
	resp, err := v12Client(server.URL).UpdateMemory(context.Background(), "go-wire", "go-wire-1", UpdateMemoryRequest{Content: &content})
	require.NoError(t, err)
	assert.Equal(t, "go-wire-1", resp.Memory.ID)
	require.NotNil(t, resp.Truncation)
	assert.Equal(t, 3, resp.Truncation.Windows)
	assert.Equal(t, 4718, resp.Truncation.ContentTokens)
	assert.False(t, resp.Truncation.TokensEstimated)

	server = v0122Server(t, &got, 200, map[string]interface{}{"id": "m2", "content": "short", "memory_type": "episodic"})
	resp, err = v12Client(server.URL).UpdateMemory(context.Background(), "a1", "m2", UpdateMemoryRequest{Content: &content})
	require.NoError(t, err)
	assert.Equal(t, "m2", resp.Memory.ID)
	assert.Nil(t, resp.Truncation)
}

func TestV0123_StoreMemoriesBatch_TruncatedAndWarnings(t *testing.T) {
	var got captured
	server := v0122Server(t, &got, 200, map[string]interface{}{
		"stored":                  []interface{}{map[string]interface{}{"id": "go-wire-long", "content": "w0", "agent_id": "go-wire"}},
		"stored_count":            1,
		"total_embedding_time_ms": 2037,
		"truncated": []interface{}{map[string]interface{}{
			"id": "go-wire-long", "truncated": true, "content_tokens": 4718, "embedded_tokens": 512, "max_tokens": 512, "windows": 0,
		}},
		"warnings": []string{"unknown field 'memories[].created_at' ignored (in 1 items)"},
	})
	resp, err := v12Client(server.URL).StoreMemoriesBatch(context.Background(), BatchStoreMemoryRequest{
		AgentID: "go-wire", Memories: []BatchStoreMemoryItem{{Content: "w0"}},
	})
	require.NoError(t, err)
	require.Len(t, resp.Truncated, 1)
	assert.Equal(t, "go-wire-long", resp.Truncated[0].ID)
	assert.True(t, resp.Truncated[0].Truncated)
	assert.Equal(t, 512, resp.Truncated[0].EmbeddedTokens)
	assert.Equal(t, []string{"unknown field 'memories[].created_at' ignored (in 1 items)"}, resp.Warnings)

	// An older server: no truncated / warnings fields.
	server = v0122Server(t, &got, 200, map[string]interface{}{"stored": []interface{}{}, "stored_count": 0, "total_embedding_time_ms": 1})
	resp, err = v12Client(server.URL).StoreMemoriesBatch(context.Background(), BatchStoreMemoryRequest{AgentID: "a"})
	require.NoError(t, err)
	assert.Nil(t, resp.Truncated)
	assert.Nil(t, resp.Warnings)
}

// Every RecallRequest field is one the server's RecallRequest knows.
func TestV0123_Recall_AllFieldsAreServerFields(t *testing.T) {
	var got captured
	server := v0122Server(t, &got, 200, map[string]interface{}{"memories": []interface{}{}})
	routing := RoutingModeHybrid
	fusion := FusionStrategyMinMax
	iterations := uint8(2)
	since, until := "2023-01-01T00:00:00Z", "2024-01-01T00:00:00Z"
	_, err := v12Client(server.URL).Recall(context.Background(), "go-wire", RecallRequest{
		Query: "where did Alice move", TopK: 5, MemoryType: "episodic", MinImportance: f32Ptr(0.1),
		SessionID: "chat:2023-05-20", Tags: []string{"move"}, ImportanceWeighted: boolPtrRH(true),
		IncludeAssociated: true, AssociatedMemoriesCap: intPtr(5), AssociatedMemoriesDepth: intPtr(2),
		AssociatedMemoriesMinWeight: f32Ptr(0.1), Since: &since, Until: &until, Routing: &routing,
		Rerank: boolPtrRH(true), RerankCandidates: intPtr(50), Fusion: &fusion, VectorWeight: f32Ptr(0.6),
		Iterations: &iterations, Neighborhood: boolPtrRH(true), NeighborhoodMode: NeighborhoodModeSeparate,
		IncludeSummaries: boolPtrRH(false), Lang: "en",
	})
	require.NoError(t, err)
	assert.ElementsMatch(t, []string{
		"agent_id", "query", "top_k", "memory_type", "min_importance", "session_id", "tags",
		"importance_weighted", "include_associated", "associated_memories_cap",
		"associated_memories_depth", "associated_memories_min_weight", "since", "until", "routing",
		"rerank", "rerank_candidates", "fusion", "vector_weight", "iterations", "neighborhood",
		"neighborhood_mode", "include_summaries", "lang",
	}, jsonKeys(got.jsonBody(t)))
}

func TestV0123_Recall_WarningsAndRerankCapped(t *testing.T) {
	var got captured
	server := v0122Server(t, &got, 200, map[string]interface{}{
		"memories":                []interface{}{},
		"query_embedding_time_ms": 4,
		"search_time_ms":          1,
		"rerank":                  map[string]interface{}{"applied": true, "candidates": 180, "capped": true},
		"rerank_report":           map[string]interface{}{"applied": true, "candidates": 180, "capped": true},
		"warnings":                []string{"unknown field 'sesion_id' ignored"},
	})
	resp, err := v12Client(server.URL).Recall(context.Background(), "a1", RecallRequest{Query: "q"})
	require.NoError(t, err)
	assert.Equal(t, []string{"unknown field 'sesion_id' ignored"}, resp.Warnings)
	require.NotNil(t, resp.RerankReport)
	assert.True(t, resp.RerankReport.Capped)
	assert.Equal(t, 180, resp.RerankReport.Candidates)

	// An agent that has stored nothing: 200 with no memories (v0.12.3).
	server = v0122Server(t, &got, 200, map[string]interface{}{
		"memories": []interface{}{}, "query_embedding_time_ms": 45, "search_time_ms": 0,
		"rerank":        map[string]interface{}{"applied": false, "candidates": 0, "skipped": "no_candidates"},
		"rerank_report": map[string]interface{}{"applied": false, "candidates": 0, "skipped": "no_candidates"},
	})
	resp, err = v12Client(server.URL).Recall(context.Background(), "empty-agent", RecallRequest{Query: "q"})
	require.NoError(t, err)
	assert.Empty(t, resp.Memories)
	assert.Nil(t, resp.Warnings)
	assert.False(t, resp.RerankReport.Capped)
	assert.Equal(t, "no_candidates", resp.RerankReport.Skipped)
}

func TestV0123_BatchRecall_AllFilterFieldsAndWarnings(t *testing.T) {
	var got captured
	server := v0122Server(t, &got, 200, map[string]interface{}{
		"memories": []interface{}{map[string]interface{}{"id": "go-wire-b1", "content": "Dana visited Rome.", "memory_type": "episodic"}},
		"total":    8, "filtered": 1, "truncated": false,
		"warnings": []string{"unknown field 'filter.sesion_id' ignored"},
	})
	resp, err := v12Client(server.URL).BatchRecall(context.Background(), BatchRecallRequest{
		AgentID: "go-wire",
		Filter: BatchMemoryFilter{
			Tags: []string{"trip"}, MinImportance: f32Ptr(0.1), MaxImportance: f32Ptr(1),
			CreatedAfter: i64Ptr(1), CreatedBefore: i64Ptr(4102444800), MemoryType: "episodic",
			SessionID: "chat:2023-06-01",
		},
		Limit: 10,
	})
	require.NoError(t, err)
	body := got.jsonBody(t)
	assert.ElementsMatch(t, []string{"agent_id", "filter", "limit"}, jsonKeys(body))
	assert.ElementsMatch(t, []string{
		"tags", "min_importance", "max_importance", "created_after", "created_before", "memory_type", "session_id",
	}, jsonKeys(body["filter"].(map[string]interface{})))
	require.Len(t, resp.Memories, 1)
	assert.Equal(t, "go-wire-b1", resp.Memories[0].ID)
	assert.Equal(t, []string{"unknown field 'filter.sesion_id' ignored"}, resp.Warnings)
}

// ---------------------------------------------------------------------------
// Deduplicated memories
// ---------------------------------------------------------------------------

func TestV0123_ListDeduplicated(t *testing.T) {
	var got captured
	server := v0122Server(t, &got, 200, map[string]interface{}{
		"agent_id": "go-wire-dd",
		"memories": []interface{}{map[string]interface{}{
			"id": "dd-2", "kind": "deduplication", "survivor_id": "dd-1", "session_id": "s1",
			"deleted_at": 1792643733, "restorable_until": 1792730133,
		}},
		"count": 1,
	})
	resp, err := v12Client(server.URL).ListDeduplicated(context.Background(), "go-wire-dd")
	require.NoError(t, err)
	assert.Equal(t, "GET", got.method)
	assert.Equal(t, "/v1/memory/deduplicated", got.path)
	assert.Equal(t, []string{"go-wire-dd"}, got.query["agent_id"])
	assert.Equal(t, "go-wire-dd", resp.AgentID)
	assert.Equal(t, 1, resp.Count)
	require.Len(t, resp.Memories, 1)
	assert.Equal(t, DeduplicatedMemory{
		ID: "dd-2", Kind: DeduplicatedKindDeduplication, SurvivorID: "dd-1", SessionID: "s1",
		DeletedAt: 1792643733, RestorableUntil: 1792730133,
	}, resp.Memories[0])

	server = v0122Server(t, &got, 200, map[string]interface{}{"agent_id": "fresh", "memories": []interface{}{}, "count": 0})
	resp, err = v12Client(server.URL).ListDeduplicated(context.Background(), "fresh")
	require.NoError(t, err)
	assert.Equal(t, 0, resp.Count)
	assert.Empty(t, resp.Memories)
}

func TestV0123_RestoreDeduplicated(t *testing.T) {
	var got captured
	server := v0122Server(t, &got, 200, map[string]interface{}{
		"id": "dd/2", "memory_type": "episodic", "content": "The office wifi password is in the blue binder!",
		"agent_id": "go-wire-dd", "session_id": "s1", "importance": 0.5, "tags": []string{},
		"created_at": 1792643733, "last_accessed_at": 1792643733, "access_count": 0,
	})
	mem, err := v12Client(server.URL).RestoreDeduplicated(context.Background(), "go-wire-dd", "dd/2")
	require.NoError(t, err)
	assert.Equal(t, "POST", got.method)
	assert.Equal(t, "/v1/memory/deduplicated/dd/2/restore", got.path)
	assert.Equal(t, []string{"go-wire-dd"}, got.query["agent_id"])
	assert.Empty(t, got.body, "restore sends no body")
	assert.Equal(t, "dd/2", mem.ID)
	assert.Equal(t, "s1", mem.SessionID)

	server = v0122Server(t, &got, 404, map[string]interface{}{
		"error": "No restorable copy of memory nope", "code": "VECTOR_NOT_FOUND", "status": 404, "resource": "memory",
	})
	_, err = v12Client(server.URL).RestoreDeduplicated(context.Background(), "a", "nope")
	var nf *NotFoundError
	require.ErrorAs(t, err, &nf)
	assert.Equal(t, "memory", nf.Resource)
	assert.Contains(t, nf.Message, "No restorable copy")

	server = v0122Server(t, &got, 409, map[string]interface{}{"error": "agent holds dd-2 again", "code": "CONFLICT"})
	_, err = v12Client(server.URL).RestoreDeduplicated(context.Background(), "a", "dd-2")
	var conflict *ConflictError
	require.ErrorAs(t, err, &conflict)
}

func TestV0123_RestoreDeduplicated_EscapesID(t *testing.T) {
	var rawPath string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		rawPath = r.URL.EscapedPath()
		v12JSON(w, 200, map[string]interface{}{"id": "x/y?z"})
	}))
	t.Cleanup(server.Close)
	_, err := v12Client(server.URL).RestoreDeduplicated(context.Background(), "a", "x/y?z")
	require.NoError(t, err)
	assert.Equal(t, "/v1/memory/deduplicated/x%2Fy%3Fz/restore", rawPath)
}

// ---------------------------------------------------------------------------
// GetMemory without agent_id
// ---------------------------------------------------------------------------

func TestV0123_GetMemory_AgentIDOptional(t *testing.T) {
	var got captured
	server := v0122Server(t, &got, 200, map[string]interface{}{"id": "go-wire-b1", "content": "Dana visited Rome.", "agent_id": "go-wire"})
	mem, err := v12Client(server.URL).GetMemory(context.Background(), "", "go-wire-b1")
	require.NoError(t, err)
	assert.Equal(t, "/v1/memory/get/go-wire-b1", got.path)
	assert.NotContains(t, got.query, "agent_id")
	assert.Equal(t, "go-wire", mem.AgentID)

	_, err = v12Client(server.URL).GetMemory(context.Background(), "go-wire", "go-wire-b1")
	require.NoError(t, err)
	assert.Equal(t, []string{"go-wire"}, got.query["agent_id"])

	server = v0122Server(t, &got, 400, map[string]interface{}{
		"error": "Invalid request: memory id dd-1 exists for 2 agents you can read (a, b); pass ?agent_id= to choose one",
		"code":  "INVALID_REQUEST", "status": 400,
	})
	_, err = v12Client(server.URL).GetMemory(context.Background(), "", "dd-1")
	var ve *ValidationError
	require.ErrorAs(t, err, &ve)
	assert.Contains(t, ve.Message, "pass ?agent_id=")
}

// ---------------------------------------------------------------------------
// AutoPilot dedup modes
// ---------------------------------------------------------------------------

func TestV0123_AutopilotStatus_NewFields(t *testing.T) {
	var got captured
	server := v0122Server(t, &got, 200, map[string]interface{}{
		"config": map[string]interface{}{
			"enabled": true, "dedup_threshold": 0.93, "dedup_interval_hours": 6, "consolidation_interval_hours": 12,
			"dedup_mode": "report", "dedup_cross_session": true, "consolidate_cross_session": false,
			"trash_retention_hours": 168,
		},
		"last_dedup_at":         1792643733,
		"last_consolidation_at": nil,
		"last_dedup":            map[string]interface{}{"namespaces_processed": 2, "memories_scanned": 10, "duplicates_removed": 0, "duplicates_found": 1},
		"last_consolidation":    nil,
		"total_dedup_removed":   0,
		"total_consolidated":    0,
	})
	status, err := v12Client(server.URL).AutopilotStatus(context.Background())
	require.NoError(t, err)
	assert.Equal(t, DedupModeReport, status.Config.DedupMode)
	assert.True(t, status.Config.DedupCrossSession)
	assert.False(t, status.Config.ConsolidateCrossSession)
	assert.Equal(t, uint64(168), status.Config.TrashRetentionHours)
	require.NotNil(t, status.LastDedup)
	assert.Equal(t, 1, status.LastDedup.DuplicatesFound)

	// An older server: the new fields stay zero.
	server = v0122Server(t, &got, 200, map[string]interface{}{
		"config": map[string]interface{}{"enabled": true, "dedup_threshold": 0.93, "dedup_interval_hours": 6, "consolidation_interval_hours": 12},
		"total_dedup_removed": 0, "total_consolidated": 0,
	})
	status, err = v12Client(server.URL).AutopilotStatus(context.Background())
	require.NoError(t, err)
	assert.Equal(t, DedupMode(""), status.Config.DedupMode)
	assert.Equal(t, uint64(0), status.Config.TrashRetentionHours)
}

func TestV0123_AutopilotUpdateConfig_NewFieldsOnlyWhenSet(t *testing.T) {
	answer := map[string]interface{}{
		"success": true, "message": "AutoPilot configuration updated",
		"config": map[string]interface{}{
			"enabled": true, "dedup_threshold": 0.93, "dedup_interval_hours": 6, "consolidation_interval_hours": 12,
			"dedup_mode": "merge", "dedup_cross_session": false, "consolidate_cross_session": true, "trash_retention_hours": 24,
		},
	}
	var got captured
	server := v0122Server(t, &got, 200, answer)
	resp, err := v12Client(server.URL).AutopilotUpdateConfig(context.Background(), AutoPilotConfigRequest{
		DedupMode: DedupModeMerge, DedupCrossSession: boolPtrRH(false), ConsolidateCrossSession: boolPtrRH(true),
		TrashRetentionHours: func() *uint64 { v := uint64(24); return &v }(),
	})
	require.NoError(t, err)
	assert.Equal(t, map[string]interface{}{
		"dedup_mode": "merge", "dedup_cross_session": false, "consolidate_cross_session": true, "trash_retention_hours": float64(24),
	}, got.jsonBody(t))
	assert.Equal(t, DedupModeMerge, resp.Config.DedupMode)
	assert.Equal(t, uint64(24), resp.Config.TrashRetentionHours)

	enabled := true
	_, err = v12Client(server.URL).AutopilotUpdateConfig(context.Background(), AutoPilotConfigRequest{Enabled: &enabled})
	require.NoError(t, err)
	assert.Equal(t, map[string]interface{}{"enabled": true}, got.jsonBody(t))
}

func TestV0123_AutopilotTrigger_DedupModeAndFound(t *testing.T) {
	var got captured
	server := v0122Server(t, &got, 200, map[string]interface{}{
		"success": true, "action": "dedup", "consolidation": nil, "message": "Dedup cycle completed",
		"dedup": map[string]interface{}{"namespaces_processed": 2, "memories_scanned": 10, "duplicates_removed": 1, "duplicates_found": 1, "mode": "merge"},
	})
	resp, err := v12Client(server.URL).AutopilotTrigger(context.Background(), "dedup")
	require.NoError(t, err)
	require.NotNil(t, resp.Dedup)
	assert.Equal(t, 1, resp.Dedup.DuplicatesFound)
	assert.Equal(t, DedupModeMerge, resp.Dedup.Mode)
}

// ---------------------------------------------------------------------------
// Capabilities, audit, events, errors
// ---------------------------------------------------------------------------

func TestV0123_CapabilitiesNamingSessionID(t *testing.T) {
	caps, err := ParseCapabilities([]byte(`{
		"capabilities_version": 2, "server_version": "0.12.3",
		"naming": {"agent_id_pattern": "^[a-zA-Z0-9][a-zA-Z0-9_\\-.]*$", "agent_id_max_bytes": 241,
		           "session_id_pattern": "^[a-zA-Z0-9][a-zA-Z0-9_\\-:.]*$", "session_id_max_bytes": 256}
	}`))
	require.NoError(t, err)
	require.NotNil(t, caps.Naming)
	assert.Equal(t, `^[a-zA-Z0-9][a-zA-Z0-9_\-:.]*$`, caps.Naming.SessionIDPattern)
	assert.Equal(t, 256, caps.Naming.SessionIDMaxBytes)

	old, err := ParseCapabilities([]byte(`{"capabilities_version": 2, "server_version": "0.12.2", "naming": {"agent_id_max_bytes": 241}}`))
	require.NoError(t, err)
	assert.Empty(t, old.Naming.SessionIDPattern)
	assert.Equal(t, 0, old.Naming.SessionIDMaxBytes)
}

func TestV0123_AuditEventReason(t *testing.T) {
	var got captured
	server := v0122Server(t, &got, 200, map[string]interface{}{
		"events": []interface{}{
			map[string]interface{}{"id": 11, "event_type": "deduplicated", "agent_id": "go-wire-dd", "memory_id": "dd-2",
				"session_id": "s1", "importance": 0.5, "timestamp": 1792643733599, "reason": "absorbed into dd-1"},
			map[string]interface{}{"id": 7, "event_type": "batch_stored", "agent_id": "go-wire", "timestamp": 1792643698786},
		},
		"count": 2,
	})
	resp, err := v12Client(server.URL).ListAuditEvents(context.Background(), AuditQuery{
		AgentID: "go-wire-dd", EventType: "deduplicated", FromTs: 1792643000000, ToTs: 1792644000000, Limit: 2,
	})
	require.NoError(t, err)
	assert.Equal(t, "/v1/audit", got.path)
	assert.Equal(t, []string{"1792643000000"}, got.query["from"])
	assert.Equal(t, []string{"1792644000000"}, got.query["to"])
	assert.Equal(t, 2, resp.Count)
	assert.Equal(t, 2, resp.Total)
	require.Len(t, resp.Events, 2)
	assert.Equal(t, "11", resp.Events[0].ID)
	assert.Equal(t, "absorbed into dd-1", resp.Events[0].Reason)
	assert.Equal(t, int64(1792643733599), resp.Events[0].Timestamp)
	assert.Empty(t, resp.Events[1].Reason)
}

func TestV0123_MemoryEventReason(t *testing.T) {
	var ev MemoryEvent
	require.NoError(t, json.Unmarshal([]byte(`{"event_type":"deduplicated","agent_id":"a","memory_id":"dd-2","timestamp":1792643733599,"reason":"absorbed into dd-1"}`), &ev))
	require.NotNil(t, ev.Reason)
	assert.Equal(t, "absorbed into dd-1", *ev.Reason)
	require.NoError(t, json.Unmarshal([]byte(`{"event_type":"restored","agent_id":"a","memory_id":"dd-2","timestamp":1}`), &ev))
	assert.Equal(t, "restored", ev.EventType)
}

func TestV0123_ErrorCodes(t *testing.T) {
	assert.Equal(t, "HOST_NOT_ALLOWED", string(ErrorCodeHostNotAllowed))
	assert.Equal(t, "UNPROCESSABLE_CONTENT", string(ErrorCodeUnprocessableContent))
	assert.Equal(t, "EXTRACTION_TIMEOUT", string(ErrorCodeExtractionTimeout))
	assert.Equal(t, "RATE_LIMIT_EXCEEDED", string(ErrorCodeRateLimitExceeded))

	// 421 has no dedicated error type: the server's message and code are kept.
	var got captured
	server := v0122Server(t, &got, 421, map[string]interface{}{
		"error": "authentication is disabled, so requests are served only for localhost (DNS rebinding protection)",
		"code":  "HOST_NOT_ALLOWED", "status": 421,
	})
	_, err := v12Client(server.URL).Health(context.Background())
	require.Error(t, err)
	var de *DakeraError
	require.ErrorAs(t, err, &de)
	assert.Equal(t, 421, de.StatusCode)
	assert.Equal(t, ErrorCodeHostNotAllowed, de.Code)
	assert.Contains(t, de.Message, "DNS rebinding protection")

	// 422 keeps the server's text and code too.
	server = v0122Server(t, &got, 422, map[string]interface{}{
		"error": "unknown field `permissions`", "code": "UNPROCESSABLE_CONTENT", "status": 422,
	})
	_, err = v12Client(server.URL).ListDeduplicated(context.Background(), "a")
	require.ErrorAs(t, err, &de)
	assert.Equal(t, ErrorCodeUnprocessableContent, de.Code)
	assert.Contains(t, de.Message, "unknown field")
}

func TestV0123_RateLimitKeepsServerMessageAndCode(t *testing.T) {
	var got captured
	server := v0122Server(t, &got, 429, map[string]interface{}{
		"error": "server overloaded: recall queue is full", "code": "RATE_LIMIT_EXCEEDED", "status": 429,
		"details": "queue=recall",
	})
	_, err := v12Client(server.URL).Recall(context.Background(), "a", RecallRequest{Query: "q"})
	var rl *RateLimitError
	require.ErrorAs(t, err, &rl)
	assert.Equal(t, ErrorCodeRateLimitExceeded, rl.Code)
	assert.Equal(t, "server overloaded: recall queue is full", rl.Message)
	assert.Equal(t, "queue=recall", rl.Details)
}
