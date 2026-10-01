package dakera

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"
)

// ErrorBody is the JSON error document every v0.12 server answer carries (and
// v0.11 servers carried for most errors). It is what DakeraError.ResponseBody
// holds. Unknown fields are ignored.
type ErrorBody struct {
	// Error is the human-readable message.
	Error string `json:"error"`
	// Code is the machine-readable code (SCREAMING_SNAKE_CASE).
	Code ErrorCode `json:"code"`
	// Details carries extra context (what was exceeded, which flag enables a feature, ...).
	Details string `json:"details"`
	// Resource names what a 404 did not find ("namespace", "vector", "memory",
	// "job", "attachment", ...). Server v0.12+.
	Resource string `json:"resource"`
	// Reason is set by a starting server's /health and /health/ready 503 bodies.
	Reason string `json:"reason"`
}

// apiResponse is a successful (2xx) answer.
type apiResponse struct {
	Body   []byte
	Header http.Header
	Status int
}

// parseRetryAfter reads a Retry-After header: delay-seconds (what the server
// sends) or an HTTP date. It returns -1 when the header is absent or invalid.
func parseRetryAfter(value string) int {
	value = strings.TrimSpace(value)
	if value == "" {
		return -1
	}
	if n, err := strconv.Atoi(value); err == nil {
		if n < 0 {
			return -1
		}
		return n
	}
	if t, err := http.ParseTime(value); err == nil {
		secs := int(time.Until(t).Seconds() + 0.5)
		if secs < 0 {
			secs = 0
		}
		return secs
	}
	return -1
}

// newAPIError maps a non-2xx answer to a typed error.
//
//	400 ValidationError          401 AuthenticationError   403 AuthorizationError
//	404 NotFoundError            409 ConflictError         413 PayloadTooLargeError
//	429 RateLimitError           501 FeatureDisabledError / NotImplementedError
//	other 5xx ServerError (503 carries RetryAfter)
func newAPIError(status int, header http.Header, respBody []byte) error {
	var body ErrorBody
	_ = json.Unmarshal(respBody, &body)
	msg := body.Error
	if msg == "" && body.Reason != "" {
		msg = "server is not ready: " + body.Reason
	}
	if msg == "" {
		msg = fmt.Sprintf("HTTP %d", status)
	}
	code := body.Code
	if code == "" {
		code = ErrorCodeUnknown
	}
	base := DakeraError{
		Message:      msg,
		StatusCode:   status,
		Code:         code,
		ResponseBody: body,
		Details:      body.Details,
		Resource:     body.Resource,
	}
	retryAfter := parseRetryAfter(header.Get("Retry-After"))

	switch {
	case status == 400:
		return &ValidationError{DakeraError: base}
	case status == 401:
		base.Message = "Authentication failed"
		return &AuthenticationError{DakeraError: base}
	case status == 403:
		return &AuthorizationError{DakeraError: base}
	case status == 404:
		return &NotFoundError{DakeraError: base}
	case status == 409:
		return &ConflictError{DakeraError: base}
	case status == 413:
		return &PayloadTooLargeError{DakeraError: base}
	case status == 429:
		base.Message = "Rate limit exceeded"
		return &RateLimitError{DakeraError: base, RetryAfter: retryAfter}
	case status == 501:
		if code == ErrorCodeFeatureDisabled {
			return &FeatureDisabledError{DakeraError: base}
		}
		return &NotImplementedError{DakeraError: base}
	case status >= 500:
		return &ServerError{DakeraError: base, RetryAfter: retryAfter}
	}
	return &base
}

