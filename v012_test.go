package dakera

// Server v0.12.0 support: Retry-After, JSON error mapping (413 / 501 / 409),
// readiness probes, PUT-based clearing of entity_types (TRACKER K34),
// attachments, jobs, records and the per-request lang field.

import (
	"context"
	"encoding/json"
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

func v12JSON(w http.ResponseWriter, status int, v interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func v12Client(url string) *Client {
	return NewClientWithOptions(ClientOptions{
		BaseURL: url,
		RetryBackoff: &RetryConfig{
			MaxRetries: 3,
			BaseDelay:  time.Millisecond,
			MaxDelay:   5 * time.Second,
			Jitter:     false,
		},
	})
}

// ---------------------------------------------------------------------------
// Retry-After
// ---------------------------------------------------------------------------

func TestV012_503RetryAfterIsHonoured(t *testing.T) {
	var calls int32
	var first, second time.Time
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		n := atomic.AddInt32(&calls, 1)
		if n == 1 {
			first = time.Now()
			w.Header().Set("Retry-After", "1")
			v12JSON(w, 503, map[string]interface{}{"error": "Service unavailable", "code": "SERVICE_UNAVAILABLE", "details": "starting"})
			return
		}
		second = time.Now()
		v12JSON(w, 200, map[string]interface{}{"upserted_count": 2})
	}))
	defer server.Close()

	// BaseDelay is 1ms: only Retry-After can explain a ~1s gap.
	resp, err := v12Client(server.URL).Upsert(context.Background(), "ns", []VectorInput{{ID: "a", Values: []float32{1}}})
	require.NoError(t, err)
	assert.Equal(t, 2, resp.UpsertedCount)
	assert.Equal(t, int32(2), atomic.LoadInt32(&calls))
	assert.GreaterOrEqual(t, second.Sub(first), 900*time.Millisecond)
}

func TestV012_503WithoutRetryAfterBacksOff(t *testing.T) {
	var calls int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&calls, 1)
		v12JSON(w, 503, map[string]interface{}{"error": "Service unavailable", "code": "SERVICE_UNAVAILABLE"})
	}))
	defer server.Close()

	_, err := v12Client(server.URL).Query(context.Background(), "ns", []float32{1}, nil)
	require.Error(t, err)
	var serverErr *ServerError
	require.ErrorAs(t, err, &serverErr)
	assert.Equal(t, 503, serverErr.StatusCode)
	assert.Equal(t, -1, serverErr.RetryAfter)
	assert.Equal(t, ErrorCodeServiceUnavailable, serverErr.Code)
	assert.Equal(t, int32(3), atomic.LoadInt32(&calls))
}

func TestV012_503ExposesRetryAfterOnTheError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Retry-After", "7")
		v12JSON(w, 503, map[string]interface{}{"error": "Service unavailable", "code": "SERVICE_UNAVAILABLE"})
	}))
	defer server.Close()

	client := NewClientWithOptions(ClientOptions{BaseURL: server.URL, MaxRetries: 1})
	_, err := client.Query(context.Background(), "ns", []float32{1}, nil)
	var serverErr *ServerError
	require.ErrorAs(t, err, &serverErr)
	assert.Equal(t, 7, serverErr.RetryAfter)
}

func TestV012_RetryAfterIsCappedAtMaxDelay(t *testing.T) {
	var calls int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if atomic.AddInt32(&calls, 1) == 1 {
			w.Header().Set("Retry-After", "3600")
			v12JSON(w, 503, map[string]interface{}{"error": "busy"})
			return
		}
		v12JSON(w, 200, map[string]interface{}{"upserted_count": 1})
	}))
	defer server.Close()

	client := NewClientWithOptions(ClientOptions{
		BaseURL:      server.URL,
		RetryBackoff: &RetryConfig{MaxRetries: 2, BaseDelay: time.Millisecond, MaxDelay: 50 * time.Millisecond},
	})
	start := time.Now()
	_, err := client.Upsert(context.Background(), "ns", []VectorInput{{ID: "a", Values: []float32{1}}})
	require.NoError(t, err)
	assert.Less(t, time.Since(start), 2*time.Second)
}

func TestV012_RetryWaitIsCancelledByContext(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Retry-After", "30")
		v12JSON(w, 503, map[string]interface{}{"error": "busy"})
	}))
	defer server.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()
	start := time.Now()
	_, err := v12Client(server.URL).Query(ctx, "ns", []float32{1}, nil)
	require.Error(t, err)
	assert.True(t, IsTimeoutError(err))
	assert.Less(t, time.Since(start), 5*time.Second)
}

func TestV012_ParseRetryAfter(t *testing.T) {
	assert.Equal(t, 5, parseRetryAfter("5"))
	assert.Equal(t, 0, parseRetryAfter("0"))
	assert.Equal(t, -1, parseRetryAfter(""))
	assert.Equal(t, -1, parseRetryAfter("soon"))
	assert.Equal(t, -1, parseRetryAfter("-3"))
	future := time.Now().Add(10 * time.Second).UTC().Format(http.TimeFormat)
	got := parseRetryAfter(future)
	assert.GreaterOrEqual(t, got, 8)
	assert.LessOrEqual(t, got, 11)
}

// ---------------------------------------------------------------------------
// Error mapping
// ---------------------------------------------------------------------------

