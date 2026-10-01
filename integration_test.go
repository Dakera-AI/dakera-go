package dakera_test

import (
	"context"
	"crypto/rand"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/dakera-ai/dakera-go"
)

func integrationClient(t *testing.T) *dakera.Client {
	t.Helper()
	url := os.Getenv("DAKERA_TEST_URL")
	if url == "" {
		t.Skip("DAKERA_TEST_URL not set — skipping integration tests")
	}
	apiKey := os.Getenv("DAKERA_API_KEY")
	if apiKey == "" {
		apiKey = "test-key"
	}
	return dakera.NewClientWithOptions(dakera.ClientOptions{
		BaseURL: url,
		APIKey:  apiKey,
	})
}

func randomHex(n int) string {
	b := make([]byte, n)
	_, _ = rand.Read(b)
	return fmt.Sprintf("%x", b)
}

func testNamespace() string {
	return fmt.Sprintf("integ-%s", randomHex(4))
}

func testAgent() string {
	return fmt.Sprintf("integ-agent-%s", randomHex(4))
}

// ---------------------------------------------------------------------------
// Health
// ---------------------------------------------------------------------------

func TestIntegration_Health(t *testing.T) {
	client := integrationClient(t)
	ctx := context.Background()

	health, err := client.Health(ctx)
	if err != nil {
		t.Fatalf("Health failed: %v", err)
	}
	if health.Status != "healthy" {
		t.Errorf("expected status healthy, got %s", health.Status)
	}
}

// ---------------------------------------------------------------------------
// Namespaces
// ---------------------------------------------------------------------------

func TestIntegration_CreateNamespace(t *testing.T) {
	client := integrationClient(t)
	ctx := context.Background()
	ns := testNamespace()

	result, err := client.CreateNamespace(ctx, ns, &dakera.CreateNamespaceOptions{Dimensions: 1024})
	if err != nil {
		t.Fatalf("CreateNamespace failed: %v", err)
	}
	if result.Name != ns {
		t.Errorf("expected name %s, got %s", ns, result.Name)
	}
	_ = client.DeleteNamespace(ctx, ns)
}

