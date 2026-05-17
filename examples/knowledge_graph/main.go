// Example: Dakera Go SDK — Knowledge Graph (build, traverse, query, path, export, entity extraction)
package main

import (
	"context"
	"fmt"
	"log"
	"os"

	dakera "github.com/dakera-ai/dakera-go"
)

func dakeraURL() string {
	if u := os.Getenv("DAKERA_API_URL"); u != "" {
		return u
	}
	return "http://localhost:3300"
}

func dakeraAPIKey() string {
	if k := os.Getenv("DAKERA_API_KEY"); k != "" {
		return k
	}
	return "dk-mykey"
}

func main() {
	client := dakera.NewClientWithOptions(dakera.ClientOptions{
		BaseURL: dakeraURL(),
		APIKey:  dakeraAPIKey(),
	})
	ctx := context.Background()

	agentID := "kg-demo-agent"

	// -------------------------------------------------------------------------
	// Store some memories to build a graph from
	// -------------------------------------------------------------------------
	fmt.Println("--- Store Memories for KG ---")

	memories := []dakera.StoreMemoryRequest{
		{Content: "Alice is the CTO of Acme Corp and leads the engineering team.", MemoryType: "semantic"},
		{Content: "Bob reports to Alice and manages the backend infrastructure.", MemoryType: "semantic"},
		{Content: "Acme Corp uses Kubernetes for container orchestration in production.", MemoryType: "episodic"},
		{Content: "The backend team migrated from PostgreSQL to CockroachDB last quarter.", MemoryType: "episodic"},
		{Content: "Alice approved the budget for the new observability platform.", MemoryType: "semantic"},
	}

	var memoryIDs []string
	for i, req := range memories {
		resp, err := client.StoreMemory(ctx, agentID, req)
		if err != nil {
			log.Fatalf("StoreMemory[%d] failed: %v", i, err)
		}
		memoryIDs = append(memoryIDs, resp.Memory.ID)
		fmt.Printf("  Stored: %s\n", resp.Memory.ID)
	}

	if len(memoryIDs) < 5 {
		log.Fatalf("unexpected: expected 5 memory IDs, got %d", len(memoryIDs))
	}

	// -------------------------------------------------------------------------
	// Build Knowledge Graph
	// -------------------------------------------------------------------------
	fmt.Println("\n--- Build Knowledge Graph ---")

	depth := 3
	minSim := float32(0.3)
	kgResp, err := client.KnowledgeGraph(ctx, dakera.KnowledgeGraphRequest{
		AgentID:       agentID,
		Depth:         &depth,
		MinSimilarity: &minSim,
	})
	if err != nil {
		log.Fatalf("KnowledgeGraph failed: %v", err)
	}
	if kgResp == nil {
		log.Fatalf("unexpected: knowledge graph response is nil")
	}
	fmt.Printf("Graph: %d nodes, %d edges, %d clusters\n",
		len(kgResp.Nodes), len(kgResp.Edges), len(kgResp.Clusters))

	for _, node := range kgResp.Nodes {
		fmt.Printf("  Node %s: %s\n", node.ID, truncate(node.Content, 60))
	}

	// -------------------------------------------------------------------------
	// Graph Traverse (from a specific memory)
	// -------------------------------------------------------------------------
	fmt.Println("\n--- Graph Traverse ---")

	traverseOpts := &dakera.GraphOptions{
		Depth: 2,
	}
	graph, err := client.MemoryGraph(ctx, memoryIDs[0], traverseOpts)
	if err != nil {
		log.Fatalf("MemoryGraph failed: %v", err)
	}
	if graph == nil {
		log.Fatalf("unexpected: memory graph is nil")
	}
	fmt.Printf("Traversal from %s: %d nodes, %d edges\n",
		memoryIDs[0], len(graph.Nodes), len(graph.Edges))

	// -------------------------------------------------------------------------
	// Knowledge Query (edge-based)
	// -------------------------------------------------------------------------
	fmt.Println("\n--- Knowledge Query ---")

	queryResp, err := client.KnowledgeQuery(ctx, agentID, "", "", 0.0, 3, 20)
	if err != nil {
		log.Fatalf("KnowledgeQuery failed: %v", err)
	}
	if queryResp == nil {
		log.Fatalf("unexpected: knowledge query response is nil")
	}
	fmt.Printf("Query result: %d nodes, %d edges\n", queryResp.NodeCount, queryResp.EdgeCount)
	for _, edge := range queryResp.Edges {
		fmt.Printf("  %s -[%s]-> %s (weight: %.2f)\n",
			edge.SourceID, edge.EdgeType, edge.TargetID, edge.Weight)
	}

	// -------------------------------------------------------------------------
	// Knowledge Path
	// -------------------------------------------------------------------------
	fmt.Println("\n--- Knowledge Path ---")

	if len(memoryIDs) >= 2 {
		pathResp, err := client.KnowledgePath(ctx, agentID, memoryIDs[0], memoryIDs[3])
		if err != nil {
			log.Fatalf("KnowledgePath failed: %v", err)
		}
		if pathResp == nil {
			log.Fatalf("unexpected: path response is nil")
		}
		fmt.Printf("Path from %s to %s: %d hops\n",
			pathResp.FromID, pathResp.ToID, pathResp.HopCount)
		fmt.Printf("  Path: %v\n", pathResp.Path)
	}

	// -------------------------------------------------------------------------
	// Knowledge Export
	// -------------------------------------------------------------------------
	fmt.Println("\n--- Knowledge Export ---")

	exportResp, err := client.KnowledgeExport(ctx, agentID, "json")
	if err != nil {
		log.Fatalf("KnowledgeExport failed: %v", err)
	}
	if exportResp == nil {
		log.Fatalf("unexpected: export response is nil")
	}
	fmt.Printf("Export: format=%s, nodes=%d, edges=%d\n",
		exportResp.Format, exportResp.NodeCount, exportResp.EdgeCount)

	// -------------------------------------------------------------------------
	// Entity Extraction (ODE)
	// -------------------------------------------------------------------------
	fmt.Println("\n--- Entity Extraction ---")

	entityResp, err := client.OdeExtractEntities(ctx, dakera.ExtractEntitiesRequest{
		Content: "Alice from Acme Corp approved the Kubernetes migration project with Bob.",
		AgentID: agentID,
	})
	if err != nil {
		log.Fatalf("OdeExtractEntities failed: %v", err)
	}
	if entityResp == nil {
		log.Fatalf("unexpected: entity extraction response is nil")
	}
	fmt.Printf("Extracted %d entities (model: %s, %dms)\n",
		len(entityResp.Entities), entityResp.Model, entityResp.ProcessingTimeMs)
	for _, ent := range entityResp.Entities {
		fmt.Printf("  [%s] %q (score: %.3f, span: %d-%d)\n",
			ent.Label, ent.Text, ent.Score, ent.Start, ent.End)
	}

	fmt.Println("\nDone!")
}

func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen-3] + "..."
}
