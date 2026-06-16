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
