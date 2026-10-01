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
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
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

// Version is the dakera-go client version, sent in the User-Agent header so the
// Dakera engine can attribute Go SDK usage.
const Version = "0.12.1"

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

	// R9: per-instance capabilities cache (GET /v1/capabilities) + pre-flight switch
	capMu                   sync.Mutex
	capabilities            *ServerCapabilities
	capabilitiesUnavailable bool
	preflight               bool
}

// ===========================================================================
// Server capabilities (R9 / DAK-10004)
// ===========================================================================

// Capabilities returns what the connected server can do — GET /v1/capabilities
// (server v0.12+): the models it can load (and which one is active), index
// kinds, distance metrics, the search mode it runs, whether the R2 records
// surface is enabled and whether a re-embed is still pending. The document is
// cached on this client; use RefreshCapabilities to fetch it again. Unknown
// fields and unknown strings in the document are kept rather than rejected.
// A server that predates the endpoint returns a *NotFoundError.
func (c *Client) Capabilities(ctx context.Context) (*ServerCapabilities, error) {
	c.capMu.Lock()
	cached := c.capabilities
	c.capMu.Unlock()
	if cached != nil {
		return cached, nil
	}
	return c.RefreshCapabilities(ctx)
}

// RefreshCapabilities fetches GET /v1/capabilities again and replaces the cache.
func (c *Client) RefreshCapabilities(ctx context.Context) (*ServerCapabilities, error) {
	respBody, err := c.request(ctx, "GET", "/v1/capabilities", nil)
	if err != nil {
		return nil, err
	}
	caps, err := ParseCapabilities(respBody)
	if err != nil {
		return nil, fmt.Errorf("failed to parse capabilities: %w", err)
	}
	c.capMu.Lock()
	c.capabilities = caps
	c.capabilitiesUnavailable = false
	c.capMu.Unlock()
	return caps, nil
}

// RequireSupported returns an *UnsupportedCapabilityError unless the server
// advertises value for kind (CapabilityModel, CapabilityIndexKind,
// CapabilityDistanceMetric, CapabilitySearchMode, CapabilityQueryLanguage).
// Fetches (and caches) capabilities on first use. Search mode is process-wide
// on the server (DAKERA_SEARCH_MODE), so this is the pre-flight for tooling
// that configures it rather than for a per-request field.
func (c *Client) RequireSupported(ctx context.Context, kind CapabilityKind, value string) error {
	caps, err := c.Capabilities(ctx)
	if err != nil {
		return err
	}
	return caps.Require(kind, value)
}

// preflightCheck validates value against cached capabilities before a request.
// Uses the cache when populated; fetches only when ClientOptions.Preflight was
// set. A 404 (pre-0.12 server) disables the check for the lifetime of this client.
func (c *Client) preflightCheck(ctx context.Context, kind CapabilityKind, value string) error {
	c.capMu.Lock()
	caps := c.capabilities
	unavailable := c.capabilitiesUnavailable
	c.capMu.Unlock()
	if caps == nil {
		if !c.preflight || unavailable {
			return nil
		}
		fetched, err := c.RefreshCapabilities(ctx)
		if err != nil {
			var notFound *NotFoundError
			if errors.As(err, &notFound) {
				c.capMu.Lock()
				c.capabilitiesUnavailable = true
				c.capMu.Unlock()
				return nil
			}
			return err
		}
		caps = fetched
	}
	return caps.Require(kind, value)
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
		preflight: opts.Preflight,
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

	respBody, err := c.request(ctx, "POST", fmt.Sprintf("/v1/namespaces/%s/vectors/delete", namespace), body)
	if err != nil {
		return nil, err
	}

	var resp DeleteResponse
	if err := json.Unmarshal(respBody, &resp); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}
	return &resp, nil
}

// BulkUpdateVectors bulk updates vector metadata matching a filter.
func (c *Client) BulkUpdateVectors(ctx context.Context, namespace string, filter map[string]interface{}, update map[string]interface{}) (*BulkUpdateResponse, error) {
	body := map[string]interface{}{
		"filter": filter,
		"update": update,
	}
	respBody, err := c.request(ctx, "POST", fmt.Sprintf("/v1/namespaces/%s/vectors/bulk-update", namespace), body)
	if err != nil {
		return nil, err
	}
	var resp BulkUpdateResponse
	if err := json.Unmarshal(respBody, &resp); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}
	return &resp, nil
}

// BulkDeleteVectors bulk deletes vectors matching a filter.
func (c *Client) BulkDeleteVectors(ctx context.Context, namespace string, filter map[string]interface{}) (*BulkDeleteResponse, error) {
	body := map[string]interface{}{
		"filter": filter,
	}
	respBody, err := c.request(ctx, "POST", fmt.Sprintf("/v1/namespaces/%s/vectors/bulk-delete", namespace), body)
	if err != nil {
		return nil, err
	}
	var resp BulkDeleteResponse
	if err := json.Unmarshal(respBody, &resp); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}
	return &resp, nil
}

// CountVectors counts vectors in a namespace, optionally filtered.
func (c *Client) CountVectors(ctx context.Context, namespace string, filter map[string]interface{}) (*CountVectorsResponse, error) {
	body := make(map[string]interface{})
	if filter != nil {
		body["filter"] = filter
	}
	respBody, err := c.request(ctx, "POST", fmt.Sprintf("/v1/namespaces/%s/vectors/count", namespace), body)
	if err != nil {
		return nil, err
	}
	var resp CountVectorsResponse
	if err := json.Unmarshal(respBody, &resp); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}
	return &resp, nil
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
		if err := c.preflightCheck(ctx, CapabilityModel, string(opts.Model)); err != nil {
			return nil, err
		}
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
			if err := c.preflightCheck(ctx, CapabilityModel, string(opts.Model)); err != nil {
				return nil, err
			}
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
			if err := c.preflightCheck(ctx, CapabilityModel, string(opts.Model)); err != nil {
				return nil, err
			}
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
// When provided, results are blended with vector similarity according to opts.VectorWeight.
func (c *Client) HybridSearch(ctx context.Context, namespace string, vector []float32, query string, opts *HybridSearchOptions) ([]HybridSearchResult, error) {
	body := map[string]interface{}{
		"text": query,
	}
	if vector != nil {
		body["vector"] = vector
	}

	if opts != nil {
		if opts.TopK > 0 {
			body["top_k"] = opts.TopK
		}
		if opts.VectorWeight > 0 {
			body["vector_weight"] = opts.VectorWeight
		}
		if opts.Filter != nil {
			body["filter"] = opts.Filter
		}
	} else {
		body["top_k"] = 10
		body["vector_weight"] = 0.5
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
func (c *Client) ListNamespaces(ctx context.Context) ([]string, error) {
	respBody, err := c.request(ctx, "GET", "/v1/namespaces", nil)
	if err != nil {
		return nil, err
	}

	var resp struct {
		Namespaces []string `json:"namespaces"`
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
			body["dimension"] = opts.Dimensions
		}
		if opts.IndexType != "" {
			if err := c.preflightCheck(ctx, CapabilityIndexKind, opts.IndexType); err != nil {
				return nil, err
			}
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
	if req.Distance != "" {
		if err := c.preflightCheck(ctx, CapabilityDistanceMetric, string(req.Distance)); err != nil {
			return nil, err
		}
	}
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

// Health checks the server health — GET /health.
//
// A 503 means the server is starting (v0.12 binds its port while models load)
// or not serving: Health returns it at once as a *ServiceUnavailableError
// (Starting, Reason and RetryAfter filled in) and does not retry it. Other 5xx
// answers and connection failures are still retried with backoff. To wait for a
// starting server use WaitUntilReady.
func (c *Client) Health(ctx context.Context) (*HealthResponse, error) {
	resp, err := c.sendOpts(ctx, "GET", "/health", "application/json", nil, true, true)
	if err != nil {
		var serverErr *ServerError
		if errors.As(err, &serverErr) && serverErr.StatusCode == 503 {
			unavailable := &ServiceUnavailableError{ServerError: *serverErr}
			if body, ok := serverErr.ResponseBody.(ErrorBody); ok {
				unavailable.Reason = body.Reason
				unavailable.Starting = body.Starting || body.Status == "starting"
			}
			return nil, unavailable
		}
		return nil, err
	}

	var out HealthResponse
	if err := json.Unmarshal(resp.Body, &out); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}
	return &out, nil
}

// HealthReady checks the readiness probe, GET /health/ready — storage,
// embedding engine and dependencies. It makes a single attempt (no retries).
//
// A 200 means the server is ready. Anything else is an error: in particular a
// server that is still starting (v0.12 binds its port while models download)
// answers 503 with Retry-After, which is returned as a *ServerError (its
// RetryAfter and ResponseBody.Reason say why) — never as a healthy result. Use
// IsReady for a boolean and WaitUntilReady to block until ready.
func (c *Client) HealthReady(ctx context.Context) (*ReadinessResponse, error) {
	resp, err := c.send(ctx, "GET", "/health/ready", "application/json", nil, false)
	if err != nil {
		return nil, err
	}
	var out ReadinessResponse
	if err := json.Unmarshal(resp.Body, &out); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}
	return &out, nil
}

// HealthLive checks the liveness probe, GET /health/live — the process is
// alive (it answers while models are still loading). Single attempt.
func (c *Client) HealthLive(ctx context.Context) (*LivenessResponse, error) {
	resp, err := c.send(ctx, "GET", "/health/live", "application/json", nil, false)
	if err != nil {
		return nil, err
	}
	var out LivenessResponse
	if err := json.Unmarshal(resp.Body, &out); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}
	return &out, nil
}

// IsReady reports whether the server's /health/ready answers 200. A 503 (still
// starting, or storage / embedding unavailable) is (false, nil); connection
// failures and other errors are returned.
func (c *Client) IsReady(ctx context.Context) (bool, error) {
	resp, err := c.HealthReady(ctx)
	if err != nil {
		var serverErr *ServerError
		if errors.As(err, &serverErr) && serverErr.StatusCode == 503 {
			return false, nil
		}
		return false, err
	}
	return resp.Ready, nil
}

// ReadyWaitOptions configures WaitUntilReady.
type ReadyWaitOptions struct {
	// Timeout bounds the whole wait; zero waits until ctx is done.
	Timeout time.Duration
	// PollInterval is the delay between probes. Zero uses the server's
	// Retry-After (at most 5s) and otherwise 1s.
	PollInterval time.Duration
}

// WaitUntilReady polls GET /health/ready until the server answers 200 and
// returns that answer. It keeps polling while the server is unreachable or
// answers 503 (starting); any other error (an unexpected status) ends the wait.
// On timeout it returns a *TimeoutError naming the last error.
func (c *Client) WaitUntilReady(ctx context.Context, opts ReadyWaitOptions) (*ReadinessResponse, error) {
	if opts.Timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, opts.Timeout)
		defer cancel()
	}
	var last error
	for {
		resp, err := c.HealthReady(ctx)
		if err == nil && resp.Ready {
			return resp, nil
		}
		interval := opts.PollInterval
		if err != nil {
			if ctx.Err() != nil {
				return nil, NewTimeoutError(fmt.Sprintf("server not ready: %v (last: %v)", ctx.Err(), last))
			}
			last = err
			var serverErr *ServerError
			var connErr *ConnectionError
			switch {
			case errors.As(err, &serverErr) && serverErr.StatusCode == 503:
				if interval <= 0 && serverErr.RetryAfter > 0 {
					interval = time.Duration(serverErr.RetryAfter) * time.Second
					if interval > 5*time.Second {
						interval = 5 * time.Second
					}
				}
			case errors.As(err, &connErr):
			default:
				return nil, err
			}
		} else {
			last = fmt.Errorf("server reported ready=false")
		}
		if interval <= 0 {
			interval = time.Second
		}
		if serr := sleepCtx(ctx, interval); serr != nil {
			return nil, NewTimeoutError(fmt.Sprintf("server not ready: %v (last: %v)", serr, last))
		}
	}
}

