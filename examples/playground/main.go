// Example: Dakera Go SDK — Playground Quickstart
//
// Demonstrates the 4 core memory operations against the Dakera Playground.
//
// Run:
//
//	go run ./examples/playground
package main

import (
	"context"
	"fmt"
	"log"
	"os"

	dakera "github.com/dakera-ai/dakera-go"
)

const agentID = "playground-agent"

func playgroundURL() string {
	if u := os.Getenv("DAKERA_API_URL"); u != "" {
		return u
	}
	return "https://5-75-177-31.sslip.io"
}

func playgroundKey() string {
	if k := os.Getenv("DAKERA_API_KEY"); k != "" {
		return k
	}
	return "playground-demo"
}

func main() {
	client := dakera.NewClientWithOptions(dakera.ClientOptions{
		BaseURL: playgroundURL(),
		APIKey:  playgroundKey(),
	})
	ctx := context.Background()

	health, err := client.Health(ctx)
	if err != nil {
		log.Fatalf("health check failed: %v", err)
	}
	fmt.Printf("Playground: status=%s version=%s\n", health.Status, health.Version)

	// -------------------------------------------------------------------------
	// 1. Store memories
	// -------------------------------------------------------------------------
	fmt.Println("\n--- 1. Store Memories ---")

	imp1 := float32(0.9)
	mem1, err := client.StoreMemory(ctx, agentID, dakera.StoreMemoryRequest{
		Content:    "Dakera provides persistent, decay-weighted memory for AI agents.",
		MemoryType: "semantic",
		Importance: &imp1,
		Tags:       []string{"dakera", "memory", "overview"},
	})
	if err != nil {
		log.Fatalf("StoreMemory failed: %v", err)
	}
	fmt.Printf("Stored: %s\n", mem1.Memory.ID)

	imp2 := float32(0.8)
	mem2, err := client.StoreMemory(ctx, agentID, dakera.StoreMemoryRequest{
		Content:    "The recall API returns semantically similar memories ranked by relevance.",
		MemoryType: "semantic",
		Importance: &imp2,
		Tags:       []string{"dakera", "recall", "api"},
	})
	if err != nil {
		log.Fatalf("StoreMemory failed: %v", err)
	}
	fmt.Printf("Stored: %s\n", mem2.Memory.ID)

	imp3 := float32(0.7)
	mem3, err := client.StoreMemory(ctx, agentID, dakera.StoreMemoryRequest{
		Content:    "Session scoping lets agents isolate memories per task or conversation.",
		MemoryType: "episodic",
		Importance: &imp3,
		Tags:       []string{"sessions", "isolation"},
	})
	if err != nil {
		log.Fatalf("StoreMemory failed: %v", err)
	}
	fmt.Printf("Stored: %s\n", mem3.Memory.ID)

	// -------------------------------------------------------------------------
	// 2. Recall by query (semantic search)
	// -------------------------------------------------------------------------
	fmt.Println("\n--- 2. Recall by Query ---")

	recallResp, err := client.Recall(ctx, agentID, dakera.RecallRequest{
		Query: "How does Dakera memory work?",
		TopK:  5,
	})
	if err != nil {
		log.Fatalf("Recall failed: %v", err)
	}
	fmt.Printf("Recalled %d memories:\n", len(recallResp.Memories))
	for _, m := range recallResp.Memories {
		content := m.Content
		if len(content) > 80 {
			content = content[:80]
		}
		fmt.Printf("  [%.3f] %s\n", m.Score, content)
	}

	// -------------------------------------------------------------------------
	// 3. Search with filters (type=semantic)
	// -------------------------------------------------------------------------
	fmt.Println("\n--- 3. Search with Filters ---")

	searched, err := client.SearchMemories(ctx, agentID, dakera.SearchMemoriesRequest{
		Query:      "memory API",
		MemoryType: "semantic",
		TopK:       3,
	})
	if err != nil {
		log.Printf("SearchMemories error (may not be available): %v", err)
	} else {
		fmt.Printf("Filtered search (%d results):\n", len(searched))
		for _, m := range searched {
			content := m.Content
			if len(content) > 80 {
				content = content[:80]
			}
			fmt.Printf("  [%.3f] %s\n", m.Score, content)
		}
	}

	// -------------------------------------------------------------------------
	// 4. Knowledge graph link
	// Note: requires a full Dakera account; not available on the public sandbox.
	// -------------------------------------------------------------------------
	fmt.Println("\n--- 4. Knowledge Graph Link ---")

	linkResp, err := client.MemoryLink(ctx, mem1.Memory.ID, mem2.Memory.ID, dakera.EdgeTypeRelatedTo)
	if err != nil {
		log.Printf("KG link not available in sandbox: %v", err)
		fmt.Println("  Sign up at https://dakera.ai for full knowledge graph access.")
	} else {
		fmt.Printf("Linked %s → %s: edge_type=%s\n",
			mem1.Memory.ID, mem2.Memory.ID, linkResp.Edge.EdgeType)
	}

	fmt.Printf("\nPlayground quickstart complete! Visit https://dakera.ai to learn more.\n")

	_ = mem3 // stored but not linked (demonstrate store without link)
}
