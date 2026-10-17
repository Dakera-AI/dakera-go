package dakera

// Next-release support: recall neighbourhood modes and result provenance
// (#917), the admin routes that refuse unknown fields (#920 F10), the
// super_admin shutdown (#920 F11), and client hardening (retry safety, path
// escaping, secret redaction, event streams). Shapes follow the server's
// serializers.

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func boolPtrRH(v bool) *bool { return &v }

func jsonKeys(m map[string]interface{}) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	return keys
}

// ---------------------------------------------------------------------------
// Recall request
// ---------------------------------------------------------------------------

func TestRelease_Recall_NewRequestFieldsOmittedWhenUnset(t *testing.T) {
	var got captured
	server := v0122Server(t, &got, 200, map[string]interface{}{"memories": []interface{}{}})
	_, err := v12Client(server.URL).Recall(context.Background(), "a1", RecallRequest{Query: "q", TopK: 3})
	require.NoError(t, err)
	body := got.jsonBody(t)
	assert.Equal(t, "/v1/memory/recall", got.path)
	assert.NotContains(t, body, "neighborhood_mode")
	assert.NotContains(t, body, "include_summaries")
	assert.NotContains(t, body, "session_id")
	assert.NotContains(t, body, "rerank_candidates")
	assert.ElementsMatch(t, []string{"agent_id", "query", "top_k"}, jsonKeys(body))
}

func TestRelease_Recall_SendsNeighborhoodModeAndIncludeSummaries(t *testing.T) {
	for _, mode := range []NeighborhoodMode{NeighborhoodModeInline, NeighborhoodModeSeparate, NeighborhoodModeOff} {
		var got captured
		server := v0122Server(t, &got, 200, map[string]interface{}{"memories": []interface{}{}})
		_, err := v12Client(server.URL).Recall(context.Background(), "a1", RecallRequest{
			Query:            "q",
			NeighborhoodMode: mode,
			IncludeSummaries: boolPtrRH(false),
		})
		require.NoError(t, err)
		body := got.jsonBody(t)
		assert.Equal(t, string(mode), body["neighborhood_mode"])
		assert.Equal(t, false, body["include_summaries"])
	}
}

// ---------------------------------------------------------------------------
// Recall / search responses
// ---------------------------------------------------------------------------

func TestRelease_Recall_SeparateModeResponse(t *testing.T) {
	var got captured
	server := v0122Server(t, &got, 200, map[string]interface{}{
		"memories": []interface{}{
			map[string]interface{}{
				"memory": map[string]interface{}{
					"id": "m1", "content": "Alice moved to Paris in 2021. She likes tea.",
					"agent_id": "a1", "session_id": "s1", "memory_type": "episodic",
					"importance": 0.7, "tags": []string{}, "created_at": 1, "last_accessed_at": 1,
				},
				"score": 0.9, "smart_score": 0.8,
				"matched_text": "Alice moved to Paris in 2021.",
				"matched_ids":  []string{"m1-s0", "m1-s1"},
			},
		},
		"context_memories": []interface{}{
			map[string]interface{}{
				"memory":     map[string]interface{}{"id": "m2", "content": "ctx", "session_id": "s1"},
				"score":      0.31,
				"retrieval":  "neighborhood",
				"context_of": "m1",
			},
		},
		"query_embedding_time_ms": 4,
		"search_time_ms":          12,
		"rerank":                  map[string]interface{}{"applied": true, "candidates": 100},
		"rerank_report":           map[string]interface{}{"applied": true, "candidates": 100},
		// A field from a future server must not break decoding.
		"future_field": map[string]interface{}{"x": 1},
	})
	resp, err := v12Client(server.URL).Recall(context.Background(), "a1", RecallRequest{
		Query: "where does alice live", TopK: 1, NeighborhoodMode: NeighborhoodModeSeparate,
	})
	require.NoError(t, err)
	require.Len(t, resp.Memories, 1)
	m := resp.Memories[0]
	assert.Equal(t, "m1", m.ID)
	assert.Equal(t, "s1", m.SessionID)
	assert.Equal(t, "Alice moved to Paris in 2021.", m.MatchedText)
	assert.Equal(t, []string{"m1-s0", "m1-s1"}, m.MatchedIDs)
	assert.Empty(t, m.Retrieval)
	assert.False(t, m.IsNeighborhood())

	require.Len(t, resp.ContextMemories, 1)
	c := resp.ContextMemories[0]
	assert.Equal(t, "m2", c.ID)
	assert.Equal(t, "m1", c.ContextOf)
	assert.Equal(t, RetrievalNeighborhood, c.Retrieval)
	assert.True(t, c.IsNeighborhood())

	assert.EqualValues(t, 4, resp.QueryEmbeddingTimeMs)
	assert.EqualValues(t, 12, resp.SearchTimeMs)
	require.NotNil(t, resp.RerankReport)
	assert.True(t, resp.RerankReport.Applied)
	assert.Equal(t, 100, resp.RerankReport.Candidates)
}