func TestV012_413QuotaIsDistinctFromOversize(t *testing.T) {
	var calls int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&calls, 1)
		if strings.HasSuffix(r.URL.Path, "/vectors") {
			v12JSON(w, 413, map[string]interface{}{"error": "Quota exceeded for namespace 'ns'", "code": "QUOTA_EXCEEDED", "details": "namespace: ns, reason: hard quota"})
			return
		}
		v12JSON(w, 413, map[string]interface{}{"error": "attachment over DAKERA_ATTACHMENT_MAX_BYTES (26214400 bytes)", "code": "PAYLOAD_TOO_LARGE"})
	}))
	defer server.Close()
	client := v12Client(server.URL)

	_, err := client.Upsert(context.Background(), "ns", []VectorInput{{ID: "a", Values: []float32{1}}})
	require.Error(t, err)
	assert.True(t, IsPayloadTooLargeError(err))
	assert.True(t, IsQuotaExceededError(err))
	var tooLarge *PayloadTooLargeError
	require.ErrorAs(t, err, &tooLarge)
	assert.True(t, tooLarge.IsQuota())
	assert.Equal(t, 413, tooLarge.StatusCode)
	assert.Equal(t, ErrorCodeQuotaExceeded, tooLarge.Code)
	assert.Equal(t, "namespace: ns, reason: hard quota", tooLarge.Details)

	_, err = client.UploadAttachment(context.Background(), "ns", []byte("x"), "audio/wav")
	require.Error(t, err)
	assert.True(t, IsPayloadTooLargeError(err))
	assert.False(t, IsQuotaExceededError(err))
	require.ErrorAs(t, err, &tooLarge)
	assert.False(t, tooLarge.IsQuota())
	assert.Equal(t, ErrorCodePayloadTooLarge, tooLarge.Code)
	assert.Contains(t, tooLarge.Error(), "PayloadTooLargeError")

	// 413 is final: never retried.
	assert.Equal(t, int32(2), atomic.LoadInt32(&calls))
}

func TestV012_501FeatureDisabledAndNotImplemented(t *testing.T) {
	var calls int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&calls, 1)
		if strings.Contains(r.URL.Path, "/records") {
			v12JSON(w, 501, map[string]interface{}{"error": "The records API is not enabled on this server", "code": "FEATURE_DISABLED", "details": "set DAKERA_RECORDS to enable it"})
			return
		}
		v12JSON(w, 501, map[string]interface{}{"error": "operation not supported by the configured backend", "code": "NOT_IMPLEMENTED", "details": "use the filesystem backend"})
	}))
	defer server.Close()
	client := v12Client(server.URL)

	_, err := client.UpsertRecords(context.Background(), "ns", []RecordInput{{ID: "r", Values: []float32{1}}})
	require.Error(t, err)
	assert.True(t, IsFeatureDisabledError(err))
	assert.False(t, IsNotImplementedError(err))
	var disabled *FeatureDisabledError
	require.ErrorAs(t, err, &disabled)
	assert.Equal(t, ErrorCodeFeatureDisabled, disabled.Code)
	assert.Contains(t, disabled.Error(), "DAKERA_RECORDS")

	_, err = client.GetIndexStats(context.Background(), "ns")
	require.Error(t, err)
	assert.True(t, IsNotImplementedError(err))
	assert.False(t, IsFeatureDisabledError(err))
	assert.False(t, IsServerError(err))

	// 501 is final: never retried (it used to be retried as a generic 5xx).
	assert.Equal(t, int32(2), atomic.LoadInt32(&calls))
}

func TestV012_404CarriesResourceAndJobNotFoundCode(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		v12JSON(w, 404, map[string]interface{}{"error": "transcription job 'job_1_0' is unknown: the server restarted", "code": "JOB_NOT_FOUND", "resource": "job"})
	}))
	defer server.Close()

	_, err := v12Client(server.URL).GetTranscriptionJob(context.Background(), "ns", "sha256:abc", "job_1_0")
	require.Error(t, err)
	require.True(t, IsNotFoundError(err))
	nf := err.(*NotFoundError)
	assert.Equal(t, ErrorCodeJobNotFound, nf.Code)
	assert.Equal(t, "job", nf.Resource)
	body, ok := nf.ResponseBody.(ErrorBody)
	require.True(t, ok)
	assert.Equal(t, "job", body.Resource)
}

func TestV012_409IsConflictError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "DELETE", r.Method)
		v12JSON(w, 409, map[string]interface{}{"error": "attachment is referenced by a memory", "code": "CONFLICT", "details": "forget the memory instead"})
	}))
	defer server.Close()

	err := v12Client(server.URL).DeleteAttachment(context.Background(), "ns", "sha256:abc")
	require.Error(t, err)
	assert.True(t, IsConflictError(err))
	assert.Equal(t, ErrorCodeConflict, err.(*ConflictError).Code)
}

func TestV012_NonJSONErrorBodyStillMaps(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(413)
		_, _ = w.Write([]byte("plain text from a proxy"))
	}))
	defer server.Close()

	_, err := v12Client(server.URL).Upsert(context.Background(), "ns", nil)
	require.Error(t, err)
	assert.True(t, IsPayloadTooLargeError(err))
	assert.Contains(t, err.Error(), "HTTP 413")
}

// ---------------------------------------------------------------------------
// Health: ready / live
// ---------------------------------------------------------------------------

func TestV012_HealthReadyStartingIsAnErrorNotHealthy(t *testing.T) {
	var calls int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&calls, 1)
		assert.Equal(t, "/health/ready", r.URL.Path)
		w.Header().Set("Retry-After", "5")
		v12JSON(w, 503, map[string]interface{}{"ready": false, "version": "0.12.0", "starting": true, "reason": "loading models", "downloads": []string{}})
	}))
	defer server.Close()
	client := v12Client(server.URL)

	resp, err := client.HealthReady(context.Background())
	require.Error(t, err)
	assert.Nil(t, resp)
	var serverErr *ServerError
	require.ErrorAs(t, err, &serverErr)
	assert.Equal(t, 503, serverErr.StatusCode)
	assert.Equal(t, 5, serverErr.RetryAfter)
	assert.Contains(t, serverErr.Message, "loading models")
	// A probe is one attempt: the 503 is the answer.
	assert.Equal(t, int32(1), atomic.LoadInt32(&calls))

	ready, err := client.IsReady(context.Background())
	require.NoError(t, err)
	assert.False(t, ready)
}

