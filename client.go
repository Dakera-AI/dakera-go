// Package dakera provides a Go client for Dakera AI memory platform.
//
// Example usage:
//
//	client := dakera.NewClient("http://localhost:3000")
//
//	// Upsert vectors
//	resp, err := client.Upsert(ctx, "my-namespace", []dakera.VectorInput{
//	    {ID: "vec1", Values: []float32{0.1, 0.2, 0.3}},
//	})
//
//	// Query similar vectors
//	results, err := client.Query(ctx, "my-namespace", []float32{0.1, 0.2, 0.3}, nil)
package dakera

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"math/rand"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"
)

const defaultTimeout = 30 * time.Second

// Client is the Dakera client for interacting with the vector database.
type Client struct {
	baseURL     string
	odeURL      string
	apiKey      string
	retryConfig RetryConfig
	headers     map[string]string
	httpClient  *http.Client

	// OPS-1: last seen rate-limit headers
	rlMu                sync.Mutex
	lastRateLimitHeaders *RateLimitHeaders
}

// LastRateLimitHeaders returns the rate-limit headers from the most recent
// API response (OPS-1).  Returns nil until the first request has been made.
func (c *Client) LastRateLimitHeaders() *RateLimitHeaders {
	c.rlMu.Lock()
	defer c.rlMu.Unlock()
	if c.lastRateLimitHeaders == nil {
		return nil
	}
	cp := *c.lastRateLimitHeaders
	return &cp
}

// NewClient creates a new Dakera client with the given base URL.
func NewClient(baseURL string) *Client {
	return NewClientWithOptions(ClientOptions{
		BaseURL: baseURL,
	})
}

// NewClientWithOptions creates a new Dakera client with custom options.
func NewClientWithOptions(opts ClientOptions) *Client {
	timeout := opts.Timeout
	if timeout == 0 {
		timeout = defaultTimeout
	}

	connectTimeout := opts.ConnectTimeout
	if connectTimeout == 0 {
		connectTimeout = timeout
	}

	// Build retry config: RetryBackoff wins over MaxRetries
	rc := DefaultRetryConfig()
	if opts.RetryBackoff != nil {
		rc = *opts.RetryBackoff
		if rc.MaxRetries == 0 {
			rc.MaxRetries = DefaultRetryConfig().MaxRetries
		}
		if rc.BaseDelay == 0 {
			rc.BaseDelay = DefaultRetryConfig().BaseDelay
		}
		if rc.MaxDelay == 0 {
			rc.MaxDelay = DefaultRetryConfig().MaxDelay
		}
	} else if opts.MaxRetries > 0 {
		rc.MaxRetries = opts.MaxRetries
	}

	baseURL := strings.TrimSuffix(opts.BaseURL, "/")

	transport := &http.Transport{
		DialContext: (&net.Dialer{
			Timeout: connectTimeout,
		}).DialContext,
	}

	return &Client{
		baseURL:     baseURL,
		odeURL:      strings.TrimSuffix(opts.OdeURL, "/"),
		apiKey:      opts.APIKey,
		retryConfig: rc,
		headers:     opts.Headers,
		httpClient: &http.Client{
			Timeout:   timeout,
			Transport: transport,
		},
	}
}

// computeBackoff returns the wait duration for a given attempt number.
func (c *Client) computeBackoff(attempt int) time.Duration {
	rc := c.retryConfig
	backoff := float64(rc.BaseDelay) * math.Pow(2, float64(attempt))
	if backoff > float64(rc.MaxDelay) {
		backoff = float64(rc.MaxDelay)
	}
	if rc.Jitter {
		backoff *= 0.5 + rand.Float64()
	}
	return time.Duration(backoff)
}

// parseRateLimitHeaders extracts OPS-1 rate-limit and quota headers.
func parseRateLimitHeaders(h http.Header) *RateLimitHeaders {
	parseI := func(name string) int64 {
		v := h.Get(name)
		if v == "" {
			return 0
		}
		n, err := strconv.ParseInt(v, 10, 64)
		if err != nil {
			return 0
		}
		return n
	}
	return &RateLimitHeaders{
		Limit:      parseI("X-RateLimit-Limit"),
		Remaining:  parseI("X-RateLimit-Remaining"),
		Reset:      parseI("X-RateLimit-Reset"),
		QuotaUsed:  parseI("X-Quota-Used"),
		QuotaLimit: parseI("X-Quota-Limit"),
	}
}

// request makes an HTTP request with retry logic.
func (c *Client) request(ctx context.Context, method, path string, body interface{}) ([]byte, error) {
	reqURL := c.baseURL + path
	rc := c.retryConfig
	var lastErr error

	for attempt := 0; attempt < rc.MaxRetries; attempt++ {
		var reqBody io.Reader
		if body != nil {
			jsonBody, err := json.Marshal(body)
			if err != nil {
				return nil, fmt.Errorf("failed to marshal request body: %w", err)
			}
			reqBody = bytes.NewReader(jsonBody)
		}

		req, err := http.NewRequestWithContext(ctx, method, reqURL, reqBody)
		if err != nil {
			return nil, fmt.Errorf("failed to create request: %w", err)
		}

		req.Header.Set("Content-Type", "application/json")
		if c.apiKey != "" {
			req.Header.Set("Authorization", "Bearer "+c.apiKey)
		}
		for k, v := range c.headers {
			req.Header.Set(k, v)
		}

		resp, err := c.httpClient.Do(req)
		if err != nil {
			if ctx.Err() != nil {
				return nil, NewTimeoutError(fmt.Sprintf("request timed out: %v", err))
			}
			lastErr = NewConnectionError(fmt.Sprintf("failed to connect: %v", err))
			if attempt < rc.MaxRetries-1 {
				time.Sleep(c.computeBackoff(attempt))
				continue
			}
			return nil, lastErr
		}
		defer resp.Body.Close()

		// OPS-1: capture rate-limit headers on every response
		c.rlMu.Lock()
		c.lastRateLimitHeaders = parseRateLimitHeaders(resp.Header)
		c.rlMu.Unlock()

		respBody, err := io.ReadAll(resp.Body)
		if err != nil {
			lastErr = fmt.Errorf("failed to read response body: %w", err)
			continue
		}

		if resp.StatusCode >= 200 && resp.StatusCode < 300 {
			return respBody, nil
		}

		// Parse error response
		var errBody struct {
			Error   string    `json:"error"`
			Code    ErrorCode `json:"code"`
			Details string    `json:"details"`
		}
		json.Unmarshal(respBody, &errBody)
		errMsg := errBody.Error
		if errMsg == "" {
			errMsg = fmt.Sprintf("HTTP %d", resp.StatusCode)
		}
		errorCode := errBody.Code
		if errorCode == "" {
			errorCode = ErrorCodeUnknown
		}

		switch resp.StatusCode {
		case 400:
			return nil, NewValidationError(errMsg, resp.StatusCode, errBody, errorCode)
		case 401:
			return nil, NewAuthenticationError("Authentication failed", resp.StatusCode, errBody, errorCode)
		case 403:
			return nil, NewAuthorizationError(errMsg, resp.StatusCode, errorCode, errBody)
		case 404:
			return nil, NewNotFoundError(errMsg, resp.StatusCode, errBody, errorCode)
		case 429:
			retryAfterSecs := -1
			if ra := resp.Header.Get("Retry-After"); ra != "" {
				retryAfterSecs, _ = strconv.Atoi(ra)
			}
			rlErr := NewRateLimitError("Rate limit exceeded", resp.StatusCode, errBody, errorCode, retryAfterSecs)
			if attempt < rc.MaxRetries-1 {
				var wait time.Duration
				if retryAfterSecs >= 0 {
					wait = time.Duration(retryAfterSecs) * time.Second
				} else {
					wait = c.computeBackoff(attempt)
				}
				time.Sleep(wait)
				lastErr = rlErr
				continue
			}
			return nil, rlErr
		default:
			if resp.StatusCode >= 500 {
				lastErr = NewServerError(errMsg, resp.StatusCode, errBody, errorCode)
				if attempt < rc.MaxRetries-1 {
					time.Sleep(c.computeBackoff(attempt))
					continue
				}
				return nil, lastErr
			}
			return nil, &DakeraError{Message: errMsg, StatusCode: resp.StatusCode, Code: errorCode, ResponseBody: errBody}
		}
	}

	if lastErr != nil {
		return nil, lastErr
	}
	return nil, &DakeraError{Message: "request failed after retries"}
}

// ===========================================================================
// Vector Operations
// ===========================================================================

// Upsert inserts or updates vectors in a namespace.
func (c *Client) Upsert(ctx context.Context, namespace string, vectors []VectorInput) (*UpsertResponse, error) {
	body := map[string]interface{}{
		"vectors": vectors,
	}

	respBody, err := c.request(ctx, "POST", fmt.Sprintf("/v1/namespaces/%s/vectors", namespace), body)
	if err != nil {
		return nil, err
	}

	var resp UpsertResponse
	if err := json.Unmarshal(respBody, &resp); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}
	return &resp, nil
}

// Query searches for similar vectors in a namespace.
func (c *Client) Query(ctx context.Context, namespace string, vector []float32, opts *QueryOptions) (*SearchResult, error) {
	body := map[string]interface{}{
		"vector": vector,
	}

	if opts != nil {
		if opts.TopK > 0 {
			body["top_k"] = opts.TopK
		}
		if opts.Filter != nil {
			body["filter"] = opts.Filter
		}
		body["include_values"] = opts.IncludeValues
		body["include_metadata"] = opts.IncludeMetadata
	} else {
		body["top_k"] = 10
		body["include_metadata"] = true
	}

	respBody, err := c.request(ctx, "POST", fmt.Sprintf("/v1/namespaces/%s/query", namespace), body)
	if err != nil {
		return nil, err
	}

	var resp SearchResult
	if err := json.Unmarshal(respBody, &resp); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}
	return &resp, nil
}

// Delete removes vectors from a namespace.
func (c *Client) Delete(ctx context.Context, namespace string, opts DeleteOptions) (*DeleteResponse, error) {
	body := make(map[string]interface{})
	if opts.IDs != nil {
		body["ids"] = opts.IDs
	}
	if opts.Filter != nil {
		body["filter"] = opts.Filter
	}
	if opts.DeleteAll {
		body["delete_all"] = true
	}

	respBody, err := c.request(ctx, "POST", fmt.Sprintf("/v1/namespaces/%s/delete", namespace), body)
	if err != nil {
		return nil, err
	}

	var resp DeleteResponse
	if err := json.Unmarshal(respBody, &resp); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}
	return &resp, nil
}