func TestRelease_Recall_InlineNeighborhoodItem(t *testing.T) {
	var got captured
	server := v0122Server(t, &got, 200, map[string]interface{}{
		"memories": []interface{}{
			map[string]interface{}{"memory": map[string]interface{}{"id": "m1", "content": "a"}, "score": 0.9},
			map[string]interface{}{"memory": map[string]interface{}{"id": "m2", "content": "b"}, "score": 0.2, "retrieval": "neighborhood"},
		},
	})
	resp, err := v12Client(server.URL).Recall(context.Background(), "a1", RecallRequest{Query: "q", TopK: 1})
	require.NoError(t, err)
	require.Len(t, resp.Memories, 2)
	assert.False(t, resp.Memories[0].IsNeighborhood())
	assert.True(t, resp.Memories[1].IsNeighborhood())
	assert.Nil(t, resp.ContextMemories)
}

func TestRelease_Recall_OldServerResponse(t *testing.T) {
	// v0.12.2 shape: no matched_*, retrieval, context_*; the report under "rerank" only.
	var got captured
	server := v0122Server(t, &got, 200, map[string]interface{}{
		"memories": []interface{}{
			map[string]interface{}{"memory": map[string]interface{}{"id": "m1", "content": "a"}, "score": 0.9, "vector_score": 0.5},
		},
		"query_embedding_time_ms": 1, "search_time_ms": 2,
		"rerank": map[string]interface{}{"applied": false, "candidates": 0, "skipped": "bm25_routing"},
	})
	resp, err := v12Client(server.URL).Recall(context.Background(), "a1", RecallRequest{Query: "q"})
	require.NoError(t, err)
	require.Len(t, resp.Memories, 1)
	m := resp.Memories[0]
	assert.Empty(t, m.MatchedText)
	assert.Nil(t, m.MatchedIDs)
	assert.Empty(t, m.Retrieval)
	assert.Empty(t, m.ContextOf)
	require.NotNil(t, m.VectorScore)
	assert.InDelta(t, 0.5, *m.VectorScore, 1e-6)
	assert.Nil(t, resp.ContextMemories)
	require.NotNil(t, resp.RerankReport)
	assert.Equal(t, "bm25_routing", resp.RerankReport.Skipped)
}

func TestRelease_Search_ResultsCarryMatchedFields(t *testing.T) {
	var got captured
	answer := map[string]interface{}{
		"memories": []interface{}{
			map[string]interface{}{
				"memory": map[string]interface{}{"id": "m1", "content": "src"}, "score": 0.9,
				"matched_text": "best sentence", "matched_ids": []string{"m1-s2"},
			},
		},
		"total_count":     7,
		"rerank_report":   map[string]interface{}{"applied": false, "candidates": 0, "skipped": "disabled"},
		"effective_top_k": 5,
		"another_future":  true,
	}
	server := v0122Server(t, &got, 200, answer)
	client := v12Client(server.URL)

	detailed, err := client.SearchMemoriesDetailed(context.Background(), "a1", SearchMemoriesRequest{Query: "q", TopK: 5})
	require.NoError(t, err)
	assert.Equal(t, 7, detailed.TotalCount)
	assert.Equal(t, 5, detailed.EffectiveTopK)
	require.NotNil(t, detailed.RerankReport)
	assert.Equal(t, "disabled", detailed.RerankReport.Skipped)
	require.Len(t, detailed.Memories, 1)
	assert.Equal(t, "best sentence", detailed.Memories[0].MatchedText)
	assert.Equal(t, []string{"m1-s2"}, detailed.Memories[0].MatchedIDs)

	plain, err := client.SearchMemories(context.Background(), "a1", SearchMemoriesRequest{Query: "q"})
	require.NoError(t, err)
	require.Len(t, plain, 1)
	assert.Equal(t, "best sentence", plain[0].MatchedText)
}