func TestV012_IsReadyTrueOn200(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		v12JSON(w, 200, map[string]interface{}{"ready": true, "version": "0.12.0", "checks": map[string]interface{}{"storage": map[string]interface{}{"status": "ok"}}})
	}))
	defer server.Close()
	ready, err := v12Client(server.URL).IsReady(context.Background())
	require.NoError(t, err)
	assert.True(t, ready)
}

func TestV012_IsReadyReturnsConnectionErrors(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	url := server.URL
	server.Close()

	ready, err := NewClientWithOptions(ClientOptions{BaseURL: url, MaxRetries: 1}).IsReady(context.Background())
	require.Error(t, err)
	assert.False(t, ready)
	assert.True(t, IsConnectionError(err))
}

func TestV012_WaitUntilReadyPollsThroughStarting(t *testing.T) {
	var calls int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if atomic.AddInt32(&calls, 1) < 3 {
			w.Header().Set("Retry-After", "5")
			v12JSON(w, 503, map[string]interface{}{"ready": false, "starting": true, "reason": "loading models"})
			return
		}
		v12JSON(w, 200, map[string]interface{}{"ready": true, "version": "0.12.0"})
	}))
	defer server.Close()

	resp, err := v12Client(server.URL).WaitUntilReady(context.Background(), ReadyWaitOptions{Timeout: 5 * time.Second, PollInterval: 5 * time.Millisecond})
	require.NoError(t, err)
	assert.True(t, resp.Ready)
	assert.Equal(t, int32(3), atomic.LoadInt32(&calls))
}

func TestV012_WaitUntilReadyTimesOut(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		v12JSON(w, 503, map[string]interface{}{"ready": false, "starting": true, "reason": "loading models"})
	}))
	defer server.Close()

	_, err := v12Client(server.URL).WaitUntilReady(context.Background(), ReadyWaitOptions{Timeout: 100 * time.Millisecond, PollInterval: 10 * time.Millisecond})
	require.Error(t, err)
	assert.True(t, IsTimeoutError(err))
	assert.Contains(t, err.Error(), "loading models")
}

func TestV012_WaitUntilReadyStopsOnUnexpectedStatus(t *testing.T) {
	var calls int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&calls, 1)
		v12JSON(w, 404, map[string]interface{}{"error": "no route", "code": "ROUTE_NOT_FOUND"})
	}))
	defer server.Close()

	_, err := v12Client(server.URL).WaitUntilReady(context.Background(), ReadyWaitOptions{Timeout: 2 * time.Second, PollInterval: 5 * time.Millisecond})
	require.Error(t, err)
	assert.True(t, IsNotFoundError(err))
	assert.Equal(t, int32(1), atomic.LoadInt32(&calls))
}

func TestV012_HealthLiveAnswersWhileStarting(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/health/live", r.URL.Path)
		v12JSON(w, 200, map[string]interface{}{"alive": true, "version": "0.12.0", "uptime_seconds": 3, "starting": true})
	}))
	defer server.Close()
	live, err := v12Client(server.URL).HealthLive(context.Background())
	require.NoError(t, err)
	assert.True(t, live.Alive)
	assert.Equal(t, int64(3), live.UptimeSeconds)
}

// ---------------------------------------------------------------------------
// K34: clearing entity_types
// ---------------------------------------------------------------------------

type v12Captured struct {
	Method string
	Path   string
	Body   map[string]interface{}
}

func v12Capture(t *testing.T, status func(method string) int) (*httptest.Server, *[]v12Captured) {
	t.Helper()
	var captured []v12Captured
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		raw, _ := io.ReadAll(r.Body)
		var body map[string]interface{}
		_ = json.Unmarshal(raw, &body)
		captured = append(captured, v12Captured{Method: r.Method, Path: r.URL.Path, Body: body})
		code := status(r.Method)
		if code != 200 {
			v12JSON(w, code, map[string]interface{}{"error": "method not allowed", "code": "METHOD_NOT_ALLOWED"})
			return
		}
		v12JSON(w, 200, map[string]interface{}{"namespace": "ns", "extract_entities": true, "entity_types": []string{}})
	}))
	return server, &captured
}

func TestV012_ConfigureNamespaceNerClearsEntityTypesWithPUT(t *testing.T) {
	server, captured := v12Capture(t, func(string) int { return 200 })
	defer server.Close()

	_, err := v12Client(server.URL).ConfigureNamespaceNer(context.Background(), "ns", NamespaceNerConfig{ExtractEntities: true})
	require.NoError(t, err)
	require.Len(t, *captured, 1)
	got := (*captured)[0]
	assert.Equal(t, "PUT", got.Method)
	assert.Equal(t, "/v1/namespaces/ns/config", got.Path)
	assert.Equal(t, true, got.Body["extract_entities"])
	types, present := got.Body["entity_types"]
	require.True(t, present, "entity_types must be sent so the list is cleared")
	assert.Equal(t, []interface{}{}, types)
}

