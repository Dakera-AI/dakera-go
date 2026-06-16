package dakera_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	dakera "github.com/dakera-ai/dakera-go"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newTestClient(baseURL string) *dakera.Client {
	return dakera.NewClientWithOptions(dakera.ClientOptions{BaseURL: baseURL, APIKey: "test-key"})
}

func sessionStartHandler(sessionID, agentID string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"session": map[string]interface{}{
				"id":           sessionID,
				"agent_id":     agentID,
				"started_at":   0,
				"memory_count": 0,
			},
		})
	}
}

func TestNewChatMemorySession(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/v1/sessions/start", sessionStartHandler("sess-chat-001", "agent-chat"))
	srv := httptest.NewServer(mux)
	defer srv.Close()

	session, err := dakera.NewChatMemorySession(context.Background(), newTestClient(srv.URL), "agent-chat", nil)
	require.NoError(t, err)
	assert.Equal(t, "sess-chat-001", session.SessionID())
	assert.Equal(t, "agent-chat", session.AgentID())
}

func TestChatMemorySession_Store(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/v1/sessions/start", sessionStartHandler("sess-002", "agent-x"))
	mux.HandleFunc("/v1/memory/store", func(w http.ResponseWriter, r *http.Request) {
		var body map[string]interface{}
		json.NewDecoder(r.Body).Decode(&body)
		tags, _ := body["tags"].([]interface{})
		hasUserTag := false
		for _, tag := range tags {
			if tag == "user" {
				hasUserTag = true
			}
		}
		assert.True(t, hasUserTag, "role tag must be present in tags")

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"memory": map[string]interface{}{
				"id": "mem-001", "content": "Hello from Go!", "agent_id": "agent-x",
			},
		})
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	session, err := dakera.NewChatMemorySession(context.Background(), newTestClient(srv.URL), "agent-x", nil)
	require.NoError(t, err)

	resp, err := session.Store(context.Background(), "user", "Hello from Go!")
	require.NoError(t, err)
	assert.NotNil(t, resp)
}

func TestChatMemorySession_Recall(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/v1/sessions/start", sessionStartHandler("sess-003", "agent-y"))
	mux.HandleFunc("/v1/memory/recall", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"memories": []map[string]interface{}{
				{"memory": map[string]interface{}{"id": "m1", "content": "Go is great"}, "score": 0.9},
			},
		})
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	session, err := dakera.NewChatMemorySession(context.Background(), newTestClient(srv.URL), "agent-y", nil)
	require.NoError(t, err)

	memories, err := session.Recall(context.Background(), "Go language", 5)
	require.NoError(t, err)
	assert.Len(t, memories, 1)
}

func TestChatMemorySession_DefaultTopK(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/v1/sessions/start", sessionStartHandler("sess-004", "agent-z"))
	mux.HandleFunc("/v1/memory/recall", func(w http.ResponseWriter, r *http.Request) {
		var body map[string]interface{}
		json.NewDecoder(r.Body).Decode(&body)
		topK, _ := body["top_k"].(float64)
		assert.Equal(t, float64(5), topK, "default top_k must be 5")
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{"memories": []interface{}{}})
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	session, err := dakera.NewChatMemorySession(context.Background(), newTestClient(srv.URL), "agent-z", nil)
	require.NoError(t, err)

	_, err = session.Recall(context.Background(), "anything", 0) // topK=0 → default 5
	require.NoError(t, err)
}

func TestChatMemorySession_StoreWithOptions(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/v1/sessions/start", sessionStartHandler("sess-005", "agent-opts"))
	mux.HandleFunc("/v1/memory/store", func(w http.ResponseWriter, r *http.Request) {
		var body map[string]interface{}
		json.NewDecoder(r.Body).Decode(&body)

		// Custom importance must be sent.
		imp, _ := body["importance"].(float64)
		assert.InDelta(t, 0.9, imp, 0.001, "custom importance must be 0.9")

		// Tags must include role + extra tag but not duplicate the role.
		tags, _ := body["tags"].([]interface{})
		tagSet := make(map[string]bool)
		for _, tag := range tags {
			tagSet[tag.(string)] = true
		}
		assert.True(t, tagSet["assistant"], "role tag 'assistant' must be present")
		assert.True(t, tagSet["important"], "extra tag 'important' must be present")
		// Role tag must not appear twice.
		count := 0
		for _, tag := range tags {
			if tag == "assistant" {
				count++
			}
		}
		assert.Equal(t, 1, count, "role tag must not be duplicated")

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"memory": map[string]interface{}{
				"id": "mem-opts", "content": "custom opts", "agent_id": "agent-opts",
			},
		})
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	session, err := dakera.NewChatMemorySession(context.Background(), newTestClient(srv.URL), "agent-opts", nil)
	require.NoError(t, err)

	resp, err := session.StoreWithOptions(context.Background(), "assistant", "custom opts", dakera.ChatMemorySessionOptions{
		Importance: 0.9,
		ExtraTags:  []string{"important", "assistant"}, // "assistant" is a dupe — must be deduped
	})
	require.NoError(t, err)
	assert.NotNil(t, resp)
}

func TestChatMemorySession_Close(t *testing.T) {
	sessionEndCalled := false
	mux := http.NewServeMux()
	mux.HandleFunc("/v1/sessions/start", sessionStartHandler("sess-006", "agent-close"))
	mux.HandleFunc("/v1/sessions/sess-006/end", func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodPost, r.Method)
		sessionEndCalled = true
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"session": map[string]interface{}{
				"id": "sess-006", "agent_id": "agent-close", "ended_at": 9999,
			},
		})
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	session, err := dakera.NewChatMemorySession(context.Background(), newTestClient(srv.URL), "agent-close", nil)
	require.NoError(t, err)

	resp, err := session.Close(context.Background())
	require.NoError(t, err)
	assert.NotNil(t, resp)
	assert.True(t, sessionEndCalled, "EndSession must be called on Close()")
}