func TestRelease_Search_FilterOnlyOmitsQuery(t *testing.T) {
	var got captured
	server := v0122Server(t, &got, 200, map[string]interface{}{"memories": []interface{}{}, "total_count": 0})
	_, err := v12Client(server.URL).SearchMemories(context.Background(), "a1", SearchMemoriesRequest{SortBy: "importance", TopK: 10})
	require.NoError(t, err)
	body := got.jsonBody(t)
	assert.NotContains(t, body, "query")
	assert.Equal(t, "importance", body["sort_by"])
}

// ---------------------------------------------------------------------------
// Admin requests that refuse unknown fields (422 on the server)
// ---------------------------------------------------------------------------

func TestRelease_CreateKey_SendsOnlyServerFields(t *testing.T) {
	var got captured
	server := v0122Server(t, &got, 201, map[string]interface{}{
		"key_id": "dk_key_1", "key": "dk_live_secret", "name": "k", "scope": "read",
		"created_at": 1, "warning": "save it",
	})
	_, err := v12Client(server.URL).CreateKey(context.Background(), CreateKeyRequest{Name: "k", Scope: KeyScopeRead})
	require.NoError(t, err)
	assert.Equal(t, map[string]interface{}{"name": "k", "scope": "read"}, got.jsonBody(t))

	days := 30
	_, err = v12Client(server.URL).CreateKey(context.Background(), CreateKeyRequest{
		Name: "k", Scope: KeyScopeRead, Namespaces: GrantNamespaces("a", "b*"), ExpiresInDays: &days,
	})
	require.NoError(t, err)
	body := got.jsonBody(t)
	allowed := map[string]bool{"name": true, "scope": true, "namespaces": true, "expires_in_days": true}
	for k := range body {
		assert.True(t, allowed[k], "unexpected field %q sent to POST /admin/keys", k)
	}
	assert.EqualValues(t, 30, body["expires_in_days"])
}

func TestRelease_CreateKey_RefusesLegacyFieldsWithoutSending(t *testing.T) {
	var calls int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&calls, 1)
		w.WriteHeader(201)
	}))
	defer server.Close()
	_, err := v12Client(server.URL).CreateKey(context.Background(), CreateKeyRequest{Name: "k", Scope: "read", Permissions: []string{"read"}})
	var verr *ValidationError
	require.True(t, errors.As(err, &verr), "got %v", err)
	_, err = v12Client(server.URL).CreateKey(context.Background(), CreateKeyRequest{Name: "k", Scope: "read", ExpiresAt: "2030-01-01"})
	require.True(t, errors.As(err, &verr), "got %v", err)
	assert.EqualValues(t, 0, atomic.LoadInt32(&calls))
}

func TestRelease_CreateNamespaceKey_SendsOnlyServerFields(t *testing.T) {
	var got captured
	server := v0122Server(t, &got, 201, map[string]interface{}{"key_id": "k1", "key": "dk_x", "name": "n"})
	_, err := v12Client(server.URL).CreateNamespaceKey(context.Background(), "ns-1", CreateNamespaceKeyRequest{Name: "n", Scope: "read"})
	require.NoError(t, err)
	assert.Equal(t, "/v1/namespaces/ns-1/keys", got.path)
	assert.Equal(t, map[string]interface{}{"name": "n", "scope": "read"}, got.jsonBody(t))

	days := 7
	_, err = v12Client(server.URL).CreateNamespaceKey(context.Background(), "ns-1", CreateNamespaceKeyRequest{
		Name: "n", Scope: "write", ExtraNamespaces: []string{"ns-2"}, ExpiresInDays: &days,
	})
	require.NoError(t, err)
	assert.ElementsMatch(t, []string{"name", "scope", "extra_namespaces", "expires_in_days"}, jsonKeys(got.jsonBody(t)))
}