func TestV012_ConfigureNamespaceNerWithTypesStaysPATCH(t *testing.T) {
	server, captured := v12Capture(t, func(string) int { return 200 })
	defer server.Close()

	_, err := v12Client(server.URL).ConfigureNamespaceNer(context.Background(), "ns", NamespaceNerConfig{ExtractEntities: true, EntityTypes: []string{"person"}})
	require.NoError(t, err)
	require.Len(t, *captured, 1)
	assert.Equal(t, "PATCH", (*captured)[0].Method)
	assert.Equal(t, []interface{}{"person"}, (*captured)[0].Body["entity_types"])
}

func TestV012_ConfigureNamespaceNerFallsBackToPATCHOnPre012Server(t *testing.T) {
	// v0.11.108 answers PUT /config with 405; its PATCH replaces the config.
	server, captured := v12Capture(t, func(method string) int {
		if method == "PUT" {
			return 405
		}
		return 200
	})
	defer server.Close()

	_, err := v12Client(server.URL).ConfigureNamespaceNer(context.Background(), "ns", NamespaceNerConfig{ExtractEntities: false})
	require.NoError(t, err)
	require.Len(t, *captured, 2)
	assert.Equal(t, "PUT", (*captured)[0].Method)
	assert.Equal(t, "PATCH", (*captured)[1].Method)
	assert.Equal(t, []interface{}{}, (*captured)[1].Body["entity_types"])
	assert.Equal(t, false, (*captured)[1].Body["extract_entities"])
}

func TestV012_PutNamespaceEntityConfig(t *testing.T) {
	server, captured := v12Capture(t, func(string) int { return 200 })
	defer server.Close()

	resp, err := v12Client(server.URL).PutNamespaceEntityConfig(context.Background(), "ns", NamespaceNerConfig{ExtractEntities: true, EntityTypes: nil})
	require.NoError(t, err)
	assert.Equal(t, "ns", resp.Namespace)
	assert.True(t, resp.ExtractEntities)
	require.Len(t, *captured, 1)
	assert.Equal(t, "PUT", (*captured)[0].Method)
	assert.Equal(t, []interface{}{}, (*captured)[0].Body["entity_types"])

	// PUT is strict: an old server's 405 is returned, not papered over.
	old, _ := v12Capture(t, func(string) int { return 405 })
	defer old.Close()
	_, err = v12Client(old.URL).PutNamespaceEntityConfig(context.Background(), "ns", NamespaceNerConfig{})
	require.Error(t, err)
}

func TestV012_PatchNamespaceEntityConfigIsAMerge(t *testing.T) {
	server, captured := v12Capture(t, func(string) int { return 200 })
	defer server.Close()
	client := v12Client(server.URL)

	enable := true
	_, err := client.PatchNamespaceEntityConfig(context.Background(), "ns", NamespaceEntityConfigPatch{ExtractEntities: &enable})
	require.NoError(t, err)
	_, hasTypes := (*captured)[0].Body["entity_types"]
	assert.False(t, hasTypes, "an unset field must not be sent: PATCH merges")
	assert.Equal(t, true, (*captured)[0].Body["extract_entities"])

	cleared := []string{}
	_, err = client.PatchNamespaceEntityConfig(context.Background(), "ns", NamespaceEntityConfigPatch{EntityTypes: &cleared})
	require.NoError(t, err)
	assert.Equal(t, []interface{}{}, (*captured)[1].Body["entity_types"])
	_, hasExtract := (*captured)[1].Body["extract_entities"]
	assert.False(t, hasExtract)
	assert.Equal(t, "PATCH", (*captured)[1].Method)
}

// ---------------------------------------------------------------------------
// Attachments
// ---------------------------------------------------------------------------

const v12Ref = "sha256:e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855"

func TestV012_UploadAttachmentSendsRawBodyWithContentType(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "POST", r.Method)
		assert.Equal(t, "/v1/namespaces/_dakera_agent_a1/attachments", r.URL.Path)
		assert.Equal(t, "audio/wav", r.Header.Get("Content-Type"))
		assert.Equal(t, "Bearer k", r.Header.Get("Authorization"))
		assert.Equal(t, "dakera-go/"+Version, r.Header.Get("User-Agent"))
		raw, _ := io.ReadAll(r.Body)
		assert.Equal(t, []byte("RIFFdata"), raw)
		v12JSON(w, 201, map[string]interface{}{"attachment_ref": v12Ref, "content_type": "audio/wav", "size_bytes": 8, "created": true})
	}))
	defer server.Close()

	client := NewClientWithOptions(ClientOptions{BaseURL: server.URL, APIKey: "k"})
	resp, err := client.UploadAttachment(context.Background(), AgentMemoryNamespace("a1"), []byte("RIFFdata"), "audio/wav")
	require.NoError(t, err)
	assert.Equal(t, v12Ref, resp.AttachmentRef)
	assert.Equal(t, uint64(8), resp.SizeBytes)
	assert.True(t, resp.Created)
}

func TestV012_UploadAttachmentDefaultsContentTypeAndReportsDuplicates(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "application/octet-stream", r.Header.Get("Content-Type"))
		v12JSON(w, 200, map[string]interface{}{"attachment_ref": v12Ref, "content_type": "audio/wav", "size_bytes": 3, "created": false})
	}))
	defer server.Close()

	resp, err := v12Client(server.URL).UploadAttachment(context.Background(), "ns", []byte("abc"), "")
	require.NoError(t, err)
	assert.False(t, resp.Created)
	assert.Equal(t, "audio/wav", resp.ContentType)
}