// Fetch retrieves vectors by ID from a namespace.
func (c *Client) Fetch(ctx context.Context, namespace string, ids []string, opts *FetchOptions) ([]Vector, error) {
	body := map[string]interface{}{
		"ids": ids,
	}

	if opts != nil {
		body["include_values"] = opts.IncludeValues
		body["include_metadata"] = opts.IncludeMetadata
	} else {
		body["include_values"] = true
		body["include_metadata"] = true
	}

	respBody, err := c.request(ctx, "POST", fmt.Sprintf("/v1/namespaces/%s/fetch", namespace), body)
	if err != nil {
		return nil, err
	}

	var resp struct {
		Vectors []Vector `json:"vectors"`
	}
	if err := json.Unmarshal(respBody, &resp); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}
	return resp.Vectors, nil
}

// BatchQuery executes multiple queries in a single request.
func (c *Client) BatchQuery(ctx context.Context, namespace string, queries []BatchQuerySpec) ([]SearchResult, error) {
	reqQueries := make([]map[string]interface{}, len(queries))
	for i, q := range queries {
		reqQuery := map[string]interface{}{
			"vector": q.Vector,
		}
		if q.TopK > 0 {
			reqQuery["top_k"] = q.TopK
		} else {
			reqQuery["top_k"] = 10
		}
		if q.Filter != nil {
			reqQuery["filter"] = q.Filter
		}
		reqQuery["include_values"] = q.IncludeValues
		reqQuery["include_metadata"] = q.IncludeMetadata
		reqQueries[i] = reqQuery
	}

	body := map[string]interface{}{
		"queries": reqQueries,
	}

	respBody, err := c.request(ctx, "POST", fmt.Sprintf("/v1/namespaces/%s/batch-query", namespace), body)
	if err != nil {
		return nil, err
	}

	var resp struct {
		Results []SearchResult `json:"results"`
	}
	if err := json.Unmarshal(respBody, &resp); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}
	return resp.Results, nil
}

// ===========================================================================
// Text-Based Inference Operations (Auto-Embedding)
// ===========================================================================

// UpsertText upserts text documents with automatic embedding generation.
// The text is embedded using the specified model (default: MiniLM) and stored as vectors.
func (c *Client) UpsertText(ctx context.Context, namespace string, documents []TextDocument, opts *TextUpsertOptions) (*TextUpsertResponse, error) {
	body := map[string]interface{}{
		"documents": documents,
	}

	if opts != nil && opts.Model != "" {
		body["model"] = opts.Model
	}

	respBody, err := c.request(ctx, "POST", fmt.Sprintf("/v1/namespaces/%s/upsert-text", namespace), body)
	if err != nil {
		return nil, err
	}

	var resp TextUpsertResponse
	if err := json.Unmarshal(respBody, &resp); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}
	return &resp, nil
}

// QueryText queries using natural language text with automatic embedding.
// The query text is embedded and used for similarity search.
func (c *Client) QueryText(ctx context.Context, namespace string, text string, opts *TextQueryOptions) (*TextQueryResponse, error) {
	body := map[string]interface{}{
		"text": text,
	}

	if opts != nil {
		if opts.TopK > 0 {
			body["top_k"] = opts.TopK
		} else {
			body["top_k"] = 10
		}
		body["include_text"] = opts.IncludeText
		body["include_vectors"] = opts.IncludeVectors
		if opts.Filter != nil {
			body["filter"] = opts.Filter
		}
		if opts.Model != "" {
			body["model"] = opts.Model
		}
	} else {
		body["top_k"] = 10
		body["include_text"] = true
	}

	respBody, err := c.request(ctx, "POST", fmt.Sprintf("/v1/namespaces/%s/query-text", namespace), body)
	if err != nil {
		return nil, err
	}

	var resp TextQueryResponse
	if err := json.Unmarshal(respBody, &resp); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}
	return &resp, nil
}

// BatchQueryText executes multiple text queries with automatic embedding in a single request.
func (c *Client) BatchQueryText(ctx context.Context, namespace string, queries []string, opts *BatchTextQueryOptions) (*BatchTextQueryResponse, error) {
	body := map[string]interface{}{
		"queries": queries,
	}

	if opts != nil {
		if opts.TopK > 0 {
			body["top_k"] = opts.TopK
		} else {
			body["top_k"] = 10
		}
		body["include_vectors"] = opts.IncludeVectors
		if opts.Filter != nil {
			body["filter"] = opts.Filter
		}
		if opts.Model != "" {
			body["model"] = opts.Model
		}
	} else {
		body["top_k"] = 10
	}

	respBody, err := c.request(ctx, "POST", fmt.Sprintf("/v1/namespaces/%s/batch-query-text", namespace), body)
	if err != nil {
		return nil, err
	}

	var resp BatchTextQueryResponse
	if err := json.Unmarshal(respBody, &resp); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}
	return &resp, nil
}

// ===========================================================================
// Full-Text Search Operations
// ===========================================================================

// IndexDocuments indexes documents for full-text search.
func (c *Client) IndexDocuments(ctx context.Context, namespace string, documents []DocumentInput) (*IndexDocumentsResponse, error) {
	body := map[string]interface{}{
		"documents": documents,
	}

	respBody, err := c.request(ctx, "POST", fmt.Sprintf("/v1/namespaces/%s/fulltext/index", namespace), body)
	if err != nil {
		return nil, err
	}

	var resp IndexDocumentsResponse
	if err := json.Unmarshal(respBody, &resp); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}
	return &resp, nil
}

// FulltextSearch performs a full-text search.
func (c *Client) FulltextSearch(ctx context.Context, namespace string, query string, opts *FullTextSearchOptions) ([]FullTextSearchResult, error) {
	body := map[string]interface{}{
		"query": query,
	}

	if opts != nil {
		if opts.TopK > 0 {
			body["top_k"] = opts.TopK
		}
		if opts.Filter != nil {
			body["filter"] = opts.Filter
		}
	} else {
		body["top_k"] = 10
	}

	respBody, err := c.request(ctx, "POST", fmt.Sprintf("/v1/namespaces/%s/fulltext/search", namespace), body)
	if err != nil {
		return nil, err
	}

	var resp struct {
		Results []FullTextSearchResult `json:"results"`
	}
	if err := json.Unmarshal(respBody, &resp); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}
	return resp.Results, nil
}

// HybridSearch performs a hybrid search combining vector and full-text.
//
// When vector is nil the server falls back to BM25-only full-text search.
// When provided, results are blended with vector similarity according to opts.Alpha.
func (c *Client) HybridSearch(ctx context.Context, namespace string, vector []float32, query string, opts *HybridSearchOptions) ([]HybridSearchResult, error) {
	body := map[string]interface{}{
		"query": query,
	}
	if vector != nil {
		body["vector"] = vector
	}

	if opts != nil {
		if opts.TopK > 0 {
			body["top_k"] = opts.TopK
		}
		if opts.Alpha > 0 {
			body["alpha"] = opts.Alpha
		}
		if opts.Filter != nil {
			body["filter"] = opts.Filter
		}
	} else {
		body["top_k"] = 10
		body["alpha"] = 0.5
	}

	respBody, err := c.request(ctx, "POST", fmt.Sprintf("/v1/namespaces/%s/hybrid", namespace), body)
	if err != nil {
		return nil, err
	}

	var resp struct {
		Results []HybridSearchResult `json:"results"`
	}
	if err := json.Unmarshal(respBody, &resp); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}
	return resp.Results, nil
}

// ===========================================================================
// Namespace Operations
// ===========================================================================

// ListNamespaces returns all namespaces.
func (c *Client) ListNamespaces(ctx context.Context) ([]NamespaceInfo, error) {
	respBody, err := c.request(ctx, "GET", "/v1/namespaces", nil)
	if err != nil {
		return nil, err
	}

	var resp struct {
		Namespaces []NamespaceInfo `json:"namespaces"`
	}
	if err := json.Unmarshal(respBody, &resp); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}
	return resp.Namespaces, nil
}

// GetNamespace returns information about a specific namespace.
func (c *Client) GetNamespace(ctx context.Context, namespace string) (*NamespaceInfo, error) {
	respBody, err := c.request(ctx, "GET", fmt.Sprintf("/v1/namespaces/%s", namespace), nil)
	if err != nil {
		return nil, err
	}

	var resp NamespaceInfo
	if err := json.Unmarshal(respBody, &resp); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}
	return &resp, nil
}

// CreateNamespace creates a new namespace.
func (c *Client) CreateNamespace(ctx context.Context, namespace string, opts *CreateNamespaceOptions) (*NamespaceInfo, error) {
	body := map[string]interface{}{
		"name": namespace,
	}

	if opts != nil {
		if opts.Dimensions > 0 {
			body["dimensions"] = opts.Dimensions
		}
		if opts.IndexType != "" {
			body["index_type"] = opts.IndexType
		}
		if opts.Metadata != nil {
			body["metadata"] = opts.Metadata
		}
	}

	respBody, err := c.request(ctx, "POST", "/v1/namespaces", body)
	if err != nil {
		return nil, err
	}

	var resp NamespaceInfo
	if err := json.Unmarshal(respBody, &resp); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}
	return &resp, nil
}

// ConfigureNamespace creates or updates a namespace configuration (upsert semantics — v0.6.0).
//
// Creates the namespace if it does not exist, or updates its distance-metric
// configuration if it already exists. Dimension changes are rejected by the
// server to prevent silent data corruption. Requires Write scope.
func (c *Client) ConfigureNamespace(ctx context.Context, namespace string, req ConfigureNamespaceRequest) (*ConfigureNamespaceResponse, error) {
	respBody, err := c.request(ctx, "PUT", fmt.Sprintf("/v1/namespaces/%s", namespace), req)
	if err != nil {
		return nil, err
	}

	var resp ConfigureNamespaceResponse
	if err := json.Unmarshal(respBody, &resp); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}
	return &resp, nil
}

// DeleteNamespace deletes a namespace.
func (c *Client) DeleteNamespace(ctx context.Context, namespace string) error {
	_, err := c.request(ctx, "DELETE", fmt.Sprintf("/v1/namespaces/%s", namespace), nil)
	return err
}

// ===========================================================================
// Admin Operations
// ===========================================================================

// Health checks the server health.
func (c *Client) Health(ctx context.Context) (*HealthResponse, error) {
	respBody, err := c.request(ctx, "GET", "/health", nil)
	if err != nil {
		return nil, err
	}

	var resp HealthResponse
	if err := json.Unmarshal(respBody, &resp); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}
	return &resp, nil
}