func TestRelease_AdminCreateBackup_SendsOnlyServerFields(t *testing.T) {
	var got captured
	server := v0122Server(t, &got, 200, map[string]interface{}{
		"backup": map[string]interface{}{"backup_id": "bk-1", "name": "n", "backup_type": "full", "status": "completed", "created_at": 10},
	})
	_, err := v12Client(server.URL).AdminCreateBackup(context.Background(), CreateBackupRequest{Name: "n"})
	require.NoError(t, err)
	assert.Equal(t, map[string]interface{}{"name": "n"}, got.jsonBody(t))

	_, err = v12Client(server.URL).AdminCreateBackup(context.Background(), CreateBackupRequest{
		Name: "n", BackupType: "snapshot", Namespaces: []string{"a"}, Encrypt: boolPtrRH(true), Compression: "zstd",
	})
	require.NoError(t, err)
	assert.ElementsMatch(t, []string{"name", "backup_type", "namespaces", "encrypt", "compression"}, jsonKeys(got.jsonBody(t)))
}

func TestRelease_LegacyCreateBackup_SendsServerShape(t *testing.T) {
	var got captured
	server := v0122Server(t, &got, 200, map[string]interface{}{
		"backup": map[string]interface{}{"backup_id": "bk-9", "name": "backup-1", "backup_type": "full", "status": "completed", "created_at": 1700000000, "size_bytes": 42},
	})
	info, err := v12Client(server.URL).CreateBackup(context.Background(), true)
	require.NoError(t, err)
	body := got.jsonBody(t)
	assert.NotContains(t, body, "include_data")
	assert.ElementsMatch(t, []string{"name", "backup_type"}, jsonKeys(body))
	assert.True(t, strings.HasPrefix(body["name"].(string), "backup-"))
	assert.Equal(t, "bk-9", info.ID)
	assert.Equal(t, "1700000000", info.CreatedAt)
	assert.EqualValues(t, 42, info.SizeBytes)
}

func TestRelease_LegacyListBackups_ServerShape(t *testing.T) {
	var got captured
	server := v0122Server(t, &got, 200, map[string]interface{}{
		"backups": []interface{}{map[string]interface{}{"backup_id": "bk-1", "status": "completed", "created_at": 5}},
		"total":   1,
	})
	list, err := v12Client(server.URL).ListBackups(context.Background())
	require.NoError(t, err)
	require.Len(t, list, 1)
	assert.Equal(t, "bk-1", list[0].ID)
}

func TestRelease_RotateEncryptionKey_SendsOnlySetFields(t *testing.T) {
	var got captured
	server := v0122Server(t, &got, 200, map[string]interface{}{
		"key_id": "k2", "previous_key_id": "k1", "scope": "global", "reseal": "completed",
		"rotated": 3, "skipped": 0, "namespaces": []string{"a"}, "fulltext_indices_rotated": 1,
		"status_url": "/admin/encryption/status",
	})
	client := v12Client(server.URL)

	resp, err := client.RotateEncryptionKey(context.Background(), "", "")
	require.NoError(t, err)
	assert.Equal(t, "/v1/admin/encryption/rotate-key", got.path)
	assert.Equal(t, map[string]interface{}{}, got.jsonBody(t))
	assert.Equal(t, "k2", resp.KeyID)
	assert.Equal(t, "k1", resp.PreviousKeyID)
	assert.Equal(t, "completed", resp.Reseal)
	assert.Equal(t, 3, resp.Rotated)
	assert.Equal(t, 1, resp.FulltextIndicesRotated)

	wait := 0
	_, err = client.RotateEncryptionKeyWithOptions(context.Background(), RotateEncryptionKeyRequest{
		NewKey: "correct horse battery staple", Namespace: "ns", WaitSecs: &wait,
	})
	require.NoError(t, err)
	assert.Equal(t, map[string]interface{}{
		"new_key": "correct horse battery staple", "namespace": "ns", "wait_secs": float64(0),
	}, got.jsonBody(t))
}