func TestV012_ListDownloadDeleteAttachment(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == "GET" && r.URL.Path == "/v1/namespaces/ns/attachments":
			v12JSON(w, 200, map[string]interface{}{"attachments": []map[string]interface{}{
				{"attachment_ref": v12Ref, "content_type": "audio/wav", "size_bytes": 8},
			}})
		case r.Method == "GET" && r.URL.Path == "/v1/namespaces/ns/attachments/"+v12Ref:
			w.Header().Set("Content-Type", "audio/wav")
			w.Header().Set("ETag", `"`+v12Ref+`"`)
			_, _ = w.Write([]byte{0, 1, 2, 3})
		case r.Method == "DELETE" && r.URL.Path == "/v1/namespaces/ns/attachments/"+v12Ref:
			w.WriteHeader(204)
		default:
			t.Errorf("unexpected %s %s", r.Method, r.URL.Path)
			w.WriteHeader(500)
		}
	}))
	defer server.Close()
	client := v12Client(server.URL)

	list, err := client.ListAttachments(context.Background(), "ns")
	require.NoError(t, err)
	require.Len(t, list, 1)
	assert.Equal(t, v12Ref, list[0].AttachmentRef)

	content, err := client.DownloadAttachment(context.Background(), "ns", v12Ref)
	require.NoError(t, err)
	assert.Equal(t, []byte{0, 1, 2, 3}, content.Data)
	assert.Equal(t, "audio/wav", content.ContentType)
	assert.Equal(t, `"`+v12Ref+`"`, content.ETag)

	require.NoError(t, client.DeleteAttachment(context.Background(), "ns", v12Ref))
}

func TestV012_AttachmentsDisabledIs501FeatureDisabled(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		v12JSON(w, 501, map[string]interface{}{"error": "The attachments API is not enabled on this server", "code": "FEATURE_DISABLED", "details": "set DAKERA_ATTACHMENTS to enable it"})
	}))
	defer server.Close()

	_, err := v12Client(server.URL).ListAttachments(context.Background(), "ns")
	require.Error(t, err)
	assert.True(t, IsFeatureDisabledError(err))
	assert.Contains(t, err.Error(), "DAKERA_ATTACHMENTS")
}

func TestV012_TranscribeAttachmentAndJobStatus(t *testing.T) {
	var polls int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		base := "/v1/namespaces/uploads/attachments/" + v12Ref
		switch {
		case r.Method == "POST" && r.URL.Path == base+"/transcribe":
			var body map[string]interface{}
			_ = json.NewDecoder(r.Body).Decode(&body)
			assert.Equal(t, "my-agent", body["agent_id"])
			assert.Equal(t, "de", body["lang"])
			assert.Equal(t, []interface{}{"voice"}, body["tags"])
			assert.EqualValues(t, 0.7, body["importance"])
			v12JSON(w, 202, map[string]interface{}{
				"job_id": "job_1a2b3c4d_0", "attachment_ref": v12Ref, "agent_id": "my-agent",
				"memory_id": "mem_17f3", "model": "whisper-tiny.en",
				"status_url": base + "/transcribe/job_1a2b3c4d_0",
			})
		case r.Method == "GET" && r.URL.Path == base+"/transcribe/job_1a2b3c4d_0":
			if atomic.AddInt32(&polls, 1) < 2 {
				v12JSON(w, 200, map[string]interface{}{"id": "job_1a2b3c4d_0", "job_type": "transcription", "status": "Running", "created_at": 1, "progress": 40, "message": "transcribing", "metadata": map[string]string{}})
				return
			}
			v12JSON(w, 200, map[string]interface{}{"id": "job_1a2b3c4d_0", "job_type": "transcription", "status": "Completed", "created_at": 1, "progress": 100, "message": "memory mem_17f3 stored", "metadata": map[string]string{"namespace": "uploads"}})
		default:
			t.Errorf("unexpected %s %s", r.Method, r.URL.Path)
			w.WriteHeader(500)
		}
	}))
	defer server.Close()
	client := v12Client(server.URL)

	imp := float32(0.7)
	accepted, err := client.TranscribeAttachment(context.Background(), "uploads", v12Ref, TranscribeRequest{AgentID: "my-agent", Tags: []string{"voice"}, Importance: &imp, Lang: "de"})
	require.NoError(t, err)
	assert.Equal(t, "job_1a2b3c4d_0", accepted.JobID)
	assert.Equal(t, "mem_17f3", accepted.MemoryID)
	assert.Equal(t, "whisper-tiny.en", accepted.Model)
	assert.Contains(t, accepted.StatusURL, "/transcribe/job_1a2b3c4d_0")

	job, err := client.GetTranscriptionJob(context.Background(), "uploads", v12Ref, accepted.JobID)
	require.NoError(t, err)
	assert.Equal(t, JobStatusRunning, job.Status)
	assert.False(t, job.IsDone())

	done, err := client.WaitForTranscription(context.Background(), "uploads", v12Ref, accepted.JobID, JobWaitOptions{Timeout: 5 * time.Second, PollInterval: 5 * time.Millisecond})
	require.NoError(t, err)
	assert.Equal(t, JobStatusCompleted, done.Status)
	assert.Equal(t, uint8(100), done.Progress)
}

func TestV012_WaitForJobReportsFailureWithCode(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/v1/namespaces/ns/attachments/"+v12Ref+"/index/job_9_0", r.URL.Path)
		v12JSON(w, 200, map[string]interface{}{
			"id": "job_9_0", "job_type": "image_index", "status": "Failed", "created_at": 1, "progress": 10,
			"message": "the image cannot be decoded", "metadata": map[string]string{},
			"error": map[string]interface{}{"status": 400, "code": "INVALID_REQUEST"},
		})
	}))
	defer server.Close()

	job, err := v12Client(server.URL).WaitForImageIndex(context.Background(), "ns", v12Ref, "job_9_0", JobWaitOptions{Timeout: time.Second, PollInterval: time.Millisecond})
	require.Error(t, err)
	assert.True(t, IsJobFailedError(err))
	require.NotNil(t, job)
	require.NotNil(t, job.Error)
	assert.Equal(t, 400, job.Error.Status)
	assert.Equal(t, ErrorCodeInvalidRequest, job.Error.Code)
	assert.Contains(t, err.Error(), "INVALID_REQUEST")
}