// retryPlan decides whether an error answer is worth another attempt and how
// long to wait first. 429 and 503 honour Retry-After (capped at the retry
// config's MaxDelay); other 5xx answers back off exponentially; 501 (a feature
// that is off, or a backend limit) and every other 4xx are final.
func (c *Client) retryPlan(status int, header http.Header, attempt int) (bool, time.Duration) {
	retryAfter := parseRetryAfter(header.Get("Retry-After"))
	switch {
	case status == 429 || status == 503:
		if retryAfter >= 0 {
			wait := time.Duration(retryAfter) * time.Second
			if limit := c.retryConfig.MaxDelay; limit > 0 && wait > limit {
				wait = limit
			}
			return true, wait
		}
		return true, c.computeBackoff(attempt)
	case status == 501:
		return false, 0
	case status >= 500:
		return true, c.computeBackoff(attempt)
	}
	return false, 0
}

// sleepCtx waits for d or until ctx is done.
func sleepCtx(ctx context.Context, d time.Duration) error {
	if d <= 0 {
		return ctx.Err()
	}
	timer := time.NewTimer(d)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}

// send performs one logical request. With retry set it retries connection
// failures, 429, 503 (honouring Retry-After) and other 5xx answers up to
// RetryConfig.MaxRetries attempts; without it exactly one attempt is made.
func (c *Client) send(ctx context.Context, method, path, contentType string, payload []byte, retry bool) (*apiResponse, error) {
	reqURL := c.baseURL + path
	attempts := c.retryConfig.MaxRetries
	if !retry || attempts < 1 {
		attempts = 1
	}
	var lastErr error

	for attempt := 0; attempt < attempts; attempt++ {
		var reqBody io.Reader
		if payload != nil {
			reqBody = bytes.NewReader(payload)
		}

		req, err := http.NewRequestWithContext(ctx, method, reqURL, reqBody)
		if err != nil {
			return nil, fmt.Errorf("failed to create request: %w", err)
		}

		req.Header.Set("Content-Type", contentType)
		req.Header.Set("User-Agent", "dakera-go/"+Version)
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
			if attempt < attempts-1 {
				if serr := sleepCtx(ctx, c.computeBackoff(attempt)); serr != nil {
					return nil, NewTimeoutError(fmt.Sprintf("request cancelled while retrying: %v", serr))
				}
				continue
			}
			return nil, lastErr
		}

		respBody, readErr := io.ReadAll(resp.Body)
		resp.Body.Close()

		// OPS-1: capture rate-limit headers on every response
		c.rlMu.Lock()
		c.lastRateLimitHeaders = parseRateLimitHeaders(resp.Header)
		c.rlMu.Unlock()

		if readErr != nil {
			lastErr = fmt.Errorf("failed to read response body: %w", readErr)
			continue
		}

		if resp.StatusCode >= 200 && resp.StatusCode < 300 {
			return &apiResponse{Body: respBody, Header: resp.Header, Status: resp.StatusCode}, nil
		}

		failure := newAPIError(resp.StatusCode, resp.Header, respBody)
		retryable, wait := c.retryPlan(resp.StatusCode, resp.Header, attempt)
		if !retryable || attempt >= attempts-1 {
			return nil, failure
		}
		lastErr = failure
		if serr := sleepCtx(ctx, wait); serr != nil {
			return nil, NewTimeoutError(fmt.Sprintf("request cancelled while retrying: %v", serr))
		}
	}

	if lastErr != nil {
		return nil, lastErr
	}
	return nil, &DakeraError{Message: "request failed after retries"}
}

// request makes a JSON HTTP request with retry logic.
func (c *Client) request(ctx context.Context, method, path string, body interface{}) ([]byte, error) {
	var payload []byte
	if body != nil {
		encoded, err := json.Marshal(body)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal request body: %w", err)
		}
		payload = encoded
	}
	resp, err := c.send(ctx, method, path, "application/json", payload, true)
	if err != nil {
		return nil, err
	}
	return resp.Body, nil
}

// requestRaw sends an HTTP request with a raw byte body and a custom content type.
// It reuses the same retry and error-handling logic as request but skips JSON marshaling.
func (c *Client) requestRaw(ctx context.Context, method, path, contentType string, body []byte) ([]byte, error) {
	resp, err := c.send(ctx, method, path, contentType, body, true)
	if err != nil {
		return nil, err
	}
	return resp.Body, nil
}