func TestRelease_RotateEncryptionKeyRequest_RedactsKey(t *testing.T) {
	req := RotateEncryptionKeyRequest{NewKey: "s3cr3t-passphrase", Namespace: "ns"}
	for _, verb := range []string{"%v", "%+v", "%#v", "%s"} {
		out := fmt.Sprintf(verb, req)
		assert.NotContains(t, out, "s3cr3t", verb)
		assert.Contains(t, out, "ns", verb)
	}
}

func TestRelease_EncryptionAdminRoutes(t *testing.T) {
	var got captured
	server := v0122Server(t, &got, 200, map[string]interface{}{"reseal": "running"})
	client := v12Client(server.URL)

	_, err := client.EncryptionStatus(context.Background())
	require.NoError(t, err)
	assert.Equal(t, "GET", got.method)
	assert.Equal(t, "/v1/admin/encryption/status", got.path)

	_, err = client.ResealEncryption(context.Background(), ResealEncryptionRequest{Namespace: "ns"})
	require.NoError(t, err)
	assert.Equal(t, "POST", got.method)
	assert.Equal(t, "/v1/admin/encryption/reseal", got.path)
	assert.Equal(t, map[string]interface{}{"namespace": "ns"}, got.jsonBody(t))

	_, err = client.RevokeEncryptionMaster(context.Background(), "key 1")
	require.NoError(t, err)
	assert.Equal(t, "POST", got.method)
	assert.Equal(t, "/v1/admin/encryption/keys/key 1/revoke-master", got.path)
}

func TestRelease_OpsShutdown_AdminKeyGets403(t *testing.T) {
	var calls int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&calls, 1)
		v12JSON(w, 403, map[string]interface{}{"error": "requires super_admin scope", "code": "INSUFFICIENT_SCOPE"})
	}))
	defer server.Close()
	_, err := v12Client(server.URL).OpsShutdown(context.Background())
	var authz *AuthorizationError
	require.True(t, errors.As(err, &authz), "got %v", err)
	assert.EqualValues(t, 1, atomic.LoadInt32(&calls))
}

// ---------------------------------------------------------------------------
// Retry safety
// ---------------------------------------------------------------------------

// flakyServer fails the first `failures` requests with status, then answers 200.
func flakyServer(t *testing.T, failures int32, status int, header map[string]string, ok interface{}) (*httptest.Server, *int32) {
	t.Helper()
	var calls int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		n := atomic.AddInt32(&calls, 1)
		if n <= failures {
			for k, v := range header {
				w.Header().Set(k, v)
			}
			v12JSON(w, status, map[string]interface{}{"error": "boom"})
			return
		}
		v12JSON(w, 200, ok)
	}))
	t.Cleanup(server.Close)
	return server, &calls
}

func TestRelease_Retry_WritePostNotRetriedOn500(t *testing.T) {
	server, calls := flakyServer(t, 1, 500, nil, map[string]interface{}{"memory_id": "m1"})
	_, err := v12Client(server.URL).StoreMemory(context.Background(), "a1", StoreMemoryRequest{Content: "x"})
	var serr *ServerError
	require.True(t, errors.As(err, &serr), "got %v", err)
	assert.EqualValues(t, 1, atomic.LoadInt32(calls), "a write POST must not be replayed after a 500")
}

func TestRelease_Retry_ReadOnlyPostRetriedOn502(t *testing.T) {
	server, calls := flakyServer(t, 1, 502, nil, map[string]interface{}{"memories": []interface{}{}})
	_, err := v12Client(server.URL).Recall(context.Background(), "a1", RecallRequest{Query: "q"})
	require.NoError(t, err)
	assert.EqualValues(t, 2, atomic.LoadInt32(calls))
}

func TestRelease_Retry_GetRetriedOn500(t *testing.T) {
	server, calls := flakyServer(t, 2, 500, nil, map[string]interface{}{"status": "ok"})
	_, err := v12Client(server.URL).request(context.Background(), "GET", "/v1/ops/stats", nil)
	require.NoError(t, err)
	assert.EqualValues(t, 3, atomic.LoadInt32(calls))
}