func TestV012_WaitForJobTimesOutWhileRunning(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		v12JSON(w, 200, map[string]interface{}{"id": "job_1_0", "job_type": "transcription", "status": "Pending", "created_at": 1, "progress": 2, "metadata": map[string]string{}})
	}))
	defer server.Close()

	_, err := v12Client(server.URL).WaitForTranscription(context.Background(), "ns", v12Ref, "job_1_0", JobWaitOptions{Timeout: 60 * time.Millisecond, PollInterval: 10 * time.Millisecond})
	require.Error(t, err)
	assert.True(t, IsTimeoutError(err))
}

func TestV012_IndexAttachmentImage(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		base := "/v1/namespaces/pages/attachments/" + v12Ref
		switch {
		case r.Method == "POST" && r.URL.Path == base+"/index":
			var body map[string]interface{}
			_ = json.NewDecoder(r.Body).Decode(&body)
			assert.Equal(t, "p1", body["agent_id"])
			assert.Equal(t, "Page 1", body["content"])
			v12JSON(w, 202, map[string]interface{}{"job_id": "job_2_0", "attachment_ref": v12Ref, "agent_id": "p1", "memory_id": "mem_1", "model": "colmodernvbert", "status_url": base + "/index/job_2_0"})
		case r.Method == "GET" && r.URL.Path == base+"/index/job_2_0":
			v12JSON(w, 200, map[string]interface{}{"id": "job_2_0", "job_type": "image_index", "status": "Completed", "created_at": 1, "progress": 100, "metadata": map[string]string{}})
		default:
			t.Errorf("unexpected %s %s", r.Method, r.URL.Path)
			w.WriteHeader(500)
		}
	}))
	defer server.Close()
	client := v12Client(server.URL)

	accepted, err := client.IndexAttachmentImage(context.Background(), "pages", v12Ref, IndexImageRequest{AgentID: "p1", Content: "Page 1"})
	require.NoError(t, err)
	assert.Equal(t, "colmodernvbert", accepted.Model)
	job, err := client.GetImageIndexJob(context.Background(), "pages", v12Ref, accepted.JobID)
	require.NoError(t, err)
	assert.True(t, job.IsDone())
}

func TestV012_StoreMemoryCarriesAttachmentRefAndLang(t *testing.T) {
	var sent map[string]interface{}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewDecoder(r.Body).Decode(&sent)
		v12JSON(w, 200, map[string]interface{}{"memory": map[string]interface{}{"id": "m1", "content": "voice note", "memory_type": "episodic", "importance": 0.5, "attachment_ref": v12Ref}, "embedding_time_ms": 3})
	}))
	defer server.Close()

	resp, err := v12Client(server.URL).StoreMemory(context.Background(), "a1", StoreMemoryRequest{Content: "voice note", AttachmentRef: v12Ref, Lang: "pt-BR", ID: "m1"})
	require.NoError(t, err)
	assert.Equal(t, v12Ref, sent["attachment_ref"])
	assert.Equal(t, "pt-BR", sent["lang"])
	assert.Equal(t, "m1", sent["id"])
	assert.Equal(t, v12Ref, resp.Memory.AttachmentRef)
}

func TestV012_RecalledMemoryDecodesAttachmentRef(t *testing.T) {
	var nested RecalledMemory
	require.NoError(t, json.Unmarshal([]byte(`{"memory":{"id":"m1","content":"c","attachment_ref":"`+v12Ref+`"},"score":0.5}`), &nested))
	assert.Equal(t, v12Ref, nested.AttachmentRef)
	var flat RecalledMemory
	require.NoError(t, json.Unmarshal([]byte(`{"id":"m1","content":"c","attachment_ref":"`+v12Ref+`","score":0.5}`), &flat))
	assert.Equal(t, v12Ref, flat.AttachmentRef)
}

// ---------------------------------------------------------------------------
// lang — omitted from the wire unless set (v0.11 compatible)
// ---------------------------------------------------------------------------

func TestV012_LangIsSentOnEveryRequestThatTakesIt(t *testing.T) {
	bodies := map[string]map[string]interface{}{}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body map[string]interface{}
		_ = json.NewDecoder(r.Body).Decode(&body)
		bodies[r.URL.Path] = body
		switch r.URL.Path {
		case "/v1/memory/recall":
			v12JSON(w, 200, map[string]interface{}{"memories": []interface{}{}})
		case "/v1/memory/search":
			v12JSON(w, 200, map[string]interface{}{"memories": []interface{}{}})
		case "/v1/memories/store/batch":
			v12JSON(w, 200, map[string]interface{}{"stored": []interface{}{}, "stored_count": 0, "total_embedding_time_ms": 0})
		case "/v1/memories/extract":
			v12JSON(w, 200, map[string]interface{}{"entities": []interface{}{}})
		default:
			v12JSON(w, 200, map[string]interface{}{"memory": map[string]interface{}{"id": "m1"}})
		}
	}))
	defer server.Close()
	client := v12Client(server.URL)
	ctx := context.Background()

	_, err := client.Recall(ctx, "a", RecallRequest{Query: "gestern", Lang: "de"})
	require.NoError(t, err)
	_, err = client.SearchMemories(ctx, "a", SearchMemoriesRequest{Query: "hier", Lang: "fr"})
	require.NoError(t, err)
	_, err = client.StoreMemoriesBatch(ctx, BatchStoreMemoryRequest{AgentID: "a", Lang: "es", Memories: []BatchStoreMemoryItem{{Content: "x", AttachmentRef: v12Ref}}})
	require.NoError(t, err)
	content := "neu"
	_, err = client.UpdateMemory(ctx, "a", "m1", UpdateMemoryRequest{Content: &content, Lang: "it"})
	require.NoError(t, err)
	_, err = client.ExtractMemoryEntities(ctx, ExtractMemoryEntitiesRequest{Content: "Hans in Berlin", Lang: "nl"})
	require.NoError(t, err)

	assert.Equal(t, "de", bodies["/v1/memory/recall"]["lang"])
	assert.Equal(t, "fr", bodies["/v1/memory/search"]["lang"])
	assert.Equal(t, "es", bodies["/v1/memories/store/batch"]["lang"])
	items := bodies["/v1/memories/store/batch"]["memories"].([]interface{})
	assert.Equal(t, v12Ref, items[0].(map[string]interface{})["attachment_ref"])
	assert.Equal(t, "it", bodies["/v1/memory/update/m1"]["lang"])
	assert.Equal(t, "nl", bodies["/v1/memories/extract"]["lang"])
}

