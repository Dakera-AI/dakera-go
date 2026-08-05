package dakera

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ===========================================================================
// StreamGlobalEvents — previously untested (DAK-7692)
// ===========================================================================

func TestStreamGlobalEvents_ConnectedEvent(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/ops/events", r.URL.Path)
		assert.Equal(t, "text/event-stream", r.Header.Get("Accept"))

		w.Header().Set("Content-Type", "text/event-stream")
		w.Header().Set("Cache-Control", "no-cache")
		w.WriteHeader(http.StatusOK)
		if f, ok := w.(http.Flusher); ok {
			fmt.Fprintf(w, "data: {\"type\":\"connected\",\"timestamp\":1700000000001}\n\n")
			f.Flush()
		}
		<-r.Context().Done()
	}))
	defer server.Close()

	client := NewClient(server.URL)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	ch, err := client.StreamGlobalEvents(ctx)
	require.NoError(t, err)

	result := <-ch
	cancel()

	require.NoError(t, result.Err)
	require.NotNil(t, result.Event)
	assert.Equal(t, "connected", result.Event.Type)
	assert.Equal(t, int64(1700000000001), result.Event.Timestamp)
}

func TestStreamGlobalEvents_RequiresAdminScope(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusUnauthorized)
		json.NewEncoder(w).Encode(map[string]interface{}{"error": "Admin scope required"})
	}))
	defer server.Close()

	client := NewClientWithOptions(ClientOptions{BaseURL: server.URL, MaxRetries: 1})
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	_, err := client.StreamGlobalEvents(ctx)
	require.Error(t, err)
}

func TestStreamGlobalEvents_ChannelClosesOnContextCancel(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		w.Header().Set("Cache-Control", "no-cache")
		w.WriteHeader(http.StatusOK)
		<-r.Context().Done()
	}))
	defer server.Close()

	client := NewClient(server.URL)
	ctx, cancel := context.WithCancel(context.Background())

	ch, err := client.StreamGlobalEvents(ctx)
	require.NoError(t, err)

	// Cancel immediately; channel must eventually close.
	cancel()
	select {
	case _, ok := <-ch:
		assert.False(t, ok, "channel should be closed after context cancel")
	case <-time.After(2 * time.Second):
		t.Fatal("channel was not closed within 2s after context cancel")
	}
}

// ===========================================================================
// Error-path coverage for methods with happy-path-only tests (DAK-7692)
// ===========================================================================

func TestConsolidateAgent_Error(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]interface{}{"error": "consolidation failed"})
	}))
	defer server.Close()
	client := NewClientWithOptions(ClientOptions{BaseURL: server.URL, MaxRetries: 1})
	_, err := client.ConsolidateAgent(context.Background(), "agent-42")
	require.Error(t, err)
}

func TestGetConsolidationLog_Error(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusForbidden)
		json.NewEncoder(w).Encode(map[string]interface{}{"error": "forbidden"})
	}))
	defer server.Close()
	client := NewClientWithOptions(ClientOptions{BaseURL: server.URL, MaxRetries: 1})
	_, err := client.GetConsolidationLog(context.Background(), "agent-42")
	require.Error(t, err)
}

func TestSummarize_Error(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]interface{}{"error": "requires at least 2 memory IDs"})
	}))
	defer server.Close()
	client := NewClientWithOptions(ClientOptions{BaseURL: server.URL, MaxRetries: 1})
	_, err := client.Summarize(context.Background(), SummarizeRequest{
		AgentID:   "agent-42",
		MemoryIDs: []string{"m1"}, // deliberately too few
	})
	require.Error(t, err)
}

func TestDeduplicate_SendsThreshold(t *testing.T) {
	var capturedBody map[string]interface{}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "POST", r.Method)
		require.NoError(t, json.NewDecoder(r.Body).Decode(&capturedBody))
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"duplicates_found":  float64(5),
			"duplicates_merged": float64(5),
			"groups":            []interface{}{},
		})
	}))
	defer server.Close()
	client := NewClient(server.URL)
	threshold := float32(0.90)
	dryRun := true
	resp, err := client.Deduplicate(context.Background(), DeduplicateRequest{
		AgentID:   "agent-42",
		Threshold: &threshold,
		DryRun:    dryRun,
	})
	require.NoError(t, err)
	assert.Equal(t, 5, resp.DuplicatesFound)
	assert.InDelta(t, 0.90, capturedBody["threshold"], 0.001)
	assert.Equal(t, true, capturedBody["dry_run"])
}

func TestKnowledgeGraph_Error(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		json.NewEncoder(w).Encode(map[string]interface{}{"error": "agent not found"})
	}))
	defer server.Close()
	client := NewClientWithOptions(ClientOptions{BaseURL: server.URL, MaxRetries: 1})
	_, err := client.KnowledgeGraph(context.Background(), KnowledgeGraphRequest{AgentID: "missing"})
	require.Error(t, err)
}

func TestGetWakeUpContext_WithMinImportance(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "GET", r.Method)
		assert.Contains(t, r.URL.Path, "/agents/agent-1/wake-up")
		minImp := r.URL.Query().Get("min_importance")
		assert.NotEmpty(t, minImp)
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"agent_id":       "agent-1",
			"memories_count": float64(3),
			"memories":       []interface{}{},
		})
	}))
	defer server.Close()
	client := NewClient(server.URL)
	minImp := float32(0.8)
	resp, err := client.GetWakeUpContext(context.Background(), "agent-1", &WakeUpOptions{
		MinImportance: &minImp,
	})
	require.NoError(t, err)
	assert.Equal(t, "agent-1", resp.AgentID)
}

func TestCrossAgentNetwork_Error(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusServiceUnavailable)
		json.NewEncoder(w).Encode(map[string]interface{}{"error": "service unavailable"})
	}))
	defer server.Close()
	client := NewClientWithOptions(ClientOptions{BaseURL: server.URL, MaxRetries: 1})
	_, err := client.CrossAgentNetwork(context.Background(), CrossAgentNetworkRequest{AgentIDs: []string{"a1", "a2"}})
	require.Error(t, err)
}