// GetIndexStats returns index statistics for a namespace.
func (c *Client) GetIndexStats(ctx context.Context, namespace string) (*IndexStats, error) {
	respBody, err := c.request(ctx, "GET", fmt.Sprintf("/v1/namespaces/%s/stats", namespace), nil)
	if err != nil {
		return nil, err
	}

	var resp IndexStats
	if err := json.Unmarshal(respBody, &resp); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}
	return &resp, nil
}

// Compact triggers compaction for a namespace.
func (c *Client) Compact(ctx context.Context, namespace string) (*StatusResponse, error) {
	respBody, err := c.request(ctx, "POST", fmt.Sprintf("/v1/namespaces/%s/compact", namespace), nil)
	if err != nil {
		return nil, err
	}

	var resp StatusResponse
	if err := json.Unmarshal(respBody, &resp); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}
	return &resp, nil
}

// Flush flushes pending writes for a namespace.
func (c *Client) Flush(ctx context.Context, namespace string) (*StatusResponse, error) {
	respBody, err := c.request(ctx, "POST", fmt.Sprintf("/v1/namespaces/%s/flush", namespace), nil)
	if err != nil {
		return nil, err
	}

	var resp StatusResponse
	if err := json.Unmarshal(respBody, &resp); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}
	return &resp, nil
}

// ===========================================================================
// Memory Operations
// ===========================================================================

// StoreMemory stores a memory for an agent.
func (c *Client) StoreMemory(ctx context.Context, agentID string, req StoreMemoryRequest) (*StoreMemoryResponse, error) {
	respBody, err := c.request(ctx, "POST", fmt.Sprintf("/v1/agents/%s/memories", agentID), req)
	if err != nil {
		return nil, err
	}

	var result StoreMemoryResponse
	if err := json.Unmarshal(respBody, &result); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}
	return &result, nil
}

// Recall recalls memories for an agent.
func (c *Client) Recall(ctx context.Context, agentID string, req RecallRequest) ([]RecalledMemory, error) {
	respBody, err := c.request(ctx, "POST", fmt.Sprintf("/v1/agents/%s/memories/recall", agentID), req)
	if err != nil {
		return nil, err
	}

	var wrapper struct {
		Memories []RecalledMemory `json:"memories"`
	}
	if err := json.Unmarshal(respBody, &wrapper); err != nil {
		// Try direct array
		var memories []RecalledMemory
		if err2 := json.Unmarshal(respBody, &memories); err2 != nil {
			return nil, fmt.Errorf("failed to parse response: %w", err)
		}
		return memories, nil
	}
	return wrapper.Memories, nil
}

// GetMemory gets a specific memory.
func (c *Client) GetMemory(ctx context.Context, agentID, memoryID string) (*Memory, error) {
	respBody, err := c.request(ctx, "GET", fmt.Sprintf("/v1/agents/%s/memories/%s", agentID, memoryID), nil)
	if err != nil {
		return nil, err
	}

	var result Memory
	if err := json.Unmarshal(respBody, &result); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}
	return &result, nil
}

// UpdateMemory updates an existing memory.
func (c *Client) UpdateMemory(ctx context.Context, agentID, memoryID string, req UpdateMemoryRequest) (*StoreMemoryResponse, error) {
	respBody, err := c.request(ctx, "PUT", fmt.Sprintf("/v1/agents/%s/memories/%s", agentID, memoryID), req)
	if err != nil {
		return nil, err
	}

	var result StoreMemoryResponse
	if err := json.Unmarshal(respBody, &result); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}
	return &result, nil
}

// Forget deletes a memory.
func (c *Client) Forget(ctx context.Context, agentID, memoryID string) error {
	_, err := c.request(ctx, "DELETE", fmt.Sprintf("/v1/agents/%s/memories/%s", agentID, memoryID), nil)
	return err
}

// BatchRecall bulk-recalls memories using filter predicates (CE-2).
//
// Uses POST /v1/memories/recall/batch — no embedding required.
//
// Example:
//
//	minImp := float32(0.7)
//	resp, err := client.BatchRecall(ctx, BatchRecallRequest{
//	    AgentID: "agent-1",
//	    Filter:  BatchMemoryFilter{MinImportance: &minImp},
//	    Limit:   50,
//	})
func (c *Client) BatchRecall(ctx context.Context, req BatchRecallRequest) (*BatchRecallResponse, error) {
	respBody, err := c.request(ctx, "POST", "/v1/memories/recall/batch", req)
	if err != nil {
		return nil, err
	}

	var result BatchRecallResponse
	if err := json.Unmarshal(respBody, &result); err != nil {
		return nil, fmt.Errorf("failed to parse batch recall response: %w", err)
	}
	return &result, nil
}

// BatchForget bulk-deletes memories using filter predicates (CE-2).
//
// Uses DELETE /v1/memories/forget/batch.  At least one filter predicate must
// be set (server safety guard).
//
// Example:
//
//	ts := time.Now().Add(-24 * time.Hour).Unix()
//	resp, err := client.BatchForget(ctx, BatchForgetRequest{
//	    AgentID: "agent-1",
//	    Filter:  BatchMemoryFilter{CreatedBefore: &ts},
//	})
func (c *Client) BatchForget(ctx context.Context, req BatchForgetRequest) (*BatchForgetResponse, error) {
	respBody, err := c.request(ctx, "DELETE", "/v1/memories/forget/batch", req)
	if err != nil {
		return nil, err
	}

	var result BatchForgetResponse
	if err := json.Unmarshal(respBody, &result); err != nil {
		return nil, fmt.Errorf("failed to parse batch forget response: %w", err)
	}
	return &result, nil
}

// SearchMemories searches memories for an agent.
func (c *Client) SearchMemories(ctx context.Context, agentID string, req SearchMemoriesRequest) ([]RecalledMemory, error) {
	respBody, err := c.request(ctx, "POST", fmt.Sprintf("/v1/agents/%s/memories/search", agentID), req)
	if err != nil {
		return nil, err
	}

	var wrapper struct {
		Memories []RecalledMemory `json:"memories"`
	}
	if err := json.Unmarshal(respBody, &wrapper); err != nil {
		var memories []RecalledMemory
		if err2 := json.Unmarshal(respBody, &memories); err2 != nil {
			return nil, fmt.Errorf("failed to parse response: %w", err)
		}
		return memories, nil
	}
	return wrapper.Memories, nil
}

// UpdateImportance updates the importance of memories.
func (c *Client) UpdateImportance(ctx context.Context, agentID string, req UpdateImportanceRequest) error {
	_, err := c.request(ctx, "PUT", fmt.Sprintf("/v1/agents/%s/memories/importance", agentID), req)
	return err
}

// Consolidate consolidates memories for an agent.
func (c *Client) Consolidate(ctx context.Context, agentID string, req ConsolidateRequest) (*ConsolidateResponse, error) {
	respBody, err := c.request(ctx, "POST", fmt.Sprintf("/v1/agents/%s/memories/consolidate", agentID), req)
	if err != nil {
		return nil, err
	}

	var result ConsolidateResponse
	if err := json.Unmarshal(respBody, &result); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}
	return &result, nil
}

// MemoryFeedback submits feedback on a memory recall.
func (c *Client) MemoryFeedback(ctx context.Context, agentID string, req MemoryFeedbackRequest) (*MemoryFeedbackResponse, error) {
	respBody, err := c.request(ctx, "POST", fmt.Sprintf("/v1/agents/%s/memories/feedback", agentID), req)
	if err != nil {
		return nil, err
	}

	var result MemoryFeedbackResponse
	if err := json.Unmarshal(respBody, &result); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}
	return &result, nil
}

// ===========================================================================
// Memory Feedback Loop — INT-1
// ===========================================================================

// FeedbackMemory submits upvote/downvote/flag feedback on a memory (INT-1).
//
// Signals:
//   - FeedbackSignalUpvote: boosts importance ×1.15 (capped at 1.0).
//   - FeedbackSignalDownvote: penalises importance ×0.85 (floor 0.0).
//   - FeedbackSignalFlag: marks as irrelevant — accelerates decay on next cycle.
func (c *Client) FeedbackMemory(ctx context.Context, memoryID string, agentID string, signal FeedbackSignal) (*FeedbackResponse, error) {
	req := MemoryFeedbackBodyRequest{AgentID: agentID, Signal: signal}
	respBody, err := c.request(ctx, "POST", fmt.Sprintf("/v1/memories/%s/feedback", memoryID), req)
	if err != nil {
		return nil, err
	}
	var result FeedbackResponse
	if err := json.Unmarshal(respBody, &result); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}
	return &result, nil
}

// GetMemoryFeedbackHistory returns the full feedback history for a memory (INT-1).
func (c *Client) GetMemoryFeedbackHistory(ctx context.Context, memoryID string) (*FeedbackHistoryResponse, error) {
	respBody, err := c.request(ctx, "GET", fmt.Sprintf("/v1/memories/%s/feedback", memoryID), nil)
	if err != nil {
		return nil, err
	}
	var result FeedbackHistoryResponse
	if err := json.Unmarshal(respBody, &result); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}
	return &result, nil
}

// GetAgentFeedbackSummary returns aggregate feedback counts and health score for an agent (INT-1).
func (c *Client) GetAgentFeedbackSummary(ctx context.Context, agentID string) (*AgentFeedbackSummary, error) {
	respBody, err := c.request(ctx, "GET", fmt.Sprintf("/v1/agents/%s/feedback/summary", agentID), nil)
	if err != nil {
		return nil, err
	}
	var result AgentFeedbackSummary
	if err := json.Unmarshal(respBody, &result); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}
	return &result, nil
}

// PatchMemoryImportance directly overrides a memory's importance score (INT-1).
func (c *Client) PatchMemoryImportance(ctx context.Context, memoryID string, agentID string, importance float32) (*FeedbackResponse, error) {
	req := MemoryImportancePatchRequest{AgentID: agentID, Importance: importance}
	respBody, err := c.request(ctx, "PATCH", fmt.Sprintf("/v1/memories/%s/importance", memoryID), req)
	if err != nil {
		return nil, err
	}
	var result FeedbackResponse
	if err := json.Unmarshal(respBody, &result); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}
	return &result, nil
}