func TestV012_LangIsAbsentWhenUnset(t *testing.T) {
	var raw []byte
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		raw, _ = io.ReadAll(r.Body)
		v12JSON(w, 200, map[string]interface{}{"memories": []interface{}{}})
	}))
	defer server.Close()

	_, err := v12Client(server.URL).Recall(context.Background(), "a", RecallRequest{Query: "q"})
	require.NoError(t, err)
	assert.NotContains(t, string(raw), "lang")
	assert.NotContains(t, string(raw), "attachment_ref")

	_, err = v12Client(server.URL).ExtractEntities(context.Background(), "text", nil)
	require.NoError(t, err)
	assert.NotContains(t, string(raw), "lang")
}

// ---------------------------------------------------------------------------
// Records
// ---------------------------------------------------------------------------

func TestV012_UpsertRecordsWithRepresentations(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "POST", r.Method)
		assert.Equal(t, "/v1/namespaces/docs/records", r.URL.Path)
		var body struct {
			Records []map[string]interface{} `json:"records"`
		}
		assert.NoError(t, json.NewDecoder(r.Body).Decode(&body))
		if len(body.Records) != 1 {
			t.Errorf("want 1 record, got %d", len(body.Records))
			w.WriteHeader(500)
			return
		}
		rec := body.Records[0]
		assert.Equal(t, "r1", rec["id"])
		assert.Equal(t, []interface{}{0.5, 0.25}, rec["values"])
		reps := rec["representations"].([]interface{})
		rep := reps[0].(map[string]interface{})
		assert.Equal(t, "tokens", rep["name"])
		assert.Equal(t, "token_multivector", rep["kind"])
		assert.Equal(t, "f16", rep["store_as"])
		assert.Equal(t, []interface{}{[]interface{}{0.5, 0.25}, []interface{}{0.75, 0.125}}, rep["vectors"])
		_, hasModel := rep["model"]
		assert.False(t, hasModel)
		v12JSON(w, 200, map[string]interface{}{"upserted_count": 1})
	}))
	defer server.Close()

	resp, err := v12Client(server.URL).UpsertRecords(context.Background(), "docs", []RecordInput{{
		ID:     "r1",
		Values: []float32{0.5, 0.25},
		Representations: []RepresentationInput{{
			Name:    "tokens",
			Kind:    RepresentationKindTokenMultivector,
			Vectors: [][]float32{{0.5, 0.25}, {0.75, 0.125}},
			StoreAs: BlockDTypeF16,
		}},
		Metadata: map[string]interface{}{"source": "demo"},
	}})
	require.NoError(t, err)
	assert.Equal(t, 1, resp.UpsertedCount)
}

func TestV012_UpsertRecordsOverLimitIs413(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		v12JSON(w, 413, map[string]interface{}{"error": "records[0] (id 'r1'): too many vectors", "code": "PAYLOAD_TOO_LARGE"})
	}))
	defer server.Close()

	_, err := v12Client(server.URL).UpsertRecords(context.Background(), "docs", []RecordInput{{ID: "r1", Values: []float32{1}}})
	require.Error(t, err)
	assert.True(t, IsPayloadTooLargeError(err))
	assert.False(t, IsQuotaExceededError(err))
}

func TestV012_GetRecordManifestAndVectors(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/v1/namespaces/docs/records/r1", r.URL.Path)
		reps := []map[string]interface{}{{"name": "tokens", "kind": "token_multivector", "model": "bge-m3", "dim": 2, "count": 2, "dtype": "f16", "bytes": 8}}
		out := map[string]interface{}{"id": "r1", "dimension": 4, "representations": reps, "unsupported_representations": 1, "metadata": map[string]interface{}{"source": "demo"}}
		if r.URL.Query().Get("include_vectors") == "true" {
			out["values"] = []float32{0.1, 0.2, 0.3, 0.4}
			reps[0]["vectors"] = [][]float32{{0.5, 0.25}, {0.75, 0.125}}
		}
		v12JSON(w, 200, out)
	}))
	defer server.Close()
	client := v12Client(server.URL)

	manifest, err := client.GetRecord(context.Background(), "docs", "r1", false)
	require.NoError(t, err)
	assert.Equal(t, 4, manifest.Dimension)
	assert.Nil(t, manifest.Values)
	require.Len(t, manifest.Representations, 1)
	rep := manifest.Representations[0]
	assert.Equal(t, RepresentationKindTokenMultivector, rep.Kind)
	assert.Equal(t, BlockDTypeF16, rep.DType)
	assert.Equal(t, uint32(2), rep.Dim)
	assert.Equal(t, uint32(2), rep.Count)
	assert.Equal(t, 8, rep.Bytes)
	assert.Nil(t, rep.Vectors)
	assert.Equal(t, 1, manifest.UnsupportedRepresentations)

	full, err := client.GetRecord(context.Background(), "docs", "r1", true)
	require.NoError(t, err)
	assert.Equal(t, []float32{0.1, 0.2, 0.3, 0.4}, full.Values)
	assert.Equal(t, [][]float32{{0.5, 0.25}, {0.75, 0.125}}, full.Representations[0].Vectors)
}

