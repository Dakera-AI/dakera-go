package dakera

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
)

// EventResult wraps a DakeraEvent or an error from the SSE stream.
type EventResult struct {
	Event *DakeraEvent
	Err   error
}

// StreamNamespaceEvents subscribes to namespace-scoped SSE events.
//
// It opens a long-lived connection to GET /v1/namespaces/{namespace}/events
// and sends [EventResult] values to the returned channel.  The channel is
// closed when the server closes the stream or ctx is cancelled.
//
// Requires a Read-scoped API key.
//
// Example:
//
//	ch, err := client.StreamNamespaceEvents(ctx, "my-ns")
//	if err != nil {
//	    log.Fatal(err)
//	}
//	for result := range ch {
//	    if result.Err != nil {
//	        log.Println("stream error:", result.Err)
//	        break
//	    }
//	    fmt.Printf("event: %s\n", result.Event.Type)
//	}
func (c *Client) StreamNamespaceEvents(ctx context.Context, namespace string) (<-chan EventResult, error) {
	path := fmt.Sprintf("/v1/namespaces/%s/events", url.PathEscape(namespace))
	return c.streamSSE(ctx, path)
}

// StreamGlobalEvents subscribes to the global SSE event stream (all namespaces).
//
// It opens a long-lived connection to GET /ops/events and sends [EventResult]
// values to the returned channel.
//
// Requires an Admin-scoped API key.
func (c *Client) StreamGlobalEvents(ctx context.Context) (<-chan EventResult, error) {
	return c.streamSSE(ctx, "/ops/events")
}

// streamSSE opens an SSE connection and pumps events into a channel.
func (c *Client) streamSSE(ctx context.Context, path string) (<-chan EventResult, error) {
	rawURL := c.baseURL + path

	req, err := http.NewRequestWithContext(ctx, "GET", rawURL, nil)
	if err != nil {
		return nil, fmt.Errorf("dakera: failed to create SSE request: %w", err)
	}
	req.Header.Set("Accept", "text/event-stream")
	req.Header.Set("Cache-Control", "no-cache")
	if c.apiKey != "" {
		req.Header.Set("Authorization", "Bearer "+c.apiKey)
	}
	for k, v := range c.headers {
		req.Header.Set(k, v)
	}

	// Use a client without read timeout so the connection stays open.
	sseClient := &http.Client{}
	resp, err := sseClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("dakera: SSE connection failed: %w", err)
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		resp.Body.Close()
		return nil, fmt.Errorf("dakera: SSE connection returned HTTP %d", resp.StatusCode)
	}

	ch := make(chan EventResult, 64)

	go func() {
		defer close(ch)
		defer resp.Body.Close()

		scanner := bufio.NewScanner(resp.Body)
		var dataLines []string

		for scanner.Scan() {
			// Check for context cancellation.
			select {
			case <-ctx.Done():
				return
			default:
			}

			line := scanner.Text()

			if strings.HasPrefix(line, ":") {
				continue // SSE comment / heartbeat
			}

			if strings.HasPrefix(line, "data:") {
				data := strings.TrimPrefix(line, "data:")
				dataLines = append(dataLines, strings.TrimPrefix(data, " "))
				continue
			}

			if line == "" {
				// Event boundary — dispatch accumulated data lines.
				if len(dataLines) > 0 {
					payload := strings.Join(dataLines, "\n")
					dataLines = dataLines[:0]

					var event DakeraEvent
					if jsonErr := json.Unmarshal([]byte(payload), &event); jsonErr != nil {
						select {
						case ch <- EventResult{Err: fmt.Errorf("dakera: SSE parse error: %w", jsonErr)}:
						case <-ctx.Done():
							return
						}
						continue
					}
					select {
					case ch <- EventResult{Event: &event}:
					case <-ctx.Done():
						return
					}
				}
			}
		}

		if err := scanner.Err(); err != nil {
			select {
			case ch <- EventResult{Err: fmt.Errorf("dakera: SSE read error: %w", err)}:
			case <-ctx.Done():
			}
		}
	}()

	return ch, nil
}
