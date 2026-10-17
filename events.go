package dakera

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"strings"
	"time"
)

// EventResult wraps a DakeraEvent or an error from the SSE stream.
type EventResult struct {
	Event *DakeraEvent
	Err   error
}

// MemoryEvent is a memory lifecycle event from GET /v1/events/stream.
//
// EventType values:
//   - "connected"           — emitted on subscribe to confirm the stream is live; AgentID will be ""
//   - "stored"              — memory stored (Content, Importance, Tags present)
//   - "recalled"            — memory recalled
//   - "forgotten"           — memory deleted
//   - "consolidated"        — memories merged
//   - "importance_updated"  — importance changed
//   - "session_started" / "session_ended" — agent session lifecycle
//   - "stream_lagged"       — consumer fell behind; some events were dropped
type MemoryEvent struct {
	EventType  string   `json:"event_type"`
	AgentID    string   `json:"agent_id"`
	Timestamp  int64    `json:"timestamp"`
	MemoryID   *string  `json:"memory_id,omitempty"`
	Content    *string  `json:"content,omitempty"`
	Importance *float32 `json:"importance,omitempty"`
	Tags       []string `json:"tags,omitempty"`
	SessionID  *string  `json:"session_id,omitempty"`
	// Reason is why a session_ended event's session ended: SessionEndedByClient
	// or SessionEndedIdle (server v0.12.2+). nil on every other event type.
	Reason *string `json:"reason,omitempty"`
}