// GetIndexStats returns how searches on a namespace are served (index type,
// whether it is built, size, indexed vectors). It reads GET /admin/indexes/stats
// (Admin scope — the server has no per-namespace stats route) and picks the
// namespace's entry; a namespace the server does not list is a *NotFoundError.
func (c *Client) GetIndexStats(ctx context.Context, namespace string) (*IndexStats, error) {
	respBody, err := c.request(ctx, "GET", "/admin/indexes/stats", nil)
	if err != nil {
		return nil, err
	}

	var wrapper struct {
		Namespaces map[string]IndexStats `json:"namespaces"`
	}
	if err := json.Unmarshal(respBody, &wrapper); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}
	stats, ok := wrapper.Namespaces[namespace]
	if !ok {
		return nil, NewNotFoundError(fmt.Sprintf("namespace '%s' has no index stats", namespace), 404, nil, ErrorCodeNamespaceNotFound)
	}
	stats.Namespace = namespace
	return &stats, nil
}

// Compact triggers compaction for a namespace — POST /ops/compact with the
// namespace (Admin scope; there is no per-namespace compact route). The
// server runs it as a job and answers with its job id and a message, which
// Status carries. A backend with no on-request compaction answers 501
// (*NotImplementedError). Use OpsCompact for the job id and force option.
func (c *Client) Compact(ctx context.Context, namespace string) (*StatusResponse, error) {
	resp, err := c.OpsCompact(ctx, CompactionRequest{Namespace: namespace})
	if err != nil {
		return nil, err
	}
	return &StatusResponse{Status: resp.Message}, nil
}

// ===========================================================================
// Memory Operations
// ===========================================================================

// StoreMemory stores a memory for an agent.
func (c *Client) StoreMemory(ctx context.Context, agentID string, req StoreMemoryRequest) (*StoreMemoryResponse, error) {
	req.AgentID = agentID
	respBody, err := c.request(ctx, "POST", "/v1/memory/store", req)
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
//
// Set req.IncludeAssociated = true to enable COG-2 associative recall —
// the response will include AssociatedMemories surfaced via KG depth-1
// traversal from the primary results.
func (c *Client) Recall(ctx context.Context, agentID string, req RecallRequest) (*RecallResponse, error) {
	req.AgentID = agentID
	respBody, err := c.request(ctx, "POST", "/v1/memory/recall", req)
	if err != nil {
		return nil, err
	}

	var result RecallResponse
	if err := json.Unmarshal(respBody, &result); err != nil {
		// Fallback: try direct array (legacy server response)
		var memories []RecalledMemory
		if err2 := json.Unmarshal(respBody, &memories); err2 != nil {
			return nil, fmt.Errorf("failed to parse recall response: %w", err)
		}
		return &RecallResponse{Memories: memories}, nil
	}
	return &result, nil
}

// GetMemory gets a specific memory.
func (c *Client) GetMemory(ctx context.Context, agentID, memoryID string) (*Memory, error) {
	respBody, err := c.request(ctx, "GET", fmt.Sprintf("/v1/memory/get/%s?agent_id=%s", memoryID, url.QueryEscape(agentID)), nil)
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
//
// PUT /v1/memory/update/{id}?agent_id=... — the server reads agent_id from the
// query string and answers with the updated memory object, which is returned
// in StoreMemoryResponse.Memory.
func (c *Client) UpdateMemory(ctx context.Context, agentID, memoryID string, req UpdateMemoryRequest) (*StoreMemoryResponse, error) {
	path := fmt.Sprintf("/v1/memory/update/%s?agent_id=%s", url.PathEscape(memoryID), url.QueryEscape(agentID))
	respBody, err := c.request(ctx, "PUT", path, req)
	if err != nil {
		return nil, err
	}
	return parseUpdatedMemory(respBody)
}

// parseUpdatedMemory reads the answer of PUT /v1/memory/update/{id}: the flat
// memory object the server sends, or a {"memory": {...}} wrapper.
func parseUpdatedMemory(respBody []byte) (*StoreMemoryResponse, error) {
	var probe map[string]json.RawMessage
	if err := json.Unmarshal(respBody, &probe); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}
	if _, wrapped := probe["memory"]; wrapped {
		var result StoreMemoryResponse
		if err := json.Unmarshal(respBody, &result); err != nil {
			return nil, fmt.Errorf("failed to parse response: %w", err)
		}
		return &result, nil
	}
	var memory Memory
	if err := json.Unmarshal(respBody, &memory); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}
	return &StoreMemoryResponse{Memory: &memory}, nil
}

// Forget deletes a memory.
func (c *Client) Forget(ctx context.Context, agentID, memoryID string) error {
	_, err := c.request(ctx, "POST", "/v1/memory/forget", map[string]interface{}{"agent_id": agentID, "memory_ids": []string{memoryID}})
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

// StoreMemoriesBatch stores multiple memories in a single request (DAK-5508).
//
// Uses POST /v1/memories/store/batch. The server embeds all contents in a
// single ONNX inference pass, yielding ≥100× throughput vs. N sequential
// single-store calls. Accepts up to 1 000 memories per call.
func (c *Client) StoreMemoriesBatch(ctx context.Context, req BatchStoreMemoryRequest) (*BatchStoreMemoryResponse, error) {
	respBody, err := c.request(ctx, "POST", "/v1/memories/store/batch", req)
	if err != nil {
		return nil, err
	}

	var result BatchStoreMemoryResponse
	if err := json.Unmarshal(respBody, &result); err != nil {
		return nil, fmt.Errorf("failed to parse batch store response: %w", err)
	}
	return &result, nil
}

// SearchMemories searches memories for an agent.
func (c *Client) SearchMemories(ctx context.Context, agentID string, req SearchMemoriesRequest) ([]RecalledMemory, error) {
	req.AgentID = agentID
	respBody, err := c.request(ctx, "POST", "/v1/memory/search", req)
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
	for _, mid := range req.MemoryIDs {
		body := map[string]interface{}{
			"agent_id":   agentID,
			"memory_id":  mid,
			"importance": req.Importance,
		}
		if _, err := c.request(ctx, "POST", "/v1/memory/importance", body); err != nil {
			return err
		}
	}
	return nil
}

// Consolidate consolidates memories for an agent.
func (c *Client) Consolidate(ctx context.Context, agentID string, req ConsolidateRequest) (*ConsolidateResponse, error) {
	req.AgentID = agentID
	respBody, err := c.request(ctx, "POST", "/v1/memory/consolidate", req)
	if err != nil {
		return nil, err
	}

	var result ConsolidateResponse
	if err := json.Unmarshal(respBody, &result); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}
	return &result, nil
}