func TestRelease_Retry_WritePostRetriedOn429And503(t *testing.T) {
	for _, status := range []int{429, 503} {
		server, calls := flakyServer(t, 1, status, map[string]string{"Retry-After": "0"}, map[string]interface{}{"memory_id": "m1"})
		_, err := v12Client(server.URL).StoreMemory(context.Background(), "a1", StoreMemoryRequest{Content: "x"})
		require.NoError(t, err, "status %d", status)
		assert.EqualValues(t, 2, atomic.LoadInt32(calls), "status %d: the server refused the request, a retry is safe", status)
	}
}

func TestRelease_RetrySafeTable(t *testing.T) {
	cases := []struct {
		method, path string
		want         bool
	}{
		{"GET", "/v1/memory/get/x?agent_id=a", true},
		{"DELETE", "/v1/namespaces/a", true},
		{"PUT", "/v1/memory/update/x", true},
		{"POST", "/v1/memory/recall", true},
		{"POST", "/v1/memory/search", true},
		{"POST", "/v1/namespaces/a/query", true},
		{"POST", "/v1/memory/store", false},
		{"POST", "/v1/namespaces/a/vectors", false},
		{"POST", "/ops/shutdown", false},
		{"POST", "/admin/keys", false},
		{"PATCH", "/admin/keys/k", false},
	}
	for _, c := range cases {
		assert.Equal(t, c.want, retrySafe(c.method, c.path), "%s %s", c.method, c.path)
	}
}

// ---------------------------------------------------------------------------
// Path escaping
// ---------------------------------------------------------------------------

func TestRelease_PathParametersAreEscaped(t *testing.T) {
	var rawPath, rawQuery string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		rawPath = r.URL.EscapedPath()
		rawQuery = r.URL.RawQuery
		v12JSON(w, 200, map[string]interface{}{"id": "x", "content": "c"})
	}))
	defer server.Close()
	_, err := v12Client(server.URL).GetMemory(context.Background(), "agent&x=1", "../../admin/keys?y")
	require.NoError(t, err)
	assert.Equal(t, "/v1/memory/get/..%2F..%2Fadmin%2Fkeys%3Fy", rawPath)
	assert.Equal(t, "agent_id=agent%26x%3D1", rawQuery)
}

// ---------------------------------------------------------------------------
// Secret redaction
// ---------------------------------------------------------------------------

func TestRelease_ClientAndOptionsNeverPrintAPIKey(t *testing.T) {
	const secret = "dk_live_TOPSECRET"
	opts := ClientOptions{BaseURL: "http://localhost:3000", APIKey: secret, Headers: map[string]string{"X-Other": "hdr-secret"}}
	client := NewClientWithOptions(opts)
	for _, verb := range []string{"%v", "%+v", "%#v", "%s"} {
		for _, v := range []interface{}{opts, &opts, client} {
			out := fmt.Sprintf(verb, v)
			assert.NotContains(t, out, secret, "%s of %T", verb, v)
			assert.NotContains(t, out, "hdr-secret", "%s of %T", verb, v)
		}
	}
	assert.Contains(t, client.String(), "<redacted>")
}

func TestRelease_ErrorsDoNotCarryAPIKey(t *testing.T) {
	const secret = "dk_live_TOPSECRET"
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		v12JSON(w, 401, map[string]interface{}{"error": "invalid api key", "code": "INVALID_API_KEY"})
	}))
	defer server.Close()
	client := NewClientWithOptions(ClientOptions{BaseURL: server.URL, APIKey: secret})
	_, err := client.Recall(context.Background(), "a", RecallRequest{Query: "q"})
	require.Error(t, err)
	assert.NotContains(t, err.Error(), secret)
	assert.NotContains(t, fmt.Sprintf("%+v", err), secret)
}

// ---------------------------------------------------------------------------
// Event streams
// ---------------------------------------------------------------------------