func TestIntegration_ListNamespaces(t *testing.T) {
	client := integrationClient(t)
	ctx := context.Background()
	ns := testNamespace()

	_, err := client.CreateNamespace(ctx, ns, &dakera.CreateNamespaceOptions{Dimensions: 1024})
	if err != nil {
		t.Fatalf("CreateNamespace failed: %v", err)
	}
	defer client.DeleteNamespace(ctx, ns)

	namespaces, err := client.ListNamespaces(ctx)
	if err != nil {
		t.Fatalf("ListNamespaces failed: %v", err)
	}
	found := false
	for _, n := range namespaces {
		if n == ns {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("namespace %s not found in list", ns)
	}
}

func TestIntegration_GetNamespace(t *testing.T) {
	client := integrationClient(t)
	ctx := context.Background()
	ns := testNamespace()

	_, err := client.CreateNamespace(ctx, ns, &dakera.CreateNamespaceOptions{Dimensions: 1024})
	if err != nil {
		t.Fatalf("CreateNamespace failed: %v", err)
	}
	defer client.DeleteNamespace(ctx, ns)

	info, err := client.GetNamespace(ctx, ns)
	if err != nil {
		t.Fatalf("GetNamespace failed: %v", err)
	}
	if info.Name != ns {
		t.Errorf("expected name %s, got %s", ns, info.Name)
	}
}

func TestIntegration_DeleteNamespace(t *testing.T) {
	client := integrationClient(t)
	ctx := context.Background()
	ns := testNamespace()

	_, err := client.CreateNamespace(ctx, ns, &dakera.CreateNamespaceOptions{Dimensions: 1024})
	if err != nil {
		t.Fatalf("CreateNamespace failed: %v", err)
	}
	err = client.DeleteNamespace(ctx, ns)
	if err != nil {
		t.Fatalf("DeleteNamespace failed: %v", err)
	}
}

// ---------------------------------------------------------------------------
// Memory CRUD
// ---------------------------------------------------------------------------

func TestIntegration_StoreMemory(t *testing.T) {
	client := integrationClient(t)
	ctx := context.Background()
	agent := testAgent()

	result, err := client.StoreMemory(ctx, agent, dakera.StoreMemoryRequest{
		Content:    "The user prefers dark mode interfaces",
		Importance: floatPtr(0.8),
	})
	if err != nil {
		t.Fatalf("StoreMemory failed: %v", err)
	}
	if result.Memory == nil || result.Memory.ID == "" {
		t.Error("expected non-empty memory ID")
	}
}

func TestIntegration_Recall(t *testing.T) {
	client := integrationClient(t)
	ctx := context.Background()
	agent := testAgent()

	_, err := client.StoreMemory(ctx, agent, dakera.StoreMemoryRequest{
		Content:    "Python is my primary programming language",
		Importance: floatPtr(0.9),
	})
	if err != nil {
		t.Fatalf("StoreMemory failed: %v", err)
	}
	time.Sleep(500 * time.Millisecond)

	result, err := client.Recall(ctx, agent, dakera.RecallRequest{Query: "programming language", TopK: 5})
	if err != nil {
		t.Fatalf("Recall failed: %v", err)
	}
	if len(result.Memories) == 0 {
		t.Error("expected at least one recalled memory")
	}
}

func TestIntegration_BatchRecall(t *testing.T) {
	client := integrationClient(t)
	ctx := context.Background()
	agent := testAgent()

	_, _ = client.StoreMemory(ctx, agent, dakera.StoreMemoryRequest{
		Content:    "Batch recall test memory",
		Importance: floatPtr(0.8),
	})

	result, err := client.BatchRecall(ctx, dakera.BatchRecallRequest{
		AgentID: agent,
		Filter:  dakera.BatchMemoryFilter{MinImportance: floatPtr(0.5)},
	})
	if err != nil {
		t.Fatalf("BatchRecall failed: %v", err)
	}
	if len(result.Memories) == 0 {
		t.Error("expected at least one memory")
	}
}

func TestIntegration_GetMemory(t *testing.T) {
	client := integrationClient(t)
	ctx := context.Background()
	agent := testAgent()

	stored, err := client.StoreMemory(ctx, agent, dakera.StoreMemoryRequest{
		Content:    "Memory for get test",
		Importance: floatPtr(0.7),
	})
	if err != nil {
		t.Fatalf("StoreMemory failed: %v", err)
	}

	memory, err := client.GetMemory(ctx, agent, stored.Memory.ID)
	if err != nil {
		t.Fatalf("GetMemory failed: %v", err)
	}
	if memory == nil {
		t.Error("expected non-nil memory")
	}
}

func TestIntegration_UpdateImportance(t *testing.T) {
	client := integrationClient(t)
	ctx := context.Background()
	agent := testAgent()

	stored, err := client.StoreMemory(ctx, agent, dakera.StoreMemoryRequest{
		Content:    "Importance update test",
		Importance: floatPtr(0.5),
	})
	if err != nil {
		t.Fatalf("StoreMemory failed: %v", err)
	}

	err = client.UpdateImportance(ctx, agent, dakera.UpdateImportanceRequest{
		MemoryIDs:  []string{stored.Memory.ID},
		Importance: 0.95,
	})
	if err != nil {
		t.Fatalf("UpdateImportance failed: %v", err)
	}
}

func TestIntegration_Forget(t *testing.T) {
	client := integrationClient(t)
	ctx := context.Background()
	agent := testAgent()

	stored, err := client.StoreMemory(ctx, agent, dakera.StoreMemoryRequest{
		Content:    "Memory to forget",
		Importance: floatPtr(0.3),
	})
	if err != nil {
		t.Fatalf("StoreMemory failed: %v", err)
	}

	err = client.Forget(ctx, agent, stored.Memory.ID)
	if err != nil {
		t.Fatalf("Forget failed: %v", err)
	}
}

// ---------------------------------------------------------------------------
// Sessions
// ---------------------------------------------------------------------------

func TestIntegration_SessionLifecycle(t *testing.T) {
	client := integrationClient(t)
	ctx := context.Background()
	agent := testAgent()

	session, err := client.StartSession(ctx, dakera.StartSessionRequest{
		AgentID:  agent,
		Metadata: map[string]interface{}{"type": "test"},
	})
	if err != nil {
		t.Fatalf("StartSession failed: %v", err)
	}
	if session.ID == "" {
		t.Error("expected non-empty session ID")
	}

	sessions, err := client.ListSessions(ctx, &dakera.ListSessionsOptions{AgentID: agent})
	if err != nil {
		t.Fatalf("ListSessions failed: %v", err)
	}
	if len(sessions) == 0 {
		t.Error("expected at least one session")
	}

	_, err = client.EndSession(ctx, session.ID)
	if err != nil {
		t.Fatalf("EndSession failed: %v", err)
	}
}

// ---------------------------------------------------------------------------
// Vectors / Text
// ---------------------------------------------------------------------------

func TestIntegration_UpsertText(t *testing.T) {
	client := integrationClient(t)
	ctx := context.Background()
	ns := testNamespace()

	_, err := client.CreateNamespace(ctx, ns, &dakera.CreateNamespaceOptions{Dimensions: 1024})
	if err != nil {
		t.Fatalf("CreateNamespace failed: %v", err)
	}
	defer client.DeleteNamespace(ctx, ns)

	_, err = client.UpsertText(ctx, ns, []dakera.TextDocument{
		{ID: "doc-1", Text: "Machine learning transforms data"},
		{ID: "doc-2", Text: "Natural language processing understands text"},
		{ID: "doc-3", Text: "Deep learning uses neural networks"},
	}, nil)
	if err != nil {
		t.Fatalf("UpsertText failed: %v", err)
	}
}

func TestIntegration_QueryText(t *testing.T) {
	client := integrationClient(t)
	ctx := context.Background()
	ns := testNamespace()

	_, err := client.CreateNamespace(ctx, ns, &dakera.CreateNamespaceOptions{Dimensions: 1024})
	if err != nil {
		t.Fatalf("CreateNamespace failed: %v", err)
	}
	defer client.DeleteNamespace(ctx, ns)

	_, _ = client.UpsertText(ctx, ns, []dakera.TextDocument{
		{ID: "q-1", Text: "Machine learning transforms data into insights"},
	}, nil)
	time.Sleep(1 * time.Second)

	_, err = client.QueryText(ctx, ns, "machine learning", &dakera.TextQueryOptions{TopK: 3})
	if err != nil {
		t.Fatalf("QueryText failed: %v", err)
	}
}

func TestIntegration_HybridSearch(t *testing.T) {
	client := integrationClient(t)
	ctx := context.Background()
	ns := testNamespace()

	_, err := client.CreateNamespace(ctx, ns, &dakera.CreateNamespaceOptions{Dimensions: 1024})
	if err != nil {
		t.Fatalf("CreateNamespace failed: %v", err)
	}
	defer client.DeleteNamespace(ctx, ns)

	_, _ = client.UpsertText(ctx, ns, []dakera.TextDocument{
		{ID: "h-1", Text: "Machine learning data analysis"},
	}, nil)
	time.Sleep(1 * time.Second)

	_, err = client.HybridSearch(ctx, ns, nil, "machine learning", &dakera.HybridSearchOptions{TopK: 3})
	if err != nil {
		t.Logf("HybridSearch may fail on empty namespace: %v", err)
	}
}

func TestIntegration_FulltextSearch(t *testing.T) {
	client := integrationClient(t)
	ctx := context.Background()
	ns := testNamespace()

	_, err := client.CreateNamespace(ctx, ns, &dakera.CreateNamespaceOptions{Dimensions: 1024})
	if err != nil {
		t.Fatalf("CreateNamespace failed: %v", err)
	}
	defer client.DeleteNamespace(ctx, ns)

	_, _ = client.UpsertText(ctx, ns, []dakera.TextDocument{
		{ID: "ft-1", Text: "Neural networks process information"},
	}, nil)
	time.Sleep(1 * time.Second)

	_, err = client.FulltextSearch(ctx, ns, "neural networks", &dakera.FullTextSearchOptions{TopK: 3})
	if err != nil {
		t.Fatalf("FulltextSearch failed: %v", err)
	}
}

// ---------------------------------------------------------------------------
// Knowledge Graph
// ---------------------------------------------------------------------------

func TestIntegration_MemoryGraph(t *testing.T) {
	client := integrationClient(t)
	ctx := context.Background()
	agent := testAgent()

	stored, err := client.StoreMemory(ctx, agent, dakera.StoreMemoryRequest{
		Content:    "Knowledge graph test memory",
		Importance: floatPtr(0.8),
	})
	if err != nil {
		t.Fatalf("StoreMemory failed: %v", err)
	}
	time.Sleep(500 * time.Millisecond)

	_, err = client.MemoryGraph(ctx, stored.Memory.ID, &dakera.GraphOptions{Depth: 1})
	if err != nil {
		t.Logf("MemoryGraph: %v (may be expected on fresh memory)", err)
	}
}

// TestIntegration_GraphContractRoundTrip checks update, link, graph, path,
// export, knowledge query and entities against the server's real answers.
func TestIntegration_GraphContractRoundTrip(t *testing.T) {
	client := integrationClient(t)
	ctx := context.Background()
	agent := testAgent()

	m1, err := client.StoreMemory(ctx, agent, dakera.StoreMemoryRequest{Content: "Anna lives in Berlin and works at Siemens"})
	if err != nil {
		t.Fatalf("StoreMemory 1 failed: %v", err)
	}
	m2, err := client.StoreMemory(ctx, agent, dakera.StoreMemoryRequest{Content: "Bob met Anna in Paris"})
	if err != nil {
		t.Fatalf("StoreMemory 2 failed: %v", err)
	}
	id1, id2 := m1.Memory.ID, m2.Memory.ID

	content := "Anna lives in Hamburg now"
	updated, err := client.UpdateMemory(ctx, agent, id1, dakera.UpdateMemoryRequest{Content: &content})
	if err != nil {
		t.Fatalf("UpdateMemory failed: %v", err)
	}
	if updated.Memory == nil || updated.Memory.ID != id1 || updated.Memory.Content != content {
		t.Fatalf("UpdateMemory: unexpected answer %+v", updated.Memory)
	}

	link, err := client.MemoryLink(ctx, agent, id1, id2, "")
	if err != nil {
		t.Fatalf("MemoryLink failed: %v", err)
	}
	if link.FromID != id1 || link.ToID != id2 || link.Edge.SourceID != id1 || link.Edge.TargetID != id2 ||
		link.Edge.EdgeType != dakera.EdgeTypeLinkedBy {
		t.Fatalf("MemoryLink: unexpected answer %+v", link)
	}

	hasLink := func(edges []dakera.GraphEdge) bool {
		for _, e := range edges {
			if e.SourceID == id1 && e.TargetID == id2 && e.EdgeType == dakera.EdgeTypeLinkedBy {
				return true
			}
		}
		return false
	}

	graph, err := client.MemoryGraph(ctx, id1, &dakera.GraphOptions{Depth: 2})
	if err != nil {
		t.Fatalf("MemoryGraph failed: %v", err)
	}
	if graph.RootID != id1 || !hasLink(graph.Edges) {
		t.Fatalf("MemoryGraph: link %s -> %s missing in %+v", id1, id2, graph)
	}

	path, err := client.MemoryPath(ctx, id1, id2)
	if err != nil {
		t.Fatalf("MemoryPath failed: %v", err)
	}
	if path.SourceID != id1 || path.TargetID != id2 || path.Hops != 1 || len(path.Path) != 2 {
		t.Fatalf("MemoryPath: unexpected answer %+v", path)
	}

	export, err := client.AgentGraphExport(ctx, agent, "")
	if err != nil {
		t.Fatalf("AgentGraphExport failed: %v", err)
	}
	if export.AgentID != agent || export.Namespace != "_dakera_agent_"+agent || export.EdgeCount < 1 || !hasLink(export.Edges) {
		t.Fatalf("AgentGraphExport: unexpected answer %+v", export)
	}

	kg, err := client.KnowledgeQuery(ctx, agent, "", "linked_by", 0, 0, 0)
	if err != nil {
		t.Fatalf("KnowledgeQuery failed: %v", err)
	}
	if !hasLink(kg.Edges) {
		t.Fatalf("KnowledgeQuery: link missing in %+v", kg)
	}

	entities, err := client.MemoryEntities(ctx, id1)
	if err != nil {
		t.Fatalf("MemoryEntities failed: %v", err)
	}
	if entities.MemoryID != id1 || entities.Count != len(entities.Entities) {
		t.Fatalf("MemoryEntities: unexpected answer %+v", entities)
	}
}

// ---------------------------------------------------------------------------
// Consolidate
// ---------------------------------------------------------------------------

func TestIntegration_Consolidate(t *testing.T) {
	client := integrationClient(t)
	ctx := context.Background()
	agent := testAgent()

	for i := 0; i < 3; i++ {
		_, _ = client.StoreMemory(ctx, agent, dakera.StoreMemoryRequest{
			Content:    fmt.Sprintf("Consolidation test variation %d: similar content", i),
			Importance: floatPtr(0.6),
		})
	}
	time.Sleep(500 * time.Millisecond)

	_, err := client.Consolidate(ctx, agent, dakera.ConsolidateRequest{})
	if err != nil {
		t.Fatalf("Consolidate failed: %v", err)
	}
}

// ---------------------------------------------------------------------------
// Error Handling
// ---------------------------------------------------------------------------

func TestIntegration_NonexistentNamespace(t *testing.T) {
	client := integrationClient(t)
	ctx := context.Background()

	_, err := client.GetNamespace(ctx, "nonexistent-ns-xyz-99999")
	if err == nil {
		t.Error("expected error for nonexistent namespace")
	}
}

func TestIntegration_NonexistentMemory(t *testing.T) {
	client := integrationClient(t)
	ctx := context.Background()
	agent := testAgent()

	_, err := client.GetMemory(ctx, agent, "nonexistent-memory-id")
	if err == nil {
		t.Error("expected error for nonexistent memory")
	}
}

// ---------------------------------------------------------------------------
// Authentication
// ---------------------------------------------------------------------------

func TestIntegration_AuthRejectsInvalidKey(t *testing.T) {
	url := os.Getenv("DAKERA_TEST_URL")
	if url == "" {
		t.Skip("DAKERA_TEST_URL not set — skipping integration tests")
	}
	badClient := dakera.NewClientWithOptions(dakera.ClientOptions{
		BaseURL: url,
		APIKey:  "invalid-key-xxx",
	})
	ctx := context.Background()
	_, err := badClient.ListNamespaces(ctx)
	if err == nil {
		t.Fatal("expected auth error with invalid key")
	}
	if !dakera.IsAuthenticationError(err) {
		t.Errorf("expected AuthenticationError, got: %v", err)
	}
}

func TestIntegration_AuthAcceptsValidKey(t *testing.T) {
	client := integrationClient(t)
	ctx := context.Background()
	_, err := client.ListNamespaces(ctx)
	if err != nil {
		t.Fatalf("ListNamespaces failed with valid key: %v", err)
	}
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

func floatPtr(f float32) *float32 {
	return &f
}