// ConsolidateAgent consolidates memories directly for an agent (DBSCAN clustering).
func (c *Client) ConsolidateAgent(ctx context.Context, agentID string) (*AgentConsolidateResponse, error) {
	respBody, err := c.request(ctx, "POST", fmt.Sprintf("/v1/agents/%s/consolidate", agentID), nil)
	if err != nil {
		return nil, err
	}
	var resp AgentConsolidateResponse
	if err := json.Unmarshal(respBody, &resp); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}
	return &resp, nil
}

// GetConsolidationLog gets the consolidation execution log for an agent.
func (c *Client) GetConsolidationLog(ctx context.Context, agentID string) ([]AgentConsolidationLogEntry, error) {
	respBody, err := c.request(ctx, "GET", fmt.Sprintf("/v1/agents/%s/consolidation/log", agentID), nil)
	if err != nil {
		return nil, err
	}
	var entries []AgentConsolidationLogEntry
	if err := json.Unmarshal(respBody, &entries); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}
	return entries, nil
}

// PatchConsolidationConfig updates the consolidation configuration for an agent.
func (c *Client) PatchConsolidationConfig(ctx context.Context, agentID string, patch ConsolidationConfigPatch) (*AgentConsolidationConfig, error) {
	respBody, err := c.request(ctx, "PATCH", fmt.Sprintf("/v1/agents/%s/consolidation/config", agentID), patch)
	if err != nil {
		return nil, err
	}
	var resp AgentConsolidationConfig
	if err := json.Unmarshal(respBody, &resp); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}
	return &resp, nil
}

