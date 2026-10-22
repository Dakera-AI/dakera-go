package dakera

// Document import (#923): shapes from the server's
// crates/common/src/types/document.rs, documents/routes.rs and the JobInfo of
// operations.rs.

import (
	"bytes"
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const acceptedJSON = `{"job_id":"job_1_7","document_id":"manual-v3","agent_id":"docs-bot","status_url":"/v1/memory/documents/jobs/job_1_7?agent_id=docs-bot"}`

func rawServer(t *testing.T, got *captured, status int, body string) *httptest.Server {
	t.Helper()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		buf := new(bytes.Buffer)
		_, _ = buf.ReadFrom(r.Body)
		got.method = r.Method
		got.path = r.URL.EscapedPath()
		got.query = r.URL.Query()
		got.body = []byte(buf.String())
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(status)
		_, _ = w.Write([]byte(body))
	}))
	t.Cleanup(server.Close)
	return server
}

func TestDocuments_ImportText_MinimalBody(t *testing.T) {
	var got captured
	server := rawServer(t, &got, 202, acceptedJSON)
	acc, err := v12Client(server.URL).ImportDocument(context.Background(), ImportDocumentRequest{
		AgentID: "docs-bot", Content: "# Title\n\nHello",
	})
	require.NoError(t, err)
	assert.Equal(t, "POST", got.method)
	assert.Equal(t, "/v1/memory/documents", got.path)
	assert.Equal(t, map[string]interface{}{"agent_id": "docs-bot", "content": "# Title\n\nHello"}, got.jsonBody(t))
	assert.Equal(t, "job_1_7", acc.JobID)
	assert.Equal(t, "manual-v3", acc.DocumentID)
	assert.Equal(t, "docs-bot", acc.AgentID)
	assert.Contains(t, acc.StatusURL, "/v1/memory/documents/jobs/job_1_7")
}

func TestDocuments_ImportText_AllFields(t *testing.T) {
	var got captured
	server := rawServer(t, &got, 202, acceptedJSON)
	imp := float32(0.8)
	ttl := uint64(3600)
	maxTok, overlap := 200, 20
	_, err := v12Client(server.URL).ImportDocument(context.Background(), ImportDocumentRequest{
		AgentID: "docs-bot", Content: "<h1>T</h1>", MediaType: MediaTypeHTML, Title: "Manual", DocumentID: "manual-v3",
		DocumentImportOptions: DocumentImportOptions{
			MemoryType: "semantic", SessionID: "s1", Importance: &imp, Tags: []string{"manual"},
			Metadata: map[string]interface{}{"team": "ops"}, TTLSeconds: &ttl, Lang: "en",
			Chunking:  &ChunkingOptions{MaxTokens: &maxTok, OverlapTokens: &overlap, Strategy: ChunkStrategyFixed},
			Decompose: true,
		},
	})
	require.NoError(t, err)
	body := got.jsonBody(t)
	assert.ElementsMatch(t, []string{
		"agent_id", "content", "media_type", "title", "document_id", "memory_type", "session_id",
		"importance", "tags", "metadata", "ttl_seconds", "lang", "chunking", "decompose",
	}, jsonKeys(body))
	assert.Equal(t, map[string]interface{}{"max_tokens": float64(200), "overlap_tokens": float64(20), "strategy": "fixed"}, body["chunking"])
	assert.Equal(t, true, body["decompose"])
}

func TestDocuments_ImportAttachment(t *testing.T) {
	var got captured
	server := rawServer(t, &got, 202, acceptedJSON)
	_, err := v12Client(server.URL).ImportAttachmentDocument(context.Background(), "library", "sha256:abc",
		ImportAttachmentDocumentRequest{AgentID: "docs-bot", DocumentID: "manual-v3",
			DocumentImportOptions: DocumentImportOptions{Tags: []string{"manual"}, Chunking: &ChunkingOptions{}}})
	require.NoError(t, err)
	assert.Equal(t, "/v1/namespaces/library/attachments/sha256:abc/document", got.path)
	assert.Equal(t, map[string]interface{}{
		"agent_id": "docs-bot", "document_id": "manual-v3", "tags": []interface{}{"manual"}, "chunking": map[string]interface{}{},
	}, got.jsonBody(t))
}