// MemoryEventResult wraps a MemoryEvent or an error from the memory event SSE stream.
type MemoryEventResult struct {
	Event *MemoryEvent
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

// StreamMemoryEvents subscribes to memory lifecycle SSE events.
//
// It opens a long-lived connection to GET /v1/events/stream and sends
// [MemoryEventResult] values to the returned channel.  The channel is closed
// when the server closes the stream or ctx is cancelled.
//
// Requires a Read-scoped API key.
func (c *Client) StreamMemoryEvents(ctx context.Context) (<-chan MemoryEventResult, error) {
	return c.streamMemorySSE(ctx, "/v1/events/stream")
}

// SubscribeAgentMemories subscribes to real-time memory lifecycle events for a
// specific agent.
//
// It wraps [Client.StreamMemoryEvents], filtering events by agentID and an
// optional tags list.  When tags is non-empty, only events whose tags have at
// least one element in common with the filter are forwarded.  The connection
// is re-established automatically on error until ctx is cancelled, except
// after a 401, 403 or 404: that error is sent on the channel and it is closed.
//
// The returned channel is closed when ctx is cancelled or the parent context
// is done.
//
// Requires a Read-scoped API key.
//
// Example:
//
//	ch, err := client.SubscribeAgentMemories(ctx, "my-bot", []string{"important"})
//	if err != nil {
//	    log.Fatal(err)
//	}
//	for result := range ch {
//	    if result.Err != nil {
//	        log.Println("stream error:", result.Err)
//	        continue
//	    }
//	    fmt.Printf("%s: %v\n", result.Event.EventType, result.Event.MemoryID)
//	}
func (c *Client) SubscribeAgentMemories(ctx context.Context, agentID string, tags []string) (<-chan MemoryEventResult, error) {
	ch := make(chan MemoryEventResult, 64)

	go func() {
		defer close(ch)
		for {
			select {
			case <-ctx.Done():
				return
			default:
			}

			inner, err := c.StreamMemoryEvents(ctx)
			if err != nil {
				if permanentStreamError(err) {
					// A bad key, a missing scope or a missing route will not
					// fix itself: report it and stop instead of reconnecting
					// forever in silence.
					select {
					case ch <- MemoryEventResult{Err: err}:
					case <-ctx.Done():
					}
					return
				}
				select {
				case <-ctx.Done():
					return
				case <-time.After(time.Second):
					continue
				}
			}

			for result := range inner {
				if result.Err != nil {
					select {
					case ch <- result:
					case <-ctx.Done():
						return
					}
					break // Reconnect on error.
				}
				event := result.Event
				if event.EventType == "connected" {
					continue
				}
				if event.AgentID != agentID {
					continue
				}
				if len(tags) > 0 && !hasTagOverlap(event.Tags, tags) {
					continue
				}
				select {
				case ch <- MemoryEventResult{Event: event}:
				case <-ctx.Done():
					return
				}
			}

			// Stream ended cleanly — reconnect after a short delay.
			select {
			case <-ctx.Done():
				return
			case <-time.After(time.Second):
			}
		}
	}()

	return ch, nil
}

// permanentStreamError reports whether reconnecting cannot help: 401, 403,
// 404 (*AuthenticationError, *AuthorizationError, *NotFoundError).
func permanentStreamError(err error) bool {
	var authn *AuthenticationError
	var authz *AuthorizationError
	var notFound *NotFoundError
	return errors.As(err, &authn) || errors.As(err, &authz) || errors.As(err, &notFound)
}

// hasTagOverlap reports whether slice a and slice b share at least one element.
func hasTagOverlap(a, b []string) bool {
	set := make(map[string]struct{}, len(a))
	for _, t := range a {
		set[t] = struct{}{}
	}
	for _, t := range b {
		if _, ok := set[t]; ok {
			return true
		}
	}
	return false
}

// streamMemorySSE opens an SSE connection and pumps MemoryEvent values into a channel.
func (c *Client) streamMemorySSE(ctx context.Context, path string) (<-chan MemoryEventResult, error) {
	resp, err := c.openStream(ctx, path)
	if err != nil {
		return nil, err
	}

	ch := make(chan MemoryEventResult, 64)

	go func() {
		defer close(ch)
		defer resp.Body.Close()

		scanner := newSSEScanner(resp.Body)
		var dataLines []string
		var sseEventType string

		for scanner.Scan() {
			select {
			case <-ctx.Done():
				return
			default:
			}

			line := scanner.Text()

			if strings.HasPrefix(line, ":") {
				continue // SSE comment / heartbeat
			}

			if strings.HasPrefix(line, "event:") {
				sseEventType = strings.TrimSpace(strings.TrimPrefix(line, "event:"))
				continue
			}

			if strings.HasPrefix(line, "data:") {
				data := strings.TrimPrefix(line, "data:")
				dataLines = append(dataLines, strings.TrimPrefix(data, " "))
				continue
			}

			if line == "" {
				if len(dataLines) > 0 {
					payload := strings.Join(dataLines, "\n")
					dataLines = dataLines[:0]
					evtType := sseEventType
					sseEventType = ""

					var event MemoryEvent
					if jsonErr := json.Unmarshal([]byte(payload), &event); jsonErr != nil {
						select {
						case ch <- MemoryEventResult{Err: fmt.Errorf("dakera: SSE parse error: %w", jsonErr)}:
						case <-ctx.Done():
							return
						}
						continue
					}
					// Populate EventType from the SSE event: field when the JSON
					// payload does not carry it (e.g. the connected handshake).
					if event.EventType == "" && evtType != "" {
						event.EventType = evtType
					}
					select {
					case ch <- MemoryEventResult{Event: &event}:
					case <-ctx.Done():
						return
					}
				} else {
					sseEventType = ""
				}
			}
		}

		if err := scanner.Err(); err != nil && ctx.Err() == nil {
			select {
			case ch <- MemoryEventResult{Err: fmt.Errorf("dakera: SSE read error: %w", err)}:
			case <-ctx.Done():
			}
		}
	}()

	return ch, nil
}

// streamSSE opens an SSE connection and pumps events into a channel.
func (c *Client) streamSSE(ctx context.Context, path string) (<-chan EventResult, error) {
	resp, err := c.openStream(ctx, path)
	if err != nil {
		return nil, err
	}

	ch := make(chan EventResult, 64)

	go func() {
		defer close(ch)
		defer resp.Body.Close()

		scanner := newSSEScanner(resp.Body)
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

		if err := scanner.Err(); err != nil && ctx.Err() == nil {
			select {
			case ch <- EventResult{Err: fmt.Errorf("dakera: SSE read error: %w", err)}:
			case <-ctx.Done():
			}
		}
	}()

	return ch, nil
}