// GetFeedbackHealth returns overall feedback health score for an agent (INT-1).
//
// The health score is the mean importance of all non-expired memories (0.0–1.0).
// A higher score indicates a healthier, more relevant memory store.
func (c *Client) GetFeedbackHealth(ctx context.Context, agentID string) (*FeedbackHealthResponse, error) {
	respBody, err := c.request(ctx, "GET", fmt.Sprintf("/v1/feedback/health?agent_id=%s", agentID), nil)
	if err != nil {
		return nil, err
	}
	var result FeedbackHealthResponse
	if err := json.Unmarshal(respBody, &result); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}
	return &result, nil
}

// ===========================================================================
// Memory Knowledge Graph Operations (CE-5 / SDK-9)
// ===========================================================================

// MemoryGraph traverses the knowledge graph from a memory node.
//
// Requires CE-5 (Memory Knowledge Graph) on the server.
//
// Example:
//
//	graph, err := client.MemoryGraph(ctx, "mem-abc", &GraphOptions{Depth: 2})
//	if err != nil { ... }
//	fmt.Printf("%d nodes, %d edges\n", len(graph.Nodes), len(graph.Edges))
func (c *Client) MemoryGraph(ctx context.Context, memoryID string, opts *GraphOptions) (*MemoryGraph, error) {
	depth := 1
	if opts != nil && opts.Depth > 0 {
		depth = opts.Depth
	}
	path := fmt.Sprintf("/v1/memories/%s/graph?depth=%d", url.PathEscape(memoryID), depth)
	if opts != nil && len(opts.Types) > 0 {
		typeStrs := make([]string, len(opts.Types))
		for i, t := range opts.Types {
			typeStrs[i] = string(t)
		}
		path += "&types=" + strings.Join(typeStrs, ",")
	}
	respBody, err := c.request(ctx, "GET", path, nil)
	if err != nil {
		return nil, err
	}
	var result MemoryGraph
	if err := json.Unmarshal(respBody, &result); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}
	return &result, nil
}

// MemoryPath finds the shortest path between two memories in the knowledge graph.
//
// Requires CE-5 (Memory Knowledge Graph) on the server.
func (c *Client) MemoryPath(ctx context.Context, sourceID, targetID string) (*GraphPath, error) {
	path := fmt.Sprintf("/v1/memories/%s/path?target=%s",
		url.PathEscape(sourceID),
		url.QueryEscape(targetID),
	)
	respBody, err := c.request(ctx, "GET", path, nil)
	if err != nil {
		return nil, err
	}
	var result GraphPath
	if err := json.Unmarshal(respBody, &result); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}
	return &result, nil
}

// MemoryLink creates an explicit edge between two memories.
//
// Requires CE-5 (Memory Knowledge Graph) on the server.
func (c *Client) MemoryLink(ctx context.Context, sourceID, targetID string, edgeType EdgeType) (*GraphLinkResponse, error) {
	req := GraphLinkRequest{
		TargetID: targetID,
		EdgeType: edgeType,
	}
	respBody, err := c.request(ctx, "POST", fmt.Sprintf("/v1/memories/%s/links", url.PathEscape(sourceID)), req)
	if err != nil {
		return nil, err
	}
	var result GraphLinkResponse
	if err := json.Unmarshal(respBody, &result); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}
	return &result, nil
}

// AgentGraphExport exports the full knowledge graph for an agent.
//
// Requires CE-5 (Memory Knowledge Graph) on the server.
//
// format should be "json" (default), "graphml", or "csv".
func (c *Client) AgentGraphExport(ctx context.Context, agentID, format string) (*GraphExport, error) {
	if format == "" {
		format = "json"
	}
	path := fmt.Sprintf("/v1/agents/%s/graph/export?format=%s", url.PathEscape(agentID), url.QueryEscape(format))
	respBody, err := c.request(ctx, "GET", path, nil)
	if err != nil {
		return nil, err
	}
	var result GraphExport
	if err := json.Unmarshal(respBody, &result); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}
	return &result, nil
}

// ===========================================================================
// Session Operations
// ===========================================================================

// StartSession starts a new session.
func (c *Client) StartSession(ctx context.Context, req StartSessionRequest) (*Session, error) {
	respBody, err := c.request(ctx, "POST", "/v1/sessions/start", req)
	if err != nil {
		return nil, err
	}

	var result Session
	if err := json.Unmarshal(respBody, &result); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}
	return &result, nil
}

// EndSession ends a session.
func (c *Client) EndSession(ctx context.Context, sessionID string) error {
	_, err := c.request(ctx, "POST", fmt.Sprintf("/v1/sessions/%s/end", sessionID), nil)
	return err
}

// GetSession gets session details.
func (c *Client) GetSession(ctx context.Context, sessionID string) (*Session, error) {
	respBody, err := c.request(ctx, "GET", fmt.Sprintf("/v1/sessions/%s", sessionID), nil)
	if err != nil {
		return nil, err
	}

	var result Session
	if err := json.Unmarshal(respBody, &result); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}
	return &result, nil
}

// ListSessions lists sessions with optional filters.
func (c *Client) ListSessions(ctx context.Context, opts *ListSessionsOptions) ([]Session, error) {
	path := "/v1/sessions"
	if opts != nil {
		params := url.Values{}
		if opts.AgentID != "" {
			params.Set("agent_id", opts.AgentID)
		}
		if opts.ActiveOnly != nil {
			params.Set("active_only", fmt.Sprintf("%v", *opts.ActiveOnly))
		}
		if opts.Limit != nil {
			params.Set("limit", fmt.Sprintf("%d", *opts.Limit))
		}
		if opts.Offset != nil {
			params.Set("offset", fmt.Sprintf("%d", *opts.Offset))
		}
		if encoded := params.Encode(); encoded != "" {
			path += "?" + encoded
		}
	}

	respBody, err := c.request(ctx, "GET", path, nil)
	if err != nil {
		return nil, err
	}

	var result []Session
	if err := json.Unmarshal(respBody, &result); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}
	return result, nil
}

// SessionMemories gets memories for a session.
func (c *Client) SessionMemories(ctx context.Context, sessionID string) ([]RecalledMemory, error) {
	respBody, err := c.request(ctx, "GET", fmt.Sprintf("/v1/sessions/%s/memories", sessionID), nil)
	if err != nil {
		return nil, err
	}

	var result []RecalledMemory
	if err := json.Unmarshal(respBody, &result); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}
	return result, nil
}

// ===========================================================================
// Agent Operations
// ===========================================================================

// ListAgents lists all agents.
func (c *Client) ListAgents(ctx context.Context) ([]AgentSummary, error) {
	respBody, err := c.request(ctx, "GET", "/v1/agents", nil)
	if err != nil {
		return nil, err
	}

	var result []AgentSummary
	if err := json.Unmarshal(respBody, &result); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}
	return result, nil
}

// AgentMemories gets memories for an agent.
func (c *Client) AgentMemories(ctx context.Context, agentID string, opts *AgentMemoriesOptions) ([]RecalledMemory, error) {
	path := fmt.Sprintf("/v1/agents/%s/memories", agentID)
	if opts != nil {
		params := url.Values{}
		if opts.MemoryType != "" {
			params.Set("memory_type", opts.MemoryType)
		}
		if opts.Limit != nil {
			params.Set("limit", fmt.Sprintf("%d", *opts.Limit))
		}
		if encoded := params.Encode(); encoded != "" {
			path += "?" + encoded
		}
	}

	respBody, err := c.request(ctx, "GET", path, nil)
	if err != nil {
		return nil, err
	}

	var result []RecalledMemory
	if err := json.Unmarshal(respBody, &result); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}
	return result, nil
}

// AgentStats gets stats for an agent.
func (c *Client) AgentStats(ctx context.Context, agentID string) (*AgentStats, error) {
	respBody, err := c.request(ctx, "GET", fmt.Sprintf("/v1/agents/%s/stats", agentID), nil)
	if err != nil {
		return nil, err
	}

	var result AgentStats
	if err := json.Unmarshal(respBody, &result); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}
	return &result, nil
}

// AgentSessions gets sessions for an agent.
func (c *Client) AgentSessions(ctx context.Context, agentID string, opts *AgentSessionsOptions) ([]Session, error) {
	path := fmt.Sprintf("/v1/agents/%s/sessions", agentID)
	if opts != nil {
		params := url.Values{}
		if opts.ActiveOnly != nil {
			params.Set("active_only", fmt.Sprintf("%v", *opts.ActiveOnly))
		}
		if opts.Limit != nil {
			params.Set("limit", fmt.Sprintf("%d", *opts.Limit))
		}
		if encoded := params.Encode(); encoded != "" {
			path += "?" + encoded
		}
	}

	respBody, err := c.request(ctx, "GET", path, nil)
	if err != nil {
		return nil, err
	}

	var result []Session
	if err := json.Unmarshal(respBody, &result); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}
	return result, nil
}

// ===========================================================================
// Knowledge Graph Operations
// ===========================================================================