func TestDocuments_JobStatusAndWait(t *testing.T) {
	var calls int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/v1/memory/documents/jobs/job_1_7", r.URL.Path)
		assert.Equal(t, "docs-bot", r.URL.Query().Get("agent_id"))
		w.Header().Set("Content-Type", "application/json")
		switch atomic.AddInt32(&calls, 1) {
		case 1:
			_, _ = w.Write([]byte(`{"id":"job_1_7","job_type":"document_import","status":"Pending","created_at":1,"started_at":null,"completed_at":null,"progress":0,"message":null,"metadata":{"agent_id":"docs-bot","document_id":"manual-v3"}}`))
		case 2:
			_, _ = w.Write([]byte(`{"id":"job_1_7","job_type":"document_import","status":"Running","created_at":1,"started_at":2,"completed_at":null,"progress":40,"message":"chunks_stored 32/80","metadata":{}}`))
		default:
			_, _ = w.Write([]byte(`{"id":"job_1_7","job_type":"document_import","status":"Completed","created_at":1,"started_at":2,"completed_at":3,"progress":100,"message":"80 chunks","metadata":{},"future":1}`))
		}
	}))
	defer server.Close()
	client := v12Client(server.URL)
	job, err := client.GetDocumentJob(context.Background(), "docs-bot", "job_1_7")
	require.NoError(t, err)
	assert.Equal(t, "Pending", job.Status)
	assert.Equal(t, "manual-v3", job.Metadata["document_id"])

	done, err := client.WaitForDocumentJob(context.Background(), "docs-bot", "job_1_7", JobWaitOptions{PollInterval: time.Millisecond, Timeout: 5 * time.Second})
	require.NoError(t, err)
	assert.Equal(t, JobStatusCompleted, done.Status)
	assert.EqualValues(t, 100, done.Progress)
}

func TestDocuments_WaitFailedJob(t *testing.T) {
	var got captured
	server := rawServer(t, &got, 200, `{"id":"j","job_type":"document_import","status":"Failed","created_at":1,"progress":10,"message":"extraction timed out","metadata":{},"error":{"status":504,"code":"EXTRACTION_TIMEOUT"}}`)
	job, err := v12Client(server.URL).WaitForDocumentJob(context.Background(), "a", "j", JobWaitOptions{PollInterval: time.Millisecond})
	var failed *JobFailedError
	require.True(t, errors.As(err, &failed), "got %v", err)
	require.NotNil(t, job.Error)
	assert.Equal(t, 504, job.Error.Status)
	assert.EqualValues(t, "EXTRACTION_TIMEOUT", job.Error.Code)
}

func TestDocuments_WaitTimesOut(t *testing.T) {
	var got captured
	server := rawServer(t, &got, 200, `{"id":"j","job_type":"document_import","status":"Running","created_at":1,"progress":5,"metadata":{}}`)
	_, err := v12Client(server.URL).WaitForDocumentJob(context.Background(), "a", "j", JobWaitOptions{PollInterval: 5 * time.Millisecond, Timeout: 30 * time.Millisecond})
	var timeout *TimeoutError
	assert.True(t, errors.As(err, &timeout), "got %v", err)
}

func TestDocuments_ListGetDelete(t *testing.T) {
	var got captured
	server := rawServer(t, &got, 200, `{"documents":[{"document_id":"a-doc","title":"A","media_type":"text/markdown","chunk_count":3,"bytes":1200,"created_at":1,"updated_at":2,"sha256":"ab"},{"document_id":"b-doc","media_type":"application/pdf","chunk_count":9,"bytes":99,"pages":4,"created_at":1,"updated_at":1,"sha256":"cd","attachment_ref":"sha256:cd"}],"next_cursor":"b-doc"}`)
	client := v12Client(server.URL)
	list, err := client.ListDocuments(context.Background(), "docs bot", &ListDocumentsOptions{Cursor: "a&b", Limit: 2})
	require.NoError(t, err)
	assert.Equal(t, "/v1/memory/documents", got.path)
	assert.Equal(t, "docs bot", got.query["agent_id"][0])
	assert.Equal(t, "a&b", got.query["cursor"][0])
	assert.Equal(t, "2", got.query["limit"][0])
	require.Len(t, list.Documents, 2)
	assert.Equal(t, "b-doc", list.NextCursor)
	require.NotNil(t, list.Documents[1].Pages)
	assert.EqualValues(t, 4, *list.Documents[1].Pages)
	assert.Equal(t, "sha256:cd", list.Documents[1].AttachmentRef)

	server = rawServer(t, &got, 200, `{"documents":[]}`)
	last, err := v12Client(server.URL).ListDocuments(context.Background(), "a", nil)
	require.NoError(t, err)
	assert.Empty(t, last.NextCursor)
	_, hasLimit := got.query["limit"]
	assert.False(t, hasLimit)

	server = rawServer(t, &got, 200, `{"document":{"document_id":"x/y","media_type":"text/plain","chunk_count":2,"bytes":10,"created_at":1,"updated_at":1,"sha256":"ef"},"chunks":[{"id":"doc_1","chunk_index":0,"heading_path":["T"],"char_start":0,"char_end":5},{"id":"doc_2","chunk_index":1,"page_start":2,"page_end":3,"heading_path":["T","A"],"char_start":5,"char_end":10}]}`)
	detail, err := v12Client(server.URL).GetDocument(context.Background(), "a", "x/y")
	require.NoError(t, err)
	assert.Equal(t, "GET", got.method)
	assert.Equal(t, "/v1/memory/documents/x%2Fy", got.path)
	require.Len(t, detail.Chunks, 2)
	assert.Equal(t, []string{"T", "A"}, detail.Chunks[1].HeadingPath)
	require.NotNil(t, detail.Chunks[1].PageEnd)
	assert.EqualValues(t, 3, *detail.Chunks[1].PageEnd)

	server = rawServer(t, &got, 200, `{"document_id":"manual-v3","deleted_chunks":212}`)
	del, err := v12Client(server.URL).DeleteDocument(context.Background(), "a", "manual-v3")
	require.NoError(t, err)
	assert.Equal(t, "DELETE", got.method)
	assert.Equal(t, "/v1/memory/documents/manual-v3", got.path)
	assert.Equal(t, 212, del.DeletedChunks)
}