// MemoryFeedback submits feedback on a memory — POST /v1/memory/feedback with
// {agent_id, memory_id, signal}. req.Feedback is mapped to a signal:
// "relevant" / "positive" / "upvote" → upvote, "irrelevant" / "negative" /
// "downvote" → downvote, "flag" → flag; any other value is sent as-is (the
// server answers 400 for an unknown signal). RelevanceScore is not a server
// field and is ignored. Prefer FeedbackMemory, which takes a typed signal.
func (c *Client) MemoryFeedback(ctx context.Context, agentID string, req MemoryFeedbackRequest) (*MemoryFeedbackResponse, error) {
	signal := strings.ToLower(strings.TrimSpace(req.Feedback))
	switch signal {
	case "relevant":
		signal = "upvote"
	case "irrelevant":
		signal = "downvote"
	}
	body := map[string]interface{}{
		"agent_id":  agentID,
		"memory_id": req.MemoryID,
		"signal":    signal,
	}
	respBody, err := c.request(ctx, "POST", "/v1/memory/feedback", body)
	if err != nil {
		return nil, err
	}

	var wire struct {
		Status            string   `json:"status"`
		UpdatedImportance *float32 `json:"updated_importance"`
		NewImportance     *float32 `json:"new_importance"`
	}
	if err := json.Unmarshal(respBody, &wire); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}
	result := MemoryFeedbackResponse{Status: wire.Status, UpdatedImportance: wire.UpdatedImportance}
	if result.UpdatedImportance == nil {
		result.UpdatedImportance = wire.NewImportance
	}
	if result.Status == "" {
		result.Status = "updated"
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

// EvaluateTif computes a T-I-F reliability score for a memory (T-I-F RFC Phase 3).
//
// It fetches the memory's full feedback history and reduces it to a TifScore
// with Truth/Indeterminacy/Falsity proportions and a Classification label.
func (c *Client) EvaluateTif(ctx context.Context, memoryID string) (TifScore, error) {
	history, err := c.GetMemoryFeedbackHistory(ctx, memoryID)
	if err != nil {
		return TifScore{}, err
	}
	return ComputeTifScore(history), nil
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
	var wanted map[EdgeType]bool
	if opts != nil && len(opts.Types) > 0 {
		typeStrs := make([]string, len(opts.Types))
		wanted = make(map[EdgeType]bool, len(opts.Types))
		for i, t := range opts.Types {
			typeStrs[i] = string(t)
			wanted[t] = true
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
	collectGraphEdges(&result, wanted)
	return &result, nil
}

// collectGraphEdges fills MemoryGraph.Edges from the per-node edges the server
// sends (when the answer has no top-level edges) and, when wanted is non-nil,
// keeps only edges of those types (the server does not filter by type).
func collectGraphEdges(g *MemoryGraph, wanted map[EdgeType]bool) {
	keep := func(e GraphEdge) bool { return wanted == nil || wanted[e.EdgeType] }
	if len(g.Edges) == 0 {
		type edgeKey struct {
			from, to string
			kind     EdgeType
		}
		seen := make(map[edgeKey]bool)
		for _, n := range g.Nodes {
			for _, e := range n.Edges {
				k := edgeKey{e.SourceID, e.TargetID, e.EdgeType}
				if !seen[k] {
					seen[k] = true
					g.Edges = append(g.Edges, e)
				}
			}
		}
	}
	if wanted == nil {
		return
	}
	filtered := g.Edges[:0]
	for _, e := range g.Edges {
		if keep(e) {
			filtered = append(filtered, e)
		}
	}
	g.Edges = filtered
	for i := range g.Nodes {
		nodeEdges := g.Nodes[i].Edges[:0]
		for _, e := range g.Nodes[i].Edges {
			if keep(e) {
				nodeEdges = append(nodeEdges, e)
			}
		}
		g.Nodes[i].Edges = nodeEdges
	}
}

// MemoryPath finds the shortest path between two memories in the knowledge graph.
// GET /v1/memories/{sourceID}/path?to={targetID}
//
// Requires CE-5 (Memory Knowledge Graph) on the server.
func (c *Client) MemoryPath(ctx context.Context, sourceID, targetID string) (*GraphPath, error) {
	path := fmt.Sprintf("/v1/memories/%s/path?to=%s",
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

// MemoryLink creates an explicit linked_by edge from sourceID to targetID.
// POST /v1/memories/{sourceID}/links
//
// agentID is the agent that owns both memories (the server requires it and
// answers 404 when either memory is not one of the agent's). label is an
// optional human-readable label; pass "" for none.
//
// Requires CE-5 (Memory Knowledge Graph) on the server.
func (c *Client) MemoryLink(ctx context.Context, agentID, sourceID, targetID, label string) (*GraphLinkResponse, error) {
	req := GraphLinkRequest{
		TargetID: targetID,
		AgentID:  agentID,
		Label:    label,
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

// AgentGraphExport exports all graph edges of an agent's memory namespace.
// GET /v1/agents/{agentID}/graph/export
//
// The server always answers JSON ({agent_id, namespace, node_count,
// edge_count, edges}); format is ignored and kept for compatibility. For a
// GraphML export use KnowledgeExport.
//
// Requires CE-5 (Memory Knowledge Graph) on the server.
func (c *Client) AgentGraphExport(ctx context.Context, agentID, format string) (*GraphExport, error) {
	_ = format
	path := fmt.Sprintf("/v1/agents/%s/graph/export", url.PathEscape(agentID))
	respBody, err := c.request(ctx, "GET", path, nil)
	if err != nil {
		return nil, err
	}
	var result GraphExport
	if err := json.Unmarshal(respBody, &result); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}
	if result.Format == "" {
		result.Format = "json"
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

	var result SessionStartResponse
	if err := json.Unmarshal(respBody, &result); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}
	return &result.Session, nil
}

// EndSession ends a session and returns the session state and memory count.
func (c *Client) EndSession(ctx context.Context, sessionID string) (*SessionEndResponse, error) {
	respBody, err := c.request(ctx, "POST", fmt.Sprintf("/v1/sessions/%s/end", sessionID), map[string]interface{}{})
	if err != nil {
		return nil, err
	}

	var result SessionEndResponse
	if err := json.Unmarshal(respBody, &result); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}
	return &result, nil
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

	var wrapper struct {
		Sessions []Session `json:"sessions"`
	}
	if err := json.Unmarshal(respBody, &wrapper); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}
	return wrapper.Sessions, nil
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

	var wrapper struct {
		Sessions []Session `json:"sessions"`
	}
	if err := json.Unmarshal(respBody, &wrapper); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}
	return wrapper.Sessions, nil
}

// GetWakeUpContext returns top-N wake-up context memories for an agent (DAK-1690).
//
// Calls GET /v1/agents/{agent_id}/wake-up. Returns memories ranked by
// importance × exp(-ln2 × age / 14d) — no embedding inference, served from
// the metadata index for sub-millisecond latency.
//
// Requires Read scope on the agent namespace.
func (c *Client) GetWakeUpContext(ctx context.Context, agentID string, opts *WakeUpOptions) (*WakeUpResponse, error) {
	path := fmt.Sprintf("/v1/agents/%s/wake-up", agentID)
	if opts != nil {
		params := url.Values{}
		if opts.TopN != nil {
			params.Set("top_n", fmt.Sprintf("%d", *opts.TopN))
		}
		if opts.MinImportance != nil {
			params.Set("min_importance", fmt.Sprintf("%g", *opts.MinImportance))
		}
		if encoded := params.Encode(); encoded != "" {
			path += "?" + encoded
		}
	}

	respBody, err := c.request(ctx, "GET", path, nil)
	if err != nil {
		return nil, err
	}

	var result WakeUpResponse
	if err := json.Unmarshal(respBody, &result); err != nil {
		return nil, fmt.Errorf("failed to unmarshal wake-up response: %w", err)
	}
	return &result, nil
}

// CompressAgent runs a server-side compression pass on the agent's memory
// namespace (CE-12). Returns statistics about the compression operation.
func (c *Client) CompressAgent(ctx context.Context, agentID string) (*CompressResponse, error) {
	path := fmt.Sprintf("/v1/agents/%s/compress", agentID)
	respBody, err := c.request(ctx, "POST", path, nil)
	if err != nil {
		return nil, err
	}
	var result CompressResponse
	if err := json.Unmarshal(respBody, &result); err != nil {
		return nil, fmt.Errorf("failed to unmarshal compress response: %w", err)
	}
	return &result, nil
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

// DebugConfig returns all active DAKERA_* env vars (non-secret) from the running server (DAK-7477).
// Requires Admin scope. The returned map contains DAKERA_* env var names mapped to their values,
// plus "_version" and optionally "_build_sha". Secret-bearing keys (TOKEN, KEY, SECRET, PASSWORD,
// CRED, URL, URI, DSN) are filtered server-side.
//
// Used by bench harnesses to verify the server is running with the exact feature-flag configuration
// requested before scoring.
func (c *Client) DebugConfig(ctx context.Context) (map[string]string, error) {
	data, err := c.request(ctx, "GET", "/debug/config", nil)
	if err != nil {
		return nil, err
	}
	var result map[string]string
	if err := json.Unmarshal(data, &result); err != nil {
		return nil, fmt.Errorf("failed to unmarshal debug config: %w", err)
	}
	return result, nil
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

// AdminIndexStats gets index statistics across all namespaces.
func (c *Client) AdminIndexStats(ctx context.Context) (map[string]interface{}, error) {
	data, err := c.request(ctx, "GET", "/v1/admin/indexes/stats", nil)
	if err != nil {
		return nil, err
	}
	var result map[string]interface{}
	if err := json.Unmarshal(data, &result); err != nil {
		return nil, fmt.Errorf("failed to unmarshal index stats: %w", err)
	}
	return result, nil
}

// RebuildIndexes rebuilds indexes. Pass namespace to target a specific namespace, or empty string for all.
func (c *Client) RebuildIndexes(ctx context.Context, namespace string) (*StatusResponse, error) {
	var body interface{}
	if namespace != "" {
		body = map[string]string{"namespace": namespace}
	}
	data, err := c.request(ctx, "POST", "/v1/admin/indexes/rebuild", body)
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

// UpdateQuotas updates quota settings — PUT /admin/quotas/{namespace} when
// quotas names a "namespace", else PUT /admin/quotas/default. The remaining
// keys are the quota config (max_vectors, max_storage_bytes, max_dimensions,
// max_metadata_bytes, enforcement); a "config" key is passed through as the
// config. Admin scope. Prefer AdminSetQuota / AdminSetDefaultQuota, which are
// typed.
func (c *Client) UpdateQuotas(ctx context.Context, quotas map[string]interface{}) (map[string]interface{}, error) {
	path := "/v1/admin/quotas/default"
	config := map[string]interface{}{}
	for k, v := range quotas {
		if k == "namespace" {
			if ns, ok := v.(string); ok && ns != "" {
				path = fmt.Sprintf("/v1/admin/quotas/%s", url.PathEscape(ns))
				continue
			}
		}
		config[k] = v
	}
	var body map[string]interface{}
	if inner, ok := config["config"]; ok && len(config) == 1 {
		body = map[string]interface{}{"config": inner}
	} else {
		body = map[string]interface{}{"config": config}
	}
	data, err := c.request(ctx, "PUT", path, body)
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
	data, err := c.request(ctx, "POST", "/v1/admin/backups/restore", map[string]string{"backup_id": backupID})
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
// Product KPI Snapshot (OBS-2)
// ===========================================================================

// GetKpis returns a point-in-time product KPI snapshot (OBS-2).
//
// Calls GET /v1/kpis. Returns 8 operational metrics covering latency, error
// rate, and retention. Sub-millisecond — served from in-memory counters.
// Requires Admin scope.
func (c *Client) GetKpis(ctx context.Context) (*KpiSnapshot, error) {
	data, err := c.request(ctx, "GET", "/v1/kpis", nil)
	if err != nil {
		return nil, err
	}
	var result KpiSnapshot
	if err := json.Unmarshal(data, &result); err != nil {
		return nil, fmt.Errorf("failed to unmarshal kpi snapshot: %w", err)
	}
	return &result, nil
}

// ===========================================================================
// API Key Operations
// ===========================================================================

// CreateKey creates a new API key.
func (c *Client) CreateKey(ctx context.Context, req CreateKeyRequest) (*ApiKey, error) {
	data, err := c.request(ctx, "POST", "/admin/keys", req)
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
	data, err := c.request(ctx, "GET", "/admin/keys", nil)
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
	data, err := c.request(ctx, "GET", fmt.Sprintf("/admin/keys/%s", url.PathEscape(keyID)), nil)
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
	_, err := c.request(ctx, "DELETE", fmt.Sprintf("/admin/keys/%s", url.PathEscape(keyID)), nil)
	return err
}

// DeactivateKey deactivates an API key.
func (c *Client) DeactivateKey(ctx context.Context, keyID string) (*ApiKey, error) {
	data, err := c.request(ctx, "POST", fmt.Sprintf("/admin/keys/%s/deactivate", url.PathEscape(keyID)), nil)
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
	data, err := c.request(ctx, "POST", fmt.Sprintf("/admin/keys/%s/rotate", url.PathEscape(keyID)), nil)
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
	data, err := c.request(ctx, "GET", fmt.Sprintf("/admin/keys/%s/usage", url.PathEscape(keyID)), nil)
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

// GetNamespaceEntityConfig gets entity extraction configuration for a namespace.
func (c *Client) GetNamespaceEntityConfig(ctx context.Context, namespace string) (*NamespaceEntityConfig, error) {
	respBody, err := c.request(ctx, "GET", fmt.Sprintf("/v1/namespaces/%s/config", namespace), nil)
	if err != nil {
		return nil, err
	}
	var resp NamespaceEntityConfig
	if err := json.Unmarshal(respBody, &resp); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}
	return &resp, nil
}

// namespaceEntityConfigBody is the PUT body: both fields always present, so an
// empty entity_types is sent as [] and clears the list.
func namespaceEntityConfigBody(config NamespaceNerConfig) map[string]interface{} {
	types := config.EntityTypes
	if types == nil {
		types = []string{}
	}
	return map[string]interface{}{
		"extract_entities": config.ExtractEntities,
		"entity_types":     types,
	}
}

// PutNamespaceEntityConfig replaces a namespace's entity-extraction config —
// PUT /v1/namespaces/{namespace}/config, Write scope, server v0.12+ (a
// pre-0.12 server answers 405). extract_entities and entity_types are both
// always sent, so an empty config.EntityTypes clears the list. This is the
// supported way to clear entity_types; PATCH merges.
func (c *Client) PutNamespaceEntityConfig(ctx context.Context, namespace string, config NamespaceNerConfig) (*NamespaceEntityConfig, error) {
	data, err := c.request(ctx, "PUT", fmt.Sprintf("/v1/namespaces/%s/config", url.PathEscape(namespace)), namespaceEntityConfigBody(config))
	if err != nil {
		return nil, err
	}
	var resp NamespaceEntityConfig
	if err := json.Unmarshal(data, &resp); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}
	return &resp, nil
}

// PatchNamespaceEntityConfig merges a partial change into a namespace's
// entity-extraction config — PATCH /v1/namespaces/{namespace}/config, Write
// scope. Fields left nil are unchanged on server v0.12+ (a v0.11 server
// replaces the whole config). Unknown fields are refused with a 400.
func (c *Client) PatchNamespaceEntityConfig(ctx context.Context, namespace string, patch NamespaceEntityConfigPatch) (*NamespaceEntityConfig, error) {
	data, err := c.request(ctx, "PATCH", fmt.Sprintf("/v1/namespaces/%s/config", url.PathEscape(namespace)), patch)
	if err != nil {
		return nil, err
	}
	var resp NamespaceEntityConfig
	if err := json.Unmarshal(data, &resp); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}
	return &resp, nil
}

// GetNamespaceExtractor gets the extractor provider configuration for a namespace.
func (c *Client) GetNamespaceExtractor(ctx context.Context, namespace string) (*ExtractorConfigResponse, error) {
	respBody, err := c.request(ctx, "GET", fmt.Sprintf("/v1/namespaces/%s/extractor", namespace), nil)
	if err != nil {
		return nil, err
	}
	var resp ExtractorConfigResponse
	if err := json.Unmarshal(respBody, &resp); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}
	return &resp, nil
}

// ConfigureNamespaceNer configures entity extraction for a namespace (CE-4) —
// requires Write scope. config is a full configuration: an empty EntityTypes
// clears the configured list.
//
// With entity types it sends PATCH /v1/namespaces/{namespace}/config (which
// every server version reads the same way). With none it sends PUT, the full
// replacement: a v0.12 server merges PATCH bodies, so a PATCH that omits
// entity_types would leave the old list in place (and an omitempty field cannot
// say "clear"). A pre-0.12 server answers PUT with 405, in which case the call
// falls back to a PATCH with an explicit empty entity_types.
func (c *Client) ConfigureNamespaceNer(ctx context.Context, namespace string, config NamespaceNerConfig) (map[string]interface{}, error) {
	path := fmt.Sprintf("/v1/namespaces/%s/config", url.PathEscape(namespace))
	var data []byte
	var err error
	if len(config.EntityTypes) > 0 {
		data, err = c.request(ctx, "PATCH", path, config)
	} else {
		body := namespaceEntityConfigBody(config)
		data, err = c.request(ctx, "PUT", path, body)
		var apiErr *DakeraError
		if err != nil && errors.As(err, &apiErr) && apiErr.StatusCode == 405 {
			data, err = c.request(ctx, "PATCH", path, body)
		}
	}
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
// Use ExtractMemoryEntities to also set the request language (server v0.12+).
func (c *Client) ExtractEntities(ctx context.Context, text string, entityTypes []string) (*EntityExtractionResponse, error) {
	return c.ExtractMemoryEntities(ctx, ExtractMemoryEntitiesRequest{Content: text, EntityTypes: entityTypes})
}

// ExtractMemoryEntities is ExtractEntities with the full request body,
// including the per-request Lang (server v0.12+).
func (c *Client) ExtractMemoryEntities(ctx context.Context, req ExtractMemoryEntitiesRequest) (*EntityExtractionResponse, error) {
	body := map[string]interface{}{
		"content": req.Content,
	}
	if req.EntityTypes != nil {
		body["entity_types"] = req.EntityTypes
	}
	if req.Lang != "" {
		body["lang"] = req.Lang
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
	// The server answers {entities, count} without the memory id.
	if result.MemoryID == "" {
		result.MemoryID = memoryID
	}
	if result.Count == 0 {
		result.Count = len(result.Entities)
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
	if result.Total == 0 {
		result.Total = result.Count
	}
	return &result, nil
}

// ExportAudit bulk-exports audit log entries (OBS-1).
// GET /v1/audit/export (Admin scope). format is "json" (default) or "csv".
// agentID, eventType, fromTs, and toTs are optional (zero/empty = omitted).
// Data holds the response body as text (the JSON document {"events","count"},
// or the CSV); Count is the number of events.
func (c *Client) ExportAudit(ctx context.Context, format string, agentID string, eventType string, fromTs int64, toTs int64) (*AuditExportResponse, error) {
	if format == "" {
		format = "json"
	}
	params := url.Values{"format": {format}}
	if agentID != "" {
		params.Set("agent_id", agentID)
	}
	if eventType != "" {
		params.Set("event_type", eventType)
	}
	if fromTs > 0 {
		params.Set("from", strconv.FormatInt(fromTs, 10))
	}
	if toTs > 0 {
		params.Set("to", strconv.FormatInt(toTs, 10))
	}
	resp, err := c.send(ctx, "GET", "/v1/audit/export?"+params.Encode(), "application/json", nil, true)
	if err != nil {
		return nil, err
	}
	result := AuditExportResponse{Data: string(resp.Body), Format: format}
	if format == "csv" {
		if lines := strings.Count(strings.TrimRight(result.Data, "\n"), "\n"); result.Data != "" {
			result.Count = lines
		}
		return &result, nil
	}
	var doc struct {
		Count int `json:"count"`
	}
	if err := json.Unmarshal(resp.Body, &doc); err != nil {
		return nil, fmt.Errorf("failed to unmarshal audit export response: %w", err)
	}
	result.Count = doc.Count
	return &result, nil
}

// StreamAuditEvents opens a long-lived SSE connection to GET /v1/audit/stream (OBS-1) and
// returns a channel that yields *AuditEvent values as they arrive. The channel is closed
// when the context is cancelled, the connection drops, or the stream ends.
//
// agentID and eventType are optional filters (empty = no filter).
//
// Example:
//
//	events, err := client.StreamAuditEvents(ctx, "", "")
//	for event := range events {
//	    fmt.Printf("event: %s agent: %s\n", event.EventType, event.AgentID)
//	}
func (c *Client) StreamAuditEvents(ctx context.Context, agentID string, eventType string) (<-chan *AuditEvent, error) {
	params := url.Values{}
	if agentID != "" {
		params.Set("agent_id", agentID)
	}
	if eventType != "" {
		params.Set("event_type", eventType)
	}
	path := "/v1/audit/stream"
	if len(params) > 0 {
		path += "?" + params.Encode()
	}

	reqURL := c.baseURL + path
	req, err := http.NewRequestWithContext(ctx, "GET", reqURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create stream request: %w", err)
	}
	req.Header.Set("Accept", "text/event-stream")
	req.Header.Set("Cache-Control", "no-cache")
	if c.apiKey != "" {
		req.Header.Set("Authorization", "Bearer "+c.apiKey)
	}
	for k, v := range c.headers {
		req.Header.Set(k, v)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, NewConnectionError(fmt.Sprintf("failed to connect to audit stream: %v", err))
	}
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		var errBody struct {
			Error string    `json:"error"`
			Code  ErrorCode `json:"code"`
		}
		json.Unmarshal(body, &errBody)
		errMsg := errBody.Error
		if errMsg == "" {
			errMsg = fmt.Sprintf("HTTP %d", resp.StatusCode)
		}
		errorCode := errBody.Code
		if errorCode == "" {
			errorCode = ErrorCodeUnknown
		}
		switch resp.StatusCode {
		case 401:
			return nil, NewAuthenticationError("Authentication failed", resp.StatusCode, errBody, errorCode)
		case 403:
			return nil, NewAuthorizationError(errMsg, resp.StatusCode, errorCode, errBody)
		case 404:
			return nil, NewNotFoundError(errMsg, resp.StatusCode, errBody, errorCode)
		default:
			return nil, NewServerError(errMsg, resp.StatusCode, errBody, errorCode)
		}
	}

	ch := make(chan *AuditEvent, 16)
	go func() {
		defer close(ch)
		defer resp.Body.Close()
		scanner := bufio.NewScanner(resp.Body)
		for scanner.Scan() {
			line := scanner.Text()
			if !strings.HasPrefix(line, "data:") {
				continue
			}
			payload := strings.TrimSpace(strings.TrimPrefix(line, "data:"))
			if payload == "" || payload == "[DONE]" {
				continue
			}
			var event AuditEvent
			if err := json.Unmarshal([]byte(payload), &event); err != nil {
				continue
			}
			select {
			case ch <- &event:
			case <-ctx.Done():
				return
			}
		}
	}()
	return ch, nil
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

// ===========================================================================
// CE-54: Fulltext Reindex (Admin)
// ===========================================================================

// AdminFulltextReindex backfills the BM25 fulltext index for memories stored
// before CE-12 auto-indexing was added (CE-54).
//
// Sends POST /admin/fulltext/reindex. Requires Admin scope.
//
// Pass a non-empty namespace to limit reindexing to that namespace, or an
// empty string to reindex all agent namespaces. Safe to call multiple times —
// already-indexed memories are counted in TotalSkipped and not re-processed.
func (c *Client) AdminFulltextReindex(ctx context.Context, namespace string) (*FulltextReindexResponse, error) {
	var body interface{}
	if namespace != "" {
		body = map[string]string{"namespace": namespace}
	} else {
		body = map[string]string{}
	}
	data, err := c.request(ctx, "POST", "/v1/admin/fulltext/reindex", body)
	if err != nil {
		return nil, err
	}
	var result FulltextReindexResponse
	if err := json.Unmarshal(data, &result); err != nil {
		return nil, fmt.Errorf("failed to unmarshal fulltext reindex response: %w", err)
	}
	return &result, nil
}

// ===========================================================================
// COG-1: Per-namespace Memory Lifecycle Policy
// ===========================================================================

// GetMemoryPolicy returns the memory lifecycle policy for a namespace (COG-1).
//
// Sends GET /v1/namespaces/{namespace}/memory_policy.
//
// When no explicit policy has been configured the server returns the COG-1
// defaults: working=4 h, episodic=30 d, semantic=365 d, procedural=730 d;
// exponential/power_law/logarithmic/flat decay curves; SR factor 1.0.
func (c *Client) GetMemoryPolicy(ctx context.Context, namespace string) (*MemoryPolicy, error) {
	resp, err := c.request(ctx, "GET", fmt.Sprintf("/v1/namespaces/%s/memory_policy", url.PathEscape(namespace)), nil)
	if err != nil {
		return nil, err
	}
	var result MemoryPolicy
	if err := json.Unmarshal(resp, &result); err != nil {
		return nil, fmt.Errorf("failed to unmarshal memory policy response: %w", err)
	}
	return &result, nil
}

// SetMemoryPolicy sets the memory lifecycle policy for a namespace (COG-1).
//
// Sends PUT /v1/namespaces/{namespace}/memory_policy.
//
// The policy is persisted in namespace config and applied immediately to the
// decay engine background task.  Only populate the fields you want to
// override — all have safe server-side defaults.
func (c *Client) SetMemoryPolicy(ctx context.Context, namespace string, policy MemoryPolicy) (*MemoryPolicy, error) {
	resp, err := c.request(ctx, "PUT", fmt.Sprintf("/v1/namespaces/%s/memory_policy", url.PathEscape(namespace)), policy)
	if err != nil {
		return nil, err
	}
	var result MemoryPolicy
	if err := json.Unmarshal(resp, &result); err != nil {
		return nil, fmt.Errorf("failed to unmarshal memory policy response: %w", err)
	}
	return &result, nil
}

// =============================================================================
// Phase 2 — Cluster & Maintenance
// =============================================================================

// AdminClusterReplication returns cluster replication status.
func (c *Client) AdminClusterReplication(ctx context.Context) (*ReplicationStatus, error) {
	resp, err := c.request(ctx, "GET", "/v1/admin/cluster/replication", nil)
	if err != nil {
		return nil, err
	}
	var result ReplicationStatus
	if err := json.Unmarshal(resp, &result); err != nil {
		return nil, fmt.Errorf("failed to unmarshal replication status: %w", err)
	}
	return &result, nil
}

// AdminListShards returns the list of shards.
func (c *Client) AdminListShards(ctx context.Context) (*ShardListResponse, error) {
	resp, err := c.request(ctx, "GET", "/v1/admin/cluster/shards", nil)
	if err != nil {
		return nil, err
	}
	var result ShardListResponse
	if err := json.Unmarshal(resp, &result); err != nil {
		return nil, fmt.Errorf("failed to unmarshal shard list: %w", err)
	}
	return &result, nil
}

// AdminRebalanceShards triggers shard rebalancing.
func (c *Client) AdminRebalanceShards(ctx context.Context, req ShardRebalanceRequest) (*ShardRebalanceResponse, error) {
	resp, err := c.request(ctx, "POST", "/v1/admin/cluster/shards/rebalance", req)
	if err != nil {
		return nil, err
	}
	var result ShardRebalanceResponse
	if err := json.Unmarshal(resp, &result); err != nil {
		return nil, fmt.Errorf("failed to unmarshal rebalance response: %w", err)
	}
	return &result, nil
}

// AdminMaintenanceStatus returns current maintenance mode status.
func (c *Client) AdminMaintenanceStatus(ctx context.Context) (*MaintenanceStatus, error) {
	resp, err := c.request(ctx, "GET", "/v1/admin/cluster/maintenance", nil)
	if err != nil {
		return nil, err
	}
	var result MaintenanceStatus
	if err := json.Unmarshal(resp, &result); err != nil {
		return nil, fmt.Errorf("failed to unmarshal maintenance status: %w", err)
	}
	return &result, nil
}

// AdminEnableMaintenance enables maintenance mode.
func (c *Client) AdminEnableMaintenance(ctx context.Context, req EnableMaintenanceRequest) (*MaintenanceStatus, error) {
	resp, err := c.request(ctx, "POST", "/v1/admin/cluster/maintenance/enable", req)
	if err != nil {
		return nil, err
	}
	var result MaintenanceStatus
	if err := json.Unmarshal(resp, &result); err != nil {
		return nil, fmt.Errorf("failed to unmarshal maintenance status: %w", err)
	}
	return &result, nil
}

// AdminDisableMaintenance disables maintenance mode.
func (c *Client) AdminDisableMaintenance(ctx context.Context, req DisableMaintenanceRequest) (*MaintenanceStatus, error) {
	resp, err := c.request(ctx, "POST", "/v1/admin/cluster/maintenance/disable", req)
	if err != nil {
		return nil, err
	}
	var result MaintenanceStatus
	if err := json.Unmarshal(resp, &result); err != nil {
		return nil, fmt.Errorf("failed to unmarshal maintenance status: %w", err)
	}
	return &result, nil
}

// =============================================================================
// Phase 2 — Quotas
// =============================================================================

// AdminListQuotas lists all namespace quotas.
func (c *Client) AdminListQuotas(ctx context.Context) (*QuotaListResponse, error) {
	resp, err := c.request(ctx, "GET", "/v1/admin/quotas", nil)
	if err != nil {
		return nil, err
	}
	var result QuotaListResponse
	if err := json.Unmarshal(resp, &result); err != nil {
		return nil, fmt.Errorf("failed to unmarshal quota list: %w", err)
	}
	return &result, nil
}

// AdminGetDefaultQuota returns the default quota configuration.
func (c *Client) AdminGetDefaultQuota(ctx context.Context) (*DefaultQuotaResponse, error) {
	resp, err := c.request(ctx, "GET", "/v1/admin/quotas/default", nil)
	if err != nil {
		return nil, err
	}
	var result DefaultQuotaResponse
	if err := json.Unmarshal(resp, &result); err != nil {
		return nil, fmt.Errorf("failed to unmarshal default quota: %w", err)
	}
	return &result, nil
}

// AdminSetDefaultQuota sets the default quota configuration.
func (c *Client) AdminSetDefaultQuota(ctx context.Context, req SetDefaultQuotaRequest) (*SetQuotaResponse, error) {
	resp, err := c.request(ctx, "PUT", "/v1/admin/quotas/default", req)
	if err != nil {
		return nil, err
	}
	var result SetQuotaResponse
	if err := json.Unmarshal(resp, &result); err != nil {
		return nil, fmt.Errorf("failed to unmarshal set quota response: %w", err)
	}
	return &result, nil
}

// AdminGetQuota returns the quota for a specific namespace.
func (c *Client) AdminGetQuota(ctx context.Context, namespace string) (*QuotaStatus, error) {
	resp, err := c.request(ctx, "GET", fmt.Sprintf("/v1/admin/quotas/%s", url.PathEscape(namespace)), nil)
	if err != nil {
		return nil, err
	}
	var result QuotaStatus
	if err := json.Unmarshal(resp, &result); err != nil {
		return nil, fmt.Errorf("failed to unmarshal quota status: %w", err)
	}
	return &result, nil
}

// AdminSetQuota sets the quota for a specific namespace.
func (c *Client) AdminSetQuota(ctx context.Context, namespace string, req SetQuotaRequest) (*SetQuotaResponse, error) {
	resp, err := c.request(ctx, "PUT", fmt.Sprintf("/v1/admin/quotas/%s", url.PathEscape(namespace)), req)
	if err != nil {
		return nil, err
	}
	var result SetQuotaResponse
	if err := json.Unmarshal(resp, &result); err != nil {
		return nil, fmt.Errorf("failed to unmarshal set quota response: %w", err)
	}
	return &result, nil
}

// AdminDeleteQuota removes the quota for a specific namespace.
func (c *Client) AdminDeleteQuota(ctx context.Context, namespace string) (map[string]interface{}, error) {
	resp, err := c.request(ctx, "DELETE", fmt.Sprintf("/v1/admin/quotas/%s", url.PathEscape(namespace)), nil)
	if err != nil {
		return nil, err
	}
	var result map[string]interface{}
	if err := json.Unmarshal(resp, &result); err != nil {
		return nil, fmt.Errorf("failed to unmarshal delete quota response: %w", err)
	}
	return result, nil
}

// AdminCheckQuota checks if an operation would exceed quota for a namespace.
func (c *Client) AdminCheckQuota(ctx context.Context, namespace string, req QuotaCheckRequest) (*QuotaCheckResult, error) {
	resp, err := c.request(ctx, "POST", fmt.Sprintf("/v1/admin/quotas/%s/check", url.PathEscape(namespace)), req)
	if err != nil {
		return nil, err
	}
	var result QuotaCheckResult
	if err := json.Unmarshal(resp, &result); err != nil {
		return nil, fmt.Errorf("failed to unmarshal quota check result: %w", err)
	}
	return &result, nil
}

// =============================================================================
// Phase 2 — Slow Queries
// =============================================================================

// AdminListSlowQueries lists recent slow queries.
func (c *Client) AdminListSlowQueries(ctx context.Context, namespace, queryType string, limit int) ([]map[string]interface{}, error) {
	path := "/v1/admin/slow-queries"
	params := url.Values{}
	if namespace != "" {
		params.Set("namespace", namespace)
	}
	if queryType != "" {
		params.Set("query_type", queryType)
	}
	if limit > 0 {
		params.Set("limit", fmt.Sprintf("%d", limit))
	}
	if len(params) > 0 {
		path += "?" + params.Encode()
	}
	resp, err := c.request(ctx, "GET", path, nil)
	if err != nil {
		return nil, err
	}
	var result []map[string]interface{}
	if err := json.Unmarshal(resp, &result); err != nil {
		return nil, fmt.Errorf("failed to unmarshal slow queries: %w", err)
	}
	return result, nil
}

// AdminSlowQuerySummary returns the slow query summary.
func (c *Client) AdminSlowQuerySummary(ctx context.Context) (map[string]interface{}, error) {
	resp, err := c.request(ctx, "GET", "/v1/admin/slow-queries/summary", nil)
	if err != nil {
		return nil, err
	}
	var result map[string]interface{}
	if err := json.Unmarshal(resp, &result); err != nil {
		return nil, fmt.Errorf("failed to unmarshal slow query summary: %w", err)
	}
	return result, nil
}

// AdminClearSlowQueries clears the slow query log.
func (c *Client) AdminClearSlowQueries(ctx context.Context, namespace string) (map[string]interface{}, error) {
	path := "/v1/admin/slow-queries"
	if namespace != "" {
		path += "?namespace=" + url.QueryEscape(namespace)
	}
	resp, err := c.request(ctx, "DELETE", path, nil)
	if err != nil {
		return nil, err
	}
	var result map[string]interface{}
	if err := json.Unmarshal(resp, &result); err != nil {
		return nil, fmt.Errorf("failed to unmarshal clear slow queries response: %w", err)
	}
	return result, nil
}

// AdminUpdateSlowQueryConfig updates the slow query configuration.
func (c *Client) AdminUpdateSlowQueryConfig(ctx context.Context, config map[string]interface{}) (map[string]interface{}, error) {
	resp, err := c.request(ctx, "PATCH", "/v1/admin/slow-queries/config", config)
	if err != nil {
		return nil, err
	}
	var result map[string]interface{}
	if err := json.Unmarshal(resp, &result); err != nil {
		return nil, fmt.Errorf("failed to unmarshal slow query config response: %w", err)
	}
	return result, nil
}

// =============================================================================
// Phase 2 — Backups
// =============================================================================

// AdminListBackups lists all backups.
func (c *Client) AdminListBackups(ctx context.Context) (*BackupListResponse, error) {
	resp, err := c.request(ctx, "GET", "/v1/admin/backups", nil)
	if err != nil {
		return nil, err
	}
	var result BackupListResponse
	if err := json.Unmarshal(resp, &result); err != nil {
		return nil, fmt.Errorf("failed to unmarshal backup list: %w", err)
	}
	return &result, nil
}

// AdminCreateBackup creates a new backup.
func (c *Client) AdminCreateBackup(ctx context.Context, req CreateBackupRequest) (*CreateBackupResponse, error) {
	resp, err := c.request(ctx, "POST", "/v1/admin/backups", req)
	if err != nil {
		return nil, err
	}
	var result CreateBackupResponse
	if err := json.Unmarshal(resp, &result); err != nil {
		return nil, fmt.Errorf("failed to unmarshal create backup response: %w", err)
	}
	return &result, nil
}

// AdminGetBackup returns backup details by ID.
func (c *Client) AdminGetBackup(ctx context.Context, backupID string) (*AdminBackupInfo, error) {
	resp, err := c.request(ctx, "GET", fmt.Sprintf("/v1/admin/backups/%s", url.PathEscape(backupID)), nil)
	if err != nil {
		return nil, err
	}
	var result AdminBackupInfo
	if err := json.Unmarshal(resp, &result); err != nil {
		return nil, fmt.Errorf("failed to unmarshal backup info: %w", err)
	}
	return &result, nil
}

// AdminDeleteBackup deletes a backup by ID.
func (c *Client) AdminDeleteBackup(ctx context.Context, backupID string) (map[string]interface{}, error) {
	resp, err := c.request(ctx, "DELETE", fmt.Sprintf("/v1/admin/backups/%s", url.PathEscape(backupID)), nil)
	if err != nil {
		return nil, err
	}
	var result map[string]interface{}
	if err := json.Unmarshal(resp, &result); err != nil {
		return nil, fmt.Errorf("failed to unmarshal delete backup response: %w", err)
	}
	return result, nil
}

// AdminGetBackupSchedule returns the backup schedule configuration.
func (c *Client) AdminGetBackupSchedule(ctx context.Context) (*BackupSchedule, error) {
	resp, err := c.request(ctx, "GET", "/v1/admin/backups/schedule", nil)
	if err != nil {
		return nil, err
	}
	var result BackupSchedule
	if err := json.Unmarshal(resp, &result); err != nil {
		return nil, fmt.Errorf("failed to unmarshal backup schedule: %w", err)
	}
	return &result, nil
}

// AdminUpdateBackupSchedule updates the backup schedule.
func (c *Client) AdminUpdateBackupSchedule(ctx context.Context, req UpdateBackupScheduleRequest) (*BackupSchedule, error) {
	resp, err := c.request(ctx, "POST", "/v1/admin/backups/schedule", req)
	if err != nil {
		return nil, err
	}
	var result BackupSchedule
	if err := json.Unmarshal(resp, &result); err != nil {
		return nil, fmt.Errorf("failed to unmarshal backup schedule: %w", err)
	}
	return &result, nil
}

// AdminRestoreBackup initiates a backup restore.
func (c *Client) AdminRestoreBackup(ctx context.Context, req RestoreBackupRequest) (*RestoreBackupResponse, error) {
	resp, err := c.request(ctx, "POST", "/v1/admin/backups/restore", req)
	if err != nil {
		return nil, err
	}
	var result RestoreBackupResponse
	if err := json.Unmarshal(resp, &result); err != nil {
		return nil, fmt.Errorf("failed to unmarshal restore response: %w", err)
	}
	return &result, nil
}

// AdminGetRestoreStatus returns the status of a restore operation.
func (c *Client) AdminGetRestoreStatus(ctx context.Context, restoreID string) (*RestoreBackupResponse, error) {
	resp, err := c.request(ctx, "GET", fmt.Sprintf("/v1/admin/backups/restore/%s", url.PathEscape(restoreID)), nil)
	if err != nil {
		return nil, err
	}
	var result RestoreBackupResponse
	if err := json.Unmarshal(resp, &result); err != nil {
		return nil, fmt.Errorf("failed to unmarshal restore status: %w", err)
	}
	return &result, nil
}

// =============================================================================
// Phase 2 — Ops: Diagnostics & Jobs
// =============================================================================

// OpsDiagnostics returns system diagnostics.
func (c *Client) OpsDiagnostics(ctx context.Context) (map[string]interface{}, error) {
	resp, err := c.request(ctx, "GET", "/ops/diagnostics", nil)
	if err != nil {
		return nil, err
	}
	var result map[string]interface{}
	if err := json.Unmarshal(resp, &result); err != nil {
		return nil, fmt.Errorf("failed to unmarshal diagnostics: %w", err)
	}
	return result, nil
}

// OpsListJobs lists background jobs.
func (c *Client) OpsListJobs(ctx context.Context) ([]JobInfo, error) {
	resp, err := c.request(ctx, "GET", "/ops/jobs", nil)
	if err != nil {
		return nil, err
	}
	var result []JobInfo
	if err := json.Unmarshal(resp, &result); err != nil {
		return nil, fmt.Errorf("failed to unmarshal jobs list: %w", err)
	}
	return result, nil
}

// OpsGetJob returns the status of a specific background job.
func (c *Client) OpsGetJob(ctx context.Context, jobID string) (*JobInfo, error) {
	resp, err := c.request(ctx, "GET", fmt.Sprintf("/ops/jobs/%s", url.PathEscape(jobID)), nil)
	if err != nil {
		return nil, err
	}
	var result JobInfo
	if err := json.Unmarshal(resp, &result); err != nil {
		return nil, fmt.Errorf("failed to unmarshal job info: %w", err)
	}
	return &result, nil
}

// OpsCompact triggers a compaction pass.
func (c *Client) OpsCompact(ctx context.Context, req CompactionRequest) (*CompactionResponse, error) {
	resp, err := c.request(ctx, "POST", "/ops/compact", req)
	if err != nil {
		return nil, err
	}
	var result CompactionResponse
	if err := json.Unmarshal(resp, &result); err != nil {
		return nil, fmt.Errorf("failed to unmarshal compaction response: %w", err)
	}
	return &result, nil
}

// OpsShutdown requests a graceful server shutdown.
func (c *Client) OpsShutdown(ctx context.Context) (map[string]interface{}, error) {
	resp, err := c.request(ctx, "POST", "/ops/shutdown", nil)
	if err != nil {
		return nil, err
	}
	var result map[string]interface{}
	if err := json.Unmarshal(resp, &result); err != nil {
		return nil, fmt.Errorf("failed to unmarshal shutdown response: %w", err)
	}
	return result, nil
}

// ===========================================================================
// Fulltext Operations
// ===========================================================================

// FulltextStats returns full-text index statistics for the given namespace.
func (c *Client) FulltextStats(ctx context.Context, namespace string) (*FullTextIndexStats, error) {
	resp, err := c.request(ctx, "GET", fmt.Sprintf("/v1/namespaces/%s/fulltext/stats", url.PathEscape(namespace)), nil)
	if err != nil {
		return nil, err
	}
	var result FullTextIndexStats
	if err := json.Unmarshal(resp, &result); err != nil {
		return nil, fmt.Errorf("failed to unmarshal fulltext stats: %w", err)
	}
	return &result, nil
}

// FulltextDelete deletes full-text entries by their IDs within the given namespace.
func (c *Client) FulltextDelete(ctx context.Context, namespace string, ids []string) (*FulltextDeleteResponse, error) {
	body := FulltextDeleteRequest{IDs: ids}
	resp, err := c.request(ctx, "POST", fmt.Sprintf("/v1/namespaces/%s/fulltext/delete", url.PathEscape(namespace)), body)
	if err != nil {
		return nil, err
	}
	var result FulltextDeleteResponse
	if err := json.Unmarshal(resp, &result); err != nil {
		return nil, fmt.Errorf("failed to unmarshal fulltext delete response: %w", err)
	}
	return &result, nil
}

// ===========================================================================
// Admin Operations
// ===========================================================================

// AdminTtlStats returns TTL statistics across all namespaces.
func (c *Client) AdminTtlStats(ctx context.Context) (*TtlStatsResponse, error) {
	resp, err := c.request(ctx, "GET", "/v1/admin/ttl/stats", nil)
	if err != nil {
		return nil, err
	}
	var result TtlStatsResponse
	if err := json.Unmarshal(resp, &result); err != nil {
		return nil, fmt.Errorf("failed to unmarshal ttl stats: %w", err)
	}
	return &result, nil
}

// AdminTtlCleanup removes expired vectors. Optionally scoped to namespace (empty = global).
func (c *Client) AdminTtlCleanup(ctx context.Context, namespace string) (*TtlCleanupResponse, error) {
	var body interface{}
	if namespace != "" {
		body = TtlCleanupRequest{Namespace: namespace}
	}
	resp, err := c.request(ctx, "POST", "/v1/admin/ttl/cleanup", body)
	if err != nil {
		return nil, err
	}
	var result TtlCleanupResponse
	if err := json.Unmarshal(resp, &result); err != nil {
		return nil, fmt.Errorf("failed to unmarshal ttl cleanup response: %w", err)
	}
	return &result, nil
}

// ===========================================================================
// Routing Operations
// ===========================================================================

// RouteQuery routes a query to the most relevant namespaces.
func (c *Client) RouteQuery(ctx context.Context, req RouteRequest) (*RouteResponse, error) {
	resp, err := c.request(ctx, "POST", "/v1/route", req)
	if err != nil {
		return nil, err
	}
	var result RouteResponse
	if err := json.Unmarshal(resp, &result); err != nil {
		return nil, fmt.Errorf("failed to unmarshal route response: %w", err)
	}
	return &result, nil
}

// ===========================================================================
// Import Operations
// ===========================================================================

// ImportJobStatus returns the status of an import job.
func (c *Client) ImportJobStatus(ctx context.Context, jobID string) (*ImportJobStatus, error) {
	resp, err := c.request(ctx, "GET", fmt.Sprintf("/v1/import/%s/status", url.PathEscape(jobID)), nil)
	if err != nil {
		return nil, err
	}
	var result ImportJobStatus
	if err := json.Unmarshal(resp, &result); err != nil {
		return nil, fmt.Errorf("failed to unmarshal import job status: %w", err)
	}
	return &result, nil
}

// ===========================================================================
// Backup Operations
// ===========================================================================

// AdminDownloadBackup downloads a backup archive as raw bytes.
func (c *Client) AdminDownloadBackup(ctx context.Context, backupID string) ([]byte, error) {
	resp, err := c.request(ctx, "GET", fmt.Sprintf("/v1/admin/backups/%s/download", url.PathEscape(backupID)), nil)
	if err != nil {
		return nil, err
	}
	return resp, nil
}

// AdminUploadBackup uploads a raw backup archive and returns the server response.
func (c *Client) AdminUploadBackup(ctx context.Context, data []byte) (map[string]interface{}, error) {
	resp, err := c.requestRaw(ctx, "POST", "/v1/admin/backups/upload", "application/octet-stream", data)
	if err != nil {
		return nil, err
	}
	var result map[string]interface{}
	if err := json.Unmarshal(resp, &result); err != nil {
		return nil, fmt.Errorf("failed to unmarshal upload backup response: %w", err)
	}
	return result, nil
}

// ===========================================================================
// Storage Tier Operations
// ===========================================================================

// AdminStorageTierOverview returns an overview of the storage tier architecture and activity.
func (c *Client) AdminStorageTierOverview(ctx context.Context) (*StorageTierOverview, error) {
	resp, err := c.request(ctx, "GET", "/v1/admin/storage/tiers", nil)
	if err != nil {
		return nil, err
	}
	var result StorageTierOverview
	if err := json.Unmarshal(resp, &result); err != nil {
		return nil, fmt.Errorf("failed to unmarshal storage tier overview: %w", err)
	}
	return &result, nil
}

// AdminBackgroundActivity returns the current background activity as dynamic JSON.
func (c *Client) AdminBackgroundActivity(ctx context.Context) (map[string]interface{}, error) {
	resp, err := c.request(ctx, "GET", "/v1/admin/background-activity", nil)
	if err != nil {
		return nil, err
	}
	var result map[string]interface{}
	if err := json.Unmarshal(resp, &result); err != nil {
		return nil, fmt.Errorf("failed to unmarshal background activity: %w", err)
	}
	return result, nil
}

// AdminMemoryTypeStats returns memory type distribution statistics.
func (c *Client) AdminMemoryTypeStats(ctx context.Context) (*MemoryTypeStatsResponse, error) {
	resp, err := c.request(ctx, "GET", "/v1/admin/memory-type-stats", nil)
	if err != nil {
		return nil, err
	}
	var result MemoryTypeStatsResponse
	if err := json.Unmarshal(resp, &result); err != nil {
		return nil, fmt.Errorf("failed to unmarshal memory type stats: %w", err)
	}
	return &result, nil
}

// AdminMigrateNamespaceDimensions migrates namespace vector dimensions to a target dimension.
func (c *Client) AdminMigrateNamespaceDimensions(ctx context.Context, req MigrateNamespaceDimensionsRequest) (*MigrateDimensionsResponse, error) {
	resp, err := c.request(ctx, "POST", "/v1/admin/namespaces/migrate-dimensions", req)
	if err != nil {
		return nil, err
	}
	var result MigrateDimensionsResponse
	if err := json.Unmarshal(resp, &result); err != nil {
		return nil, fmt.Errorf("failed to unmarshal migrate dimensions response: %w", err)
	}
	return &result, nil
}

// AdminDrainReembed synchronously drains all static vectors to full ONNX quality via
// POST /admin/reembed/drain (v0.11.82+).
//
// Runs the re-embedding upgrade loop until zero _embedding_kind=static candidates remain
// across all namespaces, or req.TimeoutSecs elapses. Requires Admin scope. Useful as a
// pre-benchmark steady-state gate when DAKERA_TIERED=1.
//
// A DrainReembedResponse.Remaining of 0 guarantees all vectors are at full ONNX quality.
func (c *Client) AdminDrainReembed(ctx context.Context, req DrainReembedRequest) (*DrainReembedResponse, error) {
	resp, err := c.request(ctx, "POST", "/v1/admin/reembed/drain", req)
	if err != nil {
		return nil, err
	}
	var result DrainReembedResponse
	if err := json.Unmarshal(resp, &result); err != nil {
		return nil, fmt.Errorf("failed to unmarshal drain reembed response: %w", err)
	}
	return &result, nil
}

// AdminReembedStaticCount returns the count of static vectors pending re-embedding via
// GET /admin/reembed/static-count (v0.11.91+).
//
// Operators can poll this alongside AdminDrainReembed to monitor drain progress.
// A StaticCountResponse.StaticCount of 0 means steady state — all vectors are
// at full ONNX quality. Requires Admin scope.
func (c *Client) AdminReembedStaticCount(ctx context.Context) (*StaticCountResponse, error) {
	resp, err := c.request(ctx, "GET", "/v1/admin/reembed/static-count", nil)
	if err != nil {
		return nil, err
	}
	var result StaticCountResponse
	if err := json.Unmarshal(resp, &result); err != nil {
		return nil, fmt.Errorf("failed to unmarshal static count response: %w", err)
	}
	return &result, nil
}
