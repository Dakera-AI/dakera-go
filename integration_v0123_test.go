package dakera_test

// Integration tests for server v0.12.3 behaviour. They skip on an older
// server (CI pins an earlier image) and when DAKERA_TEST_URL is not set.

import (
	"context"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/dakera-ai/dakera-go"
)

// versionAtLeast reports whether version ("0.12.3", "v0.12.3-rc1") is at
// least major.minor.patch.
func versionAtLeast(version string, want ...int) bool {
	version = strings.TrimPrefix(strings.TrimSpace(version), "v")
	parts := strings.SplitN(version, ".", 3)
	for i, w := range want {
		if i >= len(parts) {
			return false
		}
		digits := parts[i]
		if end := strings.IndexFunc(digits, func(r rune) bool { return r < '0' || r > '9' }); end >= 0 {
			digits = digits[:end]
		}
		n, err := strconv.Atoi(digits)
		if err != nil {
			return false
		}
		if n != w {
			return n > w
		}
	}
	return true
}

// integrationClient0123 is integrationClient on a v0.12.3+ server; it skips
// the test otherwise.
func integrationClient0123(t *testing.T) *dakera.Client {
	t.Helper()
	client := integrationClient(t)
	caps, err := client.Capabilities(context.Background())
	if err != nil {
		t.Skipf("capabilities unavailable (%v) — skipping v0.12.3 tests", err)
	}
	if !versionAtLeast(caps.ServerVersion, 0, 12, 3) {
		t.Skipf("server %s is older than 0.12.3 — skipping", caps.ServerVersion)
	}
	return client
}

func TestVersionAtLeast(t *testing.T) {
	cases := map[string]bool{
		"0.12.3": true, "v0.12.3": true, "0.12.4": true, "0.13.0": true, "1.0.0": true,
		"0.12.3-rc1": true, "0.12.2": false, "0.12.0": false, "0.11.99": false, "": false, "0.12": false,
	}
	for version, want := range cases {
		if got := versionAtLeast(version, 0, 12, 3); got != want {
			t.Errorf("versionAtLeast(%q) = %v, want %v", version, got, want)
		}
	}
}

func TestIntegration_V0123_EmptyAgentAnswersEmpty(t *testing.T) {
	client := integrationClient0123(t)
	ctx := context.Background()
	agent := testAgent()

	recall, err := client.Recall(ctx, agent, dakera.RecallRequest{Query: "anything", TopK: 5})
	if err != nil {
		t.Fatalf("Recall for an empty agent: %v", err)
	}
	if len(recall.Memories) != 0 {
		t.Errorf("expected no memories, got %d", len(recall.Memories))
	}
	search, err := client.SearchMemories(ctx, agent, dakera.SearchMemoriesRequest{Query: "anything"})
	if err != nil {
		t.Fatalf("SearchMemories for an empty agent: %v", err)
	}
	if len(search) != 0 {
		t.Errorf("expected no search results, got %d", len(search))
	}
	batch, err := client.BatchRecall(ctx, dakera.BatchRecallRequest{AgentID: agent})
	if err != nil {
		t.Fatalf("BatchRecall for an empty agent: %v", err)
	}
	if len(batch.Memories) != 0 {
		t.Errorf("expected no batch memories, got %d", len(batch.Memories))
	}
}

func TestIntegration_V0123_EventTime(t *testing.T) {
	client := integrationClient0123(t)
	ctx := context.Background()
	agent := testAgent()

	stored, err := client.StoreMemory(ctx, agent, dakera.StoreMemoryRequest{
		Content:   "Alice moved to Paris.",
		EventTime: dakera.EventTimeUnix(1684549260),
	})
	if err != nil {
		t.Fatalf("StoreMemory with EventTime: %v", err)
	}
	if got, ok := stored.Memory.Metadata["_dakera_event_time"].(float64); !ok || int64(got) != 1684549260 {
		t.Errorf("expected metadata _dakera_event_time 1684549260, got %v", stored.Memory.Metadata["_dakera_event_time"])
	}

	batch, err := client.StoreMemoriesBatch(ctx, dakera.BatchStoreMemoryRequest{
		AgentID: agent,
		Memories: []dakera.BatchStoreMemoryItem{{
			Content:   "Bob joined the team.",
			EventTime: dakera.EventTimeAt(time.Date(2023, 5, 20, 2, 21, 0, 0, time.UTC)),
		}},
	})
	if err != nil {
		t.Fatalf("StoreMemoriesBatch with EventTime: %v", err)
	}
	if len(batch.Stored) != 1 {
		t.Fatalf("expected 1 stored memory, got %d", len(batch.Stored))
	}
	got, err := client.GetMemory(ctx, agent, batch.Stored[0].ID)
	if err != nil {
		t.Fatalf("GetMemory: %v", err)
	}
	if v, ok := got.Metadata["_dakera_event_time"].(float64); !ok || int64(v) != 1684549260 {
		t.Errorf("expected batch item _dakera_event_time 1684549260, got %v", got.Metadata["_dakera_event_time"])
	}
}