func TestV012_GetRecordUnknownKindStillDecodes(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		v12JSON(w, 200, map[string]interface{}{"id": "r1", "dimension": 4, "representations": []map[string]interface{}{{"name": "x", "kind": "sparse_future", "dim": 1, "count": 1, "dtype": "bf16", "bytes": 2}}})
	}))
	defer server.Close()

	rec, err := v12Client(server.URL).GetRecord(context.Background(), "docs", "r1", false)
	require.NoError(t, err)
	assert.False(t, rec.Representations[0].Kind.IsKnown())
	assert.False(t, rec.Representations[0].DType.IsKnown())
}

// ---------------------------------------------------------------------------
// Capabilities (v0.12 additions) and response-shape fixes
// ---------------------------------------------------------------------------

func TestV012_CapabilitiesAttachmentsVisionScoring(t *testing.T) {
	doc := `{"capabilities_version":1,"server_version":"0.12.0","default_model":"bge-large","models":[
	  {"name":"colbert-small","aliases":["answerai-colbert-small"],"dimension":96,"max_seq_length":512,"effective_max_seq_length":512,"active":false,"modality":"text"}],
	 "search_mode":"rabitq","search_modes_accepted":"hybrid, binary, float, scalar (alias sq), rabitq",
	 "scoring":{"strategy":"late-interaction","strategies_accepted":"single-vector, late-interaction","late_interaction":{"enabled":true,"model_supported":true,"lane":"text","token_slot":"colbert","fde_slot":"colbert.fde","fde_k_sim":4,"fde_d_proj":16,"fde_reps":20,"fde_dim":2048,"candidates":200}},
	 "attachments":{"enabled":true,"max_bytes":26214400,"transcription":{"model":"whisper-tiny.en","models":["whisper-tiny.en"],"media_types":["audio/wav"],"languages":["en"],"sample_rate_hz":16000}},
	 "vision":{"enabled":false,"model":"colmodernvbert","models":["colmodernvbert"],"media_types":["image/png"],"dimension":128,"patch_slot":"patch","patch_fde_slot":"patch.fde","tile_size":512,"longest_edge":2048,"image_seq_len":64,"max_tiles":17},
	 "query_languages":["en","de","fr","es","it","pt","nl"],"unreadable_records":2,"late_interaction_stats":{"searches":1}}`
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/v1/capabilities", r.URL.Path)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(doc))
	}))
	defer server.Close()

	caps, err := v12Client(server.URL).Capabilities(context.Background())
	require.NoError(t, err)
	assert.True(t, caps.SupportsAttachments())
	assert.False(t, caps.SupportsVision())
	assert.EqualValues(t, 26214400, caps.Attachments.MaxBytes)
	assert.Equal(t, "whisper-tiny.en", caps.Attachments.Transcription.Model)
	assert.Equal(t, []string{"audio/wav"}, caps.Attachments.Transcription.MediaTypes)
	assert.Equal(t, 16000, caps.Attachments.Transcription.SampleRateHz)
	assert.Equal(t, 128, caps.Vision.Dimension)
	assert.Equal(t, "late-interaction", caps.Scoring.Strategy)
	assert.Equal(t, []string{"single-vector", "late-interaction"}, []string(caps.Scoring.StrategiesAccepted))
	assert.True(t, caps.Scoring.LateInteraction.Enabled)
	assert.Equal(t, 200, caps.Scoring.LateInteraction.Candidates)
	assert.Equal(t, uint64(2), caps.UnreadableRecords)
	assert.Equal(t, SearchModeRaBitQ, caps.SearchMode)
	assert.Contains(t, []string(caps.SearchModesAccepted), "rabitq")
	assert.Contains(t, []string(caps.SearchModesAccepted), "sq")
	assert.NotNil(t, caps.FindModel("answerai-colbert-small"))
	assert.True(t, EmbeddingModelColbertSmall.IsKnown())
	assert.NotNil(t, caps.Raw["late_interaction_stats"])
}

func TestV012_UpsertAndDeleteCountsDecodeSnakeCase(t *testing.T) {
	var up UpsertResponse
	require.NoError(t, json.Unmarshal([]byte(`{"upserted_count":4}`), &up))
	assert.Equal(t, 4, up.UpsertedCount)
	require.NoError(t, json.Unmarshal([]byte(`{"upsertedCount":5}`), &up))
	assert.Equal(t, 5, up.UpsertedCount)

	var del DeleteResponse
	require.NoError(t, json.Unmarshal([]byte(`{"deleted_count":6}`), &del))
	assert.Equal(t, 6, del.DeletedCount)
	require.NoError(t, json.Unmarshal([]byte(`{"deletedCount":7}`), &del))
	assert.Equal(t, 7, del.DeletedCount)
}

func TestV012_AgentMemoryNamespace(t *testing.T) {
	assert.Equal(t, "_dakera_agent_my-agent", AgentMemoryNamespace("my-agent"))
}