// KnowledgeGraph builds a knowledge graph from a seed memory.
func (c *Client) KnowledgeGraph(ctx context.Context, req KnowledgeGraphRequest) (*KnowledgeGraphResponse, error) {
	data, err := c.request(ctx, "POST", "/v1/knowledge/graph", req)
	if err != nil {
		return nil, err
	}
	var result KnowledgeGraphResponse
	if err := json.Unmarshal(data, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

// FullKnowledgeGraph builds a full knowledge graph for an agent.
func (c *Client) FullKnowledgeGraph(ctx context.Context, req FullKnowledgeGraphRequest) (*KnowledgeGraphResponse, error) {
	data, err := c.request(ctx, "POST", "/v1/knowledge/graph/full", req)
	if err != nil {
		return nil, err
	}
	var result KnowledgeGraphResponse
	if err := json.Unmarshal(data, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

// Summarize summarizes memories.
func (c *Client) Summarize(ctx context.Context, req SummarizeRequest) (*SummarizeResponse, error) {
	data, err := c.request(ctx, "POST", "/v1/knowledge/summarize", req)
	if err != nil {
		return nil, err
	}
	var result SummarizeResponse
	if err := json.Unmarshal(data, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

// Deduplicate deduplicates memories.
func (c *Client) Deduplicate(ctx context.Context, req DeduplicateRequest) (*DeduplicateResponse, error) {
	data, err := c.request(ctx, "POST", "/v1/knowledge/deduplicate", req)
	if err != nil {
		return nil, err
	}
	var result DeduplicateResponse
	if err := json.Unmarshal(data, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

// ===========================================================================
// Analytics Operations
// ===========================================================================

// AnalyticsOverview gets the analytics overview.
func (c *Client) AnalyticsOverview(ctx context.Context, opts *AnalyticsOptions) (*AnalyticsOverview, error) {
	path := "/v1/analytics/overview"
	if opts != nil {
		params := url.Values{}
		if opts.Period != "" {
			params.Set("period", opts.Period)
		}
		if opts.Namespace != "" {
			params.Set("namespace", opts.Namespace)
		}
		if encoded := params.Encode(); encoded != "" {
			path += "?" + encoded
		}
	}
	data, err := c.request(ctx, "GET", path, nil)
	if err != nil {
		return nil, err
	}
	var result AnalyticsOverview
	if err := json.Unmarshal(data, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

// AnalyticsLatency gets latency analytics.
func (c *Client) AnalyticsLatency(ctx context.Context, opts *AnalyticsOptions) (*LatencyAnalytics, error) {
	path := "/v1/analytics/latency"
	if opts != nil {
		params := url.Values{}
		if opts.Period != "" {
			params.Set("period", opts.Period)
		}
		if opts.Namespace != "" {
			params.Set("namespace", opts.Namespace)
		}
		if encoded := params.Encode(); encoded != "" {
			path += "?" + encoded
		}
	}
	data, err := c.request(ctx, "GET", path, nil)
	if err != nil {
		return nil, err
	}
	var result LatencyAnalytics
	if err := json.Unmarshal(data, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

// AnalyticsThroughput gets throughput analytics.
func (c *Client) AnalyticsThroughput(ctx context.Context, opts *AnalyticsOptions) (*ThroughputAnalytics, error) {
	path := "/v1/analytics/throughput"
	if opts != nil {
		params := url.Values{}
		if opts.Period != "" {
			params.Set("period", opts.Period)
		}
		if opts.Namespace != "" {
			params.Set("namespace", opts.Namespace)
		}
		if encoded := params.Encode(); encoded != "" {
			path += "?" + encoded
		}
	}
	data, err := c.request(ctx, "GET", path, nil)
	if err != nil {
		return nil, err
	}
	var result ThroughputAnalytics
	if err := json.Unmarshal(data, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

// AnalyticsStorage gets storage analytics.
func (c *Client) AnalyticsStorage(ctx context.Context, namespace string) (*StorageAnalytics, error) {
	path := "/v1/analytics/storage"
	if namespace != "" {
		path += "?namespace=" + url.QueryEscape(namespace)
	}
	data, err := c.request(ctx, "GET", path, nil)
	if err != nil {
		return nil, err
	}
	var result StorageAnalytics
	if err := json.Unmarshal(data, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

// ===========================================================================
// Advanced Search Operations
// ===========================================================================

// MultiVectorSearch performs a multi-vector search with positive/negative vectors and optional MMR.
func (c *Client) MultiVectorSearch(ctx context.Context, namespace string, req MultiVectorSearchRequest) (*MultiVectorSearchResponse, error) {
	data, err := c.request(ctx, "POST", fmt.Sprintf("/v1/namespaces/%s/multi-vector", url.PathEscape(namespace)), req)
	if err != nil {
		return nil, err
	}
	var result MultiVectorSearchResponse
	if err := json.Unmarshal(data, &result); err != nil {
		return nil, fmt.Errorf("failed to unmarshal multi-vector search response: %w", err)
	}
	return &result, nil
}

// UnifiedQuery performs a unified query combining vector and text search.
func (c *Client) UnifiedQuery(ctx context.Context, namespace string, req UnifiedQueryRequest) (*UnifiedQueryResponse, error) {
	data, err := c.request(ctx, "POST", fmt.Sprintf("/v1/namespaces/%s/unified-query", url.PathEscape(namespace)), req)
	if err != nil {
		return nil, err
	}
	var result UnifiedQueryResponse
	if err := json.Unmarshal(data, &result); err != nil {
		return nil, fmt.Errorf("failed to unmarshal unified query response: %w", err)
	}
	return &result, nil
}

// Aggregate performs aggregation with grouping.
func (c *Client) Aggregate(ctx context.Context, namespace string, req AggregationRequest) (*AggregationResponse, error) {
	data, err := c.request(ctx, "POST", fmt.Sprintf("/v1/namespaces/%s/aggregate", url.PathEscape(namespace)), req)
	if err != nil {
		return nil, err
	}
	var result AggregationResponse
	if err := json.Unmarshal(data, &result); err != nil {
		return nil, fmt.Errorf("failed to unmarshal aggregation response: %w", err)
	}
	return &result, nil
}

// ExportVectors exports vectors with pagination.
func (c *Client) ExportVectors(ctx context.Context, namespace string, req ExportRequest) (*ExportResponse, error) {
	data, err := c.request(ctx, "POST", fmt.Sprintf("/v1/namespaces/%s/export", url.PathEscape(namespace)), req)
	if err != nil {
		return nil, err
	}
	var result ExportResponse
	if err := json.Unmarshal(data, &result); err != nil {
		return nil, fmt.Errorf("failed to unmarshal export response: %w", err)
	}
	return &result, nil
}

// ExplainQuery explains a query execution plan and returns timing information.
func (c *Client) ExplainQuery(ctx context.Context, namespace string, req QueryExplainRequest) (*QueryExplainResponse, error) {
	data, err := c.request(ctx, "POST", fmt.Sprintf("/v1/namespaces/%s/explain", url.PathEscape(namespace)), req)
	if err != nil {
		return nil, err
	}
	var result QueryExplainResponse
	if err := json.Unmarshal(data, &result); err != nil {
		return nil, fmt.Errorf("failed to unmarshal explain response: %w", err)
	}
	return &result, nil
}

// UpsertColumns performs a column-format upsert for efficient bulk operations.
func (c *Client) UpsertColumns(ctx context.Context, namespace string, req ColumnUpsertRequest) (*UpsertResponse, error) {
	data, err := c.request(ctx, "POST", fmt.Sprintf("/v1/namespaces/%s/upsert-columns", url.PathEscape(namespace)), req)
	if err != nil {
		return nil, err
	}
	var result UpsertResponse
	if err := json.Unmarshal(data, &result); err != nil {
		return nil, fmt.Errorf("failed to unmarshal upsert columns response: %w", err)
	}
	return &result, nil
}

// WarmCache warms the cache for vectors in a namespace.
func (c *Client) WarmCache(ctx context.Context, namespace string, req WarmCacheRequest) (*WarmCacheResponse, error) {
	data, err := c.request(ctx, "POST", fmt.Sprintf("/v1/namespaces/%s/cache/warm", url.PathEscape(namespace)), req)
	if err != nil {
		return nil, err
	}
	var result WarmCacheResponse
	if err := json.Unmarshal(data, &result); err != nil {
		return nil, fmt.Errorf("failed to unmarshal warm cache response: %w", err)
	}
	return &result, nil
}

// ===========================================================================
// Admin Operations (Extended)
// ===========================================================================

// OpsStats gets server stats (version, total_vectors, namespace_count, uptime_seconds, timestamp, state).
// Requires Read scope — works with read-only API keys, unlike ClusterStatus.
func (c *Client) OpsStats(ctx context.Context) (*OpsStats, error) {
	data, err := c.request(ctx, "GET", "/v1/ops/stats", nil)
	if err != nil {
		return nil, err
	}
	var result OpsStats
	if err := json.Unmarshal(data, &result); err != nil {
		return nil, fmt.Errorf("failed to unmarshal ops stats: %w", err)
	}
	return &result, nil
}

// OpsMetrics returns the Prometheus metrics in text exposition format (INFRA-3).
// Requires Admin scope. Returns the raw Prometheus text exposition format string
// suitable for scraping by a Prometheus server.
func (c *Client) OpsMetrics(ctx context.Context) (string, error) {
	data, err := c.request(ctx, "GET", "/v1/ops/metrics", nil)
	if err != nil {
		return "", err
	}
	return string(data), nil
}

// ClusterStatus gets the cluster status.
func (c *Client) ClusterStatus(ctx context.Context) (*ClusterStatus, error) {
	data, err := c.request(ctx, "GET", "/v1/admin/cluster/status", nil)
	if err != nil {
		return nil, err
	}
	var result ClusterStatus
	if err := json.Unmarshal(data, &result); err != nil {
		return nil, fmt.Errorf("failed to unmarshal cluster status: %w", err)
	}
	return &result, nil
}

// ClusterNodes gets the cluster nodes.
func (c *Client) ClusterNodes(ctx context.Context) ([]ClusterNode, error) {
	data, err := c.request(ctx, "GET", "/v1/admin/cluster/nodes", nil)
	if err != nil {
		return nil, err
	}
	var result []ClusterNode
	if err := json.Unmarshal(data, &result); err != nil {
		return nil, fmt.Errorf("failed to unmarshal cluster nodes: %w", err)
	}
	return result, nil
}

// OptimizeNamespace optimizes a namespace.
func (c *Client) OptimizeNamespace(ctx context.Context, namespace string) (*StatusResponse, error) {
	data, err := c.request(ctx, "POST", fmt.Sprintf("/v1/admin/namespaces/%s/optimize", url.PathEscape(namespace)), nil)
	if err != nil {
		return nil, err
	}
	var result StatusResponse
	if err := json.Unmarshal(data, &result); err != nil {
		return nil, fmt.Errorf("failed to unmarshal response: %w", err)
	}
	return &result, nil
}

// AdminIndexStats gets index stats for a namespace via admin endpoint.
func (c *Client) AdminIndexStats(ctx context.Context, namespace string) (map[string]interface{}, error) {
	data, err := c.request(ctx, "GET", fmt.Sprintf("/v1/admin/namespaces/%s/index/stats", url.PathEscape(namespace)), nil)
	if err != nil {
		return nil, err
	}
	var result map[string]interface{}
	if err := json.Unmarshal(data, &result); err != nil {
		return nil, fmt.Errorf("failed to unmarshal index stats: %w", err)
	}
	return result, nil
}

// RebuildIndexes rebuilds indexes for a namespace.
func (c *Client) RebuildIndexes(ctx context.Context, namespace string) (*StatusResponse, error) {
	data, err := c.request(ctx, "POST", fmt.Sprintf("/v1/admin/namespaces/%s/index/rebuild", url.PathEscape(namespace)), nil)
	if err != nil {
		return nil, err
	}
	var result StatusResponse
	if err := json.Unmarshal(data, &result); err != nil {
		return nil, fmt.Errorf("failed to unmarshal response: %w", err)
	}
	return &result, nil
}

// CacheStats gets cache statistics.
func (c *Client) CacheStats(ctx context.Context) (*CacheStats, error) {
	data, err := c.request(ctx, "GET", "/v1/admin/cache/stats", nil)
	if err != nil {
		return nil, err
	}
	var result CacheStats
	if err := json.Unmarshal(data, &result); err != nil {
		return nil, fmt.Errorf("failed to unmarshal cache stats: %w", err)
	}
	return &result, nil
}

// CacheClear clears cache, optionally for a specific namespace.
func (c *Client) CacheClear(ctx context.Context, namespace string) (*StatusResponse, error) {
	var body interface{}
	if namespace != "" {
		body = map[string]string{"namespace": namespace}
	}
	data, err := c.request(ctx, "POST", "/v1/admin/cache/clear", body)
	if err != nil {
		return nil, err
	}
	var result StatusResponse
	if err := json.Unmarshal(data, &result); err != nil {
		return nil, fmt.Errorf("failed to unmarshal response: %w", err)
	}
	return &result, nil
}

// GetConfig gets the server configuration.
func (c *Client) GetConfig(ctx context.Context) (map[string]interface{}, error) {
	data, err := c.request(ctx, "GET", "/v1/admin/config", nil)
	if err != nil {
		return nil, err
	}
	var result map[string]interface{}
	if err := json.Unmarshal(data, &result); err != nil {
		return nil, fmt.Errorf("failed to unmarshal config: %w", err)
	}
	return result, nil
}

// UpdateConfig updates the server configuration.
func (c *Client) UpdateConfig(ctx context.Context, config map[string]interface{}) (map[string]interface{}, error) {
	data, err := c.request(ctx, "PUT", "/v1/admin/config", config)
	if err != nil {
		return nil, err
	}
	var result map[string]interface{}
	if err := json.Unmarshal(data, &result); err != nil {
		return nil, fmt.Errorf("failed to unmarshal config: %w", err)
	}
	return result, nil
}

// GetQuotas gets quota settings.
func (c *Client) GetQuotas(ctx context.Context) (map[string]interface{}, error) {
	data, err := c.request(ctx, "GET", "/v1/admin/quotas", nil)
	if err != nil {
		return nil, err
	}
	var result map[string]interface{}
	if err := json.Unmarshal(data, &result); err != nil {
		return nil, fmt.Errorf("failed to unmarshal quotas: %w", err)
	}
	return result, nil
}

// UpdateQuotas updates quota settings.
func (c *Client) UpdateQuotas(ctx context.Context, quotas map[string]interface{}) (map[string]interface{}, error) {
	data, err := c.request(ctx, "PUT", "/v1/admin/quotas", quotas)
	if err != nil {
		return nil, err
	}
	var result map[string]interface{}
	if err := json.Unmarshal(data, &result); err != nil {
		return nil, fmt.Errorf("failed to unmarshal quotas: %w", err)
	}
	return result, nil
}

// SlowQueries gets slow queries.
func (c *Client) SlowQueries(ctx context.Context, opts *SlowQueryOptions) ([]SlowQuery, error) {
	path := "/v1/admin/slow-queries"
	if opts != nil {
		params := url.Values{}
		if opts.Limit > 0 {
			params.Set("limit", strconv.Itoa(opts.Limit))
		}
		if opts.MinDurationMs > 0 {
			params.Set("min_duration_ms", strconv.Itoa(opts.MinDurationMs))
		}
		if encoded := params.Encode(); encoded != "" {
			path += "?" + encoded
		}
	}
	data, err := c.request(ctx, "GET", path, nil)
	if err != nil {
		return nil, err
	}
	var result []SlowQuery
	if err := json.Unmarshal(data, &result); err != nil {
		return nil, fmt.Errorf("failed to unmarshal slow queries: %w", err)
	}
	return result, nil
}

// CreateBackup creates a backup.
func (c *Client) CreateBackup(ctx context.Context, includeData bool) (*BackupInfo, error) {
	body := map[string]interface{}{"include_data": includeData}
	data, err := c.request(ctx, "POST", "/v1/admin/backups", body)
	if err != nil {
		return nil, err
	}
	var result BackupInfo
	if err := json.Unmarshal(data, &result); err != nil {
		return nil, fmt.Errorf("failed to unmarshal backup info: %w", err)
	}
	return &result, nil
}

// ListBackups lists all backups.
func (c *Client) ListBackups(ctx context.Context) ([]BackupInfo, error) {
	data, err := c.request(ctx, "GET", "/v1/admin/backups", nil)
	if err != nil {
		return nil, err
	}
	var result []BackupInfo
	if err := json.Unmarshal(data, &result); err != nil {
		return nil, fmt.Errorf("failed to unmarshal backups: %w", err)
	}
	return result, nil
}

// RestoreBackup restores a backup.
func (c *Client) RestoreBackup(ctx context.Context, backupID string) (*StatusResponse, error) {
	data, err := c.request(ctx, "POST", fmt.Sprintf("/v1/admin/backups/%s/restore", url.PathEscape(backupID)), nil)
	if err != nil {
		return nil, err
	}
	var result StatusResponse
	if err := json.Unmarshal(data, &result); err != nil {
		return nil, fmt.Errorf("failed to unmarshal response: %w", err)
	}
	return &result, nil
}

// DeleteBackup deletes a backup.
func (c *Client) DeleteBackup(ctx context.Context, backupID string) error {
	_, err := c.request(ctx, "DELETE", fmt.Sprintf("/v1/admin/backups/%s", url.PathEscape(backupID)), nil)
	return err
}

// ConfigureTTL configures TTL for a namespace.
func (c *Client) ConfigureTTL(ctx context.Context, namespace string, ttlSeconds int, strategy string) (*TtlConfig, error) {
	body := map[string]interface{}{
		"ttl_seconds": ttlSeconds,
	}
	if strategy != "" {
		body["strategy"] = strategy
	}
	data, err := c.request(ctx, "POST", fmt.Sprintf("/v1/admin/namespaces/%s/ttl", url.PathEscape(namespace)), body)
	if err != nil {
		return nil, err
	}
	var result TtlConfig
	if err := json.Unmarshal(data, &result); err != nil {
		return nil, fmt.Errorf("failed to unmarshal ttl config: %w", err)
	}
	return &result, nil
}

// ===========================================================================
// AutoPilot Management (PILOT-1 / PILOT-2 / PILOT-3)
// ===========================================================================

// AutopilotStatus returns the current AutoPilot config and last-run statistics (PILOT-1).
func (c *Client) AutopilotStatus(ctx context.Context) (*AutoPilotStatusResponse, error) {
	data, err := c.request(ctx, "GET", "/v1/admin/autopilot/status", nil)
	if err != nil {
		return nil, err
	}
	var result AutoPilotStatusResponse
	if err := json.Unmarshal(data, &result); err != nil {
		return nil, fmt.Errorf("failed to unmarshal autopilot status: %w", err)
	}
	return &result, nil
}

// AutopilotUpdateConfig updates the AutoPilot configuration at runtime (PILOT-2).
// All fields in req are optional — nil means "keep current value".
func (c *Client) AutopilotUpdateConfig(ctx context.Context, req AutoPilotConfigRequest) (*AutoPilotConfigResponse, error) {
	data, err := c.request(ctx, "PUT", "/v1/admin/autopilot/config", req)
	if err != nil {
		return nil, err
	}
	var result AutoPilotConfigResponse
	if err := json.Unmarshal(data, &result); err != nil {
		return nil, fmt.Errorf("failed to unmarshal autopilot config response: %w", err)
	}
	return &result, nil
}

// AutopilotTrigger manually triggers an AutoPilot dedup or consolidation cycle (PILOT-3).
// action must be one of "dedup", "consolidate", or "all".
func (c *Client) AutopilotTrigger(ctx context.Context, action string) (*AutoPilotTriggerResponse, error) {
	body := map[string]string{"action": action}
	data, err := c.request(ctx, "POST", "/v1/admin/autopilot/trigger", body)
	if err != nil {
		return nil, err
	}
	var result AutoPilotTriggerResponse
	if err := json.Unmarshal(data, &result); err != nil {
		return nil, fmt.Errorf("failed to unmarshal autopilot trigger response: %w", err)
	}
	return &result, nil
}

// DecayConfig returns the current decay engine configuration (DECAY-1).
// Requires Admin scope.
func (c *Client) DecayConfig(ctx context.Context) (*DecayConfigResponse, error) {
	data, err := c.request(ctx, "GET", "/v1/admin/decay/config", nil)
	if err != nil {
		return nil, err
	}
	var result DecayConfigResponse
	if err := json.Unmarshal(data, &result); err != nil {
		return nil, fmt.Errorf("failed to unmarshal decay config response: %w", err)
	}
	return &result, nil
}

// DecayUpdateConfig updates the decay engine configuration at runtime (DECAY-1).
// Changes take effect on the next decay cycle — no restart required.
// All fields in req are optional; omit any to keep its current value.
// Requires Admin scope.
func (c *Client) DecayUpdateConfig(ctx context.Context, req DecayConfigUpdateRequest) (*DecayConfigUpdateResponse, error) {
	data, err := c.request(ctx, "PUT", "/v1/admin/decay/config", req)
	if err != nil {
		return nil, err
	}
	var result DecayConfigUpdateResponse
	if err := json.Unmarshal(data, &result); err != nil {
		return nil, fmt.Errorf("failed to unmarshal decay config update response: %w", err)
	}
	return &result, nil
}

// DecayStats returns cumulative decay counters and a last-cycle snapshot (DECAY-2).
// Requires Admin scope.
func (c *Client) DecayStats(ctx context.Context) (*DecayStatsResponse, error) {
	data, err := c.request(ctx, "GET", "/v1/admin/decay/stats", nil)
	if err != nil {
		return nil, err
	}
	var result DecayStatsResponse
	if err := json.Unmarshal(data, &result); err != nil {
		return nil, fmt.Errorf("failed to unmarshal decay stats response: %w", err)
	}
	return &result, nil
}

// ===========================================================================
// API Key Operations
// ===========================================================================

// CreateKey creates a new API key.
func (c *Client) CreateKey(ctx context.Context, req CreateKeyRequest) (*ApiKey, error) {
	data, err := c.request(ctx, "POST", "/v1/keys", req)
	if err != nil {
		return nil, err
	}
	var result ApiKey
	if err := json.Unmarshal(data, &result); err != nil {
		return nil, fmt.Errorf("failed to unmarshal api key: %w", err)
	}
	return &result, nil
}

// ListKeys lists all API keys.
func (c *Client) ListKeys(ctx context.Context) ([]ApiKey, error) {
	data, err := c.request(ctx, "GET", "/v1/keys", nil)
	if err != nil {
		return nil, err
	}
	var result []ApiKey
	if err := json.Unmarshal(data, &result); err != nil {
		return nil, fmt.Errorf("failed to unmarshal api keys: %w", err)
	}
	return result, nil
}

// GetKey gets an API key by ID.
func (c *Client) GetKey(ctx context.Context, keyID string) (*ApiKey, error) {
	data, err := c.request(ctx, "GET", fmt.Sprintf("/v1/keys/%s", url.PathEscape(keyID)), nil)
	if err != nil {
		return nil, err
	}
	var result ApiKey
	if err := json.Unmarshal(data, &result); err != nil {
		return nil, fmt.Errorf("failed to unmarshal api key: %w", err)
	}
	return &result, nil
}

// DeleteKey deletes an API key.
func (c *Client) DeleteKey(ctx context.Context, keyID string) error {
	_, err := c.request(ctx, "DELETE", fmt.Sprintf("/v1/keys/%s", url.PathEscape(keyID)), nil)
	return err
}

// DeactivateKey deactivates an API key.
func (c *Client) DeactivateKey(ctx context.Context, keyID string) (*ApiKey, error) {
	data, err := c.request(ctx, "POST", fmt.Sprintf("/v1/keys/%s/deactivate", url.PathEscape(keyID)), nil)
	if err != nil {
		return nil, err
	}
	var result ApiKey
	if err := json.Unmarshal(data, &result); err != nil {
		return nil, fmt.Errorf("failed to unmarshal api key: %w", err)
	}
	return &result, nil
}

// RotateKey rotates an API key.
func (c *Client) RotateKey(ctx context.Context, keyID string) (*ApiKey, error) {
	data, err := c.request(ctx, "POST", fmt.Sprintf("/v1/keys/%s/rotate", url.PathEscape(keyID)), nil)
	if err != nil {
		return nil, err
	}
	var result ApiKey
	if err := json.Unmarshal(data, &result); err != nil {
		return nil, fmt.Errorf("failed to unmarshal api key: %w", err)
	}
	return &result, nil
}

// KeyUsage gets usage statistics for an API key.
func (c *Client) KeyUsage(ctx context.Context, keyID string) (*KeyUsage, error) {
	data, err := c.request(ctx, "GET", fmt.Sprintf("/v1/keys/%s/usage", url.PathEscape(keyID)), nil)
	if err != nil {
		return nil, err
	}
	var result KeyUsage
	if err := json.Unmarshal(data, &result); err != nil {
		return nil, fmt.Errorf("failed to unmarshal key usage: %w", err)
	}
	return &result, nil
}

// ===========================================================================
// Cross-Agent Network Operations (DASH-A)
// ===========================================================================

// CrossAgentNetwork builds the cross-agent memory similarity network.
// POST /v1/knowledge/network/cross-agent — requires Admin scope.
func (c *Client) CrossAgentNetwork(ctx context.Context, req CrossAgentNetworkRequest) (*CrossAgentNetworkResponse, error) {
	data, err := c.request(ctx, "POST", "/v1/knowledge/network/cross-agent", req)
	if err != nil {
		return nil, err
	}
	var result CrossAgentNetworkResponse
	if err := json.Unmarshal(data, &result); err != nil {
		return nil, fmt.Errorf("failed to unmarshal cross-agent network response: %w", err)
	}
	return &result, nil
}

// ===========================================================================
// KG-2: Graph Query & Export
// ===========================================================================

// KnowledgeQuery queries the memory knowledge graph using a filter DSL (KG-2).
// GET /v1/knowledge/query
//
// agentID is required. Optional params: rootID (BFS root memory), edgeType
// (comma-separated, e.g. "related_to,shares_entity"), minWeight (0.0–1.0),
// maxDepth (1–5, default 3), limit (default 100, max 1000).
func (c *Client) KnowledgeQuery(
	ctx context.Context,
	agentID string,
	rootID string,
	edgeType string,
	minWeight float64,
	maxDepth int,
	limit int,
) (*KgQueryResponse, error) {
	params := url.Values{"agent_id": {agentID}}
	if rootID != "" {
		params.Set("root_id", rootID)
	}
	if edgeType != "" {
		params.Set("edge_type", edgeType)
	}
	if minWeight > 0 {
		params.Set("min_weight", fmt.Sprintf("%g", minWeight))
	}
	if maxDepth > 0 {
		params.Set("max_depth", fmt.Sprintf("%d", maxDepth))
	}
	if limit > 0 {
		params.Set("limit", fmt.Sprintf("%d", limit))
	}
	data, err := c.request(ctx, "GET", "/v1/knowledge/query?"+params.Encode(), nil)
	if err != nil {
		return nil, err
	}
	var result KgQueryResponse
	if err := json.Unmarshal(data, &result); err != nil {
		return nil, fmt.Errorf("failed to unmarshal kg query response: %w", err)
	}
	return &result, nil
}

// KnowledgePath finds the BFS shortest path between two memory IDs (KG-2).
// GET /v1/knowledge/path
//
// Returns an error if no path exists between fromID and toID.
func (c *Client) KnowledgePath(ctx context.Context, agentID, fromID, toID string) (*KgPathResponse, error) {
	params := url.Values{
		"agent_id": {agentID},
		"from":     {fromID},
		"to":       {toID},
	}
	data, err := c.request(ctx, "GET", "/v1/knowledge/path?"+params.Encode(), nil)
	if err != nil {
		return nil, err
	}
	var result KgPathResponse
	if err := json.Unmarshal(data, &result); err != nil {
		return nil, fmt.Errorf("failed to unmarshal kg path response: %w", err)
	}
	return &result, nil
}

// KnowledgeExport exports the memory knowledge graph as JSON or GraphML (KG-2).
// GET /v1/knowledge/export
//
// format is "json" (default) or "graphml". For graphml the server returns
// application/xml — this method deserializes JSON only.
func (c *Client) KnowledgeExport(ctx context.Context, agentID, format string) (*KgExportResponse, error) {
	if format == "" {
		format = "json"
	}
	params := url.Values{
		"agent_id": {agentID},
		"format":   {format},
	}
	data, err := c.request(ctx, "GET", "/v1/knowledge/export?"+params.Encode(), nil)
	if err != nil {
		return nil, err
	}
	var result KgExportResponse
	if err := json.Unmarshal(data, &result); err != nil {
		return nil, fmt.Errorf("failed to unmarshal kg export response: %w", err)
	}
	return &result, nil
}

// ===========================================================================
// CE-4 Entity Extraction (GLiNER)
// ===========================================================================

// ConfigureNamespaceNer configures entity extraction for a namespace (CE-4).
// PATCH /v1/namespaces/{namespace}/config — requires Write scope.
func (c *Client) ConfigureNamespaceNer(ctx context.Context, namespace string, config NamespaceNerConfig) (map[string]interface{}, error) {
	data, err := c.request(ctx, "PATCH", fmt.Sprintf("/v1/namespaces/%s/config", url.PathEscape(namespace)), config)
	if err != nil {
		return nil, err
	}
	var result map[string]interface{}
	if err := json.Unmarshal(data, &result); err != nil {
		return nil, fmt.Errorf("failed to unmarshal namespace ner config response: %w", err)
	}
	return result, nil
}

// ExtractEntities extracts named entities from arbitrary text using GLiNER (CE-4).
// POST /v1/memories/extract — requires Read scope.
// entityTypes may be nil to use the server default types.
func (c *Client) ExtractEntities(ctx context.Context, text string, entityTypes []string) (*EntityExtractionResponse, error) {
	body := map[string]interface{}{
		"text": text,
	}
	if entityTypes != nil {
		body["entity_types"] = entityTypes
	}
	data, err := c.request(ctx, "POST", "/v1/memories/extract", body)
	if err != nil {
		return nil, err
	}
	var result EntityExtractionResponse
	if err := json.Unmarshal(data, &result); err != nil {
		return nil, fmt.Errorf("failed to unmarshal entity extraction response: %w", err)
	}
	return &result, nil
}

// MemoryEntities returns the entity tags attached to a stored memory (CE-4).
// GET /v1/memory/entities/{memoryID} — requires Read scope.
func (c *Client) MemoryEntities(ctx context.Context, memoryID string) (*MemoryEntitiesResponse, error) {
	data, err := c.request(ctx, "GET", fmt.Sprintf("/v1/memory/entities/%s", url.PathEscape(memoryID)), nil)
	if err != nil {
		return nil, err
	}
	var result MemoryEntitiesResponse
	if err := json.Unmarshal(data, &result); err != nil {
		return nil, fmt.Errorf("failed to unmarshal memory entities response: %w", err)
	}
	return &result, nil
}

// CreateNamespaceKey creates a namespace-scoped API key (SEC-1).
// POST /v1/namespaces/{namespace}/keys
// The Key field in the response is shown only once — store it securely.
func (c *Client) CreateNamespaceKey(ctx context.Context, namespace string, req CreateNamespaceKeyRequest) (*CreateNamespaceKeyResponse, error) {
	data, err := c.request(ctx, "POST", fmt.Sprintf("/v1/namespaces/%s/keys", url.PathEscape(namespace)), req)
	if err != nil {
		return nil, err
	}
	var result CreateNamespaceKeyResponse
	if err := json.Unmarshal(data, &result); err != nil {
		return nil, fmt.Errorf("failed to unmarshal create namespace key response: %w", err)
	}
	return &result, nil
}

// ListNamespaceKeys lists all API keys scoped to a namespace (SEC-1).
// GET /v1/namespaces/{namespace}/keys
func (c *Client) ListNamespaceKeys(ctx context.Context, namespace string) (*ListNamespaceKeysResponse, error) {
	data, err := c.request(ctx, "GET", fmt.Sprintf("/v1/namespaces/%s/keys", url.PathEscape(namespace)), nil)
	if err != nil {
		return nil, err
	}
	var result ListNamespaceKeysResponse
	if err := json.Unmarshal(data, &result); err != nil {
		return nil, fmt.Errorf("failed to unmarshal list namespace keys response: %w", err)
	}
	return &result, nil
}

// DeleteNamespaceKey revokes a namespace-scoped API key (SEC-1).
// DELETE /v1/namespaces/{namespace}/keys/{keyID}
func (c *Client) DeleteNamespaceKey(ctx context.Context, namespace string, keyID string) (*KeySuccessResponse, error) {
	data, err := c.request(ctx, "DELETE", fmt.Sprintf("/v1/namespaces/%s/keys/%s", url.PathEscape(namespace), url.PathEscape(keyID)), nil)
	if err != nil {
		return nil, err
	}
	var result KeySuccessResponse
	if err := json.Unmarshal(data, &result); err != nil {
		return nil, fmt.Errorf("failed to unmarshal delete namespace key response: %w", err)
	}
	return &result, nil
}

// NamespaceKeyUsage returns usage statistics for a namespace-scoped API key (SEC-1).
// GET /v1/namespaces/{namespace}/keys/{keyID}/usage
func (c *Client) NamespaceKeyUsage(ctx context.Context, namespace string, keyID string) (*NamespaceKeyUsageResponse, error) {
	data, err := c.request(ctx, "GET", fmt.Sprintf("/v1/namespaces/%s/keys/%s/usage", url.PathEscape(namespace), url.PathEscape(keyID)), nil)
	if err != nil {
		return nil, err
	}
	var result NamespaceKeyUsageResponse
	if err := json.Unmarshal(data, &result); err != nil {
		return nil, fmt.Errorf("failed to unmarshal namespace key usage response: %w", err)
	}
	return &result, nil
}

// ImportMemories imports memories from an external format (DX-1).
// POST /v1/import
// format: "mem0", "zep", "jsonl", or "csv". agentID and namespace are optional.
func (c *Client) ImportMemories(ctx context.Context, data interface{}, format string, agentID string, namespace string) (*MemoryImportResponse, error) {
	body := map[string]interface{}{
		"data":   data,
		"format": format,
	}
	if agentID != "" {
		body["agent_id"] = agentID
	}
	if namespace != "" {
		body["namespace"] = namespace
	}
	resp, err := c.request(ctx, "POST", "/v1/import", body)
	if err != nil {
		return nil, err
	}
	var result MemoryImportResponse
	if err := json.Unmarshal(resp, &result); err != nil {
		return nil, fmt.Errorf("failed to unmarshal import memories response: %w", err)
	}
	return &result, nil
}

// ExportMemories exports memories in a portable format (DX-1).
// GET /v1/export
// format: "mem0", "zep", "jsonl", or "csv". agentID, namespace, and limit are optional (zero/empty = omitted).
func (c *Client) ExportMemories(ctx context.Context, format string, agentID string, namespace string, limit int) (*MemoryExportResponse, error) {
	params := url.Values{}
	params.Set("format", format)
	if agentID != "" {
		params.Set("agent_id", agentID)
	}
	if namespace != "" {
		params.Set("namespace", namespace)
	}
	if limit > 0 {
		params.Set("limit", fmt.Sprintf("%d", limit))
	}
	path := "/v1/export?" + params.Encode()
	resp, err := c.request(ctx, "GET", path, nil)
	if err != nil {
		return nil, err
	}
	var result MemoryExportResponse
	if err := json.Unmarshal(resp, &result); err != nil {
		return nil, fmt.Errorf("failed to unmarshal export memories response: %w", err)
	}
	return &result, nil
}

// ListAuditEvents queries the paginated business-event audit log (OBS-1).
// GET /v1/audit
func (c *Client) ListAuditEvents(ctx context.Context, query AuditQuery) (*AuditListResponse, error) {
	params := url.Values{}
	if query.AgentID != "" {
		params.Set("agent_id", query.AgentID)
	}
	if query.EventType != "" {
		params.Set("event_type", query.EventType)
	}
	if query.FromTs > 0 {
		params.Set("from", fmt.Sprintf("%d", query.FromTs))
	}
	if query.ToTs > 0 {
		params.Set("to", fmt.Sprintf("%d", query.ToTs))
	}
	if query.Limit > 0 {
		params.Set("limit", fmt.Sprintf("%d", query.Limit))
	}
	if query.Cursor != "" {
		params.Set("cursor", query.Cursor)
	}
	path := "/v1/audit"
	if len(params) > 0 {
		path += "?" + params.Encode()
	}
	resp, err := c.request(ctx, "GET", path, nil)
	if err != nil {
		return nil, err
	}
	var result AuditListResponse
	if err := json.Unmarshal(resp, &result); err != nil {
		return nil, fmt.Errorf("failed to unmarshal audit list response: %w", err)
	}
	return &result, nil
}

// ExportAudit bulk-exports audit log entries (OBS-1).
// POST /v1/audit/export
// agentID, eventType, fromTs, and toTs are optional (zero/empty = omitted).
func (c *Client) ExportAudit(ctx context.Context, format string, agentID string, eventType string, fromTs int64, toTs int64) (*AuditExportResponse, error) {
	body := map[string]interface{}{
		"format": format,
	}
	if agentID != "" {
		body["agent_id"] = agentID
	}
	if eventType != "" {
		body["event_type"] = eventType
	}
	if fromTs > 0 {
		body["from"] = fromTs
	}
	if toTs > 0 {
		body["to"] = toTs
	}
	resp, err := c.request(ctx, "POST", "/v1/audit/export", body)
	if err != nil {
		return nil, err
	}
	var result AuditExportResponse
	if err := json.Unmarshal(resp, &result); err != nil {
		return nil, fmt.Errorf("failed to unmarshal audit export response: %w", err)
	}
	return &result, nil
}

// ExtractText extracts entities from text using a pluggable provider (EXT-1).
// POST /v1/extract
// namespace, provider, and model are optional (empty = server default).
func (c *Client) ExtractText(ctx context.Context, text string, namespace string, provider string, model string) (*ExtractionResult, error) {
	body := map[string]interface{}{
		"text": text,
	}
	if namespace != "" {
		body["namespace"] = namespace
	}
	if provider != "" {
		body["provider"] = provider
	}
	if model != "" {
		body["model"] = model
	}
	resp, err := c.request(ctx, "POST", "/v1/extract", body)
	if err != nil {
		return nil, err
	}
	var result ExtractionResult
	if err := json.Unmarshal(resp, &result); err != nil {
		return nil, fmt.Errorf("failed to unmarshal extraction result: %w", err)
	}
	return &result, nil
}

// ListExtractProviders lists available extraction providers (EXT-1).
// GET /v1/extract/providers
func (c *Client) ListExtractProviders(ctx context.Context) ([]ExtractionProviderInfo, error) {
	resp, err := c.request(ctx, "GET", "/v1/extract/providers", nil)
	if err != nil {
		return nil, err
	}
	// Server may return a top-level array or {"providers": [...]}
	resp = []byte(strings.TrimSpace(string(resp)))
	if len(resp) > 0 && resp[0] == '[' {
		var result []ExtractionProviderInfo
		if err := json.Unmarshal(resp, &result); err != nil {
			return nil, fmt.Errorf("failed to unmarshal extract providers response: %w", err)
		}
		return result, nil
	}
	var wrapper extractProvidersResponse
	if err := json.Unmarshal(resp, &wrapper); err != nil {
		return nil, fmt.Errorf("failed to unmarshal extract providers response: %w", err)
	}
	return wrapper.Providers, nil
}

// ConfigureNamespaceExtractor sets the default extraction provider for a namespace (EXT-1).
// PATCH /v1/namespaces/{namespace}/extractor
// model is optional (empty = server default).
func (c *Client) ConfigureNamespaceExtractor(ctx context.Context, namespace string, provider string, model string) error {
	body := map[string]interface{}{
		"provider": provider,
	}
	if model != "" {
		body["model"] = model
	}
	_, err := c.request(ctx, "PATCH", fmt.Sprintf("/v1/namespaces/%s/extractor", url.PathEscape(namespace)), body)
	return err
}

// ===========================================================================
// SEC-3: AES-256-GCM Encryption Key Rotation
// ===========================================================================

// RotateEncryptionKey re-encrypts all memory content blobs with a new
// AES-256-GCM key (SEC-3). POST /v1/admin/encryption/rotate-key.
//
// After this call the new key is active in the running process. The operator
// must update DAKERA_ENCRYPTION_KEY and restart to make the rotation durable.
//
// Requires Admin scope. Pass namespace="" to rotate all namespaces.
func (c *Client) RotateEncryptionKey(ctx context.Context, newKey string, namespace string) (*RotateEncryptionKeyResponse, error) {
	req := RotateEncryptionKeyRequest{NewKey: newKey}
	if namespace != "" {
		req.Namespace = namespace
	}
	resp, err := c.request(ctx, "POST", "/v1/admin/encryption/rotate-key", req)
	if err != nil {
		return nil, err
	}
	var result RotateEncryptionKeyResponse
	if err := json.Unmarshal(resp, &result); err != nil {
		return nil, fmt.Errorf("failed to unmarshal rotate encryption key response: %w", err)
	}
	return &result, nil
}

// ===========================================================================
// ODE-2: GLiNER Entity Extraction (dakera-ode sidecar)
// ===========================================================================

// OdeExtractEntities extracts named entities from text using the GLiNER
// sidecar (ODE-2). Calls POST /ode/extract on the dakera-ode sidecar.
//
// Unlike ExtractEntities (CE-4 server-side NER), this method calls the
// dedicated GLiNER sidecar and returns character offsets, model name, and
// processing time.
//
// Requires OdeURL to be set in ClientOptions.
func (c *Client) OdeExtractEntities(ctx context.Context, req ExtractEntitiesRequest) (*ExtractEntitiesResponse, error) {
	if c.odeURL == "" {
		return nil, fmt.Errorf("OdeURL must be configured to use ExtractEntities(); " +
			"pass OdeURL: \"http://localhost:8080\" in ClientOptions")
	}
	jsonBody, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal extract entities request: %w", err)
	}
	httpReq, err := http.NewRequestWithContext(ctx, "POST", c.odeURL+"/ode/extract", bytes.NewReader(jsonBody))
	if err != nil {
		return nil, fmt.Errorf("failed to create ODE request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")
	if c.apiKey != "" {
		httpReq.Header.Set("Authorization", "Bearer "+c.apiKey)
	}
	for k, v := range c.headers {
		httpReq.Header.Set(k, v)
	}
	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		if ctx.Err() != nil {
			return nil, NewTimeoutError(fmt.Sprintf("ODE request timed out: %v", err))
		}
		return nil, NewConnectionError(fmt.Sprintf("failed to connect to ODE: %v", err))
	}
	defer resp.Body.Close()
	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read ODE response: %w", err)
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("ODE sidecar returned %d: %s", resp.StatusCode, string(respBody))
	}
	var result ExtractEntitiesResponse
	if err := json.Unmarshal(respBody, &result); err != nil {
		return nil, fmt.Errorf("failed to unmarshal extract entities response: %w", err)
	}
	return &result, nil
}