// Every optional field the SDK can send is one the server knows: the
// answers carry no warnings.
func TestIntegration_V0123_NoWarningsWithEveryOptionalField(t *testing.T) {
	client := integrationClient0123(t)
	ctx := context.Background()
	agent := testAgent()
	session := "chat:" + randomHex(4)
	ttl := 86400
	expires := time.Now().Add(240 * time.Hour).Unix()
	validFrom := int64(1684549260)

	stored, err := client.StoreMemory(ctx, agent, dakera.StoreMemoryRequest{
		Content:    "Alice moved to Paris in May 2023.",
		MemoryType: "episodic",
		Importance: floatPtr(0.8),
		Tags:       []string{"move"},
		Metadata:   map[string]interface{}{"source": "import"},
		TTLSeconds: &ttl,
		ExpiresAt:  &expires,
		SessionID:  session,
		ValidFrom:  &validFrom,
		ID:         "v0123-" + randomHex(4),
		Lang:       "en",
		EventTime:  dakera.EventTimeUnix(1684549260),
	})
	if err != nil {
		t.Fatalf("StoreMemory: %v", err)
	}
	if len(stored.Warnings) != 0 {
		t.Errorf("store warnings: %v", stored.Warnings)
	}
	if stored.Memory.SessionID != session {
		t.Errorf("session id with ':' not kept: %q", stored.Memory.SessionID)
	}

	ttl64 := int64(ttl)
	batch, err := client.StoreMemoriesBatch(ctx, dakera.BatchStoreMemoryRequest{
		AgentID: agent,
		Lang:    "en",
		Memories: []dakera.BatchStoreMemoryItem{{
			Content: "Dana visited Rome.", MemoryType: "episodic", Importance: floatPtr(0.6),
			Tags: []string{"trip"}, SessionID: session, Metadata: map[string]any{"k": "v"},
			TTLSeconds: &ttl64, ExpiresAt: &expires, ID: "v0123-b-" + randomHex(4),
			EventTime: dakera.EventTimeRFC3339("2023-06-01T00:00:00Z"),
		}},
	})
	if err != nil {
		t.Fatalf("StoreMemoriesBatch: %v", err)
	}
	if len(batch.Warnings) != 0 {
		t.Errorf("batch store warnings: %v", batch.Warnings)
	}

	routing := dakera.RoutingModeHybrid
	fusion := dakera.FusionStrategyMinMax
	iterations := uint8(2)
	cap5, depth, candidates := 5, 2, 50
	since, until := "2023-01-01T00:00:00Z", "2030-01-01T00:00:00Z"
	weighted, rerank, neighborhood, summaries := true, true, true, false
	recall, err := client.Recall(ctx, agent, dakera.RecallRequest{
		Query: "where did Alice move", TopK: 5, MemoryType: "episodic", MinImportance: floatPtr(0.1),
		SessionID: session, Tags: []string{"move"}, ImportanceWeighted: &weighted,
		IncludeAssociated: true, AssociatedMemoriesCap: &cap5, AssociatedMemoriesDepth: &depth,
		AssociatedMemoriesMinWeight: floatPtr(0.1), Since: &since, Until: &until, Routing: &routing,
		Rerank: &rerank, RerankCandidates: &candidates, Fusion: &fusion, VectorWeight: floatPtr(0.6),
		Iterations: &iterations, Neighborhood: &neighborhood, NeighborhoodMode: dakera.NeighborhoodModeSeparate,
		IncludeSummaries: &summaries, Lang: "en",
	})
	if err != nil {
		t.Fatalf("Recall: %v", err)
	}
	if len(recall.Warnings) != 0 {
		t.Errorf("recall warnings: %v", recall.Warnings)
	}

	created := int64(1)
	batchRecall, err := client.BatchRecall(ctx, dakera.BatchRecallRequest{
		AgentID: agent,
		Filter: dakera.BatchMemoryFilter{
			Tags: []string{"trip"}, MinImportance: floatPtr(0.1), MaxImportance: floatPtr(1),
			CreatedAfter: &created, CreatedBefore: &expires, MemoryType: "episodic", SessionID: session,
		},
		Limit: 10,
	})
	if err != nil {
		t.Fatalf("BatchRecall: %v", err)
	}
	if len(batchRecall.Warnings) != 0 {
		t.Errorf("batch recall warnings: %v", batchRecall.Warnings)
	}
}

func TestIntegration_V0123_Deduplicated(t *testing.T) {
	client := integrationClient0123(t)
	ctx := context.Background()
	agent := testAgent()

	list, err := client.ListDeduplicated(ctx, agent)
	if err != nil {
		t.Fatalf("ListDeduplicated: %v", err)
	}
	if list.Count != 0 || len(list.Memories) != 0 {
		t.Errorf("expected no deduplicated memories for a fresh agent, got %d", list.Count)
	}

	_, err = client.RestoreDeduplicated(ctx, agent, "no-such-memory-"+randomHex(4))
	if !dakera.IsNotFoundError(err) {
		t.Errorf("expected a NotFoundError restoring an unknown id, got %v", err)
	}
}

func TestIntegration_V0123_GetMemoryWithoutAgentID(t *testing.T) {
	client := integrationClient0123(t)
	ctx := context.Background()
	agent := testAgent()

	stored, err := client.StoreMemory(ctx, agent, dakera.StoreMemoryRequest{
		Content: "Memory found without its agent id",
		ID:      "v0123-get-" + randomHex(6),
	})
	if err != nil {
		t.Fatalf("StoreMemory: %v", err)
	}
	got, err := client.GetMemory(ctx, "", stored.Memory.ID)
	if err != nil {
		t.Fatalf("GetMemory without agent id: %v", err)
	}
	if got.ID != stored.Memory.ID || got.AgentID != agent {
		t.Errorf("expected memory %s of %s, got %s of %s", stored.Memory.ID, agent, got.ID, got.AgentID)
	}
}