func TestRelease_SSE_LongLineAndCRLF(t *testing.T) {
	big := strings.Repeat("x", 200*1024) // over bufio.Scanner's 64 KiB default
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "text/event-stream", r.Header.Get("Accept"))
		w.Header().Set("Content-Type", "text/event-stream")
		fmt.Fprintf(w, ": keep-alive\r\n")
		fmt.Fprintf(w, "event: stored\r\ndata: {\"event_type\":\"stored\",\"agent_id\":\"a\",\"timestamp\":1,\"content\":%q}\r\n\r\n", big)
	}))
	defer server.Close()
	ch, err := v12Client(server.URL).StreamMemoryEvents(context.Background())
	require.NoError(t, err)
	var events []*MemoryEvent
	for res := range ch {
		require.NoError(t, res.Err)
		events = append(events, res.Event)
	}
	require.Len(t, events, 1)
	require.NotNil(t, events[0].Content)
	assert.Len(t, *events[0].Content, len(big))
}

func TestRelease_SSE_OutlivesRequestTimeout(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(200)
		w.(http.Flusher).Flush()
		time.Sleep(400 * time.Millisecond) // longer than the client's request timeout
		fmt.Fprint(w, "data: {\"type\":\"namespace_deleted\",\"namespace\":\"n\"}\n\n")
	}))
	defer server.Close()
	client := NewClientWithOptions(ClientOptions{BaseURL: server.URL, Timeout: 150 * time.Millisecond})
	ch, err := client.StreamNamespaceEvents(context.Background(), "n")
	require.NoError(t, err)
	res, ok := <-ch
	require.True(t, ok)
	require.NoError(t, res.Err)
	assert.Equal(t, "namespace_deleted", res.Event.Type)
}

func TestRelease_SSE_TypedErrors(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasPrefix(r.URL.Path, "/ops/") {
			v12JSON(w, 403, map[string]interface{}{"error": "admin required", "code": "INSUFFICIENT_SCOPE"})
			return
		}
		v12JSON(w, 401, map[string]interface{}{"error": "no key", "code": "AUTHENTICATION_REQUIRED"})
	}))
	defer server.Close()
	client := v12Client(server.URL)

	_, err := client.StreamNamespaceEvents(context.Background(), "n")
	var authn *AuthenticationError
	assert.True(t, errors.As(err, &authn), "got %v", err)

	_, err = client.StreamGlobalEvents(context.Background())
	var authz *AuthorizationError
	assert.True(t, errors.As(err, &authz), "got %v", err)

	_, err = client.StreamAuditEvents(context.Background(), "", "")
	assert.True(t, errors.As(err, &authn), "got %v", err)
}

func TestRelease_AuditStream_OutlivesRequestTimeout(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(200)
		w.(http.Flusher).Flush()
		time.Sleep(400 * time.Millisecond)
		fmt.Fprint(w, "data: {\"id\":\"e1\",\"event_type\":\"stored\",\"timestamp\":1}\n\n")
	}))
	defer server.Close()
	client := NewClientWithOptions(ClientOptions{BaseURL: server.URL, Timeout: 150 * time.Millisecond})
	ch, err := client.StreamAuditEvents(context.Background(), "", "")
	require.NoError(t, err)
	select {
	case ev, ok := <-ch:
		require.True(t, ok, "audit stream closed before its first event")
		assert.Equal(t, "e1", ev.ID)
	case <-time.After(5 * time.Second):
		t.Fatal("no audit event")
	}
}

func TestRelease_SubscribeAgentMemories_StopsOnAuthError(t *testing.T) {
	var calls int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&calls, 1)
		v12JSON(w, 401, map[string]interface{}{"error": "no key", "code": "AUTHENTICATION_REQUIRED"})
	}))
	defer server.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	ch, err := v12Client(server.URL).SubscribeAgentMemories(ctx, "a", nil)
	require.NoError(t, err)
	res, ok := <-ch
	require.True(t, ok)
	var authn *AuthenticationError
	assert.True(t, errors.As(res.Err, &authn), "got %v", res.Err)
	_, ok = <-ch
	assert.False(t, ok, "the channel is closed after a permanent error")
	assert.EqualValues(t, 1, atomic.LoadInt32(&calls))
}