func TestDocuments_OldServer404SaysWhy(t *testing.T) {
	var got captured
	server := rawServer(t, &got, 404, `{"error":"No route for POST /v1/memory/documents","code":"ROUTE_NOT_FOUND"}`)
	_, err := v12Client(server.URL).ImportDocument(context.Background(), ImportDocumentRequest{AgentID: "a", Content: "x"})
	var nf *NotFoundError
	require.True(t, errors.As(err, &nf), "got %v", err)
	assert.Contains(t, err.Error(), "0.12.3")
}

func TestDocuments_FeatureDisabled(t *testing.T) {
	var got captured
	server := rawServer(t, &got, 501, `{"error":"document import is disabled","code":"FEATURE_DISABLED","details":"DAKERA_DOCUMENTS"}`)
	_, err := v12Client(server.URL).ImportDocument(context.Background(), ImportDocumentRequest{AgentID: "a", Content: "x"})
	var fd *FeatureDisabledError
	assert.True(t, errors.As(err, &fd), "got %v", err)
}

func TestDocuments_ChunkMetaFromRecall(t *testing.T) {
	var got captured
	server := rawServer(t, &got, 200, `{"memories":[
	  {"memory":{"id":"doc_9","content":"Field Manual › Install\n\nRun it.","metadata":{"team":"ops","_dakera_document":{
	    "id":"manual-v3","title":"Field Manual","media_type":"application/pdf","chunk_index":41,"chunk_count":212,
	    "page_start":39,"page_end":40,"heading_path":["Field Manual"],"char_start":80412,"char_end":81733,
	    "sha256":"ab","bytes":1843311,"pages":212,"created_at":1792252596,"updated_at":1792252596,"fingerprint":"f","later":true}}},"score":0.9},
	  {"memory":{"id":"m2","content":"plain"},"score":0.5}]}`)
	resp, err := v12Client(server.URL).Recall(context.Background(), "a", RecallRequest{Query: "install"})
	require.NoError(t, err)
	meta := resp.Memories[0].DocumentChunk()
	require.NotNil(t, meta)
	assert.Equal(t, "manual-v3", meta.ID)
	assert.Equal(t, "Field Manual", meta.Title)
	assert.Equal(t, 41, meta.ChunkIndex)
	require.NotNil(t, meta.PageStart)
	assert.EqualValues(t, 39, *meta.PageStart)
	assert.Equal(t, []string{"Field Manual"}, meta.HeadingPath)
	assert.Nil(t, resp.Memories[1].DocumentChunk())
	assert.Nil(t, DocumentChunkMetaOf(nil))
	assert.Nil(t, DocumentChunkMetaOf(map[string]interface{}{DocumentMetaKey: "not an object"}))
}

func TestDocuments_Capabilities(t *testing.T) {
	caps, err := ParseCapabilities([]byte(`{"server_version":"0.12.3","documents":{"enabled":true,
	  "text_media_types":["text/plain","text/markdown","text/html"],"media_types":["application/pdf"],
	  "max_bytes":26214400,"max_text_bytes":20971520,"max_chunks":5000,"default_chunk_tokens":400,
	  "default_overlap_tokens":50,"strategies":["structure","fixed","future"]}}`))
	require.NoError(t, err)
	require.True(t, caps.SupportsDocuments())
	assert.EqualValues(t, 26214400, caps.Documents.MaxBytes)
	assert.Equal(t, 5000, caps.Documents.MaxChunks)
	assert.Equal(t, []ChunkStrategy{ChunkStrategyStructure, ChunkStrategyFixed, "future"}, caps.Documents.Strategies)

	old, err := ParseCapabilities([]byte(`{"server_version":"0.12.2"}`))
	require.NoError(t, err)
	assert.False(t, old.SupportsDocuments())
}
