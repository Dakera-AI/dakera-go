package dakera

import (
	"context"
)

// ChatMemorySession is a high-level session helper for LLM chat comparison patterns.
//
// It groups conversation turns under a single Dakera session so that:
//   - Every stored message is associated with the session ID for scoped retrieval.
//   - Recall queries the agent's full memory (not just this session) so prior
//     conversations can inform the current exchange.
//
// Use [NewChatMemorySession] to create a session.
//
// Example:
//
//	session, err := dakera.NewChatMemorySession(ctx, client, "chat-agent", nil)
//	if err != nil { /* handle */ }
//	defer session.Close(ctx)
//
//	session.Store(ctx, "user", "My name is Alice and I like Go.")
//	memories, _ := session.Recall(ctx, "user preferences", 5)
//	// pass memories to your LLM — or skip for the baseline arm
type ChatMemorySession struct {
	client    *Client
	agentID   string
	sessionID string
}

// ChatMemorySessionOptions customises [ChatMemorySession.StoreWithOptions] behaviour.
type ChatMemorySessionOptions struct {
	// Importance is the importance score (0.0–1.0) for the stored memory.
	// Defaults to 0.6 when zero.
	Importance float32
	// ExtraTags are additional tags appended alongside the role tag.
	ExtraTags []string
}

// NewChatMemorySession creates a new Dakera session and returns a [ChatMemorySession].
//
// metadata may be nil.
func NewChatMemorySession(ctx context.Context, client *Client, agentID string, metadata map[string]interface{}) (*ChatMemorySession, error) {
	session, err := client.StartSession(ctx, StartSessionRequest{
		AgentID:  agentID,
		Metadata: metadata,
	})
	if err != nil {
		return nil, err
	}
	return &ChatMemorySession{
		client:    client,
		agentID:   agentID,
		sessionID: session.ID,
	}, nil
}

// Store persists a conversation turn in the session with default importance (0.6).
//
// The role (e.g. "user" or "assistant") is appended to the memory's tags automatically.
func (s *ChatMemorySession) Store(ctx context.Context, role, content string) (*StoreMemoryResponse, error) {
	return s.StoreWithOptions(ctx, role, content, ChatMemorySessionOptions{})
}

// StoreWithOptions persists a conversation turn with custom importance and tags.
func (s *ChatMemorySession) StoreWithOptions(ctx context.Context, role, content string, opts ChatMemorySessionOptions) (*StoreMemoryResponse, error) {
	importance := opts.Importance
	if importance == 0 {
		importance = 0.6
	}

	// Deduplicate role into tags.
	tags := []string{role}
	for _, t := range opts.ExtraTags {
		if t != role {
			tags = append(tags, t)
		}
	}

	memType := "episodic"
	imp := importance
	return s.client.StoreMemory(ctx, s.agentID, StoreMemoryRequest{
		Content:    content,
		MemoryType: memType,
		Importance: &imp,
		Tags:       tags,
		SessionID:  s.sessionID,
	})
}

// Recall returns up to topK memories relevant to query for this agent.
//
// It searches the agent's full memory — not just the current session — so
// context from prior conversations can inform the current exchange.
// Pass topK ≤ 0 to use the default of 5.
func (s *ChatMemorySession) Recall(ctx context.Context, query string, topK int) ([]RecalledMemory, error) {
	if topK <= 0 {
		topK = 5
	}
	resp, err := s.client.Recall(ctx, s.agentID, RecallRequest{
		Query: query,
		TopK:  topK,
	})
	if err != nil {
		return nil, err
	}
	return resp.Memories, nil
}

// Close ends the Dakera session.
func (s *ChatMemorySession) Close(ctx context.Context) (*SessionEndResponse, error) {
	return s.client.EndSession(ctx, s.sessionID)
}

// SessionID returns the underlying Dakera session ID.
func (s *ChatMemorySession) SessionID() string { return s.sessionID }

// AgentID returns the agent ID this session is bound to.
func (s *ChatMemorySession) AgentID() string { return s.agentID }
